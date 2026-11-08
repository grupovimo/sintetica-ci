package servidor

// Las herramientas del MCP de la v1.9 que son de la nube: agenda y música.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

func (s *Servidor) sumarHerramientasAgendaMusica(srv *mcp.Server) {
	herramienta(s, srv, "agenda", "Tu agenda",
		"Los próximos eventos del calendario que conectaste en la web (Agenda: la dirección secreta en formato iCal de Google Calendar o cualquier .ics). "+
			"dias: cuántos días mirar (1 a 14; por defecto 2: hoy y mañana). Los robots avisan unos minutos antes de cada evento. Horas de Argentina.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{
		 "dias":{"type":"integer","minimum":1,"maximum":14}}}`), lee, s.hAgenda)

	herramienta(s, srv, "musica", "Música",
		"ver: qué suena en el Spotify del usuario (si lo conectó en la web; la nube se lo manda solo a los robots conectados, que bailan al ritmo). "+
			"mandar: le dice a un robot conectado qué suena (sonando, titulo hasta 40 bytes, artista hasta 24, bpm opcional) para que baile; sonando: false lo para.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","mandar"]},
		 "sonando":{"type":"boolean"},
		 "titulo":{"type":"string","maxLength":120},
		 "artista":{"type":"string","maxLength":120},
		 "bpm":{"type":"integer","minimum":0,"maximum":255}}}`), cambia, s.hMusica)
}

func (s *Servidor) hAgenda(ctx context.Context, q quien, in struct {
	Dias int `json:"dias"`
}) (*mcp.CallToolResult, error) {
	dias := in.Dias
	if dias == 0 {
		dias = 2
	}
	hoy := time.Now().In(nube.Zona())
	inicio := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 0, 0, 0, 0, hoy.Location())
	l, err := s.integ.EventosAgenda(ctx, q.usuario, inicio.AddDate(0, 0, dias))
	if errors.Is(err, integraciones.ErrSinAgenda) {
		return nil, problemaf("No conectó un calendario: se hace en la web, en Agenda (pegando la dirección secreta en formato iCal).")
	}
	if err != nil {
		return nil, err
	}
	if len(l) == 0 {
		return resultado(fmt.Sprintf("No hay nada en los próximos %d días.", dias), map[string]any{"eventos": l}), nil
	}
	var b strings.Builder
	for _, e := range l {
		cuando := e.Inicio.In(nube.Zona()).Format("02/01 15:04")
		if e.TodoElDia {
			cuando = e.Inicio.In(nube.Zona()).Format("02/01") + " (todo el día)"
		}
		fmt.Fprintf(&b, "• %s %s\n", cuando, e.Titulo)
	}
	return resultado(strings.TrimSpace(b.String()), map[string]any{"eventos": l}), nil
}

func (s *Servidor) hMusica(ctx context.Context, q quien, in struct {
	Robot   string `json:"robot"`
	Accion  string `json:"accion"`
	Sonando *bool  `json:"sonando"`
	Titulo  string `json:"titulo"`
	Artista string `json:"artista"`
	BPM     int    `json:"bpm"`
}) (*mcp.CallToolResult, error) {
	if in.Accion == "ver" {
		sinCambios(ctx)
		e, err := s.integ.EstadoMusica(ctx, q.usuario)
		if err != nil {
			return nil, err
		}
		switch {
		case e["disponible"] == false:
			return resultado("Spotify no está prendido en este servidor.", e), nil
		case e["conectada"] == false:
			return resultado("No conectó Spotify (se hace en la web).", e), nil
		case e["suena"] == nil:
			return resultado("No suena nada ahora.", e), nil
		}
		return resultado("", e), nil
	}
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	sonando := in.Sonando == nil || *in.Sonando
	titulo := protocolo.CortarUTF8(strings.TrimSpace(in.Titulo), protocolo.MaxTituloMusica)
	artista := protocolo.CortarUTF8(strings.TrimSpace(in.Artista), protocolo.MaxArtistaMusica)
	if err := s.integ.MandarMusica(q.usuario, v.ID, sonando, titulo, artista, in.BPM); err != nil {
		return nil, problemaf("%s no está conectado ahora.", nombreRobot(v))
	}
	if !sonando {
		return resultado(nombreRobot(v)+" dejó de bailar.", nil), nil
	}
	return resultado(nombreRobot(v)+" está bailando.", nil), nil
}
