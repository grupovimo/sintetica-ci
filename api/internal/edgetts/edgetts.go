// Package edgetts habla con el servicio "leer en voz alta" de Microsoft Edge
// (las voces neuronales, sin clave): un websocket con un SSML de ida y el mp3
// de vuelta. NO es un servicio oficial: puede cambiar o cortarse sin aviso.
// Está aislado acá para que sea fácil de arreglar; el protocolo sale del
// paquete edge-tts (github.com/rany2/edge-tts: constants.py, drm.py y
// communicate.py), que lo sigue de cerca. Lo que suele cambiar: la versión de
// Chromium (VersionChromium) y el token Sec-MS-GEC (gec.go).
package edgetts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Lo del servicio (de constants.py de edge-tts, noviembre de 2026).
const (
	TokenCliente    = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	VersionChromium = "143.0.3650.75"
	base            = "speech.platform.bing.com/consumer/speech/synthesize/readaloud"
	// Formato: mp3 mono de 24 kHz a 48 kbps (el que usa edge-tts; 6 KB por
	// segundo, un globo de 10 s son 60 KB).
	Formato = "audio-24khz-48kbitrate-mono-mp3"
	// VozPorDefecto: una de Argentina.
	VozPorDefecto = "es-AR-ElenaNeural"
	// MaxAudio: lo más que se acepta de vuelta.
	MaxAudio = 512 << 10
	// MaxTexto: bytes de texto por pedido (los globos son de 60).
	MaxTexto = 1000
)

func versionMayor() string { return strings.SplitN(VersionChromium, ".", 2)[0] }

// Errores.
var (
	ErrVoz      = errors.New("edge-tts: voz inválida")
	ErrTexto    = errors.New("edge-tts: texto vacío o largo")
	ErrSinAudio = errors.New("edge-tts: no llegó audio")
)

// Cliente habla con el servicio. El cero no sirve: usá Nuevo.
type Cliente struct {
	// URLWS y URLVoces: las del servicio (los tests ponen un servidor falso).
	URLWS, URLVoces string
	HTTP            *http.Client
	Ahora           func() time.Time

	mu      sync.Mutex
	desfase time.Duration // el reloj de Microsoft menos el nuestro (si nos rechazó por la hora)
}

// Nuevo: el cliente con las URLs de verdad.
func Nuevo() *Cliente {
	return &Cliente{
		URLWS:    "wss://" + base + "/edge/v1",
		URLVoces: "https://" + base + "/voices/list",
		HTTP:     &http.Client{Timeout: 15 * time.Second},
		Ahora:    time.Now,
	}
}

var patronVoz = regexp.MustCompile(`^[a-z]{2,3}-[A-Z]{2}(-[A-Za-z]+)?-[A-Za-z]+Neural$`)

// VozValida: el nombre corto de una voz ("es-AR-ElenaNeural").
func VozValida(v string) bool { return len(v) <= 64 && patronVoz.MatchString(v) }

func (c *Cliente) token() string {
	c.mu.Lock()
	d := c.desfase
	c.mu.Unlock()
	return TokenGEC(c.Ahora().Add(d).Unix())
}

// corregirReloj: con un 403, la hora del servidor (Date) para el próximo token.
func (c *Cliente) corregirReloj(r *http.Response) bool {
	if r == nil {
		return false
	}
	t, err := http.ParseTime(r.Header.Get("Date"))
	if err != nil {
		return false
	}
	c.mu.Lock()
	c.desfase = t.Sub(c.Ahora())
	c.mu.Unlock()
	return true
}

func encabezados() http.Header {
	m := versionMayor()
	h := http.Header{}
	h.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/"+m+
		".0.0.0 Safari/537.36 Edg/"+m+".0.0.0")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Pragma", "no-cache")
	h.Set("Cache-Control", "no-cache")
	h.Set("Cookie", "muid="+aleatorio()+";")
	return h
}

func aleatorio() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return strings.ToUpper(hex.EncodeToString(b))
}

func (c *Cliente) consulta(conConexion bool) string {
	q := url.Values{}
	if conConexion {
		q.Set("TrustedClientToken", TokenCliente)
		q.Set("ConnectionId", strings.ToLower(aleatorio()))
	} else {
		q.Set("trustedclienttoken", TokenCliente)
	}
	q.Set("Sec-MS-GEC", c.token())
	q.Set("Sec-MS-GEC-Version", "2-"+VersionChromium)
	return q.Encode()
}

// fechaJS: la fecha como la da JavaScript (lo que manda Edge).
func fechaJS(t time.Time) string {
	return t.UTC().Format("Mon Jan 02 2006 15:04:05") + " GMT+0000 (Coordinated Universal Time)"
}

// limpiar: el servicio no acepta algunos caracteres de control.
func limpiar(s string) string {
	return strings.Map(func(r rune) rune {
		if r <= 8 || r == 11 || r == 12 || (r >= 14 && r <= 31) {
			return ' '
		}
		return r
	}, s)
}

func ssml(voz, texto string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(limpiar(texto))) //nolint:errcheck
	return "<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'><voice name='" + voz +
		"'><prosody pitch='+0Hz' rate='+0%' volume='+0%'>" + b.String() + "</prosody></voice></speak>"
}

// Hablar: el mp3 del texto con esa voz.
func (c *Cliente) Hablar(ctx context.Context, voz, texto string) ([]byte, error) {
	if !VozValida(voz) {
		return nil, ErrVoz
	}
	if strings.TrimSpace(texto) == "" || len(texto) > MaxTexto {
		return nil, ErrTexto
	}
	audio, r, err := c.hablar(ctx, voz, texto)
	if r != nil && r.StatusCode == http.StatusForbidden && c.corregirReloj(r) {
		audio, _, err = c.hablar(ctx, voz, texto) // con la hora de ellos
	}
	return audio, err
}

func (c *Cliente) hablar(ctx context.Context, voz, texto string) ([]byte, *http.Response, error) {
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Proxy: http.ProxyFromEnvironment}
	h := encabezados()
	h.Set("Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold")
	ws, r, err := d.DialContext(ctx, c.URLWS+"?"+c.consulta(true), h)
	if err != nil {
		if r != nil {
			return nil, r, fmt.Errorf("edge-tts: respondió %d", r.StatusCode)
		}
		return nil, nil, fmt.Errorf("edge-tts: no me pude conectar: %w", err)
	}
	defer ws.Close()
	if dl, ok := ctx.Deadline(); ok {
		ws.SetReadDeadline(dl)  //nolint:errcheck
		ws.SetWriteDeadline(dl) //nolint:errcheck
	} else {
		ws.SetReadDeadline(time.Now().Add(20 * time.Second)) //nolint:errcheck
	}
	ahora := fechaJS(c.Ahora())
	config := "X-Timestamp:" + ahora + "\r\nContent-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},` +
		`"outputFormat":"` + Formato + `"}}}}` + "\r\n"
	if err := ws.WriteMessage(websocket.TextMessage, []byte(config)); err != nil {
		return nil, nil, fmt.Errorf("edge-tts: %w", err)
	}
	pedido := "X-RequestId:" + strings.ToLower(aleatorio()) + "\r\nContent-Type:application/ssml+xml\r\nX-Timestamp:" + ahora +
		"Z\r\nPath:ssml\r\n\r\n" + ssml(voz, texto) // la Z de más es como lo manda Edge
	if err := ws.WriteMessage(websocket.TextMessage, []byte(pedido)); err != nil {
		return nil, nil, fmt.Errorf("edge-tts: %w", err)
	}
	var audio []byte
	for {
		tipo, m, err := ws.ReadMessage()
		if err != nil {
			return nil, nil, fmt.Errorf("edge-tts: se cortó: %w", err)
		}
		switch tipo {
		case websocket.TextMessage:
			cab, _, _ := bytes.Cut(m, []byte("\r\n\r\n"))
			if camino(cab) == "turn.end" {
				if len(audio) == 0 {
					return nil, nil, ErrSinAudio
				}
				return audio, nil, nil
			}
		case websocket.BinaryMessage:
			if len(m) < 2 {
				return nil, nil, errors.New("edge-tts: mensaje binario cortado")
			}
			n := int(binary.BigEndian.Uint16(m))
			if 2+n > len(m) {
				return nil, nil, errors.New("edge-tts: encabezado más largo que el mensaje")
			}
			cab, datos := m[2:2+n], m[2+n:]
			if camino(cab) != "audio" || len(datos) == 0 {
				continue // el último viene sin Content-Type y vacío
			}
			if len(audio)+len(datos) > MaxAudio {
				return nil, nil, errors.New("edge-tts: el audio es demasiado largo")
			}
			audio = append(audio, datos...)
		}
	}
}

// camino: el valor de "Path:" en los encabezados de un mensaje.
func camino(cab []byte) string {
	for _, l := range strings.Split(string(cab), "\r\n") {
		if k, v, ok := strings.Cut(l, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Path") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Voz: una de la lista.
type Voz struct {
	Nombre string `json:"id"`     // ShortName: es-AR-ElenaNeural
	Titulo string `json:"nombre"` // "Elena (Argentina)"
	Idioma string `json:"idioma"` // Locale: es-AR
	Genero string `json:"genero"` // Female / Male
}

// Voces: las del servicio que empiezan con ese prefijo de idioma ("es-").
func (c *Cliente) Voces(ctx context.Context, prefijo string) ([]Voz, error) {
	l, r, err := c.voces(ctx)
	if r != nil && r.StatusCode == http.StatusForbidden && c.corregirReloj(r) {
		l, _, err = c.voces(ctx)
	}
	if err != nil {
		return nil, err
	}
	var out []Voz
	for _, v := range l {
		if strings.HasPrefix(v.ShortName, prefijo) && VozValida(v.ShortName) {
			t := v.FriendlyName
			if i := strings.Index(t, " - "); i > 0 {
				t = strings.TrimPrefix(t[:i], "Microsoft ") // "Microsoft Elena Online (Natural) - Spanish (Argentina)"
			}
			out = append(out, Voz{Nombre: v.ShortName, Titulo: t, Idioma: v.Locale, Genero: v.Gender})
		}
	}
	return out, nil
}

type vozCruda struct {
	ShortName, FriendlyName, Locale, Gender string
}

func (c *Cliente) voces(ctx context.Context) ([]vozCruda, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URLVoces+"?"+c.consulta(false), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header = encabezados()
	m := versionMayor()
	req.Header.Set("Sec-CH-UA", `" Not;A Brand";v="99", "Microsoft Edge";v="`+m+`", "Chromium";v="`+m+`"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Accept", "*/*")
	r, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("edge-tts: no me pude conectar: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, r, fmt.Errorf("edge-tts: respondió %d", r.StatusCode)
	}
	var l []vozCruda
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&l); err != nil {
		return nil, nil, fmt.Errorf("edge-tts: la lista de voces no se entiende: %w", err)
	}
	return l, nil, nil
}
