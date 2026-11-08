// Decodificador de GIF (87a y 89a, animados) para pasarlos a la pantalla del
// bicho. Devuelve cada cuadro ya compuesto (la imagen entera, como se ve), con
// su duración. Sin dependencias: lo usan el simulador, la web y los tests.

// decodificarGif(bytes) → { ancho, alto, cuadros: [{ rgba: Uint8ClampedArray, ms }] }
export function decodificarGif(bytes) {
  const b = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  const firma = String.fromCharCode(...b.subarray(0, 6));
  if (firma !== 'GIF87a' && firma !== 'GIF89a') throw new Error('no es un GIF');
  const ancho = b[6] | (b[7] << 8);
  const alto = b[8] | (b[9] << 8);
  if (!ancho || !alto) throw new Error('GIF sin tamaño');
  let p = 13;
  let global = null;
  if (b[10] & 0x80) {
    const n = 3 * (1 << ((b[10] & 7) + 1));
    global = b.subarray(p, p + n);
    p += n;
  }

  const lienzo = new Uint8ClampedArray(ancho * alto * 4); // transparente
  const cuadros = [];
  let control = { reemplazo: 0, transparente: -1, ms: 100 };
  let anterior = null; // para "volver a como estaba" (reemplazo 3)
  let ultimo = null; // el rectángulo y el reemplazo del cuadro anterior

  const subBloques = () => {
    const partes = [];
    let total = 0;
    while (p < b.length) {
      const n = b[p++];
      if (n === 0) break;
      partes.push(b.subarray(p, p + n));
      total += n;
      p += n;
    }
    const out = new Uint8Array(total);
    let o = 0;
    for (const x of partes) out.set(x, o), (o += x.length);
    return out;
  };

  while (p < b.length) {
    const tipo = b[p++];
    if (tipo === 0x3b) break; // fin
    if (tipo === 0x21) {
      const etiqueta = b[p++];
      if (etiqueta === 0xf9 && b[p] === 4) {
        const empaquetado = b[p + 1];
        const demora = b[p + 2] | (b[p + 3] << 8);
        control = {
          reemplazo: (empaquetado >> 2) & 7,
          transparente: empaquetado & 1 ? b[p + 4] : -1,
          // Como los navegadores: menos de 20 ms se toma como 100.
          ms: demora < 2 ? 100 : demora * 10,
        };
        p += 5;
        subBloques();
      } else {
        subBloques();
      }
      continue;
    }
    if (tipo !== 0x2c) throw new Error('GIF roto');

    // Antes de dibujar: lo que pidió el cuadro anterior que se haga con él.
    if (ultimo) {
      if (ultimo.reemplazo === 2) limpiar(lienzo, ancho, ultimo);
      else if (ultimo.reemplazo === 3 && anterior) lienzo.set(anterior);
    }
    const x = b[p] | (b[p + 1] << 8);
    const y = b[p + 2] | (b[p + 3] << 8);
    const w = b[p + 4] | (b[p + 5] << 8);
    const h = b[p + 6] | (b[p + 7] << 8);
    const emp = b[p + 8];
    p += 9;
    let paleta = global;
    if (emp & 0x80) {
      const n = 3 * (1 << ((emp & 7) + 1));
      paleta = b.subarray(p, p + n);
      p += n;
    }
    if (!paleta) throw new Error('GIF sin paleta');
    const minimo = b[p++];
    const indices = lzw(minimo, subBloques(), w * h);
    const filas = emp & 0x40 ? entrelazado(h) : null;

    anterior = control.reemplazo === 3 ? lienzo.slice() : null;
    for (let fy = 0; fy < h; fy++) {
      const yy = y + (filas ? filas[fy] : fy);
      if (yy >= alto) continue;
      for (let fx = 0; fx < w; fx++) {
        const xx = x + fx;
        if (xx >= ancho) continue;
        const k = indices[fy * w + fx];
        if (k === control.transparente) continue;
        const o = (yy * ancho + xx) * 4;
        lienzo[o] = paleta[k * 3];
        lienzo[o + 1] = paleta[k * 3 + 1];
        lienzo[o + 2] = paleta[k * 3 + 2];
        lienzo[o + 3] = 255;
      }
    }
    cuadros.push({ rgba: lienzo.slice(), ms: control.ms });
    ultimo = { x, y, w, h, reemplazo: control.reemplazo };
    control = { reemplazo: 0, transparente: -1, ms: 100 };
  }
  if (!cuadros.length) throw new Error('GIF sin cuadros');
  return { ancho, alto, cuadros };
}

function limpiar(lienzo, ancho, r) {
  const alto = lienzo.length / 4 / ancho;
  for (let y = r.y; y < Math.min(r.y + r.h, alto); y++)
    lienzo.fill(0, (y * ancho + r.x) * 4, (y * ancho + Math.min(r.x + r.w, ancho)) * 4);
}

// En un GIF entrelazado las filas vienen en 4 pasadas: la fila de la imagen de cada una.
function entrelazado(h) {
  const filas = [];
  for (const [desde, paso] of [[0, 8], [4, 8], [2, 4], [1, 2]]) for (let y = desde; y < h; y += paso) filas.push(y);
  return filas;
}

// LZW de GIF: códigos de largo variable (de minimo+1 hasta 12 bits), LSB primero.
function lzw(minimo, datos, n) {
  const salida = new Uint8Array(n);
  const limpio = 1 << minimo;
  const fin = limpio + 1;
  const prefijo = new Uint16Array(4096);
  const sufijo = new Uint8Array(4096);
  const pila = new Uint8Array(4097);
  for (let i = 0; i < limpio; i++) sufijo[i] = i;
  let largo = minimo + 1;
  let siguiente = fin + 1;
  let previo = -1;
  let primero = 0;
  let acumulado = 0;
  let bits = 0;
  let pos = 0;
  let o = 0;
  while (o < n) {
    while (bits < largo) {
      if (pos >= datos.length) return salida;
      acumulado |= datos[pos++] << bits;
      bits += 8;
    }
    const codigo = acumulado & ((1 << largo) - 1);
    acumulado >>>= largo;
    bits -= largo;
    if (codigo === limpio) {
      largo = minimo + 1;
      siguiente = fin + 1;
      previo = -1;
      continue;
    }
    if (codigo === fin) break;
    if (previo === -1) {
      salida[o++] = sufijo[codigo];
      previo = primero = codigo;
      continue;
    }
    let c = codigo;
    let sp = 0;
    if (codigo >= siguiente) {
      pila[sp++] = primero;
      c = previo;
    }
    while (c >= limpio) {
      pila[sp++] = sufijo[c];
      c = prefijo[c];
    }
    pila[sp++] = c;
    primero = c;
    while (sp > 0 && o < n) salida[o++] = pila[--sp];
    if (siguiente < 4096) {
      prefijo[siguiente] = previo;
      sufijo[siguiente] = primero;
      siguiente++;
      if (siguiente === 1 << largo && largo < 12) largo++;
    }
    previo = codigo;
  }
  return salida;
}
