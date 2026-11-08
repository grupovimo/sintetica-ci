package servidor

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
)

// openMeteoFalso: el pronóstico y la búsqueda de lugares.
func openMeteoFalso(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/buscar") {
			if r.URL.Query().Get("name") == "caido" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Write([]byte(`{"results":[{"name":"Córdoba","admin1":"Córdoba","country":"Argentina","latitude":-31.4135,"longitude":-64.18105}]}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"current":{"temperature_2m":18.4,"weather_code":3,"is_day":1},"daily":{"temperature_2m_max":[22],"temperature_2m_min":[12]}}`)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func servidorHablaDePrueba(t *testing.T) (*Servidor, *almacen.Almacen, http.Handler) {
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
	om := openMeteoFalso(t)
	i := integraciones.Nuevo(integraciones.Opciones{ClimaURL: om.URL + "/pronostico", GeocodingURL: om.URL + "/buscar"}, alm, n, hub, log)
	n.Escuchar(i)
	s := Nuevo(Opciones{WebURL: webPrueba, URLPublica: apiPrueba, Secreto: []byte("secreto de prueba 1234")}, alm, n, i, hub, nil, nil, log)
	return s, alm, s.Rutas()
}

func TestDecirAlarmaYClimaHTTP(t *testing.T) {
	s, alm, h := servidorHablaDePrueba(t)
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)
	base := "/v1/bichos/" + bichos[0]

	malos := []string{`{}`, `{"texto":""}`, `{"texto":"` + strings.Repeat("a", 61) + `"}`, `{"texto":"hola","expresion":"Mal Nombre"}`, `{"texto":"hola","otra":1}`}
	for _, c := range malos {
		if cod, v := pedir(t, h, token, "POST", base+"/decir", c); cod != http.StatusBadRequest {
			t.Errorf("decir %s: %d %v", c, cod, v)
		}
	}
	// Bien armado, pero el robot no está conectado.
	if cod, _ := pedir(t, h, token, "POST", base+"/decir", `{"texto":"¡Hola!","expresion":"contento"}`); cod != http.StatusConflict {
		t.Fatalf("decir desconectado: %d", cod)
	}
	if cod, _ := pedir(t, h, otro, "POST", base+"/decir", `{"texto":"hola"}`); cod != http.StatusNotFound {
		t.Fatalf("decir ajeno: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/alarma", `{"accion":"sonar"}`); cod != http.StatusBadRequest {
		t.Fatalf("alarma rara: %d", cod)
	}
	for _, a := range []string{"apagar", "posponer", "probar", "despertar"} {
		if cod, _ := pedir(t, h, token, "POST", base+"/alarma", `{"accion":"`+a+`"}`); cod != http.StatusConflict {
			t.Fatalf("alarma %s desconectado: %d", a, cod)
		}
	}

	// Buscar lugares.
	cod, v := pedir(t, h, token, "GET", "/v1/clima/buscar?q=c%C3%B3rdoba", "")
	l, _ := v["lugares"].([]any)
	if cod != http.StatusOK || len(l) != 1 {
		t.Fatalf("buscar: %d %v", cod, v)
	}
	if lg := l[0].(map[string]any); lg["ciudad"] != "Córdoba, Argentina" || lg["lat"] != -31.41 || lg["region"] != "Córdoba" {
		t.Fatalf("lugar: %v", lg)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/clima/buscar?q=x", ""); cod != http.StatusBadRequest {
		t.Fatalf("q corta: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "GET", "/v1/clima/buscar?q=caido", ""); cod != http.StatusBadGateway {
		t.Fatalf("caído: %d", cod)
	}
	if cod, _ := pedir(t, h, "", "GET", "/v1/clima/buscar?q=cordoba", ""); cod != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", cod)
	}

	// El clima del robot: sin ubicación, null; con ubicación, el de ahora.
	cod, v = pedir(t, h, token, "GET", base+"/clima", "")
	if cod != http.StatusOK || v["ubicacion"] != nil || v["clima"] != nil {
		t.Fatalf("sin ubicación: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, token, "PUT", base+"/config", `{"version":2,"ubicacion":{"lat":-34.6037,"lon":-58.3816,"ciudad":"Buenos Aires"}}`); cod != http.StatusOK {
		t.Fatalf("config con ubicación: %d %v", cod, v)
	}
	cod, v = pedir(t, h, token, "GET", base+"/clima", "")
	u, _ := v["ubicacion"].(map[string]any)
	c, _ := v["clima"].(map[string]any)
	if cod != http.StatusOK || u["ciudad"] != "Buenos Aires" || c["temp"] != 18.4 || c["descripcion"] != "nublado" || c["max"] != 22.0 || c["dia"] != true || c["en"] == nil {
		t.Fatalf("con ubicación: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, otro, "GET", base+"/clima", ""); cod != http.StatusNotFound {
		t.Fatalf("clima ajeno: %d", cod)
	}

	// La acción "modo" de una automatización guarda la config por el mismo
	// camino (versión nueva, autor automatizacion, validada por el núcleo).
	if err := s.integ.GuardarAutomatizaciones(context.Background(), usuarioDeToken(t, alm, token), bichos[0], []integraciones.Automatizacion{
		{ID: "m", Activa: true, Cuando: "hora", Hora: "08:00", Hacer: []integraciones.Accion{{Tipo: "modo", Modo: "reloj"}}}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.integ.Probar(context.Background(), usuarioDeToken(t, alm, token), bichos[0], "m")
	if err != nil || len(res) != 1 || !res[0].Ok {
		t.Fatalf("modo: %+v %v", res, err)
	}
	cfg, _ := alm.UltimaConfig(context.Background(), bichos[0])
	var o map[string]any
	json.Unmarshal([]byte(cfg.Texto), &o) //nolint:errcheck
	if cfg.Autor != "automatizacion" || o["modo"] != "reloj" || o["ubicacion"] == nil {
		t.Fatalf("config: %s %s", cfg.Autor, cfg.Texto)
	}
}

func usuarioDeToken(t *testing.T, alm *almacen.Almacen, token string) string {
	t.Helper()
	ses, err := alm.SesionPorToken(context.Background(), hash(token), duracionApp)
	if err != nil {
		t.Fatal(err)
	}
	return ses.Usuario.ID
}

func TestMCPHabla(t *testing.T) {
	_, alm, h := servidorHablaDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	robot := u.bichos[0]
	ver := func() map[string]any {
		t.Helper()
		c, err := alm.UltimaConfig(context.Background(), robot)
		if err != nil {
			t.Fatal(err)
		}
		var o map[string]any
		json.Unmarshal([]byte(c.Texto), &o) //nolint:errcheck
		return o
	}
	ok := func(r resultadoHerramienta, que string) {
		t.Helper()
		if r.error || r.rpcErr != nil {
			t.Fatalf("%s: %s %v", que, r.texto, r.rpcErr)
		}
	}
	falla := func(r resultadoHerramienta, que, contiene string) {
		t.Helper()
		if !r.error || !strings.Contains(r.texto, contiene) {
			t.Fatalf("%s: esperaba un error con %q: %s %v", que, contiene, r.texto, r.rpcErr)
		}
	}

	// decir: el robot no está conectado.
	falla(llamar(t, h, u.token, "decir", map[string]any{"robot": robot, "texto": "hola"}), "decir", "no está conectado")
	falla(llamar(t, h, u.token, "decir", map[string]any{"robot": robot, "texto": "hola", "expresion": "inventada"}), "decir", "No existe")

	// despertador.
	falla(llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "poner"}), "sin hora", "hace falta la hora")
	ok(llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "poner", "hora": "07:30", "dias": "LMXJV", "texto": "¡Arriba!"}), "poner")
	ok(llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "poner", "posponer": 5}), "cambiar")
	d, _ := ver()["despertador"].(map[string]any)
	if d["hora"] != "07:30" || d["dias"] != "LMXJV" || d["texto"] != "¡Arriba!" || d["posponer"] != 5.0 {
		t.Fatalf("despertador: %v", d)
	}
	if r := llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "ver"}); !strings.Contains(r.texto, "07:30") {
		t.Fatalf("ver: %s", r.texto)
	}
	falla(llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "probar"}), "probar", "no está conectado")
	ok(llamar(t, h, u.token, "despertador", map[string]any{"robot": robot, "accion": "sacar"}), "sacar")
	if ver()["despertador"] != nil {
		t.Fatal("no lo sacó")
	}

	// clima.
	if r := llamar(t, h, u.token, "clima", map[string]any{"accion": "buscar", "q": "córdoba"}); r.error || !strings.Contains(r.texto, "Córdoba, Argentina") {
		t.Fatalf("buscar: %s", r.texto)
	}
	if r := llamar(t, h, u.token, "clima", map[string]any{"robot": robot, "accion": "ver"}); !strings.Contains(r.texto, "no tiene ubicación") {
		t.Fatalf("ver sin ubicación: %s", r.texto)
	}
	ok(llamar(t, h, u.token, "clima", map[string]any{"robot": robot, "accion": "ubicar", "q": "córdoba"}), "ubicar")
	if ub, _ := ver()["ubicacion"].(map[string]any); ub["ciudad"] != "Córdoba, Argentina" || ub["lat"] != -31.41 {
		t.Fatalf("ubicación: %v", ub)
	}
	if r := llamar(t, h, u.token, "clima", map[string]any{"robot": robot, "accion": "ver"}); !strings.Contains(r.texto, "18° y nublado") {
		t.Fatalf("ver: %s", r.texto)
	}
	falla(llamar(t, h, u.token, "clima", map[string]any{"robot": robot, "accion": "ubicar", "lat": 1, "lon": 2}), "sin ciudad", "ciudad")

	// rutinas.
	falla(llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "crear", "hora": "07:30"}), "sin hacer", "hace falta")
	ok(llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "crear", "hora": "07:30", "dias": "LMXJV",
		"hacer": []any{map[string]any{"tipo": "decir", "texto": "Buen día, {dueno}: {clima}"}, map[string]any{"tipo": "modo", "modo": "clima"}}}), "crear")
	falla(llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "crear", "hora": "08:00",
		"hacer": []any{map[string]any{"tipo": "modo", "modo": "clima", "texto": "x"}}}), "inválida", "no es válida")
	r := llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "listar"})
	if !strings.Contains(r.texto, "1 rutinas") || !strings.Contains(r.texto, "rutina-0730") {
		t.Fatalf("listar: %s", r.texto)
	}
	ok(llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "borrar", "id": "rutina-0730"}), "borrar")
	falla(llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "borrar", "id": "rutina-0730"}), "borrar dos veces", "No hay")
	// La herramienta automatizaciones también acepta lo nuevo.
	ok(llamar(t, h, u.token, "automatizaciones", map[string]any{"robot": robot, "accion": "guardar", "automatizaciones": []any{
		map[string]any{"id": "finde", "activa": true, "cuando": "hora", "hora": "09:00", "dias": "SD", "hacer": []any{map[string]any{"tipo": "modo", "modo": "reloj"}}},
		map[string]any{"id": "alarma", "activa": true, "cuando": "despertador", "hacer": []any{map[string]any{"tipo": "decir", "texto": "¡Arriba, {dueno}!"}}},
	}}), "automatizaciones")
	if r := llamar(t, h, u.token, "rutinas", map[string]any{"robot": robot, "accion": "listar"}); !strings.Contains(r.texto, "finde") || strings.Contains(r.texto, `"alarma"`) {
		t.Fatalf("rutinas después de automatizaciones: %s", r.texto)
	}

	// habla.
	ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "frecuencia", "frecuencia": 0.8}), "frecuencia")
	ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "agregar", "situacion": "caricia", "texto": "Más, más"}), "agregar")
	ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "agregar", "situacion": "despertar", "texto": "¡Arriba, {dueno}!"}), "agregar 2")
	hb, _ := ver()["habla"].(map[string]any)
	if hb["frecuencia"] != 0.8 || hb["propias"].(map[string]any)["caricia"].([]any)[0] != "Más, más" {
		t.Fatalf("habla: %v", hb)
	}
	ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "borrar", "situacion": "caricia", "texto": "Más, más"}), "borrar frase")
	ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "borrar", "situacion": "despertar"}), "borrar situación")
	if hb, _ := ver()["habla"].(map[string]any); hb["propias"] != nil || hb["frecuencia"] != 0.8 {
		t.Fatalf("habla después de borrar: %v", hb)
	}
	for k := 0; k < MaxFrasesPropias; k++ {
		ok(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "agregar", "situacion": "saludo", "texto": "hola " + string(rune('a'+k))}), "llenar")
	}
	falla(llamar(t, h, u.token, "habla", map[string]any{"robot": robot, "accion": "agregar", "situacion": "saludo", "texto": "una más"}), "más de 12", "máximo")
}
