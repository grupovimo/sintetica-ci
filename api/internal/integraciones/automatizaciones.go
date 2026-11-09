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
	"sintetica/api/internal/clima"
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
	Hora   string   `json:"hora,omitempty"`  // solo para "hora": HH:MM (hora de los bichos)
	Dias   string   `json:"dias,omitempty"`  // solo para "hora": letras LMXJVSD (vacío = todos)
	Hacer  []Accion `json:"hacer"`
}

// Accion es una de las cosas que hace una automatización.
type Accion struct {
	Tipo      string `json:"tipo"`                // "telegram", "webhook", "robot", "decir", "cara", "modo", "despertar" o "jugar"
	Texto     string `json:"texto,omitempty"`     // telegram y decir (con plantillas)
	URL       string `json:"url,omitempty"`       // webhook
	Bicho     string `json:"bicho,omitempty"`     // robot: otro bicho del mismo dueño
	Expresion string `json:"expresion,omitempty"` // robot y cara: la cara que pone; decir: opcional
	Modo      string `json:"modo,omitempty"`      // modo: mascota, reloj, pomodoro, clima o notificaciones
	Juego     string `json:"juego,omitempty"`     // jugar: ppt, reflejos, mirada, dado o moneda
}

// TiposDeAccion: los "tipo" de una acción.
var TiposDeAccion = []string{"telegram", "webhook", "robot", "decir", "cara", "modo", "despertar", "jugar"}

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
	MaxTextoDecir       = 200 // el texto de "decir" con plantillas (expandido se corta a 60 bytes)
	DisparoCada         = 10 * time.Second
	DisparosPorDia      = 200
)

// CuandoDeEstado son los "cuando" que salen del estado y la conexión.
var CuandoDeEstado = []string{"conectado", "desconectado", "bateria-baja", "expresion", "despierta", "duerme"}

// Cuandos devuelve todos los "cuando" que la API detecta de verdad.
func Cuandos() []string {
	l := append([]string{}, CuandoDeEstado...)
	l = append(l, "recordatorio", "hito", "hora", "despertador", "juego", "animo", "habito", "agenda")
	vistos := map[string]bool{"recordatorio": true, "hito": true, "hora": true, "despertador": true, "juego": true, "animo": true, "habito": true, "agenda": true}
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

var patronHora = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// diasValidos: letras de LMXJVSD sin repetir (vacío = todos).
func diasValidos(d string) bool {
	if len(d) > 7 {
		return false
	}
	vistos := map[rune]bool{}
	for _, c := range d {
		if !strings.ContainsRune("LMXJVSD", c) || vistos[c] {
			return false
		}
		vistos[c] = true
	}
	return true
}

func modoValido(m string) bool {
	for _, x := range protocolo.Modos {
		if x == m {
			return true
		}
	}
	return false
}

func expresionValida(e string) bool { return e != "" && len(e) <= 40 && utf8.ValidString(e) }

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
		if a.Cuando == "hora" {
			if !patronHora.MatchString(a.Hora) {
				return invalida("%s: la hora es HH:MM (de 00:00 a 23:59)", donde)
			}
			if !diasValidos(a.Dias) {
				return invalida("%s: los días son letras L M X J V S D sin repetir (vacío = todos)", donde)
			}
		} else if a.Hora != "" || a.Dias != "" {
			return invalida("%s: \"hora\" y \"dias\" son solo para \"cuando\": \"hora\"", donde)
		}
		if len(a.Hacer) == 0 || len(a.Hacer) > MaxAcciones {
			return invalida("%s: hace de 1 a %d cosas", donde, MaxAcciones)
		}
		for j, h := range a.Hacer {
			donde := fmt.Sprintf("%s, acción %d", donde, j+1)
			if h.Juego != "" && h.Tipo != "jugar" {
				return invalida("%s: \"juego\" es solo para \"jugar\"", donde)
			}
			switch h.Tipo {
			case "telegram":
				if strings.TrimSpace(h.Texto) == "" || len(h.Texto) > MaxTexto || !utf8.ValidString(h.Texto) {
					return invalida("%s: el texto es de 1 a %d bytes", donde, MaxTexto)
				}
				if h.URL != "" || h.Bicho != "" || h.Expresion != "" || h.Modo != "" {
					return invalida("%s: un telegram lleva solo \"texto\"", donde)
				}
			case "webhook":
				if h.Texto != "" || h.Bicho != "" || h.Expresion != "" || h.Modo != "" {
					return invalida("%s: un webhook lleva solo \"url\"", donde)
				}
				if err := w.ValidarURL(h.URL); err != nil {
					return invalida("%s: %v", donde, err)
				}
			case "robot":
				if h.Texto != "" || h.URL != "" || h.Modo != "" {
					return invalida("%s: un robot lleva \"bicho\" y \"expresion\"", donde)
				}
				id := strings.ToUpper(h.Bicho)
				if !protocolo.IDValido(id) || !esMio(id) {
					return invalida("%s: ese robot no es tuyo", donde)
				}
				if h.Expresion == "" || len(h.Expresion) > 40 || !utf8.ValidString(h.Expresion) {
					return invalida("%s: la expresión es de 1 a 40 bytes", donde)
				}
			case "decir":
				if strings.TrimSpace(h.Texto) == "" || len(h.Texto) > MaxTextoDecir || !utf8.ValidString(h.Texto) {
					return invalida("%s: el texto es de 1 a %d bytes (el robot muestra hasta 60)", donde, MaxTextoDecir)
				}
				if h.URL != "" || h.Bicho != "" || h.Modo != "" {
					return invalida("%s: decir lleva \"texto\" y, si querés, \"expresion\"", donde)
				}
				if h.Expresion != "" && !expresionValida(h.Expresion) {
					return invalida("%s: la expresión es de 1 a 40 bytes", donde)
				}
			case "cara":
				if h.Texto != "" || h.URL != "" || h.Bicho != "" || h.Modo != "" {
					return invalida("%s: cara lleva solo \"expresion\"", donde)
				}
				if !expresionValida(h.Expresion) {
					return invalida("%s: la expresión es de 1 a 40 bytes", donde)
				}
			case "modo":
				if h.Texto != "" || h.URL != "" || h.Bicho != "" || h.Expresion != "" {
					return invalida("%s: modo lleva solo \"modo\"", donde)
				}
				if !modoValido(h.Modo) {
					return invalida("%s: el modo es %s", donde, strings.Join(protocolo.Modos, ", "))
				}
			case "despertar":
				if h.Texto != "" || h.URL != "" || h.Bicho != "" || h.Expresion != "" || h.Modo != "" {
					return invalida("%s: despertar no lleva nada más", donde)
				}
			case "jugar":
				if h.Texto != "" || h.URL != "" || h.Bicho != "" || h.Expresion != "" || h.Modo != "" {
					return invalida("%s: jugar lleva solo \"juego\"", donde)
				}
				if j, ok := protocolo.NombreJuego(h.Juego); !ok || j == "terminar" {
					return invalida("%s: el juego es %s", donde, strings.Join(protocolo.Juegos, ", "))
				}
			default:
				return invalida("%s: el tipo es %s", donde, strings.Join(TiposDeAccion, ", "))
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
		if !a.Activa || a.Cuando != d.evento || (d.id != "" && a.ID != d.id) {
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
	if !i.tope(ctx, "auto:"+bicho+"/"+a.ID, DisparoCada, DisparosPorDia, 24*time.Hour) {
		return nil, ErrLimite
	}
	ctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
	defer cancelar()
	c := i.contexto(ctx, bicho, evento, datos)
	if usaClima(a) && c.ubicacion != nil {
		if cl, err := i.clima.Actual(ctx, c.ubicacion.Lat, c.ubicacion.Lon); err == nil {
			c.ponerClima(&cl)
		}
	}
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
		case "decir":
			err = i.decirEnBicho(bicho, c.completar(h.Texto), h.Expresion)
		case "cara":
			expr := h.Expresion
			if expr == "auto" {
				expr = ""
			}
			err = i.mandarAlBicho(bicho, protocolo.Expresion(expr))
		case "modo":
			err = i.cambiarModo(ctx, usuario, bicho, h.Modo)
		case "despertar":
			m, _ := protocolo.Alarma("despertar")
			err = i.mandarAlBicho(bicho, m)
		case "jugar":
			var m []byte
			if m, err = protocolo.Jugar(h.Juego); err == nil {
				err = i.mandarAlBicho(bicho, m)
			}
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
// {expresion}, {evento}, {texto}, {hora}, {dueno}, {ciudad}, los del clima
// ({clima}, {temp}, {max}, {min}; solo si el robot tiene ubicación) y los de
// la v1.7: {juego} y {resultado} (cuando: juego) y {animo} (cuando: animo);
// y de la v1.9, {habito} y {racha} (cuando: habito).
type contexto struct {
	bichoID, robot, bateria, expresion, evento, texto, hora string
	dueno, ciudad, clima, temp, max, min                    string
	juego, resultado, animo                                 string
	habito, racha                                           string
	ubicacion                                               *clima.Ubicacion
	datos                                                   map[string]any
}

func (i *Integraciones) contexto(ctx context.Context, bicho, evento string, datos map[string]any) contexto {
	c := contexto{bichoID: bicho, robot: bicho, bateria: "?", evento: evento, datos: map[string]any{},
		hora: time.Now().In(nube.Zona()).Format("15:04"), clima: "sin datos del clima", temp: "?", max: "?", min: "?"}
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
	c.juego, _ = datos["juego"].(string)
	c.resultado, _ = datos["resultado"].(string)
	if v, ok := datos["valor"].(string); ok && evento == "animo" {
		c.animo = v
	}
	c.habito, _ = datos["habito"].(string)
	if v, ok := datos["racha"].(int); ok {
		c.racha = strconv.Itoa(v)
	}
	cfg, errCfg := i.alm.UltimaConfig(ctx, bicho)
	if errCfg == nil {
		d := leerDatosConfig(cfg.Texto)
		c.dueno = strings.TrimSpace(d.Dueno.Nombre)
		if d.Ubicacion != nil {
			c.ubicacion, c.ciudad = d.Ubicacion, d.Ubicacion.Ciudad
		}
	}
	if v, ok := datos["texto"].(string); ok {
		c.texto = v
	} else if k, ok := datos["indice"].(int); ok && evento == "recordatorio" && errCfg == nil {
		// El bicho dice cuál recordatorio mostró: el texto está en su config.
		if l := recordatoriosDe(cfg.Texto); k >= 0 && k < len(l) {
			c.texto = l[k].Texto
			c.datos["texto"] = l[k].Texto
			c.datos["hora"] = l[k].Hora
		}
	}
	return c
}

func (c contexto) completar(s string) string {
	return strings.NewReplacer("{robot}", c.robot, "{bateria}", c.bateria, "{expresion}", c.expresion,
		"{evento}", c.evento, "{texto}", c.texto, "{hora}", c.hora, "{dueno}", c.dueno, "{ciudad}", c.ciudad,
		"{clima}", c.clima, "{temp}", c.temp, "{max}", c.max, "{min}", c.min,
		"{juego}", c.juego, "{resultado}", c.resultado, "{animo}", c.animo,
		"{habito}", c.habito, "{racha}", c.racha).Replace(s)
}
