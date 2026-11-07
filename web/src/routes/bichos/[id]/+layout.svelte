<script lang="ts">
  import { onDestroy } from 'svelte';
  import { page } from '$app/state';
  import { BichoAbierto, darBicho } from '$lib/bicho.svelte';
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import EstadoChip from '$lib/componentes/EstadoChip.svelte';
  import { pantallaDe } from '$lib/tipos';

  let { children } = $props();
  const b = new BichoAbierto(page.params.id.toUpperCase());
  darBicho(b);
  b.cargar();
  b.escucharEventos();
  onDestroy(() => b.cerrar());

  const base = $derived(`/bichos/${b.id}`);
  const pestanas = [
    { ruta: '', nombre: 'Resumen' },
    { ruta: '/config', nombre: 'Config' },
    { ruta: '/caras', nombre: 'Caras' },
    { ruta: '/animaciones', nombre: 'Animaciones' },
    { ruta: '/reglas', nombre: 'Reglas' },
    { ruta: '/imagenes', nombre: 'Imágenes' },
    { ruta: '/actualizar', nombre: 'Firmware' },
    { ruta: '/historial', nombre: 'Historial' },
  ];
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });
  const nombre = $derived(b.bicho?.apodo || b.bicho?.estado?.nombre || b.config.nombre || 'Bicho');
</script>

<svelte:head><title>{nombre} · Sintética</title></svelte:head>

<div class="contenedor">
  <a class="volver etiqueta" href="/">← Mis bichos</a>
  {#if b.error}
    <p class="aviso-caja error">{b.error}</p>
  {:else if b.bicho}
    <section class="cabecera">
      <Pantalla config={b.config} visible={b.bicho.estado?.expresion ?? ''} redonda={pantalla.redonda} ancho={pantalla.redonda ? 220 : 360} />
      <div class="info">
        <p class="etiqueta">{b.bicho.perfil}{b.bicho.virtual ? ' · virtual' : ''} · {b.id}</p>
        <h1>{nombre}</h1>
        <div class="fila"><EstadoChip bicho={b.bicho} />
          {#if b.bicho.estado?.wifi}<span class="chip ok">WiFi</span>{/if}
        </div>
        <dl>
          <div><dt class="etiqueta">Se ve</dt><dd>{b.bicho.estado?.expresion ?? '—'}{b.bicho.estado?.fija ? ' (fija)' : ''}</dd></div>
          <div><dt class="etiqueta">Humor</dt><dd>{b.bicho.estado ? (b.bicho.estado.humor >= 0 ? '+' : '') + b.bicho.estado.humor.toFixed(2) : '—'}</dd></div>
          <div><dt class="etiqueta">Batería</dt><dd>{b.bicho.estado?.bateria != null ? b.bicho.estado.bateria + '%' : 'sin batería'}</dd></div>
          <div><dt class="etiqueta">Firmware</dt><dd>{b.bicho.estado?.version ?? '—'}</dd></div>
          <div><dt class="etiqueta">Config</dt><dd>v{b.versionConfig}{b.autorConfig ? ` (${b.autorConfig})` : ''} · {b.bicho.sincro?.config ?? '—'}</dd></div>
        </dl>
      </div>
    </section>
    <nav class="pestanas" aria-label="Secciones del bicho">
      {#each pestanas as p}
        <a href={base + p.ruta} aria-current={page.url.pathname === base + p.ruta ? 'page' : undefined}>{p.nombre}</a>
      {/each}
    </nav>
    <div class="contenido">{@render children()}</div>
  {:else}
    <p class="etiqueta">Cargando…</p>
  {/if}
</div>

<style>
  .volver { display: inline-block; margin-bottom: 18px; text-decoration: none; }
  .cabecera { display: flex; gap: 32px; align-items: center; flex-wrap: wrap; margin-bottom: 24px; }
  .info { display: grid; gap: 10px; flex: 1; min-width: 260px; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin: 6px 0 0; }
  dl div { border-left: 2px solid var(--linea); padding-left: 10px; }
  dd { margin: 2px 0 0; font-family: var(--mono); font-size: 14px; }
  .contenido { padding-top: 24px; }
</style>
