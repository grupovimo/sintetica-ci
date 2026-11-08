<script lang="ts">
  // Reflejos: un patchbay. A la izquierda lo que le pasa, a la derecha cómo
  // reacciona; un cable los une. Se guardan como reglas propias simples
  // ({si, entonces: ["reflejo:x"]}), que se suman a las de fábrica. Las reglas
  // más complejas se editan en el Laboratorio y acá no se tocan.
  import { usarBicho } from '$lib/bicho.svelte';
  import { NOMBRES_EVENTO, type Regla } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const ENTRADAS = ['tacto', 'caricia', 'doble-toque', 'muchos-toques', 'levantado', 'sacudida-fuerte', 'boca-abajo', 'caida', 'volvio'];
  const SALIDAS: [string, string][] = [
    ['contento', 'Contento'], ['enamorado', 'Enamorado'], ['risa', 'Se ríe'], ['guino', 'Guiña'], ['sorpresa', 'Sorpresa'],
    ['verguenza', 'Vergüenza'], ['mareado', 'Mareado'], ['enojado', 'Se enoja'], ['ofendido', 'Ofendido'],
  ];
  const FILA = 52;
  const ALTO = ENTRADAS.length * FILA;

  const simple = (r: Regla) => ENTRADAS.includes(r.si) && !r.y && r.entonces.length === 1 && r.entonces[0].startsWith('reflejo:') &&
    SALIDAS.some(([s]) => `reflejo:${s}` === r.entonces[0]);
  const cables = $derived(
    Object.fromEntries((b.config.reglas ?? []).filter(simple).map((r) => [r.si, r.entonces[0].slice(8)])) as Record<string, string>,
  );
  const sinMovimiento = $derived(!modelo.capacidades.has('movimiento'));
  let elegida = $state<string | null>(null);

  function conectar(entrada: string, salida: string | null) {
    g.cambiar((c) => {
      const otras = (c.reglas ?? []).filter((r) => !(simple(r) && r.si === entrada));
      c.reglas = salida ? [...otras, { si: entrada, entonces: [`reflejo:${salida}`] }].slice(0, 24) : otras;
      if (!c.reglas.length) delete c.reglas;
    });
    elegida = null;
  }
  function tocarSalida(s: string) {
    if (!elegida) return;
    conectar(elegida, cables[elegida] === s ? null : s);
  }
  const y = (i: number) => i * FILA + FILA / 2;
  const necesitaMovimiento = (e: string) => ['levantado', 'sacudida-fuerte', 'boca-abajo', 'caida'].includes(e);
</script>

<div class="reflejos">
  <span class="grabado">{elegida ? `${NOMBRES_EVENTO[elegida]} → ¿qué hace?` : 'Uní un punto con otro'}</span>
  <div class="bahia">
    <div class="columna" role="listbox" aria-label="Cuando…">
      {#each ENTRADAS as e}
        {@const off = sinMovimiento && necesitaMovimiento(e)}
        <button class="jack izq" role="option" aria-selected={elegida === e} class:elegida={elegida === e} class:conectada={!!cables[e]} disabled={off}
          onclick={() => (elegida = elegida === e ? null : e)}>
          <span class="nombre">{NOMBRES_EVENTO[e]}</span><span class="toma"></span>
        </button>
      {/each}
    </div>
    <svg class="cables" viewBox="0 0 100 {ALTO}" preserveAspectRatio="none" aria-hidden="true" style="height: {ALTO}px">
      {#each ENTRADAS as e, i}
        {#if cables[e]}
          {@const j = SALIDAS.findIndex(([s]) => s === cables[e])}
          <path d="M0 {y(i)} C 45 {y(i) + 26}, 55 {y(j) + 26}, 100 {y(j)}" class:viva={elegida === e} vector-effect="non-scaling-stroke" />
        {/if}
      {/each}
    </svg>
    <div class="columna" role="listbox" aria-label="…reacciona">
      {#each SALIDAS as [s, t]}
        <button class="jack der" role="option" aria-selected={!!elegida && cables[elegida] === s} class:conectada={Object.values(cables).includes(s)}
          disabled={!elegida} onclick={() => tocarSalida(s)}>
          <span class="toma"></span><span class="nombre">{t}</span>
        </button>
      {/each}
    </div>
  </div>
  {#if sinMovimiento}<p class="texto">Este modelo no siente el movimiento.</p>{/if}
</div>

<style>
  .reflejos { display: flex; flex-direction: column; gap: 14px; }
  .bahia { display: grid; grid-template-columns: minmax(0, 1fr) minmax(60px, 1fr) minmax(0, 1fr); align-items: start; padding: 16px;
    border-radius: 22px; background: #1c1a17; box-shadow: inset 0 3px 12px rgba(0,0,0,.8); }
  .columna { display: flex; flex-direction: column; }
  .jack { display: flex; align-items: center; gap: 10px; height: 52px; padding: 0 4px; border: 0; background: none; cursor: pointer;
    color: #b9b1a6; font: 700 14px/1.15 var(--marca); text-align: left; }
  .jack.izq { justify-content: flex-end; text-align: right; }
  .jack:disabled { cursor: default; }
  .jack.izq:disabled { opacity: .35; }
  .jack:not(:disabled):hover .nombre, .jack.elegida .nombre, .jack.conectada .nombre { color: #f3eee8; }
  .nombre { min-width: 0; }
  .toma { flex: none; width: 26px; height: 26px; border-radius: 50%; background: radial-gradient(circle, #050505 34%, #6b645b 36%, #3a3632 62%, #24211e 64%);
    box-shadow: 0 2px 0 rgba(255,255,255,.08), inset 0 1px 2px rgba(0,0,0,.8); transition: box-shadow .15s; }
  .conectada .toma { background: radial-gradient(circle, #ff8a5c 30%, #6b645b 34%, #3a3632 62%, #24211e 64%); }
  .elegida .toma { box-shadow: 0 0 0 3px #ff8a5c, 0 0 16px #ff6a33; }
  .jack.der:not(:disabled) .toma { box-shadow: 0 0 0 2px rgba(255,138,92,.35); }
  .cables { width: 100%; overflow: visible; }
  .cables path { fill: none; stroke: #ff6a33; stroke-width: 5; stroke-linecap: round; filter: drop-shadow(0 4px 3px rgba(0,0,0,.6)); }
  .cables path.viva { stroke: #ffb347; }
  @media (max-width: 560px) { .jack { font-size: 12px; } .bahia { padding: 10px 6px; grid-template-columns: minmax(0, 1fr) 36px minmax(0, 1fr); } }
</style>
