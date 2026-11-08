<script lang="ts">
  // La página de una pieza: la foto grande, qué es y reservarla.
  import { page } from '$app/state';
  import Cabecera from '$lib/landing/Cabecera.svelte';
  import Foto from '$lib/landing/Foto.svelte';
  import Pie from '$lib/landing/Pie.svelte';
  import { PIEZAS, pieza } from '$lib/landing/piezas';
  import '$lib/landing/marca.css';

  let { data } = $props();
  const p = $derived(pieza(page.params.clave ?? ''));
  const otras = $derived(PIEZAS.filter((x) => x.clave !== p?.clave));
</script>

<svelte:head>
  <title>{p ? `${p.nombre} · Sintética` : 'Sintética'}</title>
  {#if p}<meta name="description" content="{p.nombre}: {p.frase} {p.tipo}, hecha a mano en Mar del Plata." />{/if}
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="landing quieto">
  <div class="grano" aria-hidden="true"></div>
  <section class="cabeza">
    <div class="puntos" aria-hidden="true"></div>
    <Cabecera usuario={data.usuario} base="/" soloLanding={data.soloLanding} />
    {#if p}
      <div class="contenedor detalle">
        <div class="foto-grande">
          <Foto src={p.foto} alt={p.queMuestra} clara />
        </div>
        <div class="pila-32 texto">
          <span class="ficha arriba-sola">{p.tipo}</span>
          <h1 class="t-display" style:--largo={Math.max(...p.nombre.split(' ').map((w) => w.length))}>{p.nombre}</h1>
          <p class="t-lead">{p.frase}</p>
          <div class="fila chica">
            {#each p.fichas as f}<span class="ficha">{f}</span>{/each}
          </div>
          <div class="entre">
            <span class="precio">[PRECIO]</span>
            <a class="inflado" href="/?pieza={p.clave}#reservar">Reservar</a>
          </div>
        </div>
      </div>
    {:else}
      <div class="contenedor pila-32 nada">
        <h1 class="t-h2">Esa pieza no existe.</h1>
        <a class="inflado arriba-sola" href="/">Ver todas</a>
      </div>
    {/if}
  </section>

  <section class="hueso seccion">
    <div class="contenedor pila-48">
      <h2 class="t-h3">Otras piezas</h2>
      <div class="otras">
        {#each otras as o}
          <a class="otra" href="/piezas/{o.clave}">
            <Foto src={o.foto} alt={o.queMuestra} clara radio="28px" />
            <span class="t-h4">{o.nombre}</span>
            <span class="tipo">{o.tipo}</span>
          </a>
        {/each}
      </div>
    </div>
  </section>
  <Pie />
</div>

<style>
  .cabeza { position: relative; overflow: hidden; padding-bottom: 96px; }
  .detalle { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 440px), 1fr)); gap: 48px; align-items: center; padding-top: 32px; }
  .foto-grande :global(.foto) { box-shadow: 0 32px 48px -32px rgba(0, 0, 0, 0.6); }
  /* Que la palabra más larga entre entera en la columna (~600 px). */
  .texto .t-display { font-size: min(112px, calc(600px / var(--largo) * 1.05), calc(9vw * 8 / var(--largo))); }
  .precio { font: 900 32px/1 'Archivo', sans-serif; font-stretch: 120%; }
  .nada { padding: 96px 40px; }
  .otras { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 24px; }
  .otra .t-h4 { font-size: clamp(20px, 1.8vw, 26px); overflow-wrap: anywhere; }
  .otra { display: flex; flex-direction: column; gap: 12px; color: #111; transition: transform 0.25s cubic-bezier(0.3, 1.7, 0.45, 1); }
  .otra:hover { transform: translateY(-4px); }
  .tipo { font: 700 15px/1.2 'Archivo', sans-serif; color: #4a4741; }
  @media (max-width: 720px) {
    .landing :global(.contenedor) { padding: 0 16px; }
  }
</style>
