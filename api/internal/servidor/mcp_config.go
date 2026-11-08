package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
	nucleo "sintetica/firmware/api/cgo"
)

// objeto es un objeto JSON que conserva el orden de sus claves (la config se
// guarda como la escribió el dueño; el núcleo lee las expresiones en orden).
type objeto struct {
	claves  []string
	valores map[string]json.RawMessage
}

func objetoVacio() *objeto { return &objeto{valores: map[string]json.RawMessage{}} }

func leerObjeto(b []byte) (*objeto, error) {
	o := objetoVacio()
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if t != json.Delim('{') {
		return nil, errors.New("no es un objeto JSON")
	}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return nil, err
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		if _, ya := o.valores[k]; !ya {
			o.claves = append(o.claves, k)
		}
		o.valores[k] = v
	}
	return o, nil
}

func (o *objeto) crudo(k string) (json.RawMessage, bool) { v, ok := o.valores[k]; return v, ok }

// leer decodifica el valor de k en v (false si no está o no encaja).
func (o *objeto) leer(k string, v any) bool {
	c, ok := o.valores[k]
	return ok && json.Unmarshal(c, v) == nil
}

// objetoEn: el valor de k como objeto ordenado (vacío si no está).
func (o *objeto) objetoEn(k string) *objeto {
	if c, ok := o.valores[k]; ok {
		if h, err := leerObjeto(c); err == nil {
			return h
		}
	}
	return objetoVacio()
}

func (o *objeto) poner(k string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // solo se ponen cosas que se pueden escribir
	}
	if _, ya := o.valores[k]; !ya {
		o.claves = append(o.claves, k)
	}
	o.valores[k] = b
}

func (o *objeto) quitar(k string) {
	if _, ok := o.valores[k]; !ok {
		return
	}
	delete(o.valores, k)
	for i, c := range o.claves {
		if c == k {
			o.claves = append(o.claves[:i], o.claves[i+1:]...)
			break
		}
	}
}

func (o *objeto) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.claves {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		if err := json.Compact(&b, o.valores[k]); err != nil {
			return nil, err
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// --- Los robots del usuario ---

// nombreRobot: el apodo, o el nombre que muestra, o su id.
func nombreRobot(v VistaBicho) string {
	if v.Apodo != "" {
		return v.Apodo
	}
	if v.Estado != nil && v.Estado.Nombre != "" {
		return v.Estado.Nombre
	}
	if v.Config != nil {
		var c struct {
			Nombre string `json:"nombre"`
		}
		if json.Unmarshal([]byte(v.Config.Texto), &c) == nil && c.Nombre != "" {
			return c.Nombre
		}
	}
	return v.ID
}

// pantallaDe: la del estado; si nunca mandó uno, la del perfil.
func pantallaDe(v VistaBicho) protocolo.Pantalla {
	if v.Estado != nil && v.Estado.Pantalla.Ancho > 0 {
		return v.Estado.Pantalla
	}
	if strings.Contains(v.Perfil, "broche") {
		return protocolo.Pantalla{Ancho: 240, Alto: 240, Redonda: true}
	}
	return protocolo.Pantalla{Ancho: 320, Alto: 172}
}

func (s *Servidor) robotsDe(ctx context.Context, usuario string) ([]VistaBicho, error) {
	l, err := s.alm.BichosDe(ctx, usuario)
	if err != nil {
		return nil, err
	}
	vs := make([]VistaBicho, 0, len(l))
	for _, b := range l {
		vs = append(vs, s.vista(ctx, b))
	}
	return vs, nil
}

var sinTildesMCP = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

func normalMCP(s string) string {
	return strings.Join(strings.Fields(sinTildesMCP.Replace(strings.ToLower(s))), " ")
}

// elegirRobot: por id o por nombre (sin importar mayúsculas ni tildes); con
// un solo robot, "" es ese. Solo los del usuario: uno ajeno es "no lo encontré".
func (s *Servidor) elegirRobot(ctx context.Context, usuario, robot string) (VistaBicho, error) {
	vs, err := s.robotsDe(ctx, usuario)
	if err != nil {
		return VistaBicho{}, err
	}
	if len(vs) == 0 {
		return VistaBicho{}, problemaf("No tenés robots en tu cuenta todavía: sumalo desde la app.")
	}
	robot = strings.TrimSpace(robot)
	if robot == "" {
		if len(vs) == 1 {
			anotarBicho(ctx, vs[0].ID)
			return vs[0], nil
		}
		return VistaBicho{}, problemaf("Tenés %d robots: decime cuál (%s).", len(vs), listaNombres(vs))
	}
	for _, v := range vs {
		if strings.EqualFold(v.ID, robot) {
			anotarBicho(ctx, v.ID)
			return v, nil
		}
	}
	n := normalMCP(robot)
	var hallados []VistaBicho
	for _, v := range vs {
		if normalMCP(nombreRobot(v)) == n || normalMCP(v.Apodo) == n {
			hallados = append(hallados, v)
		}
	}
	switch len(hallados) {
	case 1:
		anotarBicho(ctx, hallados[0].ID)
		return hallados[0], nil
	case 0:
		return VistaBicho{}, problemaf("No encontré el robot %q. Tus robots: %s.", robot, listaNombres(vs))
	}
	return VistaBicho{}, problemaf("Hay %d robots que se llaman %q: usá el id (%s).", len(hallados), robot, listaNombres(hallados))
}

func listaNombres(vs []VistaBicho) string {
	var p []string
	for _, v := range vs {
		p = append(p, fmt.Sprintf("%s (%s)", nombreRobot(v), v.ID))
	}
	return strings.Join(p, ", ")
}

// enviar un comando al robot (tiene que estar conectado).
func (s *Servidor) enviarMCP(v VistaBicho, marco []byte) error {
	err := s.nube.Enviar(v.ID, protocolo.CarComando, marco)
	if errors.Is(err, nube.ErrDesconectado) {
		return problemaf("%s no está conectado ahora: cuando se conecte, probá de nuevo.", nombreRobot(v))
	}
	return err
}

// --- La config ---

// configTexto: la última que se quiere en el bicho (o una vacía v2).
func (s *Servidor) configTexto(ctx context.Context, id string) (string, error) {
	c, err := s.alm.UltimaConfig(ctx, id)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return `{"version":2}`, nil
	}
	return c.Texto, err
}

func (s *Servidor) configObjeto(ctx context.Context, id string) (*objeto, error) {
	t, err := s.configTexto(ctx, id)
	if err != nil {
		return nil, err
	}
	o, err := leerObjeto([]byte(t))
	if err != nil {
		return nil, fmt.Errorf("la config guardada de %s no es un objeto: %w", id, err)
	}
	return o, nil
}

// validarConfig: el mismo núcleo que en el bicho. Un rechazo es un problema
// que la IA puede arreglar (con el detalle del núcleo).
func validarConfig(o *objeto) ([]byte, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	if err := nucleo.ValidarConfig(b); err != nil {
		return nil, problemaf("El robot no aceptaría ese cambio: %s.", err.Error())
	}
	return b, nil
}

// cambiarConfig lee la config, le aplica f, la valida con el núcleo y la
// guarda como versión nueva por el mismo camino que PUT /v1/bichos/{id}/config
// (autor "mcp:<cliente>", evento "config" para la web y sincronización).
func (s *Servidor) cambiarConfig(ctx context.Context, q quien, v VistaBicho, f func(o *objeto) error) (almacen.Config, error) {
	return s.cambiarConfigDe(ctx, v.ID, q.usuario, q.autor(), f)
}

// cambiarConfigDe: lo mismo con el autor que se quiera (las automatizaciones
// usan "automatizacion"). El bicho tiene que ser del usuario (no se revisa acá).
func (s *Servidor) cambiarConfigDe(ctx context.Context, bicho, usuario, autor string, f func(o *objeto) error) (almacen.Config, error) {
	m := s.mcp.candado(bicho)
	m.Lock()
	defer m.Unlock()
	o, err := s.configObjeto(ctx, bicho)
	if err != nil {
		return almacen.Config{}, err
	}
	if err := f(o); err != nil {
		return almacen.Config{}, err
	}
	b, err := validarConfig(o)
	if err != nil {
		return almacen.Config{}, err
	}
	return s.nube.GuardarConfig(ctx, bicho, string(b), autor, usuario)
}

// guardada: lo que se le cuenta a la IA después de guardar.
func (s *Servidor) guardada(v VistaBicho, c almacen.Config) string {
	if s.nube.Conectado(v.ID) {
		return fmt.Sprintf("Guardado (versión %d de la config): ya se lo estoy mandando a %s.", c.Version, nombreRobot(v))
	}
	return fmt.Sprintf("Guardado (versión %d de la config): le llega a %s cuando se conecte.", c.Version, nombreRobot(v))
}
