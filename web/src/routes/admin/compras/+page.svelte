<script lang="ts">
  // Compras: lo pedido a cada proveedor. Al llegar suma al stock.
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { api, ErrorApi } from '$lib/api';
  import { enPesos, fecha, hoy, plata, type Compra, type Componente, type Moneda } from '$lib/control/control';

  let { data } = $props();
  let compras = $state<Compra[]>([]);
  let componentes = $state<Componente[]>([]);
  let dolar = $state(0);
  let error = $state('');
  let borrando = $state<number | null>(null);

  // La nueva
  let componenteId = $state(Number(page.url.searchParams.get('componente')) || 0);
  let cantidad = $state(1);
  let costoUnitario = $state(0);
  let moneda = $state<Moneda>('USD');
  let proveedor = $state('');
  let link = $state('');
  let pedida = $state(hoy());

  const porId = $derived(new Map(componentes.map((c) => [c.id, c])));
  const enCamino = $derived(compras.filter((c) => !c.recibida));
  const gastadoMes = $derived(
    compras.filter((c) => c.pedida.startsWith(hoy().slice(0, 7))).reduce((t, c) => t + enPesos(c.costoUnitario * c.cantidad, c.moneda, dolar), 0),
  );

  function elegir(id: number) {
    componenteId = id;
    const c = porId.get(id);
    if (c) {
      costoUnitario = c.costo;
      moneda = c.moneda;
      proveedor = c.proveedor;
      link = c.link;
    }
  }

  async function cargar() {
    try {
      const [l, c, d] = await Promise.all([
        api<{ compras: Compra[] }>('/v1/admin/compras'),
        api<{ componentes: Componente[] }>('/v1/admin/componentes'),
        api<{ valor: number }>('/v1/admin/dolar').catch(() => ({ valor: 0 })),
      ]);
      compras = l.compras;
      componentes = c.componentes;
      dolar = d.valor;
      if (componenteId) elegir(componenteId);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer las compras.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function anotar() {
    try {
      const c = await api<Compra>('/v1/admin/compras', { cuerpo: { componenteId, cantidad, costoUnitario, moneda, proveedor, link, pedida, notas: '' } });
      compras = [c, ...compras];
      cantidad = 1;
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude anotarla.';
    }
  }

  async function recibir(c: Compra) {
    try {
      const n = await api<Compra>(`/v1/admin/compras/${c.id}`, { metodo: 'PATCH', cuerpo: { recibida: !c.recibida } });
      compras = compras.map((x) => (x.id === n.id ? n : x));
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude marcarla.';
    }
  }

  async function borrar(c: Compra) {
    try {
      await api(`/v1/admin/compras/${c.id}`, { metodo: 'DELETE' });
      compras = compras.filter((x) => x.id !== c.id);
      borrando = null;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude borrarla.';
    }
  }
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

<div class="visores">
  <div class="visor dato"><span class="grabado">En camino</span><span class="digitos">{enCamino.length}</span></div>
  <div class="visor dato"><span class="grabado">Gastado este mes</span><span class="digitos chicos">{plata(gastadoMes)}</span></div>
</div>

<section class="chapa bloque">
  <span class="grabado">Anotar compra</span>
  {#if componentes.length === 0}
    <a class="vacio" href="/admin/componentes"><span class="led"></span>Primero cargá los componentes</a>
  {:else}
    <div class="campos">
      <label class="visor ancho"><span class="grabado">Componente</span>
        <select class="campo" value={componenteId} onchange={(e) => elegir(Number(e.currentTarget.value))}>
          <option value={0}>Elegí…</option>
          {#each componentes as c}<option value={c.id}>{c.nombre}</option>{/each}
        </select></label>
      <label class="visor"><span class="grabado">Cantidad</span><input class="campo" type="number" min="1" step="1" bind:value={cantidad} /></label>
      <label class="visor"><span class="grabado">Precio c/u</span><input class="campo" type="number" min="0" step="any" bind:value={costoUnitario} /></label>
      <label class="visor"><span class="grabado">Moneda</span><select class="campo" bind:value={moneda}><option>USD</option><option>ARS</option></select></label>
      <label class="visor"><span class="grabado">Pedida</span><input class="campo" type="date" bind:value={pedida} /></label>
      <label class="visor"><span class="grabado">Proveedor</span><input class="campo" bind:value={proveedor} /></label>
      <label class="visor"><span class="grabado">Link</span><input class="campo claro" type="url" bind:value={link} placeholder="https://…" /></label>
    </div>
    <div class="acciones">
      <button class="tecla naranja" disabled={!componenteId || cantidad < 1} onclick={anotar}>Anotar</button>
      <span class="marbete">Total {plata(costoUnitario * cantidad, moneda)}</span>
    </div>
  {/if}
</section>

<section class="seccion">
  <span class="grabado">Compras · {compras.length}</span>
  {#if compras.length === 0}<p class="vacio"><span class="led"></span>Ninguna todavía</p>{/if}
  <div class="lista">
    {#each compras as c (c.id)}
      <div class="fila compra">
        <div>
          <div class="titulo">{c.cantidad} × {porId.get(c.componenteId)?.nombre ?? '—'}</div>
          <div class="sub">{c.proveedor || '—'} · {fecha(c.pedida)}{c.recibida ? ' → ' + fecha(c.recibida) : ''}</div>
        </div>
        <span class="num">{plata(c.costoUnitario * c.cantidad, c.moneda)}</span>
        <div class="acciones">
          {#if c.link}<a class="tecla negra chica" href={c.link} target="_blank" rel="noopener noreferrer" aria-label="Abrir el link">↗</a>{/if}
          <button class="llave" aria-pressed={!!c.recibida} onclick={() => recibir(c)}><span class="led"></span>Llegó</button>
          {#if borrando === c.id}
            <button class="tecla naranja chica" onclick={() => borrar(c)}>Sí</button>
          {:else}
            <button class="tecla negra chica" aria-label="Borrar" onclick={() => (borrando = c.id)}>×</button>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</section>

<style>
  .compra { grid-template-columns: minmax(0, 1fr) auto auto; }
  @media (max-width: 700px) {
    .compra { grid-template-columns: minmax(0, 1fr) auto; }
    .compra .acciones { grid-column: 1 / -1; }
  }
</style>
