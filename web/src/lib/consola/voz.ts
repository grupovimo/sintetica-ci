// La voz y la IA de la nube: tipos de lo que devuelve la API (ver docs/nube.md,
// "Voz" y "La clave de IA propia") y cómo escuchar una prueba en el navegador.

export type VozEdge = {
  disponible: boolean;
  prendida?: boolean;
  voz?: string;
  voces?: { id: string; nombre: string; idioma: string; genero: string }[];
  usados_hoy?: number;
  tope?: number;
};

export type VozEleven = {
  conectado: boolean;
  voz?: string;
  voz_nombre?: string;
  voces?: { id: string; nombre: string; muestra?: string }[];
  leer?: boolean;
  usados_hoy?: number;
  tope?: number;
  segundos_hoy?: number;
  tope_minutos?: number;
};

export type PaqueteVoz = {
  estado: 'nuevo' | 'generando' | 'listo' | 'parcial' | 'error';
  error?: string;
  version: number;
  voz: string;
  frases: number;
  bytes: number;
  disponible: boolean;
  proveedor?: string;
  voz_nueva?: string;
  total: number;
  faltan: number;
  caracteres: number;
  gasta: number;
  bytes_estimados: number;
  tope_dia?: number;
  quedan_hoy?: number;
  progreso?: { hechas: number; de: number };
};

export type IA = {
  propia: boolean;
  servidor?: boolean;
  proveedor?: 'anthropic' | 'compatible';
  modelo?: string;
  url_pista?: string;
  usadas_hoy?: number;
  tope?: number;
};

// Los servicios conocidos: completan la URL y sugieren un modelo.
export const SERVICIOS = [
  { clave: 'anthropic', nombre: 'Anthropic', proveedor: 'anthropic', url: '', modelo: 'claude-haiku-5-5', claves: 'https://console.anthropic.com/settings/keys' },
  { clave: 'groq', nombre: 'Groq', proveedor: 'compatible', url: 'https://api.groq.com/openai/v1', modelo: 'llama-3.3-70b-versatile', claves: 'https://console.groq.com/keys' },
  { clave: 'openrouter', nombre: 'OpenRouter', proveedor: 'compatible', url: 'https://openrouter.ai/api/v1', modelo: 'meta-llama/llama-3.3-70b-instruct', claves: 'https://openrouter.ai/keys' },
  { clave: 'gemini', nombre: 'Gemini', proveedor: 'compatible', url: 'https://generativelanguage.googleapis.com/v1beta/openai', modelo: 'gemini-2.5-flash', claves: 'https://aistudio.google.com/apikey' },
  { clave: 'otro', nombre: 'Otro', proveedor: 'compatible', url: '', modelo: '', claves: '' },
] as const;

// Una sola prueba sonando a la vez.
let sonando: HTMLAudioElement | null = null;
export async function escucharPrueba(ruta: string, texto?: string): Promise<void> {
  const r = await fetch('/api' + ruta, {
    method: 'POST',
    headers: { 'X-Sintetica': '1', 'Content-Type': 'application/json' },
    body: JSON.stringify(texto ? { texto } : {}),
  });
  if (!r.ok) {
    let m = `Error ${r.status}`;
    try { m = (await r.json()).error ?? m; } catch { /* sin json */ }
    throw new Error(m);
  }
  const url = URL.createObjectURL(await r.blob());
  sonando?.pause();
  sonando = new Audio(url);
  sonando.onended = () => URL.revokeObjectURL(url);
  await sonando.play();
}

// 1500 → "1,5 k"; 200000 → "200 k".
export const miles = (n: number) => (n >= 1000 ? `${(n / 1000).toLocaleString('es-AR', { maximumFractionDigits: 1 })} k` : String(n));
export const megas = (b: number) => `${(b / 1048576).toLocaleString('es-AR', { maximumFractionDigits: 1 })}`;
