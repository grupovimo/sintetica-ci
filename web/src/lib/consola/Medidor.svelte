<script lang="ts">
  // Un medidor de LEDs: cuánto se usó de un tope (hoy). Sin tope, solo el número.
  let { nombre, valor, de = 0, mostrar = String }: { nombre: string; valor: number; de?: number; mostrar?: (n: number) => string } = $props();
  const SEGMENTOS = 12;
  const lleno = $derived(de > 0 ? Math.min(1, valor / de) : 0);
</script>

<div class="medidor visor" role="meter" aria-label={nombre} aria-valuemin={0} aria-valuemax={de || undefined} aria-valuenow={valor}>
  <span class="grabado">{nombre}</span>
  <div class="barra" aria-hidden="true">
    {#each Array.from({ length: SEGMENTOS }, (_, i) => i) as i}
      <i class:on={de > 0 && i < Math.ceil(lleno * SEGMENTOS)} class:alto={i >= SEGMENTOS - 2}></i>
    {/each}
  </div>
  <span class="cifra"><b>{mostrar(valor)}</b>{#if de > 0}<span>/{mostrar(de)}</span>{:else}<span>/∞</span>{/if}</span>
</div>

<style>
  .medidor { gap: 8px; padding: 12px 14px; }
  .barra { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); gap: 3px; }
  .barra i { height: 8px; border-radius: 2px; background: #2c2925; }
  .barra i.on { background: var(--verde); box-shadow: 0 0 6px rgba(74,222,128,.6); }
  .barra i.on.alto { background: var(--led); box-shadow: 0 0 6px var(--led); }
  .cifra { display: flex; align-items: baseline; gap: 2px; font: 900 18px/1 var(--marca); font-variant-numeric: tabular-nums; color: var(--brasa); }
  .cifra span { font-size: 12px; color: var(--gris); }
</style>
