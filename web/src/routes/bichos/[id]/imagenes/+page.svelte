<script lang="ts">
  import { onMount } from 'svelte';
  import { api, conBinario, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { animar, cuadrosAUrls, prepararImagen } from '$lib/imagenes';
  import { pantallaDe, type Imagen } from '$lib/tipos';

  const b = usarBicho();
  let imagenes = $state<Imagen[]>([]);
  let vistas = $state<Record<number, { url: string; ms: number }[]>>({});
  let subiendo = $state(-1);
  let aviso = $state('');
  let error = $state('');
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });

  async function cargar() {
    const r = await api<{ imagenes: Imagen[] }>(`/v1/bichos/${b.id}/imagenes`);
    imagenes = r.imagenes;
    for (const i of imagenes) {
      if (vistas[i.espacio + i.crc * 8]) continue;
      const resp = await fetch(`/api/v1/bichos/${b.id}/imagenes/${i.espacio}`);
      if (resp.ok) vistas[i.espacio + i.crc * 8] = cuadrosAUrls(new Uint8Array(await resp.arrayBuffer()));
    }
  }
  onMount(() => {
    cargar();
    return escuchar((e) => e.bicho === b.id && e.tipo === 'imagenes' && cargar());
  });

  const de = (e: number) => imagenes.find((i) => i.espacio === e);

  async function subir(espacio: number, archivo: File | undefined) {
    if (!archivo) return;
    subiendo = espacio;
    error = aviso = '';
    try {
      const { qoi, cuadros } = await prepararImagen(archivo, pantalla.ancho, pantalla.alto, pantalla.redonda);
      await conBinario(`/v1/bichos/${b.id}/imagenes/${espacio}`, 'PUT', qoi);
      aviso = `Guardada en el espacio ${espacio}${cuadros > 1 ? `, animada (${cuadros} cuadros)` : ''} (${(qoi.length / 1024).toFixed(0)} KB). Para que se vea, ponela en la lista (Config).`;
      await cargar();
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : e instanceof Error && e.message.startsWith('El GIF') ? e.message : 'No pude prepararla: ¿es una imagen o un GIF?';
    } finally {
      subiendo = -1;
    }
  }

  async function borrar(espacio: number) {
    await api(`/v1/bichos/${b.id}/imagenes/${espacio}`, { metodo: 'DELETE' });
    await cargar();
  }

  const enLista = (e: number) => (b.config.lista ?? []).includes(`imagen:${e}`);
</script>

<div class="grilla">
  <p class="nota" style="margin: 0">Hasta 8 imágenes o GIFs animados, del tamaño exacto de la pantalla ({pantalla.ancho} × {pantalla.alto}{pantalla.redonda ? ', redonda' : ''}). Se recortan solas. Mientras se ve una imagen, cada tanto vuelve la cara un par de segundos. Estado: {b.bicho?.sincro?.imagenes ?? '—'}.</p>
  {#if error}<p class="aviso-caja error">{error}</p>{/if}
  {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
  <div class="espacios">
    {#each Array.from({ length: 8 }, (_, i) => i) as e}
      {@const img = de(e)}
      <div class="espacio tarjeta">
        <div class="marco" class:redonda={pantalla.redonda} style="aspect-ratio: {pantalla.ancho} / {pantalla.alto}">
          {#if img && vistas[e + img.crc * 8]}<img use:animar={vistas[e + img.crc * 8]} alt="Imagen del espacio {e}" />{#if vistas[e + img.crc * 8].length > 1}<span class="gif">GIF</span>{/if}
          {:else if img}<span class="etiqueta">cargando…</span>
          {:else}<span class="etiqueta">vacío</span>{/if}
        </div>
        <div class="fila" style="justify-content: space-between">
          <span class="mono">{e}{img ? ` · ${(img.largo / 1024).toFixed(0)} KB` : ''}{enLista(e) ? ' · en la lista' : ''}</span>
          <span class="fila">
            <label class="boton chico">{subiendo === e ? 'Subiendo…' : img ? 'Cambiar' : 'Subir'}
              <input class="solo-lectores" type="file" accept="image/*,.gif" disabled={subiendo >= 0} onchange={(ev) => subir(e, ev.currentTarget.files?.[0])} />
            </label>
            {#if img}<button class="chico peligro" onclick={() => borrar(e)}>Borrar</button>{/if}
          </span>
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  .espacios { display: grid; gap: 14px; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); }
  .espacio { padding: 12px; display: grid; gap: 10px; box-shadow: none; }
  .marco { position: relative; background: #0b0a09; border-radius: 8px; display: grid; place-items: center; overflow: hidden; }
  .gif { position: absolute; right: 8px; bottom: 8px; padding: 3px 8px; border-radius: 999px; background: #fff; color: #111; font: 700 11px/1.2 var(--mono); }
  .marco.redonda { border-radius: 50%; max-width: 200px; justify-self: center; width: 100%; }
  .marco img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .marco .etiqueta { color: #77716a; }
</style>
