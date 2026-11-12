package integraciones

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// Las frases de listas: cómo se dicen de verdad (con y sin tildes, con
// signos, con "che" y "porfa") y las que NO son de listas (siguen al motor).
func TestEntenderLista(t *testing.T) {
	existentes := []string{"Pendientes", "regalos para mamá"}
	casos := []struct {
		texto  string
		accion accionLista
		lista  string
		items  []string
		nomb   bool
	}{
		{"anotá yerba en la lista del súper", listaAgregar, "súper", []string{"yerba"}, false},
		{"anota yerba en la lista del super", listaAgregar, "súper", []string{"yerba"}, false},
		{"Agregá leche y pan al súper", listaAgregar, "súper", []string{"leche", "pan"}, false},
		{"agrega leche, pan y queso a la lista del super", listaAgregar, "súper", []string{"leche", "pan", "queso"}, false},
		{"sumá pilas a pendientes", listaAgregar, "Pendientes", []string{"pilas"}, false},
		{"che, anotame la yerba en el super porfa", listaAgregar, "súper", []string{"yerba"}, false},
		{"¿me anotás dulce de leche en la lista del súper?", listaAgregar, "súper", []string{"dulce de leche"}, false},
		{"agregá leche en polvo al súper", listaAgregar, "súper", []string{"leche en polvo"}, false},
		{"poné 1,5 kilos de papa en la lista de la compra", listaAgregar, "súper", []string{"1,5 kilos de papa"}, false},
		{"anotá en el súper: yerba, azúcar y café", listaAgregar, "súper", []string{"yerba", "azúcar", "café"}, false},
		{"agregá papel de regalo a la lista de regalos para mamá", listaAgregar, "regalos para mamá", []string{"papel de regalo"}, false},
		{"sumá un libro a la lista de cumple de Juan", listaAgregar, "cumple de juan", []string{"un libro"}, false},
		{"anotá llamar al plomero en la lista", listaAgregar, "", []string{"llamar al plomero"}, false},
		{"agregá ibuprofeno a la farmacia", listaAgregar, "farmacia", []string{"ibuprofeno"}, false},
		{"sacá la yerba de la lista", listaSacar, "", []string{"yerba"}, true},
		{"sacá la yerba", listaSacar, "", []string{"yerba"}, false},
		{"borrá el pan de la lista del súper", listaSacar, "súper", []string{"pan"}, true},
		{"quitá las pilas de pendientes", listaSacar, "Pendientes", []string{"pilas"}, true},
		{"ya compré el pan", listaTachar, "", []string{"pan"}, false},
		{"ya compre pan y leche", listaTachar, "", []string{"pan", "leche"}, false},
		{"compré la yerba", listaTachar, "", []string{"yerba"}, false},
		{"tachá la yerba del súper", listaTachar, "súper", []string{"yerba"}, true},
		{"marcá las pilas como compradas", listaTachar, "", []string{"pilas"}, false},
		{"ya tengo el regalo", listaTachar, "", []string{"regalo"}, false},
		{"¿qué hay en la lista del súper?", listaVer, "súper", nil, false},
		{"que hay en la lista del super", listaVer, "súper", nil, false},
		{"¿Qué me falta comprar?", listaVer, "súper", nil, false},
		{"qué tengo que comprar", listaVer, "súper", nil, false},
		{"qué tengo en pendientes", listaVer, "Pendientes", nil, false},
		{"leeme la lista", listaVer, "", nil, false},
		{"mostrame la lista de regalos para mama", listaVer, "regalos para mamá", nil, false},
		{"la lista del súper", listaVer, "súper", nil, false},
		{"borrá la lista del súper", listaVaciar, "súper", nil, true},
		{"vaciá la lista de pendientes", listaVaciar, "Pendientes", nil, true},
		{"limpiá la lista del súper", listaLimpiar, "súper", nil, true},
		{"sacá los tachados", listaLimpiar, "", nil, false},
		{"borrá todo de la lista del súper", listaVaciar, "súper", nil, true},
		{"¿qué listas tengo?", listaCuales, "", nil, false},
		{"mis listas", listaCuales, "", nil, false},
	}
	for _, c := range casos {
		p, ok := entenderLista(c.texto, existentes)
		if !ok || p.accion != c.accion || p.lista != c.lista || !reflect.DeepEqual(p.items, c.items) || p.nombrada != c.nomb {
			t.Errorf("%q: %+v %v; esperaba %v %q %q nombrada=%v", c.texto, p, ok, nombresAccionLista[c.accion], c.lista, c.items, c.nomb)
		}
	}
	// No son de listas.
	for _, texto := range []string{
		"recordame comprar pan a las 18", "anotá que mañana tengo dentista", "anotá yerba", "agregá una alarma a las 7",
		"poné música en spotify", "despertame a las 7", "borrá todo", "qué hay de nuevo", "¿qué tengo hoy?", "que me falta",
		"limpiá la pantalla", "vaciá", "anotá llevar el auto al taller", "hola", "ya está", "sumá 2 y 2", "listo",
	} {
		if p, ok := entenderLista(texto, existentes); ok && (p.accion == listaAgregar || p.accion == listaVer || p.accion == listaVaciar ||
			p.accion == listaLimpiar || p.accion == listaCuales) {
			t.Errorf("%q no es de listas: %+v", texto, p)
		}
	}
}

func TestEnumerarYSeparar(t *testing.T) {
	if s := enumerar("Súper: ", []string{"yerba", "pan", "queso", "leche descremada", "galletitas de agua", "fideos"}, 60); s != "Súper: yerba, pan, queso, leche descremada y 2 más" {
		t.Fatalf("enumerar: %q (%d)", s, len(s))
	}
	if s := enumerar("Anotado: ", []string{"yerba", "pan"}, 60); s != "Anotado: yerba y pan" {
		t.Fatalf("enumerar: %q", s)
	}
	if l := SepararItems("leche,pan y el queso, la yerba"); !reflect.DeepEqual(l, []string{"leche", "pan", "queso", "yerba"}) {
		t.Fatalf("separar: %q", l)
	}
	if l := SepararItems("pan, Pan"); len(l) != 1 {
		t.Fatalf("repetidos: %q", l)
	}
}

// El motor de charla (el mismo del robot) entiende "anotá…" como un
// recordatorio: por eso las listas van antes.
func TestElMotorSeRoboLasListas(t *testing.T) {
	var e nucleo.EstadoCharla
	r, err := nucleo.Charlar("anotá yerba en la lista del súper", nucleo.NuevoContextoCharla(), &e)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("el motor solo: %q → %s (no entendió: %v)", "anotá yerba en la lista del súper", r.Intencion, r.NoEntendi)
}

// Por charla, con la base: anotar, leer, tachar, sacar, vaciar; sin llamar a
// la IA; el robot lo dice en su pantalla; la web se entera.
func TestCharlaDeListas(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	ia := &anthropicFalso{clave: "sk-prueba", respuesta: `{"texto":"Hola.","expresion":"contento"}`}
	sia := httptest.NewServer(ia)
	defer sia.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	hub := eventos.Nuevo()
	sse := hub.Suscribir(u.ID, "")
	defer hub.Desuscribir(sse)
	i := Nuevo(Opciones{V19: OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: sia.URL}}, a, robots, hub, sinLog)

	charlar := func(texto string) RespuestaCharla {
		t.Helper()
		r, err := i.Charlar(ctx, u.ID, id, texto, "web")
		if err != nil {
			t.Fatalf("%q: %v", texto, err)
		}
		return r
	}
	r := charlar("anotá yerba y pan en la lista del súper")
	if r.Texto != "Anotado: yerba y pan" || r.Origen != OrigenMotor || r.Intencion != "lista" || !r.Enviado {
		t.Fatalf("anotar: %+v", r)
	}
	md, _ := protocolo.Decir("Anotado: yerba y pan", "contento")
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, md))
	hayListas := false
	for len(sse.C) > 0 {
		if (<-sse.C).Tipo == "listas" {
			hayListas = true
		}
	}
	if !hayListas {
		t.Fatal("no avisó a la web")
	}
	charlar("sumá queso, leche descremada, galletitas de agua y fideos al super")
	if r := charlar("¿qué hay en la lista del súper?"); r.Texto != "Súper: yerba, pan, queso, leche descremada y 2 más" {
		t.Fatalf("leer: %q", r.Texto)
	}
	if r := charlar("ya compré el pan"); r.Texto != "Tachado: pan" {
		t.Fatalf("tachar: %q", r.Texto)
	}
	if r := charlar("sacá la yerba"); r.Texto != "Saqué: yerba" {
		t.Fatalf("sacar: %q", r.Texto)
	}
	if r := charlar("anotá yerba en el súper"); r.Texto != "Anotado: yerba" {
		t.Fatalf("de nuevo: %q", r.Texto)
	}
	if r := charlar("anotá yerba en el súper"); r.Texto != "Ya estaba: yerba" {
		t.Fatalf("repetida: %q", r.Texto)
	}
	if r := charlar("sacá el champú de la lista"); r.Texto != "No estaba: champú" {
		t.Fatalf("no estaba: %q", r.Texto)
	}
	if r := charlar("sumá pilas a pendientes"); r.Texto != "Anotado en pendientes: pilas" {
		t.Fatalf("pendientes: %q", r.Texto)
	}
	ls, _ := a.Listas(ctx, u.ID)
	if len(ls) != 2 || ls[0].Nombre != "súper" || len(ls[0].Items) != 6 || len(ls[0].Pendientes()) != 5 {
		t.Fatalf("listas: %+v", ls)
	}
	// La IA las conoce.
	if s := i.resumenListas(ctx, u.ID); !strings.HasPrefix(s, "súper: queso, leche descremada") || !strings.Contains(s, "pendientes: pilas") {
		t.Fatalf("resumen: %q", s)
	}
	if ia.cantidad() != 0 {
		t.Fatalf("llamó a la IA %d veces", ia.cantidad())
	}
	// Lo que no es de listas sigue: "ya hice la tarea" no está en ninguna.
	if r := charlar("¿qué opinás de la vida?"); r.Origen != OrigenIA {
		t.Fatalf("no es de listas: %+v", r)
	}
	ia.mu.Lock()
	sistema := fmt.Sprint(ia.pedidos[len(ia.pedidos)-1]["system"])
	ia.mu.Unlock()
	if !strings.Contains(sistema, "súper: queso") {
		t.Fatalf("el prompt no tiene las listas: %s", sistema)
	}
	if r := charlar("ya hice la tarea"); r.Intencion == "lista" {
		t.Fatalf("tarea: %+v", r)
	}
	if r := charlar("limpiá la lista del súper"); r.Texto != "Saqué 1 tachadas de súper" {
		t.Fatalf("limpiar: %q", r.Texto)
	}
	if r := charlar("borrá la lista del súper"); r.Texto != "Vacié la lista súper" {
		t.Fatalf("vaciar: %q", r.Texto)
	}
	if r := charlar("qué me falta comprar"); r.Texto != "Súper: no falta nada" {
		t.Fatalf("vacía: %q", r.Texto)
	}
	// Por MQTT (el escritorio y el robot-celu): el mismo camino.
	robots.mu.Lock()
	robots.enviados = nil
	robots.mu.Unlock()
	i.CharlaMQTT(id, []byte(`{"texto":"anotá café en el súper"}`))
	md, _ = protocolo.Decir("Anotado: café", "contento")
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, md))
}

// Telegram: las frases de listas y /lista (aunque no tenga robots).
func TestListasPorTelegram(t *testing.T) {
	a := abrirBase(t)
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	token := "123456:TOKEN-LISTAS-" + azar(4)
	falso := &telegramFalso{token: token}
	srv := httptest.NewServer(falso)
	defer srv.Close()
	robots := &robotsFalsos{duenos: map[string]string{}, estados: map[string]protocolo.Estado{}}
	i := Nuevo(Opciones{TelegramToken: token, TelegramURL: srv.URL, TelegramEspera: 1}, a, robots, eventos.Nuevo(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	u, err := a.UsuarioPorEmail(ctx, strings.ToLower(azar(4))+"@ejemplo.com", false) // sin robots
	if err != nil {
		t.Fatal(err)
	}
	enlace, _, _ := i.CrearVinculo(ctx, u.ID)
	chat := chatAzar()
	if _, err := i.Vincular(ctx, strings.TrimPrefix(enlace, "https://t.me/sinteticalabs_bot?start="), chat, "@ana"); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UnixNano() % 1_000_000_000
	falso.updates = []map[string]any{
		mensajeDe(base+1, chat, "/lista"),
		mensajeDe(base+2, chat, "anotá yerba, pan y queso en el súper"),
		mensajeDe(base+3, chat, "ya compré el pan"),
		mensajeDe(base+4, chat, "sumá pilas a pendientes"),
		mensajeDe(base+5, chat, "/lista"),
		mensajeDe(base+6, chat, "/lista pendientes"),
		mensajeDe(base+7, chat, "/lista regalos"),
		mensajeDe(base+8, chat, "¿qué me falta comprar?"),
	}
	go i.escucharTelegram(ctx)
	m := falso.esperar(t, 8)
	textos := make([]string, len(m))
	for k := range m {
		textos[k], _ = m[k]["text"].(string)
	}
	esperado := []string{
		"No tenés listas todavía. Escribime «anotá yerba en el súper» y la armo.",
		"Anotado: yerba, pan y queso (súper).",
		"Tachado: pan.",
		"Anotado en pendientes: pilas (pendientes).",
		"Súper:\n1. yerba\n2. queso\n\n(1 tachada)\n\nOtras: pendientes (/lista nombre).",
		"Pendientes:\n1. pilas\n\nOtras: súper (/lista nombre).",
		"No tenés la lista «regalos». Tus listas: súper, pendientes.",
		"Súper:\n1. yerba\n2. queso\n\n(1 tachada)",
	}
	for k, e := range esperado {
		if textos[k] != e {
			t.Errorf("%d: %q; esperaba %q", k, textos[k], e)
		}
	}
}
