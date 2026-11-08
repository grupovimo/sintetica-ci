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
	return b.String()
}

func contiene(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
