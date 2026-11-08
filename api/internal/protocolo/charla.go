package protocolo

// La charla sin IA (v1.10, característica Comando): 0x4C Charlar y su
// respuesta 0xCC, que el robot también manda solo cuando charla desde adentro
// y hace una acción. Ver docs/protocolo-ble.md.

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

// CmdCharlar: [n][texto 1-200 bytes] → [0xCC][código][flags][confianza]...
const CmdCharlar = 0x4C

// MaxTextoCharlar: lo que entra en un 0x4C.
const MaxTextoCharlar = 200

// AccionesCharla: las del u8 acción del 0xCC, en orden (ACCIONES_CHARLA de
// charla_base.h; las mismas que nucleo.RespuestaCharla.Accion).
var AccionesCharla = []string{"ninguna", "alarma", "cancelar-alarma", "posponer", "apagar-alarma", "recordatorio",
	"habito", "jugar", "modo", "gusto", "olvidar-gusto", "toque", "musica", "dormir", "despertar", "dueno",
	"volumen", "callar", "animo"}

// ErrTextoCharlar: vacío, de más de 200 bytes o que no es UTF-8.
var ErrTextoCharlar = errors.New("el texto de la charla es de 1 a 200 bytes UTF-8")

// Charlar arma el marco 0x4C: el robot lo charla él (sin IA).
func Charlar(texto string) ([]byte, error) {
	if len(texto) == 0 || len(texto) > MaxTextoCharlar || !utf8.ValidString(texto) {
		return nil, ErrTextoCharlar
	}
	m := []byte{CmdCharlar, byte(len(texto))}
	return append(m, texto...), nil
}

// RespuestaCharla: el 0xCC (lo que entendió el robot y lo que hizo).
type RespuestaCharla struct {
	Codigo    byte
	NoEntendi bool
	Falta     bool
	HizoAlgo  bool
	Confianza int
	Intencion string
	Accion    string // de AccionesCharla ("" si es una que no se conoce)
	Dato      int    // -1 = nada
	Minuto    int    // del día (-1 = no dijo hora)
	EnMinutos int    // -1 = no
	Dia       int    // días desde hoy (-1 = no dijo)
	Dias      int    // bit 0 lunes ... bit 6 domingo (0 = una vez)
	Texto     string
	Frase     string
}

// LeerCharla decodifica un 0xCC: [0xCC][código] y, con Ok,
// [flags][confianza][n][intención][u8 acción][i8 dato][i16 minuto][i16 en
// minutos][i8 día][u8 días][n][texto][n][frase].
func LeerCharla(b []byte) (RespuestaCharla, error) {
	mal := errors.New("respuesta de charla mal armada")
	if len(b) < 2 || b[0] != CmdCharlar|Respuesta {
		return RespuestaCharla{}, mal
	}
	r := RespuestaCharla{Codigo: b[1], Dato: -1, Minuto: -1, EnMinutos: -1, Dia: -1}
	if r.Codigo != Ok {
		return r, nil
	}
	i := 2
	texto := func() (string, bool) {
		if i >= len(b) || i+1+int(b[i]) > len(b) {
			return "", false
		}
		s := string(b[i+1 : i+1+int(b[i])])
		i += 1 + int(b[i])
		return s, utf8.ValidString(s)
	}
	if len(b) < i+2 {
		return r, mal
	}
	f := b[i]
	r.NoEntendi, r.Falta, r.HizoAlgo = f&1 != 0, f&2 != 0, f&4 != 0
	r.Confianza = int(b[i+1])
	i += 2
	var ok bool
	if r.Intencion, ok = texto(); !ok {
		return r, mal
	}
	if len(b) < i+8 {
		return r, mal
	}
	if a := int(b[i]); a < len(AccionesCharla) {
		r.Accion = AccionesCharla[a]
	}
	r.Dato = int(int8(b[i+1]))
	r.Minuto = int(int16(binary.LittleEndian.Uint16(b[i+2:])))
	r.EnMinutos = int(int16(binary.LittleEndian.Uint16(b[i+4:])))
	r.Dia = int(int8(b[i+6]))
	r.Dias = int(b[i+7])
	i += 8
	if r.Texto, ok = texto(); !ok {
		return r, mal
	}
	if r.Frase, ok = texto(); !ok {
		return r, mal
	}
	return r, nil
}

// ArmarCharla arma un 0xCC (para el robot falso de los tests y el bicho virtual).
func ArmarCharla(r RespuestaCharla) []byte {
	f := byte(0)
	if r.NoEntendi {
		f |= 1
	}
	if r.Falta {
		f |= 2
	}
	if r.HizoAlgo {
		f |= 4
	}
	a := 0
	for k, n := range AccionesCharla {
		if n == r.Accion {
			a = k
		}
	}
	m := []byte{CmdCharlar | Respuesta, Ok, f, byte(r.Confianza)}
	m = conLargo(m, r.Intencion, 24)
	m = append(m, byte(a), byte(int8(r.Dato)))
	m = binary.LittleEndian.AppendUint16(m, uint16(int16(r.Minuto)))
	m = binary.LittleEndian.AppendUint16(m, uint16(int16(r.EnMinutos)))
	m = append(m, byte(int8(r.Dia)), byte(r.Dias))
	m = conLargo(m, r.Texto, 40)
	return conLargo(m, r.Frase, MaxDecir)
}
