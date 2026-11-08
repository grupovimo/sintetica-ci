package almacen

// Presencia, hábitos y avisos entrantes (v1.9).

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Presencia ---

// Presencia es una llegada, una salida o una dormida.
type Presencia struct {
	Tipo string    // llegaste, te-fuiste o duerme
	En   time.Time // UTC
}

// AnotarPresencia la guarda, salvo que la última de ese robot sea del mismo
// tipo y de hace menos de `junta` (es la misma: la avisaron el celu y el
// robot). nueva = false si no se guardó.
func (a *Almacen) AnotarPresencia(ctx context.Context, bichoID, tipo string, en time.Time, junta time.Duration) (nueva bool, err error) {
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		// Un candado por robot: dos avisos a la vez no se anotan los dos.
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('presencia:' || $1))`, bichoID); err != nil {
			return err
		}
		var ultimo string
		var cuando time.Time
		err := tx.QueryRow(ctx, `select tipo, en from presencias where bicho_id = $1 and tipo in ('llegaste', 'te-fuiste')
			order by en desc limit 1`, bichoID).Scan(&ultimo, &cuando)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if tipo != "duerme" && err == nil && ultimo == tipo && en.Sub(cuando).Abs() < junta {
			return nil
		}
		nueva = true
		_, err = tx.Exec(ctx, `insert into presencias (bicho_id, tipo, en) values ($1, $2, $3)`, bichoID, tipo, en)
		return err
	})
	return nueva, err
}

// Presencias devuelve las de un robot desde `desde` (la más vieja primero).
func (a *Almacen) Presencias(ctx context.Context, bichoID string, desde time.Time) ([]Presencia, error) {
	filas, err := a.db.Query(ctx, `select tipo, en from presencias where bicho_id = $1 and en >= $2 order by en`, bichoID, desde)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Presencia
	for filas.Next() {
		var p Presencia
		if err := filas.Scan(&p.Tipo, &p.En); err != nil {
			return nil, err
		}
		l = append(l, p)
	}
	return l, filas.Err()
}

// --- Hábitos ---

// MarcarHabito anota (hecho) o borra (deshacer) el hábito de ese día.
func (a *Almacen) MarcarHabito(ctx context.Context, bichoID string, indice int, nombre string, dia time.Time, hecho bool) error {
	d := dia.Format("2006-01-02")
	if !hecho {
		_, err := a.db.Exec(ctx, `delete from habitos_hechos where bicho_id = $1 and indice = $2 and dia = $3`, bichoID, indice, d)
		return err
	}
	_, err := a.db.Exec(ctx, `insert into habitos_hechos (bicho_id, indice, dia, nombre) values ($1, $2, $3, $4)
		on conflict (bicho_id, indice, dia) do update set nombre = excluded.nombre, en = now()`, bichoID, indice, d, nombre)
	return err
}

// DiasHabito: los días ("YYYY-MM-DD") desde `desde` en que se hizo el hábito
// de ese lugar con ese nombre.
func (a *Almacen) DiasHabito(ctx context.Context, bichoID string, indice int, nombre string, desde time.Time) (map[string]bool, error) {
	filas, err := a.db.Query(ctx, `select to_char(dia, 'YYYY-MM-DD') from habitos_hechos
		where bicho_id = $1 and indice = $2 and nombre = $3 and dia >= $4`, bichoID, indice, nombre, desde.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	l, err := pgx.CollectRows(filas, pgx.RowTo[string])
	m := map[string]bool{}
	for _, d := range l {
		m[d] = true
	}
	return m, err
}

// --- Avisos entrantes ---

// Entrante es la dirección de avisos entrantes de un usuario.
type Entrante struct {
	UsuarioID string
	Semilla   []byte
	TokenHash []byte
	MPCifrado []byte
	Creado    time.Time
}

// EntranteDe devuelve la del usuario.
func (a *Almacen) EntranteDe(ctx context.Context, usuarioID string) (Entrante, error) {
	var e Entrante
	err := a.db.QueryRow(ctx, `select usuario_id, semilla, token_hash, mp_cifrado, creado from entrantes where usuario_id = $1`, usuarioID).
		Scan(&e.UsuarioID, &e.Semilla, &e.TokenHash, &e.MPCifrado, &e.Creado)
	return e, noEncontrado(err)
}

// EntrantePorToken busca por el hash del token.
func (a *Almacen) EntrantePorToken(ctx context.Context, tokenHash []byte) (Entrante, error) {
	var e Entrante
	err := a.db.QueryRow(ctx, `select usuario_id, semilla, token_hash, mp_cifrado, creado from entrantes where token_hash = $1`, tokenHash).
		Scan(&e.UsuarioID, &e.Semilla, &e.TokenHash, &e.MPCifrado, &e.Creado)
	return e, noEncontrado(err)
}

// GuardarEntrante crea o reemplaza la semilla (el token viejo deja de andar;
// el de Mercado Pago queda).
func (a *Almacen) GuardarEntrante(ctx context.Context, usuarioID string, semilla, tokenHash []byte) error {
	_, err := a.db.Exec(ctx, `insert into entrantes (usuario_id, semilla, token_hash) values ($1, $2, $3)
		on conflict (usuario_id) do update set semilla = excluded.semilla, token_hash = excluded.token_hash, creado = now()`,
		usuarioID, semilla, tokenHash)
	return err
}

// GuardarTokenMP pone (o saca, con nil) el token de Mercado Pago cifrado.
func (a *Almacen) GuardarTokenMP(ctx context.Context, usuarioID string, cifrado []byte) error {
	t, err := a.db.Exec(ctx, `update entrantes set mp_cifrado = $2 where usuario_id = $1`, usuarioID, cifrado)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// limpiarV19: las invitaciones vencidas, las presencias viejas y los hábitos
// de hace más de un año.
func (a *Almacen) limpiarV19(ctx context.Context) error {
	if err := a.limpiarElevenLabs(ctx); err != nil {
		return err
	}
	if err := a.limpiarAmigos(ctx); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from presencias where en < now() - interval '60 days'`); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, `delete from habitos_hechos where dia < now() - interval '400 days'`)
	return err
}
