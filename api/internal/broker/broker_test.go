package broker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	bichoA = "A1B2C3D4E5F6"
	bichoB = "0123456789AB"
)

// Las mismas reglas que la API (servidor.AutorizarMQTT), para probar el broker solo.
func autorizar(usuario, topico string, publicar bool) bool {
	resto, ok := strings.CutPrefix(topico, "bicho/"+usuario+"/")
	if !ok {
		return false
	}
	if publicar {
		return resto == "state" || resto == "resp" || resto == "conexion"
	}
	return resto == "cmd"
}

func autenticar(_ context.Context, usuario, clave, clientID string) bool {
	switch usuario {
	case "api":
		return clave == "clave-api"
	case bichoA, bichoB:
		return clientID == usuario && clave == "clave-"+usuario
	}
	return false
}

func nuevoBroker(t *testing.T, op Opciones) *Broker {
	t.Helper()
	if op.Autenticar == nil {
		op.Autenticar = autenticar
	}
	op.Autorizar = autorizar
	op.Superusuario = "api"
	op.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	b, err := Nuevo(op)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Cerrar)
	return b
}

func arrancar(t *testing.T, b *Broker) {
	t.Helper()
	if err := b.Arrancar(); err != nil {
		t.Fatal(err)
	}
}

func opciones(url, id, usuario, clave string) *paho.ClientOptions {
	return paho.NewClientOptions().AddBroker(url).SetClientID(id).SetUsername(usuario).SetPassword(clave).
		SetAutoReconnect(false).SetConnectTimeout(3 * time.Second).SetKeepAlive(30 * time.Second).SetCleanSession(true).
		SetProtocolVersion(4)
}

func conectar(t *testing.T, o *paho.ClientOptions) (paho.Client, error) {
	t.Helper()
	c := paho.NewClient(o)
	tk := c.Connect()
	if !tk.WaitTimeout(5 * time.Second) {
		return c, io.ErrNoProgress
	}
	if tk.Error() == nil {
		t.Cleanup(func() { c.Disconnect(50) })
	}
	return c, tk.Error()
}

func debeConectar(t *testing.T, o *paho.ClientOptions) paho.Client {
	t.Helper()
	c, err := conectar(t, o)
	if err != nil {
		t.Fatalf("no conectó %s: %v", o.ClientID, err)
	}
	return c
}

func suscribir(t *testing.T, c paho.Client, filtro string, f paho.MessageHandler) byte {
	t.Helper()
	tk := c.Subscribe(filtro, 1, f)
	if !tk.WaitTimeout(3*time.Second) || tk.Error() != nil {
		t.Fatalf("suscribir %s: %v", filtro, tk.Error())
	}
	return tk.(*paho.SubscribeToken).Result()[filtro]
}

func publicar(t *testing.T, c paho.Client, topico string, retener bool, datos string) {
	t.Helper()
	tk := c.Publish(topico, 1, retener, datos)
	if !tk.WaitTimeout(3*time.Second) || tk.Error() != nil {
		t.Fatalf("publicar %s: %v", topico, tk.Error())
	}
}

// buzon junta lo que le llega a una suscripción de la API (en proceso).
type buzon struct {
	mu   sync.Mutex
	msjs []string
	hay  chan struct{}
}

func nuevoBuzon() *buzon { return &buzon{hay: make(chan struct{}, 100)} }

func (b *buzon) recibir(topico string, datos []byte) {
	b.mu.Lock()
	b.msjs = append(b.msjs, topico+"="+string(datos))
	b.mu.Unlock()
	b.hay <- struct{}{}
}

func (b *buzon) esperar(t *testing.T, quiero string) {
	t.Helper()
	limite := time.After(3 * time.Second)
	for {
		b.mu.Lock()
		for _, m := range b.msjs {
			if m == quiero {
				b.mu.Unlock()
				return
			}
		}
		b.mu.Unlock()
		select {
		case <-b.hay:
		case <-limite:
			t.Fatalf("no llegó %q (llegaron %v)", quiero, b.todos())
		}
	}
}

func (b *buzon) todos() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.msjs...)
}

func TestAutenticacion(t *testing.T) {
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0"})
	arrancar(t, b)
	url := "tcp://" + b.DireccionTCP

	debeConectar(t, opciones(url, bichoA, bichoA, "clave-"+bichoA))
	debeConectar(t, opciones(url, "cualquiera", "api", "clave-api"))
	casos := map[string]*paho.ClientOptions{
		"clave mala":                 opciones(url, bichoB, bichoB, "otra"),
		"sin clave":                  opciones(url, bichoB, bichoB, ""),
		"client id de otro":          opciones(url, bichoA, bichoB, "clave-"+bichoB),
		"usuario que no existe":      opciones(url, "X", "nadie", "x"),
		"el id de la API en proceso": opciones(url, "inline", "api", "clave-api"),
	}
	for nombre, o := range casos {
		if _, err := conectar(t, o); err == nil {
			t.Errorf("%s: lo dejó entrar", nombre)
		}
	}
	if !b.Conectado(bichoA) || b.Conectado(bichoB) || b.Conectado("inline") {
		t.Error("Conectado no coincide")
	}
}

func TestAutorizacion(t *testing.T) {
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0"})
	api := nuevoBuzon()
	for _, f := range []string{"bicho/+/state", "bicho/+/resp", "bicho/+/conexion"} {
		if err := b.Suscribir(f, api.recibir); err != nil {
			t.Fatal(err)
		}
	}
	arrancar(t, b)
	url := "tcp://" + b.DireccionTCP

	a := debeConectar(t, opciones(url, bichoA, bichoA, "clave-"+bichoA))
	cmdA := make(chan string, 10)
	if q := suscribir(t, a, "bicho/"+bichoA+"/cmd", func(_ paho.Client, m paho.Message) { cmdA <- string(m.Payload()) }); q != 1 {
		t.Fatalf("su cmd: SUBACK %#x", q)
	}
	// Los de otro (o con comodines, o lo que no es cmd): 0x80 en MQTT 3.1.1.
	for _, f := range []string{"bicho/" + bichoB + "/cmd", "bicho/+/cmd", "bicho/#", "#", "bicho/" + bichoA + "/state", "$SYS/#"} {
		if q := suscribir(t, a, f, func(paho.Client, paho.Message) {}); q != 0x80 {
			t.Errorf("suscribirse a %s: SUBACK %#x, quiero 0x80", f, q)
		}
	}
	// Publicar en lo de otro: se ignora (con PUBACK, sin echarlo).
	publicar(t, a, "bicho/"+bichoB+"/state", true, "falso")
	publicar(t, a, "bicho/"+bichoB+"/conexion", true, "0")
	publicar(t, a, "bicho/"+bichoA+"/cmd", false, "a mí mismo")
	publicar(t, a, "bicho/"+bichoA+"/state", true, "mio")
	api.esperar(t, "bicho/"+bichoA+"/state=mio")
	for _, m := range api.todos() {
		if strings.Contains(m, bichoB) {
			t.Errorf("le llegó a la API algo publicado en lo de otro: %s", m)
		}
	}
	if !a.IsConnectionOpen() {
		t.Error("lo echó por publicar donde no debía (EMQX lo ignoraba)")
	}
	select {
	case m := <-cmdA:
		t.Errorf("se mandó un comando a sí mismo: %q", m)
	default:
	}

	// La API (en proceso) le manda un comando.
	if err := b.Publicar("bicho/"+bichoA+"/cmd", []byte("hola"), false, 1); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-cmdA:
		if m != "hola" {
			t.Errorf("comando: %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no le llegó el comando de la API")
	}

	// El superusuario de afuera puede todo.
	s := debeConectar(t, opciones(url, "mirador", "api", "clave-api"))
	estados := make(chan string, 10)
	if q := suscribir(t, s, "bicho/+/state", func(_ paho.Client, m paho.Message) { estados <- m.Topic() + "=" + string(m.Payload()) }); q != 1 {
		t.Fatalf("superusuario: SUBACK %#x", q)
	}
	select {
	case m := <-estados: // el retenido
		if m != "bicho/"+bichoA+"/state=mio" {
			t.Errorf("retenido: %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no le llegó el estado retenido")
	}
	publicar(t, s, "bicho/"+bichoA+"/cmd", false, "del superusuario")
	if m := <-cmdA; m != "del superusuario" {
		t.Errorf("comando del superusuario: %q", m)
	}
}

func TestConexionYExpulsion(t *testing.T) {
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0"})
	api := nuevoBuzon()
	if err := b.Suscribir("bicho/+/conexion", api.recibir); err != nil {
		t.Fatal(err)
	}
	arrancar(t, b)
	url := "tcp://" + b.DireccionTCP
	conexion := "bicho/" + bichoA + "/conexion"

	// Se va con DISCONNECT (sin última voluntad): igual se avisa el "0".
	o := opciones(url, bichoA, bichoA, "clave-"+bichoA).SetWill(conexion, "0", 1, true)
	a := debeConectar(t, o)
	publicar(t, a, conexion, true, "1")
	api.esperar(t, conexion+"=1")
	a.Disconnect(50)
	api.esperar(t, conexion+"=0")

	// Se reconecta antes de que se note el corte: la sesión vieja no avisa nada.
	api.msjs = nil
	viejo := debeConectar(t, opciones(url, bichoA, bichoA, "clave-"+bichoA).SetWill(conexion, "0", 1, true))
	publicar(t, viejo, conexion, true, "1")
	api.esperar(t, conexion+"=1")
	suscribir(t, viejo, "bicho/"+bichoA+"/cmd", func(paho.Client, paho.Message) {})
	nuevo := debeConectar(t, opciones(url, bichoA, bichoA, "clave-"+bichoA).SetWill(conexion, "0", 1, true))
	cmd := make(chan string, 1)
	suscribir(t, nuevo, "bicho/"+bichoA+"/cmd", func(_ paho.Client, m paho.Message) { cmd <- string(m.Payload()) })
	publicar(t, nuevo, conexion, true, "1")
	time.Sleep(300 * time.Millisecond)
	if m := api.todos(); m[len(m)-1] != conexion+"=1" {
		t.Errorf("después de reemplazar la sesión, lo último es %v", m)
	}
	if !b.Conectado(bichoA) {
		t.Fatal("no figura conectado")
	}
	// Y la sesión nueva conserva su suscripción.
	b.Publicar("bicho/"+bichoA+"/cmd", []byte("sigo"), false, 1) //nolint:errcheck
	select {
	case m := <-cmd:
		if m != "sigo" {
			t.Errorf("comando: %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("la sesión nueva perdió su suscripción")
	}

	// Echarlo (se le revocó la clave): se corta y se avisa.
	if !b.Expulsar(bichoA) {
		t.Fatal("Expulsar dice que no estaba")
	}
	api.esperar(t, conexion+"=0")
	if b.Expulsar(bichoB) || b.Expulsar("inline") {
		t.Error("Expulsar a uno que no está")
	}
	limite := time.Now().Add(3 * time.Second)
	for nuevo.IsConnectionOpen() && time.Now().Before(limite) {
		time.Sleep(20 * time.Millisecond)
	}
	if nuevo.IsConnectionOpen() {
		t.Error("sigue conectado después de expulsarlo")
	}
}

func TestTopeDeConexiones(t *testing.T) {
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0", TCPMax: 1})
	arrancar(t, b)
	url := "tcp://" + b.DireccionTCP
	a := debeConectar(t, opciones(url, bichoA, bichoA, "clave-"+bichoA))
	if _, err := conectar(t, opciones(url, bichoB, bichoB, "clave-"+bichoB)); err == nil {
		t.Fatal("pasó el tope")
	}
	a.Disconnect(50)
	time.Sleep(200 * time.Millisecond)
	debeConectar(t, opciones(url, bichoB, bichoB, "clave-"+bichoB))
}

func TestPaqueteGrande(t *testing.T) {
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0"})
	api := nuevoBuzon()
	if err := b.Suscribir("bicho/+/resp", api.recibir); err != nil {
		t.Fatal(err)
	}
	arrancar(t, b)
	a := debeConectar(t, opciones("tcp://"+b.DireccionTCP, bichoA, bichoA, "clave-"+bichoA))
	casi := strings.Repeat("x", 1<<20-100) // entra en 1 MB
	publicar(t, a, "bicho/"+bichoA+"/resp", false, casi)
	api.esperar(t, "bicho/"+bichoA+"/resp="+casi)
	a.Publish("bicho/"+bichoA+"/resp", 0, false, strings.Repeat("x", 1<<20+10)).WaitTimeout(time.Second)
	limite := time.Now().Add(3 * time.Second)
	for a.IsConnectionOpen() && time.Now().Before(limite) {
		time.Sleep(20 * time.Millisecond)
	}
	if a.IsConnectionOpen() {
		t.Error("aceptó un paquete de más de 1 MB")
	}
}

func TestAutenticarEspera(t *testing.T) {
	// La autenticación tiene su plazo (el request_timeout de EMQX).
	var plazo time.Duration
	b := nuevoBroker(t, Opciones{TCP: "127.0.0.1:0", Autenticar: func(ctx context.Context, u, c, id string) bool {
		d, _ := ctx.Deadline()
		plazo = time.Until(d)
		return autenticar(ctx, u, c, id)
	}})
	arrancar(t, b)
	debeConectar(t, opciones("tcp://"+b.DireccionTCP, bichoA, bichoA, "clave-"+bichoA))
	if plazo <= 0 || plazo > tiempoAutenticar {
		t.Errorf("plazo de la autenticación: %v", plazo)
	}
}

// --- TLS ---

func escribirCertificado(t *testing.T, dir, nombre string) *x509.Certificate {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serie, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	plantilla := &x509.Certificate{
		SerialNumber: serie, Subject: pkix.Name{CommonName: nombre}, DNSNames: []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:   time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}
	kder, _ := x509.MarshalECPrivateKey(clave)
	// Como lego: se escriben los dos de nuevo.
	if err := os.WriteFile(filepath.Join(dir, "mqtt.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mqtt.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return c
}

func TestTLS(t *testing.T) {
	dir := t.TempDir()
	primero := escribirCertificado(t, dir, "primero")
	if _, err := Nuevo(Opciones{TLS: "127.0.0.1:0", Autenticar: autenticar, Autorizar: autorizar}); err == nil {
		t.Error("arrancó el TLS sin certificado")
	}
	b := nuevoBroker(t, Opciones{TLS: "127.0.0.1:0", TLSMax: 10,
		Certificado: filepath.Join(dir, "mqtt.crt"), ClaveCert: filepath.Join(dir, "mqtt.key")})
	arrancar(t, b)
	if b.DireccionTCP != "" {
		t.Error("abrió el tcp sin pedirlo")
	}

	confiar := func(c *x509.Certificate, min, max uint16) *tls.Config {
		p := x509.NewCertPool()
		p.AddCert(c)
		return &tls.Config{RootCAs: p, ServerName: "localhost", MinVersion: min, MaxVersion: max}
	}
	url := "ssl://" + b.DireccionTLS
	for _, v := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		o := opciones(url, bichoA, bichoA, "clave-"+bichoA).SetTLSConfig(confiar(primero, v, v))
		c := debeConectar(t, o)
		c.Disconnect(50)
	}
	// TLS 1.1: no.
	if c, err := tls.Dial("tcp", b.DireccionTLS, confiar(primero, tls.VersionTLS10, tls.VersionTLS11)); err == nil {
		c.Close()
		t.Error("aceptó TLS 1.1")
	}
	// Una clave mala por TLS tampoco entra.
	if _, err := conectar(t, opciones(url, bichoA, bichoA, "mala").SetTLSConfig(confiar(primero, 0, 0))); err == nil {
		t.Error("entró con una clave mala por TLS")
	}

	// Lego renueva el certificado: el handshake siguiente ya usa el nuevo, sin reiniciar.
	time.Sleep(10 * time.Millisecond) // que cambie la fecha de los archivos
	segundo := escribirCertificado(t, dir, "segundo")
	c, err := tls.Dial("tcp", b.DireccionTLS, confiar(segundo, 0, 0))
	if err != nil {
		t.Fatalf("con el certificado renovado: %v", err)
	}
	if cn := c.ConnectionState().PeerCertificates[0].Subject.CommonName; cn != "segundo" {
		t.Errorf("certificado: %s", cn)
	}
	c.Close()

	// Un certificado a medio escribir: sigue con el último bueno.
	os.WriteFile(filepath.Join(dir, "mqtt.crt"), []byte("roto"), 0o644) //nolint:errcheck
	c, err = tls.Dial("tcp", b.DireccionTLS, confiar(segundo, 0, 0))
	if err != nil {
		t.Fatalf("con el certificado roto en el disco: %v", err)
	}
	c.Close()
}
