<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import type { VersionConfig } from '$lib/tipos';

  const b = usarBicho();
  let versiones = $state<VersionConfig[]>([]);
  let aviso = $state('');
  const AUTORES: Record<string, string> = { web: 'Web', app: 'App', bicho: 'En el robot', api: 'Nube' };
  const autor = (a: string) => AUTORES[a] ?? (a.startsWith('mcp:') ? `IA · ${a.slice(4)}` : a);

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
      aviso = `Volviste a la v${v}`;
      setTimeout(() => (aviso = ''), 3000);
    } catch (e) {
      aviso = e instanceof ErrorApi ? e.message : 'No pude.';
    }
  }
</script>

<div class="historial chapa">
  <div class="cabeza"><span class="grabado">Cambios</span>{#if aviso}<span class="grabado" style="color: var(--verde)">{aviso}</span>{/if}</div>
  <ol class="linea">
    {#each versiones as v}
      {@const es = v.version === b.versionConfig}
      <li class:actual={es}>
        <span class="punto"></span>
        <b>v{v.version}</b>
        <span class="quien">{autor(v.autor)}</span>
        <span class="cuando">{new Date(v.creada).toLocaleString('es-AR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span>
        {#if es}<span class="grabado" style="color: var(--brasa)">Actual</span>{:else}<button class="tecla chica" onclick={() => restaurar(v.version)}>↺ Volver</button>{/if}
      </li>
    {:else}
      <li class="vacio"><span class="texto">De fábrica.</span></li>
    {/each}
  </ol>
</div>

<style>
  .historial { max-width: 720px; padding: 22px; display: flex; flex-direction: column; gap: 14px; }
  .cabeza { display: flex; justify-content: space-between; }
  .linea { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
  li { display: grid; grid-template-columns: 22px 64px minmax(0, 1fr) auto auto; gap: 12px; align-items: center; min-height: 56px; position: relative; }
  li:not(:last-child)::after { content: ''; position: absolute; left: 10px; top: 36px; bottom: -20px; width: 2px; background: #3a3632; }
  .punto { width: 12px; height: 12px; margin-left: 5px; border-radius: 50%; background: #3a3632; position: relative; z-index: 1; }
  .actual .punto { background: var(--led); box-shadow: 0 0 10px var(--led); }
  li b { font: 900 17px/1 var(--marca); font-variant-numeric: tabular-nums; }
  .quien { font: 700 14px/1.2 var(--marca); color: #d9d2c6; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .cuando { font: 600 12px/1 var(--marca); color: var(--gris); }
  @media (max-width: 560px) { li { grid-template-columns: 22px 54px minmax(0, 1fr) auto; } .cuando { display: none; } }
</style>
