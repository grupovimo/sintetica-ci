package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"sintetica/api/internal/correo"
)

type bufSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *bufSeguro) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *bufSeguro) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

// Entrar con un Telegram de ADMIN_TELEGRAM deja la cuenta como admin; después
// le suma un mail con el código que llega al mail (acá, del log de desarrollo).
func TestAdminPorTelegramYSumarMail(t *testing.T) {
	s, _, integ, falso := servidorConTelegram(t, false)
	h := s.Rutas()
	ctx := context.Background()
	chat := chatPrueba()
	s.op.AdminsTelegram = []int64{chat}
	mail := fmt.Sprintf("nahuel-%d@ejemplo.com", chat)

	w := pedirConCookie(t, h, nil, "POST", "/v1/sesion/telegram", `{"cliente":"web"}`)
	var p struct{ Token string }
	json.Unmarshal(w.Body.Bytes(), &p) //nolint:errcheck
	secreto := cookieDe(w, cookieEntrada)
	integ.AtenderTelegram(ctx, startUpdate(chat, "Nahuel", "/start entrar-"+p.Token)) //nolint:errcheck
	teclado := falso.ultimo(t)["reply_markup"].(map[string]any)["inline_keyboard"].([]any)[0].([]any)
	integ.AtenderTelegram(ctx, botonUpdate(chat, teclado[0].(map[string]any)["callback_data"].(string))) //nolint:errcheck
	w = pedirConCookie(t, h, []*http.Cookie{secreto}, "GET", "/v1/sesion/telegram/"+p.Token, "")
	sesion := cookieDe(w, cookieSesion)
	if w.Code != 200 || sesion == nil || !strings.Contains(w.Body.String(), `"admin":true`) {
		t.Fatalf("admin por Telegram: %d %s", w.Code, w.Body)
	}

	// Sin SMTP no se puede sumar un mail.
	r := conX(t, h, sesion, "POST", "/v1/sesion/email", `{"email":"nahuel@ejemplo.com"}`)
	if r.Code != http.StatusServiceUnavailable {
		t.Fatalf("sin SMTP: %d %s", r.Code, r.Body)
	}
	logs := &bufSeguro{}
	s.log = slog.New(slog.NewTextHandler(logs, nil))
	s.op.Desarrollo = true
	s.correo = &correo.Correo{Servidor: "127.0.0.1:1"} // el mail no sale: el código, del log
	if r := conX(t, h, sesion, "POST", "/v1/sesion/email", `{"email":"no es un mail"}`); r.Code != http.StatusBadRequest {
		t.Fatalf("mail inválido: %d", r.Code)
	}
	if r := conX(t, h, sesion, "POST", "/v1/sesion/email", `{"email":"`+strings.ToUpper(mail[:1])+mail[1:]+`"}`); r.Code != http.StatusAccepted {
		t.Fatalf("pedir: %d %s", r.Code, r.Body)
	}
	m := regexp.MustCompile(`codigo=([A-Z0-9]{8})`).FindStringSubmatch(logs.String())
	if m == nil {
		t.Fatalf("sin código en el log: %s", logs.String())
	}
	if r := conX(t, h, sesion, "POST", "/v1/sesion/email/confirmar", `{"codigo":"AAAA-AAAA"}`); r.Code != http.StatusUnauthorized {
		t.Fatalf("código malo: %d", r.Code)
	}
	r = conX(t, h, sesion, "POST", "/v1/sesion/email/confirmar", `{"codigo":"`+strings.ToLower(m[1][:4]+"-"+m[1][4:])+`"}`)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"email":"`+mail+`"`) || !strings.Contains(r.Body.String(), `"unida":false`) {
		t.Fatalf("confirmar: %d %s", r.Code, r.Body)
	}
	if r := conX(t, h, sesion, "POST", "/v1/sesion/email", `{"email":"otro@ejemplo.com"}`); r.Code != http.StatusConflict {
		t.Fatalf("ya tiene mail: %d", r.Code)
	}
	// Entrar por mail con ese mail es la misma cuenta.
	if u, _ := s.alm.UsuarioPorEmail(ctx, mail, false); !u.Admin {
		t.Fatal("la misma cuenta (admin)")
	}
}

// conX: un pedido de la web con sesión (cookie y X-Sintetica).
func conX(t *testing.T, h http.Handler, sesion *http.Cookie, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	r.AddCookie(sesion)
	r.Header.Set("X-Sintetica", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
