package almacen

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"
)

// Necesita un Postgres vacío: SINTETICA_PRUEBAS_DB=postgres://…
func abrirPrueba(t *testing.T) *Almacen {
	t.Helper()
	url := os.Getenv("SINTETICA_PRUEBAS_DB")
	if url == "" {
		t.Skip("sin SINTETICA_PRUEBAS_DB")
	}
	ctx := context.Background()
	a, err := Abrir(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"actualizaciones", "firmwares", "biblioteca", "sincro", "imagenes", "configs", "credenciales", "bichos", "sesiones", "enlaces", "usuarios"} {
		if _, err := a.db.Exec(ctx, "delete from "+tabla); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(a.Cerrar)
	return a
}

func h(s string) []byte { x := sha256.Sum256([]byte(s)); return x[:] }

func TestEnlacesYSesiones(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	vence := time.Now().Add(15 * time.Minute)
	if err := a.CrearEnlace(ctx, "ana@ejemplo.com", h("token"), h("CODIGO12"), "web", "1.2.3.4", vence); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CanjearCodigo(ctx, "ana@ejemplo.com", h("MAL")); err != ErrEnlaceInvalido {
		t.Fatalf("código malo: %v", err)
	}
	email, cliente, err := a.CanjearToken(ctx, h("token"))
	if err != nil || email != "ana@ejemplo.com" || cliente != "web" {
		t.Fatalf("canjear: %v %s %s", err, email, cliente)
	}
	if _, _, err := a.CanjearToken(ctx, h("token")); err != ErrEnlaceInvalido {
		t.Fatal("un link se usa una sola vez")
	}
	// Cinco intentos y el código muere.
	a.CrearEnlace(ctx, "bea@ejemplo.com", h("t2"), h("BUENO123"), "app", "", vence) //nolint:errcheck
	for i := 0; i < 5; i++ {
		a.CanjearCodigo(ctx, "bea@ejemplo.com", h("MAL")) //nolint:errcheck
	}
	if _, err := a.CanjearCodigo(ctx, "bea@ejemplo.com", h("BUENO123")); err != ErrDemasiadosIntentos {
		t.Fatalf("esperaba demasiados intentos: %v", err)
	}
	u, err := a.UsuarioPorEmail(ctx, email, false)
	if err != nil {
		t.Fatal(err)
	}
	u2, _ := a.UsuarioPorEmail(ctx, email, true)
	if u2.ID != u.ID || !u2.Admin {
		t.Fatal("el mismo email es la misma cuenta")
	}
	if err := a.CrearSesion(ctx, h("sesion"), u.ID, "web", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	s, err := a.SesionPorToken(ctx, h("sesion"), time.Hour)
	if err != nil || s.Usuario.Email != email {
		t.Fatalf("sesión: %v", err)
	}
	a.BorrarSesion(ctx, h("sesion")) //nolint:errcheck
	if _, err := a.SesionPorToken(ctx, h("sesion"), time.Hour); err != ErrNoEncontrado {
		t.Fatal("la sesión se cerró")
	}
}

func TestReclamoYCambioDeDueno(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	ana, _ := a.UsuarioPorEmail(ctx, "ana@ejemplo.com", false)
	beto, _ := a.UsuarioPorEmail(ctx, "beto@ejemplo.com", false)
	const id = "A1B2C3D4E5F6"
	if err := a.AsegurarBicho(ctx, id, "dev-147b", false); err != nil {
		t.Fatal(err)
	}
	vence := time.Now().Add(30 * time.Minute)
	a.CrearCredencialPendiente(ctx, id, ana.ID, h("clave-ana"), vence) //nolint:errcheck
	if acc, _ := a.UsarCredencial(ctx, id, h("otra")); acc.Ok {
		t.Fatal("una clave que no es")
	}
	acc, err := a.UsarCredencial(ctx, id, h("clave-ana"))
	if err != nil || !acc.Ok || !acc.Reclamado || acc.UsuarioID != ana.ID {
		t.Fatalf("reclamo de Ana: %+v %v", acc, err)
	}
	if acc, _ := a.UsarCredencial(ctx, id, h("clave-ana")); !acc.Ok || acc.Reclamado {
		t.Fatal("la segunda vez ya es activa")
	}
	a.NuevaConfig(ctx, id, `{"version":1}`, 1, "web", &ana.ID) //nolint:errcheck
	a.GuardarImagen(ctx, id, 2, []byte("qoif"), 9)             //nolint:errcheck
	// Beto lo tiene en la mano y lo suma a su cuenta: se borra lo de Ana.
	a.CrearCredencialPendiente(ctx, id, beto.ID, h("clave-beto"), vence) //nolint:errcheck
	acc, _ = a.UsarCredencial(ctx, id, h("clave-beto"))
	if !acc.Reclamado || acc.UsuarioAntes != ana.ID || acc.UsuarioID != beto.ID {
		t.Fatalf("reclamo de Beto: %+v", acc)
	}
	if acc, _ := a.UsarCredencial(ctx, id, h("clave-ana")); acc.Ok {
		t.Fatal("la clave de Ana quedó revocada")
	}
	if _, err := a.UltimaConfig(ctx, id); err != ErrNoEncontrado {
		t.Fatal("la config de Ana se borró")
	}
	if l, _ := a.Imagenes(ctx, id); len(l) != 0 {
		t.Fatal("las imágenes de Ana se borraron")
	}
	if _, err := a.BichoDe(ctx, id, ana.ID); err != ErrNoEncontrado {
		t.Fatal("ya no es de Ana")
	}
	if err := a.QuitarDeCuenta(ctx, id, beto.ID); err != nil {
		t.Fatal(err)
	}
	if acc, _ := a.UsarCredencial(ctx, id, h("clave-beto")); acc.Ok {
		t.Fatal("sacarlo de la cuenta revoca la clave")
	}
}

func TestConfigsYSincro(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	u, _ := a.UsuarioPorEmail(ctx, "ana@ejemplo.com", false)
	const id = "00AABBCCDDEE"
	a.AsegurarBicho(ctx, id, "", true) //nolint:errcheck
	c1, _ := a.NuevaConfig(ctx, id, `{"version":1}`, 11, "web", &u.ID)
	c2, _ := a.NuevaConfig(ctx, id, `{"version":1}`, 11, "web", &u.ID)
	c3, _ := a.NuevaConfig(ctx, id, `{"version":1,"nombre":"Toto"}`, 22, "app", &u.ID)
	if c1.Version != 1 || c2.Version != 1 || c3.Version != 2 {
		t.Fatalf("versiones %d %d %d", c1.Version, c2.Version, c3.Version)
	}
	ult, _ := a.UltimaConfig(ctx, id)
	if ult.Crc != 22 || ult.Autor != "app" {
		t.Fatalf("última: %+v", ult)
	}
	s, _ := a.LeerSincro(ctx, id)
	if s.ConfigCrc != nil || len(s.Imagenes) != 0 {
		t.Fatal("sin acuerdo todavía")
	}
	a.AcordarConfig(ctx, id, 22)    //nolint:errcheck
	a.AcordarImagen(ctx, id, 3, 77) //nolint:errcheck
	a.AcordarImagen(ctx, id, 5, 88) //nolint:errcheck
	a.AcordarImagen(ctx, id, 3, 0)  //nolint:errcheck
	s, _ = a.LeerSincro(ctx, id)
	if s.ConfigCrc == nil || *s.ConfigCrc != 22 || len(s.Imagenes) != 1 || s.Imagenes[5] != 88 {
		t.Fatalf("sincro: %+v", s)
	}
}
