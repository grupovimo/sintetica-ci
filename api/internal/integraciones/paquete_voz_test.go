package integraciones

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

func TestCompletarFrase(t *testing.T) {
	casos := []struct {
		p, dueno, robot, quiere string
		ok                      bool
	}{
		{"¡Hola, {dueno}!", "Ana", "Pipo", "¡Hola, Ana!", true},
		{"¡Hola, {dueno}!", "", "Pipo", "¡Hola!", true},
		{"{dueno}, ¿me cargás?", "", "Pipo", "", false},
		{"Soy {robot}, {dueno}.", "", "Pipo", "Soy Pipo.", true},
		{"Hace {clima}.", "Ana", "Pipo", "", false},
		{"Un mail de {de}.", "Ana", "Pipo", "", false},
		{"Llaves {} raras", "Ana", "Pipo", "Llaves {} raras", true},
		{strings.Repeat("á", 40), "", "", strings.Repeat("á", 30), true}, // 60 bytes sin partir letras
	}
	if h := HashFrase("¡Hola, Ana!"); h != "b34a676f" { // el ejemplo de docs/protocolo-ble.md
		t.Fatalf("hash: %s", h)
	}
	for _, c := range casos {
		got, ok := completarFrase(c.p, c.dueno, c.robot)
		if ok != c.ok || got != c.quiere {
			t.Errorf("%q: %q %v", c.p, got, ok)
		}
	}
}

func TestFrasesDelRobot(t *testing.T) {
	cfg := `{"version":2,"nombre":"Pipo","personalidad":{"preset":"grunon"},"dueno":{"nombre":"Ana"},
		"habla":{"propias":{"despertar":["¡Arriba, {dueno}! Soy {robot}."],"caricia":["Más, más."]}},
		"despertador":{"hora":"07:00","texto":"¡Arriba, dormilona!"},"recordatorios":[{"hora":"10:00","texto":"Tomar agua"}]}`
	fs := FrasesDelRobot(cfg, "OTRO")
	if len(fs) < 50 {
		t.Fatalf("pocas: %d", len(fs))
	}
	hashes := map[string]bool{}
	idx := map[string]int{}
	for k, f := range fs {
		if strings.Contains(f.Texto, "{") || len(f.Texto) > protocolo.MaxDecir || f.Hash != HashFrase(f.Texto) || hashes[f.Hash] {
			t.Fatalf("frase %d: %+v", k, f)
		}
		hashes[f.Hash] = true
		if _, ya := idx[f.Situacion]; !ya {
			idx[f.Situacion] = k
		}
		if !f.Propia && (f.Situacion == "despertar" || f.Situacion == "caricia") {
			t.Fatalf("con propias, las de fábrica de %s no van: %+v", f.Situacion, f)
		}
		if f.Preset != "" && f.Preset != "grunon" {
			t.Fatalf("de otro preset: %+v", f)
		}
	}
	// Primero las frecuentes, después las propias, después el resto.
	if fs[0].Situacion != "saludo" || idx["buenas-noches"] > idx["recordatorio"] || idx["recordatorio"] > idx["gane"] {
		t.Fatalf("orden: %v (primera %+v)", idx, fs[0])
	}
	quiere := map[string]bool{"¡Arriba, Ana! Soy Pipo.": false, "Más, más.": false, "¡Arriba, dormilona!": false, "Tomar agua": false,
		"Mmm. Buen día.": true /* de despertar: no va (tiene propias) */}
	for _, f := range fs {
		if _, ok := quiere[f.Texto]; ok {
			quiere[f.Texto] = !quiere[f.Texto]
		}
	}
	for k, v := range quiere {
		if !v {
			t.Errorf("%q: mal", k)
		}
	}
	// En una situación, las de su preset primero.
	var bn []FrasePaquete
	for _, f := range fs {
		if f.Situacion == "buenas-noches" {
			bn = append(bn, f)
		}
	}
	if len(bn) < 2 || bn[0].Preset != "grunon" || bn[0].Texto != "Por fin, a dormir." {
		t.Fatalf("buenas noches: %+v", bn)
	}
	// Sin dueño: las que empiezan con {dueno} no van y "¡Hola, {dueno}!" queda "¡Hola!".
	sin := FrasesDelRobot(`{"version":2}`, "Pipo")
	hola := false
	for _, f := range sin {
		hola = hola || f.Texto == "¡Hola!"
		if f.Preset != "" {
			t.Fatalf("equilibrado no tiene frases propias: %+v", f)
		}
	}
	if !hola {
		t.Fatal("sin dueño, ¡Hola!")
	}
	// Los rasgos mandan: un preset equilibrado con mucho mal genio es gruñón.
	if p := presetCercano("equilibrado", map[string]float64{"malGenio": 0.95, "sociabilidad": 0.2}); p != "grunon" {
		t.Fatalf("cercano: %s", p)
	}
}

// urlDe: lo último que se publicó en voz/paquete/url.
func urlDe(t *testing.T, rs *robotsConParlante, n int) map[string]any {
	t.Helper()
	var m map[string]any
	esperar(t, "la respuesta del paquete", func() bool {
		k := 0
		for _, p := range rs.lista() {
			if s, ok := strings.CutPrefix(p, "voz/paquete/url "); ok {
				k++
				if k == n {
					json.Unmarshal([]byte(s), &m) //nolint:errcheck
					return true
				}
			}
		}
		return false
	})
	return m
}

func esperarPaquete(t *testing.T, i *Integraciones, bicho string) {
	t.Helper()
	esperar(t, "que termine el paquete", func() bool {
		i.paq.mu.Lock()
		defer i.paq.mu.Unlock()
		_, ya := i.paq.enCurso[bicho]
		return !ya
	})
}

func TestPaqueteVoz(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	u, id := usuarioConBicho(t, a)
	_, ajeno := usuarioConBicho(t, a)
	cfg := `{"version":2,"nombre":"Pipo","personalidad":{"preset":"carinoso"},"dueno":{"nombre":"Ana"}}`
	if _, err := a.NuevaConfig(ctx, id, cfg, 0, "web", &u.ID); err != nil {
		t.Fatal(err)
	}
	rs := &robotsConParlante{robotsFalsos: robotsFalsos{duenos: map[string]string{id: u.ID},
		estados: map[string]protocolo.Estado{id: {Perfil: "dev-147b", Nombre: "Pipo"}}}}
	var edgeFalla atomic.Bool
	ed := edgeFalso(t, &edgeFalla)
	f := &elevenFalso{}
	el := f.servidor(t)
	i := Nuevo(Opciones{V19: OpcionesV19{Secreto: []byte("secreto de prueba 1234"), APIURL: "https://api.ejemplo.com", ElevenLabsURL: el.URL,
		EdgeURL: "ws" + strings.TrimPrefix(ed.URL, "http") + "/edge/v1", EdgeVocesURL: ed.URL + "/voces"}}, a, rs, eventos.Nuevo(), sinLog)
	i.paq.pausa = 0

	// Sin generar: lo que falta, con la voz gratis (no gasta).
	e, err := i.EstadoPaqueteVoz(ctx, u.ID, id, 0)
	if err != nil || e["estado"] != PaqueteNuevo || e["proveedor"] != "edge" || e["faltan"] != e["total"] || e["total"].(int) < 50 || e["gasta"] != 0 {
		t.Fatalf("estado: %v %v", e, err)
	}
	total := e["total"].(int)
	if _, err := i.GenerarPaqueteVoz(ctx, u.ID, ajeno, false, 0); err == nil {
		t.Fatal("el robot de otro")
	}
	if _, err := i.GenerarPaqueteVoz(ctx, u.ID, id, false, 0); err != nil {
		t.Fatal(err)
	}
	esperarPaquete(t, i, id)
	e, _ = i.EstadoPaqueteVoz(ctx, u.ID, id, 0)
	if e["estado"] != PaqueteListo || e["version"] != 1 || e["frases"] != total || e["faltan"] != 0 || e["voz"] != "edge:es-AR-ElenaNeural" {
		t.Fatalf("listo: %v", e)
	}
	usados, _ := a.CaracteresPaqueteDeHoy(ctx, u.ID, i.ahora().In(time.FixedZone("AR", -3*3600)))
	if n, _ := a.CaracteresEdgeDeHoy(ctx, u.ID, i.ahora().In(time.FixedZone("AR", -3*3600))); usados == 0 || n != 0 {
		t.Fatalf("los caracteres del paquete van aparte: %d %d", usados, n)
	}

	// El robot lo pide por MQTT con un tope chico: entra solo una parte.
	cada := len("ID3edge:es-AR-ElenaNeural")
	tope := cada * 10
	i.PaqueteVozMQTT(id, []byte(`{"pedir":true,"tope":`+itoa(tope)+`}`))
	m := urlDe(t, rs, 1)
	if m["version"] != 1.0 || m["frases"] != 10.0 || m["bytes"] != float64(tope) || m["formato"] != 1.0 {
		t.Fatalf("url: %v", m)
	}
	lu, _ := url.Parse(m["url"].(string))
	if !strings.HasPrefix(m["url"].(string), "https://api.ejemplo.com/v1/voz/paquete/"+id+"?") {
		t.Fatalf("url: %s", lu)
	}
	p, err := i.AbrirPaqueteVoz(ctx, id, lu.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Indice.Frases) != 10 || len(p.Indice.Fuera) != total-10 || p.Indice.Bytes != tope || p.Indice.Frases[0].Situacion != "despertar" {
		t.Fatalf("índice: %d %d %d %+v", len(p.Indice.Frases), len(p.Indice.Fuera), p.Indice.Bytes, p.Indice.Frases[0])
	}
	var tb bytes.Buffer
	if err := p.EscribirTar(&tb); err != nil || int64(tb.Len()) != p.Tamano() {
		t.Fatalf("tar: %v %d %d", err, tb.Len(), p.Tamano())
	}
	tr := tar.NewReader(&tb)
	h, _ := tr.Next()
	var ind IndicePaquete
	if h == nil || h.Name != "indice.json" || json.NewDecoder(tr).Decode(&ind) != nil || ind.Version != 1 {
		t.Fatalf("indice.json: %v", h)
	}
	for k := 0; ; k++ {
		h, err := tr.Next()
		if err == io.EOF {
			if k != 10 {
				t.Fatalf("mp3: %d", k)
			}
			break
		}
		d, _ := io.ReadAll(tr)
		if h.Name != ind.Frases[k].Hash+".mp3" || string(d) != "ID3edge:es-AR-ElenaNeural" ||
			ind.Frases[k].Hash != HashFrase(ind.Frases[k].Texto) {
			t.Fatalf("%d: %s %q", k, h.Name, d)
		}
	}
	// Una firma tocada, otro robot u otro tope: no. Vencida: tampoco.
	for _, cambio := range []func(q url.Values){
		func(q url.Values) { q.Set("firma", strings.Repeat("0", 32)) },
		func(q url.Values) { q.Set("tope", "0") },
		func(q url.Values) { q.Set("vence", itoa(int(time.Now().Add(time.Hour).Unix()))) },
	} {
		q := lu.Query()
		cambio(q)
		if _, err := i.AbrirPaqueteVoz(ctx, id, q); !errors.Is(err, ErrFirmaPaquete) {
			t.Errorf("firma: %v", err)
		}
	}
	if _, err := i.AbrirPaqueteVoz(ctx, ajeno, lu.Query()); !errors.Is(err, ErrFirmaPaquete) {
		t.Fatalf("ajeno: %v", err)
	}
	i.v19.ahora = func() time.Time { return time.Now().Add(VencePaquete + time.Minute) }
	if _, err := i.AbrirPaqueteVoz(ctx, id, lu.Query()); !errors.Is(err, ErrFirmaPaquete) {
		t.Fatalf("vencida: %v", err)
	}
	i.v19.ahora = time.Now

	// Cambia una frase propia: se genera solo esa y sube la versión.
	cfg2 := strings.Replace(cfg, `"dueno"`, `"habla":{"propias":{"agua":["¡A tomar agüita, {dueno}!"]}},"dueno"`, 1)
	if _, err := a.NuevaConfig(ctx, id, cfg2, 0, "web", &u.ID); err != nil {
		t.Fatal(err)
	}
	e, _ = i.EstadoPaqueteVoz(ctx, u.ID, id, 0)
	if e["faltan"] != 1 || e["caracteres"] != len([]rune("¡A tomar agüita, Ana!")) {
		t.Fatalf("lo que cambió: %v", e)
	}
	i.limite = nuevoLimitador()
	i.PaqueteVozMQTT(id, []byte(`{"pedir":true}`)) // con Edge lo genera solo y manda la URL al terminar
	if m := urlDe(t, rs, 2); m["estado"] != PaqueteGenerando || m["faltan"] != 1.0 {
		t.Fatalf("generando: %v", m)
	}
	m = urlDe(t, rs, 3)
	esperarPaquete(t, i, id)
	if m["version"] != 2.0 || m["url"] == nil {
		t.Fatalf("la URL al terminar: %v", m)
	}
	ahora, _ := a.CaracteresPaqueteDeHoy(ctx, u.ID, i.ahora().In(time.FixedZone("AR", -3*3600)))
	if ahora-usados != len([]rune("¡A tomar agüita, Ana!")) {
		t.Fatalf("regeneró de más: %d", ahora-usados)
	}

	// Con ElevenLabs: gasta de su cuenta, así que lo pide el dueño.
	buena := claveBuena
	if err := i.ConectarVoz(ctx, u.ID, CambiosVoz{Clave: &buena}); err != nil {
		t.Fatal(err)
	}
	i.limite = nuevoLimitador()
	i.PaqueteVozMQTT(id, []byte(`{"pedir":true}`))
	if m := urlDe(t, rs, 4); m["estado"] != "falta-pedir" || m["gasta"].(float64) <= 0 {
		t.Fatalf("eleven por MQTT: %v", m)
	}
	_, err = i.GenerarPaqueteVoz(ctx, u.ID, id, false, 0)
	var conf ErrConfirmarPaquete
	if !errors.As(err, &conf) || conf.Estado["proveedor"] != "elevenlabs" || conf.Estado["gasta"].(int) <= 0 {
		t.Fatalf("sin confirmar: %v", err)
	}
	gasta := conf.Estado["gasta"].(int)
	antes := f.cuantos("POST /v1/text-to-speech/")
	if _, err := i.GenerarPaqueteVoz(ctx, u.ID, id, true, 0); err != nil {
		t.Fatal(err)
	}
	esperarPaquete(t, i, id)
	e, _ = i.EstadoPaqueteVoz(ctx, u.ID, id, 0)
	n, _ := a.CaracteresDeHoy(ctx, u.ID, i.ahora().In(time.FixedZone("AR", -3*3600)))
	if f.cuantos("POST /v1/text-to-speech/") == antes || e["voz"] != "elevenlabs:voz1" || e["version"] != 3 {
		t.Fatalf("eleven: %v", e)
	}
	// Su tope de caracteres (5000 por defecto) se respeta: si no entra todo, parcial.
	if gasta > TopeCaracteresPorDefecto {
		if e["estado"] != PaqueteParcial || n > TopeCaracteresPorDefecto {
			t.Fatalf("tope de eleven: %v %d", e, n)
		}
	} else if e["estado"] != PaqueteListo || n != gasta {
		t.Fatalf("eleven listo: %v %d %d", e, n, gasta)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }
