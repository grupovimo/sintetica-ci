package integraciones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// anthropicConModelos: el falso de /v1/messages más GET /v1/models/{modelo}.
func anthropicConModelos(f *anthropicFalso, modelos ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m, ok := strings.CutPrefix(r.URL.Path, "/v1/models/"); ok {
			if r.Header.Get("X-Api-Key") != f.clave || r.Header.Get("Anthropic-Version") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			for _, x := range modelos {
				if x == m {
					fmt.Fprintf(w, `{"type":"model","id":%q}`, m)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.ServeHTTP(w, r)
	})
}

func TestIAPropiaAnthropic(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	falso := &anthropicFalso{clave: "sk-ant-propia", respuesta: `{"texto":"¡Con mi clave!","expresion":"contento"}`}
	srv := httptest.NewServer(anthropicConModelos(falso, ModeloCharla, "claude-otro"))
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	// El servidor sin IA: sin clave propia, apagada.
	i := Nuevo(Opciones{V19: OpcionesV19{Secreto: []byte("secreto de prueba 1234"), AnthropicURL: srv.URL}}, a, robots, eventos.Nuevo(), sinLog)
	if e := i.EstadoCharla(ctx, u.ID); e["disponible"] != false {
		t.Fatalf("sin nada: %v", e)
	}
	if _, err := i.Charlar(ctx, u.ID, id, "hola", "web"); !errors.Is(err, ErrCharlaApagada) {
		t.Fatal(err)
	}

	// Una clave mala, un modelo que no está o un proveedor raro no se guardan.
	mala, buena := "sk-ant-mala", "sk-ant-propia"
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IAAnthropic, Clave: &mala}); !errors.Is(err, ErrClaveIA) {
		t.Fatalf("clave mala: %v", err)
	}
	otro := "claude-que-no-esta"
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IAAnthropic, Clave: &buena, Modelo: &otro}); !errors.Is(err, ErrModeloIA) {
		t.Fatalf("modelo: %v", err)
	}
	var cv ErrCambioIA
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: "openai", Clave: &buena}); !errors.As(err, &cv) {
		t.Fatalf("proveedor: %v", err)
	}
	mucho := MaxTopeIA + 1
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IAAnthropic, Clave: &buena, Tope: &mucho}); !errors.As(err, &cv) {
		t.Fatalf("tope: %v", err)
	}
	if _, err := a.IADe(ctx, u.ID); err == nil {
		t.Fatal("guardó algo malo")
	}

	// La buena: cifrada, sin tope.
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IAAnthropic, Clave: &buena}); err != nil {
		t.Fatal(err)
	}
	c, err := a.IADe(ctx, u.ID)
	if err != nil || bytes.Contains(c.ClaveCifrada, []byte(buena)) || c.Modelo != ModeloCharla || c.Tope != 0 {
		t.Fatalf("cuenta: %+v %v", c, err)
	}
	e, _ := i.EstadoIA(ctx, u.ID)
	b, _ := json.Marshal(e)
	if e["propia"] != true || e["proveedor"] != IAAnthropic || e["tope"] != 0 || e["servidor"] != false || strings.Contains(string(b), buena) {
		t.Fatalf("estado: %s", b)
	}
	if e := i.EstadoCharla(ctx, u.ID); e["disponible"] != true || e["propia"] != true || e["quedan"] != -1 || e["sin_tope"] != true {
		t.Fatalf("charla: %v", e)
	}
	r, err := i.Charlar(ctx, u.ID, id, "¿cómo andás?", "web")
	if err != nil || r.Texto != "¡Con mi clave!" || !r.Propia || r.Quedan != -1 {
		t.Fatalf("charlar: %+v %v", r, err)
	}
	// Por MQTT (el robot-celu) también, aunque el servidor no tenga IA.
	vaciar(robots)
	i.CharlaMQTT(id, []byte(`{"texto":"hola desde el celu"}`))
	m, _ := protocolo.Decir("¡Con mi clave!", "contento")
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, m))

	// Su tope: lo respeta.
	dos, modelo := 3, "claude-otro"
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Tope: &dos, Modelo: &modelo}); err != nil {
		t.Fatal(err)
	}
	r, err = i.Charlar(ctx, u.ID, id, "otra", "web")
	if err != nil || r.Quedan != 0 {
		t.Fatalf("con tope: %+v %v", r, err)
	}
	falso.mu.Lock()
	ultimo := falso.pedidos[len(falso.pedidos)-1]["model"]
	falso.mu.Unlock()
	if ultimo != "claude-otro" {
		t.Fatalf("modelo: %v", ultimo)
	}
	if _, err := i.Charlar(ctx, u.ID, id, "y otra", "web"); !errors.Is(err, ErrTopeCharla) || !strings.Contains(err.Error(), "3 veces") {
		t.Fatalf("tope lleno: %v", err)
	}

	// Sin clave propia, otra vez apagada.
	if err := i.DesconectarIA(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if e := i.EstadoCharla(ctx, u.ID); e["disponible"] != false {
		t.Fatalf("borrada: %v", e)
	}
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Tope: &dos}); !errors.Is(err, ErrSinIA) {
		t.Fatalf("sin cuenta: %v", err)
	}
}

func TestIAPropiaCompatible(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	var mu sync.Mutex
	var pedidos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gsk-propia" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		pedidos = append(pedidos, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/openai/v1/models":
			io.WriteString(w, `{"object":"list","data":[{"id":"llama-propia"},{"id":"otro"}]}`) //nolint:errcheck
		case "/openai/v1/chat/completions":
			var p map[string]any
			json.NewDecoder(r.Body).Decode(&p)                                                                                                                 //nolint:errcheck
			io.WriteString(w, `{"choices":[{"message":{"content":"{\"texto\":\"Desde mi servicio: `+p["model"].(string)+`\",\"expresion\":\"contento\"}"}}]}`) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}

	// En producción: solo https y nada interno (sin tocar la red).
	prod := Nuevo(Opciones{V19: OpcionesV19{Secreto: []byte("secreto de prueba 1234")}}, a, robots, eventos.Nuevo(), sinLog)
	clave := "gsk-propia"
	modelo := "llama-propia"
	for _, u2 := range []string{"", "http://api.groq.com/openai/v1", "https://127.0.0.1/v1", "https://10.1.2.3/v1", "https://localhost/v1",
		"https://user:pw@api.groq.com/v1", "https://api.groq.com/v1?x=1", "ftp://api.groq.com"} {
		var cv ErrCambioIA
		if err := prod.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IACompatible, Clave: &clave, URL: u2, Modelo: &modelo}); !errors.As(err, &cv) {
			t.Errorf("url %q: %v", u2, err)
		}
	}
	// Sin modelo, no.
	var cv ErrCambioIA
	if err := prod.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IACompatible, Clave: &clave, URL: "https://api.groq.com/openai/v1"}); !errors.As(err, &cv) {
		t.Fatalf("sin modelo: %v", err)
	}

	// El servidor con su Anthropic; el usuario, con su servicio: gana el suyo.
	i := Nuevo(Opciones{Desarrollo: true, V19: OpcionesV19{Secreto: []byte("secreto de prueba 1234"), AnthropicClave: "sk-del-servidor",
		AnthropicURL: "http://127.0.0.1:1"}}, a, robots, eventos.Nuevo(), sinLog)
	mala := "gsk-mala"
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IACompatible, Clave: &mala, URL: srv.URL + "/openai/v1", Modelo: &modelo}); !errors.Is(err, ErrClaveIA) {
		t.Fatalf("clave mala: %v", err)
	}
	nada := "no-esta"
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IACompatible, Clave: &clave, URL: srv.URL + "/openai/v1", Modelo: &nada}); !errors.Is(err, ErrModeloIA) {
		t.Fatalf("modelo: %v", err)
	}
	cinco := 5
	if err := i.ConectarIA(ctx, u.ID, CambiosIA{Proveedor: IACompatible, Clave: &clave, URL: srv.URL + "/openai/v1/", Modelo: &modelo, Tope: &cinco}); err != nil {
		t.Fatal(err)
	}
	e, _ := i.EstadoIA(ctx, u.ID)
	if e["proveedor"] != IACompatible || e["modelo"] != "llama-propia" || e["url_pista"] != strings.TrimPrefix(srv.URL, "http://") || e["tope"] != 5 ||
		e["servidor"] != true {
		t.Fatalf("estado: %v", e)
	}
	r, err := i.Charlar(ctx, u.ID, id, "hola", "web")
	if err != nil || r.Texto != "Desde mi servicio: llama-propia" || r.Quedan != 4 || !r.Propia {
		t.Fatalf("charlar: %+v %v", r, err)
	}
	if ec := i.EstadoCharla(ctx, u.ID); ec["propia"] != true || ec["porDia"] != 5 || ec["quedan"] != 4 {
		t.Fatalf("charla: %v", ec)
	}
}
