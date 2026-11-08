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
		"Los robots con parlante leen en voz alta con ElevenLabs (si el usuario conectó su cuenta en la web, con su propia clave: la factura es suya) "+
			"o, si no, con la voz gratis (Edge TTS, prendida por defecto). ver: qué está conectado, las voces y cuánto usó hoy de los topes. "+
			"probar: el robot conectado dice un texto (hasta 60 bytes; si no, un saludo) en un globo con esa voz. La clave no se puede ver ni cambiar desde acá.",
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
		g, err := s.integ.EstadoEdge(ctx, q.usuario, false)
		if err != nil {
			return nil, err
		}
		datos := map[string]any{"elevenlabs": e, "edge": g}
		gratis := "la voz gratis (Edge) está apagada"
		if g["prendida"] == true {
			gratis = fmt.Sprintf("la voz gratis (Edge) está prendida con «%v» (hoy %v de %v caracteres)", g["voz"], g["usados_hoy"], g["tope"])
		}
		if e["conectado"] != true {
			return resultado("No conectó ElevenLabs (se hace en la web, en Voz, con su propia clave); "+gratis+".", datos), nil
		}
		leer := "no lee en voz alta"
		if e["leer"] == true {
			leer = "lee en voz alta"
		}
		return resultado(fmt.Sprintf("ElevenLabs conectado, voz «%v», %s (hoy %v de %v caracteres); %s.", e["voz_nombre"], leer, e["usados_hoy"], e["tope"], gratis), datos), nil
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
	voz, err := s.integ.DecirConVoz(ctx, v.ID, texto)
	if err != nil {
		return nil, problemaf("%s no está conectado ahora: cuando se conecte, probá de nuevo.", nombreRobot(v))
	}
	switch voz {
	case "elevenlabs":
		return resultado(fmt.Sprintf("%s lo está diciendo con la voz de ElevenLabs.", nombreRobot(v)), map[string]any{"voz": voz}), nil
	case "edge":
		return resultado(fmt.Sprintf("%s lo está diciendo con la voz gratis.", nombreRobot(v)), map[string]any{"voz": voz}), nil
	}
	return resultado(fmt.Sprintf("%s lo dice en un globo con su voz propia, si tiene (no tiene parlante o la voz de la nube está apagada).", nombreRobot(v)), map[string]any{"voz": ""}), nil
}
