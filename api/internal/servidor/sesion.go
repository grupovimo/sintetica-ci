package servidor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/correo"
	"sintetica/shared/formato"
)

const (
	cookieSesion   = "sintetica_sesion"
	duracionWeb    = 30 * 24 * time.Hour
	duracionApp    = 180 * 24 * time.Hour
	duracionEnlace = 15 * time.Minute
	alfabetoCodigo = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
)

type claveCtx int

const claveUsuario claveCtx = 0

func hash(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func aleatorio(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return base64.RawURLEncoding.EncodeToString(b)
}

func codigoCorto() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	for i := range b {
		b[i] = alfabetoCodigo[int(b[i])%len(alfabetoCodigo)]
	}
	return string(b)
}

func normalizarCodigo(c string) string {
	c = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(c, " ", ""), "-", ""))
	return c
}

// usuarioDe devuelve el usuario de la sesión (lo pone conSesion).
func usuarioDe(r *http.Request) almacen.Sesion {
	return r.Context().Value(claveUsuario).(almacen.Sesion)
}

// conSesion exige una sesión: "Authorization: Bearer <token>" (apps) o la
// cookie (web). Con cookie, lo que cambia algo además necesita el
// encabezado X-Sintetica (un sitio ajeno no lo puede mandar: CSRF).
func (s *Servidor) conSesion(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, porCookie := "", false
		if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
			token = strings.TrimPrefix(a, "Bearer ")
		} else if c, err := r.Cookie(cookieSesion); err == nil {
			token, porCookie = c.Value, true
		}
		if token == "" {
			fallar(w, http.StatusUnauthorized, "tenés que entrar con tu email")
			return
		}
		if porCookie && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Sintetica") == "" {
			fallar(w, http.StatusForbidden, "falta el encabezado X-Sintetica")
			return
		}
		ses, err := s.alm.SesionPorToken(r.Context(), hash(token), duracionDe(token, porCookie))
		if errors.Is(err, almacen.ErrNoEncontrado) {
			if porCookie {
				http.SetCookie(w, s.cookie("", -1))
			}
			fallar(w, http.StatusUnauthorized, "la sesión venció: entrá de nuevo")
			return
		}
		if err != nil {
			s.interno(w, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), claveUsuario, ses)))
	}
}

func duracionDe(_ string, porCookie bool) time.Duration {
	if porCookie {
		return duracionWeb
	}
	return duracionApp
}

func (s *Servidor) cookie(valor string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: cookieSesion, Value: valor, Path: "/", HttpOnly: true, Secure: s.op.CookieSegura,
		SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

// POST /v1/sesion/enlace {email, cliente: "web"|"app"}: manda el link mágico.
// Siempre responde lo mismo (no dice si el email tiene cuenta).
func (s *Servidor) pedirEnlace(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Email   string `json:"email"`
		Cliente string `json:"cliente"`
		Volver  string `json:"volver"` // a dónde ir después de entrar en la web (una ruta)
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	dir, err := mail.ParseAddress(strings.TrimSpace(p.Email))
	if err != nil || len(dir.Address) > 254 {
		fallar(w, http.StatusBadRequest, "ese email no parece válido")
		return
	}
	email := strings.ToLower(dir.Address)
	if p.Cliente == "" {
		p.Cliente = "web"
	}
	if p.Cliente != "web" && p.Cliente != "app" {
		fallar(w, http.StatusBadRequest, "cliente es web o app")
		return
	}
	ip := ipDe(r)
	if !s.limites.permitir("ip:"+ip, 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchos links: probá en un rato")
		return
	}
	n, err := s.alm.EnlacesRecientes(r.Context(), email, time.Now().Add(-time.Hour))
	if err != nil {
		s.interno(w, err)
		return
	}
	if n >= 5 {
		fallar(w, http.StatusTooManyRequests, "pediste muchos links para ese email: probá en un rato")
		return
	}
	token, codigo := aleatorio(32), codigoCorto()
	if err := s.alm.CrearEnlace(r.Context(), email, hash(token), hash(codigo), p.Cliente, ip, time.Now().Add(duracionEnlace)); err != nil {
		s.interno(w, err)
		return
	}
	volver := ""
	if strings.HasPrefix(p.Volver, "/") && !strings.HasPrefix(p.Volver, "//") {
		volver = "&volver=" + url.QueryEscape(p.Volver)
	}
	link := fmt.Sprintf("%s/entrar/confirmar?t=%s%s", strings.TrimRight(s.op.WebURL, "/"), token, volver)
	if p.Cliente == "app" {
		link = fmt.Sprintf("%s/entrar/app?t=%s", strings.TrimRight(s.op.WebURL, "/"), token)
	}
	m := mailDeEnlace(email, link, codigo, p.Cliente)
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelar()
		if err := s.correo.Mandar(ctx, m); err != nil {
			s.log.Error("no pude mandar el link", "email", email, "error", err)
		}
	}()
	if s.op.Desarrollo {
		s.log.Info("link mágico (desarrollo)", "email", email, "link", link, "codigo", codigo)
	}
	responder(w, http.StatusAccepted, map[string]string{"mensaje": "Si el email está bien, te llega un link (vale 15 minutos)."})
}

func mailDeEnlace(email, link, codigo, cliente string) correo.Mensaje {
	codigoLindo := codigo[:4] + "-" + codigo[4:]
	donde := "en la web"
	if cliente == "app" {
		donde = "en la app"
	}
	texto := fmt.Sprintf("Hola,\n\nPara entrar a Sintética %s, abrí este link (vale 15 minutos):\n\n%s\n\n"+
		"O escribí este código: %s\n\nSi no fuiste vos, ignorá este mail.\n\nSintética Labs\n", donde, link, codigoLindo)
	htmlTexto := fmt.Sprintf(`<!doctype html><html><body style="margin:0;background:#F5F2EC;font-family:Georgia,serif;color:#1A1714">
<div style="max-width:480px;margin:0 auto;padding:40px 24px">
<p style="font-size:20px;margin:0 0 24px">Hola,</p>
<p style="font-size:16px;line-height:1.5">Para entrar a Sintética %s, tocá el botón (vale 15 minutos).</p>
<p style="margin:32px 0"><a href="%s" style="background:#FF6A33;color:#0B0A09;text-decoration:none;padding:14px 22px;border-radius:999px;font-family:monospace;font-weight:bold;letter-spacing:.06em">ENTRAR</a></p>
<p style="font-size:14px;line-height:1.5;color:#5C5650">O escribí este código:</p>
<p style="font-family:monospace;font-size:28px;letter-spacing:.12em;margin:8px 0 32px">%s</p>
<p style="font-size:13px;color:#8A847C">Si no fuiste vos, ignorá este mail.<br>Sintética Labs</p>
</div></body></html>`, donde, html.EscapeString(link), codigoLindo)
	return correo.Mensaje{Para: email, Asunto: "Tu link para entrar a Sintética", Texto: texto, HTML: htmlTexto}
}

// POST /v1/sesion/canjear {token} o {email, codigo}, más {cliente, descripcion}:
// abre la sesión. A la web le deja la cookie; a la app, el token en la respuesta.
func (s *Servidor) canjear(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Token       string `json:"token"`
		Email       string `json:"email"`
		Codigo      string `json:"codigo"`
		Descripcion string `json:"descripcion"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if !s.limites.permitir("canjear:"+ipDe(r), 30, 10*time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados intentos: probá en un rato")
		return
	}
	var email, cliente string
	var err error
	if p.Token != "" {
		email, cliente, err = s.alm.CanjearToken(r.Context(), hash(p.Token))
	} else {
		email = strings.ToLower(strings.TrimSpace(p.Email))
		cliente, err = s.alm.CanjearCodigo(r.Context(), email, hash(normalizarCodigo(p.Codigo)))
	}
	if errors.Is(err, almacen.ErrEnlaceInvalido) || errors.Is(err, almacen.ErrDemasiadosIntentos) {
		fallar(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	u, err := s.alm.UsuarioPorEmail(r.Context(), email, s.esAdmin(email))
	if err != nil {
		s.interno(w, err)
		return
	}
	token := aleatorio(32)
	dur := duracionWeb
	if cliente == "app" {
		dur = duracionApp
	}
	desc := p.Descripcion
	if len(desc) > 80 {
		desc = desc[:80]
	}
	if desc == "" {
		desc = r.UserAgent()
		if len(desc) > 80 {
			desc = desc[:80]
		}
	}
	if err := s.alm.CrearSesion(r.Context(), hash(token), u.ID, cliente, desc, time.Now().Add(dur)); err != nil {
		s.interno(w, err)
		return
	}
	if cliente == "web" {
		http.SetCookie(w, s.cookie(token, int(dur.Seconds())))
		responder(w, http.StatusOK, map[string]any{"usuario": u})
		return
	}
	responder(w, http.StatusOK, map[string]any{"usuario": u, "token": token})
}

// esAdmin: el email está en ADMIN_EMAILS (o su dominio, escrito "@dominio").
func (s *Servidor) esAdmin(email string) bool {
	_, dominio, _ := strings.Cut(email, "@")
	return slices.Contains(s.op.Admins, email) || slices.Contains(s.op.Admins, "@"+dominio)
}

func (s *Servidor) yo(w http.ResponseWriter, r *http.Request) {
	responder(w, http.StatusOK, map[string]any{"usuario": usuarioDe(r).Usuario, "desarrollo": s.op.Desarrollo})
}

func (s *Servidor) cambiarNombre(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Nombre string `json:"nombre"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	p.Nombre = strings.TrimSpace(p.Nombre)
	if len(p.Nombre) > 60 {
		fallar(w, http.StatusBadRequest, "el nombre es de hasta 60 caracteres")
		return
	}
	u := usuarioDe(r).Usuario
	if err := s.alm.ActualizarNombre(r.Context(), u.ID, p.Nombre); err != nil {
		s.interno(w, err)
		return
	}
	u.Nombre = p.Nombre
	responder(w, http.StatusOK, map[string]any{"usuario": u})
}

func (s *Servidor) salir(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if c, err := r.Cookie(cookieSesion); err == nil && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		token = c.Value
	}
	if err := s.alm.BorrarSesion(r.Context(), hash(token)); err != nil {
		s.interno(w, err)
		return
	}
	http.SetCookie(w, s.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/formato: el schema, las caras base y las animaciones de ejemplo (público).
func (s *Servidor) formato(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	responder(w, http.StatusOK, map[string]any{
		"schema":      jsonCrudo(formato.Schema()),
		"ejemplo":     jsonCrudo(formato.Ejemplo()),
		"caras":       formato.Caras(),
		"animaciones": formato.Animaciones(),
	})
}

type jsonCrudo []byte

func (j jsonCrudo) MarshalJSON() ([]byte, error) { return j, nil }
