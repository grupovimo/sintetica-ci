package servidor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

func TestAmigosHTTP(t *testing.T) {
	s, alm := servidorDePrueba(t)
	h := s.Rutas()
	ana, robotsAna := cuentaDePrueba(t, alm)
	beto, robotsBeto := cuentaDePrueba(t, alm)
	carlos, _ := cuentaDePrueba(t, alm)
	alm.ActualizarNombre(context.Background(), usuarioDeToken(t, alm, ana), "Ana") //nolint:errcheck

	cod, inv := pedir(t, h, ana, "POST", "/v1/amigos/invitacion", "")
	codigo, _ := inv["codigo"].(string)
	if cod != http.StatusOK || len(codigo) != 8 || !strings.HasSuffix(inv["link"].(string), "/robots?amigo="+codigo) || inv["telegram"] != nil {
		t.Fatalf("invitación: %d %v", cod, inv)
	}
	for _, c := range []string{`{"codigo":"XXXXXXXX"}`, `{"codigo":"corto"}`, `{}`} {
		if cod, v := pedir(t, h, beto, "POST", "/v1/amigos", c); cod != http.StatusBadRequest {
			t.Fatalf("código malo %s: %d %v", c, cod, v)
		}
	}
	cod, am := pedir(t, h, beto, "POST", "/v1/amigos", `{"codigo":"`+codigo+`"}`)
	if cod != http.StatusOK || am["nombre"] != "Ana" {
		t.Fatalf("aceptar: %d %v", cod, am)
	}
	// Del amigo, solo el nombre: ni el email ni los ids de sus robots.
	cod, l := pedir(t, h, ana, "GET", "/v1/amigos", "")
	crudo, _ := json.Marshal(l)
	if cod != http.StatusOK || strings.Contains(string(crudo), "@ejemplo.com") || strings.Contains(string(crudo), robotsBeto[0]) ||
		!strings.Contains(string(crudo), robotsAna[0]) {
		t.Fatalf("amigos: %d %s", cod, crudo)
	}
	id := jsonNum(am["id"])
	base := "/v1/amigos/" + id
	if cod, v := pedir(t, h, beto, "POST", base+"/toque", `{"tipo":"abrazo"}`); cod != http.StatusOK || v["robots"] != 0.0 || v["telegram"] != false {
		t.Fatalf("toque: %d %v", cod, v)
	}
	for _, c := range []string{`{"tipo":"pellizco"}`, `{"tipo":"carita"}`, `{"tipo":"toque","desde":"` + robotsAna[0] + `"}`, `{"tipo":"toque","x":1}`} {
		if cod, v := pedir(t, h, beto, "POST", base+"/toque", c); cod != http.StatusBadRequest {
			t.Fatalf("toque malo %s: %d %v", c, cod, v)
		}
	}
	for _, x := range []struct{ metodo, ruta, cuerpo string }{
		{"POST", base + "/toque", `{"tipo":"toque"}`}, {"DELETE", base, ""}, {"PUT", base, `{"robots":null}`},
		{"POST", "/v1/amigos/abc/toque", `{}`},
	} {
		if cod, _ := pedir(t, h, carlos, x.metodo, x.ruta, x.cuerpo); cod != http.StatusNotFound {
			t.Fatalf("un extraño %s %s: %d", x.metodo, x.ruta, cod)
		}
	}
	if cod, _ := pedir(t, h, ana, "PUT", base, `{"robots":["`+robotsBeto[0]+`"]}`); cod != http.StatusBadRequest {
		t.Fatalf("robots ajenos: %d", cod)
	}
	if cod, v := pedir(t, h, ana, "PUT", base, `{"robots":["`+strings.ToLower(robotsAna[1])+`"]}`); cod != http.StatusOK || v["todos"] != false ||
		len(v["robots"].([]any)) != 1 || v["robots"].([]any)[0] != robotsAna[1] {
		t.Fatalf("robots: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, beto, "DELETE", base, ""); cod != http.StatusOK {
		t.Fatalf("borrar: %d", cod)
	}
	if cod, _ := pedir(t, h, ana, "POST", base+"/toque", `{}`); cod != http.StatusNotFound {
		t.Fatalf("toque después de borrar: %d", cod)
	}
}

func jsonNum(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestPresenciaYHabitosHTTP(t *testing.T) {
	s, alm := servidorDePrueba(t)
	h := s.Rutas()
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)
	base := "/v1/bichos/" + bichos[0]
	if cod, v := pedir(t, h, token, "POST", base+"/presencia", `{"llegaste":true}`); cod != http.StatusOK || v["nueva"] != true || v["enviado"] != false {
		t.Fatalf("presencia: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "POST", base+"/presencia", `{"llegaste":true}`); cod != http.StatusOK || v["nueva"] != false {
		t.Fatalf("la misma: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/presencia", `{}`); cod != http.StatusBadRequest {
		t.Fatalf("sin llegaste: %d", cod)
	}
	if cod, _ := pedir(t, h, otro, "POST", base+"/presencia", `{"llegaste":false}`); cod != http.StatusNotFound {
		t.Fatalf("ajeno: %d", cod)
	}
	if cod, v := pedir(t, h, token, "GET", base+"/horarios", ""); cod != http.StatusOK || v["llegas"] != nil {
		t.Fatalf("horarios: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/habitos/0", `{"hecho":true}`); cod != http.StatusNotFound {
		t.Fatalf("sin hábitos: %d", cod)
	}
	alm.NuevaConfig(context.Background(), bichos[0], `{"version":2,"habitos":[{"nombre":"agua","icono":"gota"}]}`, 7, "api", nil) //nolint:errcheck
	if cod, v := pedir(t, h, token, "POST", base+"/habitos/0", `{"hecho":true}`); cod != http.StatusOK || v["habito"].(map[string]any)["racha"] != 1.0 {
		t.Fatalf("marcar: %d %v", cod, v)
	}
	for _, x := range []string{"/habitos/1", "/habitos/9", "/habitos/x"} {
		if cod, _ := pedir(t, h, token, "POST", base+x, `{"hecho":true}`); cod != http.StatusNotFound {
			t.Fatalf("%s: %d", x, cod)
		}
	}
	r := httptest.NewRequest("GET", base+"/habitos", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var l []map[string]any
	json.Unmarshal(w.Body.Bytes(), &l) //nolint:errcheck
	if w.Code != http.StatusOK || len(l) != 1 || l[0]["nombre"] != "agua" || len(l[0]["dias"].(map[string]any)) != 1 {
		t.Fatalf("hábitos: %d %s", w.Code, w.Body.String())
	}
}

// servidorEntrantes: con secreto y un Mercado Pago falso.
func servidorEntrantes(t *testing.T, mp string) (*Servidor, http.Handler, *eventos.Hub) {
	t.Helper()
	_, alm := servidorDePrueba(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{}, alm, n, hub, log)
	s := Nuevo(Opciones{URLPublica: "https://api.prueba", Secreto: []byte("secreto de prueba 1234"), MercadoPagoURL: mp}, alm, n, i, hub, nil, nil, log)
	return s, s.Rutas(), hub
}

func mandarEntrante(h http.Handler, ruta, cuerpo string, encabezados map[string]string) (int, map[string]any) {
	r := httptest.NewRequest("POST", ruta, strings.NewReader(cuerpo))
	for k, v := range encabezados {
		r.Header.Set(k, v)
	}
	r.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
	return w.Code, v
}

func firmarGitHub(token, cuerpo string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(cuerpo))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestEntrantesHTTP(t *testing.T) {
	var pedidoMP *http.Request
	mp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidoMP = r
		if r.URL.Path == "/v1/payments/777" {
			w.Write([]byte(`{"status":"approved","transaction_amount":12000,"currency_id":"ARS","payer":{"first_name":"Ana"}}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"status":"pending","transaction_amount":5,"currency_id":"ARS"}`)) //nolint:errcheck
	}))
	defer mp.Close()
	s, h, hub := servidorEntrantes(t, mp.URL)
	token, _ := cuentaDePrueba(t, s.alm)
	otro, _ := cuentaDePrueba(t, s.alm)
	sub := hub.Suscribir(usuarioDeToken(t, s.alm, token), "")
	defer hub.Desuscribir(sub)

	cod, v := pedir(t, h, token, "GET", "/v1/entrantes", "")
	tok, _ := v["token"].(string)
	if cod != http.StatusOK || !patronToken.MatchString(tok) || v["url"] != "https://api.prueba/v1/entrantes/"+tok || v["mercadoPago"] != false {
		t.Fatalf("ver: %d %v", cod, v)
	}
	if _, v2 := pedir(t, h, token, "GET", "/v1/entrantes", ""); v2["token"] != tok {
		t.Fatal("se tiene que poder volver a mostrar")
	}
	if _, v3 := pedir(t, h, otro, "GET", "/v1/entrantes", ""); v3["token"] == tok {
		t.Fatal("cada uno el suyo")
	}
	ruta := "/v1/entrantes/" + tok
	evento := func() map[string]any {
		t.Helper()
		select {
		case e := <-sub.C:
			if e.Tipo != "entrante" {
				t.Fatalf("evento %s", e.Tipo)
			}
			return e.Datos.(map[string]any)
		case <-time.After(time.Second):
			t.Fatal("sin evento entrante")
		}
		return nil
	}

	// JSON propio.
	if cod, v := mandarEntrante(h, ruta, `{"tipo":"pedido","titulo":"Llegó el pedido","de":"Rappi"}`, nil); cod != http.StatusOK || v["robots"] != 0.0 {
		t.Fatalf("propio: %d %v", cod, v)
	}
	if d := evento(); d["tipo"] != "pedido" || d["titulo"] != "Llegó el pedido" || d["de"] != "Rappi" || d["origen"] != "json" {
		t.Fatalf("evento: %v", d)
	}
	for _, c := range []string{`{"tipo":"llamada","titulo":"x"}`, `{"tipo":"pago"}`, `{"tipo":"pago","titulo":"x","otro":1}`, `no`,
		`{"tipo":"pago","titulo":"x","robot":"no-existe"}`} {
		if cod, v := mandarEntrante(h, ruta, c, nil); cod != http.StatusBadRequest {
			t.Fatalf("propio malo %s: %d %v", c, cod, v)
		}
	}
	if cod, _ := mandarEntrante(h, "/v1/entrantes/ent_"+strings.Repeat("a", 32), `{"tipo":"pago","titulo":"x"}`, nil); cod != http.StatusNotFound {
		t.Fatalf("token inventado: %d", cod)
	}

	// GitHub, firmado con el token.
	pr := `{"action":"opened","pull_request":{"number":12},"repository":{"name":"sintetica"},"sender":{"login":"ana"}}`
	gh := func(ev, cuerpo, firma string) (int, map[string]any) {
		return mandarEntrante(h, ruta, cuerpo, map[string]string{"X-GitHub-Event": ev, "X-Hub-Signature-256": firma})
	}
	if cod, _ := gh("pull_request", pr, firmarGitHub("otro", pr)); cod != http.StatusUnauthorized {
		t.Fatalf("firma mala: %d", cod)
	}
	if cod, _ := gh("pull_request", pr, ""); cod != http.StatusUnauthorized {
		t.Fatalf("sin firma: %d", cod)
	}
	if cod, v := gh("ping", `{}`, firmarGitHub(tok, `{}`)); cod != http.StatusOK || v["ping"] != true {
		t.Fatalf("ping: %d %v", cod, v)
	}
	if cod, _ := gh("pull_request", pr, firmarGitHub(tok, pr)); cod != http.StatusOK {
		t.Fatalf("PR: %d", cod)
	}
	if d := evento(); d["tipo"] != "github" || d["titulo"] != "PR #12" || d["de"] != "ana" {
		t.Fatalf("PR: %v", d)
	}
	ci := `{"action":"completed","workflow_run":{"name":"CI","conclusion":"failure"},"repository":{"name":"sintetica"},"sender":{"login":"bot"}}`
	if cod, _ := gh("workflow_run", ci, firmarGitHub(tok, ci)); cod != http.StatusOK {
		t.Fatalf("CI: %d", cod)
	}
	if d := evento(); d["titulo"] != "CI falló" || d["de"] != "sintetica" {
		t.Fatalf("CI: %v", d)
	}
	push := `{"ref":"refs/heads/main"}`
	if cod, v := gh("push", push, firmarGitHub(tok, push)); cod != http.StatusAccepted || v["ignorado"] != true {
		t.Fatalf("push: %d %v", cod, v)
	}

	// Mercado Pago sin token: "¡Te pagaron!", una vez por pago.
	pago := `{"action":"payment.created","type":"payment","data":{"id":"555"}}`
	if cod, _ := mandarEntrante(h, ruta, pago, nil); cod != http.StatusOK {
		t.Fatalf("pago: %d", cod)
	}
	if d := evento(); d["tipo"] != "pago" || d["titulo"] != "¡Te pagaron!" || d["origen"] != "mercadopago" {
		t.Fatalf("pago: %v", d)
	}
	if cod, _ := mandarEntrante(h, ruta, pago, nil); cod != http.StatusAccepted {
		t.Fatalf("el mismo pago: %d", cod)
	}
	if pedidoMP != nil {
		t.Fatal("sin token no se le pregunta a Mercado Pago")
	}
	// Con token: el monto, solo los aprobados.
	if cod, _ := pedir(t, h, token, "PUT", "/v1/entrantes/mercadopago", `{"accessToken":"cualquiera"}`); cod != http.StatusBadRequest {
		t.Fatalf("token MP malo: %d", cod)
	}
	if cod, v := pedir(t, h, token, "PUT", "/v1/entrantes/mercadopago", `{"accessToken":"APP_USR-1234567890-abc"}`); cod != http.StatusOK || v["mercadoPago"] != true {
		t.Fatalf("token MP: %d %v", cod, v)
	}
	if cod, _ := mandarEntrante(h, ruta+"?type=payment&data.id=888", ``, nil); cod != http.StatusAccepted {
		t.Fatalf("pendiente: %d", cod)
	}
	if cod, _ := mandarEntrante(h, ruta, `{"action":"payment.updated","type":"payment","data":{"id":777}}`, nil); cod != http.StatusOK {
		t.Fatalf("aprobado: %d", cod)
	}
	if pedidoMP == nil || pedidoMP.Header.Get("Authorization") != "Bearer APP_USR-1234567890-abc" {
		t.Fatal("el pedido a Mercado Pago")
	}
	if d := evento(); d["titulo"] != "$12.000" || d["de"] != "Ana" {
		t.Fatalf("monto: %v", d)
	}
	if cod, _ := mandarEntrante(h, ruta, `{"type":"payment","data":{"id":"../x"}}`, nil); cod != http.StatusAccepted {
		t.Fatalf("id raro: %d", cod)
	}

	// Regenerar: la vieja deja de andar.
	cod, v = pedir(t, h, token, "POST", "/v1/entrantes", "")
	if cod != http.StatusOK || v["token"] == tok || v["mercadoPago"] != true {
		t.Fatalf("regenerar: %d %v", cod, v)
	}
	if cod, _ := mandarEntrante(h, ruta, `{"tipo":"otro","titulo":"x"}`, nil); cod != http.StatusNotFound {
		t.Fatalf("la vieja: %d", cod)
	}
}

func TestMonto(t *testing.T) {
	for v, s := range map[float64]string{12000: "$12.000", 1234.5: "$1.234,50", 5: "$5", 1000000: "$1.000.000", 999.99: "$999,99"} {
		if m := monto(v, "ARS"); m != s {
			t.Errorf("%v: %q", v, m)
		}
	}
	if m := monto(10, "USD"); m != "US$ 10" {
		t.Errorf("dólares: %q", m)
	}
}

func TestMCPCompania(t *testing.T) {
	_, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	amigo := nuevoUsuarioMCP(t, alm)
	alm.ActualizarNombre(context.Background(), amigo.id, "Beto") //nolint:errcheck
	_, lista := rpc(t, h, u.token, "tools/list", map[string]any{})
	crudo, _ := json.Marshal(lista)
	for _, n := range []string{`"tocar"`, `"amigos"`, `"habitos"`, `"horarios"`, `"entrantes"`} {
		if !strings.Contains(string(crudo), n) {
			t.Fatalf("falta la herramienta %s", n)
		}
	}
	llamar := func(token, nombre string, args map[string]any) resultadoHerramienta {
		t.Helper()
		return llamar(t, h, token, nombre, args)
	}
	if r := llamar(u.token, "tocar", map[string]any{"tipo": "abrazo"}); !r.error || !strings.Contains(r.texto, "no tenés amigos") {
		t.Fatalf("sin amigos: %+v", r)
	}
	r := llamar(amigo.token, "amigos", map[string]any{"accion": "invitar"})
	m := regexp.MustCompile(`"codigo":"([A-Z]{8})"`).FindStringSubmatch(r.texto)
	if r.error || m == nil {
		t.Fatalf("invitar: %+v", r)
	}
	if r := llamar(u.token, "amigos", map[string]any{"accion": "aceptar", "codigo": m[1]}); r.error || !strings.Contains(r.texto, "Beto") {
		t.Fatalf("aceptar: %+v", r)
	}
	if r := llamar(u.token, "tocar", map[string]any{"amigo": "beto", "tipo": "beso"}); r.error || !strings.Contains(r.texto, "no le llegó") {
		t.Fatalf("tocar: %+v", r)
	}
	robot := u.bichos[0]
	alm.NuevaConfig(context.Background(), robot, `{"version":2,"habitos":[{"nombre":"Meditar","icono":"zen"}]}`, 9, "api", nil) //nolint:errcheck
	if r := llamar(u.token, "habitos", map[string]any{"robot": robot, "accion": "marcar", "habito": "meditar"}); r.error || !strings.Contains(r.texto, "racha: 1") {
		t.Fatalf("marcar: %+v", r)
	}
	if r := llamar(u.token, "horarios", map[string]any{"robot": robot}); r.error || !strings.Contains(r.texto, "Todavía no sé") {
		t.Fatalf("horarios: %+v", r)
	}
	if r := llamar(u.token, "entrantes", map[string]any{"accion": "ver"}); r.error || !strings.Contains(r.texto, apiPrueba+"/v1/entrantes/ent_") {
		t.Fatalf("entrantes: %+v", r)
	}
	if r := llamar(u.token, "amigos", map[string]any{"accion": "borrar", "amigo": "Beto"}); r.error {
		t.Fatalf("borrar: %+v", r)
	}
}
