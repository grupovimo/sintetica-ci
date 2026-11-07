<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import type { VersionConfig } from '$lib/tipos';

  const b = usarBicho();
  let versiones = $state<VersionConfig[]>([]);
  let aviso = $state('');
  const AUTORES: Record<string, string> = { web: 'desde la web', app: 'desde una app', bicho: 'en el bicho (por BLE, cerca)', api: 'la API' };

  async function cargar() {
    versiones = (await api<{ versiones: VersionConfig[] }>(`/v1/bichos/${b.id}/config/historial`)).versiones;
  }
  onMount(cargar);
  $effect(() => {
    if (b.versionConfig && versiones.length && versiones[0].version !== b.versionConfig) cargar();
  });

  async function restaurar(v: number) {
    try {
      await api(`/v1/bichos/${b.id}/config/restaurar`, { cuerpo: { version: v } });
      await b.cargarConfig();
      await cargar();
      aviso = `Volviste a la versión ${v} (quedó como una versión nueva).`;
    } catch (e) {
      aviso = e instanceof ErrorApi ? e.message : 'No pude.';
    }
  }
</script>

<div class="tarjeta grilla" style="max-width: 760px">
  <h2>Versiones de la config</h2>
  <p class="nota">Cada cambio queda guardado. Si el bicho y la nube cambiaron a la vez, gana el último que habló con la nube.</p>
  {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
  {#each versiones as v}
    <div class="fila version">
      <div><b class="mono">v{v.version}</b> <span class="nota">· {AUTORES[v.autor] ?? v.autor} · {new Date(v.creada).toLocaleString('es-AR')}</span></div>
      {#if v.version === b.versionConfig}<span class="chip ok">actual</span>{:else}<button class="chico" onclick={() => restaurar(v.version)}>Volver a esta</button>{/if}
    </div>
  {:else}
    <p class="nota">Todavía no hay versiones: el bicho usa la config de fábrica.</p>
  {/each}
</div>

<style>
  .version { justify-content: space-between; border-top: 1px solid var(--linea); padding-top: 10px; }
</style>
