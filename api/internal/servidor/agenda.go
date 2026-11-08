package servidor

// Lo de la v1.9 en la API HTTP que hace la nube sola: la agenda (acá), la
// música (musica.go) y la charla con IA (charla.go). Ver docs/nube.md,
// "Agenda", "Música" y "Charla".

import (
	"errors"
	"net/http"
	"time"

	"sintetica/api/internal/integraciones"
)

// rutasAgendaMusicaCharla suma las rutas de agenda.go, musica.go y charla.go.
func (s *Servidor) rutasAgendaMusicaCharla(m *http.ServeMux) {
	m.HandleFunc("GET /v1/agenda", s.conSesion(s.verAgenda))
	m.HandleFunc("PUT /v1/agenda", s.conSesion(s.guardarAgenda))
	m.HandleFunc("DELETE /v1/agenda", s.conSesion(s.borrarAgenda))

	m.HandleFunc("GET /v1/musica", s.conSesion(s.verMusica))
	m.HandleFunc("GET /v1/musica/spotify", s.conSesion(s.conectarSpotify))
	m.HandleFunc("GET /v1/musica/spotify/vuelta", s.vueltaSpotify) // la autentica el state firmado
	m.HandleFunc("DELETE /v1/musica/spotify", s.conSesion(s.desconectarSpotify))
	m.HandleFunc("POST /v1/bichos/{id}/musica", s.conBicho(s.musicaDelCelu))

	m.HandleFunc("GET /v1/charla", s.conSesion(s.verCharla))
	m.HandleFunc("POST /v1/bichos/{id}/charla", s.conBicho(s.charlar))
}

// GET /v1/agenda → {conectada, ics (solo la pista), aviso, proximos (10), leida, error}.
func (s *Servidor) verAgenda(w http.ResponseWriter, r *http.Request) {
	v, err := s.integ.VerAgenda(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, v)
}

// PUT /v1/agenda {ics?, aviso?}: conecta (o cambia) el calendario y/o cuántos
// minutos antes avisa (0 a 15).
func (s *Servidor) guardarAgenda(w http.ResponseWriter, r *http.Request) {
	var p struct {
		ICS   *string `json:"ics"`
		Aviso *int    `json:"aviso"`
	}
	if !leerJSON(w, r, 8192, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	var v integraciones.VistaAgenda
	var err error
	switch {
	case p.ICS != nil:
		if !s.limites.permitir("agenda:"+u, 20, time.Hour) {
			fallar(w, http.StatusTooManyRequests, "cambiaste mucho el calendario: esperá un rato")
			return
		}
		v, err = s.integ.ConectarAgenda(r.Context(), u, *p.ICS, p.Aviso)
	case p.Aviso != nil:
		v, err = s.integ.CambiarAvisoAgenda(r.Context(), u, *p.Aviso)
	default:
		fallar(w, http.StatusBadRequest, "falta ics o aviso")
		return
	}
	var inv integraciones.ErrAgendaInvalida
	switch {
	case errors.As(err, &inv):
		fallar(w, http.StatusBadRequest, "calendario inválido", inv.Motivo)
	case errors.Is(err, integraciones.ErrSinAgenda):
		fallar(w, http.StatusNotFound, err.Error())
	case err != nil:
		s.interno(w, err)
	default:
		responder(w, http.StatusOK, v)
	}
}

// DELETE /v1/agenda: lo olvida.
func (s *Servidor) borrarAgenda(w http.ResponseWriter, r *http.Request) {
	if err := s.integ.BorrarAgenda(r.Context(), usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.verAgenda(w, r)
}
