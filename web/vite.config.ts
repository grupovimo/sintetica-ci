import fs from 'node:fs';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

// Del schema, solo lo que la web necesita como lista (sin cargar los 70 KB):
// las partículas en el orden del núcleo (las de un paso de comportamiento).
const esquema = JSON.parse(fs.readFileSync(new URL('../shared/formato/config.schema.json', import.meta.url), 'utf8'));
const particulas: string[] = esquema.$defs.paso.properties.particula.enum;

export default defineConfig({
  plugins: [sveltekit()],
  define: {
    __PARTICULAS__: JSON.stringify(particulas),
  },
  server: {
    fs: { allow: ['..'] },
  },
  test: {
    include: ['src/**/*.test.ts'],
  },
});
