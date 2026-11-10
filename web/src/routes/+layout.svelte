<script lang="ts">
  import '../app.css';
  import { page } from '$app/state';
  import { goto, invalidateAll } from '$app/navigation';
  import { api } from '$lib/api';

  let { data, children } = $props();

  const secciones = [
    { ruta: '/robots', nombre: 'Mis robots' },
    { ruta: '/caras', nombre: 'Caras' },
    { ruta: '/animaciones', nombre: 'Animaciones' },
  ];
  const activa = (ruta: string) => page.url.pathname.startsWith(ruta);
  // La landing (/) y la consola de los robots (/robots) tienen su propia barra y su propio diseño.
  const landing = $derived(page.url.pathname === '/' || page.url.pathname.startsWith('/piezas') || page.url.pathname.startsWith('/robots') || page.url.pathname === '/entrar' ||
    ['/caras', '/animaciones', '/firmwares', '/reservas', '/admin'].some((r) => page.url.pathname.startsWith(r)));

  async function salir() {
    await api('/v1/sesion', { metodo: 'DELETE' }).catch(() => {});
    await invalidateAll();
    goto('/entrar');
  }
</script>

<svelte:head><title>Sintética</title></svelte:head>

{#if landing}
  {@render children()}
{:else}
{#if data.usuario}
  <header>
    <div class="contenedor barra">
      <a class="marca" href="/robots" aria-label="Sintética, inicio">
        <svg viewBox="0 0 40 24" width="34" height="20" aria-hidden="true"><rect x="3" y="2" width="11" height="20" rx="5.5" fill="var(--naranja)"/><rect x="26" y="2" width="11" height="20" rx="5.5" fill="var(--naranja)"/></svg>
        <span>Sintética</span>
      </a>
      <nav aria-label="Secciones">
        {#each secciones as s}
          <a href={s.ruta} aria-current={activa(s.ruta) ? 'page' : undefined}>{s.nombre}</a>
        {/each}
        {#if data.usuario.admin}
          <a href="/admin" aria-current={activa('/admin') ? 'page' : undefined}>Control</a>
          <a href="/firmwares" aria-current={activa('/firmwares') ? 'page' : undefined}>Firmwares</a>
          <a href="/reservas" aria-current={activa('/reservas') ? 'page' : undefined}>Reservas</a>
        {/if}
      </nav>
      <div class="usuario">
        <span class="etiqueta" title={data.usuario.email}>{data.usuario.nombre || data.usuario.email || 'Vos'}</span>
        <button class="chico" onclick={salir}>Salir</button>
      </div>
    </div>
  </header>
{/if}

<main>
  {@render children()}
</main>
{/if}

<style>
  header { border-bottom: 1px solid var(--linea); background: color-mix(in srgb, var(--papel) 88%, transparent); backdrop-filter: blur(8px); position: sticky; top: 0; z-index: 10; }
  .barra { display: flex; align-items: center; gap: 24px; height: 64px; }
  .marca { display: flex; align-items: center; gap: 10px; color: var(--tinta); text-decoration: none; font: 600 21px var(--serif); letter-spacing: -0.02em; }
  nav { display: flex; gap: 4px; flex: 1; overflow-x: auto; scrollbar-width: none; }
  nav a { font: 600 12px/1 var(--mono); text-transform: uppercase; letter-spacing: 0.07em; color: var(--tinta-suave); text-decoration: none; padding: 10px 12px; border-radius: 999px; white-space: nowrap; }
  nav a[aria-current='page'] { color: var(--tinta); background: var(--papel-2); }
  .usuario { display: flex; align-items: center; gap: 10px; }
  .usuario .etiqueta { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  main { padding: 32px 0 80px; }
  @media (max-width: 720px) {
    .barra { gap: 12px; height: auto; padding-top: 10px; padding-bottom: 10px; flex-wrap: wrap; }
    nav { order: 3; flex-basis: 100%; }
    .usuario .etiqueta { display: none; }
  }
</style>
