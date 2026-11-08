package servidor

// Las herramientas del MCP. Cada una valida que el robot sea del usuario del
// token (elegirRobot solo mira los suyos) y todo cambio de config pasa por
// cambiarConfig: el núcleo la valida y se guarda como PUT /v1/bichos/{id}/config.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/imagenes"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/protocolo"
	nucleo "sintetica/firmware/api/cgo"
	"sintetica/shared/formato"
)

const propRobot = `"robot": {"type": "string", "maxLength": 60, "description": "El id (12 hexadecimales) o el nombre del robot. Con un solo robot se puede omitir."}`

const patronNombre = `^[a-z][a-z0-9-]{0,23}$`

type enRobot struct {
	Robot string `json:"robot"`
}

func (s *Servidor) sumarHerramientas(srv *mcp.Server) {
	herramienta(s, srv, "listar_robots", "Listar robots",
		"Los robots de la cuenta: id, nombre, si está conectado, batería, la cara que tiene, el ánimo y su pantalla.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{}}`), lee, s.hListarRobots)

	herramienta(s, srv, "ver_robot", "Ver un robot",
		"Todo de un robot: estado (conectado, batería, cara, ánimo, dormido, modo), capacidades (pantalla, perfil, firmware) y un resumen de su config (personalidad, horario, expresiones y comportamientos propios, recordatorios, reflejos, imágenes, protector).",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`}}`), lee, s.hVerRobot)

	herramienta(s, srv, "poner_cara", "Poner una cara",
		"Le deja fija una expresión (de fábrica, propia o de imagen) al robot conectado, o con soltar: true vuelve a manejar sus caras solo. Con siempre: true la deja como su cara de siempre (se guarda en la config).",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`,
		 "expresion":{"type":"string","pattern":"`+patronNombre+`","description":"El nombre de la expresión (ver listar_expresiones)."},
		 "siempre":{"type":"boolean","description":"true: en vez de fijarla un rato, la deja como su cara de siempre (en la config: sigue reaccionando y durmiendo con ella). Con soltar, vuelve a la neutral."},
		 "soltar":{"type":"boolean","description":"true: deja de forzar una cara."}}}`), cambia, s.hPonerCara)

	herramienta(s, srv, "disparar_evento", "Disparar un evento",
		"Le hace sentir algo al robot conectado, como si pasara de verdad (una caricia, una sacudida, que escuchó su nombre…): reacciona con su personalidad.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["evento"],"properties":{`+propRobot+`,
		 "evento":{"type":"string","enum":`+jsonDe(protocolo.Eventos)+`}}}`), cambia, s.hDispararEvento)

	herramienta(s, srv, "reproducir_comportamiento", "Reproducir un comportamiento",
		"Hace que el robot conectado empiece un comportamiento ya (de fábrica, como bostezar, estornudar o bailar, o uno propio de su config).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["comportamiento"],"properties":{`+propRobot+`,
		 "comportamiento":{"type":"string","pattern":"`+patronNombre+`"}}}`), cambia, s.hComportamiento)

	herramienta(s, srv, "listar_expresiones", "Listar expresiones",
		"Las expresiones que tiene el robot: las de fábrica y las propias, con sus parámetros (lo que cambian respecto de la neutral).",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`}}`), lee, s.hListarExpresiones)

	herramienta(s, srv, "crear_expresion", "Crear una expresión",
		"Inventa (o cambia) una expresión paramétrica y la guarda en la config del robot; devuelve una IMAGEN de cómo queda en su pantalla para que la mires y ajustes. "+
			"Solo poné lo que cambia respecto de la neutral (o de \"extiende\"); los rangos están en el esquema y la guía en el recurso "+uriGuia+". "+
			"solo_ver: true la dibuja sin guardar (para iterar); mostrar: true además se la pone al robot.",
		esquemaCrearExpresion(), cambia, s.hCrearExpresion)

	herramienta(s, srv, "dibujar_cara", "Dibujar una cara",
		"Dibuja una cara de IMAGEN, la que sea (ojitos de anime, de gato, pixel art, un robot…): mandás cada parte como SVG (lo mejor), URL o base64 "+
			"(ojo, y si querés ojo_cerrado, boca, ojo_der) y el robot la anima viva como a sus caras: parpadea (con ojo_cerrado o aplastando el ojo), mira, respira, "+
			"tiembla, reacciona y se duerme. Se guarda como una expresión con sprites; reposo: true la deja como su cara de siempre. Devuelve dos IMÁGENES dibujadas por "+
			"su núcleo: abierta y parpadeando. Cómo dibujar para su pantalla: recurso "+uriGuia+" (sección \"Caras de imagen\"). solo_ver: true la dibuja sin guardar.",
		esquemaDibujarCara(), cambia, s.hDibujarCara)

	herramienta(s, srv, "borrar_expresion", "Borrar una expresión propia",
		"Saca una expresión propia de la config (si reemplazaba a una de fábrica, vuelve la de fábrica).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["nombre"],"properties":{`+propRobot+`,
		 "nombre":{"type":"string","pattern":"`+patronNombre+`"}}}`), borra, s.hBorrarExpresion)

	herramienta(s, srv, "poner_imagen", "Poner una imagen",
		"Pone una foto o un GIF animado en la pantalla del robot: desde una URL https o en base64 (PNG, JPEG o GIF, hasta 8 MB). "+
			"La recorta llenando su pantalla (redonda en el broche), la guarda en una bahía (0 a 7) y opcionalmente la suma a la lista de lo que muestra o la usa de protector de pantalla. Devuelve cómo quedó.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`,
		 "url":{"type":"string","maxLength":2000,"description":"https://… de la imagen."},
		 "base64":{"type":"string","maxLength":11500000,"description":"La imagen en base64 (en vez de url)."},
		 "bahia":{"type":"integer","minimum":0,"maximum":7,"description":"Dónde guardarla (si no, la primera libre; si están todas, la 7)."},
		 "lista":{"type":"boolean","description":"true: la suma a la lista de lo que muestra (\"imagen:N\")."},
		 "protector":{"type":"boolean","description":"true: la usa de protector de pantalla."},
		 "espera":{"type":"integer","minimum":3,"maximum":3600,"description":"Segundos sin que pase nada antes del protector (10 si no se dice)."}}}`), borra, s.hPonerImagen)

	herramienta(s, srv, "borrar_imagen", "Borrar una imagen",
		"Vacía una bahía de imagen (y la saca de la lista y del protector si estaba).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["bahia"],"properties":{`+propRobot+`,
		 "bahia":{"type":"integer","minimum":0,"maximum":7}}}`), borra, s.hBorrarImagen)

	herramienta(s, srv, "recordatorios", "Recordatorios",
		"Lista, crea o borra los recordatorios del robot (máximo 12): a esa hora muestra el texto con una expresión hasta que lo toquen. Si el robot no está conectado y el usuario vinculó Telegram, le llega por ahí.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["listar","crear","borrar"]},
		 "recordatorio":{"type":"object","additionalProperties":false,"required":["hora","texto"],"description":"Para crear.","properties":{
		   "hora":{"type":"string","pattern":"^([01][0-9]|2[0-3]):[0-5][0-9]$","description":"HH:MM, hora del robot (Argentina)."},
		   "dias":{"type":"string","pattern":"^[LMXJVSD]{0,7}$","description":"Letras L M X J V S D sin repetir; vacío = todos los días."},
		   "texto":{"type":"string","minLength":1,"maxLength":40,"description":"Hasta 40 bytes (una tilde ocupa 2)."},
		   "expresion":{"type":"string","pattern":"`+patronNombre+`","description":"La cara que pone (sorpresa si no se dice)."},
		   "sonido":{"type":"boolean"},"activo":{"type":"boolean"}}},
		 "indice":{"type":"integer","minimum":0,"maximum":11,"description":"Para borrar: la posición (desde 0, la de listar)."}}}`), borra, s.hRecordatorios)

	herramienta(s, srv, "personalidad", "Personalidad",
		"Ve o cambia la personalidad: un preset y/o rasgos de 0 a 1 (energía, sociabilidad, picardía, timidez, curiosidad, mal genio) y el horario en que duerme. Sin cambios, devuelve la actual.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`,
		 "preset":{"enum":["equilibrado","timido","picaro","dormilon","carinoso","curioso","grunon","electrico"]},
		 "energia":{"type":"number","minimum":0,"maximum":1},"sociabilidad":{"type":"number","minimum":0,"maximum":1},
		 "picardia":{"type":"number","minimum":0,"maximum":1},"timidez":{"type":"number","minimum":0,"maximum":1},
		 "curiosidad":{"type":"number","minimum":0,"maximum":1},"malGenio":{"type":"number","minimum":0,"maximum":1},
		 "sueno":{"type":"string","pattern":"^([01][0-9]|2[0-3]):[0-5][0-9]-([01][0-9]|2[0-3]):[0-5][0-9]$","description":"HH:MM-HH:MM en que duerme."}}}`), cambia, s.hPersonalidad)

	herramienta(s, srv, "horario", "Horario del dueño",
		"Ve o cambia a qué hora se despierta y se duerme el robot (fuera de ese horario duerme; por defecto 08:00 a 23:30).",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`,
		 "despierta":{"type":"string","pattern":"^([01][0-9]|2[0-3]):[0-5][0-9]$"},
		 "duerme":{"type":"string","pattern":"^([01][0-9]|2[0-3]):[0-5][0-9]$"}}}`), cambia, s.hHorario)

	herramienta(s, srv, "modo", "Modo",
		"Ve o cambia qué muestra: mascota (la cara), reloj (la hora grande con ojitos) o pomodoro (cuenta regresiva, con los minutos de trabajo y descanso).",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`,
		 "modo":{"enum":["mascota","reloj","pomodoro"]},
		 "pomodoro":{"type":"object","additionalProperties":false,"properties":{
		   "trabajo":{"type":"integer","minimum":1,"maximum":120},"descanso":{"type":"integer","minimum":1,"maximum":60}}}}}`), cambia, s.hModo)

	herramienta(s, srv, "reflejos", "Reflejos",
		"Reglas simples \"si pasa X → pone la cara Y al instante\" (las del usuario van antes que las de fábrica; hasta 24). Listar, agregar o borrar.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["listar","agregar","borrar"]},
		 "si":{"type":"string","enum":`+jsonDe(protocolo.Eventos)+`,"description":"El evento (para agregar)."},
		 "reflejo":{"type":"string","pattern":"`+patronNombre+`","description":"La expresión que pone (para agregar)."},
		 "ademas":{"type":"array","maxItems":5,"items":{"type":"string","pattern":"^((expresion|sonido|led|animacion|comportamiento|particula):[a-z0-9#-]+|estado\\.(energia|animo|afecto|aburrimiento|curiosidad|sueno|estres)[+-](0(\\.[0-9]+)?|1(\\.0+)?))$"},
		   "description":"Acciones extra, como \"estado.animo+0.2\", \"particula:corazon\" o \"comportamiento:bailar\"."},
		 "indice":{"type":"integer","minimum":0,"maximum":23,"description":"Para borrar: la posición (la de listar)."}}}`), borra, s.hReflejos)

	herramienta(s, srv, "automatizaciones", "Automatizaciones",
		"\"Cuando pasa X en el robot → hacer Y\" (mandar un Telegram, llamar un webhook, poner una cara en otro robot propio, o en este: decir algo, poner una cara, cambiar de modo, despertarlo, invitarte a jugar). "+
			"cuando: hora (con hora y dias) es una rutina diaria (más cómodo con la herramienta rutinas). listar devuelve las que hay, el historial y los \"cuando\" posibles; guardar reemplaza la lista entera (hasta 20); probar corre una ya.",
		esquemaAutomatizaciones(), borra, s.hAutomatizaciones)

	herramienta(s, srv, "mandar_telegram", "Mandar un Telegram",
		"Le escribe al usuario por Telegram (si vinculó su chat con el bot de Sintética).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["texto"],"properties":{
		 "texto":{"type":"string","minLength":1,"maxLength":1000}}}`), afuera, s.hTelegram)

	s.sumarHerramientasHabla(srv)        // mcp_habla.go
	s.sumarHerramientasCuidado(srv)      // mcp_cuidado.go
	s.sumarHerramientasCompania(srv)     // mcp_compania.go (v1.9)
	s.sumarHerramientasAgendaMusica(srv) // mcp_agenda.go
	s.sumarHerramientaVoz(srv)           // mcp_voz.go
}

func jsonDe(v any) string { b, _ := json.Marshal(v); return string(b) }

// --- Robots ---

type resumenRobot struct {
	ID        string             `json:"id"`
	Nombre    string             `json:"nombre"`
	Conectado bool               `json:"conectado"`
	Bateria   *int               `json:"bateria,omitempty"`
	Expresion string             `json:"expresion,omitempty"`
	Animo     string             `json:"animo,omitempty"`
	Humor     *float64           `json:"humor,omitempty"`
	Dormido   bool               `json:"dormido,omitempty"`
	Modo      string             `json:"modo,omitempty"`
	Pantalla  protocolo.Pantalla `json:"pantalla"`
	Perfil    string             `json:"perfil"`
	Virtual   bool               `json:"virtual,omitempty"`
	Ultima    *time.Time         `json:"ultimaConexion,omitempty"`
}

func animoDe(h float64) string {
	switch {
	case h > 0.5:
		return "muy contento"
	case h > 0.15:
		return "de buen ánimo"
	case h < -0.5:
		return "muy bajoneado"
	case h < -0.15:
		return "medio bajoneado"
	}
	return "tranquilo"
}

func resumirRobot(v VistaBicho) resumenRobot {
	r := resumenRobot{ID: v.ID, Nombre: nombreRobot(v), Conectado: v.Conectado, Pantalla: pantallaDe(v), Perfil: v.Perfil, Virtual: v.Virtual, Ultima: v.UltimaConexion}
	if e := v.Estado; e != nil {
		r.Bateria, r.Expresion, r.Dormido, r.Modo = e.Bateria, e.Expresion, e.Dormido, e.Modo
		h := e.Humor
		r.Humor, r.Animo = &h, animoDe(h)
		if e.Perfil != "" {
			r.Perfil = e.Perfil
		}
	}
	return r
}

func (s *Servidor) hListarRobots(ctx context.Context, q quien, _ struct{}) (*mcp.CallToolResult, error) {
	vs, err := s.robotsDe(ctx, q.usuario)
	if err != nil {
		return nil, err
	}
	l := make([]resumenRobot, 0, len(vs))
	for _, v := range vs {
		l = append(l, resumirRobot(v))
	}
	texto := fmt.Sprintf("%d robots.", len(l))
	if len(l) == 0 {
		texto = "No hay robots en la cuenta todavía (se suman desde la app)."
	}
	return resultado(texto, map[string]any{"robots": l}), nil
}

func (s *Servidor) hVerRobot(ctx context.Context, q quien, in enRobot) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	cfg := map[string]any{}
	for _, k := range []string{"nombre", "personalidad", "horario", "modo", "pomodoro", "protector", "lista", "rotar", "cara", "voz", "dueno", "cumple"} {
		if c, ok := o.crudo(k); ok {
			cfg[k] = c
		}
	}
	if e := o.objetoEn("expresiones"); len(e.claves) > 0 {
		cfg["expresionesPropias"] = e.claves
	}
	if c := o.objetoEn("comportamientos"); len(c.claves) > 0 {
		cfg["comportamientosPropios"] = c.claves
	}
	if c, ok := o.crudo("recordatorios"); ok {
		cfg["recordatorios"] = c
	}
	if c, ok := o.crudo("reglas"); ok {
		cfg["reflejos"] = c
	}
	imgs, err := s.alm.Imagenes(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	var bahias []map[string]any
	for _, i := range almacen.SoloImagenes(imgs) {
		bahias = append(bahias, map[string]any{"bahia": i.Espacio, "bytes": i.Largo})
	}
	caras := map[string]any{}
	for k, m := range spritesUsados(o) {
		caras[k] = m
	}
	versionCfg := 0
	if v.Config != nil {
		versionCfg = v.Config.Version
	}
	r := resumirRobot(v)
	datos := map[string]any{
		"robot": r,
		"capacidades": map[string]any{
			"pantalla": r.Pantalla, "perfil": r.Perfil, "firmware": versionDe(v), "bahiasImagen": 8,
			"maxRecordatorios": 12, "maxReflejos": 24, "maxExpresionesParametricas": 48,
			"sprites": protocolo.MaxSprites, "maxCarasDeImagen": 8,
		},
		"config":         cfg,
		"versionConfig":  versionCfg,
		"imagenes":       bahias,
		"carasDeImagen":  caras,
		"sprites":        listaSprites(imgs),
		"sincronizacion": v.Sincro,
	}
	estado := "desconectado"
	if v.Conectado {
		estado = "conectado"
	}
	return resultado(fmt.Sprintf("%s está %s.", r.Nombre, estado), datos), nil
}

func versionDe(v VistaBicho) string {
	if v.Estado != nil {
		return v.Estado.Version
	}
	return ""
}

// --- Comandos ---

func (s *Servidor) hPonerCara(ctx context.Context, q quien, in struct {
	Robot     string `json:"robot"`
	Expresion string `json:"expresion"`
	Siempre   bool   `json:"siempre"`
	Soltar    bool   `json:"soltar"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	if in.Siempre {
		// La cara de siempre va en la config ("cara": {"expresion": …}).
		if !in.Soltar {
			if in.Expresion == "" {
				return nil, problemaf("Decime cuál expresión, o soltar: true para volver a la neutral.")
			}
			if ok, err := s.existeExpresion(ctx, v.ID, in.Expresion); err != nil {
				return nil, err
			} else if !ok {
				return nil, problemaf("%s no tiene la expresión %q: mirá listar_expresiones.", nombreRobot(v), in.Expresion)
			}
		}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			cara := o.objetoEn("cara")
			if in.Soltar {
				cara.quitar("expresion")
			} else {
				cara.poner("expresion", in.Expresion)
			}
			if len(cara.claves) == 0 {
				o.quitar("cara")
			} else {
				o.poner("cara", cara)
			}
			if _, ok := o.crudo("version"); !ok {
				o.poner("version", 2)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if in.Soltar {
			return resultado("Listo: vuelve a su cara neutral de siempre. "+s.guardada(v, c), nil), nil
		}
		return resultado(fmt.Sprintf("Listo: %s es su cara de siempre. %s", in.Expresion, s.guardada(v, c)), nil), nil
	}
	if in.Soltar || in.Expresion == "" {
		if !in.Soltar {
			return nil, problemaf("Decime la expresión, o soltar: true para que vuelva a poner sus caras solo.")
		}
		if err := s.enviarMCP(v, protocolo.Expresion("")); err != nil {
			return nil, err
		}
		return resultado("Listo: "+nombreRobot(v)+" vuelve a manejar sus caras solo.", nil), nil
	}
	if ok, err := s.existeExpresion(ctx, v.ID, in.Expresion); err != nil {
		return nil, err
	} else if !ok {
		return nil, problemaf("%s no tiene la expresión %q: mirá listar_expresiones o creala con crear_expresion.", nombreRobot(v), in.Expresion)
	}
	if err := s.enviarMCP(v, protocolo.Expresion(in.Expresion)); err != nil {
		return nil, err
	}
	return resultado(fmt.Sprintf("Listo: %s tiene cara de %s (fija hasta que la sueltes).", nombreRobot(v), in.Expresion), nil), nil
}

// existeExpresion: de fábrica (paramétrica), propia o un dibujo de la config.
func (s *Servidor) existeExpresion(ctx context.Context, id, nombre string) (bool, error) {
	for _, e := range nucleo.ExpresionesBase() {
		if e.Nombre == nombre {
			return true, nil
		}
	}
	for _, c := range carasDibujoBase() {
		if c == nombre {
			return true, nil
		}
	}
	o, err := s.configObjeto(ctx, id)
	if err != nil {
		return false, err
	}
	_, ok := o.objetoEn("expresiones").crudo(nombre)
	return ok, nil
}

// carasDibujoBase: las caras de dibujos de fábrica (shared/formato/caras).
func carasDibujoBase() []string {
	var l []string
	for _, p := range formato.Caras() {
		l = append(l, p.Nombre)
	}
	return l
}

func (s *Servidor) hDispararEvento(ctx context.Context, q quien, in struct {
	Robot  string `json:"robot"`
	Evento string `json:"evento"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	m, err := protocolo.Evento(in.Evento)
	if err != nil {
		return nil, problemaf("%s", err.Error())
	}
	if err := s.enviarMCP(v, m); err != nil {
		return nil, err
	}
	return resultado(fmt.Sprintf("Listo: %s sintió «%s».", nombreRobot(v), in.Evento), nil), nil
}

func (s *Servidor) hComportamiento(ctx context.Context, q quien, in struct {
	Robot          string `json:"robot"`
	Comportamiento string `json:"comportamiento"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	nombres := o.objetoEn("comportamientos").claves
	for _, c := range nucleo.ComportamientosBase() {
		nombres = append(nombres, c.Nombre)
	}
	if !slices.Contains(nombres, in.Comportamiento) {
		return nil, problemaf("No hay un comportamiento %q. Los que hay: %s.", in.Comportamiento, strings.Join(nombres, ", "))
	}
	if err := s.enviarMCP(v, protocolo.Comportamiento(in.Comportamiento)); err != nil {
		return nil, err
	}
	return resultado(fmt.Sprintf("Listo: %s empezó «%s».", nombreRobot(v), in.Comportamiento), nil), nil
}

// --- Expresiones ---

func (s *Servidor) hListarExpresiones(ctx context.Context, q quien, in enRobot) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	propias := o.objetoEn("expresiones")
	type expr struct {
		Nombre     string          `json:"nombre"`
		Parametros json.RawMessage `json:"parametros,omitempty"`
		Reemplaza  bool            `json:"reemplazaUnaDeFabrica,omitempty"`
		Dibujo     bool            `json:"dibujo,omitempty"` // de formas (v1): no se edita con crear_expresion
		Imagen     bool            `json:"imagen,omitempty"` // de imagen (sprites): se rehace con dibujar_cara
	}
	var base, mias []expr
	deFabrica := map[string]bool{}
	for _, e := range nucleo.ExpresionesBase() {
		deFabrica[e.Nombre] = true
		if _, ok := propias.crudo(e.Nombre); ok {
			continue
		}
		base = append(base, expr{Nombre: e.Nombre, Parametros: json.RawMessage(e.JSON)})
	}
	for _, k := range propias.claves {
		c, _ := propias.crudo(k)
		var f struct {
			Formas  json.RawMessage `json:"formas"`
			Sprites json.RawMessage `json:"sprites"`
		}
		json.Unmarshal(c, &f) //nolint:errcheck
		e := expr{Nombre: k, Reemplaza: deFabrica[k], Imagen: f.Sprites != nil}
		if f.Formas != nil {
			e.Dibujo = true
		} else {
			e.Parametros = c
		}
		mias = append(mias, e)
	}
	return resultado(fmt.Sprintf("%d de fábrica y %d propias.", len(base), len(mias)),
		map[string]any{"deFabrica": base, "propias": mias}), nil
}

// esquemaCrearExpresion: las propiedades de una expresión paramétrica del
// schema de la config, más las de la herramienta.
func esquemaCrearExpresion() *jsonschema.Schema {
	props, _ := propiedadesCara()
	p := map[string]any{}
	for k, v := range props {
		p[k] = v
	}
	var robot map[string]any
	json.Unmarshal([]byte(`{`+propRobot+`}`), &robot) //nolint:errcheck
	p["robot"] = robot["robot"]
	p["nombre"] = map[string]any{"type": "string", "pattern": patronNombre,
		"description": "Nombre de la expresión: minúsculas, números y guiones, hasta 24 (sin tildes). Si ya existe, se reemplaza."}
	p["solo_ver"] = map[string]any{"type": "boolean", "description": "true: solo la dibuja (no guarda ni la muestra)."}
	p["mostrar"] = map[string]any{"type": "boolean", "description": "true: además se la pone ya al robot (si está conectado)."}
	b, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"nombre"}, "properties": p})
	return esquemaDe(string(b))
}

// clavesHerramientaExpresion no van a la config.
var clavesHerramientaExpresion = map[string]bool{"robot": true, "nombre": true, "solo_ver": true, "mostrar": true}

func (s *Servidor) hCrearExpresion(ctx context.Context, q quien, in map[string]any) (*mcp.CallToolResult, error) {
	robot, _ := in["robot"].(string)
	nombre, _ := in["nombre"].(string)
	soloVer, _ := in["solo_ver"].(bool)
	mostrar, _ := in["mostrar"].(bool)
	v, err := s.elegirRobot(ctx, q.usuario, robot)
	if err != nil {
		return nil, err
	}
	expr := map[string]any{}
	for k, x := range in {
		if !clavesHerramientaExpresion[k] {
			expr[k] = x
		}
	}
	if len(expr) == 0 {
		return nil, problemaf("La expresión no cambia nada respecto de la neutral: poné al menos un parámetro (ojo, gx, mcurve…).")
	}
	if e, _ := expr["extiende"].(string); e == nombre {
		return nil, problemaf("Una expresión no puede extenderse a sí misma.")
	}
	aplicar := func(o *objeto) error {
		ex := o.objetoEn("expresiones")
		ex.poner(nombre, expr)
		o.poner("expresiones", ex)
		if _, ok := o.crudo("version"); !ok {
			o.poner("version", 2)
		}
		return nil
	}
	pant := pantallaDe(v)
	if soloVer {
		sinCambios(ctx)
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		aplicar(o) //nolint:errcheck
		b, err := validarConfig(o)
		if err != nil {
			return nil, err
		}
		png, err := dibujarExpresionPNG(b, nombre, pant)
		if err != nil {
			return nil, err
		}
		return conImagen(resultado(fmt.Sprintf("Así quedaría «%s» en %s (%d × %d%s). No la guardé: si te gusta, llamá de nuevo sin solo_ver.",
			nombre, nombreRobot(v), pant.Ancho, pant.Alto, redondaTexto(pant)), nil), png), nil
	}
	c, err := s.cambiarConfig(ctx, q, v, aplicar)
	if err != nil {
		return nil, err
	}
	png, err := dibujarExpresionPNG([]byte(c.Texto), nombre, pant)
	if err != nil {
		return nil, err
	}
	texto := fmt.Sprintf("Guardé «%s». %s", nombre, s.guardada(v, c))
	for _, e := range nucleo.ExpresionesBase() {
		if e.Nombre == nombre {
			texto += " (Reemplaza a la de fábrica con ese nombre.)"
		}
	}
	if mostrar {
		if err := s.enviarMCP(v, protocolo.Expresion(nombre)); err != nil {
			var p problema
			if !errors.As(err, &p) {
				return nil, err
			}
			texto += " " + p.msg
		} else {
			texto += " Se la puse (fija: usá poner_cara con soltar para que vuelva a las suyas)."
			if s.nube.EstadoSincro(v.ID).Config != "al-dia" {
				texto += " Si todavía no le llegó la config nueva, la va a poner apenas le llegue."
			}
		}
	}
	return conImagen(resultado(texto, nil), png), nil
}

func redondaTexto(p protocolo.Pantalla) string {
	if p.Redonda {
		return ", redonda"
	}
	return ""
}

// dibujarExpresionPNG: el núcleo la dibuja como en el robot (al doble si
// entra, para que se vea nítida) y se pasa a PNG.
func dibujarExpresionPNG(config []byte, nombre string, p protocolo.Pantalla) ([]byte, error) {
	w, h := p.Ancho, p.Alto
	if w*2 <= 640 && h*2 <= 640 {
		w, h = w*2, h*2
	}
	px, err := nucleo.DibujarExpresion(config, nombre, w, h, p.Redonda, 0.6)
	if errors.Is(err, nucleo.ErrSinExpresion) {
		return nil, problemaf("No hay una expresión paramétrica %q para dibujar.", nombre)
	}
	var ec *nucleo.ErrorConfig
	if errors.As(err, &ec) {
		return nil, problemaf("El robot no aceptaría esa expresión: %s.", ec.Detalle)
	}
	if err != nil {
		return nil, err
	}
	return imagenes.PNG(imagenes.DesdeRGBA(px, w, h)), nil
}

func (s *Servidor) hBorrarExpresion(ctx context.Context, q quien, in struct {
	Robot  string `json:"robot"`
	Nombre string `json:"nombre"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	var antes map[string]int
	c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
		ex := o.objetoEn("expresiones")
		if _, ok := ex.crudo(in.Nombre); !ok {
			return problemaf("%s no tiene una expresión propia %q.", nombreRobot(v), in.Nombre)
		}
		antes = spritesUsados(o)[in.Nombre]
		ex.quitar(in.Nombre)
		if len(ex.claves) == 0 {
			o.quitar("expresiones")
		} else {
			o.poner("expresiones", ex)
		}
		// Si era la cara de siempre, vuelve la neutral.
		cara := o.objetoEn("cara")
		var e string
		if cara.leer("expresion", &e) && e == in.Nombre {
			cara.quitar("expresion")
			if len(cara.claves) == 0 {
				o.quitar("cara")
			} else {
				o.poner("cara", cara)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Una cara de imagen: sus sprites, si no los usa otra.
	s.borrarSpritesSueltos(ctx, v.ID, antes, mustObjeto(c.Texto))
	return resultado(fmt.Sprintf("Borré «%s». %s", in.Nombre, s.guardada(v, c)), nil), nil
}

// --- Imágenes ---

func (s *Servidor) hPonerImagen(ctx context.Context, q quien, in struct {
	Robot     string `json:"robot"`
	URL       string `json:"url"`
	Base64    string `json:"base64"`
	Bahia     *int   `json:"bahia"`
	Lista     bool   `json:"lista"`
	Protector bool   `json:"protector"`
	Espera    int    `json:"espera"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	if (in.URL == "") == (in.Base64 == "") {
		return nil, problemaf("Mandá la imagen como url o como base64 (una de las dos).")
	}
	var datos []byte
	if in.URL != "" {
		ctxB, cancelar := context.WithTimeout(ctx, integraciones.TimeoutBajar)
		defer cancelar()
		datos, _, err = s.integ.Bajar(ctxB, in.URL, imagenes.MaxEntrada)
		if err != nil {
			return nil, problemaf("No pude bajar la imagen: %s.", err.Error())
		}
	} else {
		b64 := in.Base64
		if i := strings.Index(b64, ";base64,"); strings.HasPrefix(b64, "data:") && i > 0 {
			b64 = b64[i+len(";base64,"):]
		}
		datos, err = base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			if datos, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(b64)); err != nil {
				return nil, problemaf("El base64 no es válido.")
			}
		}
	}
	pant := pantallaDe(v)
	img, err := imagenes.Preparar(datos, pant.Ancho, pant.Alto, pant.Redonda)
	if err != nil {
		return nil, problemaf("%s.", strings.TrimSuffix(err.Error(), "."))
	}
	bahia := 7
	if in.Bahia != nil {
		bahia = *in.Bahia
	} else {
		usadas, err := s.alm.Imagenes(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		ocupada := map[int]bool{}
		for _, u := range usadas {
			ocupada[u.Espacio] = true
		}
		for k := 0; k < 8; k++ {
			if !ocupada[k] {
				bahia = k
				break
			}
		}
	}
	if err := s.nube.GuardarImagen(ctx, v.ID, bahia, img.Datos); err != nil {
		return nil, err
	}
	texto := fmt.Sprintf("Guardé la imagen en la bahía %d (%d × %d%s", bahia, pant.Ancho, pant.Alto, redondaTexto(pant))
	if img.Cuadros > 1 {
		texto += fmt.Sprintf(", animada: %d cuadros", img.Cuadros)
	}
	texto += ")."
	if in.Lista || in.Protector {
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			if in.Lista {
				var l []string
				o.leer("lista", &l)
				item := fmt.Sprintf("imagen:%d", bahia)
				if !slices.Contains(l, item) {
					if len(l) >= 16 {
						return problemaf("La lista ya tiene 16 cosas: sacá alguna antes.")
					}
					l = append(l, item)
				}
				o.poner("lista", l)
			}
			if in.Protector {
				espera := in.Espera
				if espera == 0 {
					espera = 10
				}
				o.poner("protector", map[string]int{"imagen": bahia, "espera": espera})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if in.Lista {
			texto += " La sumé a la lista."
		}
		if in.Protector {
			texto += " Es el protector de pantalla."
		}
		texto += " " + s.guardada(v, c)
	} else if s.nube.Conectado(v.ID) {
		texto += " Se la estoy mandando. Para que la muestre, sumala a la lista (lista: true) o usala de protector."
	} else {
		texto += " Le llega cuando se conecte."
	}
	return conImagen(resultado(texto, nil), imagenes.PNG(img.Vista)), nil
}

func (s *Servidor) hBorrarImagen(ctx context.Context, q quien, in struct {
	Robot string `json:"robot"`
	Bahia int    `json:"bahia"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	item := fmt.Sprintf("imagen:%d", in.Bahia)
	var l []string
	o.leer("lista", &l)
	var prot struct {
		Imagen *int `json:"imagen"`
	}
	o.leer("protector", &prot)
	texto := fmt.Sprintf("Vacié la bahía %d.", in.Bahia)
	if slices.Contains(l, item) || (prot.Imagen != nil && *prot.Imagen == in.Bahia) {
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var l []string
			o.leer("lista", &l)
			l = slices.DeleteFunc(l, func(x string) bool { return x == item })
			if len(l) == 0 {
				o.quitar("lista")
			} else {
				o.poner("lista", l)
			}
			var p struct {
				Imagen *int `json:"imagen"`
			}
			if o.leer("protector", &p) && p.Imagen != nil && *p.Imagen == in.Bahia {
				o.quitar("protector")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		texto += " La saqué de la lista y del protector. " + s.guardada(v, c)
	}
	if err := s.nube.BorrarImagen(ctx, v.ID, in.Bahia); err != nil {
		return nil, err
	}
	return resultado(texto, nil), nil
}

// --- Recordatorios, personalidad, horario, modo, reflejos ---

type recordatorio struct {
	Hora      string `json:"hora"`
	Dias      string `json:"dias,omitempty"`
	Texto     string `json:"texto"`
	Expresion string `json:"expresion,omitempty"`
	Sonido    *bool  `json:"sonido,omitempty"`
	Activo    *bool  `json:"activo,omitempty"`
}

func (s *Servidor) hRecordatorios(ctx context.Context, q quien, in struct {
	Robot        string        `json:"robot"`
	Accion       string        `json:"accion"`
	Recordatorio *recordatorio `json:"recordatorio"`
	Indice       *int          `json:"indice"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "listar":
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		var l []json.RawMessage
		o.leer("recordatorios", &l)
		if l == nil {
			l = []json.RawMessage{}
		}
		return resultado(fmt.Sprintf("%s tiene %d recordatorios (la hora es la del robot: Argentina).", nombreRobot(v), len(l)), map[string]any{"recordatorios": l}), nil
	case "crear":
		r := in.Recordatorio
		if r == nil {
			return nil, problemaf("Para crear hace falta recordatorio: {hora, texto, …}.")
		}
		if !utf8.ValidString(r.Texto) || len(r.Texto) > 40 {
			return nil, problemaf("El texto es de hasta 40 bytes (una tilde ocupa 2): «%s» tiene %d.", r.Texto, len(r.Texto))
		}
		if r.Expresion != "" {
			if ok, err := s.existeExpresion(ctx, v.ID, r.Expresion); err != nil {
				return nil, err
			} else if !ok {
				return nil, problemaf("No existe la expresión %q.", r.Expresion)
			}
		}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var l []json.RawMessage
			o.leer("recordatorios", &l)
			if len(l) >= 12 {
				return problemaf("Ya tiene 12 recordatorios (el máximo): borrá alguno antes.")
			}
			b, _ := json.Marshal(r)
			o.poner("recordatorios", append(l, b))
			return nil
		})
		if err != nil {
			return nil, err
		}
		dias := "todos los días"
		if r.Dias != "" {
			dias = r.Dias
		}
		return resultado(fmt.Sprintf("Listo: a las %s (%s) %s muestra «%s». %s", r.Hora, dias, nombreRobot(v), r.Texto, s.guardada(v, c)), nil), nil
	case "borrar":
		if in.Indice == nil {
			return nil, problemaf("Para borrar hace falta indice (el de listar, desde 0).")
		}
		var borrado recordatorio
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var l []json.RawMessage
			o.leer("recordatorios", &l)
			if *in.Indice >= len(l) {
				return problemaf("No hay recordatorio %d (tiene %d).", *in.Indice, len(l))
			}
			json.Unmarshal(l[*in.Indice], &borrado) //nolint:errcheck
			l = append(l[:*in.Indice], l[*in.Indice+1:]...)
			if len(l) == 0 {
				o.quitar("recordatorios")
			} else {
				o.poner("recordatorios", l)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado(fmt.Sprintf("Borré el de las %s («%s»). %s", borrado.Hora, borrado.Texto, s.guardada(v, c)), nil), nil
	}
	return nil, problemaf("accion: listar, crear o borrar.")
}

func (s *Servidor) hPersonalidad(ctx context.Context, q quien, in map[string]any) (*mcp.CallToolResult, error) {
	robot, _ := in["robot"].(string)
	v, err := s.elegirRobot(ctx, q.usuario, robot)
	if err != nil {
		return nil, err
	}
	delete(in, "robot")
	if len(in) == 0 {
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		p, _ := o.crudo("personalidad")
		if p == nil {
			p = json.RawMessage(`{"preset":"equilibrado"}`)
		}
		return resultado("La personalidad de "+nombreRobot(v)+" (los rasgos que no están salen del preset).", map[string]any{"personalidad": p}), nil
	}
	var final json.RawMessage
	c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
		p := o.objetoEn("personalidad")
		if _, nuevo := in["preset"]; nuevo {
			// Un preset nuevo arranca de cero: los rasgos viejos no quedan pegados.
			for _, k := range []string{"energia", "sociabilidad", "picardia", "timidez", "curiosidad", "malGenio"} {
				p.quitar(k)
			}
		}
		for _, k := range []string{"preset", "energia", "sociabilidad", "picardia", "timidez", "curiosidad", "malGenio", "sueno"} {
			if x, ok := in[k]; ok {
				p.poner(k, x)
			}
		}
		o.poner("personalidad", p)
		final, _ = json.Marshal(p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resultado("Cambié la personalidad. "+s.guardada(v, c), map[string]any{"personalidad": final}), nil
}

func (s *Servidor) hHorario(ctx context.Context, q quien, in struct {
	Robot     string `json:"robot"`
	Despierta string `json:"despierta"`
	Duerme    string `json:"duerme"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	if in.Despierta == "" && in.Duerme == "" {
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		h := map[string]string{"despierta": "08:00", "duerme": "23:30"}
		o.leer("horario", &h)
		return resultado(fmt.Sprintf("%s se despierta a las %s y se duerme a las %s.", nombreRobot(v), h["despierta"], h["duerme"]), nil), nil
	}
	h := map[string]string{}
	c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
		o.leer("horario", &h)
		if in.Despierta != "" {
			h["despierta"] = in.Despierta
		}
		if in.Duerme != "" {
			h["duerme"] = in.Duerme
		}
		o.poner("horario", h)
		return nil
	})
	if err != nil {
		return nil, err
	}
	des, due := h["despierta"], h["duerme"]
	if des == "" {
		des = "08:00"
	}
	if due == "" {
		due = "23:30"
	}
	return resultado(fmt.Sprintf("Listo: se despierta a las %s y se duerme a las %s. %s", des, due, s.guardada(v, c)), nil), nil
}

func (s *Servidor) hModo(ctx context.Context, q quien, in struct {
	Robot    string          `json:"robot"`
	Modo     string          `json:"modo"`
	Pomodoro *map[string]int `json:"pomodoro"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	if in.Modo == "" && in.Pomodoro == nil {
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		modo := "mascota"
		o.leer("modo", &modo)
		p := map[string]int{"trabajo": 25, "descanso": 5}
		o.leer("pomodoro", &p)
		return resultado(fmt.Sprintf("%s está en modo %s (pomodoro: %d min de trabajo y %d de descanso).", nombreRobot(v), modo, p["trabajo"], p["descanso"]), nil), nil
	}
	c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
		if in.Modo != "" {
			o.poner("modo", in.Modo)
		}
		if in.Pomodoro != nil {
			p := map[string]int{}
			o.leer("pomodoro", &p)
			for k, x := range *in.Pomodoro {
				p[k] = x
			}
			o.poner("pomodoro", p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	texto := "Listo."
	if in.Modo != "" {
		texto = fmt.Sprintf("Listo: %s pasa a modo %s.", nombreRobot(v), in.Modo)
	}
	return resultado(texto+" "+s.guardada(v, c), nil), nil
}

type regla struct {
	Si       string   `json:"si"`
	Entonces []string `json:"entonces"`
	Y        string   `json:"y,omitempty"`
}

func (s *Servidor) hReflejos(ctx context.Context, q quien, in struct {
	Robot   string   `json:"robot"`
	Accion  string   `json:"accion"`
	Si      string   `json:"si"`
	Reflejo string   `json:"reflejo"`
	Ademas  []string `json:"ademas"`
	Indice  *int     `json:"indice"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "listar":
		o, err := s.configObjeto(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		var l []json.RawMessage
		o.leer("reglas", &l)
		if l == nil {
			l = []json.RawMessage{}
		}
		return resultado(fmt.Sprintf("%s tiene %d reglas propias (van antes que las de fábrica).", nombreRobot(v), len(l)), map[string]any{"reglas": l}), nil
	case "agregar":
		if in.Si == "" || in.Reflejo == "" {
			return nil, problemaf("Para agregar hacen falta si (el evento) y reflejo (la expresión).")
		}
		if ok, err := s.existeExpresion(ctx, v.ID, in.Reflejo); err != nil {
			return nil, err
		} else if !ok {
			return nil, problemaf("No existe la expresión %q: creala antes con crear_expresion.", in.Reflejo)
		}
		r := regla{Si: in.Si, Entonces: append([]string{"reflejo:" + in.Reflejo}, in.Ademas...)}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var l []json.RawMessage
			o.leer("reglas", &l)
			if len(l) >= 24 {
				return problemaf("Ya tiene 24 reglas (el máximo): borrá alguna antes.")
			}
			b, _ := json.Marshal(r)
			o.poner("reglas", append([]json.RawMessage{b}, l...)) // la nueva gana
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado(fmt.Sprintf("Listo: cuando pase «%s», %s pone cara de %s al instante. %s", in.Si, nombreRobot(v), in.Reflejo, s.guardada(v, c)), nil), nil
	case "borrar":
		if in.Indice == nil {
			return nil, problemaf("Para borrar hace falta indice (el de listar, desde 0).")
		}
		c, err := s.cambiarConfig(ctx, q, v, func(o *objeto) error {
			var l []json.RawMessage
			o.leer("reglas", &l)
			if *in.Indice >= len(l) {
				return problemaf("No hay regla %d (tiene %d).", *in.Indice, len(l))
			}
			l = append(l[:*in.Indice], l[*in.Indice+1:]...)
			if len(l) == 0 {
				o.quitar("reglas")
			} else {
				o.poner("reglas", l)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return resultado("Borré la regla. "+s.guardada(v, c), nil), nil
	}
	return nil, problemaf("accion: listar, agregar o borrar.")
}

// --- Automatizaciones y Telegram (las de integraciones) ---

func esquemaAutomatizaciones() *jsonschema.Schema {
	cuandos := jsonDe(integraciones.Cuandos())
	return esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{` + propRobot + `,
	 "accion":{"enum":["listar","guardar","probar"]},
	 "automatizaciones":{"type":"array","maxItems":20,"description":"Para guardar: la lista ENTERA (reemplaza la que hay; traela antes con listar).","items":{
	   "type":"object","additionalProperties":false,"required":["id","activa","cuando","hacer"],"properties":{
	     "id":{"type":"string","pattern":"^[A-Za-z0-9_-]{1,40}$"},
	     "activa":{"type":"boolean"},
	     "cuando":{"type":"string","enum":` + cuandos + `},
	     "valor":{"type":"string","description":"Solo con cuando = expresion: cuál."},
	     "hora":{"type":"string","pattern":"^([01][0-9]|2[0-3]):[0-5][0-9]$","description":"Solo con cuando = hora: HH:MM (hora del robot, Argentina)."},
	     "dias":{"type":"string","pattern":"^[LMXJVSD]{0,7}$","description":"Solo con cuando = hora: letras L M X J V S D (vacío = todos)."},
	     "hacer":{"type":"array","minItems":1,"maxItems":5,"items":{"type":"object","additionalProperties":false,"required":["tipo"],"properties":{
	       "tipo":{"enum":` + jsonDe(integraciones.TiposDeAccion) + `},
	       "texto":{"type":"string","maxLength":1000,"description":"telegram: el mensaje; decir: lo que dice (hasta 200; muestra 60 bytes). Plantillas {robot} {bateria} {expresion} {evento} {texto} {hora} {dueno} {ciudad} {clima} {temp} {max} {min}."},
	       "url":{"type":"string","maxLength":500,"description":"webhook: https://… (POST firmado)."},
	       "bicho":{"type":"string","description":"robot: el id de otro robot de la cuenta."},
	       "expresion":{"type":"string","description":"robot: la cara que le pone al otro; cara: la que pone este; decir: opcional."},
	       "modo":{"enum":` + jsonDe(protocolo.Modos) + `,"description":"modo: el que pone (se guarda en la config)."},
	       "juego":{"enum":` + jsonDe(protocolo.Juegos) + `,"description":"jugar: a qué lo invita."}}}}}}},
	 "n":{"type":"string","description":"Para probar: la posición (desde 0) o el id."}}}`)
}

func (s *Servidor) hAutomatizaciones(ctx context.Context, q quien, in struct {
	Robot            string                          `json:"robot"`
	Accion           string                          `json:"accion"`
	Automatizaciones *[]integraciones.Automatizacion `json:"automatizaciones"`
	N                string                          `json:"n"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	switch in.Accion {
	case "listar":
		l, h, err := s.integ.Automatizaciones(ctx, v.ID)
		if err != nil {
			return nil, err
		}
		if len(h) > 10 {
			h = h[:10]
		}
		if h == nil {
			h = []almacen.Ejecucion{}
		}
		return resultado(fmt.Sprintf("%s tiene %d automatizaciones.", nombreRobot(v), len(l)),
			map[string]any{"automatizaciones": l, "historial": h, "cuandos": integraciones.Cuandos()}), nil
	case "guardar":
		if in.Automatizaciones == nil {
			return nil, problemaf("Para guardar hace falta automatizaciones (la lista entera).")
		}
		err := s.integ.GuardarAutomatizaciones(ctx, q.usuario, v.ID, *in.Automatizaciones)
		var inv integraciones.ErrInvalida
		if errors.As(err, &inv) {
			return nil, problemaf("Automatizaciones inválidas: %s.", inv.Motivo)
		}
		if err != nil {
			return nil, err
		}
		return resultado(fmt.Sprintf("Guardé %d automatizaciones de %s.", len(*in.Automatizaciones), nombreRobot(v)), nil), nil
	case "probar":
		res, err := s.integ.Probar(ctx, q.usuario, v.ID, in.N)
		if errors.Is(err, integraciones.ErrNoExiste) || errors.Is(err, integraciones.ErrLimite) {
			return nil, problemaf("%s.", err.Error())
		}
		if err != nil {
			return nil, err
		}
		return resultado("La corrí.", map[string]any{"resultados": res}), nil
	}
	return nil, problemaf("accion: listar, guardar o probar.")
}

func (s *Servidor) hTelegram(ctx context.Context, q quien, in struct {
	Texto string `json:"texto"`
}) (*mcp.CallToolResult, error) {
	if !s.limites.permitir("mcp-telegram:"+q.usuario, 20, time.Hour) {
		return nil, problemaf("Ya mandaste muchos Telegrams esta hora: esperá un rato.")
	}
	if err := s.integ.MandarTelegram(ctx, q.usuario, in.Texto); err != nil {
		return nil, problemaf("No pude: %s.", err.Error())
	}
	return resultado("Listo: te llegó por Telegram.", nil), nil
}
