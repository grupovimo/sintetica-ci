<script lang="ts" module>
  export const PRESETS_DIAL = ['picaro', 'timido', 'carinoso', 'curioso', 'dormilon', 'grunon', 'electrico', 'equilibrado'];
  export const DICE_PRESET: Record<string, string> = {
    picaro: 'Hace bromas, te guiña y busca que le prestes atención.',
    timido: 'Se esconde un poco al principio; de a poco se suelta.',
    carinoso: 'Te busca, te extraña y se alegra fuerte cuando volvés.',
    curioso: 'Mira todo e investiga cada ruido y cada movimiento.',
    dormilon: 'Siestas largas y un despertar lento.',
    grunon: 'Rezonga, pero en el fondo te quiere.',
    electrico: 'No para nunca: todo es emoción.',
    equilibrado: 'Un poco de todo, sin exagerar nada.',
  };
</script>

<script lang="ts">
  // El dial de personalidad: ocho posiciones alrededor de una perilla grande.
  import { NOMBRES_PRESET } from '$lib/formato';

  let { valor, alCambiar }: { valor: string; alCambiar: (p: string) => void } = $props();
  const i = $derived(Math.max(0, PRESETS_DIAL.indexOf(valor)));

  function tecla(e: KeyboardEvent) {
    const d = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (d) { e.preventDefault(); alCambiar(PRESETS_DIAL[(i + d + 8) % 8]); }
  }
</script>

<!-- svelte-ignore a11y_interactive_supports_focus -->
<div class="dial" role="radiogroup" aria-label="Personalidad" onkeydown={tecla}>
  <div class="perilla-grande">
    <div class="aguja" style="transform: rotate({i * 45}deg)"><span></span></div>
    <span class="centro"></span>
  </div>
  {#each PRESETS_DIAL as p, j}
    {@const a = ((-90 + j * 45) * Math.PI) / 180}
    <button role="radio" aria-checked={valor === p} tabindex={valor === p ? 0 : -1} class="opcion" class:elegida={valor === p}
      style="left: {150 + Math.cos(a) * 128}px; top: {140 + Math.sin(a) * 118}px" onclick={() => alCambiar(p)}>{NOMBRES_PRESET[p]}</button>
  {/each}
</div>

<style>
  .dial { position: relative; width: 300px; height: 280px; flex: none; }
  .perilla-grande { position: absolute; left: 75px; top: 65px; width: 150px; height: 150px; border-radius: 50%;
    background: conic-gradient(from 20deg, #fbf9f5, #d4ccbf, #fbf9f5, #cfc7b9, #fbf9f5);
    box-shadow: inset 0 2px 0 #fff, 0 9px 0 #8f887c, 0 22px 30px -10px rgba(80,20,0,.6); }
  .aguja { position: absolute; inset: 0; transition: transform .45s cubic-bezier(.3,1.5,.5,1); }
  .aguja span { position: absolute; left: 50%; top: 10px; width: 7px; height: 40px; margin-left: -3.5px; border-radius: 4px; background: #ff5a1f; }
  .centro { position: absolute; inset: 52px; border-radius: 50%; background: radial-gradient(circle at 40% 35%, #f3eee8, #bdb4a6); box-shadow: inset 0 2px 5px rgba(0,0,0,.3); }
  .opcion { position: absolute; transform: translate(-50%, -50%); border: 0; cursor: pointer; padding: 8px 10px; border-radius: 9px; background: transparent;
    font: 800 11px/1 var(--marca); letter-spacing: .08em; text-transform: uppercase; white-space: nowrap; color: rgba(26,23,20,.78); }
  .opcion:hover { background: rgba(26,23,20,.1); }
  .opcion.elegida { background: #1a1714; color: #fff; }
  @media (max-width: 360px) { .dial { transform: scale(.88); margin: -16px -18px; } }
</style>
