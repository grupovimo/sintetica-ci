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

func TestAvisosJuegoYAnimo(t *testing.T) {
	// v1.7: [0x9E][0x00][0x02][u32 s][0x00][34][u32 s][0x05][juego·16 + resultado]
	// y lo mismo con el 35 y el tipo 6. Cuentan como uno (sale de la entrada nueva).
	var l lectorRegistro
	got := l.nuevas([]protocolo.EntradaRegistro{{Segundos: 70, Tipo: protocolo.RegistroEvento, Dato: 34},
		{Segundos: 70, Tipo: protocolo.RegistroJuego, Dato: 3*16 + 1}})
	if len(got) != 1 || got[0].nombre != "juego" || got[0].datos["juego"] != "dado" || got[0].datos["resultado"] != "gano" {
		t.Fatalf("juego: %+v", got)
	}
	got = l.nuevas([]protocolo.EntradaRegistro{{Segundos: 80, Tipo: protocolo.RegistroEvento, Dato: 35},
		{Segundos: 80, Tipo: protocolo.RegistroAnimo, Dato: 2}})
	if len(got) != 1 || got[0].nombre != "animo" || got[0].datos["valor"] != "mal" {
		t.Fatalf("ánimo: %+v", got)
	}
	if CuandoDelRegistro("juego") || CuandoDelRegistro("animo") || !CuandoDelRegistro("bicho-cerca") {
		t.Fatal("juego y animo los avisa solo; bicho-cerca sale del registro")
	}
}
