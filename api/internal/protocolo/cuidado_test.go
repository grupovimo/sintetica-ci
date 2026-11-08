package protocolo

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestMarcosV17(t *testing.T) {
	casos := map[string]string{"ppt": "\x43\x00", "Piedra papel tijera": "\x43\x00", "reflejos": "\x43\x01", "seguí la mirada": "\x43\x02",
		"dado": "\x43\x03", "moneda": "\x43\x04", "terminar": "\x43\xff"}
	for n, want := range casos {
		if m, err := Jugar(n); err != nil || string(m) != want {
			t.Errorf("jugar %q: %x %v", n, m, err)
		}
	}
	if _, err := Jugar("ajedrez"); err == nil {
		t.Fatal("ajedrez no existe")
	}
	m, err := Aviso("notificacion", "whatsapp", "Mamá")
	if err != nil || !bytes.Equal(m, append(append([]byte{0x44, 0, 8}, "whatsapp"...), append([]byte{5}, "Mamá"...)...)) {
		t.Fatalf("aviso: %x %v", m, err)
	}
	m, _ = Aviso("llamada", "", "ñññññññññññññññññññññññññ") // 50 bytes: se corta a 24 sin partir una letra
	if m[1] != 1 || m[2] != 0 || m[3] != 24 || len(m) != 4+24 {
		t.Fatalf("llamada: %x", m)
	}
	if _, err := Aviso("fax", "", ""); err == nil {
		t.Fatal("tipo raro")
	}
	for k, n := range []string{"bien", "más o menos", "mal"} {
		v, ok := NumeroAnimo(n)
		m, err := Animo(v)
		if !ok || err != nil || !bytes.Equal(m, []byte{0x45, byte(k)}) {
			t.Errorf("ánimo %s: %x", n, m)
		}
	}
	if _, err := Animo(3); err == nil {
		t.Fatal("ánimo 3")
	}
	if m := Mirar(-150, 40); !bytes.Equal(m, []byte{0x46, 0x9c, 40}) {
		t.Fatalf("mirar: %x", m)
	}
	if j, r := LeerJuego(3*16 + 1); j != "dado" || r != "gano" {
		t.Fatalf("juego: %s %s", j, r)
	}
	if Eventos[33] != "despertador" || Eventos[34] != "juego" || Eventos[35] != "animo" || len(Eventos) != 45 {
		t.Fatalf("eventos: %v", Eventos[33:])
	}
}

func memoriaDePrueba(f func(m []byte)) []byte {
	m := make([]byte, LargoMemoria)
	m[0] = 1
	f(m)
	binary.LittleEndian.PutUint32(m[LargoMemoria-4:], Crc32(m[:LargoMemoria-4]))
	return m
}

func TestMemoriaV17(t *testing.T) {
	zona := time.FixedZone("ART", -3*3600)
	ahora := time.Date(2026, 11, 8, 22, 0, 0, 0, zona)
	m := memoriaDePrueba(func(m []byte) {
		m[92] = 7 // afecto ganado hoy (lo de antes sigue igual)
		binary.LittleEndian.PutUint16(m[93:], 12)
		binary.LittleEndian.PutUint16(m[95:], 30)
		binary.LittleEndian.PutUint16(m[97:], 245)
		m[99], m[100], m[101] = 3, 2, 1<<3|1<<1
		m[102], m[103], m[104] = 1, 20, DiaContadores(ahora)
		m[105], m[106], m[107] = byte(2026%256), 1<<0|1<<4, 4
	})
	r, err := LeerMemoria(m)
	if err != nil {
		t.Fatal(err)
	}
	if r.AfectoHoy != 7 || r.JuegosGanados != 12 || r.JuegosJugados != 30 || r.MejorReflejo != 245 || r.JuegosHoy != 3 || r.SuenosHoy != 2 ||
		len(r.SonoConHoy) != 2 || r.SonoConHoy[0] != "estrella" || r.SonoConHoy[1] != "oveja" || r.AnimoHoy != 1 ||
		r.EsperoHoy == nil || *r.EsperoHoy != 19 || r.AguaHoy != 4 || len(r.Fechas) != 2 || r.Fechas[1] != "aniversario" {
		t.Fatalf("%+v", r)
	}
	r.AlDia(ahora)
	if !r.DeHoy(ahora) || r.JuegosHoy != 3 || len(r.Fechas) != 2 {
		t.Fatalf("hoy: %+v", r)
	}
	// Al otro día (pasada la medianoche de la zona del bicho) lo de hoy vale 0;
	// al otro año, las fechas también.
	r.AlDia(time.Date(2026, 11, 9, 0, 1, 0, 0, zona))
	if r.JuegosHoy != 0 || r.SuenosHoy != 0 || r.AnimoHoy != 0 || r.EsperoHoy != nil || len(r.SonoConHoy) != 0 || r.JuegosGanados != 12 || len(r.Fechas) != 2 {
		t.Fatalf("mañana: %+v", r)
	}
	r.AlDia(time.Date(2027, 1, 1, 10, 0, 0, 0, zona))
	if len(r.Fechas) != 0 {
		t.Fatalf("el año que viene: %v", r.Fechas)
	}
	// Una memoria vieja (ceros desde el 93) se lee igual.
	v, err := LeerMemoria(memoriaDePrueba(func([]byte) {}))
	if err != nil || v.JuegosHoy != 0 || v.EsperoHoy != nil || v.SonoConHoy == nil || v.Fechas == nil {
		t.Fatalf("vieja: %+v %v", v, err)
	}
	// El día de los contadores: día epoch local % 251 + 1.
	if d := DiaContadores(ahora); int(d) != int((ahora.Unix()-3*3600)/86400%251+1) {
		t.Fatalf("día: %d", d)
	}
}
