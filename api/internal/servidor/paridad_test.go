package servidor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nucleo "sintetica/firmware/api/cgo"
)

// Los dos caminos por los que la API valida una config (PUT /config la
// compacta; el MCP la reescribe con objeto) aceptan y rechazan el set de
// paridad (shared/formato/paridad) igual que el núcleo del bicho.
func TestParidadConfigEnLaAPI(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "shared", "formato", "paridad")
	b, err := os.ReadFile(filepath.Join(dir, "paridad.json"))
	if err != nil {
		t.Fatal(err)
	}
	var indice struct {
		Casos []struct {
			Archivo string
			Acepta  bool
			Codigo  int
			Detalle string
		}
	}
	if err := json.Unmarshal(b, &indice); err != nil {
		t.Fatal(err)
	}
	for _, c := range indice.Casos {
		texto, err := os.ReadFile(filepath.Join(dir, "configs", c.Archivo))
		if err != nil {
			t.Fatal(err)
		}
		revisar := func(camino string, err error) {
			if c.Acepta {
				if err != nil {
					t.Errorf("%s (%s): esperaba que la acepte, vino %v", c.Archivo, camino, err)
				}
				return
			}
			ec, ok := err.(*nucleo.ErrorConfig)
			if !ok || ec.Codigo != c.Codigo || !strings.HasPrefix(ec.Detalle, c.Detalle) {
				t.Errorf("%s (%s): esperaba %d %q, vino %#v", c.Archivo, camino, c.Codigo, c.Detalle, err)
			}
		}
		// Como guardarConfig: si no compacta, es JSON inválido (y el núcleo también lo rechaza).
		var compacto bytes.Buffer
		if err := json.Compact(&compacto, texto); err != nil {
			if c.Acepta || c.Codigo != 1 {
				t.Errorf("%s: no compacta (%v) y el núcleo no la rechaza por JSON", c.Archivo, err)
			}
		} else {
			revisar("compacta", nucleo.ValidarConfig(compacto.Bytes()))
		}
		// Como el MCP: leída y reescrita en orden.
		if o, err := leerObjeto(texto); err == nil && json.Valid(texto) {
			_, err := validarConfig(o)
			if c.Acepta && err != nil {
				t.Errorf("%s (mcp): esperaba que la acepte, vino %v", c.Archivo, err)
			} else if !c.Acepta && err == nil {
				t.Errorf("%s (mcp): esperaba que la rechace", c.Archivo)
			}
		}
	}
}
