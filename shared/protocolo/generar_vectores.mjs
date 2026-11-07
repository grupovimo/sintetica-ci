// Genera shared/protocolo/vectores.json: los bytes de cada operación del
// protocolo v1.4. Los arma el codificador de referencia en JS
// (sim/web/protocolo.js); el núcleo C++ los reproduce en sus tests
// (firmware/test/test_vectores.cpp) y las apps de Kotlin y Swift los usan para
// comprobar sus propios codificadores. Así las cuatro implementaciones no se
// desincronizan.
//
// Uso: node shared/protocolo/generar_vectores.mjs [--verificar]
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import crypto from 'node:crypto';

import { codificarMemoria, comando, crc32, escriturasDeConfig, escriturasDeImagen, imagen, manifiestoOta, mensajeOta, ota, pedirConfig } from '../../sim/web/protocolo.js';
import { codificarQoi } from '../../sim/web/qoi.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const hex = (b) => Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(' ');
const utf8 = (s) => new TextEncoder().encode(s);
const clave = Uint8Array.from({ length: 16 }, (_, i) => i * 17);

// Una memoria de ejemplo (la del comando Restaurar y la de "memoria" abajo).
const MEMORIA_PRUEBA = {
  nacimiento: 1790000000, ultimaInteraccion: 1793000000, rencorHasta: 0, ultimoDia: 20752, dias: 42, racha: 5, rachaMax: 9,
  caricias: 321, toques: 1234, sacudidas: 7, horas: Array.from({ length: 24 }, (_, h) => (h === 9 ? 30 : h === 21 ? 12 : 0)),
  gustos: { caricia: 40, tacto: 10, voz: 0, musica: -3, nombre: 5, levantado: 0, 'sacudida-suave': 2, 'app-abierta': 1 },
  hitos: 0x0b, anioCumpleBicho: 2026, anioCumpleDueno: 0, deriva: { sociabilidad: 0.04, malGenio: -0.02 },
  estado: { energia: 0.6, animo: 0.7, afecto: 0.55, aburrimiento: 0.1, curiosidad: 0.5, sueno: 0.2, estres: 0 },
  epochEstado: 1793000000, ausenciaMaxima: 30, afectoHoy: 6,
};

const ejemplo = JSON.stringify(JSON.parse(fs.readFileSync(path.join(aqui, '../formato/config-ejemplo.json'), 'utf8')));

function estado(c) {
  const campo = (s) => { const b = utf8(s); return [b.length, ...b]; };
  const flags = (c.dormido ? 1 : 0) | (c.fija ? 2 : 0) | (c.emparejando ? 4 : 0) | (c.autenticado ? 8 : 0) |
    (c.wifi ? 16 : 0) | (c.nube ? 32 : 0) | (c.redConfigurada ? 64 : 0) | (c.actualizando ? 128 : 0);
  return hex([4, c.bateria ?? 255, Math.round(c.humor * 100) & 0xff, c.modo, flags,
    ...campo(c.expresion), ...campo(c.nombre), ...campo(c.serie), ...campo(c.id),
    c.ancho & 255, c.ancho >> 8, c.alto & 255, c.alto >> 8, c.redonda ? 1 : 0,
    ...campo(c.version), ...campo(c.perfil), ...[0, 8, 16, 24].map((k) => (c.crcConfig >>> k) & 255),
    ...[0, 8, 16, 24].map((k) => (c.encendido >>> k) & 255)]);
}

// Una imagen chica que usa todas las operaciones de QOI: rachas, índice,
// diferencias chicas, luma y RGB completo.
function imagenDePrueba(ancho, alto) {
  const rgba = new Uint8Array(ancho * alto * 4);
  for (let y = 0; y < alto; y++) {
    for (let x = 0; x < ancho; x++) {
      const i = (y * ancho + x) * 4;
      let r = 245, g = 80, b = 26;                                // naranja (rachas)
      if (y >= 2) { r = (x * 9 + y * 3) & 255; g = (x * 5 + 40) & 255; b = (y * 11 + x) & 255; }  // degradés (diferencias y luma)
      if (y === alto - 1 && x % 3 === 0) { r = 245; g = 80; b = 26; }  // vuelve el naranja (índice)
      if (y === 4 && x === 5) { r = 12; g = 200; b = 99; }          // un salto grande (RGB)
      rgba[i] = r; rgba[i + 1] = g; rgba[i + 2] = b; rgba[i + 3] = 255;
    }
  }
  return rgba;
}
const rgbaPrueba = imagenDePrueba(12, 7);
const qoiPrueba = codificarQoi(rgbaPrueba, 12, 7);

// OTA: un manifiesto firmado con una clave de prueba fija (la firma ECDSA
// lleva azar, así que se guarda una sola vez en firma-prueba.json y se reusa).
const binarioPrueba = Uint8Array.from({ length: 1000 }, (_, i) => (i * 7 + 3) & 255);
const shaPrueba = new Uint8Array(crypto.createHash('sha256').update(binarioPrueba).digest());
const archivoFirma = path.join(aqui, 'firma-prueba.json');
let firmaPrueba;
if (fs.existsSync(archivoFirma)) {
  firmaPrueba = JSON.parse(fs.readFileSync(archivoFirma, 'utf8'));
} else {
  const { privateKey, publicKey } = crypto.generateKeyPairSync('ec', { namedCurve: 'P-256' });
  const m = { tamano: binarioPrueba.length, sha256: shaPrueba, version: '0.2.1', perfil: 'dev-147b' };
  const firma = crypto.sign('sha256', Buffer.from(mensajeOta(m)), { key: privateKey, dsaEncoding: 'ieee-p1363' });
  firmaPrueba = {
    nota: 'Clave de PRUEBA para los vectores. No sirve para firmar firmware.',
    publica: publicKey.export({ type: 'spki', format: 'pem' }),
    firma: hex(firma),
  };
  fs.writeFileSync(archivoFirma, JSON.stringify(firmaPrueba, null, 2) + '\n');
}
const desdeHex = (h) => Uint8Array.from(h.split(' ').map((x) => parseInt(x, 16)));
const manifiestoPrueba = { tamano: binarioPrueba.length, sha256: shaPrueba, firma: desdeHex(firmaPrueba.firma), version: '0.2.1', perfil: 'dev-147b' };

const vectores = {
  version: '1.4',
  nota: 'Generado por shared/protocolo/generar_vectores.mjs. No editar a mano. Bytes en hex separados por espacios.',
  comandos: [
    { nombre: 'expresion enamorado', hex: hex(comando.expresion('enamorado')) },
    { nombre: 'expresion automatica', hex: hex(comando.expresion('')) },
    { nombre: 'nombre Toto', hex: hex(comando.nombre('Toto')) },
    { nombre: 'nombre con tilde', texto: 'Ñandú', hex: hex(comando.nombre('Ñandú')) },
    { nombre: 'evento sacudida', hex: hex(comando.evento('sacudida')) },
    { nombre: 'vincular', codigo: 'K7Q2XM', hex: hex(comando.vincular('K7Q2XM')) },
    { nombre: 'autenticar', clave: hex(clave), hex: hex(comando.autenticar(clave)) },
    { nombre: 'desvincular', hex: hex(comando.desvincular()) },
    { nombre: 'pedir config', hex: hex(pedirConfig()) },
    { nombre: 'imagen borrar 3', hex: hex(imagen.borrar(3)) },
    { nombre: 'imagen listar', hex: hex(imagen.listar()) },
    { nombre: 'imagen pedir 5', hex: hex(imagen.pedir(5)) },
    { nombre: 'red', ssid: 'Casa Ñandú', clave: 'secreta123', hex: hex(comando.red('Casa Ñandú', 'secreta123')) },
    { nombre: 'red abierta', ssid: 'Bar', clave: '', hex: hex(comando.red('Bar', '')) },
    { nombre: 'nube', url: 'mqtt://192.168.1.10:1883', usuario: 'A1B2C3D4E5F6', clave: 'k3y', hex: hex(comando.nube('mqtt://192.168.1.10:1883', 'A1B2C3D4E5F6', 'k3y')) },
    { nombre: 'olvidar red', hex: hex(comando.olvidarRed()) },
    { nombre: 'reproducir saludo', hex: hex(comando.reproducir('saludo')) },
    { nombre: 'reproducir parar', hex: hex(comando.reproducir('')) },
    // v1.4: el motor de vida.
    { nombre: 'evento sacudida-fuerte', hex: hex(comando.evento('sacudida-fuerte')) },
    { nombre: 'evento caricia', hex: hex(comando.evento('caricia')) },
    { nombre: 'hora', epoch: 1793000000, zona: -180, hex: hex(comando.hora(1793000000, -180)) },
    { nombre: 'vida', hex: hex(comando.vida()) },
    { nombre: 'vida superposicion', hex: hex(comando.vida(true)) },
    { nombre: 'memoria', hex: hex(comando.memoria()) },
    { nombre: 'registro', hex: hex(comando.registro()) },
    { nombre: 'comportamiento practicar-caras', hex: hex(comando.comportamiento('practicar-caras')) },
    { nombre: 'restaurar', hex: hex(comando.restaurar(codificarMemoria(MEMORIA_PRUEBA))) },
    { nombre: 'ota cancelar', hex: hex(ota.cancelar()) },
  ],
  ota: {
    binario: hex(binarioPrueba),
    sha256: hex(shaPrueba),
    version: manifiestoPrueba.version,
    perfil: manifiestoPrueba.perfil,
    mensaje: mensajeOta(manifiestoPrueba),
    publica: firmaPrueba.publica,
    firma: firmaPrueba.firma,
    manifiesto: hex(manifiestoOta(manifiestoPrueba)),
    url: 'http://192.168.1.10:8080/v1/firmware/7/binario?t=abc',
    descargar: hex(ota.descargar(manifiestoPrueba, 'http://192.168.1.10:8080/v1/firmware/7/binario?t=abc')),
    tamanoBloque: 200,
    escrituras: ota.escrituras(manifiestoPrueba, binarioPrueba, 200).map(hex),
  },
  qoi: {
    ancho: 12,
    alto: 7,
    // Los píxeles de entrada (RGB, fila por fila): un codificador tiene que dar exactamente `hex`.
    rgb: hex(Array.from({ length: 12 * 7 }, (_, i) => [rgbaPrueba[i * 4], rgbaPrueba[i * 4 + 1], rgbaPrueba[i * 4 + 2]]).flat()),
    // RGB565 de cada píxel, fila por fila: lo que tiene que dar el decodificador.
    rgb565: Array.from({ length: 12 * 7 }, (_, i) => {
      const r = rgbaPrueba[i * 4], g = rgbaPrueba[i * 4 + 1], b = rgbaPrueba[i * 4 + 2];
      return ((r & 0xf8) << 8) | ((g & 0xfc) << 3) | (b >> 3);
    }),
    hex: hex(qoiPrueba),
    espacio: 2,
    tamanoBloque: 40,
    escrituras: escriturasDeImagen(2, qoiPrueba, 40).map(hex),
  },
  config: {
    json: ejemplo,
    tamanoBloque: 64,
    escrituras: escriturasDeConfig(ejemplo, 64).map(hex),
  },
  // La memoria del motor de vida (112 bytes con CRC) y lo que dice.
  memoria: { hex: hex(codificarMemoria(MEMORIA_PRUEBA)), campos: { dias: 42, caricias: 321, toques: 1234, racha: 5, hitos: 11, hora9: 30, afecto: 140 } },
  estados: [
    {
      campos: { bateria: null, humor: 0.2, modo: 'mascota', dormido: false, fija: false, emparejando: true, autenticado: false, wifi: false, nube: false, redConfigurada: false, actualizando: false, expresion: 'neutral', nombre: 'bicho', serie: '', id: 'A1B2C3D4E5F6', ancho: 320, alto: 172, redonda: false, version: '0.2.0', perfil: 'dev-147b', crcConfig: 0, encendido: 5 },
    },
    {
      campos: { bateria: 87, humor: -0.35, modo: 'reloj', dormido: true, fija: true, emparejando: false, autenticado: true, wifi: true, nube: true, redConfigurada: true, actualizando: true, expresion: 'animacion:saludo', nombre: 'Ñandú', serie: '001', id: '00AABBCCDDEE', ancho: 240, alto: 240, redonda: true, version: '0.2.0', perfil: 'broche-c3', crcConfig: 0xcbf43926, encendido: 86400 },
    },
  ].map((e) => ({ campos: { ...e.campos, crcConfigHex: e.campos.crcConfig.toString(16).padStart(8, '0') }, hex: estado({ ...e.campos, modo: ['mascota', 'reloj', 'pomodoro', 'clima', 'notificaciones'].indexOf(e.campos.modo) }) })),
  respuestas: [
    { hex: '90 00', op: 0x10, ok: true, codigo: 0, detalle: '' },
    { hex: hex([0x91, 9, ...utf8('vinculá o autenticá primero')]), op: 0x11, ok: false, codigo: 9, detalle: 'vinculá o autenticá primero' },
    { hex: hex([0x93, 0, ...clave]), op: 0x13, ok: true, codigo: 0, clave: hex(clave) },
    { hex: '93 0a', op: 0x13, ok: false, codigo: 10, detalle: '' },
  ],
  qr: [
    { texto: 'SINTETICA:V1/A1B2C3D4E5F6/K7Q2XM', id: 'A1B2C3D4E5F6', codigo: 'K7Q2XM' },
    { texto: 'SINTETICA:V1/A1B2C3D4E5F6/K7Q2X0', id: null },
    { texto: 'https://example.com', id: null },
  ],
  anuncios: [
    { hex: 'ff ff a1 b2 c3 d4 e5 f6', id: 'A1B2C3D4E5F6' },
    { hex: '4c 00 02 15', id: null },
  ],
  crc32: [
    { texto: '123456789', crc: 'cbf43926' },
    { texto: ejemplo, crc: crc32(utf8(ejemplo)).toString(16).padStart(8, '0') },
  ],
};

const destino = path.join(aqui, 'vectores.json');
const texto = JSON.stringify(vectores, null, 2) + '\n';
if (process.argv.includes('--verificar')) {
  if (!fs.existsSync(destino) || fs.readFileSync(destino, 'utf8') !== texto) {
    console.log('vectores.json está desactualizado: correr node shared/protocolo/generar_vectores.mjs');
    process.exit(1);
  }
  console.log('vectores.json al día');
} else {
  fs.writeFileSync(destino, texto);
  console.log(`escrito ${destino}`);
}
