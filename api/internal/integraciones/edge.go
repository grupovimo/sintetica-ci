package integraciones

// La voz gratis: Edge TTS (las voces neuronales de Microsoft Edge, sin clave;
// api/internal/edgetts). Prendida por defecto para todos, con una voz de
// Argentina; cada usuario la puede apagar o elegir otra es-*. Es un servicio
// NO oficial: si falla, el robot usa su voz propia, con un log y nada más.
// Tope por usuario y por día, y pocas a la vez, para no abusar.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/edgetts"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
)

// Lo de Edge.
const (
	TopeEdgePorDefecto = 3000 // caracteres por usuario por día (EDGE_TTS_TOPE)
	EdgeALaVez         = 4
)

// Las voces por si no se puede leer la lista.
var vocesEdgeFijas = []edgetts.Voz{
	{Nombre: "es-AR-ElenaNeural", Titulo: "Elena Online (Natural)", Idioma: "es-AR", Genero: "Female"},
	{Nombre: "es-AR-TomasNeural", Titulo: "Tomas Online (Natural)", Idioma: "es-AR", Genero: "Male"},
}

// ErrEdgeApagado: el servidor la apagó (EDGE_TTS=0).
var ErrEdgeApagado = errors.New("la voz gratis está apagada en este servidor")

// ErrTopeEdge: ya leyó lo del día.
var ErrTopeEdge = errors.New("por hoy ya se leyó mucho con la voz gratis: mañana sigue")

type cacheEdge struct {
	v  almacen.VozEdge
	en time.Time
}

func (i *Integraciones) iniciarEdge() {
	if i.op.V19.EdgeApagado {
		return
	}
	c := edgetts.Nuevo()
	if u := i.op.V19.EdgeURL; u != "" {
		c.URLWS = u
	}
	if u := i.op.V19.EdgeVocesURL; u != "" {
		c.URLVoces = u
	}
	i.voz.edge = c
	i.voz.edgeSem = make(chan struct{}, EdgeALaVez)
}

// EdgeActivo: el servidor tiene la voz gratis.
func (i *Integraciones) EdgeActivo() bool { return i.voz.edge != nil }

func (i *Integraciones) topeEdge() int {
	if t := i.op.V19.EdgeTope; t > 0 {
		return t
	}
	return TopeEdgePorDefecto
}

// vozEdgeDe: lo del usuario (cache de 30 s).
func (i *Integraciones) vozEdgeDe(ctx context.Context, usuario string) (almacen.VozEdge, error) {
	i.voz.mu.Lock()
	c, ok := i.voz.edgeUsuarios[usuario]
	i.voz.mu.Unlock()
	if ok && i.ahora().Sub(c.en) < 30*time.Second {
		return c.v, nil
	}
	v, err := i.alm.VozEdgeDe(ctx, usuario)
	if err != nil {
		return v, err
	}
	if !edgetts.VozValida(v.Voz) {
		v.Voz = edgetts.VozPorDefecto
	}
	i.voz.mu.Lock()
	if len(i.voz.edgeUsuarios) > 5000 {
		i.voz.edgeUsuarios = map[string]cacheEdge{}
	}
	i.voz.edgeUsuarios[usuario] = cacheEdge{v: v, en: i.ahora()}
	i.voz.mu.Unlock()
	return v, nil
}

// hablarEdge: el mp3, con el tope del día y pocos a la vez.
func (i *Integraciones) hablarEdge(ctx context.Context, usuario, voz, texto string) ([]byte, error) {
	if i.voz.edge == nil {
		return nil, ErrEdgeApagado
	}
	n := utf8.RuneCountInString(texto)
	if _, ok, err := i.alm.UsarCaracteresEdge(ctx, usuario, i.ahora().In(nube.Zona()), n, i.topeEdge()); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrTopeEdge
	}
	select {
	case i.voz.edgeSem <- struct{}{}:
		defer func() { <-i.voz.edgeSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return i.voz.edge.Hablar(ctx, voz, texto)
}

// vocesEdge: las es-* (la lista se lee cada 6 h; si falla, las de Argentina).
func (i *Integraciones) vocesEdge(ctx context.Context) []edgetts.Voz {
	i.voz.mu.Lock()
	l, en := i.voz.edgeVoces, i.voz.edgeVocesEn
	i.voz.mu.Unlock()
	if len(l) > 0 && i.ahora().Sub(en) < 6*time.Hour {
		return l
	}
	if i.voz.edge == nil {
		return vocesEdgeFijas
	}
	c, cancelar := context.WithTimeout(ctx, 8*time.Second)
	defer cancelar()
	l, err := i.voz.edge.Voces(c, "es-")
	if err != nil || len(l) == 0 {
		i.log.Info("edge-tts: no pude leer las voces", "error", err)
		return vocesEdgeFijas
	}
	i.voz.mu.Lock()
	i.voz.edgeVoces, i.voz.edgeVocesEn = l, i.ahora()
	i.voz.mu.Unlock()
	return l
}

// CambiosEdge: PUT /v1/voz/edge (nil = igual).
type CambiosEdge struct {
	Prendida *bool   `json:"prendida"`
	Voz      *string `json:"voz"`
}

// CambiarEdge prende o apaga la voz gratis o elige otra es-*.
func (i *Integraciones) CambiarEdge(ctx context.Context, usuario string, c CambiosEdge) error {
	if c.Voz != nil {
		v := strings.TrimSpace(*c.Voz)
		if !edgetts.VozValida(v) || !strings.HasPrefix(v, "es-") {
			return ErrCambioVoz{"la voz es una de las es-* (por ejemplo " + edgetts.VozPorDefecto + ")"}
		}
		ok := false
		for _, x := range i.vocesEdge(ctx) {
			ok = ok || x.Nombre == v
		}
		if !ok {
			return ErrCambioVoz{"esa voz no está en la lista"}
		}
		c.Voz = &v
	}
	if err := i.alm.GuardarVozEdge(ctx, usuario, c.Prendida, c.Voz); err != nil {
		return err
	}
	i.voz.mu.Lock()
	delete(i.voz.edgeUsuarios, usuario)
	i.voz.mu.Unlock()
	i.avisarVozATodos(ctx, usuario)
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "voz", Datos: map[string]any{"edge": true}})
	return nil
}

// EstadoEdge: GET /v1/voz/edge.
func (i *Integraciones) EstadoEdge(ctx context.Context, usuario string, conVoces bool) (map[string]any, error) {
	if !i.EdgeActivo() {
		return map[string]any{"disponible": false}, nil
	}
	v, err := i.vozEdgeDe(ctx, usuario)
	if err != nil {
		return nil, err
	}
	usados, err := i.alm.CaracteresEdgeDeHoy(ctx, usuario, i.ahora().In(nube.Zona()))
	if err != nil {
		return nil, err
	}
	r := map[string]any{"disponible": true, "prendida": v.Prendida, "voz": v.Voz, "usados_hoy": usados, "tope": i.topeEdge()}
	if conVoces {
		r["voces"] = i.vocesEdge(ctx)
	}
	return r, nil
}

// ProbarEdge: el mp3 de la voz gratis (cuenta para el tope).
func (i *Integraciones) ProbarEdge(ctx context.Context, usuario, texto string) ([]byte, error) {
	if !i.EdgeActivo() {
		return nil, ErrEdgeApagado
	}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		texto = "¡Hola! Así suena mi voz."
	}
	if len(texto) > MaxTextoProbar || !utf8.ValidString(texto) {
		return nil, ErrCambioVoz{fmt.Sprintf("el texto es de hasta %d bytes", MaxTextoProbar)}
	}
	v, err := i.vozEdgeDe(ctx, usuario)
	if err != nil {
		return nil, err
	}
	ctx, cancelar := context.WithTimeout(ctx, 12*time.Second)
	defer cancelar()
	a, err := i.hablarEdge(ctx, usuario, v.Voz, texto)
	if err != nil && !errors.Is(err, ErrTopeEdge) {
		i.log.Info("edge-tts: no leyó", "error", err)
		return nil, ErrEdgeCaido
	}
	return a, err
}

// ErrEdgeCaido: el servicio de Edge no contestó (no es oficial).
var ErrEdgeCaido = errors.New("la voz gratis no contestó: es un servicio de Microsoft que a veces cambia; probá en un rato")
