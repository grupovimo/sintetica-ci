// Package nucleo es el núcleo de SintéticaOS (el mismo C++ del firmware)
// para Go, por cgo: la API valida las configs con él y el bicho virtual lo
// corre entero.
//
// El núcleo usa memoria estática: hay UN bicho por proceso y todas las
// llamadas se serializan con un mutex.
//
// Los .cpp y .h del núcleo están acá como enlaces simbólicos a
// firmware/nucleo y firmware/api: cgo solo compila (y su caché solo mira) los
// archivos de la carpeta del paquete. TestEnlaces verifica que estén todos.
package nucleo

/*
#cgo CXXFLAGS: -std=c++17 -O2 -fno-exceptions -fno-rtti
#cgo LDFLAGS: -lstdc++
#include <stdlib.h>
#include "sintetica_c.h"

void sintetica_go_iniciar(const char* id, const char* perfil, const char* version, int ancho, int alto, int redonda, int broche);
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

var mu sync.Mutex

// ErrorConfig es una config que el núcleo no acepta.
type ErrorConfig struct {
	Codigo  int    // ErrorConfig del núcleo: 1 JSON inválido, 2 versión, 3 campo, 4 forma, 5 demasiadas, 6 regla
	Detalle string // qué campo, en castellano
}

func (e *ErrorConfig) Error() string { return e.Detalle }

// JSONInvalido dice si el error es de sintaxis (y no de contenido).
func (e *ErrorConfig) JSONInvalido() bool { return e.Codigo == 1 }

// MaxConfig es lo más grande que entra en un bicho.
const MaxConfig = 16384

// ValidarConfig dice si un bicho aceptaría esta config (nil si sí).
func ValidarConfig(json []byte) error {
	if len(json) == 0 {
		return &ErrorConfig{Codigo: 1, Detalle: "config vacía"}
	}
	if len(json) > MaxConfig {
		return &ErrorConfig{Codigo: 5, Detalle: "config: más de 16 KB"}
	}
	mu.Lock()
	defer mu.Unlock()
	var detalle [128]C.char
	cjson := C.CBytes(json)
	defer C.free(cjson)
	c := C.sintetica_validar_config((*C.char)(cjson), C.size_t(len(json)), &detalle[0], C.size_t(len(detalle)))
	if c == 0 {
		return nil
	}
	return &ErrorConfig{Codigo: int(c), Detalle: C.GoString(&detalle[0])}
}

// Manifiesto de una OTA (ver docs/protocolo-ble.md).
type Manifiesto struct {
	Tamano  uint32
	Sha256  [32]byte
	Firma   [64]byte
	Version string
	Perfil  string
}

func (m *Manifiesto) aC() C.sintetica_manifiesto {
	var c C.sintetica_manifiesto
	c.tamano = C.uint32_t(m.Tamano)
	for i, b := range m.Sha256 {
		c.sha256[i] = C.uint8_t(b)
	}
	for i, b := range m.Firma {
		c.firma[i] = C.uint8_t(b)
	}
	copiar(c.version[:], m.Version)
	copiar(c.perfil[:], m.Perfil)
	return c
}

func deC(c *C.sintetica_manifiesto) Manifiesto {
	m := Manifiesto{Tamano: uint32(c.tamano), Version: C.GoString(&c.version[0]), Perfil: C.GoString(&c.perfil[0])}
	for i := range m.Sha256 {
		m.Sha256[i] = byte(c.sha256[i])
	}
	for i := range m.Firma {
		m.Firma[i] = byte(c.firma[i])
	}
	return m
}

func copiar(dst []C.char, s string) {
	n := len(s)
	if n > len(dst)-1 {
		n = len(dst) - 1
	}
	for i := 0; i < n; i++ {
		dst[i] = C.char(s[i])
	}
	dst[n] = 0
}

// MensajeOta es el texto que se firma, armado por el núcleo (el mismo que
// verifica el firmware).
func MensajeOta(m Manifiesto) (string, error) {
	if len(m.Version) == 0 || len(m.Version) > 23 || len(m.Perfil) == 0 || len(m.Perfil) > 23 {
		return "", errors.New("versión y perfil de 1 a 23 bytes")
	}
	cm := m.aC()
	var b [200]C.char
	n := C.sintetica_mensaje_ota(&cm, &b[0], C.size_t(len(b)))
	if n == 0 {
		return "", errors.New("no entra el mensaje")
	}
	return C.GoStringN(&b[0], C.int(n)), nil
}

// --- El bicho entero (para el bicho virtual) ---

// Hal es lo que el anfitrión le da al bicho. Se llama siempre desde la
// misma goroutine que llamó a Tick, BleEscribir, etc. (con el mutex tomado):
// no llamar a funciones del paquete desde adentro.
type Hal interface {
	AhoraMs() uint32
	Semilla() uint32
	MinutoDelDia() int
	Bateria() int
	Guardar(clave string, datos []byte) bool
	Leer(clave string) ([]byte, bool)
	EscribirTrozo(clave string, desplazamiento uint32, datos []byte) bool
	LeerTrozo(clave string, desplazamiento uint32, n int) ([]byte, bool)
	Borrar(clave string) bool
	Notificar(car int, datos []byte)
	NotificarNube(car int, datos []byte)
	RedCambio()
	OtaDescargar(m Manifiesto, url string) int
	OtaCancelar()
	Registrar(mensaje string)
}

var hal Hal

// Perfil del bicho virtual.
type Perfil struct {
	ID      string // 12 hexadecimales
	Nombre  string // "virtual"
	Version string
	Ancho   int
	Alto    int
	Redonda bool
	Broche  bool
}

// Iniciar arranca el bicho (o lo reinicia, como al prenderlo).
func Iniciar(h Hal, p Perfil) {
	mu.Lock()
	defer mu.Unlock()
	hal = h
	id, perfil, version := C.CString(p.ID), C.CString(p.Nombre), C.CString(p.Version)
	// No se liberan: el núcleo guarda los punteros (son pocos bytes por arranque).
	C.sintetica_go_iniciar(id, perfil, version, C.int(p.Ancho), C.int(p.Alto), b2i(p.Redonda), b2i(p.Broche))
}

func b2i(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

func Tick()               { mu.Lock(); C.sintetica_tick(); mu.Unlock() }
func Tacto(apretado bool) { mu.Lock(); C.sintetica_tacto(b2i(apretado)); mu.Unlock() }
func Evento(e int)        { mu.Lock(); C.sintetica_evento(C.int(e)); mu.Unlock() }
func Imu(x, y, z float32) {
	mu.Lock()
	C.sintetica_imu(C.float(x), C.float(y), C.float(z))
	mu.Unlock()
}
func BleConexion(c bool)   { mu.Lock(); C.sintetica_ble_conexion(b2i(c)); mu.Unlock() }
func Conexiones(w, n bool) { mu.Lock(); C.sintetica_conexiones(b2i(w), b2i(n)); mu.Unlock() }

func conBytes(d []byte, f func(p *C.uint8_t, n C.size_t)) {
	if len(d) == 0 {
		return
	}
	c := C.CBytes(d)
	defer C.free(c)
	f((*C.uint8_t)(c), C.size_t(len(d)))
}

// BleEscribir es una escritura por BLE en una característica.
func BleEscribir(car int, d []byte) {
	mu.Lock()
	defer mu.Unlock()
	conBytes(d, func(p *C.uint8_t, n C.size_t) { C.sintetica_ble_escribir(C.int(car), p, n) })
}

// NubeEscribir es lo que llega por MQTT (bicho/{id}/cmd), ya sin el byte de la característica.
func NubeEscribir(car int, d []byte) {
	mu.Lock()
	defer mu.Unlock()
	conBytes(d, func(p *C.uint8_t, n C.size_t) { C.sintetica_nube_escribir(C.int(car), p, n) })
}

// Estado es lo que se lee de la característica de estado (v4).
func Estado() []byte {
	mu.Lock()
	defer mu.Unlock()
	var b [256]byte
	n := C.sintetica_ble_estado((*C.uint8_t)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
	return append([]byte(nil), b[:n]...)
}

// RGBA devuelve el último cuadro.
func RGBA() (ancho, alto int, px []byte) {
	mu.Lock()
	defer mu.Unlock()
	ancho, alto = int(C.sintetica_ancho()), int(C.sintetica_alto())
	px = make([]byte, ancho*alto*4)
	C.sintetica_pixeles_rgba((*C.uint8_t)(unsafe.Pointer(&px[0])))
	return
}

// Red devuelve el WiFi guardado.
func Red() (ssid, clave string, ok bool) {
	mu.Lock()
	defer mu.Unlock()
	var s [33]C.char
	var k [65]C.char
	if C.sintetica_red(&s[0], 33, &k[0], 65) == 0 {
		return "", "", false
	}
	return C.GoString(&s[0]), C.GoString(&k[0]), true
}

// Nube devuelve las credenciales MQTT guardadas.
func Nube() (url, usuario, clave string, ok bool) {
	mu.Lock()
	defer mu.Unlock()
	var u [161]C.char
	var us [65]C.char
	var k [129]C.char
	if C.sintetica_nube(&u[0], 161, &us[0], 65, &k[0], 129) == 0 {
		return "", "", "", false
	}
	return C.GoString(&u[0]), C.GoString(&us[0]), C.GoString(&k[0]), true
}

// OtaProgreso avisa cómo va una descarga.
func OtaProgreso(codigo, pct int, detalle string) {
	mu.Lock()
	defer mu.Unlock()
	var cd *C.char
	if detalle != "" {
		cd = C.CString(detalle)
		defer C.free(unsafe.Pointer(cd))
	}
	C.sintetica_ota_progreso(C.int(codigo), C.int(pct), cd)
}

func QR() string { mu.Lock(); defer mu.Unlock(); return C.GoString(C.sintetica_qr()) }
func Visible() string {
	mu.Lock()
	defer mu.Unlock()
	return C.GoString(C.sintetica_expresion_visible())
}
func QuiereDormir() bool    { mu.Lock(); defer mu.Unlock(); return C.sintetica_quiere_dormir() != 0 }
func CrcConfig() uint32     { mu.Lock(); defer mu.Unlock(); return uint32(C.sintetica_crc_config()) }
func VersionNucleo() string { return C.GoString(C.sintetica_version()) }

// --- Llamadas desde C (puente.cpp) ---

func clave(c *C.char) string { return C.GoString(c) }

func bytesDe(p *C.uint8_t, n C.size_t) []byte {
	if n == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(p), C.int(n))
}

//export goAhoraMs
func goAhoraMs() C.uint32_t { return C.uint32_t(hal.AhoraMs()) }

//export goSemilla
func goSemilla() C.uint32_t { return C.uint32_t(hal.Semilla()) }

//export goMinuto
func goMinuto() C.int { return C.int(hal.MinutoDelDia()) }

//export goBateria
func goBateria() C.int { return C.int(hal.Bateria()) }

//export goGuardar
func goGuardar(c *C.char, d *C.uint8_t, n C.size_t) C.int {
	if hal.Guardar(clave(c), bytesDe(d, n)) {
		return 1
	}
	return 0
}

//export goLeer
func goLeer(c *C.char, d *C.uint8_t, capacidad C.size_t) C.int {
	b, ok := hal.Leer(clave(c))
	if !ok || len(b) > int(capacidad) {
		return -1
	}
	if len(b) > 0 {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(d)), int(capacidad)), b)
	}
	return C.int(len(b))
}

//export goEscribirTrozo
func goEscribirTrozo(c *C.char, off C.uint32_t, d *C.uint8_t, n C.size_t) C.int {
	if hal.EscribirTrozo(clave(c), uint32(off), bytesDe(d, n)) {
		return 1
	}
	return 0
}

//export goLeerTrozo
func goLeerTrozo(c *C.char, off C.uint32_t, d *C.uint8_t, n C.size_t) C.int {
	b, ok := hal.LeerTrozo(clave(c), uint32(off), int(n))
	if !ok {
		return -1
	}
	if len(b) > 0 {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(d)), int(n)), b)
	}
	return C.int(len(b))
}

//export goBorrar
func goBorrar(c *C.char) C.int {
	if hal.Borrar(clave(c)) {
		return 1
	}
	return 0
}

//export goNotificar
func goNotificar(car C.int, d *C.uint8_t, n C.size_t) { hal.Notificar(int(car), bytesDe(d, n)) }

//export goNotificarNube
func goNotificarNube(car C.int, d *C.uint8_t, n C.size_t) { hal.NotificarNube(int(car), bytesDe(d, n)) }

//export goRedCambio
func goRedCambio() { hal.RedCambio() }

//export goOtaDescargar
func goOtaDescargar(m *C.sintetica_manifiesto, url *C.char) C.int {
	return C.int(hal.OtaDescargar(deC(m), C.GoString(url)))
}

//export goOtaCancelar
func goOtaCancelar() { hal.OtaCancelar() }

//export goRegistrar
func goRegistrar(m *C.char) { hal.Registrar(C.GoString(m)) }
