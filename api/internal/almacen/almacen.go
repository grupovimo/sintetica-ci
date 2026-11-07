// Package almacen guarda todo en Postgres: cuentas, sesiones, bichos y sus
// credenciales, configs (con historial), imágenes, la biblioteca de caras y
// animaciones, y los firmwares con sus actualizaciones.
package almacen

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migraciones/*.sql
var migraciones embed.FS

// ErrNoEncontrado es una fila que no está (o que no es de ese usuario).
var ErrNoEncontrado = errors.New("no encontrado")

// Almacen es la base de datos.
type Almacen struct {
	db *pgxpool.Pool
}

// Abrir se conecta (reintenta un rato: en Docker, Postgres puede tardar en
// arrancar) y aplica las migraciones que falten.
func Abrir(ctx context.Context, url string) (*Almacen, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 20
	var db *pgxpool.Pool
	for intento := 0; ; intento++ {
		db, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = db.Ping(ctx); err == nil {
				break
			}
			db.Close()
		}
		if intento >= 30 {
			return nil, fmt.Errorf("no me pude conectar a Postgres: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	a := &Almacen{db: db}
	if err := a.migrar(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

// Cerrar libera las conexiones.
func (a *Almacen) Cerrar() { a.db.Close() }

// Ping dice si la base responde.
func (a *Almacen) Ping(ctx context.Context) error { return a.db.Ping(ctx) }

func (a *Almacen) migrar(ctx context.Context) error {
	conn, err := a.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// Un solo proceso migra a la vez.
	if _, err := conn.Exec(ctx, "select pg_advisory_lock(5157)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "select pg_advisory_unlock(5157)") //nolint:errcheck
	if _, err := conn.Exec(ctx, `create table if not exists migraciones (nombre text primary key, aplicada timestamptz not null default now())`); err != nil {
		return err
	}
	nombres, _ := fs.Glob(migraciones, "migraciones/*.sql")
	sort.Strings(nombres)
	for _, n := range nombres {
		base := strings.TrimPrefix(n, "migraciones/")
		var ya bool
		if err := conn.QueryRow(ctx, "select exists(select 1 from migraciones where nombre = $1)", base).Scan(&ya); err != nil {
			return err
		}
		if ya {
			continue
		}
		sqlTexto, _ := migraciones.ReadFile(n)
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlTexto)); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return fmt.Errorf("migración %s: %w", base, err)
		}
		if _, err := tx.Exec(ctx, "insert into migraciones (nombre) values ($1)", base); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func noEncontrado(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoEncontrado
	}
	return err
}

// enTx corre f en una transacción.
func (a *Almacen) enTx(ctx context.Context, f func(tx pgx.Tx) error) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := f(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
