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

import { comando, crc32, escriturasDeConfig, pedirConfig } from '../../sim/web/protocolo.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const hex = (b) => Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(' ');
const utf8 = (s) => new TextEncoder().encode(s);
const clave = Uint8Array.from({ length: 16 }, (_, i) => i * 17);

const ejemplo = JSON.stringify(JSON.parse(fs.readFileSync(path.join(aqui, '../formato/config-ejemplo.json'), 'utf8')));

function estado(c) {
  const campo = (s) => { const b = utf8(s); return [b.length, ...b]; };
  const flags = (c.dormido ? 1 : 0) | (c.fija ? 2 : 0) | (c.emparejando ? 4 : 0) | (c.autenticado ? 8 : 0);
  return hex([2, c.bateria ?? 255, Math.round(c.humor * 100) & 0xff, c.modo, flags,
    ...campo(c.expresion), ...campo(c.nombre), ...campo(c.serie), ...campo(c.id)]);
}

const vectores = {
  version: '1.1',
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
  ],
  config: {
    json: ejemplo,
    tamanoBloque: 64,
    escrituras: escriturasDeConfig(ejemplo, 64).map(hex),
  },
  estados: [
    {
      campos: { bateria: null, humor: 0.2, modo: 'mascota', dormido: false, fija: false, emparejando: true, autenticado: false, expresion: 'neutral', nombre: 'bicho', serie: '', id: 'A1B2C3D4E5F6' },
    },
    {
      campos: { bateria: 87, humor: -0.35, modo: 'reloj', dormido: true, fija: true, emparejando: false, autenticado: true, expresion: 'dormido', nombre: 'Ñandú', serie: '001', id: '00AABBCCDDEE' },
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
