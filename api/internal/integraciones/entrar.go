package integraciones

// Entrar con Telegram (docs/nube.md, "Cuentas"): la web (o la app) pide un
// link https://t.me/<bot>?start=entrar-<token>; quien toca Start queda
// identificado por su chat. Si el chat ya es de una cuenta, la aprueba; si no,
// pregunta si crear una. El bot manda además un código de 6 dígitos. Solo el
// navegador que pidió el link (el que tiene el secreto) recibe la sesión.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
)

// DuracionEntrada: cuánto vale el link para entrar con Telegram.
const DuracionEntrada = 10 * time.Minute

// MaxEntradasPorChat: cuántas veces por hora puede entrar un mismo Telegram.
const MaxEntradasPorChat = 10

const prefijoEntrar = "entrar-"

// ErrTelegramApagado: este servidor no tiene TELEGRAM_TOKEN.
var ErrTelegramApagado = errors.New("acá no se puede entrar con Telegram")

func hashEntrada(parte, s string) []byte {
	h := sha256.Sum256([]byte("entrar:" + parte + ":" + s))
	return h[:]
}

// Entrada es lo que se le da al que pidió entrar con Telegram.
type Entrada struct {
	Token   string    // va en el link (y en GET /v1/sesion/telegram/{token})
	Secreto string    // se queda en el navegador (cookie) o en la app
	Enlace  string    // https://t.me/<bot>?start=entrar-<token>
	Vence   time.Time //
}

// CrearEntrada arma un pedido nuevo (un solo uso, 10 minutos).
func (i *Integraciones) CrearEntrada(ctx context.Context, cliente, ip string) (Entrada, error) {
	if i.tg == nil {
		return Entrada{}, ErrTelegramApagado
	}
	e := Entrada{Token: aleatorio(24), Secreto: aleatorio(32), Vence: time.Now().Add(DuracionEntrada).UTC().Truncate(time.Second)}
	if err := i.alm.CrearEntradaTelegram(ctx, hashEntrada("token", e.Token), hashEntrada("secreto", e.Secreto), cliente, ip, e.Vence); err != nil {
		return Entrada{}, err
	}
	// "start" acepta hasta 64 caracteres [A-Za-z0-9_-]: "entrar-" + 32.
	e.Enlace = "https://t.me/" + i.op.TelegramBot + "?start=" + prefijoEntrar + e.Token
	return e, nil
}

// CanjearEntrada: el que pidió el link pregunta si ya está (codigo vacío) o
// tipea el código del bot. Con el estado "usada", la cuenta es e.UsuarioID.
func (i *Integraciones) CanjearEntrada(ctx context.Context, token, secreto, codigo string) (almacen.EntradaTelegram, error) {
	var hc []byte
	if codigo != "" {
		hc = hashEntrada("codigo", soloDigitos(codigo))
	}
	return i.alm.CanjearEntradaTelegram(ctx, hashEntrada("token", token), hashEntrada("secreto", secreto), hc)
}

func soloDigitos(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func seisDigitos() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// entrarDesdeTelegram atiende "/start entrar-<token>".
func (i *Integraciones) entrarDesdeTelegram(ctx context.Context, m *mensaje, token string) {
	chat := m.Chat.ID
	n, err := i.alm.EntradasTelegramDeChat(ctx, chat, time.Now().Add(-time.Hour))
	if err != nil {
		i.log.Warn("telegram: no pude contar las entradas", "error", err)
		i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
		return
	}
	if n >= MaxEntradasPorChat {
		i.responder(ctx, chat, "Entraste muchas veces en la última hora: probá en un rato.", nil)
		return
	}
	e, err := i.alm.TomarEntradaTelegram(ctx, hashEntrada("token", token), chat)
	if errors.Is(err, almacen.ErrEntradaInvalida) {
		i.responder(ctx, chat, "Ese link para entrar ya se usó o venció (valen 10 minutos). Pedí uno nuevo en la web.", nil)
		return
	}
	if err != nil {
		i.log.Warn("telegram: no pude tomar la entrada", "error", err)
		i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
		return
	}
	v, err := i.alm.TelegramPorChat(ctx, chat)
	if err == nil {
		i.aprobarEntrada(ctx, chat, e, v.UsuarioID, false)
		return
	}
	if !errors.Is(err, almacen.ErrNoEncontrado) {
		i.log.Warn("telegram: no pude leer el vínculo", "error", err)
		return
	}
	id := strconv.FormatInt(e.ID, 10)
	i.responder(ctx, chat, "Todavía no tenés cuenta. ¿Crear tu cuenta de Sintética?",
		[][]Boton{{{Texto: "Crear mi cuenta", Datos: "entrar-si:" + id}, {Texto: "No", Datos: "entrar-no:" + id}}})
}

// aprobarEntrada deja el pedido listo para la cuenta y le manda el código.
func (i *Integraciones) aprobarEntrada(ctx context.Context, chat int64, e almacen.EntradaTelegram, usuario string, creada bool) {
	codigo := seisDigitos()
	if err := i.alm.AprobarEntradaTelegram(ctx, e.ID, chat, usuario, hashEntrada("codigo", codigo)); err != nil {
		if errors.Is(err, almacen.ErrEntradaInvalida) {
			i.responder(ctx, chat, "Ese pedido para entrar venció (valen 10 minutos). Pedí uno nuevo en la web.", nil)
			return
		}
		i.log.Warn("telegram: no pude aprobar la entrada", "error", err)
		i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
		return
	}
	donde := "la web"
	if e.Cliente == "app" {
		donde = "la app"
	}
	texto := ""
	if creada {
		texto = "¡Bienvenido! Creé tu cuenta de Sintética (el mail lo podés sumar después).\n\n"
	}
	texto += fmt.Sprintf("Listo: volvé a %s, ya estás adentro.\n\nSi no se abrió sola, escribí este código: %s %s (vale 10 minutos).",
		donde, codigo[:3], codigo[3:])
	i.responder(ctx, chat, texto, [][]Boton{{{Texto: "No fui yo", Datos: "entrar-no:" + strconv.FormatInt(e.ID, 10)}}})
}

// botonEntrar atiende "entrar-si:<id>" y "entrar-no:<id>". Devuelve false si
// el botón no es de entrar.
func (i *Integraciones) botonEntrar(ctx context.Context, c *callback) bool {
	accion, idTexto, ok := strings.Cut(c.Data, ":")
	if !ok || (accion != "entrar-si" && accion != "entrar-no") {
		return false
	}
	chat := c.Message.Chat.ID
	id, err := strconv.ParseInt(idTexto, 10, 64)
	if err != nil {
		return true
	}
	// Ya contestó: se sacan los botones.
	i.tg.llamar(ctx, "editMessageReplyMarkup", map[string]any{"chat_id": chat, "message_id": c.Message.MessageID, //nolint:errcheck
		"reply_markup": map[string]any{"inline_keyboard": [][]Boton{}}}, nil)
	if accion == "entrar-no" {
		cerro, err := i.alm.RechazarEntradaTelegram(ctx, id, chat)
		switch {
		case errors.Is(err, almacen.ErrEntradaInvalida):
			i.responder(ctx, chat, "Ese pedido ya no está.", nil)
		case err != nil:
			i.log.Warn("telegram: no pude rechazar la entrada", "error", err)
			i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
		case cerro:
			i.responder(ctx, chat, "Listo: cerré esa sesión. Nadie más entra con ese link.", nil)
		default:
			i.responder(ctx, chat, "Listo: con ese link no entra nadie.", nil)
		}
		return true
	}
	e, err := i.alm.EntradaTelegramDe(ctx, id, chat)
	if err != nil || e.Estado != "confirmar" || time.Now().After(e.Vence) {
		i.responder(ctx, chat, "Ese pedido para entrar ya se usó o venció. Pedí uno nuevo en la web.", nil)
		return true
	}
	nombre := strings.TrimSpace(c.From.FirstName)
	if r := []rune(nombre); len(r) > 60 {
		nombre = string(r[:60])
	}
	usuario, creada, err := i.alm.CuentaDeTelegram(ctx, chat, nombreTG(&c.From), nombre)
	if err != nil {
		i.log.Warn("telegram: no pude crear la cuenta", "error", err)
		i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
		return true
	}
	i.aprobarEntrada(ctx, chat, e, usuario, creada)
	return true
}

// AtenderTelegram atiende un update crudo de la Bot API como si hubiera
// llegado por getUpdates (lo usan las pruebas de la API; sirve también para
// un webhook entrante).
func (i *Integraciones) AtenderTelegram(ctx context.Context, crudo []byte) error {
	if i.tg == nil {
		return ErrTelegramApagado
	}
	var u update
	if err := json.Unmarshal(crudo, &u); err != nil {
		return err
	}
	i.atender(ctx, u)
	return nil
}
