package broker

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// certificado es el par del puerto TLS. Se relee solo cuando cambia (el
// timer sintetica-cert lo renueva con lego en /opt/sintetica/certs): en cada
// handshake se mira la fecha y el tamaño de los dos archivos, y si cambiaron
// se vuelve a cargar. Si el nuevo está roto (por ejemplo, a medio escribir),
// se sigue con el anterior y se reintenta en el próximo handshake.
type certificado struct {
	cert, clave string
	log         *slog.Logger

	mu     sync.Mutex
	actual *tls.Certificate
	marca  string
}

func nuevoCertificado(cert, clave string, log *slog.Logger) (*certificado, error) {
	if cert == "" || clave == "" {
		return nil, fmt.Errorf("broker: el puerto TLS necesita el certificado y la clave (MQTT_TLS_CERT y MQTT_TLS_CLAVE)")
	}
	c := &certificado{cert: cert, clave: clave, log: log}
	if _, err := c.obtener(nil); err != nil {
		return nil, err
	}
	return c, nil
}

func marcaDe(rutas ...string) (string, error) {
	m := ""
	for _, r := range rutas {
		st, err := os.Stat(r)
		if err != nil {
			return "", err
		}
		m += fmt.Sprintf("%d:%d;", st.ModTime().UnixNano(), st.Size())
	}
	return m, nil
}

func (c *certificado) obtener(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	marca, err := marcaDe(c.cert, c.clave)
	if err == nil && marca == c.marca && c.actual != nil {
		return c.actual, nil
	}
	if err == nil {
		var par tls.Certificate
		par, err = tls.LoadX509KeyPair(c.cert, c.clave)
		if err == nil {
			hasta := ""
			if par.Leaf != nil {
				hasta = par.Leaf.NotAfter.Format(time.DateOnly)
			}
			if c.actual != nil {
				c.log.Info("certificado de MQTT renovado", "vence", hasta)
			}
			c.actual, c.marca = &par, marca
			return c.actual, nil
		}
	}
	if c.actual != nil {
		c.log.Warn("no pude releer el certificado de MQTT: sigo con el anterior", "error", err)
		return c.actual, nil
	}
	return nil, fmt.Errorf("broker: certificado de MQTT: %w", err)
}
