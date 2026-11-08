package nucleo

/*
#include <stdlib.h>
#include "sintetica_c.h"
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// La charla sin IA (firmware/nucleo/charla.h): el MISMO motor que corre en el
// robot. Charlar es una función pura (no toca el bicho de este proceso): se
// puede llamar desde varias goroutines sin el mutex.

// LargoEstadoCharla es el tamaño del contexto entre frases que guarda quien llama.
const LargoEstadoCharla = C.SINTETICA_LARGO_ESTADO_CHARLA

// EstadoCharla es lo que recuerda entre frases ("¿y mañana?" después del
// clima, "¿a qué hora?"). Vacío (en ceros) la primera vez; guardar uno por robot.
type EstadoCharla [LargoEstadoCharla]byte

// ContextoCharla es lo que sabe la nube del robot y de su dueño. Usar
// NuevoContextoCharla: Minuto y DiasJuntos en -1 son "no se sabe".
type ContextoCharla struct {
	Dueno, Robot  string
	Preset        string // la personalidad ("grunon", "carinoso"...): elige las variantes
	Minuto        int    // del día, hora local (-1 = no se sabe)
	Epoch         uint32 // hora local en segundos desde 1970 (0 = no se sabe el día)
	Clima         string // "11° y despejado"
	ClimaManana   string
	Agenda        string // "Dentista a las 15"
	AgendaManana  string
	Musica        string
	Habitos       []string // hasta 4, en el orden de la config (el índice es el de la acción)
	Gustos        []string
	DiasJuntos    int // -1 = no se sabe
	AlarmaSonando bool
	Segundos      uint32 // un reloj que avanza: el contexto dura 90 s
	Semilla       uint32
}

// NuevoContextoCharla da un contexto sin hora ni días juntos.
func NuevoContextoCharla() ContextoCharla { return ContextoCharla{Minuto: -1, DiasJuntos: -1} }

// RespuestaCharla es lo que entendió y contesta.
type RespuestaCharla struct {
	Intencion      string // "" = no entendió
	Confianza      int    // 0-100
	NoEntendi      bool   // si hay IA, que conteste ella
	Falta          bool   // preguntó lo que faltaba (la acción queda para la próxima)
	Accion         string // "ninguna", "alarma", "recordatorio", "habito", "jugar", "modo", "gusto", "toque", ...
	Dato           int    // el juego, el modo, el toque, el hábito, ±1 de volumen o el ánimo (-1 = nada)
	Minuto         int    // del día (-1 = no dijo hora)
	EnMinutos      int    // "en 10 minutos" (-1)
	Dia            int    // días desde hoy (-1)
	Semana         int    // 0 lunes ... 6 domingo (-1)
	Dias           int    // repetir: bit 0 lunes ... bit 6 domingo (0 = una vez)
	Texto          string // el recordatorio, el gusto, el nombre, a quién
	Frase          string // lo que contesta (hasta 60 bytes, para el globo: 0x40 Decir)
	Expresion      string
	Comportamiento string
}

// ErrTextoCharla es un texto vacío, de más de 200 bytes o que no es UTF-8.
var ErrTextoCharla = errors.New("charla: texto de 1 a 200 bytes UTF-8")

// Charlar entiende `texto` con el motor propio (sin IA, sin red) y actualiza
// el estado. La idea: la nube prueba esto primero y solo si NoEntendi y hay
// IA, va a la IA.
func Charlar(texto string, ctx ContextoCharla, estado *EstadoCharla) (RespuestaCharla, error) {
	if len(texto) == 0 || len(texto) > 200 || !utf8.ValidString(texto) {
		return RespuestaCharla{}, ErrTextoCharla
	}
	var libres []unsafe.Pointer
	cs := func(s string) *C.char {
		if s == "" {
			return nil
		}
		p := C.CString(s)
		libres = append(libres, unsafe.Pointer(p))
		return p
	}
	defer func() {
		for _, p := range libres {
			C.free(p)
		}
	}()
	var c C.sintetica_charla_ctx
	c.dueno, c.robot, c.preset = cs(ctx.Dueno), cs(ctx.Robot), cs(ctx.Preset)
	c.minuto_del_dia = C.int(ctx.Minuto)
	c.epoch = C.uint32_t(ctx.Epoch)
	c.clima, c.clima_manana = cs(ctx.Clima), cs(ctx.ClimaManana)
	c.agenda, c.agenda_manana = cs(ctx.Agenda), cs(ctx.AgendaManana)
	c.musica = cs(ctx.Musica)
	for i, h := range ctx.Habitos {
		if i >= 4 {
			break
		}
		c.habitos[i] = cs(h)
		c.n_habitos = C.int(i + 1)
	}
	c.gustos = cs(strings.Join(ctx.Gustos, "\n"))
	c.dias_juntos = C.int(ctx.DiasJuntos)
	c.alarma_sonando = b2i(ctx.AlarmaSonando)
	c.segundos = C.uint32_t(ctx.Segundos)
	c.semilla = C.uint32_t(ctx.Semilla)
	ct := C.CString(texto)
	defer C.free(unsafe.Pointer(ct))
	var s C.sintetica_charla
	if r := C.sintetica_charlar_con(&c, ct, C.size_t(len(texto)), (*C.uint8_t)(unsafe.Pointer(&estado[0])), &s); r < 0 {
		return RespuestaCharla{}, ErrTextoCharla
	}
	return RespuestaCharla{
		Intencion: C.GoString(&s.intencion[0]), Confianza: int(s.confianza), NoEntendi: s.no_entendi != 0,
		Falta: s.falta != 0, Accion: C.GoString(&s.accion_nombre[0]), Dato: int(s.dato), Minuto: int(s.minuto),
		EnMinutos: int(s.en_minutos), Dia: int(s.dia), Semana: int(s.semana), Dias: int(s.dias),
		Texto: C.GoString(&s.texto[0]), Frase: C.GoString(&s.frase[0]), Expresion: C.GoString(&s.expresion[0]),
		Comportamiento: C.GoString(&s.comportamiento[0]),
	}, nil
}

// VozPropia dice `texto` con la voz del robot (el balbuceo): PCM de 16 bits,
// 16 kHz, mono. tipo: 0 grave, 1 media, 2 aguda, 3 robot; volumen 0-100;
// energía y ánimo de 0 a 1.
func VozPropia(texto string, tipo, volumen int, energia, animo float32) []int16 {
	mu.Lock()
	defer mu.Unlock()
	ct := C.CString(texto)
	defer C.free(unsafe.Pointer(ct))
	total := int(C.sintetica_voz_empezar(ct, C.int(tipo), C.int(volumen), C.float(energia), C.float(animo)))
	pcm := make([]int16, total)
	hechas := 0
	for hechas < total {
		n := int(C.sintetica_voz_generar((*C.int16_t)(unsafe.Pointer(&pcm[hechas])), C.size_t(total-hechas)))
		if n == 0 {
			break
		}
		hechas += n
	}
	return pcm[:hechas]
}

// WAV arma un .wav (PCM de 16 bits, 16 kHz, mono) con esas muestras.
func WAV(pcm []int16) []byte {
	const tasa = 16000
	datos := len(pcm) * 2
	b := make([]byte, 44+datos)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+datos))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], tasa)
	binary.LittleEndian.PutUint32(b[28:], tasa*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(datos))
	for i, m := range pcm {
		binary.LittleEndian.PutUint16(b[44+2*i:], uint16(m))
	}
	return b
}
