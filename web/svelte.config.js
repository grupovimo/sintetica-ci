import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ out: 'build' }),
    // La API ya se protege de CSRF (encabezado X-Sintetica); así la web anda
    // igual desde localhost o desde la IP de la Mac (el celu).
    csrf: { checkOrigin: false },
    alias: {
      // El protocolo y QOI de referencia: los mismos archivos que usan el
      // simulador y los tests (sim/web).
      $compartido: '../sim/web',
    },
  },
};
