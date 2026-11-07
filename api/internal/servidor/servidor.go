// Package servidor es la API HTTP de SintéticaOS: cuentas con link mágico,
// bichos (config, imágenes, comandos, estado en vivo), la biblioteca de caras
// y animaciones, los firmwares, y los endpoints internos que usa EMQX.
package servidor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/correo"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/firma"
	"sintetica/api/internal/nube"
)

// Opciones de la API (salen del entorno: ver cmd/sintetica-api).
type Opciones struct {
	WebURL          string // para los links mágicos
	MQTTURLPublica  string // la que se les da a los bichos
	MQTTURLInterna  string // la que se le da al bicho virtual (adentro de Docker)
	MQTTUsuario     string // el de la API en el broker
	MQTTClave       string
	Secreto         []byte   // firma los links de descarga de las OTA
	Admins          []string // emails que pueden subir firmwares
	Desarrollo      bool     // habilita sumar bichos virtuales y muestra los links en el log
	BichosVirtuales []string // URLs de los bichos virtuales (desarrollo)
	CookieSegura    bool
}

// Servidor tiene todo lo que usan los handlers.
type Servidor struct {
	op       Opciones
	alm      *almacen.Almacen
	nube     *nube.Nube
	relevo   *nube.Relevo
	hub      *eventos.Hub
	correo   *correo.Correo
	firmador *firma.Firmador
	log      *slog.Logger
	limites  *limitador
}

// Nuevo arma el servidor.
func Nuevo(op Opciones, alm *almacen.Almacen, n *nube.Nube, hub *eventos.Hub, c *correo.Correo, f *firma.Firmador, log *slog.Logger) *Servidor {
	return &Servidor{op: op, alm: alm, nube: n, relevo: nube.NuevoRelevo(n), hub: hub, correo: c, firmador: f, log: log, limites: nuevoLimitador()}
}

// Rutas es la API pública.
func (s *Servidor) Rutas() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /salud", s.salud)
	m.HandleFunc("GET /v1/formato", s.formato)

	m.HandleFunc("POST /v1/sesion/enlace", s.pedirEnlace)
	m.HandleFunc("POST /v1/sesion/canjear", s.canjear)
	m.HandleFunc("GET /v1/sesion", s.conSesion(s.yo))
	m.HandleFunc("PATCH /v1/sesion", s.conSesion(s.cambiarNombre))
	m.HandleFunc("DELETE /v1/sesion", s.conSesion(s.salir))

	m.HandleFunc("GET /v1/eventos", s.conSesion(s.eventos))
	m.HandleFunc("GET /v1/bichos", s.conSesion(s.listarBichos))
	m.HandleFunc("POST /v1/bichos/reclamar", s.conSesion(s.reclamar))
	m.HandleFunc("POST /v1/bichos/reclamar-virtual", s.conSesion(s.reclamarVirtual))
	m.HandleFunc("GET /v1/bichos/virtuales", s.conSesion(s.listarVirtuales))
	m.HandleFunc("GET /v1/bichos/{id}", s.conBicho(s.verBicho))
	m.HandleFunc("PATCH /v1/bichos/{id}", s.conBicho(s.cambiarBicho))
	m.HandleFunc("DELETE /v1/bichos/{id}", s.conBicho(s.quitarBicho))
	m.HandleFunc("GET /v1/bichos/{id}/reclamo", s.conSesion(s.esperarReclamo))
	m.HandleFunc("GET /v1/bichos/{id}/config", s.conBicho(s.verConfig))
	m.HandleFunc("PUT /v1/bichos/{id}/config", s.conBicho(s.guardarConfig))
	m.HandleFunc("GET /v1/bichos/{id}/config/historial", s.conBicho(s.historialConfig))
	m.HandleFunc("POST /v1/bichos/{id}/config/restaurar", s.conBicho(s.restaurarConfig))
	m.HandleFunc("GET /v1/bichos/{id}/imagenes", s.conBicho(s.listarImagenes))
	m.HandleFunc("GET /v1/bichos/{id}/imagenes/{espacio}", s.conBicho(s.verImagen))
	m.HandleFunc("PUT /v1/bichos/{id}/imagenes/{espacio}", s.conBicho(s.subirImagen))
	m.HandleFunc("DELETE /v1/bichos/{id}/imagenes/{espacio}", s.conBicho(s.borrarImagen))
	m.HandleFunc("POST /v1/bichos/{id}/comando", s.conBicho(s.comando))
	m.HandleFunc("POST /v1/bichos/{id}/marcos", s.conBicho(s.marcos))
	m.HandleFunc("GET /v1/bichos/{id}/eventos", s.conBicho(s.eventosBicho))
	m.HandleFunc("POST /v1/bichos/{id}/actualizar", s.conBicho(s.actualizar))
	m.HandleFunc("GET /v1/bichos/{id}/actualizaciones", s.conBicho(s.listarActualizaciones))

	m.HandleFunc("GET /v1/biblioteca", s.conSesion(s.listarBiblioteca))
	m.HandleFunc("POST /v1/biblioteca", s.conSesion(s.guardarPieza))
	m.HandleFunc("DELETE /v1/biblioteca/{id}", s.conSesion(s.borrarPieza))

	m.HandleFunc("GET /v1/firmwares", s.conSesion(s.listarFirmwares))
	m.HandleFunc("POST /v1/firmwares", s.conSesion(s.subirFirmware))
	m.HandleFunc("DELETE /v1/firmwares/{id}", s.conSesion(s.borrarFirmware))
	m.HandleFunc("GET /v1/firmwares/{id}/manifiesto", s.conSesion(s.manifiesto))
	m.HandleFunc("GET /v1/firmwares/{id}/binario", s.binario) // con sesión o con el token del link
	return s.envolver(m)
}

// RutasInternas son las que llama EMQX (no se publican fuera de Docker).
func (s *Servidor) RutasInternas() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /emqx/autenticar", s.emqxAutenticar)
	m.HandleFunc("POST /emqx/autorizar", s.emqxAutorizar)
	m.HandleFunc("GET /salud", s.salud)
	return s.envolver(m)
}

// --- Middleware y ayudas ---

type escritorConEstado struct {
	http.ResponseWriter
	estado int
}

func (e *escritorConEstado) WriteHeader(c int) { e.estado = c; e.ResponseWriter.WriteHeader(c) }
func (e *escritorConEstado) Flush() {
	if f, ok := e.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Servidor) envolver(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		ew := &escritorConEstado{ResponseWriter: w, estado: 200}
		defer func() {
			if p := recover(); p != nil {
				s.log.Error("pánico", "error", p, "pila", string(debug.Stack()))
				http.Error(ew, `{"error":"error interno"}`, http.StatusInternalServerError)
			}
			if r.URL.Path != "/salud" && !strings.HasPrefix(r.URL.Path, "/emqx/") {
				s.log.Info("http", "metodo", r.Method, "ruta", r.URL.Path, "estado", ew.estado, "ms", time.Since(inicio).Milliseconds())
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(ew, r)
	})
}

func responder(w http.ResponseWriter, estado int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(estado)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// Error es la respuesta de error: {"error": "...", "detalle": "..."}.
type Error struct {
	Error   string `json:"error"`
	Detalle string `json:"detalle,omitempty"`
}

func fallar(w http.ResponseWriter, estado int, msg string, detalle ...string) {
	e := Error{Error: msg}
	if len(detalle) > 0 {
		e.Detalle = detalle[0]
	}
	responder(w, estado, e)
}

func (s *Servidor) interno(w http.ResponseWriter, err error) {
	s.log.Error("error interno", "error", err)
	fallar(w, http.StatusInternalServerError, "algo salió mal de nuestro lado")
}

func leerJSON(w http.ResponseWriter, r *http.Request, max int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fallar(w, http.StatusBadRequest, "el pedido no es JSON válido", err.Error())
		return false
	}
	return true
}

func ipDe(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func (s *Servidor) salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancelar := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancelar()
	db := s.alm.Ping(ctx) == nil
	broker := s.nube.BrokerConectado()
	estado := http.StatusOK
	if !db || !broker {
		estado = http.StatusServiceUnavailable
	}
	responder(w, estado, map[string]any{"base": db, "broker": broker, "clientes": s.hub.Cantidad()})
}

// --- Tokens de descarga de OTA (HMAC, una hora) ---

// FirmarDescarga arma el token del link de descarga de un firmware para un bicho.
func (s *Servidor) FirmarDescarga(firmwareID int64, bicho string) string {
	vence := time.Now().Add(time.Hour).Unix()
	msg := fmt.Sprintf("%d.%s.%d", firmwareID, bicho, vence)
	mac := hmac.New(sha256.New, s.op.Secreto)
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s.%x", msg, mac.Sum(nil))))
}

func (s *Servidor) verificarDescarga(firmwareID int64, token string) bool {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	partes := strings.Split(string(b), ".")
	if len(partes) != 4 || partes[0] != strconv.FormatInt(firmwareID, 10) {
		return false
	}
	vence, err := strconv.ParseInt(partes[2], 10, 64)
	if err != nil || time.Now().Unix() > vence {
		return false
	}
	mac := hmac.New(sha256.New, s.op.Secreto)
	mac.Write([]byte(strings.Join(partes[:3], ".")))
	return hmac.Equal([]byte(fmt.Sprintf("%x", mac.Sum(nil))), []byte(partes[3]))
}

// --- Límite de pedidos (en memoria) ---

type limitador struct {
	mu     sync.Mutex
	cuenta map[string][]time.Time
}

func nuevoLimitador() *limitador { return &limitador{cuenta: map[string][]time.Time{}} }

// permitir: como mucho `max` veces por `ventana` para esa clave.
func (l *limitador) permitir(clave string, max int, ventana time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ahora := time.Now()
	v := l.cuenta[clave][:0]
	for _, t := range l.cuenta[clave] {
		if ahora.Sub(t) < ventana {
			v = append(v, t)
		}
	}
	if len(v) >= max {
		l.cuenta[clave] = v
		return false
	}
	l.cuenta[clave] = append(v, ahora)
	if len(l.cuenta) > 10000 {
		for k, ts := range l.cuenta {
			if len(ts) == 0 || ahora.Sub(ts[len(ts)-1]) > ventana {
				delete(l.cuenta, k)
			}
		}
	}
	return true
}

var errNoEsTuyo = errors.New("ese bicho no está en tu cuenta")
