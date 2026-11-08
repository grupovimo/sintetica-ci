package servidor

// Los sprites de las caras de imagen (docs/motor-de-vida.md, "Caras de
// imagen"): 12 por robot (0 a 11), aparte de las 8 bahías de imágenes. Se
// guardan como lo deseado (los espacios 8 a 19 de la tabla de imágenes) y la
// sincronización se los lleva al robot por la característica Imagen.

import (
	"errors"
	"image"
	"io"
	"net/http"
	"strconv"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/imagenes"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

func spriteDe(r *http.Request) (int, bool) {
	n, err := strconv.Atoi(r.PathValue("sprite"))
	return n, err == nil && n >= 0 && n < protocolo.MaxSprites
}

type spriteGuardado struct {
	Sprite int    `json:"sprite"`
	Largo  int    `json:"largo"`
	Crc    uint32 `json:"crc"`
	Ancho  int    `json:"ancho,omitempty"`
	Alto   int    `json:"alto,omitempty"`
}

// listaSprites: los sprites del robot, con su número (0 a 11).
func listaSprites(l []almacen.Imagen) []spriteGuardado {
	out := []spriteGuardado{}
	for _, i := range almacen.SoloSprites(l) {
		out = append(out, spriteGuardado{Sprite: i.Espacio - protocolo.PrimerSprite, Largo: i.Largo, Crc: i.Crc})
	}
	return out
}

// GET /v1/bichos/{id}/sprites
func (s *Servidor) listarSprites(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Imagenes(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"sprites": listaSprites(l), "max": protocolo.MaxSprites, "sincro": s.nube.EstadoSincro(bichoDe(r).ID)})
}

// GET /v1/bichos/{id}/sprites/{sprite}: el QOI; con ?png=1, en PNG (para mostrarlo).
func (s *Servidor) verSprite(w http.ResponseWriter, r *http.Request) {
	n, ok := spriteDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el sprite va de 0 a 11")
		return
	}
	img, err := s.alm.LeerImagen(r.Context(), bichoDe(r).ID, protocolo.PrimerSprite+n)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "ese sprite está vacío")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	if r.URL.Query().Get("png") != "" {
		rgba, err := imagenes.DecodificarQoi(img.Datos)
		if err != nil {
			s.interno(w, err)
			return
		}
		// El QOI guarda la transparencia sin premultiplicar: es un NRGBA.
		w.Header().Set("Content-Type", "image/png")
		w.Write(imagenes.PNG(&image.NRGBA{Pix: rgba.Pix, Stride: rgba.Stride, Rect: rgba.Rect})) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "image/qoi")
	w.Write(img.Datos) //nolint:errcheck
}

// PUT /v1/bichos/{id}/sprites/{sprite}: el cuerpo es un SVG, PNG, JPEG o GIF
// (se recorta al contenido y se achica a 128 × 128 como mucho, con la
// transparencia) o un QOI ya listo.
func (s *Servidor) subirSprite(w http.ResponseWriter, r *http.Request) {
	n, ok := spriteDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el sprite va de 0 a 11")
		return
	}
	datos, err := io.ReadAll(http.MaxBytesReader(w, r.Body, imagenes.MaxEntrada))
	if err != nil {
		fallar(w, http.StatusRequestEntityTooLarge, "la imagen es muy grande")
		return
	}
	qoi := datos
	ancho, alto, esQoi := protocolo.TamanoSprite(datos)
	if !esQoi {
		sp, err := imagenes.PrepararSprite(datos)
		if err != nil {
			fallar(w, http.StatusUnprocessableEntity, "no pude convertirla en un sprite", err.Error())
			return
		}
		qoi, ancho, alto = sp.Datos, sp.Ancho, sp.Alto
	}
	if err := s.nube.GuardarSprite(r.Context(), bichoDe(r).ID, n, qoi); err != nil {
		if errors.Is(err, nube.ErrSprite) {
			fallar(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, spriteGuardado{Sprite: n, Largo: len(qoi), Crc: protocolo.Crc32(qoi), Ancho: ancho, Alto: alto})
}

// DELETE /v1/bichos/{id}/sprites/{sprite}
func (s *Servidor) borrarSprite(w http.ResponseWriter, r *http.Request) {
	n, ok := spriteDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el sprite va de 0 a 11")
		return
	}
	if err := s.nube.BorrarSprite(r.Context(), bichoDe(r).ID, n); err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
