package protocolo

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strconv"
	"strings"
	"testing"

	vec "sintetica/shared/protocolo"
)

type vectores struct {
	Comandos []map[string]any `json:"comandos"`
	Memoria  struct {
		Hex    string         `json:"hex"`
		Campos map[string]int `json:"campos"`
	} `json:"memoria"`
	Config struct {
		JSON         string   `json:"json"`
		TamanoBloque int      `json:"tamanoBloque"`
		Escrituras   []string `json:"escrituras"`
	} `json:"config"`
	Qoi struct {
		Hex          string   `json:"hex"`
		Espacio      int      `json:"espacio"`
		TamanoBloque int      `json:"tamanoBloque"`
		Escrituras   []string `json:"escrituras"`
	} `json:"qoi"`
	Estados []struct {
		Campos map[string]any `json:"campos"`
		Hex    string         `json:"hex"`
	} `json:"estados"`
	Ota struct {
		Binario, Sha256, Version, Perfil, Mensaje, Publica, Firma, Manifiesto, URL, Descargar string
		TamanoBloque                                                                          int
		Escrituras                                                                            []string
	} `json:"ota"`
	Qr []struct {
		Texto  string  `json:"texto"`
		ID     *string `json:"id"`
		Codigo string  `json:"codigo"`
	} `json:"qr"`
}

func cargar(t *testing.T) vectores {
	var v vectores
	if err := json.Unmarshal(vec.Vectores, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func hx(b []byte) string {
	s := make([]string, len(b))
	for i, x := range b {
		s[i] = hex.EncodeToString([]byte{x})
	}
	return strings.Join(s, " ")
}

func dehex(s string) []byte {
	b, _ := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	return b
}

func TestComandos(t *testing.T) {
	v := cargar(t)
	por := map[string]map[string]any{}
	for _, c := range v.Comandos {
		por[c["nombre"].(string)] = c
	}
	s := func(n, k string) string { return por[n][k].(string) }
	red, _ := Red(s("red", "ssid"), s("red", "clave"))
	abierta, _ := Red("Bar", "")
	nube, _ := Nube(s("nube", "url"), s("nube", "usuario"), s("nube", "clave"))
	ev, _ := Evento("sacudida")
	casos := map[string][]byte{
		"expresion enamorado": Expresion("enamorado"), "expresion automatica": Expresion(""),
		"nombre Toto": Nombre("Toto"), "nombre con tilde": Nombre("Ñandú"), "evento sacudida": ev,
		"vincular": Vincular("K7Q2XM"), "autenticar": Autenticar(dehex(s("autenticar", "clave"))),
		"desvincular": Desvincular(), "pedir config": PedirConfig(), "imagen borrar 3": BorrarImagen(3),
		"imagen listar": ListarImagenes(), "imagen pedir 5": PedirImagen(5), "imagen listar sprites": ListarSprites(), "imagen borrar sprite 0": BorrarImagen(PrimerSprite), "red": red, "red abierta": abierta,
		"nube": nube, "olvidar red": OlvidarRed(), "reproducir saludo": Reproducir("saludo"), "reproducir parar": Reproducir(""),
		"ota cancelar": {OtaCancelar},
		"hora":         Hora(1793000000, -180), "vida": Vida(nil), "memoria": PedirMemoria(), "registro": PedirRegistro(),
	}
	si := true
	casos["vida superposicion"] = Vida(&si)
	casos["comportamiento practicar-caras"] = Comportamiento("practicar-caras")
	fuerte, _ := Evento("sacudida-fuerte")
	casos["evento sacudida-fuerte"] = fuerte
	for n, b := range casos {
		if hx(b) != s(n, "hex") {
			t.Errorf("%s: %s, esperaba %s", n, hx(b), s(n, "hex"))
		}
	}
}

func TestConfigEImagenEnBloques(t *testing.T) {
	v := cargar(t)
	for i, e := range EscriturasDeConfig([]byte(v.Config.JSON), v.Config.TamanoBloque) {
		if hx(e) != v.Config.Escrituras[i] {
			t.Fatalf("config, escritura %d distinta", i)
		}
	}
	for i, e := range EscriturasDeImagen(v.Qoi.Espacio, dehex(v.Qoi.Hex), v.Qoi.TamanoBloque) {
		if hx(e) != v.Qoi.Escrituras[i] {
			t.Fatalf("imagen, escritura %d distinta", i)
		}
	}
}

func TestEstados(t *testing.T) {
	v := cargar(t)
	for _, e := range v.Estados {
		est, err := LeerEstado(dehex(e.Hex))
		if err != nil {
			t.Fatal(err)
		}
		c := e.Campos
		crc, _ := strconv.ParseUint(c["crcConfigHex"].(string), 16, 32)
		if est.Expresion != c["expresion"] || est.Nombre != c["nombre"] || est.ID != c["id"] || est.Perfil != c["perfil"] ||
			est.Version != c["version"] || est.Wifi != c["wifi"] || est.Nube != c["nube"] || est.Actualizando != c["actualizando"] ||
			est.CrcConfig != uint32(crc) || est.Encendido != uint32(c["encendido"].(float64)) || est.Pantalla.Ancho != int(c["ancho"].(float64)) || est.Pantalla.Redonda != c["redonda"] {
			t.Errorf("estado mal leído: %+v", est)
		}
		if (c["bateria"] == nil) != (est.Bateria == nil) {
			t.Errorf("batería: %v", est.Bateria)
		}
	}
}

func TestOtaYFirma(t *testing.T) {
	v := cargar(t)
	bin := dehex(v.Ota.Binario)
	var m Manifiesto
	m.Tamano = uint32(len(bin))
	copy(m.Sha256[:], dehex(v.Ota.Sha256))
	copy(m.Firma[:], dehex(v.Ota.Firma))
	m.Version, m.Perfil = v.Ota.Version, v.Ota.Perfil
	if sha256.Sum256(bin) != m.Sha256 {
		t.Fatal("sha256 distinto")
	}
	if m.Mensaje() != v.Ota.Mensaje {
		t.Fatalf("mensaje %q", m.Mensaje())
	}
	mb, _ := m.Bytes()
	if hx(mb) != v.Ota.Manifiesto {
		t.Fatal("manifiesto distinto")
	}
	d, _ := DescargarOta(m, v.Ota.URL)
	if hx(d) != v.Ota.Descargar {
		t.Fatal("descargar distinto")
	}
	esc, _ := EscriturasDeOta(m, bin, v.Ota.TamanoBloque)
	for i, e := range esc {
		if hx(e) != v.Ota.Escrituras[i] {
			t.Fatalf("ota, escritura %d distinta", i)
		}
	}
	// La firma de los vectores se verifica con la clave pública de prueba.
	blk, _ := pem.Decode([]byte(v.Ota.Publica))
	pub, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(m.Mensaje()))
	r, s := new(big.Int).SetBytes(m.Firma[:32]), new(big.Int).SetBytes(m.Firma[32:])
	if !ecdsa.Verify(pub.(*ecdsa.PublicKey), h[:], r, s) {
		t.Fatal("la firma de prueba no verifica")
	}
}

func TestQrYArmador(t *testing.T) {
	v := cargar(t)
	for _, q := range v.Qr {
		d, ok := LeerQr(q.Texto)
		if ok != (q.ID != nil) || (ok && (d.ID != *q.ID || d.Codigo != q.Codigo)) {
			t.Errorf("qr %q", q.Texto)
		}
	}
	var a Armador
	texto := []byte(`{"version":1,"nombre":"Toto"}`)
	a.Agregar(append([]byte{ConfigTrozo, 10, 0, 0, 0}, texto[10:]...))
	a.Agregar(append([]byte{ConfigTrozo, 0, 0, 0, 0}, texto[:10]...))
	got, err := a.Cerrar(uint32(len(texto)), Crc32(texto))
	if err != nil || string(got) != string(texto) {
		t.Fatalf("armador: %v", err)
	}
}

func TestMemoria(t *testing.T) {
	v := cargar(t)
	b := dehex(v.Memoria.Hex)
	m, err := LeerMemoria(b)
	if err != nil {
		t.Fatal(err)
	}
	c := v.Memoria.Campos
	if int(m.Dias) != c["dias"] || int(m.Caricias) != c["caricias"] || int(m.Toques) != c["toques"] || m.Horas[9] != c["hora9"] {
		t.Errorf("memoria: %+v", m)
	}
	if len(m.Hitos) != 3 || m.Hitos[0] != "primer-dia" || m.Hitos[2] != "cien-caricias" {
		t.Errorf("hitos: %v", m.Hitos)
	}
	r, err := Restaurar(b)
	if err != nil || hx(r) != porNombre(v, "restaurar") {
		t.Errorf("restaurar: %v", err)
	}
	b[10] ^= 1
	if _, err := LeerMemoria(b); err == nil {
		t.Error("tendría que rechazar el CRC")
	}
}

func porNombre(v vectores, n string) string {
	for _, c := range v.Comandos {
		if c["nombre"] == n {
			return c["hex"].(string)
		}
	}
	return ""
}

func TestVida(t *testing.T) {
	b := []byte{CmdVida | Respuesta, Ok, 255, 128, 0, 10, 20, 30, 40, 1, 2, 3, 4, 5, 6, 3,
		5, 'h', 'i', 'p', 'o', 'x', 7, 'n', 'e', 'u', 't', 'r', 'a', 'l', 1, 2, 'a', 'b', 150, 30}
	v, err := LeerVida(b)
	if err != nil {
		t.Fatal(err)
	}
	if v.Estado["energia"] != 1 || !v.Dormido || !v.Sonando || v.Comportamiento != "hipox" || v.Visible != "neutral" ||
		len(v.Candidatos) != 1 || v.Candidatos[0].Puntaje != 1.5 || v.Fps != 30 {
		t.Errorf("vida: %+v", v)
	}
	if _, err := LeerVida(b[:12]); err == nil {
		t.Error("tendría que rechazar una corta")
	}
}

func qoiPrueba(w, h uint32) []byte {
	b := []byte("qoif")
	b = binary.BigEndian.AppendUint32(b, w)
	b = binary.BigEndian.AppendUint32(b, h)
	b = append(b, 3, 0, 0xC0|61)
	return append(b, 0, 0, 0, 0, 0, 0, 0, 1)
}

func saniPrueba(w, h int, cuadros [][]byte, ms []uint16) []byte {
	b := []byte{'s', 'a', 'n', 'i', 1, byte(len(cuadros)), byte(w), byte(w >> 8), byte(h), byte(h >> 8)}
	off := 10 + 10*len(cuadros)
	for i, c := range cuadros {
		b = binary.LittleEndian.AppendUint16(b, ms[i])
		b = binary.LittleEndian.AppendUint32(b, uint32(off))
		b = binary.LittleEndian.AppendUint32(b, uint32(len(c)))
		off += len(c)
	}
	for _, c := range cuadros {
		b = append(b, c...)
	}
	return b
}

func TestTamanoImagen(t *testing.T) {
	q := qoiPrueba(240, 240)
	if w, h, ok := TamanoImagen(q); !ok || w != 240 || h != 240 {
		t.Fatalf("qoi: %d %d %v", w, h, ok)
	}
	a := saniPrueba(240, 240, [][]byte{q, q}, []uint16{100, 100})
	if w, h, ok := TamanoImagen(a); !ok || w != 240 || h != 240 {
		t.Fatalf("animación: %d %d %v", w, h, ok)
	}
	malas := map[string][]byte{
		"basura":         []byte("nada que ver acá, ni qoi ni sani"),
		"cuadro de otro": saniPrueba(240, 240, [][]byte{q, qoiPrueba(100, 100)}, []uint16{100, 100}),
		"muy rápido":     saniPrueba(240, 240, [][]byte{q}, []uint16{5}),
		"sin cuadros":    saniPrueba(240, 240, nil, nil),
		"cortada":        a[:len(a)-3],
	}
	for nombre, b := range malas {
		if _, _, ok := TamanoImagen(b); ok {
			t.Errorf("%s: la aceptó", nombre)
		}
	}
}
