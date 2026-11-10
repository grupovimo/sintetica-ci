package almacen

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// La preventa del cubito (migraciones/020_control.sql): los números que pone
// el dueño y las señas que entran por Mercado Pago.

// Preventa: precio, seña y cupos de un modelo, en pesos.
type Preventa struct {
	Modelo      string    `json:"modelo"`
	Activa      bool      `json:"activa"`
	Precio      float64   `json:"precio"`
	PrecioFinal float64   `json:"precioFinal"`
	Sena        float64   `json:"sena"`
	Cupos       int       `json:"cupos"`
	Entrega     string    `json:"entrega"`
	Actualizado time.Time `json:"actualizado"`
}

// Sena es una seña de la preventa.
type Sena struct {
	ID            int64      `json:"id"`
	Modelo        string     `json:"modelo"`
	Nombre        string     `json:"nombre"`
	Contacto      string     `json:"contacto"`
	Email         string     `json:"email"`
	Color         string     `json:"color"`
	Monto         float64    `json:"monto"`
	Estado        string     `json:"estado"`
	MPPreferencia string     `json:"-"`
	MPPago        string     `json:"mpPago"`
	UnidadID      *int64     `json:"unidadId"`
	Creada        time.Time  `json:"creada"`
	Pagada        *time.Time `json:"pagada"`
}

// ErrSinCupos: la preventa se llenó.
var ErrSinCupos = errors.New("no quedan cupos")

// esperaSena: lo que una seña pendiente le guarda el cupo a quien está pagando.
const esperaSena = 30 * time.Minute

// VerPreventa: la de un modelo (si no hay fila, una apagada).
func (a *Almacen) VerPreventa(ctx context.Context, modelo string) (Preventa, error) {
	p := Preventa{Modelo: modelo}
	err := a.db.QueryRow(ctx, `select activa, precio_ars::float8, precio_final_ars::float8, sena_ars::float8, cupos, entrega, actualizado
		from preventa where modelo = $1`, modelo).Scan(&p.Activa, &p.Precio, &p.PrecioFinal, &p.Sena, &p.Cupos, &p.Entrega, &p.Actualizado)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	return p, err
}

// GuardarPreventa reemplaza los números de la preventa de un modelo.
func (a *Almacen) GuardarPreventa(ctx context.Context, p Preventa) (Preventa, error) {
	_, err := a.db.Exec(ctx, `insert into preventa (modelo, activa, precio_ars, precio_final_ars, sena_ars, cupos, entrega)
		values ($1, $2, $3, $4, $5, $6, $7) on conflict (modelo) do update set activa = $2, precio_ars = $3, precio_final_ars = $4,
		sena_ars = $5, cupos = $6, entrega = $7, actualizado = now()`, p.Modelo, p.Activa, p.Precio, p.PrecioFinal, p.Sena, p.Cupos, p.Entrega)
	if err != nil {
		return p, err
	}
	return a.VerPreventa(ctx, p.Modelo)
}

// CuposTomados: las señas aprobadas más las pendientes recientes (alguien está pagando).
func (a *Almacen) CuposTomados(ctx context.Context, modelo string) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from senas where modelo = $1 and
		(estado = 'aprobada' or (estado = 'pendiente' and creada > now() - make_interval(secs => $2)))`, modelo, esperaSena.Seconds()).Scan(&n)
	return n, err
}

const columnasSena = `id, modelo, nombre, contacto, email, color, monto_ars::float8, estado, mp_preferencia, mp_pago, unidad_id, creada, pagada`

func escanearSena(r pgx.Row) (Sena, error) {
	var s Sena
	err := r.Scan(&s.ID, &s.Modelo, &s.Nombre, &s.Contacto, &s.Email, &s.Color, &s.Monto, &s.Estado, &s.MPPreferencia, &s.MPPago, &s.UnidadID, &s.Creada, &s.Pagada)
	return s, noEncontrado(err)
}

// CrearSena guarda una seña pendiente si queda cupo (con un candado, para que
// dos que pagan a la vez no se lleven el último).
func (a *Almacen) CrearSena(ctx context.Context, s Sena, ip string) (Sena, error) {
	var n Sena
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var cupos int
		if err := tx.QueryRow(ctx, `select cupos from preventa where modelo = $1 for update`, s.Modelo).Scan(&cupos); err != nil {
			return noEncontrado(err)
		}
		var tomados int
		if err := tx.QueryRow(ctx, `select count(*) from senas where modelo = $1 and
			(estado = 'aprobada' or (estado = 'pendiente' and creada > now() - make_interval(secs => $2)))`, s.Modelo, esperaSena.Seconds()).Scan(&tomados); err != nil {
			return err
		}
		if tomados >= cupos {
			return ErrSinCupos
		}
		var err error
		n, err = escanearSena(tx.QueryRow(ctx, `insert into senas (modelo, nombre, contacto, email, color, monto_ars, ip)
			values ($1, $2, $3, $4, $5, $6, $7) returning `+columnasSena, s.Modelo, s.Nombre, s.Contacto, s.Email, s.Color, s.Monto, ip))
		return err
	})
	return n, err
}

// SenasDeIP cuenta las señas de una IP desde un momento (para el límite).
func (a *Almacen) SenasDeIP(ctx context.Context, ip string, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from senas where ip = $1 and creada > $2`, ip, desde).Scan(&n)
	return n, err
}

// PonerPreferencia le anota a la seña la preferencia de Mercado Pago.
func (a *Almacen) PonerPreferencia(ctx context.Context, id int64, preferencia string) error {
	_, err := a.db.Exec(ctx, `update senas set mp_preferencia = $2 where id = $1`, id, preferencia)
	return err
}

// VerSena: una por id.
func (a *Almacen) VerSena(ctx context.Context, id int64) (Sena, error) {
	return escanearSena(a.db.QueryRow(ctx, `select `+columnasSena+` from senas where id = $1`, id))
}

// Senas: las últimas, primero las más nuevas.
func (a *Almacen) Senas(ctx context.Context, limite int) ([]Sena, error) {
	filas, err := a.db.Query(ctx, `select `+columnasSena+` from senas order by creada desc, id desc limit $1`, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Sena{}
	for filas.Next() {
		s, err := escanearSena(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, s)
	}
	return l, filas.Err()
}

// PagarSena la marca aprobada con el pago de Mercado Pago. Devuelve true si
// recién ahora pasó (para avisar una sola vez).
func (a *Almacen) PagarSena(ctx context.Context, id int64, pago string) (bool, error) {
	t, err := a.db.Exec(ctx, `update senas set estado = 'aprobada', mp_pago = $2, pagada = now()
		where id = $1 and estado in ('pendiente', 'rechazada')`, id, pago)
	return err == nil && t.RowsAffected() == 1, err
}

// RechazarSena: el pago no salió (solo si seguía pendiente).
func (a *Almacen) RechazarSena(ctx context.Context, id int64, pago string) error {
	_, err := a.db.Exec(ctx, `update senas set estado = 'rechazada', mp_pago = $2 where id = $1 and estado = 'pendiente'`, id, pago)
	return err
}

// CambiarSena: el dueño la marca devuelta (o la vuelve a aprobada) o la ata a
// una unidad (0 = desatar).
func (a *Almacen) CambiarSena(ctx context.Context, id int64, estado *string, unidad *int64) (Sena, error) {
	var u any
	if unidad != nil && *unidad != 0 {
		u = *unidad
	}
	s, err := escanearSena(a.db.QueryRow(ctx, `update senas set estado = coalesce($2, estado),
		unidad_id = case when $3 then $4::bigint else unidad_id end where id = $1 returning `+columnasSena,
		id, estado, unidad != nil, u))
	if repetido(err, "senas_unidad_id_fkey") {
		return s, ErrNoEncontrado
	}
	return s, err
}
