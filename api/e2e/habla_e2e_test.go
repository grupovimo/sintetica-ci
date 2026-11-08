//go:build e2e

package e2e

import (
	"testing"
	"time"

	"sintetica/api/internal/protocolo"
)

// La v1.6 de punta a punta: la web le dice algo y aparece el globo en la
// pantalla del bicho virtual, el despertador suena y se pospone, y el modo
// clima muestra el tiempo que trae la nube (Open-Meteo, si hay internet).
func TestHablaDePuntaAPunta(t *testing.T) {
	web := entrar(t, "web")
	id := leerVirtual(t).ID
	if s := web.pedir("POST", "/v1/bichos/reclamar-virtual", map[string]string{"id": id}, nil); s != 200 {
		t.Fatalf("sumar el bicho virtual: %d", s)
	}
	defer web.pedir("DELETE", "/v1/bichos/"+id, nil, nil)
	var vb struct {
		Conectado bool
		Estado    *protocolo.Estado
	}
	cara := func() string {
		web.pedir("GET", "/v1/bichos/"+id, nil, &vb)
		if vb.Estado == nil {
			return ""
		}
		return vb.Estado.Expresion
	}
	esperar(t, "que la API vea su estado", 30*time.Second, func() bool { return cara() != "" && vb.Conectado })

	// Decir: el globo.
	if s := web.pedir("POST", "/v1/bichos/"+id+"/decir", map[string]string{"texto": "¡Hola desde la nube!"}, nil); s != 200 {
		t.Fatalf("decir: %d", s)
	}
	esperar(t, "que se vea el globo", 10*time.Second, func() bool { return cara() == "globo" })
	esperar(t, "que el globo se vaya solo", 15*time.Second, func() bool { return cara() != "globo" })
	if s := web.pedir("POST", "/v1/bichos/"+id+"/decir", map[string]string{"texto": ""}, nil); s != 400 {
		t.Errorf("decir vacío: %d, esperaba 400", s)
	}

	// El despertador: probar, posponer y apagar.
	alarma := func(a string) int {
		return web.pedir("POST", "/v1/bichos/"+id+"/alarma", map[string]string{"accion": a}, nil)
	}
	if s := alarma("probar"); s != 200 {
		t.Fatalf("alarma probar: %d", s)
	}
	esperar(t, "que suene el despertador", 10*time.Second, func() bool { return cara() == "despertador" })
	if s := alarma("posponer"); s != 200 {
		t.Fatalf("alarma posponer: %d", s)
	}
	esperar(t, "que se posponga", 10*time.Second, func() bool { return cara() != "despertador" })
	alarma("apagar")

	// El modo clima con una ubicación (Buenos Aires).
	cfg := map[string]any{"version": 2, "modo": "clima", "ubicacion": map[string]any{"lat": -34.6, "lon": -58.38, "ciudad": "Buenos Aires"}}
	if s := web.pedir("PUT", "/v1/bichos/"+id+"/config", cfg, nil); s != 200 {
		t.Fatalf("config con clima: %d", s)
	}
	esperar(t, "que muestre el modo clima", 30*time.Second, func() bool { return cara() == "clima" })
	var cl struct {
		Clima *struct {
			Temp   float64
			Codigo int
		}
	}
	web.pedir("GET", "/v1/bichos/"+id+"/clima", nil, &cl)
	if cl.Clima == nil {
		t.Log("sin clima (¿sin internet?): el modo clima muestra --°")
	} else {
		t.Logf("clima de Buenos Aires: %.1f °C, código %d", cl.Clima.Temp, cl.Clima.Codigo)
	}
}
