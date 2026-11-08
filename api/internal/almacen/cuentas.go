package almacen

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Usuario es una cuenta (se crea sola la primera vez que alguien entra con su email).
type Usuario struct {
	ID     string    `json:"id"`
	Email  string    `json:"email"`
	Nombre string    `json:"nombre"`
	Admin  bool      `json:"admin"`
	Creado time.Time `json:"creado"`
}

// Sesion es una sesión abierta (web o app).
type Sesion struct {
	Usuario Usuario
	Cliente string
	Vence   time.Time
}

// ErrEnlaceInvalido: el link o el código no sirven (vencido, usado o equivocado).
var ErrEnlaceInvalido = errors.New("el link o el código no sirven: pedí uno nuevo")

// ErrDemasiadosIntentos: se equivocaron muchas veces con el código.
var ErrDemasiadosIntentos = errors.New("demasiados intentos con ese código: pedí un link nuevo")

const maxIntentosCodigo = 5

// CrearEnlace guarda un link mágico (solo los hashes).
func (a *Almacen) CrearEnlace(ctx context.Context, email string, tokenHash, codigoHash []byte, cliente, ip string, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into enlaces (email, token_hash, codigo_hash, cliente, ip, vence) values ($1, $2, $3, $4, $5, $6)`,
		email, tokenHash, codigoHash, cliente, ip, vence)
	return err
}

// EnlacesRecientes cuenta los links pedidos para ese email desde `desde`.
func (a *Almacen) EnlacesRecientes(ctx context.Context, email string, desde time.Time) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from enlaces where email = $1 and creado > $2`, email, desde).Scan(&n)
	return n, err
}

// CanjearToken usa un link: devuelve el email y para qué cliente era.
func (a *Almacen) CanjearToken(ctx context.Context, tokenHash []byte) (email, cliente string, err error) {
	err = a.db.QueryRow(ctx, `update enlaces set usado = now()
		where token_hash = $1 and usado is null and vence > now() and intentos < $2
		returning email, cliente`, tokenHash, maxIntentosCodigo).Scan(&email, &cliente)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrEnlaceInvalido
	}
	return
}

// CanjearCodigo usa el código corto del último link de ese email.
func (a *Almacen) CanjearCodigo(ctx context.Context, email string, codigoHash []byte) (cliente string, err error) {
	equivocado := false
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		var id int64
		var hash []byte
		var intentos int
		e := tx.QueryRow(ctx, `select id, codigo_hash, intentos, cliente from enlaces
			where email = $1 and usado is null and vence > now() order by creado desc limit 1 for update`, email).
			Scan(&id, &hash, &intentos, &cliente)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrEnlaceInvalido
		}
		if e != nil {
			return e
		}
		if intentos >= maxIntentosCodigo {
			return ErrDemasiadosIntentos
		}
		if subtle.ConstantTimeCompare(hash, codigoHash) != 1 {
			// El intento cuenta: se confirma la transacción igual.
			equivocado = true
			_, e := tx.Exec(ctx, `update enlaces set intentos = intentos + 1 where id = $1`, id)
			return e
		}
		_, e = tx.Exec(ctx, `update enlaces set usado = now() where id = $1`, id)
		return e
	})
	if err == nil && equivocado {
		return "", ErrEnlaceInvalido
	}
	return cliente, err
}

// UsuarioPorEmail devuelve la cuenta de ese email, creándola si no existe.
// admin = true la marca como administradora (ADMIN_EMAILS).
func (a *Almacen) UsuarioPorEmail(ctx context.Context, email string, admin bool) (Usuario, error) {
	var u Usuario
	err := a.db.QueryRow(ctx, `insert into usuarios (email, admin) values ($1, $2)
		on conflict (email) do update set admin = usuarios.admin or excluded.admin
		returning id, email, nombre, admin, creado`, email, admin).Scan(&u.ID, &u.Email, &u.Nombre, &u.Admin, &u.Creado)
	return u, err
}

// ActualizarNombre cambia el nombre que se muestra.
func (a *Almacen) ActualizarNombre(ctx context.Context, usuarioID, nombre string) error {
	_, err := a.db.Exec(ctx, `update usuarios set nombre = $2 where id = $1`, usuarioID, nombre)
	return err
}

// CrearSesion guarda una sesión (solo el hash del token).
func (a *Almacen) CrearSesion(ctx context.Context, tokenHash []byte, usuarioID, cliente, descripcion string, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into sesiones (token_hash, usuario_id, cliente, descripcion, vence) values ($1, $2, $3, $4, $5)`,
		tokenHash, usuarioID, cliente, descripcion, vence)
	return err
}

// SesionPorToken busca una sesión vigente y la extiende (`duracion` desde ahora)
// si pasó más de una hora desde el último uso.
func (a *Almacen) SesionPorToken(ctx context.Context, tokenHash []byte, duracion time.Duration) (Sesion, error) {
	var s Sesion
	var usada time.Time
	err := a.db.QueryRow(ctx, `select u.id, u.email, u.nombre, u.admin, u.creado, s.cliente, s.vence, s.usada
		from sesiones s join usuarios u on u.id = s.usuario_id
		where s.token_hash = $1 and s.vence > now()`, tokenHash).
		Scan(&s.Usuario.ID, &s.Usuario.Email, &s.Usuario.Nombre, &s.Usuario.Admin, &s.Usuario.Creado, &s.Cliente, &s.Vence, &usada)
	if err != nil {
		return s, noEncontrado(err)
	}
	if time.Since(usada) > time.Hour {
		s.Vence = time.Now().Add(duracion)
		_, err = a.db.Exec(ctx, `update sesiones set usada = now(), vence = $2 where token_hash = $1`, tokenHash, s.Vence)
	}
	return s, err
}

// BorrarSesion cierra una sesión.
func (a *Almacen) BorrarSesion(ctx context.Context, tokenHash []byte) error {
	_, err := a.db.Exec(ctx, `delete from sesiones where token_hash = $1`, tokenHash)
	return err
}

// Limpiar borra links, sesiones, códigos de Telegram y lo vencido del MCP.
func (a *Almacen) Limpiar(ctx context.Context) error {
	if err := a.limpiarMCP(ctx); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from enlaces where vence < now() - interval '1 day'`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from sesiones where vence < now()`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from telegram_codigos where vence < now() - interval '1 day'`); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, `delete from credenciales where estado = 'pendiente' and vence < now() - interval '1 day'`)
	return err
}
