package servidor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

type claveBicho int

const claveDelBicho claveBicho = 0

func bichoDe(r *http.Request) almacen.Bicho { return r.Context().Value(claveDelBicho).(almacen.Bicho) }

// conBicho exige sesión y que el bicho {id} sea del usuario.
func (s *Servidor) conBicho(h http.HandlerFunc) http.HandlerFunc {
	return s.conSesion(func(w http.ResponseWriter, r *http.Request) {
		id := strings.ToUpper(r.PathValue("id"))
		b, err := s.alm.BichoDe(r.Context(), id, usuarioDe(r).Usuario.ID)
		if errors.Is(err, almacen.ErrNoEncontrado) {
			fallar(w, http.StatusNotFound, errNoEsTuyo.Error())
			return
		}
		if err != nil {
			s.interno(w, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), claveDelBicho, b)))
	})
}

// VistaBicho es un bicho como lo ven la web y las apps.
type VistaBicho struct {
	almacen.Bicho
	Estado *protocolo.Estado `json:"estado"`
	Sincro nube.Progreso     `json:"sincro"`
	Config *almacen.Config   `json:"config"`
}

func (s *Servidor) vista(ctx context.Context, b almacen.Bicho) VistaBicho {
	v := VistaBicho{Bicho: b, Sincro: s.nube.EstadoSincro(b.ID)}
	if e, ok := s.nube.EstadoDe(b.ID); ok {
		v.Estado = &e
	} else if len(b.Estado) > 0 {
		if e, err := protocolo.LeerEstado(b.Estado); err == nil {
			v.Estado = &e
		}
	}
	v.Conectado = s.nube.Conectado(b.ID) || (b.Conectado && v.Estado != nil && !s.nube.BrokerConectado())
	if c, err := s.alm.UltimaConfig(ctx, b.ID); err == nil {
		v.Config = &c
	}
	return v
}

func (s *Servidor) listarBichos(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.BichosDe(r.Context(), usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	vistas := make([]VistaBicho, 0, len(l))
	for _, b := range l {
		vistas = append(vistas, s.vista(r.Context(), b))
	}
	responder(w, http.StatusOK, map[string]any{"bichos": vistas})
}

func (s *Servidor) verBicho(w http.ResponseWriter, r *http.Request) {
	responder(w, http.StatusOK, s.vista(r.Context(), bichoDe(r)))
}

func (s *Servidor) cambiarBicho(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Apodo string `json:"apodo"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	p.Apodo = strings.TrimSpace(p.Apodo)
	if len(p.Apodo) > 40 {
		fallar(w, http.StatusBadRequest, "el apodo es de hasta 40 caracteres")
		return
	}
	b := bichoDe(r)
	if err := s.alm.ActualizarApodo(r.Context(), b.ID, usuarioDe(r).Usuario.ID, p.Apodo); err != nil {
		s.interno(w, err)
		return
	}
	b.Apodo = p.Apodo
	responder(w, http.StatusOK, s.vista(r.Context(), b))
}

// DELETE /v1/bichos/{id}: lo saca de la cuenta. Si está conectado, antes le
// dice que olvide el WiFi y la nube (si no, lo echa el broker al revocarle la clave).
func (s *Servidor) quitarBicho(w http.ResponseWriter, r *http.Request) {
	b := bichoDe(r)
	if s.nube.Conectado(b.ID) {
		s.nube.Enviar(b.ID, protocolo.CarComando, protocolo.OlvidarRed()) //nolint:errcheck
	}
	if err := s.alm.QuitarDeCuenta(r.Context(), b.ID, usuarioDe(r).Usuario.ID); err != nil {
		s.interno(w, err)
		return
	}
	s.nube.Quitado(b.ID, usuarioDe(r).Usuario.ID)
	w.WriteHeader(http.StatusNoContent)
}

// --- Sumar un bicho a la cuenta ---

// Credenciales MQTT para el bicho (la app se las pasa por BLE, comando 0x17).
type Credenciales struct {
	URL     string `json:"url"`
	Usuario string `json:"usuario"`
	Clave   string `json:"clave"`
}

func (s *Servidor) credencialNueva(ctx context.Context, id, perfil string, virtual bool, usuarioID string) (Credenciales, error) {
	if err := s.alm.AsegurarBicho(ctx, id, perfil, virtual); err != nil {
		return Credenciales{}, err
	}
	clave := aleatorio(24)
	if err := s.alm.CrearCredencialPendiente(ctx, id, usuarioID, hash(clave), time.Now().Add(30*time.Minute)); err != nil {
		return Credenciales{}, err
	}
	url := s.op.MQTTURLPublica
	if virtual {
		url = s.op.MQTTURLInterna
	}
	return Credenciales{URL: url, Usuario: id, Clave: clave}, nil
}

// POST /v1/bichos/reclamar {id, perfil}: la app (ya vinculada por BLE, o sea,
// con el bicho en la mano) pide credenciales para el bicho. El bicho pasa a la
// cuenta cuando se conecta con ellas.
func (s *Servidor) reclamar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		ID     string `json:"id"`
		Perfil string `json:"perfil"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	p.ID = strings.ToUpper(p.ID)
	if !protocolo.IDValido(p.ID) || len(p.Perfil) > 23 {
		fallar(w, http.StatusBadRequest, "el id son 12 caracteres hexadecimales")
		return
	}
	if !s.limites.permitir("reclamar:"+usuarioDe(r).Usuario.ID, 30, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "demasiados pedidos: probá en un rato")
		return
	}
	c, err := s.credencialNueva(r.Context(), p.ID, p.Perfil, false, usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"mqtt": c, "vence": time.Now().Add(30 * time.Minute)})
}

// GET /v1/bichos/{id}/reclamo?esperar=20: espera a que el bicho se sume (con la clave nueva).
func (s *Servidor) esperarReclamo(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(r.PathValue("id"))
	usuario := usuarioDe(r).Usuario.ID
	esperar, _ := strconv.Atoi(r.URL.Query().Get("esperar"))
	esperar = max(0, min(esperar, 60))
	ctx, cancelar := context.WithTimeout(r.Context(), time.Duration(esperar)*time.Second)
	defer cancelar()
	if _, err := s.alm.BichoDe(r.Context(), id, usuario); err != nil && esperar > 0 {
		s.nube.EsperarReclamo(ctx, id)
	}
	_, err := s.alm.BichoDe(r.Context(), id, usuario)
	responder(w, http.StatusOK, map[string]bool{"sumado": err == nil})
}

// GET /v1/bichos/virtuales: solo en desarrollo, los bichos virtuales que
// están andando (para sumarlos con un clic).
func (s *Servidor) listarVirtuales(w http.ResponseWriter, r *http.Request) {
	if !s.op.Desarrollo {
		responder(w, http.StatusOK, map[string]any{"virtuales": []any{}})
		return
	}
	ctx, cancelar := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancelar()
	l := []map[string]any{}
	for _, u := range s.op.BichosVirtuales {
		if info, err := s.infoVirtual(ctx, u); err == nil {
			b, errB := s.alm.BichoDe(ctx, info.ID, usuarioDe(r).Usuario.ID)
			l = append(l, map[string]any{"id": info.ID, "perfil": info.Perfil, "url": u, "sumado": errB == nil && b.ID != ""})
		}
	}
	responder(w, http.StatusOK, map[string]any{"virtuales": l})
}

// POST /v1/bichos/reclamar-virtual {qr} o {id}: solo en desarrollo. Hace lo
// que haría la app con un bicho virtual: lo vincula con el código del QR y le
// pasa las credenciales, por el "BLE" del bicho virtual (HTTP adentro de
// Docker). Con {id}, le pide el QR (como mantener el botón).
func (s *Servidor) reclamarVirtual(w http.ResponseWriter, r *http.Request) {
	if !s.op.Desarrollo || len(s.op.BichosVirtuales) == 0 {
		fallar(w, http.StatusNotFound, "los bichos virtuales son solo para desarrollo")
		return
	}
	var p struct {
		QR string `json:"qr"`
		ID string `json:"id"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	ctx, cancelar := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancelar()
	id := strings.ToUpper(p.ID)
	qr, ok := protocolo.LeerQr(p.QR)
	if ok {
		id = qr.ID
	} else if !protocolo.IDValido(id) {
		fallar(w, http.StatusBadRequest, "eso no es el QR de un bicho (SINTETICA:V1/<id>/<código>)")
		return
	}
	base, perfil, err := s.buscarVirtual(ctx, id)
	if err != nil {
		fallar(w, http.StatusNotFound, err.Error())
		return
	}
	if !ok {
		// Le pide el QR: el código solo sirve mientras lo muestra.
		if qr, err = s.qrVirtual(ctx, base); err != nil {
			fallar(w, http.StatusBadGateway, "el bicho virtual no mostró el QR", err.Error())
			return
		}
	}
	c, err := s.credencialNueva(ctx, qr.ID, perfil, true, usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	nubeCmd, _ := protocolo.Nube(c.URL, c.Usuario, c.Clave)
	notis, err := s.bleVirtual(ctx, base, [][2]any{{protocolo.CarComando, protocolo.Vincular(qr.Codigo)}, {protocolo.CarComando, nubeCmd}})
	if err != nil {
		fallar(w, http.StatusBadGateway, "el bicho virtual no respondió", err.Error())
		return
	}
	for _, n := range notis {
		if res, ok := protocolo.LeerResultado(n); ok && !res.Ok && (res.Op == protocolo.CmdVincular || res.Op == protocolo.CmdNube) {
			fallar(w, http.StatusConflict, "el bicho no aceptó: "+res.Mensaje, res.Detalle)
			return
		}
	}
	sumado := s.nube.EsperarReclamo(ctx, qr.ID)
	b, err := s.alm.BichoDe(r.Context(), qr.ID, usuarioDe(r).Usuario.ID)
	if !sumado && err != nil {
		fallar(w, http.StatusGatewayTimeout, "el bicho virtual no se conectó a la nube: ¿está andando EMQX?")
		return
	}
	responder(w, http.StatusOK, s.vista(r.Context(), b))
}

type infoVirtual struct {
	ID, Perfil, QR string
	Emparejando    bool
}

func (s *Servidor) infoVirtual(ctx context.Context, base string) (infoVirtual, error) {
	var info infoVirtual
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/info", nil)
	r, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return info, err
	}
	defer r.Body.Close()
	err = json.NewDecoder(r.Body).Decode(&info)
	return info, err
}

func (s *Servidor) qrVirtual(ctx context.Context, base string) (protocolo.DatosQr, error) {
	if info, err := s.infoVirtual(ctx, base); err == nil && info.Emparejando {
		if qr, ok := protocolo.LeerQr(info.QR); ok {
			return qr, nil
		}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/mostrar-qr", nil)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return protocolo.DatosQr{}, err
	}
	defer r.Body.Close()
	var p struct{ QR string }
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	qr, ok := protocolo.LeerQr(p.QR)
	if !ok {
		return qr, fmt.Errorf("QR raro: %q", p.QR)
	}
	return qr, nil
}

func (s *Servidor) buscarVirtual(ctx context.Context, id string) (base, perfil string, err error) {
	for _, u := range s.op.BichosVirtuales {
		if info, err := s.infoVirtual(ctx, u); err == nil && info.ID == id {
			return strings.TrimRight(u, "/"), info.Perfil, nil
		}
	}
	return "", "", fmt.Errorf("no encontré el bicho virtual %s (¿está corriendo?)", id)
}

// bleVirtual: una "conexión BLE" con el bicho virtual: escribe y devuelve las notificaciones.
func (s *Servidor) bleVirtual(ctx context.Context, base string, escrituras [][2]any) ([][]byte, error) {
	type esc struct {
		Car   byte   `json:"car"`
		Datos []byte `json:"datos"`
	}
	var cuerpo struct {
		Escrituras []esc `json:"escrituras"`
	}
	for _, e := range escrituras {
		cuerpo.Escrituras = append(cuerpo.Escrituras, esc{Car: byte(e[0].(int)), Datos: e[1].([]byte)})
	}
	b, _ := json.Marshal(cuerpo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/ble", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el bicho virtual respondió %d", r.StatusCode)
	}
	var resp struct {
		Notificaciones []struct {
			Car   byte   `json:"car"`
			Datos []byte `json:"datos"`
		} `json:"notificaciones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return nil, err
	}
	var l [][]byte
	for _, n := range resp.Notificaciones {
		if n.Car == protocolo.CarComando {
			l = append(l, n.Datos)
		}
	}
	return l, nil
}

// --- Config ---

func (s *Servidor) verConfig(w http.ResponseWriter, r *http.Request) {
	b := bichoDe(r)
	c, err := s.alm.UltimaConfig(r.Context(), b.ID)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		responder(w, http.StatusOK, map[string]any{"version": 0, "config": json.RawMessage(`{"version":1}`), "sincro": s.nube.EstadoSincro(b.ID)})
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"version": c.Version, "crc": c.Crc, "autor": c.Autor, "creada": c.Creada,
		"config": json.RawMessage(c.Texto), "sincro": s.nube.EstadoSincro(b.ID)})
}

// PUT /v1/bichos/{id}/config: el cuerpo es la config (JSON). Se valida con el
// mismo núcleo del bicho, se guarda como versión nueva y se sincroniza.
func (s *Servidor) guardarConfig(w http.ResponseWriter, r *http.Request) {
	texto, err := io.ReadAll(http.MaxBytesReader(w, r.Body, nucleo.MaxConfig+1))
	if err != nil {
		fallar(w, http.StatusRequestEntityTooLarge, "la config es de hasta 16 KB")
		return
	}
	// Compacta (sin perder el orden): el bicho guarda menos bytes.
	var compacto bytes.Buffer
	if err := json.Compact(&compacto, texto); err != nil {
		fallar(w, http.StatusUnprocessableEntity, "JSON inválido", err.Error())
		return
	}
	if err := nucleo.ValidarConfig(compacto.Bytes()); err != nil {
		fallar(w, http.StatusUnprocessableEntity, "el bicho no aceptaría esta config", err.Error())
		return
	}
	autor := "web"
	if usuarioDe(r).Cliente == "app" {
		autor = "app"
	}
	b := bichoDe(r)
	c, err := s.nube.GuardarConfig(r.Context(), b.ID, compacto.String(), autor, usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"version": c.Version, "crc": c.Crc, "conectado": s.nube.Conectado(b.ID)})
}

func (s *Servidor) historialConfig(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.HistorialConfig(r.Context(), bichoDe(r).ID, 50)
	if err != nil {
		s.interno(w, err)
		return
	}
	if l == nil {
		l = []almacen.Config{}
	}
	responder(w, http.StatusOK, map[string]any{"versiones": l})
}

func (s *Servidor) restaurarConfig(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Version int `json:"version"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	b := bichoDe(r)
	vieja, err := s.alm.ConfigVersion(r.Context(), b.ID, p.Version)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no hay esa versión")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	c, err := s.nube.GuardarConfig(r.Context(), b.ID, vieja.Texto, "web", usuarioDe(r).Usuario.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"version": c.Version, "crc": c.Crc})
}

// --- Imágenes ---

func espacioDe(r *http.Request) (int, bool) {
	e, err := strconv.Atoi(r.PathValue("espacio"))
	return e, err == nil && e >= 0 && e <= 7
}

func (s *Servidor) listarImagenes(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Imagenes(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	l = almacen.SoloImagenes(l)
	if l == nil {
		l = []almacen.Imagen{}
	}
	responder(w, http.StatusOK, map[string]any{"imagenes": l, "sincro": s.nube.EstadoSincro(bichoDe(r).ID)})
}

func (s *Servidor) verImagen(w http.ResponseWriter, r *http.Request) {
	e, ok := espacioDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el espacio va de 0 a 7")
		return
	}
	img, err := s.alm.LeerImagen(r.Context(), bichoDe(r).ID, e)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "ese espacio está vacío")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/qoi")
	w.Header().Set("X-Crc32", fmt.Sprintf("%08x", img.Crc))
	w.Write(img.Datos) //nolint:errcheck
}

// PUT /v1/bichos/{id}/imagenes/{espacio}: el cuerpo es un QOI del tamaño de la pantalla.
func (s *Servidor) subirImagen(w http.ResponseWriter, r *http.Request) {
	e, ok := espacioDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el espacio va de 0 a 7")
		return
	}
	datos, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 640*640*4+22))
	if err != nil {
		fallar(w, http.StatusRequestEntityTooLarge, "la imagen es muy grande")
		return
	}
	b := bichoDe(r)
	if err := s.nube.GuardarImagen(r.Context(), b.ID, e, datos); err != nil {
		if errors.Is(err, nube.ErrImagen) {
			fallar(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		s.interno(w, err)
		return
	}
	responder(w, http.StatusOK, map[string]any{"espacio": e, "largo": len(datos), "crc": protocolo.Crc32(datos)})
}

func (s *Servidor) borrarImagen(w http.ResponseWriter, r *http.Request) {
	e, ok := espacioDe(r)
	if !ok {
		fallar(w, http.StatusBadRequest, "el espacio va de 0 a 7")
		return
	}
	if err := s.nube.BorrarImagen(r.Context(), bichoDe(r).ID, e); err != nil {
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Comandos y marcos ---

// POST /v1/bichos/{id}/comando {expresion | nombre | evento | reproducir}: comandos sueltos.
func (s *Servidor) comando(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Expresion  *string `json:"expresion"`
		Nombre     *string `json:"nombre"`
		Evento     *string `json:"evento"`
		Reproducir *string `json:"reproducir"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	var m []byte
	switch {
	case p.Expresion != nil:
		m = protocolo.Expresion(*p.Expresion)
	case p.Nombre != nil:
		if len(*p.Nombre) < 1 || len(*p.Nombre) > 24 {
			fallar(w, http.StatusBadRequest, "el nombre es de 1 a 24 bytes")
			return
		}
		m = protocolo.Nombre(*p.Nombre)
	case p.Evento != nil:
		var err error
		if m, err = protocolo.Evento(*p.Evento); err != nil {
			fallar(w, http.StatusBadRequest, err.Error())
			return
		}
	case p.Reproducir != nil:
		m = protocolo.Reproducir(*p.Reproducir)
	default:
		fallar(w, http.StatusBadRequest, "mandá expresion, nombre, evento o reproducir")
		return
	}
	if err := s.nube.Enviar(bichoDe(r).ID, protocolo.CarComando, m); err != nil {
		if errors.Is(err, nube.ErrDesconectado) {
			fallar(w, http.StatusConflict, err.Error())
			return
		}
		s.interno(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /v1/bichos/{id}/marcos {car, datos (base64)}: el protocolo crudo, como
// por BLE (para las apps lejos del bicho). Encabezado X-Conexion: un id del
// cliente, para armar las transferencias de varios marcos.
func (s *Servidor) marcos(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Car   byte   `json:"car"`
		Datos []byte `json:"datos"`
	}
	if !leerJSON(w, r, 64*1024, &p) {
		return
	}
	ses := usuarioDe(r)
	conexion := r.Header.Get("X-Conexion")
	if conexion == "" || len(conexion) > 64 {
		conexion = "sesion"
	}
	conexion = ses.Usuario.ID + "/" + conexion
	autor := "web"
	if ses.Cliente == "app" {
		autor = "app"
	}
	resp, err := s.relevo.Relevar(r.Context(), ses.Usuario.ID, autor, conexion, bichoDe(r).ID, p.Car, p.Datos)
	if err != nil {
		fallar(w, http.StatusBadRequest, err.Error())
		return
	}
	if resp == nil {
		resp = []nube.Marco{}
	}
	responder(w, http.StatusOK, map[string]any{"respuestas": resp})
}

// --- Eventos en vivo (SSE) ---

func (s *Servidor) eventos(w http.ResponseWriter, r *http.Request) {
	s.transmitir(w, r, r.URL.Query().Get("bicho"))
}

func (s *Servidor) eventosBicho(w http.ResponseWriter, r *http.Request) {
	s.transmitir(w, r, bichoDe(r).ID)
}

func (s *Servidor) transmitir(w http.ResponseWriter, r *http.Request, bicho string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		fallar(w, http.StatusInternalServerError, "sin streaming")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	sub := s.hub.Suscribir(usuarioDe(r).Usuario.ID, strings.ToUpper(bicho))
	defer s.hub.Desuscribir(sub)
	escribir := func(e eventos.Evento) bool {
		b, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Tipo, b); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	// Lo primero: cómo está cada bicho ahora.
	if l, err := s.alm.BichosDe(r.Context(), usuarioDe(r).Usuario.ID); err == nil {
		for _, b := range l {
			if bicho == "" || strings.EqualFold(b.ID, bicho) {
				escribir(eventos.Evento{Tipo: "bicho", Bicho: b.ID, Datos: map[string]any{"accion": "actual", "bicho": s.vista(r.Context(), b)}})
			}
		}
	}
	latido := time.NewTicker(20 * time.Second)
	defer latido.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-latido.C:
			if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
				return
			}
			fl.Flush()
		case e := <-sub.C:
			if !escribir(e) {
				return
			}
		}
	}
}

// --- OTA ---

// POST /v1/bichos/{id}/actualizar {firmware}: manda una OTA por la nube.
func (s *Servidor) actualizar(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Firmware int64 `json:"firmware"`
	}
	if !leerJSON(w, r, 1024, &p) {
		return
	}
	b := bichoDe(r)
	f, err := s.alm.LeerFirmware(r.Context(), p.Firmware, false)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		fallar(w, http.StatusNotFound, "no hay ese firmware")
		return
	}
	if err != nil {
		s.interno(w, err)
		return
	}
	est, ok := s.nube.EstadoDe(b.ID)
	switch {
	case !s.nube.Conectado(b.ID) || !ok:
		fallar(w, http.StatusConflict, nube.ErrDesconectado.Error())
		return
	case est.Perfil != f.Perfil:
		fallar(w, http.StatusConflict, fmt.Sprintf("ese firmware es para %s y el bicho es %s", f.Perfil, est.Perfil))
		return
	case est.Actualizando:
		fallar(w, http.StatusConflict, "ya se está actualizando")
		return
	}
	u, err := s.alm.CrearActualizacion(r.Context(), b.ID, f.ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	if err := s.nube.MandarOta(r.Context(), b.ID, f); err != nil {
		s.alm.ActualizarActualizacion(r.Context(), b.ID, "fallida", 0, err.Error()) //nolint:errcheck
		fallar(w, http.StatusBadGateway, "no se la pude mandar", err.Error())
		return
	}
	responder(w, http.StatusAccepted, u)
}

func (s *Servidor) listarActualizaciones(w http.ResponseWriter, r *http.Request) {
	l, err := s.alm.Actualizaciones(r.Context(), bichoDe(r).ID)
	if err != nil {
		s.interno(w, err)
		return
	}
	if l == nil {
		l = []almacen.Actualizacion{}
	}
	responder(w, http.StatusOK, map[string]any{"actualizaciones": l})
}
