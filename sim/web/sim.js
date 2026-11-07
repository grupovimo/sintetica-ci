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
    bateria = () => -1,
    minutoDelDia = () => { const d = new Date(); return d.getHours() * 60 + d.getMinutes(); },
    semilla = () => (Math.random() * 0xffffffff) >>> 0,
  } = opciones;

  let memoria;
  const inicio = ahora();
  const bytes = (p, n) => new Uint8Array(memoria.buffer, p, n);
  const cadena = (p) => {
    const m = new Uint8Array(memoria.buffer);
    let f = p;
    while (m[f]) f++;
    return decodificador.decode(m.subarray(p, f));
  };
  let rgba = null;

  const env = {
    js_ahora: () => (Math.floor(ahora() - inicio) >>> 0),
    js_semilla: () => semilla() >>> 0,
    js_presentar: (p, w, h) => {
      const px = new Uint16Array(memoria.buffer, p, w * h);
      if (!rgba || rgba.length !== w * h * 4) rgba = new Uint8ClampedArray(w * h * 4);
      for (let i = 0, j = 0; i < px.length; i++, j += 4) {
        const c = px[i];
        const r = (c >> 11) & 0x1f, g = (c >> 5) & 0x3f, b = c & 0x1f;
        rgba[j] = (r << 3) | (r >> 2);
        rgba[j + 1] = (g << 2) | (g >> 4);
        rgba[j + 2] = (b << 3) | (b >> 2);
        rgba[j + 3] = 255;
      }
      alPresentar(rgba, w, h);
    },
    js_minuto: () => minutoDelDia(),
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
  };

  const fuente = wasm instanceof Response ? await wasm.arrayBuffer() : wasm;
  const { instance } = await WebAssembly.instantiate(fuente, { env });
  const x = instance.exports;
  memoria = x.memory;
  x.__wasm_call_ctors();
  x.sim_iniciar();

  return {
    tick: () => x.sim_tick(),
    tacto: (apretado) => x.sim_tacto(apretado ? 1 : 0),
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
    expresiones: () => Array.from({ length: x.sim_cantidad_expresiones() }, (_, i) => cadena(x.sim_nombre_expresion(i))),
    expresionVisible: () => cadena(x.sim_nombre_expresion(x.sim_expresion_visible())),
    nombre: () => cadena(x.sim_nombre()),
    cierre: () => x.sim_cierre(),
    version: () => cadena(x.sim_version()),
    // Reiniciar es volver a llamar a crearSimulador con el mismo almacén.
  };
}

// Almacén en memoria (Node, tests).
export function memoriaVolatil() {
  const m = new Map();
  return { guardar: (k, v) => (m.set(k, v), true), leer: (k) => m.get(k) ?? null, borrar: () => m.clear() };
}

// Almacén en localStorage (navegador). Si no hay (modo privado), en memoria.
export function almacenLocal(prefijo = 'sinteticaos:') {
  const respaldo = memoriaVolatil();
  const b64 = (u8) => btoa(String.fromCharCode(...u8));
  const deB64 = (s) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
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
    },
  };
}
