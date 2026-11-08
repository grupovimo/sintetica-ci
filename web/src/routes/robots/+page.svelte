<script lang="ts">
  // Mis robots: el dock donde está cada uno, con su foto real, su cara en vivo,
  // cómo está y cuánta batería tiene.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { escuchar } from '$lib/eventos.svelte';
  import type { Bicho } from '$lib/tipos';
  import Barra from '$lib/consola/Barra.svelte';
  import Producto from '$lib/consola/Producto.svelte';
  import { animoDe, hace, modeloDe, numeroDe } from '$lib/consola/modelo';

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
      error = e instanceof ErrorApi ? e.message : 'No pude cargar tus robots.';
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

  const nombreDe = (b: Bicho) => b.apodo || b.estado?.nombre || (configs[b.id] as { nombre?: string } | undefined)?.nombre || 'Sin nombre';
  const enLinea = $derived(bichos.filter((b) => b.conectado).length);
  // El LED: verde en línea, ámbar durmiendo, apagado si no está.
  const led = (b: Bicho) => (!b.conectado ? '' : b.estado?.dormido ? 'ambar' : 'verde');
  const detalle = (b: Bicho) => (b.conectado ? animoDe(b) : `Visto ${hace(b.ultimaConexion)}`);
</script>

<svelte:head><title>Mis robots · Sintética</title></svelte:head>

<Barra />

<main class="caja">
  <div class="encabezado">
    <div class="titulo">
      <span class="grabado" style="color: var(--brasa)">
        {cargando ? 'Buscando…' : `${bichos.length} ${bichos.length === 1 ? 'robot' : 'robots'} · ${enLinea} en línea`}
      </span>
      <h1 class="titular">Mis robots</h1>
    </div>
    <button class="tecla naranja grande" onclick={abrirSumar}>+ Sumar uno</button>
  </div>

  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  <div class="dock">
    {#each bichos as b (b.id)}
      {@const m = modeloDe(b)}
      {@const bat = b.estado?.bateria}
      <a class="bahia chapa" href="/robots/{b.id}">
        <div class="foto">
          <Producto modelo={m} config={configs[b.id] ?? { version: 1 }} visible={b.estado?.expresion ?? ''} apagado={!b.conectado} />
          {#if b.estado?.dormido && b.conectado}<span class="sello"><svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" /></svg>Durmiendo</span>
          {:else if !b.conectado}<span class="sello">Fuera de alcance</span>{/if}
        </div>
        <div class="base">
          <div class="fila"><span class="grabado">{m.nombre} {numeroDe(b.id)}{b.virtual ? ' · virtual' : ''}</span><span class="led {led(b)}"></span></div>
          <b class="nombre">{nombreDe(b)}</b>
          <div class="fila">
            <span class="detalle" class:apagado={!b.conectado}>{detalle(b)}</span>
            <span class="visor-chico" class:apagado={bat == null}>{bat != null ? `${bat}%` : '—'}</span>
          </div>
          <div class="carga" aria-label={bat != null ? `Batería ${bat}%` : 'Sin datos de batería'}>
            {#if bat != null}<span style="width: {bat}%" class:baja={bat < 15} class:viva={b.conectado}></span>{/if}
          </div>
        </div>
      </a>
    {/each}

    <button class="bahia libre" onclick={abrirSumar}>
      <div class="hueco"><span class="mas" aria-hidden="true">+</span></div>
      <div class="base">
        <span class="grabado">Bahía libre</span>
        <b class="nombre chico">Sumar un robot</b>
        <span class="detalle apagado">Escaneá su QR con la app.</span>
      </div>
    </button>
  </div>
</main>

{#if sumando}
  <div class="fondo" role="presentation" onclick={(e) => e.target === e.currentTarget && (sumando = false)}>
    <div class="dialogo chapa" role="dialog" aria-modal="true" aria-labelledby="t-sumar">
      <span class="grabado" style="color: var(--brasa)">Bahía libre</span>
      <h2 id="t-sumar" class="titular" style="font-size: 44px">Sumar un robot</h2>
      <ol class="pasos">
        <li><span class="num">1</span><span>Prendelo: muestra un QR.</span></li>
        <li><span class="num">2</span><span>Escanealo con la app.</span></li>
        <li><span class="num">3</span><span>Tocá <b>Sumar</b> en su pantalla.</span></li>
      </ol>
      {#if data.desarrollo}
        <div class="virtuales">
          <span class="grabado">Robots virtuales (solo en local)</span>
          {#each virtuales as v}
            <div class="virtual">
              <span class="id">{v.id}</span>
              {#if v.sumado}<span class="grabado" style="color: var(--verde)">En tu cuenta</span>
              {:else}<button class="tecla chica" disabled={!!sumandoVirtual} onclick={() => sumarVirtual(v.id)}>{sumandoVirtual === v.id ? 'Sumando…' : 'Sumar'}</button>{/if}
            </div>
          {:else}
            <p class="texto" style="font-size: 14px">No hay robots virtuales.</p>
          {/each}
        </div>
      {/if}
      {#if errorSumar}<p class="aviso error"><span class="led"></span>{errorSumar}</p>{/if}
      <div style="display: flex; justify-content: flex-end"><button class="tecla negra" onclick={() => (sumando = false)}>Cerrar</button></div>
    </div>
  </div>
{/if}

<style>
  main { display: flex; flex-direction: column; gap: 40px; padding-top: 8px; padding-bottom: 64px; }
  .encabezado { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; flex-wrap: wrap; }
  .titulo { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
  .grande { min-height: 60px; padding: 0 28px; }
  .dock { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 24px; align-items: start; }
  .bahia { display: flex; flex-direction: column; text-decoration: none; color: var(--c-papel); overflow: hidden; border-radius: 32px; transition: transform .2s; }
  .bahia:hover { transform: translateY(-4px); }
  .foto { position: relative; margin: 12px 12px 0; --radio-ventana: 22px; }
  .sello { position: absolute; left: 14px; top: 14px; display: inline-flex; align-items: center; gap: 8px; min-height: 30px; padding: 0 12px; border-radius: 9px;
    background: rgba(14,13,11,.82); font: 800 11px/1 var(--marca); letter-spacing: .12em; text-transform: uppercase; color: var(--gris-claro); }
  .base { display: flex; flex-direction: column; gap: 14px; padding: 18px 20px 20px; text-align: left; }
  .fila { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .nombre { font: 900 34px/1 var(--marca); font-stretch: 120%; letter-spacing: -.03em; text-transform: uppercase; overflow-wrap: anywhere; }
  .nombre.chico { font-size: 26px; line-height: 1.05; }
  .detalle { font: 700 14px/1.3 var(--marca); color: #d9d2c6; }
  .detalle.apagado { color: var(--gris); }
  .visor-chico { padding: 6px 10px; border-radius: 8px; background: var(--hundido); color: var(--brasa); font: 900 15px/1 var(--marca);
    font-variant-numeric: tabular-nums; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .visor-chico.apagado { color: #6b645b; }
  .carga { height: 6px; border-radius: 3px; background: var(--hundido); box-shadow: inset 0 1px 3px rgba(0,0,0,.8); overflow: hidden; }
  .carga span { display: block; height: 100%; background: linear-gradient(90deg, #f2541a, #ff8a5c); }
  .carga span.viva { box-shadow: 0 0 8px var(--led); }
  .carga span.baja { background: #ff4d4d; }
  .libre { border: 2px dashed #3a3632; background: transparent; cursor: pointer; padding: 0; font: inherit; }
  .hueco { margin: 12px 12px 0; aspect-ratio: 4 / 5; border-radius: 22px; background: var(--hundido); box-shadow: inset 0 3px 12px rgba(0,0,0,.9);
    display: grid; place-items: center; }
  .mas { display: grid; place-items: center; width: 72px; height: 72px; border-radius: 50%; background: linear-gradient(180deg, #ff8a5c, #f2541a); color: #1a1714;
    font: 700 40px/1 var(--marca); box-shadow: 0 6px 0 #9a3209, 0 14px 20px -6px rgba(0,0,0,.6); transition: transform .15s; }
  .libre:hover .mas { transform: scale(1.06); }
  .fondo { position: fixed; inset: 0; z-index: 50; display: grid; place-items: center; padding: 16px; background: rgba(8,7,6,.7); backdrop-filter: blur(6px); }
  .dialogo { width: 100%; max-width: 560px; max-height: 90vh; overflow: auto; display: flex; flex-direction: column; gap: 18px; padding: 28px; border-radius: 32px; }
  .pasos { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 12px; }
  .pasos li { display: flex; gap: 14px; align-items: flex-start; font: 600 16px/1.4 var(--marca); color: #d9d2c6; }
  .num { flex: none; display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; background: var(--hundido); color: var(--brasa); font: 900 14px/1 var(--marca); }
  .virtuales { display: flex; flex-direction: column; gap: 10px; padding-top: 16px; border-top: 1px solid #3a3632; }
  .virtual { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 14px; border-radius: 12px; background: var(--hundido); }
  .id { font: 800 14px/1 var(--marca); letter-spacing: .06em; }
</style>
