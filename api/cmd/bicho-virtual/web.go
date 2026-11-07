package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/protocolo"
)

func (b *bicho) rutas() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /", b.pagina)
	m.HandleFunc("GET /info", b.info)
	m.HandleFunc("GET /pantalla.png", b.png)
	m.HandleFunc("GET /pantalla", b.transmitir)
	m.HandleFunc("POST /ble", b.ble)
	m.HandleFunc("POST /tacto", b.tacto)
	m.HandleFunc("POST /evento", b.evento)
	m.HandleFunc("POST /mostrar-qr", b.mostrarQr)
	return m
}

func responder(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// GET /info: quién es y cómo está (la API lo usa para encontrarlo).
func (b *bicho) info(w http.ResponseWriter, _ *http.Request) {
	est, _ := protocolo.LeerEstado(nucleo.Estado())
	_, usuario, _, hayNube := nucleo.Nube()
	imagenes := map[string]int64{}
	if l, err := os.ReadDir(filepath.Join(b.dir, "archivos")); err == nil {
		for _, e := range l {
			if i, err := e.Info(); err == nil && strings.HasPrefix(e.Name(), "img") {
				imagenes[e.Name()] = i.Size()
			}
		}
	}
	responder(w, map[string]any{
		"imagenes": imagenes, "crcConfig": nucleo.CrcConfig(),
		"id": b.perfil.ID, "perfil": b.perfil.Nombre, "version": b.perfil.Version, "qr": nucleo.QR(),
		"emparejando": est.Emparejando, "nube": b.conectado.Load(), "nubeConfigurada": hayNube && usuario != "",
		"visible": nucleo.Visible(), "estado": est,
	})
}

func (b *bicho) cuadroPNG() []byte {
	ancho, alto, px := nucleo.RGBA()
	img := &image.RGBA{Pix: px, Stride: ancho * 4, Rect: image.Rect(0, 0, ancho, alto)}
	var buf bytes.Buffer
	(&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img) //nolint:errcheck
	return buf.Bytes()
}

func (b *bicho) png(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b.cuadroPNG()) //nolint:errcheck
}

// GET /pantalla: la pantalla en vivo (SSE, un PNG cada 66 ms) y el estado.
func (b *bicho) transmitir(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "sin streaming", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	t := time.NewTicker(66 * time.Millisecond)
	defer t.Stop()
	var ultimo []byte
	n := 0
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			p := b.cuadroPNG()
			if !bytes.Equal(p, ultimo) {
				ultimo = p
				fmt.Fprintf(w, "event: cuadro\ndata: %s\n\n", base64.StdEncoding.EncodeToString(p))
			}
			if n%8 == 0 {
				est, _ := protocolo.LeerEstado(nucleo.Estado())
				j, _ := json.Marshal(map[string]any{"estado": est, "qr": nucleo.QR(), "nube": b.conectado.Load(), "visible": nucleo.Visible(),
					"version": b.perfil.Version, "ota": b.otaActiva.Load()})
				fmt.Fprintf(w, "event: estado\ndata: %s\n\n", j)
			}
			n++
			fl.Flush()
		}
	}
}

// POST /ble {escrituras: [{car, datos}]}: una conexión BLE entera (conectar,
// escribir y desconectar). Devuelve todas las notificaciones.
func (b *bicho) ble(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Escrituras []notificacion `json:"escrituras"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	capturadas := []notificacion{}
	b.h.mu.Lock()
	if b.h.captura != nil {
		b.h.mu.Unlock()
		http.Error(w, "ya hay otro celu conectado", http.StatusConflict)
		return
	}
	b.h.captura = &capturadas
	b.h.mu.Unlock()
	nucleo.BleConexion(true)
	for _, e := range p.Escrituras {
		nucleo.BleEscribir(int(e.Car), e.Datos)
	}
	nucleo.BleConexion(false)
	b.h.mu.Lock()
	b.h.captura = nil
	b.h.mu.Unlock()
	responder(w, map[string]any{"notificaciones": capturadas})
}

func (b *bicho) tacto(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Apretado bool `json:"apretado"`
	}
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	nucleo.Tacto(p.Apretado)
	w.WriteHeader(http.StatusNoContent)
}

func (b *bicho) evento(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Evento string `json:"evento"`
	}
	json.NewDecoder(r.Body).Decode(&p) //nolint:errcheck
	for i, e := range protocolo.Eventos {
		if e == p.Evento {
			nucleo.Evento(i)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.Error(w, "evento desconocido", http.StatusBadRequest)
}

// POST /mostrar-qr: como mantener el botón (5 s; en el broche, 8 s).
func (b *bicho) mostrarQr(w http.ResponseWriter, _ *http.Request) {
	dur := 5200 * time.Millisecond
	if b.perfil.Broche {
		dur = 8200 * time.Millisecond
	}
	nucleo.Tacto(true)
	time.Sleep(dur)
	nucleo.Tacto(false)
	responder(w, map[string]any{"qr": nucleo.QR()})
}

func (b *bicho) pagina(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	redonda := "0"
	if b.perfil.Redonda {
		redonda = "50%"
	}
	fmt.Fprintf(w, paginaHTML, b.perfil.ID, b.perfil.Ancho, b.perfil.Alto, redonda)
}

const paginaHTML = `<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Bicho virtual</title>
<style>
:root { --fondo: #F5F2EC; --tinta: #1A1714; --suave: #5C5650; --naranja: #FF6A33; --linea: #E2DDD4; }
@media (prefers-color-scheme: dark) { :root { --fondo: #0B0A09; --tinta: #F5F2EC; --suave: #A39E96; --linea: #2C2925; } }
* { box-sizing: border-box; }
body { margin: 0; background: var(--fondo); color: var(--tinta); font: 15px/1.5 ui-monospace, "JetBrains Mono", Menlo, monospace; }
main { max-width: 760px; margin: 0 auto; padding: 32px 16px; }
h1 { font: 600 28px/1.2 Georgia, "Fraunces", serif; margin: 0 0 4px; }
.sub { color: var(--suave); margin: 0 0 24px; }
.placa { background: #000; border-radius: 22px; padding: 18px; display: inline-block; max-width: 100%%; }
img { display: block; width: min(640px, 100%%); aspect-ratio: %[2]d / %[3]d; image-rendering: auto; border-radius: %[4]s; cursor: pointer; user-select: none; }
.datos { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin: 20px 0; }
.datos div { border: 1px solid var(--linea); border-radius: 12px; padding: 10px 12px; }
.datos span { display: block; color: var(--suave); font-size: 12px; text-transform: uppercase; letter-spacing: .06em; }
button { font: inherit; background: transparent; color: var(--tinta); border: 1px solid var(--linea); border-radius: 999px; padding: 8px 14px; margin: 0 6px 8px 0; cursor: pointer; }
button:hover { border-color: var(--naranja); }
code { word-break: break-all; }
</style></head><body><main>
<h1>Bicho virtual</h1>
<p class="sub">id %[1]s · el mismo núcleo del firmware, en Docker. Tocá la pantalla (mantener = caricia).</p>
<div class="placa"><img id="pantalla" alt="Pantalla del bicho" src="/pantalla.png"></div>
<div class="datos">
 <div><span>Se ve</span><b id="visible">—</b></div>
 <div><span>Nube</span><b id="nube">—</b></div>
 <div><span>Firmware</span><b id="version">—</b></div>
 <div><span>Humor</span><b id="humor">—</b></div>
</div>
<p><span style="color:var(--suave)">QR:</span> <code id="qr">—</code></p>
<p>
 <button data-evento="doble-toque">Doble toque</button><button data-evento="levantado">Levantar</button>
 <button data-evento="sacudida">Sacudir</button><button data-evento="boca-abajo">Boca abajo</button>
 <button id="mostrar-qr">Mostrar el QR (mantener el botón)</button>
</p>
<p class="sub">Para sumarlo a tu cuenta: en la web, "Sumar un bicho" → "Bicho virtual".</p>
</main>
<script>
const $ = (id) => document.getElementById(id);
const fuente = new EventSource('/pantalla');
fuente.addEventListener('cuadro', (e) => { $('pantalla').src = 'data:image/png;base64,' + e.data; });
fuente.addEventListener('estado', (e) => {
  const d = JSON.parse(e.data);
  $('visible').textContent = d.visible + (d.ota ? ' · actualizando' : '');
  $('nube').textContent = d.nube ? 'conectado' : 'sin conectar';
  $('version').textContent = d.version;
  $('humor').textContent = (d.estado.humor >= 0 ? '+' : '') + d.estado.humor.toFixed(2);
  $('qr').textContent = d.estado.emparejando ? d.qr : '(no lo muestra: ya está vinculado)';
});
const post = (ruta, cuerpo) => fetch(ruta, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cuerpo ?? {}) });
$('pantalla').addEventListener('pointerdown', () => post('/tacto', { apretado: true }));
$('pantalla').addEventListener('pointerup', () => post('/tacto', { apretado: false }));
$('pantalla').addEventListener('pointerleave', () => post('/tacto', { apretado: false }));
for (const b of document.querySelectorAll('[data-evento]')) b.addEventListener('click', () => post('/evento', { evento: b.dataset.evento }));
$('mostrar-qr').addEventListener('click', async () => { $('mostrar-qr').disabled = true; await post('/mostrar-qr'); $('mostrar-qr').disabled = false; });
</script></body></html>`
