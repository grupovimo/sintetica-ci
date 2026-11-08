// Paridad de la config en el WebAssembly (lo que usan el simulador y la web):
// cada config de shared/formato/paridad se acepta o se rechaza igual que en el
// núcleo de la PC (firmware/test/test_paridad.cpp) y en cgo. Por los dos
// caminos de la web: validar (el editor) y la config entera por BLE (la vista previa).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { crearSimulador } from '../web/sim.js';
import { Car, escriturasDeConfig } from '../web/protocolo.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const dir = path.join(aqui, '../../shared/formato/paridad');
const wasm = fs.readFileSync(path.join(aqui, '../dist/sinteticaos.wasm'));
const indice = JSON.parse(fs.readFileSync(path.join(dir, 'paridad.json'), 'utf8'));
const leer = (c) => fs.readFileSync(path.join(dir, 'configs', c.archivo), 'utf8');

test('paridad: validar acepta y rechaza como el núcleo', async () => {
  const sim = await crearSimulador({ wasm });
  assert.ok(indice.casos.length > 50);
  const mal = [];
  for (const c of indice.casos) {
    const r = sim.validar(leer(c));
    const bien = c.acepta ? r.ok : !r.ok && r.codigo === c.codigo && r.detalle.startsWith(c.detalle);
    if (!bien) mal.push(`${c.archivo}: ${JSON.stringify(r)}`);
  }
  assert.deepEqual(mal, []);
});

test('paridad: la config entera por BLE (vista previa) acepta y rechaza igual', async () => {
  let ultima = null, antes = false;
  const sim = await crearSimulador({
    wasm, alNotificar: (car, b) => {
      if (car !== Car.config) return;
      if (b[0] !== 0x83 && b[1] !== 0) antes = true;  // la cortó antes del fin (por el largo)
      ultima = b;
    },
  });
  sim.vistaPrevia();
  const mal = [];
  for (const c of indice.casos) {
    ultima = null;
    antes = false;
    for (const e of escriturasDeConfig(leer(c), 4096)) sim.bleEscribir(Car.config, e);
    // 0x83: la respuesta al fin de la config (0 = la aplicó).
    const ok = !antes && ultima?.[0] === 0x83 && ultima[1] === 0;
    const detalle = ultima?.[0] === 0x83 ? new TextDecoder().decode(ultima.subarray(2)) : '';
    const bien = c.acepta ? ok : !ok && (antes || detalle.startsWith(c.detalle));
    if (!bien) mal.push(`${c.archivo}: ${ultima && Array.from(ultima.subarray(0, 2))} ${detalle}`);
  }
  assert.deepEqual(mal, []);
});
