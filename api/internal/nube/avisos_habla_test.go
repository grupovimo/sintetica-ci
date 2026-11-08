package nube

import (
	"testing"

	"sintetica/api/internal/protocolo"
)

func TestAvisoDespertador(t *testing.T) {
	// El evento 33 (v1.6): [0x9E][0x00][0x01][u32 s][0x00][0x21].
	if protocolo.Eventos[33] != "despertador" {
		t.Fatalf("el 33 es %q", protocolo.Eventos[33])
	}
	e := protocolo.EntradaRegistro{Segundos: 50, Tipo: protocolo.RegistroEvento, Dato: 33}
	var l lectorRegistro
	got := l.nuevas([]protocolo.EntradaRegistro{e})
	if len(got) != 1 || got[0].nombre != "despertador" {
		t.Fatalf("aviso: %+v", got)
	}
	if CuandoDelRegistro("despertador") || CuandoDelRegistro("hora") {
		t.Fatal("despertador y hora no necesitan pedir el registro")
	}
}
