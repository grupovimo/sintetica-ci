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

// Las rutas de la voz con un ElevenLabs falso (nada contra el de verdad).
func TestVozHTTP(t *testing.T) {
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	const clave = "sk_prueba_http"
	el := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != clave {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v2/voices":
			io.WriteString(w, `{"voices":[{"voice_id":"v1","name":"Lucía"},{"voice_id":"v2","name":"Mateo"}],"has_more":false}`) //nolint:errcheck
		case "POST /v1/text-to-speech/v2":
			w.Header().Set("Content-Type", "audio/mpeg")
			io.WriteString(w, "ID3audio") //nolint:errcheck
		case "POST /v1/convai/tools":
			io.WriteString(w, `{"id":"tool_x"}`) //nolint:errcheck
		case "POST /v1/convai/agents/create":
			io.WriteString(w, `{"agent_id":"agent_http"}`) //nolint:errcheck
		case "GET /v1/convai/conversation/get-signed-url":
			io.WriteString(w, `{"signed_url":"wss://api.elevenlabs.io/v1/convai/conversation?agent_id=agent_http&conversation_signature=x"}`) //nolint:errcheck
		case "GET /v1/convai/conversations":
			io.WriteString(w, `{"conversations":[],"has_more":false}`) //nolint:errcheck
		case "DELETE /v1/convai/agents/agent_http":
			io.WriteString(w, `{}`) //nolint:errcheck
		default:
			t.Errorf("ElevenLabs falso: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer el.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{V19: integraciones.OpcionesV19{Secreto: []byte("secreto de prueba 1234"), ElevenLabsURL: el.URL}}, alm, n, hub, log)
	n.UsarVoz(i)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm, n, i, hub, nil, nil, log)
	h := s.Rutas()
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)

	if cod, v := pedir(t, h, token, "GET", "/v1/voz/elevenlabs", ""); cod != 200 || v["conectado"] != false {
		t.Fatalf("GET sin cuenta: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "PUT", "/v1/voz/elevenlabs", `{"voz":"v1"}`); cod != http.StatusNotFound {
		t.Fatalf("voz sin cuenta: %d", cod)
	}
	for _, c := range []string{`{"clave":"mala"}`, `{"clave":""}`, `{"clave":"con espacio"}`, `{"otra":1}`} {
		if cod, _ := pedir(t, h, token, "PUT", "/v1/voz/elevenlabs", c); cod != http.StatusBadRequest {
			t.Errorf("PUT %s: %d", c, cod)
		}
	}
	cod, v := pedir(t, h, token, "PUT", "/v1/voz/elevenlabs", `{"clave":"`+clave+`","voz":"v2"}`)
	b, _ := json.Marshal(v)
	if cod != 200 || v["conectado"] != true || v["voz"] != "v2" || v["voz_nombre"] != "Mateo" || v["tope"] != float64(5000) ||
		v["usados_hoy"] != float64(0) || len(v["voces"].([]any)) != 2 || strings.Contains(string(b), clave) {
		t.Fatalf("PUT: %d %s", cod, b)
	}
	if cod, v := pedir(t, h, token, "PUT", "/v1/voz/elevenlabs", `{"tope":-1}`); cod != http.StatusBadRequest {
		t.Fatalf("tope negativo: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "PUT", "/v1/voz/elevenlabs", `{"tope":2000,"tope_minutos":5,"leer":false}`); cod != 200 || v["tope"] != float64(2000) ||
		v["tope_minutos"] != float64(5) || v["leer"] != false {
		t.Fatalf("topes: %d %v", cod, v)
	}
	// Otro usuario no ve nada de esta cuenta.
	if cod, v := pedir(t, h, otro, "GET", "/v1/voz/elevenlabs", ""); cod != 200 || v["conectado"] != false {
		t.Fatalf("ajeno: %d %v", cod, v)
	}

	// Probar: el mp3, y cuenta para el tope.
	r := httptest.NewRequest("POST", "/v1/voz/elevenlabs/probar", strings.NewReader(`{"texto":"Hola"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Content-Type") != "audio/mpeg" || w.Body.String() != "ID3audio" {
		t.Fatalf("probar: %d %s %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if _, v := pedir(t, h, token, "GET", "/v1/voz/elevenlabs", ""); v["usados_hoy"] != float64(4) {
		t.Fatalf("usados: %v", v["usados_hoy"])
	}
	if cod, _ := pedir(t, h, token, "POST", "/v1/voz/elevenlabs/probar", `{"texto":"`+strings.Repeat("a", 201)+`"}`); cod != http.StatusBadRequest {
		t.Fatalf("probar largo: %d", cod)
	}

	// La conversación: del dueño, no de otro.
	cod, v = pedir(t, h, token, "GET", "/v1/bichos/"+bichos[0]+"/conversacion", "")
	if cod != 200 || !strings.HasPrefix(v["url"].(string), "wss://") || v["agente"] != "agent_http" || v["segundos"] != float64(300) {
		t.Fatalf("conversación: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, otro, "GET", "/v1/bichos/"+bichos[0]+"/conversacion", ""); cod != http.StatusNotFound {
		t.Fatalf("conversación ajena: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/bichos/"+bichos[0]+"/conversacion?modo=telepatia", ""); cod != http.StatusBadRequest {
		t.Fatalf("modo: %d", cod)
	}

	if cod, v := pedir(t, h, token, "DELETE", "/v1/voz/elevenlabs", ""); cod != 200 || v["conectado"] != false {
		t.Fatalf("DELETE: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/bichos/"+bichos[0]+"/conversacion", ""); cod != http.StatusNotFound {
		t.Fatalf("conversación sin cuenta: %d", cod)
	}
	if cod, _ := pedir(t, h, "", "GET", "/v1/voz/elevenlabs", ""); cod != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", cod)
	}
}
