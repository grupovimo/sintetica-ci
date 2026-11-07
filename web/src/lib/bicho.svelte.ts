// El bicho abierto en la web: su estado en vivo y su config. Lo comparten las
// pestañas de /bichos/[id] por contexto.
import { getContext, setContext } from 'svelte';
import { api, ErrorApi } from './api';
import { escuchar } from './eventos.svelte';
import type { Bicho, Actualizacion } from './tipos';
import type { Config } from './formato';
import { nucleoEditor } from './nucleo';
import { foto } from './foto.svelte';

export class BichoAbierto {
  bicho = $state<Bicho | null>(null);
  config = $state<Config>({ version: 1 });
  versionConfig = $state(0);
  autorConfig = $state('');
  error = $state('');
  ota = $state<Actualizacion | null>(null);
  respuestas = $state<{ texto: string; ok: boolean; cuando: number }[]>([]);
  private dejar: (() => void) | null = null;

  constructor(public id: string) {}

  async cargar() {
    try {
      this.bicho = await api<Bicho>(`/v1/bichos/${this.id}`);
      await this.cargarConfig();
      this.error = '';
    } catch (e) {
      this.error = e instanceof ErrorApi ? e.message : 'No pude cargar el bicho.';
    }
  }

  async cargarConfig() {
    const c = await api<{ version: number; autor?: string; config: Config }>(`/v1/bichos/${this.id}/config`);
    this.config = c.config;
    this.versionConfig = c.version;
    this.autorConfig = c.autor ?? '';
  }

  escucharEventos() {
    this.dejar = escuchar((e) => {
      if (e.bicho !== this.id || !this.bicho) return;
      switch (e.tipo) {
        case 'estado': this.bicho.estado = e.datos; break;
        case 'conexion': this.bicho.conectado = e.datos.conectado; break;
        case 'sincro': this.bicho.sincro = e.datos; break;
        case 'config': if (e.datos.version !== this.versionConfig) this.cargarConfig(); break;
        case 'ota': this.ota = e.datos; break;
        case 'marco': {
          const b = Uint8Array.from(atob(e.datos.datos), (c) => c.charCodeAt(0));
          if (e.datos.car === 2 && b.length >= 2) {
            const ok = b[1] === 0;
            const detalle = b.length > 2 ? new TextDecoder().decode(b.subarray(2)) : '';
            this.respuestas = [{ texto: ok ? 'el bicho dijo que sí' : `el bicho dijo que no${detalle ? ': ' + detalle : ''}`, ok, cuando: Date.now() }, ...this.respuestas].slice(0, 5);
          }
          break;
        }
      }
    });
  }

  cerrar() { this.dejar?.(); }

  // Valida con el núcleo (el mismo del bicho) y guarda. Devuelve el error o null.
  async guardar(nueva: Config): Promise<string | null> {
    const n = await nucleoEditor();
    const v = n.validar(nueva);
    if (!v.ok) return v.detalle;
    try {
      const r = await api<{ version: number }>(`/v1/bichos/${this.id}/config`, { metodo: 'PUT', cuerpo: nueva });
      this.config = foto(nueva);
      this.versionConfig = r.version;
      this.autorConfig = 'web';
      return null;
    } catch (e) {
      return e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude guardar.';
    }
  }

  async comando(c: { expresion?: string; nombre?: string; evento?: string; reproducir?: string }): Promise<string | null> {
    try {
      await api(`/v1/bichos/${this.id}/comando`, { cuerpo: c });
      return null;
    } catch (e) {
      return e instanceof ErrorApi ? e.message : 'No pude mandarlo.';
    }
  }

  // Los nombres de las caras: las 16 base más las propias.
  get nombresCaras(): string[] {
    return [...new Set([...BASE, ...Object.keys(this.config.expresiones ?? {})])];
  }
  get nombresAnimaciones(): string[] {
    return Object.keys(this.config.animaciones ?? {});
  }
}

export const BASE = ['neutral', 'contento', 'sospecha', 'sorpresa', 'dormido', 'enojado', 'triste', 'guino', 'enamorado', 'mareado', 'escuchando', 'pensando', 'curioso', 'bateria-baja', 'actualizando', 'glitch'];

const CLAVE = Symbol('bicho');
export const darBicho = (b: BichoAbierto) => setContext(CLAVE, b);
export const usarBicho = () => getContext<BichoAbierto>(CLAVE);
