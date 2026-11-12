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

// Un mail sin leer (la nube lo sabe en memoria, no lo guarda).
export type MailNoLeido = { de: string; asunto: string; en: string; proveedor: 'gmail' | 'outlook' };

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
  | { tipo: 'mcp'; bicho: string; datos: { cliente: string; herramienta: string; en: string } }
  // v1.9: un toque de un amigo (o el que te devolvió) y un aviso entrante (GitHub, Mercado Pago, JSON propio).
  | { tipo: 'toque'; bicho: string; datos: { de: string; tipo: 'toque' | 'abrazo' | 'beso' | 'carita'; robot: string; devuelto?: boolean } }
  | { tipo: 'entrante'; bicho?: string; datos: { tipo: 'pago' | 'mail' | 'github' | 'pedido' | 'otro'; titulo: string; de: string; origen: 'json' | 'github' | 'mercadopago'; robots: string[] } }
  // v1.9: la agenda (cambió o se viene un evento), lo que suena (Spotify sin robot; el celu, con su robot) y la charla con IA.
  | { tipo: 'agenda'; bicho?: string; datos: { conectada?: boolean; titulo?: string; minutos?: number; manana?: number } }
  | { tipo: 'musica'; bicho?: string; datos: { conectada?: boolean; nombre?: string; sonando?: boolean; titulo?: string; artista?: string; bpm?: number; de?: string } }
  // El mail (Gmail u Outlook): se conectó o desconectó una cuenta, o llegaron mails nuevos.
  | { tipo: 'correo'; bicho?: string; datos: { proveedor?: 'gmail' | 'outlook'; conectada?: boolean; direccion?: string; nuevos?: MailNoLeido[]; texto?: string; robots?: string[] } }
  | { tipo: 'charla'; bicho: string; datos: { texto: string; expresion: string; de: string; enviado: boolean } }
  // La voz (ElevenLabs conectado o no, o cambió la de Edge), el paquete de voz sin internet y la clave de IA propia.
  | { tipo: 'voz'; bicho?: string; datos: { conectado?: boolean; edge?: boolean } }
  | { tipo: 'voz-paquete'; bicho: string; datos: { estado: string; version: number; frases: number } }
  | { tipo: 'ia'; bicho?: string; datos: { propia: boolean } };

// Cómo ve la pantalla cada perfil (también los virtuales).
export function pantallaDe(b: { perfil: string; estado: Estado | null }) {
  if (b.estado?.pantalla?.ancho) return b.estado.pantalla;
  const redonda = b.perfil.includes('broche');
  return redonda ? { ancho: 240, alto: 240, redonda: true } : { ancho: 320, alto: 172, redonda: false };
}
