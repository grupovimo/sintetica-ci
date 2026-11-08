package almacen

import (
	"context"
	"encoding/json"
	"time"
)

// Agenda es el calendario iCal de un usuario (la dirección va cifrada).
type Agenda struct {
	UsuarioID  string
	ICSCifrado []byte
	Pista      string
	Aviso      int
	Proximos   json.RawMessage
	Leida      *time.Time
	Error      string
	Avisados   []string
	ResumenDia *time.Time
}

const columnasAgenda = `usuario_id, ics_cifrado, ics_pista, aviso, proximos, leida, error, avisados, resumen_dia`

func escanearAgenda(f interface{ Scan(...any) error }) (Agenda, error) {
	var a Agenda
	var p []byte
	err := f.Scan(&a.UsuarioID, &a.ICSCifrado, &a.Pista, &a.Aviso, &p, &a.Leida, &a.Error, &a.Avisados, &a.ResumenDia)
	a.Proximos = p
	return a, err
}

// GuardarAgenda pone (o cambia) el calendario del usuario y lo que se leyó.
func (a *Almacen) GuardarAgenda(ctx context.Context, usuarioID string, cifrado []byte, pista string, aviso int, proximos json.RawMessage) error {
	_, err := a.db.Exec(ctx, `insert into agendas (usuario_id, ics_cifrado, ics_pista, aviso, proximos, leida)
		values ($1, $2, $3, $4, $5, now())
		on conflict (usuario_id) do update set ics_cifrado = excluded.ics_cifrado, ics_pista = excluded.ics_pista,
		  aviso = excluded.aviso, proximos = excluded.proximos, leida = now(), error = ''`,
		usuarioID, cifrado, pista, aviso, []byte(proximos))
	return err
}

// CambiarAvisoAgenda cambia cuántos minutos antes avisa.
func (a *Almacen) CambiarAvisoAgenda(ctx context.Context, usuarioID string, aviso int) error {
	t, err := a.db.Exec(ctx, `update agendas set aviso = $2 where usuario_id = $1`, usuarioID, aviso)
	if err == nil && t.RowsAffected() == 0 {
		return ErrNoEncontrado
	}
	return err
}

// AgendaLeida anota lo que se leyó (o el error, con los próximos de antes).
func (a *Almacen) AgendaLeida(ctx context.Context, usuarioID string, proximos json.RawMessage, error_ string) error {
	if proximos == nil {
		_, err := a.db.Exec(ctx, `update agendas set error = $2 where usuario_id = $1`, usuarioID, error_)
		return err
	}
	_, err := a.db.Exec(ctx, `update agendas set proximos = $2, leida = now(), error = '' where usuario_id = $1`, usuarioID, []byte(proximos))
	return err
}

// AgendaDe devuelve la del usuario.
func (a *Almacen) AgendaDe(ctx context.Context, usuarioID string) (Agenda, error) {
	ag, err := escanearAgenda(a.db.QueryRow(ctx, `select `+columnasAgenda+` from agendas where usuario_id = $1`, usuarioID))
	return ag, noEncontrado(err)
}

// Agendas: todas (para leerlas y avisar).
func (a *Almacen) Agendas(ctx context.Context) ([]Agenda, error) {
	filas, err := a.db.Query(ctx, `select `+columnasAgenda+` from agendas`)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var l []Agenda
	for filas.Next() {
		ag, err := escanearAgenda(filas)
		if err != nil {
			return nil, err
		}
		l = append(l, ag)
	}
	return l, filas.Err()
}

// BorrarAgenda la olvida.
func (a *Almacen) BorrarAgenda(ctx context.Context, usuarioID string) error {
	_, err := a.db.Exec(ctx, `delete from agendas where usuario_id = $1`, usuarioID)
	return err
}

// MarcarAvisado suma la clave a los avisados (deja las últimas 100). Devuelve
// false si ya estaba (otro la avisó antes).
func (a *Almacen) MarcarAvisado(ctx context.Context, usuarioID, clave string) (bool, error) {
	t, err := a.db.Exec(ctx, `update agendas set avisados = (array_append(avisados, $2::text))[greatest(1, cardinality(avisados) - 98):]
		where usuario_id = $1 and not ($2 = any(avisados))`, usuarioID, clave)
	if err != nil {
		return false, err
	}
	return t.RowsAffected() == 1, nil
}

// MarcarResumen anota que ya se mandó el resumen de ese día. false si ya estaba.
func (a *Almacen) MarcarResumen(ctx context.Context, usuarioID string, dia time.Time) (bool, error) {
	t, err := a.db.Exec(ctx, `update agendas set resumen_dia = $2::date
		where usuario_id = $1 and (resumen_dia is null or resumen_dia < $2::date)`, usuarioID, dia.Format("2006-01-02"))
	if err != nil {
		return false, err
	}
	return t.RowsAffected() == 1, nil
}
