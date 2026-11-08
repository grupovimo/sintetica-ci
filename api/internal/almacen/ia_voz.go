package almacen

import (
	"context"
	"time"
)

// --- La clave de IA propia (015_ia_propia.sql) ---

// CuentaIA: la clave (cifrada) de IA de un usuario y lo que eligió.
type CuentaIA struct {
	UsuarioID    string
	Proveedor    string // anthropic | compatible
	ClaveCifrada []byte
	URL, Modelo  string
	Tope         int // charlas por día (0 = sin tope)
}

// GuardarIA pone (o reemplaza) la cuenta.
func (a *Almacen) GuardarIA(ctx context.Context, c CuentaIA) error {
	_, err := a.db.Exec(ctx, `insert into ia_cuentas (usuario_id, proveedor, clave_cifrada, url, modelo, tope) values ($1, $2, $3, $4, $5, $6)
		on conflict (usuario_id) do update set proveedor = excluded.proveedor, clave_cifrada = excluded.clave_cifrada,
		url = excluded.url, modelo = excluded.modelo, tope = excluded.tope, creada = now()`,
		c.UsuarioID, c.Proveedor, c.ClaveCifrada, c.URL, c.Modelo, c.Tope)
	return err
}

// IADe devuelve la cuenta (ErrNoEncontrado si no puso clave).
func (a *Almacen) IADe(ctx context.Context, usuarioID string) (CuentaIA, error) {
	var c CuentaIA
	err := a.db.QueryRow(ctx, `select usuario_id::text, proveedor, clave_cifrada, url, modelo, tope from ia_cuentas where usuario_id = $1`, usuarioID).
		Scan(&c.UsuarioID, &c.Proveedor, &c.ClaveCifrada, &c.URL, &c.Modelo, &c.Tope)
	return c, noEncontrado(err)
}

// CambiarIA cambia el modelo o el tope sin tocar la clave (nil = igual).
func (a *Almacen) CambiarIA(ctx context.Context, usuarioID string, modelo *string, tope *int) error {
	t, err := a.db.Exec(ctx, `update ia_cuentas set modelo = coalesce($2, modelo), tope = coalesce($3, tope) where usuario_id = $1`,
		usuarioID, modelo, tope)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// BorrarIA olvida la clave.
func (a *Almacen) BorrarIA(ctx context.Context, usuarioID string) error {
	_, err := a.db.Exec(ctx, `delete from ia_cuentas where usuario_id = $1`, usuarioID)
	return err
}

// --- El paquete de voz (016_voz_paquete.sql) ---

// PaqueteVoz: el paquete de un robot (Frases es el JSON de lo que entró).
type PaqueteVoz struct {
	BichoID, UsuarioID string
	Voz                string
	Version            int
	Huella             string
	Frases             []byte
	Estado, Error      string
	TopeBytes          int
	Actualizado        time.Time
}

// PaqueteVozDe devuelve el paquete del robot (ErrNoEncontrado si nunca se pidió).
func (a *Almacen) PaqueteVozDe(ctx context.Context, bichoID string) (PaqueteVoz, error) {
	var p PaqueteVoz
	err := a.db.QueryRow(ctx, `select bicho_id, usuario_id::text, voz, version, huella, frases, estado, error, tope_bytes, actualizado
		from voz_paquetes where bicho_id = $1`, bichoID).
		Scan(&p.BichoID, &p.UsuarioID, &p.Voz, &p.Version, &p.Huella, &p.Frases, &p.Estado, &p.Error, &p.TopeBytes, &p.Actualizado)
	return p, noEncontrado(err)
}

// GuardarPaqueteVoz pone (o reemplaza) el paquete del robot.
func (a *Almacen) GuardarPaqueteVoz(ctx context.Context, p PaqueteVoz) error {
	if p.Frases == nil {
		p.Frases = []byte("[]")
	}
	_, err := a.db.Exec(ctx, `insert into voz_paquetes (bicho_id, usuario_id, voz, version, huella, frases, estado, error, tope_bytes, actualizado)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		on conflict (bicho_id) do update set usuario_id = excluded.usuario_id, voz = excluded.voz, version = excluded.version,
		huella = excluded.huella, frases = excluded.frases, estado = excluded.estado, error = excluded.error,
		tope_bytes = excluded.tope_bytes, actualizado = now()`,
		p.BichoID, p.UsuarioID, p.Voz, p.Version, p.Huella, p.Frases, p.Estado, p.Error, p.TopeBytes)
	return err
}

// TamanosClips: de esas claves, las que ya tienen audio (clave → bytes).
func (a *Almacen) TamanosClips(ctx context.Context, usuarioID string, claves []string) (map[string]int, error) {
	filas, err := a.db.Query(ctx, `select clave, length(datos) from voz_clips where usuario_id = $1 and clave = any($2)`, usuarioID, claves)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := map[string]int{}
	for filas.Next() {
		var c string
		var n int
		if err := filas.Scan(&c, &n); err != nil {
			return nil, err
		}
		out[c] = n
	}
	return out, filas.Err()
}

// LeerClips: los audios de esas claves (clave → mp3).
func (a *Almacen) LeerClips(ctx context.Context, usuarioID string, claves []string) (map[string][]byte, error) {
	filas, err := a.db.Query(ctx, `select clave, datos from voz_clips where usuario_id = $1 and clave = any($2)`, usuarioID, claves)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := map[string][]byte{}
	for filas.Next() {
		var c string
		var d []byte
		if err := filas.Scan(&c, &d); err != nil {
			return nil, err
		}
		out[c] = d
	}
	return out, filas.Err()
}

// BytesClips: cuánto ocupan los audios del usuario.
func (a *Almacen) BytesClips(ctx context.Context, usuarioID string) (int64, error) {
	var n int64
	err := a.db.QueryRow(ctx, `select coalesce(sum(length(datos)), 0) from voz_clips where usuario_id = $1`, usuarioID).Scan(&n)
	return n, err
}

// GuardarClip guarda un audio si con él el usuario no pasa de tope bytes.
// Devuelve si entró.
func (a *Almacen) GuardarClip(ctx context.Context, usuarioID, clave, voz, texto string, datos []byte, tope int64) (bool, error) {
	t, err := a.db.Exec(ctx, `insert into voz_clips (usuario_id, clave, voz, texto, datos)
		select $1::uuid, $2, $3, $4, $5::bytea
		where (select coalesce(sum(length(datos)), 0) from voz_clips where usuario_id = $1::uuid) + length($5::bytea) <= $6::bigint
		on conflict (usuario_id, clave) do nothing`, usuarioID, clave, voz, texto, datos, tope)
	if err != nil {
		return false, err
	}
	if t.RowsAffected() == 1 {
		return true, nil
	}
	var ya bool
	err = a.db.QueryRow(ctx, `select exists(select 1 from voz_clips where usuario_id = $1 and clave = $2)`, usuarioID, clave).Scan(&ya)
	return ya, err
}

// PodarClips borra los audios del usuario que ningún paquete suyo usa (y
// que tienen más de una hora: los de una generación en curso todavía no
// están en su paquete). Devuelve cuántos borró.
func (a *Almacen) PodarClips(ctx context.Context, usuarioID string) (int64, error) {
	t, err := a.db.Exec(ctx, `delete from voz_clips c where c.usuario_id = $1 and c.creado < now() - interval '1 hour'
		and not exists (select 1 from voz_paquetes p, jsonb_array_elements(p.frases) f
			where p.usuario_id = $1 and f->>'clave' = c.clave)`, usuarioID)
	if err != nil {
		return 0, err
	}
	return t.RowsAffected(), nil
}

// UsarCaracteresPaquete suma lo leído con la voz gratis para un paquete.
func (a *Almacen) UsarCaracteresPaquete(ctx context.Context, usuarioID string, dia time.Time, n, tope int) (int, bool, error) {
	return a.usarCaracteres(ctx, "voz_paquete_uso", usuarioID, dia, n, tope)
}

// CaracteresPaqueteDeHoy: cuánto se leyó hoy para los paquetes.
func (a *Almacen) CaracteresPaqueteDeHoy(ctx context.Context, usuarioID string, dia time.Time) (int, error) {
	return a.caracteresDeHoy(ctx, "voz_paquete_uso", usuarioID, dia)
}

func (a *Almacen) limpiarPaquetes(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `delete from voz_paquete_uso where dia < now() - interval '60 days'`)
	return err
}
