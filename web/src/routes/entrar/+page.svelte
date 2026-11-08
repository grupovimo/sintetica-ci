<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ErrorApi } from '$lib/api';
  import '$lib/consola/consola.css';
  import Producto from '$lib/consola/Producto.svelte';
  import { modeloDe } from '$lib/consola/modelo';

  let { data } = $props();
  // La foto del robot con su cara, como en la consola.
  const modelo = modeloDe({ id: '', perfil: 'unidad', apodo: '', virtual: false, conectado: true, ultimaConexion: null, estadoEn: null, creado: '', estado: null, sincro: { config: '', imagenes: '' }, config: null });

  let email = $state('');
  let codigo = $state('');
  let enviado = $state(false);
  let ocupado = $state(false);
  let error = $state('');
  const volver = $derived(page.url.searchParams.get('volver') ?? '/robots');

  async function pedir(e: Event) {
    e.preventDefault();
    ocupado = true;
    error = '';
    try {
      await api('/v1/sesion/enlace', { cuerpo: { email, cliente: 'web', volver } });
      enviado = true;
    } catch (err) {
      error = err instanceof ErrorApi ? err.message : 'Algo salió mal.';
    } finally {
      ocupado = false;
    }
  }

  async function canjear(e: Event) {
    e.preventDefault();
    ocupado = true;
    error = '';
    try {
      await api('/v1/sesion/canjear', { cuerpo: { email, codigo } });
      await invalidateAll();
      goto(volver.startsWith('/') && !volver.startsWith('//') ? volver : '/robots');
    } catch (err) {
      error = err instanceof ErrorApi ? err.message : 'Algo salió mal.';
    } finally {
      ocupado = false;
    }
  }
</script>

<svelte:head>
  <title>Entrar · Sintética</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="consola">
  <main class="caja entrar">
    <div class="robot"><Producto {modelo} config={{ version: 2 }} visible={enviado ? 'enamorado' : 'contento'} /></div>
    <div class="formulario">
      <span class="grabado" style="color: var(--brasa)">Sintética</span>
      <h1 class="titular">{enviado ? 'Fijate tu mail.' : 'Entrá.'}</h1>
      {#if !enviado}
        <form onsubmit={pedir}>
          <label class="visor"><span class="grabado">Tu email</span>
            <!-- svelte-ignore a11y_autofocus -->
            <input class="campo" type="email" required autocomplete="email" bind:value={email} placeholder="vos@ejemplo.com" autofocus /></label>
          <button class="tecla naranja ancha" disabled={ocupado || !email}>{ocupado ? 'Mandando…' : 'Mandame el link'}</button>
          <p class="texto">Sin contraseñas.</p>
        </form>
      {:else}
        <form onsubmit={canjear}>
          <p class="texto">Un link a <b>{email}</b>, o este código:</p>
          <label class="visor"><span class="grabado">Código</span>
            <input class="campo codigo" required autocomplete="one-time-code" bind:value={codigo} placeholder="ABCD-EFGH" maxlength="9" /></label>
          <div class="botones">
            <button class="tecla naranja ancha" disabled={ocupado || codigo.length < 8}>Entrar</button>
            <button class="tecla negra" type="button" onclick={() => { enviado = false; codigo = ''; }}>Otro email</button>
          </div>
          {#if data.desarrollo}<p class="texto"><a href="http://localhost:8025" target="_blank" rel="noreferrer">Mailpit</a></p>{/if}
        </form>
      {/if}
      {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}
    </div>
  </main>
</div>

<style>
  .entrar { min-height: 100vh; display: grid; grid-template-columns: minmax(0, 380px) minmax(0, 460px); gap: 64px; align-items: center; justify-content: center; padding-top: 40px; padding-bottom: 40px; }
  .formulario { display: flex; flex-direction: column; gap: 22px; }
  form { display: flex; flex-direction: column; gap: 16px; }
  .ancha { width: 100%; min-height: 60px; }
  .codigo { text-transform: uppercase; letter-spacing: .12em; }
  .botones { display: grid; grid-template-columns: 1fr auto; gap: 12px; }
  .texto a { color: var(--brasa); }
  @media (max-width: 860px) { .entrar { grid-template-columns: 1fr; gap: 28px; } .robot { max-width: 260px; width: 100%; margin: 0 auto; } }
</style>
