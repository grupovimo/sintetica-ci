package servidor

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/protocolo"
)

var (
	perfilValido  = regexp.MustCompile(`^[a-z0-9-]{1,23}$`).MatchString
	versionValida = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,23}$`).MatchString
)

const maxFirmware = 8 << 20

// VistaFirmware es un firmware con su manifiesto en hex (para las apps, que lo mandan por BLE).
type VistaFirmware struct {
	almacen.Firmware
	Sha256 string `json:"sha256"`
	Firma  string `json:"firma"`
}

func vistaFirmware(f almacen.Firmware) VistaFirmware {
	return VistaFirmware{Firmware: f, Sha256: fmt.Sprintf("%x", f.Sha256), Firma: fmt.Sprintf("%x", f.Firma)}
}

func manifiestoDe(f almacen.Firmware) protocolo.Manifiesto {
	m := protocolo.Manifiesto{Tamano: uint32(f.Tamano), Version: f.Version, Perfil: f.Perfil}
	copy(m.Sha256[:], f.Sha256)
	copy(m.Firma[:], f.Firma)
	return m
}

func (s *Servidor) listarFirmwares(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Firmwares(r.Context(), r.URL.Query().Get("perfil"))
	if err != nil {
		s.interno(w, err)
		return
	}
	v := make([]VistaFirmware, 0, len(l))
	for _, f := range l {
		v = append(v, vistaFirmware(f))
	}
	responder(w, http.StatusOK, map[string]any{"firmwares": v, "puedoSubir": usuarioDe(r).Usuario.Admin})
}

// POST /v1/firmwares (multipart: perfil, version, notas, binario): solo
// administradores. La API lo firma con su clave privada.
func (s *Servidor) subirFirmware(w http.ResponseWriter, r *http.Request) {
	if !usuarioDe(r).Usuario.Admin {
		fallar(w, http.StatusForbidden, "solo los administradores (ADMIN_EMAILS) suben firmwares")
		return
	}
	if s.firmador == nil {
		fallar(w, http.StatusServiceUnavailable, "la API no tiene la clave de firma (npm run claves)")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFirmware+64*1024)
	if err := r.ParseMultipartForm(maxFirmware); err != nil {
		fallar(w, http.StatusBadRequest, "mandá un formulario con perfil, version y binario (hasta 8 MB)")
		return
	}
	perfil, version, notas := r.FormValue("perfil"), r.FormValue("version"), r.FormValue("notas")
	if !perfilValido(perfil) || !versionValida(version) || len(notas) > 2000 {
		fallar(w, http.StatusBadRequest, "perfil y versión de hasta 23 caracteres (letras, números, puntos y guiones)")
		return
	}
	archivo, _, err := r.FormFile("binario")
	if err != nil {
		fallar(w, http.StatusBadRequest, "falta el binario")
		return
	}
	defer archivo.Close()
	bin, err := io.ReadAll(archivo)
	if err != nil || len(bin) == 0 {
		fallar(w, http.StatusBadRequest, "el binario está vacío")
		return
	}
	// Las placas ESP32 empiezan la imagen con 0xE9; el bicho virtual acepta cualquier cosa.
	if perfil != "virtual" && bin[0] != 0xE9 {
		fallar(w, http.StatusUnprocessableEntity, "eso no parece un firmware de ESP32 (firmware.bin de PlatformIO)")
		return
	}
	m, err := s.firmador.Firmar(perfil, version, bin)
	if err != nil {
		fallar(w, http.StatusBadRequest, err.Error())
		return
	}
	f, err := s.alm.CrearFirmware(r.Context(), almacen.Firmware{Perfil: perfil, Version: version, Tamano: len(bin),
		Sha256: m.Sha256[:], Firma: m.Firma[:], Binario: bin, Notas: notas})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			fallar(w, http.StatusConflict, "ya hay un firmware "+version+" para "+perfil)
			return
		}
		s.interno(w, err)
		return
	}
	responder(w, http.StatusCreated, vistaFirmware(f))
}

func idFirmware(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

func (s *Servidor) borrarFirmware(w http.ResponseWriter, r *http.Request) {
	if !usuarioDe(r).Usuario.Admin {
		fallar(w, http.StatusForbidden, "solo los administradores")
		return
	}
	id, ok := idFirmware(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := s.alm.BorrarFirmware(r.Context(), id); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no está")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/firmwares/{id}/manifiesto: los bytes del manifiesto (para mandarlo por BLE).
func (s *Servidor) manifiesto(w http.ResponseWriter, r *http.Request) {
	id, ok := idFirmware(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "id inválido")
		return
	}
	f, err := s.alm.LeerFirmware(r.Context(), id, false)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no está")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	mb, _ := manifiestoDe(f).Bytes()
	responder(w, http.StatusOK, map[string]any{"firmware": vistaFirmware(f), "manifiesto": base64.StdEncoding.EncodeToString(mb)})
}

// GET /v1/firmwares/{id}/binario: con sesión (la app, para mandarlo por BLE) o
// con ?t= (el link firmado que le llega al bicho por la nube).
func (s *Servidor) binario(w http.ResponseWriter, r *http.Request) {
	id, ok := idFirmware(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "id inválido")
		return
	}
	servir := func(w http.ResponseWriter, r *http.Request) {
		f, err := s.alm.LeerFirmware(r.Context(), id, true)
		if errors.Is(err, almacen.ErrNoEncontrado) {
			fallar(w, http.StatusNotFound, "no está")
			return
		}
		if err != nil {
			s.interno(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(f.Binario)))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="sintetica-%s-%s.bin"`, f.Perfil, f.Version))
		w.Write(f.Binario) //nolint:errcheck
	}
	if t := r.URL.Query().Get("t"); t != "" {
		if !s.verificarDescarga(id, t) {
			fallar(w, http.StatusForbidden, "el link de descarga venció")
			return
		}
		servir(w, r)
		return
	}
	s.conSesion(servir)(w, r)
}
