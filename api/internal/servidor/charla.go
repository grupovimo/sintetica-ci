package servidor

import (
	"errors"
	"net/http"
	"time"

	"sintetica/api/internal/integraciones"
)

// GET /v1/charla → {disponible, quedan, porDia}.
func (s *Servidor) verCharla(w http.ResponseWriter, r *http.Request) {
	if !s.integ.CharlaActiva() {
		responder(w, http.StatusOK, map[string]any{"disponible": false})
		return
	}
	responder(w, http.StatusOK, map[string]any{"disponible": true, "porDia": integraciones.CharlasPorDia,
		"quedan": s.integ.CharlasQuedan(r.Context(), usuarioDe(r).Usuario.ID)})
}

// POST /v1/bichos/{id}/charla {texto} → {texto, expresion, enviado, quedan}:
// el robot contesta (y lo dice en su pantalla si está conectado).
func (s *Servidor) charlar(w http.ResponseWriter, r *http.Request) {
	if !s.integ.CharlaActiva() {
		fallar(w, http.StatusServiceUnavailable, integraciones.ErrCharlaApagada.Error())
		return
	}
	var p struct {
		Texto string `json:"texto"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("charla:"+u, 10, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "le estás hablando muy rápido: esperá un poco")
		return
	}
	res, err := s.integ.Charlar(r.Context(), u, bichoDe(r).ID, p.Texto, "web")
	switch {
	case errors.Is(err, integraciones.ErrTextoCharla):
		fallar(w, http.StatusBadRequest, "texto inválido", err.Error())
	case errors.Is(err, integraciones.ErrTopeCharla):
		fallar(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		s.log.Warn("charla", "error", err)
		fallar(w, http.StatusBadGateway, "no pude hablar con la IA ahora: probá en un rato")
	default:
		responder(w, http.StatusOK, res)
	}
}
