package broker

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// ganchos son la autenticación, la autorización y el estado de conexión de
// los bichos. Hacen lo mismo que EMQX con la autenticación y la autorización
// por HTTP contra la API (no_match = deny, deny_action = ignore), pero
// llamando a la API en el mismo proceso. La autorización no tiene caché (la de
// EMQX era de 1 min): es comparar textos, no toca la base.
type ganchos struct {
	mqtt.HookBase
	op  Opciones
	srv *mqtt.Server
	log *slog.Logger

	// cerrando: la API se apaga; los bichos se reconectan enseguida, no se
	// avisa que se fueron.
	cerrando atomic.Bool

	mu sync.Mutex
	// ultimo es la última sesión de cada bicho: al tomarla otra con el mismo
	// id (el bicho que se reconecta antes de que se note el corte), la vieja
	// no tiene que avisar que se desconectó.
	ultimo map[string]*mqtt.Client
}

func (g *ganchos) ID() string { return "sintetica" }

func (g *ganchos) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnSessionEstablish,
		mqtt.OnDisconnect,
		mqtt.OnPublish,
	}, []byte{b})
}

// TopicoConexion es donde cada bicho dice si está conectado ("1" / "0",
// retenido; el "0" es su última voluntad).
func TopicoConexion(id string) string { return "bicho/" + id + "/conexion" }

func (g *ganchos) super(cl *mqtt.Client) bool {
	return g.op.Superusuario != "" && string(cl.Properties.Username) == g.op.Superusuario
}

func (g *ganchos) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if cl.ID == mqtt.InlineClientId { // la API adentro: nadie puede tomar su lugar
		return false
	}
	usuario := string(pk.Connect.Username)
	ctx, cancelar := context.WithTimeout(context.Background(), tiempoAutenticar)
	defer cancelar()
	if !g.op.Autenticar(ctx, usuario, string(pk.Connect.Password), cl.ID) {
		g.log.Info("cliente rechazado", "cliente", cl.ID, "usuario", usuario, "remoto", cl.Net.Remote)
		return false
	}
	// La última voluntad se publica sin preguntar: se revisa ahora.
	if !g.super(cl) && atomic.LoadUint32(&cl.Properties.Will.Flag) == 1 &&
		!g.op.Autorizar(usuario, cl.Properties.Will.TopicName, true) {
		g.log.Warn("última voluntad no autorizada: la descarto", "cliente", cl.ID, "topico", cl.Properties.Will.TopicName)
		atomic.StoreUint32(&cl.Properties.Will.Flag, 0)
	}
	return true
}

// OnSessionEstablish corre después de autenticar y antes de que Mochi eche a
// la sesión vieja con el mismo id (el bicho que se reconecta antes de que se
// note el corte). La echamos nosotros y esperamos a que termine de limpiarse:
// si no, Mochi la limpia por id en paralelo y se puede llevar puesta la
// entrada y las suscripciones de la nueva.
func (g *ganchos) OnSessionEstablish(cl *mqtt.Client, _ packets.Packet) {
	if cl.Net.Inline {
		return
	}
	g.mu.Lock()
	vieja := g.ultimo[cl.ID]
	g.ultimo[cl.ID] = cl
	g.mu.Unlock()
	if vieja == nil || vieja == cl {
		return
	}
	atomic.StoreUint32(&vieja.Properties.Will.Flag, 0) // su "0" llegaría después del "1" de la nueva
	if !seBorra(vieja) {
		return // sesión persistente: Mochi la hereda sin borrarla
	}
	if actual, ok := g.srv.Clients.Get(cl.ID); !ok || actual != vieja {
		return
	}
	g.srv.DisconnectClient(vieja, packets.ErrSessionTakenOver) //nolint:errcheck
	for limite := time.Now().Add(2 * time.Second); time.Now().Before(limite); time.Sleep(5 * time.Millisecond) {
		if actual, ok := g.srv.Clients.Get(cl.ID); !ok || actual != vieja {
			return
		}
	}
	g.log.Warn("la sesión vieja no terminó de irse", "cliente", cl.ID)
}

// seBorra: la sesión se borra al desconectarse (la misma cuenta que Mochi).
func seBorra(cl *mqtt.Client) bool {
	return (cl.Properties.ProtocolVersion == 5 && cl.Properties.Props.SessionExpiryInterval == 0) ||
		(cl.Properties.ProtocolVersion < 5 && cl.Properties.Clean)
}

// OnDisconnect: si se fue un bicho (y no lo reemplazó otra sesión suya), el
// broker publica su "0" retenido, aunque se haya ido con DISCONNECT (sin
// última voluntad) o lo hayamos echado. Es la verdad al instante.
func (g *ganchos) OnDisconnect(cl *mqtt.Client, _ error, _ bool) {
	if cl.Net.Inline {
		return
	}
	g.mu.Lock()
	actual := g.ultimo[cl.ID] == cl
	if actual {
		delete(g.ultimo, cl.ID)
	}
	g.mu.Unlock()
	if !actual || g.super(cl) || g.cerrando.Load() {
		return
	}
	if err := g.srv.Publish(TopicoConexion(cl.ID), []byte("0"), true, 1); err != nil {
		g.log.Warn("no pude avisar la desconexión", "bicho", cl.ID, "error", err)
	}
}

func (g *ganchos) OnACLCheck(cl *mqtt.Client, topico string, publicar bool) bool {
	if cl.Net.Inline || g.super(cl) {
		return true
	}
	if publicar {
		// Lo decide OnPublish: lo no autorizado se ignora (con su PUBACK) en
		// vez de echar al cliente, como el deny_action = ignore de EMQX.
		return true
	}
	return g.op.Autorizar(string(cl.Properties.Username), topico, false)
}

func (g *ganchos) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if cl.Net.Inline || g.super(cl) || g.op.Autorizar(string(cl.Properties.Username), pk.TopicName, true) {
		return pk, nil
	}
	g.log.Warn("publicación no autorizada: la ignoro", "cliente", cl.ID, "topico", pk.TopicName)
	return pk, packets.CodeSuccessIgnore
}

// soloAvisos deja pasar los avisos y errores de Mochi, salvo el que escribe
// por cada conexión fallida (sin mensaje): los rechazos ya los anota OnConnectAuthenticate.
type soloAvisos struct{ h slog.Handler }

func (s soloAvisos) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelWarn && s.h.Enabled(ctx, l)
}

func (s soloAvisos) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "" && r.Level < slog.LevelError {
		return nil
	}
	return s.h.Handle(ctx, r)
}

func (s soloAvisos) WithAttrs(a []slog.Attr) slog.Handler { return soloAvisos{s.h.WithAttrs(a)} }
func (s soloAvisos) WithGroup(n string) slog.Handler      { return soloAvisos{s.h.WithGroup(n)} }
