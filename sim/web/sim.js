// Carga el núcleo de SintéticaOS (WebAssembly) y le da un "hardware" de mentira:
// reloj, pantalla, almacenamiento y BLE. Sirve en el navegador y en Node.

const decodificador = new TextDecoder();

export async function crearSimulador(opciones) {
  const {
    wasm,                      // ArrayBuffer, Uint8Array o Response con sinteticaos.wasm
    ahora = () => performance.now(),
    almacen = memoriaVolatil(),
    alPresentar = () => {},    // (rgba: Uint8ClampedArray, ancho, alto)
    alNotificar = () => {},    // (caracteristica: number, bytes: Uint8Array)
    alRegistrar = () => {},
    alSonido = () => {},
    alLed = () => {},
    alAudio = () => {},        // (muestras: Int16Array) la voz propia, PCM 16 kHz mono
    bateria = () => -1,
    // La hora local. Si se pasa minutoDelDia (los tests), el día es uno fijo y
    // la hora arranca en ese minuto; si no, la del reloj de la compu.
    minutoDelDia = null,
    semilla = () => (Math.random() * 0xffffffff) >>> 0,
    identificador = 'A1B2C3D4E5F6',
    perfil = 'mascota',        // 'mascota' (la 1.47B, 320 × 172) o 'broche' (redondo, 240 × 240)
  } = opciones;

  let memoria;
  const inicio = ahora();
  // El reloj del bicho puede ir más rápido (un día en 2 minutos = ×720): se
  // lleva aparte del reloj real, sin saltos al cambiar la velocidad.
  let velocidad = 1, baseReal = inicio, baseVirtual = 0;
  const virtualMs = () => baseVirtual + (ahora() - baseReal) * velocidad;
  const epochInicio = minutoDelDia
    ? 20764 * 86400 + minutoDelDia() * 60
    : Math.floor(Date.now() / 1000) - new Date().getTimezoneOffset() * 60;
  const bytes = (p, n) => new Uint8Array(memoria.buffer, p, n);
  const cadena = (p) => {
    const m = new Uint8Array(memoria.buffer);
    let f = p;
    while (m[f]) f++;
    return decodificador.decode(m.subarray(p, f));
  };
  let rgba = null;

  let pId = 0;
  const env = {
    js_identificador: () => {
      if (!pId) { pId = x.sim_intercambio() + x.sim_intercambio_cap() - 16; bytes(pId, 13).set([...new TextEncoder().encode(identificador.slice(0, 12)), 0]); }
      return pId;
    },
    js_ahora: () => (Math.floor(virtualMs()) >>> 0),
    js_epoch: () => (epochInicio + Math.floor(virtualMs() / 1000)) >>> 0,
    js_semilla: () => semilla() >>> 0,
    js_presentar_franja: (p, w, y0, filas) => {
      const px = new Uint16Array(memoria.buffer, p, w * filas);
      const alto = perfil === 'broche' ? 240 : 172;
      if (!rgba || rgba.length !== w * alto * 4) rgba = new Uint8ClampedArray(w * alto * 4);
      for (let i = 0, j = y0 * w * 4; i < px.length; i++, j += 4) {
        const c = px[i];
        const r = (c >> 11) & 0x1f, g = (c >> 5) & 0x3f, b = c & 0x1f;
        rgba[j] = (r << 3) | (r >> 2);
        rgba[j + 1] = (g << 2) | (g >> 4);
        rgba[j + 2] = (b << 3) | (b >> 2);
        rgba[j + 3] = 255;
      }
      if (y0 + filas >= alto) alPresentar(rgba, w, alto);
    },
    js_escribir_trozo: (pc, off, pd, n) => (almacen.escribirTrozo(cadena(pc), off, bytes(pd, n).slice()) ? 1 : 0),
    js_leer_trozo: (pc, off, pd, n) => {
      const v = almacen.leerTrozo(cadena(pc), off, n);
      if (v == null) return -1;
      bytes(pd, v.length).set(v);
      return v.length;
    },
    js_borrar: (pc) => (almacen.borrarArchivo(cadena(pc)), 1),
    js_bateria: () => bateria(),
    js_guardar: (pc, pd, n) => (almacen.guardar(cadena(pc), bytes(pd, n).slice()) ? 1 : 0),
    js_leer: (pc, pd, cap) => {
      const v = almacen.leer(cadena(pc));
      if (!v || v.length > cap) return -1;
      bytes(pd, v.length).set(v);
      return v.length;
    },
    js_notificar: (car, p, n) => alNotificar(car, bytes(p, n).slice()),
    js_sonido: (p) => alSonido(cadena(p)),
    js_led: (rgb) => alLed(rgb >>> 0),
    js_registrar: (p) => alRegistrar(cadena(p)),
    js_audio: (p, n) => alAudio(new Int16Array(memoria.buffer, p, n).slice()),
  };

  const fuente = wasm instanceof Response ? await wasm.arrayBuffer() : wasm;
  const { instance } = await WebAssembly.instantiate(fuente, { env });
  var x = instance.exports;
  memoria = x.memory;
  x.__wasm_call_ctors();
  x.sim_iniciar(perfil === 'broche' ? 1 : 0);

  return {
    tick: () => x.sim_tick(),
    // El tiempo: velocidad (1 = real; 720 = un día en 2 min) y la hora local.
    velocidad: (v) => {
      if (v === undefined) return velocidad;
      baseVirtual = virtualMs();
      baseReal = ahora();
      velocidad = Math.max(0, v);
      return velocidad;
    },
    adelantar: (ms) => { baseVirtual += ms; },
    hora: () => {
      const e = epochInicio + Math.floor(virtualMs() / 1000);
      return { epoch: e, minuto: Math.floor((e % 86400) / 60) };
    },
    // El motor de vida, para el panel y las herramientas.
    vida: () => ({
      estado: Array.from({ length: 7 }, (_, i) => x.sim_vida_estado(i)),
      rasgos: Array.from({ length: 6 }, (_, i) => x.sim_vida_rasgo(i)),
      comportamiento: cadena(x.sim_vida_comportamiento()),
      dormido: (x.sim_vida_dormido() & 1) === 1,
      sonando: (x.sim_vida_dormido() & 2) === 2,
      candidatos: [0, 1, 2].map((i) => ({ nombre: cadena(x.sim_vida_candidato(i)), puntaje: x.sim_vida_puntaje(i) })).filter((c) => c.nombre),
      comportamientos: x.sim_vida_comportamientos_hechos(),
      parpadeos: x.sim_vida_parpadeos(),
    }),
    // La v3: la etapa, los tics, lo que pasó hoy y el diario (texto), y las pruebas del panel.
    caracter: () => cadena(x.sim_v3_texto()),
    accionV3: (a) => x.sim_v3_accion(a),
    caras: () => Array.from({ length: x.sim_cantidad_caras() }, (_, i) => cadena(x.sim_nombre_cara(i))),
    eventos: () => Array.from({ length: x.sim_cantidad_eventos() }, (_, i) => cadena(x.sim_nombre_evento(i))),
    forzarComportamiento: (nombre) => {
      const b = new TextEncoder().encode(nombre);
      bytes(x.sim_intercambio(), b.length + 1).set([...b, 0]);
      return x.sim_vida_comportamiento_forzar() === 1;
    },
    seguir: (mx, my) => x.sim_seguir(mx, my),
    depurar: (si) => x.sim_depurar(si ? 1 : 0),
    // Los parámetros del rostro: [{ nombre, min, max, defecto }] (los del ojo
    // van dos veces: izquierdo y derecho).
    parametros: () => Array.from({ length: x.sim_cantidad_parametros() }, (_, i) => ({
      nombre: cadena(x.sim_parametro_nombre(i)), min: x.sim_parametro_min(i), max: x.sim_parametro_max(i), defecto: x.sim_parametro_defecto(i),
    })),
    tacto: (apretado) => x.sim_tacto(apretado ? 1 : 0),
    // La charla sin IA: lo charla él (globo, acción y voz). true si entendió.
    charlar: (t) => {
      const b = new TextEncoder().encode(t);
      if (b.length < 1 || b.length > 200) return false;
      bytes(x.sim_intercambio(), b.length).set(b);
      return x.sim_charlar(b.length) === 1;
    },
    vozPropia: (si) => x.sim_voz_propia(si ? 1 : 0),
    imu: (ax, ay, az) => x.sim_imu(ax, ay, az),
    evento: (i) => x.sim_evento(i),
    // Una escritura BLE, con los mismos bytes que mandaría la app.
    bleEscribir: (caracteristica, datos) => {
      const p = x.sim_intercambio();
      if (datos.length > x.sim_intercambio_cap()) throw new Error('escritura demasiado grande');
      bytes(p, datos.length).set(datos);
      x.sim_ble_escribir(caracteristica, datos.length);
    },
    bleLeerEstado: () => bytes(x.sim_intercambio(), x.sim_ble_estado()).slice(),
    bleConexion: (conectado) => x.sim_conexion(conectado ? 1 : 0),
    qr: () => cadena(x.sim_qr()),
    emparejando: () => x.sim_emparejando() === 1,
    fijarSerie: (s) => {
      const p = x.sim_intercambio() + x.sim_intercambio_cap() - 32;
      bytes(p, 14).set([...new TextEncoder().encode(s.slice(0, 12)), 0]);
      x.sim_fijar_serie(p);
    },
    expresiones: () => Array.from({ length: x.sim_cantidad_expresiones() }, (_, i) => cadena(x.sim_nombre_expresion(i))),
    expresionVisible: () => cadena(x.sim_visible()),
    animaciones: () => Array.from({ length: x.sim_cantidad_animaciones() }, (_, i) => cadena(x.sim_nombre_animacion(i))),
    nombre: () => cadena(x.sim_nombre()),
    cierre: () => x.sim_cierre(),
    version: () => cadena(x.sim_version()),
    perfil,
    quiereDormir: () => x.sim_quiere_dormir() === 1,
    imagenVisible: () => x.sim_imagen_visible(),
    posicionLista: () => x.sim_posicion_lista(),
    crcConfig: () => x.sim_crc_config() >>> 0,
    vistaPrevia: () => x.sim_vista_previa(),
    // Lo mismo que llega por la nube (MQTT).
    nubeEscribir: (caracteristica, datos) => {
      bytes(x.sim_intercambio(), datos.length).set(datos);
      x.sim_nube_escribir(caracteristica, datos.length);
    },
    // ¿El bicho aceptaría esta config? { ok, codigo, detalle }
    validar: (json) => {
      const b = new TextEncoder().encode(typeof json === 'string' ? json : JSON.stringify(json));
      if (b.length > x.sim_intercambio_cap()) return { ok: false, codigo: 5, detalle: 'config: más de 16 KB' };
      bytes(x.sim_intercambio(), b.length).set(b);
      const codigo = x.sim_validar(b.length);
      return { ok: codigo === 0, codigo, detalle: codigo ? cadena(x.sim_detalle()) : '' };
    },
    // Editor: dibuja una expresión o un cuadro de una animación sin el sistema
    // (con una config propia), al tamaño que se pida.
    editor: (() => {
      let ancho = 640, alto = 344;
      const nombreAlIntercambio = (n) => {
        const b = new TextEncoder().encode(n);
        bytes(x.sim_intercambio(), b.length + 1).set([...b, 0]);
      };
      const leer = () => {
        const px = new Uint16Array(memoria.buffer, x.sim_editor_pixeles(), ancho * alto);
        const out = new Uint8ClampedArray(ancho * alto * 4);
        for (let i = 0, j = 0; i < px.length; i++, j += 4) {
          const c = px[i];
          const r = (c >> 11) & 0x1f, g = (c >> 5) & 0x3f, b = c & 0x1f;
          out[j] = (r << 3) | (r >> 2); out[j + 1] = (g << 2) | (g >> 4); out[j + 2] = (b << 3) | (b >> 2); out[j + 3] = 255;
        }
        return { rgba: out, ancho, alto };
      };
      return {
        iniciar: (w, h, redonda = false) => { ancho = w; alto = h; x.sim_editor_iniciar(w, h, redonda ? 1 : 0); },
        config: (json) => {
          const b = new TextEncoder().encode(typeof json === 'string' ? json : JSON.stringify(json));
          if (b.length > x.sim_intercambio_cap()) return { ok: false, codigo: 5, detalle: 'config: más de 16 KB' };
          bytes(x.sim_intercambio(), b.length).set(b);
          const codigo = x.sim_editor_config(b.length);
          return { ok: codigo === 0, codigo, detalle: codigo ? cadena(x.sim_detalle()) : '' };
        },
        indiceExpresion: (n) => (nombreAlIntercambio(n), x.sim_editor_indice_expresion()),
        indiceAnimacion: (n) => (nombreAlIntercambio(n), x.sim_editor_indice_animacion()),
        duracion: (i) => x.sim_editor_duracion(i),
        expresion: (i, cierre = 0, mx = 0, my = 0) => (x.sim_editor_expresion(i, cierre, mx, my), leer()),
        // Expresiones paramétricas.
        caras: () => Array.from({ length: x.sim_editor_cantidad_caras() }, (_, i) => cadena(x.sim_editor_nombre_cara(i))),
        indiceCara: (n) => (nombreAlIntercambio(n), x.sim_editor_indice_cara()),
        // Los parámetros completos de una cara (Float32Array).
        parametrosCara: (i) => {
          const n = x.sim_editor_parametros_cara(i);
          return new Float32Array(memoria.buffer, x.sim_intercambio(), n).slice();
        },
        // Dibuja un rostro con estos parámetros (Float32Array) en el instante t (s).
        rostro: (params, t = 0, parpadeo = 0, particula = -1) => {
          new Float32Array(memoria.buffer, x.sim_intercambio(), params.length).set(params);
          x.sim_editor_rostro(parpadeo, t, particula);
          return leer();
        },
        animacion: (i, t) => (x.sim_editor_animacion(i, t), leer()),
      };
    })(),
    // Reiniciar es volver a llamar a crearSimulador con el mismo almacén.
  };
}

// Archivos grandes (imágenes) en memoria, escritos por partes.
function archivos() {
  const a = new Map();
  return {
    escribirTrozo(k, off, datos) {
      const viejo = off === 0 ? new Uint8Array(0) : a.get(k);
      if (!viejo || viejo.length !== off) return false;
      const nuevo = new Uint8Array(off + datos.length);
      nuevo.set(viejo);
      nuevo.set(datos, off);
      a.set(k, nuevo);
      return true;
    },
    leerTrozo(k, off, n) {
      const v = a.get(k);
      if (!v) return null;
      return v.subarray(Math.min(off, v.length), Math.min(off + n, v.length));
    },
    borrarArchivo: (k) => a.delete(k),
    todos: a,
  };
}

// Almacén en memoria (Node, tests).
export function memoriaVolatil() {
  const m = new Map();
  const f = archivos();
  return {
    guardar: (k, v) => (m.set(k, v), true), leer: (k) => m.get(k) ?? null,
    borrar: () => { m.clear(); f.todos.clear(); },
    escribirTrozo: f.escribirTrozo, leerTrozo: f.leerTrozo, borrarArchivo: f.borrarArchivo,
  };
}

const b64 = (u8) => { let s = ''; for (let i = 0; i < u8.length; i += 8192) s += String.fromCharCode(...u8.subarray(i, i + 8192)); return btoa(s); };
const deB64 = (s) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

// Almacén en localStorage (navegador). Si no hay (modo privado), en memoria.
export function almacenLocal(prefijo = 'sinteticaos:') {
  const respaldo = memoriaVolatil();
  // Las imágenes viven en memoria y se copian a localStorage un rato después de
  // la última escritura (si entran: localStorage tiene unos 5 MB).
  const f = archivos();
  const pendientes = new Set();
  let reloj = null;
  const persistir = () => {
    reloj = null;
    for (const k of pendientes) {
      try {
        const v = f.todos.get(k);
        if (v) localStorage.setItem(prefijo + 'archivo:' + k, b64(v)); else localStorage.removeItem(prefijo + 'archivo:' + k);
      } catch { /* sin lugar: queda en memoria */ }
    }
    pendientes.clear();
  };
  const tocar = (k) => { pendientes.add(k); if (!reloj) reloj = setTimeout(persistir, 300); };
  try {
    for (const k of Object.keys(localStorage)) {
      if (k.startsWith(prefijo + 'archivo:')) f.todos.set(k.slice((prefijo + 'archivo:').length), deB64(localStorage.getItem(k)));
    }
  } catch { /* sin almacenamiento */ }
  return {
    guardar(k, v) {
      try { localStorage.setItem(prefijo + k, b64(v)); return true; } catch { return respaldo.guardar(k, v); }
    },
    leer(k) {
      try { const s = localStorage.getItem(prefijo + k); return s == null ? respaldo.leer(k) : deB64(s); } catch { return respaldo.leer(k); }
    },
    borrar() {
      try { for (const k of Object.keys(localStorage)) if (k.startsWith(prefijo)) localStorage.removeItem(k); } catch { /* nada */ }
      respaldo.borrar();
      f.todos.clear();
    },
    escribirTrozo: (k, off, d) => { const ok = f.escribirTrozo(k, off, d); if (ok) tocar(k); return ok; },
    leerTrozo: f.leerTrozo,
    borrarArchivo: (k) => { f.borrarArchivo(k); tocar(k); },
  };
}
