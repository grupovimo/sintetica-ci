package almacen

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// Bicho es un bicho que alguna vez se sumó a una cuenta.
type Bicho struct {
	ID             string     `json:"id"`
	UsuarioID      *string    `json:"-"`
	Perfil         string     `json:"perfil"`
	Apodo          string     `json:"apodo"`
	Virtual        bool       `json:"virtual"`
	Conectado      bool       `json:"conectado"`
	UltimaConexion *time.Time `json:"ultimaConexion"`
	Estado         []byte     `json:"-"`
	EstadoEn       *time.Time `json:"estadoEn"`
	Creado         time.Time  `json:"creado"`
}

const columnasBicho = `id, usuario_id, perfil, apodo, virtual, conectado, ultima_conexion, estado, estado_en, creado`

func escanearBicho(r pgx.Row) (Bicho, error) {
	var b Bicho
	err := r.Scan(&b.ID, &b.UsuarioID, &b.Perfil, &b.Apodo, &b.Virtual, &b.Conectado, &b.UltimaConexion, &b.Estado, &b.EstadoEn, &b.Creado)
	return b, noEncontrado(err)
}

// AsegurarBicho lo crea si no existe (sin dueño).
func (a *Almacen) AsegurarBicho(ctx context.Context, id, perfil string, virtual bool) error {
	_, err := a.db.Exec(ctx, `insert into bichos (id, perfil, virtual) values ($1, $2, $3)
		on conflict (id) do update set perfil = case when excluded.perfil = '' then bichos.perfil else excluded.perfil end,
			virtual = bichos.virtual or excluded.virtual`, id, perfil, virtual)
	return err
}

// Bicho devuelve un bicho, sea de quien sea.
func (a *Almacen) Bicho(ctx context.Context, id string) (Bicho, error) {
	return escanearBicho(a.db.QueryRow(ctx, `select `+columnasBicho+` from bichos where id = $1`, id))
}

// BichoDe devuelve el bicho solo si es de ese usuario.
func (a *Almacen) BichoDe(ctx context.Context, id, usuarioID string) (Bicho, error) {
	return escanearBicho(a.db.QueryRow(ctx, `select `+columnasBicho+` from bichos where id = $1 and usuario_id = $2`, id, usuarioID))
}

// BichosDe lista los bichos de un usuario.
func (a *Almacen) BichosDe(ctx context.Context, usuarioID string) ([]Bicho, error) {
	filas, err := a.db.Query(ctx, `select `+columnasBicho+` from bichos where usuario_id = $1 order by creado`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Bicho
	for filas.Next() {
		b, err := escanearBicho(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, b)
	}
	return l, filas.Err()
}

// BichosConDueno lista los ids de todos los bichos que están en alguna cuenta.
func (a *Almacen) BichosConDueno(ctx context.Context) ([]string, error) {
	filas, err := a.db.Query(ctx, `select id from bichos where usuario_id is not null`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(filas, pgx.RowTo[string])
}

// ActualizarApodo cambia cómo lo llama el dueño en la lista.
func (a *Almacen) ActualizarApodo(ctx context.Context, id, usuarioID, apodo string) error {
	t, err := a.db.Exec(ctx, `update bichos set apodo = $3 where id = $1 and usuario_id = $2`, id, usuarioID, apodo)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// GuardarEstado guarda el último estado que mandó el bicho.
func (a *Almacen) GuardarEstado(ctx context.Context, id string, estado []byte, perfil string) error {
	_, err := a.db.Exec(ctx, `update bichos set estado = $2, estado_en = now(),
		perfil = case when $3 = '' then perfil else $3 end where id = $1`, id, estado, perfil)
	return err
}

// GuardarConexion anota si el bicho está conectado al broker.
func (a *Almacen) GuardarConexion(ctx context.Context, id string, conectado bool) error {
	_, err := a.db.Exec(ctx, `update bichos set conectado = $2,
		ultima_conexion = case when $2 then now() else ultima_conexion end where id = $1`, id, conectado)
	return err
}

// --- Credenciales ---

// CrearCredencialPendiente guarda el hash de una clave nueva para el bicho.
func (a *Almacen) CrearCredencialPendiente(ctx context.Context, bichoID, usuarioID string, claveHash []byte, vence time.Time) error {
	_, err := a.db.Exec(ctx, `insert into credenciales (bicho_id, usuario_id, clave_hash, estado, vence) values ($1, $2, $3, 'pendiente', $4)`,
		bichoID, usuarioID, claveHash, vence)
	return err
}

// Acceso es lo que pasó cuando un bicho se conectó con una clave.
type Acceso struct {
	Ok           bool
	Reclamado    bool   // era una clave pendiente: el bicho cambió de cuenta (o se sumó por primera vez)
	UsuarioID    string // el dueño ahora
	UsuarioAntes string // el de antes ("" si no tenía)
}

// UsarCredencial verifica la clave con la que se conecta un bicho. Si es una
// pendiente, el bicho pasa a ser de quien la pidió: se revocan las otras
// claves y, si cambió de dueño, se borra lo del dueño anterior.
func (a *Almacen) UsarCredencial(ctx context.Context, bichoID string, claveHash []byte) (Acceso, error) {
	var acc Acceso
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var dueno *string
		if err := tx.QueryRow(ctx, `select usuario_id from bichos where id = $1 for update`, bichoID).Scan(&dueno); err != nil {
			return noEncontrado(err)
		}
		filas, err := tx.Query(ctx, `select id, usuario_id, clave_hash, estado from credenciales
			where bicho_id = $1 and (estado = 'activa' or (estado = 'pendiente' and vence > now()))`, bichoID)
		if err != nil {
			return err
		}
		var credID int64
		var credUsuario, credEstado string
		for filas.Next() {
			var id int64
			var u, e string
			var h []byte
			if err := filas.Scan(&id, &u, &h, &e); err != nil {
				filas.Close()
				return err
			}
			if subtle.ConstantTimeCompare(h, claveHash) == 1 {
				credID, credUsuario, credEstado = id, u, e
			}
		}
		filas.Close()
		if credID == 0 {
			return nil
		}
		acc.Ok = true
		acc.UsuarioID = credUsuario
		if dueno != nil {
			acc.UsuarioAntes = *dueno
		}
		if _, err := tx.Exec(ctx, `update credenciales set usada = now() where id = $1`, credID); err != nil {
			return err
		}
		if credEstado != "pendiente" {
			return nil
		}
		acc.Reclamado = true
		if _, err := tx.Exec(ctx, `update credenciales set estado = 'activa', vence = null where id = $1`, credID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update credenciales set estado = 'revocada' where bicho_id = $1 and estado = 'activa' and id <> $2`, bichoID, credID); err != nil {
			return err
		}
		if dueno != nil && *dueno != credUsuario {
			if err := borrarDatos(ctx, tx, bichoID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `update bichos set usuario_id = $2 where id = $1`, bichoID, credUsuario)
		return err
	})
	return acc, err
}

func borrarDatos(ctx context.Context, tx pgx.Tx, bichoID string) error {
	for _, q := range []string{
		`delete from configs where bicho_id = $1`, `delete from imagenes where bicho_id = $1`,
		`delete from sincro where bicho_id = $1`, `delete from actualizaciones where bicho_id = $1`,
		`delete from memorias where bicho_id = $1`,
		`delete from automatizaciones where bicho_id = $1`, `delete from automatizaciones_historial where bicho_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, bichoID); err != nil {
			return err
		}
	}
	return nil
}

// QuitarDeCuenta saca el bicho de la cuenta: revoca sus claves y borra sus datos.
func (a *Almacen) QuitarDeCuenta(ctx context.Context, bichoID, usuarioID string) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		t, err := tx.Exec(ctx, `update bichos set usuario_id = null, apodo = '' where id = $1 and usuario_id = $2`, bichoID, usuarioID)
		if err != nil {
			return err
		}
		if t.RowsAffected() == 0 {
			return ErrNoEncontrado
		}
		if _, err := tx.Exec(ctx, `update credenciales set estado = 'revocada' where bicho_id = $1 and estado <> 'revocada'`, bichoID); err != nil {
			return err
		}
		return borrarDatos(ctx, tx, bichoID)
	})
}

// --- Configs ---

// Config es una versión de la config de un bicho.
type Config struct {
	Version int       `json:"version"`
	Texto   string    `json:"-"`
	Crc     uint32    `json:"crc"`
	Autor   string    `json:"autor"`
	Creada  time.Time `json:"creada"`
}

// UltimaConfig es la que se quiere en el bicho ahora.
func (a *Almacen) UltimaConfig(ctx context.Context, bichoID string) (Config, error) {
	return a.configDonde(ctx, `bicho_id = $1 order by version desc limit 1`, bichoID)
}

// ConfigVersion devuelve una versión vieja.
func (a *Almacen) ConfigVersion(ctx context.Context, bichoID string, version int) (Config, error) {
	return a.configDonde(ctx, `bicho_id = $1 and version = $2`, bichoID, version)
}

func (a *Almacen) configDonde(ctx context.Context, donde string, args ...any) (Config, error) {
	var c Config
	var crc int64
	err := a.db.QueryRow(ctx, `select version, texto, crc, autor, creada from configs where `+donde, args...).
		Scan(&c.Version, &c.Texto, &crc, &c.Autor, &c.Creada)
	c.Crc = uint32(crc)
	return c, noEncontrado(err)
}

// NuevaConfig guarda una versión nueva y devuelve su número. Si es igual a la
// última, no guarda nada y devuelve la última.
func (a *Almacen) NuevaConfig(ctx context.Context, bichoID, texto string, crc uint32, autor string, usuarioID *string) (Config, error) {
	var c Config
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select 1 from bichos where id = $1 for update`, bichoID); err != nil {
			return err
		}
		var ultima int
		var ultimoCrc int64
		var ultimoTexto string
		e := tx.QueryRow(ctx, `select version, crc, texto from configs where bicho_id = $1 order by version desc limit 1`, bichoID).
			Scan(&ultima, &ultimoCrc, &ultimoTexto)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		if e == nil && uint32(ultimoCrc) == crc && ultimoTexto == texto {
			c = Config{Version: ultima, Texto: texto, Crc: crc}
			return tx.QueryRow(ctx, `select autor, creada from configs where bicho_id = $1 and version = $2`, bichoID, ultima).Scan(&c.Autor, &c.Creada)
		}
		c = Config{Version: ultima + 1, Texto: texto, Crc: crc, Autor: autor}
		return tx.QueryRow(ctx, `insert into configs (bicho_id, version, texto, crc, autor, usuario_id) values ($1, $2, $3, $4, $5, $6) returning creada`,
			bichoID, c.Version, texto, int64(crc), autor, usuarioID).Scan(&c.Creada)
	})
	return c, err
}

// HistorialConfig lista las últimas versiones (sin el texto).
func (a *Almacen) HistorialConfig(ctx context.Context, bichoID string, limite int) ([]Config, error) {
	filas, err := a.db.Query(ctx, `select version, crc, autor, creada from configs where bicho_id = $1 order by version desc limit $2`, bichoID, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Config
	for filas.Next() {
		var c Config
		var crc int64
		if err := filas.Scan(&c.Version, &crc, &c.Autor, &c.Creada); err != nil {
			return nil, err
		}
		c.Crc = uint32(crc)
		l = append(l, c)
	}
	return l, filas.Err()
}

// --- Sincronización ---

// Sincro es lo último en lo que el bicho y la API estuvieron de acuerdo.
type Sincro struct {
	ConfigCrc *uint32
	Imagenes  map[int]uint32 // espacio → CRC (sin entrada = vacío)
}

// LeerSincro devuelve el último acuerdo (vacío si nunca hubo).
func (a *Almacen) LeerSincro(ctx context.Context, bichoID string) (Sincro, error) {
	s := Sincro{Imagenes: map[int]uint32{}}
	var crc *int64
	var img []byte
	err := a.db.QueryRow(ctx, `select config_crc, imagenes from sincro where bicho_id = $1`, bichoID).Scan(&crc, &img)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if crc != nil {
		v := uint32(*crc)
		s.ConfigCrc = &v
	}
	var m map[string]uint32
	if json.Unmarshal(img, &m) == nil {
		for k, v := range m {
			if e, err := strconv.Atoi(k); err == nil {
				s.Imagenes[e] = v
			}
		}
	}
	return s, nil
}

// AcordarConfig anota que los dos tienen la config con ese CRC.
func (a *Almacen) AcordarConfig(ctx context.Context, bichoID string, crc uint32) error {
	_, err := a.db.Exec(ctx, `insert into sincro (bicho_id, config_crc) values ($1, $2)
		on conflict (bicho_id) do update set config_crc = excluded.config_crc`, bichoID, int64(crc))
	return err
}

// AcordarImagen anota que los dos tienen esa imagen en ese espacio (crc 0 = vacío).
func (a *Almacen) AcordarImagen(ctx context.Context, bichoID string, espacio int, crc uint32) error {
	clave := strconv.Itoa(espacio)
	var err error
	if crc == 0 {
		_, err = a.db.Exec(ctx, `insert into sincro (bicho_id) values ($1)
			on conflict (bicho_id) do update set imagenes = sincro.imagenes - $2::text`, bichoID, clave)
	} else {
		_, err = a.db.Exec(ctx, `insert into sincro (bicho_id, imagenes) values ($1, jsonb_build_object($2::text, $3::bigint))
			on conflict (bicho_id) do update set imagenes = sincro.imagenes || jsonb_build_object($2::text, $3::bigint)`, bichoID, clave, int64(crc))
	}
	return err
}

// --- Imágenes ---

// Imagen es una imagen guardada (sin los bytes, salvo en LeerImagen).
type Imagen struct {
	Espacio     int       `json:"espacio"`
	Largo       int       `json:"largo"`
	Crc         uint32    `json:"crc"`
	Actualizada time.Time `json:"actualizada"`
	Datos       []byte    `json:"-"`
}

// Imagenes lista las imágenes que se quieren en el bicho.
func (a *Almacen) Imagenes(ctx context.Context, bichoID string) ([]Imagen, error) {
	filas, err := a.db.Query(ctx, `select espacio, length(datos), crc, actualizada from imagenes where bicho_id = $1 order by espacio`, bichoID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Imagen
	for filas.Next() {
		var i Imagen
		var crc int64
		if err := filas.Scan(&i.Espacio, &i.Largo, &crc, &i.Actualizada); err != nil {
			return nil, err
		}
		i.Crc = uint32(crc)
		l = append(l, i)
	}
	return l, filas.Err()
}

// LeerImagen devuelve una imagen con sus bytes.
func (a *Almacen) LeerImagen(ctx context.Context, bichoID string, espacio int) (Imagen, error) {
	var i Imagen
	var crc int64
	err := a.db.QueryRow(ctx, `select espacio, length(datos), crc, actualizada, datos from imagenes where bicho_id = $1 and espacio = $2`, bichoID, espacio).
		Scan(&i.Espacio, &i.Largo, &crc, &i.Actualizada, &i.Datos)
	i.Crc = uint32(crc)
	return i, noEncontrado(err)
}

// GuardarImagen guarda (o reemplaza) una imagen.
func (a *Almacen) GuardarImagen(ctx context.Context, bichoID string, espacio int, datos []byte, crc uint32) error {
	_, err := a.db.Exec(ctx, `insert into imagenes (bicho_id, espacio, datos, crc) values ($1, $2, $3, $4)
		on conflict (bicho_id, espacio) do update set datos = excluded.datos, crc = excluded.crc, actualizada = now()`,
		bichoID, espacio, datos, int64(crc))
	return err
}

// BorrarImagen vacía un espacio.
func (a *Almacen) BorrarImagen(ctx context.Context, bichoID string, espacio int) error {
	_, err := a.db.Exec(ctx, `delete from imagenes where bicho_id = $1 and espacio = $2`, bichoID, espacio)
	return err
}
