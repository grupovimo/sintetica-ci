<script lang="ts">
  // Un robot: la consola (/robots/[id]) y el encendido son simples; todo lo
  // avanzado vive en el Laboratorio, como módulos de rack.
  import { onDestroy } from 'svelte';
  import { page } from '$app/state';
  import { BichoAbierto, darBicho } from '$lib/bicho.svelte';
  import { Guardado, darGuardado } from '$lib/consola/guardado.svelte';
  import Barra from '$lib/consola/Barra.svelte';
  import Producto from '$lib/consola/Producto.svelte';
  import { modeloDe } from '$lib/consola/modelo';

  let { children } = $props();
  const b = new BichoAbierto(page.params.id!.toUpperCase());
  darBicho(b);
  darGuardado(new Guardado(b));
  b.cargar();
  b.escucharEventos();
  onDestroy(() => b.cerrar());

  const base = $derived(`/robots/${b.id}`);
  const lab = $derived(page.url.pathname !== base && !page.url.pathname.startsWith(base + '/encender'));
  const RACKS = [
    { ruta: '/ahora', nombre: 'Ahora', dice: 'Caras fijas y eventos de prueba' },
    { ruta: '/vida', nombre: 'Vida', dice: 'El motor y su diario' },
    { ruta: '/config', nombre: 'Config', dice: 'Todo, campo por campo' },
    { ruta: '/expresiones', nombre: 'Expresiones', dice: 'Ojos paramétricos' },
    { ruta: '/comportamientos', nombre: 'Comportamientos', dice: 'Secuencias de pasos' },
    { ruta: '/reglas', nombre: 'Reglas', dice: 'Cuando pasa X, hace Y' },
    { ruta: '/caras', nombre: 'Dibujos', dice: 'Caras con formas' },
    { ruta: '/animaciones', nombre: 'Animaciones', dice: 'Pistas en el tiempo' },
    { ruta: '/imagenes', nombre: 'Imágenes', dice: 'Fotos y GIFs' },
    { ruta: '/actualizar', nombre: 'Firmware', dice: 'Actualizaciones' },
    { ruta: '/historial', nombre: 'Historial', dice: 'Versiones de la config' },
  ];
  const nombre = $derived(b.bicho?.apodo || b.bicho?.estado?.nombre || b.config.nombre || 'Tu robot');
  const modelo = $derived(b.bicho ? modeloDe(b.bicho) : null);
</script>

<svelte:head><title>{nombre} · Sintética</title></svelte:head>

{#if !lab}
  {@render children()}
{:else}
  <Barra>
    {#snippet izquierda()}<a class="tecla negra chica" href={base}>← Consola</a>{/snippet}
    {#snippet medio()}<span class="grabado" style="color: var(--brasa)">Laboratorio · {nombre}</span>{/snippet}
  </Barra>
  <div class="caja lab">
    <aside class="rack chapa" aria-label="Módulos del laboratorio">
      {#if modelo && b.bicho}
        <div class="mini"><Producto {modelo} config={b.config} visible={b.bicho.estado?.expresion ?? ''} apagado={!b.bicho.conectado} /></div>
      {/if}
      <nav>
        {#each RACKS as r, i}
          <a href={base + r.ruta} aria-current={page.url.pathname === base + r.ruta ? 'page' : undefined}>
            <span class="n">{String(i + 1).padStart(2, '0')}</span>
            <span class="t"><b>{r.nombre}</b><small>{r.dice}</small></span>
            <span class="led" class:on={page.url.pathname === base + r.ruta}></span>
          </a>
        {/each}
      </nav>
    </aside>
    <section class="monitor">
      {#if b.error}<p class="aviso-caja error">{b.error}</p>
      {:else if b.bicho}{@render children()}
      {:else}<p class="etiqueta">Cargando…</p>{/if}
    </section>
  </div>
{/if}

<style>
  .lab { display: grid; grid-template-columns: 280px minmax(0, 1fr); gap: 24px; align-items: start; padding-bottom: 56px; }
  .rack { position: sticky; top: 16px; padding: 12px; border-radius: 28px; display: flex; flex-direction: column; gap: 12px; }
  .mini { --radio-ventana: 18px; }
  nav { display: flex; flex-direction: column; gap: 4px; }
  nav a { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 14px; text-decoration: none; color: var(--c-papel); }
  nav a:hover { background: rgba(255,255,255,.05); }
  nav a[aria-current='page'] { background: var(--hundido); box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .n { font: 900 12px/1 var(--marca); color: var(--gris); font-variant-numeric: tabular-nums; }
  .t { flex: 1; display: flex; flex-direction: column; gap: 3px; min-width: 0; }
  .t b { font: 800 14px/1.1 var(--marca); font-stretch: 112%; letter-spacing: .02em; text-transform: uppercase; }
  .t small { font: 600 12px/1.2 var(--marca); color: var(--gris); }
  /* El monitor: las herramientas del laboratorio, con el estilo de siempre (claro). */
  .monitor { min-width: 0; border-radius: 28px; background: var(--papel); color: var(--tinta); font: 17px/1.55 var(--serif); color-scheme: normal;
    box-shadow: inset 0 0 0 10px #1c1a17, inset 0 0 0 11px rgba(255,255,255,.08), 0 30px 50px -30px rgba(0,0,0,.9); padding: 38px; }
  .monitor :global(h1), .monitor :global(h2), .monitor :global(h3) { font-family: var(--serif); }
  @media (max-width: 900px) {
    .lab { grid-template-columns: 1fr; }
    .rack { position: static; }
    .mini { display: none; }
    nav { flex-direction: row; overflow-x: auto; }
    nav a { flex: none; }
    .t small { display: none; }
    .monitor { padding: 22px; }
  }
</style>
