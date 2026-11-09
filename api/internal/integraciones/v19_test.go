package integraciones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

var sinLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// --- Sin base ---

func TestNormalizarICS(t *testing.T) {
	i := Nuevo(Opciones{HostsPropios: []string{"https://api.sintetica.test"}}, nil, &robotsFalsos{}, eventos.Nuevo(), sinLog)
	bien := map[string]string{
		"webcal://calendar.google.com/calendar/ical/a%40b.com/private-xyz/basic.ics": "https://calendar.google.com/calendar/ical/a%40b.com/private-xyz/basic.ics",
		" https://outlook.office365.com/owa/calendar/abc/reachcalendar.ics ":         "https://outlook.office365.com/owa/calendar/abc/reachcalendar.ics",
		"WEBCAL://p01-caldav.icloud.com/published/2/abc":                             "https://p01-caldav.icloud.com/published/2/abc",
	}
	for in, quiero := range bien {
		if got, err := i.normalizarICS(in); err != nil || got != quiero {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, mal := range []string{"", "http://calendar.google.com/x.ics", "ftp://x.com/a.ics", "https://127.0.0.1/a.ics", "https://localhost/a.ics",
		"https://10.0.0.2/a.ics", "https://usuario:clave@x.com/a.ics", "https://api.sintetica.test/v1/x.ics", "https://postgres/a.ics", "calendario"} {
		var inv ErrAgendaInvalida
		if _, err := i.normalizarICS(mal); !errors.As(err, &inv) {
			t.Errorf("%q debería fallar: %v", mal, err)
		}
	}
	if p := pistaICS("https://calendar.google.com/calendar/ical/ana%40gmail.com/private-0123456789abcdef/basic.ics"); p != "calendar.google.com/…/basic.ics" {
		t.Errorf("pista: %q", p)
	}
	if p := pistaICS("https://x.com/a/b/" + strings.Repeat("s", 40) + "abcdef.ics"); strings.Contains(p, strings.Repeat("s", 20)) {
		t.Errorf("la pista no puede mostrar el secreto: %q", p)
	}
}

func TestLimpiarRespuesta(t *testing.T) {
	validas := []string{"contento", "triste"}
	largo := strings.Repeat("ñ", 40) // 80 bytes
	t1, e1 := limpiarRespuesta("\"Hola\n  "+largo+"\"", "inventada", validas)
	if len(t1) > protocolo.MaxDecir || !utf8.ValidString(t1) || !strings.HasPrefix(t1, "Hola ñ") || e1 != "" {
		t.Fatalf("%q (%d) %q", t1, len(t1), e1)
	}
	if t2, e2 := limpiarRespuesta("  ", "triste", validas); t2 != "…" || e2 != "triste" {
		t.Fatalf("%q %q", t2, e2)
	}
	p := promptCharla(datosCharla{Nombre: "Pipo", Personalidad: personalidades["grunon"], Dueno: "Ana", Hora: "10:00 del lunes",
		Ciudad: "Córdoba", Clima: "soleado", Memoria: "hace 3 días que están juntos", Expresiones: []string{"contento"}})
	for _, s := range []string{"Sos Pipo", "gruñón", "Ana", "Córdoba", "soleado", "hace 3 días", "contento", "60 bytes"} {
		if !strings.Contains(p, s) {
			t.Errorf("al prompt le falta %q", s)
		}
	}
	if len(p) > 2000 {
		t.Errorf("el prompt es largo: %d", len(p))
	}
}

func TestMinutosQueFaltan(t *testing.T) {
	a := time.Date(2026, 11, 8, 9, 50, 10, 0, time.UTC)
	for d, quiero := range map[time.Duration]int{9*time.Minute + 50*time.Second: 10, 10 * time.Second: 1, 0: 0, -time.Minute: 0} {
		if got := minutosQueFaltan(a, a.Add(d)); got != quiero {
			t.Errorf("%v: %d", d, got)
		}
	}
	if !cuandoValido("agenda") {
		t.Fatal("agenda tiene que ser un cuando")
	}
}

// --- Con la base ---

func enviadosDe(r *robotsFalsos) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.enviados, "\n")
}

func vaciar(r *robotsFalsos) {
	r.mu.Lock()
	r.enviados = nil
	r.mu.Unlock()
}

func esperarEnviado(t *testing.T, r *robotsFalsos, s string) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if strings.Contains(enviadosDe(r), s) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no le llegó %s; llegó:\n%s", s, enviadosDe(r))
}

func TestAgendaConICSFalso(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	// Las 12 de hoy (hora de Argentina): con la hora de verdad, a la noche
	// "Más tarde" (3 h después) caía mañana y el resumen cambiaba.
	fijo := inicioDelDia(time.Now().In(nube.Zona())).Add(12*time.Hour + 10*time.Second)
	z := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	manana := inicioDelDia(fijo.In(nube.Zona())).AddDate(0, 0, 1).Add(10 * time.Hour)
	var mu sync.Mutex
	cuerpo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nUID:pronto\r\nSUMMARY:Llamada con Ana\r\nDTSTART:" + z(fijo.Truncate(time.Minute).Add(8*time.Minute)) + "\r\nDURATION:PT30M\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:luego\r\nSUMMARY:Más tarde\r\nDTSTART:" + z(fijo.Add(3*time.Hour)) + "\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:manana\r\nSUMMARY:Dentista\r\nDTSTART:" + z(manana) + "\r\nDURATION:PT1H\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	pedidos := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		pedidos++
		switch r.URL.Path {
		case "/private-0123456789abcdef/basic.ics":
			w.Header().Set("Content-Type", "text/calendar")
			io.WriteString(w, cuerpo) //nolint:errcheck
		case "/grande.ics":
			io.WriteString(w, "BEGIN:VCALENDAR\r\n"+strings.Repeat("X", MaxICS+10)) //nolint:errcheck
		case "/pagina.ics":
			io.WriteString(w, "<html>no</html>") //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	u, id := usuarioConBicho(t, a)
	otro := otroBichoDe(t, a, u)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID, otro: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	hub := eventos.Nuevo()
	sub := hub.Suscribir(u.ID, "")
	defer hub.Desuscribir(sub)
	i := Nuevo(Opciones{Desarrollo: true, V19: OpcionesV19{Secreto: []byte("secreto de prueba")}}, a, robots, hub, sinLog)
	i.v19.ahora = func() time.Time { return fijo }
	t.Cleanup(func() { i.BorrarAgenda(context.Background(), u.ID) }) //nolint:errcheck

	// Lo que no es un calendario, lo que pesa mucho y lo que no está: 400 con el motivo.
	for _, c := range []string{"/pagina.ics", "/grande.ics", "/no-esta.ics"} {
		var inv ErrAgendaInvalida
		if _, err := i.ConectarAgenda(ctx, u.ID, srv.URL+c, nil); !errors.As(err, &inv) {
			t.Fatalf("%s: %v", c, err)
		}
	}
	if v, _ := i.VerAgenda(ctx, u.ID); v.Conectada || v.Aviso != AvisoPorDefecto {
		t.Fatalf("sin agenda: %+v", v)
	}
	dir := srv.URL + "/private-0123456789abcdef/basic.ics"
	diez := 10
	v, err := i.ConectarAgenda(ctx, u.ID, dir, &diez)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Conectada || strings.Contains(v.ICS, "0123456789abcdef") || len(v.Proximos) != 3 || v.Proximos[0].Titulo != "Llamada con Ana" || v.Aviso != 10 {
		t.Fatalf("vista: %+v", v)
	}
	// En la base va cifrada.
	ag, err := a.AgendaDe(ctx, u.ID)
	if err != nil || bytes.Contains(ag.ICSCifrado, []byte("private")) || bytes.Contains(ag.ICSCifrado, []byte("127.0.0.1")) {
		t.Fatalf("guardada: %v %q", err, ag.ICSCifrado)
	}
	if _, err := i.CambiarAvisoAgenda(ctx, u.ID, 16); err == nil {
		t.Fatal("aviso de 16")
	}

	// A los 8 minutos, con aviso de 10: el 0x48 al robot conectado (y no al otro), una sola vez.
	m := fijo.In(nube.Zona()).Truncate(time.Minute)
	i.revisarAgendas(ctx, m)
	i.revisarAgendas(ctx, m.Add(time.Minute))
	marco, _ := protocolo.Agenda(8, 30, "Llamada con Ana")
	quiero := fmt.Sprintf("%s 2 %x", id, marco)
	if e := enviadosDe(robots); strings.Count(e, quiero) != 1 || strings.Contains(e, otro) {
		t.Fatalf("aviso: %s (quiero %s)", e, quiero)
	}
	// Un reinicio (otras integraciones, la misma base) no lo repite.
	i2 := Nuevo(Opciones{Desarrollo: true, V19: OpcionesV19{Secreto: []byte("secreto de prueba")}}, a, robots, hub, sinLog)
	i2.v19.ahora = i.v19.ahora
	i2.revisarAgendas(ctx, m.Add(2*time.Minute))
	if strings.Count(enviadosDe(robots), quiero) != 1 {
		t.Fatal("se repitió después de reiniciar")
	}
	// Con otro SECRETO no se puede leer: queda el error y los de antes.
	i3 := Nuevo(Opciones{Desarrollo: true, V19: OpcionesV19{Secreto: []byte("otro secreto")}}, a, robots, hub, sinLog)
	i3.v19.ahora = func() time.Time { return fijo.Add(10 * time.Minute) }
	ag, _ = a.AgendaDe(ctx, u.ID)
	if _, ok := i3.releerAgenda(ctx, ag); ok {
		t.Fatal("con otro secreto no debería poder leerla")
	}
	if v, _ := i.VerAgenda(ctx, u.ID); v.Error == "" || len(v.Proximos) != 3 {
		t.Fatalf("con error: %+v", v)
	}

	// El resumen de mañana a las 22:00 (sin horario en la config): una vez.
	vaciar(robots)
	noche := time.Date(m.Year(), m.Month(), m.Day(), 22, 0, 0, 0, nube.Zona())
	ag, _ = a.AgendaDe(ctx, u.ID)
	i.resumenManana(ctx, ag, noche.Add(-time.Minute))
	if e := enviadosDe(robots); e != "" {
		t.Fatalf("antes de hora: %s", e)
	}
	i.resumenManana(ctx, ag, noche)
	i.resumenManana(ctx, ag, noche.Add(time.Minute))
	res, _ := protocolo.ResumenManana(1, "Dentista")
	if e := enviadosDe(robots); e != fmt.Sprintf("%s 2 %x", id, res) {
		t.Fatalf("resumen: %s", e)
	}
	// /agenda por Telegram.
	tg := i.TextoAgendaTelegram(ctx, u.ID)
	if !strings.Contains(tg, "Llamada con Ana") || !strings.Contains(tg, "Mañana:\n• "+manana.In(nube.Zona()).Format("15:04")+" Dentista") {
		t.Fatalf("/agenda: %q", tg)
	}
	// Los eventos SSE.
	vio := false
	for len(sub.C) > 0 {
		if e := <-sub.C; e.Tipo == "agenda" && e.Datos.(map[string]any)["titulo"] == "Llamada con Ana" {
			vio = true
		}
	}
	if !vio {
		t.Fatal("no salió el evento agenda")
	}
	if err := i.BorrarAgenda(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if v, _ := i.VerAgenda(ctx, u.ID); v.Conectada {
		t.Fatal("sigue conectada")
	}
}

// spotifyFalso imita las cuentas y la API de Spotify.
type spotifyFalso struct {
	mu       sync.Mutex
	tema     string // "" = 204
	sonando  bool
	refresh  int
	codigos  int
	basicMal int
}

func (f *spotifyFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/api/token":
		id, sec, ok := r.BasicAuth()
		if !ok || id != "cliente" || sec != "secreto-spotify" {
			f.basicMal++
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		r.ParseForm() //nolint:errcheck
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "codigo-bueno" || !strings.HasSuffix(r.Form.Get("redirect_uri"), "/v1/musica/spotify/vuelta") {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error":"invalid_grant"}`) //nolint:errcheck
				return
			}
			f.codigos++
			io.WriteString(w, `{"access_token":"acceso-1","refresh_token":"refresco-secreto-1","expires_in":3600}`) //nolint:errcheck
		case "refresh_token":
			if r.Form.Get("refresh_token") != "refresco-secreto-1" {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error":"invalid_grant"}`) //nolint:errcheck
				return
			}
			f.refresh++
			io.WriteString(w, `{"access_token":"acceso-2","expires_in":3600}`) //nolint:errcheck
		}
	case "/v1/me":
		io.WriteString(w, `{"id":"ana123","display_name":"Ana"}`) //nolint:errcheck
	case "/v1/me/player/currently-playing":
		if a := r.Header.Get("Authorization"); a != "Bearer acceso-1" && a != "Bearer acceso-2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.tema == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"is_playing": f.sonando, "item": map[string]any{ //nolint:errcheck
			"id": f.tema, "name": "Tema " + f.tema, "artists": []map[string]any{{"name": "Soda"}, {"name": "Cerati"}}}})
	case "/v1/audio-features/t1":
		io.WriteString(w, `{"tempo":119.6}`) //nolint:errcheck
	default:
		w.WriteHeader(http.StatusForbidden) // la API de audio, para las apps nuevas
	}
}

func TestSpotifyFalso(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	falso := &spotifyFalso{}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{}}

	apagada := Nuevo(Opciones{}, a, robots, eventos.Nuevo(), sinLog)
	if e, _ := apagada.EstadoMusica(ctx, u.ID); e["disponible"] != false {
		t.Fatalf("sin claves: %v", e)
	}
	if _, err := apagada.EnlaceSpotify(u.ID); !errors.Is(err, ErrMusicaApagada) {
		t.Fatal(err)
	}

	op := OpcionesV19{Secreto: []byte("secreto de prueba"), APIURL: "https://api.sintetica.test", SpotifyID: "cliente", SpotifySecreto: "secreto-spotify",
		SpotifyCuentasURL: srv.URL, SpotifyAPIURL: srv.URL}
	i := Nuevo(Opciones{V19: op}, a, robots, eventos.Nuevo(), sinLog)
	t.Cleanup(func() { i.DesconectarSpotify(context.Background(), u.ID) }) //nolint:errcheck
	enlace, err := i.EnlaceSpotify(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	pu, _ := url.Parse(enlace)
	q := pu.Query()
	if q.Get("client_id") != "cliente" || q.Get("redirect_uri") != "https://api.sintetica.test/v1/musica/spotify/vuelta" || !strings.Contains(q.Get("scope"), "user-read-currently-playing") {
		t.Fatalf("enlace: %s", enlace)
	}
	estado := q.Get("state")
	// Un state tocado, uno vencido o un código malo no conectan.
	if _, err := i.VueltaSpotify(ctx, estado+"x", "codigo-bueno"); err == nil {
		t.Fatal("state tocado")
	}
	otroU, _ := usuarioConBicho(t, a)
	enlaceOtro, _ := i.EnlaceSpotify(otroU.ID)
	po, _ := url.Parse(enlaceOtro)
	cuerpo, _, _ := strings.Cut(po.Query().Get("state"), ".")
	_, firma, _ := strings.Cut(estado, ".")
	if _, err := i.VueltaSpotify(ctx, cuerpo+"."+firma, "codigo-bueno"); err == nil {
		t.Fatal("el state de otro con mi firma")
	}
	if _, err := i.VueltaSpotify(ctx, estado, "codigo-malo"); err == nil {
		t.Fatal("código malo")
	}
	i.v19.ahora = func() time.Time { return time.Now().Add(11 * time.Minute) }
	if _, err := i.VueltaSpotify(ctx, estado, "codigo-bueno"); err == nil {
		t.Fatal("state vencido")
	}
	i.v19.ahora = time.Now
	if got, err := i.VueltaSpotify(ctx, estado, "codigo-bueno"); err != nil || got != u.ID {
		t.Fatalf("vuelta: %q %v", got, err)
	}
	c, err := a.SpotifyDe(ctx, u.ID)
	if err != nil || c.Nombre != "Ana" || bytes.Contains(c.RefreshCifrado, []byte("refresco")) {
		t.Fatalf("guardada: %+v %v", c, err)
	}

	// Sin robots conectados no se pregunta nada.
	i.revisarMusica(ctx)
	if enviadosDe(robots) != "" {
		t.Fatal("sin robots conectados")
	}
	robots.mu.Lock()
	robots.estados[id] = protocolo.Estado{Nombre: "Pipo"}
	robots.mu.Unlock()
	i.revisarMusica(ctx) // no suena nada: nada
	falso.mu.Lock()
	falso.tema, falso.sonando = "t1", true
	falso.mu.Unlock()
	i.revisarMusica(ctx)
	i.revisarMusica(ctx) // el mismo tema: no se repite
	uno := fmt.Sprintf("%s 2 %x", id, protocolo.Musica(120, true, "Tema t1", "Soda, Cerati"))
	if e := enviadosDe(robots); e != uno {
		t.Fatalf("tema 1: %s (quiero %s)", e, uno)
	}
	// Otro tema (sin bpm: la API de audio no deja) y la pausa. El access
	// token vence y se renueva con el refresh token.
	i.v19.musicaMu.Lock()
	delete(i.v19.tokens, u.ID)
	i.v19.musicaMu.Unlock()
	falso.mu.Lock()
	falso.tema = "t2"
	falso.mu.Unlock()
	i.revisarMusica(ctx)
	falso.mu.Lock()
	falso.sonando = false
	falso.mu.Unlock()
	i.revisarMusica(ctx)
	e := enviadosDe(robots)
	if !strings.Contains(e, fmt.Sprintf("%x", protocolo.Musica(0, true, "Tema t2", "Soda, Cerati"))) ||
		!strings.Contains(e, fmt.Sprintf("%x", protocolo.Musica(0, false, "Tema t2", "Soda, Cerati"))) || falso.refresh != 1 {
		t.Fatalf("tema 2 y pausa: %s (refresh %d)", e, falso.refresh)
	}
	// Al conectarse un robot, se le vuelve a mandar lo que suena.
	falso.mu.Lock()
	falso.sonando = true
	falso.mu.Unlock()
	i.revisarMusica(ctx)
	vaciar(robots)
	i.musicaYa(id)
	i.revisarMusica(ctx)
	if !strings.Contains(enviadosDe(robots), fmt.Sprintf("%x", protocolo.Musica(0, true, "Tema t2", "Soda, Cerati"))) {
		t.Fatal("al reconectarse")
	}
	est, err := i.EstadoMusica(ctx, u.ID)
	if err != nil || est["conectada"] != true || est["nombre"] != "Ana" || est["suena"] == nil {
		t.Fatalf("estado: %v %v", est, err)
	}
	if falso.basicMal > 0 {
		t.Fatal("mandó mal el client secret")
	}
	if err := i.DesconectarSpotify(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if e, _ := i.EstadoMusica(ctx, u.ID); e["conectada"] != false {
		t.Fatalf("desconectada: %v", e)
	}
}

// anthropicFalso imita POST /v1/messages.
type anthropicFalso struct {
	mu        sync.Mutex
	pedidos   []map[string]any
	respuesta string // el JSON que "escribe" el modelo
	clave     string
}

func (f *anthropicFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != f.clave {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"no"}}`) //nolint:errcheck
		return
	}
	var p map[string]any
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	f.pedidos = append(f.pedidos, p)
	b, _ := json.Marshal(map[string]any{"id": "msg_1", "type": "message", "role": "assistant", "model": p["model"],
		"content": []map[string]any{{"type": "text", "text": f.respuesta}}, "stop_reason": "end_turn",
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 5}})
	w.Header().Set("Content-Type", "application/json")
	w.Write(b) //nolint:errcheck
}

func (f *anthropicFalso) cantidad() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.pedidos) }

func TestCharlaConAnthropicFalso(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	falso := &anthropicFalso{clave: "sk-prueba", respuesta: `{"texto":"¡Hola Ana! Hoy está lindo para quedarse adentro mirando la lluvia caer","expresion":"inventada"}`}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	ajeno, idAjeno := usuarioConBicho(t, a)
	cfg := `{"version":2,"nombre":"Pipo","personalidad":{"preset":"grunon"},"dueno":{"nombre":"Ana"}}`
	if _, err := a.NuevaConfig(ctx, id, cfg, 0, "web", &u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.NuevaConfig(ctx, idAjeno, `{"version":2,"nombre":"Secreto","dueno":{"nombre":"Zoe"}}`, 0, "web", &ajeno.ID); err != nil {
		t.Fatal(err)
	}
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID, idAjeno: ajeno.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}

	i := Nuevo(Opciones{V19: OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: srv.URL}}, a, robots, eventos.Nuevo(), sinLog)
	i.cm.espera = 20 * time.Millisecond

	r, err := i.Charlar(ctx, u.ID, id, "¿qué opinás de la vida?", "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Texto) > protocolo.MaxDecir || !utf8.ValidString(r.Texto) || !strings.HasPrefix(r.Texto, "¡Hola Ana!") || r.Expresion != "" || !r.Enviado || r.Quedan != CharlasPorDia-1 {
		t.Fatalf("respuesta: %+v", r)
	}
	m, _ := protocolo.Decir(r.Texto, "")
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, m))
	falso.mu.Lock()
	p := falso.pedidos[0]
	falso.mu.Unlock()
	sys, _ := json.Marshal(p["system"])
	if p["model"] != ModeloCharla || !strings.Contains(string(sys), "Sos Pipo") || !strings.Contains(string(sys), "Ana") ||
		!strings.Contains(string(sys), "gruñón") || strings.Contains(string(sys), "Zoe") || strings.Contains(string(sys), "Secreto") {
		t.Fatalf("pedido: %v %s", p["model"], sys)
	}
	oc, _ := json.Marshal(p["output_config"])
	if !strings.Contains(string(oc), `"json_schema"`) || !strings.Contains(string(oc), `"contento"`) {
		t.Fatalf("output_config: %s", oc)
	}
	msgs, _ := json.Marshal(p["messages"])
	if !strings.Contains(string(msgs), "¿qué opinás de la vida?") {
		t.Fatalf("mensajes: %s", msgs)
	}

	// Con una expresión que existe, la pone.
	falso.mu.Lock()
	falso.respuesta = `{"texto":"Bah, más o menos.","expresion":"enojado"}`
	falso.mu.Unlock()
	if r, err := i.Charlar(ctx, u.ID, id, "contame de los agujeros negros", "app"); err != nil || r.Texto != "Bah, más o menos." || r.Expresion != "enojado" {
		t.Fatalf("con expresión: %+v %v", r, err)
	}
	// El robot de otro: ni se le pregunta a la IA.
	antes := falso.cantidad()
	if _, err := i.Charlar(ctx, u.ID, idAjeno, "decime de Zoe", "web"); err == nil || falso.cantidad() != antes {
		t.Fatalf("ajeno: %v", err)
	}
	if _, err := i.Charlar(ctx, u.ID, id, strings.Repeat("a", MaxTextoCharla+1), "web"); !errors.Is(err, ErrTextoCharla) {
		t.Fatalf("largo: %v", err)
	}

	// Por MQTT (el robot-celu): lo que dice va por su cmd.
	vaciar(robots)
	i.CharlaMQTT(id, []byte(`{"texto":"¿qué es la fotosíntesis?"}`))
	m, _ = protocolo.Decir("Bah, más o menos.", "enojado")
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, m))
	antes = falso.cantidad()
	i.CharlaMQTT(id, []byte(`{"texto":"`+strings.Repeat("a", MaxTextoCharlaMQTT+1)+`"}`)) // muy largo
	i.CharlaMQTT(id, []byte(`no es json`))
	i.CharlaMQTT("FFFFFFFFFFFF", []byte(`{"texto":"sin dueño"}`))
	time.Sleep(200 * time.Millisecond)
	if falso.cantidad() != antes {
		t.Fatal("tenía que ignorarlos")
	}

	// El tope del día (compartido entre HTTP, Telegram y MQTT).
	for k := i.CharlasQuedan(ctx, u.ID); k > 0; k-- {
		a.UsarCharla(ctx, u.ID, time.Now().In(nube.Zona()), CharlasPorDia) //nolint:errcheck
	}
	if _, err := i.Charlar(ctx, u.ID, id, "¿qué opinás de la vida?", "web"); !errors.Is(err, ErrTopeCharla) {
		t.Fatalf("tope: %v", err)
	}
	// Lo que entiende el motor no gasta el tope: sigue andando.
	if r, err := i.Charlar(ctx, u.ID, id, "hola", "web"); err != nil || r.Origen != OrigenMotor {
		t.Fatalf("motor con el tope lleno: %+v %v", r, err)
	}
	vaciar(robots)
	i.limite = nuevoLimitador()
	i.CharlaMQTT(id, []byte(`{"texto":"¿cuál es la capital de Mongolia?"}`))
	m, _ = protocolo.Decir("Por hoy ya charlamos mucho. ¡Mañana más!", "dormido")
	esperarEnviado(t, robots, fmt.Sprintf("%x", m))
}

func TestCharlaPorTelegram(t *testing.T) {
	a := abrirBase(t)
	ia := &anthropicFalso{clave: "sk-prueba", respuesta: `{"texto":"¡Qué hacés!","expresion":"contento"}`}
	sia := httptest.NewServer(ia)
	defer sia.Close()
	token := "123456:TOKEN-CHARLA-" + azar(4)
	tg := &telegramFalso{token: token}
	stg := httptest.NewServer(tg)
	defer stg.Close()
	u, id := usuarioConBicho(t, a)
	otro := otroBichoDe(t, a, u)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID, otro: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}, otro: {Nombre: "Lola"}}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: stg.URL, TelegramEspera: 1,
		V19: OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: sia.URL}}, a, robots, eventos.Nuevo(), sinLog)
	i.cm.espera = 20 * time.Millisecond
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	enlace, _, _ := i.CrearVinculo(ctx, u.ID)
	chat := chatAzar()
	if _, err := i.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i.Desvincular(context.Background(), u.ID) }) //nolint:errcheck
	base := time.Now().UnixNano() % 1_000_000_000
	tg.mu.Lock()
	tg.updates = []map[string]any{
		mensajeDe(base+1, chat, "¿qué opinás de la vida?"), // dos robots y nunca charló: pregunta con cuál
		botonDe(base+2, chat, "charla:"+otro),
		mensajeDe(base+3, chat, "¿y ahora?"), // el último: Lola
	}
	tg.mu.Unlock()
	go i.escucharTelegram(ctx)
	m := tg.esperar(t, 3)
	texto := func(k int) string { s, _ := m[k]["text"].(string); return s }
	if !strings.Contains(texto(0), "¿Con cuál charlás?") || m[0]["reply_markup"] == nil {
		t.Fatalf("pregunta: %q", texto(0))
	}
	if texto(1) != "Lola: ¡Qué hacés!\n\n(Lola no está conectado: no lo dijo en su pantalla.)" && texto(1) != "Lola: ¡Qué hacés!" {
		t.Fatalf("respuesta: %q", texto(1))
	}
	if !strings.HasPrefix(texto(2), "Lola: ") || ia.cantidad() != 2 {
		t.Fatalf("la segunda: %q (%d)", texto(2), ia.cantidad())
	}
	ia.mu.Lock()
	ultimo, _ := json.Marshal(ia.pedidos[1]["messages"])
	ia.mu.Unlock()
	if !strings.Contains(string(ultimo), "¿y ahora?") {
		t.Fatalf("el texto: %s", ultimo)
	}
	esperarEnviado(t, robots, otro+" 2 ")
}

func TestCharlaConServicioCompatible(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	var mu sync.Mutex
	var pedido map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer gsk-prueba" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		json.NewDecoder(r.Body).Decode(&pedido) //nolint:errcheck
		mu.Unlock()
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"`+"```json\\n"+`{\"texto\":\"¡Hola Ana! Todo bien por acá\",\"expresion\":\"contento\"}`+"\\n```"+`"}}]}`) //nolint:errcheck
	}))
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	if _, err := a.NuevaConfig(ctx, id, `{"version":2,"nombre":"Pipo","dueno":{"nombre":"Ana"}}`, 0, "web", &u.ID); err != nil {
		t.Fatal(err)
	}
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}

	// Sin modelo no se prende (cada servicio tiene los suyos).
	if sin := Nuevo(Opciones{V19: OpcionesV19{CharlaURL: srv.URL + "/openai/v1"}}, a, robots, eventos.Nuevo(), sinLog); sin.CharlaActiva() {
		t.Fatal("prendida sin modelo")
	}
	i := Nuevo(Opciones{V19: OpcionesV19{CharlaURL: srv.URL + "/openai/v1/", CharlaClave: "gsk-prueba", AnthropicModelo: "llama-prueba",
		AnthropicClave: "sk-que-no-se-usa"}}, a, robots, eventos.Nuevo(), sinLog)
	r, err := i.Charlar(ctx, u.ID, id, "¿qué opinás de la vida?", "web")
	if err != nil {
		t.Fatal(err)
	}
	if r.Texto != "¡Hola Ana! Todo bien por acá" || r.Expresion != "contento" || r.Quedan != CharlasPorDia-1 {
		t.Fatalf("respuesta: %+v", r)
	}
	mu.Lock()
	defer mu.Unlock()
	msgs, _ := json.Marshal(pedido["messages"])
	if pedido["model"] != "llama-prueba" || !strings.Contains(string(msgs), "Sos Pipo") || !strings.Contains(string(msgs), `\"expresion\"`) {
		t.Fatalf("pedido: %v %s", pedido["model"], msgs)
	}
}
