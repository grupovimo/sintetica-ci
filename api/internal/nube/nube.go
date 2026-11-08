// Package nube habla con los bichos por MQTT: recibe su estado, les manda
// comandos y mantiene sincronizadas la config y las imágenes entre lo que el
// dueño quiere (en la base) y lo que el bicho tiene (sincronización de tres
// vías: si los dos cambiaron, gana el último en hablar con la API).
package nube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// ErrDesconectado: el bicho no está conectado a la nube.
var ErrDesconectado = errors.New("el bicho no está conectado a la nube")

// Opciones de la conexión al broker.
type Opciones struct {
	URL, Usuario, Clave string
	// La API REST de EMQX, para echar a un bicho cuando se le revoca la clave.
	EmqxURL, EmqxClave string // clave = "id:secreto"
	// La URL de la API que ve cada bicho, para bajar las OTA.
	URLPublica, URLInterna string
	// Firma los links de descarga de las OTA.
	FirmarDescarga func(firmwareID int64, bicho string) string
}

// Nube es la conexión al broker y un trabajador por bicho.
type Nube struct {
	op     Opciones
	alm    *almacen.Almacen
	hub    *eventos.Hub
	log    *slog.Logger
	cli    mqtt.Client
	mu     sync.Mutex
	bichos map[string]*trabajador
	duenos sync.Map // id → usuario ("" = sin dueño)

	esperasMu sync.Mutex
	esperas   map[string][]chan struct{}

	ctx      context.Context
	cancelar context.CancelFunc

	oyente Oyente
}

// Oyente se entera de lo que les pasa a los bichos (las automatizaciones).
// Evento no puede bloquear: lo llama el trabajador del bicho.
type Oyente interface {
	// Evento: "conectado", "desconectado", "bateria-baja", "expresion",
	// "duerme", "despierta" y los del registro (ver nube.EventosRegistro).
	Evento(bicho, evento string, datos map[string]any)
	// QuiereRegistro: si hay que pedirle el registro al bicho (cada 10 s)
	// para enterarse de sus eventos (tacto, sacudida, recordatorio…).
	QuiereRegistro(bicho string) bool
}

// Escuchar suma el oyente (antes de Conectar).
func (n *Nube) Escuchar(o Oyente) { n.oyente = o }

func (n *Nube) avisar(bicho, evento string, datos map[string]any) {
	if n.oyente != nil {
		n.oyente.Evento(bicho, evento, datos)
	}
}

// Zona es la zona horaria de los bichos (la de Argentina, por ahora).
func Zona() *time.Location { return zonaBichos }

// Nuevo prepara la nube (no se conecta hasta Conectar).
func Nuevo(op Opciones, alm *almacen.Almacen, hub *eventos.Hub, log *slog.Logger) *Nube {
	ctx, cancelar := context.WithCancel(context.Background())
	return &Nube{op: op, alm: alm, hub: hub, log: log, bichos: map[string]*trabajador{}, esperas: map[string][]chan struct{}{}, ctx: ctx, cancelar: cancelar}
}

// Conectar se conecta al broker (y se reconecta solo).
func (n *Nube) Conectar() error {
	o := mqtt.NewClientOptions().AddBroker(n.op.URL).SetClientID(fmt.Sprintf("sintetica-api-%d", time.Now().UnixNano()%100000)).
		SetUsername(n.op.Usuario).SetPassword(n.op.Clave).SetAutoReconnect(true).SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).SetMaxReconnectInterval(10 * time.Second).SetKeepAlive(30 * time.Second).
		SetCleanSession(true).SetOrderMatters(true)
	o.SetOnConnectHandler(func(c mqtt.Client) {
		n.log.Info("conectado al broker", "url", n.op.URL)
		go n.suscribir(c)
	})
	o.SetConnectionLostHandler(func(_ mqtt.Client, err error) { n.log.Warn("se cortó el broker", "error", err) })
	n.cli = mqtt.NewClient(o)
	tk := n.cli.Connect()
	// Con ConnectRetry el token vuelve enseguida y sigue intentando atrás.
	if tk.WaitTimeout(15*time.Second) && tk.Error() != nil {
		return tk.Error()
	}
	go n.limpieza()
	return nil
}

// suscribir se suscribe a lo de los bichos. Si EMQX la rechaza (por ejemplo,
// porque al arrancar todavía no le podía preguntar a la API si la deja),
// reintenta: sin estas suscripciones la API no ve a ningún bicho.
func (n *Nube) suscribir(c mqtt.Client) {
	for {
		rechazadas := 0
		for _, t := range []string{"bicho/+/state", "bicho/+/resp", "bicho/+/conexion"} {
			tk := c.Subscribe(t, 1, n.alLlegar)
			ok := tk.WaitTimeout(10*time.Second) && tk.Error() == nil
			if st, es := tk.(*mqtt.SubscribeToken); ok && es && st.Result()[t] == 0x80 {
				ok = false // el SUBACK dice que no
			}
			if !ok {
				rechazadas++
				n.log.Warn("no me pude suscribir (reintento)", "topico", t, "error", tk.Error())
			}
		}
		if rechazadas == 0 {
			return
		}
		select {
		case <-n.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		if !c.IsConnectionOpen() {
			return // al reconectar se vuelve a llamar
		}
	}
}

// Cerrar se desconecta.
func (n *Nube) Cerrar() {
	n.cancelar()
	if n.cli != nil {
		n.cli.Disconnect(500)
	}
}

// BrokerConectado dice si la API está conectada al broker.
func (n *Nube) BrokerConectado() bool { return n.cli != nil && n.cli.IsConnectionOpen() }

func (n *Nube) limpieza() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-t.C:
			if err := n.alm.Limpiar(n.ctx); err != nil {
				n.log.Warn("limpieza", "error", err)
			}
		}
	}
}

func (n *Nube) alLlegar(_ mqtt.Client, m mqtt.Message) {
	partes := strings.Split(m.Topic(), "/")
	if len(partes) != 3 || partes[0] != "bicho" || !protocolo.IDValido(partes[1]) {
		return
	}
	t := n.trabajador(partes[1])
	d := append([]byte(nil), m.Payload()...)
	var e entrada
	switch partes[2] {
	case "state":
		e = entrada{tipo: entEstado, datos: d}
	case "conexion":
		// Retenido = lo que ya estaba cuando la API se suscribió (al arrancar):
		// no es que se acabe de conectar.
		e = entrada{tipo: entConexion, conectado: string(d) == "1", retenido: m.Retained()}
	case "resp":
		if len(d) < 2 {
			return
		}
		e = entrada{tipo: entRespuesta, car: d[0], datos: d[1:]}
	default:
		return
	}
	select {
	case t.entrada <- e:
	default:
		n.log.Warn("cola llena: pierdo un mensaje", "bicho", t.id, "topico", partes[2])
	}
}

func (n *Nube) trabajador(id string) *trabajador {
	n.mu.Lock()
	defer n.mu.Unlock()
	t := n.bichos[id]
	if t == nil {
		t = nuevoTrabajador(n, id)
		n.bichos[id] = t
		go t.correr()
	}
	return t
}

// Dueno devuelve el usuario dueño del bicho ("" si no tiene).
func (n *Nube) Dueno(id string) string {
	if v, ok := n.duenos.Load(id); ok {
		return v.(string)
	}
	b, err := n.alm.Bicho(n.ctx, id)
	d := ""
	if err == nil && b.UsuarioID != nil {
		d = *b.UsuarioID
	}
	if err == nil || errors.Is(err, almacen.ErrNoEncontrado) {
		n.duenos.Store(id, d)
	}
	return d
}

func (n *Nube) publicar(id string, e eventos.Evento) {
	e.Bicho = id
	n.hub.Publicar(n.Dueno(id), e)
}

// enviar publica un marco al bicho (QoS 1).
func (n *Nube) enviar(id string, car byte, marco []byte) error {
	if n.cli == nil || !n.cli.IsConnectionOpen() {
		return errors.New("la API no está conectada al broker")
	}
	tk := n.cli.Publish("bicho/"+id+"/cmd", 1, false, append([]byte{car}, marco...))
	if !tk.WaitTimeout(10 * time.Second) {
		return errors.New("el broker no confirmó")
	}
	return tk.Error()
}

// --- Para el servidor HTTP ---

// Enviar manda un marco del protocolo al bicho, si está conectado.
func (n *Nube) Enviar(id string, car byte, marco []byte) error {
	if !n.Conectado(id) {
		return ErrDesconectado
	}
	return n.enviar(id, car, marco)
}

// Conectado dice si el bicho está conectado al broker.
func (n *Nube) Conectado(id string) bool {
	n.mu.Lock()
	t := n.bichos[id]
	n.mu.Unlock()
	return t != nil && t.conectadoAhora.Load()
}

// CambioDeseado avisa que cambió la config o una imagen en la base.
func (n *Nube) CambioDeseado(id string) {
	select {
	case n.trabajador(id).entrada <- entrada{tipo: entCambio}:
	default:
	}
}

// MandarOta le pide al bicho que baje e instale un firmware.
func (n *Nube) MandarOta(ctx context.Context, id string, f almacen.Firmware) error {
	if !n.Conectado(id) {
		return ErrDesconectado
	}
	b, err := n.alm.Bicho(ctx, id)
	if err != nil {
		return err
	}
	base := n.op.URLPublica
	if b.Virtual {
		base = n.op.URLInterna
	}
	url := fmt.Sprintf("%s/v1/firmwares/%d/binario?t=%s", strings.TrimRight(base, "/"), f.ID, n.op.FirmarDescarga(f.ID, id))
	var m protocolo.Manifiesto
	m.Tamano = uint32(f.Tamano)
	copy(m.Sha256[:], f.Sha256)
	copy(m.Firma[:], f.Firma)
	m.Version, m.Perfil = f.Version, f.Perfil
	marco, err := protocolo.DescargarOta(m, url)
	if err != nil {
		return err
	}
	return n.enviar(id, protocolo.CarOta, marco)
}

// Reclamado avisa que un bicho se conectó con una clave pendiente: cambió de cuenta.
func (n *Nube) Reclamado(id string, acc almacen.Acceso) {
	n.duenos.Store(id, acc.UsuarioID)
	if acc.UsuarioAntes != "" && acc.UsuarioAntes != acc.UsuarioID {
		n.hub.Publicar(acc.UsuarioAntes, eventos.Evento{Tipo: "bicho", Bicho: id, Datos: map[string]string{"accion": "quitado"}})
	}
	n.hub.Publicar(acc.UsuarioID, eventos.Evento{Tipo: "bicho", Bicho: id, Datos: map[string]string{"accion": "sumado"}})
	n.esperasMu.Lock()
	for _, c := range n.esperas[id] {
		close(c)
	}
	delete(n.esperas, id)
	n.esperasMu.Unlock()
	// Se está conectando recién (todavía no se suscribió): se sincroniza
	// cuando mande su estado.
	select {
	case n.trabajador(id).entrada <- entrada{tipo: entReclamado}:
	default:
	}
}

// Quitado avisa que el bicho salió de la cuenta: se lo echa del broker.
func (n *Nube) Quitado(id, usuario string) {
	n.duenos.Store(id, "")
	n.hub.Publicar(usuario, eventos.Evento{Tipo: "bicho", Bicho: id, Datos: map[string]string{"accion": "quitado"}})
	n.Expulsar(id)
}

// EsperarReclamo espera a que el bicho se conecte con la clave nueva.
func (n *Nube) EsperarReclamo(ctx context.Context, id string) bool {
	c := make(chan struct{})
	n.esperasMu.Lock()
	n.esperas[id] = append(n.esperas[id], c)
	n.esperasMu.Unlock()
	select {
	case <-c:
		return true
	case <-ctx.Done():
		return false
	}
}

// Expulsar desconecta al bicho del broker (por la API REST de EMQX).
func (n *Nube) Expulsar(id string) {
	if n.op.EmqxURL == "" {
		return
	}
	req, _ := http.NewRequestWithContext(n.ctx, http.MethodDelete, strings.TrimRight(n.op.EmqxURL, "/")+"/api/v5/clients/"+id, nil)
	if usuario, clave, ok := strings.Cut(n.op.EmqxClave, ":"); ok {
		req.SetBasicAuth(usuario, clave)
	}
	cli := &http.Client{Timeout: 5 * time.Second}
	r, err := cli.Do(req)
	if err != nil {
		n.log.Warn("no pude echar al bicho del broker", "bicho", id, "error", err)
		return
	}
	r.Body.Close()
	if r.StatusCode >= 300 && r.StatusCode != http.StatusNotFound {
		n.log.Warn("EMQX no echó al bicho", "bicho", id, "estado", r.StatusCode)
	}
}

// EstadoDe devuelve el último estado conocido.
func (n *Nube) EstadoDe(id string) (protocolo.Estado, bool) {
	n.mu.Lock()
	t := n.bichos[id]
	n.mu.Unlock()
	if t == nil {
		return protocolo.Estado{}, false
	}
	e := t.ultimoEstado.Load()
	if e == nil {
		return protocolo.Estado{}, false
	}
	return *e, true
}

// EstadoSincro dice cómo está la sincronización.
func (n *Nube) EstadoSincro(id string) Progreso {
	n.mu.Lock()
	t := n.bichos[id]
	n.mu.Unlock()
	if t == nil {
		return Progreso{Config: "sin-datos", Imagenes: "sin-datos"}
	}
	p := t.progreso.Load()
	if p == nil {
		return Progreso{Config: "sin-datos", Imagenes: "sin-datos"}
	}
	return *p
}
