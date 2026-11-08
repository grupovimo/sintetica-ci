package imagenes

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"sintetica/api/internal/protocolo"
)

// Un ojo de anime en SVG, como los de la guía del MCP.
const ojoSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <defs><radialGradient id="i" cx="50%" cy="60%" r="50%"><stop offset="0" stop-color="#7fe7ff"/><stop offset="1" stop-color="#1b4dff"/></radialGradient></defs>
  <ellipse cx="50" cy="55" rx="30" ry="40" fill="url(#i)"/>
  <ellipse cx="50" cy="60" rx="13" ry="18" fill="#05060a"/>
  <circle cx="40" cy="38" r="9" fill="#ffffff"/>
</svg>`

const bocaSVG = `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><path d="M35 50 Q50 62 65 50" stroke="#ff7aa8" stroke-width="5" fill="none" stroke-linecap="round"/></svg>`

func alfa(img *image.RGBA, x, y int) uint8 { return img.Pix[img.PixOffset(x, y)+3] }

func TestSpriteSVGConTransparencia(t *testing.T) {
	if !EsSVG([]byte(ojoSVG)) || !EsSVG([]byte(bocaSVG)) || EsSVG([]byte("\x89PNG")) {
		t.Fatal("EsSVG")
	}
	s, err := PrepararSprite([]byte(ojoSVG))
	if err != nil {
		t.Fatal(err)
	}
	w, h, ok := protocolo.TamanoSprite(s.Datos)
	if !ok || w != s.Ancho || h != s.Alto || s.Datos[12] != 4 {
		t.Fatalf("QOI %d × %d ok=%v canales=%d", w, h, ok, s.Datos[12])
	}
	// Recortado al contenido (la elipse: 60 × 80 de 100) y achicado a 128 de alto.
	if h != 128 || w < 90 || w > 100 {
		t.Fatalf("tamaño %d × %d (esperaba ~96 × 128)", w, h)
	}
	img, err := DecodificarQoi(s.Datos)
	if err != nil {
		t.Fatal(err)
	}
	if alfa(img, 0, 0) != 0 || alfa(img, w-1, 0) != 0 {
		t.Fatalf("las esquinas tienen que ser transparentes: %d %d", alfa(img, 0, 0), alfa(img, w-1, 0))
	}
	c := img.Pix[img.PixOffset(w/2, h*5/8):]
	if c[3] != 255 || c[0] > 30 || c[1] > 30 {
		t.Fatalf("la pupila (oscura y opaca): %v", c[:4])
	}
	brillo := img.Pix[img.PixOffset(w*1/3, h*2/7):]
	if brillo[3] < 200 || brillo[0] < 200 {
		t.Fatalf("el brillo blanco: %v", brillo[:4])
	}
}

func TestPartesConLaMismaEscala(t *testing.T) {
	l, err := PrepararPartes([][]byte{[]byte(ojoSVG), []byte(bocaSVG)})
	if err != nil {
		t.Fatal(err)
	}
	// La boca (30 de ancho en las mismas unidades que el ojo de 60) mide la mitad.
	ojo, boca := l[0], l[1]
	if r := float64(boca.Ancho) / float64(ojo.Ancho); r < 0.45 || r > 0.62 {
		t.Fatalf("la boca no quedó a la escala del ojo: %d / %d", boca.Ancho, ojo.Ancho)
	}
	if boca.Alto > 30 {
		t.Fatalf("la boca es una rayita: alto %d", boca.Alto)
	}
}

func TestSpritePNGyJPEG(t *testing.T) {
	// Un PNG de 300 × 300 transparente con un cuadrado opaco de 200 × 100 en el medio.
	img := image.NewNRGBA(image.Rect(0, 0, 300, 300))
	for y := 100; y < 200; y++ {
		for x := 50; x < 250; x++ {
			img.Set(x, y, color.NRGBA{255, 120, 0, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img) //nolint:errcheck
	s, err := PrepararSprite(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if s.Ancho != 128 || s.Alto != 64 {
		t.Fatalf("recortado y achicado: %d × %d", s.Ancho, s.Alto)
	}
	// Un JPEG (sin transparencia) con fondo negro: se recorta lo que no es negro.
	j := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 40; y < 60; y++ {
		for x := 10; x < 90; x++ {
			j.Set(x, y, color.RGBA{250, 250, 250, 255})
		}
	}
	b.Reset()
	jpeg.Encode(&b, j, &jpeg.Options{Quality: 95}) //nolint:errcheck
	s, err = PrepararSprite(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if s.Ancho < 78 || s.Ancho > 84 || s.Alto < 18 || s.Alto > 24 {
		t.Fatalf("JPEG recortado: %d × %d", s.Ancho, s.Alto)
	}
	// Todo transparente: error.
	b.Reset()
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 10, 10))) //nolint:errcheck
	if _, err := PrepararSprite(b.Bytes()); err == nil {
		t.Fatal("una imagen vacía no es un sprite")
	}
	if _, err := PrepararSprite([]byte("hola")); err == nil {
		t.Fatal("eso no es una imagen")
	}
	if _, err := PrepararSprite([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)); err == nil {
		t.Fatal("un SVG sin tamaño")
	}
}
