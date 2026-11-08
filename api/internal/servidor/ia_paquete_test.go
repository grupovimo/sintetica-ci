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

// La clave de IA propia y el paquete de voz por HTTP (con servicios falsos).
func TestIAYPaqueteHTTP(t *testing.T) {
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	const clave = "sk-ant-http"
	ia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != clave {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/models/"+integraciones.ModeloCharla:
			io.WriteString(w, `{"type":"model","id":"`+integraciones.ModeloCharla+`"}`) //nolint:errcheck
		case r.URL.Path == "/v1/messages":
			io.WriteString(w, `{"id":"m","type":"message","role":"assistant","model":"x","stop_reason":"end_turn",
				"content":[{"type":"text","text":"{\"texto\":\"¡Hola!\",\"expresion\":\"contento\"}"}],"usage":{"input_tokens":1,"output_tokens":1}}`) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ia.Close()
	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voces" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, `[{"ShortName":"es-AR-ElenaNeural","FriendlyName":"Elena","Locale":"es-AR","Gender":"Female"}]`) //nolint:errcheck
	}))
	defer edge.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{V19: integraciones.OpcionesV19{Secreto: []byte("secreto de prueba 1234"), AnthropicURL: ia.URL,
		EdgeURL: "ws" + strings.TrimPrefix(edge.URL, "http") + "/edge/v1", EdgeVocesURL: edge.URL + "/voces"}}, alm, n, hub, log)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm, n, i, hub, nil, nil, log)
	h := s.Rutas()
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)

	// Sin IA en el servidor ni propia: apagada.
	if cod, v := pedir(t, h, token, "GET", "/v1/charla", ""); cod != 200 || v["disponible"] != false {
		t.Fatalf("charla: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "GET", "/v1/ia", ""); cod != 200 || v["propia"] != false || v["servidor"] != false {
		t.Fatalf("GET ia: %d %v", cod, v)
	}
	for _, c := range []string{`{"proveedor":"anthropic","clave":"mala"}`, `{"proveedor":"x","clave":"` + clave + `"}`,
		`{"proveedor":"compatible","clave":"k","url":"http://10.0.0.1/v1","modelo":"m"}`, `{"otra":1}`, `{"proveedor":"anthropic","clave":"` + clave + `","tope":-1}`} {
		if cod, _ := pedir(t, h, token, "PUT", "/v1/ia", c); cod != http.StatusBadRequest {
			t.Errorf("PUT %s: %d", c, cod)
		}
	}
	if cod, _ := pedir(t, h, token, "PUT", "/v1/ia", `{"tope":3}`); cod != http.StatusNotFound {
		t.Fatalf("sin cuenta: %d", cod)
	}
	cod, v := pedir(t, h, token, "PUT", "/v1/ia", `{"proveedor":"anthropic","clave":"`+clave+`"}`)
	b, _ := json.Marshal(v)
	if cod != 200 || v["propia"] != true || v["modelo"] != integraciones.ModeloCharla || v["tope"] != 0.0 || strings.Contains(string(b), clave) {
		t.Fatalf("PUT ia: %d %s", cod, b)
	}
	if cod, v := pedir(t, h, token, "GET", "/v1/charla", ""); cod != 200 || v["disponible"] != true || v["propia"] != true || v["sin_tope"] != true {
		t.Fatalf("charla propia: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "POST", "/v1/bichos/"+bichos[0]+"/charla", `{"texto":"hola"}`); cod != 200 || v["texto"] != "¡Hola!" || v["propia"] != true {
		t.Fatalf("charlar: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, otro, "GET", "/v1/charla", ""); cod != 200 || v["disponible"] != false {
		t.Fatalf("otro: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "DELETE", "/v1/ia", ""); cod != 200 || v["propia"] != false {
		t.Fatalf("DELETE ia: %d %v", cod, v)
	}

	// El paquete de voz: el estado (con la voz gratis), del dueño.
	cod, v = pedir(t, h, token, "GET", "/v1/bichos/"+bichos[0]+"/voz/paquete", "")
	if cod != 200 || v["estado"] != "nuevo" || v["proveedor"] != "edge" || v["total"].(float64) < 10 || v["gasta"] != 0.0 {
		t.Fatalf("GET paquete: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, otro, "GET", "/v1/bichos/"+bichos[0]+"/voz/paquete", ""); cod != http.StatusNotFound {
		t.Fatalf("ajeno: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/bichos/"+bichos[0]+"/voz/paquete?tope=-5", ""); cod != http.StatusBadRequest {
		t.Fatalf("tope: %d", cod)
	}
	// La bajada: sin firma (o con una inventada), no.
	for _, q := range []string{"", "?v=1&tope=0&vence=99999999999&firma=00000000000000000000000000000000"} {
		if cod, _ := pedir(t, h, "", "GET", "/v1/voz/paquete/"+bichos[0]+q, ""); cod != http.StatusForbidden {
			t.Errorf("bajar %q: %d", q, cod)
		}
	}
}
