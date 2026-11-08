<script lang="ts">
  // Todo lo de los robots se ve como una consola de hardware (lib/consola).
  import '$lib/consola/consola.css';
  let { children } = $props();
</script>

<svelte:head>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="consola">{@render children()}</div>
