// Íconos dibujados para la consola (viewBox 0 0 24 24, trazo): los de los
// hábitos (los 8 del contrato), los toques y los avisos de afuera.
import type { IconoHabito, TipoEntrante, TipoToque } from '$lib/formato';

export const ICONO_HABITO: Record<IconoHabito, string> = {
  gota: '<path d="M12 3c4 5 6 8 6 11a6 6 0 0 1-12 0c0-3 2-6 6-11z"/>',
  libro: '<path d="M3 5.5c3-1 6.5-1 9 1 2.5-2 6-2 9-1v13c-3-1-6.5-1-9 1-2.5-2-6-2-9-1z"/><path d="M12 6.5v13"/>',
  pesa: '<path d="M2.5 10v4M6 7v10M18 7v10M21.5 10v4M6 12h12"/>',
  zen: '<circle cx="12" cy="5" r="2"/><path d="M12 8v5M7.5 11.5 12 10l4.5 1.5M4 18.5c2.5-3 5-4 8-4s5.5 1 8 4M6 18.5h12"/>',
  pastilla: '<rect x="8" y="2.5" width="8" height="19" rx="4" transform="rotate(45 12 12)"/><path d="M8.6 8.6l6.8 6.8"/>',
  cama: '<path d="M3 19V8M3 15h18v4M21 15v-2a3 3 0 0 0-3-3h-7v5"/><circle cx="7" cy="11.5" r="1.8"/>',
  fruta: '<path d="M12 7.5c-3-2-7.5 0-7.5 5S8 21 10 21c1 0 1.4-.5 2-.5s1 .5 2 .5c2 0 5.5-3.5 5.5-8.5S15 5.5 12 7.5z"/><path d="M12 7.5c0-2 1-3.5 3-4.5"/>',
  estrella: '<path d="M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z"/>',
};

export const ICONO_TOQUE: Record<TipoToque, string> = {
  toque: '<circle cx="12" cy="12" r="2.4" class="lleno"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="9.8" opacity=".45"/>',
  abrazo: '<path class="lleno" d="M12 20.5C5.5 16 3 12.5 3 9a4.5 4.5 0 0 1 9-1.2A4.5 4.5 0 0 1 21 9c0 3.5-2.5 7-9 11.5z"/>',
  beso: '<path d="M2.5 12c2.5-3.5 5-4.8 7-3.6 1.2.7 1.8.7 2.5.2.7.5 1.3.5 2.5-.2 2-1.2 4.5.1 7 3.6-3 4-6 5.5-9.5 5.5S5.5 16 2.5 12z"/><path d="M2.5 12h19"/>',
  carita: '<circle cx="12" cy="12" r="9.5"/><circle cx="8.7" cy="10" r="1.2" class="lleno"/><circle cx="15.3" cy="10" r="1.2" class="lleno"/><path d="M8 14.5c2 2.6 6 2.6 8 0"/>',
};

export const ICONO_ENTRANTE: Record<TipoEntrante, string> = {
  pago: '<rect x="2.5" y="6" width="19" height="12" rx="2.5"/><circle cx="12" cy="12" r="2.6"/><path d="M6 9.5v5M18 9.5v5"/>',
  mail: '<rect x="3" y="5.5" width="18" height="13" rx="2.5"/><path d="M3.5 7l8.5 6.5L20.5 7"/>',
  github: '<circle cx="6" cy="6" r="2.2"/><circle cx="6" cy="18" r="2.2"/><circle cx="18" cy="18" r="2.2"/><path d="M6 8.2v7.6M18 15.8V11a3 3 0 0 0-3-3h-4M13 5.5 10.5 8l2.5 2.5"/>',
  pedido: '<path d="M4 8.5 12 4l8 4.5v7L12 20l-8-4.5z"/><path d="M4 8.5 12 13l8-4.5M12 13v7"/>',
  otro: '<path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2h-15z"/><path d="M10 20.5h4"/>',
};

// Un ícono como SVG en línea (para {@html}).
export const svg = (cuerpo: string, tamano = 24) =>
  `<svg viewBox="0 0 24 24" width="${tamano}" height="${tamano}" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${cuerpo}</svg>`;
