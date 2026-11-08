<script lang="ts">
  // La pantalla de un robot, viva: el mismo núcleo del firmware, con su config.
  // Si se pasa `visible`, muestra eso (así copia lo que se ve en el robot real).
  import { onDestroy, onMount } from 'svelte';
  import { fijar, pintar, ponerSprites, vistaPrevia, type VistaPrevia } from '$lib/nucleo';
  import { quizasBicho } from '$lib/bicho.svelte';

  let {
    config = { version: 1 },
    visible = '',
    redonda = false,
    ancho = 320,
    etiqueta = 'Pantalla del robot',
    desnuda = false,
    alError = (_: string | null) => {},
  }: {
    config?: unknown;
    visible?: string;
    redonda?: boolean;
    ancho?: number;
    etiqueta?: string;
    // Sin la placa de alrededor: el canvas llena su caja (para encastrarlo en la foto del robot).
    desnuda?: boolean;
    alError?: (e: string | null) => void;
  } = $props();

  // En las páginas de un robot, sus sprites (las caras de imagen) van al núcleo de la vista previa.
  const bicho = quizasBicho();
  let enviados = new Map<number, number>();
  let canvas = $state<HTMLCanvasElement>();
  let sim: VistaPrevia | null = null;
  let cuadro: number | null = null;
  let aplicado = '';
  let fijado = '';

  async function arrancar() {
    if (cuadro) cancelAnimationFrame(cuadro);
    sim = await vistaPrevia({ redonda, alCuadro: (rgba, w, h) => canvas && pintar(canvas, rgba, w, h) });
    aplicado = '';
    fijado = '';
    enviados = new Map();
    sincronizar();
    let ultimo = 0;
    const vuelta = (t: number) => {
      if (t - ultimo >= 33) {
        ultimo = t;
        sim?.tick();
      }
      cuadro = requestAnimationFrame(vuelta);
    };
    cuadro = requestAnimationFrame(vuelta);
  }

  function sincronizar() {
    if (!sim) return;
    if (bicho) ponerSprites(sim, $state.snapshot(bicho.sprites) as typeof bicho.sprites, enviados);
    const texto = typeof config === 'string' ? config : JSON.stringify(config);
    if (texto !== aplicado) {
      aplicado = texto;
      alError(sim.aplicar(texto));
      fijado = '';
    }
    // Los estados del sistema (un recordatorio, el pomodoro, el reloj, una imagen) no son
    // caras: la vista previa los arma sola con la misma config y la misma hora.
    const delSistema = /^(recordatorio|pomodoro|reloj|imagen|protector):/.test(visible);
    if (visible !== fijado && !delSistema) {
      fijado = visible;
      fijar(sim, visible);
    }
  }

  onMount(arrancar);
  onDestroy(() => cuadro && cancelAnimationFrame(cuadro));

  $effect(() => {
    void config;
    void visible;
    void bicho?.sprites;
    sincronizar();
  });

  let redondaAnterior = redonda;
  $effect(() => {
    if (redonda !== redondaAnterior) {
      redondaAnterior = redonda;
      if (cuadro) cancelAnimationFrame(cuadro);
      arrancar();
    }
  });
</script>

{#if desnuda}
  <canvas class="desnuda" class:redonda bind:this={canvas} width={redonda ? 240 : 320} height={redonda ? 240 : 172} aria-label={etiqueta}
    onpointerdown={() => sim?.tacto(true)} onpointerup={() => sim?.tacto(false)} onpointerleave={() => sim?.tacto(false)}></canvas>
{:else}
<div class="placa" class:redonda style="width: {ancho + (redonda ? 32 : 28)}px">
  <canvas bind:this={canvas} width={redonda ? 240 : 320} height={redonda ? 240 : 172} aria-label={etiqueta}
    onpointerdown={() => sim?.tacto(true)} onpointerup={() => sim?.tacto(false)} onpointerleave={() => sim?.tacto(false)}></canvas>
</div>
{/if}

<style>
  canvas.desnuda { display: block; width: 100%; height: 100%; object-fit: contain; background: #000; touch-action: none; }
  canvas.desnuda.redonda { border-radius: 50%; }
</style>
