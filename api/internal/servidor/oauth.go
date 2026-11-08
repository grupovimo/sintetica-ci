package servidor

// El servidor de autorización del MCP (docs/nube.md, "MCP"): OAuth 2.1 mínimo
// para que Claude y ChatGPT se conecten solos a la cuenta de cada usuario.
//
//   - Metadatos: /.well-known/oauth-protected-resource (RFC 9728) y
//     /.well-known/oauth-authorization-server (RFC 8414).
//   - Registro dinámico de clientes (RFC 7591): POST /oauth/registrar.
//   - /oauth/autorizar: usa la sesión de la web (la cookie). Por eso el
//     authorization_endpoint que se publica es el de la web (WEB_URL/api/…,
//     que la web le pasa a la API con la cookie); sin sesión, manda a
//     /entrar?volver=… de la web. Muestra el consentimiento ("<cliente> quiere
//     manejar tus robots"), con un token anti-CSRF atado a la sesión.
//   - /oauth/token: authorization_code con PKCE S256 obligatorio (código de un
//     solo uso, 5 min) y refresh_token con rotación (si un refresco se usa dos
//     veces, se revoca toda la concesión).
//   - /oauth/revocar (RFC 7009).
//
// Los tokens son opacos y se guardan hasheados: acceso 1 h, refresco 60 días.

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
)

const (
	duracionCodigoMCP   = 5 * time.Minute
	duracionAccesoMCP   = time.Hour
	duracionRefrescoMCP = 60 * 24 * time.Hour
	alcanceMCP          = "robots"
	maxTokensPersonales = 20
)

// urlPublica es la base pública de la API (el issuer).
func (s *Servidor) urlPublica() string {
	u := strings.TrimRight(s.op.URLPublica, "/")
	if u == "" {
		u = "http://localhost:8080"
	}
	return u
}

func (s *Servidor) urlWeb() string { return strings.TrimRight(s.op.WebURL, "/") }

// recursoMCP es el identificador del recurso (RFC 8707): la URL del MCP.
func (s *Servidor) recursoMCP() string { return s.urlPublica() + "/mcp" }

func (s *Servidor) urlMetadatosRecurso() string {
	return s.urlPublica() + "/.well-known/oauth-protected-resource"
}

// cors: los metadatos, el registro, el token y la revocación no usan cookies;
// los pueden llamar clientes desde un navegador.
func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Protocol-Version")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

// GET /.well-known/oauth-protected-resource[/mcp] (RFC 9728).
func (s *Servidor) metadatosRecurso(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	responder(w, http.StatusOK, map[string]any{
		"resource":                 s.recursoMCP(),
		"authorization_servers":    []string{s.urlPublica()},
		"scopes_supported":         []string{alcanceMCP},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Sintética: tus robots",
		"resource_documentation":   s.urlWeb(),
	})
}

// GET /.well-known/oauth-authorization-server (RFC 8414).
func (s *Servidor) metadatosAutorizacion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	metodos := []string{"none", "client_secret_post", "client_secret_basic"}
	responder(w, http.StatusOK, map[string]any{
		"issuer":                                         s.urlPublica(),
		"authorization_endpoint":                         s.urlWeb() + "/api/oauth/autorizar",
		"token_endpoint":                                 s.urlPublica() + "/oauth/token",
		"registration_endpoint":                          s.urlPublica() + "/oauth/registrar",
		"revocation_endpoint":                            s.urlPublica() + "/oauth/revocar",
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          metodos,
		"revocation_endpoint_auth_methods_supported":     metodos,
		"scopes_supported":                               []string{alcanceMCP},
		"authorization_response_iss_parameter_supported": true,
		"service_documentation":                          s.urlWeb(),
	})
}

// errorOAuth: {"error", "error_description"} (RFC 6749 §5.2).
func errorOAuth(w http.ResponseWriter, estado int, codigo, descripcion string) {
	if estado == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="sintetica"`)
	}
	responder(w, estado, map[string]string{"error": codigo, "error_description": descripcion})
}

// --- Registro dinámico (RFC 7591) ---

var esquemaPropio = regexp.MustCompile(`^[a-z][a-z0-9+.-]{1,40}$`)

// redireccionValida: https a cualquier lado; http solo a loopback (Claude Code,
// las apps de escritorio: RFC 8252); o un esquema propio de una app
// (cursor://, vscode://…), salvo los peligrosos.
func redireccionValida(s string) error {
	if s == "" || len(s) > 500 {
		return errors.New("cada redirect_uri es de 1 a 500 bytes")
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Fragment != "" || strings.Contains(s, "#") {
		return errors.New("redirect_uri inválida (sin fragmento)")
	}
	switch esq := strings.ToLower(u.Scheme); {
	case esq == "https":
		if u.Host == "" || u.User != nil {
			return errors.New("redirect_uri https necesita un host (sin usuario)")
		}
	case esq == "http":
		if !esLoopback(u.Hostname()) {
			return errors.New("http solo para localhost (127.0.0.1 o [::1]); para lo demás, https")
		}
	case esq == "javascript" || esq == "data" || esq == "file" || esq == "vbscript" || esq == "blob" || esq == "about" || esq == "ftp" || esq == "ws" || esq == "wss":
		return errors.New("ese esquema de redirect_uri no se acepta")
	case !esquemaPropio.MatchString(esq):
		return errors.New("esquema de redirect_uri inválido")
	}
	return nil
}

func esLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// coincideRedireccion: exacta, salvo el puerto en loopback (RFC 8252 §7.3).
func coincideRedireccion(registradas []string, pedida string) bool {
	for _, r := range registradas {
		if r == pedida {
			return true
		}
		a, errA := url.Parse(r)
		b, errB := url.Parse(pedida)
		if errA == nil && errB == nil && a.Scheme == "http" && b.Scheme == "http" && esLoopback(a.Hostname()) &&
			strings.EqualFold(a.Hostname(), b.Hostname()) && a.Path == b.Path && a.RawQuery == b.RawQuery {
			return true
		}
	}
	return false
}

// POST /oauth/registrar (RFC 7591): cualquiera puede registrar un cliente; lo
// que da acceso es que el usuario lo permita en el consentimiento.
func (s *Servidor) registrarCliente(w http.ResponseWriter, r *http.Request) {
	if !s.limites.permitir("oauth-registro:"+ipDe(r), 30, time.Hour) {
		errorOAuth(w, http.StatusTooManyRequests, "slow_down", "demasiados registros: probá en un rato")
		return
	}
	var p struct {
		Nombre        string   `json:"client_name"`
		Redirecciones []string `json:"redirect_uris"`
		Metodo        string   `json:"token_endpoint_auth_method"`
		Grants        []string `json:"grant_types"`
		Respuestas    []string `json:"response_types"`
		URI           string   `json:"client_uri"`
		Alcance       string   `json:"scope"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errorOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "el pedido no es JSON válido")
		return
	}
	if len(p.Redirecciones) == 0 || len(p.Redirecciones) > 10 {
		errorOAuth(w, http.StatusBadRequest, "invalid_redirect_uri", "hacen falta de 1 a 10 redirect_uris")
		return
	}
	for _, u := range p.Redirecciones {
		if err := redireccionValida(u); err != nil {
			errorOAuth(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}
	if p.Metodo == "" {
		p.Metodo = "none"
	}
	if p.Metodo != "none" && p.Metodo != "client_secret_post" && p.Metodo != "client_secret_basic" {
		errorOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method: none, client_secret_post o client_secret_basic")
		return
	}
	for _, g := range p.Grants {
		if g != "authorization_code" && g != "refresh_token" {
			errorOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "grant_types: authorization_code y refresh_token")
			return
		}
	}
	for _, t := range p.Respuestas {
		if t != "code" {
			errorOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "response_types: code")
			return
		}
	}
	nombre := strings.TrimSpace(strings.Map(func(c rune) rune {
		if c < 32 || c == 127 {
			return -1
		}
		return c
	}, p.Nombre))
	if nombre == "" {
		nombre = "Una IA"
	}
	nombre = cortarUTF8(nombre, 80)
	if p.URI != "" {
		if u, err := url.Parse(p.URI); err != nil || u.Scheme != "https" || len(p.URI) > 300 {
			p.URI = ""
		}
	}
	c := almacen.ClienteMCP{ID: "sin_cli_" + aleatorio(18), Nombre: nombre, Redirecciones: p.Redirecciones, Metodo: p.Metodo, URI: p.URI, IP: ipDe(r)}
	secreto := ""
	if p.Metodo != "none" {
		secreto = "sin_cs_" + aleatorio(32)
		c.SecretoHash = hash(secreto)
	}
	if err := s.alm.CrearClienteMCP(r.Context(), c); err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("mcp: cliente registrado", "cliente", c.ID, "nombre", nombre, "metodo", p.Metodo, "ip", c.IP)
	resp := map[string]any{
		"client_id":                  c.ID,
		"client_id_issued_at":        time.Now().Unix(),
		"client_name":                nombre,
		"redirect_uris":              c.Redirecciones,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": p.Metodo,
		"scope":                      alcanceMCP,
	}
	if secreto != "" {
		resp["client_secret"] = secreto
		resp["client_secret_expires_at"] = 0
	}
	if c.URI != "" {
		resp["client_uri"] = c.URI
	}
	responder(w, http.StatusCreated, resp)
}

func cortarUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// --- Autorizar (con la sesión de la web) ---

type pedidoAutorizar struct {
	ClienteID, Redireccion, Desafio, Metodo, Estado, Alcance, Recurso, Tipo string
}

func leerPedidoAutorizar(v url.Values) pedidoAutorizar {
	return pedidoAutorizar{
		ClienteID: v.Get("client_id"), Redireccion: v.Get("redirect_uri"), Desafio: v.Get("code_challenge"),
		Metodo: v.Get("code_challenge_method"), Estado: v.Get("state"), Alcance: v.Get("scope"),
		Recurso: v.Get("resource"), Tipo: v.Get("response_type"),
	}
}

func (p pedidoAutorizar) valores() url.Values {
	v := url.Values{}
	v.Set("response_type", p.Tipo)
	v.Set("client_id", p.ClienteID)
	v.Set("redirect_uri", p.Redireccion)
	v.Set("code_challenge", p.Desafio)
	v.Set("code_challenge_method", p.Metodo)
	for k, x := range map[string]string{"state": p.Estado, "scope": p.Alcance, "resource": p.Recurso} {
		if x != "" {
			v.Set(k, x)
		}
	}
	return v
}

var desafioValido = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`) // base64url(sha256) sin relleno

// volverConError manda al cliente de vuelta con un error (RFC 6749 §4.1.2.1).
func (s *Servidor) volverConError(w http.ResponseWriter, r *http.Request, p pedidoAutorizar, codigo, descripcion string) {
	v := url.Values{}
	v.Set("error", codigo)
	v.Set("error_description", descripcion)
	if p.Estado != "" {
		v.Set("state", p.Estado)
	}
	v.Set("iss", s.urlPublica())
	http.Redirect(w, r, conQuery(p.Redireccion, v), http.StatusFound)
}

func conQuery(base string, v url.Values) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + v.Encode()
}

// validarPedido: el cliente y la redirección primero (si están mal, no se
// redirige a ningún lado: se muestra el error); lo demás vuelve al cliente.
func (s *Servidor) validarPedido(w http.ResponseWriter, r *http.Request, p pedidoAutorizar) (almacen.ClienteMCP, bool) {
	c, err := s.alm.ClienteMCPPorID(r.Context(), p.ClienteID)
	if errors.Is(err, almacen.ErrNoEncontrado) || p.ClienteID == "" {
		s.paginaOAuth(w, http.StatusBadRequest, "No conozco esa aplicación", "El pedido viene de una aplicación que no se registró. Volvé a conectarla desde tu IA.", nil)
		return c, false
	}
	if err != nil {
		s.interno(w, err)
		return c, false
	}
	if !coincideRedireccion(c.Redirecciones, p.Redireccion) {
		s.paginaOAuth(w, http.StatusBadRequest, "Dirección de vuelta desconocida", "La aplicación pidió volver a una dirección que no registró.", nil)
		return c, false
	}
	switch {
	case p.Tipo != "code":
		s.volverConError(w, r, p, "unsupported_response_type", "solo response_type=code")
	case p.Metodo != "S256" || !desafioValido.MatchString(p.Desafio):
		s.volverConError(w, r, p, "invalid_request", "hace falta PKCE: code_challenge (S256) y code_challenge_method=S256")
	case p.Alcance != "" && !alcanceValido(p.Alcance):
		s.volverConError(w, r, p, "invalid_scope", "el único alcance es robots")
	case p.Recurso != "" && strings.TrimRight(p.Recurso, "/") != s.recursoMCP() && strings.TrimRight(p.Recurso, "/") != s.urlPublica():
		s.volverConError(w, r, p, "invalid_target", "el recurso es "+s.recursoMCP())
	case len(p.Estado) > 500:
		s.volverConError(w, r, p, "invalid_request", "state demasiado largo")
	default:
		return c, true
	}
	return c, false
}

func alcanceValido(a string) bool {
	for _, x := range strings.Fields(a) {
		if x != alcanceMCP {
			return false
		}
	}
	return true
}

// sesionWeb: la sesión de la cookie (solo la web).
func (s *Servidor) sesionWeb(r *http.Request) (almacen.Sesion, []byte, bool) {
	c, err := r.Cookie(cookieSesion)
	if err != nil || c.Value == "" {
		return almacen.Sesion{}, nil, false
	}
	h := hash(c.Value)
	ses, err := s.alm.SesionPorToken(r.Context(), h, duracionWeb)
	return ses, h, err == nil
}

// anticsrf ata el formulario de consentimiento a la sesión y al pedido.
func (s *Servidor) anticsrf(sesion []byte, p pedidoAutorizar) string {
	m := hmac.New(sha256.New, s.op.Secreto)
	fmt.Fprintf(m, "oauth-consentir\x00%x\x00%s\x00%s\x00%s\x00%s", sesion, p.ClienteID, p.Redireccion, p.Desafio, p.Estado)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Servidor) mandarAEntrar(w http.ResponseWriter, r *http.Request, p pedidoAutorizar) {
	volver := "/api/oauth/autorizar?" + p.valores().Encode()
	http.Redirect(w, r, s.urlWeb()+"/entrar?volver="+url.QueryEscape(volver), http.StatusFound)
}

// GET /oauth/autorizar: el consentimiento (o a entrar, si no hay sesión).
func (s *Servidor) autorizar(w http.ResponseWriter, r *http.Request) {
	p := leerPedidoAutorizar(r.URL.Query())
	c, ok := s.validarPedido(w, r, p)
	if !ok {
		return
	}
	ses, h, ok := s.sesionWeb(r)
	if !ok {
		s.mandarAEntrar(w, r, p)
		return
	}
	destino := p.Redireccion
	if u, err := url.Parse(p.Redireccion); err == nil && u.Host != "" {
		destino = u.Host
	}
	campos := p.valores()
	campos.Set("csrf", s.anticsrf(h, p))
	s.paginaOAuth(w, http.StatusOK, c.Nombre+" quiere manejar tus robots",
		"Va a poder ver tus robots, cambiarles la cara, la configuración y las imágenes. Lo podés desconectar cuando quieras.",
		&consentimiento{campos: campos, email: cmp.Or(ses.Usuario.Email, ses.Usuario.Nombre), destino: destino})
}

// POST /oauth/autorizar: Permitir o Cancelar.
func (s *Servidor) consentir(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil {
		s.paginaOAuth(w, http.StatusBadRequest, "Pedido inválido", "Volvé a conectar desde tu IA.", nil)
		return
	}
	// Un formulario de otro sitio no pasa: el origen tiene que ser la web o la API.
	if o := r.Header.Get("Origin"); o != "" && o != "null" && !mismoOrigen(o, s.urlWeb()) && !mismoOrigen(o, s.urlPublica()) {
		s.paginaOAuth(w, http.StatusForbidden, "Pedido de otro sitio", "Por seguridad, conectá la IA desde su propia pantalla.", nil)
		return
	}
	p := leerPedidoAutorizar(r.PostForm)
	c, ok := s.validarPedido(w, r, p)
	if !ok {
		return
	}
	ses, h, ok := s.sesionWeb(r)
	if !ok {
		s.mandarAEntrar(w, r, p)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(s.anticsrf(h, p))) != 1 {
		s.paginaOAuth(w, http.StatusForbidden, "El pedido venció", "Volvé a conectar desde tu IA.", nil)
		return
	}
	if r.PostForm.Get("decision") != "permitir" {
		s.volverConError(w, r, p, "access_denied", "el usuario no lo permitió")
		return
	}
	codigo := "sin_ac_" + aleatorio(32)
	err := s.alm.CrearCodigoMCP(r.Context(), almacen.CodigoMCP{
		Hash: hash(codigo), UsuarioID: ses.Usuario.ID, ClienteID: c.ID, Redireccion: p.Redireccion, Desafio: p.Desafio,
		Alcance: alcanceMCP, Recurso: p.Recurso, Vence: time.Now().Add(duracionCodigoMCP),
	})
	if err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("mcp: permitido", "usuario", ses.Usuario.ID, "cliente", c.ID, "nombre", c.Nombre)
	v := url.Values{}
	v.Set("code", codigo)
	if p.Estado != "" {
		v.Set("state", p.Estado)
	}
	v.Set("iss", s.urlPublica())
	http.Redirect(w, r, conQuery(p.Redireccion, v), http.StatusFound)
}

func mismoOrigen(origen, base string) bool {
	a, errA := url.Parse(origen)
	b, errB := url.Parse(base)
	return errA == nil && errB == nil && strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

type consentimiento struct {
	campos  url.Values
	email   string
	destino string
}

// paginaOAuth: una página chiquita, sobria (fondo oscuro, naranja), sin
// scripts, que no se puede meter en un iframe (clickjacking).
func (s *Servidor) paginaOAuth(w http.ResponseWriter, estado int, titulo, texto string, c *consentimiento) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(estado)
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>` + html.EscapeString(titulo) + ` · Sintética</title><style>
*{box-sizing:border-box}html,body{margin:0;min-height:100%;background:#16140f;color:#f3eee6;font-family:system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:420px;margin:0 auto;padding:12vh 24px 48px}
.ojos{display:flex;gap:14px;margin-bottom:36px}.ojos i{display:block;width:26px;height:40px;border-radius:13px;background:#ff5a1f;box-shadow:0 0 24px #ff5a1f66}
h1{font-size:24px;line-height:1.25;font-weight:650;margin:0 0 14px;letter-spacing:-.01em}
p{font-size:15px;line-height:1.5;color:#b9b1a5;margin:0 0 10px}
.chico{font-size:13px;color:#857d72}
form{display:flex;gap:12px;margin-top:32px}
button{flex:1;font:inherit;font-size:16px;font-weight:600;padding:14px 18px;border-radius:999px;border:1px solid #3a352d;background:transparent;color:#f3eee6;cursor:pointer}
button.si{background:#ff5a1f;border-color:#ff5a1f;color:#16140f}
button:focus-visible{outline:2px solid #ff5a1f;outline-offset:3px}
</style></head><body><main><div class="ojos"><i></i><i></i></div>`)
	b.WriteString(`<h1>` + html.EscapeString(titulo) + `</h1><p>` + html.EscapeString(texto) + `</p>`)
	if c != nil {
		b.WriteString(`<p class="chico">Entraste como ` + html.EscapeString(c.email) + `. Después vuelve a ` + html.EscapeString(c.destino) + `.</p>`)
		b.WriteString(`<form method="post" action="autorizar">`)
		for k, vs := range c.campos {
			for _, v := range vs {
				b.WriteString(`<input type="hidden" name="` + html.EscapeString(k) + `" value="` + html.EscapeString(v) + `">`)
			}
		}
		b.WriteString(`<button type="submit" name="decision" value="cancelar">Cancelar</button>`)
		b.WriteString(`<button type="submit" name="decision" value="permitir" class="si" autofocus>Permitir</button></form>`)
	}
	b.WriteString(`</main></body></html>`)
	w.Write([]byte(b.String())) //nolint:errcheck
}

// --- Token ---

// clienteDelPedido autentica al cliente: público (solo client_id) o con su
// secreto (client_secret_post o client_secret_basic).
func (s *Servidor) clienteDelPedido(w http.ResponseWriter, r *http.Request) (almacen.ClienteMCP, bool) {
	id, secreto, basic := r.BasicAuth()
	if basic {
		id, _ = url.QueryUnescape(id)
		secreto, _ = url.QueryUnescape(secreto)
	} else {
		id, secreto = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id == "" {
		errorOAuth(w, http.StatusUnauthorized, "invalid_client", "falta client_id")
		return almacen.ClienteMCP{}, false
	}
	c, err := s.alm.ClienteMCPPorID(r.Context(), id)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		errorOAuth(w, http.StatusUnauthorized, "invalid_client", "cliente desconocido")
		return c, false
	}
	if err != nil {
		s.interno(w, err)
		return c, false
	}
	if c.Metodo != "none" && (secreto == "" || subtle.ConstantTimeCompare(hash(secreto), c.SecretoHash) != 1) {
		errorOAuth(w, http.StatusUnauthorized, "invalid_client", "secreto del cliente equivocado")
		return c, false
	}
	return c, true
}

func verificadorValido(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~') {
			return false
		}
	}
	return true
}

func desafioDe(verificador string) string {
	h := sha256.Sum256([]byte(verificador))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// POST /oauth/token.
func (s *Servidor) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Pragma", "no-cache")
	if !s.limites.permitir("oauth-token:"+ipDe(r), 60, time.Minute) {
		errorOAuth(w, http.StatusTooManyRequests, "slow_down", "demasiados pedidos: probá en un rato")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil {
		errorOAuth(w, http.StatusBadRequest, "invalid_request", "el cuerpo va como application/x-www-form-urlencoded")
		return
	}
	c, ok := s.clienteDelPedido(w, r)
	if !ok {
		return
	}
	f := r.PostForm
	if rec := strings.TrimRight(f.Get("resource"), "/"); rec != "" && rec != s.recursoMCP() && rec != s.urlPublica() {
		errorOAuth(w, http.StatusBadRequest, "invalid_target", "el recurso es "+s.recursoMCP())
		return
	}
	acceso, refresco := "sin_at_"+aleatorio(32), "sin_rt_"+aleatorio(32)
	venceA, venceR := time.Now().Add(duracionAccesoMCP), time.Now().Add(duracionRefrescoMCP)
	switch f.Get("grant_type") {
	case "authorization_code":
		verificador := f.Get("code_verifier")
		if !verificadorValido(verificador) {
			errorOAuth(w, http.StatusBadRequest, "invalid_request", "falta code_verifier (PKCE) o es inválido")
			return
		}
		cod, err := s.alm.CanjearCodigoMCP(r.Context(), hash(f.Get("code")))
		if errors.Is(err, almacen.ErrNoEncontrado) || errors.Is(err, almacen.ErrCodigoUsado) {
			errorOAuth(w, http.StatusBadRequest, "invalid_grant", "el código no sirve (vencido o ya usado)")
			return
		}
		if err != nil {
			s.interno(w, err)
			return
		}
		switch {
		case cod.ClienteID != c.ID:
			errorOAuth(w, http.StatusBadRequest, "invalid_grant", "el código es de otro cliente")
			return
		case f.Get("redirect_uri") != cod.Redireccion:
			errorOAuth(w, http.StatusBadRequest, "invalid_grant", "redirect_uri distinta de la del pedido")
			return
		case subtle.ConstantTimeCompare([]byte(desafioDe(verificador)), []byte(cod.Desafio)) != 1:
			errorOAuth(w, http.StatusBadRequest, "invalid_grant", "el code_verifier no corresponde al code_challenge")
			return
		}
		if _, err := s.alm.CrearConcesionMCP(r.Context(), cod, hash(acceso), venceA, hash(refresco), venceR); err != nil {
			s.interno(w, err)
			return
		}
		s.log.Info("mcp: tokens emitidos", "usuario", cod.UsuarioID, "cliente", c.ID)
	case "refresh_token":
		if _, err := s.alm.RefrescarMCP(r.Context(), hash(f.Get("refresh_token")), c.ID, hash(acceso), venceA, hash(refresco), venceR); err != nil {
			if errors.Is(err, almacen.ErrRefrescoInvalido) {
				errorOAuth(w, http.StatusBadRequest, "invalid_grant", "el token de refresco no sirve: conectá de nuevo")
				return
			}
			s.interno(w, err)
			return
		}
	default:
		errorOAuth(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type: authorization_code o refresh_token")
		return
	}
	responder(w, http.StatusOK, map[string]any{
		"access_token": acceso, "token_type": "Bearer", "expires_in": int(duracionAccesoMCP.Seconds()),
		"refresh_token": refresco, "scope": alcanceMCP,
	})
}

// POST /oauth/revocar (RFC 7009): siempre 200.
func (s *Servidor) revocar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil {
		errorOAuth(w, http.StatusBadRequest, "invalid_request", "el cuerpo va como application/x-www-form-urlencoded")
		return
	}
	c, ok := s.clienteDelPedido(w, r)
	if !ok {
		return
	}
	if t := r.PostForm.Get("token"); t != "" {
		if err := s.alm.RevocarTokenMCP(r.Context(), hash(t), c.ID); err != nil {
			s.interno(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// --- Tokens personales y las IAs conectadas (desde la web) ---

// POST /v1/mcp/tokens {nombre}: el token se ve una sola vez.
func (s *Servidor) crearTokenPersonal(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Nombre string `json:"nombre"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	p.Nombre = strings.TrimSpace(p.Nombre)
	if p.Nombre == "" {
		p.Nombre = "Token personal"
	}
	if len(p.Nombre) > 60 {
		fallar(w, http.StatusBadRequest, "el nombre es de hasta 60 bytes")
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("mcp-tokens:"+u, 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "creaste muchos tokens: esperá un rato")
		return
	}
	n, err := s.alm.CantidadTokensPersonales(r.Context(), u)
	if err != nil {
		s.interno(w, err)
		return
	}
	if n >= maxTokensPersonales {
		fallar(w, http.StatusConflict, fmt.Sprintf("tenés %d tokens: borrá alguno antes", n))
		return
	}
	token := "sin_pat_" + aleatorio(32)
	t, err := s.alm.CrearTokenPersonal(r.Context(), u, p.Nombre, hash(token))
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusCreated, map[string]any{"token": token, "id": t.ID, "nombre": t.Nombre, "creado": t.Creado, "url": s.recursoMCP()})
}

// GET /v1/mcp/tokens.
func (s *Servidor) listarTokensPersonales(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.TokensPersonales(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"tokens": l, "url": s.recursoMCP()})
}

// DELETE /v1/mcp/tokens/{id}.
func (s *Servidor) borrarTokenPersonal(w http.ResponseWriter, r *http.Request) {
	err := s.alm.BorrarTokenPersonal(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id"))
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no tenés ese token")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/mcp/clientes: las IAs conectadas.
func (s *Servidor) listarClientesMCP(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.ConcesionesMCP(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"clientes": l, "url": s.recursoMCP()})
}

// DELETE /v1/mcp/clientes/{id}: la desconecta (todos sus tokens dejan de andar).
func (s *Servidor) revocarClienteMCP(w http.ResponseWriter, r *http.Request) {
	err := s.alm.RevocarConcesionMCP(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id"))
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "esa IA no está conectada")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
