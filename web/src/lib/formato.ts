// El formato de la config (shared/formato/config.schema.json) para los editores.

import expresionesBase from '$formato/vida/expresiones.json';
import comportamientosBase from '$formato/vida/comportamientos.json';
import reglasBase from '$formato/vida/reglas.json';
import personalidades from '$formato/vida/personalidades.json';

// En el orden del protocolo (el índice viaja en el comando Evento).
export const EVENTOS = [
  'tacto', 'doble-toque', 'levantado', 'sacudida', 'boca-abajo', 'vinculado', 'app-abierta', 'despertado',
  'caricia', 'muchos-toques', 'sacudida-suave', 'sacudida-fuerte', 'inclinado', 'caida', 'voz', 'nombre',
  'ruido', 'musica', 'silencio', 'notificacion', 'llamada', 'bateria-baja', 'bateria-critica', 'enchufado',
  'desenchufado', 'cargado', 'sin-wifi', 'cambio-de-cara', 'bicho-cerca', 'volvio', 'hito', 'recordatorio', 'pomodoro', 'despertador',
  'juego', 'animo', 'cara-detectada', 'oscuridad', 'mano-cerca',
] as const;
export const NOMBRES_EVENTO: Record<string, string> = {
  tacto: 'Lo tocan', 'doble-toque': 'Doble toque', levantado: 'Lo levantan', sacudida: 'Lo sacuden (cualquiera)',
  'boca-abajo': 'Boca abajo', vinculado: 'Se vincula un celu', 'app-abierta': 'Abren la app', despertado: 'Lo despiertan',
  caricia: 'Una caricia', 'muchos-toques': 'Muchos toques seguidos', 'sacudida-suave': 'Sacudida suave',
  'sacudida-fuerte': 'Sacudida fuerte', inclinado: 'Lo inclinan', caida: 'Se cae', voz: 'Le hablan', nombre: 'Dicen su nombre',
  ruido: 'Un ruido fuerte', musica: 'Música', silencio: 'Silencio largo', notificacion: 'Una notificación',
  llamada: 'Una llamada', 'bateria-baja': 'Batería baja (20 %)', 'bateria-critica': 'Batería crítica (5 %)',
  enchufado: 'Lo enchufan', desenchufado: 'Lo desenchufan', cargado: 'Carga completa', 'sin-wifi': 'Se queda sin WiFi',
  'cambio-de-cara': 'Le cambian la cara', 'bicho-cerca': 'Otro robot cerca', volvio: 'Volvés después de un rato',
  hito: 'Un hito (días juntos, caricias, cumpleaños)', recordatorio: 'Muestra un recordatorio', pomodoro: 'Termina una tanda', despertador: 'Suena el despertador',
  juego: 'Termina un juego', animo: 'Le contás cómo estás', 'cara-detectada': 'Te ve', oscuridad: 'Se apaga la luz',
  'mano-cerca': 'Una mano cerca',
};

// El robot cuenta los textos en bytes UTF-8 (una tilde ocupa 2): se corta sin partir una letra.
export function cortarBytes(t: string, max: number): string {
  const c = new TextEncoder();
  let r = '';
  for (const letra of t) {
    if (c.encode(r + letra).length > max) break;
    r += letra;
  }
  return r;
}

// --- Motor de vida (config v2) ---
export const RASGOS = ['energia', 'sociabilidad', 'picardia', 'timidez', 'curiosidad', 'malGenio'] as const;
export const NOMBRES_RASGO: Record<string, [string, string, string]> = {
  energia: ['Energía', 'Se mueve mucho, duerme poco', 'Tranquilo, siestero'],
  sociabilidad: ['Sociabilidad', 'Busca atención, se aburre solo', 'Independiente'],
  picardia: ['Picardía', 'Guiños, bromas, finge dormir', 'Serio'],
  timidez: ['Timidez', 'Se asusta y se avergüenza fácil', 'Valiente'],
  curiosidad: ['Curiosidad', 'Investiga todo', 'Se concentra'],
  malGenio: ['Mal genio', 'Se enoja rápido, se le pasa lento', 'Paciente'],
};
export const ESTADO = ['energia', 'animo', 'afecto', 'aburrimiento', 'curiosidad', 'sueno', 'estres'] as const;
export const NOMBRES_ESTADO: Record<string, string> = {
  energia: 'Energía', animo: 'Ánimo', afecto: 'Afecto', aburrimiento: 'Aburrimiento', curiosidad: 'Curiosidad', sueno: 'Sueño', estres: 'Estrés',
};
export const PRESETS = personalidades as unknown as Record<string, Personalidad>;
export const NOMBRES_PRESET: Record<string, string> = {
  equilibrado: 'Equilibrado', timido: 'Tímido', picaro: 'Pícaro', dormilon: 'Dormilón', carinoso: 'Cariñoso',
  curioso: 'Curioso', grunon: 'Gruñón', electrico: 'Eléctrico',
};
export const MOVIMIENTOS = ['ninguno', 'rebote', 'saltitos', 'risa', 'mirada-lado', 'salto', 'encogerse', 'dormir', 'temblor', 'resoplido',
  'lagrima', 'achicarse', 'picaro', 'latido', 'bamboleo', 'escuchar', 'hablar', 'pensar', 'duda', 'acercarse', 'suspiro', 'quieto',
  'cabeceo', 'carga', 'orbita', 'buscar', 'caer', 'glitch', 'estirarse'] as const;
export const PARTICULAS = ['zzz', 'corazon', 'lagrima', 'gota', 'chispa', 'pregunta', 'exclamacion', 'nota', 'estrella', 'burbuja'] as const;
export const PARPADEOS = ['normal', 'menos', 'mas', 'no', 'lento', 'rapido', 'doble'] as const;
export const MIRADAS = ['libre', 'frente', 'arriba', 'abajo', 'izquierda', 'derecha', 'lado', 'alrededor', 'perdida', 'reojo', 'estimulo', 'rem', 'ovejas'] as const;
// Los parámetros del ojo y los globales, con sus límites (los de la marca: config.schema.json).
export const PARAMS_OJO: [string, number, number, string][] = [
  ['w', 0.03, 0.2, 'Ancho'], ['h', 0.03, 0.32, 'Alto'], ['r', 0, 1, 'Redondez'], ['tilt', -0.6, 0.6, 'Inclinación'],
  ['lt', 0, 0.5, 'Párpado de arriba'], ['lta', -0.8, 0.8, 'Su ángulo'], ['lb', 0, 0.5, 'Párpado de abajo'], ['lba', -0.8, 0.8, 'Su ángulo'],
  ['smile', 0, 1, 'Arco feliz'], ['heart', 0, 1, 'Corazón'], ['star', 0, 1, 'Estrella'], ['cross', 0, 1, 'Cruz'],
  ['ring', 0, 1, 'Anillo'], ['pupil', 0, 1, 'Pupila'], ['squash', 0.7, 1.3, 'Aplastado'],
];
export const PARAMS_CARA: [string, number, number, string][] = [
  ['gx', -0.3, 0.3, 'Mirada →'], ['gy', -0.25, 0.25, 'Mirada ↑'], ['ey', -0.25, 0.25, 'Altura'], ['sep', 0.15, 0.45, 'Separación'],
  ['ftilt', -0.6, 0.6, 'Inclinación'], ['fscale', 0.7, 1.3, 'Escala'], ['spin', 0, 1, 'Remolino'], ['orbit', 0, 1, 'Órbita'],
  ['pulse', 0, 1, 'Pulso'], ['beat', 0, 1, 'Latido'], ['glitch', 0, 1, 'Glitch'], ['dim', 0, 1, 'Apagado'], ['hue', 0, 1, 'Rojo'],
  ['mo', 0, 0.2, 'Boca abierta'], ['mw', 0.02, 0.2, 'Boca ancha'], ['mcurve', -1, 1, 'Sonrisa / puchero'], ['blush', 0, 1, 'Cachetes'],
  ['shake', 0, 1, 'Temblor'],
];

export type Personalidad = Partial<Record<(typeof RASGOS)[number], number>> & { preset?: string; sueno?: string };
export type OjoVida = Partial<Record<string, number>>;
export type ExpresionVida = {
  extiende?: string; ojo?: OjoVida; izq?: OjoVida; der?: OjoVida;
  movimiento?: string; particula?: string; parpadeo?: string;
  [param: string]: unknown;
};
export type Paso = {
  cara?: string; dura?: number | [number, number]; veces?: number | [number, number]; mirada?: string; parpadeo?: string;
  particula?: string; cuantas?: number; peso?: number; sonido?: string; estado?: string;
};
export type Comportamiento = {
  tipo?: 'ocioso' | 'reaccion' | 'rutina'; cuando?: string; necesidad?: string; base?: number; probabilidad?: number;
  enfriamiento?: number; interrumpible?: boolean; afinidad?: Partial<Record<string, number>>; pasos: Paso[];
};
export const EXPRESIONES_BASE = expresionesBase as unknown as Record<string, ExpresionVida>;
export const COMPORTAMIENTOS_BASE = comportamientosBase as unknown as Record<string, Comportamiento>;
export const REGLAS_BASE = reglasBase as unknown as Regla[];
export const esDibujo = (e: Expresion | ExpresionVida): e is Expresion => Array.isArray((e as Expresion).formas);
// Una cara de imagen (docs/motor-de-vida.md, "Caras de imagen"): ojos y boca con sprites (0 a 11).
export const PARTES_SPRITE = ['ojo', 'ojo_cerrado', 'boca', 'ojo_der', 'ojo_der_cerrado'] as const;
export type ParteSprite = (typeof PARTES_SPRITE)[number];
export type ExpresionImagen = ExpresionVida & {
  sprites: Partial<Record<ParteSprite, number>>; tamano?: number; boca_tamano?: number; boca_y?: number; espejar_der?: boolean;
};
export const MAX_SPRITES = 12;
export const esImagen = (e: Expresion | ExpresionVida | undefined): e is ExpresionImagen =>
  !!e && typeof (e as ExpresionImagen).sprites === 'object' && !Array.isArray((e as ExpresionImagen).sprites);
// Los sprites que usa cada cara de imagen de la config.
export function spritesEnUso(c: Config, menos = ''): Set<number> {
  const l = new Set<number>();
  for (const [n, e] of Object.entries(c.expresiones ?? {})) if (n !== menos && esImagen(e)) for (const x of Object.values(e.sprites)) if (typeof x === 'number') l.add(x);
  return l;
}
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
export type Regla = { si: string; y?: string; entonces: string[] };
export type Config = {
  version: 1 | 2;
  nombre?: string;
  semilla?: number;
  cara?: { color?: string; tamano?: number; ojos?: string; expresion?: string };
  personalidad?: Personalidad;
  horario?: { despierta?: string; duerme?: string };
  dueno?: { nombre?: string; cumple?: string };
  cumple?: string;
  limites_marca?: { color?: string; brillo_max?: number };
  expresiones?: Record<string, Expresion | ExpresionVida>;
  comportamientos?: Record<string, Comportamiento>;
  animaciones?: Record<string, Animacion>;
  reglas?: Regla[];
  modo?: string;
  lista?: string[];
  rotar?: number;
  recordatorios?: Recordatorio[];
  voz?: { volumen?: number; tipo?: 'grave' | 'media' | 'aguda' | 'robot'; palabra?: string; conversar?: boolean; microfono?: number };
  pomodoro?: { trabajo?: number; descanso?: number };
  // Protector de pantalla: tras `espera` segundos sin que pase nada, muestra esa imagen; cualquier cosa vuelve a la cara.
  protector?: { imagen: number; espera?: number };
  // Cuánto habla solo (0 a 1) y sus frases propias por situación (hasta 12 en total).
  habla?: { frecuencia?: number; propias?: Partial<Record<Situacion, string[]>> };
  despertador?: Despertador;
  // Para el clima (la usa la nube).
  ubicacion?: Ubicacion;
  cuidado?: Cuidado;
  avisos?: Aviso[];
};

// Cuidarte: agua cada X horas, pausa activa en horario de trabajo, "¿Cómo estás?"
// una vez por día, y por Telegram "te extraño" y cómo fue su día.
export type Cuidado = {
  agua?: number; // horas (0 = nunca; 1 a 8)
  pausa?: number; // minutos sin tocarlo (0 = nunca; 30 a 240)
  trabajo?: { desde?: string; hasta?: string; dias?: string };
  animo?: boolean;
  extranar?: boolean;
  extranarDias?: number; // 1 a 7
  resumen?: boolean;
};
export const CUIDADO_BASE: Required<Omit<Cuidado, 'trabajo'>> & { trabajo: { desde: string; hasta: string; dias: string } } = {
  agua: 0, pausa: 0, trabajo: { desde: '09:00', hasta: '18:00', dias: 'LMXJV' },
  animo: true, extranar: true, extranarDias: 2, resumen: false,
};

// Cómo reacciona a un aviso del celu (el primero que coincide gana; app y
// contacto vacíos = cualquiera). En `decir`, {app} y {contacto} se reemplazan.
export type Aviso = {
  tipo: 'notificacion' | 'llamada'; app?: string; contacto?: string;
  expresion?: string; comportamiento?: string; decir?: string; sonido?: boolean;
};
export const MAX_AVISOS = 8;
export const MAX_DECIR_AVISO = 40;
export const MAX_NOMBRE_AVISO = 24;

// Los juegos (protocolo 0x43): lo que se manda a POST /v1/bichos/{id}/jugar.
export const JUEGOS = [
  ['ppt', 'Piedra, papel o tijera'], ['reflejos', 'Reflejos'], ['mirada', 'Seguí la mirada'], ['dado', 'Dado'], ['moneda', 'Moneda'],
] as const;
export type Juego = (typeof JUEGOS)[number][0];

// Un solo despertador por robot: suena creciente y se pospone.
export type Despertador = { hora: string; dias?: string; activo?: boolean; sonido?: boolean; texto?: string; posponer?: number };
export type Ubicacion = { lat: number; lon: number; ciudad: string };

// Las situaciones en las que habla solo (las frases de fábrica: shared/formato/vida/frases.json).
export const SITUACIONES = [
  'despertar', 'buenas-noches', 'volviste', 'te-extrane', 'aburrido', 'sueno', 'bateria-baja', 'cargando', 'caricia', 'saludo', 'hito',
  'cumple-dueno', 'cumple-robot', 'recordatorio-hecho', 'despertador', 'posponer', 'clima-frio', 'clima-calor', 'clima-lluvia', 'clima-lindo',
] as const;
export type Situacion = (typeof SITUACIONES)[number];
export const NOMBRES_SITUACION: Record<Situacion, string> = {
  despertar: 'Al despertar', 'buenas-noches': 'Buenas noches', volviste: 'Volviste', 'te-extrane': 'Te extrañó', aburrido: 'Aburrido',
  sueno: 'Con sueño', 'bateria-baja': 'Sin pila', cargando: 'Cargando', caricia: 'Caricia', saludo: 'Saludo', hito: 'Un hito',
  'cumple-dueno': 'Tu cumple', 'cumple-robot': 'Su cumple', 'recordatorio-hecho': 'Recordatorio hecho', despertador: 'Despertador',
  posponer: 'Posponer', 'clima-frio': 'Hace frío', 'clima-calor': 'Hace calor', 'clima-lluvia': 'Llueve', 'clima-lindo': 'Día lindo',
};
export const MAX_FRASES = 12;
export const MAX_TEXTO = 60;
export const bytes = (t: string) => new TextEncoder().encode(t).length;

// Un recordatorio: a la hora, el robot lo muestra en su pantalla hasta que lo tocan.
// dias: letras de L M X J V S D (vacío = todos los días).
export type Recordatorio = { hora: string; dias?: string; texto: string; expresion?: string; sonido?: boolean; activo?: boolean };
export const MAX_RECORDATORIOS = 12;

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

// Una config "vacía" pero completa: lo que muestran los editores si el robot no tiene nada.
export function configBase(): Config {
  return { version: 2 };
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
