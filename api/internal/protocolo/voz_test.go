package protocolo

import (
	"bytes"
	"testing"
)

func TestTrozosAudio(t *testing.T) {
	audio := bytes.Repeat([]byte{1, 2, 3}, MaxTrozoAudio) // 3 trozos
	ps, err := TrozosAudio(7, FormatoMP3, OrigenDecir, Crc32([]byte("hola")), audio)
	if err != nil || len(ps) != 3 {
		t.Fatalf("%d trozos, %v", len(ps), err)
	}
	var junto []byte
	for k, p := range ps {
		if len(p) > CabeceraAudio+MaxTrozoAudio {
			t.Fatalf("trozo %d de %d bytes", k, len(p))
		}
		tr, err := LeerTrozoAudio(p)
		if err != nil || tr.ID != 7 || tr.Formato != FormatoMP3 || tr.Origen != OrigenDecir || tr.CrcTexto != Crc32([]byte("hola")) ||
			tr.Indice != k || tr.Total != 3 {
			t.Fatalf("trozo %d: %+v %v", k, tr, err)
		}
		junto = append(junto, tr.Datos...)
	}
	if !bytes.Equal(junto, audio) {
		t.Fatal("el audio no vuelve igual")
	}
	// La cabecera, byte a byte (lo que lee el robot-celu).
	p, _ := TrozosAudio(0x0102, FormatoMP3, OrigenDespertador, 0xA1B2C3D4, []byte{9})
	quiero := []byte{2, 0x02, 0x01, 1, 2, 0xD4, 0xC3, 0xB2, 0xA1, 0, 1, 9}
	if !bytes.Equal(p[0], quiero) {
		t.Fatalf("%x, quiero %x", p[0], quiero)
	}
	if _, err := TrozosAudio(1, FormatoMP3, 0, 0, nil); err == nil {
		t.Fatal("vacío")
	}
	if _, err := TrozosAudio(1, FormatoMP3, 0, 0, make([]byte, MaxAudioEntero+1)); err == nil {
		t.Fatal("grande")
	}
	if _, err := LeerTrozoAudio([]byte{2, 0, 0, 1, 0, 0, 0, 0, 0, 1, 1}); err == nil {
		t.Fatal("índice fuera del total")
	}
	if !bytes.Equal(AvisoVoz(true), []byte{3, 1}) || !bytes.Equal(AvisoVoz(false), []byte{3, 0}) {
		t.Fatal("aviso")
	}
}
