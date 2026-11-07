// Sirve el simulador en http://localhost:8147 (solo sim/ y shared/formato/).
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const permitidas = ['sim/web/', 'sim/dist/', 'shared/formato/'];
const tipos = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.wasm': 'application/wasm', '.json': 'application/json' };
const puerto = Number(process.env.PUERTO ?? 8147);

http.createServer((req, res) => {
  const url = new URL(req.url, 'http://x');
  if (url.pathname === '/') { res.writeHead(302, { Location: '/sim/web/index.html' }); res.end(); return; }
  const rel = path.posix.normalize(decodeURIComponent(url.pathname)).replace(/^\/+/, '');
  if (!permitidas.some((p) => rel.startsWith(p))) { res.writeHead(404); res.end('no'); return; }
  const archivo = path.join(raiz, rel);
  fs.readFile(archivo, (err, datos) => {
    if (err) { res.writeHead(404); res.end('no está'); return; }
    let cuerpo = datos;
    // El HTML va sin <!doctype>/<head> (así se publica); localmente se los agrego.
    if (archivo.endsWith('.html')) cuerpo = '<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">' + datos;
    res.writeHead(200, { 'Content-Type': tipos[path.extname(archivo)] ?? 'application/octet-stream', 'Cache-Control': 'no-store' });
    res.end(cuerpo);
  });
}).listen(puerto, () => console.log(`Simulador en http://localhost:${puerto}`));
