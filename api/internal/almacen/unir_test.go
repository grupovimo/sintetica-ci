package almacen

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var nBicho int

// darBicho le da a la cuenta un robot (deja de estar vacía).
func darBicho(t *testing.T, a *Almacen, usuario string) {
	t.Helper()
	nBicho++
	id := fmt.Sprintf("0A11%08X", nBicho)
	ctx := context.Background()
	if err := a.AsegurarBicho(ctx, id, "virtual", true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(ctx, `update bichos set usuario_id = $2 where id = $1`, id, usuario); err != nil {
		t.Fatal(err)
	}
}

var nChat int64 = 7_000_000

func cuentaTelegram(t *testing.T, a *Almacen, nombre string) (string, int64) {
	t.Helper()
	nChat++
	u, creada, err := a.CuentaDeTelegram(context.Background(), nChat, "@"+nombre, nombre)
	if err != nil || !creada {
		t.Fatalf("cuenta de Telegram: %v", err)
	}
	return u, nChat
}

func existe(t *testing.T, a *Almacen, id string) bool {
	_, err := a.UsuarioPorID(context.Background(), id)
	return err == nil
}

func sumarMail(t *testing.T, a *Almacen, usuario, email string) (EmailUnido, error) {
	t.Helper()
	if err := a.PedirEmail(context.Background(), usuario, email, h("COD-"+email), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return a.ConfirmarEmail(context.Background(), usuario, h("COD-"+email))
}

func TestSumarMail(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()

	// Telegram sin mail: suma uno nuevo (el código equivocado cuenta un intento).
	tg, _ := cuentaTelegram(t, a, "Ana")
	darBicho(t, a, tg)
	a.PedirEmail(ctx, tg, "ana@ejemplo.com", h("BUENO"), time.Now().Add(time.Minute)) //nolint:errcheck
	if _, err := a.ConfirmarEmail(ctx, tg, h("MALO")); err != ErrEnlaceInvalido {
		t.Fatalf("código malo: %v", err)
	}
	r, err := a.ConfirmarEmail(ctx, tg, h("BUENO"))
	if err != nil || r.Unida || r.Usuario.ID != tg || r.Usuario.Email != "ana@ejemplo.com" {
		t.Fatalf("sumar: %+v %v", r, err)
	}
	if _, err := a.ConfirmarEmail(ctx, tg, h("BUENO")); err != ErrEnlaceInvalido {
		t.Fatal("el código es de un solo uso")
	}
	// Entrar por mail con ese mail es la misma cuenta.
	if u, _ := a.UsuarioPorEmail(ctx, "ana@ejemplo.com", false); u.ID != tg {
		t.Fatal("entrar por mail duplicó la cuenta")
	}
	if _, err := sumarMail(t, a, tg, "otro@ejemplo.com"); !errors.Is(err, ErrYaTieneMail) {
		t.Fatalf("ya tiene: %v", err)
	}

	// El mail es de una cuenta vacía (entró por mail y no hizo nada): se suma a esta.
	vacia, _ := a.UsuarioPorEmail(ctx, "beto@ejemplo.com", true)
	a.CrearSesion(ctx, h("sesion-beto"), vacia.ID, "web", "", time.Now().Add(time.Hour)) //nolint:errcheck
	tg2, _ := cuentaTelegram(t, a, "Beto")
	darBicho(t, a, tg2)
	r, err = sumarMail(t, a, tg2, "beto@ejemplo.com")
	if err != nil || r.Unida || r.Usuario.ID != tg2 || r.Usuario.Email != "beto@ejemplo.com" || !r.Usuario.Admin {
		t.Fatalf("unir la vacía: %+v %v", r, err)
	}
	if existe(t, a, vacia.ID) {
		t.Fatal("la vacía se borra")
	}
	if s, err := a.SesionPorToken(ctx, h("sesion-beto"), time.Hour); err != nil || s.Usuario.ID != tg2 {
		t.Fatalf("su sesión pasa a la que queda: %+v %v", s, err)
	}

	// Esta está vacía (recién creada con Telegram) y el mail tiene robots: esta se suma a la del mail.
	conMail, _ := a.UsuarioPorEmail(ctx, "caro@ejemplo.com", false)
	darBicho(t, a, conMail.ID)
	tg3, chat3 := cuentaTelegram(t, a, "Caro")
	a.CrearSesion(ctx, h("sesion-caro"), tg3, "app", "", time.Now().Add(time.Hour)) //nolint:errcheck
	r, err = sumarMail(t, a, tg3, "caro@ejemplo.com")
	if err != nil || !r.Unida || r.Usuario.ID != conMail.ID {
		t.Fatalf("sumarse a la del mail: %+v %v", r, err)
	}
	if v, err := a.TelegramPorChat(ctx, chat3); err != nil || v.UsuarioID != conMail.ID {
		t.Fatalf("el Telegram pasa a la del mail: %+v %v", v, err)
	}
	if s, _ := a.SesionPorToken(ctx, h("sesion-caro"), time.Hour); s.Usuario.ID != conMail.ID {
		t.Fatal("la sesión abierta sigue, en la cuenta que quedó")
	}
	if existe(t, a, tg3) {
		t.Fatal("la de Telegram vacía se borra")
	}

	// Las dos con robots: no se toca nada.
	dato, _ := a.UsuarioPorEmail(ctx, "dani@ejemplo.com", false)
	darBicho(t, a, dato.ID)
	tg4, _ := cuentaTelegram(t, a, "Dani")
	darBicho(t, a, tg4)
	if _, err := sumarMail(t, a, tg4, "dani@ejemplo.com"); !errors.Is(err, ErrCuentasConDatos) {
		t.Fatalf("las dos con datos: %v", err)
	}
	if u, _ := a.UsuarioPorID(ctx, tg4); u.Email != "" {
		t.Fatal("no se sumó el mail")
	}

	// Las dos con un Telegram distinto: tampoco.
	conTG, _ := a.UsuarioPorEmail(ctx, "eli@ejemplo.com", false)
	a.db.Exec(ctx, `insert into telegram_vinculos (usuario_id, chat_id) values ($1, 6999999)`, conTG.ID) //nolint:errcheck
	tg5, _ := cuentaTelegram(t, a, "Eli")
	if _, err := sumarMail(t, a, tg5, "eli@ejemplo.com"); !errors.Is(err, ErrOtroTelegram) {
		t.Fatalf("dos Telegram: %v", err)
	}
}

func TestVincularTelegramSinDuplicar(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	codigo := func(usuario, c string) []byte {
		if err := a.CrearCodigoTelegram(ctx, usuario, h(c), time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		return h(c)
	}

	// Entró por mail (vacía) y vincula el Telegram con el que ya tenía su cuenta: queda la de Telegram, con el mail.
	mail, _ := a.UsuarioPorEmail(ctx, "fede@ejemplo.com", false)
	a.CrearSesion(ctx, h("sesion-fede"), mail.ID, "web", "", time.Now().Add(time.Hour)) //nolint:errcheck
	tg, chat := cuentaTelegram(t, a, "Fede")
	darBicho(t, a, tg)
	r, err := a.VincularTelegram(ctx, codigo(mail.ID, "c1"), chat, "@fede")
	if err != nil || !r.Unida || r.Usuario != tg || r.Pidio != mail.ID {
		t.Fatalf("unir: %+v %v", r, err)
	}
	if u, _ := a.UsuarioPorID(ctx, tg); u.Email != "fede@ejemplo.com" {
		t.Fatalf("el mail pasa a la de Telegram: %+v", u)
	}
	if s, _ := a.SesionPorToken(ctx, h("sesion-fede"), time.Hour); s.Usuario.ID != tg {
		t.Fatal("la sesión de la web sigue, en la cuenta que quedó")
	}

	// La de mail tiene robots y la de Telegram está vacía: la de Telegram se suma.
	mail2, _ := a.UsuarioPorEmail(ctx, "gabi@ejemplo.com", false)
	darBicho(t, a, mail2.ID)
	tg2, chat2 := cuentaTelegram(t, a, "Gabi")
	r, err = a.VincularTelegram(ctx, codigo(mail2.ID, "c2"), chat2, "@gabi")
	if err != nil || r.Unida || r.Usuario != mail2.ID {
		t.Fatalf("sumar la vacía: %+v %v", r, err)
	}
	if v, _ := a.TelegramPorChat(ctx, chat2); v.UsuarioID != mail2.ID || existe(t, a, tg2) {
		t.Fatal("el chat pasa y la vacía se borra")
	}

	// Las dos con robots: no, y el código sigue sirviendo.
	mail3, _ := a.UsuarioPorEmail(ctx, "hugo@ejemplo.com", false)
	darBicho(t, a, mail3.ID)
	tg3, chat3 := cuentaTelegram(t, a, "Hugo")
	darBicho(t, a, tg3)
	c3 := codigo(mail3.ID, "c3")
	if _, err := a.VincularTelegram(ctx, c3, chat3, "@hugo"); !errors.Is(err, ErrCuentasConDatos) {
		t.Fatalf("las dos con datos: %v", err)
	}
	if v, _ := a.TelegramPorChat(ctx, chat3); v.UsuarioID != tg3 {
		t.Fatal("no cambia nada")
	}
	// El chat de una cuenta con mail: pasa (la otra sigue entrando por mail), como siempre.
	mail4, _ := a.UsuarioPorEmail(ctx, "ines@ejemplo.com", false)
	darBicho(t, a, mail4.ID)
	if _, err := a.VincularTelegram(ctx, codigo(mail4.ID, "c4"), chat2, "@gabi"); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.TelegramPorChat(ctx, chat2); v.UsuarioID != mail4.ID || !existe(t, a, mail2.ID) {
		t.Fatal("el chat pasa y la otra cuenta queda")
	}
}
