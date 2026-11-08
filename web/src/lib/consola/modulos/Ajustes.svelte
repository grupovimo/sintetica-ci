<script lang="ts">
  // Ajustes: su nombre, el brillo, y lo de fondo (firmware, historial, sacarlo de la cuenta).
  import { goto } from '$app/navigation';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { usarGuardado } from '../guardado.svelte';
  import { numeroDe, type Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();
  let error = $state('');
  let seguro = $state(false);
  let quitando = $state(false);

  async function ponerNombre(v: string) {
    v = v.trim().slice(0, 24);
    try {
      const r = await api<{ apodo: string }>(`/v1/bichos/${b.id}`, { metodo: 'PATCH', cuerpo: { apodo: v } });
      if (b.bicho) b.bicho.apodo = r.apodo;
      // El nombre también va a la config: es el que muestra y al que responde.
      if (v) g.cambiar((c) => (c.nombre = v));
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude guardar el nombre.';
    }
  }
  async function quitar() {
    if (!seguro) { seguro = true; setTimeout(() => (seguro = false), 5000); return; }
    quitando = true;
    try {
      await api(`/v1/bichos/${b.id}`, { metodo: 'DELETE' });
      goto('/robots');
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude sacarlo.';
      quitando = false;
    }
  }
  const e = $derived(b.bicho?.estado);
  const base = $derived(`/robots/${b.id}`);
</script>

<div class="ajustes">
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
  <div class="fila">
    <label class="visor">
      <span class="grabado">Se llama</span>
      <input class="campo" maxlength="24" value={b.bicho?.apodo || b.config.nombre || ''} placeholder="Ponele nombre" onchange={(ev) => ponerNombre(ev.currentTarget.value)} />
    </label>
    <div class="bandeja brillo">
      <Perilla nombre="Brillo máx." valor={b.config.limites_marca?.brillo_max ?? 0.9} min={0.2} max={1} alCambiar={(v) => g.cambiar((c) => (c.limites_marca = { ...(c.limites_marca ?? {}), brillo_max: v }))}
        mostrar={(v) => `${Math.round(v * 100)}%`} />
    </div>
  </div>

  <div class="placa-datos">
    {#each [['Modelo', `${modelo.nombre} ${numeroDe(b.id)}`], ['Firmware', e?.version ?? '—'], ['Config', `v${b.versionConfig} · ${b.bicho?.sincro?.config ?? '—'}`], ['Batería', e?.bateria != null ? `${e.bateria}%` : '—'], ['WiFi', e?.wifi ? 'Conectado' : 'No'], ['Serie', b.id]] as [k, v]}
      <div class="dato"><span class="grabado">{k}</span><span class="v">{v}</span></div>
    {/each}
  </div>

  <div class="atajos">
    <a class="tecla" href="{base}/actualizar">Actualizar firmware</a>
    <a class="tecla" href="{base}/historial">Historial de cambios</a>
    <a class="tecla" href="{base}/vida">Su diario</a>
    <a class="tecla negra" href="{base}/ahora">Laboratorio</a>
  </div>

  <div class="peligro bandeja">
    <div><b>Sacarlo de tu cuenta</b><p class="texto">Se olvida del WiFi y de la nube. Para volver a sumarlo hace falta la app, cerca.</p></div>
    <button class="tecla negra" disabled={quitando} onclick={quitar}>{quitando ? 'Sacando…' : seguro ? '¿Seguro? Tocá otra vez' : 'Sacar'}</button>
  </div>
</div>

<style>
  .ajustes { display: flex; flex-direction: column; gap: 16px; }
  .fila { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 16px; }
  .brillo { display: grid; place-items: center; padding: 20px 26px; }
  .placa-datos { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 2px; border-radius: 18px; overflow: hidden; background: rgba(26,23,20,.25); }
  .dato { display: flex; flex-direction: column; gap: 8px; padding: 14px 16px; background: #0e0d0b; min-width: 0; }
  .v { font: 800 15px/1.2 var(--marca); color: #f3eee8; overflow-wrap: anywhere; }
  .atajos { display: flex; flex-wrap: wrap; gap: 12px; padding-bottom: 6px; }
  .peligro { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
  .peligro b { font: 900 17px/1 var(--marca); text-transform: uppercase; }
  @media (max-width: 640px) { .fila { grid-template-columns: 1fr; } .placa-datos { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
