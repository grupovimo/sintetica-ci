package nube

import (
	"testing"

	"sintetica/api/internal/protocolo"
)

func TestLectorRegistro(t *testing.T) {
	ev := func(seg uint32, nombre string) protocolo.EntradaRegistro {
		for i, e := range protocolo.Eventos {
			if e == nombre {
				return protocolo.EntradaRegistro{Segundos: seg, Tipo: protocolo.RegistroEvento, Dato: uint8(i)}
			}
		}
		t.Fatalf("no existe %s", nombre)
		return protocolo.EntradaRegistro{}
	}
	nombres := func(l []eventoRegistro) []string {
		var s []string
		for _, e := range l {
			s = append(s, e.nombre)
		}
		return s
	}
	var l lectorRegistro
	// La primera lectura solo marca el punto de partida.
	if n := l.nuevas([]protocolo.EntradaRegistro{ev(10, "tacto"), ev(20, "caricia")}); len(n) != 0 {
		t.Fatalf("primera: %v", nombres(n))
	}
	// Lo nuevo, una vez (incluso dos en el mismo segundo que la última vista).
	n := l.nuevas([]protocolo.EntradaRegistro{ev(10, "tacto"), ev(20, "caricia"), ev(20, "sacudida"), ev(25, "enchufado"),
		{Segundos: 26, Tipo: protocolo.RegistroComportamiento, Dato: 3}, {Segundos: 27, Tipo: protocolo.RegistroRecordatorio, Dato: 1}})
	if got := nombres(n); len(got) != 3 || got[0] != "sacudida" || got[1] != "cargando" || got[2] != "recordatorio" {
		t.Fatalf("nuevas: %v", got)
	}
	if n[2].datos["indice"] != 1 {
		t.Fatalf("índice del recordatorio: %v", n[2].datos)
	}
	if n := l.nuevas([]protocolo.EntradaRegistro{ev(20, "sacudida"), ev(25, "enchufado")}); len(n) != 0 {
		t.Fatalf("repetidas: %v", nombres(n))
	}
	// Reinició (los segundos vuelven a empezar): punto de partida de nuevo.
	if n := l.nuevas([]protocolo.EntradaRegistro{ev(3, "tacto")}); len(n) != 0 {
		t.Fatalf("reinicio: %v", nombres(n))
	}
	if got := nombres(l.nuevas([]protocolo.EntradaRegistro{ev(3, "tacto"), ev(5, "doble-toque")})); len(got) != 1 || got[0] != "doble-toque" {
		t.Fatalf("después del reinicio: %v", got)
	}

	// Los avisos que manda el bicho solo: se atienden aunque no se haya leído
	// el registro, una sola vez (el evento 31 y la entrada de tipo 4 son uno).
	var m lectorRegistro
	rec := protocolo.EntradaRegistro{Segundos: 40, Tipo: protocolo.RegistroRecordatorio, Dato: 2}
	got := m.nuevas([]protocolo.EntradaRegistro{ev(40, "recordatorio"), rec})
	if len(got) != 1 || got[0].nombre != "recordatorio" || got[0].datos["indice"] != 2 {
		t.Fatalf("aviso: %v", nombres(got))
	}
	if got := nombres(m.nuevas([]protocolo.EntradaRegistro{ev(41, "pomodoro")})); len(got) != 1 || got[0] != "pomodoro" {
		t.Fatalf("pomodoro: %v", got)
	}
	// Con el registro ya leído, un aviso no se pierde lo de antes ni se repite.
	var p lectorRegistro
	p.nuevas([]protocolo.EntradaRegistro{ev(10, "tacto")})
	if got := nombres(p.nuevas([]protocolo.EntradaRegistro{ev(40, "recordatorio"), rec})); len(got) != 1 {
		t.Fatalf("aviso con registro: %v", got)
	}
	got = p.nuevas([]protocolo.EntradaRegistro{ev(10, "tacto"), ev(30, "caricia"), ev(40, "recordatorio"), rec})
	if len(got) != 1 || got[0].nombre != "caricia" {
		t.Fatalf("el registro después del aviso: %v", nombres(got))
	}
}

type oyenteDePrueba struct{ eventos []string }

func (o *oyenteDePrueba) Evento(_, e string, _ map[string]any) { o.eventos = append(o.eventos, e) }
func (o *oyenteDePrueba) QuiereRegistro(string) bool           { return false }

func TestCambiosDeEstado(t *testing.T) {
	o := &oyenteDePrueba{}
	tr := &trabajador{n: &Nube{oyente: o}, id: "AABBCCDDEEFF"}
	b := func(v int) *int { return &v }
	tr.cambiosDeEstado(protocolo.Estado{Bateria: b(15), Expresion: "neutral"}, protocolo.Estado{Bateria: b(14), Expresion: "contento", Dormido: true})
	tr.cambiosDeEstado(protocolo.Estado{Bateria: b(14), Dormido: true, Expresion: "contento"}, protocolo.Estado{Bateria: b(10), Expresion: "contento"})
	got := o.eventos
	if len(got) != 4 || got[0] != "bateria-baja" || got[1] != "duerme" || got[2] != "expresion" || got[3] != "despierta" {
		t.Fatalf("eventos: %v", got)
	}
}
