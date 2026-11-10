<script lang="ts">
  // Componentes: el catálogo (costo, proveedor, link, stock) y la lista de
  // materiales de cada modelo, con lo que cuesta armarlo.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { MODELOS, enDolares, enPesos, plata, type Componente, type Material } from '$lib/control/control';

  let { data } = $props();
  let componentes = $state<Componente[]>([]);
  let materiales = $state<Material[]>([]);
  let dolar = $state(0);
  let error = $state('');
  let abierto = $state<number | null>(null);
  let guardado = $state<number | null>(null);
  let borrando = $state<number | null>(null);
  let modelo = $state('unidad');
  let bomGuardada = $state(false);

  const porId = $derived(new Map(componentes.map((c) => [c.id, c])));
  const bom = $derived(materiales.filter((m) => m.modelo === modelo));
  const total = $derived(
    bom.reduce(
      (t, m) => {
        const c = porId.get(m.componenteId);
        if (!c) return t;
        return { usd: t.usd + enDolares(c.costo * m.cantidad, c.moneda, dolar), ars: t.ars + enPesos(c.costo * m.cantidad, c.moneda, dolar) };
      },
      { usd: 0, ars: 0 },
    ),
  );
  const libres = $derived(componentes.filter((c) => !bom.some((m) => m.componenteId === c.id)));

  async function cargar() {
    try {
      const [l, d] = await Promise.all([
        api<{ componentes: Componente[]; materiales: Material[] }>('/v1/admin/componentes'),
        api<{ valor: number }>('/v1/admin/dolar').catch(() => ({ valor: 0 })),
      ]);
      componentes = l.componentes;
      materiales = l.materiales;
      dolar = d.valor;
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer los componentes.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function nuevo() {
    try {
      const c = await api<Componente>('/v1/admin/componentes', { cuerpo: { nombre: 'Componente nuevo', moneda: 'USD', costo: 0, stock: 0, stockMinimo: 0 } });
      componentes = [...componentes, c];
      abierto = c.id;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude crearlo.';
    }
  }

  async function guardar(c: Componente, campos: Partial<Componente>) {
    const { id, actualizado, ...resto } = { ...c, ...campos };
    try {
      const n = await api<Componente>(`/v1/admin/componentes/${id}`, { metodo: 'PUT', cuerpo: resto });
      componentes = componentes.map((x) => (x.id === n.id ? n : x));
      guardado = id;
      setTimeout(() => guardado === id && (guardado = null), 1500);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude guardarlo.';
      componentes = [...componentes];
    }
  }

  async function borrar(c: Componente) {
    try {
      await api(`/v1/admin/componentes/${c.id}`, { metodo: 'DELETE' });
      componentes = componentes.filter((x) => x.id !== c.id);
      materiales = materiales.filter((m) => m.componenteId !== c.id);
      abierto = null;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude borrarlo.';
    }
  }

  async function guardarBom(lista: Material[]) {
    try {
      const l = await api<{ materiales: Material[] }>(`/v1/admin/modelos/${modelo}/materiales`, {
        metodo: 'PUT',
        cuerpo: { materiales: lista.map((m) => ({ modelo, componenteId: m.componenteId, cantidad: m.cantidad })) },
      });
      materiales = l.materiales;
      bomGuardada = true;
      setTimeout(() => (bomGuardada = false), 1500);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude guardar la lista.';
    }
  }
  const sumar = (id: number) => id && guardarBom([...bom, { modelo, componenteId: id, cantidad: 1 }]);
  const cantidad = (id: number, n: number) => guardarBom(n > 0 ? bom.map((m) => (m.componenteId === id ? { ...m, cantidad: n } : m)) : bom.filter((m) => m.componenteId !== id));

  const texto = (e: Event) => (e.currentTarget as HTMLInputElement).value;
  const numero = (e: Event) => Number((e.currentTarget as HTMLInputElement).value) || 0;
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

<section class="placa-naranja bom">
  <div class="cabeza">
    <div class="modo" role="radiogroup" aria-label="Modelo">
      {#each MODELOS as m}<button role="radio" aria-checked={modelo === m.clave} onclick={() => (modelo = m.clave)}>{m.nombre}</button>{/each}
    </div>
    <span class="estado oscuro"><span class="led" class:on={bomGuardada}></span>{bomGuardada ? 'Guardado' : 'Lista de materiales'}</span>
  </div>
  <div class="visores">
    <div class="visor dato"><span class="grabado">Cuesta armarlo</span><span class="digitos chicos">{plata(total.usd, 'USD')}</span></div>
    <div class="visor dato"><span class="grabado">En pesos</span><span class="digitos chicos">{plata(total.ars)}</span><small>dólar {plata(dolar)}</small></div>
  </div>
  <div class="lista">
    {#each bom as m (m.componenteId)}
      {@const c = porId.get(m.componenteId)}
      {#if c}
        <div class="fila material">
          <div><div class="titulo">{c.nombre}</div><div class="sub">{plata(c.costo, c.moneda)} c/u · stock {c.stock}</div></div>
          <label class="visor cant"><span class="solo-lectores">Cantidad</span>
            <input class="campo" type="number" min="0" step="any" value={m.cantidad} onchange={(e) => cantidad(m.componenteId, numero(e))} /></label>
          <span class="num">{plata(enDolares(c.costo * m.cantidad, c.moneda, dolar), 'USD')}</span>
          <button class="tecla negra cuadrada" aria-label="Sacar {c.nombre}" onclick={() => cantidad(m.componenteId, 0)}>×</button>
        </div>
      {/if}
    {/each}
    {#if libres.length}
      <label class="visor"><span class="grabado">Sumar componente</span>
        <select class="campo claro" value="" onchange={(e) => sumar(Number(texto(e)))}>
          <option value="">Elegí…</option>
          {#each libres as c}<option value={c.id}>{c.nombre}</option>{/each}
        </select></label>
    {/if}
  </div>
</section>

<section class="seccion">
  <div class="cabeza">
    <span class="grabado">Catálogo · {componentes.length}</span>
    <button class="tecla naranja chica" onclick={nuevo}>+ Componente</button>
  </div>
  {#if componentes.length === 0}<p class="vacio"><span class="led"></span>Ninguno todavía</p>{/if}
  <div class="lista">
    {#each componentes as c (c.id)}
      {@const bajo = c.stockMinimo > 0 && c.stock <= c.stockMinimo}
      <button class="fila boton componente" aria-expanded={abierto === c.id} onclick={() => (abierto = abierto === c.id ? null : c.id)}>
        <div><div class="titulo">{c.nombre}</div><div class="sub">{[c.categoria, c.proveedor].filter(Boolean).join(' · ') || '—'}</div></div>
        <span class="num">{plata(c.costo, c.moneda)}</span>
        <span class="sub conv">{c.moneda === 'USD' ? plata(enPesos(c.costo, 'USD', dolar)) : plata(enDolares(c.costo, 'ARS', dolar), 'USD')}</span>
        <span class="estado"><span class="led" class:ambar={bajo} class:verde={!bajo && c.stock > 0}></span>{c.stock}</span>
      </button>
      {#if abierto === c.id}
        <div class="chapa bloque">
          <div class="cabeza">
            <span class="grabado">{c.nombre}</span>
            <span class="estado"><span class="led" class:verde={guardado === c.id}></span>{guardado === c.id ? 'Guardado' : 'Guarda solo'}</span>
          </div>
          <div class="campos">
            <label class="visor"><span class="grabado">Nombre</span><input class="campo" value={c.nombre} onchange={(e) => guardar(c, { nombre: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Categoría</span><input class="campo" value={c.categoria} placeholder="Placa, batería…" onchange={(e) => guardar(c, { categoria: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Proveedor</span><input class="campo" value={c.proveedor} onchange={(e) => guardar(c, { proveedor: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Costo</span><input class="campo" type="number" min="0" step="any" value={c.costo} onchange={(e) => guardar(c, { costo: numero(e) })} /></label>
            <label class="visor"><span class="grabado">Moneda</span>
              <select class="campo" value={c.moneda} onchange={(e) => guardar(c, { moneda: texto(e) as Componente['moneda'] })}><option>USD</option><option>ARS</option></select></label>
            <label class="visor"><span class="grabado">Stock</span><input class="campo" type="number" step="1" value={c.stock} onchange={(e) => guardar(c, { stock: Math.round(numero(e)) })} /></label>
            <label class="visor"><span class="grabado">Avisar con</span><input class="campo" type="number" min="0" step="1" value={c.stockMinimo} onchange={(e) => guardar(c, { stockMinimo: Math.round(numero(e)) })} /></label>
            <label class="visor ancho"><span class="grabado">Link</span><input class="campo claro" type="url" value={c.link} placeholder="https://…" onchange={(e) => guardar(c, { link: texto(e) })} /></label>
            <label class="visor ancho"><span class="grabado">Notas</span><input class="campo claro" value={c.notas} onchange={(e) => guardar(c, { notas: texto(e) })} /></label>
          </div>
          <div class="acciones">
            {#if c.link}<a class="tecla chica" href={c.link} target="_blank" rel="noopener noreferrer">Comprar ↗</a>{/if}
            <a class="tecla negra chica" href="/admin/compras?componente={c.id}">Anotar compra</a>
            {#if borrando === c.id}
              <button class="tecla naranja chica" onclick={() => borrar(c)}>Sí, borrar</button>
              <button class="tecla negra chica" onclick={() => (borrando = null)}>No</button>
            {:else}
              <button class="tecla negra chica" onclick={() => (borrando = c.id)}>Borrar</button>
            {/if}
          </div>
        </div>
      {/if}
    {/each}
  </div>
</section>

<style>
  .bom { display: flex; flex-direction: column; gap: 16px; }
  .estado.oscuro { color: #1a1714; }
  .material { grid-template-columns: minmax(0, 1fr) 96px auto auto; }
  .cant { padding: 8px 12px; }
  .componente { grid-template-columns: minmax(0, 1fr) auto 110px 64px; }
  .conv { text-align: right; }
  @media (max-width: 700px) {
    .material { grid-template-columns: minmax(0, 1fr) 76px auto; }
    .material .num { display: none; }
    .componente { grid-template-columns: minmax(0, 1fr) auto auto; }
    .conv { display: none; }
  }
</style>
