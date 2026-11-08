package integraciones

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// halPrueba: el hardware de un robot de verdad (el núcleo por cgo) adentro
// del test. Lo llama el núcleo con su mutex tomado: no llama al paquete nucleo.
type halPrueba struct {
	mu          sync.Mutex
	kv          map[string][]byte
	inicio      time.Time
	alNotificar func(car int, d []byte)
}

func (h *halPrueba) AhoraMs() uint32 { return uint32(time.Since(h.inicio).Milliseconds()) }
func (h *halPrueba) Semilla() uint32 { return 7 }
func (h *halPrueba) MinutoDelDia() int {
	t := time.Now().In(nube.Zona())
	return t.Hour()*60 + t.Minute()
}
func (h *halPrueba) Epoch() uint32 { return uint32(protocolo.EpochLocal(time.Now().In(nube.Zona()))) }
func (h *halPrueba) Bateria() int  { return -1 }
func (h *halPrueba) Guardar(k string, d []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kv[k] = append([]byte(nil), d...)
	return true
}
func (h *halPrueba) Leer(k string) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	d, ok := h.kv[k]
	return d, ok
}
func (h *halPrueba) EscribirTrozo(k string, desde uint32, d []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	b := h.kv[k]
	if need := int(desde) + len(d); len(b) < need {
		b = append(b, make([]byte, need-len(b))...)
	}
	copy(b[desde:], d)
	h.kv[k] = b
	return true
}
func (h *halPrueba) LeerTrozo(k string, desde uint32, n int) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, ok := h.kv[k]
	if !ok || int(desde) > len(b) {
		return nil, false
	}
	return append([]byte(nil), b[desde:min(len(b), int(desde)+n)]...), true
}
func (h *halPrueba) Borrar(k string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.kv, k)
	return true
}
func (h *halPrueba) Notificar(int, []byte)                      {}
func (h *halPrueba) NotificarNube(car int, d []byte)            { h.alNotificar(car, d) }
func (h *halPrueba) RedCambio()                                 {}
func (h *halPrueba) OtaDescargar(nucleo.Manifiesto, string) int { return -1 }
func (h *halPrueba) OtaCancelar()                               {}
func (h *halPrueba) Registrar(string)                           {}

// robotNucleo: un robot conectado que es el núcleo de verdad. Lo que le
// manda la nube va a nucleo.NubeEscribir; su 0xCC vuelve como el evento
// "charla" (lo que hace el trabajador de la nube).
type robotNucleo struct {
	robotsFalsos
	integ *Integraciones
}

func (r *robotNucleo) Enviar(id string, car byte, m []byte) error {
	r.robotsFalsos.Enviar(id, car, m) //nolint:errcheck
	nucleo.NubeEscribir(int(car), m)
	return nil
}

func nuevoRobotNucleo(id, usuario string) *robotNucleo {
	r := &robotNucleo{robotsFalsos: robotsFalsos{duenos: map[string]string{id: usuario}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}}
	h := &halPrueba{kv: map[string][]byte{}, inicio: time.Now(), alNotificar: func(car int, d []byte) {
		if car == protocolo.CarComando && len(d) >= 2 && d[0] == protocolo.CmdCharlar|protocolo.Respuesta && r.integ != nil {
			go r.integ.Evento(id, "charla", map[string]any{"marco": append([]byte(nil), d...)})
		}
	}}
	nucleo.Iniciar(h, nucleo.Perfil{ID: id, Nombre: "virtual", Version: "0.0.0", Ancho: 320, Alto: 172})
	nucleo.Conexiones(true, true)
	return r
}

// configEnLaBase: el cambio de claves de verdad (validado por el núcleo,
// versión nueva con su autor), como servidor.cambiarClaves.
func configEnLaBase(a *almacen.Almacen) CambiarConfig {
	var mu sync.Mutex
	return func(ctx context.Context, bicho, usuario, autor string, claves map[string]any) error {
		mu.Lock()
		defer mu.Unlock()
		m := map[string]any{}
		if c, err := a.UltimaConfig(ctx, bicho); err == nil {
			json.Unmarshal([]byte(c.Texto), &m) //nolint:errcheck
		}
		for k, v := range claves {
			if v == nil {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
		b, _ := json.Marshal(m)
		if err := nucleo.ValidarConfig(b); err != nil {
			return err
		}
		_, err := a.NuevaConfig(ctx, bicho, string(b), protocolo.Crc32(b), autor, &usuario)
		return err
	}
}

func configDe(t *testing.T, a *almacen.Almacen, id string) (map[string]json.RawMessage, string) {
	t.Helper()
	c, err := a.UltimaConfig(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal([]byte(c.Texto), &m) //nolint:errcheck
	return m, c.Autor
}

func esperarQue(t *testing.T, que string, f func() bool, extra ...func() string) {
	t.Helper()
	for limite := time.Now().Add(5 * time.Second); time.Now().Before(limite); time.Sleep(20 * time.Millisecond) {
		if f() {
			return
		}
	}
	for _, e := range extra {
		que += " (" + e() + ")"
	}
	t.Fatal("no pasó: " + que)
}

// Sin el robot conectado: lo hace la nube en la config (y lo de una vez se
// saca cuando pasa); sin IA contesta el motor; con IA, solo lo que no entiende.
func TestCharlaMotorSinElRobot(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	ia := &anthropicFalso{clave: "sk-prueba", respuesta: `{"texto":"Qué sé yo, la vida es linda.","expresion":"pensando"}`}
	sia := httptest.NewServer(ia)
	defer sia.Close()
	u, id := usuarioConBicho(t, a)
	guardarConfig(t, a, id, `{"version":2,"nombre":"Pipo","dueno":{"nombre":"Ana"},"despertador":{"hora":"09:30","dias":"SD","texto":"¡Arriba!"}}`)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{}} // desconectado
	sinIA := Nuevo(Opciones{}, a, robots, eventos.Nuevo(), sinLog)
	sinIA.UsarConfig(configEnLaBase(a))
	ahora := time.Date(2026, 11, 9, 10, 0, 0, 0, nube.Zona()) // lunes
	sinIA.v19.ahora = func() time.Time { return ahora }

	// Sin IA: contesta el motor, aunque no entienda.
	r, err := sinIA.Charlar(ctx, u.ID, id, "¿qué opinás de la vida?", "web")
	if err != nil || r.Origen != OrigenMotor || r.Texto == "" || r.Enviado || r.Guardado {
		t.Fatalf("sin IA: %+v %v", r, err)
	}

	// "despertame a las 7": mañana (martes) a las 7, en la config, con lo
	// demás que tenía el despertador; la frase, para cuando se conecte.
	r, err = sinIA.Charlar(ctx, u.ID, id, "despertame a las 7", "web")
	if err != nil || r.Origen != OrigenMotor || r.Accion != "alarma" || !r.Guardado || r.Enviado {
		t.Fatalf("alarma: %+v %v", r, err)
	}
	m, autor := configDe(t, a, id)
	if autor != AutorCharla || !iguales(m["despertador"], map[string]any{"hora": "07:00", "dias": "M", "activo": true, "texto": "¡Arriba!"}) {
		t.Fatalf("despertador: %s (%s)", m["despertador"], autor)
	}
	// Al conectarse le dice la frase.
	robots.mu.Lock()
	robots.estados[id] = protocolo.Estado{Nombre: "Pipo"}
	robots.mu.Unlock()
	sinIA.decirPendiente(id)
	md, _ := protocolo.Decir(r.Texto, r.Expresion)
	esperarEnviado(t, robots, fmt.Sprintf("%s 2 %x", id, md))
	robots.mu.Lock()
	delete(robots.estados, id)
	robots.mu.Unlock()

	// Un recordatorio de una vez y otro de todos los días; un gusto.
	if r, err := sinIA.Charlar(ctx, u.ID, id, "recordame comprar pan a las 18", "telegram"); err != nil || r.Accion != "recordatorio" || !r.Guardado {
		t.Fatalf("recordatorio: %+v %v", r, err)
	}
	if r, err := sinIA.Charlar(ctx, u.ID, id, "recordame regar las plantas todos los días a las 21", "web"); err != nil || !r.Guardado {
		t.Fatalf("todos los días: %+v %v", r, err)
	}
	if _, err := sinIA.Charlar(ctx, u.ID, id, "me gusta el fútbol", "web"); err != nil {
		t.Fatal(err)
	}
	if gs, _ := a.Gustos(ctx, id); len(gs) != 1 || gs[0] != "el fútbol" {
		t.Fatalf("gustos: %v", gs)
	}
	if c := sinIA.contextoCharla(ctx, u.ID, id); len(c.Gustos) != 1 || c.Dueno != "Ana" || c.Robot == "" || c.Minuto != 600 {
		t.Fatalf("contexto: %+v", c)
	}
	m, _ = configDe(t, a, id)
	if !iguales(m["recordatorios"], []any{
		map[string]any{"hora": "18:00", "dias": "L", "texto": "Comprar pan"},
		map[string]any{"hora": "21:00", "texto": "Regar las plantas"},
	}) {
		t.Fatalf("recordatorios: %s", m["recordatorios"])
	}

	// Pasa el día: lo de una vez se va; vuelve el despertador de antes.
	ahora = ahora.Add(24 * time.Hour)
	sinIA.limpiarUnaVez(ctx)
	m, _ = configDe(t, a, id)
	if !iguales(m["despertador"], map[string]any{"hora": "09:30", "dias": "SD", "texto": "¡Arriba!"}) ||
		!iguales(m["recordatorios"], []any{map[string]any{"hora": "21:00", "texto": "Regar las plantas"}}) {
		t.Fatalf("después: %s %s", m["despertador"], m["recordatorios"])
	}

	// Con IA: lo que entiende el motor no la llama; lo que no, sí.
	hub := eventos.Nuevo()
	sse := hub.Suscribir(u.ID, "")
	defer hub.Desuscribir(sse)
	conIA := Nuevo(Opciones{V19: OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: sia.URL}}, a, robots, hub, sinLog)
	conIA.UsarConfig(configEnLaBase(a))
	// El SSE "charla" dice de dónde vino cada respuesta.
	origenSSE := func() any {
		for {
			select {
			case e := <-sse.C:
				if e.Tipo == "charla" {
					return e.Datos.(map[string]any)["origen"]
				}
			case <-time.After(2 * time.Second):
				return nil
			}
		}
	}
	if r, err := conIA.Charlar(ctx, u.ID, id, "despertame todos los días a las 8", "web"); err != nil || r.Origen != OrigenMotor ||
		r.Quedan != CharlasPorDia || ia.cantidad() != 0 {
		t.Fatalf("con IA, el motor: %+v %v (%d)", r, err, ia.cantidad())
	}
	if o := origenSSE(); o != OrigenMotor {
		t.Fatalf("SSE del motor: %v", o)
	}
	m, _ = configDe(t, a, id)
	if !iguales(m["despertador"], map[string]any{"hora": "08:00", "activo": true, "texto": "¡Arriba!"}) {
		t.Fatalf("todos los días: %s", m["despertador"])
	}
	r, err = conIA.Charlar(ctx, u.ID, id, "¿qué opinás de la vida?", "web")
	if err != nil || r.Origen != OrigenIA || r.Texto != "Qué sé yo, la vida es linda." || ia.cantidad() != 1 || r.Quedan != CharlasPorDia-1 {
		t.Fatalf("con IA: %+v %v", r, err)
	}
	if o := origenSSE(); o != OrigenIA {
		t.Fatalf("SSE de la IA: %v", o)
	}
	ia.mu.Lock()
	sys, _ := json.Marshal(ia.pedidos[0]["system"])
	ia.mu.Unlock()
	if !strings.Contains(string(sys), "el fútbol") {
		t.Fatalf("los gustos a la IA: %s", sys)
	}
}

// Con el robot conectado (el núcleo de verdad): la nube le manda 0x4C, lo
// charla y lo hace él y su 0xCC deja guardado lo que hace falta. Por MQTT,
// como el robot-celu.
func TestCharlaMotorConElRobot(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	ia := &anthropicFalso{clave: "sk-prueba", respuesta: `{"texto":"Ni idea, pero me gusta pensarlo.","expresion":"pensando"}`}
	sia := httptest.NewServer(ia)
	defer sia.Close()
	u, id := usuarioConBicho(t, a)
	guardarConfig(t, a, id, `{"version":2,"nombre":"Pipo","dueno":{"nombre":"Ana"}}`)
	robots := nuevoRobotNucleo(id, u.ID)
	i := Nuevo(Opciones{V19: OpcionesV19{AnthropicClave: "sk-prueba", AnthropicURL: sia.URL}}, a, robots, eventos.Nuevo(), sinLog)
	i.UsarConfig(configEnLaBase(a))
	robots.integ = i

	// Por HTTP: el robot contesta con su frase (la del 0xCC).
	r, err := i.Charlar(ctx, u.ID, id, "despertame todos los días a las 7", "web")
	if err != nil || r.Origen != OrigenMotor || !r.Enviado || r.Accion != "alarma" || !strings.Contains(r.Texto, "7:00") || ia.cantidad() != 0 {
		t.Fatalf("http: %+v %v", r, err)
	}
	m4c, _ := protocolo.Charlar("despertame todos los días a las 7")
	if !strings.Contains(enviadosDe(&robots.robotsFalsos), fmt.Sprintf("%x", m4c)) {
		t.Fatalf("no le mandó el 0x4C: %s", enviadosDe(&robots.robotsFalsos))
	}
	esperarQue(t, "el despertador en la config", func() bool {
		m, autor := configDe(t, a, id)
		return autor == AutorCharla && iguales(m["despertador"], map[string]any{"hora": "07:00", "activo": true})
	})

	// Por MQTT (el robot-celu): un gusto y un recordatorio de todos los días.
	i.limite = nuevoLimitador() // de a una cada 2 s
	i.CharlaMQTT(id, []byte(`{"texto":"me gusta el mate"}`))
	esperarQue(t, "el gusto", func() bool { gs, _ := a.Gustos(ctx, id); return len(gs) == 1 && gs[0] == "el mate" })
	i.limite = nuevoLimitador() // de a una cada 2 s
	i.CharlaMQTT(id, []byte(`{"texto":"recordame tomar la pastilla todos los días a las 22"}`))
	esperarQue(t, "el recordatorio", func() bool {
		m, _ := configDe(t, a, id)
		return iguales(m["recordatorios"], []any{map[string]any{"hora": "22:00", "texto": "Tomar la pastilla"}})
	}, func() string {
		m, _ := configDe(t, a, id)
		return string(m["recordatorios"]) + " " + enviadosDe(&robots.robotsFalsos)
	})
	// Uno de una vez queda en su RAM: la config no cambia.
	antes, _ := configDe(t, a, id)
	i.limite = nuevoLimitador() // de a una cada 2 s
	i.CharlaMQTT(id, []byte(`{"texto":"en 10 minutos recordame sacar la pizza"}`))
	m4c, _ = protocolo.Charlar("en 10 minutos recordame sacar la pizza")
	esperarEnviado(t, &robots.robotsFalsos, fmt.Sprintf("%x", m4c))
	time.Sleep(200 * time.Millisecond)
	if despues, _ := configDe(t, a, id); !iguales(antes["recordatorios"], despues["recordatorios"]) {
		t.Fatalf("de una vez: %s", despues["recordatorios"])
	}
	if ia.cantidad() != 0 {
		t.Fatalf("llamó a la IA %d veces", ia.cantidad())
	}

	// Lo que no entiende va a la IA y lo dice con 0x40.
	vaciar(&robots.robotsFalsos)
	i.limite = nuevoLimitador() // de a una cada 2 s
	i.CharlaMQTT(id, []byte(`{"texto":"¿qué opinás de la vida?"}`))
	md, _ := protocolo.Decir("Ni idea, pero me gusta pensarlo.", "pensando")
	esperarEnviado(t, &robots.robotsFalsos, fmt.Sprintf("%s 2 %x", id, md))
	if ia.cantidad() != 1 {
		t.Fatalf("IA: %d", ia.cantidad())
	}
}

func TestLetrasYCuando(t *testing.T) {
	if letrasDias(0x1F) != "LMXJV" || letrasDias(0x7F) != "" || letrasDias(0x60) != "SD" {
		t.Fatal(letrasDias(0x1F), letrasDias(0x60))
	}
	i := &Integraciones{}
	ahora := time.Date(2026, 11, 9, 10, 0, 30, 0, nube.Zona())
	i.v19.ahora = func() time.Time { return ahora }
	for _, c := range []struct {
		a    accionCharla
		hora string
	}{
		{accionCharla{Minuto: 420, Dia: 1, EnMinutos: -1}, "2026-11-10 07:00"},
		{accionCharla{Minuto: 420, Dia: -1, EnMinutos: -1}, "2026-11-10 07:00"}, // ya pasó: mañana
		{accionCharla{Minuto: 1080, Dia: 0, EnMinutos: -1}, "2026-11-09 18:00"},
		{accionCharla{Minuto: -1, Dia: -1, EnMinutos: 10}, "2026-11-09 10:11"},
	} {
		if t2, ok := i.cuandoCharla(c.a); !ok || t2.Format("2006-01-02 15:04") != c.hora {
			t.Errorf("%+v: %v", c.a, t2)
		}
	}
}
