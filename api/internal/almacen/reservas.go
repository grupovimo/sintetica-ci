package almacen

import (
	"context"
	"time"
)

// Reserva es un pedido de la landing.
type Reserva struct {
	ID       int64     `json:"id"`
	Nombre   string    `json:"nombre"`
	Contacto string    `json:"contacto"`
	Color    string    `json:"color"`
	Mensaje  string    `json:"mensaje"`
	Creada   time.Time `json:"creada"`
	Atendida bool      `json:"atendida"`
}

// CrearReserva guarda una reserva y devuelve su id.
func (a *Almacen) CrearReserva(ctx context.Context, r Reserva, ip string) (int64, error) {
	var id int64
	err := a.db.QueryRow(ctx, `insert into reservas (nombre, contacto, color, mensaje, ip) values ($1, $2, $3, $4, $5) returning id`,
		r.Nombre, r.Contacto, r.Color, r.Mensaje, ip).Scan(&id)
	return id, err
}

// ReservasDeIP cuenta las reservas de una IP desde un momento (para el límite).
func (a *Almacen) ReservasDeIP(ctx context.Context, ip string, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from reservas where ip = $1 and creada > $2`, ip, desde).Scan(&n)
	return n, err
}

// Reservas: las últimas, primero las más nuevas.
func (a *Almacen) Reservas(ctx context.Context, limite int) ([]Reserva, error) {
	filas, err := a.db.Query(ctx, `select id, nombre, contacto, color, mensaje, creada, atendida from reservas order by creada desc limit $1`, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Reserva{}
	for filas.Next() {
		var r Reserva
		if err := filas.Scan(&r.ID, &r.Nombre, &r.Contacto, &r.Color, &r.Mensaje, &r.Creada, &r.Atendida); err != nil {
			return nil, err
		}
		l = append(l, r)
	}
	return l, filas.Err()
}

// MarcarReserva marca (o desmarca) una reserva como atendida.
func (a *Almacen) MarcarReserva(ctx context.Context, id int64, atendida bool) error {
	t, err := a.db.Exec(ctx, `update reservas set atendida = $2 where id = $1`, id, atendida)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}
