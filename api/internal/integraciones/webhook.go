package integraciones

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ClienteWebhook manda los webhooks sin dejar que apunten adentro: resuelve
// el DNS, valida cada IP y se conecta a la IP que validó (así un DNS que cambia
// entre la validación y la conexión no sirve para entrar: DNS rebinding). No
// sigue redirecciones, no usa proxies y corta a los 5 s.
type ClienteWebhook struct {
	cli              *http.Client
	permitirInternos bool
	propios          []string
	// resolver se puede cambiar en los tests.
	resolver func(ctx context.Context, host string) ([]netip.Addr, error)
}

// ErrDestinoInterno: el webhook apunta a una dirección interna o al propio servicio.
var ErrDestinoInterno = errors.New("ese destino es una dirección interna: no se le pueden mandar webhooks")

// TimeoutWebhook: cuánto se espera a que responda.
const TimeoutWebhook = 5 * time.Second

// NuevoClienteWebhook arma el cliente. permitirInternos es para desarrollo.
func NuevoClienteWebhook(permitirInternos bool, propios []string) *ClienteWebhook {
	c := &ClienteWebhook{permitirInternos: permitirInternos}
	for _, p := range propios {
		if h := hostDe(p); h != "" {
			c.propios = append(c.propios, h)
		}
	}
	c.resolver = func(ctx context.Context, host string) ([]netip.Addr, error) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	}
	marcador := &net.Dialer{Timeout: TimeoutWebhook}
	tr := &http.Transport{
		Proxy: nil, // un proxy se saltearía la validación de la IP
		DialContext: func(ctx context.Context, red, dir string) (net.Conn, error) {
			host, puerto, err := net.SplitHostPort(dir)
			if err != nil {
				return nil, err
			}
			ip, err := c.resolverSeguro(ctx, host)
			if err != nil {
				return nil, err
			}
			return marcador.DialContext(ctx, red, net.JoinHostPort(ip.String(), puerto))
		},
		TLSHandshakeTimeout:    TimeoutWebhook,
		ResponseHeaderTimeout:  TimeoutWebhook,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 16 << 10,
	}
	c.cli = &http.Client{
		Timeout:       TimeoutWebhook,
		Transport:     tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c
}

// hostDe saca el host de una URL o de un "host:puerto" (en minúsculas).
func hostDe(s string) string {
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return strings.ToLower(u.Hostname())
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(strings.TrimSpace(s))
}

func (c *ClienteWebhook) esPropio(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, p := range c.propios {
		if host == p {
			return true
		}
	}
	return false
}

// nombresInternos: nombres que nunca son de afuera.
func nombreInterno(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || !strings.Contains(host, ".") {
		return true // "localhost", "api", "postgres", "emqx"…: los de Docker
	}
	for _, s := range []string{".localhost", ".local", ".internal", ".lan", ".home.arpa"} {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

// ValidarURL revisa una URL antes de guardarla (sin resolver el DNS: eso se
// hace al mandar).
func (c *ClienteWebhook) ValidarURL(s string) error {
	if s == "" || len(s) > MaxURL {
		return fmt.Errorf("la url es de 1 a %d bytes", MaxURL)
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" {
		return errors.New("la url tiene que ser http:// o https:// con un host")
	}
	if u.User != nil {
		return errors.New("la url no puede llevar usuario ni clave")
	}
	if c.permitirInternos {
		return nil
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if IPBloqueada(ip) {
			return ErrDestinoInterno
		}
		return nil
	}
	if nombreInterno(host) || c.esPropio(host) {
		return ErrDestinoInterno
	}
	return nil
}

// resolverSeguro devuelve una IP pública del host (si alguna de las que da el
// DNS es interna, no se usa ninguna).
func (c *ClienteWebhook) resolverSeguro(ctx context.Context, host string) (netip.Addr, error) {
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip}
	} else {
		if !c.permitirInternos && (nombreInterno(host) || c.esPropio(host)) {
			return netip.Addr{}, ErrDestinoInterno
		}
		if ips, err = c.resolver(ctx, host); err != nil {
			return netip.Addr{}, errors.New("no encontré ese host")
		}
	}
	if len(ips) == 0 {
		return netip.Addr{}, errors.New("no encontré ese host")
	}
	if c.permitirInternos {
		return ips[0].Unmap(), nil
	}
	propias := c.ipsPropias(ctx)
	for _, ip := range ips {
		if IPBloqueada(ip) || propias[ip.Unmap()] {
			return netip.Addr{}, ErrDestinoInterno
		}
	}
	return ips[0].Unmap(), nil
}

// ipsPropias: a dónde apuntan los hosts del propio servicio (un nombre ajeno
// que apunte a la IP de la API tampoco vale).
func (c *ClienteWebhook) ipsPropias(ctx context.Context) map[netip.Addr]bool {
	m := map[netip.Addr]bool{}
	for _, h := range c.propios {
		if ip, err := netip.ParseAddr(h); err == nil {
			m[ip.Unmap()] = true
			continue
		}
		if nombreInterno(h) {
			continue
		}
		ctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
		ips, _ := c.resolver(ctx, h)
		cancelar()
		for _, ip := range ips {
			m[ip.Unmap()] = true
		}
	}
	return m
}

var redesBloqueadas = func() []netip.Prefix {
	var l []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24",
		"203.0.113.0/24", "240.0.0.0/4", "255.255.255.255/32",
		"64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001:db8::/32", "2002::/16", "fec0::/10",
	} {
		l = append(l, netip.MustParsePrefix(s))
	}
	return l
}()

// IPBloqueada: loopback, privadas, link-local, multicast, sin especificar,
// CGNAT, de documentación, reservadas y las IPv6 que llevan una IPv4 adentro.
func IPBloqueada(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range redesBloqueadas {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// Firmar es la cabecera X-Sintetica-Firma: "sha256=" + HMAC-SHA256 del cuerpo, en hex.
func Firmar(secreto string, cuerpo []byte) string {
	m := hmac.New(sha256.New, []byte(secreto))
	m.Write(cuerpo)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// VerificarFirma la usa quien recibe el webhook (y los tests).
func VerificarFirma(secreto string, cuerpo []byte, firma string) bool {
	return hmac.Equal([]byte(Firmar(secreto, cuerpo)), []byte(firma))
}

// CuerpoWebhook es lo que se manda.
type CuerpoWebhook struct {
	Evento string         `json:"evento"`
	Robot  RobotWebhook   `json:"robot"`
	Datos  map[string]any `json:"datos"`
	En     string         `json:"en"`
}

// RobotWebhook identifica al bicho.
type RobotWebhook struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// Mandar hace el POST firmado.
func (c *ClienteWebhook) Mandar(ctx context.Context, destino, secreto string, cuerpo CuerpoWebhook) error {
	if err := c.ValidarURL(destino); err != nil {
		return err
	}
	b, err := json.Marshal(cuerpo)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(b))
	if err != nil {
		return errors.New("url inválida")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Sintetica-Webhooks/1.0")
	req.Header.Set("X-Sintetica-Evento", cuerpo.Evento)
	req.Header.Set("X-Sintetica-Firma", Firmar(secreto, b))
	r, err := c.cli.Do(req)
	if err != nil {
		return sinURL(err)
	}
	defer r.Body.Close()
	io.Copy(io.Discard, io.LimitReader(r.Body, 4096)) //nolint:errcheck
	if r.StatusCode < 200 || r.StatusCode > 299 {
		return fmt.Errorf("el webhook respondió %d", r.StatusCode)
	}
	return nil
}

// sinURL saca la URL de un error de net/http (puede llevar tokens: la del
// webhook en su query o la de Telegram en su camino).
func sinURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if ue.Timeout() {
			return errors.New("no respondió a tiempo")
		}
		if errors.Is(ue.Err, ErrDestinoInterno) {
			return ErrDestinoInterno
		}
		return errors.New(ue.Op + ": " + ue.Err.Error())
	}
	return err
}

func (i *Integraciones) mandarWebhook(ctx context.Context, usuario, destino string, c contexto, en time.Time) error {
	secreto, err := i.SecretoWebhook(ctx, usuario)
	if err != nil {
		return err
	}
	datos := map[string]any{}
	for k, v := range c.datos {
		datos[k] = v
	}
	if n, err := strconv.Atoi(c.bateria); err == nil {
		datos["bateria"] = n
	}
	if c.expresion != "" {
		datos["expresion"] = c.expresion
	}
	if c.texto != "" {
		datos["texto"] = c.texto
	}
	return i.webhook.Mandar(ctx, destino, secreto, CuerpoWebhook{
		Evento: c.evento, Robot: RobotWebhook{ID: c.bichoID, Nombre: c.robot}, Datos: datos, En: en.Format(time.RFC3339),
	})
}
