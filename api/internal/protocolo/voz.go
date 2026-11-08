package protocolo

// Los paquetes de audio del tópico bicho/{id}/audio (y de la característica
// Audio por BLE). Ver docs/protocolo-ble.md, "Voz".

import (
	"encoding/binary"
	"errors"
)

// Tipos del primer byte.
const (
	AudioMicrofono = 0 // [u16 secuencia][Opus 16 kHz, 20 ms] (M7)
	AudioParlante  = 1 // [u16 secuencia][Opus 16 kHz, 20 ms] (M7)
	// AudioEntero: un audio comprimido entero (un mp3), en uno o más trozos.
	// [2][u16 id][u8 formato][u8 origen][u32 CRC-32 del texto][u8 índice][u8 total][bytes]
	AudioEntero = 2
	// AudioVoz: [3][u8 flags: bit 0 = la nube lee los globos en voz alta].
	// Llega al conectarse y cada vez que cambia.
	AudioVoz = 3
)

// Formatos de AudioEntero.
const (
	FormatoMP3 = 1 // mp3 mono: 22,05 kHz y 32 kbps (ElevenLabs) o 24 kHz y 48 kbps (Edge)
)

// Orígenes de AudioEntero: a qué globo acompaña.
const (
	OrigenDecir        = 0 // al 0x40 Decir que llega por cmd justo después (el CRC es de su texto)
	OrigenRecordatorio = 1 // a un recordatorio que el robot ya está mostrando
	OrigenDespertador  = 2 // al despertador que está sonando
)

// Topes del audio.
const (
	MaxTrozoAudio  = 48 << 10  // bytes de audio por paquete (el tope de MQTT es 1 MB)
	MaxAudioEntero = 512 << 10 // un globo dura de 3 a 10 s: a 32 kbps son unos 40 KB
	CabeceraAudio  = 11
)

// ErrAudioGrande: el audio no entra.
var ErrAudioGrande = errors.New("el audio es demasiado largo")

// TrozosAudio parte un audio entero en paquetes AudioEntero.
func TrozosAudio(id uint16, formato, origen byte, crcTexto uint32, audio []byte) ([][]byte, error) {
	if len(audio) == 0 || len(audio) > MaxAudioEntero {
		return nil, ErrAudioGrande
	}
	total := (len(audio) + MaxTrozoAudio - 1) / MaxTrozoAudio
	out := make([][]byte, 0, total)
	for i := 0; i < total; i++ {
		d := audio[i*MaxTrozoAudio : min(len(audio), (i+1)*MaxTrozoAudio)]
		p := make([]byte, 0, CabeceraAudio+len(d))
		p = append(p, AudioEntero)
		p = binary.LittleEndian.AppendUint16(p, id)
		p = append(p, formato, origen)
		p = binary.LittleEndian.AppendUint32(p, crcTexto)
		p = append(p, byte(i), byte(total))
		out = append(out, append(p, d...))
	}
	return out, nil
}

// TrozoAudio es un paquete AudioEntero leído.
type TrozoAudio struct {
	ID              uint16
	Formato, Origen byte
	CrcTexto        uint32
	Indice, Total   int
	Datos           []byte
}

// LeerTrozoAudio lee un paquete AudioEntero.
func LeerTrozoAudio(b []byte) (TrozoAudio, error) {
	if len(b) < CabeceraAudio || b[0] != AudioEntero || b[10] == 0 || b[9] >= b[10] {
		return TrozoAudio{}, errors.New("no es un trozo de audio")
	}
	return TrozoAudio{ID: binary.LittleEndian.Uint16(b[1:]), Formato: b[3], Origen: b[4], CrcTexto: binary.LittleEndian.Uint32(b[5:]),
		Indice: int(b[9]), Total: int(b[10]), Datos: b[CabeceraAudio:]}, nil
}

// AvisoVoz arma el paquete AudioVoz.
func AvisoVoz(lee bool) []byte {
	if lee {
		return []byte{AudioVoz, 1}
	}
	return []byte{AudioVoz, 0}
}
