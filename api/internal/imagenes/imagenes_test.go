package imagenes

import (
	"bytes"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"testing"

	"sintetica/api/internal/protocolo"
)

func pngDePrueba(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{200, 30, 30, 255} // mitad izquierda roja
			if x >= w/2 {
				c = color.NRGBA{30, 30, 200, 255} // derecha azul
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPNGAQoi(t *testing.T) {
	r, err := Preparar(pngDePrueba(t, 800, 300), 320, 172, false)
	if err != nil {
		t.Fatal(err)
	}
	w, h, ok := protocolo.TamanoImagen(r.Datos)
	if !ok || w != 320 || h != 172 || r.Cuadros != 1 {
		t.Fatalf("tamaño %d × %d (%v), %d cuadros", w, h, ok, r.Cuadros)
	}
	img, err := DecodificarQoi(r.Datos)
	if err != nil {
		t.Fatal(err)
	}
	// Ida y vuelta sin pérdida.
	if !bytes.Equal(img.Pix, r.Vista.Pix) {
		t.Fatal("el QOI decodificado no es igual a la vista")
	}
	// Recortada llenando, centrada: rojo a la izquierda, azul a la derecha.
	if c := img.RGBAAt(10, 86); c.R < 150 || c.B > 80 {
		t.Fatalf("izquierda: %v", c)
	}
	if c := img.RGBAAt(310, 86); c.B < 150 || c.R > 80 {
		t.Fatalf("derecha: %v", c)
	}
}

func TestRedonda(t *testing.T) {
	r, err := Preparar(pngDePrueba(t, 100, 100), 240, 240, true)
	if err != nil {
		t.Fatal(err)
	}
	img, _ := DecodificarQoi(r.Datos)
	// La esquina queda del color de la pantalla; el centro, de la foto.
	if c := img.RGBAAt(2, 2); c != (color.RGBA{0x0B, 0x0A, 0x09, 255}) {
		t.Fatalf("esquina: %v", c)
	}
	if c := img.RGBAAt(60, 120); c.R < 150 {
		t.Fatalf("adentro: %v", c)
	}
}

func gifDePrueba(t *testing.T, cuadros int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for k := 0; k < cuadros; k++ {
		p := image.NewPaletted(image.Rect(0, 0, 60, 40), palette.Plan9)
		for i := range p.Pix {
			p.Pix[i] = uint8((k*37 + i/60) % 255)
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 5) // 50 ms
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestGIFAnimado(t *testing.T) {
	r, err := Preparar(gifDePrueba(t, 5), 320, 172, false)
	if err != nil {
		t.Fatal(err)
	}
	w, h, ok := protocolo.TamanoImagen(r.Datos)
	if !ok || w != 320 || h != 172 || r.Cuadros != 5 || string(r.Datos[:4]) != "sani" {
		t.Fatalf("animación %d × %d (%v), %d cuadros", w, h, ok, r.Cuadros)
	}
	if _, err := DecodificarQoi(r.Datos); err != nil {
		t.Fatal(err)
	}
	// Muchos cuadros: quedan 64 o menos y la duración total se mantiene.
	r, err = Preparar(gifDePrueba(t, 130), 64, 64, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cuadros > MaxCuadros || r.Cuadros < 2 {
		t.Fatalf("%d cuadros", r.Cuadros)
	}
	if _, _, ok := protocolo.TamanoImagen(r.Datos); !ok {
		t.Fatal("la animación reducida no es válida")
	}
	total := 0
	for i := 0; i < r.Cuadros; i++ {
		total += int(r.Datos[10+10*i]) | int(r.Datos[11+10*i])<<8
	}
	if total != 130*50 {
		t.Fatalf("dura %d ms, esperaba %d", total, 130*50)
	}
	if n, _ := contarCuadrosGif(gifDePrueba(t, 7)); n != 7 {
		t.Fatalf("contó %d cuadros", n)
	}
}

func TestEntradaMala(t *testing.T) {
	if _, err := Preparar([]byte("hola, no soy una imagen"), 320, 172, false); err != ErrFormato {
		t.Fatalf("esperaba ErrFormato, vino %v", err)
	}
	if _, err := Preparar(nil, 320, 172, false); err == nil {
		t.Fatal("vacía")
	}
	// Una cabecera PNG que dice 50000 × 50000: no se decodifica.
	b := pngDePrueba(t, 10, 10)
	b[16], b[17], b[18], b[19] = 0, 0, 0xC3, 0x50
	b[20], b[21], b[22], b[23] = 0, 0, 0xC3, 0x50
	if _, err := Preparar(b, 320, 172, false); err == nil {
		t.Fatal("aceptó una imagen gigante")
	}
}
