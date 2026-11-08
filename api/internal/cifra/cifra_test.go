package cifra

import (
	"bytes"
	"testing"
)

func TestCifrar(t *testing.T) {
	c := Nuevo([]byte("un secreto de prueba"), "agenda")
	a, b := c.Cifrar("https://calendar.google.com/x.ics"), c.Cifrar("https://calendar.google.com/x.ics")
	if bytes.Equal(a, b) || bytes.Contains(a, []byte("calendar")) {
		t.Fatal("tiene que tener un nonce distinto y no dejar ver el texto")
	}
	if s, err := c.Descifrar(a); err != nil || s != "https://calendar.google.com/x.ics" {
		t.Fatalf("%q %v", s, err)
	}
	if _, err := Nuevo([]byte("otro secreto"), "agenda").Descifrar(a); err != ErrIlegible {
		t.Fatalf("con otra clave: %v", err)
	}
	if _, err := Nuevo([]byte("un secreto de prueba"), "spotify").Descifrar(a); err != ErrIlegible {
		t.Fatalf("para otra cosa: %v", err)
	}
	if _, err := c.Descifrar([]byte{1, 2}); err != ErrIlegible {
		t.Fatal("corto")
	}
}
