package clima

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDescripcionYTexto(t *testing.T) {
	casos := map[int]string{0: "despejado", 3: "nublado", 45: "niebla", 61: "lluvia", 95: "tormenta", 73: "nieve", 55: "llovizna", 999: "variable"}
	for c, d := range casos {
		if Descripcion(c) != d {
			t.Errorf("%d: %q", c, Descripcion(c))
		}
	}
	if s := (Clima{Temp: 18.4, Descripcion: "nublado"}).Texto(); s != "18° y nublado" {
		t.Fatal(s)
	}
	if Grados(-0.6) != "-1°" || Grados(17.5) != "18°" {
		t.Fatal(Grados(-0.6), Grados(17.5))
	}
	if Clave(-34.60372, -58.38159) != "-34.60,-58.38" || Clave(-34.6049, -58.3811) != Clave(-34.6, -58.38) {
		t.Fatal(Clave(-34.60372, -58.38159))
	}
	if Cortar("añá", 2) != "a" || Cortar("hola", 10) != "hola" {
		t.Fatal("cortar")
	}
	if !(Ubicacion{Lat: -34.6, Lon: -58.4, Ciudad: "Buenos Aires"}).Valida() || (Ubicacion{Lat: 91, Ciudad: "x"}).Valida() ||
		(Ubicacion{Ciudad: ""}).Valida() || (Ubicacion{Ciudad: strings.Repeat("x", 41)}).Valida() {
		t.Fatal("valida")
	}
}

func TestActualConCache(t *testing.T) {
	var pedidos atomic.Int32
	var falla atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		if falla.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		if q.Get("current") != "temperature_2m,weather_code,is_day" || q.Get("daily") != "temperature_2m_max,temperature_2m_min" ||
			q.Get("timezone") != "auto" || q.Get("forecast_days") != "1" || q.Get("latitude") != "-34.6037" {
			t.Errorf("pedido: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"current":{"temperature_2m":18.44,"weather_code":3,"is_day":0},"daily":{"temperature_2m_max":[22.4],"temperature_2m_min":[11.6]}}`)) //nolint:errcheck
	}))
	defer srv.Close()
	c := Nuevo(Opciones{URLPronostico: srv.URL})
	ahora := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	c.ahora = func() time.Time { return ahora }
	cl, err := c.Actual(context.Background(), -34.6037, -58.3816)
	if err != nil {
		t.Fatal(err)
	}
	if cl.Temp != 18.4 || cl.Codigo != 3 || cl.Descripcion != "nublado" || cl.Max != 22 || cl.Min != 12 || cl.Dia || !cl.En.Equal(ahora) {
		t.Fatalf("%+v", cl)
	}
	// Un lugar a menos de 0,01: del caché.
	ahora = ahora.Add(10 * time.Minute)
	if _, err := c.Actual(context.Background(), -34.6040, -58.3820); err != nil || pedidos.Load() != 1 {
		t.Fatalf("caché: %v %d", err, pedidos.Load())
	}
	// Pasados 30 min se vuelve a pedir; si falla, sirve el viejo.
	ahora = ahora.Add(25 * time.Minute)
	falla.Store(true)
	if v, err := c.Actual(context.Background(), -34.6037, -58.3816); err != nil || v.Temp != 18.4 || pedidos.Load() != 2 {
		t.Fatalf("viejo: %v %d", err, pedidos.Load())
	}
	// Sin nada guardado, el error.
	if _, err := c.Actual(context.Background(), 10, 10); !errors.Is(err, ErrServicio) {
		t.Fatalf("sin caché: %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	c := Nuevo(Opciones{URLPronostico: srv.URL, Timeout: 100 * time.Millisecond})
	inicio := time.Now()
	if _, err := c.Actual(context.Background(), 1, 1); !errors.Is(err, ErrServicio) || time.Since(inicio) > time.Second {
		t.Fatalf("timeout: %v %s", err, time.Since(inicio))
	}
	if _, err := c.Actual(context.Background(), 1, 1); err == nil || strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("el error no lleva la URL: %v", err)
	}
}

func TestBuscar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("language") != "es" || q.Get("count") != "6" {
			t.Errorf("pedido: %s", r.URL.RawQuery)
		}
		if q.Get("name") == "nada" {
			w.Write([]byte(`{"generationtime_ms":0.1}`)) //nolint:errcheck
			return
		}
		w.Write([]byte(`{"results":[
		 {"name":"Córdoba","admin1":"Córdoba","country":"Argentina","latitude":-31.4135,"longitude":-64.18105},
		 {"name":"Córdoba","admin1":"Andalucía","country":"España","latitude":37.89155,"longitude":-4.77275},
		 {"name":"Un nombre larguísimo de un pueblo que no entra","country":"Argentina","latitude":1,"longitude":2}]}`)) //nolint:errcheck
	}))
	defer srv.Close()
	c := Nuevo(Opciones{URLGeocoding: srv.URL})
	l, err := c.Buscar(context.Background(), "córdoba")
	if err != nil || len(l) != 3 {
		t.Fatalf("%v %v", l, err)
	}
	if l[0] != (Lugar{Nombre: "Córdoba", Region: "Córdoba", Pais: "Argentina", Lat: -31.41, Lon: -64.18, Ciudad: "Córdoba, Argentina"}) {
		t.Fatalf("%+v", l[0])
	}
	if len(l[2].Ciudad) > MaxCiudad || !l[2].Ubicacion().Valida() {
		t.Fatalf("ciudad larga: %q", l[2].Ciudad)
	}
	if l, err := c.Buscar(context.Background(), "nada"); err != nil || l == nil || len(l) != 0 {
		t.Fatalf("sin resultados: %v %v", l, err)
	}
	if _, err := c.Buscar(context.Background(), "x"); !errors.Is(err, ErrBusqueda) {
		t.Fatal("muy corto")
	}
}
