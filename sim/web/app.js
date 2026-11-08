// La página del simulador: conecta el núcleo (sim.js) con la pantalla, los
// botones de sensores y el panel BLE, que manda los mismos bytes que la app.
import { almacenLocal, crearSimulador } from './sim.js';
import {
  ArmadorDeConfig, Car, comando, escriturasDeConfig, escriturasDeImagen, Eventos, imagen, leerEstado,
  leerCharla, leerListaImagenes, leerQr, leerRespuesta, pedirConfig,
} from './protocolo.js';
import { codificarQoi } from './qoi.js';

const $ = (id) => document.getElementById(id);
const lienzo = $('lienzo');
const ctx = lienzo.getContext('2d');
let datosPantalla = ctx.createImageData(320, 172);
const registro = $('registro');

// La placa que se simula: la 1.47B (mascota) o el broche. Cada una con su memoria.
let perfil = 'mascota';
try { perfil = localStorage.getItem('sinteticaos-perfil') === 'broche' ? 'broche' : 'mascota'; } catch { /* sin almacenamiento */ }
const almacenes = { mascota: almacenLocal('sinteticaos:'), broche: almacenLocal('sinteticaos:broche:') };
let almacen = almacenes[perfil];
const PANTALLA = { mascota: [320, 172], broche: [240, 240] };
let espacio = 0;            // el espacio de imagen elegido
let ocupados = 0;           // máscara de espacios con imagen

const NOMBRES_CAR = { 1: 'config', 2: 'comando', 3: 'estado', 4: 'ota', 5: 'imagen' };
const NOMBRES_OP = {
  0x10: 'expresión', 0x11: 'nombre', 0x12: 'evento', 0x13: 'vincular', 0x14: 'autenticar', 0x15: 'desvincular',
  0x16: 'WiFi', 0x17: 'nube', 0x18: 'olvidar red', 0x19: 'reproducir', 0x25: 'pedir imagen',
  0x01: 'inicio', 0x02: 'bloque', 0x03: 'fin', 0x04: 'pedir',
  0x20: 'inicio', 0x21: 'bloque', 0x22: 'fin', 0x23: 'borrar', 0x24: 'listar',
};

// La clave que el bicho le dio a "este celu" al vincular (como la guardaría la app).
const claveCelu = () => `sinteticaos-celu:clave:${perfil}`;
let claveEnMemoria = null;  // por si el navegador no deja usar localStorage
function claveGuardada() {
  try { const s = localStorage.getItem(claveCelu()); if (s) return Uint8Array.from(atob(s), (c) => c.charCodeAt(0)); } catch { /* sin almacenamiento */ }
  return claveEnMemoria;
}
function guardarClave(clave) {
  claveEnMemoria = clave;
  try { if (clave) localStorage.setItem(claveCelu(), btoa(String.fromCharCode(...clave))); else localStorage.removeItem(claveCelu()); } catch { /* sin almacenamiento */ }
}

let sim = null;
let bateria = -1;
let bocaAbajo = false;
let sacudiendo = 0;
let levantando = 0;
let armador = new ArmadorDeConfig();

const hex = (b, max = 24) => Array.from(b.slice(0, max), (x) => x.toString(16).padStart(2, '0')).join(' ') + (b.length > max ? ` … (${b.length} bytes)` : '');

function anotar(texto, clase = '') {
  const linea = document.createElement('div');
  if (clase) linea.className = clase;
  linea.textContent = texto;
  registro.append(linea);
  while (registro.childElementCount > 200) registro.firstChild.remove();
  registro.scrollTop = registro.scrollHeight;
}

function escribir(car, bytes, comentario = '') {
  const op = NOMBRES_OP[bytes[0]] ?? '';
  anotar(`→ ${NOMBRES_CAR[car]} ${op}  ${hex(bytes)}${comentario ? '  ' + comentario : ''}`, 'ida');
  sim.bleEscribir(car, bytes);
}

// La voz propia: el núcleo manda PCM de 16 kHz y acá se encola en WebAudio.
let audio = null, audioHasta = 0, conVoz = true;
function tocarVoz(m) {
  if (!conVoz || typeof AudioContext === 'undefined') return;
  if (!audio) audio = new AudioContext();
  const buf = audio.createBuffer(1, m.length, 16000);
  const d = buf.getChannelData(0);
  for (let i = 0; i < m.length; i++) d[i] = m[i] / 32768;
  const src = audio.createBufferSource();
  src.buffer = buf;
  src.connect(audio.destination);
  const t = Math.max(audio.currentTime + 0.02, audioHasta);
  src.start(t);
  audioHasta = t + m.length / 16000;
}

function alNotificar(car, b) {
  if (car === Car.estado) { mostrarEstado(leerEstado(b)); return; }
  const c = car === Car.comando ? leerCharla(b) : null;
  if (c) {
    const que = c.noEntendi ? 'no entendió' : `${c.intencion}${c.accion !== 'ninguna' ? ` → ${c.accion}` : ''}${c.falta ? ' (falta)' : ''}`;
    $('v-charla').textContent = `${que} · ${c.confianza} %`;
    anotar(`← charla  ${que}  «${c.frase}»${c.texto ? `  [${c.texto}]` : ''}${c.minuto >= 0 ? `  ${Math.floor(c.minuto / 60)}:${String(c.minuto % 60).padStart(2, '0')}` : ''}`);
    return;
  }
  if (car === Car.imagen && b[0] === 0xa4 && b[1] === 0) {
    const l = leerListaImagenes(b);
    ocupados = l.mascara;
    armarEspacios();
    anotar(`← imagen listar  ${l.espacios.length ? l.espacios.map((e) => `${e.espacio}: ${(e.largo / 1024).toFixed(1)} KB`).join(', ') : 'sin imágenes'}`);
    return;
  }
  const r = leerRespuesta(b);
  if (r.tipo === 'trozo') { armador.agregar(r); return; }
  if (r.tipo === 'cierre') {
    try {
      const config = armador.agregar(r);
      $('t-config').value = JSON.stringify(config, null, 2);
      anotar(`← config  ${r.largo} bytes, CRC ${r.crc.toString(16)}: cargada en el editor`);
    } catch (e) { anotar(`← config  ${e.message}`, 'mal'); }
    return;
  }
  if (car === Car.config && r.op === 0x03 && r.ok) armarAnimaciones();
  if (r.clave) { guardarClave(r.clave); anotar('· este celu quedó vinculado: guardé la clave'); }
  if (r.op === 0x15 && r.ok) guardarClave(null);
  const op = NOMBRES_OP[r.op] ?? `0x${r.op.toString(16)}`;
  anotar(`← ${NOMBRES_CAR[car]} ${op}  ${r.mensaje}${r.detalle ? ': ' + r.detalle : ''}`, r.ok ? '' : 'mal');
}

function mostrarEstado(e) {
  $('f-nombre').textContent = e.nombre;
  $('f-expresion').textContent = e.expresion + (e.fija ? ' (fija)' : '');
  $('f-humor').textContent = (e.humor >= 0 ? '+' : '') + e.humor.toFixed(2);
  $('f-estado').textContent = e.emparejando ? 'mostrando el QR'
    : e.dormido ? 'durmiendo' : (e.bateria == null ? 'despierto' : `despierto · ${e.bateria}%`);
  $('placa-serie').textContent = `${e.serie ? 'Unidad #' + e.serie : 'Sin número'} · ${e.nombre} · SintéticaOS ${sim?.version() ?? ''}`;
  $('b-vincular').disabled = !e.emparejando;
  $('b-auto').setAttribute('aria-pressed', String(!e.fija));
  for (const b of $('animaciones').children) b.setAttribute('aria-pressed', String(e.expresion === 'animacion:' + b.dataset.nombre));
  for (const b of $('caras').children) b.setAttribute('aria-pressed', String(e.fija && b.dataset.nombre === e.expresion));
}

function armarEspacios() {
  const caja = $('espacios');
  caja.replaceChildren();
  for (let i = 0; i < 8; i++) {
    const b = document.createElement('button');
    b.textContent = String(i);
    b.className = ocupados & (1 << i) ? 'ocupado' : '';
    b.setAttribute('aria-pressed', String(i === espacio));
    b.title = ocupados & (1 << i) ? `Espacio ${i}: con imagen` : `Espacio ${i}: vacío`;
    b.addEventListener('click', () => { espacio = i; armarEspacios(); });
    caja.append(b);
  }
}

// Recorta la imagen a la pantalla (cubriendo, centrada; redonda si es el broche) y la codifica en QOI.
function prepararImagen(fuente) {
  const [w, h] = PANTALLA[perfil];
  const c = document.createElement('canvas');
  c.width = w; c.height = h;
  const g = c.getContext('2d');
  g.fillStyle = '#0B0A09';
  g.fillRect(0, 0, w, h);
  const fw = fuente.width, fh = fuente.height;
  const k = Math.max(w / fw, h / fh);
  g.save();
  if (perfil === 'broche') { g.beginPath(); g.arc(w / 2, h / 2, w / 2, 0, Math.PI * 2); g.clip(); }
  g.drawImage(fuente, (w - fw * k) / 2, (h - fh * k) / 2, fw * k, fh * k);
  g.restore();
  return codificarQoi(g.getImageData(0, 0, w, h).data, w, h);
}

// Una imagen de ejemplo hecha acá mismo: un campo naranja con la cara del isotipo.
function imagenDeEjemplo() {
  const [w, h] = PANTALLA[perfil];
  const c = document.createElement('canvas');
  c.width = w; c.height = h;
  const g = c.getContext('2d');
  const fondo = g.createLinearGradient(0, 0, w, h);
  fondo.addColorStop(0, '#FF6A33');
  fondo.addColorStop(1, '#B83C0C');
  g.fillStyle = fondo;
  g.fillRect(0, 0, w, h);
  const s = Math.min(w, h) / 172;
  g.fillStyle = '#0B0A09';
  const pildora = (x, y, ww, hh) => { g.beginPath(); g.roundRect(x, y, ww, hh, ww / 2); g.fill(); };
  pildora(w / 2 - 60 * s, h / 2 - 36 * s, 36 * s, 72 * s);
  g.beginPath();  // el ojo con el corte 1:2
  g.moveTo(w / 2 + 24 * s, h / 2 - 36 * s); g.lineTo(w / 2 + 60 * s, h / 2 - 18 * s);
  g.lineTo(w / 2 + 60 * s, h / 2 + 18 * s); g.arc(w / 2 + 42 * s, h / 2 + 18 * s, 18 * s, 0, Math.PI); g.closePath(); g.fill();
  return c;
}

async function subirImagen(qoi) {
  const escrituras = escriturasDeImagen(espacio, qoi, 180);
  const barra = $('progreso');
  barra.hidden = false;
  anotar(`· imagen QOI de ${(qoi.length / 1024).toFixed(1)} KB al espacio ${espacio}, en ${escrituras.length - 2} bloques`);
  for (let i = 0; i < escrituras.length; i++) {
    const e = escrituras[i];
    if (i === 0 || i === escrituras.length - 1) escribir(Car.imagen, e); else sim.bleEscribir(Car.imagen, e);
    barra.value = (i + 1) / escrituras.length;
    if (i % 40 === 0) await new Promise((r) => setTimeout(r, 0));  // que se vea el progreso
  }
  barra.hidden = true;
  escribir(Car.imagen, imagen.listar());
}

function armarCaras() {
  const caja = $('caras');
  caja.replaceChildren();
  for (const nombre of sim.caras()) {
    const b = document.createElement('button');
    b.textContent = nombre;
    b.dataset.nombre = nombre;
    b.setAttribute('aria-pressed', 'false');
    b.addEventListener('click', () => escribir(Car.comando, comando.expresion(nombre)));
    caja.append(b);
  }
}

function armarAnimaciones() {
  const caja = $('animaciones');
  caja.replaceChildren();
  const nombres = sim.animaciones();
  $('sin-animaciones').hidden = nombres.length > 0;
  for (const nombre of nombres) {
    const b = document.createElement('button');
    b.textContent = nombre;
    b.dataset.nombre = nombre;
    b.addEventListener('click', () => escribir(Car.comando, comando.reproducir(nombre)));
    caja.append(b);
  }
}

async function cargarWasm() {
  if (globalThis.SINTETICA_WASM_B64) return Uint8Array.from(atob(globalThis.SINTETICA_WASM_B64), (c) => c.charCodeAt(0));
  const r = await fetch('../dist/sinteticaos.wasm');
  if (!r.ok) throw new Error('No encontré sim/dist/sinteticaos.wasm: corré sh sim/compilar.sh');
  return new Uint8Array(await r.arrayBuffer());
}

async function cargarEjemplo() {
  if (globalThis.SINTETICA_EJEMPLO) return globalThis.SINTETICA_EJEMPLO;
  try { return await (await fetch('../../shared/formato/config-ejemplo.json')).text(); } catch { return '{"version": 1}'; }
}

let wasm = null;
async function arrancar() {
  armador = new ArmadorDeConfig();
  const [w, h] = PANTALLA[perfil];
  lienzo.width = w; lienzo.height = h;
  datosPantalla = ctx.createImageData(w, h);
  document.querySelector('.bicho').dataset.forma = perfil === 'broche' ? 'redonda' : 'rectangular';
  for (const el of document.querySelectorAll('.solo-mascota')) el.hidden = perfil === 'broche';
  for (const el of document.querySelectorAll('.solo-broche')) el.hidden = perfil !== 'broche';
  $('p-mascota').setAttribute('aria-pressed', String(perfil === 'mascota'));
  $('p-broche').setAttribute('aria-pressed', String(perfil === 'broche'));
  $('pantalla').title = perfil === 'broche' ? 'Toque: siguiente · 2 s: dormir · 8 s: QR' : 'Tocar (mantener = caricia)';
  sim = await crearSimulador({
    wasm,
    almacen,
    perfil,
    bateria: () => bateria,
    alPresentar: (rgba) => { datosPantalla.data.set(rgba); ctx.putImageData(datosPantalla, 0, 0); },
    alNotificar,
    alRegistrar: (m) => anotar(`· ${m}`),
    alSonido: (s) => anotar(`♪ sonido: ${s}`),
    alLed: (rgb) => { $('led').style.background = `#${rgb.toString(16).padStart(6, '0')}`; },
    alAudio: tocarVoz,
  });
  armarCaras();
  armarAnimaciones();
  armarVida();
  sim.bleConexion(true);
  const clave = claveGuardada();
  if (clave && !sim.emparejando()) escribir(Car.comando, comando.autenticar(clave), '(clave guardada de este celu)');
  sim.tick();
  mostrarEstado(leerEstado(sim.bleLeerEstado()));
  ocupados = 0;
  armarEspacios();
  if (!sim.emparejando()) escribir(Car.imagen, imagen.listar());
}

// El IMU de mentira: 50 lecturas por segundo, quieto salvo que se aprieten botones.
function leerImu() {
  if (!sim) return;
  if (sacudiendo > 0) {
    sacudiendo--;
    const s = sacudiendo % 2 ? 2.6 : -2.6;
    sim.imu(s, s * 0.4, 1);
  } else if (levantando > 0) {
    levantando--;
    sim.imu(0, 0.1, 1.3);
  } else {
    sim.imu(0, 0, bocaAbajo ? -1 : 1);
  }
}

const NOMBRES_ESTADO = ['energía', 'ánimo', 'afecto', 'aburrimiento', 'curiosidad', 'sueño', 'estrés'];
const NOMBRES_RASGOS = ['energía', 'sociabilidad', 'picardía', 'timidez', 'curiosidad', 'mal genio'];
function barras(caja, nombres, valores) {
  if (caja.childElementCount !== nombres.length) {
    caja.replaceChildren(...nombres.map((n) => {
      const d = document.createElement('div');
      d.className = 'barra';
      d.innerHTML = `<span>${n}</span><i><b></b></i><span></span>`;
      return d;
    }));
  }
  valores.forEach((v, i) => {
    const fila = caja.children[i];
    fila.querySelector('b').style.width = `${Math.round(v * 100)}%`;
    fila.lastChild.textContent = v.toFixed(2);
  });
}

function armarVida() {
  $('eventos').replaceChildren(...sim.eventos().map((n, i) => {
    const b = document.createElement('button');
    b.textContent = n;
    b.addEventListener('click', () => escribir(Car.comando, comando.evento(i), `(${n})`));
    return b;
  }));
  fetch('../../shared/formato/vida/comportamientos.json').then((r) => r.json()).then((c) => {
    $('s-comportamiento').replaceChildren(...Object.keys(c).map((n) => Object.assign(document.createElement('option'), { value: n, textContent: n })));
  }).catch(() => {});
}

let ultimoPanel = 0;
function cuadro(t) {
  if (sim) {
    sim.tick();
    if (t - ultimoPanel > 200) {
      ultimoPanel = t;
      const v = sim.vida();
      barras($('barras-estado'), NOMBRES_ESTADO, v.estado);
      barras($('barras-rasgos'), NOMBRES_RASGOS, v.rasgos);
      $('v-comportamiento').textContent = v.dormido ? (v.sonando ? 'soñando' : 'durmiendo') : (v.comportamiento || 'nada en especial');
      $('v-candidatos').textContent = v.candidatos.length
        ? '· el último sorteo: ' + v.candidatos.map((c) => `${c.nombre} ${c.puntaje.toFixed(2)}`).join(', ') : '';
      const { minuto } = sim.hora();
      $('v-hora').textContent = `${String(Math.floor(minuto / 60)).padStart(2, '0')}:${String(minuto % 60).padStart(2, '0')}` +
        (sim.velocidad() !== 1 ? ` · × ${sim.velocidad()}` : '');
    }
  }
  requestAnimationFrame(cuadro);
}

function enlazar() {
  // Pantalla y botón Tocar: apretar y soltar de verdad, así funcionan el doble toque y la caricia.
  for (const el of [$('pantalla'), $('b-tocar')]) {
    el.addEventListener('pointerdown', (e) => { e.preventDefault(); sim?.tacto(true); });
    for (const ev of ['pointerup', 'pointerleave', 'pointercancel']) el.addEventListener(ev, () => sim?.tacto(false));
  }
  $('b-doble').addEventListener('click', () => {
    sim.tacto(true); setTimeout(() => sim.tacto(false), 70);
    setTimeout(() => sim.tacto(true), 160); setTimeout(() => sim.tacto(false), 230);
  });
  $('b-sacudir').addEventListener('click', () => { sacudiendo = 24; });
  $('b-levantar').addEventListener('click', () => { levantando = 6; });
  $('b-abajo').addEventListener('click', (e) => {
    bocaAbajo = !bocaAbajo;
    e.currentTarget.setAttribute('aria-pressed', String(bocaAbajo));
  });
  // Saltar en el tiempo: el bicho vive lo que pasó en el medio (de golpe).
  const irA = (minuto) => {
    const { minuto: ahora } = sim.hora();
    const falta = ((minuto - ahora) % 1440 + 1440) % 1440;
    sim.adelantar(falta * 60000);
    anotar(`· adelanto ${Math.floor(falta / 60)} h ${falta % 60} min`);
  };
  $('b-noche').addEventListener('click', () => irA(23 * 60 + 25));
  $('b-manana').addEventListener('click', () => irA(7 * 60 + 58));
  for (const b of document.querySelectorAll('[data-velocidad]')) {
    b.addEventListener('click', () => {
      sim.velocidad(Number(b.dataset.velocidad));
      for (const o of document.querySelectorAll('[data-velocidad]')) o.setAttribute('aria-pressed', String(o === b));
    });
  }
  $('b-depurar').addEventListener('click', (e) => {
    const si = e.currentTarget.getAttribute('aria-pressed') !== 'true';
    e.currentTarget.setAttribute('aria-pressed', String(si));
    escribir(Car.comando, comando.vida(si));
  });
  $('b-comportamiento').addEventListener('click', () => {
    const n = $('s-comportamiento').value;
    if (n && !sim.forzarComportamiento(n)) anotar(`· no pude empezar ${n}`, 'mal');
  });
  // La mirada sigue al mouse sobre la pantalla.
  $('pantalla').addEventListener('pointermove', (e) => {
    const r = e.currentTarget.getBoundingClientRect();
    sim?.seguir(((e.clientX - r.left) / r.width) * 2 - 1, -(((e.clientY - r.top) / r.height) * 2 - 1));
  });
  $('r-bateria').addEventListener('input', (e) => {
    bateria = Number(e.target.value);
    $('v-bateria').textContent = bateria < 0 ? 'sin batería' : `${bateria}%`;
  });
  $('b-auto').addEventListener('click', () => escribir(Car.comando, comando.expresion('')));
  $('b-parar').addEventListener('click', () => escribir(Car.comando, comando.reproducir('')));
  $('b-nombre').addEventListener('click', () => {
    const n = $('i-nombre').value.trim();
    if (n) escribir(Car.comando, comando.nombre(n));
  });
  $('b-mandar').addEventListener('click', () => {
    let json;
    try { json = JSON.stringify(JSON.parse($('t-config').value)); } catch (e) {
      anotar(`El editor no tiene JSON válido: ${e.message}`, 'mal');
      return;
    }
    const escrituras = escriturasDeConfig(json, 180);
    escrituras.forEach((b, i) => escribir(Car.config, b, i === 0 ? `(${json.length} bytes en ${escrituras.length - 2} bloques)` : ''));
  });
  $('b-pedir').addEventListener('click', () => escribir(Car.config, pedirConfig()));
  for (const p of ['mascota', 'broche']) {
    $(`p-${p}`).addEventListener('click', async () => {
      if (perfil === p) return;
      perfil = p;
      almacen = almacenes[p];
      try { localStorage.setItem('sinteticaos-perfil', p); } catch { /* sin almacenamiento */ }
      anotar(`· ahora simulo ${p === 'broche' ? 'el broche (redondo, 240 × 240)' : 'la 1.47B (320 × 172)'}`);
      await arrancar();
    });
  }
  $('i-archivo').addEventListener('change', async (e) => {
    const archivo = e.target.files?.[0];
    if (!archivo) return;
    try { await subirImagen(prepararImagen(await createImageBitmap(archivo))); } catch (err) { anotar(`No pude leer la imagen: ${err.message}`, 'mal'); }
    e.target.value = '';
  });
  $('b-ejemplo').addEventListener('click', () => subirImagen(prepararImagen(imagenDeEjemplo())));
  $('b-borrar-imagen').addEventListener('click', () => { escribir(Car.imagen, imagen.borrar(espacio)); escribir(Car.imagen, imagen.listar()); });
  $('b-lista').addEventListener('click', () => {
    let config;
    try { config = JSON.parse($('t-config').value); } catch (e) { anotar(`El editor no tiene JSON válido: ${e.message}`, 'mal'); return; }
    config.lista = $('i-lista').value.split(',').map((x) => x.trim()).filter(Boolean);
    config.rotar = Math.max(0, Math.min(1440, parseInt($('i-rotar').value, 10) || 0));
    $('t-config').value = JSON.stringify(config, null, 2);
    const json = JSON.stringify(config);
    escriturasDeConfig(json, 180).forEach((b, i) => escribir(Car.config, b, i === 0 ? `(lista: ${config.lista.join(', ')})` : ''));
  });
  $('b-vincular').addEventListener('click', () => {
    const qr = leerQr(sim.qr());
    anotar(`· escaneé el QR: ${sim.qr()}`);
    if (qr) escribir(Car.comando, comando.vincular(qr.codigo));
  });
  $('b-desvincular').addEventListener('click', () => escribir(Car.comando, comando.desvincular()));
  $('b-serie').addEventListener('click', () => {
    const s = $('i-serie').value.trim();
    if (s) { sim.fijarSerie(s); anotar(`· taller: unidad #${s} grabada (por USB, no por BLE)`); }
  });
  $('b-reiniciar').addEventListener('click', async () => { anotar('· reinicio: la memoria queda'); await arrancar(); });
  $('b-borrar').addEventListener('click', async () => { almacen.borrar(); guardarClave(null); anotar('· memoria del bicho y clave del celu borradas'); await arrancar(); });

  document.addEventListener('keydown', (e) => {
    if (e.target.closest('input, textarea, select') || e.repeat) return;
    if (e.code === 'Space') { e.preventDefault(); sim?.tacto(true); }
    if (e.key === 's' || e.key === 'S') sacudiendo = 24;
  });
  document.addEventListener('keyup', (e) => { if (e.code === 'Space') sim?.tacto(false); });
}

(async () => {
  try {
    wasm = await cargarWasm();
    $('t-config').value = await cargarEjemplo();
    enlazar();
    await arrancar();
    setInterval(leerImu, 20);
    requestAnimationFrame(cuadro);
    anotar(`· eventos que se pueden mandar por BLE: ${Eventos.join(', ')}`);
  } catch (e) {
    anotar(`No pude arrancar el simulador: ${e.message}`, 'mal');
  }
})();

// La charla sin IA: se manda como lo mandaría la app (0x4C Charlar) y él
// contesta con su globo, su voz y lo que hizo (0xCC).
$('f-charla').addEventListener('submit', (ev) => {
  ev.preventDefault();
  const t = $('i-charla').value.trim();
  if (!t || !sim) return;
  if (!audio && typeof AudioContext !== 'undefined') audio = new AudioContext();  // el navegador pide un gesto
  escribir(Car.comando, comando.charlar(t), `«${t}»`);
  $('i-charla').select();
});
$('b-voz').addEventListener('click', () => {
  conVoz = !conVoz;
  $('b-voz').setAttribute('aria-pressed', String(conVoz));
});
