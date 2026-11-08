package servidor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

const (
	webPrueba = "http://web.prueba"
	apiPrueba = "http://api.prueba"
)

// Necesita un Postgres: SINTETICA_PRUEBAS_DB=postgres://…
func servidorMCPDePrueba(t *testing.T) (*Servidor, *almacen.Almacen, http.Handler) {
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
	var salida io.Writer = io.Discard
	if os.Getenv("MCP_LOG") != "" {
		salida = os.Stderr
	}
	log := slog.New(slog.NewTextHandler(salida, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{}, alm, n, hub, log)
	n.Escuchar(i)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm, n, i, hub, nil, nil, log)
	return s, alm, s.Rutas()
}

// usuarioMCP: un usuario con dos bichos (uno de 1.47B y un broche), una
// sesión web y un token personal del MCP.
type usuarioMCP struct {
	id, cookie, token string
	bichos            []string
}

func nuevoUsuarioMCP(t *testing.T, alm *almacen.Almacen) usuarioMCP {
	t.Helper()
	ctx := context.Background()
	u, err := alm.UsuarioPorEmail(ctx, strings.ToLower(azarHex(4))+"@ejemplo.com", false)
	if err != nil {
		t.Fatal(err)
	}
	r := usuarioMCP{id: u.ID, cookie: aleatorio(24), token: "sin_pat_" + aleatorio(24)}
	if err := alm.CrearSesion(ctx, hash(r.cookie), u.ID, "web", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := alm.CrearTokenPersonal(ctx, u.ID, "Claude Code", hash(r.token)); err != nil {
		t.Fatal(err)
	}
	for _, perfil := range []string{"dev-147b", "broche-c3"} {
		id := azarHex(6)
		alm.AsegurarBicho(ctx, id, perfil, false)                                        //nolint:errcheck
		alm.CrearCredencialPendiente(ctx, id, u.ID, hash(id), time.Now().Add(time.Hour)) //nolint:errcheck
		if acc, err := alm.UsarCredencial(ctx, id, hash(id)); err != nil || !acc.Ok {
			t.Fatal("no pude sumar el bicho")
		}
		r.bichos = append(r.bichos, id)
	}
	return r
}

// rpc hace un pedido JSON-RPC a /mcp con el token.
func rpc(t *testing.T, h http.Handler, token, metodo string, params any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": metodo, "params": params})
	r := httptest.NewRequest("POST", "/mcp", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
	if w.Code == 200 && v == nil {
		t.Fatalf("%s: respuesta rara: %s", metodo, w.Body.String())
	}
	return w.Code, v
}

type resultadoHerramienta struct {
	error   bool
	texto   string
	imagen  []byte
	crudo   map[string]any
	estado  int
	rpcErr  any
	content []any
}

func llamar(t *testing.T, h http.Handler, token, nombre string, args map[string]any) resultadoHerramienta {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	cod, v := rpc(t, h, token, "tools/call", map[string]any{"name": nombre, "arguments": args})
	res := resultadoHerramienta{estado: cod, crudo: v, rpcErr: v["error"]}
	r, _ := v["result"].(map[string]any)
	if r == nil {
		return res
	}
	res.error, _ = r["isError"].(bool)
	res.content, _ = r["content"].([]any)
	for _, c := range res.content {
		m := c.(map[string]any)
		switch m["type"] {
		case "text":
			res.texto += m["text"].(string) + "\n"
		case "image":
			res.imagen, _ = base64.StdEncoding.DecodeString(m["data"].(string))
		}
	}
	return res
}

func TestMCPSinToken(t *testing.T) {
	_, _, h := servidorMCPDePrueba(t)
	cod, _ := rpc(t, h, "", "tools/list", map[string]any{})
	if cod != http.StatusUnauthorized {
		t.Fatalf("sin token: %d", cod)
	}
	r := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), apiPrueba+"/.well-known/oauth-protected-resource") {
		t.Fatalf("WWW-Authenticate: %q", w.Header().Get("WWW-Authenticate"))
	}
	// Un token de sesión de la app no sirve para el MCP.
	if cod, _ := rpc(t, h, "sin_pat_inventado", "tools/list", map[string]any{}); cod != http.StatusUnauthorized {
		t.Fatalf("token inventado: %d", cod)
	}
}

func TestMCPHerramientas(t *testing.T) {
	s, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	otro := nuevoUsuarioMCP(t, alm)
	ctx := context.Background()
	sub := s.hub.Suscribir(u.id, "")
	defer s.hub.Desuscribir(sub)

	cod, v := rpc(t, h, u.token, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "prueba", "version": "1"}})
	if cod != 200 || v["result"] == nil {
		t.Fatalf("initialize: %d %v", cod, v)
	}
	_, v = rpc(t, h, u.token, "tools/list", map[string]any{})
	herr, _ := v["result"].(map[string]any)["tools"].([]any)
	nombres := map[string]bool{}
	for _, x := range herr {
		nombres[x.(map[string]any)["name"].(string)] = true
	}
	for _, n := range []string{"listar_robots", "ver_robot", "poner_cara", "disparar_evento", "reproducir_comportamiento", "crear_expresion",
		"listar_expresiones", "dibujar_cara", "poner_imagen", "recordatorios", "personalidad", "horario", "modo", "reflejos", "automatizaciones", "mandar_telegram", "agenda", "musica", "voz"} {
		if !nombres[n] {
			t.Errorf("falta la herramienta %s", n)
		}
	}

	// listar_robots: los dos suyos, ninguno ajeno.
	r := llamar(t, h, u.token, "listar_robots", nil)
	if r.error || !strings.Contains(r.texto, u.bichos[0]) || !strings.Contains(r.texto, u.bichos[1]) || strings.Contains(r.texto, otro.bichos[0]) {
		t.Fatalf("listar_robots: %+v", r.texto)
	}
	// Con dos robots, hay que decir cuál.
	if r := llamar(t, h, u.token, "ver_robot", nil); !r.error || !strings.Contains(r.texto, "decime cuál") {
		t.Fatalf("ver_robot sin robot: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "ver_robot", map[string]any{"robot": strings.ToLower(u.bichos[1])}); r.error || !strings.Contains(r.texto, `"redonda":true`) {
		t.Fatalf("ver_robot broche: %v", r.texto)
	}
	// Un robot ajeno: como si no existiera, y no se toca.
	for _, n := range []string{"ver_robot", "crear_expresion", "recordatorios"} {
		args := map[string]any{"robot": otro.bichos[0], "nombre": "x", "ojo": map[string]any{"w": 0.1}, "accion": "listar"}
		if n == "ver_robot" {
			args = map[string]any{"robot": otro.bichos[0]}
		} else if n == "recordatorios" {
			args = map[string]any{"robot": otro.bichos[0], "accion": "listar"}
		} else {
			delete(args, "accion")
		}
		if r := llamar(t, h, u.token, n, args); !r.error || !strings.Contains(r.texto, "No encontré") {
			t.Fatalf("%s con un robot ajeno: %v %v", n, r.texto, r.crudo)
		}
	}
	if _, err := alm.UltimaConfig(ctx, otro.bichos[0]); err == nil {
		t.Fatal("se guardó una config en un robot ajeno")
	}

	// crear_expresion inválida: fuera de rango (el esquema) y algo que el núcleo no acepta.
	r = llamar(t, h, u.token, "crear_expresion", map[string]any{"robot": u.bichos[0], "nombre": "mala", "ojo": map[string]any{"w": 9}})
	if !r.error || !strings.Contains(r.texto, "w") {
		t.Fatalf("w = 9: %v", r.texto)
	}
	r = llamar(t, h, u.token, "crear_expresion", map[string]any{"robot": u.bichos[0], "nombre": "mala", "extiende": "no-existe", "gx": 0.1})
	if !r.error || !strings.Contains(r.texto, "no aceptaría") {
		t.Fatalf("extiende inexistente: %v", r.texto)
	}
	r = llamar(t, h, u.token, "crear_expresion", map[string]any{"robot": u.bichos[0], "nombre": "mala", "brillo": 1})
	if !r.error {
		t.Fatalf("propiedad desconocida: %v", r.texto)
	}
	// solo_ver: imagen sin guardar.
	r = llamar(t, h, u.token, "crear_expresion", map[string]any{"robot": u.bichos[0], "nombre": "tierna", "ojo": map[string]any{"w": 0.14, "h": 0.2, "smile": 0.7}, "blush": 0.8, "solo_ver": true})
	if r.error || len(r.imagen) == 0 {
		t.Fatalf("solo_ver: %v", r.texto)
	}
	if _, err := alm.UltimaConfig(ctx, u.bichos[0]); err == nil {
		t.Fatal("solo_ver guardó la config")
	}
	img, err := png.Decode(bytes.NewReader(r.imagen))
	if err != nil || img.Bounds().Dx() != 640 || img.Bounds().Dy() != 344 {
		t.Fatalf("la imagen no es un PNG de 640 × 344: %v %v", err, img)
	}
	// Ahora sí: se guarda (autor mcp:<cliente>) y avisa por los eventos.
	r = llamar(t, h, u.token, "crear_expresion", map[string]any{"robot": u.bichos[0], "nombre": "tierna", "ojo": map[string]any{"w": 0.14, "h": 0.2, "smile": 0.7}, "blush": 0.8, "particula": "corazon"})
	if r.error || len(r.imagen) == 0 || !strings.Contains(r.texto, "Guardé") {
		t.Fatalf("crear_expresion: %v", r.texto)
	}
	c, err := alm.UltimaConfig(ctx, u.bichos[0])
	if err != nil || c.Autor != "mcp:Claude Code" || !strings.Contains(c.Texto, `"tierna"`) {
		t.Fatalf("config: %+v %v", c, err)
	}
	vistos := map[string]bool{}
	hasta := time.After(2 * time.Second)
	for !(vistos["config"] && vistos["mcp"]) {
		select {
		case e := <-sub.C:
			vistos[e.Tipo] = true
			if e.Tipo == "mcp" {
				d := e.Datos.(map[string]any)
				if d["cliente"] != "Claude Code" || d["herramienta"] != "crear_expresion" || e.Bicho != u.bichos[0] {
					t.Fatalf("evento mcp: %+v", e)
				}
			}
		case <-hasta:
			t.Fatalf("no llegaron los eventos: %v", vistos)
		}
	}
	// listar_expresiones la muestra; poner_cara con el robot desconectado avisa.
	if r := llamar(t, h, u.token, "listar_expresiones", map[string]any{"robot": u.bichos[0]}); r.error || !strings.Contains(r.texto, "tierna") || !strings.Contains(r.texto, "neutral") {
		t.Fatalf("listar_expresiones: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "poner_cara", map[string]any{"robot": u.bichos[0], "expresion": "tierna"}); !r.error || !strings.Contains(r.texto, "no está conectado") {
		t.Fatalf("poner_cara desconectado: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "poner_cara", map[string]any{"robot": u.bichos[0], "expresion": "inexistente"}); !r.error || !strings.Contains(r.texto, "no tiene la expresión") {
		t.Fatalf("poner_cara inexistente: %v", r.texto)
	}

	// Recordatorios.
	b0 := map[string]any{"robot": u.bichos[0]}
	con := func(m map[string]any) map[string]any {
		n := map[string]any{}
		for k, v := range b0 {
			n[k] = v
		}
		for k, v := range m {
			n[k] = v
		}
		return n
	}
	if r := llamar(t, h, u.token, "recordatorios", con(map[string]any{"accion": "crear", "recordatorio": map[string]any{"hora": "08:30", "dias": "LMXJV", "texto": "Tomar agua", "expresion": "tierna"}})); r.error {
		t.Fatalf("crear recordatorio: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "recordatorios", con(map[string]any{"accion": "crear", "recordatorio": map[string]any{"hora": "25:00", "texto": "x"}})); !r.error {
		t.Fatal("aceptó la hora 25:00")
	}
	if r := llamar(t, h, u.token, "recordatorios", con(map[string]any{"accion": "listar"})); r.error || !strings.Contains(r.texto, "Tomar agua") {
		t.Fatalf("listar recordatorios: %v", r.texto)
	}
	// Personalidad, horario, modo, reflejos.
	for _, p := range []struct {
		h    string
		args map[string]any
		ver  string
	}{
		{"personalidad", map[string]any{"preset": "picaro", "timidez": 0.2}, `"picaro"`},
		{"horario", map[string]any{"despierta": "07:15"}, `"07:15"`},
		{"modo", map[string]any{"modo": "pomodoro", "pomodoro": map[string]any{"trabajo": 50}}, `"pomodoro"`},
		{"reflejos", map[string]any{"accion": "agregar", "si": "caricia", "reflejo": "tierna", "ademas": []string{"estado.animo+0.2"}}, `reflejo:tierna`},
	} {
		if r := llamar(t, h, u.token, p.h, con(p.args)); r.error {
			t.Fatalf("%s: %v", p.h, r.texto)
		}
		c, _ := alm.UltimaConfig(ctx, u.bichos[0])
		if !strings.Contains(c.Texto, p.ver) {
			t.Fatalf("%s: la config no tiene %s: %s", p.h, p.ver, c.Texto)
		}
	}
	if r := llamar(t, h, u.token, "recordatorios", con(map[string]any{"accion": "borrar", "indice": 0})); r.error {
		t.Fatalf("borrar recordatorio: %v", r.texto)
	}
	// El orden de la config se mantiene (expresiones antes que recordatorios, etc.).
	c, _ = alm.UltimaConfig(ctx, u.bichos[0])
	if strings.Contains(c.Texto, "Tomar agua") || strings.Index(c.Texto, `"version"`) != 1 {
		t.Fatalf("config final: %s", c.Texto)
	}

	// Imágenes: un PNG en el broche (redondo) y un GIF animado en la 1.47B.
	var pb bytes.Buffer
	foto := image.NewNRGBA(image.Rect(0, 0, 50, 40))
	for i := range foto.Pix {
		foto.Pix[i] = 200
	}
	png.Encode(&pb, foto) //nolint:errcheck
	r = llamar(t, h, u.token, "poner_imagen", map[string]any{"robot": u.bichos[1], "base64": base64.StdEncoding.EncodeToString(pb.Bytes()), "protector": true, "espera": 30})
	if r.error || len(r.imagen) == 0 {
		t.Fatalf("poner_imagen png: %v", r.texto)
	}
	im, err := alm.LeerImagen(ctx, u.bichos[1], 0)
	if w, hh, ok := protocolo.TamanoImagen(im.Datos); err != nil || !ok || w != 240 || hh != 240 {
		t.Fatalf("la imagen del broche: %d × %d %v %v", w, hh, ok, err)
	}
	c, _ = alm.UltimaConfig(ctx, u.bichos[1])
	if !strings.Contains(c.Texto, `"protector":{"espera":30,"imagen":0}`) {
		t.Fatalf("protector: %s", c.Texto)
	}
	g := &gif.GIF{}
	for k := 0; k < 3; k++ {
		p := image.NewPaletted(image.Rect(0, 0, 30, 20), palette.Plan9)
		for i := range p.Pix {
			p.Pix[i] = uint8(k * 40)
		}
		g.Image, g.Delay = append(g.Image, p), append(g.Delay, 10)
	}
	var gb bytes.Buffer
	gif.EncodeAll(&gb, g) //nolint:errcheck
	r = llamar(t, h, u.token, "poner_imagen", map[string]any{"robot": u.bichos[0], "base64": base64.StdEncoding.EncodeToString(gb.Bytes()), "bahia": 3, "lista": true})
	if r.error || !strings.Contains(r.texto, "3 cuadros") {
		t.Fatalf("poner_imagen gif: %v", r.texto)
	}
	im, _ = alm.LeerImagen(ctx, u.bichos[0], 3)
	if string(im.Datos[:4]) != "sani" {
		t.Fatal("el GIF no quedó como animación")
	}
	c, _ = alm.UltimaConfig(ctx, u.bichos[0])
	if !strings.Contains(c.Texto, `"lista":["imagen:3"]`) {
		t.Fatalf("lista: %s", c.Texto)
	}
	if r := llamar(t, h, u.token, "poner_imagen", map[string]any{"robot": u.bichos[0], "base64": "bm8gc295IHVuYSBpbWFnZW4="}); !r.error || !strings.Contains(r.texto, "PNG, JPEG o GIF") {
		t.Fatalf("imagen mala: %v", r.texto)
	}
	// Una URL a la red interna no se baja.
	if r := llamar(t, h, u.token, "poner_imagen", map[string]any{"robot": u.bichos[0], "url": "https://169.254.169.254/latest/meta-data"}); !r.error || !strings.Contains(r.texto, "interna") {
		t.Fatalf("url interna: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "borrar_imagen", map[string]any{"robot": u.bichos[0], "bahia": 3}); r.error {
		t.Fatalf("borrar_imagen: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "borrar_expresion", map[string]any{"robot": u.bichos[0], "nombre": "tierna"}); !r.error || !strings.Contains(r.texto, "no aceptaría") {
		// el reflejo la usa: el núcleo no deja borrarla
		t.Logf("borrar_expresion con un reflejo que la usa: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "automatizaciones", con(map[string]any{"accion": "listar"})); r.error || !strings.Contains(r.texto, "cuandos") {
		t.Fatalf("automatizaciones: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "mandar_telegram", map[string]any{"texto": "hola"}); !r.error || !strings.Contains(r.texto, "apagado") {
		t.Fatalf("telegram apagado: %v", r.texto)
	}
	// El recurso con la guía.
	_, v = rpc(t, h, u.token, "resources/read", map[string]any{"uri": uriGuia})
	if !strings.Contains(jsonDe(v), "Párpado") {
		t.Fatalf("la guía: %v", v)
	}
}

func TestTokensPersonalesHTTP(t *testing.T) {
	_, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	web := func(metodo, ruta, cuerpo string) (int, map[string]any) {
		r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
		r.AddCookie(&http.Cookie{Name: cookieSesion, Value: u.cookie})
		r.Header.Set("X-Sintetica", "1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
		return w.Code, v
	}
	cod, v := web("POST", "/v1/mcp/tokens", `{"nombre":"Mi Claude Code"}`)
	tok, _ := v["token"].(string)
	if cod != 201 || !strings.HasPrefix(tok, "sin_pat_") || v["url"] != apiPrueba+"/mcp" {
		t.Fatalf("crear: %d %v", cod, v)
	}
	if r := llamar(t, h, tok, "listar_robots", nil); r.error || r.estado != 200 {
		t.Fatalf("el token nuevo no anda: %d %v", r.estado, r.texto)
	}
	_, v = web("GET", "/v1/mcp/tokens", "")
	if l, _ := v["tokens"].([]any); len(l) != 2 || strings.Contains(jsonDe(v), "sin_pat_") {
		t.Fatalf("listar: %v", v)
	}
	if cod, _ := web("DELETE", "/v1/mcp/tokens/"+v["tokens"].([]any)[0].(map[string]any)["id"].(string), ""); cod != 204 {
		t.Fatalf("borrar: %d", cod)
	}
	if cod, _ := rpc(t, h, tok, "tools/list", map[string]any{}); cod != http.StatusUnauthorized {
		t.Fatalf("token borrado: %d", cod)
	}
	// Uno ajeno no se puede borrar; con un token del MCP no se pueden crear tokens.
	otro := nuevoUsuarioMCP(t, alm)
	_, v = web("GET", "/v1/mcp/tokens", "")
	id := v["tokens"].([]any)[0].(map[string]any)["id"].(string)
	r := httptest.NewRequest("DELETE", "/v1/mcp/tokens/"+id, nil)
	r.AddCookie(&http.Cookie{Name: cookieSesion, Value: otro.cookie})
	r.Header.Set("X-Sintetica", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("borrar uno ajeno: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "/v1/mcp/tokens", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+u.token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("crear tokens con un token del MCP: %d", w.Code)
	}
}

// --- OAuth ---

var reCSRF = regexp.MustCompile(`name="csrf" value="([0-9a-f]+)"`)

func TestOAuth(t *testing.T) {
	_, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	pedirH := func(r *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	jsonDeW := func(w *httptest.ResponseRecorder) map[string]any {
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
		return v
	}

	// Metadatos.
	w := pedirH(httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil))
	if v := jsonDeW(w); v["resource"] != apiPrueba+"/mcp" || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("recurso: %v", v)
	}
	w = pedirH(httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	meta := jsonDeW(w)
	if meta["issuer"] != apiPrueba || meta["authorization_endpoint"] != webPrueba+"/api/oauth/autorizar" || !strings.Contains(jsonDe(meta["code_challenge_methods_supported"]), "S256") {
		t.Fatalf("autorización: %v", meta)
	}

	// Registro dinámico.
	registrar := func(cuerpo string) (int, map[string]any) {
		r := httptest.NewRequest("POST", "/oauth/registrar", strings.NewReader(cuerpo))
		r.Header.Set("Content-Type", "application/json")
		w := pedirH(r)
		return w.Code, jsonDeW(w)
	}
	for _, malo := range []string{`{"redirect_uris":[]}`, `{"redirect_uris":["http://evil.com/cb"]}`, `{"redirect_uris":["javascript:alert(1)"]}`, `{"redirect_uris":["https://x.com/cb#f"]}`} {
		if cod, _ := registrar(malo); cod != 400 {
			t.Fatalf("registró %s: %d", malo, cod)
		}
	}
	redir := "https://claude.ai/api/mcp/auth_callback"
	cod, cli := registrar(`{"client_name":"Claude","redirect_uris":["` + redir + `","http://localhost:33418/callback"],"token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"]}`)
	clienteID, _ := cli["client_id"].(string)
	if cod != 201 || clienteID == "" || cli["client_secret"] != nil {
		t.Fatalf("registro: %d %v", cod, cli)
	}

	verificador := "un-verificador-de-prueba-bien-largo-y-al-azar-" + azarHex(8)
	sum := sha256.Sum256([]byte(verificador))
	desafio := base64.RawURLEncoding.EncodeToString(sum[:])
	pedido := func(redireccion string) url.Values {
		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", clienteID)
		q.Set("redirect_uri", redireccion)
		q.Set("code_challenge", desafio)
		q.Set("code_challenge_method", "S256")
		q.Set("state", "estado123")
		q.Set("scope", "robots")
		q.Set("resource", apiPrueba+"/mcp")
		return q
	}
	conCookie := func(r *http.Request) *http.Request {
		r.AddCookie(&http.Cookie{Name: cookieSesion, Value: u.cookie})
		return r
	}
	// Sin sesión: a entrar en la web, y vuelve al mismo pedido.
	w = pedirH(httptest.NewRequest("GET", "/oauth/autorizar?"+pedido(redir).Encode(), nil))
	if w.Code != 302 || !strings.HasPrefix(w.Header().Get("Location"), webPrueba+"/entrar?volver=%2Fapi%2Foauth%2Fautorizar%3F") {
		t.Fatalf("sin sesión: %d %s", w.Code, w.Header().Get("Location"))
	}
	// Una redirección que no registró: no se redirige a ningún lado.
	w = pedirH(conCookie(httptest.NewRequest("GET", "/oauth/autorizar?"+pedido("https://evil.com/cb").Encode(), nil)))
	if w.Code != 400 || w.Header().Get("Location") != "" {
		t.Fatalf("redirección ajena: %d", w.Code)
	}
	// Sin PKCE: vuelve con error.
	sinPKCE := pedido(redir)
	sinPKCE.Del("code_challenge")
	w = pedirH(conCookie(httptest.NewRequest("GET", "/oauth/autorizar?"+sinPKCE.Encode(), nil)))
	if loc := w.Header().Get("Location"); w.Code != 302 || !strings.Contains(loc, "error=invalid_request") || !strings.Contains(loc, "state=estado123") {
		t.Fatalf("sin PKCE: %d %s", w.Code, loc)
	}

	// El consentimiento.
	conseguirCodigo := func(redireccion string) string {
		t.Helper()
		w := pedirH(conCookie(httptest.NewRequest("GET", "/oauth/autorizar?"+pedido(redireccion).Encode(), nil)))
		m := reCSRF.FindStringSubmatch(w.Body.String())
		if w.Code != 200 || m == nil || !strings.Contains(w.Body.String(), "Claude quiere manejar tus robots") || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("consentimiento: %d %s", w.Code, w.Body.String())
		}
		f := pedido(redireccion)
		f.Set("csrf", m[1])
		f.Set("decision", "permitir")
		r := conCookie(httptest.NewRequest("POST", "/oauth/autorizar", strings.NewReader(f.Encode())))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", webPrueba)
		w = pedirH(r)
		loc, _ := url.Parse(w.Header().Get("Location"))
		if w.Code != 302 || loc == nil || loc.Query().Get("code") == "" || loc.Query().Get("state") != "estado123" || loc.Query().Get("iss") != apiPrueba {
			t.Fatalf("permitir: %d %s", w.Code, w.Header().Get("Location"))
		}
		return loc.Query().Get("code")
	}
	// CSRF equivocado o de otro sitio.
	f := pedido(redir)
	f.Set("csrf", "00")
	f.Set("decision", "permitir")
	r := conCookie(httptest.NewRequest("POST", "/oauth/autorizar", strings.NewReader(f.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := pedirH(r); w.Code != 403 {
		t.Fatalf("csrf malo: %d", w.Code)
	}
	r = conCookie(httptest.NewRequest("POST", "/oauth/autorizar", strings.NewReader(f.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://evil.com")
	if w := pedirH(r); w.Code != 403 {
		t.Fatalf("otro origen: %d", w.Code)
	}

	token := func(campos url.Values) (int, map[string]any) {
		r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(campos.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := pedirH(r)
		return w.Code, jsonDeW(w)
	}
	canje := func(codigo, verif, redireccion string) url.Values {
		c := url.Values{}
		c.Set("grant_type", "authorization_code")
		c.Set("code", codigo)
		c.Set("client_id", clienteID)
		c.Set("redirect_uri", redireccion)
		c.Set("code_verifier", verif)
		return c
	}
	// PKCE malo: invalid_grant (y el código se quema).
	codigo := conseguirCodigo(redir)
	if cod, v := token(canje(codigo, "otro-verificador-que-no-es-el-del-desafio-para-nada-"+azarHex(4), redir)); cod != 400 || v["error"] != "invalid_grant" {
		t.Fatalf("PKCE malo: %d %v", cod, v)
	}
	if cod, v := token(canje(codigo, verificador, redir)); cod != 400 || v["error"] != "invalid_grant" {
		t.Fatalf("código quemado: %d %v", cod, v)
	}
	// Bueno (con loopback en otro puerto, RFC 8252).
	loop := "http://localhost:51000/callback"
	codigo = conseguirCodigo(loop)
	cod, tk := token(canje(codigo, verificador, loop))
	acceso, _ := tk["access_token"].(string)
	refresco, _ := tk["refresh_token"].(string)
	if cod != 200 || !strings.HasPrefix(acceso, "sin_at_") || !strings.HasPrefix(refresco, "sin_rt_") || tk["expires_in"] != float64(3600) {
		t.Fatalf("token: %d %v", cod, tk)
	}
	if r := llamar(t, h, acceso, "listar_robots", nil); r.error || r.estado != 200 {
		t.Fatalf("el acceso no anda: %d %v", r.estado, r.texto)
	}
	// Código de un uso: reusarlo falla y revoca lo que salió de él.
	if cod, _ := token(canje(codigo, verificador, loop)); cod != 400 {
		t.Fatalf("reusar el código: %d", cod)
	}
	if cod, _ := rpc(t, h, acceso, "tools/list", map[string]any{}); cod != 401 {
		t.Fatalf("después de reusar el código, el acceso sigue andando: %d", cod)
	}

	// Refresh con rotación.
	codigo = conseguirCodigo(redir)
	_, tk = token(canje(codigo, verificador, redir))
	acceso, refresco = tk["access_token"].(string), tk["refresh_token"].(string)
	ref := func(rt string) (int, map[string]any) {
		c := url.Values{}
		c.Set("grant_type", "refresh_token")
		c.Set("refresh_token", rt)
		c.Set("client_id", clienteID)
		return token(c)
	}
	cod, tk2 := ref(refresco)
	acceso2, refresco2 := tk2["access_token"].(string), tk2["refresh_token"].(string)
	if cod != 200 || refresco2 == "" || refresco2 == refresco {
		t.Fatalf("refresco: %d %v", cod, tk2)
	}
	if cod, _ := rpc(t, h, acceso2, "tools/list", map[string]any{}); cod != 200 {
		t.Fatalf("acceso nuevo: %d", cod)
	}
	// Las IAs conectadas.
	web := func(metodo, ruta string) (int, map[string]any) {
		r := conCookie(httptest.NewRequest(metodo, ruta, nil))
		r.Header.Set("X-Sintetica", "1")
		w := pedirH(r)
		return w.Code, jsonDeW(w)
	}
	_, v := web("GET", "/v1/mcp/clientes")
	if l, _ := v["clientes"].([]any); len(l) != 1 || l[0].(map[string]any)["cliente"] != "Claude" {
		t.Fatalf("clientes: %v", v)
	}
	// Reusar el refresco viejo: robo; se revoca todo.
	if cod, v := ref(refresco); cod != 400 || v["error"] != "invalid_grant" {
		t.Fatalf("refresco reusado: %d %v", cod, v)
	}
	if cod, _ := rpc(t, h, acceso2, "tools/list", map[string]any{}); cod != 401 {
		t.Fatalf("después del robo el acceso sigue andando: %d", cod)
	}
	if cod, _ := ref(refresco2); cod != 400 {
		t.Fatalf("después del robo el refresco nuevo sigue andando: %d", cod)
	}
	_ = acceso

	// Revocación (RFC 7009) y desconectar desde la web.
	codigo = conseguirCodigo(redir)
	_, tk = token(canje(codigo, verificador, redir))
	c := url.Values{}
	c.Set("token", tk["access_token"].(string))
	c.Set("client_id", clienteID)
	r = httptest.NewRequest("POST", "/oauth/revocar", strings.NewReader(c.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := pedirH(r); w.Code != 200 {
		t.Fatalf("revocar: %d", w.Code)
	}
	if cod, _ := rpc(t, h, tk["access_token"].(string), "tools/list", map[string]any{}); cod != 401 {
		t.Fatalf("revocado sigue andando: %d", cod)
	}
	_, tk = ref(tk["refresh_token"].(string)) // el refresco sigue sirviendo
	_, v = web("GET", "/v1/mcp/clientes")
	l, _ := v["clientes"].([]any)
	if len(l) != 1 {
		t.Fatalf("clientes: %v", v)
	}
	if cod, _ := web("DELETE", "/v1/mcp/clientes/"+l[0].(map[string]any)["id"].(string)); cod != 204 {
		t.Fatalf("desconectar: %d", cod)
	}
	if cod, _ := rpc(t, h, tk["access_token"].(string), "tools/list", map[string]any{}); cod != 401 {
		t.Fatalf("desconectada sigue andando: %d", cod)
	}
	if cod, _ := ref(tk["refresh_token"].(string)); cod != 400 {
		t.Fatalf("desconectada, el refresco sigue andando: %d", cod)
	}

	// Un cliente confidencial necesita su secreto.
	_, conf := registrar(`{"client_name":"ChatGPT","redirect_uris":["https://chatgpt.com/connector_platform_oauth_redirect"],"token_endpoint_auth_method":"client_secret_post"}`)
	if conf["client_secret"] == nil {
		t.Fatalf("confidencial sin secreto: %v", conf)
	}
	c = url.Values{}
	c.Set("grant_type", "refresh_token")
	c.Set("refresh_token", "x")
	c.Set("client_id", conf["client_id"].(string))
	c.Set("client_secret", "equivocado")
	if cod, v := token(c); cod != 401 || v["error"] != "invalid_client" {
		t.Fatalf("secreto equivocado: %d %v", cod, v)
	}
	_ = color.White
}
