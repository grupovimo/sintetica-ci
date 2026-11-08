package protocolo

// Los marcos de la v1.7 (característica Comando): jugar, los avisos del celu
// con su app y su contacto, la respuesta al "¿Cómo estás?" y hacia dónde viene
// un sonido. Ver docs/protocolo-ble.md y docs/nube.md, "Un bicho que acompaña".

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Operaciones de la v1.7.
const (
	CmdJugar = 0x43 // [u8 juego] (255 = terminar)
	CmdAviso = 0x44 // [u8 tipo][n][app][n][contacto]
	CmdAnimo = 0x45 // [u8 0 bien, 1 más o menos, 2 mal]
	CmdMirar = 0x46 // [i8 x][i8 y] (−100 a 100)
)

// Tipos de las entradas del registro de la v1.7 (los avisa el bicho solo,
// junto con los eventos 34 juego y 35 animo).
const (
	RegistroJuego = 5 // dato: juego·16 + resultado
	RegistroAnimo = 6 // dato: 0 bien, 1 más o menos, 2 mal
)

// Juegos, en el orden del protocolo.
var Juegos = []string{"ppt", "reflejos", "mirada", "dado", "moneda"}

// TerminarJuego: el dato de 0x43 que termina el juego que esté jugando.
const TerminarJuego = 255

// otrosNombresJuego: cómo más se puede decir cada juego.
var otrosNombresJuego = map[string]string{
	"piedra-papel-tijera": "ppt", "piedra-papel-o-tijera": "ppt", "piedra": "ppt",
	"segui-la-mirada": "mirada", "seguí-la-mirada": "mirada", "mirar": "mirada",
	"reflejo": "reflejos", "dados": "dado", "monedita": "moneda", "cara-o-cruz": "moneda",
}

// ResultadosJuego: el resultado de una partida (en el registro tipo 5).
var ResultadosJuego = []string{"perdio", "gano", "empate", "sin-resultado"}

// Animos: la respuesta al "¿Cómo estás?", en el orden del protocolo.
var Animos = []string{"bien", "medio", "mal"}

// NombreJuego normaliza un nombre ("Piedra papel tijera", "dado"…) al del
// protocolo; "terminar" también vale. ok = false si no existe.
func NombreJuego(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(s)), "-")
	if s == "terminar" || s == "parar" || s == "fin" {
		return "terminar", true
	}
	if o, ok := otrosNombresJuego[s]; ok {
		s = o
	}
	for _, j := range Juegos {
		if j == s {
			return j, true
		}
	}
	return "", false
}

// Jugar arma el marco 0x43 (el nombre como lo acepta NombreJuego).
func Jugar(juego string) ([]byte, error) {
	j, ok := NombreJuego(juego)
	if !ok {
		return nil, errors.New("el juego es ppt (piedra, papel o tijera), reflejos, mirada, dado, moneda o terminar")
	}
	if j == "terminar" {
		return []byte{CmdJugar, TerminarJuego}, nil
	}
	for i, x := range Juegos {
		if x == j {
			return []byte{CmdJugar, byte(i)}, nil
		}
	}
	return nil, errors.New("juego desconocido")
}

// MaxCampoAviso: los bytes de la app y del contacto de un aviso.
const MaxCampoAviso = 24

// Aviso arma el marco 0x44: tipo "notificacion" o "llamada" (v1.7), o "pago",
// "mail", "github", "pedido" u "otro" (v1.9: entonces app es el título corto,
// como "$12.000" o "PR #12", y contacto quién); la app y el contacto se cortan
// a 24 bytes sin partir una letra.
func Aviso(tipo, app, contacto string) ([]byte, error) {
	if tipo == "" {
		tipo = "notificacion"
	}
	t := -1
	for i, x := range TiposAviso {
		if x == tipo {
			t = i
		}
	}
	if t < 0 {
		return nil, errors.New("el tipo es notificacion, llamada, pago, mail, github, pedido u otro")
	}
	if !utf8.ValidString(app) || !utf8.ValidString(contacto) {
		return nil, errors.New("la app y el contacto son texto (UTF-8)")
	}
	app = CortarUTF8(strings.TrimSpace(app), MaxCampoAviso)
	contacto = CortarUTF8(strings.TrimSpace(contacto), MaxCampoAviso)
	m := []byte{CmdAviso, byte(t), byte(len(app))}
	m = append(m, app...)
	m = append(m, byte(len(contacto)))
	return append(m, contacto...), nil
}

// NumeroAnimo: "bien" 0, "medio" (o "mas o menos") 1, "mal" 2.
func NumeroAnimo(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "mas-o-menos", "más-o-menos", "mas o menos", "más o menos", "masomenos", "regular":
		s = "medio"
	}
	for i, a := range Animos {
		if a == s {
			return i, true
		}
	}
	return 0, false
}

// Animo arma el marco 0x45 (0 bien, 1 más o menos, 2 mal).
func Animo(valor int) ([]byte, error) {
	if valor < 0 || valor >= len(Animos) {
		return nil, errors.New("el ánimo es 0 (bien), 1 (más o menos) o 2 (mal)")
	}
	return []byte{CmdAnimo, byte(valor)}, nil
}

// Mirar arma el marco 0x46: hacia dónde viene un sonido (x e y de −100 a 100).
func Mirar(x, y int) []byte {
	return []byte{CmdMirar, byte(int8(entre(x, -100, 100))), byte(int8(entre(y, -100, 100)))}
}

// LeerJuego decodifica el dato de una entrada de registro tipo 5.
func LeerJuego(dato uint8) (juego, resultado string) {
	j, r := int(dato>>4), int(dato&15)
	juego, resultado = "?", "?"
	if j < len(Juegos) {
		juego = Juegos[j]
	}
	if r < len(ResultadosJuego) {
		resultado = ResultadosJuego[r]
	}
	return
}

// --- La memoria nueva (v1.7: desde el byte 93) ---

// Con qué soñó (los bits del byte 101), las fechas festejadas (los del 106).
var (
	Suenos = []string{"corazon", "estrella", "nota", "oveja", "pesadilla"}
	Fechas = []string{"anio-nuevo", "navidad", "dia-del-amigo", "dia-del-nino", "aniversario"}
)

// DiaContadores: el día de los contadores de hoy, como lo anota el bicho
// (día epoch local % 251 + 1).
func DiaContadores(ahora time.Time) uint8 {
	_, off := ahora.Zone()
	local := ahora.Unix() + int64(off)
	return uint8((local/86400)%251 + 1)
}

// EpochLocal: los segundos como los cuenta el bicho (con su zona).
func EpochLocal(ahora time.Time) int64 {
	_, off := ahora.Zone()
	return ahora.Unix() + int64(off)
}

// AlDia deja en cero lo "de hoy" si los contadores son de otro día y saca las
// fechas festejadas si son de otro año (ahora, en la zona del bicho).
func (m *Memoria) AlDia(ahora time.Time) {
	if m.DiaContadores != DiaContadores(ahora) {
		m.JuegosHoy, m.SuenosHoy, m.AnimoHoy, m.AguaHoy = 0, 0, 0, 0
		m.SonoConHoy, m.EsperoHoy = []string{}, nil
	}
	if m.AnioFechas != uint8(ahora.Year()%256) {
		m.Fechas = []string{}
	}
}

// DeHoy: los contadores de hoy son de hoy.
func (m *Memoria) DeHoy(ahora time.Time) bool { return m.DiaContadores == DiaContadores(ahora) }

func leerMemoriaNueva(m []byte, r *Memoria) {
	le := func(i int) uint16 { return uint16(m[i]) | uint16(m[i+1])<<8 }
	r.JuegosGanados, r.JuegosJugados, r.MejorReflejo = le(93), le(95), le(97)
	r.JuegosHoy, r.SuenosHoy = m[99], m[100]
	r.SonoConHoy, r.Fechas = []string{}, []string{}
	for k, s := range Suenos {
		if m[101]&(1<<k) != 0 {
			r.SonoConHoy = append(r.SonoConHoy, s)
		}
	}
	r.AnimoHoy = m[102]
	if m[103] > 0 && m[103] <= 24 {
		h := int(m[103]) - 1
		r.EsperoHoy = &h
	}
	r.DiaContadores, r.AnioFechas = m[104], m[105]
	for k, f := range Fechas {
		if m[106]&(1<<k) != 0 {
			r.Fechas = append(r.Fechas, f)
		}
	}
	r.AguaHoy = m[107]
}
