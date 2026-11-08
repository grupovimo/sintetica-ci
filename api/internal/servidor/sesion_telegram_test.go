package servidor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/correo"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

// tgFalso imita la Bot API: guarda lo que el bot manda (nunca el Telegram real).
type tgFalso struct {
	mu       sync.Mutex
	enviados []map[string]any
}

func (f *tgFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p map[string]any
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	if strings.HasSuffix(r.URL.Path, "/sendMessage") {
		f.mu.Lock()
		f.enviados = append(f.enviados, p)
		f.mu.Unlock()
	}
	w.Write([]byte(`{"ok":true,"result":{}}`)) //nolint:errcheck
}

func (f *tgFalso) ultimo(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.enviados) == 0 {
		t.Fatal("el bot no mandó nada")
	}
	return f.enviados[len(f.enviados)-1]
}

func servidorConTelegram(t *testing.T, conCorreo bool) (*Servidor, *almacen.Almacen, *integraciones.Integraciones, *tgFalso) {
	t.Helper()
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	falso := &tgFalso{}
	srv := httptest.NewServer(falso)
	t.Cleanup(srv.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{TelegramToken: "1:prueba", TelegramURL: srv.URL, TelegramEspera: 1}, alm, n, hub, log)
	var c *correo.Correo
	if conCorreo {
		c = &correo.Correo{Servidor: "127.0.0.1:1"}
	}
	return Nuevo(Opciones{}, alm, n, i, hub, c, nil, log), alm, i, falso
}

func chatPrueba() int64 { return time.Now().UnixNano()%1_000_000_000 + 1_000_000_000 }

func startUpdate(chat int64, nombre, texto string) []byte {
	b, _ := json.Marshal(map[string]any{"update_id": 1, "message": map[string]any{
		"message_id": 1, "from": map[string]any{"id": chat, "first_name": nombre},
		"chat": map[string]any{"id": chat, "type": "private"}, "text": texto}})
	return b
}

func botonUpdate(chat int64, datos string) []byte {
	b, _ := json.Marshal(map[string]any{"update_id": 2, "callback_query": map[string]any{
		"id": "c", "from": map[string]any{"id": chat, "first_name": "Ana"}, "data": datos,
		"message": map[string]any{"message_id": 5, "chat": map[string]any{"id": chat, "type": "private"}}}})
	return b
}

// pedirConCookie hace un pedido como el navegador: con las cookies que tenga.
func pedirConCookie(t *testing.T, h http.Handler, cookies []*http.Cookie, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func cookieDe(w *httptest.ResponseRecorder, nombre string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == nombre {
			return c
		}
	}
	return nil
}

func TestMetodosSesion(t *testing.T) {
	s, _, _, _ := servidorConTelegram(t, false)
	w := pedirConCookie(t, s.Rutas(), nil, "GET", "/v1/sesion/metodos", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"email":false`) || !strings.Contains(w.Body.String(), `"telegram":true`) {
		t.Fatalf("métodos: %d %s", w.Code, w.Body)
	}
	// Sin SMTP no se manda el mail.
	w = pedirConCookie(t, s.Rutas(), nil, "POST", "/v1/sesion/enlace", `{"email":"ana@ejemplo.com"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("enlace sin SMTP: %d", w.Code)
	}
	s2, _, _, _ := servidorConTelegram(t, true)
	w = pedirConCookie(t, s2.Rutas(), nil, "GET", "/v1/sesion/metodos", "")
	if !strings.Contains(w.Body.String(), `"email":true`) {
		t.Fatalf("con SMTP: %s", w.Body)
	}
	// Sin Telegram, no se ofrece.
	sinTG, _ := servidorDePrueba(t)
	w = pedirConCookie(t, sinTG.Rutas(), nil, "GET", "/v1/sesion/metodos", "")
	if !strings.Contains(w.Body.String(), `"telegram":false`) {
		t.Fatalf("sin Telegram: %s", w.Body)
	}
	if w := pedirConCookie(t, sinTG.Rutas(), nil, "POST", "/v1/sesion/telegram", `{}`); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("pedir sin Telegram: %d", w.Code)
	}
}

var reCodigo = regexp.MustCompile(`(\d{3}) (\d{3})`)

func TestEntrarConTelegramCuentaNueva(t *testing.T) {
	s, alm, integ, falso := servidorConTelegram(t, false)
	h := s.Rutas()
	ctx := context.Background()

	w := pedirConCookie(t, h, nil, "POST", "/v1/sesion/telegram", `{"cliente":"web"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("pedir: %d %s", w.Code, w.Body)
	}
	var p struct{ Token, Enlace, Secreto string }
	json.Unmarshal(w.Body.Bytes(), &p) //nolint:errcheck
	secreto := cookieDe(w, cookieEntrada)
	if secreto == nil || !secreto.HttpOnly || secreto.MaxAge != 600 || p.Secreto != "" {
		t.Fatalf("la web guarda el secreto en una cookie corta: %+v %q", secreto, p.Secreto)
	}
	if p.Enlace != "https://t.me/sinteticalabs_bot?start=entrar-"+p.Token || len("entrar-"+p.Token) > 64 {
		t.Fatalf("enlace: %s", p.Enlace)
	}
	ruta := "/v1/sesion/telegram/" + p.Token
	// Todavía nadie tocó Start.
	w = pedirConCookie(t, h, []*http.Cookie{secreto}, "GET", ruta, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"estado":"pendiente"`) {
		t.Fatalf("pendiente: %d %s", w.Code, w.Body)
	}
	// Sin el secreto (otro navegador con el link), no.
	if w := pedirConCookie(t, h, nil, "GET", ruta, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("sin secreto: %d", w.Code)
	}
	otro := &http.Cookie{Name: cookieEntrada, Value: "otro"}
	if w := pedirConCookie(t, h, []*http.Cookie{otro}, "GET", ruta, ""); w.Code != http.StatusGone {
		t.Fatalf("otro secreto: %d", w.Code)
	}

	// Start desde un Telegram sin cuenta: pregunta si crearla.
	chat := chatPrueba()
	if err := integ.AtenderTelegram(ctx, startUpdate(chat, "Ana", "/start entrar-"+p.Token)); err != nil {
		t.Fatal(err)
	}
	m := falso.ultimo(t)
	if !strings.Contains(m["text"].(string), "¿Crear tu cuenta de Sintética?") {
		t.Fatalf("pregunta: %v", m["text"])
	}
	teclado := m["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)
	si := teclado[0].(map[string]any)["callback_data"].(string)
	// Otro chat con el mismo link: ya no.
	intruso := chat + 1
	integ.AtenderTelegram(ctx, startUpdate(intruso, "Beto", "/start entrar-"+p.Token)) //nolint:errcheck
	if !strings.Contains(falso.ultimo(t)["text"].(string), "ya se usó o venció") {
		t.Fatalf("otro chat: %v", falso.ultimo(t)["text"])
	}
	// El botón de otro chat no sirve.
	integ.AtenderTelegram(ctx, botonUpdate(intruso, si)) //nolint:errcheck
	if _, err := alm.TelegramPorChat(ctx, intruso); err == nil {
		t.Fatal("el intruso no tiene cuenta")
	}
	w = pedirConCookie(t, h, []*http.Cookie{secreto}, "GET", ruta, "")
	if !strings.Contains(w.Body.String(), `"estado":"confirmar"`) {
		t.Fatalf("confirmar: %s", w.Body)
	}

	// Crear la cuenta: el navegador que esperaba queda adentro.
	listo := make(chan *httptest.ResponseRecorder, 1)
	go func() { listo <- pedirConCookie(t, h, []*http.Cookie{secreto}, "GET", ruta+"?esperar=10", "") }()
	time.Sleep(200 * time.Millisecond)
	if err := integ.AtenderTelegram(ctx, botonUpdate(chat, si)); err != nil {
		t.Fatal(err)
	}
	texto := falso.ultimo(t)["text"].(string)
	if !strings.Contains(texto, "Creé tu cuenta") || !reCodigo.MatchString(texto) {
		t.Fatalf("bienvenida: %q", texto)
	}
	var w2 *httptest.ResponseRecorder
	select {
	case w2 = <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("el navegador no se enteró")
	}
	sesion := cookieDe(w2, cookieSesion)
	if w2.Code != 200 || sesion == nil || sesion.Value == "" {
		t.Fatalf("sesión: %d %s", w2.Code, w2.Body)
	}
	if c := cookieDe(w2, cookieEntrada); c == nil || c.MaxAge >= 0 {
		t.Fatal("borra el secreto")
	}
	w = pedirConCookie(t, h, []*http.Cookie{sesion}, "GET", "/v1/sesion", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"nombre":"Ana"`) || !strings.Contains(w.Body.String(), `"email":""`) {
		t.Fatalf("yo: %d %s", w.Code, w.Body)
	}
	// Un solo uso.
	if w := pedirConCookie(t, h, []*http.Cookie{secreto}, "GET", ruta, ""); w.Code != http.StatusGone {
		t.Fatalf("de nuevo: %d", w.Code)
	}
	if _, err := alm.TelegramPorChat(ctx, chat); err != nil {
		t.Fatal("el chat quedó vinculado a la cuenta nueva")
	}

	// "No fui yo" cierra esa sesión.
	integ.AtenderTelegram(ctx, botonUpdate(chat, strings.Replace(si, "entrar-si", "entrar-no", 1))) //nolint:errcheck
	if !strings.Contains(falso.ultimo(t)["text"].(string), "cerré esa sesión") {
		t.Fatalf("no fui yo: %v", falso.ultimo(t)["text"])
	}
	if w := pedirConCookie(t, h, []*http.Cookie{sesion}, "GET", "/v1/sesion", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("sesión cerrada: %d", w.Code)
	}
}

func TestEntrarConTelegramCuentaConMailYCodigo(t *testing.T) {
	s, alm, integ, falso := servidorConTelegram(t, true)
	h := s.Rutas()
	ctx := context.Background()
	// Una cuenta por mail que ya vinculó su Telegram.
	email := strings.ToLower(azarHex(4)) + "@ejemplo.com"
	u, err := alm.UsuarioPorEmail(ctx, email, false)
	if err != nil {
		t.Fatal(err)
	}
	chat := chatPrueba()
	enlace, _, err := integ.CrearVinculo(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := integ.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}

	// La app: el secreto viene en la respuesta y va en un encabezado.
	w := pedirConCookie(t, h, nil, "POST", "/v1/sesion/telegram", `{"cliente":"app"}`)
	var p struct{ Token, Secreto string }
	json.Unmarshal(w.Body.Bytes(), &p) //nolint:errcheck
	if w.Code != http.StatusCreated || p.Secreto == "" || cookieDe(w, cookieEntrada) != nil {
		t.Fatalf("app: %d %s", w.Code, w.Body)
	}
	integ.AtenderTelegram(ctx, startUpdate(chat, "Ana", "/start entrar-"+p.Token)) //nolint:errcheck
	texto := falso.ultimo(t)["text"].(string)
	if !strings.Contains(texto, "volvé a la app") || strings.Contains(texto, email) {
		t.Fatalf("aprobada: %q", texto)
	}
	codigo := strings.ReplaceAll(reCodigo.FindString(texto), " ", "")
	codigoPost := func(secreto, cod string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/sesion/telegram/"+p.Token+"/codigo", strings.NewReader(fmt.Sprintf(`{"codigo":%q}`, cod)))
		if secreto != "" {
			r.Header.Set("X-Sintetica-Secreto", secreto)
		}
		r.Header.Set("X-Forwarded-For", "10.9."+azarHex(1)+".1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// El código sin el secreto no sirve.
	if w := codigoPost("", codigo); w.Code != http.StatusUnauthorized {
		t.Fatalf("sin secreto: %d", w.Code)
	}
	malo := "000000"
	if codigo == malo {
		malo = "111111"
	}
	if w := codigoPost(p.Secreto, malo); w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "ese código no es") {
		t.Fatalf("código malo: %d %s", w.Code, w.Body)
	}
	w = codigoPost(p.Secreto, codigo[:3]+"-"+codigo[3:])
	var res struct {
		Token   string
		Usuario almacen.Usuario
	}
	json.Unmarshal(w.Body.Bytes(), &res) //nolint:errcheck
	if w.Code != 200 || res.Token == "" || res.Usuario.ID != u.ID || res.Usuario.Email != email {
		t.Fatalf("entra a la cuenta del mail: %d %s", w.Code, w.Body)
	}
	if code, _ := pedir(t, h, res.Token, "GET", "/v1/sesion", ""); code != 200 {
		t.Fatalf("token de app: %d", code)
	}
	// Ya se usó: el poll tampoco.
	r := httptest.NewRequest("GET", "/v1/sesion/telegram/"+p.Token, nil)
	r.Header.Set("X-Sintetica-Secreto", p.Secreto)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusGone {
		t.Fatalf("usado: %d", w.Code)
	}
}

func TestEntrarConTelegramIntentosYTopes(t *testing.T) {
	s, _, integ, falso := servidorConTelegram(t, false)
	h := s.Rutas()
	ctx := context.Background()
	chat := chatPrueba()
	ip := "10.8." + azarHex(1) + ".2"
	pedirEntrada := func() (string, string) {
		r := httptest.NewRequest("POST", "/v1/sesion/telegram", strings.NewReader(`{"cliente":"app"}`))
		r.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var p struct{ Token, Secreto string }
		json.Unmarshal(w.Body.Bytes(), &p) //nolint:errcheck
		if w.Code != http.StatusCreated {
			t.Fatalf("pedir: %d", w.Code)
		}
		return p.Token, p.Secreto
	}
	// 5 intentos con el código y no más.
	token, secreto := pedirEntrada()
	integ.AtenderTelegram(ctx, startUpdate(chat, "Ana", "/start entrar-"+token))            //nolint:errcheck
	integ.AtenderTelegram(ctx, botonUpdate(chat, "entrar-si:"+entradaDelTeclado(t, falso))) //nolint:errcheck
	codigo := strings.ReplaceAll(reCodigo.FindString(falso.ultimo(t)["text"].(string)), " ", "")
	malo := "000000"
	if codigo == malo {
		malo = "111111"
	}
	for k := 0; k < 5; k++ {
		r := httptest.NewRequest("POST", "/v1/sesion/telegram/"+token+"/codigo", strings.NewReader(`{"codigo":"`+malo+`"}`))
		r.Header.Set("X-Sintetica-Secreto", secreto)
		r.Header.Set("X-Forwarded-For", "10.7."+azarHex(1)+".3")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
	}
	r := httptest.NewRequest("POST", "/v1/sesion/telegram/"+token+"/codigo", strings.NewReader(`{"codigo":"`+codigo+`"}`))
	r.Header.Set("X-Sintetica-Secreto", secreto)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "demasiados intentos") {
		t.Fatalf("después de 5: %d %s", w.Code, w.Body)
	}
	// Tope por Telegram: 10 por hora (ya tomó 1).
	for k := 0; k < integraciones.MaxEntradasPorChat-1; k++ {
		tk, _ := pedirEntradaOtraIP(t, h)
		integ.AtenderTelegram(ctx, startUpdate(chat, "Ana", "/start entrar-"+tk)) //nolint:errcheck
	}
	tk, _ := pedirEntradaOtraIP(t, h)
	integ.AtenderTelegram(ctx, startUpdate(chat, "Ana", "/start entrar-"+tk)) //nolint:errcheck
	if !strings.Contains(falso.ultimo(t)["text"].(string), "muchas veces") {
		t.Fatalf("tope por chat: %v", falso.ultimo(t)["text"])
	}
	// Tope por IP: 20 por hora.
	for k := 0; k < 19; k++ {
		pedirEntrada()
	}
	r = httptest.NewRequest("POST", "/v1/sesion/telegram", strings.NewReader(`{}`))
	r.Header.Set("X-Forwarded-For", ip)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("tope por IP: %d", w.Code)
	}
}

func pedirEntradaOtraIP(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	r := httptest.NewRequest("POST", "/v1/sesion/telegram", strings.NewReader(`{"cliente":"app"}`))
	r.Header.Set("X-Forwarded-For", "10.6."+azarHex(1)+"."+azarHex(1))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var p struct{ Token, Secreto string }
	json.Unmarshal(w.Body.Bytes(), &p) //nolint:errcheck
	return p.Token, p.Secreto
}

func entradaDelTeclado(t *testing.T, f *tgFalso) string {
	t.Helper()
	m := f.ultimo(t)
	d := m["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)[0].(map[string]any)["callback_data"].(string)
	return strings.TrimPrefix(d, "entrar-si:")
}
