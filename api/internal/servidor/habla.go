package servidor

// Lo de la v1.6 en la API HTTP: que el robot diga algo (marco 0x40), el
// despertador (0x42) y el clima (Open-Meteo). Ver docs/nube.md, "Decir,
// despertador y clima".

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/clima"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

var patronExpresion = regexp.MustCompile(patronNombre)

// cambiarClaves: el camino de guardar config que usan las automatizaciones
// (acción "modo"): pone esas claves arriba de todo y lo guarda como PUT
// /v1/bichos/{id}/config, validado por el núcleo.
func (s *Servidor) cambiarClaves(ctx context.Context, bicho, usuario, autor string, claves map[string]any) error {
	_, err := s.cambiarConfigDe(ctx, bicho, usuario, autor, func(o *objeto) error {
		for k, v := range claves {
			o.poner(k, v)
		}
		return nil
	})
	return err
}

// enviarComando manda un marco al bicho y contesta {"ok": true} (409 si no
// está conectado).
func (s *Servidor) enviarComando(w http.ResponseWriter, id string, m []byte) {
	if err := s.nube.Enviar(id, protocolo.CarComando, m); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			fallar(w, http.StatusConflict, err.Error())
			return
		}
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /v1/bichos/{id}/decir {"texto", "expresion"?}: lo dice en un globo.
func (s *Servidor) decir(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Texto     string `json:"texto"`
		Expresion string `json:"expresion"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if len(p.Texto) < 1 || len(p.Texto) > protocolo.MaxDecir || !utf8.ValidString(p.Texto) || strings.TrimSpace(p.Texto) == "" {
		fallar(w, http.StatusBadRequest, "texto inválido", protocolo.ErrTextoDecir.Error())
		return
	}
	if p.Expresion != "" && !patronExpresion.MatchString(p.Expresion) {
		fallar(w, http.StatusBadRequest, "expresión inválida", "minúsculas, números y guiones, hasta 24")
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("decir:"+id, 30, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "le estás diciendo muchas cosas: esperá un poco")
		return
	}
	m, err := protocolo.Decir(p.Texto, p.Expresion)
	if err != nil {
		fallar(w, http.StatusBadRequest, "texto inválido", err.Error())
		return
	}
	s.enviarComando(w, id, m)
}

// POST /v1/bichos/{id}/alarma {"accion": "apagar"|"posponer"|"probar"|"despertar"}.
func (s *Servidor) alarma(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Accion string `json:"accion"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	m, err := protocolo.Alarma(p.Accion)
	if err != nil {
		fallar(w, http.StatusBadRequest, "acción inválida", err.Error())
		return
	}
	s.enviarComando(w, bichoDe(r).ID, m)
}

// GET /v1/clima/buscar?q=córdoba → {"lugares": [...]} (hasta 6).
func (s *Servidor) buscarLugar(w http.ResponseWriter, r *http.Request) {
	if !s.limites.permitir("clima:"+usuarioDe(r).Usuario.ID, 30, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "buscaste muchos lugares: esperá un poco")
		return
	}
	l, err := s.integ.Clima().Buscar(r.Context(), r.URL.Query().Get("q"))
	switch {
	case errors.Is(err, clima.ErrBusqueda):
		fallar(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.log.Warn("clima: no pude buscar", "error", err)
		fallar(w, http.StatusBadGateway, "no pude buscar el lugar ahora: probá en un rato")
	default:
		responder(w, http.StatusOK, map[string]any{"lugares": l})
	}
}

// GET /v1/bichos/{id}/clima → {"ubicacion": {...} | null, "clima": {...} | null}.
func (s *Servidor) climaDelBicho(w http.ResponseWriter, r *http.Request) {
	u, c := s.integ.ClimaDe(r.Context(), bichoDe(r).ID)
	responder(w, http.StatusOK, map[string]any{"ubicacion": u, "clima": c})
}
