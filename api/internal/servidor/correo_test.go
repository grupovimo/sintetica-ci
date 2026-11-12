package servidor

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

// El mail por HTTP: apagado sin claves; con las de Google (contra un falso),
// el enlace, la vuelta (cancelada, mala y buena), el estado y desconectar.
func TestCorreoHTTP(t *testing.T) {
	// Sin claves.
	_, alm, h := servidorHablaDePrueba(t)
	token, _ := cuentaDePrueba(t, alm)
	if cod, v := pedir(t, h, token, "GET", "/v1/correo", ""); cod != 200 || v["gmail"].(map[string]any)["disponible"] != false ||
		v["outlook"].(map[string]any)["disponible"] != false || len(v["noLeidos"].([]any)) != 0 {
		t.Fatalf("GET correo apagado: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/correo/gmail", ""); cod != http.StatusServiceUnavailable {
		t.Fatalf("gmail sin claves: %d", cod)
	}

	// Con Google.
	falso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			r.ParseForm() //nolint:errcheck
			if r.Form.Get("code") != "codigo-bueno" || r.Form.Get("client_secret") != "secreto-g" {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error":"invalid_grant"}`) //nolint:errcheck
				return
			}
			io.WriteString(w, `{"access_token":"acceso","refresh_token":"refresco","expires_in":3600}`) //nolint:errcheck
		case "/gmail/v1/users/me/profile":
			io.WriteString(w, `{"emailAddress":"ana@gmail.com"}`) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer falso.Close()
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	alm2, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm2.Cerrar)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm2, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{V19: integraciones.OpcionesV19{Secreto: []byte("secreto de prueba"), APIURL: apiPrueba,
		GoogleID: "cliente-g", GoogleSecreto: "secreto-g", GoogleCuentasURL: falso.URL, GmailURL: falso.URL}}, alm2, n, hub, log)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm2, n, i, hub, nil, nil, log)
	h = s.Rutas()
	token, _ = cuentaDePrueba(t, alm2)

	if cod, _ := pedir(t, h, token, "GET", "/v1/correo/yahoo", ""); cod != http.StatusNotFound {
		t.Fatalf("proveedor raro: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/correo/outlook", ""); cod != http.StatusServiceUnavailable {
		t.Fatalf("outlook sin claves: %d", cod)
	}
	if cod, _ := pedir(t, h, "", "GET", "/v1/correo/gmail", ""); cod != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", cod)
	}
	cod, v := pedir(t, h, token, "GET", "/v1/correo/gmail", "")
	enlace, _ := v["url"].(string)
	if cod != 200 || !strings.HasPrefix(enlace, falso.URL+"/o/oauth2/v2/auth?") {
		t.Fatalf("enlace: %d %v", cod, v)
	}
	pu, _ := url.Parse(enlace)
	estado := url.QueryEscape(pu.Query().Get("state"))
	vuelta := func(q string) string {
		r := httptest.NewRequest("GET", "/v1/correo/gmail/vuelta?"+q, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusFound {
			t.Fatalf("vuelta %s: %d", q, w.Code)
		}
		return w.Header().Get("Location")
	}
	if l := vuelta("error=access_denied&state=" + estado); l != webPrueba+"/robots?correo=cancelado" {
		t.Fatalf("cancelado: %s", l)
	}
	if l := vuelta("code=codigo-malo&state=" + estado); !strings.HasPrefix(l, webPrueba+"/robots?correo=error") {
		t.Fatalf("código malo: %s", l)
	}
	if l := vuelta("code=codigo-bueno&state=" + estado); l != webPrueba+"/robots?correo=ok" {
		t.Fatalf("ok: %s", l)
	}
	cod, v = pedir(t, h, token, "GET", "/v1/correo", "")
	g := v["gmail"].(map[string]any)
	if cod != 200 || g["disponible"] != true || g["conectada"] != true || g["direccion"] != "ana@gmail.com" || v["outlook"].(map[string]any)["disponible"] != false {
		t.Fatalf("GET correo: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "DELETE", "/v1/correo/yahoo", ""); cod != http.StatusNotFound {
		t.Fatalf("DELETE raro: %d", cod)
	}
	cod, v = pedir(t, h, token, "DELETE", "/v1/correo/gmail", "")
	if cod != 200 || v["gmail"].(map[string]any)["conectada"] != false {
		t.Fatalf("DELETE: %d %v", cod, v)
	}
}
