package almacen

// Lo que sobrevive a un reinicio de la API (docs/nube.md, "Lo que sobrevive a
// un reinicio"): los topes que importan, lo que se hace una sola vez y el
// último toque de cada robot. Todo con vencimiento: Limpiar lo borra.

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Tope deja pasar (y lo anota) si para esa clave hubo menos de `max` en la
// última `ventana` y el último fue hace más de `cada`. Dos pedidos a la vez
// con la misma clave se ordenan con un candado de la transacción.
func (a *Almacen) Tope(ctx context.Context, clave string, cada time.Duration, max int, ventana time.Duration) (bool, error) {
	ok := false
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 7))`, clave); err != nil {
			return err
		}
		var n int
		var cerca bool
		if err := tx.QueryRow(ctx, `select count(*), coalesce(bool_or(en > now() - make_interval(secs => $3)), false)
			from topes where clave = $1 and en > now() - make_interval(secs => $2)`,
			clave, ventana.Seconds(), cada.Seconds()).Scan(&n, &cerca); err != nil {
			return err
		}
		if n >= max || (cada > 0 && cerca) {
			return nil
		}
		ok = true
		_, err := tx.Exec(ctx, `insert into topes (clave, vence) values ($1, now() + make_interval(secs => $2))`, clave, ventana.Seconds())
		return err
	})
	return ok && err == nil, err
}

// PrimeraVez: true la primera vez que se pide esa clave (y la anota hasta
// `dura`); false si ya estaba y no venció.
func (a *Almacen) PrimeraVez(ctx context.Context, clave string, dura time.Duration) (bool, error) {
	var x bool
	err := a.db.QueryRow(ctx, `insert into vistos (clave, vence) values ($1, now() + make_interval(secs => $2))
		on conflict (clave) do update set vence = excluded.vence where vistos.vence < now()
		returning true`, clave, dura.Seconds()).Scan(&x)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ToqueRecibido es el último toque que le llegó a un robot.
type ToqueRecibido struct {
	DeUsuario string    // el que tocó
	Desde     string    // su robot, si lo mandó desde uno
	Tipo      string    // toque, abrazo, beso o carita
	Expresion string    // la de la carita
	En        time.Time //
}

// AnotarToque guarda el último toque que le llegó a ese robot (pisa el anterior).
func (a *Almacen) AnotarToque(ctx context.Context, bicho string, t ToqueRecibido) error {
	_, err := a.db.Exec(ctx, `insert into toques_recibidos (bicho_id, de_usuario, desde, tipo, expresion, en) values ($1, $2, $3, $4, $5, $6)
		on conflict (bicho_id) do update set de_usuario = excluded.de_usuario, desde = excluded.desde, tipo = excluded.tipo,
		expresion = excluded.expresion, en = excluded.en`, bicho, t.DeUsuario, t.Desde, t.Tipo, t.Expresion, t.En)
	return err
}

// TomarToque saca el último toque de ese robot (una sola vez). ok si llegó
// dentro de `ventana`.
func (a *Almacen) TomarToque(ctx context.Context, bicho string, ventana time.Duration) (ToqueRecibido, bool, error) {
	var t ToqueRecibido
	err := a.db.QueryRow(ctx, `delete from toques_recibidos where bicho_id = $1
		returning de_usuario::text, desde, tipo, expresion, en`, bicho).Scan(&t.DeUsuario, &t.Desde, &t.Tipo, &t.Expresion, &t.En)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	return t, time.Since(t.En) <= ventana, nil
}

// limpiarTopes: lo vencido (barato: por índice).
func (a *Almacen) limpiarTopes(ctx context.Context) error {
	if _, err := a.db.Exec(ctx, `delete from topes where vence < now()`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from vistos where vence < now()`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from toques_recibidos where en < now() - interval '1 hour'`); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, `delete from email_pendientes where vence < now() - interval '1 day'`)
	return err
}
