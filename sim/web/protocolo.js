// Protocolo BLE de SintéticaOS v1, del lado de quien controla el bicho (app,
// web, simulador). Ver docs/protocolo-ble.md. Sin dependencias: sirve en el
// navegador y en Node.

export const UUID = {
  servicio: '5157e71c-a000-4b1c-8a5e-000000000001',
  config: '5157e71c-a000-4b1c-8a5e-000000000002',
  comando: '5157e71c-a000-4b1c-8a5e-000000000003',
  estado: '5157e71c-a000-4b1c-8a5e-000000000004',
  ota: '5157e71c-a000-4b1c-8a5e-000000000005',
};

export const Car = { config: 1, comando: 2, estado: 3, ota: 4 };

export const Op = {
  configInicio: 0x01, configBloque: 0x02, configFin: 0x03, configPedir: 0x04,
  cmdExpresion: 0x10, cmdNombre: 0x11, cmdEvento: 0x12,
  respuesta: 0x80, configTrozo: 0x84, configCierre: 0x85,
};

export const Codigo = {
  0: 'ok', 1: 'muy grande', 2: 'fuera de orden', 3: 'CRC distinto', 4: 'JSON inválido',
  5: 'config inválida', 6: 'sin inicio', 7: 'no existe', 8: 'malformado', 255: 'no implementado',
};

export const Eventos = ['tacto', 'doble-toque', 'levantado', 'sacudida', 'boca-abajo', 'vinculado', 'app-abierta', 'despertado'];
export const Modos = ['mascota', 'reloj', 'pomodoro', 'clima', 'notificaciones'];

const texto = new TextEncoder();
const lector = new TextDecoder();

const TABLA_CRC = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();

export function crc32(bytes) {
  let c = 0xffffffff;
  for (const b of bytes) c = TABLA_CRC[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function u32(v) {
  return [v & 0xff, (v >>> 8) & 0xff, (v >>> 16) & 0xff, (v >>> 24) & 0xff];
}

export function leerU32(b, i) {
  return (b[i] | (b[i + 1] << 8) | (b[i + 2] << 16) | (b[i + 3] << 24)) >>> 0;
}

// Comandos: devuelven los bytes a escribir en la característica Comando.
export const comando = {
  expresion: (nombre = '') => Uint8Array.from([Op.cmdExpresion, ...texto.encode(nombre)]),
  nombre: (nombre) => Uint8Array.from([Op.cmdNombre, ...texto.encode(nombre)]),
  evento: (nombreOIndice) => {
    const i = typeof nombreOIndice === 'number' ? nombreOIndice : Eventos.indexOf(nombreOIndice);
    if (i < 0) throw new Error(`evento desconocido: ${nombreOIndice}`);
    return Uint8Array.from([Op.cmdEvento, i]);
  },
};

// Parte una config (objeto o texto JSON) en las escrituras de la característica Config.
export function escriturasDeConfig(config, tamanoBloque = 180) {
  const json = typeof config === 'string' ? config : JSON.stringify(config);
  const bytes = texto.encode(json);
  const escrituras = [Uint8Array.from([Op.configInicio, ...u32(bytes.length), ...u32(crc32(bytes))])];
  const porBloque = tamanoBloque - 5;
  for (let off = 0; off < bytes.length; off += porBloque) {
    escrituras.push(Uint8Array.from([Op.configBloque, ...u32(off), ...bytes.subarray(off, off + porBloque)]));
  }
  escrituras.push(Uint8Array.from([Op.configFin]));
  return escrituras;
}

export const pedirConfig = () => Uint8Array.from([Op.configPedir]);

// Interpreta una notificación de Config o Comando.
export function leerRespuesta(b) {
  if (b[0] === Op.configTrozo) return { tipo: 'trozo', desplazamiento: leerU32(b, 1), datos: b.subarray(5) };
  if (b[0] === Op.configCierre) return { tipo: 'cierre', largo: leerU32(b, 1), crc: leerU32(b, 5) };
  const op = b[0] & 0x7f;
  return {
    tipo: 'respuesta', op, codigo: b[1], ok: b[1] === 0, mensaje: Codigo[b[1]] ?? `código ${b[1]}`,
    detalle: b.length > 2 ? lector.decode(b.subarray(2)) : '',
  };
}

// Arma la config que el bicho manda en trozos después de pedirConfig().
export class ArmadorDeConfig {
  constructor() { this.partes = []; }
  agregar(r) {
    if (r.tipo === 'trozo') { this.partes.push(r); return null; }
    if (r.tipo !== 'cierre') return null;
    const bytes = new Uint8Array(r.largo);
    for (const p of this.partes) bytes.set(p.datos, p.desplazamiento);
    this.partes = [];
    if (crc32(bytes) !== r.crc) throw new Error('la config llegó con otro CRC');
    return JSON.parse(lector.decode(bytes));
  }
}

export function leerEstado(b) {
  if (b[0] !== 1) throw new Error(`versión de estado desconocida: ${b[0]}`);
  const ne = b[5];
  const expresion = lector.decode(b.subarray(6, 6 + ne));
  const nn = b[6 + ne];
  const nombre = lector.decode(b.subarray(7 + ne, 7 + ne + nn));
  return {
    bateria: b[1] === 255 ? null : b[1],
    humor: (b[2] << 24 >> 24) / 100,
    modo: Modos[b[3]] ?? String(b[3]),
    dormido: !!(b[4] & 1),
    fija: !!(b[4] & 2),
    expresion,
    nombre,
  };
}
