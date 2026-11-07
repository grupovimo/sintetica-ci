// Package protocolo embebe shared/protocolo/vectores.json para los tests de Go.
package protocolo

import _ "embed"

//go:embed vectores.json
var Vectores []byte
