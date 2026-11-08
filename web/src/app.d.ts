declare global {
  // Las partículas del schema, en el orden del núcleo (vite.config.ts).
  const __PARTICULAS__: readonly string[];
  namespace App {
    interface Locals {
      api: string;
    }
  }
}
export {};
