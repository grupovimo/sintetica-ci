package integraciones

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

func TestLeerCuidado(t *testing.T) {
	c := leerCuidado(`{"version":2}`)
	if !c.animo || !c.extranar || c.resumen || c.extranarDias != 2 || c.despierta != 8*60 || c.duerme != 23*60+30 {
		t.Fatalf("por defecto: %+v", c)
	}
	c = leerCuidado(`{"cuidado":{"animo":false,"extranar":false,"extranarDias":9,"resumen":true},"horario":{"despierta":"22:00","duerme":"02:00"}}`)
	if c.animo || c.extranar || !c.resumen || c.extranarDias != 2 || c.horaDelResumen() != 23*60+59 {
		t.Fatalf("puestos: %+v", c)
	}
	// Un horario que cruza la medianoche: a la 01:00 lleva 3 h despierto (de 4).
	if d, l := c.despierto(time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC)); d != 180 || l != 240 {
		t.Fatalf("despierto: %d de %d", d, l)
	}
}

func TestTextoResumen(t *testing.T) {
	dos, diecinueve, bien := 2, 19, "bien"
	r := Resumen{Juegos: 3, Ganados: &dos, Suenos: 1, SonoCon: []string{"oveja"}, Espero: &diecinueve, Animo: &bien}
	if s := textoResumen("Pipo", r); s != "Hoy Pipo jugó 3 veces (ganó 2), soñó con ovejas, te esperó a las 19 y le dijiste que estabas bien." {
		t.Fatalf("%q", s)
	}
	uno, mal := 1, "mal"
	r = Resumen{Juegos: 1, Ganados: &uno, Suenos: 2, SonoCon: []string{"estrella", "pesadilla", "nota"}, Agua: 1, Animo: &mal}
	if s := textoResumen("Pipo", r); s != "Hoy Pipo jugó una vez y ganó, soñó con estrellas y música, tuvo una pesadilla, te recordó tomar agua una vez y le dijiste que estabas mal." {
		t.Fatalf("%q", s)
	}
	if s := textoResumen("Pipo", Resumen{Juegos: 4}); s != "Hoy Pipo jugó 4 veces." {
		t.Fatalf("sin saber cuántas ganó: %q", s)
	}
	if s := textoResumen("Pipo", Resumen{}); s != "Hoy Pipo tuvo un día tranquilo." {
		t.Fatalf("nada: %q", s)
	}
}

// memoriaCon arma una memoria v1 (112 bytes, con su CRC) con lo de hoy.
func memoriaCon(m time.Time, ultima int64, f func(b []byte)) []byte {
	b := make([]byte, protocolo.LargoMemoria)
	b[0] = 1
	binary.LittleEndian.PutUint32(b[5:], uint32(ultima))
	b[104] = protocolo.DiaContadores(m)
	f(b)
	binary.LittleEndian.PutUint32(b[protocolo.LargoMemoria-4:], protocolo.Crc32(b[:protocolo.LargoMemoria-4]))
	return b
}

// otroBichoDe le suma un bicho al usuario.
func otroBichoDe(t *testing.T, a *almacen.Almacen, u almacen.Usuario) string {
	t.Helper()
	ctx := context.Background()
	id := azar(6)
	if err := a.AsegurarBicho(ctx, id, "virtual", true); err != nil {
		t.Fatal(err)
	}
	clave := sha256.Sum256([]byte(id))
	a.CrearCredencialPendiente(ctx, id, u.ID, clave[:], time.Now().Add(time.Hour)) //nolint:errcheck
	if acc, err := a.UsarCredencial(ctx, id, clave[:]); err != nil || !acc.Ok {
		t.Fatalf("reclamar: %v", err)
	}
	return id
}

func botonDe(update, chat int64, datos string) map[string]any {
	return map[string]any{"update_id": update, "callback_query": map[string]any{
		"id": "q" + fmt.Sprint(update), "from": map[string]any{"id": chat, "username": "ana"}, "data": datos,
		"message": map[string]any{"message_id": 9, "chat": map[string]any{"id": chat, "type": "private"}, "text": "x"},
	}}
}

func TestCuidadoPorTelegram(t *testing.T) {
	a := abrirBase(t)
	token := "123456:TOKEN-CUIDADO-" + azar(4)
	falso := &telegramFalso{token: token}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	otro := otroBichoDe(t, a, u)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID, otro: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1}, a, robots, eventos.Nuevo(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	enlace, _, err := i.CrearVinculo(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	chat := chatAzar()
	if _, err := i.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i.Desvincular(context.Background(), u.ID) }) //nolint:errcheck

	// Hoy a las 21:00 (hora de los bichos), despierto desde las 18:00 hasta las 23:00.
	hoy := time.Now().In(nube.Zona())
	m := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 21, 0, 0, 0, nube.Zona())
	cfg := `{"version":2,"horario":{"despierta":"18:00","duerme":"23:00"}}`
	rs, _ := i.robotsDe(ctx, u.ID)
	var pipo robot
	for _, r := range rs {
		if r.ID == id {
			pipo = r
		}
	}
	// Sin memoria no se pregunta nada.
	i.revisarCuidado(ctx, chat, pipo, cfg, m)
	// Con memoria: no contestó hoy y no lo ve hace 3 días.
	if err := a.GuardarMemoria(ctx, id, memoriaCon(m, protocolo.EpochLocal(m)-3*86400, func([]byte) {})); err != nil {
		t.Fatal(err)
	}
	i.revisarCuidado(ctx, chat, pipo, cfg, m)
	i.revisarCuidado(ctx, chat, pipo, cfg, m.Add(time.Minute)) // no se repite
	enviados := falso.esperar(t, 2)
	time.Sleep(50 * time.Millisecond)
	falso.mu.Lock()
	n := len(falso.enviados)
	falso.mu.Unlock()
	if n != 2 || enviados[0]["text"] != "Pipo pregunta: ¿Cómo estás hoy?" || enviados[1]["text"] != "Te extraño 🥺 — Pipo" {
		t.Fatalf("%d mensajes: %v", n, enviados)
	}
	teclado := fmt.Sprint(enviados[0]["reply_markup"])
	if !strings.Contains(teclado, "animo:"+id+":2") || !strings.Contains(teclado, "😐 Más o menos") {
		t.Fatalf("botones: %s", teclado)
	}
	// Apagado en la config, no.
	sinNada := `{"cuidado":{"animo":false,"extranar":false}}`
	i.revisarCuidado(ctx, chat, robot{Bicho: almacen.Bicho{ID: otro}, nombre: "Pepa"}, sinNada, m)

	// Toca "Mal": le manda 0x45 2 al bicho y contesta con empatía.
	base := time.Now().UnixNano() % 1_000_000_000
	falso.mu.Lock()
	falso.updates = []map[string]any{botonDe(base+1, chat, "animo:"+id+":2"), mensajeDe(base+2, chat, "/jugar dado Pipo"),
		mensajeDe(base+3, chat, "/jugar ajedrez Pipo"), mensajeDe(base+4, chat, "/jugar Pipo")}
	falso.mu.Unlock()
	go i.escucharTelegram(ctx)
	enviados = falso.esperar(t, 6)
	texto := func(k int) string { s, _ := enviados[k]["text"].(string); return s }
	if !strings.HasPrefix(texto(2), "Uh, lo siento mucho. Pipo está con vos 💛\n\n") {
		t.Fatalf("mal: %q", texto(2))
	}
	if !strings.Contains(texto(3), "¡A jugar! Pipo empezó 🎲 Dado") || !strings.Contains(texto(4), "No conozco ese juego") ||
		!strings.Contains(fmt.Sprint(enviados[5]["reply_markup"]), "jugar:"+id+":moneda") {
		t.Fatalf("jugar: %q %q %v", texto(3), texto(4), enviados[5])
	}
	if l := robots.vaciar(); len(l) != 2 || l[0] != fmt.Sprintf("%s 2 4502", id) || l[1] != fmt.Sprintf("%s 2 4303", id) {
		t.Fatalf("al robot: %v", l)
	}
	_, h, _ := i.Automatizaciones(ctx, id)
	vistos := map[string]bool{}
	for _, e := range h {
		vistos[e.Automatizacion+"/"+e.Accion] = e.Ok
	}
	if !vistos["como-estas/telegram"] || !vistos["te-extrano/telegram"] || !vistos["animo/robot"] {
		t.Fatalf("historial: %v", vistos)
	}

	// Su día contado, a la hora de dormir (con resumen: true).
	if err := a.GuardarMemoria(ctx, id, memoriaCon(m, protocolo.EpochLocal(m), func(b []byte) {
		b[99], b[100], b[101], b[102], b[103] = 3, 1, 1<<3, 1, 20
	})); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"gano", "perdio", "gano"} {
		i.Evento(id, "juego", map[string]any{"juego": "dado", "resultado": r})
	}
	cfg = `{"version":2,"cuidado":{"resumen":true},"horario":{"despierta":"08:00","duerme":"21:00"}}`
	i.revisarCuidado(ctx, chat, pipo, cfg, m)
	i.revisarCuidado(ctx, chat, pipo, cfg, m)
	enviados = falso.esperar(t, 7)
	if s, _ := enviados[6]["text"].(string); s != "Hoy Pipo jugó 3 veces (ganó 2), soñó con ovejas, te esperó a las 19 y le dijiste que estabas bien." {
		t.Fatalf("resumen: %q", s)
	}
	r, err := i.ResumenDe(ctx, id)
	if err != nil || r.Juegos != 3 || r.Ganados == nil || *r.Ganados != 2 || r.Animo == nil || *r.Animo != "bien" || r.Espero == nil || *r.Espero != 19 ||
		len(r.SonoCon) != 1 || r.Actualizada == nil {
		t.Fatalf("ResumenDe: %+v %v", r, err)
	}
	if r, _ := i.ResumenDe(ctx, otro); !strings.Contains(r.Texto, "Todavía no sé") || r.SonoCon == nil {
		t.Fatalf("sin memoria: %+v", r)
	}
}

func TestEntreRobots(t *testing.T) {
	a := abrirBase(t)
	u, id := usuarioConBicho(t, a)
	otro := otroBichoDe(t, a, u)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID, otro: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{}, a, robots, eventos.Nuevo(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if i.QuiereRegistro(id) {
		t.Fatal("con el otro desconectado no hace falta el registro")
	}
	i.saludarEntreRobots(id)
	if l := robots.vaciar(); len(l) != 0 {
		t.Fatalf("solo: %v", l)
	}
	robots.mu.Lock()
	robots.estados[otro] = protocolo.Estado{Nombre: "Pepa"}
	robots.mu.Unlock()
	if !i.QuiereRegistro(id) {
		t.Fatal("con otro conectado se lee el registro (para bicho-cerca)")
	}
	i.saludarEntreRobots(id)
	i.saludarEntreRobots(otro) // el otro también lo vio: no se repite
	hola := func(n string) string { m, _ := protocolo.Decir("¡Hola, "+n+"!", "contento"); return fmt.Sprintf("%x", m) }
	saludar := fmt.Sprintf("%x", protocolo.Comportamiento("saludar"))
	want := []string{id + " 2 " + saludar, id + " 2 " + hola("Pepa"), otro + " 2 " + saludar, otro + " 2 " + hola("Pipo")}
	if l := robots.vaciar(); strings.Join(l, "\n") != strings.Join(want, "\n") {
		t.Fatalf("al robot:\n%s\nesperaba:\n%s", strings.Join(l, "\n"), strings.Join(want, "\n"))
	}
	_, h, _ := i.Automatizaciones(context.Background(), otro)
	if len(h) != 1 || h[0].Automatizacion != "entre-"+id || h[0].Evento != "bicho-cerca" || !h[0].Ok {
		t.Fatalf("historial: %+v", h)
	}
}
