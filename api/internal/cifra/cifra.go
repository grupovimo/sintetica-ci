// Package cifra guarda secretos de terceros (la dirección secreta de un
// calendario, el refresh token de Spotify) cifrados en la base: AES-256-GCM
// con una clave que sale de SECRETO. Si SECRETO cambia, lo guardado no se
// puede leer y hay que volver a conectar (no se pierde nada más).
package cifra

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// Cifrador cifra y descifra con una clave fija.
type Cifrador struct{ aead cipher.AEAD }

// ErrIlegible: no se cifró con esta clave (o está roto).
var ErrIlegible = errors.New("no lo puedo descifrar: volvé a conectarlo")

// Nuevo deriva la clave de `secreto` (y de `para`, así cada uso tiene la suya).
func Nuevo(secreto []byte, para string) *Cifrador {
	k := sha256.Sum256(append([]byte("sintetica/cifra/v1/"+para+"/"), secreto...))
	b, _ := aes.NewCipher(k[:])
	a, _ := cipher.NewGCM(b)
	return &Cifrador{aead: a}
}

// Cifrar: nonce + texto cifrado.
func (c *Cifrador) Cifrar(texto string) []byte {
	n := make([]byte, c.aead.NonceSize())
	rand.Read(n) //nolint:errcheck
	return c.aead.Seal(n, n, []byte(texto), nil)
}

// Descifrar lo que hizo Cifrar.
func (c *Cifrador) Descifrar(b []byte) (string, error) {
	ns := c.aead.NonceSize()
	if len(b) < ns {
		return "", ErrIlegible
	}
	t, err := c.aead.Open(nil, b[:ns], b[ns:], nil)
	if err != nil {
		return "", ErrIlegible
	}
	return string(t), nil
}
