package servidor

// Entrar con Telegram (docs/nube.md, "Cuentas"). La lógica del bot está en
// integraciones/entrar.go; acá, las rutas de la web y las apps.

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/integraciones"
)

// cookieEntrada guarda el secreto del pedido en el navegador que lo hizo:
// el link solo (sin este secreto) no alcanza para entrar desde otro lado.
const cookieEntrada = "sintetica_entrada"

// GET /v1/sesion/metodos (público): cómo se puede entrar en este servidor.
func (s *Servidor) metodosSesion(w http.ResponseWriter, r *http.Request) {
	tg := s.integ != nil && s.integ.TelegramActivo()
	m := map[string]any{"email": s.correo != nil, "telegram": tg}
	if tg {
		m["bot"] = s.integ.Bot()
	}
	responder(w, http.StatusOK, m)
}

// POST /v1/sesion/telegram {cliente: "web"|"app"}: un link para entrar con
// Telegram. A la web le deja el secreto en una cookie corta; a la app se lo
// devuelve (lo manda en X-Sintetica-Secreto).
func (s *Servidor) pedirEntradaTelegram(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Cliente string `json:"cliente"`
	}
	if r.ContentLength != 0 && !leerJSON(w, r, 4096, &p) {
		return
	}
	if p.Cliente == "" {
		p.Cliente = "web"
	}
	if p.Cliente != "web" && p.Cliente != "app" {
		fallar(w, http.StatusBadRequest, "cliente es web o app")
		return
	}
	if s.integ == nil || !s.integ.TelegramActivo() {
		fallar(w, http.StatusServiceUnavailable, integraciones.ErrTelegramApagado.Error())
		return
	}
	ip := ipDe(r)
	if !s.limites.permitir("entrada-tg:"+ip, 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchos links: probá en un rato")
		return
	}
	e, err := s.integ.CrearEntrada(r.Context(), p.Cliente, ip)
	if err != nil {
		s.interno(w, err)
		return
	}
	res := map[string]any{"token": e.Token, "enlace": e.Enlace, "vence": e.Vence, "bot": s.integ.Bot()}
	if p.Cliente == "app" {
		res["secreto"] = e.Secreto
	} else {
		http.SetCookie(w, s.cookieDeEntrada(e.Secreto, int(integraciones.DuracionEntrada.Seconds())))
	}
	responder(w, http.StatusCreated, res)
}

func (s *Servidor) cookieDeEntrada(valor string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: cookieEntrada, Value: valor, Path: "/", HttpOnly: true, Secure: s.op.CookieSegura,
		SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

func secretoDeEntrada(r *http.Request) string {
	if v := r.Header.Get("X-Sintetica-Secreto"); v != "" {
		return v
	}
	if c, err := r.Cookie(cookieEntrada); err == nil {
		return c.Value
	}
	return ""
}

// GET /v1/sesion/telegram/{token}?esperar=25: ¿ya tocó Start? Espera hasta
// `esperar` segundos (30 como mucho). Mientras no, {"estado": "pendiente" |
// "confirmar"}; cuando sí, abre la sesión como POST /v1/sesion/canjear.
func (s *Servidor) esperarEntradaTelegram(w http.ResponseWriter, r *http.Request) {
	secreto := secretoDeEntrada(r)
	if secreto == "" || s.integ == nil {
		fallar(w, http.StatusUnauthorized, almacen.ErrEntradaInvalida.Error())
		return
	}
	if !s.limites.permitir("espera-tg:"+ipDe(r), 600, 10*time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados pedidos: probá en un rato")
		return
	}
	esperar, _ := strconv.Atoi(r.URL.Query().Get("esperar"))
	limite := time.Now().Add(time.Duration(min(max(esperar, 0), 30)) * time.Second)
	for {
		e, err := s.integ.CanjearEntrada(r.Context(), r.PathValue("token"), secreto, "")
		if errors.Is(err, almacen.ErrEntradaInvalida) {
			fallar(w, http.StatusGone, err.Error())
			return
		}
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			s.interno(w, err)
			return
		}
		if e.Estado == "usada" {
			s.sesionDeEntrada(w, r, e)
			return
		}
		if !time.Now().Before(limite) {
			responder(w, http.StatusOK, map[string]any{"estado": e.Estado, "vence": e.Vence})
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// POST /v1/sesion/telegram/{token}/codigo {codigo}: el código de 6 dígitos
// que mandó el bot (con el mismo secreto; 5 intentos, y los topes por IP del
// código por mail).
func (s *Servidor) codigoEntradaTelegram(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Codigo string `json:"codigo"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	secreto := secretoDeEntrada(r)
	if secreto == "" || s.integ == nil {
		fallar(w, http.StatusUnauthorized, almacen.ErrEntradaInvalida.Error())
		return
	}
	if !s.limites.permitir("canjear:"+ipDe(r), 30, 10*time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados intentos: probá en un rato")
		return
	}
	if p.Codigo == "" {
		fallar(w, http.StatusBadRequest, "falta el código")
		return
	}
	e, err := s.integ.CanjearEntrada(r.Context(), r.PathValue("token"), secreto, p.Codigo)
	if errors.Is(err, almacen.ErrEnlaceInvalido) {
		fallar(w, http.StatusUnauthorized, "ese código no es")
		return
	}
	if errors.Is(err, almacen.ErrEntradaInvalida) || errors.Is(err, almacen.ErrDemasiadosIntentos) {
		fallar(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	s.sesionDeEntrada(w, r, e)
}

func (s *Servidor) sesionDeEntrada(w http.ResponseWriter, r *http.Request, e almacen.EntradaTelegram) {
	u, err := s.alm.UsuarioPorID(r.Context(), e.UsuarioID)
	if err != nil {
		s.interno(w, err)
		return
	}
	if e.Cliente == "web" {
		http.SetCookie(w, s.cookieDeEntrada("", -1))
	}
	if h := s.abrirSesion(w, r, u, e.Cliente, ""); h != nil {
		// Para "No fui yo" en el bot.
		if err := s.alm.AnotarSesionEntrada(r.Context(), e.ID, h); err != nil {
			s.log.Warn("no pude anotar la sesión de la entrada", "error", err)
		}
	}
}
