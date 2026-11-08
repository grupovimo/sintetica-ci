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
  // Lo que la consola no cubre. Ahora, Imágenes y Config siguen andando por su ruta.
  const RACKS = [
    { ruta: '/expresiones', nombre: 'Expresiones', icono: 'M4 12c3-5 13-5 16 0-3 5-13 5-16 0Z M12 9.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5Z' },
    { ruta: '/comportamientos', nombre: 'Comportamientos', icono: 'M5 4v16 M5 6h11l-2 3 2 3H5' },
    { ruta: '/reglas', nombre: 'Reglas', icono: 'M6 6h.01 M6 18h.01 M6 6c6 0 6 12 12 12 M18 18h.01' },
    { ruta: '/vida', nombre: 'Vida', icono: 'M3 12h4l2-6 4 12 2-6h6' },
    { ruta: '/caras', nombre: 'Dibujos', icono: 'M4 20l4-1 11-11-3-3L5 16l-1 4Z' },
    { ruta: '/animaciones', nombre: 'Animaciones', icono: 'M6 4l14 8-14 8V4Z' },
    { ruta: '/actualizar', nombre: 'Firmware', icono: 'M12 4v12 M7 11l5 5 5-5 M5 20h14' },
    { ruta: '/historial', nombre: 'Historial', icono: 'M12 7v5l3 2 M3 12a9 9 0 1 0 3-6.7L3 8 M3 3v5h5' },
  ];
  // Las herramientas ya rehechas en el lenguaje de la consola (las demás, en el monitor de siempre).
  const NUEVAS = ['/expresiones', '/comportamientos', '/reglas', '/vida', '/actualizar', '/historial'];
  const nueva = $derived(NUEVAS.some((r) => page.url.pathname === base + r));
  // Los editores con lienzo (compartidos con las bibliotecas): en la mesa, con la paleta de la consola.
  const VIEJAS = ['/caras', '/animaciones'];
  const vieja = $derived(VIEJAS.some((r) => page.url.pathname === base + r));
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
        {#each RACKS as r}
          <a href={base + r.ruta} aria-current={page.url.pathname === base + r.ruta ? 'page' : undefined}>
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={r.icono} /></svg>
            <b>{r.nombre}</b>
          </a>
        {/each}
      </nav>
    </aside>
    <section class={nueva ? 'mesa' : vieja ? 'mesa viejo' : 'monitor'}>
      {#if b.error}<p class="aviso-caja error">{b.error}</p>
      {:else if b.bicho}{@render children()}
      {:else}<p class="etiqueta">Cargando…</p>{/if}
    </section>
  </div>
{/if}

<style>
  .lab { display: grid; grid-template-columns: 236px minmax(0, 1fr); gap: 24px; align-items: start; padding-bottom: 56px; }
  .rack { position: sticky; top: 16px; padding: 12px; border-radius: 28px; display: flex; flex-direction: column; gap: 12px; }
  .mini { --radio-ventana: 18px; }
  nav { display: flex; flex-direction: column; gap: 4px; }
  nav a { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border-radius: 14px; text-decoration: none; color: var(--c-papel); }
  nav a:hover { background: rgba(255,255,255,.05); }
  nav a[aria-current='page'] { background: var(--hundido); box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  nav svg { color: var(--gris); flex: none; }
  nav a[aria-current='page'] svg { color: var(--brasa); }
  nav b { font: 800 12px/1.1 var(--marca); font-stretch: 100%; letter-spacing: .02em; text-transform: uppercase; }
  /* El monitor: las herramientas del laboratorio, con el estilo de siempre (claro). */
  .monitor { min-width: 0; border-radius: 28px; background: var(--papel); color: var(--tinta); font: 17px/1.55 var(--serif); color-scheme: normal;
    box-shadow: inset 0 0 0 10px #1c1a17, inset 0 0 0 11px rgba(255,255,255,.08), 0 30px 50px -30px rgba(0,0,0,.9); padding: 38px; }
  .monitor :global(h1), .monitor :global(h2), .monitor :global(h3) { font-family: var(--serif); }
  .mesa { min-width: 0; }
  @media (max-width: 900px) {
    .lab { grid-template-columns: 1fr; }
    .rack { position: static; }
    .mini { display: none; }
    nav { flex-direction: row; overflow-x: auto; }
    nav a { flex: none; }
    .monitor { padding: 22px; }
    nav b { display: none; }
  }
</style>
