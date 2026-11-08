package servidor

// Los avisos entrantes (v1.9, docs/nube.md, "Avisos entrantes"): una
// dirección secreta por usuario (POST /v1/entrantes/{token}) que acepta un
// JSON propio, un webhook de GitHub tal cual (firmado con el mismo token) o
// una notificación de Mercado Pago, y le manda el 0x44 con el tipo nuevo a
// sus robots conectados y el evento "entrante".
//
// El token sale del secreto del servidor y de una semilla al azar del
// usuario (HMAC): en la base hay solo la semilla y el hash del token, y así
// se puede volver a mostrar. Se lo busca por hash y la firma de GitHub se
// compara en tiempo constante.

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// Límites de los entrantes.
const (
	MaxCuerpoEntrante   = 2 << 20 // un webhook de GitHub grande
	EntrantesPorMinuto  = 30      // por usuario
	EntrantesPorIP      = 120     // por minuto, antes de buscar el token
	prefijoTokenEntrada = "ent_"
)

// tokenEntrante: el token de un usuario, de su semilla.
func (s *Servidor) tokenEntrante(semilla []byte) string {
	mac := hmac.New(sha256.New, s.op.Secreto)
	mac.Write([]byte("entrante:"))
	mac.Write(semilla)
	return prefijoTokenEntrada + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))[:32]
}

func (s *Servidor) urlEntrante(token string) string {
	base := strings.TrimRight(s.op.URLPublica, "/")
	return base + "/v1/entrantes/" + token
}

// nuevoEntrante le da (o le cambia) la semilla al usuario.
func (s *Servidor) nuevoEntrante(ctx context.Context, usuario string) (string, error) {
	semilla := make([]byte, 16)
	rand.Read(semilla) //nolint:errcheck
	t := s.tokenEntrante(semilla)
	return t, s.alm.GuardarEntrante(ctx, usuario, semilla, hash(t))
}

func (s *Servidor) responderEntrantes(w http.ResponseWriter, r *http.Request, e almacen.Entrante, token string) {
	res := map[string]any{"mercadoPago": e.MPCifrado != nil, "creado": e.Creado.UTC().Format(time.RFC3339)}
	if token != "" {
		res["token"], res["url"] = token, s.urlEntrante(token)
	} else {
		// Cambió el secreto del servidor: la dirección vieja anda, pero no se
		// puede mostrar. Hay que regenerarla.
		res["token"], res["url"], res["regenerar"] = nil, nil, true
	}
	responder(w, http.StatusOK, res)
}

// GET /v1/entrantes → {url, token, mercadoPago, creado} (la primera vez la crea).
func (s *Servidor) verEntrantes(w http.ResponseWriter, r *http.Request) {
	u := usuarioDe(r).Usuario.ID
	e, err := s.alm.EntranteDe(r.Context(), u)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		if _, err = s.nuevoEntrante(r.Context(), u); err == nil {
			e, err = s.alm.EntranteDe(r.Context(), u)
		}
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	t := s.tokenEntrante(e.Semilla)
	if !hmac.Equal(hash(t), e.TokenHash) {
		t = ""
	}
	s.responderEntrantes(w, r, e, t)
}

// POST /v1/entrantes: otra dirección (la vieja deja de andar).
func (s *Servidor) regenerarEntrantes(w http.ResponseWriter, r *http.Request) {
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("entrantes-nuevo:"+u, 10, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "la cambiaste muchas veces: esperá un rato")
		return
	}
	t, err := s.nuevoEntrante(r.Context(), u)
	if err != nil {
		s.interno(w, err)
		return
	}
	e, err := s.alm.EntranteDe(r.Context(), u)
	if err != nil {
		s.interno(w, err)
		return
	}
	s.responderEntrantes(w, r, e, t)
}

var patronTokenMP = regexp.MustCompile(`^(APP_USR|TEST)-[A-Za-z0-9_-]{10,200}$`)

// PUT /v1/entrantes/mercadopago {accessToken}: para que los pagos digan el monto.
func (s *Servidor) ponerMercadoPago(w http.ResponseWriter, r *http.Request) {
	var p struct {
		AccessToken string `json:"accessToken"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	p.AccessToken = strings.TrimSpace(p.AccessToken)
	if !patronTokenMP.MatchString(p.AccessToken) {
		fallar(w, http.StatusBadRequest, "ese no parece un access token de Mercado Pago (empieza con APP_USR-)")
		return
	}
	u := usuarioDe(r).Usuario.ID
	if _, err := s.alm.EntranteDe(r.Context(), u); errors.Is(err, almacen.ErrNoEncontrado) {
		if _, err := s.nuevoEntrante(r.Context(), u); err != nil {
			s.interno(w, err)
			return
		}
	}
	c, err := s.cifrar([]byte(p.AccessToken))
	if err == nil {
		err = s.alm.GuardarTokenMP(r.Context(), u, c)
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	s.verEntrantes(w, r)
}

// DELETE /v1/entrantes/mercadopago.
func (s *Servidor) sacarMercadoPago(w http.ResponseWriter, r *http.Request) {
	err := s.alm.GuardarTokenMP(r.Context(), usuarioDe(r).Usuario.ID, nil)
	if err != nil && !errors.Is(err, almacen.ErrNoEncontrado) {
		s.interno(w, err)
		return
	}
	s.verEntrantes(w, r)
}

// --- Cifrar el token de Mercado Pago (AES-GCM con una clave del secreto) ---

func (s *Servidor) aead() (cipher.AEAD, error) {
	k := sha256.Sum256(append([]byte("mercadopago:"), s.op.Secreto...))
	b, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

func (s *Servidor) cifrar(texto []byte) ([]byte, error) {
	g, err := s.aead()
	if err != nil {
		return nil, err
	}
	n := make([]byte, g.NonceSize())
	rand.Read(n) //nolint:errcheck
	return g.Seal(n, n, texto, nil), nil
}

func (s *Servidor) descifrar(c []byte) ([]byte, error) {
	g, err := s.aead()
	if err != nil {
		return nil, err
	}
	if len(c) < g.NonceSize() {
		return nil, errors.New("cifrado corto")
	}
	return g.Open(nil, c[:g.NonceSize()], c[g.NonceSize():], nil)
}

// --- Lo que llega ---

// avisoEntrante: lo que se le manda a los robots.
type avisoEntrante struct {
	tipo, titulo, de, origen string
	robot                    string // "" = todos
}

var patronToken = regexp.MustCompile(`^ent_[A-Za-z0-9_-]{32}$`)

// POST /v1/entrantes/{token}: JSON propio, GitHub o Mercado Pago.
func (s *Servidor) recibirEntrante(w http.ResponseWriter, r *http.Request) {
	if !s.limites.permitir("entrante-ip:"+ipDe(r), EntrantesPorIP, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados pedidos")
		return
	}
	token := r.PathValue("token")
	if !patronToken.MatchString(token) {
		fallar(w, http.StatusNotFound, "no existe")
		return
	}
	e, err := s.alm.EntrantePorToken(r.Context(), hash(token))
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no existe")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	cuerpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxCuerpoEntrante))
	if err != nil {
		fallar(w, http.StatusRequestEntityTooLarge, "el aviso es demasiado grande")
		return
	}
	if !s.limites.permitir("entrante:"+e.UsuarioID, EntrantesPorMinuto, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados avisos: esperá un poco")
		return
	}
	var a *avisoEntrante
	switch {
	case r.Header.Get("X-GitHub-Event") != "":
		if !firmaGitHubValida(token, cuerpo, r.Header.Get("X-Hub-Signature-256")) {
			fallar(w, http.StatusUnauthorized, "la firma no coincide: poné el mismo token como secreto del webhook")
			return
		}
		ev := r.Header.Get("X-GitHub-Event")
		if ev == "ping" {
			responder(w, http.StatusOK, map[string]any{"ok": true, "ping": true})
			return
		}
		if a, err = avisoGitHub(ev, cuerpo); err != nil {
			fallar(w, http.StatusBadRequest, "el webhook de GitHub no es JSON válido")
			return
		}
	case esMercadoPago(r, cuerpo):
		a = s.avisoMercadoPago(r.Context(), e, r, cuerpo)
	default:
		if a, err = avisoPropio(cuerpo); err != nil {
			fallar(w, http.StatusBadRequest, "aviso inválido", err.Error())
			return
		}
	}
	if a == nil {
		responder(w, http.StatusAccepted, map[string]any{"ok": true, "ignorado": true})
		return
	}
	n, err := s.entregarEntrante(r.Context(), e.UsuarioID, *a)
	if err != nil {
		var p problema
		if errors.As(err, &p) {
			fallar(w, http.StatusBadRequest, p.msg)
			return
		}
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true, "robots": n})
}

// entregarEntrante: el 0x44 a los robots conectados del usuario (o a uno).
func (s *Servidor) entregarEntrante(ctx context.Context, usuario string, a avisoEntrante) (int, error) {
	marco, err := protocolo.Aviso(a.tipo, a.titulo, a.de)
	if err != nil {
		return 0, problemaf("%s", err.Error())
	}
	l, err := s.alm.BichosDe(ctx, usuario)
	if err != nil {
		return 0, err
	}
	ids := []string{}
	hallado := a.robot == ""
	for _, b := range l {
		if a.robot != "" && !strings.EqualFold(b.ID, a.robot) && normalMCP(b.Apodo) != normalMCP(a.robot) {
			continue
		}
		hallado = true
		if s.nube.Enviar(b.ID, protocolo.CarComando, marco) == nil {
			ids = append(ids, b.ID)
		}
	}
	if !hallado {
		return 0, problemaf("no tenés un robot %q", a.robot)
	}
	s.hub.Publicar(usuario, eventos.Evento{Tipo: "entrante", Datos: map[string]any{
		"tipo": a.tipo, "titulo": protocolo.CortarUTF8(a.titulo, 200), "de": protocolo.CortarUTF8(a.de, 200),
		"origen": a.origen, "robots": ids}})
	return len(ids), nil
}

// avisoPropio: {tipo: pago|mail|github|pedido|otro, titulo, de?, robot?}.
func avisoPropio(cuerpo []byte) (*avisoEntrante, error) {
	var p struct {
		Tipo   string `json:"tipo"`
		Titulo string `json:"titulo"`
		De     string `json:"de"`
		Robot  string `json:"robot"`
	}
	d := json.NewDecoder(bytes.NewReader(cuerpo))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return nil, errors.New("es un JSON {tipo, titulo, de, robot}")
	}
	switch p.Tipo {
	case "pago", "mail", "github", "pedido", "otro":
	default:
		return nil, errors.New("el tipo es pago, mail, github, pedido u otro")
	}
	p.Titulo, p.De, p.Robot = strings.TrimSpace(p.Titulo), strings.TrimSpace(p.De), strings.TrimSpace(p.Robot)
	if p.Titulo == "" || len(p.Titulo) > 200 || !utf8.ValidString(p.Titulo) {
		return nil, errors.New("el título es de 1 a 200 bytes de texto (el robot muestra hasta 24)")
	}
	if len(p.De) > 200 || !utf8.ValidString(p.De) || len(p.Robot) > 60 || !utf8.ValidString(p.Robot) {
		return nil, errors.New("de es de hasta 200 bytes y robot, un id o un nombre")
	}
	return &avisoEntrante{tipo: p.Tipo, titulo: p.Titulo, de: p.De, robot: p.Robot, origen: "json"}, nil
}

// --- GitHub ---

// firmaGitHubValida: X-Hub-Signature-256 = "sha256=" + HMAC-SHA256(token, cuerpo).
func firmaGitHubValida(token string, cuerpo []byte, firma string) bool {
	h, ok := strings.CutPrefix(firma, "sha256=")
	if !ok {
		return false
	}
	recibida, err := hex.DecodeString(h)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write(cuerpo)
	return hmac.Equal(mac.Sum(nil), recibida)
}

// avisoGitHub: PR, issue, review, estrellas y CI fallido; lo demás, nil.
func avisoGitHub(evento string, cuerpo []byte) (*avisoEntrante, error) {
	var g struct {
		Action      string `json:"action"`
		PullRequest *struct {
			Number int  `json:"number"`
			Merged bool `json:"merged"`
		} `json:"pull_request"`
		Issue *struct {
			Number int `json:"number"`
		} `json:"issue"`
		Review *struct {
			State string `json:"state"`
		} `json:"review"`
		WorkflowRun *struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"workflow_run"`
		CheckSuite *struct {
			Conclusion string `json:"conclusion"`
		} `json:"check_suite"`
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(cuerpo, &g); err != nil {
		return nil, err
	}
	a := &avisoEntrante{tipo: "github", de: g.Sender.Login, origen: "github"}
	fallo := func(c string) bool { return c == "failure" || c == "timed_out" }
	switch {
	case evento == "pull_request" && g.PullRequest != nil:
		switch {
		case g.Action == "opened" || g.Action == "reopened" || g.Action == "ready_for_review":
			a.titulo = fmt.Sprintf("PR #%d", g.PullRequest.Number)
		case g.Action == "closed" && g.PullRequest.Merged:
			a.titulo = fmt.Sprintf("PR #%d mergeado", g.PullRequest.Number)
		}
	case evento == "pull_request_review" && g.Action == "submitted" && g.PullRequest != nil && g.Review != nil:
		switch strings.ToLower(g.Review.State) {
		case "approved":
			a.titulo = fmt.Sprintf("PR #%d aprobado", g.PullRequest.Number)
		case "changes_requested":
			a.titulo = fmt.Sprintf("Cambios en PR #%d", g.PullRequest.Number)
		default:
			a.titulo = fmt.Sprintf("Review PR #%d", g.PullRequest.Number)
		}
	case evento == "issues" && g.Issue != nil && (g.Action == "opened" || g.Action == "reopened"):
		a.titulo = fmt.Sprintf("Issue #%d", g.Issue.Number)
	case (evento == "star" && g.Action == "created") || (evento == "watch" && g.Action == "started"):
		a.titulo = "★ " + g.Repository.Name
	case evento == "workflow_run" && g.Action == "completed" && g.WorkflowRun != nil && fallo(g.WorkflowRun.Conclusion):
		a.titulo, a.de = "CI falló", g.Repository.Name
	case evento == "check_suite" && g.Action == "completed" && g.CheckSuite != nil && fallo(g.CheckSuite.Conclusion):
		a.titulo, a.de = "CI falló", g.Repository.Name
	}
	if a.titulo == "" {
		return nil, nil
	}
	return a, nil
}

// --- Mercado Pago ---

// Lo que se lee de una notificación de Mercado Pago (webhook o IPN).
type notificacionMP struct {
	Type   string `json:"type"`
	Topic  string `json:"topic"`
	Action string `json:"action"`
	Data   struct {
		ID json.RawMessage `json:"id"`
	} `json:"data"`
}

func leerMP(r *http.Request, cuerpo []byte) (notificacionMP, string) {
	var n notificacionMP
	json.Unmarshal(cuerpo, &n) //nolint:errcheck
	id := strings.Trim(string(n.Data.ID), `"`)
	q := r.URL.Query()
	if n.Type == "" {
		n.Type = q.Get("type")
	}
	if n.Topic == "" {
		n.Topic = q.Get("topic")
	}
	if id == "" {
		id = q.Get("data.id")
	}
	if id == "" && n.Topic == "payment" {
		id = q.Get("id")
	}
	return n, id
}

func esMercadoPago(r *http.Request, cuerpo []byte) bool {
	n, _ := leerMP(r, cuerpo)
	return n.Type == "payment" || n.Topic == "payment"
}

var patronIDPago = regexp.MustCompile(`^[0-9]{1,20}$`)

// avisoMercadoPago: sin token de Mercado Pago, "¡Te pagaron!" (solo al
// crearse el pago); con token, el monto y quién (solo los aprobados). Una
// vez por pago.
func (s *Servidor) avisoMercadoPago(ctx context.Context, e almacen.Entrante, r *http.Request, cuerpo []byte) *avisoEntrante {
	n, id := leerMP(r, cuerpo)
	if !patronIDPago.MatchString(id) {
		return nil
	}
	a := &avisoEntrante{tipo: "pago", titulo: "¡Te pagaron!", de: "Mercado Pago", origen: "mercadopago"}
	if e.MPCifrado == nil {
		if n.Action != "" && n.Action != "payment.created" {
			return nil
		}
	} else {
		tok, err := s.descifrar(e.MPCifrado)
		if err != nil {
			s.log.Warn("entrantes: no pude descifrar el token de Mercado Pago", "usuario", e.UsuarioID)
			return nil
		}
		p, err := s.pagoMP(ctx, string(tok), id)
		if err != nil {
			s.log.Info("entrantes: no pude leer el pago de Mercado Pago", "error", err)
			return nil
		}
		if p.Status != "approved" {
			return nil
		}
		a.titulo = monto(p.Monto, p.Moneda)
		if nombre := strings.TrimSpace(p.Pagador.Nombre); nombre != "" {
			a.de = nombre
		}
	}
	if !s.limites.permitir("mp:"+e.UsuarioID+":"+id, 1, 48*time.Hour) {
		return nil // ya se avisó ese pago
	}
	return a
}

type pagoMP struct {
	Status  string  `json:"status"`
	Monto   float64 `json:"transaction_amount"`
	Moneda  string  `json:"currency_id"`
	Pagador struct {
		Nombre string `json:"first_name"`
	} `json:"payer"`
}

// pagoMP lee el pago con la API de Mercado Pago (la URL es fija; el id, solo números).
func (s *Servidor) pagoMP(ctx context.Context, token, id string) (pagoMP, error) {
	base := s.op.MercadoPagoURL
	if base == "" {
		base = "https://api.mercadopago.com"
	}
	ctx, cancelar := context.WithTimeout(ctx, 8*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/v1/payments/"+id, nil)
	if err != nil {
		return pagoMP{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := clienteMP.Do(req)
	if err != nil {
		return pagoMP{}, errors.New("Mercado Pago no contestó")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return pagoMP{}, fmt.Errorf("Mercado Pago contestó %d", res.StatusCode)
	}
	var p pagoMP
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&p)
	return p, err
}

var clienteMP = &http.Client{Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// monto: "$12.000", "$1.234,50", "US$ 10".
func monto(v float64, moneda string) string {
	centavos := int64(math.Round(math.Abs(v) * 100))
	entero := strconv.FormatInt(centavos/100, 10)
	var b strings.Builder
	for k, c := range entero {
		if k > 0 && (len(entero)-k)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	s := b.String()
	if c := centavos % 100; c != 0 {
		s += fmt.Sprintf(",%02d", c)
	}
	switch moneda {
	case "ARS", "":
		return "$" + s
	case "USD":
		return "US$ " + s
	}
	return moneda + " " + s
}
