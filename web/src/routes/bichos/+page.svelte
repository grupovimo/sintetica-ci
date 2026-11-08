<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { escuchar } from '$lib/eventos.svelte';
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import EstadoChip from '$lib/componentes/EstadoChip.svelte';
  import { pantallaDe, type Bicho } from '$lib/tipos';

  let { data } = $props();

  let bichos = $state<Bicho[]>([]);
  let configs = $state<Record<string, unknown>>({});
  let cargando = $state(true);
  let error = $state('');
  let sumando = $state(false);
  let virtuales = $state<{ id: string; perfil: string; sumado: boolean }[]>([]);
  let sumandoVirtual = $state('');
  let errorSumar = $state('');

  async function cargar() {
    try {
      const r = await api<{ bichos: Bicho[] }>('/v1/bichos');
      bichos = r.bichos;
      for (const b of bichos) cargarConfig(b.id);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude cargar tus bichos.';
    } finally {
      cargando = false;
    }
  }

  async function cargarConfig(id: string) {
    try {
      const c = await api<{ config: unknown }>(`/v1/bichos/${id}/config`);
      configs[id] = c.config;
    } catch {
      /* se ve con la config de fábrica */
    }
  }

  onMount(() => {
    cargar();
    return escuchar((e) => {
      const b = bichos.find((x) => x.id === e.bicho);
      if (e.tipo === 'bicho' && e.datos.accion !== 'actual') cargar();
      if (!b) return;
      if (e.tipo === 'estado') b.estado = e.datos;
      if (e.tipo === 'conexion') b.conectado = e.datos.conectado;
      if (e.tipo === 'sincro') b.sincro = e.datos;
      if (e.tipo === 'config') cargarConfig(b.id);
    });
  });

  async function abrirSumar() {
    sumando = true;
    errorSumar = '';
    if (data.desarrollo) {
      try {
        virtuales = (await api<{ virtuales: typeof virtuales }>('/v1/bichos/virtuales')).virtuales;
      } catch {
        virtuales = [];
      }
    }
  }

  async function sumarVirtual(id: string) {
    sumandoVirtual = id;
    errorSumar = '';
    try {
      await api('/v1/bichos/reclamar-virtual', { cuerpo: { id } });
      sumando = false;
      await cargar();
    } catch (e) {
      errorSumar = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude sumarlo.';
    } finally {
      sumandoVirtual = '';
    }
  }

  const nombreDe = (b: Bicho) => b.apodo || b.estado?.nombre || 'Bicho sin nombre';
</script>

<svelte:head><title>Mis bichos · Sintética</title></svelte:head>

<div class="contenedor">
  <div class="cabecera">
    <div>
      <p class="etiqueta">Tu cuenta</p>
      <h1>Mis bichos</h1>
    </div>
    <button class="primario" onclick={abrirSumar}>Sumar un bicho</button>
  </div>

  {#if error}<p class="aviso-caja error">{error}</p>{/if}

  {#if cargando}
    <p class="etiqueta">Cargando…</p>
  {:else if bichos.length === 0}
    <div class="vacio tarjeta">
      <h2>Todavía no sumaste ningún bicho</h2>
      <p class="nota">Con la app, cerca del bicho: vinculalo por BLE, pasale el WiFi y sumalo a tu cuenta. Desde acá después lo cambiás estés donde estés.</p>
      <button class="primario" onclick={abrirSumar}>Cómo se suma</button>
    </div>
  {:else}
    <div class="lista">
      {#each bichos as b (b.id)}
        {@const p = pantallaDe(b)}
        <a class="bicho tarjeta" href="/bichos/{b.id}">
          <div class="vista">
            <Pantalla config={configs[b.id] ?? { version: 1 }} visible={b.estado?.expresion ?? ''} redonda={p.redonda} ancho={p.redonda ? 150 : 250} />
          </div>
          <div class="datos">
            <h3>{nombreDe(b)}</h3>
            <div class="fila">
              <EstadoChip bicho={b} />
              {#if b.virtual}<span class="chip">virtual</span>{/if}
            </div>
            <p class="etiqueta">{b.perfil || 'perfil desconocido'} · {b.estado?.version ?? '—'}{#if b.estado?.bateria != null} · {b.estado.bateria}%{/if}</p>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>

{#if sumando}
  <div class="fondo" role="presentation" onclick={(e) => e.target === e.currentTarget && (sumando = false)}>
    <div class="tarjeta dialogo" role="dialog" aria-modal="true" aria-labelledby="t-sumar">
      <h2 id="t-sumar">Sumar un bicho</h2>
      <ol>
        <li>Prendé el bicho: muestra un QR (si no, mantené el botón 5 s; en el broche, 8 s).</li>
        <li>En la app, con esta misma cuenta, escaneá el QR.</li>
        <li>En la pantalla del bicho, tocá <b>Sumar a mi cuenta</b> y pasale tu WiFi.</li>
      </ol>
      <p class="nota">El bicho queda en tu cuenta cuando se conecta a la nube con la clave que le da la app: así nadie puede sumar un bicho que no tiene en la mano.</p>
      {#if data.desarrollo}
        <div class="virtuales">
          <p class="etiqueta">Bichos virtuales (solo en local)</p>
          {#each virtuales as v}
            <div class="fila virtual">
              <span class="mono">{v.id}</span><span class="chip">{v.perfil}</span>
              {#if v.sumado}<span class="chip ok">en tu cuenta</span>
              {:else}<button class="chico" disabled={!!sumandoVirtual} onclick={() => sumarVirtual(v.id)}>{sumandoVirtual === v.id ? 'Sumando…' : 'Sumar'}</button>{/if}
            </div>
          {:else}
            <p class="nota">No encontré bichos virtuales. ¿Está andando <code>docker compose</code>?</p>
          {/each}
        </div>
      {/if}
      {#if errorSumar}<p class="aviso-caja error">{errorSumar}</p>{/if}
      <div class="fila" style="justify-content: flex-end"><button onclick={() => (sumando = false)}>Cerrar</button></div>
    </div>
  </div>
{/if}

<style>
  .cabecera { display: flex; justify-content: space-between; align-items: end; gap: 16px; margin-bottom: 28px; flex-wrap: wrap; }
  .lista { display: grid; gap: 20px; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); }
  .bicho { display: grid; gap: 16px; color: inherit; text-decoration: none; transition: transform 0.15s, border-color 0.15s; }
  .bicho:hover { transform: translateY(-2px); border-color: var(--tinta-tenue); }
  .vista { display: grid; place-items: center; background: var(--papel-2); border-radius: 12px; padding: 18px; min-height: 180px; }
  .datos { display: grid; gap: 8px; }
  .vacio { display: grid; gap: 12px; justify-items: start; max-width: 620px; }
  .fondo { position: fixed; inset: 0; background: rgba(11, 10, 9, 0.5); display: grid; place-items: center; padding: 16px; z-index: 50; }
  .dialogo { max-width: 560px; width: 100%; display: grid; gap: 14px; max-height: 90vh; overflow: auto; }
  .dialogo ol { margin: 0; padding-left: 20px; display: grid; gap: 6px; }
  .virtuales { border-top: 1px solid var(--linea); padding-top: 14px; display: grid; gap: 10px; }
  .virtual { justify-content: space-between; }
</style>
