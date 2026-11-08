// Las piezas de Sintética: los robots y la joyería electrónica. Cada una tiene
// su página (/piezas/<clave>) y se puede reservar. La API acepta estas claves
// (api/internal/servidor/reservas.go).

export type Pieza = {
  clave: string;
  nombre: string;
  tipo: 'Robot' | 'Joyería electrónica';
  frase: string;
  foto: string; // en static/; vacía = falta la foto real
  queMuestra: string; // lo que tiene que mostrar la foto (alt, o el lugar marcado)
  fichas: string[];
};

const joya = ['Cara en pantalla', 'Por Bluetooth', 'Recargable', 'Nº grabado', 'Hecha a mano'];

export const PIEZAS: Pieza[] = [
  {
    clave: 'unidad',
    nombre: 'Unidad',
    tipo: 'Robot',
    frase: 'Tu robot de escritorio.',
    foto: '/landing/unidad.jpg',
    queMuestra: 'La Unidad, un cubo naranja con dos ojos, sobre el escritorio',
    fichas: ['Cara en pantalla', 'WiFi + Bluetooth', 'Siente el movimiento', 'Se actualiza sola', 'Nº grabado'],
  },
  {
    clave: 'broche',
    nombre: 'Broche',
    tipo: 'Joyería electrónica',
    frase: 'Tu robot, puesto.',
    foto: '/landing/hebilla.jpg',
    queMuestra: 'el Broche naranja en un cinturón, guiñando',
    fichas: joya,
  },
  {
    clave: 'hebilla',
    nombre: 'Hebilla',
    tipo: 'Joyería electrónica',
    frase: 'El cinturón te sonríe.',
    foto: '/landing/pieza-hebilla.jpg',
    queMuestra: 'la hebilla de cinturón con una carita contenta',
    fichas: joya,
  },
  {
    clave: 'colgante',
    nombre: 'Colgante',
    tipo: 'Joyería electrónica',
    frase: 'Cerca del corazón.',
    foto: '/landing/pieza-colgante.jpg',
    queMuestra: 'el colgante redondo con cadena, guiñando',
    fichas: joya,
  },
  {
    clave: 'translucido',
    nombre: 'Translúcido',
    tipo: 'Joyería electrónica',
    frase: 'Se le ve todo.',
    foto: '/landing/pieza-translucido.jpg',
    queMuestra: 'el broche translúcido, con la electrónica a la vista y los ojos cerrados',
    fichas: joya,
  },
  {
    clave: 'alfiler',
    nombre: 'Alfiler',
    tipo: 'Joyería electrónica',
    frase: 'Una cara en la solapa.',
    foto: '/landing/pieza-alfiler.jpg',
    queMuestra: 'el alfiler con una carita sorprendida',
    fichas: joya,
  },
  {
    clave: 'hebilla-pelo',
    nombre: 'Hebilla de pelo',
    tipo: 'Joyería electrónica',
    frase: 'Enamorada de tu peinado.',
    foto: '/landing/pieza-hebilla-pelo.jpg',
    queMuestra: 'la hebilla de pelo con ojos de corazón',
    fichas: joya,
  },
];

export const pieza = (clave: string) => PIEZAS.find((p) => p.clave === clave);
export const JOYAS = PIEZAS.filter((p) => p.tipo === 'Joyería electrónica' && p.clave !== 'broche');
