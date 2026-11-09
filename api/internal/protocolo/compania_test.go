package protocolo

import (
	"strings"
	"testing"
)

// Los marcos de la v1.9 dan los mismos bytes que el JS de referencia (vectores.json).
func TestVectoresCompania(t *testing.T) {
	v := cargar(t)
	por := map[string]string{}
	for _, c := range v.Comandos {
		por[c["nombre"].(string)] = c["hex"].(string)
	}
	abrazo, _ := Toque("abrazo", "Ana", "")
	carita, _ := Toque("carita", "Martín", "contento")
	agenda, _ := Agenda(10, 30, "Reunión")
	manana, _ := ResumenManana(3, "Dentista")
	habito, _ := Habito(1, true, 4)
	pago, _ := Aviso("pago", "$12.000", "Ana")
	github, _ := Aviso("github", "PR #12", "octocat")
	casos := map[string][]byte{
		"toque abrazo": abrazo, "toque carita": carita, "agenda": agenda, "agenda manana": manana,
		"presencia llegaste": Presencia(true), "presencia te fuiste": Presencia(false),
		"habito hecho": habito, "musica": Musica(120, true, "Muchachos", "La Mosca"),
		"musica parar": Musica(0, false, "", ""), "aviso pago": pago, "aviso github": github,
	}
	for n, b := range casos {
		if por[n] == "" {
			t.Fatalf("falta el vector %q", n)
		}
		if hx(b) != por[n] {
			t.Errorf("%s: %s, esperaba %s", n, hx(b), por[n])
		}
	}
}

func TestCompaniaLimites(t *testing.T) {
	if _, err := Toque("pellizco", "Ana", ""); err == nil {
		t.Error("un toque desconocido no vale")
	}
	if _, err := Toque("toque", "  ", ""); err == nil {
		t.Error("sin de quién no vale")
	}
	largo, _ := Toque("toque", strings.Repeat("ñ", 20), "")
	if largo[2] != 24 {
		t.Errorf("el de se corta a 24 bytes sin partir letras: %d", largo[2])
	}
	if _, err := Agenda(5, 10, ""); err == nil {
		t.Error("sin título no vale")
	}
	a, _ := Agenda(70000, 10, "x")
	if a[1] != 0xFE || a[2] != 0xFF {
		t.Errorf("los minutos no llegan al resumen de mañana: % x", a[1:3])
	}
	if _, err := Habito(4, true, 0); err == nil {
		t.Error("hay 4 hábitos (0 a 3)")
	}
	if h, _ := Habito(0, false, 999); h[2] != 1 || h[3] != 255 {
		t.Errorf("deshacer y racha cortada: % x", h)
	}
	m := Musica(300, true, strings.Repeat("a", 50), "é")
	if m[1] != 255 || m[3] != 40 {
		t.Errorf("bpm y título cortados: % x", m[:4])
	}
	if _, err := Aviso("fax", "", ""); err == nil {
		t.Error("un aviso desconocido no vale")
	}
	if len(Eventos) != 46 || Eventos[39] != "toque-recibido" || Eventos[44] != "habito" {
		t.Errorf("los eventos de la v1.9 van del 39 al 44: %d", len(Eventos))
	}
	if TiposRegistro[RegistroToque] != "toque" || TiposRegistro[RegistroHabito] != "habito" {
		t.Error("los tipos de registro 7 y 8")
	}
}
