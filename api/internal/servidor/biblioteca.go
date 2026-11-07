package servidor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
)

var nombreValido = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`).MatchString

func (s *Servidor) listarBiblioteca(w http.ResponseWriter, r *http.Request) {
	tipo := r.URL.Query().Get("tipo")
	if tipo != "" && tipo != "cara" && tipo != "animacion" {
		fallar(w, http.StatusBadRequest, "tipo es cara o animacion")
		return
	}
	l, err := s.alm.Biblioteca(r.Context(), usuarioDe(r).Usuario.ID, tipo)
	if err != nil {
		s.interno(w, err)
		return
	}
	if l == nil {
		l = []almacen.Pieza{}
	}
	responder(w, http.StatusOK, map[string]any{"piezas": l})
}

// POST /v1/biblioteca {tipo, nombre, documento}: guarda una cara o una
// animación (por nombre: si ya hay una con ese nombre, la reemplaza). Se
// valida con el núcleo, como si fuera parte de una config.
func (s *Servidor) guardarPieza(w http.ResponseWriter, r *http.Request) {
	var p almacen.Pieza
	if !leerJSON(w, r, 32*1024, &p) {
		return
	}
	if p.Tipo != "cara" && p.Tipo != "animacion" {
		fallar(w, http.StatusBadRequest, "tipo es cara o animacion")
		return
	}
	if !nombreValido(p.Nombre) {
		fallar(w, http.StatusBadRequest, "el nombre va en minúsculas, números y guiones (hasta 24)")
		return
	}
	var prueba []byte
	if p.Tipo == "cara" {
		prueba, _ = json.Marshal(map[string]any{"version": 1, "expresiones": map[string]json.RawMessage{p.Nombre: p.Documento}})
	} else {
		prueba, _ = json.Marshal(map[string]any{"version": 1, "animaciones": map[string]json.RawMessage{p.Nombre: p.Documento}})
	}
	if err := nucleo.ValidarConfig(prueba); err != nil {
		fallar(w, http.StatusUnprocessableEntity, fmt.Sprintf("el bicho no aceptaría esta %s", p.Tipo), err.Error())
		return
	}
	g, err := s.alm.GuardarPieza(r.Context(), usuarioDe(r).Usuario.ID, p)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, g)
}

func (s *Servidor) borrarPieza(w http.ResponseWriter, r *http.Request) {
	err := s.alm.BorrarPieza(r.Context(), usuarioDe(r).Usuario.ID, r.PathValue("id"))
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no está")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
