package servidor

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"sintetica/api/internal/protocolo"
)

// EMQX pregunta acá si deja conectar a un cliente (autenticación HTTP) y si
// puede publicar o suscribirse a un tópico (autorización HTTP). Este puerto
// solo se ve adentro de la red de Docker.

type pedidoEmqx struct {
	Usuario  string `json:"username"`
	Clave    string `json:"password"`
	ClientID string `json:"clientid"`
	Topico   string `json:"topic"`
	Accion   string `json:"action"`
}

func respuestaEmqx(w http.ResponseWriter, permitir, superusuario bool) {
	res := "deny"
	if permitir {
		res = "allow"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"result": res, "is_superuser": superusuario}) //nolint:errcheck
}

func (s *Servidor) emqxAutenticar(w http.ResponseWriter, r *http.Request) {
	var p pedidoEmqx
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); err != nil {
		respuestaEmqx(w, false, false)
		return
	}
	// La API misma.
	if p.Usuario == s.op.MQTTUsuario {
		ok := s.op.MQTTClave != "" && subtle.ConstantTimeCompare([]byte(p.Clave), []byte(s.op.MQTTClave)) == 1
		respuestaEmqx(w, ok, ok)
		return
	}
	// Un bicho: usuario = su id = el client id.
	if !protocolo.IDValido(p.Usuario) || p.ClientID != p.Usuario || p.Clave == "" {
		respuestaEmqx(w, false, false)
		return
	}
	acc, err := s.alm.UsarCredencial(r.Context(), p.Usuario, hash(p.Clave))
	if err != nil || !acc.Ok {
		if err != nil {
			s.log.Warn("autenticación de un bicho", "bicho", p.Usuario, "error", err)
		}
		respuestaEmqx(w, false, false)
		return
	}
	if acc.Reclamado {
		s.log.Info("bicho sumado a una cuenta", "bicho", p.Usuario)
		s.nube.Reclamado(p.Usuario, acc)
	}
	respuestaEmqx(w, true, false)
}

// Cada bicho solo publica en sus tópicos y solo se suscribe a sus comandos.
func (s *Servidor) emqxAutorizar(w http.ResponseWriter, r *http.Request) {
	var p pedidoEmqx
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); err != nil || !protocolo.IDValido(p.Usuario) {
		respuestaEmqx(w, false, false)
		return
	}
	base := "bicho/" + p.Usuario + "/"
	ok := false
	if strings.HasPrefix(p.Topico, base) {
		resto := strings.TrimPrefix(p.Topico, base)
		switch p.Accion {
		case "publish":
			ok = resto == "state" || resto == "resp" || resto == "conexion"
		case "subscribe":
			ok = resto == "cmd"
		}
	}
	respuestaEmqx(w, ok, false)
}
