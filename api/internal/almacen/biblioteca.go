package almacen

import (
	"context"
	"encoding/json"
	"time"
)

// Pieza es una cara o una animación de la biblioteca de un usuario.
type Pieza struct {
	ID          string          `json:"id"`
	Tipo        string          `json:"tipo"`
	Nombre      string          `json:"nombre"`
	Documento   json.RawMessage `json:"documento"`
	Actualizado time.Time       `json:"actualizado"`
}

// Biblioteca lista las piezas de un usuario (tipo "" = todas).
func (a *Almacen) Biblioteca(ctx context.Context, usuarioID, tipo string) ([]Pieza, error) {
	filas, err := a.db.Query(ctx, `select id, tipo, nombre, documento, actualizado from biblioteca
		where usuario_id = $1 and ($2 = '' or tipo = $2) order by tipo, nombre`, usuarioID, tipo)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Pieza
	for filas.Next() {
		var p Pieza
		if err := filas.Scan(&p.ID, &p.Tipo, &p.Nombre, &p.Documento, &p.Actualizado); err != nil {
			return nil, err
		}
		l = append(l, p)
	}
	return l, filas.Err()
}

// GuardarPieza crea o reemplaza (por tipo y nombre) una pieza.
func (a *Almacen) GuardarPieza(ctx context.Context, usuarioID string, p Pieza) (Pieza, error) {
	err := a.db.QueryRow(ctx, `insert into biblioteca (usuario_id, tipo, nombre, documento) values ($1, $2, $3, $4)
		on conflict (usuario_id, tipo, nombre) do update set documento = excluded.documento, actualizado = now()
		returning id, actualizado`, usuarioID, p.Tipo, p.Nombre, p.Documento).Scan(&p.ID, &p.Actualizado)
	return p, err
}

// BorrarPieza borra una pieza del usuario.
func (a *Almacen) BorrarPieza(ctx context.Context, usuarioID, id string) error {
	t, err := a.db.Exec(ctx, `delete from biblioteca where usuario_id = $1 and id::text = $2`, usuarioID, id)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}
