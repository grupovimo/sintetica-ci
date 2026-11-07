<script lang="ts">
  // El link del mail que pidió la app: abre la app con el token.
  import { page } from '$app/state';
  const token = $derived(page.url.searchParams.get('t') ?? '');
  const enlace = $derived(`sintetica://entrar?t=${encodeURIComponent(token)}`);
</script>

<svelte:head><title>Abrir la app · Sintética</title></svelte:head>

<div class="contenedor" style="max-width: 520px">
  <div class="tarjeta grilla">
    <h1>Abrí la app</h1>
    <p>Este link es para entrar en la app de Sintética del celu donde lo pediste.</p>
    <a class="boton primario" href={enlace} style="justify-self: start">Abrir Sintética</a>
    <p class="nota">Si no abre, en la app tocá "Tengo un código" y escribí el código del mail.</p>
  </div>
</div>
