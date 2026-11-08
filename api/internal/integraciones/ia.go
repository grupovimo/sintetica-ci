package integraciones

// La clave de IA propia de cada usuario ("si alguien quiere poner su api key
// de Anthropic, la pondrá"): PUT /v1/ia la valida con un pedido barato (leer
// el modelo) y la guarda cifrada (api/internal/cifra, derivación "ia"). Nunca
// se devuelve ni se loguea. Con clave propia, su charla (HTTP, Telegram y el
// robot-celu por MQTT) usa SU clave y SU tope (por defecto, sin tope); sin
// ella, la del servidor con el tope de 60. Ver docs/nube.md, "Charla".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/cifra"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
)

// Los proveedores de la IA propia.
const (
	IAAnthropic  = "anthropic"
	IACompatible = "compatible"

	MaxClaveIA    = 300
	MaxTopeIA     = 10000
	urlAnthropic  = "https://api.anthropic.com"
	versionAnthro = "2023-06-01"
	sinTopeIA     = 1 << 30 // el contador del día sigue contando
)

// Errores de la cuenta de IA.
var (
	ErrSinIA       = errors.New("no pusiste una clave de IA propia")
	ErrClaveIA     = errors.New("el servicio no aceptó la clave: revisala")
	ErrModeloIA    = errors.New("ese modelo no está en el servicio")
	ErrIACaida     = errors.New("no pude validar la clave: el servicio no respondió, probá en un rato")
	patronModeloIA = regexp.MustCompile(`^[A-Za-z0-9._:/@+-]{1,100}$`)
)

// ErrCambioIA: un cambio que no se puede hacer (400).
type ErrCambioIA struct{ Motivo string }

func (e ErrCambioIA) Error() string { return e.Motivo }

// motorIA: con qué se charla (la clave propia del usuario o la del servidor).
type motorIA struct {
	propia    bool
	proveedor string // IAAnthropic | IACompatible
	clave     string
	url       string // la base (compatible) o vacía (Anthropic de siempre)
	modelo    string
	tope      int // charlas por día; 0 = sin tope
}

type cacheIA struct {
	m  *motorIA // nil: no tiene propia
	en time.Time
}

type estadoIA struct {
	cifra *cifra.Cifrador
	mu    sync.Mutex
	cache map[string]cacheIA
	http  *http.Client // a la URL que eligió el usuario: con las protecciones de los webhooks
}

func (i *Integraciones) iniciarIA(secreto []byte) {
	i.ia = estadoIA{cifra: cifra.Nuevo(secreto, "ia"), cache: map[string]cacheIA{},
		http: i.webhook.ClienteSeguro(TimeoutCharla + 5*time.Second)}
}

// motorServidor: la IA del servidor (CHARLA_URL o ANTHROPIC_API_KEY).
func (i *Integraciones) motorServidor() (motorIA, bool) {
	if i.charlaCompatible() {
		return motorIA{proveedor: IACompatible, clave: i.op.V19.CharlaClave, url: i.op.V19.CharlaURL,
			modelo: i.op.V19.AnthropicModelo, tope: CharlasPorDia}, true
	}
	if i.op.V19.AnthropicClave != "" {
		return motorIA{proveedor: IAAnthropic, clave: i.op.V19.AnthropicClave, modelo: i.modeloCharla(), tope: CharlasPorDia}, true
	}
	return motorIA{}, false
}

// iaPropia: la cuenta del usuario descifrada (cache de 30 s; nil si no tiene).
func (i *Integraciones) iaPropia(ctx context.Context, usuario string) (*motorIA, error) {
	i.ia.mu.Lock()
	c, ok := i.ia.cache[usuario]
	i.ia.mu.Unlock()
	if ok && i.ahora().Sub(c.en) < 30*time.Second {
		return c.m, nil
	}
	cu, err := i.alm.IADe(ctx, usuario)
	var m *motorIA
	switch {
	case errors.Is(err, almacen.ErrNoEncontrado):
	case err != nil:
		return nil, err
	default:
		if clave, err := i.ia.cifra.Descifrar(cu.ClaveCifrada); err == nil {
			m = &motorIA{propia: true, proveedor: cu.Proveedor, clave: clave, url: cu.URL, modelo: cu.Modelo, tope: cu.Tope}
			if m.modelo == "" {
				m.modelo = ModeloCharla
			}
		}
	}
	i.ia.mu.Lock()
	if len(i.ia.cache) > 5000 {
		i.ia.cache = map[string]cacheIA{}
	}
	i.ia.cache[usuario] = cacheIA{m: m, en: i.ahora()}
	i.ia.mu.Unlock()
	return m, nil
}

func (i *Integraciones) olvidarIA(usuario string) {
	i.ia.mu.Lock()
	delete(i.ia.cache, usuario)
	i.ia.mu.Unlock()
}

// motorDe: la IA de la charla de ese usuario (la propia gana).
func (i *Integraciones) motorDe(ctx context.Context, usuario string) (motorIA, bool) {
	if m, err := i.iaPropia(ctx, usuario); err == nil && m != nil {
		return *m, true
	}
	return i.motorServidor()
}

// CambiosIA: lo que manda PUT /v1/ia.
type CambiosIA struct {
	Proveedor string  `json:"proveedor"`
	Clave     *string `json:"clave"`
	URL       string  `json:"url"`
	Modelo    *string `json:"modelo"`
	Tope      *int    `json:"tope"`
}

// ConectarIA: PUT /v1/ia. Con clave, la valida y la guarda cifrada (con su
// URL, modelo y tope); sin clave, cambia el modelo o el tope de la que ya está.
func (i *Integraciones) ConectarIA(ctx context.Context, usuario string, c CambiosIA) error {
	if c.Tope != nil && (*c.Tope < 0 || *c.Tope > MaxTopeIA) {
		return ErrCambioIA{fmt.Sprintf("el tope es de 0 (sin tope) a %d charlas por día", MaxTopeIA)}
	}
	if c.Modelo != nil {
		m := strings.TrimSpace(*c.Modelo)
		if m != "" && !patronModeloIA.MatchString(m) {
			return ErrCambioIA{"el modelo no parece un nombre de modelo"}
		}
		c.Modelo = &m
	}
	defer i.olvidarIA(usuario)
	if c.Clave == nil {
		ya, err := i.iaPropia(ctx, usuario)
		if err != nil {
			return err
		}
		if ya == nil {
			return ErrSinIA
		}
		if c.Modelo != nil && *c.Modelo != ya.modelo {
			n := *ya
			n.modelo = *c.Modelo
			if n.modelo == "" && n.proveedor == IAAnthropic {
				n.modelo = ModeloCharla
			}
			if n.modelo == "" {
				return ErrCambioIA{"con un servicio compatible, el modelo es obligatorio"}
			}
			if err := i.validarIA(ctx, n); err != nil {
				return err
			}
		}
		if err := i.alm.CambiarIA(ctx, usuario, c.Modelo, c.Tope); errors.Is(err, almacen.ErrNoEncontrado) {
			return ErrSinIA
		} else if err != nil {
			return err
		}
		i.hub.Publicar(usuario, eventos.Evento{Tipo: "ia", Datos: map[string]any{"propia": true}})
		return nil
	}
	m := motorIA{propia: true, proveedor: c.Proveedor, clave: strings.TrimSpace(*c.Clave)}
	if c.Modelo != nil {
		m.modelo = *c.Modelo
	}
	if c.Tope != nil {
		m.tope = *c.Tope
	}
	if len(m.clave) > MaxClaveIA || strings.ContainsAny(m.clave, " \t\r\n") {
		return ErrCambioIA{"la clave no parece una clave"}
	}
	switch m.proveedor {
	case IAAnthropic:
		if m.clave == "" {
			return ErrCambioIA{"falta la clave"}
		}
		if c.URL != "" {
			return ErrCambioIA{"con Anthropic no va url"}
		}
		if m.modelo == "" {
			m.modelo = ModeloCharla
		}
	case IACompatible:
		u := strings.TrimRight(strings.TrimSpace(c.URL), "/")
		if err := i.validarURLIA(u); err != nil {
			return err
		}
		if m.modelo == "" {
			return ErrCambioIA{"con un servicio compatible, el modelo es obligatorio"}
		}
		m.url = u
	default:
		return ErrCambioIA{"el proveedor es anthropic o compatible"}
	}
	if err := i.validarIA(ctx, m); err != nil {
		return err
	}
	if err := i.alm.GuardarIA(ctx, almacen.CuentaIA{UsuarioID: usuario, Proveedor: m.proveedor, ClaveCifrada: i.ia.cifra.Cifrar(m.clave),
		URL: m.url, Modelo: m.modelo, Tope: m.tope}); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "ia", Datos: map[string]any{"propia": true}})
	return nil
}

// validarURLIA: como los webhooks (solo https —http en desarrollo—, sin
// usuario ni clave, sin IPs internas ni el propio servicio; el DNS se valida
// al conectar).
func (i *Integraciones) validarURLIA(s string) error {
	u, err := url.Parse(s)
	if s == "" || err != nil || (u.Scheme != "https" && !(i.op.Desarrollo && u.Scheme == "http")) {
		return ErrCambioIA{"la url del servicio tiene que ser https://"}
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return ErrCambioIA{"la url es la base del servicio, sin ? ni #"}
	}
	if err := i.webhook.ValidarURL(s); err != nil {
		return ErrCambioIA{err.Error()}
	}
	return nil
}

// validarIA: un pedido barato que no gasta tokens (leer el modelo); si el
// servicio compatible no tiene /models, una respuesta de 1 token.
func (i *Integraciones) validarIA(ctx context.Context, m motorIA) error {
	ctx, cancelar := context.WithTimeout(ctx, 15*time.Second)
	defer cancelar()
	if m.proveedor == IAAnthropic {
		base := urlAnthropic
		if u := i.op.V19.AnthropicURL; u != "" {
			base = strings.TrimRight(u, "/")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models/"+url.PathEscape(m.modelo), nil)
		if err != nil {
			return ErrIACaida
		}
		req.Header.Set("x-api-key", m.clave)
		req.Header.Set("anthropic-version", versionAnthro)
		r, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return ErrIACaida
		}
		defer r.Body.Close()
		io.Copy(io.Discard, io.LimitReader(r.Body, 64<<10)) //nolint:errcheck
		switch {
		case r.StatusCode == http.StatusOK:
			return nil
		case r.StatusCode == http.StatusUnauthorized || r.StatusCode == http.StatusForbidden:
			return ErrClaveIA
		case r.StatusCode == http.StatusNotFound:
			return ErrModeloIA
		}
		return ErrIACaida
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url+"/models", nil)
	if err != nil {
		return ErrIACaida
	}
	if m.clave != "" {
		req.Header.Set("Authorization", "Bearer "+m.clave)
	}
	r, err := i.ia.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrDestinoInterno) {
			return ErrCambioIA{ErrDestinoInterno.Error()}
		}
		return ErrIACaida
	}
	defer r.Body.Close()
	switch r.StatusCode {
	case http.StatusOK:
		var l struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&l) == nil && len(l.Data) > 0 {
			for _, x := range l.Data {
				if x.ID == m.modelo || strings.TrimPrefix(x.ID, "models/") == m.modelo {
					return nil
				}
			}
			return ErrModeloIA
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrClaveIA
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		// Sin /models: una charla de 1 token.
		_, _, err := i.preguntarCompatible(ctx, m, datosCharla{Nombre: "prueba", Expresiones: expresionesCharla}, "hola")
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			return ErrClaveIA
		}
		return ErrIACaida
	}
	return ErrIACaida
}

// DesconectarIA: DELETE /v1/ia.
func (i *Integraciones) DesconectarIA(ctx context.Context, usuario string) error {
	if err := i.alm.BorrarIA(ctx, usuario); err != nil {
		return err
	}
	i.olvidarIA(usuario)
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "ia", Datos: map[string]any{"propia": false}})
	return nil
}

// pistaURL: solo el host (como la dirección del calendario).
func pistaURL(s string) string {
	if u, err := url.Parse(s); err == nil {
		return u.Host
	}
	return ""
}

// EstadoIA: GET /v1/ia → {propia, proveedor, modelo, url_pista, usadas_hoy,
// tope (0 = sin tope), servidor (si hay una del servidor)}. La clave, nunca.
func (i *Integraciones) EstadoIA(ctx context.Context, usuario string) (map[string]any, error) {
	usadas, err := i.alm.CharlasDeHoy(ctx, usuario, i.ahora().In(nube.Zona()))
	if err != nil {
		return nil, err
	}
	_, servidor := i.motorServidor()
	m, err := i.iaPropia(ctx, usuario)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]any{"propia": false, "servidor": servidor, "usadas_hoy": usadas, "tope": CharlasPorDia}, nil
	}
	return map[string]any{"propia": true, "servidor": servidor, "proveedor": m.proveedor, "modelo": m.modelo,
		"url_pista": pistaURL(m.url), "usadas_hoy": usadas, "tope": m.tope}, nil
}

// MensajeIA: el error para el usuario (sin detalles de afuera).
func MensajeIA(err error) string {
	var c ErrCambioIA
	switch {
	case errors.As(err, &c), errors.Is(err, ErrClaveIA), errors.Is(err, ErrModeloIA), errors.Is(err, ErrSinIA):
		return err.Error()
	}
	return ErrIACaida.Error()
}
