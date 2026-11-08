// Valida las caras base y la config de ejemplo contra el JSON Schema.
// Uso: node shared/formato/herramientas/validar.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const formato = path.resolve(aqui, '..');
const schema = JSON.parse(fs.readFileSync(path.join(formato, 'config.schema.json'), 'utf8'));
const ajv = new Ajv2020({ allErrors: true, strict: true, allowUnionTypes: true });
const validarConfig = ajv.compile(schema);
const validarExpresion = ajv.compile({ $ref: 'https://sinteticalabs.com/schema/config/v2.json#/$defs/expresion' });

let errores = 0;
function revisar(nombre, validar, datos) {
  if (validar(datos)) {
    console.log(`ok   ${nombre}`);
  } else {
    errores++;
    console.log(`MAL  ${nombre}`);
    for (const e of validar.errors) console.log(`     ${e.instancePath} ${e.message}`);
  }
}

revisar('config-ejemplo.json', validarConfig, JSON.parse(fs.readFileSync(path.join(formato, 'config-ejemplo.json'), 'utf8')));
for (const f of fs.readdirSync(path.join(formato, 'caras')).filter((f) => f.endsWith('.json')).sort()) {
  revisar(`caras/${f}`, validarExpresion, JSON.parse(fs.readFileSync(path.join(formato, 'caras', f), 'utf8')));
}
const validarAnimacion = ajv.compile({ $ref: 'https://sinteticalabs.com/schema/config/v2.json#/$defs/animacion' });
const animaciones = {};
for (const f of fs.readdirSync(path.join(formato, 'animaciones')).filter((f) => f.endsWith('.json')).sort()) {
  const { nombre, ...a } = JSON.parse(fs.readFileSync(path.join(formato, 'animaciones', f), 'utf8'));
  revisar(`animaciones/${f}`, validarAnimacion, a);
  animaciones[nombre] = a;
}
revisar('config con las animaciones de ejemplo', validarConfig, { version: 1, animaciones });
// Una config con todas las caras base como expresiones propias también es válida.
const todas = { version: 1, expresiones: {} };
for (const f of fs.readdirSync(path.join(formato, 'caras')).filter((f) => f.endsWith('.json'))) {
  const c = JSON.parse(fs.readFileSync(path.join(formato, 'caras', f), 'utf8'));
  todas.expresiones[c.nombre] = c;
}
revisar('config con las 16 caras', validarConfig, todas);

// El motor de vida: las expresiones, los comportamientos y las reglas base
// tienen que ser una config v2 válida (así el usuario puede copiarlos y editarlos).
const vida = (f) => JSON.parse(fs.readFileSync(path.join(formato, 'vida', f), 'utf8'));
const validarVida = ajv.compile({ $ref: 'https://sinteticalabs.com/schema/config/v2.json#/$defs/expresionVida' });
for (const [n, e] of Object.entries(vida('expresiones.json'))) revisar(`vida/expresiones: ${n}`, validarVida, e);
const validarComp = ajv.compile({ $ref: 'https://sinteticalabs.com/schema/config/v2.json#/$defs/comportamiento' });
for (const [n, c] of Object.entries(vida('comportamientos.json'))) revisar(`vida/comportamientos: ${n}`, validarComp, c);
revisar('vida/reglas.json (de a 24)', validarConfig, { version: 2, reglas: vida('reglas.json').slice(0, 24) });
revisar('vida/reglas.json (el resto)', validarConfig, { version: 2, reglas: vida('reglas.json').slice(24) });
const per = vida('personalidades.json');
for (const [n, p] of Object.entries(per)) revisar(`vida/personalidades: ${n}`, validarConfig, { version: 2, personalidad: { preset: n, ...p } });
// Una cara de imagen (sprites) como la cara de siempre.
const conImagen = {
  version: 2,
  cara: { expresion: 'anime' },
  expresiones: { anime: { sprites: { ojo: 0, ojo_cerrado: 1, boca: 2 }, tamano: 0.4, boca_y: -0.3, sep: 0.32, ojo: { lt: 0.1 }, movimiento: 'rebote' } },
};
revisar('config con una cara de imagen', validarConfig, conImagen);
const malas = [
  { version: 2, expresiones: { a: { sprites: { boca: 1 } } } },
  { version: 2, expresiones: { a: { sprites: { ojo: 12 } } } },
  { version: 2, expresiones: { a: { tamano: 0.4, gx: 0.1 } } },
];
for (const m of malas) {
  if (validarConfig(m)) {
    errores++;
    console.log(`MAL  una cara de imagen inválida pasó: ${JSON.stringify(m)}`);
  } else console.log(`ok   rechaza ${JSON.stringify(m.expresiones)}`);
}
revisar('config-ejemplo-v2.json', validarConfig, JSON.parse(fs.readFileSync(path.join(formato, 'config-ejemplo-v2.json'), 'utf8')));
process.exit(errores ? 1 : 0);
