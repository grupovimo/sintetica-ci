package nube

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

const (
	entEstado = iota
	entConexion
	entRespuesta
	entCambio
	entReclamado // se conectó con una clave nueva: hay que esperar a que se anuncie
)

type entrada struct {
	tipo      int
	car       byte
	datos     []byte
	conectado bool
}

// Progreso de la sincronización: "al-dia", "enviando", "trayendo", "error",
// "esperando" (el bicho no está conectado) o "sin-datos".
type Progreso struct {
	Config   string `json:"config"`
	Imagenes string `json:"imagenes"`
	Detalle  string `json:"detalle,omitempty"`
}

// Tamaño de los marcos que la API manda por MQTT (el bicho los acepta de hasta 2 KB).
const tamanoMarco = 1024 + 5

var errTiempo = errors.New("el bicho no respondió a tiempo")

// trabajador atiende a un bicho: todo lo que tiene que ver con él pasa por
// acá, de a uno, así la sincronización nunca se cruza consigo misma.
type trabajador struct {
	n       *Nube
	id      string
	entrada chan entrada

	estado         *protocolo.Estado
	conectado      bool
	conectadoAhora atomic.Bool
	ultimoEstado   atomic.Pointer[protocolo.Estado]
	progreso       atomic.Pointer[Progreso]

	pendiente       bool
	esperandoAviso  bool // recién reconectado: hasta que mande su estado o su conexión, no escucha
	avisoEn         time.Time
	proximoIntento  time.Time
	otaNuevaDesde   time.Time // cuándo volvió con la versión nueva de una OTA
	estadoEn        time.Time // cuándo llegó el último estado
	configRechazada uint32    // una config que el bicho rechazó: no se reintenta
}

func nuevoTrabajador(n *Nube, id string) *trabajador {
	return &trabajador{n: n, id: id, entrada: make(chan entrada, 2048)}
}

func (t *trabajador) correr() {
	reloj := time.NewTicker(time.Second)
	defer reloj.Stop()
	vueltas := 0
	for {
		select {
		case <-t.n.ctx.Done():
			return
		case e := <-t.entrada:
			t.manejar(e)
		case <-reloj.C:
			if vueltas++; vueltas%15 == 0 && t.estado != nil {
				t.revisarOta(*t.estado)
			}
		}
		if t.pendiente && !t.puede() && time.Now().After(t.proximoIntento) {
			t.n.log.Debug("no puedo sincronizar todavía", "bicho", t.id, "conectado", t.conectado, "estado", t.estado != nil)
		}
		if t.pendiente && t.puede() && time.Now().After(t.proximoIntento) && time.Since(t.avisoEn) > time.Second {
			if err := t.sincronizar(); err != nil {
				t.n.log.Warn("sincronización", "bicho", t.id, "error", err)
				t.proximoIntento = time.Now().Add(20 * time.Second)
			} else {
				t.pendiente = false
			}
		}
	}
}

func (t *trabajador) puede() bool {
	return t.conectado && t.estado != nil && !t.esperandoAviso && t.n.Dueno(t.id) != ""
}

func (t *trabajador) avisarProgreso(config, imagenes, detalle string) {
	p := Progreso{Config: config, Imagenes: imagenes, Detalle: detalle}
	if v := t.progreso.Load(); v != nil {
		if config == "" {
			p.Config = v.Config
		}
		if imagenes == "" {
			p.Imagenes = v.Imagenes
		}
		if *v == p {
			return
		}
	}
	t.progreso.Store(&p)
	t.n.publicar(t.id, eventos.Evento{Tipo: "sincro", Datos: p})
}

// manejar atiende un mensaje fuera de una sincronización (o mientras espera una respuesta).
func (t *trabajador) manejar(e entrada) {
	ctx := t.n.ctx
	switch e.tipo {
	case entEstado:
		est, err := protocolo.LeerEstado(e.datos)
		if err != nil {
			return
		}
		cambioConfig := t.estado == nil || t.estado.CrcConfig != est.CrcConfig
		if t.esperandoAviso {
			t.avisoEn = time.Now()
		}
		t.esperandoAviso = false
		t.estado = &est
		t.estadoEn = time.Now()
		t.ultimoEstado.Store(&est)
		if err := t.n.alm.GuardarEstado(ctx, t.id, e.datos, est.Perfil); err != nil {
			t.n.log.Warn("no pude guardar el estado", "bicho", t.id, "error", err)
		}
		t.n.publicar(t.id, eventos.Evento{Tipo: "estado", Datos: est})
		t.revisarOta(est)
		if cambioConfig {
			t.pendiente = true
		}
	case entConexion:
		if e.conectado {
			if t.esperandoAviso {
				t.avisoEn = time.Now()
			}
			t.esperandoAviso = false
		}
		if t.conectado == e.conectado {
			return
		}
		t.conectado = e.conectado
		t.conectadoAhora.Store(e.conectado)
		if err := t.n.alm.GuardarConexion(ctx, t.id, e.conectado); err != nil {
			t.n.log.Warn("no pude guardar la conexión", "bicho", t.id, "error", err)
		}
		t.n.publicar(t.id, eventos.Evento{Tipo: "conexion", Datos: map[string]bool{"conectado": e.conectado}})
		if e.conectado {
			t.pendiente = true
			t.proximoIntento = time.Time{}
		} else {
			t.avisarProgreso("esperando", "esperando", "")
		}
	case entReclamado:
		t.esperandoAviso = true
		t.pendiente = true
		t.proximoIntento = time.Time{}
	case entCambio:
		t.pendiente = true
		t.proximoIntento = time.Time{}
		if !t.conectado {
			t.avisarProgreso("esperando", "esperando", "el bicho no está conectado: se le manda cuando vuelva")
		}
	case entRespuesta:
		// Lo de los comandos (y el progreso de las OTA) le interesa a quien mira.
		if e.car == protocolo.CarComando || e.car == protocolo.CarOta {
			t.n.publicar(t.id, eventos.Evento{Tipo: "marco", Datos: map[string]any{"car": e.car, "datos": e.datos}})
		}
		if e.car == protocolo.CarOta && len(e.datos) >= 2 {
			t.progresoOta(e.datos)
		}
	}
}

// esperar lee mensajes hasta que llega una respuesta que cumple `es` (y
// atiende los demás mientras tanto).
func (t *trabajador) esperar(es func(car byte, d []byte) bool, plazo time.Duration) ([]byte, error) {
	limite := time.NewTimer(plazo)
	defer limite.Stop()
	for {
		select {
		case <-t.n.ctx.Done():
			return nil, t.n.ctx.Err()
		case <-limite.C:
			return nil, errTiempo
		case e := <-t.entrada:
			if e.tipo == entRespuesta && es(e.car, e.datos) {
				return e.datos, nil
			}
			t.manejar(e)
			if e.tipo == entConexion && !e.conectado {
				return nil, ErrDesconectado
			}
		}
	}
}

// respuestaA: la respuesta a una op (o un error de otra de la misma característica).
func respuestaA(car, op byte, errores ...byte) func(byte, []byte) bool {
	return func(c byte, d []byte) bool {
		if c != car || len(d) < 2 {
			return false
		}
		if d[0] == op|protocolo.Respuesta {
			return true
		}
		for _, o := range errores {
			if d[0] == o|protocolo.Respuesta && d[1] != protocolo.Ok {
				return true
			}
		}
		return false
	}
}

func (t *trabajador) sincronizar() error {
	if err := t.sincronizarConfig(); err != nil {
		t.avisarProgreso("error", "", err.Error())
		return err
	}
	if err := t.sincronizarImagenes(); err != nil {
		t.avisarProgreso("", "error", err.Error())
		return err
	}
	return nil
}

func (t *trabajador) sincronizarConfig() error {
	ctx := t.n.ctx
	deseada, err := t.n.alm.UltimaConfig(ctx, t.id)
	hay := err == nil
	if err != nil && !errors.Is(err, almacen.ErrNoEncontrado) {
		return err
	}
	sin, err := t.n.alm.LeerSincro(ctx, t.id)
	if err != nil {
		return err
	}
	d := t.estado.CrcConfig
	acordada := "ninguna"
	if sin.ConfigCrc != nil {
		acordada = fmt.Sprintf("%08x", *sin.ConfigCrc)
	}
	t.n.log.Info("sincronizando la config", "bicho", t.id, "bicho_tiene", fmt.Sprintf("%08x", d),
		"quiero", fmt.Sprintf("%08x", deseada.Crc), "hay", hay, "acordada", acordada)
	switch {
	case hay && d == deseada.Crc:
		if sin.ConfigCrc == nil || *sin.ConfigCrc != d {
			if err := t.n.alm.AcordarConfig(ctx, t.id, d); err != nil {
				return err
			}
		}
	case !hay:
		// La API no tiene nada: se trae la del bicho (si no es la de fábrica).
		if d != 0 {
			if err := t.traerConfig(); err != nil {
				return err
			}
		}
	case d == 0 || sin.ConfigCrc == nil || *sin.ConfigCrc != deseada.Crc:
		// Cambió en la API (o el bicho volvió a fábrica): se le manda.
		if deseada.Crc == t.configRechazada {
			t.avisarProgreso("error", "", "el bicho rechazó esta config")
			return nil
		}
		if err := t.mandarConfig(deseada); err != nil {
			return err
		}
	default:
		// Cambió en el bicho (por BLE, desde una app cerca): se la trae.
		if err := t.traerConfig(); err != nil {
			return err
		}
	}
	t.avisarProgreso("al-dia", "", "")
	return nil
}

func (t *trabajador) mandarConfig(c almacen.Config) error {
	t.n.log.Info("le mando la config", "bicho", t.id, "version", c.Version)
	t.avisarProgreso("enviando", "", "")
	for _, m := range protocolo.EscriturasDeConfig([]byte(c.Texto), tamanoMarco) {
		if err := t.n.enviar(t.id, protocolo.CarConfig, m); err != nil {
			return err
		}
	}
	r, err := t.esperar(respuestaA(protocolo.CarConfig, protocolo.ConfigFin, protocolo.ConfigInicio, protocolo.ConfigBloque), 30*time.Second)
	if err != nil {
		return err
	}
	if r[0] != protocolo.ConfigFin|protocolo.Respuesta || r[1] != protocolo.Ok {
		res, _ := protocolo.LeerResultado(r)
		if r[1] == protocolo.ConfigInvalida || r[1] == protocolo.JSONInvalido {
			t.configRechazada = c.Crc
		}
		return fmt.Errorf("el bicho no aceptó la config: %s %s", res.Mensaje, res.Detalle)
	}
	if err := t.n.alm.AcordarConfig(t.n.ctx, t.id, c.Crc); err != nil {
		return err
	}
	// Que el estado ya lo refleje aunque el bicho tarde en mandarlo.
	if t.estado != nil {
		t.estado.CrcConfig = c.Crc
	}
	return nil
}

func (t *trabajador) traerConfig() error {
	t.n.log.Info("le traigo la config (cambió en el bicho)", "bicho", t.id)
	t.avisarProgreso("trayendo", "", "")
	if err := t.n.enviar(t.id, protocolo.CarConfig, protocolo.PedirConfig()); err != nil {
		return err
	}
	var a protocolo.Armador
	for {
		r, err := t.esperar(func(c byte, d []byte) bool {
			return c == protocolo.CarConfig && len(d) >= 1 && (d[0] == protocolo.ConfigTrozo || d[0] == protocolo.ConfigCierre)
		}, 30*time.Second)
		if err != nil {
			return err
		}
		if r[0] == protocolo.ConfigTrozo {
			a.Agregar(r)
			continue
		}
		if len(r) < 9 {
			return errors.New("cierre de config malformado")
		}
		texto, err := a.Cerrar(leer32(r[1:]), leer32(r[5:]))
		if err != nil {
			return err
		}
		if err := nucleo.ValidarConfig(texto); err != nil {
			return fmt.Errorf("la config del bicho no valida: %w", err)
		}
		c, err := t.n.alm.NuevaConfig(t.n.ctx, t.id, string(texto), protocolo.Crc32(texto), "bicho", nil)
		if err != nil {
			return err
		}
		if err := t.n.alm.AcordarConfig(t.n.ctx, t.id, c.Crc); err != nil {
			return err
		}
		t.n.publicar(t.id, eventos.Evento{Tipo: "config", Datos: c})
		return nil
	}
}

func leer32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func (t *trabajador) sincronizarImagenes() error {
	ctx := t.n.ctx
	if err := t.n.enviar(t.id, protocolo.CarImagen, protocolo.ListarImagenes()); err != nil {
		return err
	}
	r, err := t.esperar(respuestaA(protocolo.CarImagen, protocolo.ImgListar), 15*time.Second)
	if err != nil {
		return err
	}
	enBicho := map[int]uint32{}
	for _, e := range protocolo.LeerListaImagenes(r) {
		enBicho[e.Espacio] = e.Crc
	}
	lista, err := t.n.alm.Imagenes(ctx, t.id)
	if err != nil {
		return err
	}
	deseadas := map[int]uint32{}
	for _, i := range lista {
		deseadas[i.Espacio] = i.Crc
	}
	sin, err := t.n.alm.LeerSincro(ctx, t.id)
	if err != nil {
		return err
	}
	cambio := false
	for e := 0; e < 8; e++ {
		d, s, a := enBicho[e], deseadas[e], sin.Imagenes[e]
		switch {
		case d == s:
			if a != s {
				if err := t.n.alm.AcordarImagen(ctx, t.id, e, s); err != nil {
					return err
				}
			}
		case s != a:
			t.avisarProgreso("", "enviando", "")
			if err := t.mandarImagen(e, s); err != nil {
				return err
			}
		default:
			t.avisarProgreso("", "trayendo", "")
			if err := t.traerImagen(e, d); err != nil {
				return err
			}
			cambio = true
		}
	}
	if cambio {
		t.n.publicar(t.id, eventos.Evento{Tipo: "imagenes"})
	}
	t.avisarProgreso("", "al-dia", "")
	return nil
}

func (t *trabajador) mandarImagen(espacio int, crc uint32) error {
	ctx := t.n.ctx
	if crc == 0 {
		if err := t.n.enviar(t.id, protocolo.CarImagen, protocolo.BorrarImagen(espacio)); err != nil {
			return err
		}
		if _, err := t.esperar(respuestaA(protocolo.CarImagen, protocolo.ImgBorrar), 15*time.Second); err != nil {
			return err
		}
		return t.n.alm.AcordarImagen(ctx, t.id, espacio, 0)
	}
	img, err := t.n.alm.LeerImagen(ctx, t.id, espacio)
	if err != nil {
		return err
	}
	for _, m := range protocolo.EscriturasDeImagen(espacio, img.Datos, tamanoMarco) {
		if err := t.n.enviar(t.id, protocolo.CarImagen, m); err != nil {
			return err
		}
	}
	r, err := t.esperar(respuestaA(protocolo.CarImagen, protocolo.ImgFin, protocolo.ImgInicio, protocolo.ImgBloque), 60*time.Second)
	if err != nil {
		return err
	}
	if r[0] != protocolo.ImgFin|protocolo.Respuesta || r[1] != protocolo.Ok {
		res, _ := protocolo.LeerResultado(r)
		return fmt.Errorf("el bicho no aceptó la imagen %d: %s %s", espacio, res.Mensaje, res.Detalle)
	}
	return t.n.alm.AcordarImagen(ctx, t.id, espacio, img.Crc)
}

func (t *trabajador) traerImagen(espacio int, crc uint32) error {
	ctx := t.n.ctx
	if crc == 0 {
		// La borraron en el bicho.
		if err := t.n.alm.BorrarImagen(ctx, t.id, espacio); err != nil {
			return err
		}
		return t.n.alm.AcordarImagen(ctx, t.id, espacio, 0)
	}
	if err := t.n.enviar(t.id, protocolo.CarImagen, protocolo.PedirImagen(espacio)); err != nil {
		return err
	}
	var a protocolo.Armador
	for {
		r, err := t.esperar(func(c byte, d []byte) bool {
			return c == protocolo.CarImagen && len(d) >= 2 && (d[0] == protocolo.ImgTrozo || d[0] == protocolo.ImgCierre ||
				(d[0] == protocolo.ImgPedir|protocolo.Respuesta && d[1] != protocolo.Ok))
		}, 60*time.Second)
		if err != nil {
			return err
		}
		if r[0] == protocolo.ImgTrozo {
			a.Agregar(r)
			continue
		}
		if r[0] != protocolo.ImgCierre || len(r) < 10 {
			return fmt.Errorf("el bicho no dio la imagen %d", espacio)
		}
		datos, err := a.Cerrar(leer32(r[2:]), leer32(r[6:]))
		if err != nil {
			return err
		}
		if err := t.n.alm.GuardarImagen(ctx, t.id, espacio, datos, protocolo.Crc32(datos)); err != nil {
			return err
		}
		return t.n.alm.AcordarImagen(ctx, t.id, espacio, protocolo.Crc32(datos))
	}
}

// --- OTA ---

func (t *trabajador) progresoOta(d []byte) {
	ctx := t.n.ctx
	var estado, detalle string
	pct := 0
	switch d[0] {
	case protocolo.OtaDescargar | protocolo.Respuesta:
		if d[1] == protocolo.Ok {
			estado = "descargando"
		} else {
			res, _ := protocolo.LeerResultado(d)
			estado, detalle = "fallida", res.Mensaje+" "+res.Detalle
		}
	case protocolo.OtaProgreso | protocolo.Respuesta:
		if len(d) >= 3 {
			pct = int(d[2])
		}
		if d[1] != protocolo.Ok {
			estado = "fallida"
			detalle = protocolo.Mensaje(int(d[1]))
			if len(d) > 3 {
				detalle += ": " + string(d[3:])
			}
		} else if pct >= 100 {
			estado = "instalada"
		} else {
			estado = "descargando"
		}
	default:
		return
	}
	u, err := t.n.alm.ActualizarActualizacion(ctx, t.id, estado, pct, detalle)
	if err == nil {
		t.n.publicar(t.id, eventos.Evento{Tipo: "ota", Datos: u})
	}
}

// revisarOta: después de instalar, el bicho reinicia con la versión nueva a
// prueba. Se sabe que reinició porque arrancó (estadoEn − encendido) después
// de instalar. Si vuelve con la versión nueva y sigue conectado medio minuto,
// el firmware la confirmó (él se confirma a los 20 s conectado). Si vuelve con
// la anterior, falló y el bicho volvió atrás.
func (t *trabajador) revisarOta(est protocolo.Estado) {
	ctx, cancelar := context.WithTimeout(t.n.ctx, 5*time.Second)
	defer cancelar()
	u, err := t.n.alm.ActualizacionPendiente(ctx, t.id)
	if err != nil || u.Estado != "instalada" {
		t.otaNuevaDesde = time.Time{}
		return
	}
	terminar := func(estado, detalle string) {
		t.otaNuevaDesde = time.Time{}
		if u, err := t.n.alm.ActualizarActualizacion(ctx, t.id, estado, 100, detalle); err == nil {
			t.n.publicar(t.id, eventos.Evento{Tipo: "ota", Datos: u})
		}
	}
	arranco := t.estadoEn.Add(-time.Duration(est.Encendido) * time.Second)
	if !arranco.After(u.Actualizada.Add(-2 * time.Second)) {
		if time.Since(u.Actualizada) > 3*time.Minute {
			terminar("fallida", "no volvió después de instalar")
		}
		return
	}
	if est.Version != u.Version {
		terminar("fallida", "volvió a la versión anterior ("+est.Version+")")
		return
	}
	if t.otaNuevaDesde.IsZero() {
		t.otaNuevaDesde = time.Now()
		return
	}
	if t.conectado && time.Since(t.otaNuevaDesde) >= 30*time.Second {
		terminar("confirmada", "")
	}
}
