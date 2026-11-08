package servidor

// La herramienta "voz" del MCP: ver si el usuario conectó ElevenLabs y
// probarla en un robot. Nunca da la clave (ni la recibe: se conecta en la web).

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/protocolo"
)

func (s *Servidor) sumarHerramientaVoz(srv *mcp.Server) {
	herramienta(s, srv, "voz", "La voz (ElevenLabs)",
		"ver: si el usuario conectó su cuenta de ElevenLabs en la web (con su propia clave: la factura es suya), la voz, si lee en voz alta "+
			"y cuánto usó hoy de sus topes. probar: el robot conectado dice un texto (hasta 60 bytes; si no, un saludo) en un globo y, "+
			"si tiene parlante y la cuenta conectada, con esa voz. La clave no se puede ver ni cambiar desde acá.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","probar"]},
		 "texto":{"type":"string","maxLength":60}}}`), cambia, s.hVoz)
}

func (s *Servidor) hVoz(ctx context.Context, q quien, in struct {
	Robot  string `json:"robot"`
	Accion string `json:"accion"`
	Texto  string `json:"texto"`
}) (*mcp.CallToolResult, error) {
	if in.Accion == "ver" {
		sinCambios(ctx)
		e, err := s.integ.EstadoVoz(ctx, q.usuario, false)
		if err != nil {
			return nil, err
		}
		if e["conectado"] != true {
			return resultado("No conectó ElevenLabs: se hace en la web, en Voz, con su propia clave.", e), nil
		}
		leer := "no lee en voz alta"
		if e["leer"] == true {
			leer = "lee en voz alta"
		}
		return resultado(fmt.Sprintf("ElevenLabs conectado, voz «%v», %s. Hoy: %v de %v caracteres.", e["voz_nombre"], leer, e["usados_hoy"], e["tope"]), e), nil
	}
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	texto := strings.TrimSpace(in.Texto)
	if texto == "" {
		texto = "¡Hola! Así suena mi voz."
	}
	if len(texto) > protocolo.MaxDecir || !utf8.ValidString(texto) {
		return nil, problemaf("El texto es de 1 a 60 bytes (una tilde ocupa 2).")
	}
	conAudio, err := s.integ.DecirConVoz(ctx, v.ID, texto)
	if err != nil {
		return nil, problemaf("%s no está conectado ahora: cuando se conecte, probá de nuevo.", nombreRobot(v))
	}
	if !conAudio {
		return resultado(fmt.Sprintf("%s lo dice en un globo, sin voz de ElevenLabs (no tiene parlante, no está conectada la cuenta o está apagado leer en voz alta).", nombreRobot(v)), nil), nil
	}
	return resultado(fmt.Sprintf("%s lo está diciendo con su voz.", nombreRobot(v)), nil), nil
}
