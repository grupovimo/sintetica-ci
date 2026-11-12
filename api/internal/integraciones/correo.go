package integraciones

// El mail: Gmail y Outlook (Hotmail) con OAuth (GOOGLE_CLIENT_ID y
// GOOGLE_CLIENT_SECRET, MICROSOFT_CLIENT_ID y MICROSOFT_CLIENT_SECRET; sin las
// de un proveedor, ese queda apagado). Cada 2 minutos mira los no leídos
// importantes de la bandeja de entrada de cada cuenta conectada y, si llegó
// algo, se lo avisa al dueño: el 0x44 de tipo "mail" a sus robots conectados
// (el mismo aviso que los entrantes), el evento SSE "correo" y, con Telegram
// vinculado, un mensaje. El refresh token se guarda cifrado; el access token
// y los no leídos (quién y el asunto) viven en memoria: de los mails no se
// guarda nada en la base. Ver docs/nube.md, "Mail".

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/cifra"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// El mail.
const (
	CadaCorreo = 2 * time.Minute
	// MaxNoLeidos: cuántos no leídos se recuerdan por cuenta.
	MaxNoLeidos = 20
	// solape: cuánto para atrás de la marca se acepta un mail como nuevo (lo
	// que tarda un proveedor en mostrar uno que ya tenía hora).
	solapeCorreo = 10 * time.Minute
	// ventanaCorreo: los no leídos más viejos que esto no cuentan.
	ventanaCorreo = 14 * 24 * time.Hour
)

// Proveedores es el orden en que se muestran.
var Proveedores = []string{"gmail", "outlook"}

// ErrCorreoApagado: no hay claves de ese proveedor en este servidor.
var ErrCorreoApagado = errors.New("ese mail no está prendido en este servidor")

// ErrSinCorreo: el usuario no conectó esa cuenta (o hay que volver a conectarla).
var ErrSinCorreo = errors.New("no conectaste esa cuenta de mail")

// ErrProveedorCorreo: ni gmail ni outlook.
var ErrProveedorCorreo = errors.New("el proveedor es gmail u outlook")

// MailNoLeido: lo que se recuerda de un mail (en memoria).
type MailNoLeido struct {
	ID        string    `json:"-"`
	De        string    `json:"de"`
	Asunto    string    `json:"asunto"`
	En        time.Time `json:"en"`
	Proveedor string    `json:"proveedor"`
}

// bandeja: lo que se sabe de una cuenta desde que arrancó la API.
type bandeja struct {
	lista []MailNoLeido // los no leídos, el más nuevo primero
}

type estadoCorreo struct {
	cifra    *cifra.Cifrador
	mu       sync.Mutex
	tokens   map[string]tokenAcceso // usuario|proveedor → access token
	bandejas map[string]*bandeja    // usuario|proveedor
}

func claveCorreo(usuario, proveedor string) string { return usuario + "|" + proveedor }

func (i *Integraciones) iniciarCorreo(sec []byte) {
	i.correo = estadoCorreo{cifra: cifra.Nuevo(sec, "correo"), tokens: map[string]tokenAcceso{}, bandejas: map[string]*bandeja{}}
}

// proveedorCorreo: lo que cambia entre Google y Microsoft.
type proveedorCorreo struct {
	nombre             string
	id, secreto        string
	autorizar, token   string
	api                string
	alcances           string
	extra              url.Values
	nombreParaUsuarios string
}

func baseCorreo(u, porDefecto string) string {
	if u != "" {
		return strings.TrimRight(u, "/")
	}
	return porDefecto
}

func (i *Integraciones) proveedorCorreo(p string) (proveedorCorreo, error) {
	o := i.op.V19
	switch p {
	case "gmail":
		autorizar, token := "https://accounts.google.com/o/oauth2/v2/auth", "https://oauth2.googleapis.com/token"
		if o.GoogleCuentasURL != "" {
			c := strings.TrimRight(o.GoogleCuentasURL, "/")
			autorizar, token = c+"/o/oauth2/v2/auth", c+"/token"
		}
		return proveedorCorreo{nombre: p, id: o.GoogleID, secreto: o.GoogleSecreto, autorizar: autorizar, token: token,
			api:      baseCorreo(o.GmailURL, "https://gmail.googleapis.com"),
			alcances: "openid email https://www.googleapis.com/auth/gmail.readonly",
			extra:    url.Values{"access_type": {"offline"}, "prompt": {"consent"}}, nombreParaUsuarios: "Gmail"}, nil
	case "outlook":
		c := baseCorreo(o.MicrosoftCuentasURL, "https://login.microsoftonline.com/common/oauth2/v2.0")
		return proveedorCorreo{nombre: p, id: o.MicrosoftID, secreto: o.MicrosoftSecreto, autorizar: c + "/authorize", token: c + "/token",
			api:      baseCorreo(o.GraphURL, "https://graph.microsoft.com"),
			alcances: "offline_access Mail.Read User.Read", extra: url.Values{"response_mode": {"query"}}, nombreParaUsuarios: "Outlook"}, nil
	}
	return proveedorCorreo{}, ErrProveedorCorreo
}

func (p proveedorCorreo) activo() bool { return p.id != "" && p.secreto != "" }

// CorreoActivo: hay claves de algún proveedor.
func (i *Integraciones) CorreoActivo() bool {
	for _, n := range Proveedores {
		if p, _ := i.proveedorCorreo(n); p.activo() {
			return true
		}
	}
	return false
}

func (i *Integraciones) vueltaCorreo(p string) string {
	return strings.TrimRight(i.op.V19.APIURL, "/") + "/v1/correo/" + p + "/vuelta"
}

var clienteCorreo = &http.Client{Timeout: 15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// --- OAuth ---

func (i *Integraciones) firmaEstadoCorreo(s string) string {
	m := hmac.New(sha256.New, append([]byte("correo-estado/"), i.op.V19.Secreto...))
	m.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// EnlaceCorreo: a dónde mandar al usuario para que autorice (el state lleva
// su id y el proveedor, vence en 10 min y va firmado).
func (i *Integraciones) EnlaceCorreo(usuario, proveedor string) (string, error) {
	p, err := i.proveedorCorreo(proveedor)
	if err != nil {
		return "", err
	}
	if !p.activo() {
		return "", ErrCorreoApagado
	}
	cuerpo := usuario + "|" + proveedor + "|" + strconv.FormatInt(i.ahora().Add(venceEstado).Unix(), 10)
	estado := base64.RawURLEncoding.EncodeToString([]byte(cuerpo)) + "." + i.firmaEstadoCorreo(cuerpo)
	q := url.Values{"client_id": {p.id}, "response_type": {"code"}, "redirect_uri": {i.vueltaCorreo(proveedor)},
		"scope": {p.alcances}, "state": {estado}}
	for k, v := range p.extra {
		q[k] = v
	}
	return p.autorizar + "?" + q.Encode(), nil
}

// usuarioDelEstadoCorreo valida el state que volvió (y que sea de ese proveedor).
func (i *Integraciones) usuarioDelEstadoCorreo(estado, proveedor string) (string, bool) {
	c, f, ok := strings.Cut(estado, ".")
	if !ok {
		return "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || !hmac.Equal([]byte(i.firmaEstadoCorreo(string(b))), []byte(f)) {
		return "", false
	}
	partes := strings.Split(string(b), "|")
	if len(partes) != 3 || partes[1] != proveedor {
		return "", false
	}
	vence, err := strconv.ParseInt(partes[2], 10, 64)
	if err != nil || i.ahora().Unix() > vence {
		return "", false
	}
	return partes[0], true
}

func (i *Integraciones) pedirTokenCorreo(ctx context.Context, p proveedorCorreo, form url.Values) (respuestaToken, error) {
	form.Set("client_id", p.id)
	form.Set("client_secret", p.secreto)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.token, strings.NewReader(form.Encode()))
	if err != nil {
		return respuestaToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := clienteCorreo.Do(req)
	if err != nil {
		return respuestaToken{}, sinURL(err)
	}
	defer r.Body.Close()
	var t respuestaToken
	json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&t) //nolint:errcheck
	if r.StatusCode == http.StatusBadRequest && t.Error == "invalid_grant" {
		return t, ErrSinCorreo
	}
	if r.StatusCode != http.StatusOK || t.AccessToken == "" {
		return t, fmt.Errorf("%s respondió %d", p.nombreParaUsuarios, r.StatusCode)
	}
	return t, nil
}

// VueltaCorreo canjea el código y guarda la cuenta. Devuelve el usuario.
func (i *Integraciones) VueltaCorreo(ctx context.Context, proveedor, estado, codigo string) (string, error) {
	p, err := i.proveedorCorreo(proveedor)
	if err != nil {
		return "", err
	}
	if !p.activo() {
		return "", ErrCorreoApagado
	}
	usuario, ok := i.usuarioDelEstadoCorreo(estado, proveedor)
	if !ok || codigo == "" {
		return "", errors.New("el link venció o no es válido: probá conectar de nuevo")
	}
	t, err := i.pedirTokenCorreo(ctx, p, url.Values{"grant_type": {"authorization_code"}, "code": {codigo}, "redirect_uri": {i.vueltaCorreo(proveedor)}})
	if err != nil {
		return usuario, fmt.Errorf("%s no aceptó el código: probá de nuevo", p.nombreParaUsuarios)
	}
	if t.RefreshToken == "" {
		return usuario, fmt.Errorf("%s no devolvió un refresh token", p.nombreParaUsuarios)
	}
	dir, err := i.direccionCorreo(ctx, p, t.AccessToken)
	if err != nil {
		i.log.Info("correo: no pude leer la dirección", "proveedor", proveedor, "error", err)
	}
	if err := i.alm.GuardarCorreo(ctx, usuario, proveedor, i.correo.cifra.Cifrar(t.RefreshToken), protocolo.CortarUTF8(dir, 200)); err != nil {
		return usuario, err
	}
	k := claveCorreo(usuario, proveedor)
	i.correo.mu.Lock()
	i.correo.tokens[k] = tokenAcceso{token: t.AccessToken, vence: i.ahora().Add(time.Duration(t.ExpiresIn-60) * time.Second)}
	delete(i.correo.bandejas, k)
	i.correo.mu.Unlock()
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "correo", Datos: map[string]any{"proveedor": proveedor, "conectada": true, "direccion": dir}})
	return usuario, nil
}

// direccionCorreo: la dirección de la cuenta (para mostrarla).
func (i *Integraciones) direccionCorreo(ctx context.Context, p proveedorCorreo, token string) (string, error) {
	if p.nombre == "gmail" {
		var r struct {
			EmailAddress string `json:"emailAddress"`
		}
		err := i.getCorreo(ctx, token, p.api+"/gmail/v1/users/me/profile", &r)
		return r.EmailAddress, err
	}
	var r struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	err := i.getCorreo(ctx, token, p.api+"/v1.0/me?$select=mail,userPrincipalName", &r)
	if r.Mail == "" {
		r.Mail = r.UserPrincipalName
	}
	return r.Mail, err
}

// DesconectarCorreo olvida la cuenta.
func (i *Integraciones) DesconectarCorreo(ctx context.Context, usuario, proveedor string) error {
	if _, err := i.proveedorCorreo(proveedor); err != nil {
		return err
	}
	k := claveCorreo(usuario, proveedor)
	i.correo.mu.Lock()
	delete(i.correo.tokens, k)
	delete(i.correo.bandejas, k)
	i.correo.mu.Unlock()
	if err := i.alm.BorrarCorreo(ctx, usuario, proveedor); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "correo", Datos: map[string]any{"proveedor": proveedor, "conectada": false}})
	return nil
}

// tokenCorreo: el access token de la cuenta (lo renueva con el refresh token).
func (i *Integraciones) tokenCorreo(ctx context.Context, p proveedorCorreo, c almacen.CuentaCorreo) (string, error) {
	k := claveCorreo(c.UsuarioID, c.Proveedor)
	i.correo.mu.Lock()
	t, ok := i.correo.tokens[k]
	i.correo.mu.Unlock()
	if ok && i.ahora().Before(t.vence) {
		return t.token, nil
	}
	refresh, err := i.correo.cifra.Descifrar(c.RefreshCifrado)
	if err != nil {
		return "", ErrSinCorreo
	}
	r, err := i.pedirTokenCorreo(ctx, p, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if errors.Is(err, ErrSinCorreo) {
		// Lo revocaron (o cambió la clave): se olvida.
		i.log.Info("correo: el refresh token ya no vale", "usuario", c.UsuarioID, "proveedor", c.Proveedor)
		i.DesconectarCorreo(ctx, c.UsuarioID, c.Proveedor) //nolint:errcheck
		return "", ErrSinCorreo
	}
	if err != nil {
		return "", err
	}
	if r.RefreshToken != "" && r.RefreshToken != refresh {
		i.alm.CambiarRefreshCorreo(ctx, c.UsuarioID, c.Proveedor, i.correo.cifra.Cifrar(r.RefreshToken)) //nolint:errcheck
	}
	i.correo.mu.Lock()
	i.correo.tokens[k] = tokenAcceso{token: r.AccessToken, vence: i.ahora().Add(time.Duration(r.ExpiresIn-60) * time.Second)}
	i.correo.mu.Unlock()
	return r.AccessToken, nil
}

func (i *Integraciones) getCorreo(ctx context.Context, token, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := clienteCorreo.Do(req)
	if err != nil {
		return sinURL(err)
	}
	defer r.Body.Close()
	switch {
	case r.StatusCode == http.StatusUnauthorized:
		return ErrSinCorreo
	case r.StatusCode != http.StatusOK:
		return fmt.Errorf("respondió %d", r.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// --- Leer los no leídos ---

// noLeidos: los no leídos importantes de la bandeja de entrada (hasta
// MaxNoLeidos, de las últimas dos semanas), el más nuevo primero. `antes`
// son los que ya se conocían (Gmail no vuelve a pedir esos).
func (i *Integraciones) noLeidos(ctx context.Context, p proveedorCorreo, token string, antes []MailNoLeido) ([]MailNoLeido, error) {
	desde := i.ahora().Add(-ventanaCorreo)
	var l []MailNoLeido
	if p.nombre == "gmail" {
		q := url.Values{"q": {"is:unread in:inbox category:primary after:" + strconv.FormatInt(desde.Unix(), 10)},
			"maxResults": {strconv.Itoa(MaxNoLeidos)}}
		var r struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		}
		if err := i.getCorreo(ctx, token, p.api+"/gmail/v1/users/me/messages?"+q.Encode(), &r); err != nil {
			return nil, err
		}
		conocidos := map[string]MailNoLeido{}
		for _, m := range antes {
			conocidos[m.ID] = m
		}
		for _, m := range r.Messages {
			if len(l) >= MaxNoLeidos {
				break
			}
			if c, ok := conocidos[m.ID]; ok {
				l = append(l, c)
				continue
			}
			var d struct {
				InternalDate string `json:"internalDate"`
				Payload      struct {
					Headers []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"headers"`
				} `json:"payload"`
			}
			mq := url.Values{"format": {"metadata"}, "metadataHeaders": {"From", "Subject"}}
			if err := i.getCorreo(ctx, token, p.api+"/gmail/v1/users/me/messages/"+url.PathEscape(m.ID)+"?"+mq.Encode(), &d); err != nil {
				return nil, err
			}
			nm := MailNoLeido{ID: m.ID, Proveedor: p.nombre}
			if ms, err := strconv.ParseInt(d.InternalDate, 10, 64); err == nil {
				nm.En = time.UnixMilli(ms).UTC()
			}
			for _, h := range d.Payload.Headers {
				switch strings.ToLower(h.Name) {
				case "from":
					nm.De = remitente(h.Value)
				case "subject":
					nm.Asunto = h.Value
				}
			}
			l = append(l, limpiarMail(nm))
		}
	} else {
		// Graph pide que lo de $orderby vaya primero en $filter. Los "$" van
		// tal cual y los espacios como %20.
		var q []string
		for _, kv := range [][2]string{
			{"$filter", "receivedDateTime ge " + desde.UTC().Format("2006-01-02T15:04:05Z") + " and isRead eq false"},
			{"$orderby", "receivedDateTime desc"},
			{"$select", "from,subject,receivedDateTime,inferenceClassification"},
			{"$top", strconv.Itoa(MaxNoLeidos)},
		} {
			q = append(q, kv[0]+"="+strings.ReplaceAll(url.QueryEscape(kv[1]), "+", "%20"))
		}
		var r struct {
			Value []struct {
				ID                      string    `json:"id"`
				Subject                 string    `json:"subject"`
				ReceivedDateTime        time.Time `json:"receivedDateTime"`
				InferenceClassification string    `json:"inferenceClassification"`
				From                    struct {
					EmailAddress struct {
						Name    string `json:"name"`
						Address string `json:"address"`
					} `json:"emailAddress"`
				} `json:"from"`
			} `json:"value"`
		}
		if err := i.getCorreo(ctx, token, p.api+"/v1.0/me/mailFolders/inbox/messages?"+strings.Join(q, "&"), &r); err != nil {
			return nil, err
		}
		for _, m := range r.Value {
			if strings.EqualFold(m.InferenceClassification, "other") {
				continue // la bandeja "Otros" de Outlook: no es importante
			}
			de := m.From.EmailAddress.Name
			if de == "" {
				de = m.From.EmailAddress.Address
			}
			l = append(l, limpiarMail(MailNoLeido{ID: m.ID, De: de, Asunto: m.Subject, En: m.ReceivedDateTime.UTC(), Proveedor: p.nombre}))
		}
	}
	sort.SliceStable(l, func(a, b int) bool { return l[a].En.After(l[b].En) })
	return l, nil
}

// remitente: "Juan Pérez <juan@x.com>" → "Juan Pérez" (sin nombre, la dirección).
func remitente(s string) string {
	if a, err := mail.ParseAddress(s); err == nil {
		if a.Name != "" {
			return a.Name
		}
		return a.Address
	}
	return strings.Trim(strings.TrimSpace(s), `"`)
}

func limpiarMail(m MailNoLeido) MailNoLeido {
	m.De = protocolo.CortarUTF8(strings.Join(strings.Fields(m.De), " "), 80)
	m.Asunto = protocolo.CortarUTF8(strings.Join(strings.Fields(m.Asunto), " "), 160)
	if m.Asunto == "" {
		m.Asunto = "(sin asunto)"
	}
	return m
}

// --- El sondeo ---

func (i *Integraciones) sondearCorreo(ctx context.Context) {
	t := time.NewTicker(CadaCorreo)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		i.revisarCorreo(ctx)
	}
}

// revisarCorreo: una vuelta por todas las cuentas (los tests la llaman directo).
// Los mails nuevos de las cuentas de un usuario van en un solo aviso.
func (i *Integraciones) revisarCorreo(ctx context.Context) {
	cs, err := i.alm.CuentasCorreo(ctx)
	if err != nil {
		i.log.Warn("correo: no pude leer las cuentas", "error", err)
		return
	}
	nuevos := map[string][]MailNoLeido{}
	var orden []string
	for _, c := range cs {
		p, err := i.proveedorCorreo(c.Proveedor)
		if err != nil || !p.activo() {
			continue
		}
		cctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
		n, err := i.revisarCuenta(cctx, p, c)
		cancelar()
		if err != nil {
			if !errors.Is(err, ErrSinCorreo) {
				i.log.Info("correo: no pude mirar la bandeja", "usuario", c.UsuarioID, "proveedor", c.Proveedor, "error", err)
			}
			continue
		}
		if len(n) > 0 {
			if _, ya := nuevos[c.UsuarioID]; !ya {
				orden = append(orden, c.UsuarioID)
			}
			nuevos[c.UsuarioID] = append(nuevos[c.UsuarioID], n...)
		}
	}
	for _, u := range orden {
		l := nuevos[u]
		sort.SliceStable(l, func(a, b int) bool { return l[a].En.After(l[b].En) })
		i.avisarCorreo(ctx, u, l)
	}
}

// revisarCuenta lee los no leídos y devuelve los nuevos. La primera vez
// (recién conectada) solo pone la marca.
func (i *Integraciones) revisarCuenta(ctx context.Context, p proveedorCorreo, c almacen.CuentaCorreo) ([]MailNoLeido, error) {
	empezo := i.ahora()
	token, err := i.tokenCorreo(ctx, p, c)
	if err != nil {
		return nil, err
	}
	k := claveCorreo(c.UsuarioID, c.Proveedor)
	i.correo.mu.Lock()
	b, primada := i.correo.bandejas[k]
	var antes []MailNoLeido
	if primada {
		antes = append(antes, b.lista...)
	}
	i.correo.mu.Unlock()
	l, err := i.noLeidos(ctx, p, token, antes)
	if errors.Is(err, ErrSinCorreo) {
		// El access token venció antes de tiempo: la próxima vuelta se renueva.
		i.correo.mu.Lock()
		delete(i.correo.tokens, k)
		i.correo.mu.Unlock()
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	var nuevos []MailNoLeido
	if c.Revisado != nil {
		// Con la memoria de la vuelta anterior, lo que no se vio y llegó
		// hace poco; recién arrancada la API, lo que llegó después de la marca.
		corte := *c.Revisado
		vistos := map[string]bool{}
		if primada {
			corte = corte.Add(-solapeCorreo)
			for _, m := range antes {
				vistos[m.ID] = true
			}
		}
		for _, m := range l {
			if !vistos[m.ID] && !m.En.Before(corte) {
				nuevos = append(nuevos, m)
			}
		}
	}
	i.correo.mu.Lock()
	i.correo.bandejas[k] = &bandeja{lista: l}
	i.correo.mu.Unlock()
	if err := i.alm.MarcarCorreo(ctx, c.UsuarioID, c.Proveedor, empezo); err != nil {
		i.log.Warn("correo: no pude guardar la marca", "error", err)
	}
	return nuevos, nil
}

// textoMail: "Mail de Juan: Reunión" (para el globo, 60 bytes).
func textoMail(m MailNoLeido) string {
	return protocolo.CortarUTF8("Mail de "+m.De+": "+m.Asunto, protocolo.MaxDecir)
}

// avisarCorreo: el aviso de mail (0x44) a los robots conectados, el SSE
// "correo" y Telegram. Varios mails, un aviso: el asunto del más nuevo y
// "Juan +2".
func (i *Integraciones) avisarCorreo(ctx context.Context, usuario string, l []MailNoLeido) {
	if len(l) == 0 {
		return
	}
	m := l[0]
	de := m.De
	if len(l) > 1 {
		mas := fmt.Sprintf(" +%d", len(l)-1)
		de = protocolo.CortarUTF8(de, protocolo.MaxCampoAviso-len(mas)) + mas
	}
	ids := []string{}
	if marco, err := protocolo.Aviso("mail", m.Asunto, de); err == nil {
		if rs, err := i.robotsDe(ctx, usuario); err == nil {
			for _, r := range rs {
				if r.conectado && i.robots.Enviar(r.ID, protocolo.CarComando, marco) == nil {
					ids = append(ids, r.ID)
				}
			}
		}
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "correo", Datos: map[string]any{
		"nuevos": l, "texto": textoMail(m), "robots": ids}})
	if i.tg == nil {
		return
	}
	if _, err := i.alm.TelegramDe(ctx, usuario); err != nil {
		return
	}
	texto := textoMail(m)
	if len(l) > 1 {
		var b strings.Builder
		fmt.Fprintf(&b, "Tenés %d mails nuevos:", len(l))
		for _, x := range l[:min(len(l), 5)] {
			b.WriteString("\n• " + x.De + ": " + protocolo.CortarUTF8(x.Asunto, 80))
		}
		texto = b.String()
	}
	if err := i.MandarTelegram(ctx, usuario, texto); err != nil {
		i.log.Info("correo: no pude avisar por Telegram", "error", err)
	}
}

// NoLeidosDe: los no leídos que se conocen de las cuentas del usuario, el
// más nuevo primero (hasta MaxNoLeidos). conectada: si tiene alguna cuenta
// ya mirada.
func (i *Integraciones) NoLeidosDe(usuario string) (l []MailNoLeido, mirada bool) {
	i.correo.mu.Lock()
	for _, p := range Proveedores {
		if b, ok := i.correo.bandejas[claveCorreo(usuario, p)]; ok {
			mirada = true
			l = append(l, b.lista...)
		}
	}
	i.correo.mu.Unlock()
	sort.SliceStable(l, func(a, b int) bool { return l[a].En.After(l[b].En) })
	if len(l) > MaxNoLeidos {
		l = l[:MaxNoLeidos]
	}
	return l, mirada
}

// resumenCorreo: para la charla con IA ("3 sin leer: Juan: Reunión; …").
func (i *Integraciones) resumenCorreo(usuario string) string {
	l, mirada := i.NoLeidosDe(usuario)
	if !mirada {
		return ""
	}
	if len(l) == 0 {
		return "no tiene mails sin leer"
	}
	var partes []string
	for _, m := range l[:min(len(l), 5)] {
		partes = append(partes, protocolo.CortarUTF8(m.De, 40)+": "+protocolo.CortarUTF8(m.Asunto, 60))
	}
	return fmt.Sprintf("tiene %d sin leer (los más nuevos: %s)", len(l), strings.Join(partes, "; "))
}

// EstadoCorreo: GET /v1/correo.
func (i *Integraciones) EstadoCorreo(ctx context.Context, usuario string) (map[string]any, error) {
	r := map[string]any{}
	var cs []almacen.CuentaCorreo
	if i.CorreoActivo() {
		var err error
		if cs, err = i.alm.CorreosDe(ctx, usuario); err != nil {
			return nil, err
		}
	}
	for _, n := range Proveedores {
		p, _ := i.proveedorCorreo(n)
		e := map[string]any{"disponible": p.activo(), "conectada": false, "direccion": ""}
		for _, c := range cs {
			if c.Proveedor == n && p.activo() {
				e["conectada"], e["direccion"] = true, c.Direccion
			}
		}
		r[n] = e
	}
	l, _ := i.NoLeidosDe(usuario)
	if l == nil {
		l = []MailNoLeido{}
	}
	r["noLeidos"] = l
	return r, nil
}
