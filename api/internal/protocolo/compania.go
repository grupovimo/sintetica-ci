package protocolo

// Los marcos de la v1.9 (característica Comando): los toques de los amigos, la
// agenda, la presencia, los hábitos y la música. Ver docs/protocolo-ble.md.

import (
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf8"
)

// Operaciones de la v1.9.
const (
	CmdToque     = 0x47 // [u8 tipo][n][de][n][expresión]
	CmdAgenda    = 0x48 // [u16 minutos][u16 duración][n][título] (minutos 0xFFFF = el resumen de mañana)
	CmdPresencia = 0x49 // [u8 0 llegaste, 1 te fuiste]
	CmdHabito    = 0x4A // [u8 índice][u8 0 hecho, 1 deshacer][u8 racha]
	CmdMusica    = 0x4B // [u8 bpm][u8 flags: bit 0 sonando][n][título][n][artista]
)

// Tipos de las entradas del registro de la v1.9.
const (
	RegistroToque  = 7 // dato: el tipo de toque
	RegistroHabito = 8 // dato: el índice del hábito
)

// TiposRegistro: el nombre de cada tipo de entrada del registro, en orden.
var TiposRegistro = []string{"evento", "comportamiento", "dormir", "hito", "recordatorio", "juego", "animo", "toque", "habito"}

// TiposToque, en el orden del protocolo.
var TiposToque = []string{"toque", "abrazo", "beso", "carita"}

// TiposAviso: los del 0x44 (los dos primeros de la v1.7, el resto de la v1.9).
var TiposAviso = []string{"notificacion", "llamada", "pago", "mail", "github", "pedido", "otro"}

// Límites de los textos de la v1.9 (en bytes; se cortan sin partir letras).
const (
	MaxDeToque        = 24
	MaxExpresionToque = 24
	MaxTituloAgenda   = 40
	MaxTituloMusica   = 40
	MaxArtistaMusica  = 24
	MaxHabitos        = 4
)

// ResumenMananaMinutos: los minutos del 0x48 que lo hacen el resumen de mañana.
const ResumenMananaMinutos = 0xFFFF

func conLargo(m []byte, s string, max int) []byte {
	s = CortarUTF8(strings.TrimSpace(s), max)
	m = append(m, byte(len(s)))
	return append(m, s...)
}

// NumeroToque: el número de un tipo de toque ("" = toque).
func NumeroToque(tipo string) (int, bool) {
	tipo = strings.ToLower(strings.TrimSpace(tipo))
	if tipo == "" {
		return 0, true
	}
	for i, t := range TiposToque {
		if t == tipo {
			return i, true
		}
	}
	return 0, false
}

// Toque arma el marco 0x47: un toque de un amigo (tipo toque, abrazo, beso o
// carita), quién lo manda (1 a 24 bytes) y, para la carita, la expresión.
func Toque(tipo, de, expresion string) ([]byte, error) {
	t, ok := NumeroToque(tipo)
	if !ok {
		return nil, errors.New("el toque es toque, abrazo, beso o carita")
	}
	if !utf8.ValidString(de) || !utf8.ValidString(expresion) {
		return nil, errors.New("el nombre y la expresión son texto (UTF-8)")
	}
	if CortarUTF8(strings.TrimSpace(de), MaxDeToque) == "" {
		return nil, errors.New("falta de quién es el toque")
	}
	m := conLargo([]byte{CmdToque, byte(t)}, de, MaxDeToque)
	return conLargo(m, expresion, MaxExpresionToque), nil
}

// Agenda arma el marco 0x48: el próximo evento del calendario, en cuántos
// minutos empieza (0 a 0xFFFE) y cuánto dura; el título de 1 a 40 bytes.
func Agenda(minutos, duracion int, titulo string) ([]byte, error) {
	return agenda(entre(minutos, 0, ResumenMananaMinutos-1), duracion, titulo)
}

// ResumenManana arma el 0x48 del resumen de mañana: cuántos eventos hay (en
// el campo de la duración) y el título del primero.
func ResumenManana(cantidad int, titulo string) ([]byte, error) {
	return agenda(ResumenMananaMinutos, cantidad, titulo)
}

func agenda(minutos, duracion int, titulo string) ([]byte, error) {
	if !utf8.ValidString(titulo) || CortarUTF8(strings.TrimSpace(titulo), MaxTituloAgenda) == "" {
		return nil, errors.New("el título es de 1 a 40 bytes de texto")
	}
	m := binary.LittleEndian.AppendUint16([]byte{CmdAgenda}, uint16(minutos))
	m = binary.LittleEndian.AppendUint16(m, uint16(entre(duracion, 0, 0xFFFF)))
	return conLargo(m, titulo, MaxTituloAgenda), nil
}

// Presencia arma el marco 0x49: llegaste (0) o te fuiste (1).
func Presencia(llegaste bool) []byte {
	if llegaste {
		return []byte{CmdPresencia, 0}
	}
	return []byte{CmdPresencia, 1}
}

// Habito arma el marco 0x4A: el hábito índice (0 a 3) hecho o deshecho, con
// la racha en días (se corta a 255).
func Habito(indice int, hecho bool, racha int) ([]byte, error) {
	if indice < 0 || indice >= MaxHabitos {
		return nil, errors.New("el hábito es del 0 al 3")
	}
	var accion byte = 1
	if hecho {
		accion = 0
	}
	return []byte{CmdHabito, byte(indice), accion, byte(entre(racha, 0, 255))}, nil
}

// Musica arma el marco 0x4B: el bpm (0 = no se sabe; se corta a 255), si está
// sonando, el título (0 a 40 bytes) y el artista (0 a 24).
func Musica(bpm int, sonando bool, titulo, artista string) []byte {
	var flags byte
	if sonando {
		flags |= 1
	}
	if !utf8.ValidString(titulo) {
		titulo = ""
	}
	if !utf8.ValidString(artista) {
		artista = ""
	}
	m := conLargo([]byte{CmdMusica, byte(entre(bpm, 0, 255)), flags}, titulo, MaxTituloMusica)
	return conLargo(m, artista, MaxArtistaMusica)
}
