package nube

import (
	"sintetica/api/internal/protocolo"
)

// Lo que la API detecta de verdad para las automatizaciones (ver docs/nube.md,
// "Integraciones"):
//
//   - Del estado (llega solo, sin pedir nada): "bateria-baja" (cruza de ≥ 15 %
//     a < 15 %), "expresion" (cambió la cara visible), "duerme", "despierta".
//   - De la conexión: "conectado", "desconectado".
//   - De los avisos que el bicho manda solo (un 0x9E sin que nadie lo pida):
//     "recordatorio" (con el índice en la config), "pomodoro", "despertador"
//     (al empezar a sonar la alarma, v1.6), "juego" (terminó una partida:
//     registro tipo 5) y "animo" (respondió el "¿Cómo estás?": tipo 6), v1.7.
//   - Del registro del motor de vida (comando 0x1E, que la API le pide cada
//     10 s solo a los bichos con automatizaciones que lo necesitan): los
//     eventos de EventosRegistro y "hito".

// EventosRegistro: los eventos del registro (tipo 0) que se pueden usar en
// una automatización, con el nombre que tienen en el campo "cuando".
// "enchufado" se llama "cargando". "recordatorio" no está: sale de la entrada
// de tipo 4, que trae su índice (el bicho anota las dos).
var EventosRegistro = map[string]string{
	"tacto": "tacto", "doble-toque": "doble-toque", "caricia": "caricia", "muchos-toques": "muchos-toques",
	"levantado": "levantado", "sacudida": "sacudida", "sacudida-suave": "sacudida-suave",
	"sacudida-fuerte": "sacudida-fuerte", "boca-abajo": "boca-abajo", "inclinado": "inclinado", "caida": "caida",
	"voz": "voz", "nombre": "nombre", "ruido": "ruido", "musica": "musica",
	"enchufado": "cargando", "desenchufado": "desenchufado", "cargado": "cargado", "volvio": "volvio",
	"pomodoro": "pomodoro", "despertador": "despertador", "bicho-cerca": "bicho-cerca",
}

// avisaSolo: los eventos que el bicho avisa solo (no hace falta pedirle el registro).
func avisaSolo(evento string) bool {
	switch evento {
	case "recordatorio", "pomodoro", "despertador", "juego", "animo":
		return true
	}
	return false
}

// CuandoDelRegistro dice si ese "cuando" sale del registro (hay que pedírselo
// al bicho). "recordatorio", "pomodoro", "despertador", "juego" y "animo" no:
// el bicho los avisa solo.
func CuandoDelRegistro(cuando string) bool {
	if cuando == "hito" {
		return true
	}
	if avisaSolo(cuando) {
		return false
	}
	for _, c := range EventosRegistro {
		if c == cuando {
			return true
		}
	}
	return false
}

// UmbralBateria: debajo de esto es "bateria-baja".
const UmbralBateria = 15

// cambiosDeEstado avisa lo que cambió entre dos estados.
func (t *trabajador) cambiosDeEstado(antes, ahora protocolo.Estado) {
	if antes.Bateria != nil && ahora.Bateria != nil && *antes.Bateria >= UmbralBateria && *ahora.Bateria < UmbralBateria {
		t.n.avisar(t.id, "bateria-baja", map[string]any{"bateria": *ahora.Bateria})
	}
	if !antes.Dormido && ahora.Dormido {
		t.n.avisar(t.id, "duerme", map[string]any{})
	} else if antes.Dormido && !ahora.Dormido {
		t.n.avisar(t.id, "despierta", map[string]any{})
	}
	if antes.Expresion != ahora.Expresion && ahora.Expresion != "" {
		t.n.avisar(t.id, "expresion", map[string]any{"valor": ahora.Expresion, "antes": antes.Expresion})
	}
}

type eventoRegistro struct {
	nombre string
	datos  map[string]any
}

// lectorRegistro se acuerda de hasta dónde leyó el registro del bicho (que
// devuelve siempre sus últimas entradas): así cada una se avisa una vez. La
// primera lectura (al conectarse o después de un reinicio) solo marca el
// punto de partida. Los avisos (un 0x9E que el bicho manda solo, con un
// recordatorio o un pomodoro) se atienden siempre y no mueven el punto: se
// recuerdan para no repetirlos cuando aparezcan en el registro.
type lectorRegistro struct {
	listo    bool
	ultimo   uint32 // los segundos de la última entrada vista
	enUltimo int    // cuántas entradas había en ese mismo segundo
	avisados map[protocolo.EntradaRegistro]int
}

func (l *lectorRegistro) reiniciar() { *l = lectorRegistro{} }

// esAviso: solo trae entradas que el bicho avisa solo.
func esAviso(r []protocolo.EntradaRegistro) bool {
	for _, e := range r {
		if e.Tipo == protocolo.RegistroRecordatorio || e.Tipo == protocolo.RegistroJuego || e.Tipo == protocolo.RegistroAnimo {
			continue
		}
		if e.Tipo == protocolo.RegistroEvento && int(e.Dato) < len(protocolo.Eventos) {
			if avisaSolo(protocolo.Eventos[e.Dato]) {
				continue
			}
		}
		return false
	}
	return len(r) > 0
}

func (l *lectorRegistro) nuevas(r []protocolo.EntradaRegistro) []eventoRegistro {
	if len(r) == 0 {
		return nil
	}
	if esAviso(r) {
		if l.avisados == nil || len(l.avisados) > 64 {
			l.avisados = map[protocolo.EntradaRegistro]int{}
		}
		var salida []eventoRegistro
		for _, e := range r {
			l.avisados[e]++
			if ev, ok := traducirRegistro(e); ok {
				salida = append(salida, ev)
			}
		}
		return salida
	}
	fin := r[len(r)-1].Segundos
	marcar := func() {
		l.listo, l.ultimo, l.enUltimo = true, fin, 0
		for _, e := range r {
			if e.Segundos == fin {
				l.enUltimo++
			}
		}
		for e := range l.avisados {
			if e.Segundos < fin {
				delete(l.avisados, e)
			}
		}
	}
	if !l.listo || fin < l.ultimo {
		// Primera vez o reinició sin que lo viéramos: solo el punto de partida.
		l.avisados = nil
		marcar()
		return nil
	}
	var salida []eventoRegistro
	vistos := 0
	for _, e := range r {
		if e.Segundos < l.ultimo {
			continue
		}
		if e.Segundos == l.ultimo {
			if vistos++; vistos <= l.enUltimo {
				continue
			}
		}
		if l.avisados[e] > 0 {
			l.avisados[e]--
			continue
		}
		if ev, ok := traducirRegistro(e); ok {
			salida = append(salida, ev)
		}
	}
	marcar()
	return salida
}

func traducirRegistro(e protocolo.EntradaRegistro) (eventoRegistro, bool) {
	switch e.Tipo {
	case protocolo.RegistroEvento:
		if int(e.Dato) >= len(protocolo.Eventos) {
			return eventoRegistro{}, false
		}
		c, ok := EventosRegistro[protocolo.Eventos[e.Dato]]
		return eventoRegistro{c, map[string]any{}}, ok
	case protocolo.RegistroHito:
		return eventoRegistro{"hito", map[string]any{}}, true
	case protocolo.RegistroRecordatorio:
		return eventoRegistro{"recordatorio", map[string]any{"indice": int(e.Dato)}}, true
	case protocolo.RegistroJuego:
		j, r := protocolo.LeerJuego(e.Dato)
		return eventoRegistro{"juego", map[string]any{"juego": j, "resultado": r}}, true
	case protocolo.RegistroAnimo:
		if int(e.Dato) >= len(protocolo.Animos) {
			return eventoRegistro{}, false
		}
		return eventoRegistro{"animo", map[string]any{"valor": protocolo.Animos[e.Dato]}}, true
	}
	return eventoRegistro{}, false
}
