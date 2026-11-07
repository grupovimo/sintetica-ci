// Arma sim/dist/simulador.html: la página con los scripts y el wasm adentro,
// en un solo archivo (es lo que se publica como Artifact).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const leer = (p) => fs.readFileSync(path.join(aqui, p), 'utf8');

// Los módulos se juntan en uno: se sacan los import locales y los export.
const modulo = (texto) => texto
  .replace(/^import[\s\S]*?from '\.\/[^']+';\n/gm, '')
  .replace(/^export (async function|function|const|class)/gm, '$1');

// Los módulos van a un solo ámbito: dos nombres iguales al primer nivel romperían la página.
const modulos = ['web/protocolo.js', 'web/qoi.js', 'web/sim.js', 'web/app.js'];
const vistos = new Map();
for (const m of modulos) {
  for (const [, nombre] of leer(m).matchAll(/^(?:export )?(?:async )?(?:const|let|var|function|class) ([A-Za-z_$][\w$]*)/gm)) {
    if (vistos.has(nombre)) throw new Error(`"${nombre}" está en ${vistos.get(nombre)} y en ${m}: renombrá uno`);
    vistos.set(nombre, m);
  }
}

const wasm = fs.readFileSync(path.join(aqui, 'dist/sinteticaos.wasm')).toString('base64');
const ejemplo = fs.readFileSync(path.join(aqui, '../shared/formato/config-ejemplo.json'), 'utf8');
const codigo = [
  `globalThis.SINTETICA_WASM_B64 = ${JSON.stringify(wasm)};`,
  `globalThis.SINTETICA_EJEMPLO = ${JSON.stringify(ejemplo)};`,
  ...modulos.map((m) => modulo(leer(m))),
].join('\n');

const html = leer('web/index.html').replace(
  '<script type="module" src="app.js"></script>',
  `<script type="module">\n${codigo.replace(/<\/script/g, '<\\/script')}\n</script>`,
);
fs.writeFileSync(path.join(aqui, 'dist/simulador.html'), html);
console.log(`sim/dist/simulador.html (${(html.length / 1024).toFixed(0)} KB)`);
