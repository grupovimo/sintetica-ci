// Prepara una foto para el bicho: recortada al tamaño exacto de su pantalla
// (llenando, centrada), en el broche redonda (afuera del círculo, el color de la
// pantalla) y en QOI de 3 canales. Como en las apps (PrepararImagen).
import { codificarQoi, decodificarQoi } from '$compartido/qoi.js';

export async function prepararImagen(archivo: Blob, ancho: number, alto: number, redonda: boolean): Promise<{ qoi: Uint8Array; vista: string }> {
  const img = await createImageBitmap(archivo);
  const c = document.createElement('canvas');
  c.width = ancho;
  c.height = alto;
  const ctx = c.getContext('2d')!;
  ctx.fillStyle = '#0B0A09';
  ctx.fillRect(0, 0, ancho, alto);
  const escala = Math.max(ancho / img.width, alto / img.height);
  const w = img.width * escala, h = img.height * escala;
  ctx.imageSmoothingQuality = 'high';
  if (redonda) {
    ctx.save();
    ctx.beginPath();
    ctx.arc(ancho / 2, alto / 2, Math.min(ancho, alto) / 2, 0, Math.PI * 2);
    ctx.clip();
  }
  ctx.drawImage(img, (ancho - w) / 2, (alto - h) / 2, w, h);
  if (redonda) ctx.restore();
  const datos = ctx.getImageData(0, 0, ancho, alto);
  return { qoi: codificarQoi(datos.data, ancho, alto), vista: c.toDataURL() };
}

// Un QOI a una URL para un <img>.
export function qoiAUrl(qoi: Uint8Array): string {
  const { ancho, alto, rgba } = decodificarQoi(qoi);
  const c = document.createElement('canvas');
  c.width = ancho;
  c.height = alto;
  c.getContext('2d')!.putImageData(new ImageData(new Uint8ClampedArray(rgba), ancho, alto), 0, 0);
  return c.toDataURL();
}
