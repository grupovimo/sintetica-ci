// Package clima trae el clima de Open-Meteo (https://open-meteo.com, sin
// clave): el pronóstico de un lugar (lo que se le manda al robot con el marco
// 0x41 y lo que llenan las plantillas {clima}, {temp}…) y la búsqueda de
// lugares por nombre (geocoding). Ver docs/nube.md, "Clima".
package clima

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// URLs de Open-Meteo (los tests ponen un servidor falso).
const (
	URLPronostico = "https://api.open-meteo.com/v1/forecast"
	URLGeocoding  = "https://geocoding-api.open-meteo.com/v1/search"
)

// DuracionCache: cuánto vale el clima de un lugar.
const DuracionCache = 30 * time.Minute

// MaxLugares: cuántos lugares devuelve una búsqueda.
const MaxLugares = 6

// MaxCiudad: el largo de "ciudad" en la ubicación de la config.
const MaxCiudad = 40

// Opciones del cliente. Vacías, las de Open-Meteo.
type Opciones struct {
	URLPronostico string
	URLGeocoding  string
	Timeout       time.Duration // 5 s
}

// Cliente con caché por lugar.
type Cliente struct {
	op    Opciones
	http  *http.Client
	ahora func() time.Time

	mu    sync.Mutex
	cache map[string]Clima
}

// Nuevo arma el cliente.
func Nuevo(op Opciones) *Cliente {
	if op.URLPronostico == "" {
		op.URLPronostico = URLPronostico
	}
	if op.URLGeocoding == "" {
		op.URLGeocoding = URLGeocoding
	}
	if op.Timeout <= 0 {
		op.Timeout = 5 * time.Second
	}
	return &Cliente{op: op, http: &http.Client{Timeout: op.Timeout}, ahora: time.Now, cache: map[string]Clima{}}
}

// Ubicacion es la de la config: {"lat", "lon", "ciudad"}.
type Ubicacion struct {
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Ciudad string  `json:"ciudad"`
}

// Valida: lat −90..90, lon −180..180, ciudad de 1 a 40 bytes.
func (u Ubicacion) Valida() bool {
	return u.Lat >= -90 && u.Lat <= 90 && u.Lon >= -180 && u.Lon <= 180 &&
		len(u.Ciudad) >= 1 && len(u.Ciudad) <= MaxCiudad && utf8.ValidString(u.Ciudad) &&
		!math.IsNaN(u.Lat) && !math.IsNaN(u.Lon)
}

// Clave: el lugar redondeado a 0,01 (lo que usa el caché).
func (u Ubicacion) Clave() string { return Clave(u.Lat, u.Lon) }

// Clave de un lugar: lat y lon redondeados a 0,01 (~1 km).
func Clave(lat, lon float64) string {
	r := func(x float64) string { return strconv.FormatFloat(math.Round(x*100)/100, 'f', 2, 64) }
	return r(lat) + "," + r(lon)
}

// Clima es el de ahora en un lugar, con la máxima y la mínima del día.
type Clima struct {
	Temp        float64   `json:"temp"`   // °C, con un decimal
	Codigo      int       `json:"codigo"` // WMO
	Descripcion string    `json:"descripcion"`
	Max         int       `json:"max"` // °C
	Min         int       `json:"min"`
	Dia         bool      `json:"dia"`
	En          time.Time `json:"en"` // cuándo se trajo
}

// Texto para la plantilla {clima}: "18° y nublado".
func (c Clima) Texto() string { return Grados(c.Temp) + " y " + c.Descripcion }

// Grados: "18°" (redondeado).
func Grados(t float64) string { return strconv.Itoa(int(math.Round(t))) + "°" }

// Descripcion de un código WMO, en español.
func Descripcion(codigo int) string {
	switch codigo {
	case 0:
		return "despejado"
	case 1:
		return "casi despejado"
	case 2:
		return "algo nublado"
	case 3:
		return "nublado"
	case 45, 48:
		return "niebla"
	case 51, 53, 55:
		return "llovizna"
	case 56, 57:
		return "llovizna helada"
	case 61, 63:
		return "lluvia"
	case 65:
		return "lluvia fuerte"
	case 66, 67:
		return "lluvia helada"
	case 71, 73, 75, 77, 85, 86:
		return "nieve"
	case 80, 81:
		return "chaparrones"
	case 82:
		return "chaparrones fuertes"
	case 95:
		return "tormenta"
	case 96, 99:
		return "tormenta con granizo"
	}
	return "variable"
}

// ErrServicio: Open-Meteo no contestó bien.
var ErrServicio = errors.New("el servicio del clima no contestó")

// Actual devuelve el clima de un lugar (del caché si tiene menos de 30 min;
// si Open-Meteo falla y hay uno viejo, ese).
func (c *Cliente) Actual(ctx context.Context, lat, lon float64) (Clima, error) {
	clave := Clave(lat, lon)
	c.mu.Lock()
	viejo, hay := c.cache[clave]
	c.mu.Unlock()
	if hay && c.ahora().Sub(viejo.En) < DuracionCache {
		return viejo, nil
	}
	nuevo, err := c.traer(ctx, lat, lon)
	if err != nil {
		if hay && c.ahora().Sub(viejo.En) < 3*time.Hour {
			return viejo, nil
		}
		return Clima{}, err
	}
	c.mu.Lock()
	c.cache[clave] = nuevo
	if len(c.cache) > 5000 {
		for k, v := range c.cache {
			if c.ahora().Sub(v.En) > DuracionCache {
				delete(c.cache, k)
			}
		}
	}
	c.mu.Unlock()
	return nuevo, nil
}

func (c *Cliente) pedir(ctx context.Context, base string, q url.Values, v any) error {
	ctx, cancelar := context.WithTimeout(ctx, c.op.Timeout)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "SinteticaOS/1 (+https://sinteticalabs.com)")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrServicio, limpiar(err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(res.Body, 64*1024)) //nolint:errcheck
		return fmt.Errorf("%w: estado %d", ErrServicio, res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 512*1024)).Decode(v); err != nil {
		return fmt.Errorf("%w: respuesta inválida", ErrServicio)
	}
	return nil
}

// limpiar saca la URL de un error de red (no hace falta en los logs).
func limpiar(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func (c *Cliente) traer(ctx context.Context, lat, lon float64) (Clima, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,weather_code,is_day")
	q.Set("daily", "temperature_2m_max,temperature_2m_min")
	q.Set("timezone", "auto")
	q.Set("forecast_days", "1")
	var r struct {
		Current *struct {
			Temperatura *float64 `json:"temperature_2m"`
			Codigo      *int     `json:"weather_code"`
			EsDia       *int     `json:"is_day"`
		} `json:"current"`
		Daily struct {
			Max []*float64 `json:"temperature_2m_max"`
			Min []*float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := c.pedir(ctx, c.op.URLPronostico, q, &r); err != nil {
		return Clima{}, err
	}
	if r.Current == nil || r.Current.Temperatura == nil || r.Current.Codigo == nil {
		return Clima{}, fmt.Errorf("%w: sin el clima de ahora", ErrServicio)
	}
	t := *r.Current.Temperatura
	cl := Clima{Temp: math.Round(t*10) / 10, Codigo: *r.Current.Codigo, Dia: true, En: c.ahora().UTC().Truncate(time.Second)}
	cl.Descripcion = Descripcion(cl.Codigo)
	if r.Current.EsDia != nil {
		cl.Dia = *r.Current.EsDia != 0
	}
	cl.Max, cl.Min = int(math.Round(t)), int(math.Round(t))
	if len(r.Daily.Max) > 0 && r.Daily.Max[0] != nil {
		cl.Max = int(math.Round(*r.Daily.Max[0]))
	}
	if len(r.Daily.Min) > 0 && r.Daily.Min[0] != nil {
		cl.Min = int(math.Round(*r.Daily.Min[0]))
	}
	return cl, nil
}

// Lugar es un resultado de la búsqueda.
type Lugar struct {
	Nombre string  `json:"nombre"`
	Region string  `json:"region"`
	Pais   string  `json:"pais"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Ciudad string  `json:"ciudad"` // lo que va en ubicacion.ciudad (hasta 40 bytes)
}

// Ubicacion: la que va en la config.
func (l Lugar) Ubicacion() Ubicacion { return Ubicacion{Lat: l.Lat, Lon: l.Lon, Ciudad: l.Ciudad} }

// ErrBusqueda: q tiene que ser de 2 a 60 bytes.
var ErrBusqueda = errors.New("el lugar a buscar es de 2 a 60 bytes")

// Buscar lugares por nombre (hasta 6, en español).
func (c *Cliente) Buscar(ctx context.Context, nombre string) ([]Lugar, error) {
	nombre = strings.TrimSpace(nombre)
	if len(nombre) < 2 || len(nombre) > 60 || !utf8.ValidString(nombre) {
		return nil, ErrBusqueda
	}
	q := url.Values{}
	q.Set("name", nombre)
	q.Set("count", strconv.Itoa(MaxLugares))
	q.Set("language", "es")
	q.Set("format", "json")
	var r struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := c.pedir(ctx, c.op.URLGeocoding, q, &r); err != nil {
		return nil, err
	}
	l := []Lugar{}
	for _, x := range r.Results {
		if len(l) == MaxLugares {
			break
		}
		if x.Name == "" {
			continue
		}
		lg := Lugar{Nombre: x.Name, Region: x.Admin1, Pais: x.Country,
			Lat: math.Round(x.Latitude*100) / 100, Lon: math.Round(x.Longitude*100) / 100}
		lg.Ciudad = x.Name
		if x.Country != "" {
			lg.Ciudad += ", " + x.Country
		}
		if len(lg.Ciudad) > MaxCiudad {
			lg.Ciudad = Cortar(x.Name, MaxCiudad)
		}
		l = append(l, lg)
	}
	return l, nil
}

// Cortar deja s en max bytes sin partir una letra.
func Cortar(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
