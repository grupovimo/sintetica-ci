package almacen

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ClienteMCP es una IA que se registró sola (RFC 7591).
type ClienteMCP struct {
	ID            string
	Nombre        string
	Redirecciones []string
	Metodo        string // "none", "client_secret_post" o "client_secret_basic"
	SecretoHash   []byte
	URI           string
	IP            string
	Creado        time.Time
}

// CodigoMCP es un código de autorización (un solo uso).
type CodigoMCP struct {
	Hash        []byte
	UsuarioID   string
	ClienteID   string
	Redireccion string
	Desafio     string
	Alcance     string
	Recurso     string
	Vence       time.Time
}

// TokenMCP es un token de acceso (o personal) vigente.
type TokenMCP struct {
	ID          string
	Tipo        string // "acceso" o "personal"
	UsuarioID   string
	ConcesionID string // "" en los personales
	ClienteID   string
	Cliente     string // el nombre del cliente, o el del token personal
	Alcance     string
	Vence       *time.Time
}

// TokenPersonal es uno de los que el usuario crea desde la web.
type TokenPersonal struct {
	ID     string     `json:"id"`
	Nombre string     `json:"nombre"`
	Creado time.Time  `json:"creado"`
	Usado  *time.Time `json:"usado"`
}

// ConcesionMCP es una IA conectada a la cuenta.
type ConcesionMCP struct {
	ID        string     `json:"id"`
	ClienteID string     `json:"clienteId"`
	Cliente   string     `json:"cliente"`
	URI       string     `json:"uri,omitempty"`
	Alcance   string     `json:"alcance"`
	Creada    time.Time  `json:"creada"`
	Usada     *time.Time `json:"usada"`
}

// ErrCodigoUsado: alguien quiso canjear dos veces el mismo código (se revoca
// lo que se emitió con él).
var ErrCodigoUsado = errors.New("ese código ya se usó")

// ErrRefrescoInvalido: el token de refresco no sirve (vencido, revocado, de
// otro cliente o ya canjeado: en ese caso se revoca toda la concesión).
var ErrRefrescoInvalido = errors.New("el token de refresco no sirve")

// CrearClienteMCP guarda un cliente registrado.
func (a *Almacen) CrearClienteMCP(ctx context.Context, c ClienteMCP) error {
	_, err := a.db.Exec(ctx, `insert into mcp_clientes (id, nombre, redirecciones, metodo, secreto_hash, uri, ip)
		values ($1, $2, $3, $4, $5, $6, $7)`, c.ID, c.Nombre, c.Redirecciones, c.Metodo, c.SecretoHash, c.URI, c.IP)
	return err
}

// ClienteMCPPorID devuelve un cliente registrado.
func (a *Almacen) ClienteMCPPorID(ctx context.Context, id string) (ClienteMCP, error) {
	var c ClienteMCP
	err := a.db.QueryRow(ctx, `select id, nombre, redirecciones, metodo, secreto_hash, uri, ip, creado from mcp_clientes where id = $1`, id).
		Scan(&c.ID, &c.Nombre, &c.Redirecciones, &c.Metodo, &c.SecretoHash, &c.URI, &c.IP, &c.Creado)
	return c, noEncontrado(err)
}

// CrearCodigoMCP guarda un código de autorización (solo su hash).
func (a *Almacen) CrearCodigoMCP(ctx context.Context, c CodigoMCP) error {
	_, err := a.db.Exec(ctx, `insert into mcp_codigos (codigo_hash, usuario_id, cliente_id, redireccion, desafio, alcance, recurso, vence)
		values ($1, $2, $3, $4, $5, $6, $7, $8)`, c.Hash, c.UsuarioID, c.ClienteID, c.Redireccion, c.Desafio, c.Alcance, c.Recurso, c.Vence)
	return err
}

// CanjearCodigoMCP marca el código como usado y lo devuelve. Si ya se había
// usado, revoca la concesión que salió de él (RFC 6749 §4.1.2) y devuelve
// ErrCodigoUsado. Vencido o inexistente: ErrNoEncontrado.
func (a *Almacen) CanjearCodigoMCP(ctx context.Context, hash []byte) (CodigoMCP, error) {
	var c CodigoMCP
	var usado *time.Time
	var concesion *string
	reusado := false
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `select codigo_hash, usuario_id, cliente_id, redireccion, desafio, alcance, recurso, vence, usado, concesion_id
			from mcp_codigos where codigo_hash = $1 for update`, hash).
			Scan(&c.Hash, &c.UsuarioID, &c.ClienteID, &c.Redireccion, &c.Desafio, &c.Alcance, &c.Recurso, &c.Vence, &usado, &concesion)
		if e != nil {
			return noEncontrado(e)
		}
		if usado != nil {
			reusado = true
			if concesion != nil {
				if _, e := tx.Exec(ctx, `update mcp_concesiones set revocada = now() where id = $1 and revocada is null`, *concesion); e != nil {
					return e
				}
			}
			return nil
		}
		if time.Now().After(c.Vence) {
			return ErrNoEncontrado
		}
		_, e = tx.Exec(ctx, `update mcp_codigos set usado = now() where codigo_hash = $1`, hash)
		return e
	})
	if err == nil && reusado {
		return CodigoMCP{}, ErrCodigoUsado
	}
	return c, err
}

// CrearConcesionMCP abre una concesión para el código canjeado y guarda sus
// primeros tokens (de acceso y de refresco). Devuelve el id de la concesión.
func (a *Almacen) CrearConcesionMCP(ctx context.Context, c CodigoMCP, acceso []byte, venceAcceso time.Time, refresco []byte, venceRefresco time.Time) (string, error) {
	var id string
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if e := tx.QueryRow(ctx, `insert into mcp_concesiones (usuario_id, cliente_id, alcance, usada) values ($1, $2, $3, now()) returning id`,
			c.UsuarioID, c.ClienteID, c.Alcance).Scan(&id); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `update mcp_codigos set concesion_id = $2 where codigo_hash = $1`, c.Hash, id); e != nil {
			return e
		}
		return insertarPar(ctx, tx, c.UsuarioID, id, acceso, venceAcceso, refresco, venceRefresco)
	})
	return id, err
}

func insertarPar(ctx context.Context, tx pgx.Tx, usuario, concesion string, acceso []byte, venceAcceso time.Time, refresco []byte, venceRefresco time.Time) error {
	if _, e := tx.Exec(ctx, `insert into mcp_tokens (token_hash, tipo, usuario_id, concesion_id, vence) values ($1, 'acceso', $2, $3, $4)`,
		acceso, usuario, concesion, venceAcceso); e != nil {
		return e
	}
	_, e := tx.Exec(ctx, `insert into mcp_tokens (token_hash, tipo, usuario_id, concesion_id, vence) values ($1, 'refresco', $2, $3, $4)`,
		refresco, usuario, concesion, venceRefresco)
	return e
}

// RefrescarMCP canjea un token de refresco por un par nuevo (el viejo queda
// rotado). Si el viejo ya se había canjeado, alguien lo robó: se revoca toda la
// concesión. Devuelve el alcance de la concesión.
func (a *Almacen) RefrescarMCP(ctx context.Context, viejo []byte, clienteID string, acceso []byte, venceAcceso time.Time, refresco []byte, venceRefresco time.Time) (string, error) {
	var alcance string
	robado := false
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var usuario, concesion, cliente string
		var vence *time.Time
		var rotado, revocado, revocada *time.Time
		e := tx.QueryRow(ctx, `select t.usuario_id, t.concesion_id, t.vence, t.rotado, t.revocado, c.cliente_id, c.revocada, c.alcance
			from mcp_tokens t join mcp_concesiones c on c.id = t.concesion_id
			where t.token_hash = $1 and t.tipo = 'refresco' for update of t`, viejo).
			Scan(&usuario, &concesion, &vence, &rotado, &revocado, &cliente, &revocada, &alcance)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrRefrescoInvalido
		}
		if e != nil {
			return e
		}
		if cliente != clienteID || revocado != nil || revocada != nil || (vence != nil && time.Now().After(*vence)) {
			return ErrRefrescoInvalido
		}
		if rotado != nil {
			robado = true
			_, e := tx.Exec(ctx, `update mcp_concesiones set revocada = now() where id = $1`, concesion)
			return e
		}
		if _, e := tx.Exec(ctx, `update mcp_tokens set rotado = now() where token_hash = $1`, viejo); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `update mcp_concesiones set usada = now() where id = $1`, concesion); e != nil {
			return e
		}
		return insertarPar(ctx, tx, usuario, concesion, acceso, venceAcceso, refresco, venceRefresco)
	})
	if err == nil && robado {
		return "", ErrRefrescoInvalido
	}
	return alcance, err
}

// TokenMCPPorHash devuelve un token de acceso o personal vigente (y anota
// cuándo se usó, como mucho una vez por minuto).
func (a *Almacen) TokenMCPPorHash(ctx context.Context, hash []byte) (TokenMCP, error) {
	var t TokenMCP
	var concesion, cliente, nombreCliente, alcance *string
	var usado *time.Time
	err := a.db.QueryRow(ctx, `select t.id, t.tipo, t.usuario_id, t.concesion_id, c.cliente_id, k.nombre, c.alcance, t.nombre, t.vence, t.usado
		from mcp_tokens t
		left join mcp_concesiones c on c.id = t.concesion_id
		left join mcp_clientes k on k.id = c.cliente_id
		where t.token_hash = $1 and t.tipo in ('acceso', 'personal') and t.revocado is null
		  and (t.vence is null or t.vence > now()) and (t.concesion_id is null or c.revocada is null)`, hash).
		Scan(&t.ID, &t.Tipo, &t.UsuarioID, &concesion, &cliente, &nombreCliente, &alcance, &t.Cliente, &t.Vence, &usado)
	if err != nil {
		return t, noEncontrado(err)
	}
	t.Alcance = "robots"
	if concesion != nil {
		t.ConcesionID, t.ClienteID = *concesion, deref(cliente)
		t.Cliente, t.Alcance = deref(nombreCliente), deref(alcance)
	}
	if usado == nil || time.Since(*usado) > time.Minute {
		if _, err := a.db.Exec(ctx, `update mcp_tokens set usado = now() where token_hash = $1`, hash); err != nil {
			return t, err
		}
		if concesion != nil {
			if _, err := a.db.Exec(ctx, `update mcp_concesiones set usada = now() where id = $1`, *concesion); err != nil {
				return t, err
			}
		}
	}
	return t, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// RevocarTokenMCP revoca un token (RFC 7009). Uno de refresco se lleva toda su
// concesión. Si clienteID no es "", el token tiene que ser de ese cliente. No
// dice si existía.
func (a *Almacen) RevocarTokenMCP(ctx context.Context, hash []byte, clienteID string) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		var tipo string
		var concesion, cliente *string
		e := tx.QueryRow(ctx, `select t.tipo, t.concesion_id, c.cliente_id from mcp_tokens t
			left join mcp_concesiones c on c.id = t.concesion_id where t.token_hash = $1`, hash).Scan(&tipo, &concesion, &cliente)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if tipo == "personal" || (clienteID != "" && deref(cliente) != clienteID) {
			return nil // los personales se borran desde la web
		}
		if tipo == "refresco" && concesion != nil {
			_, e = tx.Exec(ctx, `update mcp_concesiones set revocada = now() where id = $1 and revocada is null`, *concesion)
			return e
		}
		_, e = tx.Exec(ctx, `update mcp_tokens set revocado = now() where token_hash = $1`, hash)
		return e
	})
}

// CrearTokenPersonal guarda un token personal (solo su hash).
func (a *Almacen) CrearTokenPersonal(ctx context.Context, usuarioID, nombre string, hash []byte) (TokenPersonal, error) {
	var t TokenPersonal
	err := a.db.QueryRow(ctx, `insert into mcp_tokens (token_hash, tipo, usuario_id, nombre) values ($1, 'personal', $2, $3)
		returning id, nombre, creado, usado`, hash, usuarioID, nombre).Scan(&t.ID, &t.Nombre, &t.Creado, &t.Usado)
	return t, err
}

// TokensPersonales lista los tokens personales vigentes del usuario.
func (a *Almacen) TokensPersonales(ctx context.Context, usuarioID string) ([]TokenPersonal, error) {
	filas, err := a.db.Query(ctx, `select id, nombre, creado, usado from mcp_tokens
		where usuario_id = $1 and tipo = 'personal' and revocado is null order by creado desc`, usuarioID)
	if err != nil {
		return nil, err
	}
	l := []TokenPersonal{}
	for filas.Next() {
		var t TokenPersonal
		if err := filas.Scan(&t.ID, &t.Nombre, &t.Creado, &t.Usado); err != nil {
			filas.Close()
			return nil, err
		}
		l = append(l, t)
	}
	return l, filas.Err()
}

// CantidadTokensPersonales cuenta los vigentes del usuario.
func (a *Almacen) CantidadTokensPersonales(ctx context.Context, usuarioID string) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `select count(*) from mcp_tokens where usuario_id = $1 and tipo = 'personal' and revocado is null`, usuarioID).Scan(&n)
	return n, err
}

// BorrarTokenPersonal lo borra (ErrNoEncontrado si no es de ese usuario).
func (a *Almacen) BorrarTokenPersonal(ctx context.Context, usuarioID, id string) error {
	tag, err := a.db.Exec(ctx, `delete from mcp_tokens where id::text = $2 and usuario_id = $1 and tipo = 'personal'`, usuarioID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// ConcesionesMCP lista las IAs conectadas del usuario (las vigentes: no
// revocadas y con un refresco que no venció).
func (a *Almacen) ConcesionesMCP(ctx context.Context, usuarioID string) ([]ConcesionMCP, error) {
	filas, err := a.db.Query(ctx, `select c.id, c.cliente_id, k.nombre, k.uri, c.alcance, c.creada, c.usada
		from mcp_concesiones c join mcp_clientes k on k.id = c.cliente_id
		where c.usuario_id = $1 and c.revocada is null
		  and exists (select 1 from mcp_tokens t where t.concesion_id = c.id and t.revocado is null and t.vence > now()
		              and (t.tipo = 'acceso' or t.rotado is null))
		order by c.creada desc`, usuarioID)
	if err != nil {
		return nil, err
	}
	l := []ConcesionMCP{}
	for filas.Next() {
		var c ConcesionMCP
		if err := filas.Scan(&c.ID, &c.ClienteID, &c.Cliente, &c.URI, &c.Alcance, &c.Creada, &c.Usada); err != nil {
			filas.Close()
			return nil, err
		}
		l = append(l, c)
	}
	return l, filas.Err()
}

// RevocarConcesionMCP desconecta una IA (ErrNoEncontrado si no es de ese usuario).
func (a *Almacen) RevocarConcesionMCP(ctx context.Context, usuarioID, id string) error {
	tag, err := a.db.Exec(ctx, `update mcp_concesiones set revocada = now() where id::text = $2 and usuario_id = $1 and revocada is null`, usuarioID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// limpiarMCP borra códigos y tokens vencidos o revocados (y las concesiones
// que quedaron sin nada).
func (a *Almacen) limpiarMCP(ctx context.Context) error {
	if _, err := a.db.Exec(ctx, `delete from mcp_codigos where vence < now() - interval '1 day'`); err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `delete from mcp_tokens where tipo <> 'personal' and (vence < now() - interval '1 day' or revocado < now() - interval '1 day' or rotado < now() - interval '7 days')`); err != nil {
		return err
	}
	_, err := a.db.Exec(ctx, `delete from mcp_concesiones c where (c.revocada < now() - interval '1 day')
		or (c.creada < now() - interval '1 day' and not exists (select 1 from mcp_tokens t where t.concesion_id = c.id))`)
	return err
}
