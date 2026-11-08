// Cómo se presenta cada robot en la consola: su nombre de producto, la foto
// real y dónde cae la pantalla en esa foto, y lo que trae (capacidades).
// Nada de acá nombra placas ni componentes: solo lo que ve el usuario.
import { pantallaDe, type Bicho } from '$lib/tipos';

export type Capacidad = 'pantalla' | 'movimiento' | 'bluetooth' | 'wifi' | 'microfono' | 'parlante';

export const NOMBRES_CAPACIDAD: Record<Capacidad, string> = {
  pantalla: 'Pantalla',
  movimiento: 'Movimiento',
  bluetooth: 'Bluetooth',
  wifi: 'WiFi',
  microfono: 'Micrófono',
  parlante: 'Parlante',
};

// La foto ocupa una ventana 4:5. `foto` es la posición de la imagen (en % de
// la ventana) y `pantalla` la caja de la pantalla encima de esa imagen.
export type Encuadre = {
  src: string;
  alt: string;
  foto: { ancho: number; izq: number; arriba: number };
  pantalla: { izq: number; arriba: number; ancho: number; alto: number };
  fondo: string;
};

const RECTANGULAR: Encuadre = {
  src: '/landing/hebilla.jpg',
  alt: 'El robot puesto en un cinturón',
  foto: { ancho: 155.56, izq: -27.78, arriba: -25.56 },
  pantalla: { izq: 36.11, arriba: 23.33, ancho: 25, alto: 42.78 },
  fondo: '#0e0d0b',
};

const REDONDA: Encuadre = {
  src: '/landing/pieza-colgante.jpg',
  alt: 'El robot colgado de una cadena',
  foto: { ancho: 160, izq: -28.2, arriba: -43.1 },
  pantalla: { izq: 26.8, arriba: 36.9, ancho: 46.2, alto: 37 },
  fondo: '#f3eeea',
};

export type Modelo = {
  nombre: string;
  redonda: boolean;
  encuadre: Encuadre;
  capacidades: Set<Capacidad>;
};

export function modeloDe(b: Bicho): Modelo {
  const p = pantallaDe(b);
  const broche = b.perfil.includes('broche');
  // Los virtuales traen todo, así se puede probar cada módulo sin hardware.
  const capacidades = new Set<Capacidad>(
    b.virtual
      ? ['pantalla', 'movimiento', 'bluetooth', 'wifi', 'microfono', 'parlante']
      : broche
        ? ['pantalla', 'movimiento', 'bluetooth', 'wifi']
        : ['pantalla', 'movimiento', 'bluetooth', 'wifi'],
  );
  return {
    nombre: broche ? 'Broche' : 'Unidad',
    redonda: p.redonda,
    encuadre: p.redonda ? REDONDA : RECTANGULAR,
    capacidades,
  };
}

// "Nº 001": los últimos dígitos del id, para grabar en la placa.
export function numeroDe(id: string): string {
  const n = parseInt(id.replace(/[^0-9A-F]/gi, '').slice(-4) || '0', 16) % 1000;
  return 'Nº ' + String(n).padStart(3, '0');
}

// El ánimo en palabras, a partir del humor (-1..1) y la expresión que se ve.
export function animoDe(b: Bicho): string {
  const e = b.estado;
  if (!e) return b.conectado ? 'Despertando' : 'Sin noticias';
  if (e.dormido) return 'Durmiendo';
  if (e.actualizando) return 'Actualizándose';
  if (e.humor > 0.5) return 'Muy contento';
  if (e.humor > 0.15) return 'Contento';
  if (e.humor < -0.5) return 'Bajoneado';
  if (e.humor < -0.15) return 'Medio molesto';
  return 'Tranquilo';
}

export function hace(iso: string | null): string {
  if (!iso) return 'nunca';
  const s = (Date.now() - Date.parse(iso)) / 1000;
  if (s < 90) return 'recién';
  if (s < 3600) return `hace ${Math.round(s / 60)} min`;
  if (s < 86400) return `hace ${Math.round(s / 3600)} h`;
  return `hace ${Math.round(s / 86400)} días`;
}
