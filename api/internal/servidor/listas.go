package servidor

// Las listas (almacen/listas.go; por charla, integraciones/listas.go). Ver
// docs/nube.md, "Listas". Cada cambio publica el evento "listas" (SSE) para
// que la web y el escritorio abiertos se refresquen.

import (
	"errors"
	"net/http"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
)

const cambiosListasPorMinuto = 120

func (s *Servidor) rutasListas(m *http.ServeMux) {
	m.HandleFunc("GET /v1/listas", s.conSesion(s.verListas))
	m.HandleFunc("POST /v1/listas", s.conSesion(s.crearLista))
	m.HandleFunc("DELETE /v1/listas/{id}", s.conSesion(s.borrarLista))
	m.HandleFunc("POST /v1/listas/{id}/items", s.conSesion(s.agregarItems))
	m.HandleFunc("PATCH /v1/listas/{id}/items/{item}", s.conSesion(s.cambiarItem))
	m.HandleFunc("DELETE /v1/listas/{id}/items/{item}", s.conSesion(s.borrarItem))
	m.HandleFunc("POST /v1/listas/{id}/limpiar", s.conSesion(s.limpiarLista))
}

// errorListas: los errores de las listas en HTTP.
func (s *Servidor) errorListas(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, almacen.ErrNoEncontrado):
		fallar(w, http.StatusNotFound, "esa lista (o esa cosa) no existe")
	case errors.Is(err, almacen.ErrTopeListas), errors.Is(err, almacen.ErrTopeItems):
		fallar(w, http.StatusConflict, err.Error())
	case errors.Is(err, almacen.ErrNombreLista), errors.Is(err, almacen.ErrTextoItem):
		fallar(w, http.StatusBadRequest, err.Error())
	default:
		s.interno(w, err)
	}
}

// cambioListas: el límite de cambios por minuto (false: ya contestó 429).
func (s *Servidor) cambioListas(w http.ResponseWriter, r *http.Request) bool {
	if !s.limites.permitir("listas:"+usuarioDe(r).Usuario.ID, cambiosListasPorMinuto, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados cambios: esperá un poco")
		return false
	}
	return true
}

func (s *Servidor) publicarListas(r *http.Request) {
	s.hub.Publicar(usuarioDe(r).Usuario.ID, eventos.Evento{Tipo: "listas"})
}

// GET /v1/listas → {listas: [{id, nombre, creada, items: [{id, texto, hecho, creado, hechoEn}]}], porDefecto}.
func (s *Servidor) verListas(w http.ResponseWriter, r *http.Request) {
	ls, err := s.alm.Listas(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	if ls == nil {
		ls = []almacen.Lista{}
	}
	responder(w, http.StatusOK, map[string]any{"listas": ls, "porDefecto": integraciones.ListaPorDefecto,
		"topes": map[string]int{"listas": almacen.MaxListas, "items": almacen.MaxItemsLista, "texto": almacen.MaxTextoItem, "nombre": almacen.MaxNombreLista}})
}

// POST /v1/listas {nombre} → la lista (201 si es nueva; 200 si ya existía con
// ese nombre).
func (s *Servidor) crearLista(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Nombre string `json:"nombre"`
	}
	if !leerJSON(w, r, 4<<10, &p) || !s.cambioListas(w, r) {
		return
	}
	l, nueva, err := s.alm.AsegurarLista(r.Context(), usuarioDe(r).Usuario.ID, p.Nombre)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	estado := http.StatusOK
	if nueva {
		estado = http.StatusCreated
		s.publicarListas(r)
	}
	responder(w, estado, l)
}

// DELETE /v1/listas/{id}.
func (s *Servidor) borrarLista(w http.ResponseWriter, r *http.Request) {
	if !s.cambioListas(w, r) {
		return
	}
	if err := s.alm.BorrarLista(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id")); err != nil {
		s.errorListas(w, err)
		return
	}
	s.publicarListas(r)
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/listas/{id}/items {texto} ("leche, pan y queso": varias) →
// {lista, agregados, yaEstaban}.
func (s *Servidor) agregarItems(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Texto string `json:"texto"`
	}
	if !leerJSON(w, r, 8<<10, &p) || !s.cambioListas(w, r) {
		return
	}
	items := integraciones.SepararItems(p.Texto)
	if len(items) == 0 {
		fallar(w, http.StatusBadRequest, almacen.ErrTextoItem.Error())
		return
	}
	u, id := usuarioDe(r).Usuario.ID, r.PathValue("id")
	nuevos, ya, err := s.alm.AgregarItems(r.Context(), u, id, items)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	l, err := s.alm.ListaDe(r.Context(), u, id)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	if len(nuevos) > 0 {
		s.publicarListas(r)
	}
	responder(w, http.StatusOK, map[string]any{"lista": l, "agregados": vacioSiNil(nuevos), "yaEstaban": vacioSiNil(ya)})
}

func vacioSiNil(l []almacen.ItemLista) []almacen.ItemLista {
	if l == nil {
		return []almacen.ItemLista{}
	}
	return l
}

// PATCH /v1/listas/{id}/items/{item} {hecho?, texto?} → la cosa.
func (s *Servidor) cambiarItem(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Hecho *bool   `json:"hecho"`
		Texto *string `json:"texto"`
	}
	if !leerJSON(w, r, 4<<10, &p) || !s.cambioListas(w, r) {
		return
	}
	if p.Hecho == nil && p.Texto == nil {
		fallar(w, http.StatusBadRequest, "falta hecho o texto")
		return
	}
	it, err := s.alm.CambiarItem(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id"), r.PathValue("item"), p.Hecho, p.Texto)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	s.publicarListas(r)
	responder(w, http.StatusOK, it)
}

// DELETE /v1/listas/{id}/items/{item}.
func (s *Servidor) borrarItem(w http.ResponseWriter, r *http.Request) {
	if !s.cambioListas(w, r) {
		return
	}
	if err := s.alm.BorrarItem(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id"), r.PathValue("item")); err != nil {
		s.errorListas(w, err)
		return
	}
	s.publicarListas(r)
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/listas/{id}/limpiar: saca las tachadas → {lista, sacadas}.
func (s *Servidor) limpiarLista(w http.ResponseWriter, r *http.Request) {
	if !s.cambioListas(w, r) {
		return
	}
	u, id := usuarioDe(r).Usuario.ID, r.PathValue("id")
	n, err := s.alm.LimpiarLista(r.Context(), u, id, false)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	l, err := s.alm.ListaDe(r.Context(), u, id)
	if err != nil {
		s.errorListas(w, err)
		return
	}
	if n > 0 {
		s.publicarListas(r)
	}
	responder(w, http.StatusOK, map[string]any{"lista": l, "sacadas": n})
}
