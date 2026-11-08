package almacen

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// CuentaElevenLabs: la clave (cifrada) y lo que eligió el usuario.
type CuentaElevenLabs struct {
	UsuarioID      string
	ClaveCifrada   []byte
	Voz, VozNombre string
	Leer           bool
	TopeCaracteres int
	TopeMinutos    int
	Herramientas   map[string]string
}

// GuardarClaveElevenLabs pone (o reemplaza) la clave. Con una clave nueva se
// olvidan las herramientas y los agentes (pueden ser de otra cuenta).
func (a *Almacen) GuardarClaveElevenLabs(ctx context.Context, usuarioID string, cifrada []byte, voz, vozNombre string) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `insert into elevenlabs_cuentas (usuario_id, clave_cifrada, voz, voz_nombre) values ($1, $2, $3, $4)
			on conflict (usuario_id) do update set clave_cifrada = excluded.clave_cifrada, voz = excluded.voz,
			voz_nombre = excluded.voz_nombre, herramientas = '{}', creada = now()`, usuarioID, cifrada, voz, vozNombre); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `delete from elevenlabs_agentes where usuario_id = $1`, usuarioID)
		return err
	})
}

// ElevenLabsDe devuelve la cuenta del usuario (ErrNoEncontrado si no conectó).
func (a *Almacen) ElevenLabsDe(ctx context.Context, usuarioID string) (CuentaElevenLabs, error) {
	var c CuentaElevenLabs
	var h []byte
	err := a.db.QueryRow(ctx, `select usuario_id::text, clave_cifrada, voz, voz_nombre, leer, tope_caracteres, tope_minutos, herramientas
		from elevenlabs_cuentas where usuario_id = $1`, usuarioID).
		Scan(&c.UsuarioID, &c.ClaveCifrada, &c.Voz, &c.VozNombre, &c.Leer, &c.TopeCaracteres, &c.TopeMinutos, &h)
	if err != nil {
		return c, noEncontrado(err)
	}
	c.Herramientas = map[string]string{}
	json.Unmarshal(h, &c.Herramientas) //nolint:errcheck
	return c, nil
}

// AjustesElevenLabs: lo que se puede cambiar sin tocar la clave (nil = igual).
type AjustesElevenLabs struct {
	Voz, VozNombre *string
	Leer           *bool
	TopeCaracteres *int
	TopeMinutos    *int
}

// CambiarElevenLabs cambia la voz, la llave de leer en voz alta o los topes.
func (a *Almacen) CambiarElevenLabs(ctx context.Context, usuarioID string, c AjustesElevenLabs) error {
	t, err := a.db.Exec(ctx, `update elevenlabs_cuentas set voz = coalesce($2, voz), voz_nombre = coalesce($3, voz_nombre),
		leer = coalesce($4, leer), tope_caracteres = coalesce($5, tope_caracteres), tope_minutos = coalesce($6, tope_minutos)
		where usuario_id = $1`, usuarioID, c.Voz, c.VozNombre, c.Leer, c.TopeCaracteres, c.TopeMinutos)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// GuardarHerramientasElevenLabs: las client tools creadas en su cuenta.
func (a *Almacen) GuardarHerramientasElevenLabs(ctx context.Context, usuarioID string, h map[string]string) error {
	b, _ := json.Marshal(h)
	_, err := a.db.Exec(ctx, `update elevenlabs_cuentas set herramientas = $2 where usuario_id = $1`, usuarioID, b)
	return err
}

// BorrarElevenLabs olvida la cuenta (y sus agentes). Devuelve los ids de
// los agentes, para borrarlos también de ElevenLabs.
func (a *Almacen) BorrarElevenLabs(ctx context.Context, usuarioID string) ([]string, error) {
	var ids []string
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		filas, err := tx.Query(ctx, `delete from elevenlabs_agentes where usuario_id = $1 returning agente_id`, usuarioID)
		if err != nil {
			return err
		}
		if ids, err = pgx.CollectRows(filas, pgx.RowTo[string]); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `delete from elevenlabs_cuentas where usuario_id = $1`, usuarioID)
		return err
	})
	return ids, err
}

// UsarCaracteres suma n caracteres de ElevenLabs del día si no pasa el
// tope. Devuelve cuántos lleva (con estos, si se pudo) y si se pudo.
func (a *Almacen) UsarCaracteres(ctx context.Context, usuarioID string, dia time.Time, n, tope int) (int, bool, error) {
	return a.usarCaracteres(ctx, "elevenlabs_uso", usuarioID, dia, n, tope)
}

// CaracteresDeHoy: cuántos leyó ElevenLabs para el usuario ese día.
func (a *Almacen) CaracteresDeHoy(ctx context.Context, usuarioID string, dia time.Time) (int, error) {
	return a.caracteresDeHoy(ctx, "elevenlabs_uso", usuarioID, dia)
}

// UsarCaracteresEdge y CaracteresEdgeDeHoy: lo mismo para la voz gratis.
func (a *Almacen) UsarCaracteresEdge(ctx context.Context, usuarioID string, dia time.Time, n, tope int) (int, bool, error) {
	return a.usarCaracteres(ctx, "voz_edge_uso", usuarioID, dia, n, tope)
}

// CaracteresEdgeDeHoy: cuántos leyó Edge para el usuario ese día.
func (a *Almacen) CaracteresEdgeDeHoy(ctx context.Context, usuarioID string, dia time.Time) (int, error) {
	return a.caracteresDeHoy(ctx, "voz_edge_uso", usuarioID, dia)
}

// usarCaracteres: tabla es una constante de acá (elevenlabs_uso o voz_edge_uso).
func (a *Almacen) usarCaracteres(ctx context.Context, tabla, usuarioID string, dia time.Time, n, tope int) (int, bool, error) {
	var total int
	err := a.db.QueryRow(ctx, `insert into `+tabla+` (usuario_id, dia, caracteres) select $1::uuid, $2::date, $3::int where $3::int <= $4::int
		on conflict (usuario_id, dia) do update set caracteres = `+tabla+`.caracteres + excluded.caracteres
		where `+tabla+`.caracteres + excluded.caracteres <= $4::int
		returning caracteres`, usuarioID, dia.Format("2006-01-02"), n, tope).Scan(&total)
	if noEncontrado(err) == ErrNoEncontrado {
		usados, err := a.caracteresDeHoy(ctx, tabla, usuarioID, dia)
		return usados, false, err
	}
	if err != nil {
		return 0, false, err
	}
	return total, true, nil
}

func (a *Almacen) caracteresDeHoy(ctx context.Context, tabla, usuarioID string, dia time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select coalesce((select caracteres from `+tabla+` where usuario_id = $1 and dia = $2::date), 0)`,
		usuarioID, dia.Format("2006-01-02")).Scan(&n)
	return n, err
}

// VozEdge: lo que eligió el usuario de la voz gratis.
type VozEdge struct {
	Prendida bool
	Voz      string // "" = la de por defecto
}

// VozEdgeDe: sin fila, prendida y con la de por defecto.
func (a *Almacen) VozEdgeDe(ctx context.Context, usuarioID string) (VozEdge, error) {
	v := VozEdge{Prendida: true}
	err := a.db.QueryRow(ctx, `select prendida, voz from voz_edge where usuario_id = $1`, usuarioID).Scan(&v.Prendida, &v.Voz)
	if noEncontrado(err) == ErrNoEncontrado {
		return VozEdge{Prendida: true}, nil
	}
	return v, err
}

// GuardarVozEdge cambia lo que no es nil.
func (a *Almacen) GuardarVozEdge(ctx context.Context, usuarioID string, prendida *bool, voz *string) error {
	_, err := a.db.Exec(ctx, `insert into voz_edge (usuario_id, prendida, voz) values ($1, coalesce($2::boolean, true), coalesce($3::text, ''))
		on conflict (usuario_id) do update set prendida = coalesce($2::boolean, voz_edge.prendida), voz = coalesce($3::text, voz_edge.voz)`,
		usuarioID, prendida, voz)
	return err
}

// AgenteElevenLabs: el agente de un robot en la cuenta del dueño.
type AgenteElevenLabs struct {
	BichoID, AgenteID, Huella string
}

// AgenteDe devuelve el agente del robot para ese usuario.
func (a *Almacen) AgenteDe(ctx context.Context, bichoID, usuarioID string) (AgenteElevenLabs, error) {
	g := AgenteElevenLabs{BichoID: bichoID}
	err := a.db.QueryRow(ctx, `select agente_id, huella from elevenlabs_agentes where bicho_id = $1 and usuario_id = $2`, bichoID, usuarioID).
		Scan(&g.AgenteID, &g.Huella)
	return g, noEncontrado(err)
}

// AgentesDe: todos los agentes del usuario.
func (a *Almacen) AgentesDe(ctx context.Context, usuarioID string) ([]AgenteElevenLabs, error) {
	filas, err := a.db.Query(ctx, `select bicho_id, agente_id, huella from elevenlabs_agentes where usuario_id = $1`, usuarioID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(filas, func(f pgx.CollectableRow) (AgenteElevenLabs, error) {
		var g AgenteElevenLabs
		err := f.Scan(&g.BichoID, &g.AgenteID, &g.Huella)
		return g, err
	})
}

// GuardarAgente pone (o reemplaza) el agente del robot.
func (a *Almacen) GuardarAgente(ctx context.Context, bichoID, usuarioID, agenteID, huella string) error {
	_, err := a.db.Exec(ctx, `insert into elevenlabs_agentes (bicho_id, usuario_id, agente_id, huella) values ($1, $2, $3, $4)
		on conflict (bicho_id, usuario_id) do update set agente_id = excluded.agente_id, huella = excluded.huella, actualizado = now()`,
		bichoID, usuarioID, agenteID, huella)
	return err
}

// BorrarAgente lo olvida (por ejemplo, si lo borraron en ElevenLabs).
func (a *Almacen) BorrarAgente(ctx context.Context, bichoID, usuarioID string) error {
	_, err := a.db.Exec(ctx, `delete from elevenlabs_agentes where bicho_id = $1 and usuario_id = $2`, bichoID, usuarioID)
	return err
}

func (a *Almacen) limpiarElevenLabs(ctx context.Context) error {
	if _, err := a.db.Exec(ctx, `delete from elevenlabs_uso where dia < now() - interval '60 days'`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from voz_edge_uso where dia < now() - interval '60 days'`); err != nil {
		return err
	}
	return a.limpiarPaquetes(ctx)
}
