<script lang="ts">
  // Resumen: los números del negocio de un vistazo.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { ESTADOS, nombreModelo, plata, type Resumen } from '$lib/control/control';

  let { data } = $props();
  let r = $state<Resumen | null>(null);
  let error = $state('');

  onMount(async () => {
    if (!data.usuario?.admin) return;
    try {
      r = await api<Resumen>('/v1/admin/resumen');
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer el resumen.';
    }
  });
  const mes = $derived(r ? new Date(r.mes + '-15').toLocaleDateString('es-AR', { month: 'long' }) : '');
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
{#if r}
  <section class="seccion">
    <div class="cabeza">
      <span class="grabado">Este mes · {mes}</span>
      <span class="marbete" title={r.dolarFecha ? new Date(r.dolarFecha).toLocaleString('es-AR') : ''}>
        <span class="led" class:verde={!r.dolarError} class:ambar={r.dolarError}></span>Dólar oficial {plata(r.dolar)}
      </span>
    </div>
    <div class="visores">
      <div class="visor dato"><span class="grabado">Vendidas</span><span class="digitos">{r.vendidasMes}</span></div>
      <div class="visor dato"><span class="grabado">Ventas</span><span class="digitos chicos">{plata(r.ventasMes)}</span></div>
      <div class="visor dato"><span class="grabado">Cobrado</span><span class="digitos chicos">{plata(r.cobradoMes)}</span></div>
      <div class="visor dato"><span class="grabado">Margen</span><span class="digitos chicos">{plata(r.margenMes)}</span></div>
      <div class="visor dato"><span class="grabado">Por cobrar</span><span class="digitos chicos">{plata(r.porCobrar)}</span><small>de siempre</small></div>
      <div class="visor dato"><span class="grabado">En unidades</span><span class="digitos chicos">{plata(r.invertido)}</span><small>costo armado</small></div>
    </div>
  </section>

  <section class="seccion">
    <span class="grabado">Unidades · {r.unidades}</span>
    <div class="visores">
      {#each ESTADOS as e}
        <a class="visor dato estado-dato" href="/admin/unidades?estado={e.clave}">
          <span class="estado"><span class="led {e.led}"></span>{e.nombre}</span>
          <span class="digitos">{r.estados[e.clave] ?? 0}</span>
        </a>
      {/each}
    </div>
  </section>

  <section class="seccion">
    <span class="grabado">Costo por modelo</span>
    {#if r.costos.length === 0}
      <a class="vacio" href="/admin/componentes"><span class="led"></span>Cargá los componentes</a>
    {:else}
      <div class="lista">
        {#each r.costos as c}
          <div class="fila costo">
            <div><div class="titulo">{nombreModelo(c.modelo)}</div><div class="sub">{c.piezas} componentes</div></div>
            <span class="num">{plata(c.usd, 'USD')}</span>
            <span class="num">{plata(c.ars)}</span>
            <span class="estado"><span class="led" class:verde={c.puedoArmar > 0} class:on={c.puedoArmar === 0}></span>{c.puedoArmar} para armar</span>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  {#if r.stockBajo.length}
    <section class="seccion">
      <span class="grabado">Stock bajo</span>
      <div class="lista">
        {#each r.stockBajo as c}
          <a class="fila bajo" href="/admin/compras?componente={c.id}">
            <div><div class="titulo">{c.nombre}</div><div class="sub">{c.proveedor || c.categoria || '—'}</div></div>
            <span class="estado"><span class="led ambar"></span>{c.stock} / {c.stockMinimo}</span>
          </a>
        {/each}
      </div>
    </section>
  {/if}

  <section class="seccion">
    <span class="grabado">Preventa del cubito</span>
    <a class="placa-naranja preventa" href="/admin/preventa">
      <span class="estado oscuro"><span class="led" class:on={r.preventa.activa}></span>{r.preventa.activa ? 'Abierta' : 'Cerrada'}</span>
      <div class="visores">
        <div class="visor dato"><span class="grabado">Señas</span><span class="digitos">{r.preventa.senas}</span></div>
        <div class="visor dato"><span class="grabado">Quedan</span><span class="digitos">{r.preventa.quedan}<small> / {r.preventa.cupos}</small></span></div>
        <div class="visor dato"><span class="grabado">Señado</span><span class="digitos chicos">{plata(r.preventa.senado)}</span></div>
      </div>
    </a>
  </section>
{:else if !error}
  <p class="vacio"><span class="led on"></span>Cargando</p>
{/if}

<style>
  .estado-dato { text-decoration: none; }
  .estado-dato:hover { box-shadow: inset 0 3px 10px rgba(0,0,0,.8), 0 0 0 2px #2c2925; }
  .costo { grid-template-columns: minmax(0, 1fr) auto auto auto; }
  .bajo { grid-template-columns: minmax(0, 1fr) auto; text-decoration: none; }
  .preventa { display: flex; flex-direction: column; gap: 16px; text-decoration: none; }
  .estado.oscuro { color: #1a1714; }
  .digitos small { font-size: 18px; opacity: .6; }
  @media (max-width: 700px) {
    .costo { grid-template-columns: minmax(0, 1fr) auto; }
    .costo .estado { grid-column: 1 / -1; }
  }
</style>
