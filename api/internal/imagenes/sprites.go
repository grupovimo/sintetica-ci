package imagenes

// Los sprites de las caras de imagen (docs/motor-de-vida.md, "Caras de
// imagen"): las partes de una cara (ojo, ojo cerrado, boca…) en QOI de 4
// canales (con transparencia), recortadas a su contenido y de hasta 128 × 128.
// Entran como PNG, JPEG, GIF (el primer cuadro) o SVG (la IA los escribe muy
// bien); el SVG se rasteriza acá, en Go puro (oksvg + rasterx).

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// LadoSprite: lo más grande que puede ser un sprite, por lado (protocolo.LadoSprite).
const LadoSprite = 128

// MaxSvg: lo más grande que se acepta de un SVG (es texto).
const MaxSvg = 512 << 10

// ladoRaster: a cuánto se rasteriza el SVG más grande de una cara antes de
// recortarlo y achicarlo (así el achicado promedia y queda suave).
const ladoRaster = 512

// ErrVacio: la imagen no tiene nada visible (todo transparente o negro).
var ErrVacio = errors.New("la imagen no tiene nada visible (es toda transparente o negra)")

// Sprite es una parte lista para el bicho.
type Sprite struct {
	Datos []byte       // QOI de 4 canales
	Ancho int          // 1 a 128
	Alto  int          // 1 a 128
	Vista *image.NRGBA // lo mismo, para mostrarlo
}

// EsSVG dice si los datos parecen un SVG (texto con una etiqueta <svg).
func EsSVG(b []byte) bool {
	n := min(len(b), 2048)
	s := strings.TrimSpace(strings.TrimPrefix(string(b[:n]), "\uFEFF"))
	return strings.HasPrefix(s, "<svg") || ((strings.HasPrefix(s, "<?xml") || strings.HasPrefix(s, "<!")) && strings.Contains(s, "<svg"))
}

// PrepararSprite prepara una sola parte (recortada y achicada a 128 como mucho).
func PrepararSprite(datos []byte) (Sprite, error) {
	l, err := PrepararPartes([][]byte{datos})
	if err != nil {
		return Sprite{}, err
	}
	return l[0], nil
}

// PrepararPartes prepara las partes de una cara con la misma escala: cada una
// se recorta a su contenido y todas se achican por igual hasta que la más
// grande entre en 128 × 128 (así el ojo cerrado y la boca quedan del tamaño
// que se dibujaron al lado del ojo). Los SVG se rasterizan a la misma escala
// (la de sus unidades).
func PrepararPartes(partes [][]byte) ([]Sprite, error) {
	if len(partes) == 0 {
		return nil, errors.New("no hay partes")
	}
	// Los SVG, primero: la escala común de sus unidades.
	iconos := make([]*oksvg.SvgIcon, len(partes))
	maxVB := 0.0
	for i, d := range partes {
		if len(d) == 0 {
			return nil, fmt.Errorf("la parte %d está vacía", i+1)
		}
		if !EsSVG(d) {
			continue
		}
		if len(d) > MaxSvg {
			return nil, fmt.Errorf("el SVG pesa más de %d KB", MaxSvg>>10)
		}
		ic, err := oksvg.ReadIconStream(bytes.NewReader(d), oksvg.IgnoreErrorMode)
		if err != nil {
			return nil, fmt.Errorf("no entiendo el SVG: %v", err)
		}
		if ic.ViewBox.W <= 0 || ic.ViewBox.H <= 0 || ic.ViewBox.W > 1e5 || ic.ViewBox.H > 1e5 {
			return nil, errors.New("el SVG necesita un viewBox (o width y height) de tamaño razonable")
		}
		iconos[i] = ic
		maxVB = math.Max(maxVB, math.Max(ic.ViewBox.W, ic.ViewBox.H))
	}
	recortes := make([]*image.NRGBA, len(partes))
	maxLado := 0
	for i, d := range partes {
		var img *image.NRGBA
		if ic := iconos[i]; ic != nil {
			img = rasterizar(ic, ladoRaster/maxVB)
		} else {
			var err error
			if img, err = decodificarConAlfa(d); err != nil {
				return nil, err
			}
		}
		r := recortar(img)
		if r == nil {
			return nil, fmt.Errorf("parte %d: %w", i+1, ErrVacio)
		}
		recortes[i] = r
		maxLado = max(maxLado, r.Bounds().Dx(), r.Bounds().Dy())
	}
	escala := math.Min(1, float64(LadoSprite)/float64(maxLado))
	l := make([]Sprite, len(partes))
	for i, r := range recortes {
		w := max(1, min(LadoSprite, int(math.Round(float64(r.Bounds().Dx())*escala))))
		h := max(1, min(LadoSprite, int(math.Round(float64(r.Bounds().Dy())*escala))))
		v := escalarConAlfa(r, w, h)
		l[i] = Sprite{Datos: CodificarQoiRGBA(v), Ancho: w, Alto: h, Vista: v}
	}
	return l, nil
}

func rasterizar(ic *oksvg.SvgIcon, k float64) *image.NRGBA {
	w := max(1, int(math.Ceil(ic.ViewBox.W*k)))
	h := max(1, int(math.Ceil(ic.ViewBox.H*k)))
	ic.SetTarget(0, 0, float64(w), float64(h))
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	escaner := rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())
	ic.Draw(rasterx.NewDasher(w, h, escaner), 1)
	n := image.NewNRGBA(rgba.Bounds())
	draw.Draw(n, n.Bounds(), rgba, image.Point{}, draw.Src)
	return n
}

// decodificarConAlfa: un PNG, JPEG o GIF (el primer cuadro), con su transparencia.
func decodificarConAlfa(datos []byte) (*image.NRGBA, error) {
	if len(datos) > MaxEntrada {
		return nil, fmt.Errorf("la imagen pesa más de %d MB", MaxEntrada>>20)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil {
		return nil, errors.New("la imagen tiene que ser SVG, PNG, JPEG o GIF")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxLadoEntrada || cfg.Height > maxLadoEntrada || cfg.Width*cfg.Height > maxPixeles {
		return nil, fmt.Errorf("la imagen es demasiado grande (%d × %d)", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(datos))
	if err != nil {
		return nil, errors.New("la imagen tiene que ser SVG, PNG, JPEG o GIF")
	}
	return aNRGBA(img), nil
}

// recortar al contenido: lo que no es transparente; si la imagen no tiene
// transparencia (un JPEG), lo que no es casi negro (en la pantalla, el negro
// es el fondo). nil si no queda nada.
func recortar(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	conAlfa := false
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 250 {
			conAlfa = true
			break
		}
	}
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.Pix[img.PixOffset(x, y):]
			visible := p[3] > 10
			if !conAlfa {
				visible = int(p[0])+int(p[1])+int(p[2]) > 60
			}
			if visible {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return nil
	}
	out := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(out, out.Bounds(), img, image.Point{x0, y0}, draw.Src)
	return out
}

// escalarConAlfa achica (o agranda) a w × h promediando con la transparencia
// premultiplicada (los bordes no quedan con halo).
func escalarConAlfa(src *image.NRGBA, w, h int) *image.NRGBA {
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	px, py := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	paso := math.Max(px, py)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, a := muestrear(src, float64(b.Min.X)+float64(x)*px, float64(b.Min.Y)+float64(y)*py, paso)
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = byteDe(r), byteDe(g), byteDe(bl), byteDe(a*255)
		}
	}
	return out
}

// CodificarQoiRGBA codifica en QOI de 4 canales (con la transparencia).
func CodificarQoiRGBA(img *image.NRGBA) []byte {
	ancho, alto := img.Bounds().Dx(), img.Bounds().Dy()
	s := make([]byte, 0, 14+ancho*alto*2+8)
	s = append(s, 'q', 'o', 'i', 'f')
	s = binary.BigEndian.AppendUint32(s, uint32(ancho))
	s = binary.BigEndian.AppendUint32(s, uint32(alto))
	s = append(s, 4, 0)
	var indice [64][4]uint8
	pr, pg, pb, pa := uint8(0), uint8(0), uint8(0), uint8(255)
	racha := 0
	total := ancho * alto
	for k := 0; k < total; k++ {
		i := img.PixOffset(img.Bounds().Min.X+k%ancho, img.Bounds().Min.Y+k/ancho)
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if a == 0 {
			r, g, b = 0, 0, 0 // lo transparente, siempre igual (comprime mejor)
		}
		if r == pr && g == pg && b == pb && a == pa {
			racha++
			if racha == 62 || k == total-1 {
				s = append(s, 0xC0|byte(racha-1))
				racha = 0
			}
			continue
		}
		if racha > 0 {
			s = append(s, 0xC0|byte(racha-1))
			racha = 0
		}
		h := hashQoi(r, g, b, a)
		if indice[h] == [4]uint8{r, g, b, a} {
			s = append(s, byte(h))
		} else {
			indice[h] = [4]uint8{r, g, b, a}
			vr, vg, vb := int(int8(r-pr)), int(int8(g-pg)), int(int8(b-pb))
			vgr, vgb := vr-vg, vb-vg
			switch {
			case a != pa:
				s = append(s, 0xFF, r, g, b, a)
			case vr > -3 && vr < 2 && vg > -3 && vg < 2 && vb > -3 && vb < 2:
				s = append(s, 0x40|byte((vr+2)<<4|(vg+2)<<2|(vb+2)))
			case vgr > -9 && vgr < 8 && vg > -33 && vg < 32 && vgb > -9 && vgb < 8:
				s = append(s, 0x80|byte(vg+32), byte((vgr+8)<<4|(vgb+8)))
			default:
				s = append(s, 0xFE, r, g, b)
			}
		}
		pr, pg, pb, pa = r, g, b, a
	}
	return append(s, 0, 0, 0, 0, 0, 0, 0, 1)
}
