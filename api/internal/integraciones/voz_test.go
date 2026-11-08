package integraciones

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"sintetica/api/internal/edgetts"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

const claveBuena = "sk_clave_de_prueba_123"

// elevenFalso: lo que se usa de la API de ElevenLabs (como en elevenlabs.io/docs).
type elevenFalso struct {
	mu        sync.Mutex
	pedidos   []string
	cuerpos   map[string][]map[string]any // "POST /v1/convai/agents/create" → cuerpos
	segundos  int                         // lo que charlaron hoy
	agentes   int
	sinAgente atomic.Bool // los PATCH dan 404
}

func (f *elevenFalso) servidor(t *testing.T) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != claveBuena {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"detail":{"status":"invalid_api_key"}}`) //nolint:errcheck
			return
		}
		var cuerpo map[string]any
		json.NewDecoder(r.Body).Decode(&cuerpo) //nolint:errcheck
		ruta := r.Method + " " + r.URL.Path
		f.mu.Lock()
		f.pedidos = append(f.pedidos, ruta+"?"+r.URL.RawQuery)
		if f.cuerpos == nil {
			f.cuerpos = map[string][]map[string]any{}
		}
		f.cuerpos[ruta] = append(f.cuerpos[ruta], cuerpo)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case ruta == "GET /v2/voices":
			io.WriteString(w, `{"voices":[{"voice_id":"voz1","name":"Lucía","preview_url":"https://storage.example/lucia.mp3","category":"premade"},
				{"voice_id":"voz2","name":"Mateo","preview_url":null}],"has_more":false,"next_page_token":null}`) //nolint:errcheck
		case strings.HasPrefix(ruta, "POST /v1/text-to-speech/"):
			if r.URL.Query().Get("output_format") != FormatoVoz || cuerpo["model_id"] != ModeloVoz {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			io.WriteString(w, "ID3"+strings.TrimPrefix(r.URL.Path, "/v1/text-to-speech/")+":"+cuerpo["text"].(string)) //nolint:errcheck
		case ruta == "POST /v1/convai/tools":
			tc := cuerpo["tool_config"].(map[string]any)
			io.WriteString(w, `{"id":"tool_`+tc["name"].(string)+`","tool_config":{}}`) //nolint:errcheck
		case ruta == "POST /v1/convai/agents/create":
			f.mu.Lock()
			f.agentes++
			n := f.agentes
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"agent_id": "agent_" + string(rune('0'+n))}) //nolint:errcheck
		case strings.HasPrefix(ruta, "PATCH /v1/convai/agents/"):
			if f.sinAgente.Load() {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			io.WriteString(w, `{}`) //nolint:errcheck
		case strings.HasPrefix(ruta, "DELETE /v1/convai/agents/"):
			io.WriteString(w, `{}`) //nolint:errcheck
		case ruta == "GET /v1/convai/conversation/get-signed-url":
			io.WriteString(w, `{"signed_url":"wss://api.elevenlabs.io/v1/convai/conversation?agent_id=`+r.URL.Query().Get("agent_id")+`&conversation_signature=firma"}`) //nolint:errcheck
		case ruta == "GET /v1/convai/conversation/token":
			io.WriteString(w, `{"token":"tok_webrtc","conversation_id":"conv_1"}`) //nolint:errcheck
		case ruta == "GET /v1/convai/conversations":
			f.mu.Lock()
			s := f.segundos
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"conversations": []map[string]any{{"conversation_id": "c1", //nolint:errcheck
				"agent_id": r.URL.Query().Get("agent_id"), "start_time_unix_secs": time.Now().Unix() - 3600, "call_duration_secs": s, "status": "done"}},
				"has_more": false})
		default:
			t.Errorf("ElevenLabs falso: %s", ruta)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (f *elevenFalso) cuantos(prefijo string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.pedidos {
		if strings.HasPrefix(p, prefijo) {
			n++
		}
	}
	return n
}

// robotsConParlante: los robots falsos, con lo que se publica en sus tópicos.
type robotsConParlante struct {
	robotsFalsos
	pmu        sync.Mutex
	publicados []string // "sufijo datos" y "cmd" (el marco), en orden
	avisos     []bool   // los AudioVoz
}

func (r *robotsConParlante) PublicarEn(id, sufijo string, d []byte) error {
	r.pmu.Lock()
	defer r.pmu.Unlock()
	if sufijo == "audio" && len(d) == 2 && d[0] == protocolo.AudioVoz {
		r.avisos = append(r.avisos, d[1] == 1)
		return nil
	}
	r.publicados = append(r.publicados, sufijo+" "+string(d))
	return nil
}

func (r *robotsConParlante) avisoApagado() bool {
	r.pmu.Lock()
	defer r.pmu.Unlock()
	for _, a := range r.avisos {
		if !a {
			return true
		}
	}
	return false
}

func (r *robotsConParlante) anotar(s string) {
	r.pmu.Lock()
	defer r.pmu.Unlock()
	r.publicados = append(r.publicados, s)
}

func (r *robotsConParlante) lista() []string {
	r.pmu.Lock()
	defer r.pmu.Unlock()
	return append([]string(nil), r.publicados...)
}

// edgeFalso: el servicio de Edge (como en edgetts/edgetts_test.go): devuelve
// "ID3edge:<voz>".
func edgeFalso(t *testing.T, falla *atomic.Bool) *httptest.Server {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	vozDe := regexp.MustCompile(`voice name='([^']+)'`)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/voces" {
			io.WriteString(w, `[{"ShortName":"es-AR-ElenaNeural","FriendlyName":"Microsoft Elena Online (Natural) - Spanish (Argentina)","Locale":"es-AR","Gender":"Female"},
				{"ShortName":"es-AR-TomasNeural","FriendlyName":"Microsoft Tomas Online (Natural) - Spanish (Argentina)","Locale":"es-AR","Gender":"Male"},
				{"ShortName":"en-US-EmmaMultilingualNeural","FriendlyName":"Microsoft Emma","Locale":"en-US","Gender":"Female"}]`) //nolint:errcheck
			return
		}
		if falla.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		ws.ReadMessage() //nolint:errcheck
		_, p, _ := ws.ReadMessage()
		v := vozDe.FindStringSubmatch(string(p))
		if v == nil {
			return
		}
		cab := "X-RequestId:x\r\nContent-Type:audio/mpeg\r\nPath:audio\r\n"
		m := append(binary.BigEndian.AppendUint16(nil, uint16(len(cab))), cab...)
		ws.WriteMessage(websocket.BinaryMessage, append(m, "ID3edge:"+v[1]...))                    //nolint:errcheck
		ws.WriteMessage(websocket.TextMessage, []byte("X-RequestId:x\r\nPath:turn.end\r\n\r\n{}")) //nolint:errcheck
	}))
	t.Cleanup(s.Close)
	return s
}

func esperar(t *testing.T, que string, f func() bool) {
	t.Helper()
	for k := 0; k < 300; k++ {
		if f() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no pasó: %s", que)
}

func TestVozElevenLabs(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	f := &elevenFalso{}
	el := f.servidor(t)
	u, id := usuarioConBicho(t, a)
	_, otro := usuarioConBicho(t, a)
	rs := &robotsConParlante{robotsFalsos: robotsFalsos{duenos: map[string]string{id: u.ID, otro: "x"},
		estados: map[string]protocolo.Estado{id: {Perfil: "celu", Nombre: "Pipo"}, otro: {Perfil: "dev-147b"}}}}
	var edgeFalla atomic.Bool
	ed := edgeFalso(t, &edgeFalla)
	i := Nuevo(Opciones{V19: OpcionesV19{Secreto: []byte("secreto de prueba 1234"), ElevenLabsURL: el.URL,
		EdgeURL: "ws" + strings.TrimPrefix(ed.URL, "http") + "/edge/v1", EdgeVocesURL: ed.URL + "/voces"}}, a, rs, eventos.Nuevo(), sinLog)
	fijo := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
	i.v19.ahora = func() time.Time { return fijo }

	// Sin ElevenLabs: la voz gratis (Edge), con la de Argentina.
	m, _ := protocolo.Decir("Hola Ana", "")
	if !i.TomarDecir(id, m, func() error { rs.anotar("cmd"); return nil }) {
		t.Fatal("sin ElevenLabs lee con Edge")
	}
	esperar(t, "el globo con Edge", func() bool { l := rs.lista(); return len(l) == 2 && l[1] == "cmd" })
	if tr, _ := protocolo.LeerTrozoAudio([]byte(strings.TrimPrefix(rs.lista()[0], "audio "))); string(tr.Datos) != "ID3edge:es-AR-ElenaNeural" {
		t.Fatalf("edge: %q", tr.Datos)
	}
	// Si Edge falla, el globo sale igual, sin audio (el robot usa su voz).
	edgeFalla.Store(true)
	i.TomarDecir(id, m, func() error { rs.anotar("cmd"); return nil })
	esperar(t, "el globo sin Edge", func() bool { return len(rs.lista()) == 3 })
	if rs.lista()[2] != "cmd" {
		t.Fatalf("con Edge caído: %q", rs.lista())
	}
	edgeFalla.Store(false)
	// Otra voz es-*, sí; una que no está o que no es es-*, no.
	if g, _ := i.EstadoEdge(ctx, u.ID, true); g["prendida"] != true || g["voz"] != "es-AR-ElenaNeural" || len(g["voces"].([]edgetts.Voz)) != 2 ||
		g["usados_hoy"] != 16 || g["tope"] != TopeEdgePorDefecto {
		t.Fatalf("estado edge: %v", g)
	}
	for _, v := range []string{"en-US-EmmaMultilingualNeural", "es-AR-PepeNeural", "x'><y"} {
		var cv ErrCambioVoz
		if err := i.CambiarEdge(ctx, u.ID, CambiosEdge{Voz: &v}); !errors.As(err, &cv) {
			t.Errorf("voz edge %q: %v", v, err)
		}
	}
	tomas := "es-AR-TomasNeural"
	if err := i.CambiarEdge(ctx, u.ID, CambiosEdge{Voz: &tomas}); err != nil {
		t.Fatal(err)
	}
	if mp3, err := i.ProbarEdge(ctx, u.ID, ""); err != nil || string(mp3) != "ID3edge:es-AR-TomasNeural" {
		t.Fatalf("probar edge: %q %v", mp3, err)
	}
	rs.pmu.Lock()
	rs.publicados = nil
	rs.pmu.Unlock()
	if e, _ := i.EstadoVoz(ctx, u.ID, true); e["conectado"] != false || e["tope"] != TopeCaracteresPorDefecto {
		t.Fatalf("estado sin cuenta: %v", e)
	}

	// Una clave mala no se guarda; una buena sí, cifrada, con la primera voz.
	mala := "sk_otra"
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Clave: &mala}); !errors.Is(err, ErrClaveElevenLabs) {
		t.Fatalf("clave mala: %v", err)
	}
	buena := claveBuena
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Clave: &buena}); err != nil {
		t.Fatal(err)
	}
	c, err := a.ElevenLabsDe(ctx, u.ID)
	if err != nil || bytes.Contains(c.ClaveCifrada, []byte(claveBuena)) || c.Voz != "voz1" || c.VozNombre != "Lucía" || !c.Leer ||
		c.TopeCaracteres != 5000 || c.TopeMinutos != 10 {
		t.Fatalf("cuenta: %+v %v", c, err)
	}
	e, err := i.EstadoVoz(ctx, u.ID, true)
	b, _ := json.Marshal(e)
	if err != nil || e["conectado"] != true || e["voz"] != "voz1" || len(e["voces"].([]VozEleven)) != 2 || strings.Contains(string(b), claveBuena) ||
		!strings.Contains(string(b), `"muestra":"https://storage.example/lucia.mp3"`) {
		t.Fatalf("estado: %s %v", b, err)
	}
	otraVoz := "voz-que-no-esta"
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Voz: &otraVoz}); !errors.Is(err, ErrVozElevenLabs) {
		t.Fatalf("voz ajena: %v", err)
	}
	mucho := MaxTopeCaracteres + 1
	var cv ErrCambioVoz
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Tope: &mucho}); !errors.As(err, &cv) {
		t.Fatalf("tope: %v", err)
	}

	// Lo que la nube le manda a decir: primero el audio, después el globo.
	if !i.TomarDecir(id, m, func() error { rs.anotar("cmd"); return nil }) {
		t.Fatal("con ElevenLabs y parlante, lee")
	}
	esperar(t, "el globo", func() bool { l := rs.lista(); return len(l) > 0 && l[len(l)-1] == "cmd" })
	l := rs.lista()
	if len(l) != 2 || !strings.HasPrefix(l[0], "audio ") {
		t.Fatalf("publicado: %q", l)
	}
	tr, err := protocolo.LeerTrozoAudio([]byte(strings.TrimPrefix(l[0], "audio ")))
	if err != nil || string(tr.Datos) != "ID3voz1:Hola Ana" || tr.CrcTexto != protocolo.Crc32([]byte("Hola Ana")) || tr.Origen != protocolo.OrigenDecir || tr.Total != 1 {
		t.Fatalf("trozo: %+v %v", tr, err)
	}
	if n, _ := a.CaracteresDeHoy(ctx, u.ID, fijo.In(time.FixedZone("AR", -3*3600))); n != len([]rune("Hola Ana")) {
		t.Fatalf("usados: %d", n)
	}
	// Un robot sin parlante no lee.
	if i.TomarDecir(otro, m, func() error { return nil }) {
		t.Fatal("sin parlante no lee")
	}
	// Con el tope de ElevenLabs lleno, lee la voz gratis.
	diez := 10
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Tope: &diez}); err != nil {
		t.Fatal(err)
	}
	antes := len(rs.lista())
	i.TomarDecir(id, m, func() error { rs.anotar("cmd"); return nil })
	esperar(t, "el globo con Edge", func() bool { return len(rs.lista()) == antes+2 })
	if tr, _ := protocolo.LeerTrozoAudio([]byte(strings.TrimPrefix(rs.lista()[antes], "audio "))); string(tr.Datos) != "ID3edge:es-AR-TomasNeural" ||
		rs.lista()[antes+1] != "cmd" {
		t.Fatalf("con el tope lleno: %q", rs.lista()[antes:])
	}
	cinco := 5000
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Tope: &cinco}); err != nil {
		t.Fatal(err)
	}

	// Un recordatorio que el robot muestra solo: se lee una vez.
	cfg := `{"version":2,"dueno":{"nombre":"Ana"},"recordatorios":[{"hora":"08:30","texto":"La pastilla, {dueno}"}]}`
	if _, err := a.NuevaConfig(ctx, id, cfg, protocolo.Crc32([]byte(cfg)), "web", &u.ID); err != nil {
		t.Fatal(err)
	}
	antes = len(rs.lista())
	i.leerGloboPropio(id, "recordatorio", map[string]any{"indice": 0})
	esperar(t, "el audio del recordatorio", func() bool { return len(rs.lista()) == antes+1 })
	tr, _ = protocolo.LeerTrozoAudio([]byte(strings.TrimPrefix(rs.lista()[antes], "audio ")))
	if string(tr.Datos) != "ID3voz1:La pastilla, Ana" || tr.Origen != protocolo.OrigenRecordatorio {
		t.Fatalf("recordatorio: %q %d", tr.Datos, tr.Origen)
	}
	i.leerGloboPropio(id, "recordatorio", map[string]any{"indice": 0})
	time.Sleep(100 * time.Millisecond)
	if len(rs.lista()) != antes+1 {
		t.Fatal("el recordatorio se repitió")
	}

	// Apagadas las dos, no lee; el aviso se lo dice al robot.
	no := false
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Leer: &no}); err != nil {
		t.Fatal(err)
	}
	if !i.TomarDecir(id, m, func() error { return nil }) {
		t.Fatal("sin leer de ElevenLabs, lee Edge")
	}
	if err := i.CambiarEdge(ctx, u.ID, CambiosEdge{Prendida: &no}); err != nil {
		t.Fatal(err)
	}
	if i.TomarDecir(id, m, func() error { return nil }) {
		t.Fatal("con las dos apagadas no lee")
	}
	esperar(t, "el aviso de voz", rs.avisoApagado)

	// Probar: el mp3.
	if mp3, err := i.ProbarVoz(ctx, u.ID, ""); err != nil || !strings.HasPrefix(string(mp3), "ID3voz1:¡Hola!") {
		t.Fatalf("probar: %q %v", mp3, err)
	}

	// La conversación: crea las herramientas y el agente; la segunda, igual.
	cv1, err := i.PedirConversacion(ctx, u.ID, id, "")
	if err != nil || !strings.HasPrefix(cv1.URL, "wss://") || cv1.Agente != "agent_1" || cv1.Segundos != MaxConversacion || cv1.Quedan != 600 ||
		cv1.Silencio != SilencioConversacion || len(cv1.Expresiones) == 0 {
		t.Fatalf("conversación: %+v %v", cv1, err)
	}
	if f.cuantos("POST /v1/convai/tools") != 2 || f.cuantos("POST /v1/convai/agents/create") != 1 {
		t.Fatalf("pedidos: %q", f.pedidos)
	}
	ag := f.cuerpos["POST /v1/convai/agents/create"][0]
	cc := ag["conversation_config"].(map[string]any)
	agente := cc["agent"].(map[string]any)
	prompt := agente["prompt"].(map[string]any)
	if agente["language"] != "es" || !strings.Contains(prompt["prompt"].(string), "Sos Pipo") || !strings.Contains(prompt["prompt"].(string), "Ana") ||
		len(prompt["tool_ids"].([]any)) != 2 || cc["tts"].(map[string]any)["voice_id"] != "voz1" ||
		cc["turn"].(map[string]any)["silence_end_call_timeout"] != float64(SilencioConversacion) ||
		ag["platform_settings"].(map[string]any)["auth"].(map[string]any)["enable_auth"] != true {
		t.Fatalf("agente: %v", ag)
	}
	if cv2, err := i.PedirConversacion(ctx, u.ID, id, "webrtc"); err != nil || cv2.Token != "tok_webrtc" || cv2.Agente != "agent_1" {
		t.Fatalf("webrtc: %+v %v", cv2, err)
	}
	if f.cuantos("PATCH") != 0 || f.cuantos("POST /v1/convai/agents/create") != 1 {
		t.Fatalf("sin cambios no se actualiza: %q", f.pedidos)
	}
	// Cambia la voz: se actualiza; si lo borraron allá, uno nuevo.
	v2 := "voz2"
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Voz: &v2}); err != nil {
		t.Fatal(err)
	}
	if _, err := i.PedirConversacion(ctx, u.ID, id, ""); err != nil || f.cuantos("PATCH /v1/convai/agents/agent_1") != 1 {
		t.Fatalf("PATCH: %v %q", err, f.pedidos)
	}
	f.sinAgente.Store(true)
	v1 := "voz1"
	quince := 15
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Voz: &v1, TopeMinutos: &quince}); err != nil {
		t.Fatal(err)
	}
	if cv3, err := i.PedirConversacion(ctx, u.ID, id, ""); err != nil || cv3.Agente != "agent_2" {
		t.Fatalf("agente nuevo: %+v %v", cv3, err)
	}
	f.sinAgente.Store(false)
	// El tope de minutos: con 14 min 30 s charlados quedan 30 s (pero el mínimo es 60); con 15, no.
	f.mu.Lock()
	f.segundos = 14*60 + 30
	f.mu.Unlock()
	i.olvidarCuentaEleven(u.ID)
	if cv4, err := i.PedirConversacion(ctx, u.ID, id, ""); err != nil || cv4.Segundos != 60 || cv4.Quedan != 30 {
		t.Fatalf("casi sin minutos: %+v %v", cv4, err)
	}
	f.mu.Lock()
	f.segundos = 15 * 60
	f.mu.Unlock()
	i.olvidarCuentaEleven(u.ID)
	if _, err := i.PedirConversacion(ctx, u.ID, id, ""); !errors.Is(err, ErrTopeMinutos) {
		t.Fatalf("sin minutos: %v", err)
	}

	// Por MQTT: la respuesta va a conversacion/url (acá, el error del tope).
	antes = len(rs.lista())
	i.ConversacionMQTT(id, []byte(`{"pedir":true}`))
	esperar(t, "la respuesta por MQTT", func() bool { return len(rs.lista()) > antes })
	if r := rs.lista()[antes]; !strings.HasPrefix(r, "conversacion/url ") || !strings.Contains(r, `"error"`) || strings.Contains(r, claveBuena) {
		t.Fatalf("MQTT: %q", r)
	}
	i.ConversacionMQTT(id, []byte(`no es json`))

	// Desconectar: se borra la clave y los agentes de allá.
	if err := i.DesconectarVoz(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if f.cuantos("DELETE /v1/convai/agents/agent_2") != 1 {
		t.Fatalf("borrar agentes: %q", f.pedidos)
	}
	if e, _ := i.EstadoVoz(ctx, u.ID, false); e["conectado"] != false {
		t.Fatalf("desconectado: %v", e)
	}
	if _, err := i.PedirConversacion(ctx, u.ID, id, ""); !errors.Is(err, ErrSinElevenLabs) {
		t.Fatalf("sin cuenta: %v", err)
	}
}
