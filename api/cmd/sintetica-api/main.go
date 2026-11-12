// La API de SintéticaOS. Se configura con variables de entorno (ver
// .env.example en la raíz del repo y docs/nube.md).
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/broker"
	"sintetica/api/internal/correo"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/firma"
	"sintetica/api/internal/integraciones"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/servidor"
)

func env(clave, porDefecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return porDefecto
}

func lista(s string) []string {
	var l []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			l = append(l, strings.ToLower(p))
		}
	}
	return l
}

// apagable: "no" apaga un puerto.
func apagable(dir string) string {
	if dir == "no" {
		return ""
	}
	return dir
}

func numero(log *slog.Logger, clave string, porDefecto int) int {
	v := os.Getenv(clave)
	if v == "" {
		return porDefecto
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Warn("número inválido: uso el de siempre", "variable", clave, "valor", v, "por_defecto", porDefecto)
		return porDefecto
	}
	return n
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// sintetica-api claves <dir> [<copia>]: genera el par de claves de firma si
	// no está (lo usa el servicio "claves" del docker compose) y deja una copia
	// legible solo por el usuario de la API (uid 10001).
	if len(os.Args) >= 3 && os.Args[1] == "claves" {
		err := firma.GenerarSiFalta(os.Args[2])
		if err == nil && len(os.Args) == 4 {
			err = firma.Copiar(os.Args[2], os.Args[3], 10001)
		}
		if err != nil {
			log.Error("claves", "error", err)
			os.Exit(1)
		}
		return
	}
	// sintetica-api salud: para el healthcheck de Docker.
	if len(os.Args) == 2 && os.Args[1] == "salud" {
		r, err := http.Get("http://127.0.0.1" + env("DIRECCION", ":8080") + "/salud")
		if err != nil || r.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	if err := correr(log); err != nil {
		log.Error("la API se detuvo", "error", err)
		os.Exit(1)
	}
}

func correr(log *slog.Logger) error {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	alm, err := almacen.Abrir(ctx, env("DATABASE_URL", "postgres://sintetica:sintetica@localhost:5432/sintetica?sslmode=disable"))
	if err != nil {
		return err
	}
	defer alm.Cerrar()
	log.Info("base lista")

	var firmador *firma.Firmador
	if ruta := env("CLAVE_FIRMA", "/claves/ota-privada.pem"); ruta != "" {
		if firmador, err = firma.Cargar(ruta); err != nil {
			log.Warn("sin clave de firma: no se pueden subir firmwares", "error", err)
			firmador = nil
		}
	}

	secreto := []byte(os.Getenv("SECRETO"))
	if len(secreto) < 16 {
		secreto = make([]byte, 32)
		rand.Read(secreto) //nolint:errcheck
		log.Warn("SECRETO vacío o corto: uso uno al azar (los links de descarga de OTA no sobreviven a un reinicio)")
	}
	desarrollo := env("DESARROLLO", "0") == "1"
	op := servidor.Opciones{
		WebURL:          env("WEB_URL", "http://localhost:5173"),
		MQTTURLPublica:  env("MQTT_URL_PUBLICA", "mqtt://localhost:1883"),
		MQTTURLInterna:  env("MQTT_URL_INTERNA", "mqtt://api:1883"),
		MQTTUsuario:     env("MQTT_USUARIO", "api"),
		MQTTClave:       os.Getenv("MQTT_CLAVE"),
		Secreto:         secreto,
		Admins:          lista(os.Getenv("ADMIN_EMAILS")),
		AdminsTelegram:  idsChat(os.Getenv("ADMIN_TELEGRAM")),
		Desarrollo:      desarrollo,
		BichosVirtuales: strings.Split(strings.ReplaceAll(os.Getenv("BICHOS_VIRTUALES"), " ", ""), ","),
		CookieSegura:    env("COOKIE_SEGURA", "0") == "1",
		URLPublica:      env("API_URL_PUBLICA", "http://localhost:8080"),
		MPToken:         strings.TrimSpace(os.Getenv("MP_ACCESS_TOKEN")), // la preventa del cubito (vacío, apagada)
	}
	if len(op.BichosVirtuales) == 1 && op.BichosVirtuales[0] == "" {
		op.BichosVirtuales = nil
	}

	hub := eventos.Nuevo()
	var srv *servidor.Servidor
	// El broker MQTT, en el mismo proceso (ver docs/nube.md, "El broker").
	// Abre los puertos ya, pero acepta clientes recién con Arrancar.
	superusuario := op.MQTTUsuario
	if op.MQTTClave == "" {
		superusuario = ""
	}
	brk, err := broker.Nuevo(broker.Opciones{
		TCP:         apagable(env("MQTT_TCP", ":1883")),
		TCPMax:      numero(log, "MQTT_TCP_MAX", 64),
		TLS:         apagable(os.Getenv("MQTT_TLS")),
		TLSMax:      numero(log, "MQTT_TLS_MAX", 2000),
		Certificado: os.Getenv("MQTT_TLS_CERT"), ClaveCert: os.Getenv("MQTT_TLS_CLAVE"),
		Autenticar: func(ctx context.Context, usuario, clave, clientID string) bool {
			return srv.AutenticarMQTT(ctx, usuario, clave, clientID)
		},
		Autorizar:    func(usuario, topico string, publicar bool) bool { return srv.AutorizarMQTT(usuario, topico, publicar) },
		Superusuario: superusuario,
		Log:          log,
	})
	if err != nil {
		return err
	}
	defer brk.Cerrar()
	n := nube.Nuevo(nube.Opciones{
		Broker:         brk,
		URLPublica:     env("API_URL_PUBLICA", "http://localhost:8080"),
		URLInterna:     env("API_URL_INTERNA", "http://api:8080"),
		FirmarDescarga: func(id int64, bicho string) string { return srv.FirmarDescarga(id, bicho) },
	}, alm, hub, log)
	// SMTP_SERVIDOR=ninguno (o vacío a propósito): sin mail; se entra con
	// Telegram. Sin la variable, Mailpit en local.
	var c *correo.Correo
	if smtp, hay := os.LookupEnv("SMTP_SERVIDOR"); !hay || (smtp != "" && smtp != "ninguno") {
		c = &correo.Correo{Servidor: env("SMTP_SERVIDOR", "localhost:1025"), Usuario: os.Getenv("SMTP_USUARIO"),
			Clave: os.Getenv("SMTP_CLAVE"), De: env("SMTP_DE", "Sintética Labs <hola@sinteticalabs.com>")}
	} else {
		log.Info("sin SMTP: no se entra por mail (solo con Telegram)")
	}
	// Las integraciones: Telegram (sin TELEGRAM_TOKEN queda apagado),
	// automatizaciones, webhooks y el clima. Ver docs/nube.md, "Integraciones".
	integ := integraciones.Nuevo(integraciones.Opciones{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		TelegramBot:   env("TELEGRAM_BOT", "sinteticalabs_bot"),
		ChatReservas:  idChat(os.Getenv("TELEGRAM_CHAT_RESERVAS")),
		TelegramURL:   os.Getenv("TELEGRAM_API_URL"),
		ClimaURL:      os.Getenv("CLIMA_API_URL"),
		GeocodingURL:  os.Getenv("GEOCODING_API_URL"),
		Desarrollo:    desarrollo,
		HostsPropios:  []string{op.WebURL, env("API_URL_PUBLICA", ""), env("API_URL_INTERNA", "")},
		// La v1.9: agenda, Spotify (sin las claves, apagado) y la charla con IA
		// (sin ANTHROPIC_API_KEY, apagada).
		V19: integraciones.OpcionesV19{
			Secreto: secreto, APIURL: op.URLPublica, WebURL: op.WebURL,
			SpotifyID: os.Getenv("SPOTIFY_CLIENT_ID"), SpotifySecreto: os.Getenv("SPOTIFY_CLIENT_SECRET"),
			// El mail (sin las claves de un proveedor, ese apagado).
			GoogleID: os.Getenv("GOOGLE_CLIENT_ID"), GoogleSecreto: os.Getenv("GOOGLE_CLIENT_SECRET"),
			MicrosoftID: os.Getenv("MICROSOFT_CLIENT_ID"), MicrosoftSecreto: os.Getenv("MICROSOFT_CLIENT_SECRET"),
			AnthropicClave: os.Getenv("ANTHROPIC_API_KEY"), AnthropicModelo: os.Getenv("CHARLA_MODELO"),
			CharlaURL: os.Getenv("CHARLA_URL"), CharlaClave: os.Getenv("CHARLA_CLAVE"),
			// La voz gratis (Edge TTS): prendida salvo EDGE_TTS=0.
			EdgeApagado: env("EDGE_TTS", "1") == "0", EdgeTope: numero(log, "EDGE_TTS_TOPE", integraciones.TopeEdgePorDefecto),
			EdgeTopePaquete: numero(log, "EDGE_TTS_TOPE_PAQUETE", integraciones.TopeEdgePaquete),
		},
	}, alm, n, hub, log)
	n.Escuchar(integ)
	// Leer en voz alta con el ElevenLabs de cada usuario (integraciones/voz.go).
	n.UsarVoz(integ)
	// Lo que le dicen al robot-celu por voz (bicho/{id}/charla).
	if err := n.AlPublicar("charla", integ.CharlaMQTT); err != nil {
		return fmt.Errorf("charla por MQTT: %w", err)
	}
	// Lo que el robot-celu sabe por su cuenta (bicho/{id}/presencia).
	if err := n.AlPublicar("presencia", integ.PresenciaMQTT); err != nil {
		return fmt.Errorf("presencia por MQTT: %w", err)
	}
	// El robot-celu pide una conversación en tiempo real (bicho/{id}/conversacion).
	if err := n.AlPublicar("conversacion", integ.ConversacionMQTT); err != nil {
		return fmt.Errorf("conversación por MQTT: %w", err)
	}
	// El robot pide su paquete de voz sin internet (bicho/{id}/voz/paquete).
	if err := n.AlPublicar("voz/paquete", integ.PaqueteVozMQTT); err != nil {
		return fmt.Errorf("paquete de voz por MQTT: %w", err)
	}
	srv = servidor.Nuevo(op, alm, n, integ, hub, c, firmador, log)

	publico := &http.Server{Addr: env("DIRECCION", ":8080"), Handler: srv.Rutas(), ReadHeaderTimeout: 10 * time.Second}
	errores := make(chan error, 1)
	lPublico, err := net.Listen("tcp", publico.Addr)
	if err != nil {
		return fmt.Errorf("puerto público: %w", err)
	}
	go func() { errores <- publico.Serve(lPublico) }()
	log.Info("API escuchando", "publica", publico.Addr, "desarrollo", desarrollo)

	go integ.Correr(ctx)

	// La nube se suscribe antes de que entre el primer bicho.
	if err := n.Conectar(); err != nil {
		return err
	}
	defer n.Cerrar() // antes que el broker: que los bichos que se van no cuenten como desconexiones
	if err := brk.Arrancar(); err != nil {
		return fmt.Errorf("broker: %w", err)
	}

	select {
	case <-ctx.Done():
	case err := <-errores:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("servidor HTTP: %w", err)
		}
	}
	log.Info("apagando")
	apagar, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	publico.Shutdown(apagar) //nolint:errcheck
	return nil
}

// idChat: el id de un chat de Telegram de una variable de entorno (0 si está vacía o mal).
// idsChat: "123, 456" → los ids (los que no son números, afuera).
func idsChat(s string) []int64 {
	var l []int64
	for _, x := range strings.Split(s, ",") {
		if n := idChat(x); n != 0 {
			l = append(l, n)
		}
	}
	return l
}

func idChat(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
