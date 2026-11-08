<script lang="ts">
  // La biblioteca de caras o de animaciones de la cuenta: para usarlas en
  // cualquier bicho. Las del manual y las de ejemplo sirven de punto de partida.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { formatoBase } from '$lib/base';
  import { clonar, nombreValido, type Animacion, type Config, type Expresion } from '$lib/formato';
  import type { Bicho, Pieza } from '$lib/tipos';
  import { BASE } from '$lib/bicho.svelte';
  import EditorCara from './EditorCara.svelte';
  import EditorAnimacion from './EditorAnimacion.svelte';
  import Miniatura from './Miniatura.svelte';

  let { tipo }: { tipo: 'cara' | 'animacion' } = $props();
  const plural = tipo === 'cara' ? 'caras' : 'animaciones';

  let piezas = $state<Pieza[]>([]);
  let modelos = $state<{ nombre: string; documento: any }[]>([]);
  let bichos = $state<Bicho[]>([]);
  let editando = $state<{ id?: string; nombre: string; documento: any } | null>(null);
  let error = $state('');
  let aviso = $state('');
  let destino = $state('');

  async function cargar() {
    piezas = (await api<{ piezas: Pieza[] }>(`/v1/biblioteca?tipo=${tipo}`)).piezas;
  }
  onMount(async () => {
    await cargar();
    const f = await formatoBase();
    modelos = tipo === 'cara' ? f.caras : f.animaciones;
    bichos = (await api<{ bichos: Bicho[] }>('/v1/bichos')).bichos;
    destino = bichos[0]?.id ?? '';
  });

  function nueva(desde: { nombre: string; documento: any }) {
    let n = desde.nombre, k = 2;
    while (piezas.some((p) => p.nombre === n)) n = `${desde.nombre}-${k++}`;
    editando = { nombre: n, documento: clonar(desde.documento) };
  }

  async function guardar() {
    if (!editando) return;
    try {
      await api('/v1/biblioteca', { cuerpo: { tipo, nombre: editando.nombre, documento: $state.snapshot(editando.documento) } });
      aviso = `"${editando.nombre}" guardada.`;
      editando = null;
      await cargar();
    } catch (e) {
      error = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude guardarla.';
    }
  }

  async function borrar(p: Pieza) {
    if (!confirm(`¿Borrar "${p.nombre}" de tu biblioteca? Los robots que ya la tienen la conservan.`)) return;
    await api(`/v1/biblioteca/${p.id}`, { metodo: 'DELETE' });
    await cargar();
  }

  async function mandar(p: Pieza) {
    if (!destino) return;
    aviso = error = '';
    try {
      const { config } = await api<{ config: Config }>(`/v1/bichos/${destino}/config`);
      const c = clonar(config);
      if (tipo === 'cara') c.expresiones = { ...c.expresiones, [p.nombre]: p.documento };
      else c.animaciones = { ...c.animaciones, [p.nombre]: p.documento };
      await api(`/v1/bichos/${destino}/config`, { metodo: 'PUT', cuerpo: c });
      aviso = `"${p.nombre}" está en ${bichos.find((b) => b.id === destino)?.apodo || bichos.find((b) => b.id === destino)?.estado?.nombre || destino}.`;
    } catch (e) {
      error = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude mandarla.';
    }
  }
  let errorEditor = $state('');
</script>

{#if editando}
  <div class="grilla">
    <div class="fila" style="justify-content: space-between">
      <h1>{editando.nombre}</h1>
      <div class="fila"><button onclick={() => (editando = null)}>Cancelar</button>
        <button class="primario" disabled={!!errorEditor || !nombreValido(editando.nombre)} onclick={guardar}>Guardar en mi biblioteca</button></div>
    </div>
    {#if error}<p class="aviso-caja error">{error}</p>{/if}
    {#if tipo === 'cara'}
      <EditorCara bind:expresion={editando.documento as Expresion} bind:nombre={editando.nombre} bind:error={errorEditor} />
    {:else}
      <EditorAnimacion bind:animacion={editando.documento as Animacion} bind:nombre={editando.nombre} caras={BASE} bind:error={errorEditor} />
    {/if}
  </div>
{:else}
  <div class="grilla">
    <div><p class="etiqueta">Tu biblioteca</p><h1>{tipo === 'cara' ? 'Caras' : 'Animaciones'}</h1></div>
    {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
    {#if error}<p class="aviso-caja error">{error}</p>{/if}
    {#if piezas.length}
      {#if bichos.length}
        <label class="fila"><span class="etiqueta">Mandar a</span>
          <select bind:value={destino} style="width: auto">{#each bichos as b}<option value={b.id}>{b.apodo || b.estado?.nombre || b.id}</option>{/each}</select></label>
      {/if}
      <div class="lista">
        {#each piezas as p}
          <div class="item tarjeta">
            <button class="mini" onclick={() => (editando = { id: p.id, nombre: p.nombre, documento: clonar(p.documento) })} aria-label="Editar {p.nombre}">
              {#if tipo === 'cara'}<Miniatura expresion={p.documento} ancho={210} />{:else}<Miniatura animacion={p.documento} ancho={210} />{/if}
            </button>
            <div class="fila" style="justify-content: space-between">
              <span class="mono">{p.nombre}</span>
              <span class="fila">{#if bichos.length}<button class="chico" onclick={() => mandar(p)}>Al robot</button>{/if}<button class="chico peligro" onclick={() => borrar(p)}>Borrar</button></span>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <p class="nota">Todavía no guardaste {plural}. Empezá desde una de abajo.</p>
    {/if}
    <h2>Para empezar</h2>
    <div class="lista">
      {#each modelos as m}
        <div class="item tarjeta">
          {#if tipo === 'cara'}<Miniatura expresion={m.documento} ancho={210} />{:else}<Miniatura animacion={m.documento} ancho={210} />{/if}
          <div class="fila" style="justify-content: space-between"><span class="mono">{m.nombre}</span><button class="chico" onclick={() => nueva(m)}>Partir de esta</button></div>
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .lista { display: grid; gap: 14px; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); }
  .item { padding: 12px; display: grid; gap: 10px; box-shadow: none; }
  .mini { padding: 0; border: 0; background: none; justify-self: center; }
</style>
