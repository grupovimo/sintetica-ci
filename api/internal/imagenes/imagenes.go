// Package imagenes prepara una foto o un GIF para la pantalla de un bicho, en
// Go y del lado de la nube (lo usa el servidor MCP): lo mismo que hacen la web
// (web/src/lib/imagenes.ts) y las apps. Recorta llenando la pantalla
// (centrada), en el broche la deja redonda (afuera del círculo, el color de la
// pantalla), y la codifica en QOI de 3 canales; un GIF animado, como
// animación "sani" (la cabecera, la tabla de cuadros y cada cuadro en QOI; ver
// firmware/nucleo/qoi.h y sim/web/qoi.js), de hasta 64 cuadros.
package imagenes

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // se registra para image.Decode
	"image/png"
	"math"
)

// MaxCuadros: cuántos cuadros puede tener una animación (protocolo.MaxCuadros).
const MaxCuadros = 64

// MaxEntrada es lo más grande que se acepta de archivo.
const MaxEntrada = 8 << 20

// Límites contra las "bombas" de descompresión: cuántos píxeles puede tener la
// imagen y, en un GIF, todos sus cuadros juntos.
const (
	maxPixeles       = 16_000_000
	maxPixelesGif    = 64_000_000
	maxLadoEntrada   = 8192
	fondoPantallaRGB = 0x0B0A09 // el color de la pantalla (paleta::PANTALLA)
)

// ErrFormato: no es un PNG, un JPEG ni un GIF.
var ErrFormato = errors.New("la imagen tiene que ser PNG, JPEG o GIF")

// Resultado es la imagen lista para el bicho.
type Resultado struct {
	Datos   []byte      // QOI o "sani"
	Cuadros int         // 1 = una imagen quieta
	Vista   *image.RGBA // el primer cuadro, ya recortado (para mostrarlo)
}

// Limite es lo más que entra en un espacio de imagen del bicho (el peor caso
// de un QOI de la pantalla).
func Limite(ancho, alto int) int { return ancho*alto*4 + 22 }

// Preparar convierte una imagen (PNG, JPEG o GIF, animado o no) al formato
// del bicho, del tamaño exacto de su pantalla.
func Preparar(datos []byte, ancho, alto int, redonda bool) (Resultado, error) {
	if ancho < 16 || alto < 16 || ancho > 640 || alto > 640 {
		return Resultado{}, fmt.Errorf("pantalla de %d × %d: tiene que ser de 16 a 640 por lado", ancho, alto)
	}
	if len(datos) == 0 {
		return Resultado{}, errors.New("la imagen está vacía")
	}
	if len(datos) > MaxEntrada {
		return Resultado{}, fmt.Errorf("la imagen pesa más de %d MB", MaxEntrada>>20)
	}
	cfg, formato, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil {
		return Resultado{}, ErrFormato
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxLadoEntrada || cfg.Height > maxLadoEntrada || cfg.Width*cfg.Height > maxPixeles {
		return Resultado{}, fmt.Errorf("la imagen es demasiado grande (%d × %d): hasta %d × %d y 16 megapíxeles", cfg.Width, cfg.Height, maxLadoEntrada, maxLadoEntrada)
	}
	if formato == "gif" {
		n, err := contarCuadrosGif(datos)
		if err != nil {
			return Resultado{}, err
		}
		if n*cfg.Width*cfg.Height > maxPixelesGif {
			return Resultado{}, fmt.Errorf("el GIF es demasiado pesado (%d cuadros de %d × %d)", n, cfg.Width, cfg.Height)
		}
		if n > 1 {
			return prepararGif(datos, ancho, alto, redonda)
		}
	}
	img, _, err := image.Decode(bytes.NewReader(datos))
	if err != nil {
		return Resultado{}, ErrFormato
	}
	vista := Encuadrar(img, ancho, alto, redonda)
	return Resultado{Datos: CodificarQoi(vista), Cuadros: 1, Vista: vista}, nil
}

// Cuadro de una animación.
type Cuadro struct {
	Img *image.RGBA
	Ms  int
}

func prepararGif(datos []byte, ancho, alto int, redonda bool) (Resultado, error) {
	g, err := gif.DecodeAll(bytes.NewReader(datos))
	if err != nil || len(g.Image) == 0 {
		return Resultado{}, ErrFormato
	}
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	// Se arma cada cuadro entero (un GIF guarda solo lo que cambia) respetando
	// cómo se descarta cada uno.
	lienzo := image.NewRGBA(image.Rect(0, 0, w, h))
	var cuadros []Cuadro
	for k, q := range g.Image {
		var antes *image.RGBA
		disp := byte(0)
		if k < len(g.Disposal) {
			disp = g.Disposal[k]
		}
		if disp == gif.DisposalPrevious {
			antes = image.NewRGBA(lienzo.Bounds())
			copy(antes.Pix, lienzo.Pix)
		}
		draw.Draw(lienzo, q.Bounds(), q, q.Bounds().Min, draw.Over)
		ms := 100
		if k < len(g.Delay) && g.Delay[k] > 1 {
			ms = g.Delay[k] * 10 // centésimas
		}
		cuadros = append(cuadros, Cuadro{Img: Encuadrar(lienzo, ancho, alto, redonda), Ms: ms})
		switch disp {
		case gif.DisposalBackground:
			draw.Draw(lienzo, q.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			copy(lienzo.Pix, antes.Pix)
		}
	}
	return empaquetar(cuadros, ancho, alto)
}

// empaquetar: si no entra en el espacio (o son demasiados cuadros), se saltea
// uno de cada dos y el que queda dura lo de los dos (la animación sigue
// durando lo mismo). Como en la web.
func empaquetar(cuadros []Cuadro, ancho, alto int) (Resultado, error) {
	vista := cuadros[0].Img
	if len(cuadros) == 1 {
		return Resultado{Datos: CodificarQoi(vista), Cuadros: 1, Vista: vista}, nil
	}
	for {
		if len(cuadros) <= MaxCuadros {
			sani := CodificarAnimacion(cuadros, ancho, alto)
			if len(sani) <= Limite(ancho, alto) {
				return Resultado{Datos: sani, Cuadros: len(cuadros), Vista: vista}, nil
			}
		}
		if len(cuadros) <= 2 {
			return Resultado{}, errors.New("el GIF es demasiado pesado para la pantalla: probá con uno más simple o más corto")
		}
		var menos []Cuadro
		for i := 0; i < len(cuadros); i += 2 {
			c := cuadros[i]
			if i+1 < len(cuadros) {
				c.Ms += cuadros[i+1].Ms
			}
			menos = append(menos, c)
		}
		cuadros = menos
	}
}

// Encuadrar escala la imagen para que llene ancho × alto (recortando lo que
// sobra, centrada), sobre el color de la pantalla; con redonda, lo de afuera
// del círculo queda del color de la pantalla.
func Encuadrar(src image.Image, ancho, alto int, redonda bool) *image.RGBA {
	b := src.Bounds()
	fw, fh := float64(b.Dx()), float64(b.Dy())
	escala := math.Max(float64(ancho)/fw, float64(alto)/fh)
	// La parte de la imagen que se ve, en coordenadas de la imagen.
	ox := float64(b.Min.X) + (fw-float64(ancho)/escala)/2
	oy := float64(b.Min.Y) + (fh-float64(alto)/escala)/2
	paso := 1 / escala
	fondo := [3]float64{float64(fondoPantallaRGB >> 16), float64(fondoPantallaRGB >> 8 & 0xFF), float64(fondoPantallaRGB & 0xFF)}
	nrgba := aNRGBA(src)
	out := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	cx, cy, radio := float64(ancho)/2, float64(alto)/2, math.Min(float64(ancho), float64(alto))/2
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			r, g, bl, a := muestrear(nrgba, ox+float64(x)*paso, oy+float64(y)*paso, paso)
			// Sobre el fondo (los PNG con transparencia).
			col := [3]float64{r*a + fondo[0]*(1-a), g*a + fondo[1]*(1-a), bl*a + fondo[2]*(1-a)}
			if redonda {
				d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
				cob := math.Max(0, math.Min(1, radio-d+0.5))
				for k := range col {
					col[k] = col[k]*cob + fondo[k]*(1-cob)
				}
			}
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = byteDe(col[0]), byteDe(col[1]), byteDe(col[2]), 255
		}
	}
	return out
}

func byteDe(v float64) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(v)))) }

func aNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok {
		return n
	}
	b := src.Bounds()
	n := image.NewNRGBA(b)
	draw.Draw(n, b, src, b.Min, draw.Src)
	return n
}

// muestrear promedia el área de la imagen que cae en un píxel de salida (que
// empieza en x, y y mide `paso` de lado); si la imagen se agranda, interpola.
// Devuelve el color (sin premultiplicar, 0-255) y la opacidad (0-1).
func muestrear(n *image.NRGBA, x, y, paso float64) (r, g, b, a float64) {
	bb := n.Bounds()
	en := func(px, py int) (float64, float64, float64, float64) {
		px = max(bb.Min.X, min(bb.Max.X-1, px))
		py = max(bb.Min.Y, min(bb.Max.Y-1, py))
		i := n.PixOffset(px, py)
		al := float64(n.Pix[i+3]) / 255
		return float64(n.Pix[i]) * al, float64(n.Pix[i+1]) * al, float64(n.Pix[i+2]) * al, al
	}
	var sr, sg, sb, sa, peso float64
	if paso <= 1 {
		// Agranda: bilineal en el centro del píxel.
		fx, fy := x+paso/2-0.5, y+paso/2-0.5
		x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
		tx, ty := fx-float64(x0), fy-float64(y0)
		for _, p := range [4][3]float64{{0, 0, (1 - tx) * (1 - ty)}, {1, 0, tx * (1 - ty)}, {0, 1, (1 - tx) * ty}, {1, 1, tx * ty}} {
			cr, cg, cb, ca := en(x0+int(p[0]), y0+int(p[1]))
			sr, sg, sb, sa, peso = sr+cr*p[2], sg+cg*p[2], sb+cb*p[2], sa+ca*p[2], peso+p[2]
		}
	} else {
		// Achica: el promedio de los píxeles del área (como mucho 8 × 8 muestras).
		muestras := min(8, int(math.Ceil(paso)))
		d := paso / float64(muestras)
		for j := 0; j < muestras; j++ {
			for i := 0; i < muestras; i++ {
				cr, cg, cb, ca := en(int(x+(float64(i)+0.5)*d), int(y+(float64(j)+0.5)*d))
				sr, sg, sb, sa, peso = sr+cr, sg+cg, sb+cb, sa+ca, peso+1
			}
		}
	}
	sr, sg, sb, sa = sr/peso, sg/peso, sb/peso, sa/peso
	if sa <= 0 {
		return 0, 0, 0, 0
	}
	return sr / sa, sg / sa, sb / sa, sa
}

// contarCuadrosGif cuenta los cuadros de un GIF sin decodificarlos (para no
// descomprimir una bomba).
func contarCuadrosGif(b []byte) (int, error) {
	if len(b) < 13 || string(b[:3]) != "GIF" {
		return 0, ErrFormato
	}
	i := 13
	if b[10]&0x80 != 0 {
		i += 3 << (int(b[10]&7) + 1) // la paleta global
	}
	saltearBloques := func() bool {
		for i < len(b) {
			n := int(b[i])
			i++
			if n == 0 {
				return true
			}
			i += n
		}
		return false
	}
	n := 0
	for i < len(b) {
		switch b[i] {
		case 0x3B: // fin
			return max(n, 1), nil
		case 0x21: // extensión
			i += 2
			if !saltearBloques() {
				return max(n, 1), nil
			}
		case 0x2C: // un cuadro
			if i+10 > len(b) {
				return max(n, 1), nil
			}
			n++
			if n > 4096 {
				return 0, errors.New("el GIF tiene demasiados cuadros")
			}
			paleta := b[i+9]
			i += 10
			if paleta&0x80 != 0 {
				i += 3 << (int(paleta&7) + 1)
			}
			i++ // el tamaño mínimo del código LZW
			if !saltearBloques() {
				return max(n, 1), nil
			}
		default:
			return max(n, 1), nil
		}
	}
	return max(n, 1), nil
}

// --- QOI ---

func hashQoi(r, g, b, a uint8) int { return (int(r)*3 + int(g)*5 + int(b)*7 + int(a)*11) % 64 }

// CodificarQoi codifica en QOI de 3 canales (la opacidad se ignora).
func CodificarQoi(img *image.RGBA) []byte {
	ancho, alto := img.Bounds().Dx(), img.Bounds().Dy()
	s := make([]byte, 0, 14+ancho*alto+8)
	s = append(s, 'q', 'o', 'i', 'f')
	s = binary.BigEndian.AppendUint32(s, uint32(ancho))
	s = binary.BigEndian.AppendUint32(s, uint32(alto))
	s = append(s, 3, 0)
	var indice [64][4]uint8
	pr, pg, pb, pa := uint8(0), uint8(0), uint8(0), uint8(255)
	racha := 0
	total := ancho * alto
	for k := 0; k < total; k++ {
		x, y := k%ancho, k/ancho
		i := img.PixOffset(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], uint8(255)
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

// CodificarAnimacion arma una animación "sani": "sani", versión 1, la
// cantidad de cuadros, ancho y alto (u16), la tabla (ms u16, desplazamiento
// u32, largo u32 por cuadro) y cada cuadro en QOI.
func CodificarAnimacion(cuadros []Cuadro, ancho, alto int) []byte {
	qois := make([][]byte, len(cuadros))
	total := 10 + 10*len(cuadros)
	for i, c := range cuadros {
		qois[i] = CodificarQoi(c.Img)
		total += len(qois[i])
	}
	s := make([]byte, 0, total)
	s = append(s, 's', 'a', 'n', 'i', 1, byte(len(cuadros)))
	s = binary.LittleEndian.AppendUint16(s, uint16(ancho))
	s = binary.LittleEndian.AppendUint16(s, uint16(alto))
	off := 10 + 10*len(cuadros)
	for i, q := range qois {
		ms := max(20, min(65535, cuadros[i].Ms))
		s = binary.LittleEndian.AppendUint16(s, uint16(ms))
		s = binary.LittleEndian.AppendUint32(s, uint32(off))
		s = binary.LittleEndian.AppendUint32(s, uint32(len(q)))
		off += len(q)
	}
	for _, q := range qois {
		s = append(s, q...)
	}
	return s
}

// DecodificarQoi decodifica un QOI (o el primer cuadro de una animación) a RGBA.
func DecodificarQoi(b []byte) (*image.RGBA, error) {
	if len(b) >= 10 && string(b[:4]) == "sani" {
		if len(b) < 20 {
			return nil, errors.New("animación cortada")
		}
		off, largo := int(binary.LittleEndian.Uint32(b[12:])), int(binary.LittleEndian.Uint32(b[16:]))
		if off < 0 || largo < 0 || off+largo > len(b) {
			return nil, errors.New("animación cortada")
		}
		b = b[off : off+largo]
	}
	if len(b) < 22 || string(b[:4]) != "qoif" {
		return nil, errors.New("no es QOI")
	}
	ancho, alto := int(binary.BigEndian.Uint32(b[4:])), int(binary.BigEndian.Uint32(b[8:]))
	if ancho <= 0 || alto <= 0 || ancho > 4096 || alto > 4096 {
		return nil, errors.New("QOI de tamaño raro")
	}
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	var indice [64][4]uint8
	r, g, bl, a := uint8(0), uint8(0), uint8(0), uint8(255)
	p, racha := 14, 0
	for k := 0; k < ancho*alto; k++ {
		if racha > 0 {
			racha--
		} else {
			if p >= len(b) {
				return nil, errors.New("QOI cortado")
			}
			c := b[p]
			p++
			switch {
			case c == 0xFE:
				if p+3 > len(b) {
					return nil, errors.New("QOI cortado")
				}
				r, g, bl = b[p], b[p+1], b[p+2]
				p += 3
			case c == 0xFF:
				if p+4 > len(b) {
					return nil, errors.New("QOI cortado")
				}
				r, g, bl, a = b[p], b[p+1], b[p+2], b[p+3]
				p += 4
			case c&0xC0 == 0x00:
				r, g, bl, a = indice[c][0], indice[c][1], indice[c][2], indice[c][3]
			case c&0xC0 == 0x40:
				r += (c>>4)&3 - 2
				g += (c>>2)&3 - 2
				bl += c&3 - 2
			case c&0xC0 == 0x80 && p < len(b):
				c2 := b[p]
				p++
				vg := c&0x3F - 32
				r += vg - 8 + (c2>>4)&15
				g += vg
				bl += vg - 8 + c2&15
			case c&0xC0 == 0xC0:
				racha = int(c & 0x3F)
			default:
				return nil, errors.New("QOI cortado")
			}
			indice[hashQoi(r, g, bl, a)] = [4]uint8{r, g, bl, a}
		}
		i := k * 4
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, bl, a
	}
	return img, nil
}

// PNG codifica una imagen en PNG (para mostrársela a quien la pidió).
func PNG(img image.Image) []byte {
	var b bytes.Buffer
	png.Encode(&b, img) //nolint:errcheck
	return b.Bytes()
}

// DesdeRGBA arma una imagen con píxeles RGBA de 8 bits (ancho × alto × 4).
func DesdeRGBA(px []byte, ancho, alto int) *image.RGBA {
	return &image.RGBA{Pix: px, Stride: ancho * 4, Rect: image.Rect(0, 0, ancho, alto)}
}
