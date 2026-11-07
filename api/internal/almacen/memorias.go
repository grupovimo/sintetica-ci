package almacen

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Memoria es la memoria del motor de vida de un bicho (los bytes del protocolo).
type Memoria struct {
	Datos        []byte     `json:"-"`
	Actualizada  *time.Time `json:"actualizada"`
	Deseada      []byte     `json:"-"`
	DeseadaDesde string     `json:"deseadaDesde,omitempty"`
}

// GuardarMemoria guarda la que mandó el bicho.
func (a *Almacen) GuardarMemoria(ctx context.Context, bichoID string, datos []byte) error {
	_, err := a.db.Exec(ctx, `insert into memorias (bicho_id, datos, actualizada) values ($1, $2, now())
		on conflict (bicho_id) do update set datos = excluded.datos, actualizada = excluded.actualizada`, bichoID, datos)
	return err
}

// LeerMemoria: ErrNoEncontrado si nunca mandó una ni tiene una pendiente.
func (a *Almacen) LeerMemoria(ctx context.Context, bichoID string) (Memoria, error) {
	var m Memoria
	var desde *string
	err := a.db.QueryRow(ctx, `select datos, actualizada, deseada, deseada_desde from memorias where bicho_id = $1`, bichoID).
		Scan(&m.Datos, &m.Actualizada, &m.Deseada, &desde)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNoEncontrado
	}
	if desde != nil {
		m.DeseadaDesde = *desde
	}
	return m, err
}

// Mudar deja la memoria de `desde` como la que `hacia` tiene que recibir.
func (a *Almacen) Mudar(ctx context.Context, desde, hacia string) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		var datos []byte
		err := tx.QueryRow(ctx, `select datos from memorias where bicho_id = $1 and datos is not null`, desde).Scan(&datos)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoEncontrado
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `insert into memorias (bicho_id, deseada, deseada_desde) values ($1, $2, $3)
			on conflict (bicho_id) do update set deseada = excluded.deseada, deseada_desde = excluded.deseada_desde`, hacia, datos, desde)
		return err
	})
}

// MemoriaRecibida: el bicho confirmó la mudanza (y desde ahora esa es la suya).
func (a *Almacen) MemoriaRecibida(ctx context.Context, bichoID string) error {
	_, err := a.db.Exec(ctx, `update memorias set datos = deseada, actualizada = now(), deseada = null, deseada_desde = null
		where bicho_id = $1 and deseada is not null`, bichoID)
	return err
}
