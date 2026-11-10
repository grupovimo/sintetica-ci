<script lang="ts">
  // Unidades: cada robot físico, de armado a desplegado, atado a su robot de la nube.
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { api, ErrorApi } from '$lib/api';
  import {
    COLORES, ESTADOS, MODELOS, fecha, ledEstado, nombreEstado, nombreModelo, plata,
    type BichoAdmin, type EstadoUnidad, type Unidad,
  } from '$lib/control/control';

  let { data } = $props();
  let unidades = $state<Unidad[]>([]);
  let bichos = $state<BichoAdmin[]>([]);
  let error = $state('');
  let filtro = $state<EstadoUnidad | ''>((page.url.searchParams.get('estado') as EstadoUnidad) ?? '');
  let abierta = $state<number | null>(null);
  let guardado = $state<number | null>(null);
  let borrando = $state<number | null>(null);

  // Armar una nueva
  let serie = $state('');
  let modelo = $state('unidad');
  let color = $state('naranja');
  let descontar = $state(true);
  let faltan = $state('');
  let armando = $state(false);

  const visibles = $derived(filtro ? unidades.filter((u) => u.estado === filtro) : unidades);
  const sueltos = $derived(bichos.filter((b) => !b.virtual && !b.unidad));
  const bicho = (id: string) => bichos.find((b) => b.id === id);

  function proximaSerie() {
    const n = unidades.map((u) => Number(u.serie.match(/(\d+)$/)?.[1] ?? 0)).reduce((a, b) => Math.max(a, b), 0);
    return 'S-' + String(n + 1).padStart(4, '0');
  }

  async function cargar() {
    try {
      const [u, b] = await Promise.all([
        api<{ unidades: Unidad[] }>('/v1/admin/unidades'),
        api<{ bichos: BichoAdmin[] }>('/v1/admin/bichos'),
      ]);
      unidades = u.unidades;
      bichos = b.bichos;
      if (!serie) serie = proximaSerie();
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer las unidades.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function armar(forzar = false) {
    armando = true;
    faltan = '';
    try {
      const u = await api<Unidad>('/v1/admin/unidades', { cuerpo: { serie, modelo, color, descontar, forzar, notas: '' } });
      unidades = [u, ...unidades];
      abierta = u.id;
      serie = proximaSerie();
      error = '';
    } catch (e) {
      if (e instanceof ErrorApi && e.estado === 409 && e.detalle) faltan = e.detalle;
      else error = e instanceof ErrorApi ? e.message : 'No pude armarla.';
    } finally {
      armando = false;
    }
  }

  async function cambiar(u: Unidad, campos: Partial<Unidad>) {
    try {
      const n = await api<Unidad>(`/v1/admin/unidades/${u.id}`, { metodo: 'PATCH', cuerpo: campos });
      unidades = unidades.map((x) => (x.id === n.id ? n : x));
      if ('bichoId' in campos) bichos = (await api<{ bichos: BichoAdmin[] }>('/v1/admin/bichos')).bichos;
      guardado = u.id;
      setTimeout(() => guardado === u.id && (guardado = null), 1500);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude guardarla.';
      unidades = [...unidades];
    }
  }

  async function borrar(u: Unidad) {
    try {
      await api(`/v1/admin/unidades/${u.id}`, { metodo: 'DELETE' });
      unidades = unidades.filter((x) => x.id !== u.id);
      abierta = null;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude borrarla.';
    }
  }

  const texto = (e: Event) => (e.currentTarget as HTMLInputElement).value;
  const numero = (e: Event) => Number((e.currentTarget as HTMLInputElement).value) || 0;
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

<section class="chapa bloque">
  <span class="grabado">Armar</span>
  <div class="campos">
    <label class="visor"><span class="grabado">Nº de serie</span><input class="campo" bind:value={serie} maxlength="32" /></label>
    <label class="visor"><span class="grabado">Modelo</span>
      <select class="campo" bind:value={modelo}>{#each MODELOS as m}<option value={m.clave}>{m.nombre}</option>{/each}</select></label>
    <label class="visor"><span class="grabado">Color</span>
      <select class="campo" bind:value={color}>{#each COLORES as c}<option value={c.clave}>{c.nombre}</option>{/each}</select></label>
  </div>
  <div class="acciones">
    <button class="llave" aria-pressed={descontar} onclick={() => (descontar = !descontar)}><span class="led"></span>Descontar stock</button>
    <button class="tecla naranja" disabled={armando || !serie} onclick={() => armar()}>Armar</button>
  </div>
  {#if faltan}
    <div class="aviso error"><span class="led"></span>Falta: {faltan}
      <button class="tecla negra chica" onclick={() => armar(true)}>Armar igual</button></div>
  {/if}
</section>

<section class="seccion">
  <div class="cabeza">
    <span class="grabado">Unidades · {visibles.length}</span>
    <div class="modo" role="radiogroup" aria-label="Filtrar por estado">
      <button role="radio" aria-checked={filtro === ''} onclick={() => (filtro = '')}>Todas</button>
      {#each ESTADOS as e}<button role="radio" aria-checked={filtro === e.clave} onclick={() => (filtro = e.clave)}>{e.nombre}</button>{/each}
    </div>
  </div>
  {#if visibles.length === 0}
    <p class="vacio"><span class="led"></span>Ninguna</p>
  {/if}
  <div class="lista">
    {#each visibles as u (u.id)}
      {@const b = bicho(u.bichoId)}
      <button class="fila boton unidad" aria-expanded={abierta === u.id} onclick={() => (abierta = abierta === u.id ? null : u.id)}>
        <div class="quien">
          <div class="titulo">{u.serie}</div>
          <div class="sub">{nombreModelo(u.modelo)} · {u.color || '—'}{u.cliente ? ' · ' + u.cliente : ''}</div>
        </div>
        <span class="estado"><span class="led {ledEstado(u.estado)}"></span>{nombreEstado(u.estado)}</span>
        <span class="estado robot" title={b?.dueno}>
          {#if b}<span class="led" class:verde={b.conectado}></span>{b.apodo || b.id.slice(-4)}{:else}<span class="led"></span>Sin robot{/if}
        </span>
        <span class="num">{u.precio ? plata(u.precio, u.moneda) : plata(u.costoArs)}</span>
      </button>

      {#if abierta === u.id}
        <div class="chapa bloque ficha">
          <div class="cabeza">
            <span class="grabado">Ficha · {u.serie}</span>
            <span class="estado"><span class="led" class:verde={guardado === u.id}></span>{guardado === u.id ? 'Guardado' : 'Guarda solo'}</span>
          </div>
          <div class="campos">
            <label class="visor"><span class="grabado">Estado</span>
              <select class="campo" value={u.estado} onchange={(e) => cambiar(u, { estado: texto(e) as EstadoUnidad })}>
                {#each ESTADOS as e}<option value={e.clave}>{e.nombre}</option>{/each}
              </select></label>
            <label class="visor"><span class="grabado">Robot</span>
              <select class="campo" value={u.bichoId} onchange={(e) => cambiar(u, { bichoId: texto(e) })}>
                <option value="">Ninguno</option>
                {#each bichos.filter((x) => !x.unidad || x.id === u.bichoId) as x}
                  <option value={x.id}>{x.apodo || x.id}{x.virtual ? ' · virtual' : ''}</option>
                {/each}
              </select></label>
            <label class="visor"><span class="grabado">Serie</span><input class="campo" value={u.serie} maxlength="32" onchange={(e) => cambiar(u, { serie: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Color</span>
              <select class="campo" value={u.color} onchange={(e) => cambiar(u, { color: texto(e) })}>
                <option value="">—</option>{#each COLORES as c}<option value={c.clave}>{c.nombre}</option>{/each}
              </select></label>
          </div>

          {#if b}
            <div class="visores">
              <div class="visor dato"><span class="grabado">En la nube</span>
                <span class="estado"><span class="led" class:verde={b.conectado}></span>{b.conectado ? 'En línea' : b.ultimaConexion ? fecha(b.ultimaConexion) : 'Nunca'}</span></div>
              <div class="visor dato"><span class="grabado">Dueño</span><span class="campo claro corto">{b.dueno || 'Sin reclamar'}</span></div>
              <div class="visor dato"><span class="grabado">Firmware</span><span class="campo claro corto">{b.firmware || '—'}</span></div>
              <div class="visor dato"><span class="grabado">MAC</span><span class="campo claro corto">{b.id}</span></div>
            </div>
          {/if}

          <span class="grabado">Venta</span>
          <div class="campos">
            <label class="visor"><span class="grabado">Cliente</span><input class="campo" value={u.cliente} onchange={(e) => cambiar(u, { cliente: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Contacto</span><input class="campo" value={u.contacto} onchange={(e) => cambiar(u, { contacto: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Precio</span>
              <input class="campo" type="number" min="0" step="any" value={u.precio || ''} onchange={(e) => cambiar(u, { precio: numero(e) })} /></label>
            <label class="visor"><span class="grabado">Moneda</span>
              <select class="campo" value={u.moneda} onchange={(e) => cambiar(u, { moneda: texto(e) as Unidad['moneda'] })}><option>ARS</option><option>USD</option></select></label>
            <label class="visor"><span class="grabado">Canal</span><input class="campo" value={u.canal} placeholder="Instagram, feria…" onchange={(e) => cambiar(u, { canal: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Vendida</span><input class="campo" type="date" value={u.vendidaEn} onchange={(e) => cambiar(u, { vendidaEn: texto(e) })} /></label>
            <label class="visor"><span class="grabado">Desplegada</span><input class="campo" type="date" value={u.desplegadaEn} onchange={(e) => cambiar(u, { desplegadaEn: texto(e) })} /></label>
            <label class="visor ancho"><span class="grabado">Notas</span><input class="campo claro" value={u.notas} onchange={(e) => cambiar(u, { notas: texto(e) })} /></label>
          </div>
          <div class="acciones">
            <button class="llave" aria-pressed={u.cobrado} onclick={() => cambiar(u, { cobrado: !u.cobrado })}><span class="led"></span>Cobrada</button>
            <span class="marbete">Costo {plata(u.costoUsd, 'USD')} · {plata(u.costoArs)}</span>
            {#if u.precio}<span class="marbete">Margen {plata((u.moneda === 'USD' ? u.precio * u.dolar : u.precio) - u.costoArs)}</span>{/if}
          </div>

          <span class="grabado">Historial</span>
          <ol class="historial">
            {#each [...u.historial].reverse() as h}<li><time>{fecha(h.en)}</time>{h.texto}</li>{/each}
          </ol>
          <div class="acciones">
            {#if borrando === u.id}
              <button class="tecla naranja chica" onclick={() => borrar(u)}>Sí, borrar</button>
              <button class="tecla negra chica" onclick={() => (borrando = null)}>No</button>
            {:else}
              <button class="tecla negra chica" onclick={() => (borrando = u.id)}>Borrar</button>
            {/if}
          </div>
        </div>
      {/if}
    {/each}
  </div>
</section>

{#if sueltos.length}
  <section class="seccion">
    <span class="grabado">Robots sin unidad · {sueltos.length}</span>
    <div class="lista">
      {#each sueltos as b (b.id)}
        <div class="fila suelto">
          <div><div class="titulo">{b.apodo || b.id}</div><div class="sub">{b.dueno || 'Sin reclamar'} · {b.perfil}</div></div>
          <span class="estado"><span class="led" class:verde={b.conectado}></span>{b.conectado ? 'En línea' : b.ultimaConexion ? fecha(b.ultimaConexion) : 'Nunca'}</span>
          <span class="sub">{b.firmware || '—'}</span>
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .unidad { grid-template-columns: minmax(0, 1.4fr) 130px minmax(0, 1fr) auto; }
  .suelto { grid-template-columns: minmax(0, 1fr) auto auto; }
  .robot { overflow: hidden; text-overflow: ellipsis; }
  .ficha { margin-top: -4px; }
  .corto { font-size: 15px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .aviso .tecla { margin-left: auto; }
  @media (max-width: 700px) {
    .unidad { grid-template-columns: minmax(0, 1fr) auto; }
    .unidad .robot { display: none; }
    .unidad .estado:not(.robot) { grid-row: 2; }
    .unidad .num { grid-row: 1; grid-column: 2; }
    .suelto { grid-template-columns: minmax(0, 1fr) auto; }
    .suelto > .sub { display: none; }
  }
</style>
