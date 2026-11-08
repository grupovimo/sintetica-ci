package almacen

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// EntradaTelegram es un pedido de "Entrar con Telegram" (docs/nube.md, "Cuentas").
// Estados: pendiente (nadie tocó Start) → confirmar (un chat la tomó y falta
// que diga que sí, o que cree la cuenta) → aprobada → usada (abrió la sesión).
// O rechazada ("No" / "No fui yo").
type EntradaTelegram struct {
	ID        int64
	Cliente   string
	Estado    string
	ChatID    int64
	UsuarioID string
	Vence     time.Time
}

// ErrEntradaInvalida: el link de entrar no existe, venció, ya se usó o es de otro navegador.
var ErrEntradaInvalida = errors.New("ese pedido para entrar ya se usó o venció: empezá de nuevo")

// CrearEntradaTelegram guarda el pedido (solo los hashes del token y del secreto).
func (a *Almacen) CrearEntradaTelegram(ctx context.Context, tokenHash, secretoHash []byte, cliente, ip string, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into telegram_entradas (token_hash, secreto_hash, cliente, ip, vence) values ($1, $2, $3, $4, $5)`,
		tokenHash, secretoHash, cliente, ip, vence)
	return err
}

// EntradasTelegramDeChat cuenta los pedidos que tomó ese chat desde `desde`.
func (a *Almacen) EntradasTelegramDeChat(ctx context.Context, chatID int64, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from telegram_entradas where chat_id = $1 and creado > $2`, chatID, desde).Scan(&n)
	return n, err
}

// TomarEntradaTelegram: el chat que tocó Start se queda con el pedido (una
// sola vez: otro chat con el mismo link ya no puede).
func (a *Almacen) TomarEntradaTelegram(ctx context.Context, tokenHash []byte, chatID int64) (EntradaTelegram, error) {
	e := EntradaTelegram{ChatID: chatID, Estado: "confirmar"}
	err := a.db.QueryRow(ctx, `update telegram_entradas set chat_id = $2, estado = 'confirmar'
		where token_hash = $1 and estado = 'pendiente' and chat_id is null and vence > now()
		returning id, cliente, vence`, tokenHash, chatID).Scan(&e.ID, &e.Cliente, &e.Vence)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrEntradaInvalida
	}
	return e, err
}

// EntradaTelegramDe devuelve el pedido `id` si es de ese chat.
func (a *Almacen) EntradaTelegramDe(ctx context.Context, id, chatID int64) (EntradaTelegram, error) {
	e := EntradaTelegram{ID: id, ChatID: chatID}
	var usuario *string
	err := a.db.QueryRow(ctx, `select cliente, estado, usuario_id::text, vence from telegram_entradas where id = $1 and chat_id = $2`, id, chatID).
		Scan(&e.Cliente, &e.Estado, &usuario, &e.Vence)
	if usuario != nil {
		e.UsuarioID = *usuario
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrEntradaInvalida
	}
	return e, err
}

// AprobarEntradaTelegram: el chat dijo que sí; queda para la cuenta `usuarioID`
// con el código de 6 dígitos (su hash).
func (a *Almacen) AprobarEntradaTelegram(ctx context.Context, id, chatID int64, usuarioID string, codigoHash []byte) error {
	t, err := a.db.Exec(ctx, `update telegram_entradas set estado = 'aprobada', usuario_id = $3, codigo_hash = $4
		where id = $1 and chat_id = $2 and estado = 'confirmar' and vence > now()`, id, chatID, usuarioID, codigoHash)
	if err == nil && t.RowsAffected() == 0 {
		return ErrEntradaInvalida
	}
	return err
}

// RechazarEntradaTelegram: "No" o "No fui yo". Si ya había abierto una
// sesión, la cierra. Devuelve si cerró una.
func (a *Almacen) RechazarEntradaTelegram(ctx context.Context, id, chatID int64) (cerro bool, err error) {
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		var sesion []byte
		e := tx.QueryRow(ctx, `update telegram_entradas set estado = 'rechazada'
			where id = $1 and chat_id = $2 and estado <> 'rechazada' returning sesion_hash`, id, chatID).Scan(&sesion)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrEntradaInvalida
		}
		if e != nil || sesion == nil {
			return e
		}
		t, e := tx.Exec(ctx, `delete from sesiones where token_hash = $1`, sesion)
		cerro = e == nil && t.RowsAffected() > 0
		return e
	})
	return
}

// CuentaDeTelegram devuelve la cuenta vinculada a ese chat; si no hay, la
// crea (sin email, con `nombre`) y la vincula. creada dice si es nueva.
func (a *Almacen) CuentaDeTelegram(ctx context.Context, chatID int64, nombreChat, nombre string) (usuarioID string, creada bool, err error) {
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `select usuario_id from telegram_vinculos where chat_id = $1`, chatID).Scan(&usuarioID)
		if e == nil || !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if e := tx.QueryRow(ctx, `insert into usuarios (email, nombre) values (null, $1) returning id`, nombre).Scan(&usuarioID); e != nil {
			return e
		}
		creada = true
		_, e = tx.Exec(ctx, `insert into telegram_vinculos (usuario_id, chat_id, nombre) values ($1, $2, $3)`, usuarioID, chatID, nombreChat)
		return e
	})
	return
}

// CanjearEntradaTelegram: el navegador (o la app) que pidió la entrada
// pregunta si ya está. Tiene que mostrar el mismo secreto. codigoHash nil =
// solo preguntar; con código, además tiene que coincidir con el que mandó el
// bot (5 intentos). Si está aprobada, queda usada y vuelve con la cuenta;
// si no, vuelve con su estado.
func (a *Almacen) CanjearEntradaTelegram(ctx context.Context, tokenHash, secretoHash, codigoHash []byte) (EntradaTelegram, error) {
	var e EntradaTelegram
	equivocado := false
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var secreto, codigo []byte
		var usuario *string
		var intentos int
		err := tx.QueryRow(ctx, `select id, cliente, estado, coalesce(chat_id, 0), usuario_id::text, vence, secreto_hash, codigo_hash, intentos
			from telegram_entradas where token_hash = $1 and vence > now() for update`, tokenHash).
			Scan(&e.ID, &e.Cliente, &e.Estado, &e.ChatID, &usuario, &e.Vence, &secreto, &codigo, &intentos)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEntradaInvalida
		}
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(secreto, secretoHash) != 1 || e.Estado == "usada" || e.Estado == "rechazada" {
			return ErrEntradaInvalida
		}
		if codigoHash != nil {
			if intentos >= maxIntentosCodigo {
				return ErrDemasiadosIntentos
			}
			if e.Estado != "aprobada" || subtle.ConstantTimeCompare(codigo, codigoHash) != 1 {
				equivocado = true
				_, err := tx.Exec(ctx, `update telegram_entradas set intentos = intentos + 1 where id = $1`, e.ID)
				return err
			}
		} else if e.Estado != "aprobada" {
			return nil
		}
		if usuario != nil {
			e.UsuarioID = *usuario
		}
		e.Estado = "usada"
		_, err = tx.Exec(ctx, `update telegram_entradas set estado = 'usada' where id = $1`, e.ID)
		return err
	})
	if err == nil && equivocado {
		return e, ErrEnlaceInvalido
	}
	return e, err
}

// AnotarSesionEntrada guarda qué sesión abrió el pedido (para "No fui yo").
func (a *Almacen) AnotarSesionEntrada(ctx context.Context, id int64, sesionHash []byte) error {
	_, err := a.db.Exec(ctx, `update telegram_entradas set sesion_hash = $2 where id = $1`, id, sesionHash)
	return err
}

// UsuarioPorID devuelve una cuenta.
func (a *Almacen) UsuarioPorID(ctx context.Context, id string) (Usuario, error) {
	var u Usuario
	err := a.db.QueryRow(ctx, `select id, coalesce(email, ''), nombre, admin, creado from usuarios where id = $1`, id).
		Scan(&u.ID, &u.Email, &u.Nombre, &u.Admin, &u.Creado)
	return u, noEncontrado(err)
}
