// La API de SintéticaOS. Se configura con variables de entorno (ver
// .env.example en la raíz del repo y docs/nube.md).
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/correo"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/firma"
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
		MQTTURLInterna:  env("MQTT_URL_INTERNA", "mqtt://emqx:1883"),
		MQTTUsuario:     env("MQTT_USUARIO", "api"),
		MQTTClave:       os.Getenv("MQTT_CLAVE"),
		Secreto:         secreto,
		Admins:          lista(os.Getenv("ADMIN_EMAILS")),
		Desarrollo:      desarrollo,
		BichosVirtuales: strings.Split(strings.ReplaceAll(os.Getenv("BICHOS_VIRTUALES"), " ", ""), ","),
		CookieSegura:    env("COOKIE_SEGURA", "0") == "1",
	}
	if op.MQTTClave == "" {
		return errors.New("falta MQTT_CLAVE (la clave de la API en el broker)")
	}
	if len(op.BichosVirtuales) == 1 && op.BichosVirtuales[0] == "" {
		op.BichosVirtuales = nil
	}

	hub := eventos.Nuevo()
	var srv *servidor.Servidor
	n := nube.Nuevo(nube.Opciones{
		URL: env("MQTT_URL", "tcp://localhost:1883"), Usuario: op.MQTTUsuario, Clave: op.MQTTClave,
		EmqxURL: os.Getenv("EMQX_API_URL"), EmqxClave: os.Getenv("EMQX_API_CLAVE"),
		URLPublica:     env("API_URL_PUBLICA", "http://localhost:8080"),
		URLInterna:     env("API_URL_INTERNA", "http://api:8080"),
		FirmarDescarga: func(id int64, bicho string) string { return srv.FirmarDescarga(id, bicho) },
	}, alm, hub, log)
	c := &correo.Correo{Servidor: env("SMTP_SERVIDOR", "localhost:1025"), Usuario: os.Getenv("SMTP_USUARIO"),
		Clave: os.Getenv("SMTP_CLAVE"), De: env("SMTP_DE", "Sintética Labs <hola@sinteticalabs.com>")}
	srv = servidor.Nuevo(op, alm, n, hub, c, firmador, log)

	publico := &http.Server{Addr: env("DIRECCION", ":8080"), Handler: srv.Rutas(), ReadHeaderTimeout: 10 * time.Second}
	interno := &http.Server{Addr: env("DIRECCION_INTERNA", ":8081"), Handler: srv.RutasInternas(), ReadHeaderTimeout: 5 * time.Second}
	errores := make(chan error, 2)
	// Primero el puerto interno: EMQX lo necesita para dejar entrar a la API misma.
	go func() { errores <- interno.ListenAndServe() }()
	go func() { errores <- publico.ListenAndServe() }()
	log.Info("API escuchando", "publica", publico.Addr, "interna", interno.Addr, "desarrollo", desarrollo)

	if err := n.Conectar(); err != nil {
		log.Warn("todavía no me conecté al broker (sigo intentando)", "error", err)
	}
	defer n.Cerrar()

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
	interno.Shutdown(apagar) //nolint:errcheck
	return nil
}
