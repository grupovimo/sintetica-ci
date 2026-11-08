package integraciones

// Hábitos (v1.9, docs/nube.md, "Hábitos"): los de `habitos` de la config
// (hasta 4), marcados desde la consola, la app, Telegram (/habito), la IA o
// el robot mismo (registro tipo 8). La nube anota los días, calcula la racha
// (los días que no tocan, según `dias`, no la cortan) y le manda al robot el
// 0x4A con la racha.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// DiasCalendario: los días que devuelve GET …/habitos (el calendario de puntitos).
const DiasCalendario = 60

// ErrSinHabito: no hay un hábito en ese lugar.
var ErrSinHabito = errors.New("no hay un hábito con ese número")

// HabitoConfig es uno de `habitos` de la config.
type HabitoConfig struct {
	Nombre string `json:"nombre"`
	Icono  string `json:"icono"`
	Hora   string `json:"hora,omitempty"`
	Dias   string `json:"dias,omitempty"`
}

// Habito: uno con su racha y los días hechos (los últimos 60).
type Habito struct {
	Indice int             `json:"indice"`
	Nombre string          `json:"nombre"`
	Icono  string          `json:"icono"`
	Hora   string          `json:"hora,omitempty"`
	Racha  int             `json:"racha"`
	Hoy    bool            `json:"hoy"`
	Dias   map[string]bool `json:"dias"`
}

func habitosDe(cfg string) []HabitoConfig {
	var c struct {
		Habitos []HabitoConfig `json:"habitos"`
	}
	json.Unmarshal([]byte(cfg), &c) //nolint:errcheck
	if len(c.Habitos) > protocolo.MaxHabitos {
		c.Habitos = c.Habitos[:protocolo.MaxHabitos]
	}
	return c.Habitos
}

func (i *Integraciones) habitosConfig(ctx context.Context, bicho string) ([]HabitoConfig, error) {
	c, err := i.alm.UltimaConfig(ctx, bicho)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return habitosDe(c.Texto), nil
}

func tocaHoy(dias string, t time.Time) bool {
	return dias == "" || strings.IndexByte(strings.ToUpper(dias), letraDia[t.Weekday()]) >= 0
}

// racha: los días seguidos hechos hasta hoy (si hoy todavía no, hasta ayer).
// Los días que no tocan no cuentan ni la cortan.
func racha(hechos map[string]bool, dias string, hoy time.Time) int {
	n := 0
	d := inicioDelDia(hoy)
	if !hechos[d.Format("2006-01-02")] {
		d = d.AddDate(0, 0, -1)
	}
	for k := 0; k < 400; k++ {
		switch {
		case hechos[d.Format("2006-01-02")]:
			n++
		case tocaHoy(dias, d):
			return n
		}
		d = d.AddDate(0, 0, -1)
	}
	return n
}

func (i *Integraciones) habito(ctx context.Context, bicho string, k int, h HabitoConfig, hoy time.Time) (Habito, error) {
	hechos, err := i.alm.DiasHabito(ctx, bicho, k, h.Nombre, inicioDelDia(hoy).AddDate(0, 0, -400))
	if err != nil {
		return Habito{}, err
	}
	r := Habito{Indice: k, Nombre: h.Nombre, Icono: h.Icono, Hora: h.Hora, Racha: racha(hechos, h.Dias, hoy),
		Hoy: hechos[hoy.Format("2006-01-02")], Dias: map[string]bool{}}
	corte := inicioDelDia(hoy).AddDate(0, 0, -DiasCalendario+1).Format("2006-01-02")
	for d := range hechos {
		if d >= corte {
			r.Dias[d] = true
		}
	}
	return r, nil
}

// Habitos: los hábitos del robot con su racha y sus días.
func (i *Integraciones) Habitos(ctx context.Context, bicho string) ([]Habito, error) {
	hs, err := i.habitosConfig(ctx, bicho)
	if err != nil {
		return nil, err
	}
	hoy := time.Now().In(nube.Zona())
	l := make([]Habito, 0, len(hs))
	for k, h := range hs {
		x, err := i.habito(ctx, bicho, k, h, hoy)
		if err != nil {
			return nil, err
		}
		l = append(l, x)
	}
	return l, nil
}

// MarcarHabito marca (o desmarca) el hábito k de hoy. Si no lo marcó el
// robot (delRobot), le manda el 0x4A con la racha. Si quedó hecho, corren
// las automatizaciones "habito" ({habito}, {racha}). enviado: le llegó al robot.
func (i *Integraciones) MarcarHabito(ctx context.Context, bicho string, k int, hecho, delRobot bool) (Habito, bool, error) {
	hs, err := i.habitosConfig(ctx, bicho)
	if err != nil {
		return Habito{}, false, err
	}
	if k < 0 || k >= len(hs) {
		return Habito{}, false, ErrSinHabito
	}
	hoy := time.Now().In(nube.Zona())
	if err := i.alm.MarcarHabito(ctx, bicho, k, hs[k].Nombre, hoy, hecho); err != nil {
		return Habito{}, false, err
	}
	h, err := i.habito(ctx, bicho, k, hs[k], hoy)
	if err != nil {
		return Habito{}, false, err
	}
	enviado := false
	if !delRobot {
		m, _ := protocolo.Habito(k, hecho, h.Racha)
		enviado = i.robots.Enviar(bicho, protocolo.CarComando, m) == nil
	}
	if hecho {
		i.encolar(bicho, "habito", map[string]any{"indice": k, "habito": h.Nombre, "racha": h.Racha})
	}
	return h, enviado, nil
}

// habitoDelRobot: el robot avisó que marcó un hábito (registro tipo 8).
func (i *Integraciones) habitoDelRobot(bicho string, datos map[string]any) {
	k, ok := datos["indice"].(int)
	if !ok {
		return
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if _, _, err := i.MarcarHabito(ctx, bicho, k, true, true); err != nil && !errors.Is(err, ErrSinHabito) {
		i.log.Warn("hábitos: no pude anotar el del robot", "bicho", bicho, "error", err)
	}
}

// --- Telegram: /habito ---

var iconosHabito = map[string]string{"gota": "💧", "libro": "📖", "pesa": "🏋️", "zen": "🧘", "pastilla": "💊",
	"cama": "🛏️", "fruta": "🍎", "estrella": "⭐"}

func (h Habito) enTexto() string {
	s := strings.TrimSpace(iconosHabito[h.Icono] + " " + h.Nombre)
	if h.Hoy {
		s += " ✅"
	}
	if h.Racha > 0 {
		s += fmt.Sprintf(" (racha: %d)", h.Racha)
	}
	return s
}

func tecladoHabitos(id string, hs []Habito) [][]Boton {
	var t [][]Boton
	for _, h := range hs {
		t = append(t, []Boton{{Texto: strings.TrimSpace(iconosHabito[h.Icono] + " " + h.Nombre), Datos: fmt.Sprintf("habito:%s:%d", id, h.Indice)}})
	}
	return t
}

// listarHabitos: lo que contesta /habito sin decir cuál.
func (i *Integraciones) listarHabitos(ctx context.Context, r robot) (string, [][]Boton) {
	hs, err := i.Habitos(ctx, r.ID)
	if err != nil {
		i.log.Warn("telegram: no pude leer los hábitos", "bicho", r.ID, "error", err)
		return "No pude leer los hábitos de " + r.nombre + ".", nil
	}
	if len(hs) == 0 {
		return r.nombre + " no tiene hábitos todavía: armalos en la web, en la consola del robot.", nil
	}
	var l []string
	for _, h := range hs {
		l = append(l, "• "+h.enTexto())
	}
	return "Los hábitos de " + r.nombre + " (tocá el que hiciste hoy):\n" + strings.Join(l, "\n"), tecladoHabitos(r.ID, hs)
}

// habitoPorNombre: el índice del hábito que se llama así (sin tildes ni mayúsculas).
func (i *Integraciones) habitoPorNombre(ctx context.Context, bicho, nombre string) (int, bool) {
	hs, err := i.habitosConfig(ctx, bicho)
	if err != nil {
		return 0, false
	}
	n := normal(nombre)
	for k, h := range hs {
		if normal(h.Nombre) == n {
			return k, true
		}
	}
	return 0, false
}

// marcarDesdeTelegram: lo que contesta al marcar uno.
func (i *Integraciones) marcarDesdeTelegram(ctx context.Context, r robot, k int) string {
	h, enviado, err := i.MarcarHabito(ctx, r.ID, k, true, false)
	if errors.Is(err, ErrSinHabito) {
		return "Ese hábito ya no está."
	}
	if err != nil {
		i.log.Warn("telegram: no pude marcar el hábito", "bicho", r.ID, "error", err)
		return "No pude marcar el hábito. Probá de nuevo en un rato."
	}
	t := "¡Bien! " + strings.TrimSpace(iconosHabito[h.Icono]+" "+h.Nombre) + " hecho hoy."
	if h.Racha >= 2 {
		t += fmt.Sprintf(" Llevás %d días seguidos 🔥", h.Racha)
	}
	if enviado {
		t += " " + r.nombre + " se puso orgulloso."
	}
	return t
}
