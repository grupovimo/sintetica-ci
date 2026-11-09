package almacen

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTopesEnLaBase(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	clave := fmt.Sprintf("prueba:%d", time.Now().UnixNano())

	// Diez a la vez con tope 3: pasan exactamente 3 (el candado los ordena).
	var pasaron atomic.Int32
	var wg sync.WaitGroup
	for k := 0; k < 10; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := a.Tope(ctx, clave, 0, 3, time.Hour)
			if err != nil {
				t.Error(err)
			}
			if ok {
				pasaron.Add(1)
			}
		}()
	}
	wg.Wait()
	if pasaron.Load() != 3 {
		t.Fatalf("pasaron %d, esperaba 3", pasaron.Load())
	}
	// "Uno cada": el segundo enseguida no pasa.
	if ok, _ := a.Tope(ctx, clave+":cada", time.Minute, 10, time.Hour); !ok {
		t.Fatal("el primero pasa")
	}
	if ok, _ := a.Tope(ctx, clave+":cada", time.Minute, 10, time.Hour); ok {
		t.Fatal("el segundo, antes del minuto, no")
	}
	// Una ventana corta se vacía sola.
	if ok, _ := a.Tope(ctx, clave+":corta", 0, 1, 200*time.Millisecond); !ok {
		t.Fatal("primero")
	}
	time.Sleep(300 * time.Millisecond)
	if ok, _ := a.Tope(ctx, clave+":corta", 0, 1, 200*time.Millisecond); !ok {
		t.Fatal("pasó la ventana: vuelve a pasar")
	}

	// Una sola vez.
	if ok, _ := a.PrimeraVez(ctx, clave+":pago", time.Hour); !ok {
		t.Fatal("la primera vez")
	}
	if ok, _ := a.PrimeraVez(ctx, clave+":pago", time.Hour); ok {
		t.Fatal("la segunda, no")
	}
	if ok, _ := a.PrimeraVez(ctx, clave+":vence", 100*time.Millisecond); !ok {
		t.Fatal("la primera vez")
	}
	time.Sleep(200 * time.Millisecond)
	if ok, _ := a.PrimeraVez(ctx, clave+":vence", time.Hour); !ok {
		t.Fatal("vencida, cuenta como nueva")
	}

	// La limpieza borra lo vencido y deja lo vigente.
	if err := a.limpiarTopes(ctx); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.Tope(ctx, clave, 0, 3, time.Hour); ok {
		t.Fatal("lo vigente sigue contando después de limpiar")
	}
}

func TestToquesRecibidos(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	u, _ := a.UsuarioPorEmail(ctx, "toca@ejemplo.com", false)
	if err := a.AsegurarBicho(ctx, "70C0E1000001", "virtual", true); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := a.TomarToque(ctx, "70C0E1000001", time.Minute); err != nil || ok {
		t.Fatalf("sin toque: %v %v", ok, err)
	}
	en := time.Now().Add(-time.Second)
	a.AnotarToque(ctx, "70C0E1000001", ToqueRecibido{DeUsuario: u.ID, Tipo: "toque", En: en.Add(-time.Hour)}) //nolint:errcheck
	if err := a.AnotarToque(ctx, "70C0E1000001", ToqueRecibido{DeUsuario: u.ID, Desde: "OTRO", Tipo: "carita", Expresion: "guino", En: en}); err != nil {
		t.Fatal(err)
	}
	g, ok, err := a.TomarToque(ctx, "70C0E1000001", time.Minute)
	if err != nil || !ok || g.DeUsuario != u.ID || g.Desde != "OTRO" || g.Tipo != "carita" || g.Expresion != "guino" {
		t.Fatalf("el último pisa al anterior: %+v %v %v", g, ok, err)
	}
	if _, ok, _ := a.TomarToque(ctx, "70C0E1000001", time.Minute); ok {
		t.Fatal("se toma una sola vez")
	}
	a.AnotarToque(ctx, "70C0E1000001", ToqueRecibido{DeUsuario: u.ID, Tipo: "toque", En: time.Now().Add(-time.Hour)}) //nolint:errcheck
	if _, ok, _ := a.TomarToque(ctx, "70C0E1000001", 10*time.Minute); ok {
		t.Fatal("uno viejo no se devuelve")
	}
}
