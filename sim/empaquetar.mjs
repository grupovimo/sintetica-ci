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

const wasm = fs.readFileSync(path.join(aqui, 'dist/sinteticaos.wasm')).toString('base64');
const ejemplo = fs.readFileSync(path.join(aqui, '../shared/formato/config-ejemplo.json'), 'utf8');
const codigo = [
  `globalThis.SINTETICA_WASM_B64 = ${JSON.stringify(wasm)};`,
  `globalThis.SINTETICA_EJEMPLO = ${JSON.stringify(ejemplo)};`,
  modulo(leer('web/protocolo.js')),
  modulo(leer('web/sim.js')),
  modulo(leer('web/app.js')),
].join('\n');

const html = leer('web/index.html').replace(
  '<script type="module" src="app.js"></script>',
  `<script type="module">\n${codigo.replace(/<\/script/g, '<\\/script')}\n</script>`,
);
fs.writeFileSync(path.join(aqui, 'dist/simulador.html'), html);
console.log(`sim/dist/simulador.html (${(html.length / 1024).toFixed(0)} KB)`);
