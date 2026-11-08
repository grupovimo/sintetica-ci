package servidor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

func TestJugarAvisoYResumenHTTP(t *testing.T) {
	_, alm, h := servidorHablaDePrueba(t)
	token, bichos := cuentaDePrueba(t, alm)
	otro, _ := cuentaDePrueba(t, alm)
	base := "/v1/bichos/" + bichos[0]

	for _, c := range []string{`{}`, `{"juego":"ajedrez"}`, `{"juego":"dado","x":1}`} {
		if cod, v := pedir(t, h, token, "POST", base+"/jugar", c); cod != http.StatusBadRequest {
			t.Errorf("jugar %s: %d %v", c, cod, v)
		}
	}
	for _, j := range []string{"dado", "moneda", "ppt", "reflejos", "mirada", "terminar"} {
		if cod, _ := pedir(t, h, token, "POST", base+"/jugar", `{"juego":"`+j+`"}`); cod != http.StatusConflict {
			t.Fatalf("jugar %s desconectado: %d", j, cod)
		}
	}
	if cod, _ := pedir(t, h, otro, "POST", base+"/jugar", `{"juego":"dado"}`); cod != http.StatusNotFound {
		t.Fatalf("jugar ajeno: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/aviso", `{"tipo":"mail"}`); cod != http.StatusBadRequest {
		t.Fatalf("aviso raro: %d", cod)
	}
	if cod, _ := pedir(t, h, token, "POST", base+"/aviso", `{"tipo":"notificacion","app":"whatsapp","contacto":"Mamá"}`); cod != http.StatusConflict {
		t.Fatalf("aviso desconectado: %d", cod)
	}

	// Sin memoria, el resumen lo dice; con memoria de hoy, lo cuenta.
	cod, v := pedir(t, h, token, "GET", base+"/resumen", "")
	if cod != http.StatusOK || !strings.Contains(v["texto"].(string), "Todavía no sé") || v["actualizada"] != nil {
		t.Fatalf("sin memoria: %d %v", cod, v)
	}
	ahora := time.Now().In(nube.Zona())
	m := make([]byte, protocolo.LargoMemoria)
	m[0] = 1
	m[99], m[100], m[101], m[102], m[103], m[104], m[107] = 1, 1, 1, 3, 0, protocolo.DiaContadores(ahora), 2
	binary.LittleEndian.PutUint32(m[protocolo.LargoMemoria-4:], protocolo.Crc32(m[:protocolo.LargoMemoria-4]))
	if err := alm.GuardarMemoria(context.Background(), bichos[0], m); err != nil {
		t.Fatal(err)
	}
	cod, v = pedir(t, h, token, "GET", base+"/resumen", "")
	if cod != http.StatusOK || v["juegos"] != 1.0 || v["ganados"] != nil || v["suenos"] != 1.0 || v["animo"] != "mal" || v["espero"] != nil || v["agua"] != 2.0 ||
		!strings.HasSuffix(v["texto"].(string), " jugó una vez, soñó con corazones, te recordó tomar agua 2 veces y le dijiste que estabas mal.") {
		b, _ := json.Marshal(v)
		t.Fatalf("resumen: %d %s", cod, b)
	}
	if l, _ := v["sonoCon"].([]any); len(l) != 1 || l[0] != "corazon" {
		t.Fatalf("sonoCon: %v", v["sonoCon"])
	}
	// La memoria decodificada trae lo nuevo.
	cod, v = pedir(t, h, token, "GET", base+"/memoria", "")
	if mem, _ := v["memoria"].(map[string]any); cod != http.StatusOK || mem["juegosHoy"] != 1.0 || mem["animoHoy"] != 3.0 || mem["aguaHoy"] != 2.0 {
		t.Fatalf("memoria: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, otro, "GET", base+"/resumen", ""); cod != http.StatusNotFound {
		t.Fatalf("resumen ajeno: %d", cod)
	}
}

func TestMCPCuidado(t *testing.T) {
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
	ok := func(r resultadoHerramienta, que string) resultadoHerramienta {
		t.Helper()
		if r.error || r.rpcErr != nil {
			t.Fatalf("%s: %s %v", que, r.texto, r.rpcErr)
		}
		return r
	}
	falla := func(r resultadoHerramienta, que, contiene string) {
		t.Helper()
		if !r.error || !strings.Contains(r.texto, contiene) {
			t.Fatalf("%s: esperaba un error con %q: %s %v", que, contiene, r.texto, r.rpcErr)
		}
	}

	falla(llamar(t, h, u.token, "jugar", map[string]any{"robot": robot, "juego": "dado"}), "jugar", "no está conectado")

	// cuidado
	if r := ok(llamar(t, h, u.token, "cuidado", map[string]any{"robot": robot, "accion": "ver"}), "ver"); !strings.Contains(r.texto, "porDefecto") {
		t.Fatalf("ver: %s", r.texto)
	}
	falla(llamar(t, h, u.token, "cuidado", map[string]any{"robot": robot, "accion": "poner"}), "nada", "Decime qué cambiar")
	falla(llamar(t, h, u.token, "cuidado", map[string]any{"robot": robot, "accion": "poner", "pausa": 10}), "pausa corta", "de 30 a 240")
	ok(llamar(t, h, u.token, "cuidado", map[string]any{"robot": robot, "accion": "poner", "agua": 2, "pausa": 90, "resumen": true,
		"trabajo": map[string]any{"desde": "10:00", "hasta": "19:00"}}), "poner")
	ok(llamar(t, h, u.token, "cuidado", map[string]any{"robot": robot, "accion": "poner", "extranarDias": 3, "trabajo": map[string]any{"dias": "LMXJ"}}), "poner 2")
	c, _ := ver()["cuidado"].(map[string]any)
	tr, _ := c["trabajo"].(map[string]any)
	if c["agua"] != 2.0 || c["pausa"] != 90.0 || c["resumen"] != true || c["extranarDias"] != 3.0 || tr["desde"] != "10:00" || tr["dias"] != "LMXJ" {
		t.Fatalf("cuidado: %v", c)
	}

	// avisos
	falla(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar"}), "sin tipo", "hace falta tipo")
	falla(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar", "tipo": "llamada"}), "sin reacción", "cómo reacciona")
	falla(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar", "tipo": "llamada", "expresion": "inventada"}), "expresión", "No existe")
	falla(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar", "tipo": "llamada", "comportamiento": "inventado"}), "comportamiento", "No hay un comportamiento")
	ok(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar", "tipo": "notificacion", "app": "whatsapp", "contacto": "Mamá",
		"expresion": "enamorado", "decir": "{contacto} te escribe", "sonido": true}), "agregar")
	ok(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "agregar", "tipo": "llamada", "decir": "¡Te llaman!"}), "agregar 2")
	l, _ := ver()["avisos"].([]any)
	if len(l) != 2 || l[0].(map[string]any)["contacto"] != "Mamá" || l[1].(map[string]any)["tipo"] != "llamada" {
		t.Fatalf("avisos: %v", l)
	}
	if r := ok(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "listar"}), "listar"); !strings.Contains(r.texto, "2 reacciones") {
		t.Fatalf("listar: %s", r.texto)
	}
	falla(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "borrar", "indice": 5}), "borrar fuera", "indice")
	ok(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "borrar", "indice": 0}), "borrar")
	ok(llamar(t, h, u.token, "avisos", map[string]any{"robot": robot, "accion": "borrar", "indice": 0}), "borrar 2")
	if ver()["avisos"] != nil {
		t.Fatal("quedaron avisos")
	}

	// resumen
	if r := ok(llamar(t, h, u.token, "resumen", map[string]any{"robot": robot}), "resumen"); !strings.Contains(r.texto, "Todavía no sé") {
		t.Fatalf("resumen: %s", r.texto)
	}
}
