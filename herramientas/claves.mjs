// Genera el par de claves de las OTA: claves/ota-privada.pem (la usa la API
// para firmar; no sale de tu máquina ni del servidor) y claves/ota-publica.pem
// (el firmware la embebe al compilar y el bicho virtual la lee).
//
// Uso: npm run claves            (no pisa claves que ya existen)
//      npm run claves -- --nuevas (genera otras: los bichos con firmware viejo
//                                  van a rechazar lo que firme la nueva)
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const dir = path.join(raiz, 'claves');
const privada = path.join(dir, 'ota-privada.pem');
const publica = path.join(dir, 'ota-publica.pem');

if (fs.existsSync(privada) && !process.argv.includes('--nuevas')) {
  console.log('Ya hay claves en claves/ (para generar otras: npm run claves -- --nuevas).');
  process.exit(0);
}
fs.mkdirSync(dir, { recursive: true });
const par = crypto.generateKeyPairSync('ec', { namedCurve: 'P-256' });
fs.writeFileSync(privada, par.privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 });
fs.writeFileSync(publica, par.publicKey.export({ type: 'spki', format: 'pem' }));
console.log('Listo: claves/ota-privada.pem (secreta) y claves/ota-publica.pem.');
console.log('Compilá el firmware de nuevo para que la placa confíe en esta clave.');
