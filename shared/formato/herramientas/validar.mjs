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
const validarExpresion = ajv.compile({ $ref: 'https://sinteticalabs.com/schema/config/v1.json#/$defs/expresion' });

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
// Una config con todas las caras base como expresiones propias también es válida.
const todas = { version: 1, expresiones: {} };
for (const f of fs.readdirSync(path.join(formato, 'caras')).filter((f) => f.endsWith('.json'))) {
  const c = JSON.parse(fs.readFileSync(path.join(formato, 'caras', f), 'utf8'));
  todas.expresiones[c.nombre] = c;
}
revisar('config con las 16 caras', validarConfig, todas);
process.exit(errores ? 1 : 0);
