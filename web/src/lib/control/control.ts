// El centro de control del dueño (/admin): los tipos de la API
// (/v1/admin/..., api/internal/servidor/control.go) y cómo se muestran.
import { PIEZAS } from '$lib/landing/piezas';

export type Moneda = 'USD' | 'ARS';

export type Componente = {
  id: number;
  nombre: string;
  categoria: string;
  proveedor: string;
  link: string;
  moneda: Moneda;
  costo: number;
  stock: number;
  stockMinimo: number;
  notas: string;
  actualizado: string;
};

export type Material = { modelo: string; componenteId: number; cantidad: number };

export type Compra = {
  id: number;
  componenteId: number;
  cantidad: number;
  costoUnitario: number;
  moneda: Moneda;
  proveedor: string;
  link: string;
  pedida: string;
  recibida: string;
  notas: string;
};

export type EstadoUnidad = 'armada' | 'vendida' | 'desplegada' | 'garantia' | 'devuelta' | 'baja';

export type Unidad = {
  id: number;
  serie: string;
  modelo: string;
  color: string;
  estado: EstadoUnidad;
  bichoId: string;
  costoUsd: number;
  costoArs: number;
  dolar: number;
  cliente: string;
  contacto: string;
  precio: number;
  moneda: Moneda;
  canal: string;
  cobrado: boolean;
  vendidaEn: string;
  desplegadaEn: string;
  notas: string;
  historial: { en: string; texto: string }[];
  creada: string;
};

export type BichoAdmin = {
  id: string;
  perfil: string;
  apodo: string;
  virtual: boolean;
  conectado: boolean;
  ultimaConexion: string | null;
  dueno: string;
  firmware: string;
  unidad: string;
  creado: string;
};

export type Preventa = {
  modelo: string;
  activa: boolean;
  precio: number;
  precioFinal: number;
  sena: number;
  cupos: number;
  entrega: string;
};

export type Sena = {
  id: number;
  nombre: string;
  contacto: string;
  email: string;
  color: string;
  monto: number;
  estado: 'pendiente' | 'aprobada' | 'rechazada' | 'devuelta';
  mpPago: string;
  unidadId: number | null;
  creada: string;
  pagada: string | null;
};

export type CostoModelo = { modelo: string; usd: number; ars: number; puedoArmar: number; piezas: number };

export type Resumen = {
  dolar: number;
  dolarFecha: string;
  dolarError: boolean;
  mes: string;
  estados: Partial<Record<EstadoUnidad, number>>;
  unidades: number;
  vendidasMes: number;
  ventasMes: number;
  cobradoMes: number;
  margenMes: number;
  porCobrar: number;
  invertido: number;
  costos: CostoModelo[];
  stockBajo: Componente[];
  preventa: { activa: boolean; cupos: number; quedan: number; senas: number; senado: number; conMercadoPago: boolean };
};

export const MODULOS = [
  { ruta: '/admin', nombre: 'Resumen' },
  { ruta: '/admin/unidades', nombre: 'Unidades' },
  { ruta: '/admin/componentes', nombre: 'Componentes' },
  { ruta: '/admin/compras', nombre: 'Compras' },
  { ruta: '/admin/ventas', nombre: 'Ventas' },
  { ruta: '/admin/preventa', nombre: 'Preventa' },
];

export const ESTADOS: { clave: EstadoUnidad; nombre: string; led: '' | 'on' | 'verde' | 'ambar' }[] = [
  { clave: 'armada', nombre: 'Armada', led: '' },
  { clave: 'vendida', nombre: 'Vendida', led: 'ambar' },
  { clave: 'desplegada', nombre: 'Desplegada', led: 'verde' },
  { clave: 'garantia', nombre: 'Garantía', led: 'on' },
  { clave: 'devuelta', nombre: 'Devuelta', led: 'on' },
  { clave: 'baja', nombre: 'Baja', led: '' },
];
export const nombreEstado = (e: string) => ESTADOS.find((x) => x.clave === e)?.nombre ?? e;
export const ledEstado = (e: string) => ESTADOS.find((x) => x.clave === e)?.led ?? '';

export const MODELOS = PIEZAS.map((p) => ({ clave: p.clave, nombre: p.nombre }));
export const nombreModelo = (m: string) => MODELOS.find((x) => x.clave === m)?.nombre ?? m;
export const COLORES = [
  { clave: 'naranja', nombre: 'Naranja' },
  { clave: 'negro', nombre: 'Negro' },
  { clave: 'blanco', nombre: 'Blanco' },
];

const ars = new Intl.NumberFormat('es-AR', { maximumFractionDigits: 0 });
const usd = new Intl.NumberFormat('es-AR', { minimumFractionDigits: 0, maximumFractionDigits: 2 });

/** "$ 12.000" o "US$ 12,5". */
export function plata(v: number, moneda: Moneda | string = 'ARS'): string {
  if (moneda === 'USD') return 'US$ ' + usd.format(v);
  return '$ ' + ars.format(v);
}

/** En pesos, con el dólar del día. */
export const enPesos = (v: number, moneda: string, dolar: number) => (moneda === 'USD' ? v * dolar : v);
/** En dólares, con el dólar del día (0 si no hay dólar). */
export const enDolares = (v: number, moneda: string, dolar: number) => (moneda === 'USD' ? v : dolar > 0 ? v / dolar : 0);

export const hoy = () => new Date(Date.now() - 3 * 3600e3).toISOString().slice(0, 10);
export const fecha = (iso: string) =>
  iso ? new Date(iso.length === 10 ? iso + 'T12:00:00-03:00' : iso).toLocaleDateString('es-AR', { day: 'numeric', month: 'short', year: '2-digit' }) : '—';
