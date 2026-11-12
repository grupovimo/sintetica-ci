package servidor

// El mail: Gmail y Outlook con OAuth (integraciones/correo.go). Ver
// docs/nube.md, "Mail".

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sintetica/api/internal/integraciones"
)

func (s *Servidor) rutasCorreo(m *http.ServeMux) {
	m.HandleFunc("GET /v1/correo", s.conSesion(s.verCorreo))
	m.HandleFunc("GET /v1/correo/{proveedor}", s.conSesion(s.conectarCorreo))
	m.HandleFunc("GET /v1/correo/{proveedor}/vuelta", s.vueltaCorreo) // la autentica el state firmado
	m.HandleFunc("DELETE /v1/correo/{proveedor}", s.conSesion(s.desconectarCorreo))
}

// GET /v1/correo → {gmail: {disponible, conectada, direccion}, outlook: {…},
// noLeidos: [{de, asunto, en, proveedor}]}.
func (s *Servidor) verCorreo(w http.ResponseWriter, r *http.Request) {
	e, err := s.integ.EstadoCorreo(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, e)
}

// GET /v1/correo/{proveedor} → {url}: a dónde mandar al usuario para autorizar.
func (s *Servidor) conectarCorreo(w http.ResponseWriter, r *http.Request) {
	u, err := s.integ.EnlaceCorreo(usuarioDe(r).Usuario.ID, r.PathValue("proveedor"))
	switch {
	case errors.Is(err, integraciones.ErrProveedorCorreo):
		fallar(w, http.StatusNotFound, err.Error())
	case errors.Is(err, integraciones.ErrCorreoApagado):
		fallar(w, http.StatusServiceUnavailable, err.Error())
	case err != nil:
		s.interno(w, err)
	default:
		responder(w, http.StatusOK, map[string]any{"url": u})
	}
}

// GET /v1/correo/{proveedor}/vuelta?code&state: la vuelta de Google o
// Microsoft; termina en la web (?correo=ok, error o cancelado).
func (s *Servidor) vueltaCorreo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	destino := strings.TrimRight(s.op.WebURL, "/") + "/robots?correo="
	if !s.limites.permitir("correo-vuelta:"+ipDe(r), 30, time.Hour) {
		http.Redirect(w, r, destino+"error", http.StatusFound)
		return
	}
	if q.Get("error") != "" {
		http.Redirect(w, r, destino+"cancelado", http.StatusFound)
		return
	}
	if _, err := s.integ.VueltaCorreo(r.Context(), r.PathValue("proveedor"), q.Get("state"), q.Get("code")); err != nil {
		s.log.Info("correo: no se pudo conectar", "proveedor", r.PathValue("proveedor"), "error", err)
		http.Redirect(w, r, destino+"error&detalle="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destino+"ok", http.StatusFound)
}

// DELETE /v1/correo/{proveedor}: desconecta.
func (s *Servidor) desconectarCorreo(w http.ResponseWriter, r *http.Request) {
	err := s.integ.DesconectarCorreo(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("proveedor"))
	if errors.Is(err, integraciones.ErrProveedorCorreo) {
		fallar(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	s.verCorreo(w, r)
}
