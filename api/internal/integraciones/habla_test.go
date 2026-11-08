package integraciones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

func TestValidarRutinasYAccionesNuevas(t *testing.T) {
	w := NuevoClienteWebhook(false, nil)
	mio := func(string) bool { return true }
	buenas := []Automatizacion{
		{ID: "r1", Activa: true, Cuando: "hora", Hora: "07:30", Dias: "LMXJV", Hacer: []Accion{
			{Tipo: "decir", Texto: "Buen día, {dueno}: {clima}"},
			{Tipo: "decir", Texto: "hola", Expresion: "contento"},
			{Tipo: "cara", Expresion: "sorpresa"},
			{Tipo: "modo", Modo: "reloj"},
			{Tipo: "despertar"},
		}},
		{ID: "r2", Activa: true, Cuando: "hora", Hora: "23:59", Hacer: []Accion{{Tipo: "modo", Modo: "clima"}, {Tipo: "jugar", Juego: "dado"}, {Tipo: "jugar", Juego: "piedra papel tijera"}}},
		{ID: "d1", Activa: true, Cuando: "despertador", Hacer: []Accion{{Tipo: "telegram", Texto: "sonó"}}},
	}
	if err := Validar(buenas, mio, w); err != nil {
		t.Fatalf("las buenas: %v", err)
	}
	una := func(a Automatizacion) []Automatizacion { return []Automatizacion{a} }
	hora := func(h ...Accion) Automatizacion {
		return Automatizacion{ID: "a", Cuando: "hora", Hora: "08:00", Hacer: h}
	}
	malas := map[string][]Automatizacion{
		"hora sin hora":      una(Automatizacion{ID: "a", Cuando: "hora", Hacer: []Accion{{Tipo: "despertar"}}}),
		"hora mal":           una(Automatizacion{ID: "a", Cuando: "hora", Hora: "24:00", Hacer: []Accion{{Tipo: "despertar"}}}),
		"días raros":         una(Automatizacion{ID: "a", Cuando: "hora", Hora: "08:00", Dias: "LLX", Hacer: []Accion{{Tipo: "despertar"}}}),
		"días con minúscula": una(Automatizacion{ID: "a", Cuando: "hora", Hora: "08:00", Dias: "lm", Hacer: []Accion{{Tipo: "despertar"}}}),
		"hora en otro":       una(Automatizacion{ID: "a", Cuando: "tacto", Hora: "08:00", Hacer: []Accion{{Tipo: "despertar"}}}),
		"decir vacío":        una(hora(Accion{Tipo: "decir"})),
		"decir largo":        una(hora(Accion{Tipo: "decir", Texto: strings.Repeat("a", 201)})),
		"decir con url":      una(hora(Accion{Tipo: "decir", Texto: "x", URL: "https://ejemplo.com"})),
		"cara sin cara":      una(hora(Accion{Tipo: "cara"})),
		"modo raro":          una(hora(Accion{Tipo: "modo", Modo: "fiesta"})),
		"despertar con algo": una(hora(Accion{Tipo: "despertar", Texto: "x"})),
		"telegram con modo":  una(hora(Accion{Tipo: "telegram", Texto: "x", Modo: "reloj"})),
		"jugar sin juego":    una(hora(Accion{Tipo: "jugar"})),
		"jugar raro":         una(hora(Accion{Tipo: "jugar", Juego: "ajedrez"})),
		"jugar terminar":     una(hora(Accion{Tipo: "jugar", Juego: "fin"})),
		"jugar con texto":    una(hora(Accion{Tipo: "jugar", Juego: "dado", Texto: "x"})),
		"juego en otro":      una(hora(Accion{Tipo: "despertar", Juego: "dado"})),
	}
	for nombre, l := range malas {
		var inv ErrInvalida
		if err := Validar(l, mio, w); !errors.As(err, &inv) {
			t.Errorf("%s: esperaba inválida, dio %v", nombre, err)
		}
	}
	for _, c := range []string{"hora", "despertador"} {
		if !cuandoValido(c) {
			t.Errorf("falta el cuando %s", c)
		}
	}
}

func TestRutinaToca(t *testing.T) {
	a := Automatizacion{ID: "r", Activa: true, Cuando: "hora", Hora: "07:30", Dias: "LMXJV"}
	lunes := time.Date(2026, 11, 9, 7, 30, 0, 0, time.UTC)
	if !a.Toca(lunes) || a.Toca(lunes.Add(time.Minute)) || a.Toca(lunes.AddDate(0, 0, 5)) {
		t.Fatal("07:30 de lunes a viernes")
	}
	a.Activa = false
	if a.Toca(lunes) {
		t.Fatal("inactiva")
	}
}

func TestCumples(t *testing.T) {
	d := leerDatosConfig(`{"version":2,"nombre":"Pipo","dueno":{"nombre":"Ana","cumple":"11-08"},"cumple":"11-08","horario":{"despierta":"09:15"}}`)
	m := time.Date(2026, 11, 8, 9, 15, 0, 0, time.UTC)
	l := cumples(d, "Pipo", m)
	if len(l) != 2 || l[0] != [2]string{"cumple-dueno", "🎂 ¡Feliz cumple, Ana! Te saluda Pipo."} || l[1] != [2]string{"cumple-robot", "🎉 Hoy Pipo cumple años."} {
		t.Fatalf("%v", l)
	}
	if cumples(d, "Pipo", m.Add(time.Minute)) != nil || cumples(d, "Pipo", m.AddDate(0, 0, 1)) != nil {
		t.Fatal("solo a la hora de despertarse, ese día")
	}
	// Sin horario, a las 08:00; sin nombre del dueño, sin nombre.
	d = leerDatosConfig(`{"dueno":{"cumple":"02-29"}}`)
	if l := cumples(d, "Pipo", time.Date(2027, 2, 28, 8, 0, 0, 0, time.UTC)); len(l) != 1 || l[0][1] != "🎂 ¡Feliz cumple! Te saluda Pipo." {
		t.Fatalf("29/2 en un año sin: %v", l)
	}
	if cumples(d, "Pipo", time.Date(2028, 2, 28, 8, 0, 0, 0, time.UTC)) != nil {
		t.Fatal("en los bisiestos, el 29")
	}
	if leerDatosConfig(`{"ubicacion":{"lat":100,"lon":0,"ciudad":"x"}}`).Ubicacion != nil {
		t.Fatal("ubicación inválida")
	}
}

// --- Con la base ---

// climaFalso imita Open-Meteo.
func climaFalso(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"current":{"temperature_2m":18.4,"weather_code":3,"is_day":1},"daily":{"temperature_2m_max":[22],"temperature_2m_min":[12]}}`)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func guardarConfig(t *testing.T, a *almacen.Almacen, id, texto string) {
	t.Helper()
	if _, err := a.NuevaConfig(context.Background(), id, texto, protocolo.Crc32([]byte(texto)), "web", nil); err != nil {
		t.Fatal(err)
	}
}

func (r *robotsFalsos) vaciar() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	l := r.enviados
	r.enviados = nil
	return l
}

func TestClimaYAccionesEnElRobot(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{ClimaURL: climaFalso(t).URL}, a, robots, eventos.Nuevo(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	var cambios []string
	var mu sync.Mutex
	i.UsarConfig(func(_ context.Context, bicho, usuario, autor string, claves map[string]any) error {
		mu.Lock()
		defer mu.Unlock()
		cambios = append(cambios, fmt.Sprintf("%v %v %s %v", bicho == id, usuario == u.ID, autor, claves["modo"]))
		return nil
	})

	// Sin ubicación no se manda nada.
	i.mandarClima(ctx, id)
	if l := robots.vaciar(); len(l) != 0 {
		t.Fatalf("sin ubicación: %v", l)
	}
	guardarConfig(t, a, id, `{"version":2,"dueno":{"nombre":"Ana"},"ubicacion":{"lat":-34.6037,"lon":-58.3816,"ciudad":"Buenos Aires"}}`)
	i.mandarClima(ctx, id)
	marco := protocolo.Clima(18.4, 3, 22, 12, true, "Buenos Aires")
	if l := robots.vaciar(); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", id, marco) {
		t.Fatalf("clima: %v", l)
	}
	// Antes de los 30 min no se repite; si cambia el lugar, sí.
	i.mandarClima(ctx, id)
	if l := robots.vaciar(); len(l) != 0 {
		t.Fatalf("repetido: %v", l)
	}
	guardarConfig(t, a, id, `{"version":2,"dueno":{"nombre":"Ana"},"ubicacion":{"lat":-31.41,"lon":-64.18,"ciudad":"Córdoba"}}`)
	i.revisarClima(ctx)
	if l := robots.vaciar(); len(l) != 1 || !strings.Contains(l[0], fmt.Sprintf("%x", "Córdoba")) {
		t.Fatalf("lugar nuevo: %v", l)
	}
	if u, c := i.ClimaDe(ctx, id); u == nil || c == nil || c.Texto() != "18° y nublado" {
		t.Fatalf("ClimaDe: %v %v", u, c)
	}

	// Una rutina con todas las acciones nuevas.
	if err := i.GuardarAutomatizaciones(ctx, u.ID, id, []Automatizacion{{ID: "r1", Activa: true, Cuando: "hora", Hora: "07:30", Hacer: []Accion{
		{Tipo: "decir", Texto: "Buen día, {dueno}: {clima} en {ciudad} (máx. {max}). " + strings.Repeat("ñ", 40), Expresion: "contento"},
		{Tipo: "cara", Expresion: "sorpresa"},
		{Tipo: "modo", Modo: "reloj"},
		{Tipo: "despertar"},
		{Tipo: "jugar", Juego: "moneda"},
	}}}); err != nil {
		t.Fatal(err)
	}
	res, err := i.Probar(ctx, u.ID, id, "r1")
	if err != nil || len(res) != 5 {
		t.Fatalf("probar: %v %v", res, err)
	}
	for _, r := range res {
		if !r.Ok {
			t.Fatalf("%+v", res)
		}
	}
	dicho := protocolo.CortarUTF8("Buen día, Ana: 18° y nublado en Córdoba (máx. 22°). "+strings.Repeat("ñ", 40), 60)
	m1, _ := protocolo.Decir(dicho, "contento")
	m4, _ := protocolo.Alarma("despertar")
	m5, _ := protocolo.Jugar("moneda")
	want := []string{fmt.Sprintf("%s 2 %x", id, m1), fmt.Sprintf("%s 2 %x", id, protocolo.Expresion("sorpresa")), fmt.Sprintf("%s 2 %x", id, m4), fmt.Sprintf("%s 2 %x", id, m5)}
	if l := robots.vaciar(); strings.Join(l, "\n") != strings.Join(want, "\n") {
		t.Fatalf("al robot:\n%s\nesperaba:\n%s", strings.Join(l, "\n"), strings.Join(want, "\n"))
	}
	mu.Lock()
	if len(cambios) != 1 || cambios[0] != "true true automatizacion reloj" {
		t.Fatalf("modo: %v", cambios)
	}
	mu.Unlock()

	// La rutina se encola a su hora.
	lunes := time.Date(2026, 11, 9, 7, 30, 0, 0, time.UTC)
	i.revisarRutinas(ctx, lunes)
	select {
	case d := <-i.cola:
		if d.bicho != id || d.evento != "hora" || d.id != "r1" {
			t.Fatalf("disparo: %+v", d)
		}
	default:
		t.Fatal("no se encoló la rutina")
	}
	i.revisarRutinas(ctx, lunes.Add(time.Minute))
	if len(i.cola) != 0 {
		t.Fatal("a otra hora no")
	}
}

func TestTelegramDecirClimaYCumple(t *testing.T) {
	a := abrirBase(t)
	token := "123456:TOKEN-HABLA-" + azar(4)
	falso := &telegramFalso{token: token}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1, ClimaURL: climaFalso(t).URL}, a, robots, eventos.Nuevo(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	enlace, _, err := i.CrearVinculo(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	chat := chatAzar()
	if _, err := i.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UnixNano() % 1_000_000_000
	falso.mu.Lock()
	falso.updates = []map[string]any{
		mensajeDe(base+1, chat, "/clima"),
		mensajeDe(base+2, chat, "/decir ¡Hola, Pipo!"),
		mensajeDe(base+3, chat, "/decir "+strings.Repeat("a", 61)),
	}
	falso.mu.Unlock()
	go i.escucharTelegram(ctx)
	m := falso.esperar(t, 3)
	texto := func(k int) string { s, _ := m[k]["text"].(string); return s }
	if !strings.Contains(texto(0), "no sabe dónde está") {
		t.Fatalf("/clima sin ubicación: %q", texto(0))
	}
	if !strings.Contains(texto(1), "lo está diciendo") || !strings.Contains(texto(2), "hasta 60 bytes") {
		t.Fatalf("/decir: %q %q", texto(1), texto(2))
	}
	marco, _ := protocolo.Decir("¡Hola, Pipo!", "")
	if l := robots.vaciar(); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", id, marco) {
		t.Fatalf("al robot: %v", l)
	}

	// Con ubicación.
	hoy := time.Now().In(time.UTC)
	guardarConfig(t, a, id, fmt.Sprintf(`{"version":2,"dueno":{"nombre":"Ana","cumple":%q},"horario":{"despierta":%q},"ubicacion":{"lat":-34.6,"lon":-58.38,"ciudad":"Buenos Aires"}}`,
		hoy.Format("01-02"), hoy.Format("15:04")))
	falso.mu.Lock()
	falso.updates = []map[string]any{mensajeDe(base+4, chat, "/clima Pipo")}
	falso.mu.Unlock()
	m = falso.esperar(t, 4)
	if s, _ := m[3]["text"].(string); s != "En Buenos Aires: 18° y nublado (máx. 22°, mín. 12°)." {
		t.Fatalf("/clima: %q", s)
	}

	// El cumple: una sola vez por año.
	minuto := hoy.Truncate(time.Minute)
	i.revisarRecordatorios(ctx, minuto)
	i.revisarRecordatorios(ctx, minuto)
	m = falso.esperar(t, 5)
	time.Sleep(100 * time.Millisecond)
	falso.mu.Lock()
	n := len(falso.enviados)
	falso.mu.Unlock()
	if s, _ := m[4]["text"].(string); s != "🎂 ¡Feliz cumple, Ana! Te saluda Pipo." || n != 5 {
		t.Fatalf("cumple: %q (%d mensajes)", s, n)
	}
	_, h, _ := i.Automatizaciones(ctx, id)
	if len(h) != 1 || h[0].Automatizacion != "cumple-dueno" || !h[0].Ok {
		b, _ := json.Marshal(h)
		t.Fatalf("historial: %s", b)
	}
}
