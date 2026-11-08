package servidor

import (
	"context"
	"crypto/subtle"
	"strings"

	"sintetica/api/internal/protocolo"
)

// El broker MQTT (embebido en la API, internal/broker) pregunta acá si deja
// conectar a un cliente y si puede publicar o suscribirse a un tópico.

// AutenticarMQTT: la API misma (el superusuario, que puede todo) o un bicho
// con su clave (usuario = su id = el client id).
func (s *Servidor) AutenticarMQTT(ctx context.Context, usuario, clave, clientID string) bool {
	if usuario == s.op.MQTTUsuario {
		return s.op.MQTTClave != "" && subtle.ConstantTimeCompare([]byte(clave), []byte(s.op.MQTTClave)) == 1
	}
	if !protocolo.IDValido(usuario) || clientID != usuario || clave == "" {
		return false
	}
	acc, err := s.alm.UsarCredencial(ctx, usuario, hash(clave))
	if err != nil || !acc.Ok {
		if err != nil {
			s.log.Warn("autenticación de un bicho", "bicho", usuario, "error", err)
		}
		return false
	}
	if acc.Reclamado {
		s.log.Info("bicho sumado a una cuenta", "bicho", usuario)
		s.nube.Reclamado(usuario, acc)
	}
	return true
}

// AutorizarMQTT: cada bicho solo publica en sus tópicos y solo se suscribe a
// sus comandos (lo que no está acá, no: el no_match = deny de EMQX).
func (s *Servidor) AutorizarMQTT(usuario, topico string, publicar bool) bool {
	if !protocolo.IDValido(usuario) {
		return false
	}
	resto, ok := strings.CutPrefix(topico, "bicho/"+usuario+"/")
	if !ok {
		return false
	}
	if publicar {
		return resto == "state" || resto == "resp" || resto == "conexion"
	}
	return resto == "cmd"
}
