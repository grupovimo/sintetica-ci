package integraciones

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// mailFalso: un mail en el servidor falso.
type mailFalso struct {
	id, de, asunto, clase string
	en                    time.Time
	leido                 bool
}

// correoFalso imita las cuentas de Google y Microsoft, la API de Gmail y Graph.
// Google: /google/o/oauth2/v2/auth y /google/token; Gmail: /gmail/...;
// Microsoft: /ms/authorize y /ms/token; Graph: /graph/v1.0/...
type correoFalso struct {
	mu        sync.Mutex
	mails     map[string][]*mailFalso // "gmail" | "outlook"
	refreshG  string                  // el refresh token de Google que vale ("" = revocado)
	refreshM  string                  // el de Microsoft (rota en cada uso)
	accesos   map[string]bool
	n         int
	refrescos map[string]int
	metadatos int // GETs de un mail de Gmail
	secretMal int
	filtroMS  string
}

func nuevoCorreoFalso() *correoFalso {
	return &correoFalso{mails: map[string][]*mailFalso{}, refreshG: "refresco-g-secreto", refreshM: "refresco-m-1",
		accesos: map[string]bool{}, refrescos: map[string]int{}}
}

func (f *correoFalso) acceso() string {
	f.n++
	a := fmt.Sprintf("acceso-%d", f.n)
	f.accesos[a] = true
	return a
}

func (f *correoFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.HasSuffix(r.URL.Path, "/token") {
		r.ParseForm() //nolint:errcheck
		prov, id, sec, vuelta := "gmail", "cliente-g", "secreto-g", "/v1/correo/gmail/vuelta"
		if strings.HasPrefix(r.URL.Path, "/ms/") {
			prov, id, sec, vuelta = "outlook", "cliente-m", "secreto-m", "/v1/correo/outlook/vuelta"
		}
		if r.Form.Get("client_id") != id || r.Form.Get("client_secret") != sec {
			f.secretMal++
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mal := func() {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"invalid_grant"}`) //nolint:errcheck
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code") != "codigo-bueno" || !strings.HasSuffix(r.Form.Get("redirect_uri"), vuelta) {
				mal()
				return
			}
			refresh := f.refreshG
			if prov == "outlook" {
				refresh = f.refreshM
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": f.acceso(), "refresh_token": refresh, "expires_in": 3600}) //nolint:errcheck
		case "refresh_token":
			rt := r.Form.Get("refresh_token")
			if prov == "gmail" {
				if f.refreshG == "" || rt != f.refreshG {
					mal()
					return
				}
				f.refrescos[prov]++
				json.NewEncoder(w).Encode(map[string]any{"access_token": f.acceso(), "expires_in": 3600}) //nolint:errcheck
				return
			}
			if rt != f.refreshM {
				mal()
				return
			}
			f.refrescos[prov]++
			f.refreshM = fmt.Sprintf("refresco-m-%d", f.refrescos[prov]+1)
			json.NewEncoder(w).Encode(map[string]any{"access_token": f.acceso(), "refresh_token": f.refreshM, "expires_in": 3600}) //nolint:errcheck
		}
		return
	}
	if !f.accesos[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.URL.Path == "/gmail/gmail/v1/users/me/profile":
		io.WriteString(w, `{"emailAddress":"ana@gmail.com"}`) //nolint:errcheck
	case r.URL.Path == "/gmail/gmail/v1/users/me/messages":
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "is:unread") || !strings.Contains(q, "in:inbox") || !strings.Contains(q, "category:primary") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var ids []map[string]string
		l := f.mails["gmail"]
		for k := len(l) - 1; k >= 0; k-- { // el más nuevo primero
			if !l[k].leido {
				ids = append(ids, map[string]string{"id": l[k].id})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": ids}) //nolint:errcheck
	case strings.HasPrefix(r.URL.Path, "/gmail/gmail/v1/users/me/messages/"):
		id := strings.TrimPrefix(r.URL.Path, "/gmail/gmail/v1/users/me/messages/")
		for _, m := range f.mails["gmail"] {
			if m.id == id {
				f.metadatos++
				json.NewEncoder(w).Encode(map[string]any{"id": id, "internalDate": strconv.FormatInt(m.en.UnixMilli(), 10), //nolint:errcheck
					"payload": map[string]any{"headers": []map[string]string{{"name": "From", "value": m.de}, {"name": "Subject", "value": m.asunto}}}})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	case r.URL.Path == "/graph/v1.0/me":
		io.WriteString(w, `{"mail":null,"userPrincipalName":"ana@hotmail.com"}`) //nolint:errcheck
	case r.URL.Path == "/graph/v1.0/me/mailFolders/inbox/messages":
		f.filtroMS = r.URL.Query().Get("$filter")
		var v []map[string]any
		l := f.mails["outlook"]
		for k := len(l) - 1; k >= 0; k-- {
			if m := l[k]; !m.leido {
				v = append(v, map[string]any{"id": m.id, "subject": m.asunto, "receivedDateTime": m.en.UTC().Format(time.RFC3339),
					"inferenceClassification": m.clase, "from": map[string]any{"emailAddress": map[string]any{"name": m.de, "address": "x@y.com"}}})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"value": v}) //nolint:errcheck
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *correoFalso) llega(prov string, m *mailFalso) {
	f.mu.Lock()
	f.mails[prov] = append(f.mails[prov], m)
	f.mu.Unlock()
}

func opcionesCorreo(srv string) OpcionesV19 {
	return OpcionesV19{Secreto: []byte("secreto de prueba"), APIURL: "https://api.sintetica.test",
		GoogleID: "cliente-g", GoogleSecreto: "secreto-g", MicrosoftID: "cliente-m", MicrosoftSecreto: "secreto-m",
		GoogleCuentasURL: srv + "/google", GmailURL: srv + "/gmail", MicrosoftCuentasURL: srv + "/ms", GraphURL: srv + "/graph"}
}

func estadoDe(t *testing.T, enlace string) string {
	t.Helper()
	u, err := url.Parse(enlace)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func eventosCorreo(sub *eventos.Suscripcion) []map[string]any {
	var l []map[string]any
	for len(sub.C) > 0 {
		if e := <-sub.C; e.Tipo == "correo" {
			l = append(l, e.Datos.(map[string]any))
		}
	}
	return l
}

func TestCorreoSinClaves(t *testing.T) {
	i := Nuevo(Opciones{}, nil, &robotsFalsos{}, eventos.Nuevo(), sinLog)
	if i.CorreoActivo() {
		t.Fatal("sin claves, apagado")
	}
	if _, err := i.EnlaceCorreo("u", "gmail"); !errors.Is(err, ErrCorreoApagado) {
		t.Fatal(err)
	}
	if _, err := i.EnlaceCorreo("u", "yahoo"); !errors.Is(err, ErrProveedorCorreo) {
		t.Fatal(err)
	}
	e, err := i.EstadoCorreo(context.Background(), "u")
	if err != nil || e["gmail"].(map[string]any)["disponible"] != false || e["outlook"].(map[string]any)["disponible"] != false {
		t.Fatalf("%v %v", e, err)
	}
	// Con las de Google solas, Outlook sigue apagado.
	i = Nuevo(Opciones{V19: OpcionesV19{GoogleID: "a", GoogleSecreto: "b"}}, nil, &robotsFalsos{}, eventos.Nuevo(), sinLog)
	if !i.CorreoActivo() {
		t.Fatal("con Google, prendido")
	}
	if _, err := i.EnlaceCorreo("u", "outlook"); !errors.Is(err, ErrCorreoApagado) {
		t.Fatal(err)
	}
}

func TestCorreoTextos(t *testing.T) {
	for in, quiero := range map[string]string{`"Juan Pérez" <juan@x.com>`: "Juan Pérez", "juan@x.com": "juan@x.com",
		"=?UTF-8?Q?Mar=C3=ADa?= <m@x.com>": "María", "Nombre roto <": "Nombre roto <"} {
		if got := remitente(in); got != quiero {
			t.Errorf("%q: %q", in, got)
		}
	}
	m := limpiarMail(MailNoLeido{De: "Juan", Asunto: "  "})
	if m.Asunto != "(sin asunto)" {
		t.Fatal(m.Asunto)
	}
	largo := textoMail(MailNoLeido{De: "Juan", Asunto: strings.Repeat("ñ", 60)})
	if len(largo) > protocolo.MaxDecir || !strings.HasPrefix(largo, "Mail de Juan: ñ") {
		t.Fatalf("%q", largo)
	}
	p := promptCharla(datosCharla{Nombre: "Pipo", Personalidad: personalidades["equilibrado"], Hora: "10:00 del lunes",
		Correo: "tiene 2 sin leer (los más nuevos: Juan: Reunión)", Expresiones: []string{"contento"}})
	if !strings.Contains(p, "Juan: Reunión") {
		t.Fatalf("al prompt le falta el mail: %s", p)
	}
}

func TestCorreoGmail(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	falso := nuevoCorreoFalso()
	srv := httptest.NewServer(falso)
	defer srv.Close()
	tg := &telegramFalso{token: "tok"}
	srvTG := httptest.NewServer(tg)
	defer srvTG.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	hub := eventos.Nuevo()
	sub := hub.Suscribir(u.ID, "")
	defer hub.Desuscribir(sub)
	i := Nuevo(Opciones{V19: opcionesCorreo(srv.URL), TelegramToken: "tok", TelegramURL: srvTG.URL}, a, robots, hub, sinLog)
	t.Cleanup(func() { a.BorrarCorreo(context.Background(), u.ID, "gmail") }) //nolint:errcheck

	// El enlace.
	enlace, err := i.EnlaceCorreo(u.ID, "gmail")
	if err != nil {
		t.Fatal(err)
	}
	pu, _ := url.Parse(enlace)
	q := pu.Query()
	if pu.Path != "/google/o/oauth2/v2/auth" || q.Get("client_id") != "cliente-g" || q.Get("redirect_uri") != "https://api.sintetica.test/v1/correo/gmail/vuelta" ||
		!strings.Contains(q.Get("scope"), "gmail.readonly") || q.Get("access_type") != "offline" || q.Get("prompt") != "consent" {
		t.Fatalf("enlace: %s", enlace)
	}
	estado := q.Get("state")
	// Un state tocado, el de otro proveedor, uno vencido o un código malo no conectan.
	if _, err := i.VueltaCorreo(ctx, "gmail", estado+"x", "codigo-bueno"); err == nil {
		t.Fatal("state tocado")
	}
	if _, err := i.VueltaCorreo(ctx, "outlook", estado, "codigo-bueno"); err == nil {
		t.Fatal("el state de gmail en outlook")
	}
	if _, err := i.VueltaCorreo(ctx, "gmail", estado, "codigo-malo"); err == nil {
		t.Fatal("código malo")
	}
	i.v19.ahora = func() time.Time { return time.Now().Add(11 * time.Minute) }
	if _, err := i.VueltaCorreo(ctx, "gmail", estado, "codigo-bueno"); err == nil {
		t.Fatal("state vencido")
	}
	i.v19.ahora = time.Now
	if got, err := i.VueltaCorreo(ctx, "gmail", estado, "codigo-bueno"); err != nil || got != u.ID {
		t.Fatalf("vuelta: %q %v", got, err)
	}
	c, err := a.CorreoDe(ctx, u.ID, "gmail")
	if err != nil || c.Direccion != "ana@gmail.com" || c.Revisado != nil || bytes.Contains(c.RefreshCifrado, []byte("refresco")) {
		t.Fatalf("guardada: %+v %v", c, err)
	}

	// La primera vuelta solo pone la marca: el mail viejo no se avisa.
	falso.llega("gmail", &mailFalso{id: "m1", de: `"Banco" <no-responder@banco.com>`, asunto: "Tu resumen", en: time.Now().Add(-time.Hour)})
	i.revisarCorreo(ctx)
	if e := enviadosDe(robots); e != "" {
		t.Fatalf("la primera vuelta no avisa: %s", e)
	}
	if c, _ := a.CorreoDe(ctx, u.ID, "gmail"); c.Revisado == nil {
		t.Fatal("sin marca")
	}
	if l, _ := i.NoLeidosDe(u.ID); len(l) != 1 || l[0].De != "Banco" || l[0].Asunto != "Tu resumen" {
		t.Fatalf("no leídos: %+v", l)
	}
	eventosCorreo(sub) // el de "conectada"

	// Llega uno: un aviso, una vez.
	falso.llega("gmail", &mailFalso{id: "m2", de: "Juan Pérez <juan@x.com>", asunto: "Reunión del jueves", en: time.Now()})
	i.revisarCorreo(ctx)
	i.revisarCorreo(ctx)
	marco, _ := protocolo.Aviso("mail", "Reunión del jueves", "Juan Pérez")
	if e := enviadosDe(robots); e != fmt.Sprintf("%s 2 %x", id, marco) {
		t.Fatalf("aviso: %s", e)
	}
	if falso.metadatos != 2 {
		t.Fatalf("pidió %d mails (quiero 2: cada uno una vez)", falso.metadatos)
	}
	evs := eventosCorreo(sub)
	if len(evs) != 1 || evs[0]["texto"] != "Mail de Juan Pérez: Reunión del jueves" || len(evs[0]["nuevos"].([]MailNoLeido)) != 1 {
		t.Fatalf("eventos: %v", evs)
	}
	// Sin Telegram vinculado no se manda nada; vinculado, sí.
	if len(tg.enviados) != 0 {
		t.Fatal("sin vincular")
	}
	codigo := "codigo-" + azar(4)
	if err := a.CrearCodigoTelegram(ctx, u.ID, hashCodigo(codigo), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.VincularTelegram(ctx, hashCodigo(codigo), chatAzar(), "Ana"); err != nil {
		t.Fatal(err)
	}

	// Llegan dos (y el access token venció): un aviso con el más nuevo y "+1".
	vaciar(robots)
	i.correo.mu.Lock()
	clear(i.correo.tokens)
	i.correo.mu.Unlock()
	falso.llega("gmail", &mailFalso{id: "m3", de: "Ana <ana@x.com>", asunto: "Fotos", en: time.Now().Add(-2 * time.Second)})
	falso.llega("gmail", &mailFalso{id: "m4", de: "Luz <luz@x.com>", asunto: "¿Venís?", en: time.Now().Add(-time.Second)})
	i.revisarCorreo(ctx)
	marco, _ = protocolo.Aviso("mail", "¿Venís?", "Luz +1")
	if e := enviadosDe(robots); e != fmt.Sprintf("%s 2 %x", id, marco) || falso.refrescos["gmail"] != 1 {
		t.Fatalf("dos juntos: %s (refrescos %d)", e, falso.refrescos["gmail"])
	}
	if m := tg.esperar(t, 1); !strings.Contains(m[0]["text"].(string), "Tenés 2 mails nuevos") || !strings.Contains(m[0]["text"].(string), "Luz: ¿Venís?") {
		t.Fatalf("telegram: %v", m)
	}
	// La charla sabe.
	if r := i.resumenCorreo(u.ID); !strings.Contains(r, "tiene 4 sin leer") || !strings.Contains(r, "Luz: ¿Venís?") {
		t.Fatalf("resumen: %q", r)
	}
	// Leído en otro lado: sale de la lista.
	falso.mu.Lock()
	falso.mails["gmail"][0].leido = true
	falso.mu.Unlock()
	i.revisarCorreo(ctx)
	est, err := i.EstadoCorreo(ctx, u.ID)
	g := est["gmail"].(map[string]any)
	if err != nil || g["conectada"] != true || g["direccion"] != "ana@gmail.com" || len(est["noLeidos"].([]MailNoLeido)) != 3 ||
		est["outlook"].(map[string]any)["conectada"] != false {
		t.Fatalf("estado: %v %v", est, err)
	}

	// La API se reinicia (sin memoria): avisa lo que llegó después de la
	// marca, no lo de antes.
	vaciar(robots)
	i2 := Nuevo(Opciones{V19: opcionesCorreo(srv.URL)}, a, robots, hub, sinLog)
	falso.llega("gmail", &mailFalso{id: "m5", de: "Sol <sol@x.com>", asunto: "Hola", en: time.Now().Add(time.Minute)})
	i2.revisarCorreo(ctx)
	marco, _ = protocolo.Aviso("mail", "Hola", "Sol")
	if e := enviadosDe(robots); e != fmt.Sprintf("%s 2 %x", id, marco) {
		t.Fatalf("después de reiniciar: %s", e)
	}

	// Lo revocaron en Google: se olvida la cuenta.
	falso.mu.Lock()
	falso.refreshG = ""
	falso.mu.Unlock()
	i2.correo.mu.Lock()
	clear(i2.correo.tokens)
	i2.correo.mu.Unlock()
	i2.revisarCorreo(ctx)
	if _, err := a.CorreoDe(ctx, u.ID, "gmail"); !errors.Is(err, almacen.ErrNoEncontrado) {
		t.Fatalf("revocada: %v", err)
	}
	if l, _ := i2.NoLeidosDe(u.ID); len(l) != 0 {
		t.Fatal("se olvida lo que sabía")
	}
	if falso.secretMal > 0 {
		t.Fatal("mandó mal el client secret")
	}
}

func TestCorreoOutlook(t *testing.T) {
	a := abrirBase(t)
	ctx := context.Background()
	falso := nuevoCorreoFalso()
	srv := httptest.NewServer(falso)
	defer srv.Close()
	u, id := usuarioConBicho(t, a)
	robots := &robotsFalsos{duenos: map[string]string{id: u.ID}, estados: map[string]protocolo.Estado{id: {Nombre: "Pipo"}}}
	i := Nuevo(Opciones{V19: opcionesCorreo(srv.URL)}, a, robots, eventos.Nuevo(), sinLog)
	t.Cleanup(func() { a.BorrarCorreo(context.Background(), u.ID, "outlook") }) //nolint:errcheck

	enlace, _ := i.EnlaceCorreo(u.ID, "outlook")
	pu, _ := url.Parse(enlace)
	if pu.Path != "/ms/authorize" || !strings.Contains(pu.Query().Get("scope"), "offline_access Mail.Read") {
		t.Fatalf("enlace: %s", enlace)
	}
	if _, err := i.VueltaCorreo(ctx, "outlook", estadoDe(t, enlace), "codigo-bueno"); err != nil {
		t.Fatal(err)
	}
	if c, err := a.CorreoDe(ctx, u.ID, "outlook"); err != nil || c.Direccion != "ana@hotmail.com" {
		t.Fatalf("%+v %v", c, err)
	}
	i.revisarCorreo(ctx) // la marca
	if !strings.HasPrefix(falso.filtroMS, "receivedDateTime ge ") || !strings.Contains(falso.filtroMS, "isRead eq false") {
		t.Fatalf("filtro: %q", falso.filtroMS)
	}
	// Uno de "Otros" no se avisa; uno destacado, sí. Microsoft rota el refresh token.
	falso.llega("outlook", &mailFalso{id: "o1", de: "Ofertas", asunto: "50% off", clase: "other", en: time.Now()})
	falso.llega("outlook", &mailFalso{id: "o2", de: "Jefa", asunto: "Urgente", clase: "focused", en: time.Now()})
	i.correo.mu.Lock()
	clear(i.correo.tokens)
	i.correo.mu.Unlock()
	i.revisarCorreo(ctx)
	marco, _ := protocolo.Aviso("mail", "Urgente", "Jefa")
	if e := enviadosDe(robots); e != fmt.Sprintf("%s 2 %x", id, marco) {
		t.Fatalf("aviso: %s", e)
	}
	// El refresh token nuevo quedó guardado: la vuelta siguiente se renueva con ese.
	i.correo.mu.Lock()
	clear(i.correo.tokens)
	i.correo.mu.Unlock()
	i.revisarCorreo(ctx)
	if falso.refrescos["outlook"] != 2 {
		t.Fatalf("refrescos: %d", falso.refrescos["outlook"])
	}
	if _, err := a.CorreoDe(ctx, u.ID, "outlook"); err != nil {
		t.Fatal("sigue conectada", err)
	}
	if err := i.DesconectarCorreo(ctx, u.ID, "outlook"); err != nil {
		t.Fatal(err)
	}
	if e, _ := i.EstadoCorreo(ctx, u.ID); e["outlook"].(map[string]any)["conectada"] != false || len(e["noLeidos"].([]MailNoLeido)) != 0 {
		t.Fatalf("desconectada: %v", e)
	}
	if err := i.DesconectarCorreo(ctx, u.ID, "yahoo"); !errors.Is(err, ErrProveedorCorreo) {
		t.Fatal(err)
	}
}
