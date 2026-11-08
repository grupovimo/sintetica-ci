package servidor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	nucleo "sintetica/firmware/api/cgo"
	"sintetica/shared/formato"
)

const uriGuia = "sintetica://guia/caras"

// Las claves de una expresión paramétrica que la IA no toca (las usa el
// sistema: el número del protocolo y los disparadores).
var clavesInternasExpresion = map[string]bool{"nombre": true, "numero": true, "disparador": true}

var (
	unaVez       sync.Once
	propsCara    map[string]any // las propiedades de una expresión paramétrica, con el ojo adentro
	ordenGlobals []string       // los parámetros globales, en el orden del schema
)

// propiedadesCara saca las propiedades de "expresionVida" del schema de la
// config (shared/formato/config.schema.json), con las referencias resueltas:
// los límites de la herramienta son siempre los del schema.
func propiedadesCara() (map[string]any, []string) {
	unaVez.Do(func() {
		var s struct {
			Defs map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal(formato.Schema(), &s); err != nil {
			panic(err)
		}
		var ojo, nombre map[string]any
		var vida struct {
			Properties map[string]map[string]any `json:"properties"`
		}
		json.Unmarshal(s.Defs["ojo"], &ojo)                //nolint:errcheck
		json.Unmarshal(s.Defs["nombreExpresion"], &nombre) //nolint:errcheck
		json.Unmarshal(s.Defs["expresionVida"], &vida)     //nolint:errcheck
		// El orden en que aparecen en el schema (para la guía).
		var crudo struct {
			Properties json.RawMessage `json:"properties"`
		}
		json.Unmarshal(s.Defs["expresionVida"], &crudo) //nolint:errcheck
		if o, err := leerObjeto(crudo.Properties); err == nil {
			for _, k := range o.claves {
				if !clavesInternasExpresion[k] && k != "ojo" && k != "izq" && k != "der" && k != "extiende" &&
					k != "movimiento" && k != "particula" && k != "parpadeo" {
					ordenGlobals = append(ordenGlobals, k)
				}
			}
		}
		propsCara = map[string]any{}
		for k, v := range vida.Properties {
			if clavesInternasExpresion[k] {
				continue
			}
			if ref, _ := v["$ref"].(string); ref != "" {
				var dest map[string]any
				switch ref {
				case "#/$defs/ojo":
					dest = copiarMapa(ojo)
				case "#/$defs/nombreExpresion":
					dest = copiarMapa(nombre)
				default:
					continue
				}
				for kk, vv := range v {
					if kk != "$ref" {
						dest[kk] = vv
					}
				}
				v = dest
			}
			propsCara[k] = v
		}
		ponerDescripcion := func(k, d string) {
			if m, ok := propsCara[k].(map[string]any); ok {
				m["description"] = d
			}
		}
		ponerDescripcion("ojo", "Los dos ojos (lo que no pongas queda como en la neutral).")
		ponerDescripcion("izq", "Solo el ojo izquierdo (encima de \"ojo\").")
		ponerDescripcion("der", "Solo el ojo derecho (encima de \"ojo\").")
		ponerDescripcion("extiende", "Arranca de otra expresión (de fábrica o propia) en vez de la neutral.")
		ponerDescripcion("movimiento", "Cómo se mueve la cara mientras la muestra (no se ve en la imagen quieta).")
		ponerDescripcion("particula", "Lo que va soltando (en la imagen se ve una de muestra).")
		ponerDescripcion("parpadeo", "Cómo parpadea con esta cara.")
	})
	return propsCara, ordenGlobals
}

func copiarMapa(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func limites(p map[string]any) string {
	mi, okMi := p["minimum"].(float64)
	ma, okMa := p["maximum"].(float64)
	if okMi && okMa {
		return fmt.Sprintf("%g a %g", mi, ma)
	}
	if e, ok := p["enum"].([]any); ok {
		var l []string
		for _, x := range e {
			l = append(l, fmt.Sprint(x))
		}
		return strings.Join(l, ", ")
	}
	return ""
}

// guiaCaras: el recurso sintetica://guia/caras.
func guiaCaras() string {
	props, globales := propiedadesCara()
	var b strings.Builder
	b.WriteString(`# Cómo inventar caras para un robot de Sintética

La cara es **paramétrica**: dos ojos (pastillas con párpados), una boca opcional, cachetes y partículas, dibujados por el mismo núcleo que corre en el robot. Una expresión dice solo **lo que cambia respecto de la neutral** (o de la que "extiende"). Los cambios de cara siempre pasan por resortes: nunca hay un corte.

Coordenadas: el centro de la pantalla es (0, 0) y hacia arriba es positivo. La unidad es cerca de la mitad del ancho de la pantalla.

## Los ojos (` + "`ojo`" + ` para los dos, ` + "`izq`" + ` / ` + "`der`" + ` para uno)

| Parámetro | Rango | Qué hace |
| --- | --- | --- |
`)
	ojo, _ := props["ojo"].(map[string]any)
	op, _ := ojo["properties"].(map[string]any)
	ordenOjo := []string{"w", "h", "r", "tilt", "lt", "lta", "lb", "lba", "smile", "heart", "star", "cross", "ring", "pupil", "squash"}
	for k := range op {
		if !contiene(ordenOjo, k) {
			ordenOjo = append(ordenOjo, k)
		}
	}
	for _, k := range ordenOjo {
		p, ok := op[k].(map[string]any)
		if !ok {
			continue
		}
		d, _ := p["description"].(string)
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", k, limites(p), d)
	}
	b.WriteString("\nNeutral: w 0,105 · h 0,23 · r 1 (píldora) · sep 0,3 · sin párpados. `smile`, `heart`, `star`, `cross` y `ring` transforman la forma del ojo (de 0 a 1, mezclables pero conviene uno solo).\n\n## Toda la cara\n\n| Parámetro | Rango | Qué hace |\n| --- | --- | --- |\n")
	for _, k := range globales {
		p, ok := props[k].(map[string]any)
		if !ok {
			continue
		}
		d, _ := p["description"].(string)
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", k, limites(p), d)
	}
	for _, k := range []string{"movimiento", "particula", "parpadeo"} {
		if p, ok := props[k].(map[string]any); ok {
			d, _ := p["description"].(string)
			fmt.Fprintf(&b, "\n**`%s`**: %s Opciones: %s.\n", k, d, limites(p))
		}
	}
	b.WriteString(`
## Consejos

- Cambiá pocas cosas: 3 a 6 parámetros bien elegidos se leen mejor que 15.
- Los párpados (` + "`lt`, `lta`, `lb`" + `) dan la emoción: ` + "`lta`" + ` positivo = enojo, negativo = tristeza.
- La mirada (` + "`gx`, `gy`" + `) y la inclinación (` + "`ftilt`" + `) dan intención. La boca (` + "`mo`, `mw`, `mcurve`" + `) es opcional: sin ` + "`mo`" + ` no se abre.
- ` + "`hue`" + ` lleva el naranja al rojo (enojo); ` + "`dim`" + ` apaga (cansancio); ` + "`blush`" + ` suma cachetes (vergüenza, ternura).
- Con ` + "`crear_expresion`" + ` y ` + "`solo_ver: true`" + ` ves la imagen sin guardar: iterá mirándola. Después guardala y, si querés, mostrala (` + "`mostrar: true`" + `).
- Los nombres son ` + "`[a-z][a-z0-9-]`" + ` (hasta 24, sin tildes): ` + "`contenta-tierna`" + `. Usar el nombre de una de fábrica la reemplaza.
- Entran pocas propias (el robot tiene lugar para 48 expresiones paramétricas, contando las de fábrica): si no entra, borrá alguna con ` + "`borrar_expresion`" + `.

## Ejemplos (las de fábrica)

`)
	elegidas := map[string]bool{"contento": true, "enojado": true, "triste": true, "guino": true, "enamorado": true, "sospecha": true, "pensando": true, "verguenza": true}
	base := nucleo.ExpresionesBase()
	sort.SliceStable(base, func(i, j int) bool { return elegidas[base[i].Nombre] && !elegidas[base[j].Nombre] })
	for _, e := range base {
		if elegidas[e.Nombre] {
			fmt.Fprintf(&b, "- `%s`: `%s`\n", e.Nombre, e.JSON)
		}
	}
	b.WriteString("\nTodas las de fábrica, con sus parámetros: herramienta `listar_expresiones`.\n")
	b.WriteString(guiaImagen)
	return b.String()
}

// guiaImagen: cómo dibujar una cara de imagen (dibujar_cara).
const guiaImagen = `
## Caras de imagen (dibujar_cara): la cara que quieras

Si la cara no sale con parámetros (ojitos de anime, de gato, de pixel art, un visor de robot, corazones…), dibujala: ` + "`dibujar_cara`" + ` recibe cada parte como **SVG** (lo mejor), una URL https o base64 (PNG, JPEG o GIF) y el robot la anima **viva**, igual que a sus caras: parpadea, mira para los costados, respira, tiembla, se inclina, suelta partículas, se apaga de noche y entra y sale con un fundido (nunca un corte). Las otras expresiones la siguen moviendo: sorpresa agranda los ojos, dormido los cierra, hablar abre la boca.

Partes (solo ` + "`ojo`" + ` es obligatoria):

| Parte | Qué es |
| --- | --- |
| ` + "`ojo`" + ` | El ojo **izquierdo abierto, mirando al frente**. El derecho es este espejado (salvo que mandes ` + "`ojo_der`" + ` o ` + "`espejar_der: false`" + `). |
| ` + "`ojo_cerrado`" + ` | El ojo cerrado (una rayita, un arco ^ ^): para parpadear y dormir. Sin él, el parpadeo aplasta el ojo. |
| ` + "`boca`" + ` | La boca (se estira y se abre cuando habla). Opcional. |
| ` + "`ojo_der`" + `, ` + "`ojo_der_cerrado`" + ` | Solo si el derecho no es el izquierdo espejado (un guiño, ojos distintos). |

Cómo dibujar para su pantalla:

- **Fondo transparente** (o negro): la pantalla es casi negra y todo lo oscuro se funde con ella. No dibujes el fondo ni la cara: solo los ojos y la boca sueltos.
- **Colores que brillan**: la pantalla es de noche. Celestes, rosas, violetas, naranja (#FF6A33, el de la marca), blanco para los brillos. Los oscuros (pupila, contorno) se ven como fondo: sirven para recortar.
- **El ojo izquierdo** (el de la izquierda de la pantalla): las pestañas y los detalles de afuera van a la izquierda; el derecho sale espejado.
- **Mirando al frente**: el robot mueve los ojos solo (con la mirada, ` + "`gx`/`gy`" + `); no dibujes la pupila corrida. Tampoco hace falta animar nada.
- **Todas las partes con el mismo viewBox y a la misma escala**: se recortan a lo dibujado y la boca y el ojo cerrado quedan del tamaño que tenían al lado del ojo. El más grande se achica a 128 × 128 px: con formas simples y trazos de 4 o más (en un viewBox de 100) se ve nítido.
- Formas que se rasterizan bien: ` + "`path`, `ellipse`, `circle`, `rect`, `polygon`" + `, colores sólidos y gradientes lineales o radiales. Sin texto, filtros, máscaras ni imágenes adentro.
- ` + "`tamano`" + ` es el ancho de cada ojo (0,36 por defecto; 0,45 a 0,55 para ojos grandes de anime), ` + "`sep`" + ` la separación (0,15 a 0,45), ` + "`ey`" + ` la altura y ` + "`boca_y`" + ` dónde va la boca (-0,36). También valen ` + "`movimiento`, `particula`, `parpadeo`, `blush`, `ftilt`" + `.
- Usá ` + "`solo_ver: true`" + ` y mirá las dos imágenes (abierta y parpadeando). Con ` + "`reposo: true`" + ` queda como su cara de siempre; ` + "`poner_cara`" + ` con ` + "`siempre: true`" + ` elige otra (o la neutral con ` + "`soltar`" + `).
- Hay lugar para 12 sprites (un ojo, un ojo cerrado y una boca son 3) y 8 caras de imagen; ` + "`borrar_expresion`" + ` libera los suyos.

### Ejemplo: ojitos de chica de anime

` + "`ojo`" + `:

` + "```svg" + `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <defs>
    <linearGradient id="iris" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#3b2bff"/><stop offset="0.6" stop-color="#9b6bff"/><stop offset="1" stop-color="#ff9be8"/>
    </linearGradient>
  </defs>
  <path d="M12 34 Q50 14 88 30" stroke="#f5f2ec" stroke-width="6" fill="none" stroke-linecap="round"/>
  <path d="M20 30 L8 24" stroke="#f5f2ec" stroke-width="4" stroke-linecap="round"/>
  <ellipse cx="50" cy="58" rx="27" ry="33" fill="url(#iris)"/>
  <ellipse cx="50" cy="62" rx="12" ry="17" fill="#120a2a"/>
  <ellipse cx="40" cy="46" rx="9" ry="11" fill="#ffffff"/>
  <circle cx="60" cy="74" r="5" fill="#ffffff" opacity="0.85"/>
</svg>
` + "```" + `

` + "`ojo_cerrado`" + ` (un arco feliz, en el mismo viewBox):

` + "```svg" + `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <path d="M18 62 Q50 40 82 62" stroke="#f5f2ec" stroke-width="7" fill="none" stroke-linecap="round"/>
  <path d="M24 56 L10 50" stroke="#f5f2ec" stroke-width="4" stroke-linecap="round"/>
</svg>
` + "```" + `

` + "`boca`" + ` (chiquita, como en el anime):

` + "```svg" + `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <path d="M40 50 Q50 60 60 50" stroke="#ff8fb8" stroke-width="5" fill="none" stroke-linecap="round"/>
</svg>
` + "```" + `

Con ` + "`tamano: 0.46`, `sep: 0.3`, `boca_y: -0.42`, `blush: 0.5`" + ` y ` + "`reposo: true`" + ` queda una carita de anime que vive todo el día.

Otras ideas: ojos de gato (una almendra verde con la pupila vertical), un visor de robot (una franja cian con líneas), corazones (` + "`ojo`" + ` un corazón rosa, ` + "`ojo_cerrado`" + ` una rayita), pixel art (rectángulos de 10 × 10).
`

func contiene(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
