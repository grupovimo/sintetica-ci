package servidor

// El escritorio (la app de Mac y Windows) se vende con pago único: una
// licencia por cuenta, con 7 días de prueba desde que se suma el primer robot
// de escritorio. El precio es la fila "escritorio" de la preventa (lo pone el
// dueño); se cobra con Mercado Pago, como las señas.

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
)

const (
	productoEscritorio = "escritorio"
	pruebaEscritorio   = 7 * 24 * time.Hour
)

// GET /v1/escritorio → {licencia, pruebaHasta, enPrueba, venta, precio, moneda}:
// lo que la app necesita para saber si pide comprar.
func (s *Servidor) verEscritorio(w http.ResponseWriter, r *http.Request) {
	u := usuarioDe(r).Usuario.ID
	ctx := r.Context()
	licencia, err := s.alm.TieneLicencia(ctx, u, productoEscritorio)
	if err != nil {
		s.interno(w, err)
		return
	}
	desde, err := s.alm.PrimerRobot(ctx, u, productoEscritorio)
	if err != nil {
		s.interno(w, err)
		return
	}
	inicio := time.Now()
	if desde != nil {
		inicio = *desde
	}
	hasta := inicio.Add(pruebaEscritorio)
	pv, err := s.alm.VerPreventa(ctx, productoEscritorio)
	if err != nil {
		s.interno(w, err)
		return
	}
	venta := pv.Activa && pv.Precio > 0 && s.op.MPToken != ""
	responder(w, http.StatusOK, map[string]any{"licencia": licencia, "pruebaHasta": hasta, "enPrueba": time.Now().Before(hasta),
		"venta": venta, "precio": pv.Precio, "moneda": "ARS"})
}

// POST /v1/escritorio/comprar → {url}: la licencia pendiente y el link de pago.
func (s *Servidor) comprarEscritorio(w http.ResponseWriter, r *http.Request) {
	us := usuarioDe(r).Usuario
	ctx := r.Context()
	if ya, err := s.alm.TieneLicencia(ctx, us.ID, productoEscritorio); err != nil {
		s.interno(w, err)
		return
	} else if ya {
		fallar(w, http.StatusConflict, "ya tenés el escritorio")
		return
	}
	pv, err := s.alm.VerPreventa(ctx, productoEscritorio)
	if err != nil {
		s.interno(w, err)
		return
	}
	if !pv.Activa || pv.Precio <= 0 || s.op.MPToken == "" {
		fallar(w, http.StatusConflict, "todavía no se vende")
		return
	}
	if !s.limites.permitir("comprar:"+us.ID, 10, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "probaste muchas veces: esperá un rato")
		return
	}
	l, err := s.alm.CrearLicencia(ctx, us.ID, productoEscritorio, pv.Precio, "ARS")
	if err != nil {
		s.interno(w, err)
		return
	}
	email := us.Email
	if !reEmail.MatchString(email) {
		email = ""
	}
	web := strings.TrimRight(s.op.WebURL, "/")
	pref, link, err := s.preferenciaMP(ctx, pedidoMP{
		Item: "escritorio", Titulo: "Sintética · escritorio (licencia)", Monto: l.Monto,
		Referencia: referenciaLicencia(l.ID), Volver: web + "/robots?compra=%s", Email: email, Nombre: us.Nombre,
	})
	if err != nil {
		s.log.Warn("escritorio: Mercado Pago no armó el pago", "licencia", l.ID, "error", err)
		s.alm.RechazarLicencia(ctx, l.ID, "") //nolint:errcheck // queda como intento fallido
		fallar(w, http.StatusBadGateway, "Mercado Pago no contestó: probá de nuevo en un rato")
		return
	}
	if err := s.alm.PonerPreferenciaLicencia(ctx, l.ID, pref); err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("licencia nueva", "id", l.ID, "monto", l.Monto)
	responder(w, http.StatusCreated, map[string]any{"url": link, "id": l.ID})
}

// referenciaLicencia: el external_reference del pago ("lic-<id>").
func referenciaLicencia(id int64) string { return "lic-" + strconv.FormatInt(id, 10) }

// avisoLicencia: el aviso de Mercado Pago de un pago "lic-<id>" (ya leído con el token).
func (s *Servidor) avisoLicencia(w http.ResponseWriter, r *http.Request, pago pagoPreventa, pagoID string) {
	id, err := strconv.ParseInt(strings.TrimPrefix(pago.Referencia, "lic-"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	l, err := s.alm.VerLicencia(r.Context(), id)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	switch pago.Status {
	case "approved":
		if pago.Moneda != l.Moneda || pago.Monto+0.005 < l.Monto {
			s.log.Warn("escritorio: el pago no coincide con la licencia", "licencia", l.ID, "pago", pagoID, "monto", pago.Monto, "moneda", pago.Moneda)
			w.WriteHeader(http.StatusOK)
			return
		}
		nueva, err := s.alm.PagarLicencia(r.Context(), l.ID, pagoID)
		if err != nil {
			s.interno(w, err)
			return
		}
		if nueva {
			s.log.Info("licencia pagada", "licencia", l.ID, "pago", pagoID)
			s.avisarLicencia(l)
		}
	case "rejected", "cancelled":
		if err := s.alm.RechazarLicencia(r.Context(), l.ID, pagoID); err != nil {
			s.interno(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// avisarLicencia: a Nahuel por Telegram (el chat de las reservas).
func (s *Servidor) avisarLicencia(l almacen.Licencia) {
	if s.integ == nil {
		return
	}
	texto := "🖥 <b>Venta del escritorio</b> · " + monto(l.Monto, l.Moneda) + "\nlicencia " + html.EscapeString(referenciaLicencia(l.ID))
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if err := s.integ.AvisarReserva(ctx, texto); err != nil {
			s.log.Warn("licencia: no la pude avisar por Telegram", "id", l.ID, "error", err)
		}
	}()
}
