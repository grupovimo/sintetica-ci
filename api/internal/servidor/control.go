package servidor

// El centro de control del dueño (/admin en la web): componentes con su costo
// y dónde se compran, la lista de materiales de cada modelo, las compras, las
// unidades (de armada a desplegada, atadas a su robot) y la preventa. Todo solo
// para administradores (conAdmin).

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
)

func (s *Servidor) rutasControl(m *http.ServeMux) {
	m.HandleFunc("GET /v1/admin/resumen", s.conAdmin(s.resumenControl))
	m.HandleFunc("GET /v1/admin/dolar", s.conAdmin(s.verDolar))
	m.HandleFunc("GET /v1/admin/componentes", s.conAdmin(s.listarComponentes))
	m.HandleFunc("POST /v1/admin/componentes", s.conAdmin(s.guardarComponente))
	m.HandleFunc("PUT /v1/admin/componentes/{id}", s.conAdmin(s.guardarComponente))
	m.HandleFunc("DELETE /v1/admin/componentes/{id}", s.conAdmin(s.borrarComponente))
	m.HandleFunc("PUT /v1/admin/modelos/{modelo}/materiales", s.conAdmin(s.guardarMateriales))
	m.HandleFunc("GET /v1/admin/compras", s.conAdmin(s.listarCompras))
	m.HandleFunc("POST /v1/admin/compras", s.conAdmin(s.crearCompra))
	m.HandleFunc("PATCH /v1/admin/compras/{id}", s.conAdmin(s.recibirCompra))
	m.HandleFunc("DELETE /v1/admin/compras/{id}", s.conAdmin(s.borrarCompra))
	m.HandleFunc("GET /v1/admin/unidades", s.conAdmin(s.listarUnidades))
	m.HandleFunc("POST /v1/admin/unidades", s.conAdmin(s.armarUnidad))
	m.HandleFunc("PATCH /v1/admin/unidades/{id}", s.conAdmin(s.cambiarUnidad))
	m.HandleFunc("DELETE /v1/admin/unidades/{id}", s.conAdmin(s.borrarUnidad))
	m.HandleFunc("GET /v1/admin/bichos", s.conAdmin(s.listarBichosAdmin))
	m.HandleFunc("GET /v1/admin/preventa", s.conAdmin(s.verPreventa))
	m.HandleFunc("PUT /v1/admin/preventa", s.conAdmin(s.guardarPreventa))
	m.HandleFunc("GET /v1/admin/senas", s.conAdmin(s.listarSenas))
	m.HandleFunc("PATCH /v1/admin/senas/{id}", s.conAdmin(s.cambiarSena))
}

// conAdmin: con sesión y administrador.
func (s *Servidor) conAdmin(h http.HandlerFunc) http.HandlerFunc {
	return s.conSesion(func(w http.ResponseWriter, r *http.Request) {
		if !usuarioDe(r).Usuario.Admin {
			fallar(w, http.StatusForbidden, "solo para administradores")
			return
		}
		h(w, r)
	})
}

var (
	monedas         = map[string]bool{"USD": true, "ARS": true}
	estadosUnidad   = map[string]bool{"armada": true, "vendida": true, "desplegada": true, "garantia": true, "devuelta": true, "baja": true}
	reFecha         = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reSerie         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)
	reBicho         = regexp.MustCompile(`^[0-9A-F]{12}$`)
	errLinkInvalido = errors.New("el link tiene que empezar con http:// o https://")
)

func idDe(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fallar(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func linkValido(l string) error {
	if l == "" {
		return nil
	}
	u, err := url.Parse(l)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(l) > 500 {
		return errLinkInvalido
	}
	return nil
}

func largo(s string, max int) bool { return utf8.RuneCountInString(s) <= max }

func numeroValido(v float64) bool { return v >= 0 && v < 1e12 && !math.IsNaN(v) && !math.IsInf(v, 0) }

func fechaValida(f string) bool {
	if f == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", f)
	return reFecha.MatchString(f) && err == nil
}

// --- Resumen ---

type costoModelo struct {
	Modelo     string  `json:"modelo"`
	USD        float64 `json:"usd"`
	ARS        float64 `json:"ars"`
	PuedoArmar int     `json:"puedoArmar"`
	Piezas     int     `json:"piezas"`
}

// GET /v1/admin/resumen: los números de la portada del control.
func (s *Servidor) resumenControl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dolar, fechaDolar, errDolar := s.dolar(ctx)
	unidades, err := s.alm.Unidades(ctx)
	if err != nil {
		s.interno(w, err)
		return
	}
	comps, err := s.alm.Componentes(ctx)
	if err != nil {
		s.interno(w, err)
		return
	}
	mats, err := s.alm.Materiales(ctx)
	if err != nil {
		s.interno(w, err)
		return
	}
	senas, err := s.alm.Senas(ctx, 1000)
	if err != nil {
		s.interno(w, err)
		return
	}
	pv, err := s.alm.VerPreventa(ctx, "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	tomados, err := s.alm.CuposTomados(ctx, "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}

	// El precio en pesos: si fue en dólares, con el dólar congelado al armarla
	// (o el de hoy).
	aARS := func(u almacen.Unidad) float64 {
		if u.Moneda != "USD" {
			return u.Precio
		}
		if u.Dolar > 0 {
			return u.Precio * u.Dolar
		}
		return u.Precio * dolar
	}
	mes := time.Now().In(horaArgentina).Format("2006-01")
	estados := map[string]int{}
	var vendidasMes int
	var ventasMes, cobradoMes, porCobrar, margenMes, invertido float64
	for _, u := range unidades {
		estados[u.Estado]++
		vendida := u.Estado != "armada" && u.Estado != "baja" && u.VendidaEn != ""
		if u.Estado != "baja" {
			invertido += u.CostoARS
		}
		if !vendida || u.Estado == "devuelta" {
			continue
		}
		precio := aARS(u)
		if !u.Cobrado {
			porCobrar += precio
		}
		if strings.HasPrefix(u.VendidaEn, mes) {
			vendidasMes++
			ventasMes += precio
			margenMes += precio - u.CostoARS
			if u.Cobrado {
				cobradoMes += precio
			}
		}
	}

	porID := map[int64]almacen.Componente{}
	bajo := []almacen.Componente{}
	for _, c := range comps {
		porID[c.ID] = c
		if c.StockMinimo > 0 && c.Stock <= c.StockMinimo {
			bajo = append(bajo, c)
		}
	}
	modelos := map[string]*costoModelo{}
	orden := []string{}
	for _, m := range mats {
		c, ok := porID[m.ComponenteID]
		if !ok {
			continue
		}
		cm := modelos[m.Modelo]
		if cm == nil {
			cm = &costoModelo{Modelo: m.Modelo, PuedoArmar: math.MaxInt32}
			modelos[m.Modelo] = cm
			orden = append(orden, m.Modelo)
		}
		cm.Piezas++
		costo := m.Cantidad * c.Costo
		if c.Moneda == "USD" {
			cm.USD += costo
			cm.ARS += costo * dolar
		} else {
			cm.ARS += costo
			if dolar > 0 {
				cm.USD += costo / dolar
			}
		}
		cm.PuedoArmar = min(cm.PuedoArmar, max(c.Stock, 0)/int(math.Ceil(m.Cantidad)))
	}
	costos := []costoModelo{}
	for _, k := range orden {
		cm := modelos[k]
		cm.USD, cm.ARS = math.Round(cm.USD*100)/100, math.Round(cm.ARS*100)/100
		costos = append(costos, *cm)
	}

	var senasAprobadas int
	var senado float64
	for _, x := range senas {
		if x.Estado == "aprobada" {
			senasAprobadas++
			senado += x.Monto
		}
	}
	res := map[string]any{
		"dolar": dolar, "dolarFecha": fechaDolar, "dolarError": errDolar != nil,
		"mes": mes, "estados": estados, "unidades": len(unidades),
		"vendidasMes": vendidasMes, "ventasMes": ventasMes, "cobradoMes": cobradoMes, "margenMes": margenMes,
		"porCobrar": porCobrar, "invertido": invertido,
		"costos": costos, "stockBajo": bajo,
		"preventa": map[string]any{"activa": pv.Activa, "cupos": pv.Cupos, "quedan": max(pv.Cupos-tomados, 0),
			"senas": senasAprobadas, "senado": senado, "conMercadoPago": s.op.MPToken != ""},
	}
	responder(w, http.StatusOK, res)
}

// --- Componentes ---

// GET /v1/admin/componentes → {componentes, materiales}.
func (s *Servidor) listarComponentes(w http.ResponseWriter, r *http.Request) {
	c, err := s.alm.Componentes(r.Context())
	if err != nil {
		s.interno(w, err)
		return
	}
	m, err := s.alm.Materiales(r.Context())
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"componentes": c, "materiales": m})
}

// POST /v1/admin/componentes y PUT /v1/admin/componentes/{id}: el componente entero.
func (s *Servidor) guardarComponente(w http.ResponseWriter, r *http.Request) {
	var c almacen.Componente
	if r.PathValue("id") != "" {
		id, ok := idDe(w, r)
		if !ok {
			return
		}
		c.ID = id
	}
	var p struct {
		Nombre      string  `json:"nombre"`
		Categoria   string  `json:"categoria"`
		Proveedor   string  `json:"proveedor"`
		Link        string  `json:"link"`
		Moneda      string  `json:"moneda"`
		Costo       float64 `json:"costo"`
		Stock       int     `json:"stock"`
		StockMinimo int     `json:"stockMinimo"`
		Notas       string  `json:"notas"`
	}
	if !leerJSON(w, r, 8192, &p) {
		return
	}
	c.Nombre, c.Categoria, c.Proveedor = strings.TrimSpace(p.Nombre), strings.TrimSpace(p.Categoria), strings.TrimSpace(p.Proveedor)
	c.Link, c.Moneda, c.Costo, c.Stock, c.StockMinimo, c.Notas = strings.TrimSpace(p.Link), p.Moneda, p.Costo, p.Stock, p.StockMinimo, strings.TrimSpace(p.Notas)
	switch {
	case c.Nombre == "" || !largo(c.Nombre, 120):
		fallar(w, http.StatusBadRequest, "el nombre va de 1 a 120 letras")
		return
	case !largo(c.Categoria, 60) || !largo(c.Proveedor, 120) || !largo(c.Notas, 2000):
		fallar(w, http.StatusBadRequest, "algún texto es muy largo")
		return
	case linkValido(c.Link) != nil:
		fallar(w, http.StatusBadRequest, errLinkInvalido.Error())
		return
	case !monedas[c.Moneda]:
		fallar(w, http.StatusBadRequest, "la moneda es USD o ARS")
		return
	case !numeroValido(c.Costo):
		fallar(w, http.StatusBadRequest, "el costo no puede ser negativo")
		return
	case c.StockMinimo < 0 || c.Stock < -1e6 || c.Stock > 1e7 || c.StockMinimo > 1e7:
		fallar(w, http.StatusBadRequest, "el stock no cierra")
		return
	}
	n, err := s.alm.GuardarComponente(r.Context(), c)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "ese componente no existe")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	estado := http.StatusOK
	if c.ID == 0 {
		estado = http.StatusCreated
	}
	responder(w, estado, n)
}

// DELETE /v1/admin/componentes/{id}.
func (s *Servidor) borrarComponente(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	if err := s.alm.BorrarComponente(r.Context(), id); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "ese componente no existe")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /v1/admin/modelos/{modelo}/materiales {materiales: [{componenteId, cantidad}]}.
func (s *Servidor) guardarMateriales(w http.ResponseWriter, r *http.Request) {
	modelo := r.PathValue("modelo")
	if !piezas[modelo] {
		fallar(w, http.StatusBadRequest, "ese modelo no existe")
		return
	}
	var p struct {
		Materiales []almacen.Material `json:"materiales"`
	}
	if !leerJSON(w, r, 32<<10, &p) {
		return
	}
	if len(p.Materiales) > 200 {
		fallar(w, http.StatusBadRequest, "son muchos componentes")
		return
	}
	vistos := map[int64]bool{}
	for _, m := range p.Materiales {
		if m.Cantidad <= 0 || m.Cantidad > 10000 || !numeroValido(m.Cantidad) {
			fallar(w, http.StatusBadRequest, "la cantidad tiene que ser mayor a cero")
			return
		}
		if vistos[m.ComponenteID] {
			fallar(w, http.StatusBadRequest, "hay un componente repetido")
			return
		}
		vistos[m.ComponenteID] = true
	}
	if err := s.alm.GuardarMateriales(r.Context(), modelo, p.Materiales); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusBadRequest, "algún componente no existe")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	s.listarComponentes(w, r)
}

// --- Compras ---

// GET /v1/admin/compras.
func (s *Servidor) listarCompras(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Compras(r.Context(), 1000)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"compras": l})
}

// POST /v1/admin/compras {componenteId, cantidad, costoUnitario, moneda, proveedor, link, pedida, notas}.
func (s *Servidor) crearCompra(w http.ResponseWriter, r *http.Request) {
	var c almacen.Compra
	if !leerJSON(w, r, 8192, &c) {
		return
	}
	c.Proveedor, c.Link, c.Notas = strings.TrimSpace(c.Proveedor), strings.TrimSpace(c.Link), strings.TrimSpace(c.Notas)
	switch {
	case c.Cantidad <= 0 || c.Cantidad > 1e7:
		fallar(w, http.StatusBadRequest, "la cantidad tiene que ser mayor a cero")
		return
	case !numeroValido(c.CostoUnitario):
		fallar(w, http.StatusBadRequest, "el costo no puede ser negativo")
		return
	case !monedas[c.Moneda]:
		fallar(w, http.StatusBadRequest, "la moneda es USD o ARS")
		return
	case linkValido(c.Link) != nil:
		fallar(w, http.StatusBadRequest, errLinkInvalido.Error())
		return
	case !fechaValida(c.Pedida):
		fallar(w, http.StatusBadRequest, "la fecha va como AAAA-MM-DD")
		return
	case !largo(c.Proveedor, 120) || !largo(c.Notas, 2000):
		fallar(w, http.StatusBadRequest, "algún texto es muy largo")
		return
	}
	n, err := s.alm.CrearCompra(r.Context(), c)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusBadRequest, "ese componente no existe")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusCreated, n)
}

// PATCH /v1/admin/compras/{id} {recibida}: llegó (suma al stock) o no.
func (s *Servidor) recibirCompra(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var p struct {
		Recibida bool `json:"recibida"`
	}
	if !leerJSON(w, r, 256, &p) {
		return
	}
	c, err := s.alm.RecibirCompra(r.Context(), id, p.Recibida)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "esa compra no existe")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, c)
}

// DELETE /v1/admin/compras/{id}.
func (s *Servidor) borrarCompra(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	if err := s.alm.BorrarCompra(r.Context(), id); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "esa compra no existe")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Unidades ---

// GET /v1/admin/unidades.
func (s *Servidor) listarUnidades(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Unidades(r.Context())
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"unidades": l})
}

// POST /v1/admin/unidades {serie, modelo, color, notas, descontar, forzar}:
// armar una. Congela el costo con el dólar de hoy.
func (s *Servidor) armarUnidad(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Serie     string `json:"serie"`
		Modelo    string `json:"modelo"`
		Color     string `json:"color"`
		Notas     string `json:"notas"`
		Descontar bool   `json:"descontar"`
		Forzar    bool   `json:"forzar"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	u := almacen.Unidad{Serie: strings.TrimSpace(p.Serie), Modelo: p.Modelo, Color: p.Color, Notas: strings.TrimSpace(p.Notas)}
	switch {
	case !reSerie.MatchString(u.Serie):
		fallar(w, http.StatusBadRequest, "el número de serie va con letras, números, punto o guion (hasta 32)")
		return
	case !piezas[u.Modelo]:
		fallar(w, http.StatusBadRequest, "ese modelo no existe")
		return
	case u.Color != "" && !coloresCarcasa[u.Color]:
		fallar(w, http.StatusBadRequest, "el color es naranja, negro o blanco")
		return
	case !largo(u.Notas, 2000):
		fallar(w, http.StatusBadRequest, "las notas son muy largas")
		return
	}
	dolar, _, err := s.dolar(r.Context())
	if err != nil {
		fallar(w, http.StatusBadGateway, "no pude traer el dólar para congelar el costo", err.Error())
		return
	}
	n, err := s.alm.ArmarUnidad(r.Context(), u, dolar, p.Descontar, p.Forzar)
	switch {
	case errors.Is(err, almacen.ErrSinStock):
		fallar(w, http.StatusConflict, "no alcanza el stock", strings.TrimPrefix(err.Error(), almacen.ErrSinStock.Error()+": "))
		return
	case errors.Is(err, almacen.ErrSerieRepetida):
		fallar(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		s.interno(w, err)
		return
	}
	responder(w, http.StatusCreated, n)
}

// PATCH /v1/admin/unidades/{id}: estado, venta, robot, notas.
func (s *Servidor) cambiarUnidad(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var c almacen.CambioUnidad
	if !leerJSON(w, r, 8192, &c) {
		return
	}
	textos := []*string{c.Cliente, c.Contacto, c.Canal}
	for _, t := range textos {
		if t != nil && !largo(*t, 200) {
			fallar(w, http.StatusBadRequest, "algún texto es muy largo")
			return
		}
	}
	switch {
	case c.Serie != nil && !reSerie.MatchString(*c.Serie):
		fallar(w, http.StatusBadRequest, "el número de serie va con letras, números, punto o guion (hasta 32)")
		return
	case c.Color != nil && *c.Color != "" && !coloresCarcasa[*c.Color]:
		fallar(w, http.StatusBadRequest, "el color es naranja, negro o blanco")
		return
	case c.Estado != nil && !estadosUnidad[*c.Estado]:
		fallar(w, http.StatusBadRequest, "ese estado no existe")
		return
	case c.BichoID != nil && *c.BichoID != "" && !reBicho.MatchString(*c.BichoID):
		fallar(w, http.StatusBadRequest, "ese robot no existe")
		return
	case c.Precio != nil && !numeroValido(*c.Precio):
		fallar(w, http.StatusBadRequest, "el precio no puede ser negativo")
		return
	case c.Moneda != nil && !monedas[*c.Moneda]:
		fallar(w, http.StatusBadRequest, "la moneda es USD o ARS")
		return
	case (c.VendidaEn != nil && !fechaValida(*c.VendidaEn)) || (c.DesplegadaEn != nil && !fechaValida(*c.DesplegadaEn)):
		fallar(w, http.StatusBadRequest, "la fecha va como AAAA-MM-DD")
		return
	case c.Notas != nil && !largo(*c.Notas, 2000):
		fallar(w, http.StatusBadRequest, "las notas son muy largas")
		return
	}
	u, err := s.alm.CambiarUnidad(r.Context(), id, c)
	switch {
	case errors.Is(err, almacen.ErrNoEncontrado):
		fallar(w, http.StatusNotFound, "esa unidad (o ese robot) no existe")
		return
	case errors.Is(err, almacen.ErrSerieRepetida), errors.Is(err, almacen.ErrBichoOcupado):
		fallar(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, u)
}

// DELETE /v1/admin/unidades/{id}.
func (s *Servidor) borrarUnidad(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	if err := s.alm.BorrarUnidad(r.Context(), id); errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "esa unidad no existe")
		return
	} else if err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/admin/bichos: todos los robots de la nube.
func (s *Servidor) listarBichosAdmin(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.BichosAdmin(r.Context())
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"bichos": l})
}

// --- Preventa (lo del dueño; lo público está en preventa.go) ---

// GET /v1/admin/preventa → la preventa de la Unidad + {conMercadoPago, tomados}.
func (s *Servidor) verPreventa(w http.ResponseWriter, r *http.Request) {
	p, err := s.alm.VerPreventa(r.Context(), "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	tomados, err := s.alm.CuposTomados(r.Context(), "unidad")
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"preventa": p, "tomados": tomados, "conMercadoPago": s.op.MPToken != ""})
}

// PUT /v1/admin/preventa {activa, precio, precioFinal, sena, cupos, entrega}.
func (s *Servidor) guardarPreventa(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Activa      bool    `json:"activa"`
		Precio      float64 `json:"precio"`
		PrecioFinal float64 `json:"precioFinal"`
		Sena        float64 `json:"sena"`
		Cupos       int     `json:"cupos"`
		Entrega     string  `json:"entrega"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	p.Entrega = strings.TrimSpace(p.Entrega)
	switch {
	case !numeroValido(p.Precio) || !numeroValido(p.PrecioFinal) || !numeroValido(p.Sena):
		fallar(w, http.StatusBadRequest, "los montos no pueden ser negativos")
		return
	case p.Cupos < 0 || p.Cupos > 100000:
		fallar(w, http.StatusBadRequest, "los cupos no cierran")
		return
	case !largo(p.Entrega, 60):
		fallar(w, http.StatusBadRequest, "la entrega es de hasta 60 letras")
		return
	case p.Activa && (p.Sena <= 0 || p.Precio <= 0 || p.Cupos <= 0):
		fallar(w, http.StatusBadRequest, "para prenderla falta el precio, la seña y los cupos")
		return
	case p.Sena > p.Precio && p.Precio > 0:
		fallar(w, http.StatusBadRequest, "la seña no puede ser más que el precio")
		return
	}
	n, err := s.alm.GuardarPreventa(r.Context(), almacen.Preventa{Modelo: "unidad", Activa: p.Activa, Precio: p.Precio,
		PrecioFinal: p.PrecioFinal, Sena: p.Sena, Cupos: p.Cupos, Entrega: p.Entrega})
	if err != nil {
		s.interno(w, err)
		return
	}
	s.log.Info("preventa", "activa", n.Activa, "precio", n.Precio, "sena", n.Sena, "cupos", n.Cupos)
	s.verPreventa(w, r)
}

// GET /v1/admin/senas.
func (s *Servidor) listarSenas(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Senas(r.Context(), 1000)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"senas": l})
}

// PATCH /v1/admin/senas/{id} {estado?: "devuelta"|"aprobada", unidadId?}.
func (s *Servidor) cambiarSena(w http.ResponseWriter, r *http.Request) {
	id, ok := idDe(w, r)
	if !ok {
		return
	}
	var p struct {
		Estado   *string `json:"estado"`
		UnidadID *int64  `json:"unidadId"`
	}
	if !leerJSON(w, r, 256, &p) {
		return
	}
	if p.Estado != nil && *p.Estado != "devuelta" && *p.Estado != "aprobada" {
		fallar(w, http.StatusBadRequest, "una seña se marca devuelta o aprobada")
		return
	}
	n, err := s.alm.CambiarSena(r.Context(), id, p.Estado, p.UnidadID)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "esa seña (o esa unidad) no existe")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, n)
}
