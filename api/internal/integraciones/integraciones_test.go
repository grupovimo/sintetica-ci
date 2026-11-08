package integraciones

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// --- Sin base ---

func TestValidar(t *testing.T) {
	w := NuevoClienteWebhook(false, []string{"https://api.sinteticalabs.com"})
	mio := func(id string) bool { return id == "AABBCCDDEEFF" }
	buena := []Automatizacion{{ID: "a1", Activa: true, Cuando: "bateria-baja", Hacer: []Accion{
		{Tipo: "telegram", Texto: "{robot} tiene {bateria}% de batería"},
		{Tipo: "webhook", URL: "https://ejemplo.com/gancho"},
		{Tipo: "robot", Bicho: "AABBCCDDEEFF", Expresion: "sorpresa"},
	}}, {ID: "a2", Cuando: "expresion", Valor: "contento", Hacer: []Accion{{Tipo: "telegram", Texto: "contento"}}},
		{ID: "a3", Cuando: "tacto", Hacer: []Accion{{Tipo: "telegram", Texto: "me tocaron"}}}}
	if err := Validar(buena, mio, w); err != nil {
		t.Fatalf("la buena: %v", err)
	}
	una := func(a Automatizacion) []Automatizacion { return []Automatizacion{a} }
	tg := []Accion{{Tipo: "telegram", Texto: "hola"}}
	muchas := make([]Automatizacion, 21)
	for k := range muchas {
		muchas[k] = Automatizacion{ID: fmt.Sprint("a", k), Cuando: "conectado", Hacer: tg}
	}
	malas := map[string][]Automatizacion{
		"más de 20":         muchas,
		"sin id":            una(Automatizacion{Cuando: "conectado", Hacer: tg}),
		"id repetido":       {{ID: "x", Cuando: "conectado", Hacer: tg}, {ID: "x", Cuando: "duerme", Hacer: tg}},
		"cuando raro":       una(Automatizacion{ID: "a", Cuando: "explota", Hacer: tg}),
		"valor sin expr":    una(Automatizacion{ID: "a", Cuando: "conectado", Valor: "x", Hacer: tg}),
		"sin acciones":      una(Automatizacion{ID: "a", Cuando: "conectado"}),
		"tipo raro":         una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "mail"}}}),
		"telegram vacío":    una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "telegram"}}}),
		"webhook ftp":       una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "webhook", URL: "ftp://ejemplo.com"}}}),
		"webhook loopback":  una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "webhook", URL: "http://127.0.0.1:8080/x"}}}),
		"webhook docker":    una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "webhook", URL: "http://api:8081/emqx/autenticar"}}}),
		"webhook propio":    una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "webhook", URL: "https://api.sinteticalabs.com/v1"}}}),
		"webhook con clave": una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "webhook", URL: "https://a:b@ejemplo.com"}}}),
		"robot ajeno":       una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "robot", Bicho: "112233445566", Expresion: "x"}}}),
		"robot sin cara":    una(Automatizacion{ID: "a", Cuando: "conectado", Hacer: []Accion{{Tipo: "robot", Bicho: "AABBCCDDEEFF"}}}),
	}
	for nombre, l := range malas {
		var inv ErrInvalida
		if err := Validar(l, mio, w); !errors.As(err, &inv) {
			t.Errorf("%s: esperaba inválida, dio %v", nombre, err)
		}
	}
}

func TestFirma(t *testing.T) {
	cuerpo := []byte(`{"evento":"tacto"}`)
	f := Firmar("secreto", cuerpo)
	if !strings.HasPrefix(f, "sha256=") || len(f) != 7+64 {
		t.Fatalf("firma: %s", f)
	}
	if !VerificarFirma("secreto", cuerpo, f) || VerificarFirma("otro", cuerpo, f) || VerificarFirma("secreto", []byte("{}"), f) {
		t.Fatal("verificar")
	}
	// Un webhook de verdad (a un servidor local: en desarrollo se permite).
	var recibido []byte
	var firma, evento string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido, _ = io.ReadAll(r.Body)
		firma, evento = r.Header.Get("X-Sintetica-Firma"), r.Header.Get("X-Sintetica-Evento")
	}))
	defer srv.Close()
	c := NuevoClienteWebhook(true, nil)
	err := c.Mandar(context.Background(), srv.URL+"/gancho", "whsec_x", CuerpoWebhook{
		Evento: "tacto", Robot: RobotWebhook{ID: "AABBCCDDEEFF", Nombre: "Pipo"}, Datos: map[string]any{"bateria": 80}, En: "2026-11-08T12:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if evento != "tacto" || !VerificarFirma("whsec_x", recibido, firma) {
		t.Fatalf("firma o evento: %q %q", firma, evento)
	}
	var cuerpoRecibido map[string]any
	json.Unmarshal(recibido, &cuerpoRecibido) //nolint:errcheck
	if cuerpoRecibido["robot"].(map[string]any)["nombre"] != "Pipo" || cuerpoRecibido["en"] == nil {
		t.Fatalf("cuerpo: %s", recibido)
	}
	mac := Firmar("clave", []byte("hola"))
	if mac != "sha256="+hex.EncodeToString(hmacSha256([]byte("clave"), []byte("hola"))) {
		t.Fatal("no es HMAC-SHA256")
	}
}

func TestBloqueoDeIPsInternas(t *testing.T) {
	bloqueadas := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.10", "169.254.169.254", "0.0.0.0",
		"100.64.0.1", "::1", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "224.0.0.1", "64:ff9b::7f00:1", "::"}
	for _, s := range bloqueadas {
		if !IPBloqueada(netip.MustParseAddr(s)) {
			t.Errorf("%s debería estar bloqueada", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111", "93.184.216.34"} {
		if IPBloqueada(netip.MustParseAddr(s)) {
			t.Errorf("%s es pública", s)
		}
	}
	// DNS rebinding: el nombre pasa la validación, pero resuelve adentro al mandar.
	llegó := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { llegó = true }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	c := NuevoClienteWebhook(false, nil)
	c.resolver = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	destino := "http://gancho.ejemplo.com:" + u.Port() + "/x"
	if err := c.ValidarURL(destino); err != nil {
		t.Fatalf("el nombre es público: %v", err)
	}
	if err := c.Mandar(context.Background(), destino, "s", CuerpoWebhook{Evento: "tacto"}); !errors.Is(err, ErrDestinoInterno) {
		t.Fatalf("esperaba destino interno, dio %v", err)
	}
	// Si una de las IPs es interna, no se usa ninguna.
	c.resolver = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	if _, err := c.resolverSeguro(context.Background(), "mixto.ejemplo.com"); !errors.Is(err, ErrDestinoInterno) {
		t.Fatalf("mixto: %v", err)
	}
	// Un nombre ajeno que apunta a la IP del propio servicio.
	c = NuevoClienteWebhook(false, []string{"https://api.sinteticalabs.com"})
	c.resolver = func(_ context.Context, h string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("203.0.114.7")}, nil
	}
	if _, err := c.resolverSeguro(context.Background(), "otro.ejemplo.com"); !errors.Is(err, ErrDestinoInterno) {
		t.Fatalf("propio: %v", err)
	}
	if llegó {
		t.Fatal("el pedido no tenía que llegar")
	}
	// No sigue redirecciones.
	destinoFinal := false
	final := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { destinoFinal = true }))
	defer final.Close()
	redir := httptest.NewServer(http.RedirectHandler(final.URL, http.StatusFound))
	defer redir.Close()
	if err := NuevoClienteWebhook(true, nil).Mandar(context.Background(), redir.URL, "s", CuerpoWebhook{}); err == nil || destinoFinal {
		t.Fatalf("redirección: %v %v", err, destinoFinal)
	}
}

func TestLimiteDeDisparos(t *testing.T) {
	l := nuevoLimitador()
	ahora := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	l.ahora = func() time.Time { return ahora }
	if !l.permitir("x", DisparoCada, DisparosPorDia, 24*time.Hour) {
		t.Fatal("el primero pasa")
	}
	ahora = ahora.Add(5 * time.Second)
	if l.permitir("x", DisparoCada, DisparosPorDia, 24*time.Hour) {
		t.Fatal("a los 5 s no")
	}
	if !l.permitir("y", DisparoCada, DisparosPorDia, 24*time.Hour) {
		t.Fatal("otra automatización sí")
	}
	n := 1
	for k := 0; k < 300; k++ {
		ahora = ahora.Add(11 * time.Second)
		if l.permitir("x", DisparoCada, DisparosPorDia, 24*time.Hour) {
			n++
		}
	}
	if n != DisparosPorDia {
		t.Fatalf("en un día pasaron %d", n)
	}
	ahora = ahora.Add(24 * time.Hour)
	if !l.permitir("x", DisparoCada, DisparosPorDia, 24*time.Hour) {
		t.Fatal("al otro día sí")
	}
}

func TestRecordatorios(t *testing.T) {
	cfg := `{"version":2,"recordatorios":[{"hora":"08:30","dias":"LMXJV","texto":"la pastilla","activo":true},
		{"hora":"09:00","texto":"todos los días"},{"hora":"10:00","texto":"apagado","activo":false}]}`
	l := recordatoriosDe(cfg)
	if len(l) != 3 {
		t.Fatalf("%d recordatorios", len(l))
	}
	lunes := time.Date(2026, 11, 9, 8, 30, 0, 0, time.UTC)
	sabado := time.Date(2026, 11, 14, 8, 30, 0, 0, time.UTC)
	if !l[0].Toca(lunes) || l[0].Toca(sabado) || l[0].Toca(lunes.Add(time.Minute)) {
		t.Fatal("08:30 de lunes a viernes")
	}
	if !l[1].Toca(sabado.Add(30*time.Minute)) || l[2].Toca(lunes.Add(90*time.Minute)) {
		t.Fatal("sin días = todos; apagado = nunca")
	}
	if recordatoriosDe(`{"version":2}`) != nil || recordatoriosDe("no es json") != nil {
		t.Fatal("sin recordatorios")
	}
}

func hmacSha256(clave, msg []byte) []byte {
	// HMAC a mano (RFC 2104), para no comparar Firmar consigo misma.
	const bloque = 64
	if len(clave) > bloque {
		h := sha256.Sum256(clave)
		clave = h[:]
	}
	k := make([]byte, bloque)
	copy(k, clave)
	ipad, opad := make([]byte, bloque), make([]byte, bloque)
	for i := range k {
		ipad[i], opad[i] = k[i]^0x36, k[i]^0x5c
	}
	in := sha256.Sum256(append(ipad, msg...))
	out := sha256.Sum256(append(opad, in[:]...))
	return out[:]
}

// --- Con la base (SINTETICA_PRUEBAS_DB) ---

type robotsFalsos struct {
	mu       sync.Mutex
	duenos   map[string]string
	estados  map[string]protocolo.Estado
	enviados []string
}

func (r *robotsFalsos) Dueno(id string) string { r.mu.Lock(); defer r.mu.Unlock(); return r.duenos[id] }
func (r *robotsFalsos) EstadoDe(id string) (protocolo.Estado, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.estados[id]
	return e, ok
}
func (r *robotsFalsos) Conectado(id string) bool { _, ok := r.EstadoDe(id); return ok }
func (r *robotsFalsos) Enviar(id string, car byte, m []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enviados = append(r.enviados, fmt.Sprintf("%s %d %x", id, car, m))
	return nil
}

func abrirBase(t *testing.T) *almacen.Almacen {
	t.Helper()
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	a, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Cerrar)
	return a
}

func azar(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return strings.ToUpper(hex.EncodeToString(b))
}

func chatAzar() int64 {
	b := make([]byte, 4)
	rand.Read(b) //nolint:errcheck
	return int64(b[0])<<24 | int64(b[1])<<16 | int64(b[2])<<8 | int64(b[3]) + 1
}

// usuarioConBicho crea un usuario con un bicho suyo.
func usuarioConBicho(t *testing.T, a *almacen.Almacen) (almacen.Usuario, string) {
	t.Helper()
	ctx := context.Background()
	u, err := a.UsuarioPorEmail(ctx, strings.ToLower(azar(4))+"@ejemplo.com", false)
	if err != nil {
		t.Fatal(err)
	}
	id := azar(6)
	if err := a.AsegurarBicho(ctx, id, "virtual", true); err != nil {
		t.Fatal(err)
	}
	clave := sha256.Sum256([]byte(id))
	a.CrearCredencialPendiente(ctx, id, u.ID, clave[:], time.Now().Add(time.Hour)) //nolint:errcheck
	if acc, err := a.UsarCredencial(ctx, id, clave[:]); err != nil || !acc.Ok {
		t.Fatalf("reclamar: %v", err)
	}
	return u, id
}

func TestVincularConCodigo(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	hub := eventos.Nuevo()
	i := Nuevo(Opciones{TelegramToken: "x"}, a, &robotsFalsos{}, hub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	u, _ := usuarioConBicho(t, a)
	sub := hub.Suscribir(u.ID, "")
	defer hub.Desuscribir(sub)

	enlace, vence, err := i.CrearVinculo(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enlace, "https://t.me/sinteticalabs_bot?start=") || time.Until(vence) > DuracionCodigo || time.Until(vence) < 14*time.Minute {
		t.Fatalf("enlace %s vence %s", enlace, vence)
	}
	codigo := strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start=")
	chat := chatAzar()
	if got, err := i.Vincular(ctx, codigo, chat, "@ana"); err != nil || got != u.ID {
		t.Fatalf("vincular: %v", err)
	}
	if v, err := a.TelegramDe(ctx, u.ID); err != nil || v.ChatID != chat || v.Nombre != "@ana" {
		t.Fatalf("vínculo: %+v %v", v, err)
	}
	select {
	case e := <-sub.C:
		if e.Tipo != "telegram" {
			t.Fatalf("evento %s", e.Tipo)
		}
	case <-time.After(time.Second):
		t.Fatal("sin evento telegram")
	}
	if _, err := i.Vincular(ctx, codigo, chatAzar(), "@otro"); !errors.Is(err, almacen.ErrCodigoInvalido) {
		t.Fatalf("un solo uso: %v", err)
	}
	// Uno vencido no sirve.
	viejo := "viejo" + azar(8)
	if err := a.CrearCodigoTelegram(ctx, u.ID, hashCodigo(viejo), time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Vincular(ctx, viejo, chatAzar(), "@ana"); !errors.Is(err, almacen.ErrCodigoInvalido) {
		t.Fatalf("vencido: %v", err)
	}
	if _, err := i.Vincular(ctx, "inventado", chatAzar(), "@ana"); !errors.Is(err, almacen.ErrCodigoInvalido) {
		t.Fatalf("inventado: %v", err)
	}
	if err := i.Desvincular(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TelegramDe(ctx, u.ID); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatal("desvinculado")
	}
}

// telegramFalso imita la Bot API: da los updates de a uno y guarda lo que se manda.
type telegramFalso struct {
	mu       sync.Mutex
	updates  []map[string]any
	enviados []map[string]any
	token    string
	malos    int // pedidos con otro token
}

func (f *telegramFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pre := "/bot" + f.token + "/"
	if !strings.HasPrefix(r.URL.Path, pre) {
		f.malos++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`)) //nolint:errcheck
		return
	}
	var p map[string]any
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	switch strings.TrimPrefix(r.URL.Path, pre) {
	case "getUpdates":
		var res []map[string]any
		if len(f.updates) > 0 {
			res, f.updates = f.updates[:1], f.updates[1:]
		} else {
			f.mu.Unlock()
			time.Sleep(50 * time.Millisecond) // el long polling
			f.mu.Lock()
		}
		if res == nil {
			res = []map[string]any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": res}) //nolint:errcheck
	case "sendMessage":
		f.enviados = append(f.enviados, p)
		w.Write([]byte(`{"ok":true,"result":{}}`)) //nolint:errcheck
	default:
		w.Write([]byte(`{"ok":true,"result":true}`)) //nolint:errcheck
	}
}

func (f *telegramFalso) esperar(t *testing.T, n int) []map[string]any {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		f.mu.Lock()
		if len(f.enviados) >= n {
			l := append([]map[string]any(nil), f.enviados...)
			f.mu.Unlock()
			return l
		}
		f.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("esperaba %d mensajes", n)
	return nil
}

func mensajeDe(update int64, chat int64, texto string) map[string]any {
	return map[string]any{"update_id": update, "message": map[string]any{
		"message_id": 1, "from": map[string]any{"id": chat, "username": "ana"},
		"chat": map[string]any{"id": chat, "type": "private"}, "text": texto,
	}}
}

func TestTelegramConServidorFalso(t *testing.T) {
	a := abrirBase(t)
	token := "123456:TOKEN-DE-PRUEBA-" + azar(4)
	falso := &telegramFalso{token: token}
	srv := httptest.NewServer(falso)
	defer srv.Close()

	var logs bytes.Buffer
	var logsMu sync.Mutex
	log := slog.New(slog.NewTextHandler(escritorSeguro{&logs, &logsMu}, nil))
	u, id := usuarioConBicho(t, a)
	bat := 80
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID},
		estados: map[string]protocolo.Estado{id: {Nombre: "Pipo", Bateria: &bat, Expresion: "contento", Humor: 0.6}}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1}, a, robots, eventos.Nuevo(), log)
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	enlace, _, err := i.CrearVinculo(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	codigo := strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start=")
	chat, ajeno := chatAzar(), chatAzar()
	base := time.Now().UnixNano() % 1_000_000_000
	falso.updates = []map[string]any{
		mensajeDe(base+1, ajeno, "/robots"),
		mensajeDe(base+2, chat, "/start "+codigo),
		mensajeDe(base+3, chat, "/robots"),
		mensajeDe(base+4, chat, "/cara sorpresa"),
		{"update_id": base + 5, "callback_query": map[string]any{"id": "c1", "from": map[string]any{"id": chat},
			"data": "cara:" + id + ":triste", "message": map[string]any{"message_id": 2, "chat": map[string]any{"id": chat, "type": "private"}}}},
	}
	listo := make(chan struct{})
	go func() { i.escucharTelegram(ctx); close(listo) }()
	m := falso.esperar(t, 5)
	texto := func(k int) string { s, _ := m[k]["text"].(string); return s }
	if !strings.Contains(texto(0), "vinculá este chat") || int64(m[0]["chat_id"].(float64)) != ajeno {
		t.Fatalf("al que no está vinculado: %q", texto(0))
	}
	if !strings.Contains(texto(1), "quedó vinculado") {
		t.Fatalf("saludo: %q", texto(1))
	}
	if !strings.Contains(texto(2), "Pipo: conectado, 80 % de batería, cara contento") || m[2]["reply_markup"] == nil {
		t.Fatalf("/robots: %q", texto(2))
	}
	if !strings.Contains(texto(3), "cara de sorpresa") || !strings.Contains(texto(4), "cara de triste") {
		t.Fatalf("/cara: %q %q", texto(3), texto(4))
	}
	robots.mu.Lock()
	enviados := strings.Join(robots.enviados, "\n")
	robots.mu.Unlock()
	if !strings.Contains(enviados, fmt.Sprintf("%s 2 %x", id, protocolo.Expresion("sorpresa"))) ||
		!strings.Contains(enviados, fmt.Sprintf("%x", protocolo.Expresion("triste"))) {
		t.Fatalf("al robot: %s", enviados)
	}
	// Una automatización con Telegram le llega al chat vinculado.
	if err := i.GuardarAutomatizaciones(ctx, u.ID, id, []Automatizacion{{ID: "a1", Activa: true, Cuando: "bateria-baja",
		Hacer: []Accion{{Tipo: "telegram", Texto: "{robot} tiene {bateria}% de batería ({evento})"}}}}); err != nil {
		t.Fatal(err)
	}
	res, err := i.Probar(ctx, u.ID, id, "0")
	if err != nil || len(res) != 1 || !res[0].Ok {
		t.Fatalf("probar: %+v %v", res, err)
	}
	m = falso.esperar(t, 6)
	if s, _ := m[5]["text"].(string); s != "Pipo tiene 80% de batería (bateria-baja)" {
		t.Fatalf("automatización: %q", s)
	}
	if _, err := i.Probar(ctx, u.ID, id, "a1"); !errors.Is(err, ErrLimite) {
		t.Fatalf("límite: %v", err)
	}
	_, h, _ := i.Automatizaciones(ctx, id)
	if len(h) != 1 || !h[0].Ok || h[0].Automatizacion != "a1" {
		t.Fatalf("historial: %+v", h)
	}
	cancelar()
	select {
	case <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("no se apagó con el contexto")
	}
	logsMu.Lock()
	defer logsMu.Unlock()
	if strings.Contains(logs.String(), token) || falso.malos > 0 {
		t.Fatal("el token apareció en los logs (o se usó otro)")
	}
}

type escritorSeguro struct {
	b  *bytes.Buffer
	mu *sync.Mutex
}

func (e escritorSeguro) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.b.Write(p)
}

func TestTelegramBackoffSinToken(t *testing.T) {
	// Telegram caído: el trabajador reintenta y se apaga limpio, sin loguear el token.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	a := abrirBase(t)
	var logs bytes.Buffer
	var mu sync.Mutex
	token := "999:SECRETO-" + azar(4)
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1}, a, &robotsFalsos{}, eventos.Nuevo(),
		slog.New(slog.NewTextHandler(escritorSeguro{&logs, &mu}, nil)))
	ctx, cancelar := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancelar()
	inicio := time.Now()
	i.escucharTelegram(ctx)
	if time.Since(inicio) > 3*time.Second {
		t.Fatal("no se apagó con el contexto")
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(logs.String(), "getUpdates falló") || strings.Contains(logs.String(), token) {
		t.Fatalf("logs: %s", logs.String())
	}
	// Y sin token, apagado.
	if Nuevo(Opciones{}, a, &robotsFalsos{}, eventos.Nuevo(), slog.Default()).TelegramActivo() {
		t.Fatal("sin token tiene que estar apagado")
	}
}
