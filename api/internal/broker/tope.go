package broker

import (
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// tope limita las conexiones abiertas de un puerto (el max_connections de
// cada listener de EMQX): pasado el tope, la conexión nueva se cierra en el
// acto y el puerto sigue aceptando. Además, el que no manda su CONNECT en 15 s
// se corta (el idle_timeout de EMQX): Mochi pone el plazo del keepalive recién
// después del CONNECT.
type tope struct {
	net.Listener
	max      int64
	abiertas atomic.Int64
	nombre   string
	log      *slog.Logger
	avisado  atomic.Int64 // último aviso (unix), para no llenar el log
}

func conTope(l net.Listener, max int, nombre string, log *slog.Logger) net.Listener {
	return &tope{Listener: l, max: int64(max), nombre: nombre, log: log}
}

func (t *tope) Accept() (net.Conn, error) {
	for {
		c, err := t.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if t.max > 0 && t.abiertas.Add(1) > t.max {
			t.abiertas.Add(-1)
			c.Close()
			if ahora := time.Now().Unix(); t.avisado.Swap(ahora) != ahora {
				t.log.Warn("demasiadas conexiones: rechazo", "puerto", t.nombre, "tope", t.max)
			}
			continue
		}
		if t.max <= 0 {
			t.abiertas.Add(1)
		}
		c.SetDeadline(time.Now().Add(plazoConnect)) //nolint:errcheck
		return &conexionContada{Conn: c, t: t}, nil
	}
}

const plazoConnect = 15 * time.Second

type conexionContada struct {
	net.Conn
	t   *tope
	una sync.Once
}

func (c *conexionContada) Close() error {
	c.una.Do(func() { c.t.abiertas.Add(-1) })
	return c.Conn.Close()
}
