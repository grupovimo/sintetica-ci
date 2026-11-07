// La web le habla a la API a través de este mismo servidor (/api/*): así la
// cookie de sesión es de la web y no hace falta CORS.
import type { Handle } from '@sveltejs/kit';

const API = process.env.API_INTERNA ?? 'http://localhost:8080';
const PASAN = ['content-type', 'cookie', 'authorization', 'x-sintetica', 'x-conexion', 'accept', 'last-event-id'];

export const handle: Handle = async ({ event, resolve }) => {
  const { pathname, search } = event.url;
  if (!pathname.startsWith('/api/')) return resolve(event);
  const encabezados = new Headers();
  for (const k of PASAN) {
    const v = event.request.headers.get(k);
    if (v) encabezados.set(k, v);
  }
  encabezados.set('x-forwarded-for', event.getClientAddress());
  const conCuerpo = !['GET', 'HEAD'].includes(event.request.method);
  let r: Response;
  try {
    r = await fetch(API + pathname.slice(4) + search, {
      method: event.request.method,
      headers: encabezados,
      body: conCuerpo ? event.request.body : undefined,
      // @ts-expect-error: Node necesita esto para mandar el cuerpo como stream
      duplex: conCuerpo ? 'half' : undefined,
      redirect: 'manual',
      signal: event.request.signal,
    });
  } catch {
    return new Response(JSON.stringify({ error: 'la API no responde' }), { status: 502, headers: { 'content-type': 'application/json' } });
  }
  const salida = new Headers();
  r.headers.forEach((v, k) => {
    if (!['content-encoding', 'content-length', 'transfer-encoding', 'connection'].includes(k)) salida.set(k, v);
  });
  for (const c of r.headers.getSetCookie()) salida.append('set-cookie', c);
  return new Response(r.body, { status: r.status, headers: salida });
};
