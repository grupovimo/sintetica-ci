package nucleo

import (
	"strings"
	"sync"
	"testing"
)

// Lunes 9 de noviembre de 2026, 14:00 (hora local).
const lunes1400 = 1794182400 + 14*3600

func ctxPrueba() ContextoCharla {
	c := NuevoContextoCharla()
	c.Dueno, c.Robot = "Nahuel", "Toto"
	c.Minuto, c.Epoch = 14*60, lunes1400
	c.Clima, c.ClimaManana = "24° y despejado", "18° y lluvia"
	c.Habitos = []string{"Tomar agua", "Leer"}
	c.Semilla = 7
	return c
}

func TestCharlar(t *testing.T) {
	casos := []struct {
		texto, intencion, accion string
		minuto, dato             int
		textoAccion              string
	}{
		{"hola", "saludo", "ninguna", -1, -1, ""},
		{"q hora es", "hora", "ninguna", -1, -1, ""},
		{"poneme una alarma a las 7", "alarma", "alarma", 420, -1, ""},
		{"despertame a las 7 y media", "alarma", "alarma", 450, -1, ""},
		{"recordame comprar pan a las 6", "recordatorio", "recordatorio", 1080, -1, "Comprar pan"},
		{"juguemos a piedra papel o tijera", "jugar", "jugar", -1, 0, ""},
		{"ya tomé agua", "habito-hecho", "habito", -1, 0, ""},
		{"me gusta el mate", "gusto", "gusto", -1, -1, "el mate"},
		{"mandale un abrazo a Ana", "toque-amigo", "toque", -1, 1, "Ana"},
	}
	for _, c := range casos {
		var e EstadoCharla
		r, err := Charlar(c.texto, ctxPrueba(), &e)
		if err != nil {
			t.Fatal(err)
		}
		if r.Intencion != c.intencion || r.Accion != c.accion || r.Minuto != c.minuto || r.Dato != c.dato || r.Texto != c.textoAccion {
			t.Errorf("%q: %+v", c.texto, r)
		}
		if r.Frase == "" || len(r.Frase) > 60 || strings.Contains(r.Frase, "{") {
			t.Errorf("%q: frase %q", c.texto, r.Frase)
		}
	}
}

// Lo dudoso que cambia algo: pregunta antes (Confirma y Falta, sin acción) y
// con el "sí" (en el mismo EstadoCharla) lo hace.
func TestCharlarConfirma(t *testing.T) {
	var e EstadoCharla
	ctx := ctxPrueba()
	r, _ := Charlar("decime Zoe", ctx, &e)
	if !r.Confirma || !r.Falta || r.Accion != "ninguna" || !strings.Contains(r.Frase, "Zoe") {
		t.Fatalf("decime Zoe: %+v", r)
	}
	ctx.Segundos += 5
	if r, _ = Charlar("sí", ctx, &e); r.Confirma || r.Accion != "dueno" || r.Texto != "Zoe" {
		t.Fatalf("sí: %+v", r)
	}
	var e2 EstadoCharla
	if r, _ = Charlar("decime de Zoe", ctx, &e2); r.Accion != "ninguna" || r.Confirma || !r.NoEntendi {
		t.Fatalf("decime de Zoe: %+v", r)
	}
	if r, _ = Charlar("olvidate que me gusta el mate", ctx, &e2); r.Accion != "olvidar-gusto" || r.Texto != "el mate" {
		t.Fatalf("olvidate que me gusta: %+v", r)
	}
}

func TestCharlarConContextoYSinEntender(t *testing.T) {
	var e EstadoCharla
	ctx := ctxPrueba()
	if r, _ := Charlar("¿qué tiempo hace?", ctx, &e); r.Intencion != "clima" || !strings.Contains(r.Frase, "24°") {
		t.Fatalf("clima: %+v", r)
	}
	ctx.Segundos += 5
	if r, _ := Charlar("¿y mañana?", ctx, &e); r.Intencion != "clima" || !strings.Contains(r.Frase, "18°") {
		t.Fatalf("y mañana: %+v", r)
	}
	// Lo que no entiende: la nube prueba la IA.
	r, _ := Charlar("asdf qwerty", ctx, &e)
	if !r.NoEntendi || r.Frase == "" {
		t.Fatalf("no entendió: %+v", r)
	}
	if _, err := Charlar("", ctx, &e); err == nil {
		t.Fatal("un texto vacío tiene que dar error")
	}
	if _, err := Charlar(strings.Repeat("a", 201), ctx, &e); err == nil {
		t.Fatal("más de 200 bytes tiene que dar error")
	}
}

func TestCharlarEnParalelo(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 50; k++ {
				var e EstadoCharla
				if r, _ := Charlar("poneme una alarma a las 7", ctxPrueba(), &e); r.Minuto != 420 {
					t.Errorf("en paralelo: %+v", r)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestVozPropia(t *testing.T) {
	a := VozPropia("¡Hola, Nahuel!", 1, 70, 0.5, 0.5)
	if len(a) < 16000/4 || len(a) > 16000*5 {
		t.Fatalf("%d muestras", len(a))
	}
	b := VozPropia("¡Hola, Nahuel!", 1, 70, 0.5, 0.5)
	if len(a) != len(b) {
		t.Fatal("no es determinista")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("no es determinista")
		}
	}
	w := WAV(a)
	if string(w[0:4]) != "RIFF" || len(w) != 44+2*len(a) {
		t.Fatal("WAV mal armado")
	}
}
