package edgetts

// El token Sec-MS-GEC v2 que pide el servicio "leer en voz alta" de Edge en
// cada conexión: cambia cada 5 minutos y sale de la hora (en ticks de
// Windows) más el TrustedClientToken. Es un port a Go de drm.py del paquete
// edge-tts (github.com/rany2/edge-tts, "feat: support Sec-MS-GEC v2 tokens",
// octubre de 2026): un SHA-256 con un prefijo escondido (arranca de un estado
// intermedio con constantes ajustadas) y un CS64. Si Microsoft lo cambia otra
// vez, mirá ese archivo y actualizá las constantes de acá y de constantes.go.

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
)

const epochWindows = 11644473600

var sha256Inicial = [8]uint32{0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A, 0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19}

var sha256Estado = [8]uint32{0xFD9FBEA2, 0x91243CE7, 0xBE499094, 0xCD2E1E0A, 0x6C006F49, 0x04BD35AB, 0xFEB0A7E4, 0x03EFFBD2}

var sha256Agenda = [14]uint32{0xEB24BF9E, 0x1558691F, 0xA0C09EE2, 0x0CFA32BA, 0xA622EE8C, 0xD1DD9491, 0xC0DD25A0,
	0xF9E70AB6, 0xF0C31573, 0x7058CE4E, 0x7188E606, 0x00E5AB33, 0xD258B060, 0x72766564}

var sha256K = [64]uint32{
	0x428A2F98, 0x71374491, 0xB5C0FBCF, 0xE9B5DBA5, 0x3956C25B, 0x59F111F1, 0x923F82A4, 0xAB1C5ED5,
	0xD807AA98, 0x12835B01, 0x243185BE, 0x550C7DC3, 0x72BE5D74, 0x80DEB1FE, 0x9BDC06A7, 0xC19BF174,
	0xE49B69C1, 0xEFBE4786, 0x0FC19DC6, 0x240CA1CC, 0x2DE92C6F, 0x4A7484AA, 0x5CB0A9DC, 0x76F988DA,
	0x983E5152, 0xA831C66D, 0xB00327C8, 0xBF597FC7, 0xC6E00BF3, 0xD5A79147, 0x06CA6351, 0x14292967,
	0x27B70A85, 0x2E1B2138, 0x4D2C6DFC, 0x53380D13, 0x650A7354, 0x766A0ABB, 0x81C2C92E, 0x92722C85,
	0xA2BFE8A1, 0xA81A664B, 0xC24B8B70, 0xC76C51A3, 0xD192E819, 0xD6990624, 0xF40E3585, 0x106AA070,
	0x19A4C116, 0x1E376C08, 0x2748774C, 0x34B0BCB5, 0x391C0CB3, 0x4ED8AA4A, 0x5B9CCA4F, 0x682E6FF3,
	0x748F82EE, 0x78A5636F, 0x84C87814, 0x8CC70208, 0x90BEFFFA, 0xA4506CEB, 0xBEF9A3F7, 0xC67178F2,
}

const (
	sha256Ajuste       = 0xC9DC06A7
	sha256AjusteSigma0 = 0x059C0B80
	sha256AjusteSigma1 = 0x000B92C0
	cs64Factor         = 0x82893933
	cs64Ajuste0        = 0xF2F90B1A
	cs64Ajuste1        = 0x0B1A0000
)

var cs64Coeficientes = [14][2]uint32{
	{0xE82E91B1, 0xB32700D0}, {0x44DE24C6, 0x88DFF51D}, {0x3BEB65BC, 0x29514038}, {0xC89C5933, 0xB328CBDF},
	{0x8D1EC211, 0xD405B4D5}, {0x314EF5D5, 0x26E55F3C}, {0xE92491B1, 0x607801C6}, {0xF0A5B9D5, 0x82B2C1FB},
	{0x06064CF3, 0xFA51A3DA}, {0xDA79FA60, 0x207CC464}, {0xBED20560, 0x3C3C74F3}, {0x89F9C9F8, 0xD2D14B61},
	{0x35FA38C6, 0xBB04A2C1}, {0xCB49E3C6, 0xC5626167},
}

func rotr(v uint32, n uint) uint32 { return v>>n | v<<(32-n) }
func rol16(v uint32) uint32        { return v<<16 | v>>16 }
func sigma0(v uint32) uint32       { return rotr(v, 7) ^ rotr(v, 18) ^ v>>3 }
func sigma1(v uint32) uint32       { return rotr(v, 17) ^ rotr(v, 19) ^ v>>10 }

// rondas: las de SHA-256 desde la ronda `desde` (14 para el primer bloque,
// que ya trae hechas las del prefijo escondido).
func rondas(estado [8]uint32, w []uint32, desde int) [8]uint32 {
	k := sha256K
	if desde == 14 {
		k[14] = sha256Ajuste
	}
	a, b, c, d, e, f, g, h := estado[0], estado[1], estado[2], estado[3], estado[4], estado[5], estado[6], estado[7]
	for i := desde; i < 64; i++ {
		t1 := h + (rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)) + (g ^ (e & (f ^ g))) + k[i] + w[i]
		t2 := (rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)) + ((a & b) | (c & (a | b)))
		a, b, c, d, e, f, g, h = t1+t2, a, b, c, d+t1, e, f, g
	}
	return [8]uint32{a, b, c, d, e, f, g, h}
}

func expandir(w []uint32, desde int) []uint32 {
	for i := desde; i < 64; i++ {
		w = append(w, sigma1(w[i-2])+w[i-7]+sigma0(w[i-15])+w[i-16])
	}
	return w
}

// inicioSha: el estado después del primer bloque (el prefijo escondido y los
// primeros 7 bytes).
func inicioSha(d []byte) [8]uint32 {
	w := make([]uint32, 14, 64)
	w = append(w, uint32(d[0])<<16|uint32(d[1])<<8|uint32(d[2]), binary.BigEndian.Uint32(d[3:7]))
	for i := 16; i < 30; i++ {
		v := sha256Agenda[i-16]
		if i-2 >= 14 {
			s := sigma1(w[i-2])
			if i == 16 {
				s ^= sha256AjusteSigma1
			}
			v += s
		}
		if i-7 >= 14 {
			v += w[i-7]
		}
		if i-15 >= 14 {
			s := sigma0(w[i-15])
			if i == 29 {
				s ^= sha256AjusteSigma0
			}
			v += s
		}
		w = append(w, v)
	}
	w = append(w, sigma1(w[28])+w[23]+sigma0(w[15])+w[14]+sha256Ajuste-sha256K[14])
	w = expandir(w, 31)
	t := rondas(sha256Estado, w, 14)
	var out [8]uint32
	for i := range out {
		out[i] = sha256Inicial[i] + t[i]
	}
	return out
}

func digestSha(d []byte) []byte {
	largo := 57 + len(d)
	relleno := append(append([]byte(nil), d[7:]...), 0x80)
	relleno = append(relleno, make([]byte, ((55-largo)%64+64)%64)...)
	relleno = binary.BigEndian.AppendUint64(relleno, uint64(largo)*8)
	estado := inicioSha(d[:7])
	for off := 0; off < len(relleno); off += 64 {
		w := make([]uint32, 16, 64)
		for i := range 16 {
			w[i] = binary.BigEndian.Uint32(relleno[off+4*i:])
		}
		t := rondas(estado, expandir(w, 16), 0)
		for i := range estado {
			estado[i] += t[i]
		}
	}
	out := make([]byte, 0, 32)
	for _, v := range estado {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

func palabrasPares(d []byte) []uint32 {
	r := append([]byte(nil), d...)
	for len(r)%4 != 0 {
		r = append(r, 0)
	}
	w := make([]uint32, 0, len(r)/4+1)
	for i := 0; i < len(r); i += 4 {
		w = append(w, binary.LittleEndian.Uint32(r[i:]))
	}
	if len(w)%2 == 1 {
		w = append(w, 0)
	}
	return w
}

func digestCS64(d, sha []byte) (uint32, uint32) {
	ka := binary.LittleEndian.Uint32(sha[0:]) | 1
	kb := binary.LittleEndian.Uint32(sha[4:]) | 1
	factores := [2]uint32{ka * cs64Factor, kb * cs64Factor}
	var terminos, desplaz []uint32
	for i, c := range cs64Coeficientes {
		terminos = append(terminos, c[0]*factores[i%2])
		desplaz = append(desplaz, c[1])
	}
	for i, w := range palabrasPares(append([]byte{0}, d...)) {
		var t, o uint32
		if i%2 == 0 {
			t, o = w*ka, rol16(w)*0xEF0569FB
		} else {
			t, o = w*kb, rol16(w)*0xC31713DB
		}
		if i == 0 {
			t += cs64Ajuste0 * factores[0]
			o += cs64Ajuste1
		}
		terminos = append(terminos, t)
		desplaz = append(desplaz, o)
	}
	var swapAcc, swapUlt, estado, acc uint32
	for i := 0; i < len(terminos); i += 2 {
		x, y := terminos[i], terminos[i+1]
		q := rol16(desplaz[i]+x) * 0x76F50F34
		swapAcc += rol16(q)*0xC3EFEA97 + q
		q = rol16(desplaz[i+1]+y) * 0x1881CE3C
		swapUlt = rol16(q)*0x35BD1EC9 + q
		swapAcc += swapUlt

		estado = estado*ka + x
		estado = rol16(estado) * 0xCF98B111
		estado = rol16(estado) * 0x87085B9F
		estado = rol16(estado) * 0x12CEB96D
		estado = rol16(estado) * 0x257E1D83
		acc += estado
		t := estado*kb + y
		t = rol16(t) * 0xA27416F5
		t = rol16(t) * 0xD38396FF
		estado = rol16(rol16(t)*0x7C932B89) * 0xBFA49F69
		acc += estado
	}
	return swapUlt ^ estado, swapAcc ^ acc
}

// TokenGEC: el Sec-MS-GEC v2 para ese momento (segundos unix, ya con la
// corrección del reloj): "<16 hex>-<64 hex>".
func TokenGEC(unix int64) string {
	s := unix + epochWindows
	s -= s % 300
	d := []byte(strconv.FormatInt(s, 10) + "0000000" + TokenCliente) // ticks de 100 ns
	sha := digestSha(d)
	a, b := digestCS64(d, sha)
	cs := binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, a), b)
	return strings.ToUpper(hex.EncodeToString(cs)) + "-" + strings.ToUpper(hex.EncodeToString(sha))
}
