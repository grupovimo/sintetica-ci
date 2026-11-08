// La web en "solo landing" (SOLO_LANDING=1): sirve la portada y las páginas de
// las piezas, sin API atrás. Las reservas no se guardan: llegan por Telegram
// (el bot de Sintética, al chat de TELEGRAM_CHAT_RESERVAS). Mismas reglas que
// POST /v1/reservas de la API (api/internal/servidor/reservas.go).

const PIEZAS: Record<string, string> = {
  unidad: 'Unidad', broche: 'Broche', hebilla: 'Hebilla', colgante: 'Colgante',
  translucido: 'Translúcido', alfiler: 'Alfiler', 'hebilla-pelo': 'Hebilla de pelo',
};
const COLORES: Record<string, string> = { naranja: 'Naranja', negro: 'Negro', blanco: 'Blanco' };

// Lo que se puede ver: la portada, las piezas, lo estático y el armado de la app.
export function rutaPermitida(p: string): boolean {
  return p === '/' || p.startsWith('/piezas') || p.startsWith('/_app/') || p.startsWith('/landing/') ||
    p.startsWith('/fuentes/') || p === '/favicon.svg' || p === '/sinteticaos.wasm' || p === '/robots.txt';
}

// Topes por IP en memoria: 5 por hora y 10 por día.
const pedidos = new Map<string, number[]>();
function permitir(ip: string): boolean {
  const ahora = Date.now();
  const l = (pedidos.get(ip) ?? []).filter((t) => ahora - t < 86_400_000);
  if (l.filter((t) => ahora - t < 3_600_000).length >= 5 || l.length >= 10) return false;
  l.push(ahora);
  pedidos.set(ip, l);
  if (pedidos.size > 10_000) pedidos.clear();
  return true;
}

const json = (estado: number, cuerpo: unknown) =>
  new Response(JSON.stringify(cuerpo), { status: estado, headers: { 'content-type': 'application/json' } });
const fallar = (estado: number, error: string) => json(estado, { error });
const largo = (s: string) => [...s].length;
// Telegram con parse_mode HTML: se escapa todo lo que escribió la persona.
const escapar = (s: string) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');

export async function reservar(pedido: Request, ip: string): Promise<Response> {
  let p: Record<string, unknown>;
  try {
    p = await pedido.json();
  } catch {
    return fallar(400, 'no entendí el pedido');
  }
  const texto = (k: string) => (typeof p[k] === 'string' ? (p[k] as string).trim() : '');
  const r = { nombre: texto('nombre'), pieza: texto('pieza'), contacto: texto('contacto'), color: texto('color'), mensaje: texto('mensaje') };
  if (!r.nombre || largo(r.nombre) > 80) return fallar(400, 'decinos tu nombre (hasta 80 letras)');
  if (largo(r.contacto) < 5 || largo(r.contacto) > 120) return fallar(400, 'dejanos un email o un WhatsApp para escribirte');
  if (!PIEZAS[r.pieza]) return fallar(400, 'esa pieza no existe');
  if (!COLORES[r.color]) return fallar(400, 'el color es naranja, negro o blanco');
  if (largo(r.mensaje) > 500) return fallar(400, 'el mensaje es de hasta 500 letras');
  // La trampa para bots: se contesta bien y no se manda.
  if (texto('sitio')) return json(201, { ok: true });
  if (!permitir(ip)) return fallar(429, 'ya nos dejaste varias reservas: te escribimos pronto');

  const token = process.env.TELEGRAM_TOKEN, chat = process.env.TELEGRAM_CHAT_RESERVAS;
  if (!token || !chat) return fallar(503, 'las reservas no están andando: probá en un rato');
  const mensaje = [
    `🧡 <b>Reserva nueva</b> · ${PIEZAS[r.pieza]} ${COLORES[r.color].toLowerCase()}`,
    `<b>${escapar(r.nombre)}</b>`,
    escapar(r.contacto),
    r.mensaje ? `\n${escapar(r.mensaje)}` : '',
  ].filter(Boolean).join('\n');
  try {
    const t = await fetch(`https://api.telegram.org/bot${token}/sendMessage`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ chat_id: chat, text: mensaje, parse_mode: 'HTML', disable_web_page_preview: true }),
      signal: AbortSignal.timeout(8000),
    });
    if (!t.ok) throw new Error(`telegram ${t.status}`);
  } catch (e) {
    console.error('reserva: no la pude mandar por Telegram', e instanceof Error ? e.message : e);
    return fallar(502, 'no la pude mandar: probá en un rato');
  }
  return json(201, { ok: true });
}
