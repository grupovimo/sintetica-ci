package servidor

// La clave de IA propia (integraciones/ia.go) y el paquete de voz sin
// internet (integraciones/paquete_voz.go). La clave entra por PUT y nunca sale.

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/integraciones"
)

// GET /v1/ia → {propia, servidor, proveedor, modelo, url_pista, usadas_hoy, tope}.
func (s *Servidor) verIA(w http.ResponseWriter, r *http.Request) {
	e, err := s.integ.EstadoIA(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, e)
}

// PUT /v1/ia {proveedor, clave, url?, modelo?, tope?} (sin clave: cambia el
// modelo o el tope de la que ya está).
func (s *Servidor) guardarIA(w http.ResponseWriter, r *http.Request) {
	var p integraciones.CambiosIA
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("ia:"+u, 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "cambiaste mucho la IA: esperá un poco")
		return
	}
	err := s.integ.ConectarIA(r.Context(), u, p)
	var c integraciones.ErrCambioIA
	switch {
	case err == nil:
		s.verIA(w, r)
	case errors.As(err, &c), errors.Is(err, integraciones.ErrClaveIA), errors.Is(err, integraciones.ErrModeloIA):
		fallar(w, http.StatusBadRequest, integraciones.MensajeIA(err))
	case errors.Is(err, integraciones.ErrSinIA):
		fallar(w, http.StatusNotFound, err.Error())
	case errors.Is(err, integraciones.ErrIACaida):
		fallar(w, http.StatusBadGateway, err.Error())
	default:
		s.interno(w, err)
	}
}

// DELETE /v1/ia: olvida la clave (vuelve la del servidor).
func (s *Servidor) borrarIA(w http.ResponseWriter, r *http.Request) {
	if err := s.integ.DesconectarIA(r.Context(), usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.verIA(w, r)
}

func topeDe(r *http.Request) (int, bool) {
	t := r.URL.Query().Get("tope")
	if t == "" {
		return 0, true
	}
	n, err := strconv.Atoi(t)
	return n, err == nil
}

// GET /v1/bichos/{id}/voz/paquete[?tope=bytes] → el estado: {estado,
// version, voz, frases, bytes, total, faltan, caracteres, gasta,
// bytes_estimados, quedan_hoy, usados_bytes, tope_bytes, progreso?…}.
func (s *Servidor) verPaqueteVoz(w http.ResponseWriter, r *http.Request) {
	tope, ok := topeDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "tope inválido")
		return
	}
	e, err := s.integ.EstadoPaqueteVoz(r.Context(), usuarioDe(r).Usuario.ID, bichoDe(r).ID, tope)
	s.responderPaquete(w, e, err)
}

// POST /v1/bichos/{id}/voz/paquete {confirmar?, tope?}: lo genera en segundo
// plano. Con ElevenLabs, sin confirmar y si gasta, 409 con el estado.
func (s *Servidor) generarPaqueteVoz(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Confirmar bool `json:"confirmar"`
		Tope      int  `json:"tope"`
	}
	if r.ContentLength != 0 && !leerJSON(w, r, 1024, &p) {
		return
	}
	u := usuarioDe(r).Usuario.ID
	if !s.limites.permitir("voz-paquete:"+u, 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchos paquetes: esperá un poco")
		return
	}
	e, err := s.integ.GenerarPaqueteVoz(r.Context(), u, bichoDe(r).ID, p.Confirmar, p.Tope)
	var c integraciones.ErrConfirmarPaquete
	if errors.As(err, &c) {
		c.Estado["mensaje"] = c.Error()
		responder(w, http.StatusConflict, c.Estado)
		return
	}
	if err == nil {
		responder(w, http.StatusAccepted, e)
		return
	}
	s.responderPaquete(w, e, err)
}

func (s *Servidor) responderPaquete(w http.ResponseWriter, e map[string]any, err error) {
	var cv integraciones.ErrCambioVoz
	switch {
	case err == nil:
		responder(w, http.StatusOK, e)
	case errors.As(err, &cv):
		fallar(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, integraciones.ErrSinVozPaquete):
		fallar(w, http.StatusConflict, err.Error())
	case errors.Is(err, almacen.ErrNoEncontrado):
		fallar(w, http.StatusNotFound, "no encontrado")
	default:
		s.interno(w, err)
	}
}

// GET /v1/voz/paquete/{id}?v&tope&vence&firma[&indice=1]: la bajada del
// robot, sin sesión (la autentica la firma HMAC del link, que vence en 10
// min). Un tar (indice.json y un <hash>.mp3 por frase) o, con indice=1,
// solo el índice.
func (s *Servidor) bajarPaqueteVoz(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(r.PathValue("id"))
	if !s.limites.permitir("voz-paquete-bajar:"+id, 30, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "demasiadas bajadas: esperá un poco")
		return
	}
	p, err := s.integ.AbrirPaqueteVoz(r.Context(), id, r.URL.Query())
	switch {
	case errors.Is(err, integraciones.ErrFirmaPaquete):
		fallar(w, http.StatusForbidden, err.Error())
		return
	case errors.Is(err, integraciones.ErrVersionPaq):
		fallar(w, http.StatusGone, err.Error())
		return
	case errors.Is(err, almacen.ErrNoEncontrado):
		fallar(w, http.StatusNotFound, "no hay paquete")
		return
	case err != nil:
		s.interno(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Query().Get("indice") == "1" {
		responder(w, http.StatusOK, p.Indice)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Length", strconv.FormatInt(p.Tamano(), 10))
	w.Header().Set("Content-Disposition", `attachment; filename="voz-`+id+`-v`+strconv.Itoa(p.Indice.Version)+`.tar"`)
	w.WriteHeader(http.StatusOK)
	if err := p.EscribirTar(w); err != nil {
		s.log.Info("paquete de voz: se cortó la bajada", "bicho", id, "error", err)
	}
}
