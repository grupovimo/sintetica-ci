//go:build e2e

package e2e

// La v1.9 de punta a punta, con los dos bichos virtuales del compose: amigos
// y toques entre dos cuentas (y devolverlo con una caricia), hábitos con
// racha, presencia, avisos entrantes (GitHub firmado y Mercado Pago, una vez),
// la agenda con un .ics que sirve el test, la charla con el motor propio (una
// alarma que llega al robot) y entrar con Telegram (con un Telegram falso que
// sirve el test), sumarle un mail y ser admin por ADMIN_TELEGRAM.
//
// El .ics y el Telegram falso los sirve el test: la API (en Docker) los busca
// en E2E_HOST (host.docker.internal; en Linux, el compose lo mapea con
// host-gateway). El Telegram falso solo con E2E_TELEGRAM (":8097") y la API
// levantada con TELEGRAM_TOKEN y TELEGRAM_API_URL=http://host.docker.internal:8097.

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	virtual2  = env("E2E_VIRTUAL2", "http://localhost:8091")
	hostLocal = env("E2E_HOST", "host.docker.internal")
	sufijo    = time.Now().UnixNano() / 1000
	tg        *telegramFalso
)

func TestMain(m *testing.M) {
	if dir := os.Getenv("E2E_TELEGRAM"); dir != "" {
		tg = arrancarTelegramFalso(dir)
	}
	os.Exit(m.Run())
}

// --- Los eventos de la cuenta (SSE) ---

type evento struct {
	Tipo  string
	Bicho string
	Datos map[string]any
}

// escucha junta todos los eventos (sin perder ninguno) y los lee en orden.
type escucha struct {
	mu      sync.Mutex
	l       []evento
	k       int // el próximo que se lee
	cerrado bool
	nuevo   chan struct{}
}

func escuchar(t *testing.T, c *cliente) *escucha {
	t.Helper()
	ctx, cancelar := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", api+"/v1/eventos", nil)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := (&http.Client{Jar: c.http.Jar}).Do(req)
	if err != nil || resp.StatusCode != 200 {
		cancelar()
		t.Fatalf("SSE: %v", err)
	}
	e := &escucha{nuevo: make(chan struct{}, 1)}
	avisar := func() {
		select {
		case e.nuevo <- struct{}{}:
		default:
		}
	}
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "data: ") {
				var ev evento
				if json.Unmarshal([]byte(l[6:]), &ev) == nil {
					e.mu.Lock()
					e.l = append(e.l, ev)
					e.mu.Unlock()
					avisar()
				}
			}
		}
		e.mu.Lock()
		e.cerrado = true
		e.mu.Unlock()
		avisar()
	}()
	t.Cleanup(func() { cancelar(); resp.Body.Close() })
	return e
}

// intentar espera un evento que cumpla ok (false si no llega en el plazo).
func (e *escucha) intentar(plazo time.Duration, ok func(evento) bool) (evento, bool) {
	fin := time.After(plazo)
	for {
		e.mu.Lock()
		for e.k < len(e.l) {
			ev := e.l[e.k]
			e.k++
			if ok(ev) {
				e.mu.Unlock()
				return ev, true
			}
		}
		cerrado := e.cerrado
		e.mu.Unlock()
		if cerrado {
			return evento{}, false
		}
		select {
		case <-e.nuevo:
		case <-fin:
			return evento{}, false
		}
	}
}

func (e *escucha) esperar(t *testing.T, que string, plazo time.Duration, ok func(evento) bool) evento {
	t.Helper()
	ev, llego := e.intentar(plazo, ok)
	if !llego {
		t.Fatalf("no llegó el evento: %s", que)
	}
	return ev
}

// --- Los bichos virtuales ---

func leerVirtualEn(t *testing.T, base string) infoVirtual {
	t.Helper()
	r, err := http.Get(base + "/info")
	if err != nil {
		t.Fatalf("el bicho virtual %s no responde: %v", base, err)
	}
	defer r.Body.Close()
	var i infoVirtual
	json.NewDecoder(r.Body).Decode(&i)
	return i
}

// reclamar suma el bicho virtual de `base` a la cuenta y espera que esté conectado.
func reclamar(t *testing.T, c *cliente, base string) string {
	t.Helper()
	id := leerVirtualEn(t, base).ID
	if s := c.pedir("POST", "/v1/bichos/reclamar-virtual", map[string]string{"id": id}, nil); s != 200 {
		t.Fatalf("sumar %s: %d", id, s)
	}
	esperar(t, "que "+id+" se conecte a la nube", 30*time.Second, func() bool { return leerVirtualEn(t, base).Nube })
	var vb struct{ Conectado bool }
	esperar(t, "que la API vea a "+id, 30*time.Second, func() bool {
		c.pedir("GET", "/v1/bichos/"+id, nil, &vb)
		return vb.Conectado
	})
	return id
}

func postVirtual(t *testing.T, base, ruta string, cuerpo any) {
	t.Helper()
	b, _ := json.Marshal(cuerpo)
	r, err := http.Post(base+ruta, "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
}

// acariciar: un toque en la pantalla del bicho virtual.
func acariciar(t *testing.T, base string) {
	t.Helper()
	postVirtual(t, base, "/tacto", map[string]bool{"apretado": true})
	time.Sleep(150 * time.Millisecond)
	postVirtual(t, base, "/tacto", map[string]bool{"apretado": false})
}

func TestV19DePuntaAPunta(t *testing.T) {
	ana := entrarComo(t, fmt.Sprintf("ana-%d@ejemplo.com", sufijo), "web")
	beto := entrarComo(t, fmt.Sprintf("beto-%d@ejemplo.com", sufijo), "web")
	ana.pedir("PATCH", "/v1/sesion", map[string]string{"nombre": "Ana"}, nil)
	beto.pedir("PATCH", "/v1/sesion", map[string]string{"nombre": "Beto"}, nil)
	ra := reclamar(t, ana, virtual)
	rb := reclamar(t, beto, virtual2)
	t.Cleanup(func() {
		ana.pedir("DELETE", "/v1/bichos/"+ra, nil, nil)
		beto.pedir("DELETE", "/v1/bichos/"+rb, nil, nil)
	})
	ea := escuchar(t, ana)
	eb := escuchar(t, beto)

	t.Run("amigos y toques", func(t *testing.T) {
		var inv struct{ Codigo string }
		if s := ana.pedir("POST", "/v1/amigos/invitacion", nil, &inv); s != 200 || len(inv.Codigo) != 8 {
			t.Fatalf("invitar: %d %q", s, inv.Codigo)
		}
		var am struct{ Nombre string }
		if s := beto.pedir("POST", "/v1/amigos", map[string]string{"codigo": strings.ToLower(inv.Codigo[:4] + "-" + inv.Codigo[4:])}, &am); s != 200 || am.Nombre != "Ana" {
			t.Fatalf("aceptar: %d %+v", s, am)
		}
		if s := beto.pedir("POST", "/v1/amigos", map[string]string{"codigo": inv.Codigo}, nil); s < 400 {
			t.Fatalf("el código es de un solo uso: %d", s)
		}
		var l struct {
			Amigos []struct {
				ID        int64
				Nombre    string
				Conectado bool
			}
		}
		ana.pedir("GET", "/v1/amigos", nil, &l)
		if len(l.Amigos) != 1 || l.Amigos[0].Nombre != "Beto" || !l.Amigos[0].Conectado {
			t.Fatalf("los amigos de Ana: %+v", l)
		}
		// Ana le manda un abrazo desde su robot: le llega al de Beto.
		var res struct {
			Robots   int
			Telegram bool
		}
		if s := ana.pedir("POST", fmt.Sprintf("/v1/amigos/%d/toque", l.Amigos[0].ID), map[string]string{"tipo": "abrazo", "desde": ra}, &res); s != 200 || res.Robots != 1 {
			t.Fatalf("toque: %d %+v", s, res)
		}
		eb.esperar(t, "el toque en la cuenta de Beto", 10*time.Second, func(e evento) bool {
			return e.Tipo == "toque" && e.Datos["tipo"] == "abrazo" && e.Datos["robot"] == rb && e.Datos["devuelto"] == nil
		})
		// Beto acaricia su robot (dentro de los 30 s): vuelve al robot de Ana.
		devuelto := false
		for k := 0; k < 4 && !devuelto; k++ {
			time.Sleep(time.Second)
			acariciar(t, virtual2)
			_, devuelto = ea.intentar(4*time.Second, func(e evento) bool {
				return e.Tipo == "toque" && e.Datos["devuelto"] == true && e.Datos["robot"] == ra
			})
		}
		if !devuelto {
			t.Fatal("el toque no volvió al robot de Ana")
		}
	})

	t.Run("hábitos con racha", func(t *testing.T) {
		cfg := map[string]any{"version": 2, "nombre": "Pipo", "dueno": map[string]any{"nombre": "Ana"},
			"habitos": []any{map[string]any{"nombre": "agua", "icono": "gota"}}}
		var g struct{ Crc uint32 }
		if s := ana.pedir("PUT", "/v1/bichos/"+ra+"/config", cfg, &g); s != 200 {
			t.Fatalf("config con hábitos: %d", s)
		}
		esperar(t, "que la config llegue al robot", 20*time.Second, func() bool { return leerVirtualEn(t, virtual).CrcConfig == g.Crc })
		type habito struct {
			Nombre string
			Racha  int
			Hoy    bool
			Dias   map[string]bool
		}
		marcar := func(hecho bool) habito {
			var r struct {
				Habito  habito
				Enviado bool
			}
			if s := ana.pedir("POST", "/v1/bichos/"+ra+"/habitos/0", map[string]bool{"hecho": hecho}, &r); s != 200 || !r.Enviado {
				t.Fatalf("marcar %v: %d %+v", hecho, s, r)
			}
			return r.Habito
		}
		if h := marcar(true); h.Racha != 1 || !h.Hoy {
			t.Fatalf("hecho: %+v", h)
		}
		var l []habito
		ana.pedir("GET", "/v1/bichos/"+ra+"/habitos", nil, &l)
		if len(l) != 1 || l[0].Nombre != "agua" || l[0].Racha != 1 || len(l[0].Dias) != 1 {
			t.Fatalf("hábitos: %+v", l)
		}
		if h := marcar(false); h.Racha != 0 || h.Hoy {
			t.Fatalf("deshecho: %+v", h)
		}
		if h := marcar(true); h.Racha != 1 {
			t.Fatalf("de nuevo: %+v", h)
		}
		if s := ana.pedir("POST", "/v1/bichos/"+ra+"/habitos/3", map[string]bool{"hecho": true}, nil); s != 404 {
			t.Fatalf("un hábito que no está: %d", s)
		}
	})

	t.Run("presencia", func(t *testing.T) {
		var p struct{ Ok, Nueva, Enviado bool }
		presencia := func(llegaste bool) {
			p = struct{ Ok, Nueva, Enviado bool }{}
			if s := ana.pedir("POST", "/v1/bichos/"+ra+"/presencia", map[string]bool{"llegaste": llegaste}, &p); s != 200 || !p.Ok {
				t.Fatalf("presencia %v: %d %+v", llegaste, s, p)
			}
		}
		presencia(false)
		if p.Nueva && !p.Enviado {
			t.Fatalf("te fuiste: %+v", p)
		}
		presencia(true)
		if !p.Nueva || !p.Enviado {
			t.Fatalf("llegaste: %+v", p)
		}
		presencia(true)
		if p.Nueva {
			t.Fatal("la misma, enseguida, no es nueva")
		}
		if s := ana.pedir("GET", "/v1/bichos/"+ra+"/horarios", nil, nil); s != 200 {
			t.Fatalf("horarios: %d", s)
		}
	})

	t.Run("avisos entrantes", func(t *testing.T) {
		var en struct{ Token string }
		if s := ana.pedir("GET", "/v1/entrantes", nil, &en); s != 200 || !strings.HasPrefix(en.Token, "ent_") {
			t.Fatalf("entrantes: %d %+v", s, en)
		}
		mandar := func(cuerpo string, enc map[string]string) (int, map[string]any) {
			req, _ := http.NewRequest("POST", api+"/v1/entrantes/"+en.Token, strings.NewReader(cuerpo))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range enc {
				req.Header.Set(k, v)
			}
			r, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Body.Close()
			var v map[string]any
			json.NewDecoder(r.Body).Decode(&v)
			return r.StatusCode, v
		}
		firma := func(cuerpo string) string {
			m := hmac.New(sha256.New, []byte(en.Token))
			m.Write([]byte(cuerpo))
			return "sha256=" + hex.EncodeToString(m.Sum(nil))
		}
		pr := `{"action":"opened","pull_request":{"number":12},"repository":{"name":"sintetica"},"sender":{"login":"ana"}}`
		if s, _ := mandar(pr, map[string]string{"X-GitHub-Event": "pull_request", "X-Hub-Signature-256": firma("otro")}); s != 401 {
			t.Fatalf("firma mala: %d", s)
		}
		gh := map[string]string{"X-GitHub-Event": "pull_request", "X-Hub-Signature-256": firma(pr), "X-GitHub-Delivery": fmt.Sprintf("e2e-%d", sufijo)}
		if s, v := mandar(pr, gh); s != 200 || v["robots"] != 1.0 {
			t.Fatalf("PR: %d %v", s, v)
		}
		ea.esperar(t, "el aviso de GitHub", 10*time.Second, func(e evento) bool {
			return e.Tipo == "entrante" && e.Datos["titulo"] == "PR #12" && e.Datos["origen"] == "github" && e.Datos["de"] == "ana"
		})
		if s, v := mandar(pr, gh); s != 202 || v["repetido"] != true {
			t.Fatalf("la misma entrega: %d %v", s, v)
		}
		// Mercado Pago (sin token): una vez por pago.
		pago := fmt.Sprintf(`{"action":"payment.created","type":"payment","data":{"id":"%d"}}`, sufijo%1_000_000_000)
		if s, _ := mandar(pago, nil); s != 200 {
			t.Fatalf("pago: %d", s)
		}
		ea.esperar(t, "el aviso del pago", 10*time.Second, func(e evento) bool {
			return e.Tipo == "entrante" && e.Datos["tipo"] == "pago" && e.Datos["titulo"] == "¡Te pagaron!"
		})
		if s, _ := mandar(pago, nil); s != 202 {
			t.Fatalf("el mismo pago: %d", s)
		}
	})

	t.Run("agenda con un .ics", func(t *testing.T) {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		inicio := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Minute)
		ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//e2e//ES\r\n" +
			"BEGIN:VEVENT\r\nUID:e2e-" + strconv.FormatInt(sufijo, 10) + "\r\nSUMMARY:Reunión e2e\r\nDTSTART:" + inicio.Format("20060102T150405Z") +
			"\r\nDURATION:PT30M\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/calendar")
			w.Write([]byte(ics))
		})}
		go srv.Serve(ln)
		defer srv.Close()
		dir := fmt.Sprintf("http://%s:%d/privado-e2e/basic.ics", hostLocal, ln.Addr().(*net.TCPAddr).Port)
		var v struct {
			Conectada bool
			Proximos  []struct{ Titulo string }
			Error     string
		}
		if s := ana.pedir("PUT", "/v1/agenda", map[string]string{"ics": dir}, &v); s != 200 || !v.Conectada || len(v.Proximos) != 1 || v.Proximos[0].Titulo != "Reunión e2e" {
			t.Fatalf("conectar la agenda (%s): %d %+v", dir, s, v)
		}
		defer ana.pedir("DELETE", "/v1/agenda", nil, nil)
		// En el minuto siguiente avisa (falta menos que el aviso, 10 min) a los robots conectados.
		ev := ea.esperar(t, "el aviso de la agenda", 100*time.Second, func(e evento) bool {
			return e.Tipo == "agenda" && e.Datos["titulo"] == "Reunión e2e" && e.Datos["minutos"] != nil
		})
		if rs, _ := ev.Datos["robots"].([]any); len(rs) != 1 || rs[0] != ra {
			t.Fatalf("el aviso tiene que llegarle al robot de Ana: %v", ev.Datos)
		}
	})

	t.Run("charla con el motor propio", func(t *testing.T) {
		var r struct {
			Origen, Accion, Texto string
			Guardado, Enviado     bool
		}
		if s := ana.pedir("POST", "/v1/bichos/"+ra+"/charla", map[string]string{"texto": "despertame a las 7"}, &r); s != 200 ||
			r.Origen != "motor" || r.Accion != "alarma" || !r.Guardado || !r.Enviado {
			t.Fatalf("charla: %d %+v", s, r)
		}
		var c struct {
			Crc    uint32
			Autor  string
			Config struct {
				Despertador struct {
					Hora   string
					Activo bool
				}
			}
		}
		ana.pedir("GET", "/v1/bichos/"+ra+"/config", nil, &c)
		if c.Autor != "charla" || c.Config.Despertador.Hora != "07:00" || !c.Config.Despertador.Activo {
			t.Fatalf("la alarma en la config: %+v", c)
		}
		esperar(t, "que la alarma llegue al robot", 20*time.Second, func() bool { return leerVirtualEn(t, virtual).CrcConfig == c.Crc })
	})
}

// --- Telegram falso ---

// telegramFalso imita la Bot API (getUpdates con long polling, sendMessage y
// lo demás con ok): el test le pone updates y mira lo que mandó el bot.
type telegramFalso struct {
	mu       sync.Mutex
	updates  []map[string]any
	proximo  int64
	enviados []map[string]any
	hay      chan struct{}
}

func arrancarTelegramFalso(dir string) *telegramFalso {
	f := &telegramFalso{proximo: time.Now().UnixMilli(), hay: make(chan struct{}, 1)}
	ln, err := net.Listen("tcp", dir)
	if err != nil {
		panic("el Telegram falso: " + err.Error())
	}
	go http.Serve(ln, f)
	return f
}

func (f *telegramFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p map[string]any
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	d.Decode(&p)
	responder := func(res any) {
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": res})
	}
	switch path.Base(r.URL.Path) {
	case "getUpdates":
		n, _ := p["offset"].(json.Number)
		offset, _ := n.Int64()
		fin := time.After(20 * time.Second)
		for {
			f.mu.Lock()
			var l []map[string]any
			for _, u := range f.updates {
				if id, _ := u["update_id"].(int64); id >= offset {
					l = append(l, u)
				}
			}
			f.mu.Unlock()
			if len(l) > 0 {
				responder(l)
				return
			}
			select {
			case <-f.hay:
			case <-fin:
				responder([]any{})
				return
			case <-r.Context().Done():
				return
			}
		}
	case "sendMessage":
		f.mu.Lock()
		f.enviados = append(f.enviados, p)
		n := len(f.enviados)
		f.mu.Unlock()
		responder(map[string]any{"message_id": n, "chat": map[string]any{"id": p["chat_id"]}})
	default:
		responder(true)
	}
}

func (f *telegramFalso) encolar(u map[string]any) {
	f.mu.Lock()
	f.proximo++
	u["update_id"] = f.proximo
	f.updates = append(f.updates, u)
	f.mu.Unlock()
	select {
	case f.hay <- struct{}{}:
	default:
	}
}

func (f *telegramFalso) cantidad() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.enviados)
}

// esperarMensaje: el primero que el bot le mandó a ese chat después de `desde`.
func (f *telegramFalso) esperarMensaje(t *testing.T, chat int64, desde int, plazo time.Duration) (map[string]any, int) {
	t.Helper()
	for fin := time.Now().Add(plazo); time.Now().Before(fin); time.Sleep(200 * time.Millisecond) {
		f.mu.Lock()
		for k := desde; k < len(f.enviados); k++ {
			id, _ := f.enviados[k]["chat_id"].(json.Number)
			if n, _ := id.Int64(); n == chat {
				m := f.enviados[k]
				f.mu.Unlock()
				return m, k + 1
			}
		}
		f.mu.Unlock()
	}
	t.Fatalf("el bot no le escribió al chat %d", chat)
	return nil, 0
}

func mensajeTG(chat int64, nombre, texto string) map[string]any {
	return map[string]any{"message": map[string]any{"message_id": 1, "from": map[string]any{"id": chat, "first_name": nombre},
		"chat": map[string]any{"id": chat, "type": "private"}, "text": texto}}
}

func botonTG(chat int64, nombre, datos string) map[string]any {
	return map[string]any{"callback_query": map[string]any{"id": "c1", "from": map[string]any{"id": chat, "first_name": nombre}, "data": datos,
		"message": map[string]any{"message_id": 2, "chat": map[string]any{"id": chat, "type": "private"}}}}
}

type usuarioE2E struct {
	ID, Email, Nombre string
	Admin             bool
}

var reSeisDigitos = regexp.MustCompile(`(\d{3}) (\d{3})`)

// entrarConTelegram: la web pide el link, "el celu" toca Start (y Crear mi
// cuenta, si es nueva) y la web escribe el código de 6 dígitos que mandó el bot.
func entrarConTelegram(t *testing.T, chat int64, nombre string) (*cliente, usuarioE2E) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &cliente{t: t, http: &http.Client{Jar: jar, Timeout: 60 * time.Second}, web: true}
	var p struct{ Token, Enlace string }
	if s := c.pedir("POST", "/v1/sesion/telegram", map[string]string{"cliente": "web"}, &p); s != 201 || !strings.HasSuffix(p.Enlace, "?start=entrar-"+p.Token) {
		t.Fatalf("pedir el link de Telegram: %d %+v", s, p)
	}
	desde := tg.cantidad()
	tg.encolar(mensajeTG(chat, nombre, "/start entrar-"+p.Token))
	m, desde := tg.esperarMensaje(t, chat, desde, 90*time.Second) // la API puede estar esperando para reintentar getUpdates
	if texto, _ := m["text"].(string); strings.Contains(texto, "¿Crear tu cuenta") {
		teclado := m["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)
		tg.encolar(botonTG(chat, nombre, teclado[0].(map[string]any)["callback_data"].(string)))
		m, _ = tg.esperarMensaje(t, chat, desde, 30*time.Second)
	}
	cod := reSeisDigitos.FindStringSubmatch(m["text"].(string))
	if cod == nil {
		t.Fatalf("el bot no mandó el código: %v", m["text"])
	}
	var r struct{ Usuario usuarioE2E }
	if s := c.pedir("POST", "/v1/sesion/telegram/"+p.Token+"/codigo", map[string]string{"codigo": cod[1] + " " + cod[2]}, &r); s != 200 || r.Usuario.ID == "" {
		t.Fatalf("entrar con el código: %d %+v", s, r)
	}
	return c, r.Usuario
}

func TestEntrarConTelegramDePuntaAPunta(t *testing.T) {
	if tg == nil {
		t.Skip("sin Telegram falso: E2E_TELEGRAM=:8097 (y la API con TELEGRAM_TOKEN y TELEGRAM_API_URL)")
	}
	var met struct{ Telegram bool }
	web := &cliente{t: t, http: &http.Client{Timeout: 10 * time.Second}}
	if web.pedir("GET", "/v1/sesion/metodos", nil, &met); !met.Telegram {
		t.Fatal("la API no tiene TELEGRAM_TOKEN")
	}

	// Una cuenta nueva desde Telegram: sin mail.
	chat := 900_000_000 + sufijo%100_000_000
	c, u := entrarConTelegram(t, chat, "Zoe")
	if u.Email != "" || u.Nombre != "Zoe" {
		t.Fatalf("la cuenta de Telegram: %+v", u)
	}
	// Vuelve a entrar con el mismo Telegram: la misma cuenta (sin preguntar).
	if _, u2 := entrarConTelegram(t, chat, "Zoe"); u2.ID != u.ID {
		t.Fatalf("otra cuenta: %s ≠ %s", u2.ID, u.ID)
	}

	// Le suma un mail: el código llega al mail.
	mail := fmt.Sprintf("zoe-%d@ejemplo.com", sufijo)
	desde := time.Now()
	if s := c.pedir("POST", "/v1/sesion/email", map[string]string{"email": mail}, nil); s != 202 {
		t.Fatalf("sumar el mail: %d", s)
	}
	texto := mailDe(t, mail, desde, "sumar este mail")
	m := regexp.MustCompile(`código: ([A-Z0-9]{4}-[A-Z0-9]{4})`).FindStringSubmatch(texto)
	if m == nil {
		t.Fatalf("el mail no trae el código: %s", texto)
	}
	var r struct {
		Usuario usuarioE2E
		Unida   bool
	}
	if s := c.pedir("POST", "/v1/sesion/email/confirmar", map[string]string{"codigo": m[1]}, &r); s != 200 || r.Usuario.ID != u.ID || r.Usuario.Email != mail || r.Unida {
		t.Fatalf("confirmar el mail: %d %+v", s, r)
	}
	if s := c.pedir("POST", "/v1/sesion/email", map[string]string{"email": "otro-" + mail}, nil); s != 409 {
		t.Fatalf("ya tiene mail: %d", s)
	}
	// Entrar por mail con esa dirección es la misma cuenta (no se duplica).
	app := entrarComo(t, mail, "app")
	var yo struct{ Usuario usuarioE2E }
	if app.pedir("GET", "/v1/sesion", nil, &yo); yo.Usuario.ID != u.ID {
		t.Fatalf("por mail entré a otra cuenta: %s ≠ %s", yo.Usuario.ID, u.ID)
	}

	// Admin por ADMIN_TELEGRAM (el compose del CI lo pone con este id).
	if id := os.Getenv("E2E_TELEGRAM_ADMIN"); id != "" {
		n, _ := strconv.ParseInt(id, 10, 64)
		_, ua := entrarConTelegram(t, n, "Nahuel")
		if !ua.Admin {
			t.Fatalf("el de ADMIN_TELEGRAM no es admin: %+v", ua)
		}
	}
}
