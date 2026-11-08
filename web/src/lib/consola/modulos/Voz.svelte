<script lang="ts">
  // Voz: para los modelos con micrófono y parlante. Cómo suena, cuánto, a qué
  // nombre responde y si conversa.
  import { usarBicho } from '$lib/bicho.svelte';
  import { cortarBytes, MAX_PALABRA } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const habla = $derived(modelo.capacidades.has('parlante'));
  const escucha = $derived(modelo.capacidades.has('microfono'));
  const v = $derived(b.config.voz ?? {});
  const TIPOS: [NonNullable<NonNullable<typeof b.config.voz>['tipo']>, string][] = [['grave', 'Grave'], ['media', 'Media'], ['aguda', 'Aguda'], ['robot', 'Robot']];
  const nombre = $derived(b.bicho?.apodo || b.config.nombre || 'tu robot');

  const poner = (f: (x: NonNullable<typeof b.config.voz>) => void) => g.cambiar((c) => { const x = { ...(c.voz ?? {}) }; f(x); c.voz = x; });
</script>

{#if !habla && !escucha}
  <div class="bloqueado bandeja">
    <svg width="44" height="44" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" /></svg>
    <div>
      <b>Sin micrófono ni parlante</b>
      <p class="texto">Llega con los modelos que hablan.</p>
    </div>
  </div>
{:else}
  <div class="voz">
    <div class="bandeja perillas">
      {#if habla}<Perilla nombre="Volumen" valor={v.volumen ?? 0.7} alCambiar={(x) => poner((y) => (y.volumen = x))} />{/if}
      {#if escucha}<Perilla nombre="Oído" valor={v.microfono ?? 0.6} alCambiar={(x) => poner((y) => (y.microfono = x))} />{/if}
      {#if habla}
        <div class="tipos">
          <span class="grabado">Su voz</span>
          <div class="teclas" role="group" aria-label="Su voz">
            {#each TIPOS as [k, t]}<button class="tecla chica" aria-pressed={(v.tipo ?? 'media') === k} onclick={() => poner((y) => (y.tipo = k))}>{t}</button>{/each}
          </div>
        </div>
      {/if}
    </div>
    {#if escucha}
      <div class="fila">
        <label class="visor">
          <span class="grabado">Responde cuando le dicen <span class="pista">· una palabra común</span></span>
          <input class="campo" maxlength="16" placeholder={nombre} value={v.palabra ?? ''}
            onchange={(e) => { const t = cortarBytes(e.currentTarget.value.trim(), MAX_PALABRA); poner((y) => { if (t) y.palabra = t; else delete y.palabra; }); }} />
        </label>
        <button class="llave grande" aria-pressed={!!v.conversar} onclick={() => poner((y) => (y.conversar = !y.conversar))}>
          <span class="led"></span><span>Charlar<small>Después de su nombre</small></span>
        </button>
      </div>
      <p class="texto">“{v.palabra || nombre}, ¿qué hora es?”</p>
    {/if}
  </div>
{/if}

<style>
  .bloqueado { display: flex; gap: 20px; align-items: center; color: rgba(26,23,20,.75); }
  .bloqueado b { font: 900 20px/1.1 var(--marca); text-transform: uppercase; color: #1a1714; }
  .voz { display: flex; flex-direction: column; gap: 16px; }
  .perillas { display: flex; flex-wrap: wrap; align-items: center; gap: 28px; padding: 22px; }
  .tipos { display: flex; flex-direction: column; gap: 12px; }
  .teclas { display: flex; flex-wrap: wrap; gap: 10px 8px; padding-bottom: 6px; }
  .fila { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 16px; }
  .grande { min-height: 100%; padding: 0 20px; text-align: left; }
  .grande span:last-child { display: flex; flex-direction: column; gap: 6px; }
  .grande small { font: 600 12px/1.2 var(--marca); letter-spacing: 0; text-transform: none; color: #8f877c; }
  .pista { color: var(--brasa); letter-spacing: .08em; }
  @media (max-width: 640px) { .fila { grid-template-columns: 1fr; } .grande { min-height: 64px; } }
</style>
