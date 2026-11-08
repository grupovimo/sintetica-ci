package integraciones

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
)

// Recordatorio es uno de la sección "recordatorios" de la config v2:
// {hora: "08:30", dias: "LMXJV", texto, expresion, sonido, activo}.
type Recordatorio struct {
	Hora      string          `json:"hora"`
	Dias      string          `json:"dias"`
	Texto     string          `json:"texto"`
	Expresion string          `json:"expresion"`
	Sonido    json.RawMessage `json:"sonido"`
	Activo    *bool           `json:"activo"`
}

// recordatoriosDe saca los recordatorios de una config (arriba de todo o
// adentro de "vida"). Una config sin recordatorios da una lista vacía.
func recordatoriosDe(texto string) []Recordatorio {
	var c struct {
		Recordatorios []Recordatorio `json:"recordatorios"`
		Vida          struct {
			Recordatorios []Recordatorio `json:"recordatorios"`
		} `json:"vida"`
	}
	if json.Unmarshal([]byte(texto), &c) != nil {
		return nil
	}
	if len(c.Recordatorios) > 0 {
		return c.Recordatorios
	}
	return c.Vida.Recordatorios
}

// letras de los días: L M X J V S D (lunes a domingo).
var letraDia = map[time.Weekday]byte{
	time.Monday: 'L', time.Tuesday: 'M', time.Wednesday: 'X', time.Thursday: 'J',
	time.Friday: 'V', time.Saturday: 'S', time.Sunday: 'D',
}

// Toca dice si el recordatorio suena en ese minuto (hora local del bicho).
// Sin días, todos los días.
func (r Recordatorio) Toca(t time.Time) bool {
	if r.Activo != nil && !*r.Activo {
		return false
	}
	h, err := time.Parse("15:04", strings.TrimSpace(r.Hora))
	if err != nil || h.Hour() != t.Hour() || h.Minute() != t.Minute() {
		return false
	}
	d := strings.ToUpper(strings.TrimSpace(r.Dias))
	return d == "" || strings.IndexByte(d, letraDia[t.Weekday()]) >= 0
}

// recordatorios: cada minuto, a los usuarios con Telegram les manda los
// recordatorios de sus bichos que no están conectados (el bicho conectado
// lo muestra él; si hay una automatización "recordatorio", esa avisa).
func (i *Integraciones) recordatorios(ctx context.Context) {
	ultimo := time.Now().In(nube.Zona()).Truncate(time.Minute)
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		ahora := time.Now().In(nube.Zona()).Truncate(time.Minute)
		if !ahora.After(ultimo) {
			continue
		}
		// Si se atrasó (la máquina dormía), se ponen al día como mucho 3 minutos.
		desde := ultimo.Add(time.Minute)
		if ahora.Sub(desde) > 3*time.Minute {
			desde = ahora.Add(-2 * time.Minute)
		}
		for m := desde; !m.After(ahora); m = m.Add(time.Minute) {
			i.revisarRecordatorios(ctx, m)
		}
		ultimo = ahora
	}
}

// revisarRecordatorios manda los del minuto m.
func (i *Integraciones) revisarRecordatorios(ctx context.Context, m time.Time) {
	vs, err := i.alm.VinculosTelegram(ctx)
	if err != nil {
		i.log.Warn("recordatorios: no pude leer los vínculos", "error", err)
		return
	}
	for _, v := range vs {
		rs, err := i.robotsDe(ctx, v.UsuarioID)
		if err != nil {
			continue
		}
		for _, r := range rs {
			if r.conectado {
				continue
			}
			cfg, err := i.alm.UltimaConfig(ctx, r.ID)
			if errors.Is(err, almacen.ErrNoEncontrado) || err != nil {
				continue
			}
			for _, rec := range recordatoriosDe(cfg.Texto) {
				if !rec.Toca(m) {
					continue
				}
				texto := "Recordatorio de " + r.nombre + " (" + m.Format("15:04") + "): " + rec.Texto
				err := i.tg.mandar(ctx, v.ChatID, texto, nil)
				e := almacen.Ejecucion{Automatizacion: "recordatorio-nube", Evento: "recordatorio", Accion: "telegram", Ok: err == nil, En: time.Now().UTC()}
				if err != nil {
					e.Detalle = err.Error()
					i.log.Warn("recordatorios: no pude mandar", "bicho", r.ID, "error", err)
				}
				i.alm.AnotarEjecucion(ctx, r.ID, e) //nolint:errcheck
			}
		}
	}
}
