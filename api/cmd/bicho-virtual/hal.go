package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	nucleo "sintetica/firmware/api/cgo"
)

// hal es el hardware del bicho virtual: el disco (/datos), el reloj y la radio
// (MQTT y el "BLE" por HTTP). Lo llama el núcleo con su mutex tomado: no
// llamar al paquete nucleo desde acá.
type hal struct {
	b      *bicho
	inicio time.Time
	dir    string

	mu      sync.Mutex
	captura *[]notificacion // mientras hay una "conexión BLE" por HTTP
}

type notificacion struct {
	Car   byte   `json:"car"`
	Datos []byte `json:"datos"`
}

func (h *hal) ruta(tipo, clave string) string {
	// Las claves del núcleo son cortas y sin barras; por las dudas, se hashean las raras.
	if strings.ContainsAny(clave, "/\\.") || clave == "" {
		s := sha256.Sum256([]byte(clave))
		clave = hex.EncodeToString(s[:8])
	}
	return filepath.Join(h.dir, tipo, clave)
}

func (h *hal) AhoraMs() uint32 { return uint32(time.Since(h.inicio).Milliseconds()) }
func (h *hal) Semilla() uint32 {
	var b [4]byte
	leerAzar(b[:])
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func (h *hal) MinutoDelDia() int {
	t := time.Now().In(zonaArgentina)
	return t.Hour()*60 + t.Minute()
}
func (h *hal) Epoch() uint32 {
	t := time.Now().In(zonaArgentina)
	_, offset := t.Zone()
	return uint32(t.Unix() + int64(offset))
}
func (h *hal) Bateria() int { return -1 }

func (h *hal) Guardar(clave string, d []byte) bool {
	return escribirAtomico(h.ruta("kv", clave), d) == nil
}

func (h *hal) Leer(clave string) ([]byte, bool) {
	b, err := os.ReadFile(h.ruta("kv", clave))
	return b, err == nil
}

func (h *hal) EscribirTrozo(clave string, off uint32, d []byte) bool {
	r := h.ruta("archivos", clave)
	flags := os.O_WRONLY | os.O_CREATE
	if off == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(r, flags, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() != int64(off) {
		return false
	}
	_, err = f.WriteAt(d, int64(off))
	return err == nil
}

func (h *hal) LeerTrozo(clave string, off uint32, n int) ([]byte, bool) {
	f, err := os.Open(h.ruta("archivos", clave))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b := make([]byte, n)
	m, _ := f.ReadAt(b, int64(off))
	return b[:m], true
}

func (h *hal) Borrar(clave string) bool {
	err := os.Remove(h.ruta("archivos", clave))
	return err == nil || os.IsNotExist(err)
}

func (h *hal) Notificar(car int, d []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.captura != nil {
		*h.captura = append(*h.captura, notificacion{Car: byte(car), Datos: d})
	}
}

func (h *hal) NotificarNube(car int, d []byte) { h.b.publicar(byte(car), d) }
func (h *hal) RedCambio()                      { h.b.reconectar() }

func (h *hal) OtaDescargar(m nucleo.Manifiesto, url string) int { return h.b.empezarOta(m, url) }
func (h *hal) OtaCancelar()                                     { h.b.cancelarOta() }
func (h *hal) Registrar(m string)                               { h.b.log.Info(m) }

func escribirAtomico(ruta string, d []byte) error {
	tmp := ruta + ".tmp"
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ruta)
}
