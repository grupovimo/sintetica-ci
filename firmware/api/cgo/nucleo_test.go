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
