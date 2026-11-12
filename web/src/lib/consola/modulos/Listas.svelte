<script lang="ts">
  // Listas: la del súper, pendientes o la que sea. Son de tu cuenta (no del
  // robot): lo que anotás acá aparece en el celu, en el escritorio, en
  // Telegram y charlando ("anotá yerba en el súper"). En vivo por el evento
  // "listas".
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { escuchar } from '$lib/eventos.svelte';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();

  type Item = { id: string; texto: string; hecho: boolean; creado: string; hechoEn?: string };
  type Lista = { id: string; nombre: string; creada: string; items: Item[] };
  type Respuesta = { listas: Lista[]; porDefecto: string; topes: { listas: number; items: number; texto: number; nombre: number } };

  let datos = $state<Respuesta | null>(null);
  let elegida = $state('');
  let nuevo = $state('');
  let nombreNueva = $state('');
  let creando = $state(false);
  let error = $state('');

  const listas = $derived(datos?.listas ?? []);
  const lista = $derived(listas.find((l) => l.id === elegida) ?? listas[0]);
  const pendientes = $derived(lista?.items.filter((i) => !i.hecho) ?? []);
  const tachadas = $derived(lista?.items.filter((i) => i.hecho) ?? []);

  function fallo(e: unknown, si: string) {
    error = e instanceof ErrorApi ? e.message : si;
    setTimeout(() => (error = ''), 3000);
  }

  async function cargar() {
    try { datos = await api<Respuesta>('/v1/listas'); } catch (e) { fallo(e, 'No pude traer las listas.'); }
  }
  onMount(() => {
    cargar();
    return escuchar((e) => { if (e.tipo === 'listas') cargar(); });
  });

  // Sin listas, la primera cosa crea la del súper.
  async function listaActual(): Promise<Lista | undefined> {
    if (lista) return lista;
    const l = await api<Lista>('/v1/listas', { cuerpo: { nombre: datos?.porDefecto ?? 'súper' } });
    await cargar();
    elegida = l.id;
    return l;
  }

  async function agregar() {
    const t = nuevo.trim();
    if (!t) return;
    try {
      const l = await listaActual();
      if (!l) return;
      const r = await api<{ lista: Lista }>(`/v1/listas/${l.id}/items`, { cuerpo: { texto: t } });
      nuevo = '';
      reemplazar(r.lista);
    } catch (e) { fallo(e, 'No pude anotarlo.'); }
  }
  function reemplazar(l: Lista) {
    if (!datos) return;
    const k = datos.listas.findIndex((x) => x.id === l.id);
    if (k >= 0) datos.listas[k] = l;
  }
  async function tachar(i: Item) {
    if (!lista) return;
    i.hecho = !i.hecho; // al toque; si falla, vuelve
    try { await api(`/v1/listas/${lista.id}/items/${i.id}`, { metodo: 'PATCH', cuerpo: { hecho: i.hecho } }); }
    catch (e) { i.hecho = !i.hecho; fallo(e, 'No pude.'); }
  }
  async function sacar(i: Item) {
    if (!lista) return;
    try { await api(`/v1/listas/${lista.id}/items/${i.id}`, { metodo: 'DELETE' }); lista.items = lista.items.filter((x) => x.id !== i.id); }
    catch (e) { fallo(e, 'No pude.'); }
  }
  async function limpiar() {
    if (!lista) return;
    try { const r = await api<{ lista: Lista }>(`/v1/listas/${lista.id}/limpiar`, { cuerpo: {} }); reemplazar(r.lista); }
    catch (e) { fallo(e, 'No pude.'); }
  }
  async function crear() {
    const n = nombreNueva.trim();
    if (!n) { creando = false; return; }
    try {
      const l = await api<Lista>('/v1/listas', { cuerpo: { nombre: n } });
      nombreNueva = '';
      creando = false;
      await cargar();
      elegida = l.id;
    } catch (e) { fallo(e, 'No pude crearla.'); }
  }
  async function borrar() {
    if (!lista || !confirm(`¿Borrar «${lista.nombre}»?`)) return;
    try { await api(`/v1/listas/${lista.id}`, { metodo: 'DELETE' }); elegida = ''; await cargar(); }
    catch (e) { fallo(e, 'No pude.'); }
  }
</script>

<div class="listas">
  <div class="chips" role="tablist" aria-label="Listas">
    {#each listas as l (l.id)}
      {@const n = l.items.filter((i) => !i.hecho).length}
      <button role="tab" class="llave" aria-selected={l.id === lista?.id} onclick={() => (elegida = l.id)}>
        <span class="led"></span>{l.nombre}{#if n}<span class="n">{n}</span>{/if}
      </button>
    {/each}
    {#if creando}
      <form class="visor nueva" onsubmit={(e) => { e.preventDefault(); crear(); }}>
        <!-- svelte-ignore a11y_autofocus -->
        <input class="campo claro" aria-label="Nombre de la lista" placeholder="pendientes" maxlength={datos?.topes.nombre ?? 40} bind:value={nombreNueva} autofocus onblur={crear} />
      </form>
    {:else if listas.length < (datos?.topes.listas ?? 30)}
      <button class="llave mas" aria-label="Nueva lista" onclick={() => (creando = true)}>+</button>
    {/if}
  </div>

  <form class="anotar visor" onsubmit={(e) => { e.preventDefault(); agregar(); }}>
    <input class="campo claro" aria-label="Anotar" placeholder="yerba, pan y queso" maxlength="600" bind:value={nuevo} />
    <button class="tecla naranja chica" disabled={!nuevo.trim()}>+</button>
  </form>

  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  {#if lista}
    <ul class="items">
      {#each pendientes as i (i.id)}
        <li class="item visor">
          <button class="caja" aria-pressed="false" aria-label="Tachar {i.texto}" onclick={() => tachar(i)}></button>
          <span class="texto">{i.texto}</span>
          <button class="x" aria-label="Sacar {i.texto}" onclick={() => sacar(i)}>✕</button>
        </li>
      {/each}
      {#each tachadas as i (i.id)}
        <li class="item visor hecho">
          <button class="caja" aria-pressed="true" aria-label="Destachar {i.texto}" onclick={() => tachar(i)}>✓</button>
          <span class="texto">{i.texto}</span>
          <button class="x" aria-label="Sacar {i.texto}" onclick={() => sacar(i)}>✕</button>
        </li>
      {/each}
      {#if !lista.items.length}<li class="visor vacio"><span class="grabado">Vacía</span></li>{/if}
    </ul>
    <div class="pie">
      {#if tachadas.length}<button class="tecla negra chica" onclick={limpiar}>Limpiar tachadas ({tachadas.length})</button>{/if}
      <button class="tecla negra chica borrar" onclick={borrar}>Borrar lista</button>
    </div>
  {:else if datos}
    <div class="visor vacio"><span class="grabado">«anotá yerba en el súper»</span></div>
  {:else}
    <p class="grabado">Buscando…</p>
  {/if}
</div>

<style>
  .listas { display: flex; flex-direction: column; gap: 14px; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .chips .n { font: 900 11px/1 var(--marca); opacity: .7; font-variant-numeric: tabular-nums; }
  .mas { font: 900 20px/1 var(--marca); }
  .nueva { padding: 6px 14px; min-width: 160px; }
  .anotar { flex-direction: row; align-items: center; gap: 12px; padding: 10px 10px 10px 18px; }
  .anotar .campo { font-size: 18px; }
  .items { display: flex; flex-direction: column; gap: 6px; margin: 0; padding: 0; list-style: none; }
  .item { flex-direction: row; align-items: center; gap: 14px; padding: 10px 12px; }
  .caja { flex: none; display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; cursor: pointer; color: #ff8a5c;
    background: #0e0d0b; box-shadow: inset 0 2px 4px rgba(0,0,0,.8), 0 0 0 2px #3a3632; font: 900 16px/1 var(--marca); }
  .texto { flex: 1; min-width: 0; font: 800 17px/1.25 var(--marca); color: var(--c-papel); overflow-wrap: anywhere; }
  .hecho .texto { color: #5a524a; text-decoration: line-through; }
  .x { flex: none; width: 32px; height: 32px; border: 0; border-radius: 8px; cursor: pointer; background: none; color: #5a524a; font-size: 13px; }
  .x:hover { color: var(--c-papel); }
  .vacio { align-items: center; padding: 22px; }
  .pie { display: flex; flex-wrap: wrap; gap: 8px; justify-content: space-between; }
  .borrar { margin-left: auto; }
</style>
