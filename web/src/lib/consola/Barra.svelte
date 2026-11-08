<script lang="ts">
  // La barra de arriba de la consola: la marca, lo que pase en el medio y la cuenta.
  import type { Snippet } from 'svelte';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { conexionEnVivo } from '$lib/eventos.svelte';

  let { izquierda, medio }: { izquierda?: Snippet; medio?: Snippet } = $props();
  let abierto = $state(false);
  const usuario = $derived(page.data.usuario as { nombre: string; email: string; admin: boolean } | null);

  async function salir() {
    await api('/v1/sesion', { metodo: 'DELETE' }).catch(() => {});
    await invalidateAll();
    goto('/entrar');
  }
</script>

<header class="barra caja">
  <div class="lado">
    {#if izquierda}{@render izquierda()}{:else}
      <a class="marca" href="/robots" aria-label="Sintética, mis robots">
        <svg width="36" height="22" viewBox="0 0 40 26" aria-hidden="true"><rect x="4" y="3" width="11" height="20" rx="5.5" fill="#ff5a1f" /><path d="M25 8 L36 3 V18 A5.5 5.5 0 0 1 25 18 Z" fill="#ff5a1f" /></svg>
        <span>SINTÉTICA</span>
      </a>
    {/if}
  </div>
  {#if medio}<div class="medio">{@render medio()}</div>{/if}
  <div class="lado derecha">
    {#if usuario}
      <div class="cuenta">
        <button class="tecla negra chica" aria-expanded={abierto} onclick={() => (abierto = !abierto)}>
          <span class="led" class:verde={conexionEnVivo.conectado} title={conexionEnVivo.conectado ? 'En vivo' : 'Reconectando…'}></span><span class="nombre">{usuario.nombre || usuario.email.split('@')[0]}</span>
        </button>
        {#if abierto}
          <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
          <div class="menu chapa" onclick={() => (abierto = false)}>
            <span class="grabado">{usuario.email}</span>
            <a href="/robots">Mis robots</a>
            <a href="/caras">Biblioteca de caras</a>
            <a href="/animaciones">Biblioteca de animaciones</a>
            {#if usuario.admin}<a href="/firmwares">Firmwares</a><a href="/reservas">Reservas</a>{/if}
            <button onclick={salir}>Salir</button>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  .barra { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding-top: 24px; padding-bottom: 24px; position: relative; z-index: 5; }
  .lado { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .derecha { justify-content: flex-end; }
  .medio { flex: 1; display: flex; justify-content: center; min-width: 0; }
  .marca { display: flex; align-items: center; gap: 10px; text-decoration: none; font: 900 22px/1 var(--marca); font-stretch: 125%; letter-spacing: -.02em; }
  .cuenta { position: relative; }
  .nombre { max-width: 140px; overflow: hidden; text-overflow: ellipsis; }
  .menu { position: absolute; right: 0; top: calc(100% + 14px); display: flex; flex-direction: column; gap: 2px; min-width: 240px; padding: 14px; border-radius: 18px; }
  .menu .grabado { padding: 6px 10px 10px; text-transform: none; letter-spacing: .02em; font-size: 12px; overflow: hidden; text-overflow: ellipsis; }
  .menu a, .menu button { display: block; width: 100%; text-align: left; padding: 11px 10px; border: 0; border-radius: 10px; background: none; color: var(--c-papel);
    font: 700 15px/1 var(--marca); text-decoration: none; cursor: pointer; }
  .menu a:hover, .menu button:hover { background: rgba(255,255,255,.06); }
  @media (max-width: 560px) { .marca span { display: none; } }
</style>
