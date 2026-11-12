package servidor

// La preventa del cubito en la landing: los números (que pone el dueño en
// /admin/preventa), la seña con Mercado Pago (Checkout Pro, en pesos) y el
// aviso de Mercado Pago cuando el pago sale. Sin MP_ACCESS_TOKEN la preventa
// igual se muestra, pero la seña queda como reserva (sin cobrar).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
)

var (
	reEmail  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	rePagoMP = regexp.MustCompile(`^[0-9]{1,20}$`)
)

// horaArgentina: el "hoy" del control.
var horaArgentina = time.FixedZone("ART", -3*60*60)

func (s *Servidor) baseMP() string {
	if s.op.MercadoPagoURL != "" {
		return strings.TrimRight(s.op.MercadoPagoURL, "/")
	}
	return "https://api.mercadopago.com"
}

// GET /v1/preventa → {activa, precio, precioFinal, sena, cupos, quedan, entrega}.
// Público: lo lee la landing.
func (s *Servidor) verPreventaPublica(w http.ResponseWriter, r *http.Request) {
	p, err := s.alm.VerPreventa(r.Context(), "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	activa := p.Activa && p.Sena > 0
	if !activa {
		responder(w, http.StatusOK, map[string]any{"activa": false})
		return
	}
	tomados, err := s.alm.CuposTomados(r.Context(), "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"activa": true, "modelo": "unidad", "precio": p.Precio, "precioFinal": p.PrecioFinal,
		"sena": p.Sena, "cupos": p.Cupos, "quedan": max(p.Cupos-tomados, 0), "entrega": p.Entrega})
}

// POST /v1/preventa/senas {nombre, contacto, email, color, sitio} → {url}: la
// seña pendiente y el link de pago de Mercado Pago. "sitio" es la trampa para
// bots, como en las reservas.
func (s *Servidor) senar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Nombre   string `json:"nombre"`
		Contacto string `json:"contacto"`
		Email    string `json:"email"`
		Color    string `json:"color"`
		Sitio    string `json:"sitio"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	sena := almacen.Sena{Modelo: "unidad", Nombre: strings.TrimSpace(p.Nombre), Contacto: strings.TrimSpace(p.Contacto),
		Email: strings.ToLower(strings.TrimSpace(p.Email)), Color: strings.TrimSpace(p.Color)}
	switch {
	case sena.Nombre == "" || utf8.RuneCountInString(sena.Nombre) > 80:
		fallar(w, http.StatusBadRequest, "decinos tu nombre (hasta 80 letras)")
		return
	case utf8.RuneCountInString(sena.Contacto) < 5 || utf8.RuneCountInString(sena.Contacto) > 120:
		fallar(w, http.StatusBadRequest, "dejanos un email o un WhatsApp para escribirte")
		return
	case sena.Email != "" && (!reEmail.MatchString(sena.Email) || len(sena.Email) > 120):
		fallar(w, http.StatusBadRequest, "ese email no parece válido")
		return
	case !coloresCarcasa[sena.Color]:
		fallar(w, http.StatusBadRequest, "el color es naranja, negro o blanco")
		return
	}
	if sena.Email == "" && reEmail.MatchString(sena.Contacto) {
		sena.Email = strings.ToLower(sena.Contacto)
	}
	pv, err := s.alm.VerPreventa(r.Context(), "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	if !pv.Activa || pv.Sena <= 0 {
		fallar(w, http.StatusConflict, "la preventa no está abierta")
		return
	}
	if p.Sitio != "" {
		fallar(w, http.StatusConflict, "la preventa no está abierta")
		return
	}
	ip := ipDe(r)
	if !s.limites.permitir("sena:"+ip, 6, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "probaste muchas veces: esperá un rato")
		return
	}
	if n, err := s.alm.SenasDeIP(r.Context(), ip, time.Now().Add(-24*time.Hour)); err != nil {
		s.interno(w, err)
		return
	} else if n >= 10 {
		fallar(w, http.StatusTooManyRequests, "probaste muchas veces: esperá un rato")
		return
	}
	if s.op.MPToken == "" {
		s.reservarPreventa(w, r, sena, ip)
		return
	}
	sena.Monto = pv.Sena
	n, err := s.alm.CrearSena(r.Context(), sena, ip)
	if errors.Is(err, almacen.ErrSinCupos) {
		fallar(w, http.StatusConflict, "se llenaron los cupos de la preventa")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	pref, link, err := s.crearPreferencia(r.Context(), n)
	if err != nil {
		s.log.Warn("preventa: Mercado Pago no armó el pago", "sena", n.ID, "error", err)
		s.alm.RechazarSena(r.Context(), n.ID, "") //nolint:errcheck // libera el cupo
		fallar(w, http.StatusBadGateway, "Mercado Pago no contestó: probá de nuevo en un rato")
		return
	}
	if err := s.alm.PonerPreferencia(r.Context(), n.ID, pref); err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("seña nueva", "id", n.ID, "color", n.Color, "monto", n.Monto)
	responder(w, http.StatusCreated, map[string]any{"url": link, "id": n.ID})
}

// reservarPreventa: sin Mercado Pago todavía, la seña queda como reserva de la
// Unidad (sin cobrar) y avisa por Telegram → {ok}.
func (s *Servidor) reservarPreventa(w http.ResponseWriter, r *http.Request, sena almacen.Sena, ip string) {
	id, err := s.alm.CrearReserva(r.Context(), almacen.Reserva{Nombre: sena.Nombre, Pieza: "unidad", Contacto: sena.Contacto,
		Color: sena.Color, Mensaje: "Preventa"}, ip)
	if err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("reserva de preventa", "id", id, "color", sena.Color)
	if s.integ != nil {
		texto := "🧡 <b>Reserva de preventa</b> · unidad " + html.EscapeString(sena.Color) +
			"\n<b>" + html.EscapeString(sena.Nombre) + "</b>\n" + html.EscapeString(sena.Contacto)
		go func() {
			ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelar()
			if err := s.integ.AvisarReserva(ctx, texto); err != nil {
				s.log.Warn("reserva de preventa: no la pude avisar por Telegram", "id", id, "error", err)
			}
		}()
	}
	responder(w, http.StatusCreated, map[string]any{"ok": true})
}

// referenciaSena: el external_reference del pago ("sena-<id>").
func referenciaSena(id int64) string { return "sena-" + strconv.FormatInt(id, 10) }

// crearPreferencia arma el pago de una seña → (id, init_point).
func (s *Servidor) crearPreferencia(ctx context.Context, sena almacen.Sena) (string, string, error) {
	web := strings.TrimRight(s.op.WebURL, "/")
	return s.preferenciaMP(ctx, pedidoMP{
		Item: "sena-unidad", Titulo: "Seña · Unidad Sintética (preventa)", Monto: sena.Monto,
		Referencia: referenciaSena(sena.ID), Volver: web + "/?sena=%s#reservar", Email: sena.Email, Nombre: sena.Nombre,
	})
}

// pedidoMP: un cobro de Checkout Pro, en pesos. Volver lleva un %s que se
// reemplaza por ok, pendiente o error.
type pedidoMP struct {
	Item, Titulo       string
	Monto              float64
	Referencia, Volver string
	Email, Nombre      string
}

// preferenciaMP arma el pago en Mercado Pago (Checkout Pro) → (id, init_point).
// El aviso llega a /v1/preventa/mp, que mira el prefijo de la referencia.
func (s *Servidor) preferenciaMP(ctx context.Context, p pedidoMP) (string, string, error) {
	volver := func(e string) string { return fmt.Sprintf(p.Volver, e) }
	cuerpo := map[string]any{
		"items": []map[string]any{{
			"id": p.Item, "title": p.Titulo, "quantity": 1, "unit_price": p.Monto, "currency_id": "ARS",
		}},
		"external_reference":   p.Referencia,
		"statement_descriptor": "SINTETICA",
		"back_urls":            map[string]string{"success": volver("ok"), "pending": volver("pendiente"), "failure": volver("error")},
		"expires":              true, "expiration_date_to": time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04:05.000-07:00"),
	}
	if strings.HasPrefix(p.Volver, "https://") {
		cuerpo["auto_return"] = "approved" // MP no lo acepta con back_urls http
	}
	if u := strings.TrimRight(s.op.URLPublica, "/"); strings.HasPrefix(u, "https://") {
		cuerpo["notification_url"] = u + "/v1/preventa/mp?source_news=webhooks"
	}
	if p.Email != "" {
		cuerpo["payer"] = map[string]any{"email": p.Email, "name": p.Nombre}
	}
	b, _ := json.Marshal(cuerpo)
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseMP()+"/checkout/preferences", bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.op.MPToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", p.Referencia)
	res, err := clienteMP.Do(req)
	if err != nil {
		return "", "", errors.New("Mercado Pago no contestó")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		detalle, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", "", fmt.Errorf("Mercado Pago contestó %d: %s", res.StatusCode, detalle)
	}
	var r struct {
		ID        string `json:"id"`
		InitPoint string `json:"init_point"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&r); err != nil {
		return "", "", err
	}
	if r.ID == "" || !strings.HasPrefix(r.InitPoint, "https://") {
		return "", "", errors.New("Mercado Pago no devolvió el link")
	}
	return r.ID, r.InitPoint, nil
}

// POST /v1/preventa/mp: el aviso de Mercado Pago. No se le cree al cuerpo: con
// el id del pago se lo pide a Mercado Pago con el token y recién ahí se marca.
// Llega como ?type=payment&data.id=N (webhooks), ?topic=payment&id=N (IPN) o
// con {"type":"payment","data":{"id":"N"}} en el cuerpo.
func (s *Servidor) avisoMP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tipo, id := q.Get("type"), q.Get("data.id")
	if tipo == "" {
		tipo, id = q.Get("topic"), q.Get("id")
	}
	var c struct {
		Type   string `json:"type"`
		Action string `json:"action"`
		Data   struct {
			ID json.RawMessage `json:"id"`
		} `json:"data"`
	}
	if b, _ := io.ReadAll(io.LimitReader(r.Body, 16<<10)); len(b) > 0 && json.Unmarshal(b, &c) == nil {
		if tipo == "" {
			tipo = c.Type
		}
		if id == "" {
			id = strings.Trim(string(c.Data.ID), `"`)
		}
	}
	if tipo != "payment" || !rePagoMP.MatchString(id) || s.op.MPToken == "" {
		w.WriteHeader(http.StatusOK) // otros avisos (merchant_order, etc.): nada que hacer
		return
	}
	if !s.limites.permitir("mp-aviso:"+ipDe(r), 120, time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados avisos")
		return
	}
	pago, err := s.pagoSena(r.Context(), id)
	if err != nil {
		s.log.Warn("preventa: no pude leer el pago", "pago", id, "error", err)
		fallar(w, http.StatusBadGateway, "no pude leer el pago") // Mercado Pago reintenta
		return
	}
	if strings.HasPrefix(pago.Referencia, "lic-") {
		s.avisoLicencia(w, r, pago, id)
		return
	}
	senaID, err := strconv.ParseInt(strings.TrimPrefix(pago.Referencia, "sena-"), 10, 64)
	if !strings.HasPrefix(pago.Referencia, "sena-") || err != nil {
		w.WriteHeader(http.StatusOK) // un pago que no es de la preventa
		return
	}
	sena, err := s.alm.VerSena(r.Context(), senaID)
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
		if pago.Moneda != "ARS" || pago.Monto+0.005 < sena.Monto {
			s.log.Warn("preventa: el pago no coincide con la seña", "sena", sena.ID, "pago", id, "monto", pago.Monto, "moneda", pago.Moneda)
			w.WriteHeader(http.StatusOK)
			return
		}
		nueva, err := s.alm.PagarSena(r.Context(), sena.ID, id)
		if err != nil {
			s.interno(w, err)
			return
		}
		if nueva {
			s.log.Info("seña pagada", "sena", sena.ID, "pago", id)
			s.avisarSena(sena)
		}
	case "rejected", "cancelled":
		if err := s.alm.RechazarSena(r.Context(), sena.ID, id); err != nil {
			s.interno(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

type pagoPreventa struct {
	Status     string  `json:"status"`
	Monto      float64 `json:"transaction_amount"`
	Moneda     string  `json:"currency_id"`
	Referencia string  `json:"external_reference"`
}

// pagoSena lee un pago con el token de la cuenta de Sintética.
func (s *Servidor) pagoSena(ctx context.Context, id string) (pagoPreventa, error) {
	ctx, cancelar := context.WithTimeout(ctx, 8*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseMP()+"/v1/payments/"+id, nil)
	if err != nil {
		return pagoPreventa{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.op.MPToken)
	res, err := clienteMP.Do(req)
	if err != nil {
		return pagoPreventa{}, errors.New("Mercado Pago no contestó")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return pagoPreventa{}, fmt.Errorf("Mercado Pago contestó %d", res.StatusCode)
	}
	var p pagoPreventa
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&p)
	return p, err
}

// avisarSena: a Nahuel por Telegram (el chat de las reservas).
func (s *Servidor) avisarSena(sena almacen.Sena) {
	if s.integ == nil {
		return
	}
	texto := "💸 <b>Seña del cubito</b> · " + monto(sena.Monto, "ARS") + "\n<b>" + html.EscapeString(sena.Nombre) + "</b> · " +
		html.EscapeString(sena.Color) + "\n" + html.EscapeString(sena.Contacto)
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if err := s.integ.AvisarReserva(ctx, texto); err != nil {
			s.log.Warn("seña: no la pude avisar por Telegram", "id", sena.ID, "error", err)
		}
	}()
}
