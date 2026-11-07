// Genera shared/protocolo/vectores.json: los bytes de cada operación del
// protocolo BLE v1.1. Los arma el codificador de referencia en JS
// (sim/web/protocolo.js); el núcleo C++ los reproduce en sus tests
// (firmware/test/test_vectores.cpp) y las apps de Kotlin y Swift los usan para
// comprobar sus propios codificadores. Así las cuatro implementaciones no se
// desincronizan.
//
// Uso: node shared/protocolo/generar_vectores.mjs [--verificar]
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { comando, crc32, escriturasDeConfig, escriturasDeImagen, imagen, pedirConfig } from '../../sim/web/protocolo.js';
import { codificarQoi } from '../../sim/web/qoi.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const hex = (b) => Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(' ');
const utf8 = (s) => new TextEncoder().encode(s);
const clave = Uint8Array.from({ length: 16 }, (_, i) => i * 17);

const ejemplo = JSON.stringify(JSON.parse(fs.readFileSync(path.join(aqui, '../formato/config-ejemplo.json'), 'utf8')));

function estado(c) {
  const campo = (s) => { const b = utf8(s); return [b.length, ...b]; };
  const flags = (c.dormido ? 1 : 0) | (c.fija ? 2 : 0) | (c.emparejando ? 4 : 0) | (c.autenticado ? 8 : 0);
  return hex([3, c.bateria ?? 255, Math.round(c.humor * 100) & 0xff, c.modo, flags,
    ...campo(c.expresion), ...campo(c.nombre), ...campo(c.serie), ...campo(c.id),
    c.ancho & 255, c.ancho >> 8, c.alto & 255, c.alto >> 8, c.redonda ? 1 : 0]);
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

const vectores = {
  version: '1.2',
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
  ],
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
  estados: [
    {
      campos: { bateria: null, humor: 0.2, modo: 'mascota', dormido: false, fija: false, emparejando: true, autenticado: false, expresion: 'neutral', nombre: 'bicho', serie: '', id: 'A1B2C3D4E5F6', ancho: 320, alto: 172, redonda: false },
    },
    {
      campos: { bateria: 87, humor: -0.35, modo: 'reloj', dormido: true, fija: true, emparejando: false, autenticado: true, expresion: 'dormido', nombre: 'Ñandú', serie: '001', id: '00AABBCCDDEE', ancho: 240, alto: 240, redonda: true },
    },
  ].map((e) => ({ ...e, hex: estado({ ...e.campos, modo: ['mascota', 'reloj', 'pomodoro', 'clima', 'notificaciones'].indexOf(e.campos.modo) }) })),
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
