// La consola de los robots se mudó a /robots: los links viejos siguen andando.
import { redirect } from '@sveltejs/kit';

export function load({ params, url }) {
  redirect(308, `/robots${params.resto ? '/' + params.resto : ''}${url.search}`);
}
