package integraciones

// Lo de la v1.6 en la nube: el clima que se le manda a cada robot (marco
// 0x41), las automatizaciones "cuando: hora" (rutinas), el cumple implícito
// por Telegram y el camino para cambiar la config desde una automatización.
// Ver docs/nube.md, "Clima", "Rutinas" y "Cumpleaños".

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/clima"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// CambiarConfig cambia claves de arriba de todo de la config de un bicho por el
// mismo camino que PUT /v1/bichos/{id}/config (validada por el núcleo, versión
// nueva con ese autor; una clave en nil se saca). La pone el servidor (UsarConfig).
type CambiarConfig func(ctx context.Context, bicho, usuario, autor string, claves map[string]any) error

// UsarConfig le da a las integraciones cómo guardar la config (sin esto, la
// acción "modo" falla con un error claro).
func (i *Integraciones) UsarConfig(f CambiarConfig) { i.cambiarConfig = f }

// Clima es el cliente del clima (lo usa también el servidor HTTP y el MCP).
func (i *Integraciones) Clima() *clima.Cliente { return i.clima }

// --- Lo que la nube lee de la config ---

// datosConfig: lo de la config que usa la nube.
type datosConfig struct {
	Nombre    string           `json:"nombre"`
	Ubicacion *clima.Ubicacion `json:"ubicacion"`
	Dueno     struct {
		Nombre string `json:"nombre"`
		Cumple string `json:"cumple"`
	} `json:"dueno"`
	Cumple  string `json:"cumple"`
	Horario struct {
		Despierta string `json:"despierta"`
	} `json:"horario"`
}

func leerDatosConfig(texto string) datosConfig {
	var d datosConfig
	json.Unmarshal([]byte(texto), &d) //nolint:errcheck
	if d.Ubicacion != nil && !d.Ubicacion.Valida() {
		d.Ubicacion = nil
	}
	return d
}

func (i *Integraciones) datosDe(ctx context.Context, bicho string) (datosConfig, bool) {
	c, err := i.alm.UltimaConfig(ctx, bicho)
	if err != nil {
		return datosConfig{}, false
	}
	return leerDatosConfig(c.Texto), true
}

// UbicacionDe: la de la config del bicho (nil si no tiene).
func (i *Integraciones) UbicacionDe(ctx context.Context, bicho string) *clima.Ubicacion {
	d, _ := i.datosDe(ctx, bicho)
	return d.Ubicacion
}

// ClimaDe: la ubicación del bicho y su clima (el clima, nil si no tiene
// ubicación o si Open-Meteo no contestó).
func (i *Integraciones) ClimaDe(ctx context.Context, bicho string) (*clima.Ubicacion, *clima.Clima) {
	u := i.UbicacionDe(ctx, bicho)
	if u == nil {
		return nil, nil
	}
	c, err := i.clima.Actual(ctx, u.Lat, u.Lon)
	if err != nil {
		i.log.Info("clima: no lo pude traer", "bicho", bicho, "error", err)
		return u, nil
	}
	return u, &c
}

// --- El clima de cada robot ---

// CadaClima: cada cuánto se le manda el clima a un robot conectado.
const CadaClima = 30 * time.Minute

type climaMandado struct {
	lugar string
	en    time.Time
}

type estadoClima struct {
	mu        sync.Mutex
	mandado   map[string]climaMandado // bicho → el último que se le mandó
	conectado map[string]bool         // los que estaban conectados la última vuelta
	ya        chan string             // "mandale ya" (al conectarse o al cambiar la ubicación)
}

// ClimaYa le manda el clima al robot apenas se pueda (si está conectado y
// tiene ubicación). No bloquea.
func (i *Integraciones) ClimaYa(bicho string) {
	i.climaEst.mu.Lock()
	delete(i.climaEst.mandado, bicho)
	i.climaEst.mu.Unlock()
	select {
	case i.climaEst.ya <- bicho:
	default:
	}
}

// trabajadorClima: cada minuto revisa los robots conectados con ubicación y
// les manda el clima si pasaron 30 min, si se acaban de conectar o si
// cambió la ubicación.
func (i *Integraciones) trabajadorClima(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	i.revisarClima(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-i.climaEst.ya:
			// Que el trabajador del bicho termine de atender la conexión.
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			i.mandarClima(ctx, b)
		case <-t.C:
			i.revisarClima(ctx)
		}
	}
}

func (i *Integraciones) revisarClima(ctx context.Context) {
	ctx, cancelar := context.WithTimeout(ctx, 50*time.Second)
	defer cancelar()
	ids, err := i.alm.BichosConDueno(ctx)
	if err != nil {
		i.log.Warn("clima: no pude listar los robots", "error", err)
		return
	}
	ahora := map[string]bool{}
	for _, id := range ids {
		if i.robots.Conectado(id) {
			ahora[id] = true
		}
	}
	i.climaEst.mu.Lock()
	for id := range ahora {
		if !i.climaEst.conectado[id] {
			delete(i.climaEst.mandado, id) // se acaba de conectar
		}
	}
	for id := range i.climaEst.mandado {
		if !ahora[id] {
			delete(i.climaEst.mandado, id)
		}
	}
	i.climaEst.conectado = ahora
	i.climaEst.mu.Unlock()
	for id := range ahora {
		if ctx.Err() != nil {
			return
		}
		i.mandarClima(ctx, id)
	}
}

// mandarClima se lo manda si toca (30 min, o cambió el lugar, o nunca).
func (i *Integraciones) mandarClima(ctx context.Context, bicho string) {
	if !i.robots.Conectado(bicho) {
		return
	}
	u := i.UbicacionDe(ctx, bicho)
	if u == nil {
		return
	}
	lugar := u.Clave() + "|" + u.Ciudad
	i.climaEst.mu.Lock()
	m, hay := i.climaEst.mandado[bicho]
	i.climaEst.mu.Unlock()
	if hay && m.lugar == lugar && time.Since(m.en) < CadaClima {
		return
	}
	c, err := i.clima.Actual(ctx, u.Lat, u.Lon)
	if err != nil {
		i.log.Info("clima: no lo pude traer", "bicho", bicho, "error", err)
		return
	}
	marco := protocolo.Clima(c.Temp, c.Codigo, c.Max, c.Min, c.Dia, u.Ciudad)
	if err := i.robots.Enviar(bicho, protocolo.CarComando, marco); err != nil {
		if !errors.Is(err, nube.ErrDesconectado) {
			i.log.Warn("clima: no se lo pude mandar", "bicho", bicho, "error", err)
		}
		return
	}
	i.climaEst.mu.Lock()
	i.climaEst.mandado[bicho] = climaMandado{lugar: lugar, en: time.Now()}
	i.climaEst.mu.Unlock()
}

// --- Cada minuto: rutinas (cuando: hora) ---

// porMinuto llama a f con cada minuto (hora de los bichos), apenas empieza.
// Si se atrasó (la máquina dormía), se pone al día como mucho 3 minutos.
func porMinuto(ctx context.Context, f func(m time.Time)) {
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
		desde := ultimo.Add(time.Minute)
		if ahora.Sub(desde) > 3*time.Minute {
			desde = ahora.Add(-2 * time.Minute)
		}
		for m := desde; !m.After(ahora); m = m.Add(time.Minute) {
			f(m)
		}
		ultimo = ahora
	}
}

// Toca dice si una automatización "cuando: hora" corre en ese minuto.
func (a Automatizacion) Toca(m time.Time) bool {
	return a.Activa && a.Cuando == "hora" && Recordatorio{Hora: a.Hora, Dias: a.Dias}.Toca(m)
}

// revisarRutinas encola las automatizaciones "cuando: hora" del minuto m.
func (i *Integraciones) revisarRutinas(ctx context.Context, m time.Time) {
	ctx, cancelar := context.WithTimeout(ctx, 20*time.Second)
	defer cancelar()
	ids, err := i.alm.BichosConCuando(ctx, "hora")
	if err != nil {
		i.log.Warn("rutinas: no pude leerlas", "error", err)
		return
	}
	for _, id := range ids {
		for _, a := range i.automatizacionesDe(id) {
			if !a.Toca(m) {
				continue
			}
			select {
			case i.cola <- disparo{bicho: id, evento: "hora", id: a.ID, datos: map[string]any{"hora": m.Format("15:04")}}:
			default:
				i.log.Warn("rutinas: cola llena, pierdo una", "bicho", id, "automatizacion", a.ID)
			}
		}
	}
}

// --- Cumpleaños (por Telegram, una vez por año) ---

// mismoDia: "MM-DD" cae en m (el 29/2, en los años sin, se festeja el 28).
func mismoDia(mmdd string, m time.Time) bool {
	mmdd = strings.TrimSpace(mmdd)
	if len(mmdd) != 5 || mmdd[2] != '-' {
		return false
	}
	hoy := m.Format("01-02")
	if mmdd == hoy {
		return true
	}
	bisiesto := time.Date(m.Year(), 2, 29, 0, 0, 0, 0, m.Location()).Month() == time.February
	return mmdd == "02-29" && !bisiesto && hoy == "02-28"
}

// horaDeDespertar: la de horario.despierta (08:00 si no hay o es inválida).
func (d datosConfig) horaDeDespertar() string {
	if h, err := time.Parse("15:04", strings.TrimSpace(d.Horario.Despierta)); err == nil {
		return h.Format("15:04")
	}
	return "08:00"
}

// cumples devuelve los saludos que tocan en el minuto m para un robot:
// automatización del historial → texto.
func cumples(d datosConfig, robot string, m time.Time) [][2]string {
	if m.Format("15:04") != d.horaDeDespertar() {
		return nil
	}
	var l [][2]string
	if mismoDia(d.Dueno.Cumple, m) {
		t := "🎂 ¡Feliz cumple! Te saluda " + robot + "."
		if n := strings.TrimSpace(d.Dueno.Nombre); n != "" {
			t = "🎂 ¡Feliz cumple, " + n + "! Te saluda " + robot + "."
		}
		l = append(l, [2]string{"cumple-dueno", t})
	}
	if mismoDia(d.Cumple, m) {
		l = append(l, [2]string{"cumple-robot", "🎉 Hoy " + robot + " cumple años."})
	}
	return l
}

// saludarCumples manda los cumples de un robot (si no se mandaron este año).
func (i *Integraciones) saludarCumples(ctx context.Context, chat int64, r robot, d datosConfig, m time.Time) {
	for _, c := range cumples(d, r.nombre, m) {
		inicio := time.Date(m.Year(), 1, 1, 0, 0, 0, 0, m.Location())
		ya, err := i.alm.YaCorrio(ctx, r.ID, c[0], inicio)
		if err != nil || ya {
			continue
		}
		err = i.tg.mandar(ctx, chat, c[1], nil)
		e := almacen.Ejecucion{Automatizacion: c[0], Evento: "cumple", Accion: "telegram", Ok: err == nil, En: time.Now().UTC()}
		if err != nil {
			e.Detalle = err.Error()
			i.log.Warn("cumple: no pude mandar", "bicho", r.ID, "error", err)
		}
		i.alm.AnotarEjecucion(ctx, r.ID, e) //nolint:errcheck
	}
}

// --- Acciones nuevas ---

// mandarAlBicho manda un marco a este bicho (tiene que estar conectado).
func (i *Integraciones) mandarAlBicho(bicho string, marco []byte) error {
	if err := i.robots.Enviar(bicho, protocolo.CarComando, marco); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			return errors.New("el robot no está conectado")
		}
		return err
	}
	return nil
}

// decirEnBicho: el texto (ya con las plantillas) cortado a 60 bytes.
func (i *Integraciones) decirEnBicho(bicho, texto, expresion string) error {
	texto = strings.TrimSpace(protocolo.CortarUTF8(strings.TrimSpace(texto), protocolo.MaxDecir))
	if texto == "" {
		return errors.New("el texto quedó vacío")
	}
	m, err := protocolo.Decir(texto, expresion)
	if err != nil {
		return err
	}
	return i.mandarAlBicho(bicho, m)
}

func (i *Integraciones) cambiarModo(ctx context.Context, usuario, bicho, modo string) error {
	if i.cambiarConfig == nil {
		return errors.New("no puedo cambiar la config desde acá")
	}
	return i.cambiarConfig(ctx, bicho, usuario, "automatizacion", map[string]any{"modo": modo})
}

// textoClima: lo de las plantillas del clima.
func (c *contexto) ponerClima(cl *clima.Clima) {
	if cl == nil {
		return
	}
	c.clima, c.temp = cl.Texto(), clima.Grados(cl.Temp)
	c.max, c.min = clima.Grados(float64(cl.Max)), clima.Grados(float64(cl.Min))
}

// usaClima: alguna acción usa las plantillas del clima.
func usaClima(a Automatizacion) bool {
	for _, h := range a.Hacer {
		for _, p := range []string{"{clima}", "{temp}", "{max}", "{min}"} {
			if strings.Contains(h.Texto, p) {
				return true
			}
		}
	}
	return false
}
