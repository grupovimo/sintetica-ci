package nube

import (
	"errors"
	"strings"

	"sintetica/api/internal/protocolo"
)

// AlPublicar llama a f cada vez que un bicho publica en bicho/{id}/{sufijo}
// (por ahora, "charla": el robot-celu manda lo que le dijeron). f corre en la
// goroutine del broker: no puede bloquear. La ACL (servidor/mqtt.go) deja que
// cada bicho publique solo en el suyo; acá se revisa igual el id.
func (n *Nube) AlPublicar(sufijo string, f func(bicho string, datos []byte)) error {
	if n.op.Broker == nil {
		return errors.New("la nube no tiene broker")
	}
	return n.op.Broker.Suscribir("bicho/+/"+sufijo, func(topico string, datos []byte) {
		partes := strings.Split(topico, "/")
		if len(partes) != 3 || partes[0] != "bicho" || partes[2] != sufijo || !protocolo.IDValido(partes[1]) {
			return
		}
		f(partes[1], append([]byte(nil), datos...))
	})
}
