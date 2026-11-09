package almacen

// Unir cuentas (docs/nube.md, "Cuentas", "Una sola cuenta por persona"): una
// cuenta creada con Telegram (sin mail) puede sumar un mail, y una de mail
// puede vincular Telegram. Si el mail o el Telegram ya son de otra cuenta, no
// se duplican: la cuenta vacía (sin robots, amigos, biblioteca ni avisos
// entrantes) se suma a la otra (su mail, su Telegram y sus sesiones abiertas
// pasan a la que queda) y se borra. Si las dos tienen cosas, no se tocan:
// ErrCuentasConDatos.

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Errores de unir cuentas (se muestran tal cual).
var (
	ErrCuentasConDatos = errors.New("ese mail o ese Telegram ya es de otra cuenta con robots o amigos: entrá con esa y sacá lo que no quieras de una de las dos")
	ErrOtroTelegram    = errors.New("las dos cuentas tienen un Telegram distinto: desvinculá uno primero")
	ErrYaTieneMail     = errors.New("tu cuenta ya tiene un mail")
)

// cuentaVacia: sin robots, amigos, biblioteca ni avisos entrantes (lo que se
// perdería al borrarla).
func cuentaVacia(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var hay bool
	err := tx.QueryRow(ctx, `select exists(select 1 from bichos where usuario_id = $1)
		or exists(select 1 from amistades where usuario_a = $1 or usuario_b = $1)
		or exists(select 1 from biblioteca where usuario_id = $1)
		or exists(select 1 from entrantes where usuario_id = $1)`, id).Scan(&hay)
	return !hay, err
}

// unir suma la cuenta `desde` (vacía) a `hacia` y la borra: el mail (si
// `hacia` no tiene), el admin, el nombre (si `hacia` no tiene), el Telegram
// (si `hacia` no tiene: si tienen uno cada una, ErrOtroTelegram) y las
// sesiones abiertas.
func unir(ctx context.Context, tx pgx.Tx, desde, hacia string) error {
	var tgDesde, tgHacia bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from telegram_vinculos where usuario_id = $1),
		exists(select 1 from telegram_vinculos where usuario_id = $2)`, desde, hacia).Scan(&tgDesde, &tgHacia); err != nil {
		return err
	}
	if tgDesde && tgHacia {
		return ErrOtroTelegram
	}
	var email *string
	var admin bool
	var nombre string
	if err := tx.QueryRow(ctx, `select email, admin, nombre from usuarios where id = $1 for update`, desde).Scan(&email, &admin, &nombre); err != nil {
		return noEncontrado(err)
	}
	if _, err := tx.Exec(ctx, `update usuarios set email = null where id = $1`, desde); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update usuarios set email = coalesce(email, $2), admin = admin or $3,
		nombre = case when nombre = '' then $4 else nombre end where id = $1`, hacia, email, admin, nombre); err != nil {
		return err
	}
	for _, q := range []string{
		`update telegram_vinculos set usuario_id = $2 where usuario_id = $1`,
		`update sesiones set usuario_id = $2 where usuario_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, desde, hacia); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `delete from usuarios where id = $1`, desde)
	return err
}

// --- Sumar un mail ---

// PedirEmail guarda el código (su hash) para sumarle `email` a la cuenta. Uno
// pendiente por cuenta: pedir otro pisa el anterior.
func (a *Almacen) PedirEmail(ctx context.Context, usuarioID, email string, codigoHash []byte, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into email_pendientes (usuario_id, email, codigo_hash, vence) values ($1, $2, $3, $4)
		on conflict (usuario_id) do update set email = excluded.email, codigo_hash = excluded.codigo_hash,
		intentos = 0, vence = excluded.vence, creado = now()`, usuarioID, email, codigoHash, vence)
	return err
}

// EmailUnido es lo que quedó después de sumar un mail.
type EmailUnido struct {
	Usuario Usuario // la cuenta que queda (puede ser la otra, si esta estaba vacía)
	Unida   bool    // se unieron dos cuentas
}

// ConfirmarEmail canjea el código (5 intentos): la cuenta queda con ese mail.
// Si el mail ya es de otra cuenta, se unen (la vacía se suma a la otra).
func (a *Almacen) ConfirmarEmail(ctx context.Context, usuarioID string, codigoHash []byte) (EmailUnido, error) {
	var r EmailUnido
	equivocado := false
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var email string
		var hash []byte
		var intentos int
		err := tx.QueryRow(ctx, `select email, codigo_hash, intentos from email_pendientes
			where usuario_id = $1 and vence > now() for update`, usuarioID).Scan(&email, &hash, &intentos)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEnlaceInvalido
		}
		if err != nil {
			return err
		}
		if intentos >= maxIntentosCodigo {
			return ErrDemasiadosIntentos
		}
		if subtle.ConstantTimeCompare(hash, codigoHash) != 1 {
			equivocado = true
			_, err := tx.Exec(ctx, `update email_pendientes set intentos = intentos + 1 where usuario_id = $1`, usuarioID)
			return err
		}
		if _, err := tx.Exec(ctx, `delete from email_pendientes where usuario_id = $1`, usuarioID); err != nil {
			return err
		}
		var mio *string
		if err := tx.QueryRow(ctx, `select email from usuarios where id = $1 for update`, usuarioID).Scan(&mio); err != nil {
			return noEncontrado(err)
		}
		if mio != nil && *mio != email {
			return ErrYaTieneMail
		}
		queda := usuarioID
		var otro string
		err = tx.QueryRow(ctx, `select id from usuarios where email = $1 and id <> $2 for update`, email, usuarioID).Scan(&otro)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := tx.Exec(ctx, `update usuarios set email = $2 where id = $1`, usuarioID, email); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			otraVacia, err := cuentaVacia(ctx, tx, otro)
			if err != nil {
				return err
			}
			estaVacia, err := cuentaVacia(ctx, tx, usuarioID)
			if err != nil {
				return err
			}
			switch {
			case otraVacia:
				err = unir(ctx, tx, otro, usuarioID) // le trae el mail
			case estaVacia:
				err = unir(ctx, tx, usuarioID, otro) // esta se suma a la del mail
				queda, r.Unida = otro, true
			default:
				err = ErrCuentasConDatos
			}
			if err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `select id, coalesce(email, ''), nombre, admin, creado from usuarios where id = $1`, queda).
			Scan(&r.Usuario.ID, &r.Usuario.Email, &r.Usuario.Nombre, &r.Usuario.Admin, &r.Usuario.Creado)
	})
	if err == nil && equivocado {
		return r, ErrEnlaceInvalido
	}
	return r, err
}

// MarcarAdmin deja la cuenta como administradora (ADMIN_TELEGRAM).
func (a *Almacen) MarcarAdmin(ctx context.Context, usuarioID string) error {
	_, err := a.db.Exec(ctx, `update usuarios set admin = true where id = $1`, usuarioID)
	return err
}
