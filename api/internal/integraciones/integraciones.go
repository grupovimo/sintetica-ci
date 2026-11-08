// Package integraciones hace que los bichos sirvan fuera de su pantalla: el
// bot de Telegram (vincular el chat, hablarle al bicho, que el bicho avise), las
// automatizaciones por bicho ("cuando pasa X → hacer Y": un Telegram, un
// webhook firmado o algo en otro bicho propio) y los recordatorios que la
// nube manda por Telegram cuando el bicho no está conectado. Ver docs/nube.md,
// "Integraciones".
package integraciones

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/clima"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// Robots es lo que se usa de la nube (nube.Nube lo cumple).
type Robots interface {
	Dueno(id string) string
	EstadoDe(id string) (protocolo.Estado, bool)
	Conectado(id string) bool
	Enviar(id string, car byte, marco []byte) error
}

// Opciones salen del entorno (ver cmd/sintetica-api).
type Opciones struct {
	TelegramToken string // sin token, todo lo de Telegram queda apagado
	TelegramBot   string // el usuario del bot, sin "@" (sinteticalabs_bot)
	// El chat (de Nahuel) al que llegan las reservas de la landing; 0, no se avisan.
	ChatReservas   int64
	TelegramURL    string // la API de Telegram (los tests ponen un servidor falso)
	TelegramEspera int    // segundos de long polling (30)
	// Desarrollo deja que los webhooks vayan a direcciones internas.
	Desarrollo bool
	// HostsPropios son los del propio servicio (la API, la web): no se les
	// puede mandar un webhook.
	HostsPropios []string
	// El clima (Open-Meteo): vacías, las URLs de siempre (los tests ponen un
	// servidor falso).
	ClimaURL     string
	GeocodingURL string
	// Lo de la v1.9 (agenda, música y charla; ver v19.go).
	V19 OpcionesV19
}

// Integraciones tiene todo lo de este paquete.
type Integraciones struct {
	op     Opciones
	alm    *almacen.Almacen
	robots Robots
	hub    *eventos.Hub
	log    *slog.Logger

	tg      *Telegram
	webhook *ClienteWebhook
	limite  *limitador

	cola chan disparo

	cacheMu sync.Mutex
	cache   map[string]entradaCache

	pendMu     sync.Mutex
	pendientes map[int64]pendiente // lo que espera que el usuario elija un robot

	chats *limitador // pedidos por chat de Telegram

	clima         *clima.Cliente
	climaEst      estadoClima
	cambiarConfig CambiarConfig

	cuid estadoCuidado // lo de la v1.7 (cuidado.go)

	toques estadoToques  // los últimos toques, para devolverlos (amigos.go)
	v19    estadoV19     // lo de la v1.9 (v19.go)
	voz    estadoVoz     // ElevenLabs (voz.go)
	ia     estadoIA      // la clave de IA propia de cada usuario (ia.go)
	paq    estadoPaquete // el paquete de voz sin internet (paquete_voz.go)

	charlaLocal CharlaLocal // el motor de charla sin IA (nil: solo la IA)
}

type entradaCache struct {
	usuario string
	lista   []Automatizacion
	en      time.Time
}

type disparo struct {
	bicho, evento string
	id            string // si no está vacío, solo esa automatización (las de "hora")
	datos         map[string]any
}

// Nuevo arma las integraciones (no arranca nada hasta Correr).
func Nuevo(op Opciones, alm *almacen.Almacen, r Robots, hub *eventos.Hub, log *slog.Logger) *Integraciones {
	if op.TelegramBot == "" {
		op.TelegramBot = "sinteticalabs_bot"
	}
	op.TelegramBot = strings.TrimPrefix(op.TelegramBot, "@")
	if op.TelegramEspera <= 0 {
		op.TelegramEspera = 30
	}
	i := &Integraciones{
		op: op, alm: alm, robots: r, hub: hub, log: log,
		webhook:    NuevoClienteWebhook(op.Desarrollo, op.HostsPropios),
		limite:     nuevoLimitador(),
		cola:       make(chan disparo, 512),
		cache:      map[string]entradaCache{},
		pendientes: map[int64]pendiente{},
		chats:      nuevoLimitador(),
		clima:      clima.Nuevo(clima.Opciones{URLPronostico: op.ClimaURL, URLGeocoding: op.GeocodingURL}),
		climaEst:   estadoClima{mandado: map[string]climaMandado{}, conectado: map[string]bool{}, ya: make(chan string, 64)},
	}
	if op.TelegramToken != "" {
		i.tg = NuevoTelegram(op.TelegramURL, op.TelegramToken, op.TelegramEspera)
	}
	i.iniciarV19()
	return i
}

// TelegramActivo: hay token.
func (i *Integraciones) TelegramActivo() bool { return i.tg != nil }

// Bot es el usuario del bot (sin "@").
func (i *Integraciones) Bot() string { return i.op.TelegramBot }

// Correr atiende las automatizaciones, las rutinas (cuando: hora), el clima
// de los robots y, con token, el bot, los recordatorios y los cumples. Vuelve
// cuando se cancela el contexto.
func (i *Integraciones) Correr(ctx context.Context) {
	var wg sync.WaitGroup
	for k := 0; k < 8; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case d := <-i.cola:
					i.disparar(ctx, d)
				}
			}
		}()
	}
	wg.Add(2)
	go func() { defer wg.Done(); porMinuto(ctx, func(m time.Time) { i.revisarRutinas(ctx, m) }) }()
	go func() { defer wg.Done(); i.trabajadorClima(ctx) }()
	wg.Add(1)
	go func() { defer wg.Done(); i.correrV19(ctx) }()
	if i.tg != nil {
		wg.Add(2)
		go func() { defer wg.Done(); i.escucharTelegram(ctx) }()
		go func() { defer wg.Done(); i.recordatorios(ctx) }()
		i.log.Info("Telegram prendido", "bot", "@"+i.op.TelegramBot)
	} else {
		i.log.Info("Telegram apagado (sin TELEGRAM_TOKEN)")
	}
	wg.Wait()
}

// --- nube.Oyente ---

// Evento lo llama la nube cuando le pasa algo a un bicho (no bloquea).
func (i *Integraciones) Evento(bicho, evento string, datos map[string]any) {
	switch evento {
	case "conectado":
		i.ClimaYa(bicho)
		i.musicaYa(bicho)
		go func() { time.Sleep(3 * time.Second); i.avisarVoz(bicho) }() // cuando ya mandó su estado
	case "recordatorio", "despertador": // lo lee en voz alta (voz.go) y sigue
		go i.leerGloboPropio(bicho, evento, datos)
	case "juego":
		i.anotarJuego(bicho, datos)
	case "bicho-cerca":
		go i.saludarEntreRobots(bicho)
	case "duerme":
		go i.anotarDormida(bicho)
	case "devolver-toque": // v1.9 (amigos.go)
		go i.devolverToque(bicho)
		return
	case "llegaste", "te-fuiste": // v1.9: los avisa el robot (presencia.go)
		go i.presenciaDelRobot(bicho, evento)
		return
	case "habito": // v1.9: lo marcó con un toque (habitos.go)
		go i.habitoDelRobot(bicho, datos)
		return
	}
	i.encolar(bicho, evento, datos)
}

// encolar: si el bicho tiene automatizaciones para ese evento, las corre.
func (i *Integraciones) encolar(bicho, evento string, datos map[string]any) {
	if !i.tieneAutomatizacionPara(bicho, evento) {
		return
	}
	select {
	case i.cola <- disparo{bicho: bicho, evento: evento, datos: datos}:
	default:
		i.log.Warn("automatizaciones: cola llena, pierdo un evento", "bicho", bicho, "evento", evento)
	}
}

// QuiereRegistro: si el bicho tiene automatizaciones activas con eventos que
// salen del registro del motor de vida, o si su dueño tiene otro robot
// conectado (para que se saluden al verse: "bicho-cerca").
func (i *Integraciones) QuiereRegistro(bicho string) bool {
	for _, a := range i.automatizacionesDe(bicho) {
		if a.Activa && nube.CuandoDelRegistro(a.Cuando) {
			return true
		}
	}
	return i.tieneOtroConectado(bicho)
}

func (i *Integraciones) tieneAutomatizacionPara(bicho, evento string) bool {
	for _, a := range i.automatizacionesDe(bicho) {
		if a.Activa && a.Cuando == evento {
			return true
		}
	}
	return false
}

// automatizacionesDe usa un caché (se pide cada segundo desde el trabajador
// del bicho). Si el bicho cambió de dueño, se vuelve a leer.
func (i *Integraciones) automatizacionesDe(bicho string) []Automatizacion {
	usuario := i.robots.Dueno(bicho)
	if usuario == "" {
		return nil
	}
	i.cacheMu.Lock()
	e, ok := i.cache[bicho]
	i.cacheMu.Unlock()
	if ok && e.usuario == usuario && time.Since(e.en) < 10*time.Minute {
		return e.lista
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	crudo, err := i.alm.Automatizaciones(ctx, bicho)
	if err != nil {
		i.log.Warn("no pude leer las automatizaciones", "bicho", bicho, "error", err)
		return e.lista
	}
	var l []Automatizacion
	if err := json.Unmarshal(crudo, &l); err != nil {
		i.log.Warn("automatizaciones guardadas inválidas", "bicho", bicho, "error", err)
	}
	i.cacheMu.Lock()
	i.cache[bicho] = entradaCache{usuario: usuario, lista: l, en: time.Now()}
	i.cacheMu.Unlock()
	return l
}

// --- Para el servidor HTTP ---

// Automatizaciones devuelve la lista de un bicho y su historial.
func (i *Integraciones) Automatizaciones(ctx context.Context, bicho string) ([]Automatizacion, []almacen.Ejecucion, error) {
	crudo, err := i.alm.Automatizaciones(ctx, bicho)
	if err != nil {
		return nil, nil, err
	}
	l := []Automatizacion{}
	if err := json.Unmarshal(crudo, &l); err != nil {
		return nil, nil, err
	}
	h, err := i.alm.Historial(ctx, bicho)
	return l, h, err
}

// GuardarAutomatizaciones valida y reemplaza la lista entera.
func (i *Integraciones) GuardarAutomatizaciones(ctx context.Context, usuario, bicho string, l []Automatizacion) error {
	err := Validar(l, func(id string) bool {
		_, err := i.alm.BichoDe(ctx, id, usuario)
		return err == nil
	}, i.webhook)
	if err != nil {
		return err
	}
	if l == nil {
		l = []Automatizacion{}
	}
	b, _ := json.Marshal(l)
	if err := i.alm.GuardarAutomatizaciones(ctx, bicho, b); err != nil {
		return err
	}
	i.cacheMu.Lock()
	delete(i.cache, bicho)
	i.cacheMu.Unlock()
	return nil
}

// ErrLimite: la automatización corrió hace muy poco (o demasiadas veces hoy).
var ErrLimite = errors.New("esta automatización ya corrió hace muy poco: como mucho una vez cada 10 s y 200 por día")

// ErrNoExiste: no hay automatización con ese número o id.
var ErrNoExiste = errors.New("no hay una automatización con ese número")

// Probar corre una automatización ya, como si hubiera pasado su evento.
// `n` es su posición (desde 0) o su id.
func (i *Integraciones) Probar(ctx context.Context, usuario, bicho, n string) ([]Resultado, error) {
	l, _, err := i.Automatizaciones(ctx, bicho)
	if err != nil {
		return nil, err
	}
	var a *Automatizacion
	for k := range l {
		if l[k].ID == n || fmt.Sprint(k) == n {
			a = &l[k]
			break
		}
	}
	if a == nil {
		return nil, ErrNoExiste
	}
	datos := map[string]any{"prueba": true}
	if a.Valor != "" {
		datos["valor"] = a.Valor
	}
	if a.Cuando == "recordatorio" {
		datos["texto"] = "esto es una prueba"
	}
	if a.Cuando == "agenda" {
		datos["texto"], datos["titulo"], datos["minutos"] = "Evento de prueba", "Evento de prueba", 10
	}
	switch a.Cuando {
	case "hora":
		datos["hora"] = time.Now().In(nube.Zona()).Format("15:04")
	case "juego":
		datos["juego"], datos["resultado"] = "dado", "gano"
	case "animo":
		datos["valor"] = "bien"
	case "habito":
		datos["indice"], datos["habito"], datos["racha"] = 0, "agua", 3
	}
	return i.correr(ctx, usuario, bicho, *a, a.Cuando, datos)
}

// SecretoWebhook devuelve el secreto con el que se firman los webhooks del usuario.
func (i *Integraciones) SecretoWebhook(ctx context.Context, usuario string) (string, error) {
	return i.alm.SecretoWebhook(ctx, usuario, secretoNuevo())
}

// RotarSecretoWebhook le da uno nuevo.
func (i *Integraciones) RotarSecretoWebhook(ctx context.Context, usuario string) (string, error) {
	s := secretoNuevo()
	return s, i.alm.RotarSecretoWebhook(ctx, usuario, s)
}

func secretoNuevo() string { return "whsec_" + aleatorio(24) }

func aleatorio(n int) string {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashCodigo(c string) []byte { h := sha256.Sum256([]byte("telegram:" + c)); return h[:] }

// DuracionCodigo: cuánto vale el link para vincular Telegram.
const DuracionCodigo = 15 * time.Minute

// CrearVinculo arma el link de un solo uso para vincular Telegram.
func (i *Integraciones) CrearVinculo(ctx context.Context, usuario string) (enlace string, vence time.Time, err error) {
	codigo := aleatorio(18) // 24 caracteres [A-Za-z0-9_-]: lo que acepta "start"
	vence = time.Now().Add(DuracionCodigo).UTC().Truncate(time.Second)
	if err := i.alm.CrearCodigoTelegram(ctx, usuario, hashCodigo(codigo), vence); err != nil {
		return "", time.Time{}, err
	}
	return "https://t.me/" + i.op.TelegramBot + "?start=" + url.QueryEscape(codigo), vence, nil
}

// Vincular canjea un código (lo que hace "/start <código>").
func (i *Integraciones) Vincular(ctx context.Context, codigo string, chatID int64, nombre string) (string, error) {
	usuario, err := i.alm.VincularTelegram(ctx, hashCodigo(codigo), chatID, nombre)
	if err != nil {
		return "", err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "telegram", Datos: map[string]any{"vinculado": true, "nombre": nombre}})
	return usuario, nil
}

// Desvincular olvida el chat del usuario.
func (i *Integraciones) Desvincular(ctx context.Context, usuario string) error {
	if err := i.alm.DesvincularTelegram(ctx, usuario); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "telegram", Datos: map[string]any{"vinculado": false}})
	return nil
}

// --- Límite de disparos (en memoria) ---

type limitador struct {
	mu     sync.Mutex
	cuenta map[string][]time.Time
	ahora  func() time.Time
}

func nuevoLimitador() *limitador {
	return &limitador{cuenta: map[string][]time.Time{}, ahora: time.Now}
}

// permitir: como mucho una vez cada `cada` y `max` por `ventana`.
func (l *limitador) permitir(clave string, cada time.Duration, max int, ventana time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ahora := l.ahora()
	v := l.cuenta[clave][:0]
	for _, t := range l.cuenta[clave] {
		if ahora.Sub(t) < ventana {
			v = append(v, t)
		}
	}
	l.cuenta[clave] = v
	if len(v) >= max || (len(v) > 0 && ahora.Sub(v[len(v)-1]) < cada) {
		return false
	}
	l.cuenta[clave] = append(v, ahora)
	if len(l.cuenta) > 20000 {
		for k, ts := range l.cuenta {
			if len(ts) == 0 || ahora.Sub(ts[len(ts)-1]) > ventana {
				delete(l.cuenta, k)
			}
		}
	}
	return true
}
