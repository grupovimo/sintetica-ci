// Protocolo de SintéticaOS v1.3 (BLE y nube), del lado de quien controla el bicho (app,
// web, simulador). Ver docs/protocolo-ble.md. Sin dependencias: sirve en el
// navegador y en Node.

export const UUID = {
  servicio: '5157e71c-a000-4b1c-8a5e-000000000001',
  config: '5157e71c-a000-4b1c-8a5e-000000000002',
  comando: '5157e71c-a000-4b1c-8a5e-000000000003',
  estado: '5157e71c-a000-4b1c-8a5e-000000000004',
  ota: '5157e71c-a000-4b1c-8a5e-000000000005',
  imagen: '5157e71c-a000-4b1c-8a5e-000000000006',
};

export const Car = { config: 1, comando: 2, estado: 3, ota: 4, imagen: 5 };

export const Op = {
  configInicio: 0x01, configBloque: 0x02, configFin: 0x03, configPedir: 0x04,
  cmdExpresion: 0x10, cmdNombre: 0x11, cmdEvento: 0x12,
  cmdVincular: 0x13, cmdAutenticar: 0x14, cmdDesvincular: 0x15,
  cmdRed: 0x16, cmdNube: 0x17, cmdOlvidarRed: 0x18, cmdReproducir: 0x19,
  imgInicio: 0x20, imgBloque: 0x21, imgFin: 0x22, imgBorrar: 0x23, imgListar: 0x24, imgPedir: 0x25,
  otaInicio: 0x30, otaBloque: 0x31, otaFin: 0x32, otaDescargar: 0x33, otaProgreso: 0x34, otaCancelar: 0x35,
  respuesta: 0x80, configTrozo: 0x84, configCierre: 0x85, imgTrozo: 0xa7, imgCierre: 0xa8,
};

export const Codigo = {
  0: 'ok', 1: 'muy grande', 2: 'fuera de orden', 3: 'CRC distinto', 4: 'JSON inválido',
  5: 'config inválida', 6: 'sin inicio', 7: 'no existe', 8: 'malformado', 9: 'no autorizado',
  10: 'no está mostrando el QR', 11: 'imagen inválida', 12: 'firma inválida', 13: 'hash distinto',
  14: 'firmware de otra placa', 15: 'sin WiFi', 16: 'ocupado', 17: 'falló la actualización',
  255: 'no implementado',
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

function u16(v) {
  return [v & 0xff, (v >>> 8) & 0xff];
}

// Texto con su largo adelante (u8).
function corto(s, max = 255) {
  const b = texto.encode(s);
  if (b.length > max) throw new Error(`"${s.slice(0, 20)}…" es muy largo (máximo ${max} bytes)`);
  return [b.length, ...b];
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
  vincular: (codigo) => Uint8Array.from([Op.cmdVincular, ...texto.encode(codigo)]),
  autenticar: (clave) => Uint8Array.from([Op.cmdAutenticar, ...clave]),
  desvincular: () => Uint8Array.from([Op.cmdDesvincular]),
  red: (ssid, clave = '') => Uint8Array.from([Op.cmdRed, ...corto(ssid, 32), ...corto(clave, 64)]),
  nube: (url, usuario, clave) => Uint8Array.from([Op.cmdNube, ...corto(url, 160), ...corto(usuario, 64), ...corto(clave, 128)]),
  olvidarRed: () => Uint8Array.from([Op.cmdOlvidarRed]),
  reproducir: (animacion = '') => Uint8Array.from([Op.cmdReproducir, ...texto.encode(animacion)]),
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

// Una imagen QOI (del tamaño de la pantalla) partida en las escrituras de la característica Imagen.
export function escriturasDeImagen(espacio, qoi, tamanoBloque = 180) {
  const escrituras = [Uint8Array.from([Op.imgInicio, espacio, ...u32(qoi.length), ...u32(crc32(qoi))])];
  const porBloque = tamanoBloque - 5;
  for (let off = 0; off < qoi.length; off += porBloque) {
    escrituras.push(Uint8Array.from([Op.imgBloque, ...u32(off), ...qoi.subarray(off, off + porBloque)]));
  }
  escrituras.push(Uint8Array.from([Op.imgFin]));
  return escrituras;
}

export const imagen = {
  borrar: (espacio) => Uint8Array.from([Op.imgBorrar, espacio]),
  listar: () => Uint8Array.from([Op.imgListar]),
  pedir: (espacio) => Uint8Array.from([Op.imgPedir, espacio]),
};

// Arma la imagen que el bicho manda en trozos después de imagen.pedir().
export class ArmadorDeImagen {
  constructor() { this.partes = []; }
  // Devuelve { espacio, qoi } al llegar el cierre; null mientras tanto.
  agregar(b) {
    if (b[0] === Op.imgTrozo) { this.partes.push({ desplazamiento: leerU32(b, 1), datos: b.slice(5) }); return null; }
    if (b[0] !== Op.imgCierre) return null;
    const largo = leerU32(b, 2);
    const qoi = new Uint8Array(largo);
    for (const p of this.partes) qoi.set(p.datos, p.desplazamiento);
    this.partes = [];
    if (crc32(qoi) !== leerU32(b, 6)) throw new Error('la imagen llegó con otro CRC');
    return { espacio: b[1], qoi };
  }
}

// OTA. El manifiesto: { tamano, sha256 (32 bytes), firma (64 bytes: r || s), version, perfil }.
export function manifiestoOta(m) {
  if (m.sha256.length !== 32 || m.firma.length !== 64) throw new Error('sha256 de 32 bytes y firma de 64');
  return [...u32(m.tamano), ...m.sha256, ...m.firma, ...corto(m.version, 23), ...corto(m.perfil, 23)];
}

// El texto que se firma (ECDSA P-256 sobre su SHA-256).
export function mensajeOta(m) {
  const hex = Array.from(m.sha256, (x) => x.toString(16).padStart(2, '0')).join('');
  return `SINTETICA-OTA-V1\n${m.perfil}\n${m.version}\n${m.tamano}\n${hex}\n`;
}

export const ota = {
  descargar: (m, url) => {
    const u = texto.encode(url);
    return Uint8Array.from([Op.otaDescargar, ...manifiestoOta(m), ...u16(u.length), ...u]);
  },
  cancelar: () => Uint8Array.from([Op.otaCancelar]),
  // Por BLE: el manifiesto, los bloques del binario y el fin.
  escrituras: (m, binario, tamanoBloque = 180) => {
    const escrituras = [Uint8Array.from([Op.otaInicio, ...manifiestoOta(m)])];
    const porBloque = tamanoBloque - 5;
    for (let off = 0; off < binario.length; off += porBloque) {
      escrituras.push(Uint8Array.from([Op.otaBloque, ...u32(off), ...binario.subarray(off, off + porBloque)]));
    }
    escrituras.push(Uint8Array.from([Op.otaFin]));
    return escrituras;
  },
};

// La notificación de progreso de una OTA: [0xB4][código][porcentaje][detalle].
export function leerProgresoOta(b) {
  return { codigo: b[1], ok: b[1] === 0, porcentaje: b[2], mensaje: Codigo[b[1]] ?? `código ${b[1]}`, detalle: b.length > 3 ? lector.decode(b.subarray(3)) : '' };
}

// La respuesta a imagen.listar(): los espacios ocupados.
export function leerListaImagenes(b) {
  const espacios = [];
  for (let i = 3; i + 9 <= b.length; i += 9) espacios.push({ espacio: b[i], largo: leerU32(b, i + 1), crc: leerU32(b, i + 5) });
  return { mascara: b[2], espacios };
}

// El QR del bicho: "SINTETICA:V1/<id de 12>/<código de 6>".
export function leerQr(texto) {
  const m = /^SINTETICA:V1\/([0-9A-F]{12})\/([2-9A-HJKMNP-Z]{6})$/.exec(texto.trim());
  return m ? { id: m[1], codigo: m[2] } : null;
}

// El id también viaja en el anuncio BLE: datos de fabricante 0xFFFF + la MAC.
export function idDeAnuncio(datosFabricante) {
  const b = datosFabricante;
  if (!b || b.length < 8 || b[0] !== 0xff || b[1] !== 0xff) return null;
  return Array.from(b.slice(2, 8), (x) => x.toString(16).padStart(2, '0')).join('').toUpperCase();
}

// Interpreta una notificación de Config o Comando.
export function leerRespuesta(b) {
  if (b[0] === Op.configTrozo) return { tipo: 'trozo', desplazamiento: leerU32(b, 1), datos: b.subarray(5) };
  if (b[0] === Op.configCierre) return { tipo: 'cierre', largo: leerU32(b, 1), crc: leerU32(b, 5) };
  const op = b[0] & 0x7f;
  if (op === Op.cmdVincular && b[1] === 0) return { tipo: 'respuesta', op, codigo: 0, ok: true, mensaje: 'ok', detalle: '', clave: b.slice(2, 18) };
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
  if (b[0] < 1 || b[0] > 4) throw new Error(`versión de estado desconocida: ${b[0]}`);
  let i = 5;
  const campo = () => { const n = b[i]; const s = lector.decode(b.subarray(i + 1, i + 1 + n)); i += 1 + n; return s; };
  const expresion = campo();
  const nombre = campo();
  const serie = b[0] >= 2 ? campo() : '';
  const id = b[0] >= 2 ? campo() : '';
  const pantalla = b[0] >= 3
    ? { ancho: b[i] | (b[i + 1] << 8), alto: b[i + 2] | (b[i + 3] << 8), redonda: b[i + 4] === 1 }
    : { ancho: 320, alto: 172, redonda: false };
  if (b[0] >= 3) i += 5;
  const version = b[0] >= 4 ? campo() : '';
  const perfil = b[0] >= 4 ? campo() : '';
  const crcConfig = b[0] >= 4 ? leerU32(b, i) : 0;
  const encendido = b[0] >= 4 ? leerU32(b, i + 4) : 0;
  return {
    bateria: b[1] === 255 ? null : b[1],
    humor: (b[2] << 24 >> 24) / 100,
    modo: Modos[b[3]] ?? String(b[3]),
    dormido: !!(b[4] & 1),
    fija: !!(b[4] & 2),
    emparejando: !!(b[4] & 4),
    autenticado: !!(b[4] & 8),
    wifi: !!(b[4] & 16),
    nube: !!(b[4] & 32),
    redConfigurada: !!(b[4] & 64),
    actualizando: !!(b[4] & 128),
    version,
    perfil,
    crcConfig,
    encendido,
    expresion,
    nombre,
    serie,
    id,
    pantalla,
  };
}
