package protocolo

// El diario que escribe el robot (motor de vida v3, característica Comando):
// 0x4D Diario y su respuesta 0xCD, que también llega sola como aviso cuando el
// robot escribe una línea (a la noche). Ver docs/protocolo-ble.md, "Diario".

import (
	"errors"
	"unicode/utf8"
)

// CmdDiario: (sin datos) → una notificación 0xCD por línea.
const CmdDiario = 0x4D

// RegistroDiario: el tipo de entrada del registro (0x1E) cuando escribe una línea (dato: el tipo de episodio).
const RegistroDiario = 9

// DiarioTranquilo: el tipo de una línea sin episodio ("Hoy fue un día tranquilo").
const DiarioTranquilo = 255

// TiposEpisodio: lo que recuerda (el tipo de cada línea), en el orden del núcleo.
var TiposEpisodio = []string{"gane", "perdi", "empate", "toque", "animo-bien", "animo-medio", "animo-mal", "tarde",
	"posponer", "habito", "caricias", "sacudida", "musica", "volviste", "solo", "crecio"}

// Etapas: cómo va creciendo el vínculo (shared/formato/vida/caracter.json), en orden.
var Etapas = []string{"recien-llegado", "conociendonos", "confiado", "companero"}

// Diario arma el pedido del diario.
func Diario() []byte { return []byte{CmdDiario} }

// LineaDiario: una línea del 0xCD. Dia es (epoch local − 4 h) / 86400 (lo de la
// madrugada es del día anterior). Total = 0: el diario está vacío.
type LineaDiario struct {
	Dia    uint16
	Indice byte
	Total  byte
	Tipo   string // un TiposEpisodio o "tranquilo"
	Dato   byte   // el juego, el tipo de toque, el hábito, cuántas veces…
	Etapa  string
	Texto  string
}

var errDiario = errors.New("no es una línea del diario")

// LeerDiario lee un 0xCD: [0xCD][0][u16 día][u8 i][u8 n][u8 tipo][u8 dato][u8 etapa][texto UTF-8].
func LeerDiario(b []byte) (LineaDiario, error) {
	if len(b) < 9 || b[0] != CmdDiario|Respuesta || b[1] != Ok || !utf8.Valid(b[9:]) {
		return LineaDiario{}, errDiario
	}
	l := LineaDiario{Dia: uint16(b[2]) | uint16(b[3])<<8, Indice: b[4], Total: b[5], Dato: b[7], Texto: string(b[9:])}
	switch {
	case b[6] == DiarioTranquilo:
		l.Tipo = "tranquilo"
	case int(b[6]) < len(TiposEpisodio):
		l.Tipo = TiposEpisodio[b[6]]
	default:
		return LineaDiario{}, errDiario
	}
	if int(b[8]) < len(Etapas) {
		l.Etapa = Etapas[b[8]]
	}
	return l, nil
}
