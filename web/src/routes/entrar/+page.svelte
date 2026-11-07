<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ErrorApi } from '$lib/api';

  let email = $state('');
  let codigo = $state('');
  let enviado = $state(false);
  let ocupado = $state(false);
  let error = $state('');
  const volver = $derived(page.url.searchParams.get('volver') ?? '/');

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
      goto(volver.startsWith('/') && !volver.startsWith('//') ? volver : '/');
    } catch (err) {
      error = err instanceof ErrorApi ? err.message : 'Algo salió mal.';
    } finally {
      ocupado = false;
    }
  }
</script>

<svelte:head><title>Entrar · Sintética</title></svelte:head>

<div class="contenedor entrar">
  <div class="cara" aria-hidden="true">
    <svg viewBox="0 0 320 172"><rect width="320" height="172" rx="14" fill="#0B0A09"/><rect x="88" y="46" width="44" height="80" rx="22" fill="#FF6A33"/><rect x="188" y="46" width="44" height="80" rx="22" fill="#FF6A33"/></svg>
  </div>
  <div class="tarjeta formulario">
    <h1>Entrá a Sintética</h1>
    {#if !enviado}
      <p class="nota">Te mandamos un link por mail. Sin contraseñas.</p>
      <form onsubmit={pedir} class="grilla">
        <label><span class="etiqueta">Tu email</span>
          <input type="email" required autocomplete="email" bind:value={email} placeholder="vos@ejemplo.com" />
        </label>
        <button class="primario" disabled={ocupado || !email}>{ocupado ? 'Mandando…' : 'Mandame el link'}</button>
      </form>
    {:else}
      <p>Listo: si <b>{email}</b> está bien, te llegó un link. Vale 15 minutos.</p>
      <p class="nota">En tu Mac, con Docker, los mails llegan a <a href="http://localhost:8025" target="_blank" rel="noreferrer">Mailpit (localhost:8025)</a>.</p>
      <form onsubmit={canjear} class="grilla">
        <label><span class="etiqueta">O escribí el código del mail</span>
          <input required autocomplete="one-time-code" bind:value={codigo} placeholder="ABCD-EFGH" maxlength="9" style="text-transform: uppercase; letter-spacing: 0.1em" />
        </label>
        <div class="fila">
          <button class="primario" disabled={ocupado || codigo.length < 8}>Entrar</button>
          <button type="button" onclick={() => { enviado = false; codigo = ''; }}>Otro email</button>
        </div>
      </form>
    {/if}
    {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
  </div>
</div>

<style>
  .entrar { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; align-items: center; min-height: 70vh; }
  .cara svg { width: 100%; max-width: 460px; display: block; margin: 0 auto; filter: drop-shadow(0 30px 40px rgba(0,0,0,.25)); }
  .formulario { max-width: 440px; display: grid; gap: 16px; }
  .formulario p { margin: 0; }
  @media (max-width: 760px) { .entrar { grid-template-columns: 1fr; gap: 24px; min-height: 0; } .cara svg { max-width: 300px; } }
</style>
