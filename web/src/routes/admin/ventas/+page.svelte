<script lang="ts">
  // Ventas: las unidades vendidas, mes por mes, con lo cobrado y el margen.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { enPesos, fecha, nombreModelo, plata, type Unidad } from '$lib/control/control';

  let { data } = $props();
  let unidades = $state<Unidad[]>([]);
  let dolar = $state(0);
  let error = $state('');
  let soloSinCobrar = $state(false);

  const vendidas = $derived(
    unidades
      .filter((u) => u.vendidaEn && u.estado !== 'armada' && u.estado !== 'baja' && u.estado !== 'devuelta')
      .filter((u) => !soloSinCobrar || !u.cobrado)
      .sort((a, b) => b.vendidaEn.localeCompare(a.vendidaEn)),
  );
  // El precio en pesos con el dólar del día de la venta (el congelado al armar) o el de hoy.
  const precio = (u: Unidad) => enPesos(u.precio, u.moneda, u.dolar || dolar);
  const meses = $derived.by(() => {
    const m = new Map<string, Unidad[]>();
    for (const u of vendidas) m.set(u.vendidaEn.slice(0, 7), [...(m.get(u.vendidaEn.slice(0, 7)) ?? []), u]);
    return [...m.entries()].map(([mes, l]) => ({
      mes,
      nombre: new Date(mes + '-15').toLocaleDateString('es-AR', { month: 'long', year: 'numeric' }),
      unidades: l,
      ventas: l.reduce((t, u) => t + precio(u), 0),
      cobrado: l.filter((u) => u.cobrado).reduce((t, u) => t + precio(u), 0),
      margen: l.reduce((t, u) => t + precio(u) - u.costoArs, 0),
    }));
  });
  const porCobrar = $derived(vendidas.filter((u) => !u.cobrado).reduce((t, u) => t + precio(u), 0));

  async function cargar() {
    try {
      const [l, d] = await Promise.all([
        api<{ unidades: Unidad[] }>('/v1/admin/unidades'),
        api<{ valor: number }>('/v1/admin/dolar').catch(() => ({ valor: 0 })),
      ]);
      unidades = l.unidades;
      dolar = d.valor;
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer las ventas.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function cobrar(u: Unidad) {
    try {
      const n = await api<Unidad>(`/v1/admin/unidades/${u.id}`, { metodo: 'PATCH', cuerpo: { cobrado: !u.cobrado } });
      unidades = unidades.map((x) => (x.id === n.id ? n : x));
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude marcarla.';
    }
  }
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

<div class="cabeza">
  <div class="visores corto">
    <div class="visor dato"><span class="grabado">Vendidas</span><span class="digitos">{vendidas.length}</span></div>
    <div class="visor dato"><span class="grabado">Por cobrar</span><span class="digitos chicos">{plata(porCobrar)}</span></div>
  </div>
  <button class="llave" aria-pressed={soloSinCobrar} onclick={() => (soloSinCobrar = !soloSinCobrar)}><span class="led"></span>Sin cobrar</button>
</div>

{#if meses.length === 0}
  <a class="vacio" href="/admin/unidades"><span class="led"></span>Ninguna venta todavía</a>
{/if}
{#each meses as m (m.mes)}
  <section class="seccion">
    <div class="cabeza">
      <span class="grabado">{m.nombre} · {m.unidades.length}</span>
      <div class="acciones">
        <span class="marbete">Ventas {plata(m.ventas)}</span>
        <span class="marbete"><span class="led verde"></span>{plata(m.cobrado)}</span>
        <span class="marbete">Margen {plata(m.margen)}</span>
      </div>
    </div>
    <div class="lista">
      {#each m.unidades as u (u.id)}
        <div class="fila venta">
          <div><div class="titulo">{u.cliente || 'Sin nombre'}</div><div class="sub">{u.serie} · {nombreModelo(u.modelo)} · {fecha(u.vendidaEn)}{u.canal ? ' · ' + u.canal : ''}</div></div>
          <span class="num">{plata(u.precio, u.moneda)}</span>
          <span class="sub margen">+{plata(precio(u) - u.costoArs)}</span>
          <button class="llave" aria-pressed={u.cobrado} onclick={() => cobrar(u)}><span class="led"></span>Cobrada</button>
        </div>
      {/each}
    </div>
  </section>
{/each}

<style>
  .visores.corto { grid-template-columns: repeat(2, minmax(160px, 220px)); }
  .venta { grid-template-columns: minmax(0, 1fr) auto 110px auto; }
  .margen { text-align: right; }
  @media (max-width: 700px) {
    .venta { grid-template-columns: minmax(0, 1fr) auto; }
    .margen { display: none; }
    .venta .llave { grid-column: 1 / -1; justify-self: start; }
  }
</style>
