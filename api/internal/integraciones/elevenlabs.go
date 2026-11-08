package integraciones

// ElevenLabs con la cuenta (y la factura) de cada usuario: su clave va
// cifrada en la base y solo sale de la nube en el encabezado xi-api-key
// hacia api.elevenlabs.io. Acá está el cliente HTTP y la cuenta (conectar,
// voces, topes); lo de leer en voz alta en voz.go y la conversación en
// tiempo real en conversacion.go. Ver docs/nube.md, "Voz".

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// Lo de ElevenLabs (verificado en elevenlabs.io/docs).
const (
	// ModeloVoz: el de baja latencia (~100 ms) y multilingüe que sirve en el
	// texto a voz común y en los agentes (eleven_v4_turbo, el que recomiendan
	// para tiempo real, va solo por el websocket de Text to Dialogue).
	ModeloVoz = "eleven_flash_v2_5"
	// FormatoVoz: mp3 mono de 22,05 kHz a 32 kbps (4 KB por segundo: un globo
	// de 10 s son 40 KB). Android lo reproduce sin nada extra.
	FormatoVoz = "mp3_22050_32"

	TopeCaracteresPorDefecto = 5000
	MaxTopeCaracteres        = 200000
	TopeMinutosPorDefecto    = 10
	MaxTopeMinutos           = 240
	MaxClaveElevenLabs       = 200
	MaxTextoProbar           = 200
)

// Errores de la cuenta.
var (
	ErrSinElevenLabs   = errors.New("no conectaste ElevenLabs")
	ErrClaveElevenLabs = errors.New("ElevenLabs no aceptó la clave: revisala (Configuración → API Keys)")
	ErrPermisoEleven   = errors.New("a la clave le faltan permisos: hacen falta Voces, Texto a voz y Agentes")
	ErrVozElevenLabs   = errors.New("esa voz no está en tu cuenta de ElevenLabs")
	ErrTopeCaracteres  = errors.New("por hoy ya se leyó todo lo que elegiste: mañana sigue (o subí el tope)")
	ErrTopeMinutos     = errors.New("por hoy ya charlaron los minutos que elegiste: mañana sigue (o subí el tope)")
	ErrElevenLabsCaido = errors.New("ElevenLabs no respondió: probá en un rato")
)

var clienteEleven = &http.Client{Timeout: 15 * time.Second}

func (i *Integraciones) urlEleven() string {
	if u := i.op.V19.ElevenLabsURL; u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://api.elevenlabs.io"
}

// errorEleven: el estado HTTP de una respuesta que no fue 2xx.
type errorEleven struct{ estado int }

func (e errorEleven) Error() string { return fmt.Sprintf("ElevenLabs respondió %d", e.estado) }

func estadoEleven(err error) int {
	var e errorEleven
	if errors.As(err, &e) {
		return e.estado
	}
	return 0
}

// pedirEleven hace un pedido con la clave del usuario. cuerpo puede ser nil;
// si salida es *[]byte, se guarda el cuerpo crudo (el audio).
func (i *Integraciones) pedirEleven(ctx context.Context, clave, metodo, camino string, cuerpo, salida any) error {
	var r io.Reader
	if cuerpo != nil {
		b, err := json.Marshal(cuerpo)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, i.urlEleven()+camino, r)
	if err != nil {
		return err
	}
	req.Header.Set("xi-api-key", clave)
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := clienteEleven.Do(req)
	if err != nil {
		// Sin la URL (no tiene la clave, pero tampoco hace falta en el log).
		return fmt.Errorf("%w (%v)", ErrElevenLabsCaido, sinURL(err))
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10)) //nolint:errcheck
		switch res.StatusCode {
		case http.StatusUnauthorized:
			return ErrClaveElevenLabs
		case http.StatusForbidden:
			return ErrPermisoEleven
		}
		return errorEleven{res.StatusCode}
	}
	switch s := salida.(type) {
	case nil:
		return nil
	case *[]byte:
		b, err := io.ReadAll(io.LimitReader(res.Body, protocolo.MaxAudioEntero+1))
		if err != nil {
			return err
		}
		if len(b) > protocolo.MaxAudioEntero {
			return protocolo.ErrAudioGrande
		}
		*s = b
		return nil
	default:
		return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(salida)
	}
}

// VozEleven: una voz de la cuenta del usuario.
type VozEleven struct {
	ID      string `json:"id"`
	Nombre  string `json:"nombre"`
	Muestra string `json:"muestra,omitempty"` // un mp3 de ejemplo (preview_url)
}

// vocesEleven: las voces de la cuenta (hasta 300). Sirve también para
// validar la clave.
func (i *Integraciones) vocesEleven(ctx context.Context, clave string) ([]VozEleven, error) {
	var out []VozEleven
	token := ""
	for pag := 0; pag < 3; pag++ {
		q := url.Values{"page_size": {"100"}, "include_total_count": {"false"}}
		if token != "" {
			q.Set("next_page_token", token)
		}
		var r struct {
			Voices []struct {
				VoiceID    string `json:"voice_id"`
				Name       string `json:"name"`
				PreviewURL string `json:"preview_url"`
			} `json:"voices"`
			HasMore       bool   `json:"has_more"`
			NextPageToken string `json:"next_page_token"`
		}
		if err := i.pedirEleven(ctx, clave, http.MethodGet, "/v2/voices?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for _, v := range r.Voices {
			if v.VoiceID == "" {
				continue
			}
			m := v.PreviewURL
			if !strings.HasPrefix(m, "https://") {
				m = ""
			}
			out = append(out, VozEleven{ID: v.VoiceID, Nombre: protocolo.CortarUTF8(v.Name, 80), Muestra: m})
		}
		if !r.HasMore || r.NextPageToken == "" {
			break
		}
		token = r.NextPageToken
	}
	return out, nil
}

// hablarEleven: el mp3 del texto con esa voz.
func (i *Integraciones) hablarEleven(ctx context.Context, clave, voz, texto string) ([]byte, error) {
	var audio []byte
	err := i.pedirEleven(ctx, clave, http.MethodPost, "/v1/text-to-speech/"+url.PathEscape(voz)+"?output_format="+FormatoVoz,
		map[string]any{"text": texto, "model_id": ModeloVoz}, &audio)
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 {
		return nil, errors.New("ElevenLabs devolvió un audio vacío")
	}
	return audio, nil
}

// --- La cuenta ---

// cuentaEleven: la cuenta con la clave descifrada (cache de 30 s).
type cuentaEleven struct {
	almacen.CuentaElevenLabs
	clave string
}

func (i *Integraciones) cuentaEleven(ctx context.Context, usuario string) (*cuentaEleven, error) {
	i.voz.mu.Lock()
	c, ok := i.voz.cuentas[usuario]
	i.voz.mu.Unlock()
	if ok && i.ahora().Sub(c.en) < 30*time.Second {
		if c.cuenta == nil {
			return nil, ErrSinElevenLabs
		}
		return c.cuenta, nil
	}
	a, err := i.alm.ElevenLabsDe(ctx, usuario)
	var cu *cuentaEleven
	switch {
	case errors.Is(err, almacen.ErrNoEncontrado):
	case err != nil:
		return nil, err
	default:
		clave, err := i.voz.cifra.Descifrar(a.ClaveCifrada)
		if err == nil {
			cu = &cuentaEleven{CuentaElevenLabs: a, clave: clave}
		}
	}
	i.voz.mu.Lock()
	if len(i.voz.cuentas) > 5000 {
		i.voz.cuentas = map[string]cacheCuenta{}
	}
	i.voz.cuentas[usuario] = cacheCuenta{cuenta: cu, en: i.ahora()}
	i.voz.mu.Unlock()
	if cu == nil {
		return nil, ErrSinElevenLabs
	}
	return cu, nil
}

func (i *Integraciones) olvidarCuentaEleven(usuario string) {
	i.voz.mu.Lock()
	delete(i.voz.cuentas, usuario)
	delete(i.voz.usoMin, usuario)
	i.voz.mu.Unlock()
}

// CambiosVoz: lo que manda PUT /v1/voz/elevenlabs (nil = igual).
type CambiosVoz struct {
	Clave       *string `json:"clave"`
	Voz         *string `json:"voz"`
	Leer        *bool   `json:"leer"`
	Tope        *int    `json:"tope"`
	TopeMinutos *int    `json:"tope_minutos"`
}

// ErrCambioVoz: un cambio que no se puede hacer (400).
type ErrCambioVoz struct{ Motivo string }

func (e ErrCambioVoz) Error() string { return e.Motivo }

// ConectarVoz: PUT /v1/voz/elevenlabs. Con clave, la valida contra
// ElevenLabs (leyendo sus voces) y la guarda cifrada; si no, cambia la voz,
// la llave de leer en voz alta o los topes.
func (i *Integraciones) ConectarVoz(ctx context.Context, usuario string, c CambiosVoz) error {
	if c.Tope != nil && (*c.Tope < 0 || *c.Tope > MaxTopeCaracteres) {
		return ErrCambioVoz{fmt.Sprintf("el tope es de 0 a %d caracteres por día", MaxTopeCaracteres)}
	}
	if c.TopeMinutos != nil && (*c.TopeMinutos < 0 || *c.TopeMinutos > MaxTopeMinutos) {
		return ErrCambioVoz{fmt.Sprintf("el tope es de 0 a %d minutos por día", MaxTopeMinutos)}
	}
	if c.Voz != nil && (len(*c.Voz) > 64 || !utf8.ValidString(*c.Voz)) {
		return ErrVozElevenLabs
	}
	defer i.olvidarCuentaEleven(usuario)
	var clave string
	var vozActual string
	if c.Clave != nil {
		clave = strings.TrimSpace(*c.Clave)
		if clave == "" || len(clave) > MaxClaveElevenLabs || strings.ContainsAny(clave, " \t\r\n") {
			return ErrCambioVoz{"la clave no parece de ElevenLabs"}
		}
	} else {
		cu, err := i.cuentaEleven(ctx, usuario)
		if err != nil {
			return err
		}
		clave, vozActual = cu.clave, cu.Voz
	}
	var voces []VozEleven
	if c.Clave != nil || c.Voz != nil {
		var err error
		if voces, err = i.vocesEleven(ctx, clave); err != nil {
			return err
		}
	}
	elegida, nombre := vozActual, ""
	if c.Voz != nil {
		elegida = *c.Voz
	}
	if c.Clave != nil || c.Voz != nil {
		ok := false
		for _, v := range voces {
			if v.ID == elegida {
				ok, nombre = true, v.Nombre
			}
		}
		if !ok {
			if c.Voz != nil {
				return ErrVozElevenLabs
			}
			elegida = ""
			if len(voces) > 0 {
				elegida, nombre = voces[0].ID, voces[0].Nombre
			}
		}
	}
	if c.Clave != nil {
		if err := i.alm.GuardarClaveElevenLabs(ctx, usuario, i.voz.cifra.Cifrar(clave), elegida, nombre); err != nil {
			return err
		}
	}
	aj := almacen.AjustesElevenLabs{Leer: c.Leer, TopeCaracteres: c.Tope, TopeMinutos: c.TopeMinutos}
	if c.Voz != nil {
		aj.Voz, aj.VozNombre = &elegida, &nombre
	}
	if err := i.alm.CambiarElevenLabs(ctx, usuario, aj); errors.Is(err, almacen.ErrNoEncontrado) {
		return ErrSinElevenLabs
	} else if err != nil {
		return err
	}
	i.olvidarCuentaEleven(usuario)
	i.avisarVozATodos(ctx, usuario)
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "voz", Datos: map[string]any{"conectado": true}})
	return nil
}

// DesconectarVoz: DELETE /v1/voz/elevenlabs. Borra la clave y, de paso, los
// agentes que se crearon en su cuenta (si ElevenLabs contesta).
func (i *Integraciones) DesconectarVoz(ctx context.Context, usuario string) error {
	cu, _ := i.cuentaEleven(ctx, usuario)
	ids, err := i.alm.BorrarElevenLabs(ctx, usuario)
	if err != nil {
		return err
	}
	i.olvidarCuentaEleven(usuario)
	if cu != nil {
		for _, id := range ids {
			c, cancelar := context.WithTimeout(ctx, 5*time.Second)
			i.pedirEleven(c, cu.clave, http.MethodDelete, "/v1/convai/agents/"+url.PathEscape(id), nil, nil) //nolint:errcheck
			cancelar()
		}
	}
	i.avisarVozATodos(ctx, usuario)
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "voz", Datos: map[string]any{"conectado": false}})
	return nil
}

// EstadoVoz: GET /v1/voz/elevenlabs.
func (i *Integraciones) EstadoVoz(ctx context.Context, usuario string, conVoces bool) (map[string]any, error) {
	cu, err := i.cuentaEleven(ctx, usuario)
	if errors.Is(err, ErrSinElevenLabs) {
		return map[string]any{"conectado": false, "tope": TopeCaracteresPorDefecto, "tope_minutos": TopeMinutosPorDefecto}, nil
	}
	if err != nil {
		return nil, err
	}
	hoy := i.ahora().In(nube.Zona())
	usados, err := i.alm.CaracteresDeHoy(ctx, usuario, hoy)
	if err != nil {
		return nil, err
	}
	r := map[string]any{"conectado": true, "voz": cu.Voz, "voz_nombre": cu.VozNombre, "leer": cu.Leer,
		"usados_hoy": usados, "tope": cu.TopeCaracteres, "tope_minutos": cu.TopeMinutos, "segundos_hoy": nil}
	c, cancelar := context.WithTimeout(ctx, 8*time.Second)
	defer cancelar()
	if s, err := i.segundosDeHoy(c, usuario, cu); err == nil {
		r["segundos_hoy"] = s
	}
	if conVoces {
		voces, err := i.vocesEleven(c, cu.clave)
		if err != nil {
			r["error"] = err.Error()
			voces = []VozEleven{}
		}
		r["voces"] = voces
	}
	return r, nil
}

// ProbarVoz: el mp3 del texto con la voz elegida (cuenta para el tope).
func (i *Integraciones) ProbarVoz(ctx context.Context, usuario, texto string) ([]byte, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		texto = "¡Hola! Así suena mi voz."
	}
	if len(texto) > MaxTextoProbar || !utf8.ValidString(texto) {
		return nil, ErrCambioVoz{fmt.Sprintf("el texto es de hasta %d bytes", MaxTextoProbar)}
	}
	cu, err := i.cuentaEleven(ctx, usuario)
	if err != nil {
		return nil, err
	}
	return i.leerConTope(ctx, cu, texto)
}

// leerConTope: el audio del texto si entra en el tope del día.
func (i *Integraciones) leerConTope(ctx context.Context, cu *cuentaEleven, texto string) ([]byte, error) {
	if cu.Voz == "" {
		return nil, ErrVozElevenLabs
	}
	n := utf8.RuneCountInString(texto)
	if _, ok, err := i.alm.UsarCaracteres(ctx, cu.UsuarioID, i.ahora().In(nube.Zona()), n, cu.TopeCaracteres); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrTopeCaracteres
	}
	return i.hablarEleven(ctx, cu.clave, cu.Voz, texto)
}
