package almacen

// Las listas (docs/nube.md, "Listas"): la del súper, pendientes o la que sea.
// Son de la cuenta: se anotan desde la web, el escritorio, Telegram, el MCP o
// charlando con el robot, y se ven en todos lados.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Topes de las listas.
const (
	MaxListas      = 30
	MaxItemsLista  = 200
	MaxTextoItem   = 120 // bytes
	MaxNombreLista = 40  // bytes
)

// Errores de las listas (se muestran tal cual).
var (
	ErrTopeListas  = fmt.Errorf("ya tenés %d listas: borrá alguna", MaxListas)
	ErrTopeItems   = fmt.Errorf("la lista ya tiene %d cosas: limpiá las tachadas", MaxItemsLista)
	ErrNombreLista = fmt.Errorf("el nombre de la lista es de 1 a %d bytes", MaxNombreLista)
	ErrTextoItem   = fmt.Errorf("cada cosa es de 1 a %d bytes", MaxTextoItem)
)

// Lista con sus cosas (las de antes primero).
type Lista struct {
	ID     string      `json:"id"`
	Nombre string      `json:"nombre"`
	Creada time.Time   `json:"creada"`
	Items  []ItemLista `json:"items"`
}

// ItemLista es una cosa de una lista.
type ItemLista struct {
	ID      string     `json:"id"`
	Texto   string     `json:"texto"`
	Hecho   bool       `json:"hecho"`
	Creado  time.Time  `json:"creado"`
	HechoEn *time.Time `json:"hechoEn,omitempty"`
}

// Pendientes: las que no están tachadas.
func (l Lista) Pendientes() []ItemLista {
	var p []ItemLista
	for _, it := range l.Items {
		if !it.Hecho {
			p = append(p, it)
		}
	}
	return p
}

// textoLimpio: una línea, sin caracteres de control y sin espacios de más.
func textoLimpio(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// NombreLista valida y limpia el nombre de una lista.
func NombreLista(n string) (string, error) {
	n = textoLimpio(n)
	if n == "" || len(n) > MaxNombreLista || !utf8.ValidString(n) {
		return "", ErrNombreLista
	}
	return n, nil
}

// TextoItem valida y limpia el texto de una cosa.
func TextoItem(t string) (string, error) {
	t = textoLimpio(t)
	if t == "" || len(t) > MaxTextoItem || !utf8.ValidString(t) {
		return "", ErrTextoItem
	}
	return t, nil
}

var patronUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Listas devuelve todas las del usuario (la más vieja primero), con sus cosas.
func (a *Almacen) Listas(ctx context.Context, usuarioID string) ([]Lista, error) {
	filas, err := a.db.Query(ctx, `select id, nombre, creada from listas where usuario_id = $1 order by creada, id`, usuarioID)
	if err != nil {
		return nil, err
	}
	ls, err := pgx.CollectRows(filas, func(f pgx.CollectableRow) (Lista, error) {
		l := Lista{Items: []ItemLista{}}
		return l, f.Scan(&l.ID, &l.Nombre, &l.Creada)
	})
	if err != nil || len(ls) == 0 {
		return ls, err
	}
	filas, err = a.db.Query(ctx, `select i.lista_id, i.id, i.texto, i.hecho, i.creado, i.hecho_en from listas_items i
		join listas l on l.id = i.lista_id where l.usuario_id = $1 order by i.orden, i.creado, i.id`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	pos := make(map[string]int, len(ls))
	for k, l := range ls {
		pos[l.ID] = k
	}
	for filas.Next() {
		var lista string
		var it ItemLista
		if err := filas.Scan(&lista, &it.ID, &it.Texto, &it.Hecho, &it.Creado, &it.HechoEn); err != nil {
			return nil, err
		}
		if k, ok := pos[lista]; ok {
			ls[k].Items = append(ls[k].Items, it)
		}
	}
	return ls, filas.Err()
}

// ListaDe devuelve una lista del usuario (ErrNoEncontrado si no es suya).
func (a *Almacen) ListaDe(ctx context.Context, usuarioID, id string) (Lista, error) {
	if !patronUUID.MatchString(id) {
		return Lista{}, ErrNoEncontrado
	}
	ls, err := a.Listas(ctx, usuarioID)
	if err != nil {
		return Lista{}, err
	}
	for _, l := range ls {
		if l.ID == id {
			return l, nil
		}
	}
	return Lista{}, ErrNoEncontrado
}

// AsegurarLista devuelve la lista con ese nombre (sin mirar mayúsculas) o la
// crea. nueva: si la creó.
func (a *Almacen) AsegurarLista(ctx context.Context, usuarioID, nombre string) (l Lista, nueva bool, err error) {
	nombre, err = NombreLista(nombre)
	if err != nil {
		return Lista{}, false, err
	}
	l.Items = []ItemLista{}
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('listas:' || $1))`, usuarioID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `select id, nombre, creada from listas where usuario_id = $1 and lower(nombre) = lower($2)`,
			usuarioID, nombre).Scan(&l.ID, &l.Nombre, &l.Creada)
		if err == nil || !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `select count(*) from listas where usuario_id = $1`, usuarioID).Scan(&n); err != nil {
			return err
		}
		if n >= MaxListas {
			return ErrTopeListas
		}
		nueva = true
		return tx.QueryRow(ctx, `insert into listas (usuario_id, nombre) values ($1, $2) returning id, nombre, creada`,
			usuarioID, nombre).Scan(&l.ID, &l.Nombre, &l.Creada)
	})
	if err != nil {
		return Lista{}, false, err
	}
	if !nueva {
		l, err = a.ListaDe(ctx, usuarioID, l.ID)
		return l, false, err
	}
	return l, true, nil
}

// BorrarLista la borra con todo lo que tiene.
func (a *Almacen) BorrarLista(ctx context.Context, usuarioID, id string) error {
	if !patronUUID.MatchString(id) {
		return ErrNoEncontrado
	}
	t, err := a.db.Exec(ctx, `delete from listas where id = $1 and usuario_id = $2`, id, usuarioID)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// lockLista: la toma (o ErrNoEncontrado si no es del usuario).
func lockLista(ctx context.Context, tx pgx.Tx, usuarioID, id string) error {
	if !patronUUID.MatchString(id) {
		return ErrNoEncontrado
	}
	var x string
	err := tx.QueryRow(ctx, `select id from listas where id = $1 and usuario_id = $2 for update`, id, usuarioID).Scan(&x)
	return noEncontrado(err)
}

// AgregarItems suma cosas al final de la lista. Lo que ya estaba sin tachar
// no se repite (va en yaEstaban); lo que estaba tachado vuelve a estar
// pendiente (va en agregados). Si no entran todas, no suma ninguna
// (ErrTopeItems).
func (a *Almacen) AgregarItems(ctx context.Context, usuarioID, listaID string, textos []string) (agregados, yaEstaban []ItemLista, err error) {
	limpios := make([]string, 0, len(textos))
	for _, t := range textos {
		t, err := TextoItem(t)
		if err != nil {
			return nil, nil, err
		}
		limpios = append(limpios, t)
	}
	if len(limpios) == 0 {
		return nil, nil, ErrTextoItem
	}
	err = a.enTx(ctx, func(tx pgx.Tx) error {
		if err := lockLista(ctx, tx, usuarioID, listaID); err != nil {
			return err
		}
		filas, err := tx.Query(ctx, `select id, texto, hecho, creado, hecho_en from listas_items where lista_id = $1`, listaID)
		if err != nil {
			return err
		}
		hay, err := pgx.CollectRows(filas, func(f pgx.CollectableRow) (ItemLista, error) {
			var it ItemLista
			return it, f.Scan(&it.ID, &it.Texto, &it.Hecho, &it.Creado, &it.HechoEn)
		})
		if err != nil {
			return err
		}
		var nuevos []string
		var revivir []ItemLista
	afuera:
		for _, t := range limpios {
			for _, it := range hay {
				if strings.EqualFold(it.Texto, t) {
					if it.Hecho {
						revivir = append(revivir, it)
					} else {
						yaEstaban = append(yaEstaban, it)
					}
					continue afuera
				}
			}
			for _, n := range nuevos {
				if strings.EqualFold(n, t) {
					continue afuera
				}
			}
			nuevos = append(nuevos, t)
		}
		if len(hay)+len(nuevos) > MaxItemsLista {
			return ErrTopeItems
		}
		for _, it := range revivir {
			if _, err := tx.Exec(ctx, `update listas_items set hecho = false, hecho_en = null where id = $1`, it.ID); err != nil {
				return err
			}
			it.Hecho, it.HechoEn = false, nil
			agregados = append(agregados, it)
		}
		for _, t := range nuevos {
			var it ItemLista
			if err := tx.QueryRow(ctx, `insert into listas_items (lista_id, texto, orden)
				values ($1, $2, coalesce((select max(orden) + 1 from listas_items where lista_id = $1), 0))
				returning id, texto, hecho, creado`, listaID, t).Scan(&it.ID, &it.Texto, &it.Hecho, &it.Creado); err != nil {
				return err
			}
			agregados = append(agregados, it)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return agregados, yaEstaban, nil
}

// CambiarItem tacha o destacha (hecho) o cambia el texto de una cosa.
func (a *Almacen) CambiarItem(ctx context.Context, usuarioID, listaID, itemID string, hecho *bool, texto *string) (ItemLista, error) {
	var it ItemLista
	if texto != nil {
		t, err := TextoItem(*texto)
		if err != nil {
			return it, err
		}
		texto = &t
	}
	if !patronUUID.MatchString(itemID) {
		return it, ErrNoEncontrado
	}
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if err := lockLista(ctx, tx, usuarioID, listaID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `update listas_items set
			texto = coalesce($3, texto),
			hecho = coalesce($4, hecho),
			hecho_en = case when $4::boolean is null then hecho_en when $4 then coalesce(hecho_en, now()) else null end
			where id = $2 and lista_id = $1
			returning id, texto, hecho, creado, hecho_en`, listaID, itemID, texto, hecho).
			Scan(&it.ID, &it.Texto, &it.Hecho, &it.Creado, &it.HechoEn)
		return noEncontrado(err)
	})
	return it, err
}

// BorrarItem saca una cosa de la lista.
func (a *Almacen) BorrarItem(ctx context.Context, usuarioID, listaID, itemID string) error {
	if !patronUUID.MatchString(itemID) || !patronUUID.MatchString(listaID) {
		return ErrNoEncontrado
	}
	t, err := a.db.Exec(ctx, `delete from listas_items i using listas l
		where i.id = $3 and i.lista_id = $2 and l.id = i.lista_id and l.usuario_id = $1`, usuarioID, listaID, itemID)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// LimpiarLista saca las tachadas (todas, con todas = true). Devuelve cuántas.
func (a *Almacen) LimpiarLista(ctx context.Context, usuarioID, listaID string, todas bool) (int, error) {
	n := 0
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if err := lockLista(ctx, tx, usuarioID, listaID); err != nil {
			return err
		}
		t, err := tx.Exec(ctx, `delete from listas_items where lista_id = $1 and (hecho or $2)`, listaID, todas)
		n = int(t.RowsAffected())
		return err
	})
	return n, err
}

// unirListas pasa las listas de una cuenta a otra al unirlas (unir.go): las
// de nombre repetido se juntan en la que queda.
func unirListas(ctx context.Context, tx pgx.Tx, desde, hacia string) error {
	if _, err := tx.Exec(ctx, `update listas_items i set lista_id = l2.id from listas l1, listas l2
		where i.lista_id = l1.id and l1.usuario_id = $1 and l2.usuario_id = $2 and lower(l2.nombre) = lower(l1.nombre)`, desde, hacia); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from listas l1 where l1.usuario_id = $1 and exists
		(select 1 from listas l2 where l2.usuario_id = $2 and lower(l2.nombre) = lower(l1.nombre))`, desde, hacia); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `update listas set usuario_id = $2 where usuario_id = $1`, desde, hacia)
	return err
}
