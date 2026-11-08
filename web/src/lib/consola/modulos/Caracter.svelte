<script lang="ts">
  // Carácter: un dial con los 8 presets y una perilla por rasgo.
  import { usarBicho } from '$lib/bicho.svelte';
  import { NOMBRES_PRESET, NOMBRES_RASGO, PRESETS, RASGOS } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';
  import Dial, { DICE_PRESET } from '../Dial.svelte';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const preset = $derived(b.config.personalidad?.preset ?? 'equilibrado');
  const rasgo = (r: string) =>
    (b.config.personalidad?.[r as keyof typeof PRESETS.equilibrado] as number | undefined) ??
    (PRESETS[preset]?.[r as keyof typeof PRESETS.equilibrado] as number) ?? 0.5;
  const tocado = $derived(RASGOS.some((r) => Math.abs(rasgo(r) - ((PRESETS[preset]?.[r] as number) ?? 0.5)) > 0.001));

  const ponerPreset = (p: string) => g.cambiar((c) => (c.personalidad = { preset: p, ...(PRESETS[p] ?? {}) }));
  const ponerRasgo = (r: string, v: number) => g.cambiar((c) => (c.personalidad = { ...(c.personalidad ?? {}), [r]: v }));
</script>

<div class="caracter">
  <div class="dial-caja">
    <Dial valor={preset} alCambiar={ponerPreset} />
    <div class="dice">
      <span class="grabado">Carácter{tocado ? ' · a tu medida' : ''}</span>
      <b>{NOMBRES_PRESET[preset]}</b>
      <p class="texto">{DICE_PRESET[preset]}</p>
    </div>
  </div>
  <div class="bandeja perillas">
    {#each RASGOS as r}
      <div title="{NOMBRES_RASGO[r][0]}: alto = {NOMBRES_RASGO[r][1].toLowerCase()}; bajo = {NOMBRES_RASGO[r][2].toLowerCase()}">
        <Perilla nombre={NOMBRES_RASGO[r][0]} valor={rasgo(r)} alCambiar={(v) => ponerRasgo(r, v)} />
      </div>
    {/each}
  </div>
</div>

<style>
  .caracter { display: flex; flex-direction: column; gap: 22px; }
  .dial-caja { display: grid; grid-template-columns: 300px minmax(0, 1fr); gap: 28px; align-items: center; }
  .dice { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .dice b { font: 900 clamp(34px, 4vw, 52px)/.95 var(--marca); font-stretch: 120%; letter-spacing: -.04em; text-transform: uppercase; overflow-wrap: anywhere; }
  .perillas { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 12px; padding: 22px 12px 16px; }
  @media (max-width: 760px) {
    .dial-caja { grid-template-columns: 1fr; justify-items: center; text-align: center; }
    .dice { align-items: center; }
    .perillas { grid-template-columns: repeat(3, minmax(0, 1fr)); row-gap: 22px; }
  }
</style>
