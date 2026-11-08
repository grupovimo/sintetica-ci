// El robot abierto en la web: su estado en vivo y su config. Lo comparten las
// pestañas de /robots/[id] por contexto.
import { getContext, setContext } from 'svelte';
import { api, ErrorApi } from './api';
import { escuchar } from './eventos.svelte';
import type { Bicho, Actualizacion } from './tipos';
import { EXPRESIONES_BASE, type Config } from './formato';
import { Car, comando, leerRegistro, leerVida } from '$compartido/protocolo.js';
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
  // El motor de vida en vivo (respuesta al comando Vida, que se pide desde la pestaña Vida).
  vida = $state<ReturnType<typeof leerVida> | null>(null);
  vidaEn = $state(0);
  // Lo que el robot acaba de avisar solo (un recordatorio, el fin de un pomodoro), para mostrarlo al toque.
  aviso = $state<{ texto: string; cuando: number; icono: string } | null>(null);
  private dejar: (() => void) | null = null;

  constructor(public id: string) {}

  async cargar() {
    try {
      this.bicho = await api<Bicho>(`/v1/bichos/${this.id}`);
      await this.cargarConfig();
      this.error = '';
    } catch (e) {
      this.error = e instanceof ErrorApi ? e.message : 'No pude cargar el robot.';
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
        case 'mcp': this.avisar(`${e.datos.cliente || 'Tu IA'}: ${e.datos.herramienta.replaceAll('_', ' ')}`, '✦'); break;
        case 'marco': {
          const b = Uint8Array.from(atob(e.datos.datos), (c) => c.charCodeAt(0));
          if (e.datos.car === 2 && b[0] === 0x9b && b[1] === 0) {
            try { this.vida = leerVida(b); this.vidaEn = Date.now(); } catch { /* una respuesta rara */ }
            break;
          }
          // El robot avisa solo con un registro cortito: un recordatorio o el fin de un pomodoro.
          if (e.datos.car === 2 && b[0] === 0x9e && b[1] === 0 && b[2] <= 2) {
            try {
              for (const x of leerRegistro(b)) {
                if (x.tipo === 'recordatorio') this.avisar(this.config.recordatorios?.[x.dato]?.texto ?? 'Recordatorio');
                else if (x.tipo === 'evento' && x.dato === 32) this.avisar(this.bicho?.estado?.expresion === 'pomodoro:trabajo' ? 'A trabajar' : 'Tanda terminada');
              }
            } catch { /* un registro raro */ }
          }
          // Las respuestas de Memoria y Registro no son para mostrar.
          if (e.datos.car === 2 && (b[0] === 0x9c || b[0] === 0x9e || b[0] === 0x9a)) break;
          if (e.datos.car === 2 && b.length >= 2) {
            const ok = b[1] === 0;
            const detalle = b.length > 2 ? new TextDecoder().decode(b.subarray(2)) : '';
            this.respuestas = [{ texto: ok ? 'el robot dijo que sí' : `el robot dijo que no${detalle ? ': ' + detalle : ''}`, ok, cuando: Date.now() }, ...this.respuestas].slice(0, 5);
          }
          break;
        }
      }
    });
  }

  private avisar(texto: string, icono = '⏰') {
    const cuando = Date.now();
    this.aviso = { texto, cuando, icono };
    setTimeout(() => { if (this.aviso?.cuando === cuando) this.aviso = null; }, icono === '⏰' ? 120000 : 5000);
  }

  cerrar() { this.dejar?.(); }

  // Valida con el núcleo (el mismo del robot) y guarda. Devuelve el error o null.
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

  // Un marco crudo del protocolo (como por BLE): la respuesta llega por los eventos.
  async marco(car: number, datos: Uint8Array): Promise<string | null> {
    try {
      let s = '';
      for (const x of datos) s += String.fromCharCode(x);
      await api(`/v1/bichos/${this.id}/marcos`, { cuerpo: { car, datos: btoa(s) } });
      return null;
    } catch (e) {
      return e instanceof ErrorApi ? e.message : 'No pude mandarlo.';
    }
  }

  // Pide el estado del motor de vida (y prende o apaga la superposición de depuración).
  pedirVida(superposicion?: boolean) {
    return this.marco(Car.comando, comando.vida(superposicion));
  }

  async comando(c: { expresion?: string; nombre?: string; evento?: string; reproducir?: string }): Promise<string | null> {
    try {
      await api(`/v1/bichos/${this.id}/comando`, { cuerpo: c });
      return null;
    } catch (e) {
      return e instanceof ErrorApi ? e.message : 'No pude mandarlo.';
    }
  }

  // Los nombres de las caras: las del motor de vida (paramétricas), las 16 del manual y las propias.
  get nombresCaras(): string[] {
    return [...new Set([...Object.keys(EXPRESIONES_BASE), ...BASE, ...Object.keys(this.config.expresiones ?? {})])];
  }
  get nombresAnimaciones(): string[] {
    return Object.keys(this.config.animaciones ?? {});
  }
}

export const BASE = ['neutral', 'contento', 'sospecha', 'sorpresa', 'dormido', 'enojado', 'triste', 'guino', 'enamorado', 'mareado', 'escuchando', 'pensando', 'curioso', 'bateria-baja', 'actualizando', 'glitch'];

const CLAVE = Symbol('bicho');
export const darBicho = (b: BichoAbierto) => setContext(CLAVE, b);
export const usarBicho = () => getContext<BichoAbierto>(CLAVE);
