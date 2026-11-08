<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import type { Actualizacion, Firmware } from '$lib/tipos';
  import { modeloDe } from '$lib/consola/modelo';

  const b = usarBicho();
  let firmwares = $state<Firmware[]>([]);
  let historial = $state<Actualizacion[]>([]);
  let error = $state('');
  let mandando = $state(0);

  const perfil = $derived(b.bicho?.estado?.perfil || b.bicho?.perfil || '');
  async function cargar() {
    if (!perfil) return;
    firmwares = (await api<{ firmwares: Firmware[] }>(`/v1/firmwares?perfil=${encodeURIComponent(perfil)}`)).firmwares;
    historial = (await api<{ actualizaciones: Actualizacion[] }>(`/v1/bichos/${b.id}/actualizaciones`)).actualizaciones;
  }
  onMount(cargar);
  $effect(() => {
    if (b.ota) {
      const i = historial.findIndex((h) => h.id === b.ota!.id);
      if (i >= 0) historial[i] = b.ota;
      else historial = [b.ota, ...historial];
    }
  });

  let seguro = $state(0);
  async function instalar(f: Firmware) {
    if (seguro !== f.id) { seguro = f.id; setTimeout(() => (seguro = 0), 5000); return; }
    seguro = 0;
    mandando = f.id;
    error = '';
    try {
      const u = await api<Actualizacion>(`/v1/bichos/${b.id}/actualizar`, { cuerpo: { firmware: f.id } });
      historial = [u, ...historial];
    } catch (e) {
      error = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude mandarla.';
    } finally {
      mandando = 0;
    }
  }

  const clase = (e: Actualizacion['estado']) => (e === 'confirmada' ? 'ok' : e === 'fallida' ? 'error' : 'aviso');
  const NOMBRES: Record<string, string> = { enviada: 'Enviada', descargando: 'Bajando', instalada: 'Probando', confirmada: 'Lista', fallida: 'Falló' };
  const actual = $derived(b.bicho?.estado?.version ?? '—');
  const puede = $derived(!!b.bicho?.conectado && !!b.bicho?.estado?.wifi && !b.bicho?.estado?.actualizando);
  const modelo = $derived(b.bicho ? modeloDe(b.bicho).nombre : '');
</script>

<div class="firmware">
  <section class="placa-naranja">
    <div class="visor ahora"><span class="grabado">{modelo} · tiene</span><span class="digitos">{actual}</span></div>
    {#if !b.bicho?.estado?.wifi}<p class="texto">Sin WiFi: se actualiza desde la app, cerca.</p>{/if}
    {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
    <div class="versiones">
      {#each firmwares as f}
        {@const es = f.version === actual}
        <div class="version bandeja" class:actual={es}>
          <div class="v"><b>{f.version}</b><span class="grabado">{new Date(f.creado).toLocaleDateString('es-AR')}</span></div>
          {#if f.notas}<p class="notas">{f.notas}</p>{/if}
          <button class="tecla {seguro === f.id ? 'naranja' : 'negra'} chica" disabled={es || !puede || mandando > 0} onclick={() => instalar(f)}>
            {es ? 'Instalada' : mandando === f.id ? 'Mandando…' : seguro === f.id ? '¿Seguro?' : 'Instalar'}</button>
        </div>
      {:else}
        <p class="texto">No hay versiones nuevas.</p>
      {/each}
    </div>
  </section>
  <section class="historial chapa">
    <span class="grabado">Historial</span>
    {#each historial as h (h.id)}
      <div class="item">
        <span class="led" class:verde={h.estado === 'confirmada'} class:on={h.estado !== 'confirmada' && h.estado !== 'fallida'}></span>
        <b>{h.version}</b><span class="estado" class:mal={h.estado === 'fallida'}>{NOMBRES[h.estado]}</span>
        <span class="cuando">{new Date(h.creada).toLocaleDateString('es-AR')}</span>
        {#if h.estado === 'descargando'}<span class="barra"><i style="width: {h.progreso}%"></i></span>{/if}
      </div>
    {:else}
      <p class="texto">Nunca se actualizó.</p>
    {/each}
  </section>
</div>

<style>
  .firmware { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); gap: 20px; align-items: start; }
  .placa-naranja { display: flex; flex-direction: column; gap: 14px; }
  .ahora .digitos { font-size: 44px; }
  .versiones { display: flex; flex-direction: column; gap: 10px; }
  .version { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px 14px; align-items: center; padding: 14px 16px 18px; }
  .version.actual { box-shadow: inset 0 0 0 2px #1a1714; }
  .v { display: flex; align-items: baseline; gap: 12px; }
  .v b { font: 900 22px/1 var(--marca); font-stretch: 110%; color: #1a1714; }
  .notas { grid-column: 1; margin: 0; font: 600 14px/1.4 var(--marca); color: rgba(26,23,20,.8); max-width: 40ch; }
  .version .tecla { grid-row: 1 / span 2; grid-column: 2; }
  .historial { display: flex; flex-direction: column; gap: 10px; padding: 20px; }
  .item { display: grid; grid-template-columns: auto auto 1fr auto; gap: 10px; align-items: center; padding: 10px 12px; border-radius: 12px; background: var(--hundido); }
  .item b { font: 800 15px/1 var(--marca); }
  .estado { font: 700 13px/1 var(--marca); color: var(--gris-claro); }
  .estado.mal { color: #ff7a7a; }
  .cuando { font: 600 12px/1 var(--marca); color: var(--gris); }
  .barra { grid-column: 1 / -1; height: 6px; border-radius: 3px; background: #2a2622; overflow: hidden; }
  .barra i { display: block; height: 100%; background: var(--led); }
  @media (max-width: 900px) { .firmware { grid-template-columns: 1fr; } }
</style>
