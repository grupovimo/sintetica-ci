package almacen

import (
	"context"
	"time"
)

// Firmware es una versión firmada para un perfil.
type Firmware struct {
	ID      int64     `json:"id"`
	Perfil  string    `json:"perfil"`
	Version string    `json:"version"`
	Tamano  int       `json:"tamano"`
	Sha256  []byte    `json:"-"`
	Firma   []byte    `json:"-"`
	Binario []byte    `json:"-"`
	Notas   string    `json:"notas"`
	Creado  time.Time `json:"creado"`
}

// CrearFirmware guarda una versión nueva.
func (a *Almacen) CrearFirmware(ctx context.Context, f Firmware) (Firmware, error) {
	err := a.db.QueryRow(ctx, `insert into firmwares (perfil, version, tamano, sha256, firma, binario, notas)
		values ($1, $2, $3, $4, $5, $6, $7) returning id, creado`,
		f.Perfil, f.Version, f.Tamano, f.Sha256, f.Firma, f.Binario, f.Notas).Scan(&f.ID, &f.Creado)
	return f, err
}

// Firmwares lista las versiones (perfil "" = todas), sin los binarios.
func (a *Almacen) Firmwares(ctx context.Context, perfil string) ([]Firmware, error) {
	filas, err := a.db.Query(ctx, `select id, perfil, version, tamano, sha256, firma, notas, creado from firmwares
		where $1 = '' or perfil = $1 order by creado desc`, perfil)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Firmware
	for filas.Next() {
		var f Firmware
		if err := filas.Scan(&f.ID, &f.Perfil, &f.Version, &f.Tamano, &f.Sha256, &f.Firma, &f.Notas, &f.Creado); err != nil {
			return nil, err
		}
		l = append(l, f)
	}
	return l, filas.Err()
}

// LeerFirmware devuelve una versión (con el binario si conBinario).
func (a *Almacen) LeerFirmware(ctx context.Context, id int64, conBinario bool) (Firmware, error) {
	var f Firmware
	bin := "null::bytea"
	if conBinario {
		bin = "binario"
	}
	err := a.db.QueryRow(ctx, `select id, perfil, version, tamano, sha256, firma, `+bin+`, notas, creado from firmwares where id = $1`, id).
		Scan(&f.ID, &f.Perfil, &f.Version, &f.Tamano, &f.Sha256, &f.Firma, &f.Binario, &f.Notas, &f.Creado)
	return f, noEncontrado(err)
}

// BorrarFirmware borra una versión.
func (a *Almacen) BorrarFirmware(ctx context.Context, id int64) error {
	t, err := a.db.Exec(ctx, `delete from firmwares where id = $1`, id)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// Actualizacion es una OTA mandada a un bicho.
type Actualizacion struct {
	ID          int64     `json:"id"`
	BichoID     string    `json:"bicho"`
	FirmwareID  int64     `json:"firmware"`
	Version     string    `json:"version"`
	Estado      string    `json:"estado"`
	Progreso    int       `json:"progreso"`
	Detalle     string    `json:"detalle"`
	Creada      time.Time `json:"creada"`
	Actualizada time.Time `json:"actualizada"`
}

// CrearActualizacion anota una OTA mandada.
func (a *Almacen) CrearActualizacion(ctx context.Context, bichoID string, firmwareID int64) (Actualizacion, error) {
	u := Actualizacion{BichoID: bichoID, FirmwareID: firmwareID, Estado: "enviada"}
	err := a.db.QueryRow(ctx, `insert into actualizaciones (bicho_id, firmware_id, estado) values ($1, $2, 'enviada')
		returning id, creada, actualizada, (select version from firmwares where id = $2)`, bichoID, firmwareID).
		Scan(&u.ID, &u.Creada, &u.Actualizada, &u.Version)
	return u, err
}

// ActualizarActualizacion cambia el estado de la última OTA de un bicho que no terminó.
func (a *Almacen) ActualizarActualizacion(ctx context.Context, bichoID, estado string, progreso int, detalle string) (Actualizacion, error) {
	var u Actualizacion
	err := a.db.QueryRow(ctx, `update actualizaciones set estado = $2, progreso = $3, detalle = $4, actualizada = now()
		where id = (select id from actualizaciones where bicho_id = $1 and estado in ('enviada', 'descargando', 'instalada')
			order by creada desc limit 1)
		returning id, bicho_id, firmware_id, estado, progreso, detalle, creada, actualizada,
			(select version from firmwares where id = firmware_id)`, bichoID, estado, progreso, detalle).
		Scan(&u.ID, &u.BichoID, &u.FirmwareID, &u.Estado, &u.Progreso, &u.Detalle, &u.Creada, &u.Actualizada, &u.Version)
	return u, noEncontrado(err)
}

// ActualizacionPendiente es la última OTA de un bicho que todavía no terminó.
func (a *Almacen) ActualizacionPendiente(ctx context.Context, bichoID string) (Actualizacion, error) {
	var u Actualizacion
	err := a.db.QueryRow(ctx, `select a.id, a.bicho_id, a.firmware_id, a.estado, a.progreso, a.detalle, a.creada, a.actualizada, f.version
		from actualizaciones a join firmwares f on f.id = a.firmware_id
		where a.bicho_id = $1 and a.estado in ('enviada', 'descargando', 'instalada') order by a.creada desc limit 1`, bichoID).
		Scan(&u.ID, &u.BichoID, &u.FirmwareID, &u.Estado, &u.Progreso, &u.Detalle, &u.Creada, &u.Actualizada, &u.Version)
	return u, noEncontrado(err)
}

// Actualizaciones lista las OTA de un bicho.
func (a *Almacen) Actualizaciones(ctx context.Context, bichoID string) ([]Actualizacion, error) {
	filas, err := a.db.Query(ctx, `select a.id, a.bicho_id, a.firmware_id, a.estado, a.progreso, a.detalle, a.creada, a.actualizada, f.version
		from actualizaciones a join firmwares f on f.id = a.firmware_id where a.bicho_id = $1 order by a.creada desc limit 20`, bichoID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Actualizacion
	for filas.Next() {
		var u Actualizacion
		if err := filas.Scan(&u.ID, &u.BichoID, &u.FirmwareID, &u.Estado, &u.Progreso, &u.Detalle, &u.Creada, &u.Actualizada, &u.Version); err != nil {
			return nil, err
		}
		l = append(l, u)
	}
	return l, filas.Err()
}
