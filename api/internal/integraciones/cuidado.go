package integraciones

// Lo de la v1.7 en la nube: un bicho que acompaña el día. El "¿Cómo estás?"
// por Telegram con botones (y la respuesta al bicho, marco 0x45), el "te
// extraño", su día contado a la hora de dormir (y GET …/resumen), el saludo
// entre robots del mismo dueño y los juegos (0x43). Todo con el historial
// (automatizaciones_historial) para no repetir, en la zona de los bichos
// (nube.Zona()) y respetando `cuidado` de la última config. Ver docs/nube.md,
// "Un bicho que acompaña el día".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// --- Lo que se lee de la config ---

// cuidado: lo de `cuidado` y `horario` que usa la nube, con sus valores por
// defecto (los del contrato: animo y extranar prendidos, 2 días, sin resumen;
// horario de 08:00 a 23:30).
type cuidado struct {
	animo, extranar, resumen bool
	extranarDias             int
	despierta, duerme        int // minutos desde las 00:00
}

func leerCuidado(texto string) cuidado {
	var c struct {
		Cuidado struct {
			Animo        *bool `json:"animo"`
			Extranar     *bool `json:"extranar"`
			ExtranarDias int   `json:"extranarDias"`
			Resumen      *bool `json:"resumen"`
		} `json:"cuidado"`
		Horario struct {
			Despierta string `json:"despierta"`
			Duerme    string `json:"duerme"`
		} `json:"horario"`
	}
	json.Unmarshal([]byte(texto), &c) //nolint:errcheck
	r := cuidado{animo: true, extranar: true, extranarDias: 2,
		despierta: minutosDe(c.Horario.Despierta, 8*60), duerme: minutosDe(c.Horario.Duerme, 23*60+30)}
	if v := c.Cuidado.Animo; v != nil {
		r.animo = *v
	}
	if v := c.Cuidado.Extranar; v != nil {
		r.extranar = *v
	}
	if v := c.Cuidado.Resumen; v != nil {
		r.resumen = *v
	}
	if d := c.Cuidado.ExtranarDias; d >= 1 && d <= 7 {
		r.extranarDias = d
	}
	return r
}

func minutosDe(hhmm string, siNo int) int {
	h, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		return siNo
	}
	return h.Hour()*60 + h.Minute()
}

func minutos(m time.Time) int { return m.Hour()*60 + m.Minute() }

// despierto: cuántos minutos pasaron desde que se despertó y cuánto dura el
// día del bicho (el horario puede cruzar la medianoche).
func (c cuidado) despierto(m time.Time) (desde, largo int) {
	desde = (minutos(m) - c.despierta + 1440) % 1440
	largo = (c.duerme - c.despierta + 1440) % 1440
	if largo == 0 {
		largo = 1440
	}
	return
}

// horaDelResumen: a la hora de dormir; si duerme pasada la medianoche (antes
// de las 06:00), a las 23:59 (así los "de hoy" son de hoy).
func (c cuidado) horaDelResumen() int {
	if c.duerme < 6*60 {
		return 23*60 + 59
	}
	return c.duerme
}

func inicioDelDia(m time.Time) time.Time {
	return time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, m.Location())
}

// --- Lo que se recuerda en memoria ---

type juegosDelDia struct {
	dia              uint8
	jugados, ganados int
}

type robotsDelDueno struct {
	ids []string
	en  time.Time
}

// estadoCuidado: lo que la nube lleva en memoria (el valor cero sirve).
type estadoCuidado struct {
	mu     sync.Mutex
	juegos map[string]juegosDelDia   // bicho → las partidas de hoy que vio la nube
	duenos map[string]robotsDelDueno // usuario → sus robots (caché de 1 min)
}

// anotarJuego: llegó el aviso de una partida (registro tipo 5).
func (i *Integraciones) anotarJuego(bicho string, datos map[string]any) {
	dia := protocolo.DiaContadores(time.Now().In(nube.Zona()))
	e := &i.cuid
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.juegos == nil {
		e.juegos = map[string]juegosDelDia{}
	}
	j := e.juegos[bicho]
	if j.dia != dia {
		j = juegosDelDia{dia: dia}
	}
	j.jugados++
	if r, _ := datos["resultado"].(string); r == "gano" {
		j.ganados++
	}
	e.juegos[bicho] = j
	if len(e.juegos) > 10000 {
		for k, v := range e.juegos {
			if v.dia != dia {
				delete(e.juegos, k)
			}
		}
	}
}

func (i *Integraciones) juegosVistos(bicho string, dia uint8) (juegosDelDia, bool) {
	i.cuid.mu.Lock()
	defer i.cuid.mu.Unlock()
	j, ok := i.cuid.juegos[bicho]
	return j, ok && j.dia == dia
}

// otrosRobots: los otros robots del dueño de este (caché de 1 min).
func (i *Integraciones) otrosRobots(bicho string) []string {
	usuario := i.robots.Dueno(bicho)
	if usuario == "" {
		return nil
	}
	e := &i.cuid
	e.mu.Lock()
	d, ok := e.duenos[usuario]
	e.mu.Unlock()
	if !ok || time.Since(d.en) > time.Minute {
		ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelar()
		l, err := i.alm.BichosDe(ctx, usuario)
		if err != nil {
			return nil
		}
		d = robotsDelDueno{en: time.Now()}
		for _, b := range l {
			d.ids = append(d.ids, b.ID)
		}
		e.mu.Lock()
		if e.duenos == nil || len(e.duenos) > 10000 {
			e.duenos = map[string]robotsDelDueno{}
		}
		e.duenos[usuario] = d
		e.mu.Unlock()
	}
	var otros []string
	for _, id := range d.ids {
		if id != bicho {
			otros = append(otros, id)
		}
	}
	return otros
}

// tieneOtroConectado: el dueño tiene otro robot conectado (para saludarse
// hace falta leer el registro, donde aparece "bicho-cerca").
func (i *Integraciones) tieneOtroConectado(bicho string) bool {
	for _, id := range i.otrosRobots(bicho) {
		if i.robots.Conectado(id) {
			return true
		}
	}
	return false
}

// --- Cada minuto, por robot (con Telegram) ---

// memoriaDe: la última memoria que mandó el bicho, al día (nil si no hay).
func (i *Integraciones) memoriaDe(ctx context.Context, bicho string, m time.Time) (*protocolo.Memoria, *time.Time) {
	g, err := i.alm.LeerMemoria(ctx, bicho)
	if err != nil || g.Datos == nil {
		return nil, nil
	}
	mem, err := protocolo.LeerMemoria(g.Datos)
	if err != nil {
		return nil, nil
	}
	mem.AlDia(m)
	return mem, g.Actualizada
}

// revisarCuidado: el "¿Cómo estás?", el "te extraño" y el resumen de un robot
// en el minuto m (hora de los bichos).
func (i *Integraciones) revisarCuidado(ctx context.Context, chat int64, r robot, cfg string, m time.Time) {
	c := leerCuidado(cfg)
	if !c.animo && !c.extranar && !c.resumen {
		return
	}
	mem, _ := i.memoriaDe(ctx, r.ID, m)
	if mem == nil {
		return // nunca mandó su memoria: no sabemos nada de su día
	}
	desde, largo := c.despierto(m)
	if c.animo && desde >= 120 && desde < largo && mem.AnimoHoy == 0 {
		i.unaVez(ctx, chat, r.ID, "como-estas", "animo", inicioDelDia(m), r.nombre+" pregunta: ¿Cómo estás hoy?", tecladoAnimo(r.ID))
	}
	if c.extranar && desde < largo && mem.UltimaInteraccion != 0 {
		ausencia := protocolo.EpochLocal(m) - int64(mem.UltimaInteraccion)
		dias := time.Duration(c.extranarDias) * 24 * time.Hour
		if ausencia >= int64(dias.Seconds()) && ausencia < 30*86400 {
			i.unaVez(ctx, chat, r.ID, "te-extrano", "cuidado", m.Add(-dias), "Te extraño 🥺 — "+r.nombre, nil)
		}
	}
	if c.resumen && minutos(m) == c.horaDelResumen() && mem.DeHoy(m) {
		res := i.resumenDe(r.ID, r.nombre, mem, m)
		i.unaVez(ctx, chat, r.ID, "resumen", "cuidado", inicioDelDia(m), res.Texto, nil)
	}
}

// unaVez manda el Telegram si en el historial no hay uno igual desde `desde`
// (y lo anota).
func (i *Integraciones) unaVez(ctx context.Context, chat int64, bicho, nombre, evento string, desde time.Time, texto string, teclado [][]Boton) {
	if ya, err := i.alm.YaCorrio(ctx, bicho, nombre, desde); err != nil || ya {
		return
	}
	err := i.tg.mandar(ctx, chat, texto, teclado)
	e := almacen.Ejecucion{Automatizacion: nombre, Evento: evento, Accion: "telegram", Ok: err == nil, En: time.Now().UTC()}
	if err != nil {
		e.Detalle = err.Error()
		i.log.Warn("cuidado: no pude mandar", "bicho", bicho, "que", nombre, "error", err)
	}
	i.alm.AnotarEjecucion(ctx, bicho, e) //nolint:errcheck
}

// --- ¿Cómo estás? ---

func tecladoAnimo(id string) [][]Boton {
	return [][]Boton{{{Texto: "😀 Bien", Datos: "animo:" + id + ":0"}, {Texto: "😐 Más o menos", Datos: "animo:" + id + ":1"},
		{Texto: "😞 Mal", Datos: "animo:" + id + ":2"}}}
}

// frasesLindas: para cuando contestan "mal".
var frasesLindas = []string{
	"Acordate de que los días feos también pasan.",
	"No tenés que poder con todo hoy. Con respirar un poco alcanza.",
	"Sos más fuerte de lo que pensás, aunque hoy no lo parezca.",
	"Tomá un vaso de agua, abrí la ventana y date un ratito. Te lo merecés.",
	"Está bien no estar bien. Mañana es otro día.",
	"Si querés, contale a alguien que te quiera cómo estás.",
}

// contestarAnimo: tocaron un botón del "¿Cómo estás?". Le manda al bicho el
// marco 0x45, lo anota y devuelve la respuesta con empatía.
func (i *Integraciones) contestarAnimo(ctx context.Context, r robot, valor int) string {
	marco, err := protocolo.Animo(valor)
	if err != nil {
		return "No entendí la respuesta."
	}
	errEnvio := i.robots.Enviar(r.ID, protocolo.CarComando, marco)
	e := almacen.Ejecucion{Automatizacion: "animo", Evento: "animo", Accion: "robot", Ok: errEnvio == nil, Detalle: protocolo.Animos[valor], En: time.Now().UTC()}
	if errEnvio != nil {
		e.Detalle += ": " + errEnvio.Error()
	}
	i.alm.AnotarEjecucion(ctx, r.ID, e) //nolint:errcheck
	var t string
	switch valor {
	case 0:
		t = "¡Qué bueno! " + r.nombre + " se puso contento de saberlo 😊"
	case 1:
		t = "Bueno, hay días así. " + r.nombre + " te manda un abrazo 🤗"
	default:
		t = "Uh, lo siento mucho. " + r.nombre + " está con vos 💛\n\n" + frasesLindas[rand.IntN(len(frasesLindas))]
	}
	if errors.Is(errEnvio, nube.ErrDesconectado) {
		t += "\n\n(" + r.nombre + " no está conectado ahora: no lo va a ver en su pantalla.)"
	} else if errEnvio != nil {
		i.log.Warn("animo: no se lo pude mandar", "bicho", r.ID, "error", errEnvio)
	}
	return t
}

// --- Su día contado ---

// Resumen: cómo fue el día del robot (GET /v1/bichos/{id}/resumen, el MCP y
// el Telegram de la hora de dormir).
type Resumen struct {
	Texto       string     `json:"texto"`
	Juegos      int        `json:"juegos"`
	Ganados     *int       `json:"ganados"` // null si la nube no vio todas las partidas
	Suenos      int        `json:"suenos"`
	SonoCon     []string   `json:"sonoCon"`
	Animo       *string    `json:"animo"`  // bien, medio, mal o null
	Espero      *int       `json:"espero"` // a qué hora te esperó (null = no)
	Agua        int        `json:"agua"`
	Actualizada *time.Time `json:"actualizada"` // cuándo mandó la memoria (null = nunca)
}

// ResumenDe arma el resumen de hoy de un robot (de su última memoria).
func (i *Integraciones) ResumenDe(ctx context.Context, bicho string) (Resumen, error) {
	m := time.Now().In(nube.Zona())
	nombre := i.nombreDe(ctx, bicho)
	mem, en := i.memoriaDe(ctx, bicho, m)
	if mem == nil {
		return Resumen{Texto: "Todavía no sé cómo fue el día de " + nombre + ": me lo cuenta cada 5 minutos mientras está conectado.", SonoCon: []string{}}, nil
	}
	r := i.resumenDe(bicho, nombre, mem, m)
	r.Actualizada = en
	return r, nil
}

func (i *Integraciones) nombreDe(ctx context.Context, bicho string) string {
	nombre := bicho
	b, err := i.alm.Bicho(ctx, bicho)
	if e, ok := i.robots.EstadoDe(bicho); ok && e.Nombre != "" {
		nombre = e.Nombre
	} else if err == nil && len(b.Estado) > 0 {
		if e, err := protocolo.LeerEstado(b.Estado); err == nil && e.Nombre != "" {
			nombre = e.Nombre
		}
	}
	if err == nil && b.Apodo != "" {
		nombre = b.Apodo
	}
	return nombre
}

func (i *Integraciones) resumenDe(bicho, nombre string, mem *protocolo.Memoria, m time.Time) Resumen {
	r := Resumen{Juegos: int(mem.JuegosHoy), Suenos: int(mem.SuenosHoy), SonoCon: mem.SonoConHoy, Espero: mem.EsperoHoy, Agua: int(mem.AguaHoy)}
	if r.SonoCon == nil {
		r.SonoCon = []string{}
	}
	if j, ok := i.juegosVistos(bicho, protocolo.DiaContadores(m)); ok && j.jugados >= r.Juegos {
		r.Juegos, r.Ganados = j.jugados, &j.ganados
	} else if r.Juegos == 0 {
		cero := 0
		r.Ganados = &cero
	}
	if a := int(mem.AnimoHoy); a >= 1 && a <= len(protocolo.Animos) {
		v := protocolo.Animos[a-1]
		r.Animo = &v
	}
	r.Texto = textoResumen(nombre, r)
	return r
}

// Cómo se cuenta con qué soñó.
var suenosEnTexto = map[string]string{"corazon": "corazones", "estrella": "estrellas", "nota": "música", "oveja": "ovejas"}

func veces(n int) string {
	if n == 1 {
		return "una vez"
	}
	return fmt.Sprintf("%d veces", n)
}

// enLista: "a", "a y b", "a, b y c".
func enLista(l []string) string {
	if len(l) <= 1 {
		return strings.Join(l, "")
	}
	return strings.Join(l[:len(l)-1], ", ") + " y " + l[len(l)-1]
}

// textoResumen: "Hoy Pipo jugó 3 veces (ganó 2), soñó con ovejas, te esperó a
// las 19 y le dijiste que estabas bien."
func textoResumen(robot string, r Resumen) string {
	var partes []string
	if r.Juegos > 0 {
		p := "jugó " + veces(r.Juegos)
		if r.Ganados != nil {
			switch {
			case r.Juegos == 1 && *r.Ganados == 1:
				p += " y ganó"
			case r.Juegos == 1:
			case *r.Ganados == 0:
				p += " (no ganó ninguna)"
			default:
				p += fmt.Sprintf(" (ganó %d)", *r.Ganados)
			}
		}
		partes = append(partes, p)
	}
	if r.Suenos > 0 {
		var con []string
		pesadilla := false
		for _, s := range r.SonoCon {
			if s == "pesadilla" {
				pesadilla = true
			} else if t, ok := suenosEnTexto[s]; ok {
				con = append(con, t)
			}
		}
		switch {
		case len(con) > 0:
			partes = append(partes, "soñó con "+enLista(con))
		case !pesadilla:
			partes = append(partes, "soñó "+veces(r.Suenos))
		}
		if pesadilla {
			partes = append(partes, "tuvo una pesadilla")
		}
	}
	if r.Espero != nil {
		partes = append(partes, fmt.Sprintf("te esperó a las %d", *r.Espero))
	}
	if r.Agua > 0 {
		partes = append(partes, "te recordó tomar agua "+veces(r.Agua))
	}
	if r.Animo != nil {
		partes = append(partes, "le dijiste que estabas "+map[string]string{"bien": "bien", "medio": "más o menos", "mal": "mal"}[*r.Animo])
	}
	if len(partes) == 0 {
		return "Hoy " + robot + " tuvo un día tranquilo."
	}
	return "Hoy " + robot + " " + enLista(partes) + "."
}

// --- Entre robots ---

// CadaSaludo: cada cuánto se pueden saludar dos robots.
const CadaSaludo = 10 * time.Minute

// saludarEntreRobots: el robot vio a otro cerca (bicho-cerca); si el dueño
// tiene otro robot conectado, los dos hacen "saludar" y se dicen hola (como
// mucho una vez cada 10 min por par).
func (i *Integraciones) saludarEntreRobots(bicho string) {
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	usuario := i.robots.Dueno(bicho)
	if usuario == "" {
		return
	}
	rs, err := i.robotsDe(ctx, usuario)
	if err != nil {
		return
	}
	var este *robot
	for k := range rs {
		if rs[k].ID == bicho {
			este = &rs[k]
		}
	}
	if este == nil || !este.conectado {
		return
	}
	desde := time.Now().Add(-CadaSaludo)
	for _, otro := range rs {
		if otro.ID == bicho || !otro.conectado {
			continue
		}
		par := min(bicho, otro.ID) + "|" + max(bicho, otro.ID)
		if !i.limite.permitir("entre:"+par, CadaSaludo, 1, CadaSaludo) {
			continue
		}
		if ya, err := i.alm.YaCorrio(ctx, bicho, "entre-"+otro.ID, desde); err != nil || ya {
			continue
		}
		if ya, err := i.alm.YaCorrio(ctx, otro.ID, "entre-"+bicho, desde); err != nil || ya {
			continue
		}
		i.saludar(ctx, *este, otro)
		i.saludar(ctx, otro, *este)
	}
}

// saludar: a manda "saludar" y dice "¡Hola, {b}!".
func (i *Integraciones) saludar(ctx context.Context, a, b robot) {
	err := i.mandarAlBicho(a.ID, protocolo.Comportamiento("saludar"))
	if err == nil {
		err = i.decirEnBicho(a.ID, "¡Hola, "+b.nombre+"!", "contento")
	}
	e := almacen.Ejecucion{Automatizacion: "entre-" + b.ID, Evento: "bicho-cerca", Accion: "saludar", Ok: err == nil, Detalle: "con " + b.nombre, En: time.Now().UTC()}
	if err != nil {
		e.Detalle += ": " + err.Error()
	}
	i.alm.AnotarEjecucion(ctx, a.ID, e) //nolint:errcheck
}

// --- Jugar ---

// Los juegos, como se los muestra al usuario.
var juegosEnTexto = map[string]string{"ppt": "✊ Piedra, papel o tijera", "reflejos": "⚡ Reflejos", "mirada": "👀 Seguí la mirada",
	"dado": "🎲 Dado", "moneda": "🪙 Moneda"}

func tecladoJuegos(id string) [][]Boton {
	var t [][]Boton
	for k := 0; k < len(protocolo.Juegos); k += 2 {
		fila := []Boton{}
		for _, j := range protocolo.Juegos[k:min(k+2, len(protocolo.Juegos))] {
			fila = append(fila, Boton{Texto: juegosEnTexto[j], Datos: "jugar:" + id + ":" + j})
		}
		t = append(t, fila)
	}
	return t
}

// jugar: el marco 0x43 (lo que hace /jugar).
func (i *Integraciones) jugar(r robot, juego string) string {
	m, err := protocolo.Jugar(juego)
	if err != nil {
		return "No conozco ese juego. Los que hay: dado, moneda, ppt (piedra, papel o tijera), reflejos y mirada."
	}
	if err := i.robots.Enviar(r.ID, protocolo.CarComando, m); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			return r.nombre + " no está conectado ahora."
		}
		i.log.Warn("telegram: no pude mandar el juego", "bicho", r.ID, "error", err)
		return "No pude empezar el juego con " + r.nombre + "."
	}
	j, _ := protocolo.NombreJuego(juego)
	if j == "terminar" {
		return "Listo: " + r.nombre + " dejó de jugar."
	}
	return "¡A jugar! " + r.nombre + " empezó " + juegosEnTexto[j] + ": mirá su pantalla."
}
