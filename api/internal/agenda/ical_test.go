package agenda

import (
	"strings"
	"testing"
	"time"
)

var ba = func() *time.Location {
	l, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		panic(err)
	}
	return l
}()

func ics(eventos ...string) []byte {
	return []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//prueba//ES\r\n" + strings.Join(eventos, "") + "END:VCALENDAR\r\n")
}

func ev(lineas ...string) string {
	return "BEGIN:VEVENT\r\n" + strings.Join(lineas, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func horas(l []Evento) []string {
	var s []string
	for _, e := range l {
		s = append(s, e.Inicio.In(ba).Format("2006-01-02 15:04")+" "+e.Titulo)
	}
	return s
}

func igual(t *testing.T, que string, got []Evento, quiero ...string) {
	t.Helper()
	h := horas(got)
	if strings.Join(h, "|") != strings.Join(quiero, "|") {
		t.Errorf("%s:\n got   %q\n quiero %q", que, h, quiero)
	}
}

func leer(t *testing.T, b []byte) *Calendario {
	t.Helper()
	c, err := Leer(b, ba)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNoEsCalendario(t *testing.T) {
	if _, err := Leer([]byte("<html>hola</html>"), ba); err != ErrNoEsCalendario {
		t.Fatalf("html: %v", err)
	}
}

func TestEventoSimpleZonasYDobladas(t *testing.T) {
	c := leer(t, ics(
		ev("UID:1", "SUMMARY:Dentista\\, con Ana", "DTSTART;TZID=America/Argentina/Buenos_Aires:20261110T100000", "DTEND;TZID=America/Argentina/Buenos_Aires:20261110T110000"),
		// En UTC (13:00Z = 10:30 en Buenos Aires... no: 13:30Z = 10:30).
		ev("UID:2", "SUMMARY:Llamada con", "  Madrid", "DTSTART:20261110T133000Z", "DURATION:PT30M"),
		// Con un TZID de Windows (Outlook) y uno con prefijo.
		ev("UID:3", "SUMMARY:Outlook", `DTSTART;TZID="Argentina Standard Time":20261110T150000`, "DTEND;TZID=\"Argentina Standard Time\":20261110T153000"),
		ev("UID:4", "SUMMARY:Madrid", "DTSTART;TZID=/mozilla.org/20050126_1/Europe/Madrid:20261110T200000"),
		// Flotante (sin zona): la de los robots.
		ev("UID:5", "SUMMARY:Flotante", "DTSTART:20261110T180000"),
		// De día entero.
		ev("UID:6", "SUMMARY:Feriado", "DTSTART;VALUE=DATE:20261111", "DTEND;VALUE=DATE:20261112"),
		// Cancelado: no está.
		ev("UID:7", "SUMMARY:Cancelado", "STATUS:CANCELLED", "DTSTART:20261110T120000Z"),
		// Con una alarma adentro (su DESCRIPTION no pisa nada).
		ev("UID:8", "SUMMARY:Con alarma", "DTSTART:20261110T210000Z", "BEGIN:VALARM", "TRIGGER:-PT10M", "SUMMARY:otra cosa", "END:VALARM"),
	))
	desde := time.Date(2026, 11, 10, 0, 0, 0, 0, ba)
	l := c.Entre(desde, desde.Add(48*time.Hour))
	igual(t, "simples", l,
		"2026-11-10 10:00 Dentista, con Ana",
		"2026-11-10 10:30 Llamada con Madrid",
		"2026-11-10 15:00 Outlook",
		"2026-11-10 16:00 Madrid",
		"2026-11-10 18:00 Con alarma",
		"2026-11-10 18:00 Flotante",
		"2026-11-11 00:00 Feriado",
	)
	if l[0].Minutos() != 60 || l[1].Minutos() != 30 || !l[6].TodoElDia || l[6].Minutos() != 1440 {
		t.Errorf("duraciones: %d %d %v %d", l[0].Minutos(), l[1].Minutos(), l[6].TodoElDia, l[6].Minutos())
	}
	// Lo que ya empezó pero sigue también cuenta; lo que terminó, no.
	igual(t, "en curso", c.Entre(time.Date(2026, 11, 10, 10, 40, 0, 0, ba), time.Date(2026, 11, 10, 12, 0, 0, 0, ba)),
		"2026-11-10 10:00 Dentista, con Ana", "2026-11-10 10:30 Llamada con Madrid")
}

func TestRepeticiones(t *testing.T) {
	z := "DTSTART;TZID=America/Argentina/Buenos_Aires:"
	c := leer(t, ics(
		// Lunes, miércoles y viernes, 3 veces en total.
		ev("UID:a", "SUMMARY:Gimnasio", z+"20261102T080000", "RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=3"),
		// Todos los días hasta el 5, menos el 4.
		ev("UID:b", "SUMMARY:Pastilla", z+"20261102T220000", "RRULE:FREQ=DAILY;UNTIL=20261106T020000Z", "EXDATE;TZID=America/Argentina/Buenos_Aires:20261104T220000"),
		// Cada dos semanas.
		ev("UID:c", "SUMMARY:Terapia", z+"20261103T190000", "RRULE:FREQ=WEEKLY;INTERVAL=2"),
		// El primer lunes de cada mes.
		ev("UID:d", "SUMMARY:Reunión", z+"20261102T100000", "RRULE:FREQ=MONTHLY;BYDAY=1MO;COUNT=4"),
		// Una frecuencia que no se acepta: solo la primera vez.
		ev("UID:e", "SUMMARY:Molesto", z+"20261102T100000", "RRULE:FREQ=SECONDLY"),
	))
	desde := time.Date(2026, 11, 1, 0, 0, 0, 0, ba)
	igual(t, "noviembre", c.Entre(desde, time.Date(2026, 11, 20, 0, 0, 0, 0, ba)),
		"2026-11-02 08:00 Gimnasio",
		"2026-11-02 10:00 Molesto",
		"2026-11-02 10:00 Reunión",
		"2026-11-02 22:00 Pastilla",
		"2026-11-03 19:00 Terapia",
		"2026-11-03 22:00 Pastilla",
		"2026-11-04 08:00 Gimnasio",
		"2026-11-05 22:00 Pastilla",
		"2026-11-06 08:00 Gimnasio",
		"2026-11-17 19:00 Terapia",
	)
	var reuniones []Evento
	for _, e := range c.Entre(time.Date(2026, 12, 1, 0, 0, 0, 0, ba), time.Date(2027, 3, 10, 0, 0, 0, 0, ba)) {
		if e.UID == "d" {
			reuniones = append(reuniones, e)
		}
	}
	igual(t, "reuniones (COUNT=4)", reuniones, "2026-12-07 10:00 Reunión", "2027-01-04 10:00 Reunión", "2027-02-01 10:00 Reunión")
}

func TestRepeticionCambiada(t *testing.T) {
	z := "DTSTART;TZID=America/Argentina/Buenos_Aires:"
	c := leer(t, ics(
		ev("UID:x", "SUMMARY:Daily", z+"20261109T090000", "DURATION:PT15M", "RRULE:FREQ=DAILY;COUNT=3"),
		ev("UID:x", "RECURRENCE-ID;TZID=America/Argentina/Buenos_Aires:20261110T090000", "SUMMARY:Daily (movida)", z+"20261110T113000", "DURATION:PT15M"),
	))
	desde := time.Date(2026, 11, 9, 0, 0, 0, 0, ba)
	igual(t, "cambiada", c.Entre(desde, desde.Add(72*time.Hour)),
		"2026-11-09 09:00 Daily", "2026-11-10 11:30 Daily (movida)", "2026-11-11 09:00 Daily")
}

func TestLimites(t *testing.T) {
	var b strings.Builder
	for k := 0; k < MaxEventos+50; k++ {
		b.WriteString(ev("UID:n", "SUMMARY:x", "DTSTART:20261110T100000Z"))
	}
	c := leer(t, ics(b.String()))
	if c.Cantidad() != MaxEventos {
		t.Fatalf("eventos: %d", c.Cantidad())
	}
	desde := time.Date(2026, 11, 10, 0, 0, 0, 0, ba)
	if n := len(c.Entre(desde, desde.Add(24*time.Hour))); n != MaxOcurrencias {
		t.Fatalf("ocurrencias: %d", n)
	}
	// Un diario desde 1900 no traba: corta a los MaxPasos.
	c = leer(t, ics(ev("UID:v", "SUMMARY:viejo", "DTSTART:19000101T100000Z", "RRULE:FREQ=DAILY")))
	inicio := time.Now()
	c.Entre(desde, desde.Add(24*time.Hour))
	if time.Since(inicio) > 2*time.Second {
		t.Fatal("tarda demasiado")
	}
	if got := cortar("ñañañá", 5); got != "ñañ" {
		t.Fatalf("cortar: %q", got)
	}
}

func TestDuracion(t *testing.T) {
	for s, d := range map[string]time.Duration{"PT1H30M": 90 * time.Minute, "P1D": 24 * time.Hour, "P1W": 7 * 24 * time.Hour, "-PT15M": -15 * time.Minute, "P1DT2H": 26 * time.Hour} {
		if got, ok := duracion(s); !ok || got != d {
			t.Errorf("%s: %v %v", s, got, ok)
		}
	}
	for _, s := range []string{"", "1H", "PT1X", "PT"} {
		if _, ok := duracion(s); ok && s != "PT" {
			t.Errorf("%s debería fallar", s)
		}
	}
}
