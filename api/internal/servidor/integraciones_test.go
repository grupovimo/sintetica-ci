package servidor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

// Necesita un Postgres: SINTETICA_PRUEBAS_DB=postgres://…
func servidorDePrueba(t *testing.T) (*Servidor, *almacen.Almacen) {
	t.Helper()
	u := os.Getenv("SINTETICA_PRUEBAS_DB")
	if u == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	alm, err := almacen.Abrir(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(alm.Cerrar)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := eventos.Nuevo()
	n := nube.Nuevo(nube.Opciones{}, alm, hub, log)
	t.Cleanup(n.Cerrar)
	i := integraciones.Nuevo(integraciones.Opciones{}, alm, n, hub, log) // sin token: Telegram apagado
	n.Escuchar(i)
	return Nuevo(Opciones{}, alm, n, i, hub, nil, nil, log), alm
}

func azarHex(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return strings.ToUpper(hex.EncodeToString(b))
}

// cuentaDePrueba: un usuario con una sesión de app y dos bichos suyos.
func cuentaDePrueba(t *testing.T, alm *almacen.Almacen) (token string, bichos []string) {
	t.Helper()
	ctx := context.Background()
	u, err := alm.UsuarioPorEmail(ctx, strings.ToLower(azarHex(4))+"@ejemplo.com", false)
	if err != nil {
		t.Fatal(err)
	}
	token = aleatorio(24)
	if err := alm.CrearSesion(ctx, hash(token), u.ID, "app", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for k := 0; k < 2; k++ {
		id := azarHex(6)
		alm.AsegurarBicho(ctx, id, "virtual", true)                                      //nolint:errcheck
		alm.CrearCredencialPendiente(ctx, id, u.ID, hash(id), time.Now().Add(time.Hour)) //nolint:errcheck
		if acc, err := alm.UsarCredencial(ctx, id, hash(id)); err != nil || !acc.Ok {
			t.Fatal("no pude sumar el bicho")
		}
		bichos = append(bichos, id)
	}
	return token, bichos
}

func pedir(t *testing.T, h http.Handler, token, metodo, ruta, cuerpo string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v) //nolint:errcheck
	return w.Code, v
}

func TestAutomatizacionesHTTP(t *testing.T) {
	s, alm := servidorDePrueba(t)
	h := s.Rutas()
	token, bichos := cuentaDePrueba(t, alm)
	otro, ajenos := cuentaDePrueba(t, alm)
	ruta := "/v1/bichos/" + bichos[0] + "/automatizaciones"

	malas := []string{
		`no es json`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"explota","hacer":[{"tipo":"telegram","texto":"x"}]}]}`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"tacto","hacer":[]}]}`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"tacto","hacer":[{"tipo":"webhook","url":"http://127.0.0.1:8081/emqx"}]}]}`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"tacto","hacer":[{"tipo":"webhook","url":"http://169.254.169.254/"}]}]}`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"tacto","hacer":[{"tipo":"robot","bicho":"` + ajenos[0] + `","expresion":"sorpresa"}]}]}`,
		`{"automatizaciones":[{"id":"a1","activa":true,"cuando":"tacto","hacer":[{"tipo":"telegram","texto":"x"}],"otra":1}]}`,
	}
	for _, c := range malas {
		if cod, v := pedir(t, h, token, "PUT", ruta, c); cod != http.StatusBadRequest {
			t.Errorf("esperaba 400 para %s: %d %v", c, cod, v)
		}
	}
	var muchas []string
	for k := 0; k < 21; k++ {
		muchas = append(muchas, `{"id":"a`+azarHex(2)+`","activa":true,"cuando":"tacto","hacer":[{"tipo":"telegram","texto":"x"}]}`)
	}
	if cod, _ := pedir(t, h, token, "PUT", ruta, `{"automatizaciones":[`+strings.Join(muchas, ",")+`]}`); cod != http.StatusBadRequest {
		t.Errorf("más de 20: %d", cod)
	}

	buena := `{"automatizaciones":[
	 {"id":"a1","activa":true,"cuando":"bateria-baja","hacer":[
	   {"tipo":"telegram","texto":"{robot} tiene {bateria}% de batería"},
	   {"tipo":"webhook","url":"https://ejemplo.com/gancho"},
	   {"tipo":"robot","bicho":"` + bichos[1] + `","expresion":"sorpresa"}
	 ]},
	 {"id":"a2","activa":false,"cuando":"expresion","valor":"contento","hacer":[{"tipo":"robot","bicho":"` + bichos[1] + `","expresion":"contento"}]}
	]}`
	cod, v := pedir(t, h, token, "PUT", ruta, buena)
	if cod != http.StatusOK {
		t.Fatalf("la buena: %d %v", cod, v)
	}
	cod, v = pedir(t, h, token, "GET", ruta, "")
	l, _ := v["automatizaciones"].([]any)
	if cod != http.StatusOK || len(l) != 2 || v["historial"] == nil {
		t.Fatalf("GET: %d %v", cod, v)
	}
	if a := l[1].(map[string]any); a["valor"] != "contento" || a["activa"] != false {
		t.Fatalf("se guardó distinta: %v", a)
	}
	// Otro usuario no la ve.
	if cod, _ := pedir(t, h, otro, "GET", ruta, ""); cod != http.StatusNotFound {
		t.Fatalf("ajeno: %d", cod)
	}

	// Probar: el robot está desconectado y Telegram apagado → corre, pero las acciones fallan.
	cod, v = pedir(t, h, token, "POST", ruta+"/a2/probar", "")
	res, _ := v["resultados"].([]any)
	if cod != http.StatusOK || v["ok"] != false || len(res) != 1 {
		t.Fatalf("probar: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, token, "POST", ruta+"/1/probar", ""); cod != http.StatusTooManyRequests {
		t.Fatalf("límite: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "POST", ruta+"/7/probar", ""); cod != http.StatusNotFound {
		t.Fatalf("no existe: %d", cod)
	}
	_, v = pedir(t, h, token, "GET", ruta, "")
	if hist, _ := v["historial"].([]any); len(hist) != 1 || hist[0].(map[string]any)["ok"] != false {
		t.Fatalf("historial: %v", v["historial"])
	}

	// Telegram apagado (sin token).
	if cod, v := pedir(t, h, token, "GET", "/v1/telegram", ""); cod != http.StatusOK || v["activo"] != false {
		t.Fatalf("telegram: %d %v", cod, v)
	}
	if _, v := pedir(t, h, token, "POST", "/v1/telegram/vincular", ""); v["activo"] != false {
		t.Fatalf("vincular apagado: %v", v)
	}

	// El secreto de los webhooks: el mismo hasta que se rota.
	_, v1 := pedir(t, h, token, "GET", "/v1/webhooks/secreto", "")
	_, v2 := pedir(t, h, token, "GET", "/v1/webhooks/secreto", "")
	_, v3 := pedir(t, h, token, "POST", "/v1/webhooks/secreto", "")
	s1, _ := v1["secreto"].(string)
	if !strings.HasPrefix(s1, "whsec_") || v2["secreto"] != s1 || v3["secreto"] == s1 {
		t.Fatalf("secreto: %v %v %v", v1, v2, v3)
	}
}
