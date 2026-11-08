<script lang="ts">
  // Imágenes: ocho bahías para fotos o GIFs. Cada una se puede sumar a lo que
  // muestra (la lista: un toque en el robot pasa a la siguiente).
  import { onMount } from 'svelte';
  import { api, conBinario, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { animar, cuadrosAUrls, prepararImagen } from '$lib/imagenes';
  import { pantallaDe, type Imagen } from '$lib/tipos';
  import type { Modelo } from '../modelo';
  import { usarGuardado } from '../guardado.svelte';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();
  let imagenes = $state<Imagen[]>([]);
  let vistas = $state<Record<number, { url: string; ms: number }[]>>({});
  let subiendo = $state(-1);
  let error = $state('');
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });

  async function cargar() {
    try {
      imagenes = (await api<{ imagenes: Imagen[] }>(`/v1/bichos/${b.id}/imagenes`)).imagenes;
    } catch {
      return;
    }
    for (const i of imagenes) {
      if (vistas[i.espacio + i.crc * 8]) continue;
      const r = await fetch(`/api/v1/bichos/${b.id}/imagenes/${i.espacio}`);
      if (r.ok) vistas[i.espacio + i.crc * 8] = cuadrosAUrls(new Uint8Array(await r.arrayBuffer()));
    }
  }
  onMount(() => {
    cargar();
    return escuchar((e) => e.bicho === b.id && e.tipo === 'imagenes' && cargar());
  });

  const de = (e: number) => imagenes.find((i) => i.espacio === e);
  const enLista = (e: number) => (b.config.lista ?? []).includes(`imagen:${e}`);

  async function subir(espacio: number, archivo: File | undefined) {
    if (!archivo) return;
    subiendo = espacio;
    error = '';
    try {
      const { qoi } = await prepararImagen(archivo, pantalla.ancho, pantalla.alto, pantalla.redonda);
      await conBinario(`/v1/bichos/${b.id}/imagenes/${espacio}`, 'PUT', qoi);
      if (!enLista(espacio)) mostrar(espacio, true);
      await cargar();
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : e instanceof Error && e.message.startsWith('El GIF') ? e.message : 'No pude prepararla: ¿es una imagen o un GIF?';
    } finally {
      subiendo = -1;
    }
  }

  async function borrar(espacio: number) {
    await api(`/v1/bichos/${b.id}/imagenes/${espacio}`, { metodo: 'DELETE' }).catch(() => {});
    if (enLista(espacio)) mostrar(espacio, false);
    if (protector?.imagen === espacio) ponerProtector(null);
    await cargar();
  }

  // El protector de pantalla: una bahía que se ve cuando no pasa nada.
  const protector = $derived(b.config.protector);
  const espera = $derived(protector?.espera ?? 10);
  const ESPERAS = [3, 5, 10, 30, 60, 300, 900, 3600];
  const textoEspera = (s: number) => (s < 60 ? `${s} s` : `${s / 60} min`);
  function ponerProtector(espacio: number | null) {
    g.cambiar((c) => {
      if (espacio == null) delete c.protector;
      else c.protector = { imagen: espacio, ...(c.protector?.espera ? { espera: c.protector.espera } : {}) };
    });
  }
  function moverEspera(d: number) {
    const i = Math.max(0, ESPERAS.findIndex((x) => x >= espera));
    const n = ESPERAS[Math.min(ESPERAS.length - 1, Math.max(0, i + d))];
    g.cambiar((c) => { if (c.protector) c.protector = { ...c.protector, espera: n }; });
  }

  // Sumarla a lo que muestra: si la lista estaba vacía, primero va su cara.
  function mostrar(espacio: number, si: boolean) {
    g.cambiar((c) => {
      const l = c.lista?.length ? [...c.lista] : ['cara:neutral'];
      const k = `imagen:${espacio}`;
      c.lista = si ? (l.includes(k) ? l : [...l, k].slice(0, 16)) : l.filter((x) => x !== k);
      if (c.lista.length === 1 && c.lista[0] === 'cara:neutral') delete c.lista;
    });
  }
</script>

<div class="imagenes">
  <div class="protector bandeja" class:prendido={!!protector}>
    <div class="que">
      <span class="grabado">Protector de pantalla</span>
      <b>{protector ? `Bahía ${String(protector.imagen + 1).padStart(2, '0')}` : 'Apagado'}</b>
      <p class="texto">{protector ? 'Lo tocás y vuelve su cara.' : 'Tocá «Protector» en una bahía.'}</p>
    </div>
    {#if protector}
      <div class="visor espera">
        <span class="grabado">Aparece tras</span>
        <div class="fila-espera">
          <button class="tecla negra cuadrada" aria-label="Esperar menos" onclick={() => moverEspera(-1)}>−</button>
          <span class="digitos chicos">{textoEspera(espera)}</span>
          <button class="tecla negra cuadrada" aria-label="Esperar más" onclick={() => moverEspera(1)}>+</button>
        </div>
      </div>
      <button class="tecla negra chica" onclick={() => ponerProtector(null)}>Apagar</button>
    {/if}
  </div>
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
  <div class="bahias">
    {#each Array.from({ length: 8 }, (_, i) => i) as e}
      {@const img = de(e)}
      {@const vista = img ? vistas[e + img.crc * 8] : undefined}
      <div class="bahia">
        <label class="hueco" class:redonda={modelo.redonda} class:llena={!!img}>
          <span class="grabado num">Bahía {String(e + 1).padStart(2, '0')}</span>
          {#if vista}<img use:animar={vista} alt="Imagen de la bahía {e + 1}" />{#if vista.length > 1}<span class="gif">GIF</span>{/if}
          {:else if subiendo === e}<span class="grabado">Subiendo…</span>
          {:else if img}<span class="grabado">Cargando…</span>
          {:else}<span class="mas" aria-hidden="true">+</span><span class="solo-lectores">Subir una imagen a la bahía {e + 1}</span>{/if}
          <input class="solo-lectores" type="file" accept="image/*,.gif,video/*" disabled={subiendo >= 0} onchange={(ev) => subir(e, ev.currentTarget.files?.[0])} />
        </label>
        {#if img}
          <div class="botones">
            <button class="llave" aria-pressed={enLista(e)} onclick={() => mostrar(e, !enLista(e))}><span class="led"></span>En la lista</button>
            <button class="llave" aria-pressed={protector?.imagen === e} title="Usarla de protector de pantalla" onclick={() => ponerProtector(protector?.imagen === e ? null : e)}><span class="led"></span>Protector</button>
            <button class="tecla negra chica" aria-label="Borrar la imagen de la bahía {e + 1}" onclick={() => borrar(e)}>✕</button>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .imagenes { display: flex; flex-direction: column; gap: 16px; }
  .bahias { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; }
  .bahia { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .hueco { position: relative; display: grid; place-items: center; aspect-ratio: 4 / 3; border-radius: 16px; overflow: hidden; cursor: pointer;
    background: #0e0d0b; box-shadow: inset 0 3px 12px rgba(0,0,0,.9); transition: box-shadow .15s; }
  .hueco:hover, .hueco:focus-within { box-shadow: inset 0 3px 12px rgba(0,0,0,.9), 0 0 0 2px #1a1714; }
  .hueco img { width: 100%; height: 100%; object-fit: contain; }
  .hueco.redonda img { width: auto; height: 86%; aspect-ratio: 1; object-fit: cover; border-radius: 50%; }
  .num { position: absolute; left: 10px; top: 10px; font-size: 9px; z-index: 1; }
  .llena .num { background: rgba(14,13,11,.75); padding: 4px 6px; border-radius: 6px; }
  .mas { display: grid; place-items: center; width: 40px; height: 40px; border-radius: 50%; background: linear-gradient(180deg, #ff8a5c, #f2541a); color: #1a1714;
    font: 700 24px/1 var(--marca); box-shadow: 0 4px 0 #9a3209; }
  .gif { position: absolute; right: 8px; bottom: 8px; padding: 4px 6px; border-radius: 6px; background: #ff5a1f; color: #1a1714; font: 900 10px/1 var(--marca); letter-spacing: .08em; }
  .botones { display: grid; grid-template-columns: 1fr 1fr auto; gap: 6px; }
  .botones .llave { min-width: 0; padding: 0 8px; gap: 6px; font-size: 9.5px; letter-spacing: .06em; }
  .protector { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 16px; align-items: center; }
  .protector .que { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .protector b { font: 900 22px/1 var(--marca); font-stretch: 112%; text-transform: uppercase; }
  .espera { padding: 12px 14px; gap: 8px; }
  .fila-espera { display: flex; align-items: center; gap: 12px; }
  .fila-espera .digitos { min-width: 72px; text-align: center; }
  @media (max-width: 760px) { .protector { grid-template-columns: 1fr; } }
  @media (max-width: 760px) { .bahias { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
