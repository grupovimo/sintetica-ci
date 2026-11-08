package almacen

// La charla sin IA en la nube: los gustos de cada robot y lo que la charla
// puso en la config para una sola vez (018_charla_motor.sql).

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// MaxGustos: los que se guardan por robot (los más nuevos).
const MaxGustos = 8

func claveGusto(g string) string { return strings.ToLower(strings.Join(strings.Fields(g), " ")) }

// SumarGusto lo anota (si ya estaba, lo renueva) y deja los MaxGustos más nuevos.
func (a *Almacen) SumarGusto(ctx context.Context, bichoID, gusto string) error {
	gusto = strings.Join(strings.Fields(gusto), " ")
	if gusto == "" {
		return nil
	}
	if _, err := a.db.Exec(ctx, `insert into charla_gustos (bicho_id, gusto, clave) values ($1, $2, $3)
		on conflict (bicho_id, clave) do update set gusto = excluded.gusto, en = now()`, bichoID, gusto, claveGusto(gusto)); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, `delete from charla_gustos where bicho_id = $1 and clave not in
		(select clave from charla_gustos where bicho_id = $1 order by en desc, clave limit $2)`, bichoID, MaxGustos)
	return err
}

// SacarGusto lo olvida (no está: nada).
func (a *Almacen) SacarGusto(ctx context.Context, bichoID, gusto string) error {
	_, err := a.db.Exec(ctx, `delete from charla_gustos where bicho_id = $1 and clave = $2`, bichoID, claveGusto(gusto))
	return err
}

// Gustos: los de ese robot, del más nuevo al más viejo.
func (a *Almacen) Gustos(ctx context.Context, bichoID string) ([]string, error) {
	filas, err := a.db.Query(ctx, `select gusto from charla_gustos where bicho_id = $1 order by en desc, clave`, bichoID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []string{}
	for filas.Next() {
		var g string
		if err := filas.Scan(&g); err != nil {
			return nil, err
		}
		l = append(l, g)
	}
	return l, filas.Err()
}

// UnaVez: algo que la charla puso en la config para una sola vez.
type UnaVez struct {
	ID     int64
	Bicho  string
	Tipo   string // alarma | recordatorio
	Vence  time.Time
	Puesto json.RawMessage
	Antes  json.RawMessage // nil: no había
}

// AnotarUnaVez lo guarda para sacarlo cuando venza.
func (a *Almacen) AnotarUnaVez(ctx context.Context, u UnaVez) error {
	var antes any
	if len(u.Antes) > 0 {
		antes = u.Antes
	}
	_, err := a.db.Exec(ctx, `insert into charla_una_vez (bicho_id, tipo, vence, puesto, antes) values ($1, $2, $3, $4, $5)`,
		u.Bicho, u.Tipo, u.Vence, u.Puesto, antes)
	return err
}

// UnasVez: las de ese robot y tipo ("" = todas) que vencen antes de `hasta`
// (bicho "" = de todos los robots), de la más vieja a la más nueva.
func (a *Almacen) UnasVez(ctx context.Context, bichoID, tipo string, hasta time.Time) ([]UnaVez, error) {
	filas, err := a.db.Query(ctx, `select id, bicho_id, tipo, vence, puesto, antes from charla_una_vez
		where ($1 = '' or bicho_id = $1) and ($2 = '' or tipo = $2) and vence < $3 order by id limit 200`, bichoID, tipo, hasta)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []UnaVez
	for filas.Next() {
		var u UnaVez
		if err := filas.Scan(&u.ID, &u.Bicho, &u.Tipo, &u.Vence, &u.Puesto, &u.Antes); err != nil {
			return nil, err
		}
		l = append(l, u)
	}
	return l, filas.Err()
}

// BorrarUnaVez: ya se sacó (o no hace falta).
func (a *Almacen) BorrarUnaVez(ctx context.Context, id int64) error {
	_, err := a.db.Exec(ctx, `delete from charla_una_vez where id = $1`, id)
	return err
}
