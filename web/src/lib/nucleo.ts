// El núcleo de SintéticaOS en WebAssembly: el mismo C++ que corre en el robot.
// Las vistas previas y los editores dibujan con él, así se ve igual.
import { crearSimulador } from '$compartido/sim.js';
import { Car, escriturasDeConfig, escriturasDeImagen, comando, imagen, PRIMER_SPRITE } from '$compartido/protocolo.js';

let bytesWasm: Promise<ArrayBuffer> | null = null;

function cargar(): Promise<ArrayBuffer> {
  bytesWasm ??= fetch('/sinteticaos.wasm').then((r) => {
    if (!r.ok) throw new Error('No encontré el núcleo (sinteticaos.wasm)');
    return r.arrayBuffer();
  });
  return bytesWasm;
}

export type Simulador = Awaited<ReturnType<typeof crearSimulador>>;

export type VistaPrevia = Simulador & {
  // Le manda una config entera (como la app por BLE). null = la aceptó; si no, por qué.
  aplicar: (config: unknown) => string | null;
};

// Un robot entero en vista previa: vive (parpadea, mira), con la config que
// se le pase. `alCuadro` recibe cada cuadro.
export async function vistaPrevia(op: {
  redonda?: boolean;
  alCuadro: (rgba: Uint8ClampedArray, ancho: number, alto: number) => void;
}): Promise<VistaPrevia> {
  let ultima: Uint8Array | null = null;
  const sim = await crearSimulador({
    wasm: await cargar(),
    perfil: op.redonda ? 'broche' : 'mascota',
    alPresentar: op.alCuadro,
    alNotificar: (car: number, b: Uint8Array) => {
      if (car === Car.config) ultima = b;
    },
  });
  sim.vistaPrevia();
  return Object.assign(sim, {
    aplicar(config: unknown) {
      ultima = null;
      for (const e of escriturasDeConfig(typeof config === 'string' ? config : JSON.stringify(config), 4096)) sim.bleEscribir(Car.config, e);
      const r = ultima as Uint8Array | null;
      if (!r || r[0] !== 0x83) return 'el núcleo no respondió';
      return r[1] === 0 ? null : new TextDecoder().decode(r.subarray(2)) || 'config inválida';
    },
  });
}

// Deja fija una expresión ("animacion:<nombre>" reproduce esa animación).
export function fijar(sim: Simulador, visible: string) {
  if (visible.startsWith('animacion:')) sim.bleEscribir(Car.comando, comando.reproducir(visible.slice(10)));
  else sim.bleEscribir(Car.comando, comando.expresion(visible));
}

// Un núcleo para validar y para los editores (dibujar sin el sistema).
let editor: Promise<Simulador> | null = null;
export function nucleoEditor(): Promise<Simulador> {
  editor ??= cargar().then((wasm) => crearSimulador({ wasm }));
  return editor;
}

// Pinta un cuadro RGBA en un canvas.
export function pintar(canvas: HTMLCanvasElement, rgba: Uint8ClampedArray, ancho: number, alto: number) {
  if (canvas.width !== ancho) canvas.width = ancho;
  if (canvas.height !== alto) canvas.height = alto;
  const ctx = canvas.getContext('2d');
  if (!ctx) return;
  ctx.putImageData(new ImageData(new Uint8ClampedArray(rgba), ancho, alto), 0, 0);
}

// Carga los sprites de las caras de imagen en una vista previa (como la app por
// BLE, espacios 8 a 19) y borra los que ya no están. `enviados` recuerda qué
// CRC tiene cada uno para no mandarlo de nuevo.
export function ponerSprites(sim: Simulador, sprites: Record<number, { qoi: Uint8Array; crc: number }>, enviados: Map<number, number>) {
  for (const [k, s] of Object.entries(sprites)) {
    const n = Number(k);
    if (enviados.get(n) === s.crc) continue;
    for (const e of escriturasDeImagen(PRIMER_SPRITE + n, s.qoi, 4096)) sim.bleEscribir(Car.imagen, e);
    enviados.set(n, s.crc);
  }
  for (const n of [...enviados.keys()]) {
    if (sprites[n]) continue;
    sim.bleEscribir(Car.imagen, imagen.borrar(PRIMER_SPRITE + n));
    enviados.delete(n);
  }
}
