package servidor

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
)

// Las piezas (web/src/lib/landing/piezas.ts) y los colores de carcasa.
var (
	piezas         = map[string]bool{"unidad": true, "broche": true, "hebilla": true, "colgante": true, "translucido": true, "alfiler": true, "hebilla-pelo": true}
	coloresCarcasa = map[string]bool{"naranja": true, "negro": true, "blanco": true}
)

// POST /v1/reservas {nombre, pieza, contacto, color, mensaje, sitio}: "Reservá el
// tuyo" de la landing, sin cuenta. "sitio" es una trampa para bots (un campo
// que la persona no ve): si viene lleno se contesta bien y no se guarda.
func (s *Servidor) reservar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Nombre   string `json:"nombre"`
		Pieza    string `json:"pieza"`
		Contacto string `json:"contacto"`
		Color    string `json:"color"`
		Mensaje  string `json:"mensaje"`
		Sitio    string `json:"sitio"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	res := almacen.Reserva{
		Nombre: strings.TrimSpace(p.Nombre), Pieza: strings.TrimSpace(p.Pieza), Contacto: strings.TrimSpace(p.Contacto),
		Color: strings.TrimSpace(p.Color), Mensaje: strings.TrimSpace(p.Mensaje),
	}
	switch {
	case res.Nombre == "" || utf8.RuneCountInString(res.Nombre) > 80:
		fallar(w, http.StatusBadRequest, "decinos tu nombre (hasta 80 letras)")
		return
	case utf8.RuneCountInString(res.Contacto) < 5 || utf8.RuneCountInString(res.Contacto) > 120:
		fallar(w, http.StatusBadRequest, "dejanos un email o un WhatsApp para escribirte")
		return
	case !piezas[res.Pieza]:
		fallar(w, http.StatusBadRequest, "esa pieza no existe")
		return
	case !coloresCarcasa[res.Color]:
		fallar(w, http.StatusBadRequest, "el color es naranja, negro o blanco")
		return
	case utf8.RuneCountInString(res.Mensaje) > 500:
		fallar(w, http.StatusBadRequest, "el mensaje es de hasta 500 letras")
		return
	}
	if p.Sitio != "" {
		responder(w, http.StatusCreated, map[string]any{"ok": true})
		return
	}
	ip := ipDe(r)
	if !s.limites.permitir("reserva:"+ip, 5, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "ya nos dejaste varias reservas: te escribimos pronto")
		return
	}
	if n, err := s.alm.ReservasDeIP(r.Context(), ip, time.Now().Add(-24*time.Hour)); err != nil {
		s.interno(w, err)
		return
	} else if n >= 10 {
		fallar(w, http.StatusTooManyRequests, "ya nos dejaste varias reservas: te escribimos pronto")
		return
	}
	id, err := s.alm.CrearReserva(r.Context(), res, ip)
	if err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("reserva nueva", "id", id, "pieza", res.Pieza, "color", res.Color)
	// El aviso a Nahuel por Telegram (como hacía la landing sola); si falla, la
	// reserva igual quedó guardada.
	if s.integ != nil {
		texto := "🧡 <b>Reserva nueva</b> · " + html.EscapeString(res.Pieza) + " " + html.EscapeString(res.Color) +
			"\n<b>" + html.EscapeString(res.Nombre) + "</b>\n" + html.EscapeString(res.Contacto)
		if res.Mensaje != "" {
			texto += "\n\n" + html.EscapeString(res.Mensaje)
		}
		go func() {
			ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelar()
			if err := s.integ.AvisarReserva(ctx, texto); err != nil {
				s.log.Warn("reserva: no la pude avisar por Telegram", "id", id, "error", err)
			}
		}()
	}
	responder(w, http.StatusCreated, map[string]any{"ok": true})
}

// GET /v1/reservas: solo administradores.
func (s *Servidor) listarReservas(w http.ResponseWriter, r *http.Request) {
	if !usuarioDe(r).Usuario.Admin {
		fallar(w, http.StatusForbidden, "solo para administradores")
		return
	}
	l, err := s.alm.Reservas(r.Context(), 500)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"reservas": l})
}

// PATCH /v1/reservas/{id} {atendida}: solo administradores.
func (s *Servidor) marcarReserva(w http.ResponseWriter, r *http.Request) {
	if !usuarioDe(r).Usuario.Admin {
		fallar(w, http.StatusForbidden, "solo para administradores")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fallar(w, http.StatusBadRequest, "id inválido")
		return
	}
	var p struct {
		Atendida bool `json:"atendida"`
	}
	if !leerJSON(w, r, 256, &p) {
		return
	}
	if err := s.alm.MarcarReserva(r.Context(), id, p.Atendida); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no existe esa reserva")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"ok": true})
}
