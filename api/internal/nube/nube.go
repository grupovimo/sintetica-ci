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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// ErrDesconectado: el bicho no está conectado a la nube.
var ErrDesconectado = errors.New("el bicho no está conectado a la nube")

// Broker es el broker MQTT, embebido en la API (internal/broker): la nube
// publica y se suscribe en el mismo proceso, sin red.
type Broker interface {
	Publicar(topico string, datos []byte, retener bool, qos byte) error
	// Suscribir: f se llama en la goroutine del que publicó (no puede bloquear).
	Suscribir(filtro string, f func(topico string, datos []byte)) error
	// Expulsar desconecta a un cliente.
	Expulsar(clientID string) bool
}

// Opciones de la nube.
type Opciones struct {
	Broker Broker
	// La URL de la API que ve cada bicho, para bajar las OTA.
	URLPublica, URLInterna string
	// Firma los links de descarga de las OTA.
	FirmarDescarga func(firmwareID int64, bicho string) string
}

// Nube es la conexión al broker y un trabajador por bicho.
type Nube struct {
	op    Opciones
	alm   *almacen.Almacen
	hub   *eventos.Hub
	log   *slog.Logger
	listo atomic.Bool // suscripta al broker
	// Los que figuraban conectados cuando arrancó la API: el broker arranca
	// con ella, así que se reconectan todos; esa reconexión no es "conectado".
	arranque     time.Time
	yaConectados sync.Map // id → true
	mu           sync.Mutex
	bichos       map[string]*trabajador
	duenos       sync.Map // id → usuario ("" = sin dueño)

	esperasMu sync.Mutex
	esperas   map[string][]chan struct{}

	ctx      context.Context
	cancelar context.CancelFunc

	oyente Oyente
	voz    Voz
}

// Voz puede quedarse con un 0x40 Decir para mandarlo junto con su audio
// (ElevenLabs, en integraciones/voz.go). TomarDecir no puede bloquear: si
// devuelve true, se encarga de llamar a mandar (después del audio).
type Voz interface {
	TomarDecir(bicho string, marco []byte, mandar func() error) bool
}

// UsarVoz suma la voz (antes de Conectar).
func (n *Nube) UsarVoz(v Voz) { n.voz = v }

// Oyente se entera de lo que les pasa a los bichos (las automatizaciones).
// Evento no puede bloquear: lo llama el trabajador del bicho.
type Oyente interface {
	// Evento: "conectado", "desconectado", "bateria-baja", "expresion",
	// "duerme", "despierta", los del registro (ver nube.EventosRegistro) y
	// "charla" (el 0xCC de la charla sin IA, en datos["marco"]).
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

// Conectar suscribe la nube a lo de los bichos (antes de que el broker
// acepte clientes: así no se pierde ningún mensaje).
func (n *Nube) Conectar() error {
	if n.op.Broker == nil {
		return errors.New("la nube no tiene broker")
	}
	ids, err := n.alm.BichosConectados(n.ctx)
	if err != nil {
		n.log.Warn("no sé quiénes estaban conectados", "error", err)
	}
	for _, id := range ids {
		n.yaConectados.Store(id, true)
	}
	n.arranque = time.Now()
	for _, t := range []string{"bicho/+/state", "bicho/+/resp", "bicho/+/conexion"} {
		if err := n.op.Broker.Suscribir(t, n.alLlegar); err != nil {
			return fmt.Errorf("suscribirse a %s: %w", t, err)
		}
	}
	n.listo.Store(true)
	go n.limpieza()
	go n.olvidarAusentes()
	return nil
}

// graciaReinicio: lo que tardan en volver los bichos después de un reinicio
// de la API (reintentan cada 5 s).
const graciaReinicio = time.Minute

// olvidarAusentes: los que estaban conectados y no volvieron, no lo están
// (lo que antes decía el "0" retenido que guardaba EMQX).
func (n *Nube) olvidarAusentes() {
	select {
	case <-n.ctx.Done():
		return
	case <-time.After(graciaReinicio):
	}
	n.yaConectados.Range(func(k, _ any) bool {
		if _, ok := n.yaConectados.LoadAndDelete(k); ok && !n.Conectado(k.(string)) {
			if err := n.alm.GuardarConexion(n.ctx, k.(string), false); err != nil {
				n.log.Warn("no pude guardar la conexión", "bicho", k, "error", err)
			}
		}
		return true
	})
}

// Cerrar para a los que trabajan atrás (el broker lo cierra quien lo armó).
func (n *Nube) Cerrar() { n.cancelar() }

// BrokerConectado dice si la nube está suscripta al broker.
func (n *Nube) BrokerConectado() bool { return n.listo.Load() }

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

func (n *Nube) alLlegar(topico string, datos []byte) {
	partes := strings.Split(topico, "/")
	if len(partes) != 3 || partes[0] != "bicho" || !protocolo.IDValido(partes[1]) {
		return
	}
	t := n.trabajador(partes[1])
	d := append([]byte(nil), datos...)
	var e entrada
	switch partes[2] {
	case "state":
		e = entrada{tipo: entEstado, datos: d}
	case "conexion":
		// El broker arranca con la API y la API se suscribe antes de que entre
		// nadie: no hay retenidos de antes. Cuenta como retenido (no se avisa
		// a las automatizaciones) la vuelta de uno que ya estaba conectado
		// antes de reiniciar la API.
		e = entrada{tipo: entConexion, conectado: string(d) == "1"}
		if e.conectado && time.Since(n.arranque) < graciaReinicio {
			_, e.retenido = n.yaConectados.LoadAndDelete(partes[1])
		}
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
	if !n.listo.Load() {
		return errors.New("la API no está conectada al broker")
	}
	return n.op.Broker.Publicar("bicho/"+id+"/cmd", append([]byte{car}, marco...), false, 1)
}

// --- Para el servidor HTTP ---

// Enviar manda un marco del protocolo al bicho, si está conectado.
func (n *Nube) Enviar(id string, car byte, marco []byte) error {
	if !n.Conectado(id) {
		return ErrDesconectado
	}
	if n.voz != nil && car == protocolo.CarComando && len(marco) > 0 && marco[0] == protocolo.CmdDecir &&
		n.voz.TomarDecir(id, append([]byte(nil), marco...), func() error { return n.enviar(id, car, marco) }) {
		return nil
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

// Expulsar desconecta al bicho del broker (el broker avisa su "0").
func (n *Nube) Expulsar(id string) {
	if n.op.Broker != nil && n.op.Broker.Expulsar(id) {
		n.log.Info("bicho echado del broker", "bicho", id)
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
