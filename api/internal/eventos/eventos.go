// Package eventos reparte lo que pasa con los bichos a quien esté mirando
// (las páginas de la web y las apps, por SSE).
package eventos

import (
	"sync"
)

// Evento es un mensaje para los clientes. Tipos: "estado", "conexion",
// "marco", "sincro", "config", "imagenes", "ota", "bicho".
type Evento struct {
	Tipo  string `json:"tipo"`
	Bicho string `json:"bicho,omitempty"`
	Datos any    `json:"datos,omitempty"`
}

// Suscripcion recibe los eventos de un usuario (de un bicho, si Bicho no es "").
type Suscripcion struct {
	C       chan Evento
	usuario string
	bicho   string
}

// Hub reparte eventos. Si un cliente no lee, se le pierden eventos (nunca
// frena al que publica).
type Hub struct {
	mu   sync.RWMutex
	subs map[*Suscripcion]struct{}
}

// Nuevo crea un hub.
func Nuevo() *Hub { return &Hub{subs: map[*Suscripcion]struct{}{}} }

// Suscribir empieza a recibir.
func (h *Hub) Suscribir(usuario, bicho string) *Suscripcion {
	s := &Suscripcion{C: make(chan Evento, 64), usuario: usuario, bicho: bicho}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Desuscribir deja de recibir.
func (h *Hub) Desuscribir(s *Suscripcion) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// Publicar manda un evento a las suscripciones de ese usuario.
func (h *Hub) Publicar(usuario string, e Evento) {
	if usuario == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.usuario != usuario || (s.bicho != "" && e.Bicho != "" && s.bicho != e.Bicho) {
			continue
		}
		select {
		case s.C <- e:
		default:
		}
	}
}

// Cantidad de suscripciones (para los tests y /salud).
func (h *Hub) Cantidad() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
