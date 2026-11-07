// Tipos mínimos de los módulos JS compartidos (sim/web).
declare module '$compartido/sim.js' {
  export function crearSimulador(op: any): Promise<any>;
  export function memoriaVolatil(): any;
}
declare module '$compartido/protocolo.js' {
  export const Car: { config: number; comando: number; estado: number; ota: number; imagen: number };
  export const Op: Record<string, number>;
  export const Eventos: string[];
  export const Modos: string[];
  export const comando: Record<string, (...a: any[]) => Uint8Array>;
  export function escriturasDeConfig(config: unknown, tamanoBloque?: number): Uint8Array[];
  export function leerEstado(b: Uint8Array): any;
  export function leerRespuesta(b: Uint8Array): any;
  export function leerQr(t: string): { id: string; codigo: string } | null;
  export function crc32(b: Uint8Array): number;
  export const VariablesEstado: string[];
  export const Rasgos: string[];
  export function leerVida(b: Uint8Array): {
    estado: Record<string, number>; rasgos: Record<string, number>; dormido: boolean; sonando: boolean; superposicion: boolean;
    comportamiento: string; visible: string; candidatos: { nombre: string; puntaje: number }[]; fps: number;
  };
  export function leerMemoria(b: Uint8Array): any;
}
declare module '$compartido/qoi.js' {
  export function codificarQoi(rgba: Uint8ClampedArray | Uint8Array, ancho: number, alto: number): Uint8Array;
  export function decodificarQoi(b: Uint8Array): { ancho: number; alto: number; rgba: Uint8Array };
}
