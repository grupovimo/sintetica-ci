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
	ConfigInicio      = 0x01
	ConfigBloque      = 0x02
	ConfigFin         = 0x03
	ConfigPedir       = 0x04
	CmdExpresion      = 0x10
	CmdNombre         = 0x11
	CmdEvento         = 0x12
	CmdVincular       = 0x13
	CmdAutenticar     = 0x14
	CmdDesvincular    = 0x15
	CmdRed            = 0x16
	CmdNube           = 0x17
	CmdOlvidarRed     = 0x18
	CmdReproducir     = 0x19
	CmdHora           = 0x1A // v1.4: la hora
	CmdVida           = 0x1B // v1.4: el estado del motor de vida
	CmdMemoria        = 0x1C // v1.4: pedir la memoria
	CmdRestaurar      = 0x1D // v1.4: restaurarla (mudanza)
	CmdRegistro       = 0x1E // v1.4: el registro de eventos
	CmdComportamiento = 0x1F // v1.4: empezar un comportamiento (para probarlo)
	ImgInicio         = 0x20
	ImgBloque         = 0x21
	ImgFin            = 0x22
	ImgBorrar         = 0x23
	ImgListar         = 0x24
	ImgPedir          = 0x25
	ImgListarSprites  = 0x26 // v1.5: los sprites de las caras de imagen (espacios 8 a 19)
	OtaInicio         = 0x30
	OtaBloque         = 0x31
	OtaFin            = 0x32
	OtaDescargar      = 0x33
	OtaProgreso       = 0x34
	OtaCancelar       = 0x35
	Respuesta         = 0x80
	ConfigTrozo       = 0x84
	ConfigCierre      = 0x85
	ImgTrozo          = 0xA7
	ImgCierre         = 0xA8
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
// Los primeros 8 son de la v1; el resto, del motor de vida (v1.4).
var Eventos = []string{
	"tacto", "doble-toque", "levantado", "sacudida", "boca-abajo", "vinculado", "app-abierta", "despertado",
	"caricia", "muchos-toques", "sacudida-suave", "sacudida-fuerte", "inclinado", "caida", "voz", "nombre",
	"ruido", "musica", "silencio", "notificacion", "llamada", "bateria-baja", "bateria-critica", "enchufado",
	"desenchufado", "cargado", "sin-wifi", "cambio-de-cara", "bicho-cerca", "volvio", "hito",
	"recordatorio", "pomodoro", // los genera el sistema (y los avisa solo: ver docs/protocolo-ble.md)
	"despertador", // v1.6: empezó a sonar la alarma (también lo avisa solo)
}

// VariablesEstado y Rasgos: el estado interno y la personalidad, en el orden del protocolo.
var VariablesEstado = []string{"energia", "animo", "afecto", "aburrimiento", "curiosidad", "sueno", "estres"}
var Rasgos = []string{"energia", "sociabilidad", "picardia", "timidez", "curiosidad", "malGenio"}

// LargoMemoria es el largo de la memoria del motor de vida (con su CRC).
const LargoMemoria = 112

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
func Expresion(nombre string) []byte      { return append([]byte{CmdExpresion}, nombre...) }
func Nombre(nombre string) []byte         { return append([]byte{CmdNombre}, nombre...) }
func Reproducir(nombre string) []byte     { return append([]byte{CmdReproducir}, nombre...) }
func Vincular(codigo string) []byte       { return append([]byte{CmdVincular}, codigo...) }
func Autenticar(clave []byte) []byte      { return append([]byte{CmdAutenticar}, clave...) }
func Desvincular() []byte                 { return []byte{CmdDesvincular} }
func OlvidarRed() []byte                  { return []byte{CmdOlvidarRed} }
func PedirConfig() []byte                 { return []byte{ConfigPedir} }
func PedirMemoria() []byte                { return []byte{CmdMemoria} }
func PedirRegistro() []byte               { return []byte{CmdRegistro} }
func Comportamiento(nombre string) []byte { return append([]byte{CmdComportamiento}, nombre...) }

// Hora arma el comando Hora: epoch UTC y la zona en minutos (Argentina: -180).
func Hora(epochUtc uint32, zonaMinutos int16) []byte {
	b := append([]byte{CmdHora}, u32(epochUtc)...)
	return binary.LittleEndian.AppendUint16(b, uint16(zonaMinutos))
}

// Vida pide el estado del motor de vida; con superposicion != nil la prende o la apaga.
func Vida(superposicion *bool) []byte {
	if superposicion == nil {
		return []byte{CmdVida}
	}
	if *superposicion {
		return []byte{CmdVida, 1}
	}
	return []byte{CmdVida, 0}
}

// Restaurar manda una memoria (la mudanza de una placa a otra).
func Restaurar(memoria []byte) ([]byte, error) {
	if _, err := LeerMemoria(memoria); err != nil {
		return nil, err
	}
	return append([]byte{CmdRestaurar}, memoria...), nil
}

// Memoria es lo que el bicho recuerda (el diario de la app y la web).
type Memoria struct {
	Nacimiento        uint32             `json:"nacimiento"`
	UltimaInteraccion uint32             `json:"ultimaInteraccion"`
	RencorHasta       uint32             `json:"rencorHasta"`
	UltimoDia         uint32             `json:"ultimoDia"`
	Dias              uint16             `json:"dias"`
	Racha             uint16             `json:"racha"`
	RachaMax          uint16             `json:"rachaMax"`
	Caricias          uint32             `json:"caricias"`
	Toques            uint32             `json:"toques"`
	Sacudidas         uint32             `json:"sacudidas"`
	Horas             []int              `json:"horas"`
	Gustos            map[string]int     `json:"gustos"`
	Hitos             []string           `json:"hitos"`
	AnioCumpleBicho   uint16             `json:"anioCumpleBicho"`
	AnioCumpleDueno   uint16             `json:"anioCumpleDueno"`
	Deriva            map[string]float64 `json:"deriva"`
	Estado            map[string]float64 `json:"estado"`
	EpochEstado       uint32             `json:"epochEstado"`
	AusenciaMaxima    uint16             `json:"ausenciaMaxima"`
	AfectoHoy         uint8              `json:"afectoHoy"`
}

// Los gustos y los hitos, en el orden del núcleo (motor_vida.cpp).
var Gustos = []string{"caricia", "tacto", "voz", "musica", "nombre", "levantado", "sacudida-suave", "app-abierta"}
var Hitos = []string{"primer-dia", "siete-dias", "treinta-dias", "cien-caricias", "cien-dias", "mil-caricias"}

// LeerMemoria decodifica los 112 bytes de la memoria (y verifica su CRC).
func LeerMemoria(m []byte) (*Memoria, error) {
	if len(m) != LargoMemoria || m[0] != 1 {
		return nil, fmt.Errorf("memoria inválida")
	}
	if binary.LittleEndian.Uint32(m[LargoMemoria-4:]) != Crc32(m[:LargoMemoria-4]) {
		return nil, fmt.Errorf("memoria con otro CRC")
	}
	le := binary.LittleEndian
	r := &Memoria{
		Nacimiento: le.Uint32(m[1:]), UltimaInteraccion: le.Uint32(m[5:]), RencorHasta: le.Uint32(m[9:]), UltimoDia: le.Uint32(m[13:]),
		Dias: le.Uint16(m[17:]), Racha: le.Uint16(m[19:]), RachaMax: le.Uint16(m[21:]),
		Caricias: le.Uint32(m[23:]), Toques: le.Uint32(m[27:]), Sacudidas: le.Uint32(m[31:]),
		Gustos: map[string]int{}, Deriva: map[string]float64{}, Estado: map[string]float64{}, Hitos: []string{},
		AnioCumpleBicho: le.Uint16(m[69:]), AnioCumpleDueno: le.Uint16(m[71:]),
		EpochEstado: le.Uint32(m[86:]), AusenciaMaxima: le.Uint16(m[90:]), AfectoHoy: m[92],
	}
	for h := 0; h < 24; h++ {
		r.Horas = append(r.Horas, int(m[35+h]))
	}
	for k, g := range Gustos {
		r.Gustos[g] = int(int8(m[59+k]))
	}
	bits := le.Uint16(m[67:])
	for k, h := range Hitos {
		if bits&(1<<k) != 0 {
			r.Hitos = append(r.Hitos, h)
		}
	}
	for k, n := range Rasgos {
		r.Deriva[n] = float64(int8(m[73+k])) / 100
	}
	for k, n := range VariablesEstado {
		r.Estado[n] = float64(m[79+k]) / 255
	}
	return r, nil
}

// EstadoVida es la respuesta al comando Vida.
type EstadoVida struct {
	Estado         map[string]float64 `json:"estado"`
	Rasgos         map[string]float64 `json:"rasgos"`
	Dormido        bool               `json:"dormido"`
	Sonando        bool               `json:"sonando"`
	Superposicion  bool               `json:"superposicion"`
	Comportamiento string             `json:"comportamiento"`
	Visible        string             `json:"visible"`
	Candidatos     []Candidato        `json:"candidatos"`
	Fps            int                `json:"fps"`
}

type Candidato struct {
	Nombre  string  `json:"nombre"`
	Puntaje float64 `json:"puntaje"`
}

// LeerVida decodifica [0x9B][0][7 estado][6 rasgos][flags][n][comportamiento][n][visible][n]{[n][nombre][puntaje]}[fps].
func LeerVida(b []byte) (*EstadoVida, error) {
	if len(b) < 2+7+6+1+3 || b[0] != CmdVida|Respuesta || b[1] != Ok {
		return nil, fmt.Errorf("no es una respuesta de Vida")
	}
	v := &EstadoVida{Estado: map[string]float64{}, Rasgos: map[string]float64{}, Candidatos: []Candidato{}}
	i := 2
	for _, n := range VariablesEstado {
		v.Estado[n] = float64(b[i]) / 255
		i++
	}
	for _, n := range Rasgos {
		v.Rasgos[n] = float64(b[i]) / 255
		i++
	}
	f := b[i]
	i++
	v.Dormido, v.Sonando, v.Superposicion = f&1 != 0, f&2 != 0, f&4 != 0
	campo := func() (string, bool) {
		if i >= len(b) || i+1+int(b[i]) > len(b) {
			return "", false
		}
		s := string(b[i+1 : i+1+int(b[i])])
		i += 1 + int(b[i])
		return s, true
	}
	var ok bool
	if v.Comportamiento, ok = campo(); !ok {
		return nil, fmt.Errorf("vida: corta")
	}
	if v.Visible, ok = campo(); !ok || i >= len(b) {
		return nil, fmt.Errorf("vida: corta")
	}
	n := int(b[i])
	i++
	for k := 0; k < n; k++ {
		nombre, ok := campo()
		if !ok || i >= len(b) {
			return nil, fmt.Errorf("vida: corta")
		}
		v.Candidatos = append(v.Candidatos, Candidato{nombre, float64(b[i]) / 100})
		i++
	}
	if i < len(b) {
		v.Fps = int(b[i])
	}
	return v, nil
}

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

// Los sprites de las caras de imagen (v1.5): los espacios 8 a 19 de la
// característica Imagen (el sprite n es el espacio 8 + n), QOI de 3 o 4
// canales de hasta 128 × 128.
const (
	PrimerSprite = 8
	MaxSprites   = 12
	LadoSprite   = 128
	// LimiteSprite: el peor caso de un QOI de 4 canales de 128 × 128.
	LimiteSprite = LadoSprite*LadoSprite*5 + 22
)

// EsSprite dice si un espacio de la característica Imagen es de un sprite.
func EsSprite(espacio int) bool { return espacio >= PrimerSprite && espacio < PrimerSprite+MaxSprites }

// ListarSprites pide los sprites ocupados.
func ListarSprites() []byte { return []byte{ImgListarSprites} }

// LeerListaSprites interpreta la respuesta a ListarSprites: [0xA6][0][máscara
// u16] + por sprite [espacio][u32 largo][u32 crc]. Los espacios son los de la
// característica (8 a 19).
func LeerListaSprites(b []byte) []EspacioImagen {
	var l []EspacioImagen
	for i := 4; i+9 <= len(b); i += 9 {
		l = append(l, EspacioImagen{int(b[i]), binary.LittleEndian.Uint32(b[i+1:]), binary.LittleEndian.Uint32(b[i+5:])})
	}
	return l
}

// TamanoSprite: el tamaño de un sprite (un QOI de 3 o 4 canales de 1 a 128 por lado).
func TamanoSprite(b []byte) (ancho, alto int, ok bool) {
	if len(b) < 22 || len(b) > LimiteSprite || string(b[:4]) != "qoif" || (b[12] != 3 && b[12] != 4) {
		return 0, 0, false
	}
	w, h := binary.BigEndian.Uint32(b[4:]), binary.BigEndian.Uint32(b[8:])
	return int(w), int(h), w >= 1 && h >= 1 && w <= LadoSprite && h <= LadoSprite
}

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
	} else if len(b) > 2 && r.Op != ImgListar && r.Op != ImgListarSprites && r.Op != OtaProgreso {
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
	// Los bytes tal cual (las apps lejos del bicho los decodifican como por BLE).
	Crudo []byte `json:"crudo,omitempty"`
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
	e.Crudo = append([]byte(nil), b...)
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

// TamanoImagen dice de qué tamaño es una imagen para la pantalla: un QOI o una
// animación "sani" (la cabecera, la tabla de cuadros y los cuadros en QOI; ver
// firmware/nucleo/qoi.h). ok = false si está mal armada.
func TamanoImagen(b []byte) (ancho, alto int, ok bool) {
	qoi := func(c []byte) (int, int, bool) {
		if len(c) < 22 || string(c[:4]) != "qoif" || (c[12] != 3 && c[12] != 4) {
			return 0, 0, false
		}
		w, h := binary.BigEndian.Uint32(c[4:]), binary.BigEndian.Uint32(c[8:])
		return int(w), int(h), w > 0 && h > 0 && w <= 4096 && h <= 4096
	}
	if len(b) < 10 || string(b[:4]) != "sani" {
		return qoi(b)
	}
	cuadros := int(b[5])
	ancho, alto = int(binary.LittleEndian.Uint16(b[6:])), int(binary.LittleEndian.Uint16(b[8:]))
	tabla := 10 + 10*cuadros
	if b[4] != 1 || cuadros < 1 || cuadros > MaxCuadros || len(b) < tabla {
		return 0, 0, false
	}
	for i := 0; i < cuadros; i++ {
		e := b[10+10*i:]
		ms := binary.LittleEndian.Uint16(e)
		off, largo := int(binary.LittleEndian.Uint32(e[2:])), int(binary.LittleEndian.Uint32(e[6:]))
		if ms < 20 || off < tabla || largo < 22 || off > len(b) || largo > len(b)-off {
			return 0, 0, false
		}
		if w, h, ok := qoi(b[off : off+largo]); !ok || w != ancho || h != alto {
			return 0, 0, false
		}
	}
	return ancho, alto, true
}

// MaxCuadros: cuántos cuadros puede tener una imagen animada.
const MaxCuadros = 64

// Tipos de las entradas del registro (comando 0x1E).
const (
	RegistroEvento         = 0
	RegistroComportamiento = 1
	RegistroDormir         = 2 // dato 1 = se durmió, 0 = se despertó
	RegistroHito           = 3
	RegistroRecordatorio   = 4 // el bicho mostró un recordatorio (dato: su índice en la config)
)

// EntradaRegistro es una entrada del registro del motor de vida.
type EntradaRegistro struct {
	Segundos uint32 `json:"segundos"` // desde que arrancó
	Tipo     uint8  `json:"tipo"`
	Dato     uint8  `json:"dato"`
}

// LeerRegistro decodifica [0x9E][0x00][n][n × (u32 segundos, u8 tipo, u8 dato)].
func LeerRegistro(b []byte) ([]EntradaRegistro, error) {
	if len(b) < 3 || b[0] != CmdRegistro|Respuesta || b[1] != Ok {
		return nil, errors.New("no es una respuesta de Registro")
	}
	n := int(b[2])
	if len(b) < 3+6*n {
		return nil, errors.New("registro: corto")
	}
	l := make([]EntradaRegistro, 0, n)
	for i := 0; i < n; i++ {
		e := b[3+6*i:]
		l = append(l, EntradaRegistro{binary.LittleEndian.Uint32(e), e[4], e[5]})
	}
	return l, nil
}
