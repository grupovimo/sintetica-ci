package servidor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"sintetica/api/internal/almacen"
)

// Las listas por HTTP: crear, agregar varias, tachar, limpiar, los topes, la
// de otro (404) y el evento "listas".
func TestListasHTTP(t *testing.T) {
	s, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	otro := nuevoUsuarioMCP(t, alm)
	sub := s.hub.Suscribir(u.id, "")
	defer s.hub.Desuscribir(sub)

	if cod, v := pedir(t, h, u.cookie, "GET", "/v1/listas", ""); cod != 200 || len(v["listas"].([]any)) != 0 || v["porDefecto"] != "súper" {
		t.Fatalf("GET vacío: %d %v", cod, v)
	}
	cod, v := pedir(t, h, u.cookie, "POST", "/v1/listas", `{"nombre":"  Súper "}`)
	if cod != http.StatusCreated || v["nombre"] != "Súper" {
		t.Fatalf("crear: %d %v", cod, v)
	}
	id := v["id"].(string)
	if cod, v := pedir(t, h, u.cookie, "POST", "/v1/listas", `{"nombre":"súper"}`); cod != 200 || v["id"] != id {
		t.Fatalf("la misma: %d %v", cod, v)
	}
	if (<-sub.C).Tipo != "listas" {
		t.Fatal("sin evento")
	}
	for _, malo := range []string{`{"nombre":""}`, `{"nombre":"` + strings.Repeat("x", 41) + `"}`, `{"otro":1}`} {
		if cod, _ := pedir(t, h, u.cookie, "POST", "/v1/listas", malo); cod != http.StatusBadRequest {
			t.Fatalf("nombre malo %s: %d", malo, cod)
		}
	}

	ruta := "/v1/listas/" + id + "/items"
	cod, v = pedir(t, h, u.cookie, "POST", ruta, `{"texto":"leche, pan y el queso"}`)
	if cod != 200 || len(v["agregados"].([]any)) != 3 || len(v["lista"].(map[string]any)["items"].([]any)) != 3 {
		t.Fatalf("agregar: %d %v", cod, v)
	}
	items := v["lista"].(map[string]any)["items"].([]any)
	pan := items[1].(map[string]any)["id"].(string)
	queso := items[2].(map[string]any)["id"].(string)
	if items[2].(map[string]any)["texto"] != "queso" {
		t.Fatalf("orden: %v", items)
	}
	if cod, v := pedir(t, h, u.cookie, "POST", ruta, `{"texto":"Pan"}`); cod != 200 || len(v["agregados"].([]any)) != 0 || len(v["yaEstaban"].([]any)) != 1 {
		t.Fatalf("repetida: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, u.cookie, "POST", ruta, `{"texto":"`+strings.Repeat("x", 121)+`"}`); cod != http.StatusBadRequest {
		t.Fatalf("muy larga: %d", cod)
	}
	cod, v = pedir(t, h, u.cookie, "PATCH", ruta+"/"+pan, `{"hecho":true}`)
	if cod != 200 || v["hecho"] != true || v["hechoEn"] == nil {
		t.Fatalf("tachar: %d %v", cod, v)
	}
	if cod, v := pedir(t, h, u.cookie, "PATCH", ruta+"/"+pan, `{"texto":"pan lactal"}`); cod != 200 || v["texto"] != "pan lactal" || v["hecho"] != true {
		t.Fatalf("renombrar: %d %v", cod, v)
	}
	if cod, _ := pedir(t, h, u.cookie, "PATCH", ruta+"/"+pan, `{}`); cod != http.StatusBadRequest {
		t.Fatalf("PATCH vacío: %d", cod)
	}
	// Lo tachado que se vuelve a anotar, vuelve.
	if cod, v := pedir(t, h, u.cookie, "POST", ruta, `{"texto":"pan lactal"}`); cod != 200 || len(v["agregados"].([]any)) != 1 {
		t.Fatalf("revivir: %d %v", cod, v)
	}
	pedir(t, h, u.cookie, "PATCH", ruta+"/"+pan, `{"hecho":true}`)
	if cod, v := pedir(t, h, u.cookie, "POST", "/v1/listas/"+id+"/limpiar", ``); cod != 200 || v["sacadas"] != float64(1) ||
		len(v["lista"].(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("limpiar: %d %v", cod, v)
	}

	// La de otro: como si no existiera (y no se toca).
	for _, p := range [][3]string{{"POST", ruta, `{"texto":"x"}`}, {"PATCH", ruta + "/" + pan, `{"hecho":false}`},
		{"DELETE", "/v1/listas/" + id, ""}, {"POST", "/v1/listas/" + id + "/limpiar", ""}, {"DELETE", ruta + "/" + pan, ""},
		{"DELETE", "/v1/listas/no-es-un-uuid", ""}} {
		if cod, _ := pedir(t, h, otro.cookie, p[0], p[1], p[2]); cod != http.StatusNotFound {
			t.Fatalf("ajena %v: %d", p, cod)
		}
	}
	if cod, v := pedir(t, h, otro.cookie, "GET", "/v1/listas", ""); cod != 200 || len(v["listas"].([]any)) != 0 {
		t.Fatalf("el otro ve: %v", v)
	}

	// Topes: 200 cosas por lista, 30 listas.
	ctx := context.Background()
	var muchas []string
	for k := 0; k < almacen.MaxItemsLista-2; k++ {
		muchas = append(muchas, fmt.Sprint("cosa ", k))
	}
	if _, _, err := alm.AgregarItems(ctx, u.id, id, muchas); err != nil {
		t.Fatal(err)
	}
	if cod, _ := pedir(t, h, u.cookie, "POST", ruta, `{"texto":"una más"}`); cod != http.StatusConflict {
		t.Fatalf("tope de cosas: %d", cod)
	}
	for k := 1; k < almacen.MaxListas; k++ {
		if _, _, err := alm.AsegurarLista(ctx, u.id, fmt.Sprint("lista ", k)); err != nil {
			t.Fatal(err)
		}
	}
	if cod, _ := pedir(t, h, u.cookie, "POST", "/v1/listas", `{"nombre":"una más"}`); cod != http.StatusConflict {
		t.Fatalf("tope de listas: %d", cod)
	}
	if cod, _ := pedir(t, h, u.cookie, "DELETE", ruta+"/"+queso, ""); cod != http.StatusNoContent {
		t.Fatalf("borrar cosa: %d", cod)
	}
	if cod, _ := pedir(t, h, u.cookie, "DELETE", "/v1/listas/"+id, ""); cod != http.StatusNoContent {
		t.Fatalf("borrar lista: %d", cod)
	}
	if cod, v := pedir(t, h, u.cookie, "GET", "/v1/listas", ""); cod != 200 || len(v["listas"].([]any)) != almacen.MaxListas-1 {
		t.Fatalf("después de borrar: %d", len(v["listas"].([]any)))
	}
}

// La herramienta "listas" del MCP.
func TestMCPListas(t *testing.T) {
	_, alm, h := servidorMCPDePrueba(t)
	u := nuevoUsuarioMCP(t, alm)
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "ver"}); r.error || !strings.Contains(r.texto, "No tiene listas") {
		t.Fatalf("ver vacío: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "agregar", "items": []string{"yerba", "pan", "Pan"}}); r.error ||
		!strings.Contains(r.texto, "En «súper»: 2 agregados") {
		t.Fatalf("agregar: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "tachar", "lista": "super", "items": []string{"panes", "champú"}}); r.error ||
		!strings.Contains(r.texto, "tachados: pan.") || !strings.Contains(r.texto, "No encontré: champú") {
		t.Fatalf("tachar: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "limpiar"}); r.error || !strings.Contains(r.texto, "Saqué 1 tachados") {
		t.Fatalf("limpiar: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "sacar", "lista": "pendientes", "items": []string{"x"}}); !r.error {
		t.Fatalf("sin esa lista: %v", r.texto)
	}
	if r := llamar(t, h, u.token, "listas", map[string]any{"accion": "ver", "lista": "Súper"}); r.error || !strings.Contains(r.texto, `"texto":"yerba"`) {
		t.Fatalf("ver: %v", r.texto)
	}
}
