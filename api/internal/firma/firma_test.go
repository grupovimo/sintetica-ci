package firma

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFirmarYVerificar(t *testing.T) {
	f := Nuevo()
	m, err := f.Firmar("dev-147b", "0.2.1", []byte("binario de prueba"))
	if err != nil {
		t.Fatal(err)
	}
	if !Verificar(f.PublicaPEM(), m) {
		t.Fatal("no verifica")
	}
	m.Version = "0.2.2"
	if Verificar(f.PublicaPEM(), m) {
		t.Fatal("otra versión no tiene que verificar")
	}
	if Verificar(Nuevo().PublicaPEM(), m) {
		t.Fatal("otra clave no tiene que verificar")
	}
}

func TestCargarLaDeNpmRunClaves(t *testing.T) {
	ruta := filepath.Join("..", "..", "..", "claves", "ota-privada.pem")
	if _, err := os.Stat(ruta); err != nil {
		t.Skip("sin claves/ (npm run claves)")
	}
	f, err := Cargar(ruta)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join("..", "..", "..", "claves", "ota-publica.pem"))
	m, _ := f.Firmar("broche-c3", "1.0.0", []byte{1, 2, 3})
	if !Verificar(pub, m) {
		t.Fatal("la pública de claves/ no verifica lo que firma la privada")
	}
}
