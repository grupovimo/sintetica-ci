// Rasteriza las caras de referencia del manual (shared/formato/referencia/*.svg)
// con Chromium, a 320 × 172 y escala 1, y las guarda como PNG en ./referencia.
// Uso: node firmware/test/oro/rasterizar.cjs
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

(async () => {
  const origen = path.resolve(__dirname, '../../../shared/formato/referencia');
  const destino = path.resolve(__dirname, 'referencia');
  fs.mkdirSync(destino, { recursive: true });
  const navegador = await chromium.launch();
  const pagina = await navegador.newPage({ viewport: { width: 320, height: 172 }, deviceScaleFactor: 1 });
  for (const archivo of fs.readdirSync(origen).filter((f) => f.endsWith('.svg')).sort()) {
    const svg = fs.readFileSync(path.join(origen, archivo), 'utf8');
    await pagina.setContent(`<html><body style="margin:0;background:#0B0A09">${svg}</body></html>`);
    await pagina.screenshot({ path: path.join(destino, archivo.replace('.svg', '.png')), clip: { x: 0, y: 0, width: 320, height: 172 } });
  }
  await navegador.close();
  console.log('referencias en', destino);
})();
