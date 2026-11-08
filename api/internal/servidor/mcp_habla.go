package servidor

// Las herramientas del MCP de la v1.6: decir, despertador, clima, rutinas y
// habla. Como las demás: el robot es del usuario (elegirRobot) y todo cambio de
// config pasa por cambiarConfig.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/clima"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/protocolo"
)

// SituacionesHabla: dónde puede hablar solo el robot (habla.propias).
var SituacionesHabla = []string{"despertar", "buenas-noches", "volviste", "te-extrane", "aburrido", "sueno", "bateria-baja",
	"cargando", "caricia", "saludo", "hito", "cumple-dueno", "cumple-robot", "recordatorio-hecho", "despertador", "posponer",
	"clima-frio", "clima-calor", "clima-lluvia", "clima-lindo",
	// v1.7
	"buen-dia", "comiste", "a-dormir", "tardaste", "agua", "pausa", "como-estas", "animo-bien", "animo-medio", "animo-mal",
	"jugar", "gane", "perdi", "empate", "anio-nuevo", "navidad", "dia-del-amigo", "dia-del-nino", "aniversario"}

// MaxFrasesPropias: las frases propias, en total.
const MaxFrasesPropias = 12

const patronHoraMCP = `^([01][0-9]|2[0-3]):[0-5][0-9]$`

func (s *Servidor) sumarHerramientasHabla(srv *mcp.Server) {
	herramienta(s, srv, "decir", "Decir algo",
		"El robot conectado dice un texto en un globo (con su cara achicada al lado), unos segundos o hasta que lo toquen. Hasta 60 bytes (una tilde ocupa 2). Opcional: la expresión que pone mientras.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["texto"],"properties":{`+propRobot+`,
		 "texto":{"type":"string","minLength":1,"maxLength":60},
		 "expresion":{"type":"string","pattern":"`+patronNombre+`"}}}`), cambia, s.hDecir)

	herramienta(s, srv, "despertador", "Despertador",
		"El despertador del robot (uno solo: lo despierta a esa hora, suena creciente y se pospone tocándolo). "+
			"ver; poner (hora HH:MM y lo que quieras cambiar: dias LMXJVSD, texto, posponer en minutos, sonido, activo); sacar; "+
			"y con el robot conectado: probar (suena ya), apagar, posponer.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","poner","sacar","probar","apagar","posponer"]},
		 "hora":{"type":"string","pattern":"`+patronHoraMCP+`","description":"HH:MM, hora del robot (Argentina)."},
		 "dias":{"type":"string","pattern":"^[LMXJVSD]{0,7}$","description":"Letras L M X J V S D; vacío = todos los días."},
		 "texto":{"type":"string","maxLength":60,"description":"Lo que dice al sonar (vacío = una frase suya, como «¡Buen día!»)."},
		 "posponer":{"type":"integer","minimum":1,"maximum":30,"description":"Minutos que pospone (9 si no se dice)."},
		 "sonido":{"type":"boolean"},"activo":{"type":"boolean"}}}`), borra, s.hDespertador)

	herramienta(s, srv, "clima", "Clima",
		"El clima donde está el robot (lo muestra en su modo clima y lo usa para hablar). ver: su ubicación y el clima de ahora; "+
			"buscar: lugares por nombre (q); ubicar: le pone la ubicación (lat, lon y ciudad, o q y se usa el primer lugar que encuentre).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","buscar","ubicar"]},
		 "q":{"type":"string","minLength":2,"maxLength":60,"description":"El lugar a buscar (por ejemplo «Córdoba»)."},
		 "lat":{"type":"number","minimum":-90,"maximum":90},"lon":{"type":"number","minimum":-180,"maximum":180},
		 "ciudad":{"type":"string","minLength":1,"maxLength":40,"description":"El nombre que muestra (hasta 40 bytes)."}}}`), cambia, s.hClima)

	herramienta(s, srv, "rutinas", "Rutinas",
		"Cosas que el robot hace todos los días a una hora (las automatizaciones con cuando: hora): decir algo (con plantillas como {clima} o {dueno}), "+
			"poner una cara, cambiar de modo, despertarlo, invitarte a jugar, mandar un Telegram… listar, crear (hora, dias, hacer) o borrar (id).",
		esquemaRutinas(), borra, s.hRutinas)

	herramienta(s, srv, "habla", "Cómo habla",
		"Cuánto habla solo el robot (frecuencia de 0 a 1; 0 = nunca, 0,5 normal, 1 el doble) y sus frases propias por situación "+
			"(hasta 12 en total, de hasta 60 bytes; {dueno} y {robot} se reemplazan). ver, frecuencia, agregar (situacion y texto) o borrar (situacion y, si querés, texto).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","frecuencia","agregar","borrar"]},
		 "situacion":{"enum":`+jsonDe(SituacionesHabla)+`},
		 "texto":{"type":"string","minLength":1,"maxLength":60},
		 "frecuencia":{"type":"number","minimum":0,"maximum":1}}}`), borra, s.hHabla)
}

// --- decir ---

func (s *Servidor) hDecir(ctx context.Context, q quien, in struct {
	Robot     string `json:"robot"`
	Texto     string `json:"texto"`
	Expresion string `json:"expresion"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	if len(in.Texto) > protocolo.MaxDecir || !utf8.ValidString(in.Texto) || strings.TrimSpace(in.Texto) == "" {
		return nil, problemaf("El texto es de 1 a 60 bytes (una tilde ocupa 2): «%s» tiene %d.", in.Texto, len(in.Texto))
	}
	if in.Expresion != "" {
		if ok, err := s.existeExpresion(ctx, v.ID, in.Expresion); err != nil {
			return nil, err
		} else if !ok {
			return nil, problemaf("No existe la expresión %q.", in.Expresion)
		}
	}
	m, err := protocolo.Decir(in.Texto, in.Expresion)
	if err != nil {
		return nil, problemaf("%s.", err.Error())
	}
	if err := s.enviarMCP(v, m); err != nil {
		return nil, err
	}
	return resultado(fmt.Sprintf("%s está diciendo «%s».", nombreRobot(v), in.Texto), nil), nil
}

// --- despertador ---

func (s *Servidor) hDespertador(ctx context.Context, q quien, in struct {
	Robot    string  `json:"robot"`
	Accion   string  `json:"accion"`
	Hora     string  `json:"hora"`
	Dias     *string `json:"dias"`
	Texto    *string `json:"texto"`
	Posponer *int    `json:"posponer"`
	Sonido   *bool   `json:"sonido"`
	Activo   *bool   `json:"activo"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "ver":
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		d, ok := o.crudo("despertador")
		if !ok {
			return resultado(nombreRobot(v)+" no tiene despertador.", nil), nil
		}
		return resultado("El despertador de "+nombreRobot(v)+" (la hora es la del robot: Argentina).", map[string]any{"despertador": d}), nil
	case "poner":
		if in.Texto != nil && (len(*in.Texto) > 60 || !utf8.ValidString(*in.Texto)) {
			return nil, problemaf("El texto es de hasta 60 bytes (una tilde ocupa 2).")
		}
		var final json.RawMessage
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			d := o.objetoEn("despertador")
			if in.Hora != "" {
				d.poner("hora", in.Hora)
			} else if _, ok := d.crudo("hora"); !ok {
				return problemaf("Para poner el despertador hace falta la hora (HH:MM).")
			}
			if in.Dias != nil {
				if *in.Dias == "" {
					d.quitar("dias")
				} else {
					d.poner("dias", *in.Dias)
				}
			}
			if in.Texto != nil {
				if strings.TrimSpace(*in.Texto) == "" {
					d.quitar("texto")
				} else {
					d.poner("texto", *in.Texto)
				}
			}
			if in.Posponer != nil {
				d.poner("posponer", *in.Posponer)
			}
			if in.Sonido != nil {
				d.poner("sonido", *in.Sonido)
			}
			if in.Activo != nil {
				d.poner("activo", *in.Activo)
			}
			o.poner("despertador", d)
			final, _ = json.Marshal(d)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Listo el despertador. "+s.guardada(v, c), map[string]any{"despertador": final}), nil
	case "sacar":
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			if _, ok := o.crudo("despertador"); !ok {
				return problemaf("%s no tiene despertador.", nombreRobot(v))
			}
			o.quitar("despertador")
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Saqué el despertador. "+s.guardada(v, c), nil), nil
	case "probar", "apagar", "posponer":
		m, _ := protocolo.Alarma(in.Accion)
		if err := s.enviarMCP(v, m); err != nil {
			return nil, err
		}
		texto := map[string]string{"probar": "Está sonando el despertador de %s (para probarlo).",
			"apagar": "Le mandé a %s que apague la alarma (si estaba sonando).", "posponer": "Le mandé a %s que posponga la alarma (si estaba sonando)."}
		return resultado(fmt.Sprintf(texto[in.Accion], nombreRobot(v)), nil), nil
	}
	return nil, problemaf("accion: ver, poner, sacar, probar, apagar o posponer.")
}

// --- clima ---

func (s *Servidor) hClima(ctx context.Context, q quien, in struct {
	Robot  string   `json:"robot"`
	Accion string   `json:"accion"`
	Q      string   `json:"q"`
	Lat    *float64 `json:"lat"`
	Lon    *float64 `json:"lon"`
	Ciudad string   `json:"ciudad"`
}) (*mcp.CallToolResult, error) {
	buscar := func() ([]clima.Lugar, error) {
		l, err := s.integ.Clima().Buscar(ctx, in.Q)
		if errors.Is(err, clima.ErrBusqueda) {
			return nil, problemaf("Para buscar hace falta q (de 2 a 60 bytes).")
		}
		if err != nil {
			return nil, problemaf("No pude buscar el lugar ahora (el servicio del clima no contesta): probá en un rato.")
		}
		return l, nil
	}
	if in.Accion == "buscar" {
		sinCambios(ctx)
		l, err := buscar()
		if err != nil {
			return nil, err
		}
		if len(l) == 0 {
			return resultado(fmt.Sprintf("No encontré ningún lugar que se llame %q.", in.Q), map[string]any{"lugares": l}), nil
		}
		return resultado(fmt.Sprintf("Encontré %d lugares (para ponerle uno: ubicar con su lat, lon y ciudad).", len(l)), map[string]any{"lugares": l}), nil
	}
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "ver":
		sinCambios(ctx)
		u, c := s.integ.ClimaDe(ctx, v.ID)
		datos := map[string]any{"ubicacion": u, "clima": c}
		switch {
		case u == nil:
			return resultado(nombreRobot(v)+" no tiene ubicación: ponésela con ubicar.", datos), nil
		case c == nil:
			return resultado(fmt.Sprintf("%s está en %s, pero no pude traer el clima ahora.", nombreRobot(v), u.Ciudad), datos), nil
		}
		return resultado(fmt.Sprintf("En %s: %s (máx. %d°, mín. %d°).", u.Ciudad, c.Texto(), c.Max, c.Min), datos), nil
	case "ubicar":
		var u clima.Ubicacion
		switch {
		case in.Lat != nil && in.Lon != nil:
			u = clima.Ubicacion{Lat: *in.Lat, Lon: *in.Lon, Ciudad: strings.TrimSpace(in.Ciudad)}
			if u.Ciudad == "" {
				return nil, problemaf("Con lat y lon hace falta ciudad (el nombre que muestra).")
			}
		case in.Q != "":
			l, err := buscar()
			if err != nil {
				return nil, err
			}
			if len(l) == 0 {
				return nil, problemaf("No encontré ningún lugar que se llame %q.", in.Q)
			}
			u = l[0].Ubicacion()
			if in.Ciudad != "" {
				u.Ciudad = strings.TrimSpace(in.Ciudad)
			}
		default:
			return nil, problemaf("Para ubicar hace falta lat, lon y ciudad, o q.")
		}
		u.Ciudad = clima.Cortar(u.Ciudad, clima.MaxCiudad)
		if !u.Valida() {
			return nil, problemaf("Esa ubicación no es válida (lat −90 a 90, lon −180 a 180, ciudad de 1 a 40 bytes).")
		}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			o.poner("ubicacion", u)
			return nil
		})
		if err != nil {
			return nil, err
		}
		s.integ.ClimaYa(v.ID)
		return resultado(fmt.Sprintf("Listo: %s está en %s. %s", nombreRobot(v), u.Ciudad, s.guardada(v, c)), map[string]any{"ubicacion": u}), nil
	}
	return nil, problemaf("accion: ver, buscar o ubicar.")
}

// --- rutinas ---

func esquemaRutinas() *jsonschema.Schema {
	return esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{` + propRobot + `,
	 "accion":{"enum":["listar","crear","borrar"]},
	 "id":{"type":"string","pattern":"^[A-Za-z0-9_-]{1,40}$","description":"crear: opcional (si no, rutina-HHMM); borrar: cuál."},
	 "hora":{"type":"string","pattern":"` + patronHoraMCP + `","description":"HH:MM, hora del robot (Argentina)."},
	 "dias":{"type":"string","pattern":"^[LMXJVSD]{0,7}$","description":"Letras L M X J V S D; vacío = todos los días."},
	 "hacer":{"type":"array","minItems":1,"maxItems":5,"items":{"type":"object","additionalProperties":false,"required":["tipo"],"properties":{
	   "tipo":{"enum":` + jsonDe(integraciones.TiposDeAccion) + `},
	   "texto":{"type":"string","maxLength":1000,"description":"decir (hasta 200; el robot muestra 60 bytes) y telegram. Plantillas: {robot} {dueno} {hora} {clima} {temp} {max} {min} {ciudad}."},
	   "expresion":{"type":"string","description":"cara: la que pone; decir: opcional; robot: la que le pone al otro."},
	   "modo":{"enum":` + jsonDe(protocolo.Modos) + `},
	   "juego":{"enum":` + jsonDe(protocolo.Juegos) + `,"description":"jugar: a qué lo invita."},
	   "url":{"type":"string","maxLength":500,"description":"webhook: https://… (POST firmado)."},
	   "bicho":{"type":"string","description":"robot: el id de otro robot de la cuenta."}}}}}}`)
}

func (s *Servidor) hRutinas(ctx context.Context, q quien, in struct {
	Robot  string                 `json:"robot"`
	Accion string                 `json:"accion"`
	ID     string                 `json:"id"`
	Hora   string                 `json:"hora"`
	Dias   string                 `json:"dias"`
	Hacer  []integraciones.Accion `json:"hacer"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	l, _, err := s.integ.Automatizaciones(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	guardar := func(nueva []integraciones.Automatizacion) error {
		err := s.integ.GuardarAutomatizaciones(ctx, q.usuario, v.ID, nueva)
		var inv integraciones.ErrInvalida
		if errors.As(err, &inv) {
			return problemaf("La rutina no es válida: %s.", inv.Motivo)
		}
		return err
	}
	switch in.Accion {
	case "listar":
		sinCambios(ctx)
		r := []integraciones.Automatizacion{}
		for _, a := range l {
			if a.Cuando == "hora" {
				r = append(r, a)
			}
		}
		return resultado(fmt.Sprintf("%s tiene %d rutinas (la hora es la del robot: Argentina).", nombreRobot(v), len(r)), map[string]any{"rutinas": r}), nil
	case "crear":
		if in.Hora == "" || len(in.Hacer) == 0 {
			return nil, problemaf("Para crear hace falta hora (HH:MM) y hacer (de 1 a 5 acciones).")
		}
		id := in.ID
		if id == "" {
			base := "rutina-" + strings.ReplaceAll(in.Hora, ":", "")
			id = base
			for k := 2; existeAutomatizacion(l, id); k++ {
				id = fmt.Sprintf("%s-%d", base, k)
			}
		} else if existeAutomatizacion(l, id) {
			return nil, problemaf("Ya hay una automatización con el id %q.", id)
		}
		a := integraciones.Automatizacion{ID: id, Activa: true, Cuando: "hora", Hora: in.Hora, Dias: in.Dias, Hacer: in.Hacer}
		if err := guardar(append(l, a)); err != nil {
			return nil, err
		}
		dias := "todos los días"
		if in.Dias != "" {
			dias = in.Dias
		}
		return resultado(fmt.Sprintf("Listo: la rutina %s corre a las %s (%s).", id, in.Hora, dias), map[string]any{"rutina": a}), nil
	case "borrar":
		for k, a := range l {
			if a.ID == in.ID && a.Cuando == "hora" {
				if err := guardar(append(l[:k:k], l[k+1:]...)); err != nil {
					return nil, err
				}
				return resultado(fmt.Sprintf("Borré la rutina %s (la de las %s).", a.ID, a.Hora), nil), nil
			}
		}
		return nil, problemaf("No hay una rutina con el id %q (mirá listar).", in.ID)
	}
	return nil, problemaf("accion: listar, crear o borrar.")
}

func existeAutomatizacion(l []integraciones.Automatizacion, id string) bool {
	for _, a := range l {
		if a.ID == id {
			return true
		}
	}
	return false
}

// --- habla ---

func (s *Servidor) hHabla(ctx context.Context, q quien, in struct {
	Robot      string   `json:"robot"`
	Accion     string   `json:"accion"`
	Situacion  string   `json:"situacion"`
	Texto      string   `json:"texto"`
	Frecuencia *float64 `json:"frecuencia"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	var final json.RawMessage
	cambiar := func(f func(h, propias *objeto) error) (*mcp.CallToolResult, error) {
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			h := o.objetoEn("habla")
			p := h.objetoEn("propias")
			if err := f(h, p); err != nil {
				return err
			}
			if len(p.claves) == 0 {
				h.quitar("propias")
			} else {
				h.poner("propias", p)
			}
			if len(h.claves) == 0 {
				o.quitar("habla")
			} else {
				o.poner("habla", h)
			}
			final, _ = json.Marshal(h)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Listo. "+s.guardada(v, c), map[string]any{"habla": final}), nil
	}
	switch in.Accion {
	case "ver":
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		h, ok := o.crudo("habla")
		if !ok {
			h = json.RawMessage(`{"frecuencia":0.5}`)
		}
		return resultado("Cómo habla "+nombreRobot(v)+" (además de sus frases propias, tiene las de fábrica).", map[string]any{"habla": h}), nil
	case "frecuencia":
		if in.Frecuencia == nil {
			return nil, problemaf("Hace falta frecuencia (de 0 a 1).")
		}
		return cambiar(func(h, _ *objeto) error { h.poner("frecuencia", *in.Frecuencia); return nil })
	case "agregar":
		if in.Situacion == "" || in.Texto == "" {
			return nil, problemaf("Para agregar hace falta situacion y texto.")
		}
		if len(in.Texto) > 60 || !utf8.ValidString(in.Texto) {
			return nil, problemaf("La frase es de hasta 60 bytes (una tilde ocupa 2): «%s» tiene %d.", in.Texto, len(in.Texto))
		}
		return cambiar(func(_, p *objeto) error {
			total := 0
			for _, k := range p.claves {
				var l []string
				p.leer(k, &l)
				total += len(l)
			}
			if total >= MaxFrasesPropias {
				return problemaf("Ya tiene %d frases propias (el máximo): borrá alguna antes.", MaxFrasesPropias)
			}
			var l []string
			p.leer(in.Situacion, &l)
			p.poner(in.Situacion, append(l, in.Texto))
			return nil
		})
	case "borrar":
		if in.Situacion == "" {
			return nil, problemaf("Para borrar hace falta situacion (y texto, si es una sola frase).")
		}
		return cambiar(func(_, p *objeto) error {
			var l []string
			if !p.leer(in.Situacion, &l) {
				return problemaf("No tiene frases propias para %s.", in.Situacion)
			}
			if in.Texto == "" {
				p.quitar(in.Situacion)
				return nil
			}
			r := l[:0:0]
			for _, t := range l {
				if t != in.Texto {
					r = append(r, t)
				}
			}
			if len(r) == len(l) {
				return problemaf("No tiene la frase «%s» en %s.", in.Texto, in.Situacion)
			}
			if len(r) == 0 {
				p.quitar(in.Situacion)
			} else {
				p.poner(in.Situacion, r)
			}
			return nil
		})
	}
	return nil, problemaf("accion: ver, frecuencia, agregar o borrar.")
}
