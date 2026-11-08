package servidor

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// GET /v1/bichos/{id}/memoria: lo que recuerda (el diario), la última vez que
// lo mandó, y si tiene una mudanza pendiente.
func (s *Servidor) verMemoria(w http.ResponseWriter, r *http.Request) {
	b := bichoDe(r)
	m, err := s.alm.LeerMemoria(r.Context(), b.ID)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		responder(w, http.StatusOK, map[string]any{"memoria": nil, "actualizada": nil, "mudanzaPendiente": false})
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	var dec *protocolo.Memoria
	if m.Datos != nil {
		if dec, _ = protocolo.LeerMemoria(m.Datos); dec != nil {
			dec.AlDia(time.Now().In(nube.Zona())) // lo "de hoy" de otro día vale 0
		}
	}
	responder(w, http.StatusOK, map[string]any{
		"memoria": dec, "actualizada": m.Actualizada, "mudanzaPendiente": m.Deseada != nil, "mudanzaDesde": m.DeseadaDesde,
	})
}

// POST /v1/bichos/{id}/mudar {desde}: el bicho "se muda" de placa. La memoria
// (y la config) del bicho viejo pasan a este; se le mandan apenas se conecte.
func (s *Servidor) mudar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Desde string `json:"desde"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	b := bichoDe(r)
	desde := strings.ToUpper(strings.TrimSpace(p.Desde))
	if desde == "" || desde == b.ID {
		fallar(w, http.StatusBadRequest, "decí de qué bicho se muda (otro de tu cuenta)")
		return
	}
	if _, err := s.alm.BichoDe(r.Context(), desde, usuarioDe(r).Usuario.ID); err != nil {
		fallar(w, http.StatusNotFound, "ese bicho no es tuyo")
		return
	}
	if err := s.alm.Mudar(r.Context(), desde, b.ID); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusConflict, "todavía no tengo la memoria de ese bicho (se trae cada 5 min mientras está conectado)")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	// La config también se muda (autor "api": la copia la API).
	if c, err := s.alm.UltimaConfig(r.Context(), desde); err == nil {
		if _, err := s.nube.GuardarConfig(r.Context(), b.ID, c.Texto, "api", usuarioDe(r).Usuario.ID); err != nil {
			s.interno(w, err)
			return
		}
	}
	s.nube.CambioDeseado(b.ID)
	w.WriteHeader(http.StatusAccepted)
}
