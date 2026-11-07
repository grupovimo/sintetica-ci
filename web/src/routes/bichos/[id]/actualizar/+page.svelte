<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import type { Actualizacion, Firmware } from '$lib/tipos';

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

  async function instalar(f: Firmware) {
    if (!confirm(`¿Instalar ${f.version}? El bicho lo baja, verifica la firma, se reinicia y, si no anda bien, vuelve solo a la versión anterior.`)) return;
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
  const NOMBRES: Record<string, string> = { enviada: 'enviada', descargando: 'bajando', instalada: 'instalada, probando', confirmada: 'confirmada', fallida: 'falló' };
</script>

<div class="dos">
  <section class="tarjeta grilla">
    <h2>Versiones para {perfil}</h2>
    <p class="nota">Tiene la <b class="mono">{b.bicho?.estado?.version ?? '—'}</b>. Por la nube se actualizan los que tienen WiFi; el broche, desde la app, por BLE.</p>
    {#if error}<p class="aviso-caja error">{error}</p>{/if}
    {#each firmwares as f}
      <div class="fila version">
        <div><b class="mono">{f.version}</b> <span class="nota">· {(f.tamano / 1024).toFixed(0)} KB · {new Date(f.creado).toLocaleDateString('es-AR')}</span>
          {#if f.notas}<p class="nota" style="margin: 4px 0 0">{f.notas}</p>{/if}</div>
        <button class="chico" disabled={!b.bicho?.conectado || !b.bicho?.estado?.wifi || b.bicho?.estado?.actualizando || mandando > 0 || f.version === b.bicho?.estado?.version} onclick={() => instalar(f)}>
          {f.version === b.bicho?.estado?.version ? 'Instalada' : mandando === f.id ? 'Mandando…' : 'Instalar'}
        </button>
      </div>
    {:else}
      <p class="nota">No hay firmwares para este perfil. Los sube un administrador en Firmwares.</p>
    {/each}
  </section>
  <section class="tarjeta grilla">
    <h2>Historial</h2>
    {#each historial as h (h.id)}
      <div class="grilla item">
        <div class="fila" style="justify-content: space-between"><b class="mono">{h.version}</b><span class="chip {clase(h.estado)}">{NOMBRES[h.estado]}</span></div>
        {#if h.estado === 'descargando'}<progress max="100" value={h.progreso}></progress>{/if}
        {#if h.detalle}<p class="nota" style="margin: 0">{h.detalle}</p>{/if}
        <p class="etiqueta">{new Date(h.creada).toLocaleString('es-AR')}</p>
      </div>
    {:else}
      <p class="nota">Nunca se actualizó por la nube.</p>
    {/each}
  </section>
</div>

<style>
  .dos { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; align-items: start; }
  .version { justify-content: space-between; border-top: 1px solid var(--linea); padding-top: 12px; }
  .item { border-top: 1px solid var(--linea); padding-top: 10px; gap: 6px; }
  progress { width: 100%; accent-color: var(--naranja); }
  @media (max-width: 860px) { .dos { grid-template-columns: 1fr; } }
</style>
