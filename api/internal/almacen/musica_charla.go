package almacen

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Spotify ---

// CuentaSpotify: el refresh token (cifrado) de un usuario.
type CuentaSpotify struct {
	UsuarioID      string
	RefreshCifrado []byte
	Nombre         string
}

// GuardarSpotify pone (o reemplaza) la cuenta del usuario.
func (a *Almacen) GuardarSpotify(ctx context.Context, usuarioID string, cifrado []byte, nombre string) error {
	_, err := a.db.Exec(ctx, `insert into spotify_cuentas (usuario_id, refresh_cifrado, nombre) values ($1, $2, $3)
		on conflict (usuario_id) do update set refresh_cifrado = excluded.refresh_cifrado, nombre = excluded.nombre, creada = now()`,
		usuarioID, cifrado, nombre)
	return err
}

// SpotifyDe devuelve la cuenta del usuario.
func (a *Almacen) SpotifyDe(ctx context.Context, usuarioID string) (CuentaSpotify, error) {
	var c CuentaSpotify
	err := a.db.QueryRow(ctx, `select usuario_id, refresh_cifrado, nombre from spotify_cuentas where usuario_id = $1`, usuarioID).
		Scan(&c.UsuarioID, &c.RefreshCifrado, &c.Nombre)
	return c, noEncontrado(err)
}

// CuentasSpotify: los usuarios con Spotify conectado.
func (a *Almacen) CuentasSpotify(ctx context.Context) ([]string, error) {
	filas, err := a.db.Query(ctx, `select usuario_id::text from spotify_cuentas`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(filas, pgx.RowTo[string])
}

// BorrarSpotify la olvida.
func (a *Almacen) BorrarSpotify(ctx context.Context, usuarioID string) error {
	_, err := a.db.Exec(ctx, `delete from spotify_cuentas where usuario_id = $1`, usuarioID)
	return err
}

// --- Charla ---

// UsarCharla suma una charla del día (en la zona de los robots) si no pasó
// el tope. Devuelve cuántas lleva (con esta) y si se pudo.
func (a *Almacen) UsarCharla(ctx context.Context, usuarioID string, dia time.Time, tope int) (int, bool, error) {
	var n int
	err := a.db.QueryRow(ctx, `insert into charlas_uso (usuario_id, dia, cantidad) values ($1, $2::date, 1)
		on conflict (usuario_id, dia) do update set cantidad = charlas_uso.cantidad + 1
		where charlas_uso.cantidad < $3
		returning cantidad`, usuarioID, dia.Format("2006-01-02"), tope).Scan(&n)
	if noEncontrado(err) == ErrNoEncontrado {
		return tope, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// CharlasDeHoy: cuántas lleva el usuario ese día.
func (a *Almacen) CharlasDeHoy(ctx context.Context, usuarioID string, dia time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select coalesce((select cantidad from charlas_uso where usuario_id = $1 and dia = $2::date), 0)`,
		usuarioID, dia.Format("2006-01-02")).Scan(&n)
	return n, err
}

// AnotarRobotCharla: el último robot con el que charló el usuario.
func (a *Almacen) AnotarRobotCharla(ctx context.Context, usuarioID, bichoID string) error {
	_, err := a.db.Exec(ctx, `insert into charla_robot (usuario_id, bicho_id) values ($1, $2)
		on conflict (usuario_id) do update set bicho_id = excluded.bicho_id, en = now()`, usuarioID, bichoID)
	return err
}

// RobotCharla: el último (ErrNoEncontrado si nunca charló).
func (a *Almacen) RobotCharla(ctx context.Context, usuarioID string) (string, error) {
	var id string
	err := a.db.QueryRow(ctx, `select bicho_id from charla_robot where usuario_id = $1`, usuarioID).Scan(&id)
	return id, noEncontrado(err)
}
