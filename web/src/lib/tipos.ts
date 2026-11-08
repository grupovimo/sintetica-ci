export type Usuario = { id: string; email: string; nombre: string; admin: boolean };

export type Estado = {
  bateria: number | null;
  humor: number;
  modo: string;
  dormido: boolean;
  fija: boolean;
  emparejando: boolean;
  wifi: boolean;
  nube: boolean;
  redConfigurada: boolean;
  actualizando: boolean;
  expresion: string;
  nombre: string;
  serie: string;
  id: string;
  pantalla: { ancho: number; alto: number; redonda: boolean };
  version: string;
  perfil: string;
  crcConfig: number;
  encendido: number;
};

export type Sincro = { config: string; imagenes: string; detalle?: string };

export type VersionConfig = { version: number; crc: number; autor: string; creada: string };

export type Bicho = {
  id: string;
  perfil: string;
  apodo: string;
  virtual: boolean;
  conectado: boolean;
  ultimaConexion: string | null;
  estadoEn: string | null;
  creado: string;
  estado: Estado | null;
  sincro: Sincro;
  config: VersionConfig | null;
};

export type Imagen = { espacio: number; largo: number; crc: number; actualizada: string };

export type Pieza = { id: string; tipo: 'cara' | 'animacion'; nombre: string; documento: any; actualizado: string };

export type Firmware = { id: number; perfil: string; version: string; tamano: number; notas: string; creado: string; sha256: string };

export type Actualizacion = {
  id: number;
  bicho: string;
  firmware: number;
  version: string;
  estado: 'enviada' | 'descargando' | 'instalada' | 'confirmada' | 'fallida';
  progreso: number;
  detalle: string;
  creada: string;
  actualizada: string;
};

export type Evento =
  | { tipo: 'estado'; bicho: string; datos: Estado }
  | { tipo: 'conexion'; bicho: string; datos: { conectado: boolean } }
  | { tipo: 'sincro'; bicho: string; datos: Sincro }
  | { tipo: 'config'; bicho: string; datos: VersionConfig }
  | { tipo: 'imagenes'; bicho: string }
  | { tipo: 'sprites'; bicho: string }
  | { tipo: 'ota'; bicho: string; datos: Actualizacion }
  | { tipo: 'marco'; bicho: string; datos: { car: number; datos: string } }
  | { tipo: 'bicho'; bicho: string; datos: { accion: 'sumado' | 'quitado' | 'actual'; bicho?: Bicho } }
  | { tipo: 'automatizacion'; bicho: string; datos: { id: string; cuando: string; evento: string; resultados: unknown; en: string } }
  | { tipo: 'telegram'; bicho?: string; datos: { vinculado: boolean; nombre?: string } }
  | { tipo: 'mcp'; bicho: string; datos: { cliente: string; herramienta: string; en: string } };

// Cómo ve la pantalla cada perfil (también los virtuales).
export function pantallaDe(b: { perfil: string; estado: Estado | null }) {
  if (b.estado?.pantalla?.ancho) return b.estado.pantalla;
  const redonda = b.perfil.includes('broche');
  return redonda ? { ancho: 240, alto: 240, redonda: true } : { ancho: 320, alto: 172, redonda: false };
}
