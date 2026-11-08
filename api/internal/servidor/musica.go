package servidor

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/protocolo"
)

// GET /v1/musica → {disponible} (sin claves de Spotify) o {disponible,
// conectada, nombre, suena: {sonando, titulo, artista, bpm} | null}.
func (s *Servidor) verMusica(w http.ResponseWriter, r *http.Request) {
	e, err := s.integ.EstadoMusica(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, e)
}

// GET /v1/musica/spotify → {url}: a dónde mandar al usuario para autorizar.
func (s *Servidor) conectarSpotify(w http.ResponseWriter, r *http.Request) {
	u, err := s.integ.EnlaceSpotify(usuarioDe(r).Usuario.ID)
	if errors.Is(err, integraciones.ErrMusicaApagada) {
		fallar(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"url": u})
}

// GET /v1/musica/spotify/vuelta?code&state: la vuelta de Spotify; termina en
// la web (?spotify=ok o ?spotify=error).
func (s *Servidor) vueltaSpotify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	destino := strings.TrimRight(s.op.WebURL, "/") + "/robots?spotify="
	if !s.limites.permitir("spotify-vuelta:"+ipDe(r), 30, time.Hour) {
		http.Redirect(w, r, destino+"error", http.StatusFound)
		return
	}
	if q.Get("error") != "" {
		http.Redirect(w, r, destino+"cancelado", http.StatusFound)
		return
	}
	if _, err := s.integ.VueltaSpotify(r.Context(), q.Get("state"), q.Get("code")); err != nil {
		s.log.Info("spotify: no se pudo conectar", "error", err)
		http.Redirect(w, r, destino+"error&detalle="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, destino+"ok", http.StatusFound)
}

// DELETE /v1/musica/spotify: desconecta.
func (s *Servidor) desconectarSpotify(w http.ResponseWriter, r *http.Request) {
	if err := s.integ.DesconectarSpotify(r.Context(), usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.verMusica(w, r)
}

// POST /v1/bichos/{id}/musica {sonando, titulo, artista, bpm?}: lo que suena
// en el celu (Android lo lee de la sesión de medios).
func (s *Servidor) musicaDelCelu(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Sonando bool   `json:"sonando"`
		Titulo  string `json:"titulo"`
		Artista string `json:"artista"`
		BPM     int    `json:"bpm"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if len(p.Titulo) > 200 || len(p.Artista) > 200 || !utf8.ValidString(p.Titulo) || !utf8.ValidString(p.Artista) || p.BPM < 0 || p.BPM > 255 {
		fallar(w, http.StatusBadRequest, "música inválida", "titulo y artista de hasta 200 bytes (se cortan a 40 y 24), bpm de 0 a 255")
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("musica:"+id, 30, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "mandaste mucha música: esperá un poco")
		return
	}
	titulo := protocolo.CortarUTF8(strings.TrimSpace(p.Titulo), protocolo.MaxTituloMusica)
	artista := protocolo.CortarUTF8(strings.TrimSpace(p.Artista), protocolo.MaxArtistaMusica)
	if err := s.integ.MandarMusica(usuarioDe(r).Usuario.ID, id, p.Sonando, titulo, artista, p.BPM); err != nil {
		fallar(w, http.StatusConflict, err.Error())
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true})
}
