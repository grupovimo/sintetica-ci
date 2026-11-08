package servidor

// Las herramientas del MCP de la v1.9: tocar (a un amigo), amigos, hábitos,
// horarios (lo que aprendió la nube de la presencia) y entrantes. Como las
// demás, solo ven lo del usuario del token: de un amigo, solo su nombre.

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/integraciones"
)

const guiaEntrantes = `Para que el robot avise cosas de afuera, mandá un POST a esa dirección (es secreta: quien la tenga puede avisarle):
- JSON propio: {"tipo": "pago|mail|github|pedido|otro", "titulo": "…", "de": "…", "robot": "id o nombre (opcional)"}. El robot muestra hasta 24 bytes de cada uno.
- GitHub: en el repo, Settings → Webhooks, Payload URL = la dirección, Content type application/json y Secret = el token. Avisa PR, issues, reviews, estrellas y CI fallido.
- Mercado Pago: en Tus integraciones → Webhooks, la dirección con el evento Pagos. Dice «¡Te pagaron!»; con el access token guardado en la consola, el monto.`

func (s *Servidor) sumarHerramientasCompania(srv *mcp.Server) {
	herramienta(s, srv, "amigos", "Amigos",
		"Los amigos del usuario (entre robots se mandan toques, abrazos y besos). listar; invitar (un código de 8 letras que vale 48 h, una vez, para pasarle a un amigo); "+
			"aceptar (codigo que te pasó un amigo); borrar (amigo: su nombre o id; dejan de ser amigos los dos).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{
		 "accion":{"enum":["listar","invitar","aceptar","borrar"]},
		 "codigo":{"type":"string","maxLength":20,"description":"Para aceptar: las 8 letras."},
		 "amigo":{"type":"string","maxLength":60,"description":"Para borrar: el nombre o el id del amigo."}}}`), borra, s.hAmigos)

	herramienta(s, srv, "tocar", "Tocar a un amigo",
		"Le manda un toque a un amigo: sus robots conectados lo despiertan suave y reaccionan (abrazo: se achica y se pone colorado; beso: corazones; carita: pone esa expresión), "+
			"con un globo de quién es. Si lo acarician en 30 s, vuelve. Si no tiene robots conectados y vinculó Telegram, le llega por ahí. Hasta 30 por hora a cada amigo.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{
		 "amigo":{"type":"string","maxLength":60,"description":"El nombre o el id del amigo (con uno solo, se puede omitir)."},
		 "tipo":{"enum":["toque","abrazo","beso","carita"]},
		 "expresion":{"type":"string","pattern":"`+patronNombre+`","description":"Solo para carita: la cara que pone (contento, guino, enamorado…)."},
		 "robot":{"type":"string","maxLength":60,"description":"Desde cuál de tus robots (sale su nombre en el globo); si no, tu nombre."}}}`), afuera, s.hTocar)

	herramienta(s, srv, "habitos", "Hábitos",
		"Los hábitos del robot (hasta 4, en habitos de su config: agua, leer, meditar…): ver (con la racha de días seguidos y los días hechos), "+
			"marcar (hecho hoy: el robot se pone orgulloso y, con racha de 3 o más, lo festeja) o deshacer.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{`+propRobot+`,
		 "accion":{"enum":["ver","marcar","deshacer"]},
		 "habito":{"type":"string","maxLength":40,"description":"Para marcar o deshacer: el nombre o el número (desde 0)."}}}`), cambia, s.hHabitos)

	herramienta(s, srv, "horarios", "Tus horarios",
		"Lo que aprendió la nube de cuándo llegás y te vas (lo avisa la app del celu por cercanía) y de cuándo se duerme el robot: llegas, salis y dormis (HH:MM, la mediana de los últimos 14 días; null si no sabe) "+
			"y seguridad de 0 a 1.",
		esquemaDe(`{"type":"object","additionalProperties":false,"properties":{`+propRobot+`}}`), lee, s.hHorarios)

	herramienta(s, srv, "entrantes", "Avisos entrantes",
		"La dirección secreta para que el robot avise cosas de afuera (un pago de Mercado Pago, un PR de GitHub, un pedido, lo que mande cualquier sistema con un JSON): "+
			"ver (la dirección y cómo usarla) o regenerar (una nueva: la vieja deja de andar).",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{
		 "accion":{"enum":["ver","regenerar"]}}}`), borra, s.hEntrantes)
}

// elegirAmigo: por id o por nombre (sin mayúsculas ni tildes); con uno solo, "" es ese.
func (s *Servidor) elegirAmigo(ctx context.Context, usuario, amigo string) (integraciones.Amigo, error) {
	l, err := s.integ.Amigos(ctx, usuario)
	if err != nil {
		return integraciones.Amigo{}, err
	}
	if len(l) == 0 {
		return integraciones.Amigo{}, problemaf("Todavía no tenés amigos: invitá a uno con amigos (accion invitar) y pasale el código.")
	}
	nombres := func() string {
		var p []string
		for _, a := range l {
			p = append(p, fmt.Sprintf("%s (%d)", a.Nombre, a.ID))
		}
		return strings.Join(p, ", ")
	}
	amigo = strings.TrimSpace(amigo)
	if amigo == "" {
		if len(l) == 1 {
			return l[0], nil
		}
		return integraciones.Amigo{}, problemaf("Tenés %d amigos: decime a cuál (%s).", len(l), nombres())
	}
	if id, err := strconv.ParseInt(amigo, 10, 64); err == nil {
		for _, a := range l {
			if a.ID == id {
				return a, nil
			}
		}
	}
	var hallados []integraciones.Amigo
	for _, a := range l {
		if normalMCP(a.Nombre) == normalMCP(amigo) {
			hallados = append(hallados, a)
		}
	}
	switch len(hallados) {
	case 1:
		return hallados[0], nil
	case 0:
		return integraciones.Amigo{}, problemaf("No tenés un amigo %q. Tus amigos: %s.", amigo, nombres())
	}
	return integraciones.Amigo{}, problemaf("Hay %d amigos que se llaman %q: usá el id (%s).", len(hallados), amigo, nombres())
}

// errorAmigos: los errores que se le muestran a la IA.
func errorAmigos(err error) error {
	var inv integraciones.ErrToqueInvalido
	switch {
	case errors.As(err, &inv):
		return problemaf("%s.", inv.Motivo)
	case errors.Is(err, integraciones.ErrNoEsAmigo), errors.Is(err, almacen.ErrInvitacionInvalida), errors.Is(err, almacen.ErrInvitacionPropia),
		errors.Is(err, almacen.ErrYaAmigos), errors.Is(err, integraciones.ErrMuchasInvitaciones), errors.Is(err, integraciones.ErrMuchosIntentos),
		errors.Is(err, integraciones.ErrMuchosToques), errors.Is(err, integraciones.ErrSinHabito):
		return problemaf("%s.", strings.ToUpper(err.Error()[:1])+err.Error()[1:])
	}
	return err
}

func (s *Servidor) hAmigos(ctx context.Context, q quien, in struct {
	Accion string `json:"accion"`
	Codigo string `json:"codigo"`
	Amigo  string `json:"amigo"`
}) (*mcp.CallToolResult, error) {
	switch in.Accion {
	case "listar":
		sinCambios(ctx)
		l, err := s.integ.Amigos(ctx, q.usuario)
		if err != nil {
			return nil, err
		}
		return resultado(fmt.Sprintf("Tenés %d amigos (conectado: tiene algún robot conectado que recibe tus toques).", len(l)), map[string]any{"amigos": l}), nil
	case "invitar":
		c, vence, err := s.integ.Invitar(ctx, q.usuario)
		if err != nil {
			return nil, errorAmigos(err)
		}
		datos := map[string]any{"codigo": c, "link": s.op.WebURL + "/robots?amigo=" + c, "vence": vence}
		if t := s.integ.LinkTelegramAmigo(c); t != "" {
			datos["telegram"] = t
		}
		return resultado("Pasale este código (o el link) a tu amigo: vale 48 horas y una sola vez.", datos), nil
	case "aceptar":
		if in.Codigo == "" {
			return nil, problemaf("Para aceptar hace falta codigo (las 8 letras que te pasó tu amigo).")
		}
		a, err := s.integ.AceptarAmigo(ctx, q.usuario, in.Codigo)
		if err != nil {
			return nil, errorAmigos(err)
		}
		return resultado("¡Listo! Ahora sos amigo de "+a.Nombre+".", a), nil
	case "borrar":
		if in.Amigo == "" {
			return nil, problemaf("Para borrar hace falta amigo (su nombre o id).")
		}
		a, err := s.elegirAmigo(ctx, q.usuario, in.Amigo)
		if err != nil {
			return nil, err
		}
		if err := s.integ.BorrarAmigo(ctx, q.usuario, a.ID); err != nil {
			return nil, errorAmigos(err)
		}
		return resultado("Listo: ya no son amigos con "+a.Nombre+".", nil), nil
	}
	return nil, problemaf("accion: listar, invitar, aceptar o borrar.")
}

func (s *Servidor) hTocar(ctx context.Context, q quien, in struct {
	Amigo     string `json:"amigo"`
	Tipo      string `json:"tipo"`
	Expresion string `json:"expresion"`
	Robot     string `json:"robot"`
}) (*mcp.CallToolResult, error) {
	a, err := s.elegirAmigo(ctx, q.usuario, in.Amigo)
	if err != nil {
		return nil, err
	}
	desde := ""
	if in.Robot != "" {
		v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
		if err != nil {
			return nil, err
		}
		desde = v.ID
	}
	res, err := s.integ.Tocar(ctx, q.usuario, a.ID, in.Tipo, in.Expresion, desde)
	if err != nil {
		return nil, errorAmigos(err)
	}
	switch {
	case res.Robots > 0:
		return resultado(fmt.Sprintf("Listo: le llegó a %d robot(s) de %s.", res.Robots, a.Nombre), res), nil
	case res.Telegram:
		return resultado(a.Nombre+" no tiene robots conectados: le llegó por Telegram.", res), nil
	}
	return resultado(a.Nombre+" no tiene robots conectados ni Telegram: no le llegó.", res), nil
}

func (s *Servidor) hHabitos(ctx context.Context, q quien, in struct {
	Robot  string `json:"robot"`
	Accion string `json:"accion"`
	Habito string `json:"habito"`
}) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	l, err := s.integ.Habitos(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	if in.Accion == "ver" {
		sinCambios(ctx)
		if len(l) == 0 {
			return resultado(nombreRobot(v)+" no tiene hábitos: se arman en habitos de su config (nombre, icono, hora y dias).", map[string]any{"habitos": l}), nil
		}
		return resultado(fmt.Sprintf("Los hábitos de %s (racha: días seguidos; los días que no tocan no la cortan).", nombreRobot(v)), map[string]any{"habitos": l}), nil
	}
	if in.Accion != "marcar" && in.Accion != "deshacer" {
		return nil, problemaf("accion: ver, marcar o deshacer.")
	}
	k := -1
	if n, err := strconv.Atoi(strings.TrimSpace(in.Habito)); err == nil {
		k = n
	}
	for _, h := range l {
		if normalMCP(h.Nombre) == normalMCP(in.Habito) {
			k = h.Indice
		}
	}
	if len(l) == 1 && in.Habito == "" {
		k = 0
	}
	if k < 0 || k >= len(l) {
		var p []string
		for _, h := range l {
			p = append(p, fmt.Sprintf("%d %s", h.Indice, h.Nombre))
		}
		return nil, problemaf("Decime cuál hábito (habito: el nombre o el número). Los que hay: %s.", strings.Join(p, ", "))
	}
	h, enviado, err := s.integ.MarcarHabito(ctx, v.ID, k, in.Accion == "marcar", false)
	if err != nil {
		return nil, errorAmigos(err)
	}
	t := fmt.Sprintf("Listo: %s hecho hoy (racha: %d).", h.Nombre, h.Racha)
	if in.Accion == "deshacer" {
		t = fmt.Sprintf("Listo: %s ya no figura hecho hoy (racha: %d).", h.Nombre, h.Racha)
	}
	if !enviado {
		t += " " + nombreRobot(v) + " no está conectado: lo va a ver en la consola."
	}
	return resultado(t, h), nil
}

func (s *Servidor) hHorarios(ctx context.Context, q quien, in enRobot) (*mcp.CallToolResult, error) {
	v, err := s.elegirRobot(ctx, q.usuario, in.Robot)
	if err != nil {
		return nil, err
	}
	h, err := s.integ.HorariosDe(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	t := "Lo que aprendí de tus horarios con " + nombreRobot(v) + " (hora de Argentina)."
	if h.Dias == 0 {
		t = "Todavía no sé tus horarios: los aprendo de cuándo llegás y te vas (la app del celu lo avisa por cercanía) y de cuándo se duerme " + nombreRobot(v) + "."
	}
	return resultado(t, h), nil
}

func (s *Servidor) hEntrantes(ctx context.Context, q quien, in struct {
	Accion string `json:"accion"`
}) (*mcp.CallToolResult, error) {
	var token string
	switch in.Accion {
	case "ver":
		sinCambios(ctx)
		e, err := s.alm.EntranteDe(ctx, q.usuario)
		if errors.Is(err, almacen.ErrNoEncontrado) {
			token, err = s.nuevoEntrante(ctx, q.usuario)
		} else if err == nil {
			token = s.tokenEntrante(e.Semilla)
			if !hmac.Equal(hash(token), e.TokenHash) {
				return nil, problemaf("La dirección no se puede mostrar (cambió el secreto del servidor): usá regenerar.")
			}
		}
		if err != nil {
			return nil, err
		}
	case "regenerar":
		if !s.limites.permitir("entrantes-nuevo:"+q.usuario, 10, time.Hour) {
			return nil, problemaf("La cambiaste muchas veces: esperá un rato.")
		}
		var err error
		if token, err = s.nuevoEntrante(ctx, q.usuario); err != nil {
			return nil, err
		}
	default:
		return nil, problemaf("accion: ver o regenerar.")
	}
	return resultado(guiaEntrantes, map[string]any{"url": s.urlEntrante(token), "token": token}), nil
}
