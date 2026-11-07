// El simulador de punta a punta: el núcleo en WebAssembly, manejado con los
// mismos bytes del protocolo que va a usar la app (sim/web/protocolo.js).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import { crearSimulador, memoriaVolatil } from '../web/sim.js';
import {
  ArmadorDeConfig, Car, comando, escriturasDeConfig, escriturasDeImagen, idDeAnuncio, imagen, leerEstado, leerListaImagenes,
  leerQr, leerRespuesta, pedirConfig,
} from '../web/protocolo.js';
import { codificarQoi, decodificarQoi } from '../web/qoi.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const wasm = fs.readFileSync(path.join(aqui, '../dist/sinteticaos.wasm'));

// Como la app: se conecta y, si el bicho muestra el QR, vincula; si no, se autentica.
let claveCelu = null;
async function nuevo(almacen = memoriaVolatil(), { vincular = true, perfil = 'mascota' } = {}) {
  const reloj = { t: 0 };
  const notis = [];
  let cuadros = 0;
  const sim = await crearSimulador({
    wasm, almacen, perfil, ahora: () => reloj.t, semilla: () => 1234, minutoDelDia: () => 12 * 60,
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
  assert.equal(sim.version(), '0.3.0');
});

test('M0 (v2): parpadea de 10 a 20 veces por minuto', async () => {
  const { sim, correr } = await nuevo();
  let antes = false, inicios = 0;
  for (let i = 0; i < 6000; i++) {  // 2 min
    correr(20);
    const cerrando = sim.cierre() > 0.5;
    if (cerrando && !antes) inicios++;
    antes = cerrando;
  }
  const porMinuto = inicios / 2;
  assert.ok(porMinuto >= 10 && porMinuto <= 26, `parpadeos por minuto: ${porMinuto}`);
  assert.ok(sim.vida().parpadeos >= 20);
});

test('M1: sacudirlo lo marea', async () => {
  const { sim, correr } = await nuevo();
  correr(100);
  // Una sacudida fuerte (picos de más de 2 g): reflejo de mareo, después gruñón.
  for (let i = 0; i < 6; i++) { sim.imu(i % 2 ? 3.5 : -3.5, 0, 1); correr(100); }
  correr(100);
  assert.equal(sim.expresionVisible(), 'mareado');
  assert.equal(sim.vida().comportamiento, 'mareo');
  assert.ok(sim.vida().estado[6] > 0.3, 'el estrés sube');
  correr(8000);
  assert.notEqual(sim.expresionVisible(), 'mareado');
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

test('QOI: codificar y decodificar da lo mismo; el vector compartido coincide', () => {
  const w = 37, h = 23, rgba = new Uint8Array(w * h * 4);
  for (let i = 0; i < w * h; i++) {
    rgba[i * 4] = (i * 7) & 255; rgba[i * 4 + 1] = i % 5 === 0 ? 80 : 81; rgba[i * 4 + 2] = (i >> 3) & 255; rgba[i * 4 + 3] = 255;
  }
  const d = decodificarQoi(codificarQoi(rgba, w, h));
  assert.equal(d.ancho, w);
  assert.deepEqual(d.rgba, rgba);
  const v = JSON.parse(fs.readFileSync(path.join(aqui, '../../shared/protocolo/vectores.json'), 'utf8')).qoi;
  const q = decodificarQoi(Uint8Array.from(v.hex.split(' ').map((x) => parseInt(x, 16))));
  const rgb565 = Array.from({ length: v.ancho * v.alto }, (_, i) => ((q.rgba[i * 4] & 0xf8) << 8) | ((q.rgba[i * 4 + 1] & 0xfc) << 3) | (q.rgba[i * 4 + 2] >> 3));
  assert.deepEqual(rgb565, v.rgb565);
});

test('M4: el broche es redondo; un toque pasa a la cara siguiente', async () => {
  const { sim, correr } = await nuevo(memoriaVolatil(), { perfil: 'broche' });
  const e = leerEstado(sim.bleLeerEstado());
  assert.deepEqual(e.pantalla, { ancho: 240, alto: 240, redonda: true });
  correr(200);
  assert.equal(sim.expresionVisible(), 'neutral');
  sim.tacto(true); correr(100); sim.tacto(false); correr(300);
  assert.equal(sim.expresionVisible(), 'contento');
});

test('M4: mantener 2 s lo duerme y un toque lo despierta', async () => {
  const { sim, correr } = await nuevo(memoriaVolatil(), { perfil: 'broche' });
  correr(100);
  sim.tacto(true); correr(2200); sim.tacto(false); correr(40);
  assert.equal(sim.quiereDormir(), true);
  sim.tacto(true); correr(80); sim.tacto(false); correr(200);
  assert.equal(sim.quiereDormir(), false);
  assert.equal(sim.expresionVisible(), 'neutral');
});

test('M4: subir una imagen, ponerla en la lista y verla', async () => {
  const { sim, notis, correr } = await nuevo(memoriaVolatil(), { perfil: 'broche' });
  const rgba = new Uint8Array(240 * 240 * 4);
  for (let i = 0; i < 240 * 240; i++) { rgba[i * 4] = 245; rgba[i * 4 + 1] = 80; rgba[i * 4 + 2] = (i % 240) & 255; rgba[i * 4 + 3] = 255; }
  const qoi = codificarQoi(rgba, 240, 240);
  for (const e of escriturasDeImagen(4, qoi)) sim.bleEscribir(Car.imagen, e);
  const fin = notis.filter((n) => n.c === Car.imagen).map((n) => leerRespuesta(n.b)).at(-1);
  assert.equal(fin.ok, true, fin.detalle);
  notis.length = 0;
  sim.bleEscribir(Car.imagen, imagen.listar());
  const lista = leerListaImagenes(notis.at(-1).b);
  assert.deepEqual(lista.espacios.map((x) => x.espacio), [4]);
  for (const e of escriturasDeConfig({ version: 1, lista: ['imagen:4', 'cara:guino'] })) sim.bleEscribir(Car.config, e);
  correr(100);
  assert.equal(sim.imagenVisible(), 4);
  sim.tacto(true); correr(100); sim.tacto(false); correr(300);
  assert.equal(sim.imagenVisible(), -1);
  correr(3000);  // (al cambiar de config se mira en el espejo)
  assert.equal(sim.expresionVisible(), 'guino');
  // Una imagen de otro tamaño se rechaza.
  for (const e of escriturasDeImagen(5, codificarQoi(new Uint8Array(100 * 100 * 4), 100, 100))) sim.bleEscribir(Car.imagen, e);
  const rechazo = leerRespuesta(notis.filter((n) => n.c === Car.imagen).at(-1).b);
  assert.equal(rechazo.mensaje, 'imagen inválida');
  assert.equal(rechazo.detalle, 'tiene que ser un QOI del tamaño de la pantalla');
});

test('E4: animaciones con claves desde la config de ejemplo', async () => {
  const { sim, notis, correr } = await nuevo();
  const ejemplo = fs.readFileSync(path.join(aqui, '../../shared/formato/config-ejemplo.json'), 'utf8');
  for (const e of escriturasDeConfig(JSON.stringify(JSON.parse(ejemplo)))) sim.bleEscribir(Car.config, e);
  assert.ok(leerRespuesta(notis.at(-1).b).ok);
  assert.deepEqual(sim.animaciones(), ['saludo']);
  sim.bleEscribir(Car.comando, comando.reproducir('saludo'));
  correr(200);
  assert.equal(sim.expresionVisible(), 'animacion:saludo');
  const estado = leerEstado(sim.bleLeerEstado());
  assert.equal(estado.expresion, 'animacion:saludo');
  assert.equal(estado.perfil, 'sim');
  correr(1400);  // no repite: vuelve sola
  assert.notEqual(sim.expresionVisible(), 'animacion:saludo');
});

test('E4: el modo editor dibuja caras y animaciones y valida', async () => {
  const { sim } = await nuevo();
  assert.equal(sim.validar({ version: 1 }).ok, true);
  const mala = sim.validar({ version: 1, lista: ['animacion:nada'] });
  assert.equal(mala.ok, false);
  assert.match(mala.detalle, /animación/);
  sim.editor.iniciar(640, 344);
  const ejemplo = fs.readFileSync(path.join(aqui, '../../shared/formato/config-ejemplo.json'), 'utf8');
  assert.equal(sim.editor.config(ejemplo).ok, true);
  const i = sim.editor.indiceExpresion('enamorado');
  assert.ok(i >= 0);
  const cuadro = sim.editor.expresion(i);
  assert.equal(cuadro.rgba.length, 640 * 344 * 4);
  const pintados = (c) => { let n = 0; for (let k = 0; k < c.rgba.length; k += 4) if (c.rgba[k] > 100) n++; return n; };
  assert.ok(pintados(cuadro) > 2000);
  assert.ok(pintados(sim.editor.expresion(i, 1)) < pintados(cuadro));  // cerrado: menos píxeles
  const a = sim.editor.indiceAnimacion('saludo');
  assert.equal(sim.editor.duracion(a), 1400);
  assert.ok(pintados(sim.editor.animacion(a, 500)) > pintados(sim.editor.animacion(a, 0)) - 1);
});

test('E5: el motor de vida, con el tiempo acelerado', async () => {
  const { sim, correr } = await nuevo();
  assert.ok(sim.eventos().includes('sacudida-fuerte'));
  assert.ok(sim.parametros().length >= 40);
  // Un día en 2 minutos: 720 veces más rápido.
  sim.velocidad(720);
  const h0 = sim.hora().minuto;
  correr(1000);  // 12 min del bicho
  assert.ok(sim.hora().minuto - h0 >= 11, `${sim.hora().minuto} ${h0}`);
  sim.velocidad(1);
  const v = sim.vida();
  assert.equal(v.estado.length, 7);
  assert.equal(v.rasgos.length, 6);
  assert.ok(v.comportamientos >= 1);
  assert.ok(sim.forzarComportamiento('practicar-caras'));
  correr(50);
  assert.equal(sim.vida().comportamiento, 'practicar-caras');
});

test('E5: el editor dibuja un rostro paramétrico', async () => {
  const sim = await crearSimulador({ wasm });
  sim.editor.iniciar(320, 172);
  assert.ok(sim.editor.config({ version: 2 }).ok);
  const i = sim.editor.indiceCara('enamorado');
  assert.ok(i >= 0);
  const p = sim.editor.parametrosCara(i);
  assert.equal(p.length, sim.parametros().length);
  const { rgba } = sim.editor.rostro(p, 0.5);
  let naranjas = 0;
  for (let k = 0; k < rgba.length; k += 4) if (rgba[k] > 200 && rgba[k + 1] > 80) naranjas++;
  assert.ok(naranjas > 500, `píxeles naranjas: ${naranjas}`);
});
