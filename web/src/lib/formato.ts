// El formato de la config (shared/formato/config.schema.json) para los editores.

export const EVENTOS = ['tacto', 'doble-toque', 'levantado', 'sacudida', 'boca-abajo', 'vinculado', 'app-abierta', 'despertado'] as const;
export const NOMBRES_EVENTO: Record<string, string> = {
  tacto: 'Lo tocan',
  'doble-toque': 'Doble toque',
  levantado: 'Lo levantan',
  sacudida: 'Lo sacuden',
  'boca-abajo': 'Boca abajo',
  vinculado: 'Se vincula un celu',
  'app-abierta': 'Abren la app',
  despertado: 'Lo despiertan',
};
export const MODOS = ['mascota', 'reloj', 'pomodoro', 'clima', 'notificaciones'] as const;
export const CURVAS = ['suave', 'lineal', 'entrada', 'salida', 'rebote', 'paso'] as const;
export const NOMBRES_CURVA: Record<string, string> = {
  suave: 'Suave', lineal: 'Lineal', entrada: 'Arranca lento', salida: 'Frena al final', rebote: 'Rebote', paso: 'Salto',
};
export const COLORES = ['naranja', 'naranja-profundo', 'tinta', 'tinta-suave', 'linea', 'pantalla'] as const;
export const HEX_COLOR: Record<string, string> = {
  naranja: '#FF6A33', 'naranja-profundo': '#FF8A5C', pantalla: '#0B0A09', tinta: '#F5F2EC', 'tinta-suave': '#A39E96', linea: '#2C2925',
};

export type Punto = [number, number];
export type Forma = {
  forma: 'pildora' | 'corte' | 'circulo' | 'arco' | 'corazon' | 'espiral' | 'poligono' | 'franjas';
  color?: string;
  fijo?: boolean;
  x?: number; y?: number; w?: number; h?: number; r?: number;
  relleno?: boolean; trazo?: number; izq?: number; der?: number; tamano?: number;
  r0?: number; paso?: number; giro?: number;
  puntos?: Punto[];
  bandas?: { y: number; h: number; dx: number }[];
  progreso?: boolean; audio?: number;
};
export type Expresion = { parpadea?: boolean; formas: Forma[] };
export type Clave = { t: number; x?: number; y?: number; escala?: number; rotacion?: number; opacidad?: number; color?: string; curva?: string };
export type Pista = { forma: Forma; centro?: Punto; claves: Clave[] };
export type Animacion = { duracion: number; repetir?: boolean; sobre?: string; pistas: Pista[] };
export type Regla = { si: string; entonces: string[] };
export type Config = {
  version: 1;
  nombre?: string;
  cara?: { color?: string; tamano?: number; ojos?: string };
  personalidad?: { energia?: number; timidez?: number; sueno?: string };
  expresiones?: Record<string, Expresion>;
  animaciones?: Record<string, Animacion>;
  reglas?: Regla[];
  modo?: string;
  lista?: string[];
  rotar?: number;
};

export const TIPOS_FORMA: { tipo: Forma['forma']; nombre: string; nueva: () => Forma }[] = [
  { tipo: 'pildora', nombre: 'Píldora', nueva: () => ({ forma: 'pildora', x: 0, y: 0, w: 0.15, h: 0.4 }) },
  { tipo: 'circulo', nombre: 'Círculo', nueva: () => ({ forma: 'circulo', x: 0, y: 0, r: 0.1 }) },
  { tipo: 'corte', nombre: 'Ojo cortado', nueva: () => ({ forma: 'corte', x: 0, y: 0, w: 0.15, h: 0.35, izq: 0.08, der: 0 }) },
  { tipo: 'arco', nombre: 'Arco', nueva: () => ({ forma: 'arco', puntos: [[-0.1, 0.05], [0, -0.1], [0.1, 0.05]], trazo: 0.06 }) },
  { tipo: 'corazon', nombre: 'Corazón', nueva: () => ({ forma: 'corazon', x: 0, y: 0, tamano: 1 }) },
  { tipo: 'espiral', nombre: 'Espiral', nueva: () => ({ forma: 'espiral', x: 0, y: 0, r0: 0.01, paso: 0.012, giro: 15, trazo: 0.04 }) },
  { tipo: 'poligono', nombre: 'Polígono', nueva: () => ({ forma: 'poligono', puntos: [[-0.1, 0.08], [0, -0.1], [0.1, 0.08]] }) },
  { tipo: 'franjas', nombre: 'Glitch', nueva: () => ({ forma: 'franjas', bandas: [{ y: -0.1, h: 0.05, dx: 0.04 }] }) },
];

export const nombreValido = (n: string) => /^[a-z][a-z0-9-]{0,23}$/.test(n);

// Una config "vacía" pero completa: lo que muestran los editores si el bicho no tiene nada.
export function configBase(): Config {
  return { version: 1 };
}

// Copia profunda (los documentos son JSON; sirve también con los proxies de $state).
export const clonar = <T>(x: T): T => JSON.parse(JSON.stringify(x));

// El centro de una forma (para moverla con el mouse).
export function centroDe(f: Forma): Punto {
  if (f.puntos?.length) {
    const xs = f.puntos.map((p) => p[0]), ys = f.puntos.map((p) => p[1]);
    return [(Math.min(...xs) + Math.max(...xs)) / 2, (Math.min(...ys) + Math.max(...ys)) / 2];
  }
  return [f.x ?? 0, f.y ?? 0];
}

export function mover(f: Forma, dx: number, dy: number): Forma {
  const g = clonar(f);
  const r = (v: number) => Math.round(v * 10000) / 10000;
  if (g.puntos) g.puntos = g.puntos.map(([x, y]) => [r(x + dx), r(y + dy)] as Punto);
  if (g.bandas) g.bandas = g.bandas.map((b) => ({ ...b, y: r(b.y + dy) }));
  if (g.x !== undefined) g.x = r(g.x + dx);
  if (g.y !== undefined) g.y = r(g.y + dy);
  return g;
}

// Lo que se ve en un cuadro de una animación, para dibujar la línea de tiempo.
export function clavesOrdenadas(p: Pista) {
  return [...p.claves].sort((a, b) => a.t - b.t);
}

export const MEDIO_ALTO = 172 / 320; // la pantalla de referencia mide de -1 a 1 de ancho y ±0,5375 de alto

// La caja de una forma en coordenadas normalizadas (como cajaDeForma del núcleo).
export function cajaDe(f: Forma): [number, number, number, number] {
  const x = f.x ?? 0, y = f.y ?? 0;
  switch (f.forma) {
    case 'pildora':
    case 'corte':
      return [x - (f.w ?? 0) / 2, y - (f.h ?? 0) / 2, x + (f.w ?? 0) / 2, y + (f.h ?? 0) / 2];
    case 'circulo':
      return [x - (f.r ?? 0), y - (f.r ?? 0), x + (f.r ?? 0), y + (f.r ?? 0)];
    case 'corazon': {
      const k = (f.tamano ?? 1) / 160;
      return [x - 24 * k, y - 18 * k, x + 24 * k, y + 21.4 * k];
    }
    case 'espiral': {
      const r = (f.r0 ?? 0) + (f.paso ?? 0) * (f.giro ?? 0);
      return [x - r, y - r, x + r, y + r];
    }
    case 'arco':
    case 'poligono': {
      const p = f.puntos ?? [[0, 0]];
      const xs = p.map((q) => q[0]), ys = p.map((q) => q[1]);
      return [Math.min(...xs), Math.min(...ys), Math.max(...xs), Math.max(...ys)];
    }
    case 'franjas': {
      const b = f.bandas ?? [];
      if (!b.length) return [-1, 0, 1, 0];
      return [-1, Math.min(...b.map((x) => x.y)), 1, Math.max(...b.map((x) => x.y + x.h))];
    }
  }
}

// El mismo ojo del otro lado (x → −x).
export function espejar(f: Forma): Forma {
  const g = clonar(f);
  const r = (v: number) => Math.round(v * 10000) / 10000;
  if (g.x !== undefined) g.x = r(-g.x);
  if (g.puntos) g.puntos = g.puntos.map(([x, y]) => [r(-x), y] as Punto).reverse();
  if (g.forma === 'corte') [g.izq, g.der] = [g.der, g.izq];
  return g;
}

// Los campos numéricos que se editan de cada forma: [clave, etiqueta, mínimo, máximo, paso].
export const CAMPOS: Record<Forma['forma'], [keyof Forma, string, number, number, number][]> = {
  pildora: [['x', 'x', -2, 2, 0.005], ['y', 'y', -2, 2, 0.005], ['w', 'ancho', 0, 2, 0.005], ['h', 'alto', 0, 2, 0.005], ['r', 'radio', 0, 2, 0.005], ['trazo', 'trazo', 0, 2, 0.005]],
  corte: [['x', 'x', -2, 2, 0.005], ['y', 'y', -2, 2, 0.005], ['w', 'ancho', 0, 2, 0.005], ['h', 'alto', 0, 2, 0.005], ['izq', 'corte izq.', 0, 2, 0.005], ['der', 'corte der.', 0, 2, 0.005]],
  circulo: [['x', 'x', -2, 2, 0.005], ['y', 'y', -2, 2, 0.005], ['r', 'radio', 0, 2, 0.005]],
  arco: [['trazo', 'trazo', 0, 2, 0.005]],
  corazon: [['x', 'x', -2, 2, 0.005], ['y', 'y', -2, 2, 0.005], ['tamano', 'tamaño', 0, 2, 0.01]],
  espiral: [['x', 'x', -2, 2, 0.005], ['y', 'y', -2, 2, 0.005], ['r0', 'radio inicial', 0, 2, 0.002], ['paso', 'paso', 0, 2, 0.001], ['giro', 'vueltas (rad)', 0, 40, 0.1], ['trazo', 'trazo', 0, 2, 0.005]],
  poligono: [],
  franjas: [],
};

// Las curvas del núcleo (formato.cpp, aplicarCurva).
export function curva(c: string | undefined, t: number): number {
  t = Math.min(1, Math.max(0, t));
  switch (c ?? 'suave') {
    case 'lineal': return t;
    case 'entrada': return t * t * t;
    case 'salida': return 1 - (1 - t) ** 3;
    case 'rebote': { const k = 1.70158, u = t - 1; return 1 + (k + 1) * u * u * u + k * u * u; }
    case 'paso': return t < 1 ? 0 : 1;
    default: return t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2;
  }
}

// Cómo está una pista en el instante t (como cuadroDePista del núcleo).
export function cuadro(p: Pista, t: number) {
  const c = clavesOrdenadas(p);
  if (!c.length) return { x: 0, y: 0, escala: 1, rotacion: 0, opacidad: 1 };
  let a = c[0], b = c[0], k = 0;
  if (t >= c[c.length - 1].t) a = b = c[c.length - 1];
  else if (t > c[0].t) {
    for (let i = 0; i + 1 < c.length; i++) {
      if (t >= c[i].t && t < c[i + 1].t) {
        a = c[i]; b = c[i + 1];
        k = b.t > a.t ? curva(a.curva, (t - a.t) / (b.t - a.t)) : 1;
        break;
      }
    }
  }
  const m = (va = 0, vb = 0) => va + (vb - va) * k;
  return {
    x: m(a.x, b.x), y: m(a.y, b.y), escala: m(a.escala ?? 1, b.escala ?? 1),
    rotacion: m(a.rotacion, b.rotacion), opacidad: Math.min(1, Math.max(0, m(a.opacidad ?? 1, b.opacidad ?? 1))),
  };
}
