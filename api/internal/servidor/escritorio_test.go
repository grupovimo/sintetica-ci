package servidor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEscritorioLicencia(t *testing.T) {
	s, alm := servidorDePrueba(t)
	h := s.Rutas()
	ctx := context.Background()
	u, err := alm.UsuarioPorEmail(ctx, strings.ToLower(azarHex(4))+"@ejemplo.com", false)
	if err != nil {
		t.Fatal(err)
	}
	usuario, token := u.ID, aleatorio(24)
	if err := alm.CrearSesion(ctx, hash(token), usuario, "app", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Sin robot de escritorio, la prueba arranca ahora; todavía no se vende.
	pv, _ := alm.VerPreventa(ctx, productoEscritorio)
	pv.Activa, pv.Precio = false, 0
	if _, err := alm.GuardarPreventa(ctx, pv); err != nil {
		t.Fatal(err)
	}
	cod, v := pedir(t, h, token, "GET", "/v1/escritorio", "")
	if cod != http.StatusOK || v["licencia"] != false || v["enPrueba"] != true || v["venta"] != false {
		t.Fatalf("sin nada: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "POST", "/v1/escritorio/comprar", ""); cod != http.StatusConflict {
		t.Errorf("comprar sin venta: %d", cod)
	}

	// Un robot de escritorio de hace 8 días: la prueba se venció.
	id := strings.ToUpper(azarHex(6))
	if err := alm.AsegurarBicho(ctx, id, productoEscritorio, false); err != nil {
		t.Fatal(err)
	}
	if err := alm.CrearCredencialPendiente(ctx, id, usuario, hash("clave"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := alm.UsarCredencial(ctx, id, hash("clave")); err != nil {
		t.Fatal(err)
	}
	db, err := pgx.Connect(ctx, os.Getenv("SINTETICA_PRUEBAS_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err := db.Exec(ctx, `update bichos set creado = now() - interval '8 days' where id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, v := pedir(t, h, token, "GET", "/v1/escritorio", ""); v["enPrueba"] != false {
		t.Errorf("a los 8 días sigue en prueba: %v", v)
	}

	// Se vende: el pago en Mercado Pago, y el aviso la marca pagada una sola vez.
	mp := &mpFalso{token: "APP_USR-prueba", pagos: map[string]string{}}
	srv := httptest.NewServer(mp)
	defer srv.Close()
	s.op.MercadoPagoURL, s.op.MPToken = srv.URL, mp.token
	s.op.WebURL, s.op.URLPublica = "https://sintetica.ejemplo.com", "https://api.ejemplo.com"
	pv.Activa, pv.Precio = true, 30000
	if _, err := alm.GuardarPreventa(ctx, pv); err != nil {
		t.Fatal(err)
	}
	if _, v := pedir(t, h, token, "GET", "/v1/escritorio", ""); v["venta"] != true || v["precio"] != 30000.0 {
		t.Errorf("a la venta: %v", v)
	}
	cod, v = pedir(t, h, token, "POST", "/v1/escritorio/comprar", "")
	if cod != http.StatusCreated || !strings.HasPrefix(fmt.Sprint(v["url"]), "https://www.mercadopago.com.ar/") {
		t.Fatalf("comprar: %d %v", cod, v)
	}
	lic := int64(v["id"].(float64))
	pref := mp.prefs[len(mp.prefs)-1]
	if pref["external_reference"] != fmt.Sprintf("lic-%d", lic) || pref["items"].([]any)[0].(map[string]any)["unit_price"] != 30000.0 ||
		pref["back_urls"].(map[string]any)["success"] != "https://sintetica.ejemplo.com/robots?compra=ok" {
		t.Errorf("preferencia: %v", pref)
	}

	mp.mu.Lock()
	mp.pagos["301"] = fmt.Sprintf(`{"status":"approved","transaction_amount":300,"currency_id":"ARS","external_reference":"lic-%d"}`, lic)
	mp.pagos["302"] = fmt.Sprintf(`{"status":"approved","transaction_amount":30000,"currency_id":"ARS","external_reference":"lic-%d"}`, lic)
	mp.mu.Unlock()
	aviso := func(pago string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/preventa/mp?type=payment&data.id="+pago, nil))
		return w.Code
	}
	if c := aviso("301"); c != http.StatusOK {
		t.Errorf("aviso de menos: %d", c)
	}
	if l, _ := alm.VerLicencia(ctx, lic); l.Estado != "pendiente" {
		t.Errorf("con un pago de menos quedó %s", l.Estado)
	}
	if c := aviso("302"); c != http.StatusOK {
		t.Errorf("aviso bueno: %d", c)
	}
	if c := aviso("302"); c != http.StatusOK {
		t.Errorf("aviso repetido: %d", c)
	}
	l, _ := alm.VerLicencia(ctx, lic)
	if l.Estado != "pagada" || l.MPPago != "302" || l.Pagada == nil {
		t.Errorf("licencia después del pago: %+v", l)
	}
	if _, v := pedir(t, h, token, "GET", "/v1/escritorio", ""); v["licencia"] != true {
		t.Errorf("con la licencia pagada: %v", v)
	}
	if cod, _ := pedir(t, h, token, "POST", "/v1/escritorio/comprar", ""); cod != http.StatusConflict {
		t.Errorf("comprar dos veces: %d", cod)
	}

	// Una licencia que no existe no rompe el aviso.
	mp.mu.Lock()
	mp.pagos["303"] = `{"status":"approved","transaction_amount":30000,"currency_id":"ARS","external_reference":"lic-999999999"}`
	mp.mu.Unlock()
	if c := aviso("303"); c != http.StatusOK {
		t.Errorf("licencia desconocida: %d", c)
	}
}
