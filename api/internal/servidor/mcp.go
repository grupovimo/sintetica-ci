package servidor

// El servidor MCP (docs/nube.md, "MCP"): cada usuario conecta su Claude
// (claude.ai, Claude Desktop, Claude Code) o su ChatGPT y la IA maneja sus
// robots con herramientas (mcp_herramientas.go). Transporte "Streamable HTTP"
// en /mcp, sin sesiones (cada pedido trae su token: anda igual detrás de un
// proxy y después de un reinicio), con respuestas JSON.
//
// Autenticación: un token de acceso de OAuth (oauth.go) o un token personal
// creado desde la web, en "Authorization: Bearer". Sin token, 401 con
// WWW-Authenticate apuntando a los metadatos (RFC 9728): así el cliente
// descubre solo el servidor de autorización.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	nucleo "sintetica/firmware/api/cgo"
)

const (
	llamadasPorMinutoMCP = 120 // herramientas por usuario
	pedidosPorMinutoMCP  = 300 // pedidos HTTP a /mcp por usuario (incluye tools/list, etc.)
)

// estadoMCP: el servidor (uno solo: las herramientas sacan al usuario del
// token de cada pedido) y un candado por bicho para no pisar cambios de config
// que llegan a la vez.
type estadoMCP struct {
	una      sync.Once
	manejo   http.Handler
	candados sync.Map // bicho → *sync.Mutex
}

func (e *estadoMCP) candado(bicho string) *sync.Mutex {
	m, _ := e.candados.LoadOrStore(bicho, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// quien hace el pedido (lo pone el verificador del token).
type quien struct {
	usuario string
	cliente string // el nombre del cliente o del token personal
	tipo    string // "acceso" o "personal"
}

func quienDe(r *mcp.CallToolRequest) (quien, bool) {
	if r == nil || r.Extra == nil || r.Extra.TokenInfo == nil || r.Extra.TokenInfo.UserID == "" {
		return quien{}, false
	}
	ti := r.Extra.TokenInfo
	q := quien{usuario: ti.UserID}
	q.cliente, _ = ti.Extra["cliente"].(string)
	q.tipo, _ = ti.Extra["tipo"].(string)
	return q, true
}

// autor de las versiones de config que cambia una IA.
func (q quien) autor() string {
	c := strings.TrimSpace(q.cliente)
	if c == "" {
		c = "ia"
	}
	return "mcp:" + cortarUTF8(c, 40)
}

// verificarTokenMCP: el TokenVerifier del SDK.
func (s *Servidor) verificarTokenMCP(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	if !strings.HasPrefix(token, "sin_at_") && !strings.HasPrefix(token, "sin_pat_") {
		return nil, fmt.Errorf("%w: ese token no es para el MCP", auth.ErrInvalidToken)
	}
	t, err := s.alm.TokenMCPPorHash(ctx, hash(token))
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return nil, fmt.Errorf("%w: el token venció o lo revocaron", auth.ErrInvalidToken)
	}
	if err != nil {
		s.log.Error("mcp: verificar token", "error", err)
		return nil, errors.New("algo salió mal de nuestro lado")
	}
	vence := time.Now().Add(24 * time.Hour) // los personales no vencen
	if t.Vence != nil {
		vence = *t.Vence
	}
	return &auth.TokenInfo{
		UserID: t.UsuarioID, Scopes: []string{t.Alcance}, Expiration: vence,
		Extra: map[string]any{"cliente": t.Cliente, "tipo": t.Tipo, "clienteID": t.ClienteID},
	}, nil
}

// manejadorMCP es /mcp: el token, el límite de pedidos y el transporte.
func (s *Servidor) manejadorMCP() http.Handler {
	s.mcp.una.Do(func() {
		srv := s.servidorMCP()
		transporte := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
			// La protección de localhost es contra DNS rebinding a servidores
			// locales sin autenticación; acá todo pide token, y en el VPS el
			// proxy (Caddy) llega desde 127.0.0.1 con el Host público.
			DisableLocalhostProtection: true,
			MaxRequestBodyBytes:        12 << 20, // una imagen de 8 MB en base64
		})
		limitar := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ti := auth.TokenInfoFromContext(r.Context())
			if ti != nil && !s.limites.permitir("mcp-http:"+ti.UserID, pedidosPorMinutoMCP, time.Minute) {
				w.Header().Set("Retry-After", "30")
				fallar(w, http.StatusTooManyRequests, "demasiados pedidos al MCP: esperá un poco")
				return
			}
			transporte.ServeHTTP(w, r)
		})
		conToken := auth.RequireBearerToken(s.verificarTokenMCP, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: s.urlMetadatosRecurso(),
			Scopes:              []string{alcanceMCP},
		})(limitar)
		s.mcp.manejo = cors(conToken.ServeHTTP)
	})
	return s.mcp.manejo
}

// instruccionesMCP: lo que lee la IA al conectarse.
const instruccionesMCP = `Manejás los robots de Sintética del usuario: bichos con una cara animada en su pantalla.
- Empezá con listar_robots. Con un solo robot, "robot" se puede omitir; si no, va su id o su nombre.
- Para inventar caras usá crear_expresion: devuelve una imagen de cómo queda; mirala y ajustá. Con solo_ver: true probás sin guardar. La guía de parámetros está en el recurso sintetica://guia/caras.
- Si la cara no sale con parámetros (ojitos de anime, de gato, pixel art, cualquier dibujo), dibujala con dibujar_cara: cada parte en SVG; sigue viva (parpadea, mira, reacciona). Con reposo: true queda como su cara de siempre.
- Los cambios de config (expresiones, recordatorios, personalidad, horario, modo, reflejos) se guardan y le llegan al robot apenas está conectado. Poner caras y disparar eventos necesita que esté conectado.
- Para que el robot diga algo ya: decir (hasta 60 bytes). Para que lo haga todos los días a una hora (decir el clima, despertar, cambiar de modo): rutinas. El despertador, el clima (su ubicación) y cuánto habla solo (habla) se guardan en su config.
- Hablale al usuario en español rioplatense.`

// registroLlamada: lo que una herramienta anota mientras corre (qué bicho
// tocó y si al final no cambió nada), para el registro y el evento "mcp".
type registroLlamada struct {
	bicho      string
	sinCambios bool
}

type claveRegistro struct{}

func anotarBicho(ctx context.Context, id string) {
	if r, ok := ctx.Value(claveRegistro{}).(*registroLlamada); ok {
		r.bicho = id
	}
}

func sinCambios(ctx context.Context) {
	if r, ok := ctx.Value(claveRegistro{}).(*registroLlamada); ok {
		r.sinCambios = true
	}
}

// problema: un error que se le muestra tal cual a la IA (los demás, no).
type problema struct{ msg string }

func (p problema) Error() string { return p.msg }

func problemaf(formato string, args ...any) error { return problema{fmt.Sprintf(formato, args...)} }

// esquemaDe arma un *jsonschema.Schema desde JSON (para escribirlos legibles).
func esquemaDe(texto string) *jsonschema.Schema {
	var e jsonschema.Schema
	if err := json.Unmarshal([]byte(texto), &e); err != nil {
		panic(fmt.Sprintf("esquema inválido: %v\n%s", err, texto))
	}
	return &e
}

// Qué hace una herramienta (para las anotaciones: los clientes piden
// confirmación según esto).
type clase int

const (
	lee    clase = iota // solo mira
	cambia              // cambia algo, sin borrar
	borra               // puede borrar o reemplazar algo
	afuera              // habla con el mundo de afuera (Telegram)
)

// herramienta suma una herramienta con su esquema de entrada estricto: el SDK
// valida los argumentos contra el esquema antes de llamar a f. Acá se pone el
// límite de llamadas, el registro y se convierten los errores.
func herramienta[In any](s *Servidor, srv *mcp.Server, nombre, titulo, descripcion string, esquema *jsonschema.Schema, c clase,
	f func(ctx context.Context, q quien, in In) (*mcp.CallToolResult, error)) {
	si, no := true, false
	an := &mcp.ToolAnnotations{Title: titulo, ReadOnlyHint: c == lee, OpenWorldHint: &no}
	switch c {
	case cambia:
		an.DestructiveHint = &no
	case borra:
		an.DestructiveHint = &si
	case afuera:
		an.DestructiveHint, an.OpenWorldHint = &no, &si
	}
	mcp.AddTool(srv, &mcp.Tool{Name: nombre, Title: titulo, Description: descripcion, InputSchema: esquema, Annotations: an},
		func(ctx context.Context, r *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			q, ok := quienDe(r)
			if !ok {
				return nil, nil, errors.New("sin usuario: conectá de nuevo")
			}
			if !s.limites.permitir("mcp:"+q.usuario, llamadasPorMinutoMCP, time.Minute) {
				return nil, nil, problemaf("Pasaste el límite de %d llamadas por minuto: esperá un poco y seguí.", llamadasPorMinutoMCP)
			}
			inicio := time.Now()
			reg := &registroLlamada{}
			res, err := f(context.WithValue(ctx, claveRegistro{}, reg), q, in)
			var p problema
			switch {
			case err == nil:
			case errors.As(err, &p):
			default:
				s.log.Error("mcp: error interno", "herramienta", nombre, "usuario", q.usuario, "error", err)
				err = errors.New("algo salió mal de nuestro lado; probá de nuevo en un rato")
			}
			s.log.Info("mcp", "herramienta", nombre, "usuario", q.usuario, "cliente", q.cliente, "bicho", reg.bicho, "ok", err == nil, "ms", time.Since(inicio).Milliseconds())
			// Lo que cambió algo sale por los eventos en vivo (la consola de la
			// web muestra "Claude cambió X"); los cambios mismos ya salen como
			// "config", "imagenes", etc. por el camino de siempre.
			if err == nil && c != lee && !reg.sinCambios {
				cliente := q.cliente
				if cliente == "" {
					cliente = "una IA"
				}
				s.hub.Publicar(q.usuario, eventos.Evento{Tipo: "mcp", Bicho: reg.bicho, Datos: map[string]any{
					"cliente": cliente, "herramienta": nombre, "titulo": titulo, "bicho": reg.bicho, "en": time.Now().UTC().Format(time.RFC3339)}})
			}
			return res, nil, err
		})
}

// resultado: texto (y, si hay, JSON con los datos).
func resultado(texto string, datos any) *mcp.CallToolResult {
	r := &mcp.CallToolResult{}
	if texto != "" {
		r.Content = append(r.Content, &mcp.TextContent{Text: texto})
	}
	if datos != nil {
		b, _ := json.Marshal(datos)
		r.Content = append(r.Content, &mcp.TextContent{Text: string(b)})
	}
	return r
}

func conImagen(r *mcp.CallToolResult, png []byte) *mcp.CallToolResult {
	if len(png) > 0 {
		r.Content = append(r.Content, &mcp.ImageContent{Data: png, MIMEType: "image/png"})
	}
	return r
}

// servidorMCP arma el servidor con sus herramientas y recursos.
func (s *Servidor) servidorMCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "sintetica", Title: "Sintética", Version: nucleo.VersionNucleo()},
		&mcp.ServerOptions{Instructions: instruccionesMCP})
	s.sumarHerramientas(srv)
	srv.AddResource(&mcp.Resource{
		URI: uriGuia, Name: "guia-caras", Title: "Guía para inventar caras",
		Description: "Qué hace cada parámetro del rostro, sus límites, ejemplos de las caras de fábrica y cómo dibujar caras de imagen (SVG) para la pantalla.",
		MIMEType:    "text/markdown",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uriGuia, MIMEType: "text/markdown", Text: guiaCaras()}}}, nil
	})
	srv.AddPrompt(&mcp.Prompt{
		Name: "inventar_cara", Title: "Inventar una cara",
		Description: "Diseñar una expresión nueva para un robot, mirando cómo queda e iterando.",
		Arguments:   []*mcp.PromptArgument{{Name: "idea", Description: "Qué cara querés (por ejemplo: «enojado pero tierno»)", Required: true}},
	}, func(_ context.Context, r *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		idea := r.Params.Arguments["idea"]
		return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: "Leé el recurso " + uriGuia +
			" y después inventá una expresión para mi robot: " + idea + ". Usá crear_expresion con solo_ver: true, mirá la imagen, ajustá hasta que quede linda y recién ahí guardala y mostrala."}}}}, nil
	})
	return srv
}
