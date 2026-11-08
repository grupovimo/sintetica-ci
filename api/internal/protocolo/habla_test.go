package protocolo

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecir(t *testing.T) {
	m, err := Decir("hola", "contento")
	if err != nil || !bytes.Equal(m, append([]byte{0x40, 8}, "contentohola"...)) {
		t.Fatalf("%x %v", m, err)
	}
	m, err = Decir("¡Buen día!", "")
	if err != nil || !bytes.Equal(m, append([]byte{0x40, 0}, "¡Buen día!"...)) {
		t.Fatalf("%x %v", m, err)
	}
	for _, malo := range []string{"", strings.Repeat("a", 61), "\xff"} {
		if _, err := Decir(malo, ""); err == nil {
			t.Errorf("%q tenía que fallar", malo)
		}
	}
	if _, err := Decir(strings.Repeat("a", 60), ""); err != nil {
		t.Fatal("60 bytes entran")
	}
}

func TestClima(t *testing.T) {
	m := Clima(18.44, 3, 22, 12, true, "Buenos Aires")
	want := append([]byte{0x41, 0xB8, 0x00, 3, 22, 12, 1, 12}, "Buenos Aires"...)
	if !bytes.Equal(m, want) {
		t.Fatalf("%x", m)
	}
	m = Clima(-5.5, 71, -1, -9, false, "")
	// -55 décimas = 0xFFC9 (LE: C9 FF); -1 = FF; -9 = F7.
	if !bytes.Equal(m, []byte{0x41, 0xC9, 0xFF, 71, 0xFF, 0xF7, 0, 0}) {
		t.Fatalf("%x", m)
	}
	// La ciudad se corta a 24 bytes sin partir una letra.
	m = Clima(0, 0, 0, 0, true, "San Martín de los Andes, Neuquén")
	if n := int(m[7]); n > 24 || len(m) != 8+n || !strings.HasPrefix("San Martín de los Andes, Neuquén", string(m[8:])) {
		t.Fatalf("%q", m[8:])
	}
}

func TestAlarma(t *testing.T) {
	for i, a := range []string{"apagar", "posponer", "probar", "despertar"} {
		if m, err := Alarma(a); err != nil || !bytes.Equal(m, []byte{0x42, byte(i)}) {
			t.Fatalf("%s: %x %v", a, m, err)
		}
	}
	if _, err := Alarma("sonar"); err == nil {
		t.Fatal("acción rara")
	}
}
