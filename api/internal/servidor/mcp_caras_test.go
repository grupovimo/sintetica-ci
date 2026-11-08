package servidor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sintetica/api/internal/protocolo"
)

const (
	ojoAnime = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <defs><linearGradient id="iris" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#3b2bff"/><stop offset="1" stop-color="#ff9be8"/></linearGradient></defs>
  <path d="M12 34 Q50 14 88 30" stroke="#f5f2ec" stroke-width="6" fill="none" stroke-linecap="round"/>
  <ellipse cx="50" cy="58" rx="27" ry="33" fill="url(#iris)"/>
  <ellipse cx="50" cy="62" rx="12" ry="17" fill="#120a2a"/>
  <ellipse cx="40" cy="46" rx="9" ry="11" fill="#ffffff"/>
</svg>`
	ojoAnimeCerrado = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><path d="M18 62 Q50 40 82 62" stroke="#f5f2ec" stroke-width="7" fill="none" stroke-linecap="round"/></svg>`
	bocaAnime       = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><path d="M40 50 Q50 60 60 50" stroke="#ff8fb8" stroke-width="5" fill="none" stroke-linecap="round"/></svg>`
)

// Las imágenes de un resultado (dibujar_cara devuelve dos).
func imagenesDe(t *testing.T, r resultadoHerramienta) [][]byte {
	t.Helper()
	var l [][]byte
	for _, c := range r.content {
		m := c.(map[string]any)
		if m["type"] == "image" {
			b, err := base64.StdEncoding.DecodeString(m["data"].(string))
			if err != nil {
				t.Fatal(err)
			}
			l = append(l, b)
		}
	}
	return l
}

// Cuántos píxeles violetas o rosas (los del iris) tiene un PNG.
func violetas(t *testing.T, b []byte) int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	bb := img.Bounds()
	for y := bb.Min.Y; y < bb.Max.Y; y++ {
		for x := bb.Min.X; x < bb.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if b>>8 > 150 && g>>8 < 140 && r>>8 > 40 {
				n++
			}
		}
	}
	return n
}

func TestMCPDibujarCara(t *testing.T) {
	s, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	ctx := context.Background()
	robot := u.bichos[0]
	base := map[string]any{"robot": robot, "nombre": "anime", "ojo": ojoAnime, "ojo_cerrado": ojoAnimeCerrado, "boca": bocaAnime,
		"tamano": 0.46, "boca_y": -0.42, "blush": 0.5}

	// solo_ver: dos imágenes, nada guardado.
	args := map[string]any{"solo_ver": true}
	for k, v := range base {
		args[k] = v
	}
	r := llamar(t, h, u.token, "dibujar_cara", args)
	if r.error {
		t.Fatalf("solo_ver: %v", r.texto)
	}
	imgs := imagenesDe(t, r)
	if len(imgs) != 2 {
		t.Fatalf("esperaba 2 imágenes (abierta y parpadeando), vinieron %d", len(imgs))
	}
	abierta, parpadeando := violetas(t, imgs[0]), violetas(t, imgs[1])
	if abierta < 2000 || parpadeando > abierta/5 {
		t.Fatalf("abierta %d px violetas, parpadeando %d", abierta, parpadeando)
	}
	if l, _ := alm.Imagenes(ctx, robot); len(l) != 0 {
		t.Fatalf("solo_ver guardó sprites: %v", l)
	}

	// De verdad, como cara de siempre.
	args = map[string]any{"reposo": true}
	for k, v := range base {
		args[k] = v
	}
	r = llamar(t, h, u.token, "dibujar_cara", args)
	if r.error || len(imagenesDe(t, r)) != 2 || !strings.Contains(r.texto, "cara de siempre") {
		t.Fatalf("dibujar_cara: %v", r.texto)
	}
	l, _ := alm.Imagenes(ctx, robot)
	if len(l) != 3 || l[0].Espacio != protocolo.PrimerSprite || l[2].Espacio != protocolo.PrimerSprite+2 {
		t.Fatalf("los sprites: %+v", l)
	}
	c, err := alm.UltimaConfig(ctx, robot)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Cara        map[string]any            `json:"cara"`
		Expresiones map[string]map[string]any `json:"expresiones"`
	}
	json.Unmarshal([]byte(c.Texto), &cfg) //nolint:errcheck
	if cfg.Cara["expresion"] != "anime" || cfg.Expresiones["anime"]["sprites"] == nil || cfg.Expresiones["anime"]["tamano"] != 0.46 {
		t.Fatalf("config: %s", c.Texto)
	}
	// Rehacerla usa los mismos lugares; otra cara usa uno libre.
	r = llamar(t, h, u.token, "dibujar_cara", base)
	if r.error {
		t.Fatal(r.texto)
	}
	r = llamar(t, h, u.token, "dibujar_cara", map[string]any{"robot": robot, "nombre": "gato", "ojo": ojoAnimeCerrado})
	if r.error {
		t.Fatal(r.texto)
	}
	if l, _ := alm.Imagenes(ctx, robot); len(l) != 4 || l[3].Espacio != protocolo.PrimerSprite+3 {
		t.Fatalf("después de gato: %+v", l)
	}
	// listar_expresiones las marca; ver_robot cuenta los sprites.
	if r := llamar(t, h, u.token, "listar_expresiones", map[string]any{"robot": robot}); !strings.Contains(r.texto, `"imagen":true`) {
		t.Fatalf("listar_expresiones: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "ver_robot", map[string]any{"robot": robot}); !strings.Contains(r.texto, `"carasDeImagen"`) || !strings.Contains(r.texto, `"sprite":3`) {
		t.Fatalf("ver_robot: %v", r.texto)
	}
	// poner_cara con siempre elige la cara de siempre.
	if r := llamar(t, h, u.token, "poner_cara", map[string]any{"robot": robot, "expresion": "gato", "siempre": true}); r.error {
		t.Fatal(r.texto)
	}
	c, _ = alm.UltimaConfig(ctx, robot)
	if !strings.Contains(c.Texto, `"cara":{"expresion":"gato"}`) {
		t.Fatalf("siempre: %s", c.Texto)
	}
	// Borrarla libera sus sprites y, si era la de siempre, vuelve la neutral.
	if r := llamar(t, h, u.token, "borrar_expresion", map[string]any{"robot": robot, "nombre": "gato"}); r.error {
		t.Fatal(r.texto)
	}
	c, _ = alm.UltimaConfig(ctx, robot)
	if strings.Contains(c.Texto, `"gato"`) || strings.Contains(c.Texto, `"cara"`) {
		t.Fatalf("después de borrar: %s", c.Texto)
	}
	if l, _ := alm.Imagenes(ctx, robot); len(l) != 3 {
		t.Fatalf("no liberó el sprite de gato: %+v", l)
	}
	// Errores: sin ojo, un SVG roto, una URL a la red interna.
	if r := llamar(t, h, u.token, "dibujar_cara", map[string]any{"robot": robot, "nombre": "x", "boca": bocaAnime}); !r.error {
		t.Fatal("sin ojo tiene que fallar")
	}
	if r := llamar(t, h, u.token, "dibujar_cara", map[string]any{"robot": robot, "nombre": "x", "ojo": "<svg xmlns='http://www.w3.org/2000/svg'></svg>"}); !r.error {
		t.Fatal("un SVG sin tamaño tiene que fallar")
	}
	if r := llamar(t, h, u.token, "dibujar_cara", map[string]any{"robot": robot, "nombre": "x", "ojo": "http://127.0.0.1:5432/x.png"}); !r.error {
		t.Fatal("una URL interna tiene que fallar")
	}
	_ = s
}

func TestRutasSprites(t *testing.T) {
	_, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	robot := u.bichos[0]
	web := func(metodo, ruta string, cuerpo []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest(metodo, ruta, bytes.NewReader(cuerpo))
		r.AddCookie(&http.Cookie{Name: cookieSesion, Value: u.cookie})
		r.Header.Set("X-Sintetica", "1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := web("PUT", "/v1/bichos/"+robot+"/sprites/5", []byte(ojoAnime))
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
	if w.Code != 200 || v["sprite"] != 5.0 || v["ancho"] != 128.0 {
		t.Fatalf("subir: %d %s", w.Code, w.Body.String())
	}
	w = web("GET", "/v1/bichos/"+robot+"/sprites", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sprite":5`) {
		t.Fatalf("listar: %s", w.Body.String())
	}
	// No aparece entre las imágenes.
	if w = web("GET", "/v1/bichos/"+robot+"/imagenes", nil); strings.Contains(w.Body.String(), `"espacio":13`) {
		t.Fatalf("imagenes: %s", w.Body.String())
	}
	w = web("GET", "/v1/bichos/"+robot+"/sprites/5?png=1", nil)
	if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); w.Code != 200 || err != nil {
		t.Fatalf("png: %d %v", w.Code, err)
	}
	if w = web("GET", "/v1/bichos/"+robot+"/sprites/5", nil); w.Code != 200 || w.Body.Bytes()[12] != 4 {
		t.Fatalf("qoi: %d", w.Code)
	}
	if w = web("PUT", "/v1/bichos/"+robot+"/sprites/12", []byte(ojoAnime)); w.Code != 400 {
		t.Fatalf("sprite 12: %d", w.Code)
	}
	if w = web("PUT", "/v1/bichos/"+robot+"/sprites/1", []byte("no es nada")); w.Code != 422 {
		t.Fatalf("basura: %d", w.Code)
	}
	if w = web("DELETE", "/v1/bichos/"+robot+"/sprites/5", nil); w.Code != 204 {
		t.Fatalf("borrar: %d", w.Code)
	}
	if w = web("GET", "/v1/bichos/"+robot+"/sprites/5", nil); w.Code != 404 {
		t.Fatalf("después de borrar: %d", w.Code)
	}
	// De otro usuario, no.
	otro := nuevoUsuarioMCP(t, alm)
	if w = web("PUT", "/v1/bichos/"+otro.bichos[0]+"/sprites/0", []byte(ojoAnime)); w.Code == 200 {
		t.Fatal("subió un sprite a un robot ajeno")
	}
}
