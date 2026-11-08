package integraciones

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// Automatizacion es "cuando pasa X en el bicho → hacer Y". El formato es el
// que usa la web (docs/nube.md, "Integraciones"): no cambiarlo.
type Automatizacion struct {
	ID     string   `json:"id"`
	Activa bool     `json:"activa"`
	Cuando string   `json:"cuando"`
	Valor  string   `json:"valor,omitempty"` // solo para "expresion": cuál
	Hacer  []Accion `json:"hacer"`
}

// Accion es una de las cosas que hace una automatización.
type Accion struct {
	Tipo      string `json:"tipo"`                // "telegram", "webhook" o "robot"
	Texto     string `json:"texto,omitempty"`     // telegram (con plantillas)
	URL       string `json:"url,omitempty"`       // webhook
	Bicho     string `json:"bicho,omitempty"`     // robot: otro bicho del mismo dueño
	Expresion string `json:"expresion,omitempty"` // robot: la cara que pone
}

// Resultado de una acción.
type Resultado struct {
	Tipo    string `json:"tipo"`
	Ok      bool   `json:"ok"`
	Detalle string `json:"detalle,omitempty"`
}

// Límites de las automatizaciones.
const (
	MaxAutomatizaciones = 20
	MaxAcciones         = 5
	MaxTexto            = 1000
	MaxURL              = 500
	DisparoCada         = 10 * time.Second
	DisparosPorDia      = 200
)

// CuandoDeEstado son los "cuando" que salen del estado y la conexión.
var CuandoDeEstado = []string{"conectado", "desconectado", "bateria-baja", "expresion", "despierta", "duerme"}

// Cuandos devuelve todos los "cuando" que la API detecta de verdad.
func Cuandos() []string {
	l := append([]string{}, CuandoDeEstado...)
	l = append(l, "recordatorio", "hito")
	vistos := map[string]bool{"recordatorio": true}
	for _, e := range protocolo.Eventos {
		if c, ok := nube.EventosRegistro[e]; ok && !vistos[c] {
			vistos[c] = true
			l = append(l, c)
		}
	}
	return l
}

func cuandoValido(c string) bool {
	for _, x := range Cuandos() {
		if x == c {
			return true
		}
	}
	return false
}

var patronID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

// ErrInvalida es una lista de automatizaciones que no se puede guardar.
type ErrInvalida struct{ Motivo string }

func (e ErrInvalida) Error() string { return e.Motivo }

func invalida(f string, a ...any) error { return ErrInvalida{fmt.Sprintf(f, a...)} }

// Validar revisa la lista entera. esMio dice si un bicho es del mismo dueño.
func Validar(l []Automatizacion, esMio func(id string) bool, w *ClienteWebhook) error {
	if len(l) > MaxAutomatizaciones {
		return invalida("son como mucho %d automatizaciones por robot", MaxAutomatizaciones)
	}
	ids := map[string]bool{}
	for k, a := range l {
		donde := fmt.Sprintf("automatización %d", k+1)
		if !patronID.MatchString(a.ID) {
			return invalida("%s: el id es de 1 a 40 letras, números, - o _", donde)
		}
		if ids[a.ID] {
			return invalida("%s: el id %q está repetido", donde, a.ID)
		}
		ids[a.ID] = true
		if !cuandoValido(a.Cuando) {
			return invalida("%s: no conozco el evento %q", donde, a.Cuando)
		}
		if a.Valor != "" && a.Cuando != "expresion" {
			return invalida("%s: \"valor\" es solo para \"expresion\"", donde)
		}
		if len(a.Valor) > 40 {
			return invalida("%s: el valor es de hasta 40 bytes", donde)
		}
		if len(a.Hacer) == 0 || len(a.Hacer) > MaxAcciones {
			return invalida("%s: hace de 1 a %d cosas", donde, MaxAcciones)
		}
		for j, h := range a.Hacer {
			donde := fmt.Sprintf("%s, acción %d", donde, j+1)
			switch h.Tipo {
			case "telegram":
				if strings.TrimSpace(h.Texto) == "" || len(h.Texto) > MaxTexto || !utf8.ValidString(h.Texto) {
					return invalida("%s: el texto es de 1 a %d bytes", donde, MaxTexto)
				}
				if h.URL != "" || h.Bicho != "" || h.Expresion != "" {
					return invalida("%s: un telegram lleva solo \"texto\"", donde)
				}
			case "webhook":
				if h.Texto != "" || h.Bicho != "" || h.Expresion != "" {
					return invalida("%s: un webhook lleva solo \"url\"", donde)
				}
				if err := w.ValidarURL(h.URL); err != nil {
					return invalida("%s: %v", donde, err)
				}
			case "robot":
				if h.Texto != "" || h.URL != "" {
					return invalida("%s: un robot lleva \"bicho\" y \"expresion\"", donde)
				}
				id := strings.ToUpper(h.Bicho)
				if !protocolo.IDValido(id) || !esMio(id) {
					return invalida("%s: ese robot no es tuyo", donde)
				}
				if h.Expresion == "" || len(h.Expresion) > 40 || !utf8.ValidString(h.Expresion) {
					return invalida("%s: la expresión es de 1 a 40 bytes", donde)
				}
			default:
				return invalida("%s: el tipo es \"telegram\", \"webhook\" o \"robot\"", donde)
			}
		}
	}
	return nil
}

// --- Correrlas ---

func (i *Integraciones) disparar(ctx context.Context, d disparo) {
	usuario := i.robots.Dueno(d.bicho)
	if usuario == "" {
		return
	}
	for _, a := range i.automatizacionesDe(d.bicho) {
		if !a.Activa || a.Cuando != d.evento {
			continue
		}
		if a.Valor != "" {
			if v, _ := d.datos["valor"].(string); v != a.Valor {
				continue
			}
		}
		if _, err := i.correr(ctx, usuario, d.bicho, a, d.evento, d.datos); errors.Is(err, ErrLimite) {
			i.log.Info("automatización limitada", "bicho", d.bicho, "automatizacion", a.ID)
		}
	}
}

// correr hace las acciones de una automatización (si el límite lo deja),
// las anota en el historial y avisa a la web.
func (i *Integraciones) correr(ctx context.Context, usuario, bicho string, a Automatizacion, evento string, datos map[string]any) ([]Resultado, error) {
	if !i.limite.permitir(bicho+"/"+a.ID, DisparoCada, DisparosPorDia, 24*time.Hour) {
		return nil, ErrLimite
	}
	ctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
	defer cancelar()
	c := i.contexto(ctx, bicho, evento, datos)
	en := time.Now().UTC()
	res := make([]Resultado, 0, len(a.Hacer))
	for _, h := range a.Hacer {
		r := Resultado{Tipo: h.Tipo, Ok: true}
		var err error
		switch h.Tipo {
		case "telegram":
			err = i.MandarTelegram(ctx, usuario, c.completar(h.Texto))
		case "webhook":
			err = i.mandarWebhook(ctx, usuario, h.URL, c, en)
		case "robot":
			err = i.mandarRobot(ctx, usuario, strings.ToUpper(h.Bicho), h.Expresion)
		default:
			err = errors.New("tipo desconocido")
		}
		if err != nil {
			r.Ok, r.Detalle = false, err.Error()
		}
		res = append(res, r)
		if err := i.alm.AnotarEjecucion(ctx, bicho, almacen.Ejecucion{
			Automatizacion: a.ID, Evento: evento, Accion: h.Tipo, Ok: r.Ok, Detalle: r.Detalle, En: en,
		}); err != nil {
			i.log.Warn("no pude anotar la ejecución", "bicho", bicho, "error", err)
		}
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "automatizacion", Bicho: bicho, Datos: map[string]any{
		"id": a.ID, "cuando": a.Cuando, "evento": evento, "resultados": res, "en": en,
	}})
	return res, nil
}

func (i *Integraciones) mandarRobot(ctx context.Context, usuario, id, expresion string) error {
	if _, err := i.alm.BichoDe(ctx, id, usuario); err != nil {
		return errors.New("ese robot ya no es tuyo")
	}
	if err := i.robots.Enviar(id, protocolo.CarComando, protocolo.Expresion(expresion)); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			return errors.New("el robot no está conectado")
		}
		return err
	}
	return nil
}

// --- Plantillas ---

// contexto es lo que se puede usar en los textos: {robot}, {bateria},
// {expresion}, {evento}, {texto}, {hora}.
type contexto struct {
	bichoID, robot, bateria, expresion, evento, texto, hora string
	datos                                                   map[string]any
}

func (i *Integraciones) contexto(ctx context.Context, bicho, evento string, datos map[string]any) contexto {
	c := contexto{bichoID: bicho, robot: bicho, bateria: "?", evento: evento, datos: map[string]any{},
		hora: time.Now().In(nube.Zona()).Format("15:04")}
	for k, v := range datos {
		c.datos[k] = v
	}
	est, hay := i.robots.EstadoDe(bicho)
	b, err := i.alm.Bicho(ctx, bicho)
	if !hay && err == nil && len(b.Estado) > 0 {
		est, err = protocolo.LeerEstado(b.Estado)
		hay = err == nil
	}
	if hay {
		if est.Nombre != "" {
			c.robot = est.Nombre
		}
		if est.Bateria != nil {
			c.bateria = strconv.Itoa(*est.Bateria)
		}
		c.expresion = est.Expresion
	}
	if b.Apodo != "" {
		c.robot = b.Apodo
	}
	if v, ok := datos["bateria"].(int); ok {
		c.bateria = strconv.Itoa(v)
	}
	if v, ok := datos["valor"].(string); ok && evento == "expresion" {
		c.expresion = v
	}
	if v, ok := datos["texto"].(string); ok {
		c.texto = v
	} else if k, ok := datos["indice"].(int); ok && evento == "recordatorio" {
		// El bicho dice cuál recordatorio mostró: el texto está en su config.
		if cfg, err := i.alm.UltimaConfig(ctx, bicho); err == nil {
			if l := recordatoriosDe(cfg.Texto); k >= 0 && k < len(l) {
				c.texto = l[k].Texto
				c.datos["texto"] = l[k].Texto
				c.datos["hora"] = l[k].Hora
			}
		}
	}
	return c
}

func (c contexto) completar(s string) string {
	return strings.NewReplacer("{robot}", c.robot, "{bateria}", c.bateria, "{expresion}", c.expresion,
		"{evento}", c.evento, "{texto}", c.texto, "{hora}", c.hora).Replace(s)
}
