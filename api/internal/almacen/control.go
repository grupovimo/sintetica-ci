package almacen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// El centro de control del dueño (migraciones/020_control.sql): componentes,
// lista de materiales, compras y unidades. Los montos van en la moneda de cada
// cosa (USD o ARS); las cuentas en las dos se hacen con el dólar del día.

// Componente es una pieza que se compra.
type Componente struct {
	ID          int64     `json:"id"`
	Nombre      string    `json:"nombre"`
	Categoria   string    `json:"categoria"`
	Proveedor   string    `json:"proveedor"`
	Link        string    `json:"link"`
	Moneda      string    `json:"moneda"`
	Costo       float64   `json:"costo"`
	Stock       int       `json:"stock"`
	StockMinimo int       `json:"stockMinimo"`
	Notas       string    `json:"notas"`
	Actualizado time.Time `json:"actualizado"`
}

// Material: cuántos de un componente lleva un modelo.
type Material struct {
	Modelo       string  `json:"modelo"`
	ComponenteID int64   `json:"componenteId"`
	Cantidad     float64 `json:"cantidad"`
}

// Compra es un pedido a un proveedor.
type Compra struct {
	ID            int64   `json:"id"`
	ComponenteID  int64   `json:"componenteId"`
	Cantidad      int     `json:"cantidad"`
	CostoUnitario float64 `json:"costoUnitario"`
	Moneda        string  `json:"moneda"`
	Proveedor     string  `json:"proveedor"`
	Link          string  `json:"link"`
	Pedida        string  `json:"pedida"`   // YYYY-MM-DD
	Recibida      string  `json:"recibida"` // "" = no llegó
	Notas         string  `json:"notas"`
}

// Evento es un renglón del historial de una unidad.
type Evento struct {
	En    time.Time `json:"en"`
	Texto string    `json:"texto"`
}

// Unidad es un robot (o una joya) físico, con su número grabado.
type Unidad struct {
	ID           int64     `json:"id"`
	Serie        string    `json:"serie"`
	Modelo       string    `json:"modelo"`
	Color        string    `json:"color"`
	Estado       string    `json:"estado"`
	BichoID      string    `json:"bichoId"` // "" = sin atar
	CostoUSD     float64   `json:"costoUsd"`
	CostoARS     float64   `json:"costoArs"`
	Dolar        float64   `json:"dolar"`
	Cliente      string    `json:"cliente"`
	Contacto     string    `json:"contacto"`
	Precio       float64   `json:"precio"`
	Moneda       string    `json:"moneda"`
	Canal        string    `json:"canal"`
	Cobrado      bool      `json:"cobrado"`
	VendidaEn    string    `json:"vendidaEn"`
	DesplegadaEn string    `json:"desplegadaEn"`
	Notas        string    `json:"notas"`
	Historial    []Evento  `json:"historial"`
	Creada       time.Time `json:"creada"`
}

// ErrSinStock: no alcanzan los componentes para armar (el texto dice cuáles).
var ErrSinStock = errors.New("no alcanza el stock")

// ErrSerieRepetida: ya hay una unidad con ese número.
var ErrSerieRepetida = errors.New("ese número de serie ya existe")

// ErrBichoOcupado: ese bicho ya está atado a otra unidad.
var ErrBichoOcupado = errors.New("ese robot ya es de otra unidad")

// horaArgentina: el "hoy" de las fechas de venta y despliegue.
var horaArgentina = time.FixedZone("ART", -3*60*60)

func repetido(err error, restriccion string) bool {
	return err != nil && strings.Contains(err.Error(), restriccion)
}

// --- Componentes ---

const columnasComponente = `id, nombre, categoria, proveedor, link, moneda, costo::float8, stock, stock_minimo, notas, actualizado`

func escanearComponente(r pgx.Row) (Componente, error) {
	var c Componente
	err := r.Scan(&c.ID, &c.Nombre, &c.Categoria, &c.Proveedor, &c.Link, &c.Moneda, &c.Costo, &c.Stock, &c.StockMinimo, &c.Notas, &c.Actualizado)
	return c, noEncontrado(err)
}

// Componentes: todos, por categoría y nombre.
func (a *Almacen) Componentes(ctx context.Context) ([]Componente, error) {
	filas, err := a.db.Query(ctx, `select `+columnasComponente+` from componentes order by categoria, nombre, id`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Componente{}
	for filas.Next() {
		c, err := escanearComponente(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, c)
	}
	return l, filas.Err()
}

// GuardarComponente crea (ID 0) o reemplaza un componente.
func (a *Almacen) GuardarComponente(ctx context.Context, c Componente) (Componente, error) {
	if c.ID == 0 {
		return escanearComponente(a.db.QueryRow(ctx, `insert into componentes (nombre, categoria, proveedor, link, moneda, costo, stock, stock_minimo, notas)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning `+columnasComponente,
			c.Nombre, c.Categoria, c.Proveedor, c.Link, c.Moneda, c.Costo, c.Stock, c.StockMinimo, c.Notas))
	}
	return escanearComponente(a.db.QueryRow(ctx, `update componentes set nombre = $2, categoria = $3, proveedor = $4, link = $5, moneda = $6,
		costo = $7, stock = $8, stock_minimo = $9, notas = $10, actualizado = now() where id = $1 returning `+columnasComponente,
		c.ID, c.Nombre, c.Categoria, c.Proveedor, c.Link, c.Moneda, c.Costo, c.Stock, c.StockMinimo, c.Notas))
}

// BorrarComponente lo saca (y de las listas de materiales y sus compras).
func (a *Almacen) BorrarComponente(ctx context.Context, id int64) error {
	t, err := a.db.Exec(ctx, `delete from componentes where id = $1`, id)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// --- Lista de materiales ---

// Materiales: las listas de todos los modelos.
func (a *Almacen) Materiales(ctx context.Context) ([]Material, error) {
	filas, err := a.db.Query(ctx, `select modelo, componente_id, cantidad::float8 from modelo_componentes order by modelo, componente_id`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Material{}
	for filas.Next() {
		var m Material
		if err := filas.Scan(&m.Modelo, &m.ComponenteID, &m.Cantidad); err != nil {
			return nil, err
		}
		l = append(l, m)
	}
	return l, filas.Err()
}

// GuardarMateriales reemplaza la lista de un modelo.
func (a *Almacen) GuardarMateriales(ctx context.Context, modelo string, l []Material) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `delete from modelo_componentes where modelo = $1`, modelo); err != nil {
			return err
		}
		for _, m := range l {
			if _, err := tx.Exec(ctx, `insert into modelo_componentes (modelo, componente_id, cantidad) values ($1, $2, $3)`,
				modelo, m.ComponenteID, m.Cantidad); err != nil {
				if repetido(err, "modelo_componentes_componente_id_fkey") {
					return ErrNoEncontrado
				}
				return err
			}
		}
		return nil
	})
}

// --- Compras ---

const columnasCompra = `id, componente_id, cantidad, costo_unitario::float8, moneda, proveedor, link, to_char(pedida, 'YYYY-MM-DD'),
	coalesce(to_char(recibida, 'YYYY-MM-DD'), ''), notas`

func escanearCompra(r pgx.Row) (Compra, error) {
	var c Compra
	err := r.Scan(&c.ID, &c.ComponenteID, &c.Cantidad, &c.CostoUnitario, &c.Moneda, &c.Proveedor, &c.Link, &c.Pedida, &c.Recibida, &c.Notas)
	return c, noEncontrado(err)
}

// Compras: las últimas, primero las más nuevas.
func (a *Almacen) Compras(ctx context.Context, limite int) ([]Compra, error) {
	filas, err := a.db.Query(ctx, `select `+columnasCompra+` from compras order by pedida desc, id desc limit $1`, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Compra{}
	for filas.Next() {
		c, err := escanearCompra(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, c)
	}
	return l, filas.Err()
}

// CrearCompra anota un pedido (todavía sin llegar).
func (a *Almacen) CrearCompra(ctx context.Context, c Compra) (Compra, error) {
	pedida := any(nil)
	if c.Pedida != "" {
		pedida = c.Pedida
	}
	n, err := escanearCompra(a.db.QueryRow(ctx, `insert into compras (componente_id, cantidad, costo_unitario, moneda, proveedor, link, pedida, notas)
		values ($1, $2, $3, $4, $5, $6, coalesce($7::date, current_date), $8) returning `+columnasCompra,
		c.ComponenteID, c.Cantidad, c.CostoUnitario, c.Moneda, c.Proveedor, c.Link, pedida, c.Notas))
	if repetido(err, "compras_componente_id_fkey") {
		return n, ErrNoEncontrado
	}
	return n, err
}

// RecibirCompra la marca como llegada (suma al stock y deja el último precio
// en el componente) o la desmarca (lo resta). Si ya estaba así, no hace nada.
func (a *Almacen) RecibirCompra(ctx context.Context, id int64, recibida bool) (Compra, error) {
	var c Compra
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var err error
		if c, err = escanearCompra(tx.QueryRow(ctx, `select `+columnasCompra+` from compras where id = $1 for update`, id)); err != nil {
			return err
		}
		if (c.Recibida != "") == recibida {
			return nil
		}
		if recibida {
			if err := tx.QueryRow(ctx, `update compras set recibida = current_date where id = $1 returning to_char(recibida, 'YYYY-MM-DD')`, id).Scan(&c.Recibida); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `update componentes set stock = stock + $2, actualizado = now(),
				costo = case when $3::numeric > 0 then $3::numeric else costo end,
				moneda = case when $3::numeric > 0 then $4 else moneda end where id = $1`,
				c.ComponenteID, c.Cantidad, c.CostoUnitario, c.Moneda)
			return err
		}
		c.Recibida = ""
		if _, err := tx.Exec(ctx, `update compras set recibida = null where id = $1`, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update componentes set stock = stock - $2, actualizado = now() where id = $1`, c.ComponenteID, c.Cantidad)
		return err
	})
	return c, err
}

// BorrarCompra la saca; si había llegado, resta lo que había sumado.
func (a *Almacen) BorrarCompra(ctx context.Context, id int64) error {
	return a.enTx(ctx, func(tx pgx.Tx) error {
		c, err := escanearCompra(tx.QueryRow(ctx, `delete from compras where id = $1 returning `+columnasCompra, id))
		if err != nil {
			return err
		}
		if c.Recibida != "" {
			_, err = tx.Exec(ctx, `update componentes set stock = stock - $2, actualizado = now() where id = $1`, c.ComponenteID, c.Cantidad)
		}
		return err
	})
}

// --- Unidades ---

const columnasUnidad = `id, serie, modelo, color, estado, coalesce(bicho_id, ''), costo_usd::float8, costo_ars::float8, dolar::float8,
	cliente, contacto, precio::float8, moneda, canal, cobrado, coalesce(to_char(vendida_en, 'YYYY-MM-DD'), ''),
	coalesce(to_char(desplegada_en, 'YYYY-MM-DD'), ''), notas, historial, creada`

func escanearUnidad(r pgx.Row) (Unidad, error) {
	var u Unidad
	var historial []byte
	err := r.Scan(&u.ID, &u.Serie, &u.Modelo, &u.Color, &u.Estado, &u.BichoID, &u.CostoUSD, &u.CostoARS, &u.Dolar,
		&u.Cliente, &u.Contacto, &u.Precio, &u.Moneda, &u.Canal, &u.Cobrado, &u.VendidaEn, &u.DesplegadaEn, &u.Notas, &historial, &u.Creada)
	if err != nil {
		return u, noEncontrado(err)
	}
	u.Historial = []Evento{}
	err = json.Unmarshal(historial, &u.Historial)
	return u, err
}

// Unidades: todas, primero las más nuevas.
func (a *Almacen) Unidades(ctx context.Context) ([]Unidad, error) {
	filas, err := a.db.Query(ctx, `select `+columnasUnidad+` from unidades order by creada desc, id desc`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []Unidad{}
	for filas.Next() {
		u, err := escanearUnidad(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, u)
	}
	return l, filas.Err()
}

// consultaCosto: la lista de materiales de un modelo ($1) con el dólar ($2), en
// dólares y en pesos.
const consultaCosto = `select
	coalesce(sum(m.cantidad * case when c.moneda = 'USD' then c.costo else c.costo / nullif($2::numeric, 0) end), 0)::float8,
	coalesce(sum(m.cantidad * case when c.moneda = 'ARS' then c.costo else c.costo * $2::numeric end), 0)::float8
	from modelo_componentes m join componentes c on c.id = m.componente_id where m.modelo = $1`

// CostoModelo: lo que cuesta armar un modelo hoy, en dólares y en pesos.
func (a *Almacen) CostoModelo(ctx context.Context, modelo string, dolar float64) (usd, ars float64, err error) {
	err = a.db.QueryRow(ctx, consultaCosto, modelo, dolar).Scan(&usd, &ars)
	return
}

// ArmarUnidad da de alta una unidad armada: congela el costo con el dólar del
// día y, si descontar, saca sus componentes del stock (si no alcanzan, ErrSinStock
// con los que faltan, salvo forzar).
func (a *Almacen) ArmarUnidad(ctx context.Context, u Unidad, dolar float64, descontar, forzar bool) (Unidad, error) {
	var n Unidad
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		if descontar && !forzar {
			filas, err := tx.Query(ctx, `select c.nombre from modelo_componentes m join componentes c on c.id = m.componente_id
				where m.modelo = $1 and c.stock < ceil(m.cantidad) order by c.nombre`, u.Modelo)
			if err != nil {
				return err
			}
			faltan, err := pgx.CollectRows(filas, pgx.RowTo[string])
			if err != nil {
				return err
			}
			if len(faltan) > 0 {
				return fmt.Errorf("%w: %s", ErrSinStock, strings.Join(faltan, ", "))
			}
		}
		var usd, ars float64
		if err := tx.QueryRow(ctx, consultaCosto, u.Modelo, dolar).Scan(&usd, &ars); err != nil {
			return err
		}
		historial, _ := json.Marshal([]Evento{{En: time.Now().UTC(), Texto: "Armada"}})
		var err error
		n, err = escanearUnidad(tx.QueryRow(ctx, `insert into unidades (serie, modelo, color, notas, costo_usd, costo_ars, dolar, historial)
			values ($1, $2, $3, $4, round($5::numeric, 2), round($6::numeric, 2), $7, $8) returning `+columnasUnidad,
			u.Serie, u.Modelo, u.Color, u.Notas, usd, ars, dolar, historial))
		if repetido(err, "unidades_serie_key") {
			return ErrSerieRepetida
		}
		if err != nil || !descontar {
			return err
		}
		_, err = tx.Exec(ctx, `update componentes c set stock = c.stock - ceil(m.cantidad)::int, actualizado = now()
			from modelo_componentes m where m.componente_id = c.id and m.modelo = $1`, u.Modelo)
		return err
	})
	return n, err
}

// CambioUnidad: lo que se puede tocar de una unidad (nil = no cambia).
type CambioUnidad struct {
	Serie        *string  `json:"serie"`
	Color        *string  `json:"color"`
	Estado       *string  `json:"estado"`
	BichoID      *string  `json:"bichoId"` // "" = desatar
	Cliente      *string  `json:"cliente"`
	Contacto     *string  `json:"contacto"`
	Precio       *float64 `json:"precio"`
	Moneda       *string  `json:"moneda"`
	Canal        *string  `json:"canal"`
	Cobrado      *bool    `json:"cobrado"`
	VendidaEn    *string  `json:"vendidaEn"`
	DesplegadaEn *string  `json:"desplegadaEn"`
	Notas        *string  `json:"notas"`
}

// CambiarUnidad aplica un cambio y lo anota en el historial ("Estado: X → Y",
// "Robot: ..."). Al pasar a vendida o desplegada, si no tenía fecha, le pone hoy.
func (a *Almacen) CambiarUnidad(ctx context.Context, id int64, c CambioUnidad) (Unidad, error) {
	var u Unidad
	err := a.enTx(ctx, func(tx pgx.Tx) error {
		var err error
		if u, err = escanearUnidad(tx.QueryRow(ctx, `select `+columnasUnidad+` from unidades where id = $1 for update`, id)); err != nil {
			return err
		}
		ahora := time.Now().UTC()
		hoy := ahora.In(horaArgentina).Format("2006-01-02")
		anotar := func(t string) { u.Historial = append(u.Historial, Evento{En: ahora, Texto: t}) }
		poner := func(dst *string, src *string) {
			if src != nil {
				*dst = *src
			}
		}
		poner(&u.Serie, c.Serie)
		poner(&u.Color, c.Color)
		poner(&u.Cliente, c.Cliente)
		poner(&u.Contacto, c.Contacto)
		poner(&u.Moneda, c.Moneda)
		poner(&u.Canal, c.Canal)
		poner(&u.VendidaEn, c.VendidaEn)
		poner(&u.DesplegadaEn, c.DesplegadaEn)
		poner(&u.Notas, c.Notas)
		if c.Precio != nil {
			u.Precio = *c.Precio
		}
		if c.Cobrado != nil && *c.Cobrado != u.Cobrado {
			u.Cobrado = *c.Cobrado
			if u.Cobrado {
				anotar("Cobrada")
			} else {
				anotar("Cobro: deshecho")
			}
		}
		if c.Estado != nil && *c.Estado != u.Estado {
			anotar("Estado: " + u.Estado + " → " + *c.Estado)
			u.Estado = *c.Estado
			if u.Estado == "vendida" && u.VendidaEn == "" {
				u.VendidaEn = hoy
			}
			if u.Estado == "desplegada" && u.DesplegadaEn == "" {
				u.DesplegadaEn = hoy
			}
		}
		if c.BichoID != nil && *c.BichoID != u.BichoID {
			if *c.BichoID == "" {
				anotar("Robot: desatado de " + u.BichoID)
			} else {
				anotar("Robot: " + *c.BichoID)
			}
			u.BichoID = *c.BichoID
		}
		historial, _ := json.Marshal(u.Historial)
		fecha := func(s string) any {
			if s == "" {
				return nil
			}
			return s
		}
		bicho := any(nil)
		if u.BichoID != "" {
			bicho = u.BichoID
		}
		u, err = escanearUnidad(tx.QueryRow(ctx, `update unidades set serie = $2, color = $3, estado = $4, bicho_id = $5, cliente = $6,
			contacto = $7, precio = $8, moneda = $9, canal = $10, cobrado = $11, vendida_en = $12::date, desplegada_en = $13::date,
			notas = $14, historial = $15 where id = $1 returning `+columnasUnidad,
			id, u.Serie, u.Color, u.Estado, bicho, u.Cliente, u.Contacto, u.Precio, u.Moneda, u.Canal, u.Cobrado,
			fecha(u.VendidaEn), fecha(u.DesplegadaEn), u.Notas, historial))
		switch {
		case repetido(err, "unidades_serie_key"):
			return ErrSerieRepetida
		case repetido(err, "unidades_bicho_id_key"):
			return ErrBichoOcupado
		case repetido(err, "unidades_bicho_id_fkey"):
			return ErrNoEncontrado
		}
		return err
	})
	return u, err
}

// BorrarUnidad la saca (el stock no vuelve: para eso está "baja").
func (a *Almacen) BorrarUnidad(ctx context.Context, id int64) error {
	t, err := a.db.Exec(ctx, `delete from unidades where id = $1`, id)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// BichoAdmin es un robot de la nube visto desde el control.
type BichoAdmin struct {
	ID             string     `json:"id"`
	Perfil         string     `json:"perfil"`
	Apodo          string     `json:"apodo"`
	Virtual        bool       `json:"virtual"`
	Conectado      bool       `json:"conectado"`
	UltimaConexion *time.Time `json:"ultimaConexion"`
	Dueno          string     `json:"dueno"` // el mail o el nombre; "" = sin reclamar
	Firmware       string     `json:"firmware"`
	Unidad         string     `json:"unidad"` // la serie de su unidad, "" = ninguna
	Creado         time.Time  `json:"creado"`
}

// BichosAdmin: todos los robots, con dueño, firmware (la última OTA
// confirmada) y la unidad a la que están atados.
func (a *Almacen) BichosAdmin(ctx context.Context) ([]BichoAdmin, error) {
	filas, err := a.db.Query(ctx, `select b.id, b.perfil, b.apodo, b.virtual, b.conectado, b.ultima_conexion,
		coalesce(nullif(u.email, ''), u.nombre, ''),
		coalesce((select f.version from actualizaciones x join firmwares f on f.id = x.firmware_id
			where x.bicho_id = b.id and x.estado = 'confirmada' order by x.actualizada desc limit 1), ''),
		coalesce(un.serie, ''), b.creado
		from bichos b left join usuarios u on u.id = b.usuario_id left join unidades un on un.bicho_id = b.id
		order by b.virtual, b.conectado desc, b.ultima_conexion desc nulls last, b.creado desc`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	l := []BichoAdmin{}
	for filas.Next() {
		var b BichoAdmin
		if err := filas.Scan(&b.ID, &b.Perfil, &b.Apodo, &b.Virtual, &b.Conectado, &b.UltimaConexion, &b.Dueno, &b.Firmware, &b.Unidad, &b.Creado); err != nil {
			return nil, err
		}
		l = append(l, b)
	}
	return l, filas.Err()
}
