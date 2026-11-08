// Prepara una foto (o un GIF animado, o un video corto) para el robot: recortada al tamaño
// exacto de su pantalla (llenando, centrada), en el broche redonda (afuera del
// círculo, el color de la pantalla) y en QOI de 3 canales; un GIF, como
// animación "sani" (cada cuadro en QOI). Como en las apps (PrepararImagen).
import { codificarAnimacion, codificarQoi, cuadrosDe, decodificarQoi } from '$compartido/qoi.js';
import { decodificarGif } from '$compartido/gif.js';

// Lo más que entra en un espacio (el peor caso de un QOI de la pantalla).
const limite = (ancho: number, alto: number) => ancho * alto * 4 + 22;
const MAX_CUADROS = 64;

function dibujar(fuente: CanvasImageSource, fw: number, fh: number, ancho: number, alto: number, redonda: boolean) {
  const c = document.createElement('canvas');
  c.width = ancho;
  c.height = alto;
  const ctx = c.getContext('2d')!;
  ctx.fillStyle = '#0B0A09';
  ctx.fillRect(0, 0, ancho, alto);
  const escala = Math.max(ancho / fw, alto / fh);
  const w = fw * escala, h = fh * escala;
  ctx.imageSmoothingQuality = 'high';
  if (redonda) {
    ctx.save();
    ctx.beginPath();
    ctx.arc(ancho / 2, alto / 2, Math.min(ancho, alto) / 2, 0, Math.PI * 2);
    ctx.clip();
  }
  ctx.drawImage(fuente, (ancho - w) / 2, (alto - h) / 2, w, h);
  if (redonda) ctx.restore();
  return c;
}

export async function prepararImagen(
  archivo: Blob,
  ancho: number,
  alto: number,
  redonda: boolean,
): Promise<{ qoi: Uint8Array; vista: string; cuadros: number }> {
  if (archivo.type.startsWith('video/')) return empaquetar(await cuadrosDeVideo(archivo, ancho, alto, redonda), ancho, alto);
  const bytes = new Uint8Array(await archivo.arrayBuffer());
  const esGif = bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46;
  if (!esGif) {
    const img = await createImageBitmap(archivo);
    const c = dibujar(img, img.width, img.height, ancho, alto, redonda);
    const datos = c.getContext('2d')!.getImageData(0, 0, ancho, alto);
    return { qoi: codificarQoi(datos.data, ancho, alto), vista: c.toDataURL(), cuadros: 1 };
  }
  const gif = decodificarGif(bytes);
  let cuadros: { rgba: Uint8ClampedArray; ms: number }[] = [];
  let vista = '';
  for (const q of gif.cuadros) {
    const bm = await createImageBitmap(new ImageData(new Uint8ClampedArray(q.rgba), gif.ancho, gif.alto));
    const c = dibujar(bm, gif.ancho, gif.alto, ancho, alto, redonda);
    if (!vista) vista = c.toDataURL();
    cuadros.push({ rgba: c.getContext('2d')!.getImageData(0, 0, ancho, alto).data, ms: q.ms });
  }
  return empaquetar({ cuadros, vista }, ancho, alto);
}

type Cuadros = { cuadros: { rgba: Uint8ClampedArray; ms: number }[]; vista: string };

function empaquetar({ cuadros, vista }: Cuadros, ancho: number, alto: number) {
  if (cuadros.length === 1) return { qoi: codificarQoi(cuadros[0].rgba, ancho, alto), vista, cuadros: 1 };
  // Si no entra (o son demasiados cuadros), se saltea uno de cada dos y el que
  // queda dura lo de los dos: la animación sigue durando lo mismo.
  let sani = codificarAnimacion(cuadros.slice(0, MAX_CUADROS), ancho, alto);
  while (cuadros.length > MAX_CUADROS || sani.length > limite(ancho, alto)) {
    if (cuadros.length <= 2) throw new Error('El GIF es demasiado pesado para la pantalla: probá con uno más simple o más corto.');
    const menos: typeof cuadros = [];
    for (let i = 0; i < cuadros.length; i += 2) menos.push({ rgba: cuadros[i].rgba, ms: cuadros[i].ms + (cuadros[i + 1]?.ms ?? 0) });
    cuadros = menos;
    sani = codificarAnimacion(cuadros.slice(0, MAX_CUADROS), ancho, alto);
  }
  return { qoi: sani, vista, cuadros: cuadros.length };
}

// Un video corto: se toman hasta 6 segundos desde el principio, a 12 cuadros
// por segundo como mucho (después empaquetar saltea los que no entren).
const SEGUNDOS_VIDEO = 6;
async function cuadrosDeVideo(archivo: Blob, ancho: number, alto: number, redonda: boolean): Promise<Cuadros> {
  const url = URL.createObjectURL(archivo);
  const v = document.createElement('video');
  v.muted = true;
  v.playsInline = true;
  v.preload = 'auto';
  v.src = url;
  try {
    await new Promise<void>((listo, mal) => {
      v.onloadeddata = () => listo();
      v.onerror = () => mal(new Error('El GIF o video no se pudo leer: probá con otro formato (mp4 o webm).'));
    });
    const dura = Math.min(isFinite(v.duration) ? v.duration : SEGUNDOS_VIDEO, SEGUNDOS_VIDEO);
    const paso = Math.max(1 / 12, dura / MAX_CUADROS);
    const ms = Math.round(paso * 1000);
    const cuadros: Cuadros['cuadros'] = [];
    let vista = '';
    for (let t = 0; t < dura - 0.001 && cuadros.length < MAX_CUADROS; t += paso) {
      await new Promise<void>((listo) => { v.onseeked = () => listo(); v.currentTime = t; });
      const c = dibujar(v, v.videoWidth, v.videoHeight, ancho, alto, redonda);
      if (!vista) vista = c.toDataURL();
      cuadros.push({ rgba: c.getContext('2d')!.getImageData(0, 0, ancho, alto).data, ms });
    }
    if (!cuadros.length) throw new Error('El GIF o video está vacío.');
    return { cuadros, vista };
  } finally {
    URL.revokeObjectURL(url);
  }
}

// Un QOI (o el primer cuadro de una animación) a una URL para un <img>.
export function qoiAUrl(qoi: Uint8Array): string {
  const { ancho, alto, rgba } = decodificarQoi(cuadrosDe(qoi)[0].qoi);
  const c = document.createElement('canvas');
  c.width = ancho;
  c.height = alto;
  c.getContext('2d')!.putImageData(new ImageData(new Uint8ClampedArray(rgba), ancho, alto), 0, 0);
  return c.toDataURL();
}

// Todos los cuadros (una imagen suelta es un cuadro) como URLs, con su duración.
export function cuadrosAUrls(b: Uint8Array): { url: string; ms: number }[] {
  return cuadrosDe(b).map((c) => ({ url: qoiAUrl(c.qoi), ms: c.ms }));
}

// Acción para un <img>: pasa por los cuadros con su duración.
export function animar(img: HTMLImageElement, cuadros: { url: string; ms: number }[]) {
  let i = 0;
  let reloj: ReturnType<typeof setTimeout> | undefined;
  const paso = () => {
    img.src = cuadros[i].url;
    if (cuadros.length > 1) reloj = setTimeout(() => ((i = (i + 1) % cuadros.length), paso()), cuadros[i].ms);
  };
  paso();
  return {
    update(nuevos: { url: string; ms: number }[]) {
      clearTimeout(reloj);
      cuadros = nuevos;
      i = 0;
      paso();
    },
    destroy: () => clearTimeout(reloj),
  };
}
