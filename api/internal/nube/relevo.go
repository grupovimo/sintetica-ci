package nube

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// Marco es una notificación para el cliente: [característica][bytes].
type Marco struct {
	Car   byte   `json:"car"`
	Datos []byte `json:"datos"`
}

// Relevo deja que un cliente lejos del bicho (la web, una app sin BLE) le
// hable con los mismos bytes del protocolo. Lo que cambia la config o las
// imágenes no va directo al bicho: se guarda como lo deseado y la
// sincronización se lo lleva (si está apagado, cuando vuelva). Los comandos
// van directo y sus respuestas llegan por los eventos ("marco").
type Relevo struct {
	n        *Nube
	mu       sync.Mutex
	clientes map[string]*recepciones
}

type recepcion struct {
	activa     bool
	largo, crc uint32
	espacio    int
	datos      []byte
}

type recepciones struct {
	config, imagen recepcion
	uso            time.Time
}

// NuevoRelevo crea el relevo.
func NuevoRelevo(n *Nube) *Relevo { return &Relevo{n: n, clientes: map[string]*recepciones{}} }

func (r *Relevo) de(conexion string) *recepciones {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.clientes {
		if time.Since(v.uso) > 10*time.Minute {
			delete(r.clientes, k)
		}
	}
	c := r.clientes[conexion]
	if c == nil {
		c = &recepciones{}
		r.clientes[conexion] = c
	}
	c.uso = time.Now()
	return c
}

func resp(car, op, codigo byte, detalle string) Marco {
	return Marco{Car: car, Datos: append([]byte{op | protocolo.Respuesta, codigo}, detalle...)}
}

// Relevar atiende un marco de un cliente. Devuelve las respuestas que da la
// API misma (las del bicho llegan por los eventos).
func (r *Relevo) Relevar(ctx context.Context, usuarioID, autor, conexion, bichoID string, car byte, m []byte) ([]Marco, error) {
	if len(m) == 0 {
		return nil, errors.New("marco vacío")
	}
	op := m[0]
	switch car {
	case protocolo.CarConfig:
		return r.config(ctx, usuarioID, autor, conexion, bichoID, op, m)
	case protocolo.CarImagen:
		return r.imagen(ctx, conexion, bichoID, op, m)
	case protocolo.CarComando:
		switch op {
		case protocolo.CmdAutenticar:
			return []Marco{resp(car, op, protocolo.Ok, "")}, nil
		case protocolo.CmdVincular, protocolo.CmdRed, protocolo.CmdNube:
			return []Marco{resp(car, op, protocolo.NoAutorizado, "eso solo se hace por BLE, cerca del bicho")}, nil
		case protocolo.CmdExpresion, protocolo.CmdNombre, protocolo.CmdEvento, protocolo.CmdDesvincular,
			protocolo.CmdOlvidarRed, protocolo.CmdReproducir, protocolo.CmdHora, protocolo.CmdVida,
			protocolo.CmdMemoria, protocolo.CmdRestaurar, protocolo.CmdRegistro, protocolo.CmdComportamiento:
			if err := r.n.Enviar(bichoID, car, m); err != nil {
				return []Marco{resp(car, op, protocolo.SinRed, err.Error())}, nil
			}
			return nil, nil
		}
		return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
	case protocolo.CarOta:
		return []Marco{resp(car, op, protocolo.NoAutorizado, "las actualizaciones se mandan desde la web")}, nil
	}
	return nil, errors.New("característica desconocida")
}

func (r *Relevo) config(ctx context.Context, usuarioID, autor, conexion, bichoID string, op byte, m []byte) ([]Marco, error) {
	const car = protocolo.CarConfig
	rc := &r.de(conexion).config
	switch op {
	case protocolo.ConfigInicio:
		if len(m) != 9 {
			return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
		}
		largo := binary.LittleEndian.Uint32(m[1:])
		if largo == 0 || largo > nucleo.MaxConfig {
			*rc = recepcion{}
			return []Marco{resp(car, op, protocolo.MuyGrande, "")}, nil
		}
		*rc = recepcion{activa: true, largo: largo, crc: binary.LittleEndian.Uint32(m[5:])}
		return []Marco{resp(car, op, protocolo.Ok, "")}, nil
	case protocolo.ConfigBloque:
		return bloque(rc, car, op, m), nil
	case protocolo.ConfigFin:
		if !rc.activa {
			return []Marco{resp(car, op, protocolo.SinInicio, "")}, nil
		}
		texto, largo, crc := rc.datos, rc.largo, rc.crc
		*rc = recepcion{}
		if uint32(len(texto)) != largo {
			return []Marco{resp(car, op, protocolo.FueraDeOrden, "")}, nil
		}
		if protocolo.Crc32(texto) != crc {
			return []Marco{resp(car, op, protocolo.CrcDistinto, "")}, nil
		}
		return r.guardarConfig(ctx, usuarioID, autor, bichoID, texto), nil
	case protocolo.ConfigPedir:
		c, err := r.n.alm.UltimaConfig(ctx, bichoID)
		texto := []byte(`{"version":1}`)
		if err == nil {
			texto = []byte(c.Texto)
		} else if !errors.Is(err, almacen.ErrNoEncontrado) {
			return nil, err
		}
		return trozos(car, protocolo.ConfigTrozo, texto, func(largo, crc uint32) []byte {
			return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32([]byte{protocolo.ConfigCierre}, largo), crc)
		}), nil
	}
	return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
}

func (r *Relevo) guardarConfig(ctx context.Context, usuarioID, autor, bichoID string, texto []byte) []Marco {
	const car, op = protocolo.CarConfig, protocolo.ConfigFin
	if err := nucleo.ValidarConfig(texto); err != nil {
		var ec *nucleo.ErrorConfig
		codigo := byte(protocolo.ConfigInvalida)
		if errors.As(err, &ec) && ec.JSONInvalido() {
			codigo = protocolo.JSONInvalido
		}
		return []Marco{resp(car, op, codigo, err.Error())}
	}
	if _, err := r.n.GuardarConfig(ctx, bichoID, string(texto), autor, usuarioID); err != nil {
		return []Marco{resp(car, op, protocolo.FalloOta, "no pude guardarla")}
	}
	return []Marco{resp(car, op, protocolo.Ok, "")}
}

// bloque agrega un bloque [op][u32 desplazamiento][datos]; un error aborta.
func bloque(rc *recepcion, car, op byte, m []byte) []Marco {
	if !rc.activa {
		return []Marco{resp(car, op, protocolo.SinInicio, "")}
	}
	if len(m) < 5 {
		return []Marco{resp(car, op, protocolo.Malformado, "")}
	}
	off := binary.LittleEndian.Uint32(m[1:])
	if off != uint32(len(rc.datos)) {
		*rc = recepcion{}
		return []Marco{resp(car, op, protocolo.FueraDeOrden, "")}
	}
	if uint32(len(rc.datos)+len(m)-5) > rc.largo {
		*rc = recepcion{}
		return []Marco{resp(car, op, protocolo.MuyGrande, "")}
	}
	rc.datos = append(rc.datos, m[5:]...)
	return nil
}

// trozos parte `datos` en notificaciones de 1 KB y un cierre.
func trozos(car, opTrozo byte, datos []byte, cierre func(largo, crc uint32) []byte) []Marco {
	var l []Marco
	for off := 0; off < len(datos); off += 1024 {
		fin := min(len(datos), off+1024)
		l = append(l, Marco{Car: car, Datos: append(binary.LittleEndian.AppendUint32([]byte{opTrozo}, uint32(off)), datos[off:fin]...)})
	}
	return append(l, Marco{Car: car, Datos: cierre(uint32(len(datos)), protocolo.Crc32(datos))})
}

func (r *Relevo) imagen(ctx context.Context, conexion, bichoID string, op byte, m []byte) ([]Marco, error) {
	const car = protocolo.CarImagen
	rc := &r.de(conexion).imagen
	switch op {
	case protocolo.ImgInicio:
		if len(m) != 10 || m[1] > 7 {
			return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
		}
		largo := binary.LittleEndian.Uint32(m[2:])
		limite := uint32(640*640*4 + 22)
		if est, ok := r.n.EstadoDe(bichoID); ok {
			limite = uint32(est.Pantalla.Ancho*est.Pantalla.Alto*4 + 22)
		}
		if largo < 22 || largo > limite {
			*rc = recepcion{}
			return []Marco{resp(car, op, protocolo.MuyGrande, "")}, nil
		}
		*rc = recepcion{activa: true, largo: largo, crc: binary.LittleEndian.Uint32(m[6:]), espacio: int(m[1])}
		return []Marco{resp(car, op, protocolo.Ok, "")}, nil
	case protocolo.ImgBloque:
		return bloque(rc, car, op, m), nil
	case protocolo.ImgFin:
		if !rc.activa {
			return []Marco{resp(car, op, protocolo.SinInicio, "")}, nil
		}
		datos, largo, crc, espacio := rc.datos, rc.largo, rc.crc, rc.espacio
		*rc = recepcion{}
		if uint32(len(datos)) != largo {
			return []Marco{resp(car, op, protocolo.FueraDeOrden, "")}, nil
		}
		if protocolo.Crc32(datos) != crc {
			return []Marco{resp(car, op, protocolo.CrcDistinto, "")}, nil
		}
		if err := r.n.GuardarImagen(ctx, bichoID, espacio, datos); err != nil {
			return []Marco{resp(car, op, protocolo.ImagenInvalida, err.Error())}, nil
		}
		return []Marco{resp(car, op, protocolo.Ok, "")}, nil
	case protocolo.ImgBorrar:
		if len(m) != 2 || m[1] > 7 {
			return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
		}
		if err := r.n.BorrarImagen(ctx, bichoID, int(m[1])); err != nil {
			return nil, err
		}
		return []Marco{resp(car, op, protocolo.Ok, "")}, nil
	case protocolo.ImgListar:
		l, err := r.n.alm.Imagenes(ctx, bichoID)
		if err != nil {
			return nil, err
		}
		b := []byte{protocolo.ImgListar | protocolo.Respuesta, protocolo.Ok, 0}
		for _, i := range l {
			b[2] |= 1 << i.Espacio
			b = append(b, byte(i.Espacio))
			b = binary.LittleEndian.AppendUint32(b, uint32(i.Largo))
			b = binary.LittleEndian.AppendUint32(b, i.Crc)
		}
		return []Marco{{Car: car, Datos: b}}, nil
	case protocolo.ImgPedir:
		if len(m) != 2 || m[1] > 7 {
			return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
		}
		img, err := r.n.alm.LeerImagen(ctx, bichoID, int(m[1]))
		if errors.Is(err, almacen.ErrNoEncontrado) {
			return []Marco{resp(car, op, protocolo.NoExiste, "")}, nil
		}
		if err != nil {
			return nil, err
		}
		espacio := m[1]
		l := []Marco{resp(car, op, protocolo.Ok, "")}
		return append(l, trozos(car, protocolo.ImgTrozo, img.Datos, func(largo, crc uint32) []byte {
			return binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32([]byte{protocolo.ImgCierre, espacio}, largo), crc)
		})...), nil
	}
	return []Marco{resp(car, op, protocolo.Malformado, "")}, nil
}

// --- Lo deseado (lo usan el relevo y el servidor HTTP) ---

// GuardarConfig guarda una config nueva (ya validada) y la manda a sincronizar.
func (n *Nube) GuardarConfig(ctx context.Context, bichoID, texto, autor, usuarioID string) (almacen.Config, error) {
	var u *string
	if usuarioID != "" {
		u = &usuarioID
	}
	c, err := n.alm.NuevaConfig(ctx, bichoID, texto, protocolo.Crc32([]byte(texto)), autor, u)
	if err != nil {
		return c, err
	}
	n.publicar(bichoID, eventos.Evento{Tipo: "config", Datos: c})
	n.CambioDeseado(bichoID)
	return c, nil
}

// ErrImagen: no es un QOI (o una animación) del tamaño de la pantalla del bicho.
var ErrImagen = errors.New("tiene que ser un QOI o una animación del tamaño de la pantalla del bicho")

// GuardarImagen guarda una imagen (verifica la cabecera QOI) y la manda a sincronizar.
func (n *Nube) GuardarImagen(ctx context.Context, bichoID string, espacio int, qoi []byte) error {
	w, h, ok := protocolo.TamanoImagen(qoi)
	if !ok {
		return ErrImagen
	}
	if est, conocido := n.EstadoDe(bichoID); conocido && (w != est.Pantalla.Ancho || h != est.Pantalla.Alto) {
		return ErrImagen
	}
	if err := n.alm.GuardarImagen(ctx, bichoID, espacio, qoi, protocolo.Crc32(qoi)); err != nil {
		return err
	}
	n.publicar(bichoID, eventos.Evento{Tipo: "imagenes"})
	n.CambioDeseado(bichoID)
	return nil
}

// BorrarImagen vacía un espacio y lo manda a sincronizar.
func (n *Nube) BorrarImagen(ctx context.Context, bichoID string, espacio int) error {
	if err := n.alm.BorrarImagen(ctx, bichoID, espacio); err != nil {
		return err
	}
	n.publicar(bichoID, eventos.Evento{Tipo: "imagenes"})
	n.CambioDeseado(bichoID)
	return nil
}
