package integraciones

// Presencia (v1.9, docs/nube.md, "Presencia"): las llegadas y salidas que
// avisa la app del celu (POST …/presencia) o el robot (eventos 42 y 43), una
// sola vez aunque lleguen por los dos lados (lo mismo en menos de 2 min es
// lo mismo), y los horarios que aprende la nube: a qué hora llegás, salís y
// te dormís (la mediana de los últimos 14 días).

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// JuntaPresencia: dos avisos iguales más cerca que esto son el mismo.
const JuntaPresencia = 2 * time.Minute

// DiasHorarios: de cuántos días se aprenden los horarios.
const DiasHorarios = 14

func tipoPresencia(llegaste bool) string {
	if llegaste {
		return "llegaste"
	}
	return "te-fuiste"
}

// Presencia: la app avisó que llegaste o te fuiste. Si es nueva, se anota, se
// le avisa al robot (0x49) y corren las automatizaciones (llegaste,
// te-fuiste). enviado: le llegó al robot.
func (i *Integraciones) Presencia(ctx context.Context, bicho string, llegaste bool) (nueva, enviado bool, err error) {
	tipo := tipoPresencia(llegaste)
	nueva, err = i.alm.AnotarPresencia(ctx, bicho, tipo, time.Now().UTC(), JuntaPresencia)
	if err != nil || !nueva {
		return nueva, false, err
	}
	enviado = i.robots.Enviar(bicho, protocolo.CarComando, protocolo.Presencia(llegaste)) == nil
	i.encolar(bicho, tipo, map[string]any{})
	return true, enviado, nil
}

// presenciaDelRobot: el robot avisó el evento 42 o 43 (lo supo por su cuenta).
func (i *Integraciones) presenciaDelRobot(bicho, tipo string) {
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	nueva, err := i.alm.AnotarPresencia(ctx, bicho, tipo, time.Now().UTC(), JuntaPresencia)
	if err != nil {
		i.log.Warn("presencia: no la pude anotar", "bicho", bicho, "error", err)
		return
	}
	if nueva {
		i.encolar(bicho, tipo, map[string]any{})
	}
}

// PresenciaMQTT: el robot-celu supo por su cuenta que llegaste o te fuiste
// (bicho/{id}/presencia {"llegaste": bool}) y ya se lo dijo a su núcleo: acá
// solo se anota para los horarios y corren las automatizaciones. Corre en la
// goroutine del broker, así que lo pesado va aparte.
func (i *Integraciones) PresenciaMQTT(bicho string, datos []byte) {
	var p struct {
		Llegaste *bool `json:"llegaste"`
	}
	if len(datos) > 256 || json.Unmarshal(datos, &p) != nil || p.Llegaste == nil {
		return
	}
	go i.presenciaDelRobot(bicho, tipoPresencia(*p.Llegaste))
}

// anotarDormida: el robot se durmió (de su estado): sirve para "dormís".
func (i *Integraciones) anotarDormida(bicho string) {
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	if _, err := i.alm.AnotarPresencia(ctx, bicho, "duerme", time.Now().UTC(), 0); err != nil {
		i.log.Warn("presencia: no pude anotar la dormida", "bicho", bicho, "error", err)
	}
}

// Horarios: lo que aprendió la nube (null = todavía no sabe). seguridad va
// de 0 (ni idea) a 1 (lo hacés siempre a la misma hora).
type Horarios struct {
	Llegas    *string `json:"llegas"`
	Salis     *string `json:"salis"`
	Dormis    *string `json:"dormis"`
	Seguridad float64 `json:"seguridad"`
	Dias      int     `json:"dias"` // los días con datos
}

// HorariosDe aprende los horarios de un robot con sus últimos 14 días.
func (i *Integraciones) HorariosDe(ctx context.Context, bicho string) (Horarios, error) {
	ahora := time.Now().In(nube.Zona())
	l, err := i.alm.Presencias(ctx, bicho, ahora.AddDate(0, 0, -DiasHorarios-1))
	if err != nil {
		return Horarios{}, err
	}
	var ps []presenciaLocal
	for _, p := range l {
		ps = append(ps, presenciaLocal{p.Tipo, p.En.In(nube.Zona())})
	}
	return aprenderHorarios(ps, ahora), nil
}

type presenciaLocal struct {
	tipo string
	en   time.Time
}

// aprenderHorarios: por cada día, la ausencia más larga (de 30 min a 20 h)
// da la salida y la llegada; la primera dormida después de las 20 (o antes
// de las 5), la hora de dormir. De cada uno, la mediana.
func aprenderHorarios(ps []presenciaLocal, ahora time.Time) Horarios {
	desde := inicioDelDia(ahora).AddDate(0, 0, -DiasHorarios)
	type ausencia struct{ sale, llega time.Time }
	mejores := map[string]ausencia{}
	noches := map[string]int{}
	dias := map[string]bool{}
	var salida *time.Time
	for _, p := range ps {
		if p.en.Before(desde) && p.tipo != "te-fuiste" {
			continue
		}
		switch p.tipo {
		case "te-fuiste":
			t := p.en
			salida = &t
		case "llegaste":
			if salida != nil {
				d := p.en.Sub(*salida)
				dia := p.en.Format("2006-01-02")
				if d >= 30*time.Minute && d <= 20*time.Hour && !salida.Before(desde) {
					if m, ok := mejores[dia]; !ok || d > m.llega.Sub(m.sale) {
						mejores[dia] = ausencia{*salida, p.en}
					}
				}
			}
			salida = nil
			dias[p.en.Format("2006-01-02")] = true
		case "duerme":
			m := minutos(p.en)
			if m >= 20*60 || m < 5*60 {
				noche := p.en.Add(-12 * time.Hour).Format("2006-01-02")
				if m < 12*60 {
					m += 1440 // las 00:30 son las 24:30 de la noche anterior
				}
				if v, ok := noches[noche]; !ok || m < v {
					noches[noche] = m
				}
				dias[p.en.Format("2006-01-02")] = true
			}
		}
	}
	var llegas, salis, dormis []int
	for _, a := range mejores {
		llegas = append(llegas, minutos(a.llega))
		salis = append(salis, minutos(a.sale))
	}
	for _, m := range noches {
		dormis = append(dormis, m)
	}
	h := Horarios{Dias: len(dias)}
	var seg float64
	for _, x := range []struct {
		l []int
		p **string
	}{{llegas, &h.Llegas}, {salis, &h.Salis}, {dormis, &h.Dormis}} {
		if len(x.l) == 0 {
			continue
		}
		med, mad := medianaYDesvio(x.l)
		s := hhmm(med)
		*x.p = &s
		seg += math.Min(1, float64(len(x.l))/7) * math.Max(0, 1-float64(mad)/90)
	}
	h.Seguridad = math.Round(seg/3*100) / 100
	return h
}

func hhmm(m int) string {
	m = ((m % 1440) + 1440) % 1440
	return time.Date(2000, 1, 1, m/60, m%60, 0, 0, time.UTC).Format("15:04")
}

func mediana(l []int) int {
	c := append([]int{}, l...)
	sort.Ints(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// medianaYDesvio: la mediana y la mediana de las distancias a ella (en minutos).
func medianaYDesvio(l []int) (int, int) {
	med := mediana(l)
	d := make([]int, len(l))
	for k, x := range l {
		d[k] = x - med
		if d[k] < 0 {
			d[k] = -d[k]
		}
	}
	return med, mediana(d)
}
