<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import type { Firmware } from '$lib/tipos';

  let { data } = $props();
  let firmwares = $state<Firmware[]>([]);
  let perfil = $state('dev-147b');
  let version = $state('');
  let notas = $state('');
  let archivo = $state<File | null>(null);
  let error = $state('');
  let ok = $state('');
  let subiendo = $state(false);

  const cargar = async () => (firmwares = (await api<{ firmwares: Firmware[] }>('/v1/firmwares')).firmwares);
  onMount(cargar);

  async function subir(e: Event) {
    e.preventDefault();
    if (!archivo) return;
    subiendo = true;
    error = ok = '';
    const f = new FormData();
    f.set('perfil', perfil);
    f.set('version', version);
    f.set('notas', notas);
    f.set('binario', archivo);
    try {
      const r = await api<Firmware>('/v1/firmwares', { cuerpo: f, crudo: true });
      ok = `Firmado y guardado: ${r.perfil} ${r.version} (sha256 ${r.sha256.slice(0, 12)}…).`;
      version = notas = '';
      await cargar();
    } catch (err) {
      error = err instanceof ErrorApi ? err.message : 'No pude subirlo.';
    } finally {
      subiendo = false;
    }
  }

  async function borrar(f: Firmware) {
    if (!confirm(`¿Borrar ${f.perfil} ${f.version}?`)) return;
    await api(`/v1/firmwares/${f.id}`, { metodo: 'DELETE' });
    await cargar();
  }
</script>

<svelte:head><title>Firmwares · Sintética</title></svelte:head>

<div class="contenedor grilla" style="max-width: 900px">
  <div><p class="etiqueta">Administración</p><h1>Firmwares</h1></div>
  {#if !data.usuario?.admin}
    <p class="aviso-caja">Solo los administradores (ADMIN_EMAILS) suben firmwares.</p>
  {:else}
    <form class="tarjeta grilla" onsubmit={subir}>
      <h2>Subir una versión</h2>
      <p class="nota">El <code>firmware.bin</code> de PlatformIO (<code>firmware/.pio/build/&lt;perfil&gt;/firmware.bin</code>), compilado con <code>-DSINTETICA_VERSION</code> igual a la versión de acá. La API lo firma con su clave; el bicho solo instala lo firmado.</p>
      <div class="campos">
        <label><span class="etiqueta">Perfil</span>
          <select bind:value={perfil}>
            <option value="dev-147b">dev-147b (1.47B, por WiFi)</option>
            <option value="broche-c3">broche-c3 (por BLE, desde la app)</option>
            <option value="virtual">virtual (bicho virtual)</option>
            <option value="virtual-broche">virtual-broche</option>
          </select>
        </label>
        <label><span class="etiqueta">Versión</span><input required bind:value={version} pattern="[0-9A-Za-z.+\-]+" maxlength="23" placeholder="0.2.1" /></label>
      </div>
      <label><span class="etiqueta">Notas</span><textarea bind:value={notas} rows="3" style="min-height: 0"></textarea></label>
      <label><span class="etiqueta">Binario</span><input type="file" required accept=".bin,application/octet-stream" onchange={(e) => (archivo = e.currentTarget.files?.[0] ?? null)} /></label>
      <button class="primario" style="justify-self: start" disabled={subiendo || !archivo || !version}>{subiendo ? 'Subiendo…' : 'Firmar y subir'}</button>
      {#if error}<p class="aviso-caja error">{error}</p>{/if}
      {#if ok}<p class="aviso-caja ok">{ok}</p>{/if}
    </form>
  {/if}
  <section class="tarjeta grilla">
    <h2>Versiones</h2>
    {#each firmwares as f}
      <div class="fila version">
        <div><span class="chip">{f.perfil}</span> <b class="mono">{f.version}</b> <span class="nota">· {(f.tamano / 1024).toFixed(0)} KB · {new Date(f.creado).toLocaleString('es-AR')}</span>
          {#if f.notas}<p class="nota" style="margin: 4px 0 0">{f.notas}</p>{/if}</div>
        {#if data.usuario?.admin}<button class="chico peligro" onclick={() => borrar(f)}>Borrar</button>{/if}
      </div>
    {:else}
      <p class="nota">Todavía no hay.</p>
    {/each}
  </section>
</div>

<style>
  .version { justify-content: space-between; border-top: 1px solid var(--linea); padding-top: 10px; }
</style>
