<script lang="ts">
  // El paquete de voz sin internet: las frases que dice solo, ya grabadas con
  // su voz, para que hable sin wifi. Con ElevenLabs gasta de tu cuenta: antes
  // de generarlo se muestra cuánto y se confirma.
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { megas, miles, type PaqueteVoz } from '../voz';

  const b = usarBicho();
  let p = $state<PaqueteVoz | null>(null);
  let confirmar = $state<PaqueteVoz | null>(null);
  let error = $state('');
  let pidiendo = $state(false);

  async function cargar() {
    try { p = await api<PaqueteVoz>(`/v1/bichos/${b.id}/voz/paquete`); } catch { p = null; }
  }
  onMount(() => {
    cargar();
    // Mientras se genera, se mira cada tanto (y al terminar llega el evento).
    const t = setInterval(() => { if (p?.estado === 'generando') cargar(); }, 3000);
    const fuera = escuchar((e) => { if ((e.tipo === 'voz-paquete' && e.bicho === b.id) || e.tipo === 'voz') cargar(); });
    return () => { clearInterval(t); fuera(); };
  });

  async function generar(si = false) {
    error = '';
    pidiendo = true;
    try {
      // A mano: el 409 trae el estado en el cuerpo (cuánto va a gastar con ElevenLabs).
      const r = await fetch(`/api/v1/bichos/${b.id}/voz/paquete`, {
        method: 'POST', headers: { 'X-Sintetica': '1', 'Content-Type': 'application/json' }, body: JSON.stringify(si ? { confirmar: true } : {}),
      });
      const d = await r.json().catch(() => null);
      if (r.status === 409 && d?.estado) confirmar = d as PaqueteVoz;
      else if (!r.ok) error = d?.error ?? `Error ${r.status}`;
      else { p = d as PaqueteVoz; confirmar = null; }
    } catch {
      error = 'No pude pedirlo.';
    }
    pidiendo = false;
  }

  const LED = { listo: 'verde', parcial: 'ambar', generando: 'ambar', nuevo: '', error: 'rojo' } as const;
  const ETIQUETA = { listo: 'Listo', parcial: 'A medias', generando: 'Grabando', nuevo: 'Sin paquete', error: 'Falló' } as const;
  const hay = $derived(!!p && p.version > 0);
  const cambio = $derived(!!p && hay && (p.faltan > 0 || (!!p.voz_nueva && p.voz_nueva !== p.voz)));
  const eleven = $derived(p?.proveedor === 'elevenlabs');
</script>

{#if p?.disponible}
  <div class="paquete bandeja">
    <div class="cabeza">
      <span class="grabado"><span class="led" class:verde={LED[p.estado] === 'verde'} class:ambar={LED[p.estado] === 'ambar'} class:rojo={LED[p.estado] === 'rojo'} class:titila={p.estado === 'generando'}></span><svg class="sin-wifi" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" role="img" aria-label="Sin internet"><path d="M2 8.5a15 15 0 0 1 20 0M5 12a10 10 0 0 1 14 0M8.5 15.5a5 5 0 0 1 7 0M12 19h.01M3 3l18 18" /></svg>{ETIQUETA[p.estado]}</span>
      <span class="grabado prov">{eleven ? 'ElevenLabs' : 'Gratis'}</span>
    </div>
    <div class="visores">
      <div class="visor"><span class="grabado">Versión</span><span class="digitos chicos">{hay ? `v${p.version}` : '—'}</span></div>
      <div class="visor"><span class="grabado">Frases</span><span class="digitos chicos">{p.frases}<small>/{p.total}</small></span></div>
      <div class="visor"><span class="grabado">MB</span><span class="digitos chicos">{megas(hay ? p.bytes : p.bytes_estimados)}{#if !hay}<small>≈</small>{/if}</span></div>
    </div>
    {#if p.estado === 'generando' && p.progreso}
      <div class="progreso" role="progressbar" aria-valuemin={0} aria-valuemax={p.progreso.de} aria-valuenow={p.progreso.hechas}>
        <i style="width: {(p.progreso.hechas / Math.max(1, p.progreso.de)) * 100}%"></i>
      </div>
    {/if}
    {#if p.error}<p class="aviso error"><span class="led"></span>{p.error}</p>{/if}
    {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

    {#if confirmar}
      <div class="confirmar visor" role="alertdialog" aria-label="Confirmar el gasto">
        <span class="grabado">Gasta de tu cuenta</span>
        <span class="digitos chicos">{miles(confirmar.gasta)} <small>letras</small></span>
        <div class="botones">
          <button class="tecla naranja chica" disabled={pidiendo} onclick={() => generar(true)}>Generar</button>
          <button class="tecla negra chica" onclick={() => (confirmar = null)}>Cancelar</button>
        </div>
      </div>
    {:else if p.estado !== 'generando' && (!hay || cambio || p.estado === 'error' || p.estado === 'parcial')}
      <button class="tecla naranja generar" disabled={pidiendo} onclick={() => generar()}>
        {pidiendo ? '…' : hay ? 'Actualizar' : 'Generar'}{#if eleven && p.gasta > 0}<small>{miles(p.gasta)} letras</small>{/if}
      </button>
    {/if}
  </div>
{/if}

<style>
  .paquete { display: flex; flex-direction: column; gap: 12px; }
  .cabeza { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
  .cabeza .grabado { display: inline-flex; align-items: center; gap: 8px; }
  .prov { color: rgba(26,23,20,.55) !important; }
  .visores { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
  .visores .visor { padding: 12px 14px; gap: 8px; }
  small { font-size: .55em; color: var(--gris); margin-left: 2px; }
  .progreso { height: 10px; border-radius: 5px; background: var(--hundido); box-shadow: inset 0 2px 4px rgba(0,0,0,.8); overflow: hidden; }
  .progreso i { display: block; height: 100%; background: var(--verde); box-shadow: 0 0 8px var(--verde); transition: width .4s; }
  .confirmar { flex-direction: row; flex-wrap: wrap; align-items: center; gap: 12px 16px; }
  .confirmar .botones { display: flex; gap: 8px; margin-left: auto; }
  .generar { align-self: flex-start; gap: 12px; }
  .generar small { font-size: 10px; color: rgba(26,23,20,.65); letter-spacing: .04em; }
  .led.rojo { background: #ff4d4d; box-shadow: 0 0 8px #ff4d4d; }
  .led.titila { animation: titila 1s steps(2) infinite; }
  @keyframes titila { 50% { opacity: .25; } }
</style>
