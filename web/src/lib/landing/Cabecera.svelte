<script lang="ts">
  // La barra de arriba de la landing y de las páginas de las piezas. En otra
  // página, los links llevan a las secciones de la landing.
  let { usuario = null, base = '', soloLanding = false }: { usuario?: unknown; base?: string; soloLanding?: boolean } = $props();
</script>

<header class="contenedor nav">
  <a class="marca" href={base || '#inicio'} aria-label="Sintética, inicio">
    <svg width="40" height="24" viewBox="0 0 40 26" aria-hidden="true"><rect x="4" y="3" width="11" height="20" rx="5.5" fill="currentColor" /><path d="M25 8 L36 3 V18 A5.5 5.5 0 0 1 25 18 Z" fill="currentColor" /></svg>
    <span>SINTÉTICA</span>
  </a>
  <nav aria-label="Secciones">
    <a class="ficha" href="{base}#piezas">Unidades</a>
    <a class="ficha" href="{base}#app">App</a>
    {#if !soloLanding}<a class="ficha" href={usuario ? '/robots' : '/entrar'}>{usuario ? 'Mis robots' : 'Entrar'}</a>{/if}
  </nav>
  <a class="inflado" href="{base}#reservar">Reservar</a>
</header>
