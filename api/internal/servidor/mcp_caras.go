package servidor

// dibujar_cara: la IA dibuja una cara de imagen (docs/motor-de-vida.md,
// "Caras de imagen"). Cada parte (ojo, ojo cerrado, boca…) llega como SVG,
// URL o base64; se convierte en sprites (QOI con transparencia, a la misma
// escala), se guardan en lugares libres de los 12 y se crea la expresión con
// "sprites". El núcleo devuelve cómo queda, abierta y parpadeando.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/imagenes"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/protocolo"
	nucleo "sintetica/firmware/api/cgo"
)

// Las partes, en el orden del núcleo (ParteSprite).
var partesCara = []string{"ojo", "ojo_cerrado", "ojo_der", "ojo_der_cerrado", "boca"}

func esquemaDibujarCara() *jsonschema.Schema {
	props, _ := propiedadesCara()
	parte := func(d string) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": 11500000,
			"description": d + " Un SVG (\"<svg …>\": lo mejor), una URL https o una imagen en base64 (PNG, JPEG o GIF). Fondo transparente o negro; se recorta a lo dibujado."}
	}
	p := map[string]any{
		"nombre": map[string]any{"type": "string", "pattern": patronNombre,
			"description": "Nombre de la cara: minúsculas, números y guiones (\"ojos-anime\"). Si ya existe, se reemplaza."},
		"ojo":             parte("El ojo izquierdo abierto, mirando al frente (el derecho es este espejado, si no mandás ojo_der)."),
		"ojo_cerrado":     parte("El ojo cerrado (para parpadear y dormir). Si no está, el parpadeo aplasta el ojo."),
		"ojo_der":         parte("Opcional: el ojo derecho, si no es el izquierdo espejado."),
		"ojo_der_cerrado": parte("Opcional: el derecho cerrado."),
		"boca":            parte("Opcional: la boca (se abre cuando habla)."),
		"tamano": map[string]any{"type": "number", "minimum": 0.08, "maximum": 0.8,
			"description": "El ancho de cada ojo en unidades del rostro (≈ media pantalla de ancho = 1). 0,36 si no se dice; 0,45 son ojos grandes de anime."},
		"boca_tamano": map[string]any{"type": "number", "minimum": 0.04, "maximum": 0.9, "description": "El ancho de la boca (si no, a la escala en que la dibujaste al lado del ojo)."},
		"boca_y":      map[string]any{"type": "number", "minimum": -0.7, "maximum": 0.3, "description": "La altura de la boca (-0,36 si no se dice; 0 es el centro, arriba es positivo)."},
		"espejar_der": map[string]any{"type": "boolean", "description": "Sin ojo_der: false para no espejar el izquierdo."},
		"reposo":      map[string]any{"type": "boolean", "description": "true: que sea su cara de siempre (en vez de la neutral). Las reacciones y los comportamientos la siguen moviendo."},
		"mostrar":     map[string]any{"type": "boolean", "description": "true: además se la pone fija ya (poner_cara con soltar la saca)."},
		"solo_ver":    map[string]any{"type": "boolean", "description": "true: solo la dibuja (no guarda nada): para iterar."},
		"movimiento":  props["movimiento"],
		"particula":   props["particula"],
		"parpadeo":    props["parpadeo"],
	}
	for _, k := range []string{"sep", "ey", "gx", "gy", "ftilt", "blush", "dim", "hue", "glitch"} {
		if v, ok := props[k]; ok {
			p[k] = v
		}
	}
	var robot map[string]any
	json.Unmarshal([]byte(`{`+propRobot+`}`), &robot) //nolint:errcheck
	p["robot"] = robot["robot"]
	b, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"nombre", "ojo"}, "properties": p})
	return esquemaDe(string(b))
}

// bytesDeParte: un SVG, una URL (con el anti-SSRF de las integraciones) o base64.
func (s *Servidor) bytesDeParte(ctx context.Context, nombre, v string) ([]byte, error) {
	t := strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(t, "<"):
		if len(t) > imagenes.MaxSvg {
			return nil, problemaf("%s: el SVG pesa más de %d KB.", nombre, imagenes.MaxSvg>>10)
		}
		return []byte(t), nil
	case strings.HasPrefix(t, "https://") || strings.HasPrefix(t, "http://"):
		ctxB, cancelar := context.WithTimeout(ctx, integraciones.TimeoutBajar)
		defer cancelar()
		d, _, err := s.integ.Bajar(ctxB, t, imagenes.MaxEntrada)
		if err != nil {
			return nil, problemaf("%s: no pude bajar la imagen: %s.", nombre, err.Error())
		}
		return d, nil
	}
	if i := strings.Index(t, ";base64,"); strings.HasPrefix(t, "data:") && i > 0 {
		t = t[i+len(";base64,"):]
	}
	d, err := base64.StdEncoding.DecodeString(t)
	if err != nil {
		if d, err = base64.RawStdEncoding.DecodeString(t); err != nil {
			return nil, problemaf("%s: no es un SVG, una URL ni base64 válido.", nombre)
		}
	}
	return d, nil
}

// spritesUsados: qué sprites usa cada expresión de imagen de la config.
func spritesUsados(o *objeto) map[string]map[string]int {
	out := map[string]map[string]int{}
	ex := o.objetoEn("expresiones")
	for _, k := range ex.claves {
		var e struct {
			Sprites map[string]int `json:"sprites"`
		}
		if c, ok := ex.crudo(k); ok && json.Unmarshal(c, &e) == nil && len(e.Sprites) > 0 {
			out[k] = e.Sprites
		}
	}
	return out
}

// lugares elige un sprite para cada parte: los que ya tenía esta cara, y si
// no, los que no usa ninguna otra.
func lugares(usados map[string]map[string]int, nombre string, partes []string) (map[string]int, error) {
	ocupado := map[int]bool{}
	for k, m := range usados {
		if k == nombre {
			continue
		}
		for _, n := range m {
			ocupado[n] = true
		}
	}
	elegidos := map[string]int{}
	tomado := map[int]bool{}
	for _, p := range partes {
		if n, ok := usados[nombre][p]; ok && !ocupado[n] && !tomado[n] && n >= 0 && n < protocolo.MaxSprites {
			elegidos[p], tomado[n] = n, true
		}
	}
	for _, p := range partes {
		if _, ok := elegidos[p]; ok {
			continue
		}
		n := 0
		for n < protocolo.MaxSprites && (ocupado[n] || tomado[n]) {
			n++
		}
		if n == protocolo.MaxSprites {
			return nil, problemaf("No hay lugar: el robot guarda %d sprites y los usan otras caras de imagen. Borrá una con borrar_expresion.", protocolo.MaxSprites)
		}
		elegidos[p], tomado[n] = n, true
	}
	return elegidos, nil
}

func (s *Servidor) hDibujarCara(ctx context.Context, q quien, in map[string]any) (*mcp.CallToolResult, error) {
	robot, _ := in["robot"].(string)
	nombre, _ := in["nombre"].(string)
	soloVer, _ := in["solo_ver"].(bool)
	mostrar, _ := in["mostrar"].(bool)
	reposo, _ := in["reposo"].(bool)
	v, err := s.elegirRobot(ctx, q.usuario, robot)
	if err != nil {
		return nil, err
	}
	var partes []string
	var datos [][]byte
	for _, p := range partesCara {
		t, _ := in[p].(string)
		if t == "" {
			continue
		}
		d, err := s.bytesDeParte(ctx, p, t)
		if err != nil {
			return nil, err
		}
		partes, datos = append(partes, p), append(datos, d)
	}
	if !slices.Contains(partes, "ojo") {
		return nil, problemaf("Falta el ojo: es la única parte obligatoria.")
	}
	sprites, err := imagenes.PrepararPartes(datos)
	if err != nil {
		return nil, problemaf("No pude convertir los dibujos: %s.", err.Error())
	}
	o, err := s.configObjeto(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	usados := spritesUsados(o)
	elegidos, err := lugares(usados, nombre, partes)
	if err != nil {
		return nil, err
	}
	// La expresión: los sprites, la geometría y lo que se pida de la cara.
	spr := map[string]int{}
	porNumero := map[int][]byte{}
	for i, p := range partes {
		spr[p] = elegidos[p]
		porNumero[elegidos[p]] = sprites[i].Datos
	}
	expr := objetoVacio()
	expr.poner("sprites", spr)
	for _, k := range []string{"tamano", "boca_tamano", "boca_y", "espejar_der", "sep", "ey", "gx", "gy", "ftilt", "blush", "dim", "hue", "glitch", "movimiento", "particula", "parpadeo"} {
		if x, ok := in[k]; ok {
			expr.poner(k, x)
		}
	}
	aplicar := func(o *objeto) error {
		ex := o.objetoEn("expresiones")
		ex.poner(nombre, expr)
		o.poner("expresiones", ex)
		if _, ok := o.crudo("version"); !ok {
			o.poner("version", 2)
		}
		if reposo {
			c := o.objetoEn("cara")
			c.poner("expresion", nombre)
			o.poner("cara", c)
		}
		return nil
	}
	pant := pantallaDe(v)
	dibujar := func(config []byte) ([]byte, []byte, error) {
		w, h := pant.Ancho, pant.Alto
		if w*2 <= 640 && h*2 <= 640 {
			w, h = w*2, h*2
		}
		un := func(cierre float32) ([]byte, error) {
			px, err := nucleo.DibujarCara(config, nombre, porNumero, w, h, pant.Redonda, 0.6, cierre)
			if err != nil {
				return nil, problemaf("El robot no aceptaría esa cara: %s.", err.Error())
			}
			return imagenes.PNG(imagenes.DesdeRGBA(px, w, h)), nil
		}
		a, err := un(0)
		if err != nil {
			return nil, nil, err
		}
		c, err := un(1)
		return a, c, err
	}
	tam := make([]string, len(partes))
	for i, p := range partes {
		tam[i] = fmt.Sprintf("%s %d × %d", p, sprites[i].Ancho, sprites[i].Alto)
	}
	if soloVer {
		sinCambios(ctx)
		aplicar(o) //nolint:errcheck
		b, err := validarConfig(o)
		if err != nil {
			return nil, err
		}
		a, c, err := dibujar(b)
		if err != nil {
			return nil, err
		}
		return conImagen(conImagen(resultado(fmt.Sprintf("Así quedaría «%s» en %s (abierta y parpadeando; sprites: %s). No guardé nada: si te gusta, llamá de nuevo sin solo_ver.",
			nombre, nombreRobot(v), strings.Join(tam, ", ")), nil), a), c), nil
	}
	// Primero los sprites (si la config llega antes, el robot muestra su cara
	// de siempre hasta que lleguen y después entra la nueva con un fundido).
	for i, p := range partes {
		if err := s.nube.GuardarSprite(ctx, v.ID, elegidos[p], sprites[i].Datos); err != nil {
			return nil, err
		}
	}
	c, err := s.cambiarConfig(ctx, q, v, aplicar)
	if err != nil {
		return nil, err
	}
	// Los sprites que esta cara usaba antes y ya no usa nadie, se borran.
	s.borrarSpritesSueltos(ctx, v.ID, usados[nombre], mustObjeto(c.Texto))
	a, ce, err := dibujar([]byte(c.Texto))
	if err != nil {
		return nil, err
	}
	texto := fmt.Sprintf("Guardé la cara de imagen «%s» (sprites: %s). %s", nombre, strings.Join(tam, ", "), s.guardada(v, c))
	if reposo {
		texto += " Es su cara de siempre: parpadea, mira, respira y reacciona con ella."
	}
	if mostrar {
		if err := s.enviarMCP(v, protocolo.Expresion(nombre)); err != nil {
			texto += " No se la pude poner ya: " + err.Error()
		} else {
			texto += " Se la puse fija (poner_cara con soltar la saca)."
		}
	}
	return conImagen(conImagen(resultado(texto, map[string]any{"sprites": spr, "expresion": json.RawMessage(mustJSON(expr))}), a), ce), nil
}

func mustObjeto(t string) *objeto {
	o, err := leerObjeto([]byte(t))
	if err != nil {
		return objetoVacio()
	}
	return o
}

func mustJSON(o *objeto) []byte {
	b, _ := json.Marshal(o)
	return b
}

// borrarSpritesSueltos borra de `antes` los sprites que la config ya no usa.
func (s *Servidor) borrarSpritesSueltos(ctx context.Context, id string, antes map[string]int, o *objeto) {
	enUso := map[int]bool{}
	for _, m := range spritesUsados(o) {
		for _, n := range m {
			enUso[n] = true
		}
	}
	for _, n := range antes {
		if !enUso[n] {
			s.nube.BorrarSprite(ctx, id, n) //nolint:errcheck
		}
	}
}
