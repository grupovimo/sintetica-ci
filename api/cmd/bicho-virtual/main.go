// El bicho virtual: SintéticaOS entero (el mismo núcleo del firmware) en un
// contenedor, con "WiFi" y MQTT, OTA verificada y una página para verlo y
// tocarlo. Su "BLE" es HTTP: POST /ble con las escrituras.
//
// Variables: BICHO_ID (si no, uno al azar que se guarda en /datos), BICHO_FORMA
// (147b o broche), DATOS, DIRECCION (:8090), CLAVE_PUBLICA (para las OTA),
// BICHO_VERSION.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/firma"
	"sintetica/api/internal/protocolo"
)

var zonaArgentina = time.FixedZone("ART", -3*3600)

func leerAzar(b []byte) { rand.Read(b) } //nolint:errcheck

func env(clave, porDefecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return porDefecto
}

type entradaNube struct {
	car   byte
	datos []byte
}

type bicho struct {
	log     *slog.Logger
	h       *hal
	perfil  nucleo.Perfil
	dir     string
	publica []byte

	cli        atomic.Pointer[mqtt.Client]
	conectado  atomic.Bool
	reconectar func()
	entrada    chan entradaNube
	reiniciar  chan struct{}

	otaMu     sync.Mutex
	otaCancel context.CancelFunc
	otaActiva atomic.Bool
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dir := env("DATOS", "/datos")
	for _, d := range []string{"kv", "archivos"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			log.Error("no puedo escribir en DATOS", "dir", dir, "error", err)
			os.Exit(1)
		}
	}
	b := &bicho{log: log, dir: dir, entrada: make(chan entradaNube, 512), reiniciar: make(chan struct{}, 1)}
	b.perfil = nucleo.Perfil{ID: idDelBicho(dir), Nombre: "virtual", Version: versionInstalada(dir), Ancho: 320, Alto: 172}
	if env("BICHO_FORMA", "147b") == "broche" {
		b.perfil.Nombre, b.perfil.Ancho, b.perfil.Alto, b.perfil.Redonda, b.perfil.Broche = "virtual-broche", 240, 240, true, true
	}
	if p, err := os.ReadFile(env("CLAVE_PUBLICA", "/claves/ota-publica.pem")); err == nil {
		b.publica = p
	} else {
		log.Warn("sin clave pública: rechazo toda OTA (npm run claves)")
	}
	b.h = &hal{b: b, inicio: time.Now(), dir: dir}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	b.arrancar()
	log.Info("bicho virtual andando", "id", b.perfil.ID, "perfil", b.perfil.Nombre, "version", b.perfil.Version, "qr", nucleo.QR())

	srv := &http.Server{Addr: env("DIRECCION", ":8090"), Handler: b.rutas(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP", "error", err)
			parar()
		}
	}()
	go b.mantenerNube(ctx)

	cuadro := time.NewTicker(33 * time.Millisecond)
	defer cuadro.Stop()
	for {
		select {
		case <-ctx.Done():
			srv.Close()
			return
		case <-cuadro.C:
			nucleo.Tick()
		case e := <-b.entrada:
			nucleo.NubeEscribir(int(e.car), e.datos)
		case <-b.reiniciar:
			// Como al prenderlo de nuevo (después de una OTA).
			b.perfil.Version = versionInstalada(dir)
			b.h.inicio = time.Now()
			b.arrancar()
			log.Info("reinicié", "version", b.perfil.Version)
			b.reconectar()
		}
	}
}

func (b *bicho) arrancar() {
	nucleo.Iniciar(b.h, b.perfil)
	nucleo.Conexiones(true, b.conectado.Load())
}

func idDelBicho(dir string) string {
	if id := strings.ToUpper(os.Getenv("BICHO_ID")); protocolo.IDValido(id) {
		return id
	}
	ruta := filepath.Join(dir, "id")
	if b, err := os.ReadFile(ruta); err == nil && protocolo.IDValido(strings.TrimSpace(string(b))) {
		return strings.TrimSpace(string(b))
	}
	var mac [6]byte
	leerAzar(mac[:])
	mac[0] = mac[0]&0xFC | 0x02 // una MAC "administrada localmente": no choca con una placa real
	id := fmt.Sprintf("%02X%02X%02X%02X%02X%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
	os.WriteFile(ruta, []byte(id), 0o644) //nolint:errcheck
	return id
}

func versionInstalada(dir string) string {
	if v, err := os.ReadFile(filepath.Join(dir, "version")); err == nil && len(bytes.TrimSpace(v)) > 0 {
		return string(bytes.TrimSpace(v))
	}
	return env("BICHO_VERSION", nucleo.VersionNucleo())
}

// --- Nube (MQTT) ---

func (b *bicho) mantenerNube(ctx context.Context) {
	cambio := make(chan struct{}, 1)
	b.reconectar = func() {
		select {
		case cambio <- struct{}{}:
		default:
		}
	}
	for {
		url, usuario, clave, ok := nucleo.Nube()
		var cli mqtt.Client
		if ok {
			cli = b.conectarMqtt(url, usuario, clave)
		} else {
			b.log.Info("sin nube configurada: esperando que me sumen a una cuenta")
		}
		select {
		case <-ctx.Done():
			if cli != nil {
				cli.Disconnect(200)
			}
			return
		case <-cambio:
			if cli != nil {
				b.conectado.Store(false)
				cli.Disconnect(300)
			}
			time.Sleep(500 * time.Millisecond) // que salga la respuesta del comando antes
		}
	}
}

func (b *bicho) conectarMqtt(url, usuario, clave string) mqtt.Client {
	broker := strings.Replace(strings.Replace(url, "mqtts://", "ssl://", 1), "mqtt://", "tcp://", 1)
	id := b.perfil.ID
	conexion := "bicho/" + id + "/conexion"
	o := mqtt.NewClientOptions().AddBroker(broker).SetClientID(id).SetUsername(usuario).SetPassword(clave).
		SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(5*time.Second).
		SetKeepAlive(30*time.Second).SetCleanSession(true).SetWill(conexion, "0", 1, true)
	o.SetOnConnectHandler(func(c mqtt.Client) {
		b.log.Info("conectado a la nube", "broker", broker)
		tk := c.Subscribe("bicho/"+id+"/cmd", 1, func(_ mqtt.Client, m mqtt.Message) {
			p := m.Payload()
			if len(p) >= 2 {
				b.entrada <- entradaNube{car: p[0], datos: append([]byte(nil), p[1:]...)}
			}
		})
		// Se anuncia recién con la suscripción confirmada: si no, lo primero
		// que le manda la API se pierde.
		tk.WaitTimeout(10 * time.Second)
		c.Publish(conexion, 1, true, "1")
		b.conectado.Store(true)
		nucleo.Conexiones(true, true)
		// El estado retenido, ya: el núcleo solo lo notifica cuando cambia algo.
		c.Publish("bicho/"+id+"/state", 1, true, nucleo.Estado())
	})
	o.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		b.log.Warn("se cortó la nube", "error", err)
		b.conectado.Store(false)
		nucleo.Conexiones(true, false)
	})
	cli := mqtt.NewClient(o)
	b.cli.Store(&cli)
	cli.Connect()
	return cli
}

// publicar lo llama el núcleo (con su mutex): no esperar a nada acá.
func (b *bicho) publicar(car byte, d []byte) {
	p := b.cli.Load()
	if p == nil || !b.conectado.Load() {
		return
	}
	cli := *p
	if car == protocolo.CarEstado {
		cli.Publish("bicho/"+b.perfil.ID+"/state", 1, true, append([]byte(nil), d...))
		return
	}
	cli.Publish("bicho/"+b.perfil.ID+"/resp", 0, false, append([]byte{car}, d...))
}

// --- OTA ---

func (b *bicho) empezarOta(m nucleo.Manifiesto, url string) int {
	if b.otaActiva.Load() {
		return protocolo.Ocupado
	}
	pm := protocolo.Manifiesto{Tamano: m.Tamano, Sha256: m.Sha256, Firma: m.Firma, Version: m.Version, Perfil: m.Perfil}
	if len(b.publica) == 0 || !firma.Verificar(b.publica, pm) {
		b.log.Warn("OTA rechazada: la firma no es de Sintética Labs")
		return protocolo.FirmaInvalida
	}
	ctx, cancelar := context.WithCancel(context.Background())
	b.otaMu.Lock()
	b.otaCancel = cancelar
	b.otaMu.Unlock()
	b.otaActiva.Store(true)
	go b.descargar(ctx, pm, url)
	return protocolo.Ok
}

func (b *bicho) cancelarOta() {
	b.otaMu.Lock()
	defer b.otaMu.Unlock()
	if b.otaCancel != nil {
		b.otaCancel()
	}
}

func (b *bicho) descargar(ctx context.Context, m protocolo.Manifiesto, url string) {
	defer b.otaActiva.Store(false)
	fallar := func(pct int, msg string) {
		b.log.Warn("OTA fallida", "detalle", msg)
		nucleo.OtaProgreso(protocolo.FalloOta, pct, msg)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	r, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		fallar(0, "no me pude conectar")
		return
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		fallar(0, fmt.Sprintf("el servidor respondió %d", r.StatusCode))
		return
	}
	h := sha256.New()
	var bin bytes.Buffer
	buf := make([]byte, 16*1024)
	ultimo := 0
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			bin.Write(buf[:n])
			if bin.Len() > int(m.Tamano) {
				fallar(ultimo, "otro tamaño que el firmado")
				return
			}
			if pct := bin.Len() * 99 / int(m.Tamano); pct >= ultimo+5 {
				ultimo = pct
				nucleo.OtaProgreso(protocolo.Ok, pct, "")
				time.Sleep(150 * time.Millisecond) // que se vea el progreso
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fallar(ultimo, "se cortó la descarga")
			return
		}
	}
	var suma [32]byte
	copy(suma[:], h.Sum(nil))
	if uint32(bin.Len()) != m.Tamano || suma != m.Sha256 {
		nucleo.OtaProgreso(protocolo.HashDistinto, ultimo, "")
		return
	}
	// "Instalar": la versión nueva queda a prueba. Un binario que empieza con
	// FALLAR simula un firmware roto: arranca y vuelve solo a la versión anterior.
	anterior := b.perfil.Version
	if bytes.HasPrefix(bin.Bytes(), []byte("FALLAR")) {
		b.log.Warn("el firmware nuevo no anda: vuelvo al anterior", "anterior", anterior)
	} else if err := escribirAtomico(filepath.Join(b.dir, "version"), []byte(m.Version)); err != nil {
		fallar(ultimo, "no pude guardar")
		return
	}
	nucleo.OtaProgreso(protocolo.Ok, 100, "")
	b.log.Info("OTA instalada; reinicio", "version", m.Version)
	time.Sleep(1500 * time.Millisecond)
	b.reiniciar <- struct{}{}
}
