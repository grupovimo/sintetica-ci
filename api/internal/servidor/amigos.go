package servidor

// Lo de la v1.9 en la API HTTP: amigos y toques, presencia (y los horarios
// aprendidos) y hábitos. Los avisos entrantes, en entrantes.go. Ver
// docs/nube.md, "Amigos y toques", "Presencia" y "Hábitos".

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/integraciones"
)

// rutasCompania suma las rutas de la v1.9 (amigos, presencia, hábitos y
// avisos entrantes).
func (s *Servidor) rutasCompania(m *http.ServeMux) {
	m.HandleFunc("POST /v1/amigos/invitacion", s.conSesion(s.invitarAmigo))
	m.HandleFunc("GET /v1/amigos", s.conSesion(s.listarAmigos))
	m.HandleFunc("POST /v1/amigos", s.conSesion(s.aceptarAmigo))
	m.HandleFunc("PUT /v1/amigos/{amigo}", s.conSesion(s.cambiarAmigo))
	m.HandleFunc("DELETE /v1/amigos/{amigo}", s.conSesion(s.borrarAmigo))
	m.HandleFunc("POST /v1/amigos/{amigo}/toque", s.conSesion(s.tocarAmigo))

	m.HandleFunc("POST /v1/bichos/{id}/presencia", s.conBicho(s.presencia))
	m.HandleFunc("GET /v1/bichos/{id}/horarios", s.conBicho(s.horarios))
	m.HandleFunc("GET /v1/bichos/{id}/habitos", s.conBicho(s.verHabitos))
	m.HandleFunc("POST /v1/bichos/{id}/habitos/{i}", s.conBicho(s.marcarHabito))

	m.HandleFunc("GET /v1/entrantes", s.conSesion(s.verEntrantes))
	m.HandleFunc("POST /v1/entrantes", s.conSesion(s.regenerarEntrantes))
	m.HandleFunc("PUT /v1/entrantes/mercadopago", s.conSesion(s.ponerMercadoPago))
	m.HandleFunc("DELETE /v1/entrantes/mercadopago", s.conSesion(s.sacarMercadoPago))
	m.HandleFunc("POST /v1/entrantes/{token}", s.recibirEntrante) // sin sesión: el token es la llave
}

// fallarAmigos traduce los errores de los amigos.
func (s *Servidor) fallarAmigos(w http.ResponseWriter, err error) {
	var inv integraciones.ErrToqueInvalido
	switch {
	case errors.As(err, &inv):
		fallar(w, http.StatusBadRequest, inv.Motivo)
	case errors.Is(err, integraciones.ErrNoEsAmigo):
		fallar(w, http.StatusNotFound, err.Error())
	case errors.Is(err, almacen.ErrInvitacionInvalida), errors.Is(err, almacen.ErrInvitacionPropia):
		fallar(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, almacen.ErrYaAmigos):
		fallar(w, http.StatusConflict, err.Error())
	case errors.Is(err, integraciones.ErrMuchasInvitaciones), errors.Is(err, integraciones.ErrMuchosIntentos),
		errors.Is(err, integraciones.ErrMuchosToques):
		fallar(w, http.StatusTooManyRequests, err.Error())
	default:
		s.interno(w, err)
	}
}

func idAmigo(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("amigo"), 10, 64)
	if err != nil || id <= 0 {
		fallar(w, http.StatusNotFound, integraciones.ErrNoEsAmigo.Error())
		return 0, false
	}
	return id, true
}

// POST /v1/amigos/invitacion → {codigo, link, telegram?, vence}.
func (s *Servidor) invitarAmigo(w http.ResponseWriter, r *http.Request) {
	c, vence, err := s.integ.Invitar(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.fallarAmigos(w, err)
		return
	}
	res := map[string]any{"codigo": c, "link": s.op.WebURL + "/robots?amigo=" + c, "vence": vence.Format(time.RFC3339)}
	if t := s.integ.LinkTelegramAmigo(c); t != "" {
		res["telegram"] = t
	}
	responder(w, http.StatusOK, res)
}

// GET /v1/amigos → {amigos: [{id, nombre, robots, todos, conectado, creada}]}.
func (s *Servidor) listarAmigos(w http.ResponseWriter, r *http.Request) {
	l, err := s.integ.Amigos(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"amigos": l})
}

// POST /v1/amigos {codigo} → el amigo nuevo.
func (s *Servidor) aceptarAmigo(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Codigo string `json:"codigo"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	if !s.limites.permitir("aceptar-ip:"+ipDe(r), 30, time.Hour) {
		fallar(w, http.StatusTooManyRequests, integraciones.ErrMuchosIntentos.Error())
		return
	}
	a, err := s.integ.AceptarAmigo(r.Context(), usuarioDe(r).Usuario.ID, p.Codigo)
	if err != nil {
		s.fallarAmigos(w, err)
		return
	}
	responder(w, http.StatusOK, a)
}

// PUT /v1/amigos/{id} {robots: [ids míos] | null (todos)}.
func (s *Servidor) cambiarAmigo(w http.ResponseWriter, r *http.Request) {
	id, ok := idAmigo(w, r)
	if !ok {
		return
	}
	var p struct {
		Robots []string `json:"robots"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if len(p.Robots) > 50 {
		fallar(w, http.StatusBadRequest, "son demasiados robots")
		return
	}
	a, err := s.integ.PonerRobotsAmigo(r.Context(), usuarioDe(r).Usuario.ID, id, p.Robots)
	if err != nil {
		s.fallarAmigos(w, err)
		return
	}
	responder(w, http.StatusOK, a)
}

// DELETE /v1/amigos/{id}: dejan de ser amigos (para los dos).
func (s *Servidor) borrarAmigo(w http.ResponseWriter, r *http.Request) {
	id, ok := idAmigo(w, r)
	if !ok {
		return
	}
	if err := s.integ.BorrarAmigo(r.Context(), usuarioDe(r).Usuario.ID, id); err != nil {
		s.fallarAmigos(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /v1/amigos/{id}/toque {tipo, expresion?, desde?} → {robots, telegram}.
func (s *Servidor) tocarAmigo(w http.ResponseWriter, r *http.Request) {
	id, ok := idAmigo(w, r)
	if !ok {
		return
	}
	var p struct {
		Tipo      string `json:"tipo"`
		Expresion string `json:"expresion"`
		Desde     string `json:"desde"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	res, err := s.integ.Tocar(r.Context(), usuarioDe(r).Usuario.ID, id, p.Tipo, p.Expresion, p.Desde)
	if err != nil {
		s.fallarAmigos(w, err)
		return
	}
	responder(w, http.StatusOK, res)
}

// --- Presencia ---

// POST /v1/bichos/{id}/presencia {llegaste: bool} → {nueva, enviado}.
func (s *Servidor) presencia(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Llegaste *bool `json:"llegaste"`
	}
	if !leerJSON(w, r, 256, &p) {
		return
	}
	if p.Llegaste == nil {
		fallar(w, http.StatusBadRequest, "falta llegaste (true o false)")
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("presencia:"+id, 60, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "demasiados avisos de presencia: esperá un rato")
		return
	}
	nueva, enviado, err := s.integ.Presencia(r.Context(), id, *p.Llegaste)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true, "nueva": nueva, "enviado": enviado})
}

// GET /v1/bichos/{id}/horarios → {llegas, salis, dormis, seguridad, dias}.
func (s *Servidor) horarios(w http.ResponseWriter, r *http.Request) {
	h, err := s.integ.HorariosDe(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, h)
}

// --- Hábitos ---

// GET /v1/bichos/{id}/habitos → [{indice, nombre, icono, hora?, racha, hoy, dias}].
func (s *Servidor) verHabitos(w http.ResponseWriter, r *http.Request) {
	l, err := s.integ.Habitos(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, l)
}

// POST /v1/bichos/{id}/habitos/{i} {hecho: bool} → {habito, enviado}.
func (s *Servidor) marcarHabito(w http.ResponseWriter, r *http.Request) {
	k, err := strconv.Atoi(r.PathValue("i"))
	if err != nil || k < 0 || k > 3 {
		fallar(w, http.StatusNotFound, integraciones.ErrSinHabito.Error())
		return
	}
	var p struct {
		Hecho *bool `json:"hecho"`
	}
	if !leerJSON(w, r, 256, &p) {
		return
	}
	if p.Hecho == nil {
		fallar(w, http.StatusBadRequest, "falta hecho (true o false)")
		return
	}
	id := bichoDe(r).ID
	if !s.limites.permitir("habito:"+id, 60, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados cambios seguidos: esperá un poco")
		return
	}
	h, enviado, err := s.integ.MarcarHabito(r.Context(), id, k, *p.Hecho, false)
	if errors.Is(err, integraciones.ErrSinHabito) {
		fallar(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"habito": h, "enviado": enviado})
}
