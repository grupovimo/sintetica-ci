package almacen

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Las licencias del escritorio (migraciones/021_licencias.sql): pago único,
// una por cuenta, cobrada con Mercado Pago como las señas.

// Licencia de un producto (hoy, "escritorio") para una cuenta.
type Licencia struct {
	ID            int64      `json:"id"`
	UsuarioID     string     `json:"usuarioId"`
	Producto      string     `json:"producto"`
	Monto         float64    `json:"monto"`
	Moneda        string     `json:"moneda"`
	Estado        string     `json:"estado"`
	MPPreferencia string     `json:"-"`
	MPPago        string     `json:"mpPago"`
	Creada        time.Time  `json:"creada"`
	Pagada        *time.Time `json:"pagada"`
}

const columnasLicencia = `id, usuario_id::text, producto, monto::float8, moneda, estado, mp_preferencia, mp_pago, creada, pagada`

func escanearLicencia(r pgx.Row) (Licencia, error) {
	var l Licencia
	err := r.Scan(&l.ID, &l.UsuarioID, &l.Producto, &l.Monto, &l.Moneda, &l.Estado, &l.MPPreferencia, &l.MPPago, &l.Creada, &l.Pagada)
	return l, noEncontrado(err)
}

// CrearLicencia guarda una licencia pendiente (alguien está pagando).
func (a *Almacen) CrearLicencia(ctx context.Context, usuarioID, producto string, monto float64, moneda string) (Licencia, error) {
	return escanearLicencia(a.db.QueryRow(ctx, `insert into licencias (usuario_id, producto, monto, moneda)
		values ($1, $2, $3, $4) returning `+columnasLicencia, usuarioID, producto, monto, moneda))
}

// PonerPreferenciaLicencia: el pago de Mercado Pago que la cobra.
func (a *Almacen) PonerPreferenciaLicencia(ctx context.Context, id int64, preferencia string) error {
	_, err := a.db.Exec(ctx, `update licencias set mp_preferencia = $2 where id = $1`, id, preferencia)
	return err
}

// VerLicencia: una por su id.
func (a *Almacen) VerLicencia(ctx context.Context, id int64) (Licencia, error) {
	return escanearLicencia(a.db.QueryRow(ctx, `select `+columnasLicencia+` from licencias where id = $1`, id))
}

// PagarLicencia la marca pagada; true si recién ahora (el aviso de MP llega más de una vez).
func (a *Almacen) PagarLicencia(ctx context.Context, id int64, pago string) (bool, error) {
	t, err := a.db.Exec(ctx, `update licencias set estado = 'pagada', mp_pago = $2, pagada = now()
		where id = $1 and estado in ('pendiente', 'rechazada')`, id, pago)
	return err == nil && t.RowsAffected() == 1, err
}

// RechazarLicencia: el pago no salió.
func (a *Almacen) RechazarLicencia(ctx context.Context, id int64, pago string) error {
	_, err := a.db.Exec(ctx, `update licencias set estado = 'rechazada', mp_pago = $2 where id = $1 and estado = 'pendiente'`, id, pago)
	return err
}

// TieneLicencia: si la cuenta pagó el producto.
func (a *Almacen) TieneLicencia(ctx context.Context, usuarioID, producto string) (bool, error) {
	var si bool
	err := a.db.QueryRow(ctx, `select exists(select 1 from licencias where usuario_id = $1 and producto = $2 and estado = 'pagada')`,
		usuarioID, producto).Scan(&si)
	return si, err
}

// PrimerRobot: cuándo se sumó el primer robot de ese perfil a la cuenta (nil = ninguno).
// La prueba del escritorio corre desde ahí.
func (a *Almacen) PrimerRobot(ctx context.Context, usuarioID, perfil string) (*time.Time, error) {
	var t *time.Time
	err := a.db.QueryRow(ctx, `select min(creado) from bichos where usuario_id = $1 and perfil = $2`, usuarioID, perfil).Scan(&t)
	return t, err
}
