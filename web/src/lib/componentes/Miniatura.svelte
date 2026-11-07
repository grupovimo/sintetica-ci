<script lang="ts">
  // Una cara (o un cuadro de una animación) en chiquito, dibujada por el núcleo.
  import { nucleoEditor, pintar } from '$lib/nucleo';
  import type { Animacion, Config, Expresion } from '$lib/formato';

  let {
    expresion = null,
    animacion = null,
    t = -1,
    contexto = { version: 1 },
    ancho = 160,
  }: { expresion?: Expresion | null; animacion?: Animacion | null; t?: number; contexto?: Config; ancho?: number } = $props();

  let canvas: HTMLCanvasElement;

  async function dibujar() {
    const n = await nucleoEditor();
    n.editor.iniciar(320, 172, false);
    if (animacion) {
      const r = n.editor.config({ ...$state.snapshot(contexto), version: 1, animaciones: { miniatura: $state.snapshot(animacion) } });
      if (!r.ok) return pintar(canvas, new Uint8ClampedArray(320 * 172 * 4), 320, 172);
      const i = n.editor.indiceAnimacion('miniatura');
      const c = n.editor.animacion(i, t < 0 ? Math.round(animacion.duracion / 2) : t);
      pintar(canvas, c.rgba, c.ancho, c.alto);
    } else if (expresion) {
      const r = n.editor.config({ ...$state.snapshot(contexto), version: 1, expresiones: { miniatura: $state.snapshot(expresion) } });
      if (!r.ok) return;
      const c = n.editor.expresion(n.editor.indiceExpresion('miniatura'));
      pintar(canvas, c.rgba, c.ancho, c.alto);
    }
  }

  $effect(() => {
    void JSON.stringify([expresion, animacion, t, contexto?.cara]);
    dibujar();
  });
</script>

<canvas bind:this={canvas} width="320" height="172" style="width: {ancho}px" aria-hidden="true"></canvas>

<style>
  canvas { display: block; height: auto; border-radius: 8px; background: #0b0a09; }
</style>
