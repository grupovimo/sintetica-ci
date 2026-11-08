package servidor

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/integraciones"
)

// --- Telegram ---

func (s *Servidor) estadoTelegram(r *http.Request) (map[string]any, error) {
	if !s.integ.TelegramActivo() {
		return map[string]any{"activo": false}, nil
	}
	v, err := s.alm.TelegramDe(r.Context(), usuarioDe(r).Usuario.ID)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return map[string]any{"activo": true, "bot": s.integ.Bot(), "vinculado": false, "nombre": ""}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"activo": true, "bot": s.integ.Bot(), "vinculado": true, "nombre": v.Nombre}, nil
}

// GET /v1/telegram: {activo, bot, vinculado, nombre}.
func (s *Servidor) verTelegram(w http.ResponseWriter, r *http.Request) {
	e, err := s.estadoTelegram(r)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, e)
}

// POST /v1/telegram/vincular: un link de un solo uso (15 min) para el bot.
func (s *Servidor) vincularTelegram(w http.ResponseWriter, r *http.Request) {
	if !s.integ.TelegramActivo() {
		responder(w, http.StatusOK, map[string]any{"activo": false})
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("telegram:"+u, 10, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchos links: esperá un rato")
		return
	}
	enlace, vence, err := s.integ.CrearVinculo(r.Context(), u)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"enlace": enlace, "vence": vence.Format(time.RFC3339)})
}

// DELETE /v1/telegram: desvincula el chat.
func (s *Servidor) desvincularTelegram(w http.ResponseWriter, r *http.Request) {
	if !s.integ.TelegramActivo() {
		responder(w, http.StatusOK, map[string]any{"activo": false})
		return
	}
	if err := s.integ.Desvincular(r.Context(), usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.verTelegram(w, r)
}

// --- Automatizaciones ---

func (s *Servidor) responderAutomatizaciones(w http.ResponseWriter, r *http.Request) {
	l, h, err := s.integ.Automatizaciones(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"automatizaciones": l, "historial": h, "cuandos": integraciones.Cuandos()})
}

// GET /v1/bichos/{id}/automatizaciones: {automatizaciones, historial, cuandos}.
func (s *Servidor) verAutomatizaciones(w http.ResponseWriter, r *http.Request) {
	s.responderAutomatizaciones(w, r)
}

// PUT /v1/bichos/{id}/automatizaciones {automatizaciones}: reemplaza la lista entera.
func (s *Servidor) guardarAutomatizaciones(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Automatizaciones []integraciones.Automatizacion `json:"automatizaciones"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		fallar(w, http.StatusBadRequest, "el pedido no es JSON válido", err.Error())
		return
	}
	err := s.integ.GuardarAutomatizaciones(r.Context(), usuarioDe(r).Usuario.ID, bichoDe(r).ID, p.Automatizaciones)
	var inv integraciones.ErrInvalida
	if errors.As(err, &inv) {
		fallar(w, http.StatusBadRequest, "automatizaciones inválidas", inv.Motivo)
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	s.responderAutomatizaciones(w, r)
}

// POST /v1/bichos/{id}/automatizaciones/{n}/probar: la corre ya ({n}: la
// posición desde 0, o su id).
func (s *Servidor) probarAutomatizacion(w http.ResponseWriter, r *http.Request) {
	res, err := s.integ.Probar(r.Context(), usuarioDe(r).Usuario.ID, bichoDe(r).ID, r.PathValue("n"))
	switch {
	case errors.Is(err, integraciones.ErrNoExiste):
		fallar(w, http.StatusNotFound, err.Error())
	case errors.Is(err, integraciones.ErrLimite):
		fallar(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		s.interno(w, err)
	default:
		ok := true
		for _, x := range res {
			ok = ok && x.Ok
		}
		responder(w, http.StatusOK, map[string]any{"ok": ok, "resultados": res})
	}
}

// --- Webhooks ---

// GET /v1/webhooks/secreto: {secreto} (se crea la primera vez).
func (s *Servidor) verSecretoWebhook(w http.ResponseWriter, r *http.Request) {
	sec, err := s.integ.SecretoWebhook(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"secreto": sec})
}

// POST /v1/webhooks/secreto: lo rota.
func (s *Servidor) rotarSecretoWebhook(w http.ResponseWriter, r *http.Request) {
	sec, err := s.integ.RotarSecretoWebhook(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"secreto": sec})
}
