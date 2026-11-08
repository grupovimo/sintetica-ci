// Package broker es el broker MQTT de SintéticaOS, embebido en la API (Mochi
// MQTT). Los bichos se conectan por tcp (:1883, en desarrollo) o por TLS
// (:8883, en producción); la API publica y se suscribe en el mismo proceso,
// sin red. La autenticación y la autorización son ganchos que llaman a la
// lógica de la API (ver ganchos.go).
package broker

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Opciones del broker.
type Opciones struct {
	// TCP es la dirección del puerto sin TLS (":1883"; "" = sin él) y TCPMax
	// el tope de conexiones en ese puerto.
	TCP    string
	TCPMax int
	// TLS es la dirección del puerto con TLS (":8883"; "" = sin él), TLSMax
	// su tope de conexiones, y Certificado y ClaveCert los PEM (se releen
	// solos cuando cambian: ver certificado.go).
	TLS                    string
	TLSMax                 int
	Certificado, ClaveCert string

	// Autenticar dice si deja entrar a un cliente. Superusuario es el usuario
	// que puede todo (el de la API, para conectarse de afuera a mirar).
	Autenticar   func(ctx context.Context, usuario, clave, clientID string) bool
	Superusuario string
	// Autorizar dice si un cliente que no es superusuario puede publicar en un
	// tópico (publicar = true) o suscribirse a un filtro / recibir de un tópico.
	Autorizar func(usuario, topico string, publicar bool) bool

	Log *slog.Logger
}

// Lo mismo que tenía EMQX.
const (
	tamanoMaximo   = 1 << 20 // max_packet_size = 1MB
	expiraSesion   = 2 * 60 * 60
	escriturasCola = 1024 // mensajes pendientes por cliente (una config entera son ~110)
	// El request_timeout de la autenticación por HTTP de EMQX.
	tiempoAutenticar = 5 * time.Second
)

// Broker es el broker MQTT.
type Broker struct {
	srv      *mqtt.Server
	ganchos  *ganchos
	log      *slog.Logger
	cerrarMu sync.Once
	// Las direcciones reales de los puertos (para las pruebas, con ":0").
	DireccionTCP, DireccionTLS string
}

// Nuevo arma el broker y abre los puertos (todavía no acepta clientes: eso
// es Arrancar, después de suscribir a la API).
func Nuevo(op Opciones) (*Broker, error) {
	if op.Autenticar == nil || op.Autorizar == nil {
		return nil, errors.New("broker: faltan Autenticar y Autorizar")
	}
	if op.Log == nil {
		op.Log = slog.Default()
	}
	log := op.Log.With("modulo", "broker")
	capac := mqtt.NewDefaultServerCapabilities()
	capac.MaximumPacketSize = tamanoMaximo
	capac.MaximumSessionExpiryInterval = expiraSesion
	capac.MaximumClientWritesPending = escriturasCola
	capac.MaximumQos = 2
	srv := mqtt.New(&mqtt.Options{
		Capabilities: capac,
		InlineClient: true,
		// Mochi cuenta cada conexión en Info: con avisos alcanza.
		Logger: slog.New(soloAvisos{log.Handler()}),
		// Los $SYS se refrescan cada tanto; nadie más que el superusuario los ve.
		SysTopicResendInterval: 60,
	})
	b := &Broker{srv: srv, log: log}
	b.ganchos = &ganchos{op: op, srv: srv, log: log, ultimo: map[string]*mqtt.Client{}}
	if err := srv.AddHook(b.ganchos, nil); err != nil {
		return nil, err
	}

	var abiertos []net.Listener
	fallar := func(err error) (*Broker, error) {
		for _, l := range abiertos {
			l.Close()
		}
		return nil, err
	}
	if op.TCP != "" {
		l, err := net.Listen("tcp", op.TCP)
		if err != nil {
			return fallar(fmt.Errorf("broker: puerto tcp: %w", err))
		}
		abiertos = append(abiertos, l)
		b.DireccionTCP = l.Addr().String()
		if err := srv.AddListener(listeners.NewNet("tcp", conTope(l, op.TCPMax, "tcp", log))); err != nil {
			return fallar(err)
		}
	}
	if op.TLS != "" {
		cert, err := nuevoCertificado(op.Certificado, op.ClaveCert, log)
		if err != nil {
			return fallar(err)
		}
		l, err := net.Listen("tcp", op.TLS)
		if err != nil {
			return fallar(fmt.Errorf("broker: puerto tls: %w", err))
		}
		abiertos = append(abiertos, l)
		b.DireccionTLS = l.Addr().String()
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: cert.obtener}
		if err := srv.AddListener(listeners.NewNet("tls", tls.NewListener(conTope(l, op.TLSMax, "tls", log), cfg))); err != nil {
			return fallar(err)
		}
	}
	return b, nil
}

// Arrancar empieza a aceptar clientes.
func (b *Broker) Arrancar() error {
	if err := b.srv.Serve(); err != nil {
		return err
	}
	b.log.Info("broker MQTT escuchando", "tcp", b.DireccionTCP, "tls", b.DireccionTLS)
	return nil
}

// Cerrar echa a todos y cierra los puertos.
func (b *Broker) Cerrar() {
	b.cerrarMu.Do(func() {
		b.ganchos.cerrando.Store(true)
		b.srv.Close() //nolint:errcheck
	})
}

// Publicar publica desde la API (sin pasar por la autorización).
func (b *Broker) Publicar(topico string, datos []byte, retener bool, qos byte) error {
	return b.srv.Publish(topico, datos, retener, qos)
}

// Suscribir suscribe a la API a un filtro. f se llama en la goroutine del
// que publicó: no puede bloquear.
func (b *Broker) Suscribir(filtro string, f func(topico string, datos []byte)) error {
	return b.srv.Subscribe(filtro, 1, func(_ *mqtt.Client, _ packets.Subscription, pk packets.Packet) {
		f(pk.TopicName, pk.Payload)
	})
}

// Expulsar desconecta a un cliente (cuando al bicho se le revoca la clave).
// Dice si estaba conectado.
func (b *Broker) Expulsar(clientID string) bool {
	if clientID == mqtt.InlineClientId {
		return false
	}
	cl, ok := b.srv.Clients.Get(clientID)
	if !ok || cl.Closed() {
		return false
	}
	b.srv.DisconnectClient(cl, packets.ErrAdministrativeAction) //nolint:errcheck
	return true
}

// Conectado dice si hay un cliente conectado con ese id.
func (b *Broker) Conectado(clientID string) bool {
	if clientID == mqtt.InlineClientId {
		return false
	}
	cl, ok := b.srv.Clients.Get(clientID)
	return ok && !cl.Closed()
}

// Conectados cuenta los clientes conectados (sin la API).
func (b *Broker) Conectados() int64 {
	return b.srv.Info.Clone().ClientsConnected
}
