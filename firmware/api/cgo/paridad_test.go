package nucleo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// casoParidad es una entrada de shared/formato/paridad/paridad.json.
type casoParidad struct {
	Archivo string
	Acepta  bool
	Codigo  int
	Detalle string
}

// La misma config se acepta o se rechaza igual que en el núcleo
// (firmware/test/test_paridad.cpp): con el código y el detalle del índice.
func TestParidadConfig(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "shared", "formato", "paridad")
	b, err := os.ReadFile(filepath.Join(dir, "paridad.json"))
	if err != nil {
		t.Fatal(err)
	}
	var indice struct{ Casos []casoParidad }
	if err := json.Unmarshal(b, &indice); err != nil {
		t.Fatal(err)
	}
	if len(indice.Casos) < 50 {
		t.Fatalf("pocos casos: %d", len(indice.Casos))
	}
	for _, c := range indice.Casos {
		texto, err := os.ReadFile(filepath.Join(dir, "configs", c.Archivo))
		if err != nil {
			t.Fatal(err)
		}
		err = ValidarConfig(texto)
		if c.Acepta {
			if err != nil {
				t.Errorf("%s: esperaba que la acepte, vino %v", c.Archivo, err)
			}
			continue
		}
		ec, ok := err.(*ErrorConfig)
		if !ok || ec.Codigo != c.Codigo || !strings.HasPrefix(ec.Detalle, c.Detalle) {
			t.Errorf("%s: esperaba %d %q, vino %#v", c.Archivo, c.Codigo, c.Detalle, err)
		}
	}
}
