<script lang="ts">
  // Una página suelta de la cuenta (bibliotecas, admin) con el marco de la
  // consola: su barra, su letra y su paleta. Lo de adentro usa las variables
  // de siempre, remapeadas (clase .viejo).
  import type { Snippet } from 'svelte';
  import './consola.css';
  import Barra from './Barra.svelte';

  let { titulo, children }: { titulo: string; children: Snippet } = $props();
</script>

<svelte:head>
  <title>{titulo} · Sintética</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="consola">
  <Barra />
  <main class="caja">
    <h1 class="titular">{titulo}</h1>
    <div class="viejo contenido">{@render children()}</div>
  </main>
</div>

<style>
  main { display: flex; flex-direction: column; gap: 32px; padding-top: 4px; padding-bottom: 64px; }
  .titular { font-size: clamp(44px, 7vw, 88px); }
</style>
