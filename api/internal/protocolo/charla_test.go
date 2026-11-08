package protocolo

import (
	"strings"
	"testing"
)

func TestCharlarYSuRespuesta(t *testing.T) {
	m, err := Charlar("despertame a las 7")
	if err != nil || m[0] != CmdCharlar || int(m[1]) != len("despertame a las 7") {
		t.Fatalf("%x %v", m, err)
	}
	for _, mal := range []string{"", strings.Repeat("a", MaxTextoCharlar+1), "\xff"} {
		if _, err := Charlar(mal); err == nil {
			t.Errorf("%q", mal)
		}
	}
	r := RespuestaCharla{HizoAlgo: true, Confianza: 100, Intencion: "recordatorio", Accion: "recordatorio", Dato: -1, Minuto: 1260,
		EnMinutos: -1, Dia: -1, Dias: 127, Texto: "Regar las plantas", Frase: "Te aviso todos los días a las 21:00."}
	l, err := LeerCharla(ArmarCharla(r))
	if err != nil || l != r {
		t.Fatalf("%+v %v", l, err)
	}
	b := ArmarCharla(r)
	for k := 3; k < len(b); k++ {
		if _, err := LeerCharla(b[:k]); err == nil {
			t.Fatalf("cortado en %d", k)
		}
	}
	if l, err := LeerCharla([]byte{CmdCharlar | Respuesta, Malformado}); err != nil || l.Codigo != Malformado {
		t.Fatalf("error: %+v %v", l, err)
	}
}
