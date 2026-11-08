// La web es una aplicación del navegador (sin render en el servidor): el
// servidor de SvelteKit sirve los archivos y hace de proxy a la API.
import { redirect } from '@sveltejs/kit';
import type { Usuario } from '$lib/tipos';

export const ssr = false;
export const trailingSlash = 'never';

export async function load({ fetch, url }) {
  let usuario: Usuario | null = null;
  let desarrollo = false;
  let soloLanding = false;
  try {
    const r = await fetch('/api/v1/sesion');
    if (r.ok) {
      const d = await r.json();
      usuario = d.usuario ?? null;
      desarrollo = !!d.desarrollo;
      soloLanding = !!d.soloLanding;
    }
  } catch {
    /* la API no responde: se ve en cada página */
  }
  // Públicas: la landing y entrar.
  const publica = url.pathname === '/' || url.pathname.startsWith('/piezas') || url.pathname.startsWith('/entrar');
  if (!usuario && !publica) {
    redirect(307, `/entrar?volver=${encodeURIComponent(url.pathname + url.search)}`);
  }
  return { usuario, desarrollo, soloLanding };
}
