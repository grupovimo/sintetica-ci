// Protocolo de SintéticaOS v1.9 (BLE y nube), del lado de quien controla el bicho (app,
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
  cmdHora: 0x1a, cmdVida: 0x1b, cmdMemoria: 0x1c, cmdRestaurar: 0x1d, cmdRegistro: 0x1e, cmdComportamiento: 0x1f,
  cmdDecir: 0x40, cmdClima: 0x41, cmdAlarma: 0x42, cmdJugar: 0x43, cmdAviso: 0x44, cmdAnimo: 0x45, cmdMirar: 0x46,
  cmdToque: 0x47, cmdAgenda: 0x48, cmdPresencia: 0x49, cmdHabito: 0x4a, cmdMusica: 0x4b, cmdCharlar: 0x4c,
  imgInicio: 0x20, imgBloque: 0x21, imgFin: 0x22, imgBorrar: 0x23, imgListar: 0x24, imgPedir: 0x25, imgListarSprites: 0x26,
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

// En el orden del núcleo (el índice viaja en el comando Evento). Los primeros 8 son de la v1.
export const Eventos = [
  'tacto', 'doble-toque', 'levantado', 'sacudida', 'boca-abajo', 'vinculado', 'app-abierta', 'despertado',
  'caricia', 'muchos-toques', 'sacudida-suave', 'sacudida-fuerte', 'inclinado', 'caida', 'voz', 'nombre',
  'ruido', 'musica', 'silencio', 'notificacion', 'llamada', 'bateria-baja', 'bateria-critica', 'enchufado',
  'desenchufado', 'cargado', 'sin-wifi', 'cambio-de-cara', 'bicho-cerca', 'volvio', 'hito',
  'recordatorio', 'pomodoro', 'despertador', 'juego', 'animo', 'cara-detectada', 'oscuridad', 'mano-cerca',
  'toque-recibido', 'devolver-toque', 'agenda', 'llegaste', 'te-fuiste', 'habito', // v1.9
];
// Los tipos del comando Toque y del comando Aviso (v1.9), en orden.
export const TiposToque = ['toque', 'abrazo', 'beso', 'carita'];
export const TiposAviso = ['notificacion', 'llamada', 'pago', 'mail', 'github', 'pedido', 'otro'];
// Los tipos de las entradas del registro, en orden.
export const TiposRegistro = ['evento', 'comportamiento', 'sueño', 'hito', 'recordatorio', 'juego', 'animo', 'toque', 'habito'];
// Los juegos del comando Jugar (v1.7), en orden, y los ánimos del comando Ánimo.
export const Juegos = ['ppt', 'reflejos', 'mirada', 'dado', 'moneda'];
export const Animos = ['bien', 'medio', 'mal'];
// Las acciones del comando Alarma (v1.6), en orden.
export const AccionesAlarma = ['apagar', 'posponer', 'probar', 'despertar'];
// Las situaciones de las frases (habla.propias y vida/frases.json).
export const Situaciones = [
  'despertar', 'buenas-noches', 'volviste', 'te-extrane', 'aburrido', 'sueno', 'bateria-baja', 'cargando',
  'caricia', 'saludo', 'hito', 'cumple-dueno', 'cumple-robot', 'recordatorio-hecho', 'despertador', 'posponer',
  'clima-frio', 'clima-calor', 'clima-lluvia', 'clima-lindo', 'buen-dia', 'comiste', 'a-dormir', 'tardaste',
  'agua', 'pausa', 'como-estas', 'animo-bien', 'animo-medio', 'animo-mal', 'jugar', 'gane',
  'perdi', 'empate', 'anio-nuevo', 'navidad', 'dia-del-amigo', 'dia-del-nino', 'aniversario',
  // v1.9
  'toque-recibido', 'toque-devuelto', 'agenda', 'agenda-manana', 'te-vas', 'habito-hecho', 'habito-racha',
  'habito-cortado', 'musica', 'pago', 'mail', 'github', 'pedido', 'aviso',
];
// El estado interno y los rasgos, en el orden del protocolo (comando Vida).
export const VariablesEstado = ['energia', 'animo', 'afecto', 'aburrimiento', 'curiosidad', 'sueno', 'estres'];
export const Rasgos = ['energia', 'sociabilidad', 'picardia', 'timidez', 'curiosidad', 'malGenio'];
export const LARGO_MEMORIA = 112;
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

// Texto cortado a max bytes de UTF-8 sin partir una letra, con su largo adelante (u8).
function cortado(s, max) {
  let b = texto.encode((s ?? '').trim());
  if (b.length > max) {
    b = b.slice(0, max);
    // Si quedó una letra a medias, se va entera.
    let i = b.length;
    while (i > 0 && (b[i - 1] & 0xc0) === 0x80) i--;
    if (i > 0 && b[i - 1] >= 0xc0) {
      const largo = b[i - 1] >= 0xf0 ? 4 : b[i - 1] >= 0xe0 ? 3 : 2;
      if (b.length - (i - 1) < largo) b = b.slice(0, i - 1);
    }
  }
  return [b.length, ...b];
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
  // La hora (v1.4): epoch UTC en segundos y la zona en minutos (Argentina: -180).
  hora: (epochUtc = Math.floor(Date.now() / 1000), zonaMinutos = -new Date().getTimezoneOffset()) => {
    const b = new Uint8Array(7);
    b[0] = Op.cmdHora;
    new DataView(b.buffer).setUint32(1, epochUtc >>> 0, true);
    new DataView(b.buffer).setInt16(5, zonaMinutos, true);
    return b;
  },
  // El estado del motor de vida; con superposicion (true/false) prende o apaga la de depuración.
  vida: (superposicion) => (superposicion === undefined ? Uint8Array.from([Op.cmdVida]) : Uint8Array.from([Op.cmdVida, superposicion ? 1 : 0])),
  memoria: () => Uint8Array.from([Op.cmdMemoria]),
  restaurar: (memoria) => Uint8Array.from([Op.cmdRestaurar, ...memoria]),
  registro: () => Uint8Array.from([Op.cmdRegistro]),
  // Empieza un comportamiento ya (para probarlo).
  comportamiento: (nombre) => Uint8Array.from([Op.cmdComportamiento, ...texto.encode(nombre)]),
  // Lo muestra en un globo (v1.6): texto de 1 a 60 bytes y, opcional, una expresión.
  decir: (t, expresion = '') => {
    const b = texto.encode(t), e = texto.encode(expresion);
    if (b.length < 1 || b.length > 60) throw new Error('el texto es de 1 a 60 bytes');
    return Uint8Array.from([Op.cmdDecir, e.length, ...e, ...b]);
  },
  // El clima (v1.6): °C actual (se manda en décimas), código WMO, máx, mín, si es de día y la ciudad.
  clima: ({ temp, codigo, max, min, dia = true, ciudad = '' }) => {
    let c = texto.encode(ciudad);
    if (c.length > 24) c = c.slice(0, 24);
    while (c.length && (c[c.length - 1] & 0xc0) === 0x80) c = c.slice(0, -1);
    if (c.length && c[c.length - 1] >= 0xc0) c = c.slice(0, -1);
    const b = new Uint8Array(8 + c.length);
    b[0] = Op.cmdClima;
    const v = new DataView(b.buffer);
    v.setInt16(1, Math.round(temp * 10), true);
    b[3] = codigo;
    v.setInt8(4, max);
    v.setInt8(5, min);
    b[6] = dia ? 1 : 0;
    b[7] = c.length;
    b.set(c, 8);
    return b;
  },
  // Un juego (v1.7): 'ppt', 'reflejos', 'mirada', 'dado', 'moneda' o 'terminar'.
  jugar: (juego) => {
    const i = juego === 'terminar' ? 255 : Juegos.indexOf(juego);
    if (i < 0) throw new Error(`juego desconocido: ${juego}`);
    return Uint8Array.from([Op.cmdJugar, i]);
  },
  // Una notificación o llamada del celu con su app y su contacto (v1.7); con
  // tipo (v1.9) 'pago', 'mail', 'github', 'pedido' u 'otro', app es el título corto y contacto quién.
  aviso: ({ llamada = false, tipo, app = '', contacto = '' }) => {
    const t = tipo === undefined ? (llamada ? 1 : 0) : TiposAviso.indexOf(tipo);
    if (t < 0) throw new Error(`aviso desconocido: ${tipo}`);
    return Uint8Array.from([Op.cmdAviso, t, ...corto(app, 24), ...corto(contacto, 24)]);
  },
  // Un toque de un amigo (v1.9): tipo 'toque', 'abrazo', 'beso' o 'carita', de quién (1 a 24 bytes) y la expresión.
  toque: (tipo, de, expresion = '') => {
    const t = TiposToque.indexOf(tipo || 'toque');
    if (t < 0) throw new Error(`toque desconocido: ${tipo}`);
    const d = cortado(de, 24);
    if (d[0] === 0) throw new Error('falta de quién es el toque');
    return Uint8Array.from([Op.cmdToque, t, ...d, ...cortado(expresion, 24)]);
  },
  // El próximo evento de la agenda (v1.9): en cuántos minutos, cuánto dura y el título (1 a 40 bytes).
  agenda: (minutos, duracion, titulo) => {
    const t = cortado(titulo, 40);
    if (t[0] === 0) throw new Error('falta el título');
    const m = Math.max(0, Math.min(0xfffe, Math.round(minutos)));
    return Uint8Array.from([Op.cmdAgenda, ...u16(m), ...u16(Math.max(0, Math.min(0xffff, Math.round(duracion)))), ...t]);
  },
  // El resumen de mañana (v1.9): el 0x48 con minutos = 0xFFFF y la cantidad de eventos en la duración.
  resumenManana: (cantidad, titulo) => {
    const t = cortado(titulo, 40);
    if (t[0] === 0) throw new Error('falta el título');
    return Uint8Array.from([Op.cmdAgenda, 0xff, 0xff, ...u16(Math.max(0, Math.min(0xffff, cantidad))), ...t]);
  },
  // Llegaste (true) o te fuiste (false) (v1.9).
  presencia: (llegaste) => Uint8Array.from([Op.cmdPresencia, llegaste ? 0 : 1]),
  // Un hábito (v1.9): índice 0 a 3, hecho o deshecho y la racha en días.
  habito: (indice, hecho = true, racha = 0) => {
    if (!(indice >= 0 && indice < 4)) throw new Error('el hábito es del 0 al 3');
    return Uint8Array.from([Op.cmdHabito, indice, hecho ? 0 : 1, Math.max(0, Math.min(255, racha))]);
  },
  // Lo que suena (v1.9): bpm (0 = no se sabe), si suena, el título (0 a 40 bytes) y el artista (0 a 24).
  musica: ({ bpm = 0, sonando = true, titulo = '', artista = '' }) =>
    Uint8Array.from([Op.cmdMusica, Math.max(0, Math.min(255, Math.round(bpm))), sonando ? 1 : 0, ...cortado(titulo, 40), ...cortado(artista, 24)]),
  // Que lo charle él (v1.10, la charla sin IA): lo que dijiste, de 1 a 200 bytes.
  charlar: (t) => {
    const b = texto.encode(t);
    if (b.length < 1 || b.length > 200) throw new Error('el texto es de 1 a 200 bytes');
    return Uint8Array.from([Op.cmdCharlar, b.length, ...b]);
  },
  // La respuesta al "¿cómo estás?" (v1.7): 'bien', 'medio' o 'mal'.
  animo: (a) => {
    const i = Animos.indexOf(a);
    if (i < 0) throw new Error(`ánimo desconocido: ${a}`);
    return Uint8Array.from([Op.cmdAnimo, i]);
  },
  // De dónde viene un sonido (v1.7): x e y de -1 a 1.
  mirar: (x, y = 0) => Uint8Array.from([Op.cmdMirar, Math.round(x * 100) & 0xff, Math.round(y * 100) & 0xff]),
  // El despertador (v1.6): 'apagar', 'posponer', 'probar' o 'despertar'.
  alarma: (accion) => {
    const i = AccionesAlarma.indexOf(accion);
    if (i < 0) throw new Error(`acción desconocida: ${accion}`);
    return Uint8Array.from([Op.cmdAlarma, i]);
  },
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

// Los sprites de las caras de imagen (v1.5): espacios 8 a 19 de la característica
// Imagen (el sprite n es el espacio 8 + n), QOI de 3 o 4 canales de hasta 128 × 128.
export const PRIMER_SPRITE = 8;
export const MAX_SPRITES = 12;
export const LADO_SPRITE = 128;

// Una imagen QOI (del tamaño de la pantalla, o un sprite) partida en las escrituras de la característica Imagen.
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
  // Los sprites ocupados (v1.5): responde [0xA6][0][máscara u16] + por cada uno [espacio][u32 largo][u32 crc].
  listarSprites: () => Uint8Array.from([Op.imgListarSprites]),
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

// La respuesta a imagen.listarSprites(): los sprites ocupados (sprite = espacio - 8).
export function leerListaSprites(b) {
  const sprites = [];
  for (let i = 4; i + 9 <= b.length; i += 9) sprites.push({ espacio: b[i], sprite: b[i] - PRIMER_SPRITE, largo: leerU32(b, i + 1), crc: leerU32(b, i + 5) });
  return { mascara: b[2] | (b[3] << 8), sprites };
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
// Lo que entendió (la respuesta a charlar, 0xCC): [0xCC][código][flags][confianza][n][intención]
// [acción][dato][i16 minuto][i16 en minutos][i8 día][u8 días][n][texto][n][frase].
export const AccionesCharla = ['ninguna', 'alarma', 'cancelar-alarma', 'posponer', 'apagar-alarma', 'recordatorio', 'habito',
  'jugar', 'modo', 'gusto', 'olvidar-gusto', 'toque', 'musica', 'dormir', 'despertar', 'dueno', 'volumen', 'callar', 'animo'];
export function leerCharla(b) {
  if (b[0] !== (Op.cmdCharlar | 0x80) || b[1] !== 0) return null;
  let i = 4;
  const corto = () => { const n = b[i++]; const s = lector.decode(b.subarray(i, i + n)); i += n; return s; };
  const intencion = corto();
  const accion = AccionesCharla[b[i++]] ?? 'ninguna';
  const dato = (b[i++] << 24) >> 24;
  const i16 = () => { const v = (b[i] | (b[i + 1] << 8)) << 16 >> 16; i += 2; return v; };
  const minuto = i16(), enMinutos = i16();
  const dia = (b[i++] << 24) >> 24, dias = b[i++];
  const textoAccion = corto(), frase = corto();
  return { noEntendi: (b[2] & 1) !== 0, falta: (b[2] & 2) !== 0, hizo: (b[2] & 4) !== 0, confianza: b[3], intencion, accion, dato, minuto, enMinutos, dia, dias, texto: textoAccion, frase };
}

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

// La respuesta al comando Vida: [0x9B][0][7 estado][6 rasgos][flags][n][comportamiento][n][visible][n cand]...[fps]
export function leerVida(b) {
  if ((b[0] & 0x7f) !== Op.cmdVida || b[1] !== 0) throw new Error('no es una respuesta de Vida');
  let i = 2;
  const estado = {}, rasgos = {};
  for (const v of VariablesEstado) estado[v] = b[i++] / 255;
  for (const r of Rasgos) rasgos[r] = b[i++] / 255;
  const flags = b[i++];
  const campo = () => { const n = b[i]; const s = lector.decode(b.subarray(i + 1, i + 1 + n)); i += 1 + n; return s; };
  const comportamiento = campo();
  const visible = campo();
  const candidatos = [];
  const nc = b[i++];
  for (let k = 0; k < nc; k++) candidatos.push({ nombre: campo(), puntaje: b[i++] / 100 });
  return { estado, rasgos, dormido: !!(flags & 1), sonando: !!(flags & 2), superposicion: !!(flags & 4), comportamiento, visible, candidatos, fps: b[i] };
}

// La memoria (112 bytes, la del comando Memoria sin los 2 de la respuesta).
export function leerMemoria(m) {
  if (m.length !== LARGO_MEMORIA || m[0] !== 1) throw new Error('memoria inválida');
  if (leerU32(m, LARGO_MEMORIA - 4) !== crc32(m.subarray(0, LARGO_MEMORIA - 4))) throw new Error('memoria con otro CRC');
  const u16 = (k) => m[k] | (m[k + 1] << 8);
  const i8 = (k) => (m[k] << 24) >> 24;
  const estado = {};
  VariablesEstado.forEach((v, k) => { estado[v] = m[79 + k] / 255; });
  const deriva = {};
  Rasgos.forEach((r, k) => { deriva[r] = i8(73 + k) / 100; });
  return {
    nacimiento: leerU32(m, 1), ultimaInteraccion: leerU32(m, 5), rencorHasta: leerU32(m, 9), ultimoDia: leerU32(m, 13),
    dias: u16(17), racha: u16(19), rachaMax: u16(21),
    caricias: leerU32(m, 23), toques: leerU32(m, 27), sacudidas: leerU32(m, 31),
    horas: Array.from(m.subarray(35, 59)),
    gustos: Object.fromEntries(['caricia', 'tacto', 'voz', 'musica', 'nombre', 'levantado', 'sacudida-suave', 'app-abierta'].map((g, k) => [g, i8(59 + k)])),
    hitos: u16(67), anioCumpleBicho: u16(69), anioCumpleDueno: u16(71),
    deriva, estado, epochEstado: leerU32(m, 86), ausenciaMaxima: u16(90), afectoHoy: m[92],
    // v1.7
    juegosGanados: u16(93), juegosJugados: u16(95), mejorReflejo: u16(97),
    juegosHoy: m[99], suenosHoy: m[100], sonoConHoy: m[101], animoHoy: m[102], esperoHoy: m[103],
    diaContadores: m[104], anioFechas: m[105], fechas: m[106], aguaHoy: m[107],
  };
}

// El registro: [0x9E][0][n][n × (u32 segundos, u8 tipo, u8 dato)].
export function leerRegistro(b) {
  const n = b[2];
  const tipos = TiposRegistro;
  return Array.from({ length: n }, (_, k) => {
    const i = 3 + k * 6;
    return { segundos: leerU32(b, i), tipo: tipos[b[i + 4]] ?? String(b[i + 4]), dato: b[i + 5] };
  });
}

// La memoria al revés (para los tests y las herramientas; el bicho la arma solo).
export function codificarMemoria(m) {
  const b = new Uint8Array(LARGO_MEMORIA);
  const v = new DataView(b.buffer);
  b[0] = 1;
  v.setUint32(1, m.nacimiento ?? 0, true); v.setUint32(5, m.ultimaInteraccion ?? 0, true);
  v.setUint32(9, m.rencorHasta ?? 0, true); v.setUint32(13, m.ultimoDia ?? 0, true);
  v.setUint16(17, m.dias ?? 0, true); v.setUint16(19, m.racha ?? 0, true); v.setUint16(21, m.rachaMax ?? 0, true);
  v.setUint32(23, m.caricias ?? 0, true); v.setUint32(27, m.toques ?? 0, true); v.setUint32(31, m.sacudidas ?? 0, true);
  (m.horas ?? []).forEach((h, k) => { b[35 + k] = h; });
  ['caricia', 'tacto', 'voz', 'musica', 'nombre', 'levantado', 'sacudida-suave', 'app-abierta'].forEach((g, k) => v.setInt8(59 + k, m.gustos?.[g] ?? 0));
  v.setUint16(67, m.hitos ?? 0, true); v.setUint16(69, m.anioCumpleBicho ?? 0, true); v.setUint16(71, m.anioCumpleDueno ?? 0, true);
  Rasgos.forEach((r, k) => v.setInt8(73 + k, Math.round((m.deriva?.[r] ?? 0) * 100)));
  VariablesEstado.forEach((e, k) => { b[79 + k] = Math.round((m.estado?.[e] ?? 0) * 255); });
  v.setUint32(86, m.epochEstado ?? 0, true); v.setUint16(90, m.ausenciaMaxima ?? 0, true); b[92] = m.afectoHoy ?? 0;
  v.setUint16(93, m.juegosGanados ?? 0, true); v.setUint16(95, m.juegosJugados ?? 0, true); v.setUint16(97, m.mejorReflejo ?? 0, true);
  [m.juegosHoy, m.suenosHoy, m.sonoConHoy, m.animoHoy, m.esperoHoy, m.diaContadores, m.anioFechas, m.fechas, m.aguaHoy]
    .forEach((x, k) => { b[99 + k] = x ?? 0; });
  v.setUint32(LARGO_MEMORIA - 4, crc32(b.subarray(0, LARGO_MEMORIA - 4)), true);
  return b;
}
