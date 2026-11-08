package nube

import (
	"testing"

	"sintetica/api/internal/protocolo"
)

func numeroEvento(t *testing.T, nombre string) uint8 {
	t.Helper()
	for i, e := range protocolo.Eventos {
		if e == nombre {
			return uint8(i)
		}
	}
	t.Fatalf("no hay evento %q", nombre)
	return 0
}

func TestAvisosCompania(t *testing.T) {
	// v1.9: devolver-toque (40) y el hábito (44 + registro tipo 8) los avisa solo.
	var l lectorRegistro
	got := l.nuevas([]protocolo.EntradaRegistro{{Segundos: 90, Tipo: protocolo.RegistroEvento, Dato: numeroEvento(t, "devolver-toque")}})
	if len(got) != 1 || got[0].nombre != "devolver-toque" {
		t.Fatalf("devolver: %+v", got)
	}
	got = l.nuevas([]protocolo.EntradaRegistro{{Segundos: 91, Tipo: protocolo.RegistroEvento, Dato: numeroEvento(t, "habito")},
		{Segundos: 91, Tipo: protocolo.RegistroHabito, Dato: 2}})
	if len(got) != 1 || got[0].nombre != "habito" || got[0].datos["indice"] != 2 {
		t.Fatalf("hábito: %+v", got)
	}
	got = l.nuevas([]protocolo.EntradaRegistro{{Segundos: 92, Tipo: protocolo.RegistroEvento, Dato: numeroEvento(t, "llegaste")},
		{Segundos: 92, Tipo: protocolo.RegistroToque, Dato: 1}})
	if len(got) != 1 || got[0].nombre != "llegaste" {
		t.Fatalf("llegaste: %+v", got)
	}
	if CuandoDelRegistro("llegaste") || CuandoDelRegistro("te-fuiste") || CuandoDelRegistro("habito") {
		t.Fatal("la presencia y el hábito los avisa solo")
	}
}
