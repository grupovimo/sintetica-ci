// Package formato embebe lo que vive en shared/formato: el JSON Schema, las
// 16 caras base, las animaciones de ejemplo y la config de ejemplo. Lo usan
// la API (para la web) y el bicho virtual.
package formato

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed config.schema.json config-ejemplo.json caras/*.json animaciones/*.json vida/frases.json vida/personalidades.json
var archivos embed.FS

// Schema devuelve el JSON Schema de la config.
func Schema() []byte { b, _ := archivos.ReadFile("config.schema.json"); return b }

// Ejemplo devuelve la config de ejemplo.
func Ejemplo() []byte { b, _ := archivos.ReadFile("config-ejemplo.json"); return b }

// Frases devuelve las frases de fábrica del motor de vida (vida/frases.json).
func Frases() []byte { b, _ := archivos.ReadFile("vida/frases.json"); return b }

// Personalidades devuelve los rasgos de cada preset (vida/personalidades.json).
func Personalidades() []byte { b, _ := archivos.ReadFile("vida/personalidades.json"); return b }

// Pieza es una cara o una animación con su nombre.
type Pieza struct {
	Nombre    string          `json:"nombre"`
	Documento json.RawMessage `json:"documento"`
}

func leerDir(dir string) []Pieza {
	var piezas []Pieza
	entradas, _ := fs.ReadDir(archivos, dir)
	for _, e := range entradas {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := archivos.ReadFile(path.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		var nombre string
		_ = json.Unmarshal(m["nombre"], &nombre)
		delete(m, "nombre")
		doc, _ := json.Marshal(m)
		piezas = append(piezas, Pieza{Nombre: nombre, Documento: doc})
	}
	sort.Slice(piezas, func(i, j int) bool { return piezas[i].Nombre < piezas[j].Nombre })
	return piezas
}

// Caras devuelve las 16 caras base (sin el campo "nombre" adentro del documento).
func Caras() []Pieza { return leerDir("caras") }

// Animaciones devuelve las animaciones de ejemplo.
func Animaciones() []Pieza { return leerDir("animaciones") }
