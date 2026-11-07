// La página del simulador: conecta el núcleo (sim.js) con la pantalla, los
// botones de sensores y el panel BLE, que manda los mismos bytes que la app.
import { almacenLocal, crearSimulador } from './sim.js';
import {
  ArmadorDeConfig, Car, comando, escriturasDeConfig, Eventos, leerEstado, leerQr, leerRespuesta, pedirConfig,
} from './protocolo.js';

const $ = (id) => document.getElementById(id);
const lienzo = $('lienzo');
const ctx = lienzo.getContext('2d');
const imagen = ctx.createImageData(320, 172);
const registro = $('registro');
const almacen = almacenLocal();

const NOMBRES_CAR = { 1: 'config', 2: 'comando', 3: 'estado', 4: 'ota' };
const NOMBRES_OP = {
  0x10: 'expresión', 0x11: 'nombre', 0x12: 'evento', 0x13: 'vincular', 0x14: 'autenticar', 0x15: 'desvincular',
  0x01: 'inicio', 0x02: 'bloque', 0x03: 'fin', 0x04: 'pedir',
};

// La clave que el bicho le dio a "este celu" al vincular (como la guardaría la app).
const CLAVE_CELU = 'sinteticaos-celu:clave';
let claveEnMemoria = null;  // por si el navegador no deja usar localStorage
function claveGuardada() {
  try { const s = localStorage.getItem(CLAVE_CELU); if (s) return Uint8Array.from(atob(s), (c) => c.charCodeAt(0)); } catch { /* sin almacenamiento */ }
  return claveEnMemoria;
}
function guardarClave(clave) {
  claveEnMemoria = clave;
  try { if (clave) localStorage.setItem(CLAVE_CELU, btoa(String.fromCharCode(...clave))); else localStorage.removeItem(CLAVE_CELU); } catch { /* sin almacenamiento */ }
}

let sim = null;
let bateria = -1;
let noche = false;
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

function alNotificar(car, b) {
  if (car === Car.estado) { mostrarEstado(leerEstado(b)); return; }
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
  for (const b of $('caras').children) b.setAttribute('aria-pressed', String(e.fija && b.dataset.nombre === e.expresion));
}

function armarCaras() {
  const caja = $('caras');
  caja.replaceChildren();
  for (const nombre of sim.expresiones()) {
    const b = document.createElement('button');
    b.textContent = nombre;
    b.dataset.nombre = nombre;
    b.setAttribute('aria-pressed', 'false');
    b.addEventListener('click', () => escribir(Car.comando, comando.expresion(nombre)));
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
  sim = await crearSimulador({
    wasm,
    almacen,
    bateria: () => bateria,
    minutoDelDia: () => (noche ? 2 * 60 : (() => { const d = new Date(); return d.getHours() * 60 + d.getMinutes(); })()),
    alPresentar: (rgba) => { imagen.data.set(rgba); ctx.putImageData(imagen, 0, 0); },
    alNotificar,
    alRegistrar: (m) => anotar(`· ${m}`),
    alSonido: (s) => anotar(`♪ sonido: ${s}`),
    alLed: (rgb) => { $('led').style.background = `#${rgb.toString(16).padStart(6, '0')}`; },
  });
  armarCaras();
  sim.bleConexion(true);
  const clave = claveGuardada();
  if (clave && !sim.emparejando()) escribir(Car.comando, comando.autenticar(clave), '(clave guardada de este celu)');
  sim.tick();
  mostrarEstado(leerEstado(sim.bleLeerEstado()));
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

function cuadro() {
  if (sim) sim.tick();
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
  $('b-noche').addEventListener('click', (e) => {
    noche = !noche;
    e.currentTarget.setAttribute('aria-pressed', String(noche));
    anotar(noche ? '· son las 2:00 (dentro del horario de sueño si la config lo define)' : '· vuelve la hora real');
  });
  $('r-bateria').addEventListener('input', (e) => {
    bateria = Number(e.target.value);
    $('v-bateria').textContent = bateria < 0 ? 'sin batería' : `${bateria}%`;
  });
  $('b-auto').addEventListener('click', () => escribir(Car.comando, comando.expresion('')));
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
