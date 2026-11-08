package integraciones

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

func TestCodigoInvitacion(t *testing.T) {
	vistos := map[string]bool{}
	for k := 0; k < 200; k++ {
		c := codigoInvitacion()
		if n, ok := NormalizarCodigo(c); !ok || n != c || vistos[c] {
			t.Fatalf("código %q", c)
		}
		vistos[c] = true
	}
	if c, ok := NormalizarCodigo(" abcd-efgh "); !ok || c != "ABCDEFGH" {
		t.Fatalf("normalizar: %q", c)
	}
	for _, c := range []string{"ABCDEFG", "ABCDEFGHJ", "ABCDEFG1", "ABCDEFGI", "ABCDEFGÑ"} {
		if _, ok := NormalizarCodigo(c); ok {
			t.Fatalf("%q no es un código", c)
		}
	}
}

func TestRacha(t *testing.T) {
	z := nube.Zona()
	hoy := time.Date(2026, 11, 6, 12, 0, 0, 0, z) // viernes
	d := func(s ...string) map[string]bool {
		m := map[string]bool{}
		for _, x := range s {
			m["2026-11-"+x] = true
		}
		return m
	}
	casos := []struct {
		hechos map[string]bool
		dias   string
		racha  int
	}{
		{d(), "", 0},
		{d("06"), "", 1},
		{d("04", "05"), "", 2},            // hoy todavía no: sigue la de ayer
		{d("03", "05", "06"), "", 2},      // el 4 cortó
		{d("02", "03", "06"), "LMXJV", 1}, // el miércoles 4 y el jueves 5 tocaban
		{d("02", "06"), "LV", 2},          // lunes 2 y viernes 6; el viernes 30 de octubre tocaba
		{d("05", "06"), "SD", 2},          // hechos en días que no tocan también suman
	}
	for k, c := range casos {
		if r := racha(c.hechos, c.dias, hoy); r != c.racha {
			t.Errorf("caso %d: racha %d, esperaba %d", k, r, c.racha)
		}
	}
}

func TestAprenderHorarios(t *testing.T) {
	z := nube.Zona()
	ahora := time.Date(2026, 11, 8, 12, 0, 0, 0, z)
	var ps []presenciaLocal
	for k := 10; k >= 1; k-- {
		dia := ahora.AddDate(0, 0, -k)
		en := func(h, m int) time.Time { return time.Date(dia.Year(), dia.Month(), dia.Day(), h, m, 0, 0, z) }
		ps = append(ps,
			presenciaLocal{"te-fuiste", en(8, 15+k%3*5)}, // 08:15, 08:20, 08:25
			presenciaLocal{"llegaste", en(19, 10)},
			presenciaLocal{"te-fuiste", en(21, 0)}, // una salida corta: no es la más larga
			presenciaLocal{"llegaste", en(21, 20)},
			presenciaLocal{"duerme", en(23, 50).Add(40 * time.Minute)}, // 00:30 del día siguiente
			presenciaLocal{"duerme", en(15, 0)},                        // una siesta: no cuenta
		)
	}
	h := aprenderHorarios(ps, ahora)
	if h.Llegas == nil || *h.Llegas != "19:10" || h.Salis == nil || *h.Salis != "08:20" || h.Dormis == nil || *h.Dormis != "00:30" {
		t.Fatalf("horarios: %+v", h)
	}
	if h.Seguridad < 0.8 || h.Seguridad > 1 || h.Dias < 10 {
		t.Fatalf("seguridad %v, días %d", h.Seguridad, h.Dias)
	}
	if v := aprenderHorarios(nil, ahora); v.Llegas != nil || v.Seguridad != 0 {
		t.Fatalf("sin datos: %+v", v)
	}
}

// dosAmigos: dos usuarios con un robot conectado cada uno, ya amigos.
func dosAmigos(t *testing.T, a *almacen.Almacen, i *Integraciones, robots *robotsFalsos) (ana, beto almacen.Usuario, ra, rb string, am Amigo) {
	t.Helper()
	ctx := context.Background()
	ana, ra = usuarioConBicho(t, a)
	beto, rb = usuarioConBicho(t, a)
	a.ActualizarNombre(ctx, ana.ID, "Ana")   //nolint:errcheck
	a.ActualizarNombre(ctx, beto.ID, "Beto") //nolint:errcheck
	robots.mu.Lock()
	robots.duenos[ra], robots.duenos[rb] = ana.ID, beto.ID
	robots.estados[ra], robots.estados[rb] = protocolo.Estado{Nombre: "Pipo"}, protocolo.Estado{Nombre: "Lola"}
	robots.mu.Unlock()
	codigo, vence, err := i.Invitar(ctx, ana.ID)
	if err != nil || time.Until(vence) < 47*time.Hour {
		t.Fatalf("invitar: %v %v", err, vence)
	}
	if _, err := i.AceptarAmigo(ctx, ana.ID, codigo); !errors.Is(err, almacen.ErrInvitacionPropia) {
		t.Fatalf("la propia: %v", err)
	}
	am, err = i.AceptarAmigo(ctx, beto.ID, strings.ToLower(codigo))
	if err != nil || am.Nombre != "Ana" || !am.Todos || len(am.Robots) != 1 || am.Robots[0] != rb || !am.Conectado {
		t.Fatalf("aceptar: %+v %v", am, err)
	}
	if _, err := i.AceptarAmigo(ctx, beto.ID, codigo); !errors.Is(err, almacen.ErrInvitacionInvalida) {
		t.Fatalf("un solo uso: %v", err)
	}
	otro, _, _ := i.Invitar(ctx, beto.ID)
	if _, err := i.AceptarAmigo(ctx, ana.ID, otro); !errors.Is(err, almacen.ErrYaAmigos) {
		t.Fatalf("ya amigos: %v", err)
	}
	return
}

func (r *robotsFalsos) vaciarDe(id string) []string {
	var l []string
	for _, x := range r.vaciar() {
		if strings.HasPrefix(x, id+" ") {
			l = append(l, x)
		}
	}
	return l
}

func TestAmigosYToques(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	hub := eventos.Nuevo()
	robots := &robotsFalsos{duenos: map[string]string{}, estados: map[string]protocolo.Estado{}}
	i := Nuevo(Opciones{}, a, robots, hub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ana, beto, ra, rb, deBeto := dosAmigos(t, a, i, robots)
	subAna := hub.Suscribir(ana.ID, "")
	defer hub.Desuscribir(subAna)

	// Beto le manda un abrazo a Ana desde su robot (Lola).
	lAna, _ := i.Amigos(ctx, ana.ID)
	if len(lAna) != 1 || lAna[0].Nombre != "Beto" || lAna[0].ID != deBeto.ID {
		t.Fatalf("los amigos de Ana: %+v", lAna)
	}
	res, err := i.Tocar(ctx, beto.ID, deBeto.ID, "abrazo", "", rb)
	if err != nil || res.Robots != 1 || res.Telegram {
		t.Fatalf("tocar: %+v %v", res, err)
	}
	marco, _ := protocolo.Toque("abrazo", "Lola", "")
	if l := robots.vaciarDe(ra); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", ra, marco) {
		t.Fatalf("al robot de Ana: %v", l)
	}
	select {
	case e := <-subAna.C:
		d, _ := e.Datos.(map[string]any)
		if e.Tipo != "toque" || e.Bicho != ra || d["de"] != "Lola" || d["tipo"] != "abrazo" {
			t.Fatalf("evento: %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("sin evento toque")
	}

	// Ana lo acaricia: vuelve a Lola, con el nombre de Pipo (una sola vez).
	i.Evento(ra, "devolver-toque", map[string]any{})
	devuelto, _ := protocolo.Toque("abrazo", "Pipo", "")
	esperar := func(id string, n int) []string {
		var l []string
		for k := 0; k < 100 && len(l) < n; k++ {
			time.Sleep(20 * time.Millisecond)
			l = append(l, robots.vaciarDe(id)...)
		}
		return l
	}
	if l := esperar(rb, 1); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", rb, devuelto) {
		t.Fatalf("devuelto: %v", l)
	}
	i.Evento(ra, "devolver-toque", map[string]any{})
	if l := esperar(rb, 1); len(l) != 0 {
		t.Fatalf("se devolvió dos veces: %v", l)
	}

	// Validaciones y privacidad.
	if _, err := i.Tocar(ctx, beto.ID, deBeto.ID, "pellizco", "", ""); !errors.As(err, &ErrToqueInvalido{}) {
		t.Fatalf("tipo raro: %v", err)
	}
	if _, err := i.Tocar(ctx, beto.ID, deBeto.ID, "carita", "", ""); !errors.As(err, &ErrToqueInvalido{}) {
		t.Fatalf("carita sin expresión: %v", err)
	}
	if _, err := i.Tocar(ctx, beto.ID, deBeto.ID, "toque", "", ra); !errors.As(err, &ErrToqueInvalido{}) {
		t.Fatalf("desde un robot ajeno: %v", err)
	}
	carlos, rc := usuarioConBicho(t, a)
	if _, err := i.Tocar(ctx, carlos.ID, deBeto.ID, "toque", "", ""); !errors.Is(err, ErrNoEsAmigo) {
		t.Fatalf("un extraño: %v", err)
	}
	if err := i.BorrarAmigo(ctx, carlos.ID, deBeto.ID); !errors.Is(err, ErrNoEsAmigo) {
		t.Fatalf("borrar ajeno: %v", err)
	}
	if _, err := i.PonerRobotsAmigo(ctx, beto.ID, deBeto.ID, []string{rc}); !errors.As(err, &ErrToqueInvalido{}) {
		t.Fatalf("robots ajenos: %v", err)
	}
	// Ana no recibe en ningún robot: no le llega (y sin Telegram, tampoco).
	if _, err := i.PonerRobotsAmigo(ctx, ana.ID, lAna[0].ID, []string{}); err != nil {
		t.Fatal(err)
	}
	if res, err := i.Tocar(ctx, beto.ID, deBeto.ID, "carita", "guino", ""); err != nil || res.Robots != 0 {
		t.Fatalf("sin robots: %+v %v", res, err)
	}
	// El tope por par.
	for k := 0; k < ToquesPorHoraPorPar; k++ {
		i.Tocar(ctx, beto.ID, deBeto.ID, "toque", "", "") //nolint:errcheck
	}
	if _, err := i.Tocar(ctx, beto.ID, deBeto.ID, "toque", "", ""); !errors.Is(err, ErrMuchosToques) {
		t.Fatalf("tope: %v", err)
	}
	if err := i.BorrarAmigo(ctx, ana.ID, lAna[0].ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := i.Amigos(ctx, beto.ID); len(l) != 0 {
		t.Fatalf("borrado: %+v", l)
	}
}

func TestToquePorTelegram(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	token := "123456:TOKEN-AMIGOS-" + azar(4)
	falso := &telegramFalso{token: token}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	robots := &robotsFalsos{duenos: map[string]string{}, estados: map[string]protocolo.Estado{}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1}, a, robots, eventos.Nuevo(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ana, beto, ra, _, deBeto := dosAmigos(t, a, i, robots)
	enlace, _, _ := i.CrearVinculo(ctx, ana.ID)
	chat := chatAzar()
	if _, err := i.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i.Desvincular(context.Background(), ana.ID) }) //nolint:errcheck
	robots.mu.Lock()
	delete(robots.estados, ra) // Pipo se desconectó
	robots.mu.Unlock()
	res, err := i.Tocar(ctx, beto.ID, deBeto.ID, "beso", "", "")
	if err != nil || res.Robots != 0 || !res.Telegram {
		t.Fatalf("tocar: %+v %v", res, err)
	}
	m := falso.esperar(t, 1)
	if s, _ := m[len(m)-1]["text"].(string); s != "Beto te mandó un beso 😘" {
		t.Fatalf("telegram: %q", s)
	}

	// La invitación por el link del bot.
	if l := i.LinkTelegramAmigo("ABCDEFGH"); l != "https://t.me/sinteticalabs_bot?start=amigo-ABCDEFGH" {
		t.Fatalf("link: %s", l)
	}
	carlos, _ := usuarioConBicho(t, a)
	codigo, _, _ := i.Invitar(ctx, carlos.ID)
	if s := i.aceptarDesdeTelegram(ctx, chat, codigo); !strings.Contains(s, "Ahora sos amigo de") {
		t.Fatalf("aceptar por Telegram: %q", s)
	}
	if s := i.aceptarDesdeTelegram(ctx, chatAzar(), codigo); !strings.Contains(s, "primero vinculá") {
		t.Fatalf("sin vincular: %q", s)
	}
}

func TestPresenciaYHabitos(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{}, a, robots, eventos.Nuevo(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	// La app y el robot avisan la misma llegada: es una sola.
	nueva, enviado, err := i.Presencia(ctx, id, true)
	if err != nil || !nueva || !enviado {
		t.Fatalf("presencia: %v %v %v", nueva, enviado, err)
	}
	if l := robots.vaciar(); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", id, protocolo.Presencia(true)) {
		t.Fatalf("al robot: %v", l)
	}
	if nueva, _, _ := i.Presencia(ctx, id, true); nueva {
		t.Fatal("la misma llegada dos veces")
	}
	i.presenciaDelRobot(id, "llegaste")
	i.presenciaDelRobot(id, "te-fuiste")
	if nueva, _, _ := i.Presencia(ctx, id, false); nueva {
		t.Fatal("la salida del robot y la de la app son la misma")
	}
	l, _ := a.Presencias(ctx, id, time.Now().Add(-time.Hour))
	if len(l) != 2 || l[0].Tipo != "llegaste" || l[1].Tipo != "te-fuiste" {
		t.Fatalf("anotadas: %+v", l)
	}
	if h, err := i.HorariosDe(ctx, id); err != nil || h.Seguridad > 0.2 {
		t.Fatalf("horarios: %+v %v", h, err)
	}

	// Hábitos.
	if hs, err := i.Habitos(ctx, id); err != nil || len(hs) != 0 {
		t.Fatalf("sin config: %+v %v", hs, err)
	}
	if _, _, err := i.MarcarHabito(ctx, id, 0, true, false); !errors.Is(err, ErrSinHabito) {
		t.Fatalf("sin hábito: %v", err)
	}
	cfg := `{"version":2,"habitos":[{"nombre":"agua","icono":"gota","hora":"10:00"},{"nombre":"leer","icono":"libro","dias":"LMXJV"}]}`
	if _, err := a.NuevaConfig(ctx, id, cfg, 1, "api", nil); err != nil {
		t.Fatal(err)
	}
	hoy := time.Now().In(nube.Zona())
	ayer := hoy.AddDate(0, 0, -1)
	if err := a.MarcarHabito(ctx, id, 0, "agua", ayer, true); err != nil {
		t.Fatal(err)
	}
	h, enviado, err := i.MarcarHabito(ctx, id, 0, true, false)
	if err != nil || !enviado || h.Racha != 2 || !h.Hoy || !h.Dias[hoy.Format("2006-01-02")] || !h.Dias[ayer.Format("2006-01-02")] {
		t.Fatalf("marcar: %+v %v %v", h, enviado, err)
	}
	m, _ := protocolo.Habito(0, true, 2)
	if l := robots.vaciar(); len(l) != 1 || l[0] != fmt.Sprintf("%s 2 %x", id, m) {
		t.Fatalf("al robot: %v", l)
	}
	// Lo marca el robot (registro tipo 8): se anota y no se le manda nada.
	i.habitoDelRobot(id, map[string]any{"indice": 1})
	hs, _ := i.Habitos(ctx, id)
	if len(hs) != 2 || !hs[1].Hoy || hs[1].Nombre != "leer" || hs[1].Icono != "libro" {
		t.Fatalf("hábitos: %+v", hs)
	}
	if l := robots.vaciar(); len(l) != 0 {
		t.Fatalf("al robot que lo marcó: %v", l)
	}
	if h, _, err := i.MarcarHabito(ctx, id, 0, false, false); err != nil || h.Hoy || h.Racha != 1 {
		t.Fatalf("deshacer: %+v %v", h, err)
	}
	// Si cambia el hábito de ese lugar, empieza de cero.
	cfg = `{"version":2,"habitos":[{"nombre":"correr","icono":"pesa"}]}`
	if _, err := a.NuevaConfig(ctx, id, cfg, 2, "api", nil); err != nil {
		t.Fatal(err)
	}
	if hs, _ := i.Habitos(ctx, id); len(hs) != 1 || hs[0].Racha != 0 || len(hs[0].Dias) != 0 {
		t.Fatalf("otro hábito: %+v", hs)
	}
	// Los cuandos nuevos se pueden usar.
	for _, c := range []string{"llegaste", "te-fuiste", "habito"} {
		if err := Validar([]Automatizacion{{ID: "x", Activa: true, Cuando: c, Hacer: []Accion{{Tipo: "telegram", Texto: "{habito} {racha}"}}}},
			func(string) bool { return true }, NuevoClienteWebhook(false, nil)); err != nil {
			t.Fatalf("cuando %s: %v", c, err)
		}
	}
}
