package integraciones

// Amigos y toques (v1.9, docs/nube.md, "Amigos y toques"): las invitaciones
// de un solo uso, el toque (marco 0x47) a los robots de un amigo, su
// devolución (el evento devolver-toque vuelve al que tocó, dentro de 10 min)
// y el Telegram si el amigo no tiene ningún robot conectado. Los toques van
// solo entre amigos aceptados, con un tope por par, y de un amigo solo se ve
// su nombre.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// Límites de los amigos.
const (
	DuracionInvitacion  = 48 * time.Hour
	InvitacionesPorDia  = 20
	IntentosPorHora     = 10 // códigos probados por usuario
	ToquesPorHoraPorPar = 30 // en cada sentido
	VentanaDevolver     = 10 * time.Minute
	LargoCodigo         = 8
)

// Errores de los amigos (se muestran tal cual).
var (
	ErrMuchasInvitaciones = errors.New("pediste muchas invitaciones hoy: esperá a mañana")
	ErrMuchosIntentos     = errors.New("probaste muchos códigos: esperá un rato")
	ErrNoEsAmigo          = errors.New("no tenés ese amigo")
	ErrMuchosToques       = fmt.Errorf("ya le mandaste muchos toques: son hasta %d por hora", ToquesPorHoraPorPar)
)

// ErrToqueInvalido: un toque mal armado.
type ErrToqueInvalido struct{ Motivo string }

func (e ErrToqueInvalido) Error() string { return e.Motivo }

// Las letras de los códigos (sin I ni O, que se confunden con 1 y 0).
const letrasCodigo = "ABCDEFGHJKLMNPQRSTUVWXYZ"

func codigoInvitacion() string {
	b := make([]byte, 0, LargoCodigo)
	buf := make([]byte, 16)
	for len(b) < LargoCodigo {
		rand.Read(buf) //nolint:errcheck
		for _, x := range buf {
			// 240 = 10 × 24: así cada letra sale con la misma probabilidad.
			if x < 240 && len(b) < LargoCodigo {
				b = append(b, letrasCodigo[int(x)%len(letrasCodigo)])
			}
		}
	}
	return string(b)
}

func hashInvitacion(c string) []byte { h := sha256.Sum256([]byte("amigo:" + c)); return h[:] }

// NormalizarCodigo: mayúsculas, sin espacios ni guiones; ok si son 8 letras
// de las que se usan.
func NormalizarCodigo(c string) (string, bool) {
	c = strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.TrimSpace(c)))
	if len(c) != LargoCodigo {
		return "", false
	}
	for _, x := range c {
		if !strings.ContainsRune(letrasCodigo, x) {
			return "", false
		}
	}
	return c, true
}

// Invitar crea un código de 8 letras (48 h, un solo uso).
func (i *Integraciones) Invitar(ctx context.Context, usuario string) (string, time.Time, error) {
	n, err := i.alm.InvitacionesRecientes(ctx, usuario, time.Now().Add(-24*time.Hour))
	if err != nil {
		return "", time.Time{}, err
	}
	if n >= InvitacionesPorDia {
		return "", time.Time{}, ErrMuchasInvitaciones
	}
	codigo := codigoInvitacion()
	vence := time.Now().Add(DuracionInvitacion).UTC().Truncate(time.Second)
	return codigo, vence, i.alm.CrearInvitacion(ctx, usuario, hashInvitacion(codigo), vence)
}

// LinkTelegramAmigo: el link para aceptar la invitación desde el bot ("" sin Telegram).
func (i *Integraciones) LinkTelegramAmigo(codigo string) string {
	if i.tg == nil {
		return ""
	}
	return "https://t.me/" + i.op.TelegramBot + "?start=amigo-" + codigo
}

// Amigo es un amigo como lo ve el usuario: su nombre, qué robots míos
// reciben sus toques y si tiene algún robot conectado que reciba los míos.
type Amigo struct {
	ID        int64     `json:"id"`
	Nombre    string    `json:"nombre"`
	Robots    []string  `json:"robots"` // los míos que reciben sus toques
	Todos     bool      `json:"todos"`  // robots es "todos los míos" (también los que sume después)
	Conectado bool      `json:"conectado"`
	Creada    time.Time `json:"creada"`
}

func nombreAmigo(n string) string {
	if n = strings.TrimSpace(n); n != "" {
		return n
	}
	return "Tu amigo"
}

func (i *Integraciones) vistaAmigo(ctx context.Context, usuario string, am almacen.Amistad) (Amigo, error) {
	a := Amigo{ID: am.ID, Nombre: nombreAmigo(am.Nombre), Todos: am.Robots == nil, Robots: []string{}, Creada: am.Creada}
	mios, err := i.robotsQueReciben(ctx, usuario, am.Robots)
	if err != nil {
		return a, err
	}
	for _, r := range mios {
		a.Robots = append(a.Robots, r.ID)
	}
	suyos, err := i.robotsQueReciben(ctx, am.UsuarioID, am.SusRobots)
	if err != nil {
		return a, err
	}
	for _, r := range suyos {
		a.Conectado = a.Conectado || r.conectado
	}
	return a, nil
}

// Amigos lista los amigos del usuario.
func (i *Integraciones) Amigos(ctx context.Context, usuario string) ([]Amigo, error) {
	l, err := i.alm.Amigos(ctx, usuario)
	if err != nil {
		return nil, err
	}
	r := make([]Amigo, 0, len(l))
	for _, am := range l {
		a, err := i.vistaAmigo(ctx, usuario, am)
		if err != nil {
			return nil, err
		}
		r = append(r, a)
	}
	return r, nil
}

// Amigo devuelve uno (ErrNoEsAmigo si no es suyo).
func (i *Integraciones) Amigo(ctx context.Context, usuario string, id int64) (Amigo, error) {
	am, err := i.alm.AmistadDe(ctx, id, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return Amigo{}, ErrNoEsAmigo
	}
	if err != nil {
		return Amigo{}, err
	}
	return i.vistaAmigo(ctx, usuario, am)
}

// AceptarAmigo canjea un código. Al que invitó le llega un Telegram.
func (i *Integraciones) AceptarAmigo(ctx context.Context, usuario, codigo string) (Amigo, error) {
	if !i.limite.permitir("aceptar:"+usuario, 0, IntentosPorHora, time.Hour) {
		return Amigo{}, ErrMuchosIntentos
	}
	c, ok := NormalizarCodigo(codigo)
	if !ok {
		return Amigo{}, almacen.ErrInvitacionInvalida
	}
	am, err := i.alm.AceptarInvitacion(ctx, hashInvitacion(c), usuario)
	if err != nil {
		return Amigo{}, err
	}
	if i.tg != nil {
		yo, _ := i.alm.NombreDeUsuario(ctx, usuario)
		texto := "🤝 " + nombreAmigo(yo) + " aceptó tu invitación: ya se pueden mandar toques."
		if err := i.MandarTelegram(ctx, am.UsuarioID, texto); err != nil {
			i.log.Debug("amigos: sin Telegram para avisar", "error", err)
		}
	}
	return i.vistaAmigo(ctx, usuario, am)
}

// BorrarAmigo deja de ser amigos (para los dos).
func (i *Integraciones) BorrarAmigo(ctx context.Context, usuario string, id int64) error {
	err := i.alm.BorrarAmistad(ctx, id, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return ErrNoEsAmigo
	}
	return err
}

// PonerRobotsAmigo: qué robots míos reciben sus toques (nil = todos). Tienen
// que ser míos.
func (i *Integraciones) PonerRobotsAmigo(ctx context.Context, usuario string, id int64, robots []string) (Amigo, error) {
	if robots != nil {
		mios, err := i.alm.BichosDe(ctx, usuario)
		if err != nil {
			return Amigo{}, err
		}
		vistos := map[string]bool{}
		limpios := []string{}
		for _, r := range robots {
			r = strings.ToUpper(strings.TrimSpace(r))
			ok := false
			for _, b := range mios {
				ok = ok || b.ID == r
			}
			if !ok {
				return Amigo{}, ErrToqueInvalido{"ese robot no es tuyo"}
			}
			if !vistos[r] {
				vistos[r] = true
				limpios = append(limpios, r)
			}
		}
		robots = limpios
	}
	err := i.alm.PonerRobotsAmistad(ctx, id, usuario, robots)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return Amigo{}, ErrNoEsAmigo
	}
	if err != nil {
		return Amigo{}, err
	}
	return i.Amigo(ctx, usuario, id)
}

// robotsQueReciben: los robots del usuario que están en la lista (nil = todos).
func (i *Integraciones) robotsQueReciben(ctx context.Context, usuario string, lista []string) ([]robot, error) {
	rs, err := i.robotsDe(ctx, usuario)
	if err != nil || lista == nil {
		return rs, err
	}
	var r []robot
	for _, x := range rs {
		for _, id := range lista {
			if x.ID == id {
				r = append(r, x)
				break
			}
		}
	}
	return r, nil
}

// aceptarDesdeTelegram: "/start amigo-CODIGO" (el link de la invitación).
func (i *Integraciones) aceptarDesdeTelegram(ctx context.Context, chat int64, codigo string) string {
	v, err := i.alm.TelegramPorChat(ctx, chat)
	if err != nil {
		return "Para aceptar la invitación, primero vinculá este chat con tu cuenta: en la web, en Integraciones, tocá \"Vincular Telegram\". " +
			"Después abrí de nuevo el link de tu amigo."
	}
	a, err := i.AceptarAmigo(ctx, v.UsuarioID, codigo)
	switch {
	case err == nil:
		return "🤝 ¡Listo! Ahora sos amigo de " + a.Nombre + ": se pueden mandar toques, abrazos y besos entre sus robots."
	case errors.Is(err, almacen.ErrInvitacionInvalida):
		return "Esa invitación ya se usó o venció (valen 48 horas). Pedile una nueva."
	case errors.Is(err, almacen.ErrInvitacionPropia):
		return "Esa invitación es tuya: mandásela a un amigo."
	case errors.Is(err, almacen.ErrYaAmigos):
		return "Ya son amigos 🙂"
	case errors.Is(err, ErrMuchosIntentos):
		return err.Error() + "."
	}
	i.log.Warn("telegram: no pude aceptar la invitación", "error", err)
	return "Algo salió mal de nuestro lado. Probá de nuevo en un rato."
}

// --- Los toques ---

// ResultadoToque: a cuántos robots le llegó y si le llegó por Telegram.
type ResultadoToque struct {
	Robots   int  `json:"robots"`
	Telegram bool `json:"telegram"`
}

// toqueVisto: el último toque que le llegó a un robot (para devolverlo).
type toqueVisto struct {
	deUsuario   string // el que tocó
	desde       string // su robot, si lo mandó desde uno
	tipo, expre string
	en          time.Time
}

type estadoToques struct {
	mu      sync.Mutex
	ultimos map[string]toqueVisto // robot que lo recibió → el toque
}

func (e *estadoToques) anotar(robot string, t toqueVisto) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ultimos == nil || len(e.ultimos) > 10000 {
		e.ultimos = map[string]toqueVisto{}
	}
	e.ultimos[robot] = t
}

// tomar: el toque que le llegó a ese robot en los últimos 10 min (una vez).
func (e *estadoToques) tomar(robot string, ahora time.Time) (toqueVisto, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.ultimos[robot]
	delete(e.ultimos, robot)
	return t, ok && ahora.Sub(t.en) <= VentanaDevolver
}

var patronExpresionToque = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

// Lo que dice el Telegram de cada toque.
var toquesEnTexto = map[string]string{"toque": "te mandó un toque 👉", "abrazo": "te mandó un abrazo 🤗",
	"beso": "te mandó un beso 😘", "carita": "te mandó una carita 🙂"}

var devueltosEnTexto = map[string]string{"toque": "te devolvió el toque 👉", "abrazo": "te devolvió el abrazo 🤗",
	"beso": "te devolvió el beso 😘", "carita": "te devolvió la carita 🙂"}

// Tocar le manda un toque a un amigo: el 0x47 a sus robots conectados que
// reciben mis toques, con de = el nombre de mi robot (desde) o el mío. Si no
// tiene ninguno conectado y vinculó Telegram, le llega por ahí.
func (i *Integraciones) Tocar(ctx context.Context, usuario string, amistad int64, tipo, expresion, desde string) (ResultadoToque, error) {
	tipo = strings.ToLower(strings.TrimSpace(tipo))
	if tipo == "" {
		tipo = "toque"
	}
	if _, ok := protocolo.NumeroToque(tipo); !ok {
		return ResultadoToque{}, ErrToqueInvalido{"el tipo es toque, abrazo, beso o carita"}
	}
	switch {
	case tipo == "carita" && !patronExpresionToque.MatchString(expresion):
		return ResultadoToque{}, ErrToqueInvalido{"la carita lleva una expresión (como contento o guino)"}
	case tipo != "carita" && expresion != "":
		return ResultadoToque{}, ErrToqueInvalido{"la expresión es solo para la carita"}
	}
	am, err := i.alm.AmistadDe(ctx, amistad, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return ResultadoToque{}, ErrNoEsAmigo
	}
	if err != nil {
		return ResultadoToque{}, err
	}
	de, desdeID := "", ""
	if desde = strings.TrimSpace(desde); desde != "" {
		mios, err := i.robotsDe(ctx, usuario)
		if err != nil {
			return ResultadoToque{}, err
		}
		for _, r := range mios {
			if strings.EqualFold(r.ID, desde) {
				de, desdeID = r.nombre, r.ID
			}
		}
		if desdeID == "" {
			return ResultadoToque{}, ErrToqueInvalido{"ese robot no es tuyo"}
		}
	} else {
		n, err := i.alm.NombreDeUsuario(ctx, usuario)
		if err != nil {
			return ResultadoToque{}, err
		}
		de = n
	}
	if strings.TrimSpace(de) == "" {
		de = "Un amigo"
	}
	if !i.limite.permitir(fmt.Sprintf("toque:%d:%s", am.ID, usuario), 0, ToquesPorHoraPorPar, time.Hour) {
		return ResultadoToque{}, ErrMuchosToques
	}
	return i.entregarToque(ctx, am.UsuarioID, am.SusRobots, toqueVisto{deUsuario: usuario, desde: desdeID, tipo: tipo, expre: expresion, en: time.Now()}, de, false)
}

// entregarToque: el 0x47 a los robots conectados del destinatario (de la
// lista), el evento "toque" y, si no llegó a ninguno, el Telegram.
func (i *Integraciones) entregarToque(ctx context.Context, para string, lista []string, t toqueVisto, de string, devuelto bool) (ResultadoToque, error) {
	marco, err := protocolo.Toque(t.tipo, de, t.expre)
	if err != nil {
		return ResultadoToque{}, ErrToqueInvalido{err.Error()}
	}
	rs, err := i.robotsQueReciben(ctx, para, lista)
	if err != nil {
		return ResultadoToque{}, err
	}
	var res ResultadoToque
	for _, r := range rs {
		if !r.conectado {
			continue
		}
		if err := i.robots.Enviar(r.ID, protocolo.CarComando, marco); err != nil {
			i.log.Info("toque: no le llegó", "bicho", r.ID, "error", err)
			continue
		}
		res.Robots++
		if !devuelto {
			i.toques.anotar(r.ID, t)
		}
		datos := map[string]any{"de": de, "tipo": t.tipo, "robot": r.ID}
		if devuelto {
			datos["devuelto"] = true
		}
		i.hub.Publicar(para, eventos.Evento{Tipo: "toque", Bicho: r.ID, Datos: datos})
	}
	if res.Robots == 0 && i.tg != nil {
		textos := toquesEnTexto
		if devuelto {
			textos = devueltosEnTexto
		}
		if err := i.MandarTelegram(ctx, para, de+" "+textos[t.tipo]); err == nil {
			res.Telegram = true
		}
	}
	return res, nil
}

// devolverToque: un robot devolvió (con una caricia o un toque) el último
// toque que le llegó: vuelve al robot desde el que se mandó o, si no, a los
// del que tocó que reciben los toques de este dueño.
func (i *Integraciones) devolverToque(bicho string) {
	t, ok := i.toques.tomar(bicho, time.Now())
	if !ok {
		return
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	dueno := i.robots.Dueno(bicho)
	if dueno == "" {
		return
	}
	am, err := i.alm.AmistadEntre(ctx, dueno, t.deUsuario)
	if err != nil {
		return // ya no son amigos
	}
	if !i.limite.permitir(fmt.Sprintf("toque:%d:%s", am.ID, dueno), 0, ToquesPorHoraPorPar, time.Hour) {
		return
	}
	lista := am.SusRobots
	if t.desde != "" && i.robots.Dueno(t.desde) == t.deUsuario && i.robots.Conectado(t.desde) {
		lista = []string{t.desde}
	}
	if _, err := i.entregarToque(ctx, t.deUsuario, lista, t, i.nombreDe(ctx, bicho), true); err != nil {
		i.log.Warn("toque: no lo pude devolver", "bicho", bicho, "error", err)
	}
}
