package servidor

// La voz de ElevenLabs con la cuenta de cada usuario (integraciones/elevenlabs.go,
// voz.go y conversacion.go). La clave entra por PUT y nunca sale.

import (
	"errors"
	"net/http"
	"time"

	"sintetica/api/internal/integraciones"
)

func (s *Servidor) rutasVoz(m *http.ServeMux) {
	m.HandleFunc("GET /v1/voz/elevenlabs", s.conSesion(s.verVoz))
	m.HandleFunc("PUT /v1/voz/elevenlabs", s.conSesion(s.guardarVoz))
	m.HandleFunc("DELETE /v1/voz/elevenlabs", s.conSesion(s.borrarVoz))
	m.HandleFunc("POST /v1/voz/elevenlabs/probar", s.conSesion(s.probarVoz))
	m.HandleFunc("GET /v1/bichos/{id}/conversacion", s.conBicho(s.pedirConversacion))
}

// fallarVoz: el error de ElevenLabs con su código.
func (s *Servidor) fallarVoz(w http.ResponseWriter, err error) {
	var c integraciones.ErrCambioVoz
	switch {
	case errors.As(err, &c), errors.Is(err, integraciones.ErrVozElevenLabs), errors.Is(err, integraciones.ErrClaveElevenLabs),
		errors.Is(err, integraciones.ErrPermisoEleven):
		fallar(w, http.StatusBadRequest, integraciones.MensajeVoz(err))
	case errors.Is(err, integraciones.ErrSinElevenLabs):
		fallar(w, http.StatusNotFound, err.Error())
	case errors.Is(err, integraciones.ErrTopeCaracteres), errors.Is(err, integraciones.ErrTopeMinutos):
		fallar(w, http.StatusTooManyRequests, err.Error())
	default:
		s.log.Info("elevenlabs", "error", err)
		fallar(w, http.StatusBadGateway, integraciones.MensajeVoz(err))
	}
}

// GET /v1/voz/elevenlabs → {conectado, voz, voz_nombre, voces: [{id, nombre,
// muestra?}], leer, usados_hoy, tope, segundos_hoy, tope_minutos}.
func (s *Servidor) verVoz(w http.ResponseWriter, r *http.Request) {
	e, err := s.integ.EstadoVoz(r.Context(), usuarioDe(r).Usuario.ID, true)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, e)
}

// PUT /v1/voz/elevenlabs {clave?, voz?, leer?, tope?, tope_minutos?}.
func (s *Servidor) guardarVoz(w http.ResponseWriter, r *http.Request) {
	var p integraciones.CambiosVoz
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("voz:"+u, 30, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "cambiaste mucho la voz: esperá un poco")
		return
	}
	if err := s.integ.ConectarVoz(r.Context(), u, p); err != nil {
		s.fallarVoz(w, err)
		return
	}
	s.verVoz(w, r)
}

// DELETE /v1/voz/elevenlabs: olvida la clave.
func (s *Servidor) borrarVoz(w http.ResponseWriter, r *http.Request) {
	if err := s.integ.DesconectarVoz(r.Context(), usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.verVoz(w, r)
}

// POST /v1/voz/elevenlabs/probar {texto?} → audio/mpeg.
func (s *Servidor) probarVoz(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Texto string `json:"texto"`
	}
	if r.ContentLength != 0 && !leerJSON(w, r, 4096, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("voz-probar:"+u, 10, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "probaste muchas veces: esperá un poco")
		return
	}
	audio, err := s.integ.ProbarVoz(r.Context(), u, p.Texto)
	if err != nil {
		s.fallarVoz(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(audio) //nolint:errcheck
}

// GET /v1/bichos/{id}/conversacion[?modo=webrtc] → {url | token, agente,
// vence, segundos, silencio, quedan, expresiones}: para hablar en tiempo real
// con el agente del robot (vence en 15 min).
func (s *Servidor) pedirConversacion(w http.ResponseWriter, r *http.Request) {
	u, id := usuarioDe(r).Usuario.ID, bichoDe(r).ID
	modo := r.URL.Query().Get("modo")
	if modo != "" && modo != "webrtc" && modo != "websocket" {
		fallar(w, http.StatusBadRequest, "modo inválido", "webrtc o websocket")
		return
	}
	if !s.limites.permitir("conversacion:"+u, 30, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchas conversaciones: esperá un poco")
		return
	}
	c, err := s.integ.PedirConversacion(r.Context(), u, id, modo)
	if err != nil {
		s.fallarVoz(w, err)
		return
	}
	responder(w, http.StatusOK, c)
}
