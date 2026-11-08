<script lang="ts">
  import { onMount } from 'svelte';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import QRCode from 'qrcode';
  import { api, ErrorApi } from '$lib/api';
  import '$lib/consola/consola.css';
  import Producto from '$lib/consola/Producto.svelte';
  import { modeloDe } from '$lib/consola/modelo';

  let { data } = $props();
  // La foto del robot con su cara, como en la consola.
  const modelo = modeloDe({ id: '', perfil: 'unidad', apodo: '', virtual: false, conectado: true, ultimaConexion: null, estadoEn: null, creado: '', estado: null, sincro: { config: '', imagenes: '' }, config: null });

  type Metodos = { email: boolean; telegram: boolean; bot?: string };
  // Cómo se entra en este servidor (sin SMTP no se ofrece el mail).
  let metodos = $state<Metodos | null>(null);
  type Paso = 'inicio' | 'telegram' | 'mail' | 'mailEnviado';
  let paso = $state<Paso>('inicio');

  let email = $state('');
  let codigo = $state('');
  let ocupado = $state(false);
  let error = $state('');
  const volver = $derived(page.url.searchParams.get('volver') ?? '/robots');

  // Telegram: el link (y su QR), y si ya tocó Start.
  let token = $state('');
  let enlace = $state('');
  let qr = $state('');
  let confirmar = $state(false);
  let vuelta = 0;

  onMount(() => {
    api<Metodos>('/v1/sesion/metodos')
      .then((m) => {
        metodos = m;
        if (!m.telegram && m.email) paso = 'mail';
      })
      .catch(() => {
        metodos = { email: true, telegram: false };
        paso = 'mail';
      });
    return () => vuelta++;
  });

  $effect(() => {
    if (!enlace) { qr = ''; return; }
    QRCode.toString(enlace, { type: 'svg', margin: 1, color: { dark: '#1a1714', light: '#f3eee8' } }).then((s) => (qr = s));
  });

  function adentro() {
    invalidateAll().then(() => goto(volver.startsWith('/') && !volver.startsWith('//') ? volver : '/robots'));
  }

  function fallo(err: unknown) {
    error = err instanceof ErrorApi ? err.message : 'Algo salió mal.';
  }

  async function conTelegram() {
    ocupado = true;
    error = '';
    try {
      const r = await api<{ token: string; enlace: string }>('/v1/sesion/telegram', { cuerpo: { cliente: 'web' } });
      token = r.token;
      enlace = r.enlace;
      confirmar = false;
      codigo = '';
      paso = 'telegram';
      esperar(++vuelta, r.token);
    } catch (err) {
      fallo(err);
    } finally {
      ocupado = false;
    }
  }

  // Espera a que toque Start (la API contesta en cuanto pasa, o a los 25 s).
  async function esperar(mia: number, t: string) {
    while (mia === vuelta && paso === 'telegram') {
      try {
        const r = await api<{ usuario?: unknown; estado?: string }>(`/v1/sesion/telegram/${encodeURIComponent(t)}?esperar=25`);
        if (mia !== vuelta) return;
        if (r.usuario) return adentro();
        confirmar = r.estado === 'confirmar';
      } catch (err) {
        if (mia !== vuelta) return;
        if (err instanceof ErrorApi && (err.estado === 410 || err.estado === 401)) {
          error = 'Venció. Pedí otro.';
          paso = 'inicio';
          return;
        }
        await new Promise((ok) => setTimeout(ok, 3000));
      }
    }
  }

  async function codigoTelegram(e: Event) {
    e.preventDefault();
    ocupado = true;
    error = '';
    try {
      await api(`/v1/sesion/telegram/${encodeURIComponent(token)}/codigo`, { cuerpo: { codigo } });
      vuelta++;
      adentro();
    } catch (err) {
      fallo(err);
    } finally {
      ocupado = false;
    }
  }

  function cancelar() {
    vuelta++;
    paso = 'inicio';
    error = '';
  }

  async function pedir(e: Event) {
    e.preventDefault();
    ocupado = true;
    error = '';
    try {
      await api('/v1/sesion/enlace', { cuerpo: { email, cliente: 'web', volver } });
      codigo = '';
      paso = 'mailEnviado';
    } catch (err) {
      fallo(err);
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
      adentro();
    } catch (err) {
      fallo(err);
    } finally {
      ocupado = false;
    }
  }

  const titulo = $derived(paso === 'mailEnviado' ? 'Fijate tu mail.' : paso === 'telegram' ? (confirmar ? 'Confirmá en Telegram.' : 'Tocá Start.') : 'Entrá.');
</script>

<svelte:head>
  <title>Entrar · Sintética</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="consola">
  <main class="caja entrar">
    <div class="robot"><Producto {modelo} config={{ version: 2 }} visible={paso === 'mailEnviado' || paso === 'telegram' ? 'enamorado' : 'contento'} /></div>
    <div class="formulario">
      <span class="grabado" style="color: var(--brasa)">Sintética</span>
      <h1 class="titular">{titulo}</h1>
      {#if paso === 'inicio' && metodos}
        <div class="opciones">
          {#if metodos.telegram}
            <button class="tecla naranja ancha grande" disabled={ocupado} onclick={conTelegram}>
              <svg class="avion" viewBox="0 0 24 24" aria-hidden="true"><path d="M21.5 3.5 2.8 10.7c-1 .4-1 1.8.1 2.1l4.6 1.4 1.8 5.6c.3.9 1.4 1.1 2 .4l2.6-2.6 4.8 3.5c.8.6 1.9.1 2.1-.9l3-15.2c.2-1.1-.9-2-2.3-1.5ZM9.6 14.4l8.6-7.6-6.9 9-.3 3.1-1.4-4.5Z" fill="currentColor" /></svg>
              {ocupado ? 'Un segundo…' : 'Entrar con Telegram'}
            </button>
          {/if}
          {#if metodos.email}
            <button class="tecla negra" onclick={() => { paso = 'mail'; error = ''; }}>Con mi email</button>
          {/if}
          <p class="texto">Sin contraseñas.</p>
        </div>
      {:else if paso === 'telegram'}
        <div class="opciones">
          <a class="tecla naranja ancha grande" href={enlace} target="_blank" rel="noreferrer">Abrir Telegram</a>
          {#if qr}<div class="qr" role="img" aria-label="Código QR para entrar con Telegram">{@html qr}</div>{/if}
          <form onsubmit={codigoTelegram}>
            <label class="visor"><span class="grabado">¿Te llegó un código?</span>
              <input class="campo codigo" inputmode="numeric" autocomplete="one-time-code" bind:value={codigo} placeholder="123 456" maxlength="7" /></label>
            <div class="botones">
              <button class="tecla naranja ancha" disabled={ocupado || codigo.replace(/\D/g, '').length !== 6}>Entrar</button>
              <button class="tecla negra" type="button" onclick={cancelar}>Volver</button>
            </div>
          </form>
        </div>
      {:else if paso === 'mail'}
        <form onsubmit={pedir}>
          <label class="visor"><span class="grabado">Tu email</span>
            <!-- svelte-ignore a11y_autofocus -->
            <input class="campo" type="email" required autocomplete="email" bind:value={email} placeholder="vos@ejemplo.com" autofocus /></label>
          <button class="tecla naranja ancha" disabled={ocupado || !email}>{ocupado ? 'Mandando…' : 'Mandame el link'}</button>
          {#if metodos?.telegram}<button class="tecla negra" type="button" onclick={cancelar}>Volver</button>{:else}<p class="texto">Sin contraseñas.</p>{/if}
        </form>
      {:else if paso === 'mailEnviado'}
        <form onsubmit={canjear}>
          <p class="texto">Un link a <b>{email}</b>, o este código:</p>
          <label class="visor"><span class="grabado">Código</span>
            <input class="campo codigo" required autocomplete="one-time-code" bind:value={codigo} placeholder="ABCD-EFGH" maxlength="9" /></label>
          <div class="botones">
            <button class="tecla naranja ancha" disabled={ocupado || codigo.length < 8}>Entrar</button>
            <button class="tecla negra" type="button" onclick={() => { paso = 'mail'; codigo = ''; }}>Otro email</button>
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
  form, .opciones { display: flex; flex-direction: column; gap: 16px; }
  .ancha { width: 100%; min-height: 60px; }
  .grande { min-height: 76px; font-size: 15px; gap: 12px; text-decoration: none; display: inline-flex; align-items: center; justify-content: center; }
  .avion { width: 26px; height: 26px; }
  .codigo { text-transform: uppercase; letter-spacing: .12em; }
  .botones { display: grid; grid-template-columns: 1fr auto; gap: 12px; }
  .texto a { color: var(--brasa); }
  .qr { align-self: center; width: 180px; padding: 10px; border-radius: 18px; background: #f3eee8; box-shadow: 0 6px 0 #a39c90; }
  .qr :global(svg) { display: block; width: 100%; height: auto; }
  @media (max-width: 860px) { .entrar { grid-template-columns: 1fr; gap: 28px; } .robot { max-width: 260px; width: 100%; margin: 0 auto; } .qr { display: none; } }
</style>
