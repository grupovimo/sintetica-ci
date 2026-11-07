// Package firma firma los firmwares: ECDSA P-256 sobre el SHA-256 del texto
// del manifiesto (protocolo.Manifiesto.Mensaje), la firma en r || s.
package firma

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"sintetica/api/internal/protocolo"
)

// Firmador tiene la clave privada.
type Firmador struct {
	clave *ecdsa.PrivateKey
}

// Cargar lee una clave privada PEM (PKCS#8 o EC).
func Cargar(ruta string) (*Firmador, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("no pude leer la clave de firma (%s): %w; generala con npm run claves", ruta, err)
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("la clave de firma no es PEM")
	}
	var k any
	if k, err = x509.ParsePKCS8PrivateKey(blk.Bytes); err != nil {
		if k, err = x509.ParseECPrivateKey(blk.Bytes); err != nil {
			return nil, fmt.Errorf("clave de firma: %w", err)
		}
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, errors.New("la clave de firma tiene que ser ECDSA P-256")
	}
	return &Firmador{clave: ec}, nil
}

// Nuevo genera una clave (para los tests).
func Nuevo() *Firmador {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return &Firmador{clave: k}
}

// GenerarSiFalta crea dir/ota-privada.pem y dir/ota-publica.pem si no están.
func GenerarSiFalta(dir string) error {
	privada, publica := filepath.Join(dir, "ota-privada.pem"), filepath.Join(dir, "ota-publica.pem")
	if _, err := os.Stat(privada); err == nil {
		f, err := Cargar(privada)
		if err != nil {
			return err
		}
		if _, err := os.Stat(publica); err != nil {
			return os.WriteFile(publica, f.PublicaPEM(), 0o644)
		}
		fmt.Println("las claves de firma ya estaban en", dir)
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f := Nuevo()
	der, err := x509.MarshalPKCS8PrivateKey(f.clave)
	if err != nil {
		return err
	}
	if err := os.WriteFile(privada, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(publica, f.PublicaPEM(), 0o644); err != nil {
		return err
	}
	fmt.Println("generé las claves de firma en", dir, "(compilá el firmware de nuevo para que confíe en ellas)")
	return nil
}

// Copiar copia las dos claves a otra carpeta, de un usuario (uid).
func Copiar(desde, hacia string, uid int) error {
	if err := os.MkdirAll(hacia, 0o755); err != nil {
		return err
	}
	for _, n := range []string{"ota-privada.pem", "ota-publica.pem"} {
		b, err := os.ReadFile(filepath.Join(desde, n))
		if err != nil {
			return err
		}
		destino := filepath.Join(hacia, n)
		if err := os.WriteFile(destino, b, 0o600); err != nil {
			return err
		}
		os.Chown(destino, uid, uid) //nolint:errcheck
	}
	os.Chown(hacia, uid, uid) //nolint:errcheck
	return nil
}

// Firmar arma el manifiesto de un binario y lo firma.
func (f *Firmador) Firmar(perfil, version string, binario []byte) (protocolo.Manifiesto, error) {
	m := protocolo.Manifiesto{Tamano: uint32(len(binario)), Sha256: sha256.Sum256(binario), Version: version, Perfil: perfil}
	if _, err := m.Bytes(); err != nil {
		return m, err
	}
	h := sha256.Sum256([]byte(m.Mensaje()))
	r, s, err := ecdsa.Sign(rand.Reader, f.clave, h[:])
	if err != nil {
		return m, err
	}
	r.FillBytes(m.Firma[:32])
	s.FillBytes(m.Firma[32:])
	return m, nil
}

// PublicaPEM es la clave pública (la que embebe el firmware).
func (f *Firmador) PublicaPEM() []byte {
	der, _ := x509.MarshalPKIXPublicKey(&f.clave.PublicKey)
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// Verificar comprueba un manifiesto con una clave pública PEM (lo usa el bicho virtual).
func Verificar(publicaPEM []byte, m protocolo.Manifiesto) bool {
	blk, _ := pem.Decode(publicaPEM)
	if blk == nil {
		return false
	}
	k, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return false
	}
	pub, ok := k.(*ecdsa.PublicKey)
	if !ok {
		return false
	}
	h := sha256.Sum256([]byte(m.Mensaje()))
	return ecdsa.Verify(pub, h[:], new(big.Int).SetBytes(m.Firma[:32]), new(big.Int).SetBytes(m.Firma[32:]))
}
