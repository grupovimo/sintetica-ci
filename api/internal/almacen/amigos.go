package almacen

// Amigos y toques (v1.9): las invitaciones de un solo uso y las amistades.
// Cada consulta mira que el usuario sea uno de los dos del par: de un amigo
// solo se ve su nombre (nunca su email ni los ids de sus robots).

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Errores de las invitaciones.
var (
	ErrInvitacionInvalida = errors.New("ese código no existe, venció o ya se usó")
	ErrInvitacionPropia   = errors.New("ese código es tuyo: mandáselo a un amigo")
	ErrYaAmigos           = errors.New("ya son amigos")
)

// Amistad es un amigo visto desde un usuario.
type Amistad struct {
	ID        int64     `json:"id"`
	UsuarioID string    `json:"-"` // el amigo
	Nombre    string    `json:"nombre"`
	Robots    []string  `json:"robots"` // los míos que reciben sus toques (nil = todos)
	SusRobots []string  `json:"-"`      // los suyos que reciben los míos (nil = todos)
	Creada    time.Time `json:"creada"`
}

// CrearInvitacion guarda el hash de un código de invitación.
func (a *Almacen) CrearInvitacion(ctx context.Context, usuarioID string, codigoHash []byte, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into amigos_invitaciones (codigo_hash, usuario_id, vence) values ($1, $2, $3)`, codigoHash, usuarioID, vence)
	return err
}

// InvitacionesRecientes cuenta las que creó el usuario desde `desde`.
func (a *Almacen) InvitacionesRecientes(ctx context.Context, usuarioID string, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from amigos_invitaciones where usuario_id = $1 and creado > $2`, usuarioID, desde).Scan(&n)
	return n, err
}

// AceptarInvitacion canjea el código (una vez, antes de que venza) y deja a
// los dos como amigos. Devuelve la amistad vista desde el que acepta.
func (a *Almacen) AceptarInvitacion(ctx context.Context, codigoHash []byte, usuarioID string) (Amistad, error) {
	var id int64
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var invita string
		err := tx.QueryRow(ctx, `select usuario_id from amigos_invitaciones
			where codigo_hash = $1 and usado is null and vence > now() for update`, codigoHash).Scan(&invita)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvitacionInvalida
		}
		if err != nil {
			return err
		}
		if invita == usuarioID {
			return ErrInvitacionPropia
		}
		ua, ub := min(invita, usuarioID), max(invita, usuarioID)
		err = tx.QueryRow(ctx, `insert into amistades (usuario_a, usuario_b) values ($1, $2)
			on conflict (usuario_a, usuario_b) do nothing returning id`, ua, ub).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrYaAmigos
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update amigos_invitaciones set usado = now(), usado_por = $2 where codigo_hash = $1`, codigoHash, usuarioID)
		return err
	})
	if err != nil {
		return Amistad{}, err
	}
	return a.AmistadDe(ctx, id, usuarioID)
}

const consultaAmistad = `select am.id, o.id, o.nombre,
	case when am.usuario_a = $1 then am.robots_a else am.robots_b end,
	case when am.usuario_a = $1 then am.robots_b else am.robots_a end, am.creada
	from amistades am join usuarios o on o.id = case when am.usuario_a = $1 then am.usuario_b else am.usuario_a end
	where (am.usuario_a = $1 or am.usuario_b = $1)`

func escanearAmistad(r pgx.Row) (Amistad, error) {
	var x Amistad
	err := r.Scan(&x.ID, &x.UsuarioID, &x.Nombre, &x.Robots, &x.SusRobots, &x.Creada)
	return x, err
}

// Amigos lista los amigos del usuario (el más viejo primero).
func (a *Almacen) Amigos(ctx context.Context, usuarioID string) ([]Amistad, error) {
	filas, err := a.db.Query(ctx, consultaAmistad+` order by am.id`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Amistad{}
	for filas.Next() {
		x, err := escanearAmistad(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, x)
	}
	return l, filas.Err()
}

// AmistadDe devuelve una amistad del usuario (ErrNoEncontrado si no es suya).
func (a *Almacen) AmistadDe(ctx context.Context, id int64, usuarioID string) (Amistad, error) {
	x, err := escanearAmistad(a.db.QueryRow(ctx, consultaAmistad+` and am.id = $2`, usuarioID, id))
	return x, noEncontrado(err)
}

// AmistadEntre devuelve la amistad entre dos usuarios (vista desde el primero).
func (a *Almacen) AmistadEntre(ctx context.Context, usuarioID, otroID string) (Amistad, error) {
	x, err := escanearAmistad(a.db.QueryRow(ctx, consultaAmistad+` and (am.usuario_a = $2 or am.usuario_b = $2)`, usuarioID, otroID))
	return x, noEncontrado(err)
}

// BorrarAmistad la borra para los dos.
func (a *Almacen) BorrarAmistad(ctx context.Context, id int64, usuarioID string) error {
	t, err := a.db.Exec(ctx, `delete from amistades where id = $1 and (usuario_a = $2 or usuario_b = $2)`, id, usuarioID)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// PonerRobotsAmistad: qué robots del usuario reciben los toques de ese amigo
// (nil = todos). Los ids ya vienen validados (son del usuario).
func (a *Almacen) PonerRobotsAmistad(ctx context.Context, id int64, usuarioID string, robots []string) error {
	t, err := a.db.Exec(ctx, `update amistades set
		robots_a = case when usuario_a = $2 then $3 else robots_a end,
		robots_b = case when usuario_b = $2 then $3 else robots_b end
		where id = $1 and (usuario_a = $2 or usuario_b = $2)`, id, usuarioID, robots)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// NombreDeUsuario: el nombre que eligió (vacío si no puso ninguno).
func (a *Almacen) NombreDeUsuario(ctx context.Context, usuarioID string) (string, error) {
	var n string
	err := a.db.QueryRow(ctx, `select nombre from usuarios where id = $1`, usuarioID).Scan(&n)
	return n, noEncontrado(err)
}

func (a *Almacen) limpiarAmigos(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `delete from amigos_invitaciones where vence < now() - interval '7 days'`)
	return err
}
