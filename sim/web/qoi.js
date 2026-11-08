// QOI ("Quite OK Image", https://qoiformat.org): el formato de las imágenes que
// guarda el bicho. Es simple, sin pérdida y se decodifica de a una fila, así
// que la placa no necesita tener la imagen entera en RAM.
// Este codificador es la referencia para las apps (Kotlin y Swift hacen lo mismo).

const hash = (r, g, b, a) => (r * 3 + g * 5 + b * 7 + a * 11) % 64;

// rgba: Uint8Array/Uint8ClampedArray de ancho × alto × 4. Se codifica con 3 canales.
export function codificarQoi(rgba, ancho, alto) {
  const salida = [];
  const u32 = (v) => salida.push((v >>> 24) & 255, (v >>> 16) & 255, (v >>> 8) & 255, v & 255);
  salida.push(0x71, 0x6f, 0x69, 0x66);  // "qoif"
  u32(ancho);
  u32(alto);
  salida.push(3, 0);
  const indice = new Uint8Array(64 * 4);
  let pr = 0, pg = 0, pb = 0, pa = 255, racha = 0;
  const total = ancho * alto;
  for (let i = 0; i < total; i++) {
    const r = rgba[i * 4], g = rgba[i * 4 + 1], b = rgba[i * 4 + 2], a = 255;
    if (r === pr && g === pg && b === pb && a === pa) {
      racha++;
      if (racha === 62 || i === total - 1) { salida.push(0xc0 | (racha - 1)); racha = 0; }
      continue;
    }
    if (racha > 0) { salida.push(0xc0 | (racha - 1)); racha = 0; }
    const h = hash(r, g, b, a);
    if (indice[h * 4] === r && indice[h * 4 + 1] === g && indice[h * 4 + 2] === b && indice[h * 4 + 3] === a) {
      salida.push(h);
    } else {
      indice[h * 4] = r; indice[h * 4 + 1] = g; indice[h * 4 + 2] = b; indice[h * 4 + 3] = a;
      const vr = ((r - pr) << 24) >> 24, vg = ((g - pg) << 24) >> 24, vb = ((b - pb) << 24) >> 24;
      const vgr = vr - vg, vgb = vb - vg;
      if (vr > -3 && vr < 2 && vg > -3 && vg < 2 && vb > -3 && vb < 2) {
        salida.push(0x40 | ((vr + 2) << 4) | ((vg + 2) << 2) | (vb + 2));
      } else if (vgr > -9 && vgr < 8 && vg > -33 && vg < 32 && vgb > -9 && vgb < 8) {
        salida.push(0x80 | (vg + 32), ((vgr + 8) << 4) | (vgb + 8));
      } else {
        salida.push(0xfe, r, g, b);
      }
    }
    pr = r; pg = g; pb = b; pa = a;
  }
  salida.push(0, 0, 0, 0, 0, 0, 0, 1);
  return Uint8Array.from(salida);
}

// Para los tests: decodifica a RGBA.
export function decodificarQoi(bytes) {
  const u32 = (i) => ((bytes[i] << 24) | (bytes[i + 1] << 16) | (bytes[i + 2] << 8) | bytes[i + 3]) >>> 0;
  if (bytes[0] !== 0x71 || bytes[1] !== 0x6f || bytes[2] !== 0x69 || bytes[3] !== 0x66) throw new Error('no es QOI');
  const ancho = u32(4), alto = u32(8);
  const rgba = new Uint8Array(ancho * alto * 4);
  const indice = new Uint8Array(64 * 4);
  let r = 0, g = 0, b = 0, a = 255, p = 14, racha = 0;
  for (let i = 0; i < ancho * alto; i++) {
    if (racha > 0) racha--;
    else {
      const c = bytes[p++];
      if (c === 0xfe) { r = bytes[p++]; g = bytes[p++]; b = bytes[p++]; }
      else if (c === 0xff) { r = bytes[p++]; g = bytes[p++]; b = bytes[p++]; a = bytes[p++]; }
      else if ((c & 0xc0) === 0x00) { r = indice[c * 4]; g = indice[c * 4 + 1]; b = indice[c * 4 + 2]; a = indice[c * 4 + 3]; }
      else if ((c & 0xc0) === 0x40) { r = (r + ((c >> 4) & 3) - 2) & 255; g = (g + ((c >> 2) & 3) - 2) & 255; b = (b + (c & 3) - 2) & 255; }
      else if ((c & 0xc0) === 0x80) {
        const c2 = bytes[p++], vg = (c & 0x3f) - 32;
        r = (r + vg - 8 + ((c2 >> 4) & 15)) & 255; g = (g + vg) & 255; b = (b + vg - 8 + (c2 & 15)) & 255;
      } else racha = c & 0x3f;
      const h = hash(r, g, b, a);
      indice[h * 4] = r; indice[h * 4 + 1] = g; indice[h * 4 + 2] = b; indice[h * 4 + 3] = a;
    }
    rgba[i * 4] = r; rgba[i * 4 + 1] = g; rgba[i * 4 + 2] = b; rgba[i * 4 + 3] = a;
  }
  return { ancho, alto, rgba };
}

// Una imagen animada ("sani", ver firmware/nucleo/qoi.h): la cabecera, la tabla
// de cuadros y cada cuadro en QOI. cuadros: [{ rgba, ms }].
export function codificarAnimacion(cuadros, ancho, alto) {
  const qois = cuadros.map((c) => codificarQoi(c.rgba, ancho, alto));
  const tabla = 10 + 10 * qois.length;
  const total = tabla + qois.reduce((s, q) => s + q.length, 0);
  const salida = new Uint8Array(total);
  const v = new DataView(salida.buffer);
  salida.set([0x73, 0x61, 0x6e, 0x69, 1, qois.length]); // "sani", versión 1
  v.setUint16(6, ancho, true);
  v.setUint16(8, alto, true);
  let off = tabla;
  qois.forEach((q, i) => {
    v.setUint16(10 + 10 * i, Math.max(20, Math.min(65535, Math.round(cuadros[i].ms))), true);
    v.setUint32(12 + 10 * i, off, true);
    v.setUint32(16 + 10 * i, q.length, true);
    salida.set(q, off);
    off += q.length;
  });
  return salida;
}

export const esAnimacion = (b) => b.length >= 10 && b[0] === 0x73 && b[1] === 0x61 && b[2] === 0x6e && b[3] === 0x69;

// Los cuadros de una animación (o la imagen sola, con ms 0): [{ qoi, ms }].
export function cuadrosDe(b) {
  if (!esAnimacion(b)) return [{ qoi: b, ms: 0 }];
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength);
  const salida = [];
  for (let i = 0; i < b[5]; i++) {
    const off = v.getUint32(12 + 10 * i, true);
    salida.push({ qoi: b.subarray(off, off + v.getUint32(16 + 10 * i, true)), ms: v.getUint16(10 + 10 * i, true) });
  }
  return salida;
}
