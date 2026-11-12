package servidor

// La herramienta "listas" del MCP (almacen/listas.go; docs/nube.md, "Listas").

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
)

func (s *Servidor) sumarHerramientaListas(srv *mcp.Server) {
	herramienta(s, srv, "listas", "Listas",
		"Las listas del usuario (la del súper, pendientes o la que sea): son de su cuenta y se ven en la web, el escritorio, Telegram y charlando con el robot. "+
			"ver: sin lista, todas; con lista, esa. agregar: suma items (crea la lista si no existe; sin lista, la del súper). "+
			"tachar / destachar: marca items como hechos o no. sacar: los borra. limpiar: saca los tachados. "+
			"Los items se buscan por parecido (sin tildes, singular o plural). Topes: 30 listas, 200 items por lista, 120 bytes por item.",
		esquemaDe(`{"type":"object","additionalProperties":false,"required":["accion"],"properties":{
		 "accion":{"enum":["ver","agregar","tachar","destachar","sacar","limpiar"]},
		 "lista":{"type":"string","maxLength":40,"description":"El nombre (\"súper\", \"pendientes\"…). Sin lista: la del súper."},
		 "items":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":120}}}}`), borra, s.hListas)
}

func (s *Servidor) hListas(ctx context.Context, q quien, in struct {
	Accion string   `json:"accion"`
	Lista  string   `json:"lista"`
	Items  []string `json:"items"`
}) (*mcp.CallToolResult, error) {
	ls, err := s.alm.Listas(ctx, q.usuario)
	if err != nil {
		return nil, err
	}
	if ls == nil {
		ls = []almacen.Lista{}
	}
	if in.Accion == "ver" {
		sinCambios(ctx)
		if strings.TrimSpace(in.Lista) == "" {
			if len(ls) == 0 {
				return resultado("No tiene listas todavía.", map[string]any{"listas": ls}), nil
			}
			var l []string
			for _, x := range ls {
				l = append(l, fmt.Sprintf("%s (%d pendientes)", x.Nombre, len(x.Pendientes())))
			}
			return resultado("Sus listas: "+strings.Join(l, ", ")+".", map[string]any{"listas": ls}), nil
		}
		l := integraciones.ListaLlamada(ls, integraciones.ResolverLista(in.Lista, ls))
		if l == nil {
			return nil, problemaf("No tiene una lista «%s». Las que tiene: %s.", in.Lista, nombresListas(ls))
		}
		return resultado("La lista «"+l.Nombre+"».", l), nil
	}
	if len(in.Items) == 0 && in.Accion != "limpiar" {
		return nil, problemaf("Faltan los items.")
	}
	nombre := integraciones.ResolverLista(in.Lista, ls)
	var l *almacen.Lista
	if in.Accion == "agregar" {
		x, _, err := s.alm.AsegurarLista(ctx, q.usuario, nombre)
		if err != nil {
			return nil, errorListaMCP(err)
		}
		l = &x
	} else if l = integraciones.ListaLlamada(ls, nombre); l == nil {
		return nil, problemaf("No tiene una lista «%s». Las que tiene: %s.", nombre, nombresListas(ls))
	}
	defer s.hub.Publicar(q.usuario, eventos.Evento{Tipo: "listas"})
	switch in.Accion {
	case "agregar":
		nuevos, ya, err := s.alm.AgregarItems(ctx, q.usuario, l.ID, in.Items)
		if err != nil {
			return nil, errorListaMCP(err)
		}
		t := fmt.Sprintf("En «%s»: %d agregados", l.Nombre, len(nuevos))
		if len(ya) > 0 {
			t += fmt.Sprintf(", %d ya estaban", len(ya))
		}
		return s.listaYTexto(ctx, q.usuario, l.ID, t+".")
	case "limpiar":
		n, err := s.alm.LimpiarLista(ctx, q.usuario, l.ID, false)
		if err != nil {
			return nil, err
		}
		return s.listaYTexto(ctx, q.usuario, l.ID, fmt.Sprintf("Saqué %d tachados de «%s».", n, l.Nombre))
	case "tachar", "destachar", "sacar":
		var hechos, faltan []string
		for _, t := range in.Items {
			it, ok := integraciones.BuscarItem(*l, t)
			if !ok {
				faltan = append(faltan, t)
				continue
			}
			if in.Accion == "sacar" {
				err = s.alm.BorrarItem(ctx, q.usuario, l.ID, it.ID)
			} else {
				hecho := in.Accion == "tachar"
				_, err = s.alm.CambiarItem(ctx, q.usuario, l.ID, it.ID, &hecho, nil)
			}
			if err != nil {
				return nil, err
			}
			hechos = append(hechos, it.Texto)
		}
		t := fmt.Sprintf("En «%s», %s: %s.", l.Nombre, map[string]string{"tachar": "tachados", "destachar": "destachados", "sacar": "sacados"}[in.Accion],
			strings.Join(hechos, ", "))
		if len(hechos) == 0 {
			t = fmt.Sprintf("En «%s» no cambió nada.", l.Nombre)
		}
		if len(faltan) > 0 {
			t += " No encontré: " + strings.Join(faltan, ", ") + "."
		}
		return s.listaYTexto(ctx, q.usuario, l.ID, t)
	}
	return nil, problemaf("accion: ver, agregar, tachar, destachar, sacar o limpiar.")
}

func (s *Servidor) listaYTexto(ctx context.Context, usuario, id, texto string) (*mcp.CallToolResult, error) {
	l, err := s.alm.ListaDe(ctx, usuario, id)
	if err != nil {
		return nil, err
	}
	return resultado(texto, l), nil
}

func nombresListas(ls []almacen.Lista) string {
	if len(ls) == 0 {
		return "ninguna"
	}
	l := make([]string, len(ls))
	for k, x := range ls {
		l[k] = x.Nombre
	}
	return strings.Join(l, ", ")
}

func errorListaMCP(err error) error {
	for _, e := range []error{almacen.ErrTopeItems, almacen.ErrTopeListas, almacen.ErrTextoItem, almacen.ErrNombreLista} {
		if errors.Is(err, e) {
			return problemaf("%s", e.Error())
		}
	}
	return err
}
