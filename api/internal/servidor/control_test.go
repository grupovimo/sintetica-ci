package servidor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// adminDePrueba: un administrador con una sesión de app.
func adminDePrueba(t *testing.T, s *Servidor) string {
	t.Helper()
	ctx := context.Background()
	u, err := s.alm.UsuarioPorEmail(ctx, strings.ToLower(azarHex(4))+"@admin.ejemplo.com", true)
	if err != nil {
		t.Fatal(err)
	}
	token := aleatorio(24)
	if err := s.alm.CrearSesion(ctx, hash(token), u.ID, "app", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return token
}

// dolarFalso: DolarApi con el oficial a 1000.
func dolarFalso(t *testing.T, s *Servidor) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"moneda":"USD","casa":"oficial","compra":950,"venta":1000,"fechaActualizacion":"2026-11-10T12:00:00.000Z"}`)
	}))
	t.Cleanup(srv.Close)
	s.op.DolarURL = srv.URL
}

func TestControlSoloAdmin(t *testing.T) {
	s, alm := servidorDePrueba(t)
	h := s.Rutas()
	token, _ := cuentaDePrueba(t, alm)
	for _, ruta := range []string{"/v1/admin/resumen", "/v1/admin/componentes", "/v1/admin/unidades", "/v1/admin/senas", "/v1/admin/bichos"} {
		if cod, _ := pedir(t, h, token, "GET", ruta, ""); cod != http.StatusForbidden {
			t.Errorf("%s sin ser admin: %d", ruta, cod)
		}
		if cod, _ := pedir(t, h, "", "GET", ruta, ""); cod != http.StatusUnauthorized {
			t.Errorf("%s sin sesión: %d", ruta, cod)
		}
	}
}

func TestControlComponentesYUnidades(t *testing.T) {
	s, alm := servidorDePrueba(t)
	dolarFalso(t, s)
	h := s.Rutas()
	admin := adminDePrueba(t, s)
	_, bichos := cuentaDePrueba(t, alm)
	modelo := "hebilla" // uno que los otros tests no tocan

	for _, c := range []string{
		`{"nombre":"","moneda":"USD","costo":1}`,
		`{"nombre":"x","moneda":"EUR","costo":1}`,
		`{"nombre":"x","moneda":"USD","costo":-1}`,
		`{"nombre":"x","moneda":"USD","costo":1,"link":"javascript:alert(1)"}`,
	} {
		if cod, v := pedir(t, h, admin, "POST", "/v1/admin/componentes", c); cod != http.StatusBadRequest {
			t.Errorf("esperaba 400 para %s: %d %v", c, cod, v)
		}
	}
	cod, a := pedir(t, h, admin, "POST", "/v1/admin/componentes",
		`{"nombre":"Pieza A","categoria":"Placa","proveedor":"Tienda","link":"https://ejemplo.com/a","moneda":"USD","costo":10,"stock":3,"stockMinimo":1}`)
	if cod != http.StatusCreated {
		t.Fatalf("crear A: %d %v", cod, a)
	}
	_, b := pedir(t, h, admin, "POST", "/v1/admin/componentes", `{"nombre":"Pieza B","moneda":"ARS","costo":500,"stock":10}`)
	idA, idB := int64(a["id"].(float64)), int64(b["id"].(float64))

	ruta := "/v1/admin/modelos/" + modelo + "/materiales"
	if cod, _ := pedir(t, h, admin, "PUT", "/v1/admin/modelos/tostadora/materiales", `{"materiales":[]}`); cod != http.StatusBadRequest {
		t.Errorf("modelo inventado: %d", cod)
	}
	if cod, _ := pedir(t, h, admin, "PUT", ruta, fmt.Sprintf(`{"materiales":[{"componenteId":%d,"cantidad":1},{"componenteId":%d,"cantidad":1}]}`, idA, idA)); cod != http.StatusBadRequest {
		t.Errorf("componente repetido: %d", cod)
	}
	if cod, v := pedir(t, h, admin, "PUT", ruta, fmt.Sprintf(`{"materiales":[{"componenteId":%d,"cantidad":1},{"componenteId":%d,"cantidad":4}]}`, idA, idB)); cod != http.StatusOK {
		t.Fatalf("materiales: %d %v", cod, v)
	}

	// Costo: 10 USD + 4 × 500 ARS = 10 + 2 USD = 12 USD = 12.000 ARS. Alcanza para 2 (B: 10/4).
	_, res := pedir(t, h, admin, "GET", "/v1/admin/resumen", "")
	var costo map[string]any
	for _, c := range res["costos"].([]any) {
		if c.(map[string]any)["modelo"] == modelo {
			costo = c.(map[string]any)
		}
	}
	if costo == nil || costo["usd"] != 12.0 || costo["ars"] != 12000.0 || costo["puedoArmar"] != 2.0 {
		t.Fatalf("costo del modelo: %v (dólar %v)", costo, res["dolar"])
	}

	serie := "T-" + azarHex(4)
	cod, u := pedir(t, h, admin, "POST", "/v1/admin/unidades", `{"serie":"`+serie+`","modelo":"`+modelo+`","color":"naranja","descontar":true}`)
	if cod != http.StatusCreated || u["costoUsd"] != 12.0 || u["costoArs"] != 12000.0 || u["estado"] != "armada" {
		t.Fatalf("armar: %d %v", cod, u)
	}
	if cod, _ := pedir(t, h, admin, "POST", "/v1/admin/unidades", `{"serie":"`+serie+`","modelo":"`+modelo+`"}`); cod != http.StatusConflict {
		t.Errorf("serie repetida: %d", cod)
	}
	pedir(t, h, admin, "POST", "/v1/admin/unidades", `{"serie":"`+serie+`-2","modelo":"`+modelo+`","descontar":true}`)
	// B quedó en 2: la tercera no alcanza (sin forzar).
	if cod, v := pedir(t, h, admin, "POST", "/v1/admin/unidades", `{"serie":"`+serie+`-3","modelo":"`+modelo+`","descontar":true}`); cod != http.StatusConflict || !strings.Contains(fmt.Sprint(v["detalle"]), "Pieza B") {
		t.Errorf("sin stock: %d %v", cod, v)
	}
	_, l := pedir(t, h, admin, "GET", "/v1/admin/componentes", "")
	for _, c := range l["componentes"].([]any) {
		c := c.(map[string]any)
		if c["nombre"] == "Pieza A" && int64(c["id"].(float64)) == idA && c["stock"] != 1.0 {
			t.Errorf("stock de A: %v", c["stock"])
		}
	}

	// La compra que llega suma al stock y deja el precio nuevo.
	cod, compra := pedir(t, h, admin, "POST", "/v1/admin/compras", fmt.Sprintf(`{"componenteId":%d,"cantidad":5,"costoUnitario":11,"moneda":"USD"}`, idA))
	if cod != http.StatusCreated || compra["recibida"] != "" {
		t.Fatalf("compra: %d %v", cod, compra)
	}
	rutaCompra := fmt.Sprintf("/v1/admin/compras/%d", int64(compra["id"].(float64)))
	pedir(t, h, admin, "PATCH", rutaCompra, `{"recibida":true}`)
	pedir(t, h, admin, "PATCH", rutaCompra, `{"recibida":true}`) // dos veces: suma una sola
	_, l = pedir(t, h, admin, "GET", "/v1/admin/componentes", "")
	for _, c := range l["componentes"].([]any) {
		c := c.(map[string]any)
		if int64(c["id"].(float64)) == idA && (c["stock"] != 6.0 || c["costo"] != 11.0) {
			t.Errorf("A después de la compra: %v", c)
		}
	}

	// Vender, atar al robot y desplegar: el historial lo cuenta.
	rutaU := fmt.Sprintf("/v1/admin/unidades/%d", int64(u["id"].(float64)))
	if cod, _ := pedir(t, h, admin, "PATCH", rutaU, `{"estado":"vendidisima"}`); cod != http.StatusBadRequest {
		t.Errorf("estado inventado: %d", cod)
	}
	cod, u = pedir(t, h, admin, "PATCH", rutaU, `{"estado":"vendida","cliente":"Ana","precio":30000,"moneda":"ARS","cobrado":true}`)
	if cod != http.StatusOK || u["vendidaEn"] == "" || u["cobrado"] != true {
		t.Fatalf("vender: %d %v", cod, u)
	}
	if cod, _ := pedir(t, h, admin, "PATCH", rutaU, `{"bichoId":"000000000000"}`); cod != http.StatusNotFound {
		t.Errorf("robot que no existe: %d", cod)
	}
	cod, u = pedir(t, h, admin, "PATCH", rutaU, `{"bichoId":"`+bichos[0]+`","estado":"desplegada"}`)
	if cod != http.StatusOK || u["bichoId"] != bichos[0] || u["desplegadaEn"] == "" {
		t.Fatalf("desplegar: %d %v", cod, u)
	}
	historial := fmt.Sprint(u["historial"])
	for _, x := range []string{"Armada", "Estado: armada → vendida", "Estado: vendida → desplegada", "Robot: " + bichos[0]} {
		if !strings.Contains(historial, x) {
			t.Errorf("al historial le falta %q: %s", x, historial)
		}
	}
	_, u2 := pedir(t, h, admin, "GET", "/v1/admin/unidades", "")
	var otra int64
	for _, x := range u2["unidades"].([]any) {
		if x.(map[string]any)["serie"] == serie+"-2" {
			otra = int64(x.(map[string]any)["id"].(float64))
		}
	}
	if cod, _ := pedir(t, h, admin, "PATCH", fmt.Sprintf("/v1/admin/unidades/%d", otra), `{"bichoId":"`+bichos[0]+`"}`); cod != http.StatusConflict {
		t.Errorf("el mismo robot en dos unidades: %d", cod)
	}
	_, bl := pedir(t, h, admin, "GET", "/v1/admin/bichos", "")
	encontrado := false
	for _, b := range bl["bichos"].([]any) {
		if b.(map[string]any)["id"] == bichos[0] {
			encontrado = b.(map[string]any)["unidad"] == serie
		}
	}
	if !encontrado {
		t.Error("el robot no dice su unidad")
	}
	_, res = pedir(t, h, admin, "GET", "/v1/admin/resumen", "")
	if res["vendidasMes"].(float64) < 1 || res["cobradoMes"].(float64) < 30000 {
		t.Errorf("resumen: %v", res)
	}

	// Borrar el componente lo saca de la lista de materiales.
	if cod, _ := pedir(t, h, admin, "DELETE", fmt.Sprintf("/v1/admin/componentes/%d", idB), ""); cod != http.StatusNoContent {
		t.Errorf("borrar componente: %d", cod)
	}
}

// mpFalso: Mercado Pago de mentira (preferencias y pagos).
type mpFalso struct {
	mu     sync.Mutex
	pagos  map[string]string // id → JSON del pago
	prefs  []map[string]any
	token  string
	falla  bool
	pedido int
}

func (m *mpFalso) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+m.token {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/checkout/preferences":
		if m.falla {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var p map[string]any
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &p) //nolint:errcheck
		m.prefs = append(m.prefs, p)
		m.pedido++
		fmt.Fprintf(w, `{"id":"pref-%d","init_point":"https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=pref-%d"}`, m.pedido, m.pedido)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/payments/"):
		p, ok := m.pagos[strings.TrimPrefix(r.URL.Path, "/v1/payments/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, p)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestPreventa(t *testing.T) {
	s, _ := servidorDePrueba(t)
	h := s.Rutas()
	admin := adminDePrueba(t, s)
	ip := fmt.Sprintf("10.%d.%d.%d", time.Now().UnixNano()%250, time.Now().UnixNano()/7%250, time.Now().UnixNano()/13%250) // el tope de señas es por IP
	senar := func(nombre string) (int, map[string]any) {
		r := httptest.NewRequest("POST", "/v1/preventa/senas", strings.NewReader(`{"nombre":"`+nombre+`","contacto":"ana@ejemplo.com","color":"naranja"}`))
		r.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
		return w.Code, v
	}

	pedir(t, h, admin, "PUT", "/v1/admin/preventa", `{"activa":false,"precio":0,"sena":0,"cupos":0}`)
	// Sin números no se prende.
	if cod, v := pedir(t, h, admin, "PUT", "/v1/admin/preventa", `{"activa":true,"precio":0,"sena":0,"cupos":0}`); cod != http.StatusBadRequest {
		t.Errorf("prender sin números: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, admin, "PUT", "/v1/admin/preventa", `{"activa":true,"precio":100000,"precioFinal":150000,"sena":20000,"cupos":0,"entrega":"marzo"}`); cod != http.StatusBadRequest {
		t.Errorf("prender sin cupos: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, "", "GET", "/v1/preventa", ""); cod != http.StatusOK || v["activa"] != false {
		t.Fatalf("pública sin nada: %d %v", cod, v)
	}

	// Sin token se ve igual y la seña entra como reserva (sin cobrar).
	if cod, v := pedir(t, h, admin, "PUT", "/v1/admin/preventa", `{"activa":true,"precio":100000,"precioFinal":150000,"sena":20000,"cupos":5}`); cod != http.StatusOK {
		t.Fatalf("prender sin token: %d %v", cod, v)
	}
	if _, v := pedir(t, h, "", "GET", "/v1/preventa", ""); v["activa"] != true {
		t.Errorf("sin token no se ve: %v", v)
	}
	if cod, v := senar("Sin token"); cod != http.StatusCreated || v["ok"] != true || v["url"] != nil {
		t.Errorf("seña sin token: %d %v", cod, v)
	}
	s.limites = nuevoLimitador()

	mp := &mpFalso{token: "APP_USR-prueba", pagos: map[string]string{}}
	srv := httptest.NewServer(mp)
	defer srv.Close()
	s.op.MercadoPagoURL, s.op.MPToken = srv.URL, mp.token
	s.op.WebURL, s.op.URLPublica = "https://sintetica.ejemplo.com", "https://api.ejemplo.com"

	// Los cupos de esta prueba: dos más que los tomados en corridas anteriores.
	tomados, _ := s.alm.CuposTomados(context.Background(), "unidad")
	cupos := tomados + 2
	cod, v := pedir(t, h, admin, "PUT", "/v1/admin/preventa", fmt.Sprintf(`{"activa":true,"precio":100000,"precioFinal":150000,"sena":20000,"cupos":%d,"entrega":"marzo"}`, cupos))
	if cod != http.StatusOK || v["conMercadoPago"] != true {
		t.Fatalf("guardar preventa: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, "", "GET", "/v1/preventa", ""); v["activa"] != true || v["quedan"] != 2.0 || v["sena"] != 20000.0 {
		t.Fatalf("pública: %d %v", cod, v)
	}

	// Mercado Pago caído: no se lleva el cupo.
	mp.falla = true
	if cod, _ := senar("Caída"); cod != http.StatusBadGateway {
		t.Errorf("MP caído: %d", cod)
	}
	mp.falla = false
	if _, v := pedir(t, h, "", "GET", "/v1/preventa", ""); v["quedan"] != 2.0 {
		t.Errorf("después de MP caído quedan %v", v["quedan"])
	}

	cod, v = senar("Ana")
	if cod != http.StatusCreated || !strings.HasPrefix(fmt.Sprint(v["url"]), "https://www.mercadopago.com.ar/") {
		t.Fatalf("señar: %d %v", cod, v)
	}
	id := int64(v["id"].(float64))
	pref := mp.prefs[len(mp.prefs)-1]
	if pref["external_reference"] != fmt.Sprintf("sena-%d", id) || pref["notification_url"] != "https://api.ejemplo.com/v1/preventa/mp?source_news=webhooks" ||
		pref["items"].([]any)[0].(map[string]any)["unit_price"] != 20000.0 {
		t.Errorf("preferencia: %v", pref)
	}
	senar("Beto")
	if cod, _ := senar("Carla"); cod != http.StatusConflict {
		t.Errorf("sin cupos: %d", cod)
	}

	// Un pago de menos no aprueba; el bueno sí, una sola vez.
	mp.mu.Lock()
	mp.pagos["111"] = fmt.Sprintf(`{"status":"approved","transaction_amount":100,"currency_id":"ARS","external_reference":"sena-%d"}`, id)
	mp.pagos["222"] = fmt.Sprintf(`{"status":"approved","transaction_amount":20000,"currency_id":"ARS","external_reference":"sena-%d"}`, id)
	mp.mu.Unlock()
	aviso := func(cuerpo, query string) int {
		r := httptest.NewRequest("POST", "/v1/preventa/mp"+query, strings.NewReader(cuerpo))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := aviso(`{"type":"payment","data":{"id":"111"}}`, ""); c != http.StatusOK {
		t.Errorf("aviso de menos: %d", c)
	}
	if x, _ := s.alm.VerSena(context.Background(), id); x.Estado != "pendiente" {
		t.Errorf("con un pago de menos quedó %s", x.Estado)
	}
	if c := aviso("", "?type=payment&data.id=222"); c != http.StatusOK {
		t.Errorf("aviso bueno: %d", c)
	}
	if c := aviso(`{"type":"payment","data":{"id":"222"}}`, ""); c != http.StatusOK {
		t.Errorf("aviso repetido: %d", c)
	}
	if c := aviso("", "?type=payment&data.id=999"); c != http.StatusBadGateway {
		t.Errorf("pago que MP no conoce: %d", c)
	}
	if c := aviso(`{"type":"merchant_order","data":{"id":"5"}}`, ""); c != http.StatusOK {
		t.Errorf("otro aviso: %d", c)
	}
	x, _ := s.alm.VerSena(context.Background(), id)
	if x.Estado != "aprobada" || x.MPPago != "222" || x.Pagada == nil {
		t.Errorf("seña después del pago: %+v", x)
	}

	// El dueño la ve, la marca devuelta y apaga la preventa.
	_, l := pedir(t, h, admin, "GET", "/v1/admin/senas", "")
	if len(l["senas"].([]any)) < 2 {
		t.Errorf("señas: %v", l)
	}
	if cod, v := pedir(t, h, admin, "PATCH", fmt.Sprintf("/v1/admin/senas/%d", id), `{"estado":"devuelta"}`); cod != http.StatusOK || v["estado"] != "devuelta" {
		t.Errorf("devolver: %d %v", cod, v)
	}
	pedir(t, h, admin, "PUT", "/v1/admin/preventa", `{"activa":false,"precio":100000,"precioFinal":150000,"sena":20000,"cupos":10}`)
	if cod, _ := senar("Tarde"); cod != http.StatusConflict {
		t.Errorf("con la preventa apagada: %d", cod)
	}
}
