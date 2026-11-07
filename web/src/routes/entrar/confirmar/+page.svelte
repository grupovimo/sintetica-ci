<script lang="ts">
  import { onMount } from 'svelte';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ErrorApi } from '$lib/api';

  let error = $state('');

  onMount(async () => {
    const token = page.url.searchParams.get('t') ?? '';
    const volver = page.url.searchParams.get('volver') ?? '/';
    try {
      await api('/v1/sesion/canjear', { cuerpo: { token } });
      await invalidateAll();
      goto(volver.startsWith('/') && !volver.startsWith('//') ? volver : '/', { replaceState: true });
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'Algo salió mal.';
    }
  });
</script>

<div class="contenedor" style="max-width: 520px">
  {#if error}
    <div class="tarjeta grilla">
      <h1>Ese link no sirve</h1>
      <p class="aviso-caja error">{error}</p>
      <a class="boton primario" href="/entrar" style="justify-self: start">Pedir otro</a>
    </div>
  {:else}
    <p class="etiqueta">Entrando…</p>
  {/if}
</div>
