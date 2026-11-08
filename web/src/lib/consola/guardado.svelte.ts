// La consola guarda sola: cada perilla o tecla cambia la config al toque (se ve
// en la pantalla en vivo) y medio segundo después se valida con el núcleo y se
// manda a la nube. Si el núcleo dice que no, se vuelve a lo guardado.
import { getContext, setContext } from 'svelte';
import type { BichoAbierto } from '$lib/bicho.svelte';
import { clonar, type Config } from '$lib/formato';

export type EstadoGuardado = 'listo' | 'guardando' | 'guardado' | 'error';

export class Guardado {
  estado = $state<EstadoGuardado>('listo');
  error = $state('');
  private pendiente: Config | null = null;
  private reloj: ReturnType<typeof setTimeout> | null = null;

  constructor(private b: BichoAbierto) {}

  // Cambia la config: la consola siempre escribe la v2 (la v1 se pasa sola).
  cambiar(f: (c: Config) => void) {
    const c = clonar(this.pendiente ?? $state.snapshot(this.b.config)) as Config;
    f(c);
    aV2(c);
    this.pendiente = c;
    this.b.config = c;
    this.estado = 'guardando';
    this.error = '';
    if (this.reloj) clearTimeout(this.reloj);
    this.reloj = setTimeout(() => this.mandar(), 600);
  }

  private async mandar() {
    const c = this.pendiente;
    this.pendiente = null;
    if (!c) return;
    const error = await this.b.guardar(c);
    if (this.pendiente) return; // llegó otro cambio mientras guardaba: lo manda ese
    if (error) {
      this.estado = 'error';
      this.error = error;
      await this.b.cargarConfig().catch(() => {});
    } else {
      this.estado = 'guardado';
    }
  }
}

// La v1 decía cuándo duerme ("sueno": "HH:MM-HH:MM"); la v2, el horario.
export function aV2(c: Config) {
  const p = c.personalidad as (Config['personalidad'] & { sueno?: string }) | undefined;
  if (p?.sueno) {
    c.horario ??= { despierta: p.sueno.slice(6), duerme: p.sueno.slice(0, 5) };
    delete p.sueno;
  }
  c.version = 2;
}

const CLAVE = Symbol('guardado');
export const darGuardado = (g: Guardado) => setContext(CLAVE, g);
export const usarGuardado = () => getContext<Guardado>(CLAVE);
