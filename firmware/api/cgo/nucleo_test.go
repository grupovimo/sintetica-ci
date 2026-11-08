package nucleo

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sintetica/shared/formato"
	"sintetica/shared/protocolo"
)

func TestValidarConfig(t *testing.T) {
	if err := ValidarConfig(formato.Ejemplo()); err != nil {
		t.Fatalf("el ejemplo no valida: %v", err)
	}
	err := ValidarConfig([]byte(`{"version":1,"lista":["animacion:nada"]}`))
	ec, ok := err.(*ErrorConfig)
	if !ok || ec.JSONInvalido() || !strings.Contains(ec.Detalle, "animación") {
		t.Fatalf("esperaba un error de la lista, vino %v", err)
	}
	err = ValidarConfig([]byte(`{"version":1,`))
	if ec, ok := err.(*ErrorConfig); !ok || !ec.JSONInvalido() {
		t.Fatalf("esperaba JSON inválido, vino %v", err)
	}
}

func TestMensajeOtaIgualAlDeLosVectores(t *testing.T) {
	var v struct {
		Ota struct {
			Sha256, Firma, Version, Perfil, Mensaje, Binario string
		}
	}
	if err := json.Unmarshal(protocolo.Vectores, &v); err != nil {
		t.Fatal(err)
	}
	var m Manifiesto
	sha, _ := hex.DecodeString(strings.ReplaceAll(v.Ota.Sha256, " ", ""))
	copy(m.Sha256[:], sha)
	bin, _ := hex.DecodeString(strings.ReplaceAll(v.Ota.Binario, " ", ""))
	m.Tamano = uint32(len(bin))
	m.Version, m.Perfil = v.Ota.Version, v.Ota.Perfil
	got, err := MensajeOta(m)
	if err != nil || got != v.Ota.Mensaje {
		t.Fatalf("mensaje %q, esperaba %q (%v)", got, v.Ota.Mensaje, err)
	}
}

// Cada .cpp y .h del núcleo tiene que estar enlazado acá (si no, cgo no lo
// compila o usa uno viejo de su caché).
func TestEnlaces(t *testing.T) {
	var fuentes []string
	for _, patron := range []string{"../../nucleo/*.cpp", "../../nucleo/*.h", "../sintetica_c.*"} {
		m, _ := filepath.Glob(patron)
		fuentes = append(fuentes, m...)
	}
	for _, f := range fuentes {
		destino, err := os.Readlink(filepath.Base(f))
		if err != nil || filepath.Clean(destino) != filepath.Clean(f) {
			t.Errorf("falta el enlace: cd firmware/api/cgo && ln -sf %s %s", f, filepath.Base(f))
		}
	}
}

func TestDibujarExpresion(t *testing.T) {
	contar := func(px []byte) (encendidos int) {
		for i := 0; i < len(px); i += 4 {
			if px[i] > 160 && px[i+1] > 40 && px[i+2] < 120 { // naranja
				encendidos++
			}
		}
		return
	}
	neutral, err := DibujarExpresion(nil, "neutral", 320, 172, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := contar(neutral); n < 2000 {
		t.Fatalf("la neutral casi no tiene ojos: %d píxeles naranjas", n)
	}
	// Una propia: ojos enormes, más píxeles encendidos que la neutral.
	cfg := []byte(`{"version":2,"expresiones":{"ojazos":{"ojo":{"w":0.2,"h":0.32,"r":1},"particula":"corazon"}}}`)
	grande, err := DibujarExpresion(cfg, "ojazos", 320, 172, false, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if contar(grande) <= contar(neutral) {
		t.Fatalf("ojazos (%d) no es más grande que la neutral (%d)", contar(grande), contar(neutral))
	}
	if _, err := DibujarExpresion(cfg, "no-existe", 320, 172, false, 0); err != ErrSinExpresion {
		t.Fatalf("esperaba ErrSinExpresion, vino %v", err)
	}
	_, err = DibujarExpresion([]byte(`{"version":2,"expresiones":{"mal":{"ojo":{"w":9}}}}`), "mal", 320, 172, false, 0)
	if ec, ok := err.(*ErrorConfig); !ok || ec.Detalle == "" {
		t.Fatalf("esperaba un error de config con detalle, vino %v", err)
	}
	// Redonda (el broche): no se rompe y dibuja algo.
	if px, err := DibujarExpresion(nil, "contento", 240, 240, true, 0); err != nil || contar(px) == 0 {
		t.Fatalf("redonda: %v", err)
	}
}

func TestVidaBase(t *testing.T) {
	e, c := ExpresionesBase(), ComportamientosBase()
	if len(e) < 20 || e[0].Nombre != "neutral" || len(c) < 20 {
		t.Fatalf("%d expresiones (%v), %d comportamientos", len(e), e[:1], len(c))
	}
	for _, x := range append(e, c...) {
		if !json.Valid([]byte(x.JSON)) {
			t.Fatalf("%s: JSON inválido", x.Nombre)
		}
	}
}
