package protocolo

// Los marcos de la v1.6 (característica Comando): que el robot diga algo en un
// globo, el clima para su modo clima y el despertador. Ver docs/protocolo-ble.md.

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf8"
)

// Operaciones de la v1.6.
const (
	CmdDecir  = 0x40 // [n][expresión][texto]
	CmdClima  = 0x41 // [i16 décimas][u8 WMO][i8 máx][i8 mín][u8 flags][n][ciudad]
	CmdAlarma = 0x42 // [u8 acción]
)

// Límites de los marcos de la v1.6.
const (
	MaxDecir        = 60 // bytes de texto
	MaxCiudadClima  = 24 // bytes de ciudad en el marco del clima
	MaxExpresionCmd = 40
)

// ErrTextoDecir: el texto es de 1 a 60 bytes de UTF-8 válido.
var ErrTextoDecir = errors.New("el texto es de 1 a 60 bytes (una tilde ocupa 2)")

// Decir arma el marco 0x40: el robot muestra el texto en un globo, con esa
// expresión ("" = la que tenga).
func Decir(texto, expresion string) ([]byte, error) {
	if len(texto) < 1 || len(texto) > MaxDecir || !utf8.ValidString(texto) {
		return nil, ErrTextoDecir
	}
	if len(expresion) > MaxExpresionCmd || !utf8.ValidString(expresion) {
		return nil, errors.New("la expresión es de hasta 40 bytes")
	}
	m := []byte{CmdDecir, byte(len(expresion))}
	m = append(m, expresion...)
	return append(m, texto...), nil
}

// Clima arma el marco 0x41. temp en °C (se manda en décimas), max y min en
// °C; la ciudad se corta a 24 bytes sin partir una letra.
func Clima(temp float64, codigo int, max, min int, dia bool, ciudad string) []byte {
	d := math.Round(temp * 10)
	d = math.Max(math.MinInt16, math.Min(math.MaxInt16, d))
	m := []byte{CmdClima}
	m = binary.LittleEndian.AppendUint16(m, uint16(int16(d)))
	var flags byte
	if dia {
		flags |= 1
	}
	ciudad = CortarUTF8(ciudad, MaxCiudadClima)
	m = append(m, byte(entre(codigo, 0, 255)), byte(int8(entre(max, -128, 127))), byte(int8(entre(min, -128, 127))), flags, byte(len(ciudad)))
	return append(m, ciudad...)
}

func entre(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// AccionesAlarma, en el orden del protocolo (0 apagar, 1 posponer, 2 sonar
// ya para probarla, 3 despertarlo sin alarma).
var AccionesAlarma = []string{"apagar", "posponer", "probar", "despertar"}

// Alarma arma el marco 0x42.
func Alarma(accion string) ([]byte, error) {
	for i, a := range AccionesAlarma {
		if a == accion {
			return []byte{CmdAlarma, byte(i)}, nil
		}
	}
	return nil, errors.New("la acción de la alarma es apagar, posponer, probar o despertar")
}

// CortarUTF8 deja s en max bytes sin partir una letra.
func CortarUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
