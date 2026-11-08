package servidor

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

// Sin claves de Spotify ni de Anthropic: todo apagado y dicho.
func TestAgendaMusicaCharlaApagadas(t *testing.T) {
	_, alm, h := servidorHablaDePrueba(t)
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)
	base := "/v1/bichos/" + bichos[0]

	if cod, v := pedir(t, h, token, "GET", "/v1/charla", ""); cod != 200 || v["disponible"] != false {
		t.Fatalf("GET charla: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/charla", `{"texto":"hola"}`); cod != http.StatusServiceUnavailable {
		t.Fatalf("charla sin clave: %d", cod)
	}
	if cod, v := pedir(t, h, token, "GET", "/v1/musica", ""); cod != 200 || v["disponible"] != false {
		t.Fatalf("GET musica: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/musica/spotify", ""); cod != http.StatusServiceUnavailable {
		t.Fatalf("spotify sin claves: %d", cod)
	}
	// La música del celu: bien armada pero desconectado (409), mal armada (400), ajena (404).
	if cod, _ := pedir(t, h, token, "POST", base+"/musica", `{"sonando":true,"titulo":"Persiana americana","artista":"Soda Stereo","bpm":120}`); cod != http.StatusConflict {
		t.Fatalf("musica desconectado: %d", cod)
	}
	for _, c := range []string{`{"sonando":true,"bpm":300}`, `{"sonando":true,"otra":1}`, `{"titulo":"` + strings.Repeat("a", 201) + `"}`} {
		if cod, _ := pedir(t, h, token, "POST", base+"/musica", c); cod != http.StatusBadRequest {
			t.Errorf("musica %s: %d", c, cod)
		}
	}
	if cod, _ := pedir(t, h, otro, "POST", base+"/musica", `{"sonando":false}`); cod != http.StatusNotFound {
		t.Fatalf("musica ajena: %d", cod)
	}

	// La agenda.
	if cod, v := pedir(t, h, token, "GET", "/v1/agenda", ""); cod != 200 || v["conectada"] != false || v["aviso"] != float64(10) {
		t.Fatalf("GET agenda: %d %v", cod, v)
	}
	for _, c := range []string{`{}`, `{"ics":"http://calendar.google.com/a.ics"}`, `{"ics":"https://127.0.0.1/a.ics"}`, `{"ics":"https://localhost/a.ics"}`, `{"otra":1}`} {
		if cod, _ := pedir(t, h, token, "PUT", "/v1/agenda", c); cod != http.StatusBadRequest {
			t.Errorf("PUT agenda %s: %d", c, cod)
		}
	}
	if cod, _ := pedir(t, h, token, "PUT", "/v1/agenda", `{"aviso":5}`); cod != http.StatusNotFound {
		t.Fatalf("aviso sin agenda: %d", cod)
	}
	if cod, v := pedir(t, h, token, "DELETE", "/v1/agenda", ""); cod != 200 || v["conectada"] != false {
		t.Fatalf("DELETE agenda: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, "", "GET", "/v1/agenda", ""); cod != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", cod)
	}
	// La vuelta de Spotify sin claves va a la web con error.
	r := httptest.NewRequest("GET", "/v1/musica/spotify/vuelta?state=x&code=y", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "spotify=error") {
		t.Fatalf("vuelta: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestCharlaHTTP(t *testing.T) {
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	ia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "msg_1", "type": "message", "role": "assistant", "model": p["model"], //nolint:errcheck
			"content":     []map[string]any{{"type": "text", "text": `{"texto":"¡Hola! Acá estoy.","expresion":"contento"}`}},
			"stop_reason": "end_turn", "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	}))
	defer ia.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{V19: integraciones.OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: ia.URL}}, alm, n, hub, log)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm, n, i, hub, nil, nil, log)
	h := s.Rutas()
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)
	base := "/v1/bichos/" + bichos[0]

	if cod, v := pedir(t, h, token, "GET", "/v1/charla", ""); cod != 200 || v["disponible"] != true || v["quedan"] != float64(integraciones.CharlasPorDia) {
		t.Fatalf("GET charla: %d %v", cod, v)
	}
	cod, v := pedir(t, h, token, "POST", base+"/charla", `{"texto":"¿cómo andás?"}`)
	if cod != 200 || v["texto"] != "¡Hola! Acá estoy." || v["expresion"] != "contento" || v["enviado"] != false || v["quedan"] != float64(integraciones.CharlasPorDia-1) {
		t.Fatalf("charla: %d %v", cod, v)
	}
	for _, c := range []string{`{"texto":""}`, `{"texto":"` + strings.Repeat("a", 501) + `"}`, `{"otra":1}`} {
		if cod, _ := pedir(t, h, token, "POST", base+"/charla", c); cod != http.StatusBadRequest {
			t.Errorf("charla %s: %d", c, cod)
		}
	}
	if cod, _ := pedir(t, h, otro, "POST", base+"/charla", `{"texto":"hola"}`); cod != http.StatusNotFound {
		t.Fatalf("charla ajena: %d", cod)
	}
}
