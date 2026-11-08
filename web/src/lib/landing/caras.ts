// Las fotos de las caras para la tira de la landing, sacadas del núcleo de
// verdad: un bicho con el reloj en nuestras manos que pasa por cada expresión y
// se queda quieto hasta que los resortes se asientan.
import { crearSimulador } from '$compartido/sim.js';
import { Car, comando } from '$compartido/protocolo.js';
import { pintar } from '$lib/nucleo';
import expresiones from '$formato/vida/expresiones.json';

// Las que no son una cara para mostrar (estados del sistema).
const SIN_FOTO = new Set(['sin-conexion', 'ko', 'pesadilla']);

export const NOMBRES: string[] = Object.keys(expresiones).filter((n) => !SIN_FOTO.has(n));

export async function fotos(nombres: string[] = NOMBRES): Promise<{ nombre: string; url: string }[]> {
  const r = await fetch('/sinteticaos.wasm');
  if (!r.ok) throw new Error('No encontré el núcleo');
  let reloj = 0;
  const lienzo = document.createElement('canvas');
  let ultimo: { rgba: Uint8ClampedArray; w: number; h: number } | null = null;
  const sim = await crearSimulador({
    wasm: await r.arrayBuffer(),
    ahora: () => reloj,
    semilla: () => 7,
    alPresentar: (rgba: Uint8ClampedArray, w: number, h: number) => (ultimo = { rgba, w, h }),
  });
  sim.vistaPrevia();
  const avanzar = (ms: number) => {
    for (let t = 0; t < ms; t += 33) {
      reloj += 33;
      sim.tick();
    }
  };
  avanzar(500);
  const salida: { nombre: string; url: string }[] = [];
  for (const nombre of nombres) {
    sim.bleEscribir(Car.comando, comando.expresion(nombre));
    avanzar(1400);
    const u = ultimo as { rgba: Uint8ClampedArray; w: number; h: number } | null;
    if (!u) continue;
    pintar(lienzo, u.rgba, u.w, u.h);
    salida.push({ nombre, url: lienzo.toDataURL('image/png') });
  }
  return salida;
}
