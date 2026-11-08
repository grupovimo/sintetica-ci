package protocolo

import "testing"

// Los marcos de la v1.6 dan los mismos bytes que el JS de referencia (vectores.json).
func TestVectoresHabla(t *testing.T) {
	v := cargar(t)
	por := map[string]string{}
	for _, c := range v.Comandos {
		por[c["nombre"].(string)] = c["hex"].(string)
	}
	hola, _ := Decir("¡Hola, Nahuel!", "")
	conCara, _ := Decir("Buen día", "contento")
	probar, _ := Alarma("probar")
	posponer, _ := Alarma("posponer")
	dado, _ := Jugar("dado")
	terminar, _ := Jugar("terminar")
	aviso, _ := Aviso("notificacion", "WhatsApp", "Mamá")
	medio, _ := Animo(1)
	casos := map[string][]byte{
		"jugar dado": dado, "jugar terminar": terminar, "aviso whatsapp": aviso, "animo medio": medio,
		"mirar derecha": Mirar(50, -20),
		"decir hola":    hola, "decir con expresion": conCara,
		"clima":         Clima(-2.5, 73, 1, -4, true, "Bariloche"),
		"alarma probar": probar, "alarma posponer": posponer,
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
