package almacen

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// CuentaCorreo: un Gmail o un Outlook conectado (el refresh token, cifrado).
type CuentaCorreo struct {
	UsuarioID      string
	Proveedor      string // "gmail" u "outlook"
	RefreshCifrado []byte
	Direccion      string
	Revisado       *time.Time // nil = todavía no se miró
}

const columnasCorreo = `usuario_id::text, proveedor, refresh_cifrado, direccion, revisado`

func leerCuentaCorreo(f pgx.CollectableRow) (CuentaCorreo, error) {
	var c CuentaCorreo
	err := f.Scan(&c.UsuarioID, &c.Proveedor, &c.RefreshCifrado, &c.Direccion, &c.Revisado)
	return c, err
}

// GuardarCorreo pone (o reemplaza) la cuenta; la marca vuelve a cero (la
// primera vuelta no avisa).
func (a *Almacen) GuardarCorreo(ctx context.Context, usuarioID, proveedor string, cifrado []byte, direccion string) error {
	_, err := a.db.Exec(ctx, `insert into correo_cuentas (usuario_id, proveedor, refresh_cifrado, direccion) values ($1, $2, $3, $4)
		on conflict (usuario_id, proveedor) do update set refresh_cifrado = excluded.refresh_cifrado,
		direccion = excluded.direccion, revisado = null, creada = now()`,
		usuarioID, proveedor, cifrado, direccion)
	return err
}

// CambiarRefreshCorreo: el proveedor dio otro refresh token (Microsoft los rota).
func (a *Almacen) CambiarRefreshCorreo(ctx context.Context, usuarioID, proveedor string, cifrado []byte) error {
	_, err := a.db.Exec(ctx, `update correo_cuentas set refresh_cifrado = $3 where usuario_id = $1 and proveedor = $2`,
		usuarioID, proveedor, cifrado)
	return err
}

// CorreoDe devuelve una cuenta.
func (a *Almacen) CorreoDe(ctx context.Context, usuarioID, proveedor string) (CuentaCorreo, error) {
	f, err := a.db.Query(ctx, `select `+columnasCorreo+` from correo_cuentas where usuario_id = $1 and proveedor = $2`, usuarioID, proveedor)
	if err != nil {
		return CuentaCorreo{}, err
	}
	c, err := pgx.CollectExactlyOneRow(f, leerCuentaCorreo)
	return c, noEncontrado(err)
}

// CorreosDe: las cuentas del usuario.
func (a *Almacen) CorreosDe(ctx context.Context, usuarioID string) ([]CuentaCorreo, error) {
	f, err := a.db.Query(ctx, `select `+columnasCorreo+` from correo_cuentas where usuario_id = $1 order by proveedor`, usuarioID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(f, leerCuentaCorreo)
}

// CuentasCorreo: todas (para el sondeo).
func (a *Almacen) CuentasCorreo(ctx context.Context) ([]CuentaCorreo, error) {
	f, err := a.db.Query(ctx, `select `+columnasCorreo+` from correo_cuentas order by usuario_id, proveedor`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(f, leerCuentaCorreo)
}

// MarcarCorreo: hasta cuándo se miró.
func (a *Almacen) MarcarCorreo(ctx context.Context, usuarioID, proveedor string, t time.Time) error {
	_, err := a.db.Exec(ctx, `update correo_cuentas set revisado = $3 where usuario_id = $1 and proveedor = $2`, usuarioID, proveedor, t)
	return err
}

// BorrarCorreo la olvida.
func (a *Almacen) BorrarCorreo(ctx context.Context, usuarioID, proveedor string) error {
	_, err := a.db.Exec(ctx, `delete from correo_cuentas where usuario_id = $1 and proveedor = $2`, usuarioID, proveedor)
	return err
}
