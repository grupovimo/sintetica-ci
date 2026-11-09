package protocolo

import "testing"

func TestDiario(t *testing.T) {
	v := cargar(t)
	if hx(Diario()) != porNombre(v, "diario") {
		t.Errorf("diario: %s, esperaba %s", hx(Diario()), porNombre(v, "diario"))
	}
	b := append([]byte{CmdDiario | Respuesta, Ok, 0x1e, 0x51, 1, 3, 1, 3, 2}, "Nahuel me ganó a los dados."...)
	l, err := LeerDiario(b)
	if err != nil || l.Dia != 0x511e || l.Indice != 1 || l.Total != 3 || l.Tipo != "perdi" || l.Dato != 3 ||
		l.Etapa != "confiado" || l.Texto != "Nahuel me ganó a los dados." {
		t.Fatalf("%+v %v", l, err)
	}
	b[6] = DiarioTranquilo
	if l, _ = LeerDiario(b); l.Tipo != "tranquilo" {
		t.Error("tranquilo")
	}
	if _, err := LeerDiario([]byte{CmdDiario | Respuesta, Ok, 0}); err == nil {
		t.Error("corto")
	}
	if len(TiposEpisodio) != 16 || TiposRegistro[RegistroDiario] != "diario" || Eventos[45] != "crecio" {
		t.Error("las tablas")
	}
}
