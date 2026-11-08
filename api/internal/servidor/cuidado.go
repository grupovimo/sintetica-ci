package servidor

// Lo de la v1.7 en la API HTTP: jugar (marco 0x43), los avisos del celu con
// su app y su contacto (0x44) y su día contado. Ver docs/nube.md, "Un bicho
// que acompaña el día".

import (
	"net/http"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/protocolo"
)

// POST /v1/bichos/{id}/jugar {"juego": "dado"|"moneda"|"ppt"|"reflejos"|"mirada"|"terminar"}.
func (s *Servidor) jugar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Juego string `json:"juego"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	m, err := protocolo.Jugar(p.Juego)
	if err != nil {
		fallar(w, http.StatusBadRequest, "juego inválido", err.Error())
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("jugar:"+id, 30, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "muchos juegos seguidos: esperá un poco")
		return
	}
	s.enviarComando(w, id, m)
}

// POST /v1/bichos/{id}/aviso {"tipo": "notificacion"|"llamada", "app"?, "contacto"?}:
// una notificación del celu; el robot reacciona según `avisos` de su config.
func (s *Servidor) aviso(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Tipo     string `json:"tipo"`
		App      string `json:"app"`
		Contacto string `json:"contacto"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if len(p.App) > 200 || len(p.Contacto) > 200 || !utf8.ValidString(p.App) || !utf8.ValidString(p.Contacto) {
		fallar(w, http.StatusBadRequest, "aviso inválido", "la app y el contacto son texto (se mandan hasta 24 bytes)")
		return
	}
	m, err := protocolo.Aviso(p.Tipo, p.App, p.Contacto)
	if err != nil {
		fallar(w, http.StatusBadRequest, "aviso inválido", err.Error())
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("aviso:"+id, 60, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados avisos: esperá un poco")
		return
	}
	s.enviarComando(w, id, m)
}

// GET /v1/bichos/{id}/resumen → {texto, juegos, ganados, suenos, sonoCon, animo, espero, agua, actualizada}.
func (s *Servidor) resumenDelDia(w http.ResponseWriter, r *http.Request) {
	res, err := s.integ.ResumenDe(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, res)
}
