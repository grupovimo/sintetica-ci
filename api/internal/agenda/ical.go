// Package agenda lee un calendario iCal (.ics: la "dirección secreta en
// formato iCal" de Google Calendar, Outlook, iCloud o cualquier otro) y dice
// qué eventos caen entre dos momentos. Lo justo para la agenda de los robots:
// VEVENT con DTSTART/DTEND/DURATION (con TZID, en UTC, flotantes o de día
// entero), RRULE (DAILY, WEEKLY, MONTHLY y YEARLY con BYDAY, INTERVAL, COUNT y
// UNTIL; la expande github.com/teambition/rrule-go), EXDATE, RECURRENCE-ID
// (una repetición cambiada) y STATUS:CANCELLED. Ver docs/nube.md, "Agenda".
package agenda

import (
	"bufio"
	"bytes"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // las zonas de los TZID aunque la imagen no tenga /usr/share/zoneinfo

	"github.com/teambition/rrule-go"
)

// Límites (un .ics enorme o malicioso no puede trabar la nube).
const (
	MaxEventos     = 3000  // VEVENT por calendario
	MaxPasos       = 20000 // repeticiones que se miran por evento
	MaxOcurrencias = 500   // lo que devuelve Entre
	MaxTitulo      = 200   // bytes del título que se guarda
)

// Evento es una ocurrencia concreta.
type Evento struct {
	UID       string    `json:"uid"`
	Titulo    string    `json:"titulo"`
	Inicio    time.Time `json:"inicio"`
	Fin       time.Time `json:"fin"`
	TodoElDia bool      `json:"todoElDia"`
}

// Minutos que dura (0 si no dice).
func (e Evento) Minutos() int {
	d := e.Fin.Sub(e.Inicio)
	if d <= 0 {
		return 0
	}
	return int(d / time.Minute)
}

type vevento struct {
	uid, titulo  string
	inicio       time.Time
	duracion     time.Duration
	todoElDia    bool
	rrule        string
	exdates      []time.Time
	recurrencia  *time.Time
	cancelado    bool
	zonaDelStart *time.Location
}

// Calendario es un .ics leído.
type Calendario struct {
	eventos []vevento
	// cambiados: uid → los RECURRENCE-ID (en Unix) que reemplazan una repetición.
	cambiados map[string]map[int64]bool
}

// ErrNoEsCalendario: lo que bajó no es un .ics.
var ErrNoEsCalendario = errors.New("eso no es un calendario iCal (.ics)")

type linea struct {
	nombre string
	params map[string]string
	valor  string
}

// lineas desdobla (RFC 5545 3.1: una línea que empieza con espacio o tab
// sigue a la anterior) y separa nombre, parámetros y valor.
func lineas(datos []byte) []linea {
	var crudas []string
	sc := bufio.NewScanner(bytes.NewReader(datos))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(crudas) > 0 {
			crudas[len(crudas)-1] += l[1:]
			continue
		}
		crudas = append(crudas, l)
	}
	out := make([]linea, 0, len(crudas))
	for _, c := range crudas {
		if l, ok := partir(c); ok {
			out = append(out, l)
		}
	}
	return out
}

// partir: NOMBRE;P1=V1;P2="V;2":valor (los dos puntos dentro de comillas no cuentan).
func partir(s string) (linea, bool) {
	enComillas := false
	corte := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			enComillas = !enComillas
		case ':':
			if !enComillas {
				corte = i
			}
		}
		if corte >= 0 {
			break
		}
	}
	if corte < 0 {
		return linea{}, false
	}
	cabeza, valor := s[:corte], s[corte+1:]
	partes := strings.Split(cabeza, ";")
	l := linea{nombre: strings.ToUpper(strings.TrimSpace(partes[0])), valor: valor, params: map[string]string{}}
	for _, p := range partes[1:] {
		k, v, ok := strings.Cut(p, "=")
		if ok {
			l.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
		}
	}
	return l, true
}

// texto saca los escapes de un TEXT (\n, \, \; \\).
func texto(s string) string {
	return strings.NewReplacer(`\n`, " ", `\N`, " ", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(s)
}

// zonasWindows: los nombres que usa Outlook en vez de los de IANA.
var zonasWindows = map[string]string{
	"argentina standard time":        "America/Argentina/Buenos_Aires",
	"sa eastern standard time":       "America/Cayenne",
	"e. south america standard time": "America/Sao_Paulo",
	"pacific standard time":          "America/Los_Angeles",
	"mountain standard time":         "America/Denver",
	"central standard time":          "America/Chicago",
	"eastern standard time":          "America/New_York",
	"gmt standard time":              "Europe/London",
	"w. europe standard time":        "Europe/Berlin",
	"romance standard time":          "Europe/Paris",
	"central europe standard time":   "Europe/Budapest",
	"utc":                            "UTC",
}

func zonaDe(tzid string, porDefecto *time.Location) *time.Location {
	tzid = strings.TrimSpace(tzid)
	if tzid == "" {
		return porDefecto
	}
	// Algunos ponen un prefijo: "/mozilla.org/20050126_1/America/Argentina/Buenos_Aires".
	candidatos := []string{tzid}
	if i := strings.Index(tzid, "/"); i == 0 {
		p := strings.Split(strings.Trim(tzid, "/"), "/")
		for k := range p {
			candidatos = append(candidatos, strings.Join(p[k:], "/"))
		}
	}
	for _, c := range candidatos {
		if l, err := time.LoadLocation(c); err == nil {
			return l
		}
	}
	if n, ok := zonasWindows[strings.ToLower(tzid)]; ok {
		if l, err := time.LoadLocation(n); err == nil {
			return l
		}
	}
	return porDefecto
}

// fecha lee un DATE o DATE-TIME con su TZID. todoElDia si es un DATE.
func fecha(l linea, valor string, zona *time.Location) (time.Time, bool, *time.Location, error) {
	valor = strings.TrimSpace(valor)
	loc := zonaDe(l.params["TZID"], zona)
	if strings.EqualFold(l.params["VALUE"], "DATE") || len(valor) == 8 {
		t, err := time.ParseInLocation("20060102", valor, zona)
		return t, true, zona, err
	}
	if strings.HasSuffix(valor, "Z") {
		t, err := time.Parse("20060102T150405Z", valor)
		return t, false, time.UTC, err
	}
	t, err := time.ParseInLocation("20060102T150405", valor, loc)
	return t, false, loc, err
}

// duracion lee un DURATION (P1W, P1D, PT1H30M, -PT15M…).
func duracion(s string) (time.Duration, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	signo := time.Duration(1)
	if strings.HasPrefix(s, "-") {
		signo, s = -1, s[1:]
	}
	s = strings.TrimPrefix(s, "+")
	if !strings.HasPrefix(s, "P") {
		return 0, false
	}
	s = s[1:]
	var d time.Duration
	enHora := false
	num := ""
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			num += string(c)
		case c == 'T':
			enHora = true
		default:
			n, err := strconv.Atoi(num)
			if err != nil {
				return 0, false
			}
			num = ""
			switch {
			case c == 'W':
				d += time.Duration(n) * 7 * 24 * time.Hour
			case c == 'D':
				d += time.Duration(n) * 24 * time.Hour
			case c == 'H' && enHora:
				d += time.Duration(n) * time.Hour
			case c == 'M' && enHora:
				d += time.Duration(n) * time.Minute
			case c == 'S' && enHora:
				d += time.Duration(n) * time.Second
			default:
				return 0, false
			}
		}
	}
	return signo * d, num == ""
}

// Leer lee un .ics. zona es la de los eventos flotantes y de día entero (la
// de los robots).
func Leer(datos []byte, zona *time.Location) (*Calendario, error) {
	if !bytes.Contains(datos[:min(len(datos), 4096)], []byte("BEGIN:VCALENDAR")) {
		return nil, ErrNoEsCalendario
	}
	c := &Calendario{cambiados: map[string]map[int64]bool{}}
	var ev *vevento
	var fin *time.Time
	var dur *time.Duration
	profundidad := 0 // dentro de un VEVENT, los VALARM no cuentan
	for _, l := range lineas(datos) {
		switch l.nombre {
		case "BEGIN":
			v := strings.ToUpper(strings.TrimSpace(l.valor))
			if v == "VEVENT" && ev == nil {
				ev, fin, dur, profundidad = &vevento{}, nil, nil, 0
			} else if ev != nil {
				profundidad++
			}
			continue
		case "END":
			if ev == nil {
				continue
			}
			if profundidad > 0 {
				profundidad--
				continue
			}
			if strings.ToUpper(strings.TrimSpace(l.valor)) != "VEVENT" {
				continue
			}
			if !ev.inicio.IsZero() {
				switch {
				case dur != nil:
					ev.duracion = *dur
				case fin != nil:
					ev.duracion = fin.Sub(ev.inicio)
				case ev.todoElDia:
					ev.duracion = 24 * time.Hour
				}
				if ev.duracion < 0 {
					ev.duracion = 0
				}
				if ev.recurrencia != nil {
					if c.cambiados[ev.uid] == nil {
						c.cambiados[ev.uid] = map[int64]bool{}
					}
					c.cambiados[ev.uid][ev.recurrencia.Unix()] = true
				}
				if len(c.eventos) >= MaxEventos {
					return c, nil
				}
				c.eventos = append(c.eventos, *ev)
			}
			ev = nil
			continue
		}
		if ev == nil || profundidad > 0 {
			continue
		}
		switch l.nombre {
		case "UID":
			ev.uid = strings.TrimSpace(l.valor)
		case "SUMMARY":
			ev.titulo = cortar(strings.TrimSpace(texto(l.valor)), MaxTitulo)
		case "STATUS":
			ev.cancelado = strings.EqualFold(strings.TrimSpace(l.valor), "CANCELLED")
		case "DTSTART":
			if t, todo, loc, err := fecha(l, l.valor, zona); err == nil {
				ev.inicio, ev.todoElDia, ev.zonaDelStart = t, todo, loc
			}
		case "DTEND":
			if t, _, _, err := fecha(l, l.valor, zona); err == nil {
				fin = &t
			}
		case "DURATION":
			if d, ok := duracion(l.valor); ok {
				dur = &d
			}
		case "RRULE":
			ev.rrule = strings.TrimSpace(l.valor)
		case "EXDATE":
			for _, v := range strings.Split(l.valor, ",") {
				if t, _, _, err := fecha(l, v, zona); err == nil {
					ev.exdates = append(ev.exdates, t)
				}
			}
		case "RECURRENCE-ID":
			if t, _, _, err := fecha(l, l.valor, zona); err == nil {
				ev.recurrencia = &t
			}
		}
	}
	return c, nil
}

// cortar a n bytes sin partir una letra.
func cortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

// Frecuencias que se aceptan (las de cada segundo, minuto u hora no: un
// calendario no las usa y sirven para trabar la expansión).
var frecuencias = map[rrule.Frequency]bool{rrule.DAILY: true, rrule.WEEKLY: true, rrule.MONTHLY: true, rrule.YEARLY: true}

// ocurrencias de un evento que empiezan en [desde, hasta).
func (e vevento) ocurrencias(desde, hasta time.Time) []time.Time {
	unaVez := func() []time.Time {
		if !e.inicio.Before(desde) && e.inicio.Before(hasta) {
			return []time.Time{e.inicio}
		}
		return nil
	}
	if e.rrule == "" {
		return unaVez()
	}
	loc := e.zonaDelStart
	if loc == nil {
		loc = time.UTC
	}
	op, err := rrule.StrToROptionInLocation(e.rrule, loc)
	if err != nil || !frecuencias[op.Freq] {
		return unaVez() // una regla que no entendemos: por lo menos, la primera vez
	}
	op.Dtstart = e.inicio
	r, err := rrule.NewRRule(*op)
	if err != nil {
		return nil
	}
	set := &rrule.Set{}
	set.RRule(r)
	for _, x := range e.exdates {
		set.ExDate(x)
	}
	next := set.Iterator()
	var l []time.Time
	for paso := 0; paso < MaxPasos; paso++ {
		t, ok := next()
		if !ok || !t.Before(hasta) {
			break
		}
		if !t.Before(desde) {
			l = append(l, t)
		}
	}
	return l
}

// Entre devuelve las ocurrencias que todavía no terminaron en `desde` y que
// empiezan antes de `hasta`, por hora de inicio (como mucho MaxOcurrencias).
func (c *Calendario) Entre(desde, hasta time.Time) []Evento {
	var out []Evento
	for _, e := range c.eventos {
		if e.cancelado {
			continue
		}
		// Lo que empezó antes pero sigue en `desde` también cuenta.
		for _, t := range e.ocurrencias(desde.Add(-e.duracion), hasta) {
			if e.recurrencia == nil && c.cambiados[e.uid][t.Unix()] {
				continue // esa repetición la reemplaza otro VEVENT con RECURRENCE-ID
			}
			fin := t.Add(e.duracion)
			if e.duracion > 0 && !fin.After(desde) {
				continue
			}
			if e.duracion == 0 && t.Before(desde) {
				continue
			}
			titulo := e.titulo
			if titulo == "" {
				titulo = "(sin título)"
			}
			out = append(out, Evento{UID: e.uid, Titulo: titulo, Inicio: t, Fin: fin, TodoElDia: e.todoElDia})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if !out[a].Inicio.Equal(out[b].Inicio) {
			return out[a].Inicio.Before(out[b].Inicio)
		}
		return out[a].Titulo < out[b].Titulo
	})
	if len(out) > MaxOcurrencias {
		out = out[:MaxOcurrencias]
	}
	return out
}

// Cantidad de VEVENT leídos (para los tests y el log).
func (c *Calendario) Cantidad() int { return len(c.eventos) }
