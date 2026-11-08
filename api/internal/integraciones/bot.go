package integraciones

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

var comandosBot = []map[string]string{
	{"command": "robots", "description": "Tus robots y cómo están"},
	{"command": "estado", "description": "El estado de un robot"},
	{"command": "cara", "description": "Cambiarle la cara a un robot"},
	{"command": "decir", "description": "Que un robot diga algo en su pantalla"},
	{"command": "clima", "description": "El clima donde está un robot"},
	{"command": "jugar", "description": "Jugar con un robot (dado, moneda, ppt, reflejos, mirada)"},
	{"command": "ayuda", "description": "Qué puedo hacer"},
}

const textoAyuda = `Esto es lo que puedo hacer:

/robots: tus robots y cómo están
/estado [nombre]: el estado de un robot
/cara <expresión> [nombre]: le cambia la cara (por ejemplo: /cara contento)
/decir <texto> [nombre]: tu robot lo dice en su pantalla (hasta 60 letras)
/clima [nombre]: el clima donde está tu robot
/jugar [dado|moneda|ppt|reflejos|mirada] [nombre]: juega con vos en su pantalla
/ayuda: esto

Si tenés un solo robot, el nombre no hace falta. Los avisos (batería baja, recordatorios…) se arman en la web, en Automatizaciones.`

// Las caras del teclado de /cara (las de fábrica; "auto" vuelve a la automática).
var carasDelTeclado = []string{"contento", "neutral", "sorpresa", "triste", "enojado", "enamorado", "guino",
	"curioso", "pensando", "sospecha", "mareado", "dormido", "auto"}

func (i *Integraciones) comoVincularse() string {
	return "Hola, soy el bot de Sintética. Para hablarle a tu robot, primero vinculá este chat con tu cuenta: " +
		"entrá a la web de Sintética, en Integraciones tocá \"Vincular Telegram\" y abrí el link que te da (vale 15 minutos)."
}

// pendiente es un /decir o un /cara que espera que se elija el robot.
type pendiente struct {
	accion, texto string
	en            time.Time
}

type robot struct {
	almacen.Bicho
	nombre    string
	conectado bool
	estado    *protocolo.Estado
}

func (i *Integraciones) robotsDe(ctx context.Context, usuario string) ([]robot, error) {
	l, err := i.alm.BichosDe(ctx, usuario)
	if err != nil {
		return nil, err
	}
	var rs []robot
	for _, b := range l {
		r := robot{Bicho: b, nombre: b.ID, conectado: i.robots.Conectado(b.ID)}
		if e, ok := i.robots.EstadoDe(b.ID); ok {
			r.estado = &e
		} else if len(b.Estado) > 0 {
			if e, err := protocolo.LeerEstado(b.Estado); err == nil {
				r.estado = &e
			}
		}
		if r.estado != nil && r.estado.Nombre != "" {
			r.nombre = r.estado.Nombre
		}
		if b.Apodo != "" {
			r.nombre = b.Apodo
		}
		rs = append(rs, r)
	}
	return rs, nil
}

// normal: minúsculas y sin tildes, para comparar nombres.
func normal(s string) string {
	return strings.Join(strings.Fields(sinTildes.Replace(strings.ToLower(s))), " ")
}

var sinTildes = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")

// elegir busca el robot al final del texto ("contento Pipo" → Pipo y
// "contento"). Con un solo robot, es ese y el texto queda entero.
func elegir(rs []robot, texto string) (*robot, string) {
	texto = strings.TrimSpace(texto)
	t := normal(texto)
	palabras := strings.Fields(texto)
	var mejor *robot
	largo := 0
	for k := range rs {
		for _, n := range []string{normal(rs[k].nombre), strings.ToLower(rs[k].ID)} {
			if n == "" {
				continue
			}
			if (t == n || strings.HasSuffix(t, " "+n)) && len(n) > largo {
				mejor, largo = &rs[k], len(n)
			}
		}
	}
	if mejor != nil {
		quitar := len(strings.Fields(normal(mejor.nombre)))
		if t == strings.ToLower(mejor.ID) || strings.HasSuffix(t, " "+strings.ToLower(mejor.ID)) {
			quitar = 1
		}
		if quitar > len(palabras) {
			quitar = len(palabras)
		}
		return mejor, strings.Join(palabras[:len(palabras)-quitar], " ")
	}
	if len(rs) == 1 {
		return &rs[0], texto
	}
	return nil, texto
}

func animo(humor float64) string {
	switch {
	case humor > 0.5:
		return "muy contento"
	case humor > 0.15:
		return "de buen ánimo"
	case humor < -0.5:
		return "muy bajoneado"
	case humor < -0.15:
		return "medio bajoneado"
	}
	return "tranquilo"
}

func (r robot) resumen() string {
	if !r.conectado {
		s := r.nombre + ": desconectado"
		if r.UltimaConexion != nil {
			s += " (la última vez, " + r.UltimaConexion.In(nube.Zona()).Format("02/01 15:04") + ")"
		}
		return s
	}
	partes := []string{"conectado"}
	if e := r.estado; e != nil {
		if e.Bateria != nil {
			partes = append(partes, fmt.Sprintf("%d %% de batería", *e.Bateria))
		}
		if e.Dormido {
			partes = append(partes, "durmiendo")
		} else if e.Expresion != "" {
			partes = append(partes, "cara "+e.Expresion)
		}
		partes = append(partes, animo(e.Humor))
	}
	return r.nombre + ": " + strings.Join(partes, ", ")
}

func (r robot) detalle() string {
	var b strings.Builder
	b.WriteString(r.nombre + "\n")
	if r.conectado {
		b.WriteString("Conectado\n")
	} else {
		b.WriteString("Desconectado")
		if r.UltimaConexion != nil {
			b.WriteString(" (la última vez, " + r.UltimaConexion.In(nube.Zona()).Format("02/01 15:04") + ")")
		}
		b.WriteString("\n")
	}
	if e := r.estado; e != nil {
		if e.Bateria != nil {
			fmt.Fprintf(&b, "Batería: %d %%\n", *e.Bateria)
		} else {
			b.WriteString("Batería: enchufado o sin dato\n")
		}
		if e.Dormido {
			b.WriteString("Está durmiendo\n")
		}
		if e.Expresion != "" {
			b.WriteString("Cara: " + e.Expresion)
			if e.Fija {
				b.WriteString(" (fija)")
			}
			b.WriteString("\n")
		}
		b.WriteString("Ánimo: " + animo(e.Humor) + "\n")
		if e.Modo != "" && e.Modo != "mascota" {
			b.WriteString("Modo: " + e.Modo + "\n")
		}
		if e.Version != "" {
			b.WriteString("Firmware: " + e.Version + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func tecladoRobots(rs []robot, prefijo string) [][]Boton {
	var t [][]Boton
	for _, r := range rs {
		if d := prefijo + r.ID; len(d) <= 64 {
			t = append(t, []Boton{{Texto: r.nombre, Datos: d}})
		}
	}
	return t
}

func tecladoCaras(id string) [][]Boton {
	var t [][]Boton
	fila := []Boton{}
	for _, c := range carasDelTeclado {
		fila = append(fila, Boton{Texto: c, Datos: "cara:" + id + ":" + c})
		if len(fila) == 3 {
			t = append(t, fila)
			fila = []Boton{}
		}
	}
	if len(fila) > 0 {
		t = append(t, fila)
	}
	return t
}

func nombreTG(u *usuarioTG) string {
	if u == nil {
		return ""
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return u.FirstName
}

func (i *Integraciones) responder(ctx context.Context, chat int64, texto string, teclado [][]Boton) {
	if err := i.tg.mandar(ctx, chat, texto, teclado); err != nil {
		i.log.Warn("telegram: no pude contestar", "error", err)
	}
}

func (i *Integraciones) atenderMensaje(ctx context.Context, m *mensaje) {
	chat := m.Chat.ID
	if m.Chat.Type != "private" {
		return // solo chats privados: los grupos no tienen un dueño
	}
	if !i.chats.permitir(fmt.Sprint(chat), 0, 30, time.Minute) {
		return
	}
	texto := strings.TrimSpace(m.Text)
	comando, resto, _ := strings.Cut(texto, " ")
	comando = strings.ToLower(comando)
	if c, bot, ok := strings.Cut(comando, "@"); ok {
		if !strings.EqualFold(bot, i.op.TelegramBot) {
			return
		}
		comando = c
	}
	resto = strings.TrimSpace(resto)

	if comando == "/start" && resto != "" {
		if _, err := i.Vincular(ctx, resto, chat, nombreTG(m.From)); err != nil {
			if errors.Is(err, almacen.ErrCodigoInvalido) {
				i.responder(ctx, chat, "Ese link ya se usó o venció (valen 15 minutos). Pedí uno nuevo en la web, en Integraciones.", nil)
				return
			}
			i.log.Warn("telegram: no pude vincular", "error", err)
			i.responder(ctx, chat, "Algo salió mal de nuestro lado. Probá de nuevo en un rato.", nil)
			return
		}
		i.responder(ctx, chat, "¡Listo! Este chat quedó vinculado con tu cuenta de Sintética. Desde acá podés hablarle a tus robots y ellos te van a avisar lo que armes en Automatizaciones.\n\n"+textoAyuda, nil)
		return
	}
	v, err := i.alm.TelegramPorChat(ctx, chat)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		i.responder(ctx, chat, i.comoVincularse(), nil)
		return
	}
	if err != nil {
		i.log.Warn("telegram: no pude leer el vínculo", "error", err)
		return
	}
	rs, err := i.robotsDe(ctx, v.UsuarioID)
	if err != nil {
		i.log.Warn("telegram: no pude leer los robots", "error", err)
		return
	}
	if len(rs) == 0 && comando != "/ayuda" && comando != "/help" && comando != "/start" {
		i.responder(ctx, chat, "Todavía no tenés robots en tu cuenta. Sumalos desde la app o la web.", nil)
		return
	}
	switch comando {
	case "/start", "/ayuda", "/help":
		i.responder(ctx, chat, textoAyuda, nil)
	case "/robots":
		var l []string
		for _, r := range rs {
			l = append(l, "• "+r.resumen())
		}
		i.responder(ctx, chat, "Tus robots:\n"+strings.Join(l, "\n"), tecladoRobots(rs, "estado:"))
	case "/estado":
		r, _ := elegir(rs, resto)
		if r == nil {
			i.responder(ctx, chat, "¿De cuál?", tecladoRobots(rs, "estado:"))
			return
		}
		i.responder(ctx, chat, r.detalle(), nil)
	case "/cara":
		r, cara := elegir(rs, resto)
		if cara == "" {
			if r == nil {
				i.responder(ctx, chat, "¿A cuál le cambio la cara?", tecladoRobots(rs, "caras:"))
				return
			}
			i.responder(ctx, chat, "¿Qué cara le pongo a "+r.nombre+"?", tecladoCaras(r.ID))
			return
		}
		if strings.Contains(cara, " ") || len(cara) > 40 {
			i.responder(ctx, chat, "La expresión es una sola palabra, por ejemplo: /cara contento", nil)
			return
		}
		if r == nil {
			i.guardarPendiente(chat, "cara", cara)
			i.responder(ctx, chat, "¿A cuál le pongo "+cara+"?", tecladoRobots(rs, "cara:"))
			return
		}
		i.responder(ctx, chat, i.ponerCara(v.UsuarioID, *r, cara), nil)
	case "/decir":
		r, texto := elegir(rs, resto)
		if texto == "" {
			i.responder(ctx, chat, "Decime qué: /decir <texto> [nombre]", nil)
			return
		}
		if r == nil {
			i.guardarPendiente(chat, "decir", texto)
			i.responder(ctx, chat, "¿A cuál se lo digo?", tecladoRobots(rs, "decir:"))
			return
		}
		i.responder(ctx, chat, i.decir(*r, texto), nil)
	case "/clima":
		r, _ := elegir(rs, resto)
		if r == nil {
			i.responder(ctx, chat, "¿De cuál?", tecladoRobots(rs, "clima:"))
			return
		}
		i.responder(ctx, chat, i.textoClima(ctx, *r), nil)
	case "/jugar":
		r, juego := elegir(rs, resto)
		if juego == "" {
			if r == nil {
				i.responder(ctx, chat, "¿Con cuál jugás?", tecladoRobots(rs, "juegos:"))
				return
			}
			i.responder(ctx, chat, "¿A qué jugamos con "+r.nombre+"?", tecladoJuegos(r.ID))
			return
		}
		if _, ok := protocolo.NombreJuego(juego); !ok {
			i.responder(ctx, chat, "No conozco ese juego. Los que hay: dado, moneda, ppt (piedra, papel o tijera), reflejos y mirada.", nil)
			return
		}
		if r == nil {
			i.guardarPendiente(chat, "jugar", juego)
			i.responder(ctx, chat, "¿Con cuál?", tecladoRobots(rs, "jugar:"))
			return
		}
		i.responder(ctx, chat, i.jugar(*r, juego), nil)
	default:
		i.responder(ctx, chat, "No entendí. "+textoAyuda, nil)
	}
}

func (i *Integraciones) guardarPendiente(chat int64, accion, texto string) {
	i.pendMu.Lock()
	defer i.pendMu.Unlock()
	for k, p := range i.pendientes {
		if time.Since(p.en) > 10*time.Minute {
			delete(i.pendientes, k)
		}
	}
	i.pendientes[chat] = pendiente{accion: accion, texto: texto, en: time.Now()}
}

func (i *Integraciones) tomarPendiente(chat int64, accion string) (string, bool) {
	i.pendMu.Lock()
	defer i.pendMu.Unlock()
	p, ok := i.pendientes[chat]
	if !ok || p.accion != accion || time.Since(p.en) > 10*time.Minute {
		return "", false
	}
	delete(i.pendientes, chat)
	return p.texto, true
}

// ponerCara usa el mismo camino que POST /v1/bichos/{id}/comando {expresion}.
func (i *Integraciones) ponerCara(usuario string, r robot, cara string) string {
	expr := cara
	if cara == "auto" || cara == "automatica" || cara == "automática" {
		expr = ""
	}
	if err := i.robots.Enviar(r.ID, protocolo.CarComando, protocolo.Expresion(expr)); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			return r.nombre + " no está conectado ahora."
		}
		i.log.Warn("telegram: no pude mandar la cara", "bicho", r.ID, "error", err)
		return "No le pude mandar la cara a " + r.nombre + "."
	}
	if expr == "" {
		return "Listo: " + r.nombre + " vuelve a poner las caras solo."
	}
	return "Listo: " + r.nombre + " tiene cara de " + cara + "."
}

// decir: el robot muestra el texto en un globo (marco 0x40).
func (i *Integraciones) decir(r robot, texto string) string {
	m, err := protocolo.Decir(texto, "")
	if err != nil {
		return "El texto es de hasta 60 bytes (una tilde ocupa 2): acortalo un poco."
	}
	if err := i.robots.Enviar(r.ID, protocolo.CarComando, m); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			return r.nombre + " no está conectado ahora."
		}
		i.log.Warn("telegram: no pude mandar el texto", "bicho", r.ID, "error", err)
		return "No se lo pude decir a " + r.nombre + "."
	}
	return "Listo: " + r.nombre + " lo está diciendo."
}

// textoClima: lo que contesta /clima.
func (i *Integraciones) textoClima(ctx context.Context, r robot) string {
	u, c := i.ClimaDe(ctx, r.ID)
	if u == nil {
		return r.nombre + " no sabe dónde está: poné su ubicación en la web (en la config, Clima)."
	}
	if c == nil {
		return "No pude traer el clima de " + u.Ciudad + " ahora: probá en un rato."
	}
	return fmt.Sprintf("En %s: %s (máx. %d°, mín. %d°).", u.Ciudad, c.Texto(), c.Max, c.Min)
}

func (i *Integraciones) atenderBoton(ctx context.Context, c *callback) {
	i.tg.llamar(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": c.ID}, nil) //nolint:errcheck
	if c.Message == nil || c.Message.Chat.Type != "private" {
		return
	}
	chat := c.Message.Chat.ID
	if !i.chats.permitir(fmt.Sprint(chat), 0, 30, time.Minute) {
		return
	}
	v, err := i.alm.TelegramPorChat(ctx, chat)
	if err != nil {
		i.responder(ctx, chat, i.comoVincularse(), nil)
		return
	}
	rs, err := i.robotsDe(ctx, v.UsuarioID)
	if err != nil {
		return
	}
	partes := strings.SplitN(c.Data, ":", 3)
	var r *robot
	if len(partes) >= 2 {
		for k := range rs {
			if rs[k].ID == partes[1] {
				r = &rs[k]
			}
		}
	}
	if r == nil {
		i.responder(ctx, chat, "Ese robot ya no está en tu cuenta.", nil)
		return
	}
	switch partes[0] {
	case "estado":
		i.responder(ctx, chat, r.detalle(), nil)
	case "clima":
		i.responder(ctx, chat, i.textoClima(ctx, *r), nil)
	case "caras":
		i.responder(ctx, chat, "¿Qué cara le pongo a "+r.nombre+"?", tecladoCaras(r.ID))
	case "cara":
		cara := ""
		if len(partes) == 3 {
			cara = partes[2]
		} else if p, ok := i.tomarPendiente(chat, "cara"); ok {
			cara = p
		}
		if cara == "" {
			i.responder(ctx, chat, "¿Qué cara le pongo a "+r.nombre+"?", tecladoCaras(r.ID))
			return
		}
		i.responder(ctx, chat, i.ponerCara(v.UsuarioID, *r, cara), nil)
	case "decir":
		texto, ok := i.tomarPendiente(chat, "decir")
		if !ok {
			i.responder(ctx, chat, "Se me olvidó qué querías decir: mandalo de nuevo con /decir.", nil)
			return
		}
		i.responder(ctx, chat, i.decir(*r, texto), nil)
	case "juegos":
		i.responder(ctx, chat, "¿A qué jugamos con "+r.nombre+"?", tecladoJuegos(r.ID))
	case "jugar":
		juego := ""
		if len(partes) == 3 {
			juego = partes[2]
		} else if p, ok := i.tomarPendiente(chat, "jugar"); ok {
			juego = p
		}
		if juego == "" {
			i.responder(ctx, chat, "¿A qué jugamos con "+r.nombre+"?", tecladoJuegos(r.ID))
			return
		}
		i.responder(ctx, chat, i.jugar(*r, juego), nil)
	case "animo":
		v, ok := -1, len(partes) == 3
		if ok {
			v, ok = map[string]int{"0": 0, "1": 1, "2": 2}[partes[2]]
		}
		if !ok {
			return
		}
		// Se sacan los botones (ya contestó) y se contesta con empatía.
		i.tg.llamar(ctx, "editMessageReplyMarkup", map[string]any{"chat_id": chat, "message_id": c.Message.MessageID, //nolint:errcheck
			"reply_markup": map[string]any{"inline_keyboard": [][]Boton{}}}, nil)
		i.responder(ctx, chat, i.contestarAnimo(ctx, *r, v), nil)
	}
}
