package nube

import (
	"errors"
	"strings"

	"sintetica/api/internal/protocolo"
)

// AlPublicar llama a f cada vez que un bicho publica en bicho/{id}/{sufijo}
// ("charla": el robot-celu manda lo que le dijeron; el sufijo puede tener
// varias partes, como "voz/paquete"). f corre en la
// goroutine del broker: no puede bloquear. La ACL (servidor/mqtt.go) deja que
// cada bicho publique solo en el suyo; acá se revisa igual el id.
func (n *Nube) AlPublicar(sufijo string, f func(bicho string, datos []byte)) error {
	if n.op.Broker == nil {
		return errors.New("la nube no tiene broker")
	}
	return n.op.Broker.Suscribir("bicho/+/"+sufijo, func(topico string, datos []byte) {
		resto, ok := strings.CutPrefix(topico, "bicho/")
		id, ok2 := strings.CutSuffix(resto, "/"+sufijo)
		if !ok || !ok2 || !protocolo.IDValido(id) {
			return
		}
		f(id, append([]byte(nil), datos...))
	})
}

// PublicarAudio manda un paquete a bicho/{id}/audio (QoS 1; ver
// protocolo/voz.go). Solo si está conectado.
func (n *Nube) PublicarAudio(id string, paquete []byte) error {
	return n.PublicarEn(id, "audio", paquete)
}

// PublicarEn manda algo a bicho/{id}/{sufijo} (QoS 1), si está conectado:
// "audio", "conversacion/url" y "voz/paquete/url" (la ACL deja que cada bicho se suscriba a los
// suyos).
func (n *Nube) PublicarEn(id, sufijo string, datos []byte) error {
	if !n.Conectado(id) {
		return ErrDesconectado
	}
	if !n.listo.Load() {
		return errors.New("la API no está conectada al broker")
	}
	return n.op.Broker.Publicar("bicho/"+id+"/"+sufijo, datos, false, 1)
}
