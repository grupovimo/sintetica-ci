package servidor

// Las herramientas del MCP de la v1.7: jugar, cuidado, avisos y resumen. Como
// las demás: el robot es del usuario (elegirRobot) y todo cambio de config
// pasa por cambiarConfig (validada por el núcleo).

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/protocolo"
	nucleo "sintetica/firmware/api/cgo"
)

// MaxAvisos: las reacciones a avisos del celu de la config.
const MaxAvisos = 8

func (s *Servidor) sumarHerramientasCuidado(srv *mcp.Server) {
	herramienta(s, srv, "jugar", "Jugar",
		"Empieza un juego en el robot conectado (lo despierta): ppt (piedra, papel o tijera), reflejos, mirada (seguí la mirada), dado o moneda; terminar lo corta. "+
			"Se juega tocándolo; cómo le fue queda en su memoria (resumen).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["juego"],"properties":{`+propRobot+`,
		 "juego":{"enum":["ppt","reflejos","mirada","dado","moneda","terminar"]}}}`), cambia, s.hJugar)

	herramienta(s, srv, "cuidado", "Cuidarte",
		"Cómo te cuida el robot (en cuidado de su config). ver; poner lo que quieras cambiar: agua (cada cuántas horas te recuerda tomar agua, 0 = nunca, hasta 8), "+
			"pausa (minutos sin tocarlo en horario de trabajo para decirte «¡pausa activa!»: 0 = nunca, o de 30 a 240), trabajo (desde, hasta HH:MM y dias LMXJVSD), "+
			"animo (te pregunta «¿Cómo estás?» una vez por día, también por Telegram con botones), extranar y extranarDias (te manda «te extraño» por Telegram si no lo ves en 1 a 7 días) "+
			"y resumen (a su hora de dormir, un Telegram con cómo fue su día).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","poner"]},
		 "agua":{"type":"integer","minimum":0,"maximum":8},
		 "pausa":{"type":"integer","minimum":0,"maximum":240},
		 "trabajo":{"type":"object","additionalProperties":false,"properties":{
		   "desde":{"type":"string","pattern":"`+patronHoraMCP+`"},"hasta":{"type":"string","pattern":"`+patronHoraMCP+`"},
		   "dias":{"type":"string","pattern":"^[LMXJVSD]{0,7}$"}}},
		 "animo":{"type":"boolean"},"extranar":{"type":"boolean"},
		 "extranarDias":{"type":"integer","minimum":1,"maximum":7},
		 "resumen":{"type":"boolean"}}}`), cambia, s.hCuidado)

	herramienta(s, srv, "avisos", "Avisos del celu",
		"Cómo reacciona el robot a una notificación o una llamada del celu (hasta 8; gana la primera que coincide; si ninguna, reacciona como siempre). "+
			"listar; agregar (tipo notificacion o llamada; app y contacto de hasta 24 bytes, vacíos = cualquiera, sin distinguir mayúsculas; "+
			"y lo que hace: expresion, comportamiento, decir de hasta 40 bytes con {app} y {contacto}, sonido); borrar (indice, desde 0).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["listar","agregar","borrar"]},
		 "tipo":{"enum":["notificacion","llamada"]},
		 "app":{"type":"string","maxLength":24,"description":"Por ejemplo whatsapp, gmail, telegram."},
		 "contacto":{"type":"string","maxLength":24,"description":"Por ejemplo «Mamá»."},
		 "expresion":{"type":"string","pattern":"`+patronNombre+`"},
		 "comportamiento":{"type":"string","pattern":"`+patronNombre+`"},
		 "decir":{"type":"string","minLength":1,"maxLength":40,"description":"Lo que dice en un globo; {app} y {contacto} se reemplazan."},
		 "sonido":{"type":"boolean"},
		 "indice":{"type":"integer","minimum":0,"maximum":7}}}`), borra, s.hAvisos)

	herramienta(s, srv, "resumen", "Su día",
		"Cómo fue el día del robot (de su memoria, que manda cada 5 minutos mientras está conectado): cuántas veces jugó y ganó, si soñó y con qué, "+
			"a qué hora te esperó, cuántas veces te recordó tomar agua y qué le dijiste al «¿Cómo estás?».",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`}}`), lee, s.hResumen)
}

// --- jugar ---

func (s *Servidor) hJugar(ctx context.Context, q quien, in struct {
	Robot string `json:"robot"`
	Juego string `json:"juego"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	m, err := protocolo.Jugar(in.Juego)
	if err != nil {
		return nil, problemaf("%s.", err.Error())
	}
	if err := s.enviarMCP(v, m); err != nil {
		return nil, err
	}
	if in.Juego == "terminar" {
		return resultado(fmt.Sprintf("Listo: %s dejó de jugar.", nombreRobot(v)), nil), nil
	}
	return resultado(fmt.Sprintf("%s empezó a jugar a %s: se juega tocándolo.", nombreRobot(v), in.Juego), nil), nil
}

// --- cuidado ---

// cuidadoPorDefecto: lo que vale lo que no está en la config.
var cuidadoPorDefecto = map[string]any{"agua": 0, "pausa": 0, "trabajo": map[string]any{"desde": "09:00", "hasta": "18:00", "dias": "LMXJV"},
	"animo": true, "extranar": true, "extranarDias": 2, "resumen": false}

func (s *Servidor) hCuidado(ctx context.Context, q quien, in struct {
	Robot   string `json:"robot"`
	Accion  string `json:"accion"`
	Agua    *int   `json:"agua"`
	Pausa   *int   `json:"pausa"`
	Trabajo *struct {
		Desde *string `json:"desde"`
		Hasta *string `json:"hasta"`
		Dias  *string `json:"dias"`
	} `json:"trabajo"`
	Animo        *bool `json:"animo"`
	Extranar     *bool `json:"extranar"`
	ExtranarDias *int  `json:"extranarDias"`
	Resumen      *bool `json:"resumen"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "ver":
		sinCambios(ctx)
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		c, _ := o.crudo("cuidado")
		return resultado("Cómo te cuida "+nombreRobot(v)+" (lo que no está en su config vale lo de porDefecto; las horas son las del robot: Argentina).",
			map[string]any{"cuidado": c, "porDefecto": cuidadoPorDefecto}), nil
	case "poner":
		if in.Pausa != nil && *in.Pausa != 0 && *in.Pausa < 30 {
			return nil, problemaf("pausa es 0 (nunca) o de 30 a 240 minutos.")
		}
		var final json.RawMessage
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			cu := o.objetoEn("cuidado")
			ponerSi := func(k string, p any, hay bool) {
				if hay {
					cu.poner(k, p)
				}
			}
			ponerSi("agua", in.Agua, in.Agua != nil)
			ponerSi("pausa", in.Pausa, in.Pausa != nil)
			if t := in.Trabajo; t != nil {
				tr := cu.objetoEn("trabajo")
				for k, p := range map[string]*string{"desde": t.Desde, "hasta": t.Hasta, "dias": t.Dias} {
					if p != nil {
						tr.poner(k, *p)
					}
				}
				cu.poner("trabajo", tr)
			}
			ponerSi("animo", in.Animo, in.Animo != nil)
			ponerSi("extranar", in.Extranar, in.Extranar != nil)
			ponerSi("extranarDias", in.ExtranarDias, in.ExtranarDias != nil)
			ponerSi("resumen", in.Resumen, in.Resumen != nil)
			if len(cu.claves) == 0 {
				return problemaf("Decime qué cambiar: agua, pausa, trabajo, animo, extranar, extranarDias o resumen.")
			}
			o.poner("cuidado", cu)
			final, _ = json.Marshal(cu)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Listo. "+s.guardada(v, c), map[string]any{"cuidado": final}), nil
	}
	return nil, problemaf("accion: ver o poner.")
}

// --- avisos ---

// avisoConfig: uno de `avisos` (las claves en el orden del contrato).
type avisoConfig struct {
	Tipo           string `json:"tipo"`
	App            string `json:"app,omitempty"`
	Contacto       string `json:"contacto,omitempty"`
	Expresion      string `json:"expresion,omitempty"`
	Comportamiento string `json:"comportamiento,omitempty"`
	Decir          string `json:"decir,omitempty"`
	Sonido         *bool  `json:"sonido,omitempty"`
}

func (s *Servidor) hAvisos(ctx context.Context, q quien, in struct {
	Robot  string `json:"robot"`
	Accion string `json:"accion"`
	avisoConfig
	Indice *int `json:"indice"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	var l []json.RawMessage
	o.leer("avisos", &l)
	switch in.Accion {
	case "listar":
		sinCambios(ctx)
		if l == nil {
			l = []json.RawMessage{}
		}
		return resultado(fmt.Sprintf("%s tiene %d reacciones a avisos del celu (gana la primera que coincide).", nombreRobot(v), len(l)), map[string]any{"avisos": l}), nil
	case "agregar":
		a := in.avisoConfig
		a.App, a.Contacto = strings.TrimSpace(a.App), strings.TrimSpace(a.Contacto)
		switch {
		case a.Tipo == "":
			return nil, problemaf("Para agregar hace falta tipo (notificacion o llamada).")
		case len(a.App) > protocolo.MaxCampoAviso || len(a.Contacto) > protocolo.MaxCampoAviso:
			return nil, problemaf("La app y el contacto son de hasta 24 bytes (una tilde ocupa 2).")
		case a.Decir != "" && (len(a.Decir) > 40 || !utf8.ValidString(a.Decir)):
			return nil, problemaf("decir es de hasta 40 bytes (una tilde ocupa 2): «%s» tiene %d.", a.Decir, len(a.Decir))
		case a.Expresion == "" && a.Comportamiento == "" && a.Decir == "" && a.Sonido == nil:
			return nil, problemaf("Decime cómo reacciona: expresion, comportamiento, decir o sonido.")
		}
		if a.Expresion != "" {
			if ok, err := s.existeExpresion(ctx, v.ID, a.Expresion); err != nil {
				return nil, err
			} else if !ok {
				return nil, problemaf("No existe la expresión %q.", a.Expresion)
			}
		}
		if a.Comportamiento != "" {
			nombres := o.objetoEn("comportamientos").claves
			for _, c := range nucleo.ComportamientosBase() {
				nombres = append(nombres, c.Nombre)
			}
			if !slices.Contains(nombres, a.Comportamiento) {
				return nil, problemaf("No hay un comportamiento %q. Los que hay: %s.", a.Comportamiento, strings.Join(nombres, ", "))
			}
		}
		if len(l) >= MaxAvisos {
			return nil, problemaf("Ya tiene %d (el máximo): borrá alguna antes.", MaxAvisos)
		}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var actual []json.RawMessage
			o.leer("avisos", &actual)
			b, _ := json.Marshal(a)
			o.poner("avisos", append(actual, b))
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado(fmt.Sprintf("Listo: %s ya sabe cómo reaccionar. %s", nombreRobot(v), s.guardada(v, c)), map[string]any{"aviso": a, "indice": len(l)}), nil
	case "borrar":
		if in.Indice == nil || *in.Indice >= len(l) {
			return nil, problemaf("Para borrar hace falta indice (de 0 a %d; mirá listar).", max(len(l)-1, 0))
		}
		k := *in.Indice
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			r := append(l[:k:k], l[k+1:]...)
			if len(r) == 0 {
				o.quitar("avisos")
			} else {
				o.poner("avisos", r)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Lo borré. "+s.guardada(v, c), map[string]any{"borrado": l[k]}), nil
	}
	return nil, problemaf("accion: listar, agregar o borrar.")
}

// --- resumen ---

func (s *Servidor) hResumen(ctx context.Context, q quien, in enRobot) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	r, err := s.integ.ResumenDe(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	return resultado(r.Texto, r), nil
}
