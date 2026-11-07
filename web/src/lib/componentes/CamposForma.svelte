<script lang="ts">
  // Las medidas de una forma, como campos.
  import { CAMPOS, COLORES, HEX_COLOR, type Forma } from '$lib/formato';

  let { forma, alCambiar, sinFijo = false }: { forma: Forma; alCambiar: (f: Forma) => void; sinFijo?: boolean } = $props();

  function poner<K extends keyof Forma>(k: K, v: Forma[K] | undefined) {
    const f = { ...forma };
    if (v === undefined || (typeof v === 'number' && Number.isNaN(v))) delete f[k];
    else f[k] = v;
    alCambiar(f);
  }
  const esHex = $derived(!!forma.color?.startsWith('#'));
</script>

<div class="campos">
  {#each CAMPOS[forma.forma] as [k, nombre, min, max, paso]}
    {#if !(k === 'r' && forma.forma === 'pildora' && forma[k] === undefined) && !(k === 'trazo' && forma.forma === 'pildora' && forma.relleno !== false)}
      <label><span class="etiqueta">{nombre}</span>
        <input type="number" {min} {max} step={paso} value={forma[k] as number} oninput={(e) => poner(k, e.currentTarget.value === '' ? undefined : +e.currentTarget.value)} />
      </label>
    {/if}
  {/each}
  {#if forma.forma === 'pildora'}
    <label class="fila"><input type="checkbox" checked={forma.r !== undefined} onchange={(e) => poner('r', e.currentTarget.checked ? 0.05 : undefined)} /> <span class="etiqueta">radio propio</span></label>
    <label class="fila"><input type="checkbox" checked={forma.relleno !== false} onchange={(e) => alCambiar(e.currentTarget.checked ? (({ relleno: _, trazo: __, ...f }) => f)(forma) : { ...forma, relleno: false, trazo: forma.trazo ?? 0.02 })} /> <span class="etiqueta">relleno</span></label>
  {/if}
  <label><span class="etiqueta">Color</span>
    <select value={esHex ? 'propio' : (forma.color ?? 'naranja')} onchange={(e) => poner('color', e.currentTarget.value === 'propio' ? '#FFFFFF' : e.currentTarget.value === 'naranja' ? undefined : e.currentTarget.value)}>
      {#each COLORES as c}<option value={c}>{c === 'naranja' ? 'naranja (el color del dueño)' : c}</option>{/each}
      <option value="propio">otro…</option>
    </select>
  </label>
  {#if esHex}
    <label><span class="etiqueta">Hex</span><input type="color" value={forma.color} oninput={(e) => poner('color', e.currentTarget.value.toUpperCase())} /></label>
  {:else}
    <span class="muestra" style="background: {HEX_COLOR[forma.color ?? 'naranja']}" aria-hidden="true"></span>
  {/if}
  {#if !sinFijo}
    <label class="fila"><input type="checkbox" checked={!!forma.fijo} onchange={(e) => poner('fijo', e.currentTarget.checked || undefined)} />
      <span class="etiqueta">fijo (un indicador: no parpadea ni mira)</span></label>
  {/if}
</div>

<style>
  .campos { grid-template-columns: repeat(auto-fill, minmax(110px, 1fr)); align-items: end; }
  .muestra { width: 40px; height: 40px; border-radius: 10px; border: 1px solid var(--linea); }
</style>
