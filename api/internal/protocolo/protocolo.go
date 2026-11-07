// Package protocolo es el protocolo de SintéticaOS v1.3 del lado de la API: los
// mismos bytes que mandan la app y la web por BLE, ahora por MQTT. Ver
// docs/protocolo-ble.md; los tests usan shared/protocolo/vectores.json.
package protocolo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"regexp"
	"strings"
)

// Características (el primer byte de cada mensaje MQTT).
const (
	CarConfig  = 1
	CarComando = 2
	CarEstado  = 3
	CarOta     = 4
	CarImagen  = 5
)

// Operaciones.
const (
	ConfigInicio   = 0x01
	ConfigBloque   = 0x02
	ConfigFin      = 0x03
	ConfigPedir    = 0x04
	CmdExpresion   = 0x10
	CmdNombre      = 0x11
	CmdEvento      = 0x12
	CmdVincular    = 0x13
	CmdAutenticar  = 0x14
	CmdDesvincular = 0x15
	CmdRed         = 0x16
	CmdNube        = 0x17
	CmdOlvidarRed  = 0x18
	CmdReproducir  = 0x19
	ImgInicio      = 0x20
	ImgBloque      = 0x21
	ImgFin         = 0x22
	ImgBorrar      = 0x23
	ImgListar      = 0x24
	ImgPedir       = 0x25
	OtaInicio      = 0x30
	OtaBloque      = 0x31
	OtaFin         = 0x32
	OtaDescargar   = 0x33
	OtaProgreso    = 0x34
	OtaCancelar    = 0x35
	Respuesta      = 0x80
	ConfigTrozo    = 0x84
	ConfigCierre   = 0x85
	ImgTrozo       = 0xA7
	ImgCierre      = 0xA8
)

// Códigos de respuesta.
const (
	Ok             = 0
	MuyGrande      = 1
	FueraDeOrden   = 2
	CrcDistinto    = 3
	JSONInvalido   = 4
	ConfigInvalida = 5
	SinInicio      = 6
	NoExiste       = 7
	Malformado     = 8
	NoAutorizado   = 9
	NoEmparejando  = 10
	ImagenInvalida = 11
	FirmaInvalida  = 12
	HashDistinto   = 13
	PerfilDistinto = 14
	SinRed         = 15
	Ocupado        = 16
	FalloOta       = 17
	NoImplementado = 0xFF
)

var mensajes = map[int]string{
	0: "ok", 1: "muy grande", 2: "fuera de orden", 3: "CRC distinto", 4: "JSON inválido", 5: "config inválida",
	6: "sin inicio", 7: "no existe", 8: "malformado", 9: "no autorizado", 10: "no está mostrando el QR",
	11: "imagen inválida", 12: "firma inválida", 13: "hash distinto", 14: "firmware de otra placa", 15: "sin WiFi",
	16: "ocupado", 17: "falló la actualización", 255: "no implementado",
}

// Mensaje es el texto de un código.
func Mensaje(codigo int) string {
	if m, ok := mensajes[codigo]; ok {
		return m
	}
	return fmt.Sprintf("código %d", codigo)
}

// Eventos, en el orden del protocolo.
var Eventos = []string{"tacto", "doble-toque", "levantado", "sacudida", "boca-abajo", "vinculado", "app-abierta", "despertado"}

// Modos, en el orden del protocolo.
var Modos = []string{"mascota", "reloj", "pomodoro", "clima", "notificaciones"}

// Crc32 es el CRC-32 IEEE del protocolo.
func Crc32(b []byte) uint32 { return crc32.ChecksumIEEE(b) }

func u32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func corto(s string, max int) ([]byte, error) {
	if len(s) > max {
		return nil, fmt.Errorf("%q es muy largo (máximo %d bytes)", s, max)
	}
	return append([]byte{byte(len(s))}, s...), nil
}

// Comandos.
func Expresion(nombre string) []byte  { return append([]byte{CmdExpresion}, nombre...) }
func Nombre(nombre string) []byte     { return append([]byte{CmdNombre}, nombre...) }
func Reproducir(nombre string) []byte { return append([]byte{CmdReproducir}, nombre...) }
func Vincular(codigo string) []byte   { return append([]byte{CmdVincular}, codigo...) }
func Autenticar(clave []byte) []byte  { return append([]byte{CmdAutenticar}, clave...) }
func Desvincular() []byte             { return []byte{CmdDesvincular} }
func OlvidarRed() []byte              { return []byte{CmdOlvidarRed} }
func PedirConfig() []byte             { return []byte{ConfigPedir} }

// Evento arma el comando de un evento por nombre.
func Evento(nombre string) ([]byte, error) {
	for i, e := range Eventos {
		if e == nombre {
			return []byte{CmdEvento, byte(i)}, nil
		}
	}
	return nil, fmt.Errorf("evento desconocido: %s", nombre)
}

// Red arma el comando de WiFi.
func Red(ssid, clave string) ([]byte, error) {
	if ssid == "" {
		return nil, errors.New("falta el ssid")
	}
	a, err := corto(ssid, 32)
	if err != nil {
		return nil, err
	}
	b, err := corto(clave, 64)
	if err != nil {
		return nil, err
	}
	return append(append([]byte{CmdRed}, a...), b...), nil
}

// Nube arma el comando con las credenciales MQTT.
func Nube(url, usuario, clave string) ([]byte, error) {
	a, err := corto(url, 160)
	if err != nil {
		return nil, err
	}
	b, err := corto(usuario, 64)
	if err != nil {
		return nil, err
	}
	c, err := corto(clave, 128)
	if err != nil {
		return nil, err
	}
	out := append([]byte{CmdNube}, a...)
	return append(append(out, b...), c...), nil
}

// EscriturasDeConfig parte una config en las escrituras de la característica Config.
func EscriturasDeConfig(json []byte, tamanoBloque int) [][]byte {
	porBloque := tamanoBloque - 5
	salida := [][]byte{append(append([]byte{ConfigInicio}, u32(uint32(len(json)))...), u32(Crc32(json))...)}
	for off := 0; off < len(json); off += porBloque {
		fin := min(len(json), off+porBloque)
		salida = append(salida, append(append([]byte{ConfigBloque}, u32(uint32(off))...), json[off:fin]...))
	}
	return append(salida, []byte{ConfigFin})
}

// EscriturasDeImagen parte un QOI en las escrituras de la característica Imagen.
func EscriturasDeImagen(espacio int, qoi []byte, tamanoBloque int) [][]byte {
	porBloque := tamanoBloque - 5
	ini := append([]byte{ImgInicio, byte(espacio)}, u32(uint32(len(qoi)))...)
	salida := [][]byte{append(ini, u32(Crc32(qoi))...)}
	for off := 0; off < len(qoi); off += porBloque {
		fin := min(len(qoi), off+porBloque)
		salida = append(salida, append(append([]byte{ImgBloque}, u32(uint32(off))...), qoi[off:fin]...))
	}
	return append(salida, []byte{ImgFin})
}

func BorrarImagen(espacio int) []byte { return []byte{ImgBorrar, byte(espacio)} }
func ListarImagenes() []byte          { return []byte{ImgListar} }
func PedirImagen(espacio int) []byte  { return []byte{ImgPedir, byte(espacio)} }

// EspacioImagen es un espacio ocupado.
type EspacioImagen struct {
	Espacio int    `json:"espacio"`
	Largo   uint32 `json:"largo"`
	Crc     uint32 `json:"crc"`
}

// LeerListaImagenes interpreta la respuesta a ListarImagenes.
func LeerListaImagenes(b []byte) []EspacioImagen {
	var l []EspacioImagen
	for i := 3; i+9 <= len(b); i += 9 {
		l = append(l, EspacioImagen{int(b[i]), binary.LittleEndian.Uint32(b[i+1:]), binary.LittleEndian.Uint32(b[i+5:])})
	}
	return l
}

// Manifiesto de una OTA.
type Manifiesto struct {
	Tamano  uint32
	Sha256  [32]byte
	Firma   [64]byte
	Version string
	Perfil  string
}

// Bytes codifica el manifiesto.
func (m Manifiesto) Bytes() ([]byte, error) {
	v, err := corto(m.Version, 23)
	if err != nil {
		return nil, err
	}
	p, err := corto(m.Perfil, 23)
	if err != nil {
		return nil, err
	}
	out := u32(m.Tamano)
	out = append(out, m.Sha256[:]...)
	out = append(out, m.Firma[:]...)
	return append(append(out, v...), p...), nil
}

// Mensaje es el texto que se firma.
func (m Manifiesto) Mensaje() string {
	return fmt.Sprintf("SINTETICA-OTA-V1\n%s\n%s\n%d\n%x\n", m.Perfil, m.Version, m.Tamano, m.Sha256[:])
}

// DescargarOta arma el pedido de descarga.
func DescargarOta(m Manifiesto, url string) ([]byte, error) {
	mb, err := m.Bytes()
	if err != nil {
		return nil, err
	}
	if len(url) == 0 || len(url) > 255 {
		return nil, errors.New("url de 1 a 255 bytes")
	}
	out := append([]byte{OtaDescargar}, mb...)
	out = binary.LittleEndian.AppendUint16(out, uint16(len(url)))
	return append(out, url...), nil
}

// EscriturasDeOta son el manifiesto, los bloques y el fin (por BLE).
func EscriturasDeOta(m Manifiesto, binario []byte, tamanoBloque int) ([][]byte, error) {
	mb, err := m.Bytes()
	if err != nil {
		return nil, err
	}
	porBloque := tamanoBloque - 5
	salida := [][]byte{append([]byte{OtaInicio}, mb...)}
	for off := 0; off < len(binario); off += porBloque {
		fin := min(len(binario), off+porBloque)
		salida = append(salida, append(append([]byte{OtaBloque}, u32(uint32(off))...), binario[off:fin]...))
	}
	return append(salida, []byte{OtaFin}), nil
}

// Resultado es una respuesta [op|0x80][código][detalle].
type Resultado struct {
	Op      int    `json:"op"`
	Codigo  int    `json:"codigo"`
	Ok      bool   `json:"ok"`
	Mensaje string `json:"mensaje"`
	Detalle string `json:"detalle,omitempty"`
	Clave   []byte `json:"-"`
}

// LeerResultado interpreta una respuesta (no un trozo ni un cierre).
func LeerResultado(b []byte) (Resultado, bool) {
	if len(b) < 2 || b[0]&0x80 == 0 || b[0] == ConfigTrozo || b[0] == ConfigCierre || b[0] == ImgTrozo || b[0] == ImgCierre {
		return Resultado{}, false
	}
	r := Resultado{Op: int(b[0] & 0x7F), Codigo: int(b[1])}
	r.Ok, r.Mensaje = r.Codigo == Ok, Mensaje(r.Codigo)
	if r.Op == CmdVincular && r.Ok && len(b) >= 18 {
		r.Clave = append([]byte(nil), b[2:18]...)
	} else if len(b) > 2 && r.Op != ImgListar && r.Op != OtaProgreso {
		r.Detalle = string(b[2:])
	}
	return r, true
}

// Pantalla del bicho.
type Pantalla struct {
	Ancho   int  `json:"ancho"`
	Alto    int  `json:"alto"`
	Redonda bool `json:"redonda"`
}

// Estado es el estado v4 decodificado.
type Estado struct {
	Bateria        *int     `json:"bateria"`
	Humor          float64  `json:"humor"`
	Modo           string   `json:"modo"`
	Dormido        bool     `json:"dormido"`
	Fija           bool     `json:"fija"`
	Emparejando    bool     `json:"emparejando"`
	Autenticado    bool     `json:"-"`
	Wifi           bool     `json:"wifi"`
	Nube           bool     `json:"nube"`
	RedConfigurada bool     `json:"redConfigurada"`
	Actualizando   bool     `json:"actualizando"`
	Expresion      string   `json:"expresion"`
	Nombre         string   `json:"nombre"`
	Serie          string   `json:"serie"`
	ID             string   `json:"id"`
	Pantalla       Pantalla `json:"pantalla"`
	Version        string   `json:"version"`
	Perfil         string   `json:"perfil"`
	CrcConfig      uint32   `json:"crcConfig"`
	Encendido      uint32   `json:"encendido"` // segundos desde que arrancó
}

// LeerEstado decodifica el estado (versiones 1 a 4).
func LeerEstado(b []byte) (Estado, error) {
	var e Estado
	if len(b) < 7 || b[0] < 1 || b[0] > 4 {
		return e, errors.New("estado desconocido")
	}
	i := 5
	campo := func() (string, error) {
		if i >= len(b) || i+1+int(b[i]) > len(b) {
			return "", errors.New("estado cortado")
		}
		s := string(b[i+1 : i+1+int(b[i])])
		i += 1 + int(b[i])
		return s, nil
	}
	var err error
	if e.Expresion, err = campo(); err != nil {
		return e, err
	}
	if e.Nombre, err = campo(); err != nil {
		return e, err
	}
	if b[0] >= 2 {
		if e.Serie, err = campo(); err != nil {
			return e, err
		}
		if e.ID, err = campo(); err != nil {
			return e, err
		}
	}
	e.Pantalla = Pantalla{320, 172, false}
	if b[0] >= 3 {
		if i+5 > len(b) {
			return e, errors.New("estado cortado")
		}
		e.Pantalla = Pantalla{int(binary.LittleEndian.Uint16(b[i:])), int(binary.LittleEndian.Uint16(b[i+2:])), b[i+4] == 1}
		i += 5
	}
	if b[0] >= 4 {
		if e.Version, err = campo(); err != nil {
			return e, err
		}
		if e.Perfil, err = campo(); err != nil {
			return e, err
		}
		if i+4 > len(b) {
			return e, errors.New("estado cortado")
		}
		e.CrcConfig = binary.LittleEndian.Uint32(b[i:])
		if i+8 <= len(b) {
			e.Encendido = binary.LittleEndian.Uint32(b[i+4:])
		}
	}
	if b[1] != 255 {
		v := int(b[1])
		e.Bateria = &v
	}
	e.Humor = float64(int8(b[2])) / 100
	if int(b[3]) < len(Modos) {
		e.Modo = Modos[b[3]]
	} else {
		e.Modo = "?"
	}
	f := b[4]
	e.Dormido, e.Fija, e.Emparejando, e.Autenticado = f&1 != 0, f&2 != 0, f&4 != 0, f&8 != 0
	e.Wifi, e.Nube, e.RedConfigurada, e.Actualizando = f&16 != 0, f&32 != 0, f&64 != 0, f&128 != 0
	return e, nil
}

// Armador junta lo que el bicho manda en trozos (config o imagen).
type Armador struct {
	partes map[uint32][]byte
}

// Agregar suma un trozo [op][u32 desplazamiento][datos].
func (a *Armador) Agregar(b []byte) {
	if len(b) < 5 {
		return
	}
	if a.partes == nil {
		a.partes = map[uint32][]byte{}
	}
	a.partes[binary.LittleEndian.Uint32(b[1:])] = append([]byte(nil), b[5:]...)
}

// Cerrar arma el total y verifica largo y CRC.
func (a *Armador) Cerrar(largo, crc uint32) ([]byte, error) {
	out := make([]byte, largo)
	var total uint32
	for off, d := range a.partes {
		if off+uint32(len(d)) > largo {
			return nil, errors.New("un trozo se pasa del largo")
		}
		copy(out[off:], d)
		total += uint32(len(d))
	}
	a.partes = nil
	if total != largo || Crc32(out) != crc {
		return nil, errors.New("llegó con otro CRC o le faltan trozos")
	}
	return out, nil
}

// DatosQr es lo que trae el QR del bicho.
type DatosQr struct{ ID, Codigo string }

var patronQr = regexp.MustCompile(`^SINTETICA:V1/([0-9A-F]{12})/([2-9A-HJKMNP-Z]{6})$`)

// LeerQr interpreta "SINTETICA:V1/<id>/<código>".
func LeerQr(texto string) (DatosQr, bool) {
	m := patronQr.FindStringSubmatch(strings.TrimSpace(texto))
	if m == nil {
		return DatosQr{}, false
	}
	return DatosQr{m[1], m[2]}, true
}

// IDValido: 12 caracteres hexadecimales en mayúscula.
var IDValido = regexp.MustCompile(`^[0-9A-F]{12}$`).MatchString
