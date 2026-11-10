<script lang="ts">
  // El centro de control del dueño: la barra de la consola, el titular y el
  // teclado de módulos. Solo administradores (la API lo vuelve a chequear).
  import '$lib/consola/consola.css';
  import '$lib/control/control.css';
  import Barra from '$lib/consola/Barra.svelte';
  import { page } from '$app/state';
  import { MODULOS } from '$lib/control/control';

  let { data, children } = $props();
  // En el celu el teclado es una tira: que se vea la tecla del módulo abierto.
  $effect(() => {
    page.url.pathname;
    document.querySelector('.control .teclado [aria-current="page"]')?.scrollIntoView({ inline: 'center', block: 'nearest' });
  });
  const actual = (ruta: string) => (ruta === '/admin' ? page.url.pathname === '/admin' : page.url.pathname.startsWith(ruta));
</script>

<svelte:head>
  <title>Control · Sintética</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="consola control">
  <Barra />
  <main class="caja">
    <h1 class="titular">Control</h1>
    {#if !data.usuario?.admin}
      <p class="vacio"><span class="led on"></span>Solo administradores</p>
    {:else}
      <nav class="placa-naranja" aria-label="Módulos del control">
        <div class="teclado">
          {#each MODULOS as m, i}
            <a class="modulo" href={m.ruta} aria-current={actual(m.ruta) ? 'page' : undefined} data-sveltekit-noscroll>
              <span class="arriba"><span class="n">{String(i + 1).padStart(2, '0')}</span><span class="led"></span></span>
              <b>{m.nombre}</b>
            </a>
          {/each}
        </div>
      </nav>
      {@render children()}
    {/if}
  </main>
</div>

<style>
  main { display: flex; flex-direction: column; gap: 32px; padding-top: 4px; padding-bottom: 64px; }
  .titular { font-size: clamp(44px, 7vw, 88px); }
</style>
