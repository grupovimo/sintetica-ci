package almacen

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Telegram ---

// VinculoTelegram es el chat de Telegram de un usuario.
type VinculoTelegram struct {
	UsuarioID string
	ChatID    int64
	Nombre    string
	Creado    time.Time
}

// ErrCodigoInvalido: el código de vincular no existe, venció o ya se usó.
var ErrCodigoInvalido = errors.New("el código no existe, venció o ya se usó")

// CrearCodigoTelegram guarda un código de un solo uso para vincular el chat.
func (a *Almacen) CrearCodigoTelegram(ctx context.Context, usuarioID string, codigoHash []byte, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into telegram_codigos (codigo_hash, usuario_id, vence) values ($1, $2, $3)`, codigoHash, usuarioID, vence)
	return err
}

// CodigosTelegramRecientes cuenta los códigos que pidió el usuario desde `desde`.
func (a *Almacen) CodigosTelegramRecientes(ctx context.Context, usuarioID string, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from telegram_codigos where usuario_id = $1 and creado > $2`, usuarioID, desde).Scan(&n)
	return n, err
}

// VincularTelegram canjea el código (una sola vez, antes de que venza) y deja
// ese chat como el del usuario. Si el chat era de otro usuario, pasa a este.
func (a *Almacen) VincularTelegram(ctx context.Context, codigoHash []byte, chatID int64, nombre string) (string, error) {
	var usuario string
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `update telegram_codigos set usado = now()
			where codigo_hash = $1 and usado is null and vence > now() returning usuario_id`, codigoHash).Scan(&usuario)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCodigoInvalido
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from telegram_vinculos where chat_id = $1 and usuario_id <> $2`, chatID, usuario); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `insert into telegram_vinculos (usuario_id, chat_id, nombre) values ($1, $2, $3)
			on conflict (usuario_id) do update set chat_id = excluded.chat_id, nombre = excluded.nombre, creado = now()`, usuario, chatID, nombre)
		return err
	})
	return usuario, err
}

// TelegramDe devuelve el vínculo de un usuario.
func (a *Almacen) TelegramDe(ctx context.Context, usuarioID string) (VinculoTelegram, error) {
	var v VinculoTelegram
	err := a.db.QueryRow(ctx, `select usuario_id, chat_id, nombre, creado from telegram_vinculos where usuario_id = $1`, usuarioID).
		Scan(&v.UsuarioID, &v.ChatID, &v.Nombre, &v.Creado)
	return v, noEncontrado(err)
}

// TelegramPorChat devuelve el vínculo de un chat.
func (a *Almacen) TelegramPorChat(ctx context.Context, chatID int64) (VinculoTelegram, error) {
	var v VinculoTelegram
	err := a.db.QueryRow(ctx, `select usuario_id, chat_id, nombre, creado from telegram_vinculos where chat_id = $1`, chatID).
		Scan(&v.UsuarioID, &v.ChatID, &v.Nombre, &v.Creado)
	return v, noEncontrado(err)
}

// VinculosTelegram lista todos los usuarios con Telegram vinculado.
func (a *Almacen) VinculosTelegram(ctx context.Context) ([]VinculoTelegram, error) {
	filas, err := a.db.Query(ctx, `select usuario_id, chat_id, nombre, creado from telegram_vinculos`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []VinculoTelegram
	for filas.Next() {
		var v VinculoTelegram
		if err := filas.Scan(&v.UsuarioID, &v.ChatID, &v.Nombre, &v.Creado); err != nil {
			return nil, err
		}
		l = append(l, v)
	}
	return l, filas.Err()
}

// DesvincularTelegram borra el vínculo del usuario.
func (a *Almacen) DesvincularTelegram(ctx context.Context, usuarioID string) error {
	_, err := a.db.Exec(ctx, `delete from telegram_vinculos where usuario_id = $1`, usuarioID)
	return err
}

// OffsetTelegram es el próximo update a pedir (0 si nunca se leyó).
func (a *Almacen) OffsetTelegram(ctx context.Context) (int64, error) {
	var o int64
	err := a.db.QueryRow(ctx, `select offset_siguiente from telegram_offset where id = 1`).Scan(&o)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return o, err
}

// GuardarOffsetTelegram anota hasta dónde se leyó.
func (a *Almacen) GuardarOffsetTelegram(ctx context.Context, o int64) error {
	_, err := a.db.Exec(ctx, `insert into telegram_offset (id, offset_siguiente) values (1, $1)
		on conflict (id) do update set offset_siguiente = excluded.offset_siguiente`, o)
	return err
}

// --- Webhooks ---

// SecretoWebhook devuelve el secreto del usuario; si no tiene, guarda `nuevo`.
func (a *Almacen) SecretoWebhook(ctx context.Context, usuarioID, nuevo string) (string, error) {
	var s string
	err := a.db.QueryRow(ctx, `insert into webhook_secretos (usuario_id, secreto) values ($1, $2)
		on conflict (usuario_id) do update set usuario_id = excluded.usuario_id returning secreto`, usuarioID, nuevo).Scan(&s)
	return s, err
}

// RotarSecretoWebhook reemplaza el secreto del usuario.
func (a *Almacen) RotarSecretoWebhook(ctx context.Context, usuarioID, nuevo string) error {
	_, err := a.db.Exec(ctx, `insert into webhook_secretos (usuario_id, secreto) values ($1, $2)
		on conflict (usuario_id) do update set secreto = excluded.secreto, creado = now()`, usuarioID, nuevo)
	return err
}

// --- Automatizaciones ---

// Automatizaciones devuelve la lista (el JSON tal cual se guardó; "[]" si no hay).
func (a *Almacen) Automatizaciones(ctx context.Context, bichoID string) (json.RawMessage, error) {
	var l []byte
	err := a.db.QueryRow(ctx, `select lista from automatizaciones where bicho_id = $1`, bichoID).Scan(&l)
	if errors.Is(err, pgx.ErrNoRows) {
		return json.RawMessage("[]"), nil
	}
	return l, err
}

// GuardarAutomatizaciones reemplaza la lista entera.
func (a *Almacen) GuardarAutomatizaciones(ctx context.Context, bichoID string, lista json.RawMessage) error {
	_, err := a.db.Exec(ctx, `insert into automatizaciones (bicho_id, lista) values ($1, $2)
		on conflict (bicho_id) do update set lista = excluded.lista, actualizada = now()`, bichoID, []byte(lista))
	return err
}

// Ejecucion es una entrada del historial de automatizaciones.
type Ejecucion struct {
	Automatizacion string    `json:"automatizacion"`
	Evento         string    `json:"evento"`
	Accion         string    `json:"accion"`
	Ok             bool      `json:"ok"`
	Detalle        string    `json:"detalle,omitempty"`
	En             time.Time `json:"en"`
}

// MaxHistorial: cuántas ejecuciones se guardan por bicho.
const MaxHistorial = 50

// AnotarEjecucion suma al historial y deja solo las últimas 50.
func (a *Almacen) AnotarEjecucion(ctx context.Context, bichoID string, e Ejecucion) error {
	if len(e.Detalle) > 300 {
		e.Detalle = e.Detalle[:300]
	}
	return a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `insert into automatizaciones_historial (bicho_id, automatizacion, evento, accion, ok, detalle, en)
			values ($1, $2, $3, $4, $5, $6, $7)`, bichoID, e.Automatizacion, e.Evento, e.Accion, e.Ok, e.Detalle, e.En); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `delete from automatizaciones_historial where bicho_id = $1 and id not in
			(select id from automatizaciones_historial where bicho_id = $1 order by id desc limit $2)`, bichoID, MaxHistorial)
		return err
	})
}

// Historial devuelve las últimas ejecuciones (la más nueva primero).
func (a *Almacen) Historial(ctx context.Context, bichoID string) ([]Ejecucion, error) {
	filas, err := a.db.Query(ctx, `select automatizacion, evento, accion, ok, detalle, en from automatizaciones_historial
		where bicho_id = $1 order by id desc limit $2`, bichoID, MaxHistorial)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Ejecucion{}
	for filas.Next() {
		var e Ejecucion
		if err := filas.Scan(&e.Automatizacion, &e.Evento, &e.Accion, &e.Ok, &e.Detalle, &e.En); err != nil {
			return nil, err
		}
		l = append(l, e)
	}
	return l, filas.Err()
}
