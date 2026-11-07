// Deja en static/ lo que la web toma del resto del repo: el núcleo en
// WebAssembly (lo compila si hace falta) y las tipografías de la marca.
import { execSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const raiz = path.resolve(web, '..');
const wasm = path.join(raiz, 'sim/dist/sinteticaos.wasm');

const fuentesNucleo = fs.readdirSync(path.join(raiz, 'firmware/nucleo')).map((f) => path.join(raiz, 'firmware/nucleo', f));
const viejo = !fs.existsSync(wasm) || fuentesNucleo.some((f) => fs.statSync(f).mtimeMs > fs.statSync(wasm).mtimeMs);
if (viejo) {
  console.log('[preparar] compilando el núcleo a WebAssembly…');
  execSync('sh sim/compilar.sh', { cwd: raiz, stdio: 'inherit' });
}
fs.mkdirSync(path.join(web, 'static/fuentes'), { recursive: true });
fs.copyFileSync(wasm, path.join(web, 'static/sinteticaos.wasm'));
for (const f of ['Fraunces.ttf', 'JetBrainsMono.ttf']) {
  fs.copyFileSync(path.join(raiz, 'firmware/herramientas/fuentes', f), path.join(web, 'static/fuentes', f));
}
console.log('[preparar] listo: static/sinteticaos.wasm y las tipografías');
