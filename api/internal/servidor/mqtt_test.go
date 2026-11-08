package servidor

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/broker"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

func TestAutorizarMQTT(t *testing.T) {
	s := &Servidor{}
	const a, b = "A1B2C3D4E5F6", "0123456789AB"
	casos := []struct {
		usuario, topico string
		publicar, ok    bool
	}{
		{a, "bicho/" + a + "/state", true, true},
		{a, "bicho/" + a + "/resp", true, true},
		{a, "bicho/" + a + "/conexion", true, true},
		{a, "bicho/" + a + "/charla", true, true}, // v1.9: lo que le dicen al robot-celu
		{a, "bicho/" + b + "/charla", true, false},
		{a, "bicho/+/charla", true, false},
		{a, "bicho/" + a + "/charla", false, false},
		{a, "bicho/" + a + "/presencia", true, true}, // v1.9: el robot-celu te vio llegar
		{a, "bicho/" + b + "/presencia", true, false},
		{a, "bicho/" + a + "/conversacion", true, true}, // ElevenLabs: pide la URL firmada
		{a, "bicho/" + b + "/conversacion", true, false},
		{a, "bicho/" + a + "/conversacion/url", false, true}, // y la recibe acá
		{a, "bicho/" + b + "/conversacion/url", false, false},
		{a, "bicho/" + a + "/conversacion/url", true, false},
		{a, "bicho/" + a + "/audio", false, true}, // el audio de la voz
		{a, "bicho/" + b + "/audio", false, false},
		{a, "bicho/" + a + "/audio", true, false}, // el micrófono, cuando haya (M7): todavía no
		{a, "bicho/+/audio", false, false},
		{a, "bicho/" + a + "/cmd", true, false}, // no se manda comandos a sí mismo
		{a, "bicho/" + a + "/otro", true, false},
		{a, "bicho/" + b + "/state", true, false},
		{a, "bicho/" + b + "/conexion", true, false},
		{a, "bicho/" + a + "/cmd", false, true},
		{a, "bicho/" + a + "/state", false, false},
		{a, "bicho/" + b + "/cmd", false, false},
		{a, "bicho/+/cmd", false, false},
		{a, "bicho/#", false, false},
		{a, "#", false, false},
		{"api", "bicho/api/cmd", false, false}, // la API no pasa por acá: es superusuario
		{"a1b2c3d4e5f6", "bicho/a1b2c3d4e5f6/cmd", false, false},
	}
	for _, c := range casos {
		if got := s.AutorizarMQTT(c.usuario, c.topico, c.publicar); got != c.ok {
			t.Errorf("%s %s publicar=%v: %v, quiero %v", c.usuario, c.topico, c.publicar, got, c.ok)
		}
	}
}

// El broker de verdad con la autenticación y la autorización de la API y la
// nube suscripta en proceso (como en cmd/sintetica-api).
func TestBrokerConLaAPI(t *testing.T) {
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	var s *Servidor
	brk, err := broker.Nuevo(broker.Opciones{
		TCP: "127.0.0.1:0",
		Autenticar: func(ctx context.Context, usuario, clave, id string) bool {
			return s.AutenticarMQTT(ctx, usuario, clave, id)
		},
		Autorizar:    func(usuario, topico string, publicar bool) bool { return s.AutorizarMQTT(usuario, topico, publicar) },
		Superusuario: "api",
		Log:          log,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(brk.Cerrar)
	n := nube.Nuevo(nube.Opciones{Broker: brk}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{}, alm, n, hub, log)
	n.Escuchar(i)
	s = Nuevo(Opciones{MQTTUsuario: "api", MQTTClave: "clave-api"}, alm, n, i, hub, nil, nil, log)
	if err := n.Conectar(); err != nil {
		t.Fatal(err)
	}
	// La charla del robot-celu (v1.9): cada uno publica solo en el suyo.
	var charlasMu sync.Mutex
	var charlas []string
	if err := n.AlPublicar("charla", func(bicho string, datos []byte) {
		charlasMu.Lock()
		charlas = append(charlas, bicho+" "+string(datos))
		charlasMu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}
	if err := brk.Arrancar(); err != nil {
		t.Fatal(err)
	}
	_, bichos := cuentaDePrueba(t, alm) // la clave de cada uno es su id
	a, b := bichos[0], bichos[1]
	url := "tcp://" + brk.DireccionTCP

	conectar := func(id, usuario, clave string) (paho.Client, error) {
		o := paho.NewClientOptions().AddBroker(url).SetClientID(id).SetUsername(usuario).SetPassword(clave).
			SetAutoReconnect(false).SetConnectTimeout(3*time.Second).SetCleanSession(true).
			SetWill("bicho/"+id+"/conexion", "0", 1, true)
		c := paho.NewClient(o)
		tk := c.Connect()
		tk.WaitTimeout(5 * time.Second)
		if tk.Error() == nil {
			t.Cleanup(func() { c.Disconnect(50) })
		}
		return c, tk.Error()
	}
	if _, err := conectar(a, a, "mala"); err == nil {
		t.Error("entró con una clave mala")
	}
	if _, err := conectar(b, a, a); err == nil {
		t.Error("entró con el client id de otro")
	}
	if _, err := conectar("mirador", "api", "otra"); err == nil {
		t.Error("entró como la API con una clave mala")
	}
	ca, err := conectar(a, a, a)
	if err != nil {
		t.Fatalf("el bicho no entró: %v", err)
	}
	sub := func(c paho.Client, filtro string) byte {
		tk := c.Subscribe(filtro, 1, func(paho.Client, paho.Message) {})
		tk.WaitTimeout(3 * time.Second)
		return tk.(*paho.SubscribeToken).Result()[filtro]
	}
	if q := sub(ca, "bicho/"+a+"/cmd"); q != 1 {
		t.Errorf("su cmd: %#x", q)
	}
	if q := sub(ca, "bicho/"+b+"/cmd"); q != 0x80 {
		t.Errorf("el cmd de otro: %#x", q)
	}
	ca.Publish("bicho/"+b+"/conexion", 1, true, "1").WaitTimeout(3 * time.Second)
	ca.Publish("bicho/"+a+"/conexion", 1, true, "1").WaitTimeout(3 * time.Second)
	esperar := func(que string, f func() bool) {
		t.Helper()
		for limite := time.Now().Add(3 * time.Second); !f(); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(limite) {
				t.Fatalf("no pasó: %s", que)
			}
		}
	}
	esperar("la nube lo ve conectado", func() bool { return n.Conectado(a) })
	if n.Conectado(b) {
		t.Error("un bicho dijo que el otro está conectado")
	}

	ca.Publish("bicho/"+b+"/charla", 1, false, `{"texto":"soy otro"}`).WaitTimeout(3 * time.Second)
	ca.Publish("bicho/"+a+"/charla", 1, false, `{"texto":"hola"}`).WaitTimeout(3 * time.Second)
	esperar("llega la charla", func() bool { charlasMu.Lock(); defer charlasMu.Unlock(); return len(charlas) > 0 })
	charlasMu.Lock()
	if strings.Join(charlas, "|") != a+` {"texto":"hola"}` {
		t.Errorf("charlas: %q", charlas)
	}
	charlasMu.Unlock()

	// La API de afuera (superusuario) se suscribe a lo de cualquiera (los
	// comodines están probados en internal/broker).
	api, err := conectar("mirador", "api", "clave-api")
	if err != nil {
		t.Fatalf("la API no entró: %v", err)
	}
	if q := sub(api, "bicho/"+b+"/cmd"); q != 1 {
		t.Errorf("superusuario: %#x", q)
	}

	// Se le revoca la clave: se lo echa y la nube se entera al instante.
	n.Expulsar(a)
	esperar("la nube lo ve desconectado", func() bool { return !n.Conectado(a) })
	esperar("el cliente se cortó", func() bool { return !ca.IsConnectionOpen() })
}
