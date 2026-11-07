// El simulador de punta a punta: el núcleo en WebAssembly, manejado con los
// mismos bytes del protocolo que va a usar la app (sim/web/protocolo.js).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { crearSimulador, memoriaVolatil } from '../web/sim.js';
import {
  ArmadorDeConfig, Car, comando, escriturasDeConfig, idDeAnuncio, leerEstado, leerQr, leerRespuesta, pedirConfig,
} from '../web/protocolo.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const wasm = fs.readFileSync(path.join(aqui, '../dist/sinteticaos.wasm'));

// Como la app: se conecta y, si el bicho muestra el QR, vincula; si no, se autentica.
let claveCelu = null;
async function nuevo(almacen = memoriaVolatil(), { vincular = true } = {}) {
  const reloj = { t: 0 };
  const notis = [];
  let cuadros = 0;
  const sim = await crearSimulador({
    wasm, almacen, ahora: () => reloj.t, semilla: () => 1234, minutoDelDia: () => 12 * 60,
    alNotificar: (c, b) => notis.push({ c, b }), alPresentar: () => cuadros++,
  });
  const correr = (ms) => { for (let i = 0; i < ms; i += 20) { reloj.t += 20; sim.tick(); } };
  sim.bleConexion(true);
  if (vincular) {
    if (sim.emparejando()) {
      sim.bleEscribir(Car.comando, comando.vincular(leerQr(sim.qr()).codigo));
      claveCelu = leerRespuesta(notis.filter((n) => n.c === Car.comando).at(-1).b).clave;
      correr(3200);  // que pase el guiño del vínculo
    } else {
      sim.bleEscribir(Car.comando, comando.autenticar(claveCelu));
    }
    notis.length = 0;
  }
  return { sim, reloj, notis, correr, cuadros: () => cuadros, almacen };
}

test('arranca con las 16 caras y en neutral', async () => {
  const { sim, correr, cuadros } = await nuevo(memoriaVolatil(), { vincular: false });
  assert.equal(sim.expresiones().length, 16);
  assert.deepEqual(sim.expresiones().slice(0, 3), ['neutral', 'contento', 'sospecha']);
  correr(200);
  assert.equal(sim.expresionVisible(), 'neutral');
  assert.equal(cuadros(), 10);
  assert.equal(sim.emparejando(), true);
  assert.equal(sim.version(), '0.1.0');
});

test('M0: parpadea al azar cada 3 a 7 s', async () => {
  const { sim, reloj, correr } = await nuevo();
  const inicios = [];
  let antes = false;
  for (let i = 0; i < 3000; i++) {
    correr(10);
    const cerrando = sim.cierre() > 0;
    if (cerrando && !antes) inicios.push(reloj.t);
    antes = cerrando;
  }
  assert.ok(inicios.length >= 4, `parpadeos: ${inicios.length}`);
  for (let i = 1; i < inicios.length; i++) {
    const d = inicios[i] - inicios[i - 1];
    assert.ok((d >= 3000 && d <= 7020) || d === 260, `intervalo ${d}`);
  }
});

test('M1: sacudirlo lo marea', async () => {
  const { sim, correr } = await nuevo();
  correr(100);
  for (let i = 0; i < 6; i++) { sim.imu(i % 2 ? 2.5 : -2.5, 0, 1); correr(100); }
  correr(100);
  assert.equal(sim.expresionVisible(), 'mareado');
  correr(3500);
  assert.equal(sim.expresionVisible(), 'neutral');
});

test('M2: expresión y nombre por BLE sobreviven a un reinicio', async () => {
  const almacen = memoriaVolatil();
  const a = await nuevo(almacen);
  a.sim.bleEscribir(Car.comando, comando.expresion('enamorado'));
  a.sim.bleEscribir(Car.comando, comando.nombre('Toto'));
  const resp = a.notis.filter((n) => n.c === Car.comando).map((n) => leerRespuesta(n.b));
  assert.deepEqual(resp.map((r) => r.ok), [true, true]);
  a.correr(300);
  assert.equal(a.sim.expresionVisible(), 'enamorado');

  const b = await nuevo(almacen);  // reinicio
  b.correr(300);
  assert.equal(b.sim.expresionVisible(), 'enamorado');
  assert.equal(b.sim.nombre(), 'Toto');
  const estado = leerEstado(b.sim.bleLeerEstado());
  assert.equal(estado.id, 'A1B2C3D4E5F6');
  assert.equal(estado.autenticado, true);
  assert.equal(estado.nombre, 'Toto');
  assert.equal(estado.expresion, 'enamorado');
  assert.equal(estado.fija, true);
});

test('M2: config por bloques, ida y vuelta', async () => {
  const { sim, notis, correr } = await nuevo();
  const config = JSON.parse(fs.readFileSync(path.join(aqui, '../../shared/formato/config-ejemplo.json'), 'utf8'));
  for (const e of escriturasDeConfig(config, 64)) sim.bleEscribir(Car.config, e);
  const fin = notis.filter((n) => n.c === Car.config).map((n) => leerRespuesta(n.b)).at(-1);
  assert.equal(fin.ok, true, fin.detalle);
  assert.equal(sim.nombre(), 'Toto');

  notis.length = 0;
  sim.bleEscribir(Car.config, pedirConfig());
  const armador = new ArmadorDeConfig();
  let vuelta = null;
  for (const n of notis.filter((n) => n.c === Car.config)) vuelta = armador.agregar(leerRespuesta(n.b)) ?? vuelta;
  assert.deepEqual(vuelta, config);

  correr(100);
  sim.bleEscribir(Car.comando, comando.evento('sacudida'));
  correr(200);
  assert.equal(sim.expresionVisible(), 'mareado');
});

test('M2: una config inválida explica por qué y no cambia nada', async () => {
  const { sim, notis } = await nuevo();
  for (const e of escriturasDeConfig({ version: 1, cara: { tamano: 9 } })) sim.bleEscribir(Car.config, e);
  const r = leerRespuesta(notis.filter((n) => n.c === Car.config).at(-1).b);
  assert.equal(r.ok, false);
  assert.equal(r.mensaje, 'config inválida');
  assert.match(r.detalle, /tamano/);
  assert.equal(sim.nombre(), 'bicho');
});

test('M3: sin vincular no se escribe; con el QR se vincula y la clave sirve después', async () => {
  const almacen = memoriaVolatil();
  const a = await nuevo(almacen, { vincular: false });
  a.sim.bleEscribir(Car.comando, comando.nombre('Intruso'));
  assert.equal(leerRespuesta(a.notis.at(-1).b).mensaje, 'no autorizado');
  const qr = leerQr(a.sim.qr());
  assert.deepEqual(qr && qr.id, 'A1B2C3D4E5F6');
  a.sim.bleEscribir(Car.comando, comando.vincular('ZZZZZZ'));
  assert.equal(leerRespuesta(a.notis.at(-1).b).ok, false);
  a.sim.bleEscribir(Car.comando, comando.vincular(qr.codigo));
  const r = leerRespuesta(a.notis.filter((n) => n.c === Car.comando).at(-1).b);
  assert.equal(r.ok, true);
  assert.equal(r.clave.length, 16);
  assert.equal(a.sim.emparejando(), false);

  const b = await nuevo(almacen, { vincular: false });  // reinicio y conexión nueva
  b.sim.bleEscribir(Car.comando, comando.nombre('Toto'));
  assert.equal(leerRespuesta(b.notis.at(-1).b).mensaje, 'no autorizado');
  b.sim.bleEscribir(Car.comando, comando.autenticar(r.clave));
  b.sim.bleEscribir(Car.comando, comando.nombre('Toto'));
  assert.equal(leerRespuesta(b.notis.at(-1).b).ok, true);
});

test('M3: el id del anuncio BLE y el número de serie', async () => {
  assert.equal(idDeAnuncio(Uint8Array.from([0xff, 0xff, 0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6])), 'A1B2C3D4E5F6');
  assert.equal(idDeAnuncio(Uint8Array.from([0x4c, 0x00, 1, 2])), null);
  const { sim } = await nuevo();
  sim.fijarSerie('001');
  assert.equal(leerEstado(sim.bleLeerEstado()).serie, '001');
});
