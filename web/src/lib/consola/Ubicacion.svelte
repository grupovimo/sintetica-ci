<script lang="ts">
  // Dónde vive, para el clima: un buscador de ciudad (lo busca la nube) y un
  // visor chiquito con el tiempo de ahora.
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { bytes, cortarBytes, MAX_CIUDAD, type Ubicacion } from '$lib/formato';
  import { usarGuardado } from './guardado.svelte';

  const b = usarBicho();
  const g = usarGuardado();

  type Lugar = { nombre: string; region?: string; pais?: string; lat: number; lon: number; ciudad: string };
  type Clima = { temp: number; codigo: number; descripcion: string; max: number; min: number; dia: boolean; en: string };

  const ubicacion = $derived(b.config.ubicacion);
  let buscando = $state(!b.config.ubicacion);
  let q = $state('');
  let lugares = $state<Lugar[]>([]);
  let estado = $state<'listo' | 'buscando' | 'nada' | 'error'>('listo');
  let clima = $state<Clima | null>(null);

  let reloj: ReturnType<typeof setTimeout> | null = null;
  function escribir(v: string) {
    q = v;
    if (reloj) clearTimeout(reloj);
    const t = v.trim();
    if (bytes(t) < 2) { lugares = []; estado = 'listo'; return; }
    reloj = setTimeout(() => buscar(cortarBytes(t, 60)), 350);
  }
  async function buscar(t: string) {
    estado = 'buscando';
    try {
      const r = await api<{ lugares: Lugar[] }>(`/v1/clima/buscar?q=${encodeURIComponent(t)}`);
      if (q.trim() !== t) return;
      lugares = r.lugares ?? [];
      estado = lugares.length ? 'listo' : 'nada';
    } catch {
      estado = 'error';
    }
  }
  function elegir(l: Lugar) {
    const u: Ubicacion = { lat: +l.lat.toFixed(4), lon: +l.lon.toFixed(4), ciudad: cortarBytes(l.ciudad || l.nombre, MAX_CIUDAD) };
    g.cambiar((c) => (c.ubicacion = u));
    buscando = false;
    q = '';
    lugares = [];
    clima = null;
    // La nube trae el clima apenas se guarda la ubicación.
    setTimeout(cargarClima, 2500);
    setTimeout(cargarClima, 8000);
  }

  async function cargarClima() {
    try {
      clima = (await api<{ clima: Clima | null }>(`/v1/bichos/${b.id}/clima`)).clima;
    } catch {
      clima = null;
    }
  }
  onMount(() => {
    if (b.config.ubicacion) cargarClima();
    const t = setInterval(() => { if (b.config.ubicacion) cargarClima(); }, 10 * 60e3);
    return () => clearInterval(t);
  });
  const ciudadCorta = $derived(ubicacion?.ciudad.split(',')[0] ?? '');
</script>

<div class="ubicacion">
  {#if ubicacion && !buscando}
    <div class="ahora visor">
      <div class="donde">
        <span class="grabado">Clima</span>
        <b title={ubicacion.ciudad}>{ciudadCorta}</b>
      </div>
      {#if clima}
        <span class="digitos">{Math.round(clima.temp)}°</span>
        <div class="detalle"><span>{clima.descripcion}</span><span class="mm">↑{Math.round(clima.max)}° ↓{Math.round(clima.min)}°</span></div>
      {:else}
        <span class="digitos apagado">--°</span>
      {/if}
      <button class="tecla negra chica" onclick={() => (buscando = true)}>Cambiar</button>
    </div>
  {:else}
    <div class="buscador visor">
      <span class="grabado">Su ciudad</span>
      <div class="linea">
        <input class="campo claro" type="search" placeholder="Buscá tu ciudad" maxlength="60" value={q} oninput={(e) => escribir(e.currentTarget.value)}
          onkeydown={(e) => { if (e.key === 'Enter' && lugares[0]) elegir(lugares[0]); if (e.key === 'Escape' && ubicacion) buscando = false; }} />
        <span class="led" class:on={estado === 'buscando'} class:ambar={estado === 'nada' || estado === 'error'} title={estado === 'error' ? 'No pude buscar' : ''}></span>
        {#if ubicacion}<button class="tecla negra chica" onclick={() => { buscando = false; q = ''; lugares = []; }}>✕</button>{/if}
      </div>
    </div>
    {#if lugares.length}
      <div class="lugares">
        {#each lugares as l}
          <button class="tecla lugar" onclick={() => elegir(l)}><b>{l.nombre}</b><small>{[l.region, l.pais].filter((x) => x && x !== l.nombre).join(' · ')}</small></button>
        {/each}
      </div>
    {:else if estado === 'nada'}<p class="nota">Nada con ese nombre</p>
    {:else if estado === 'error'}<p class="nota">No pude buscar</p>{/if}
  {/if}
</div>

<style>
  .ubicacion { display: flex; flex-direction: column; gap: 10px; }
  .ahora { flex-direction: row; align-items: center; gap: 18px; padding: 12px 12px 12px 20px; }
  .donde { display: flex; flex-direction: column; gap: 8px; min-width: 0; flex: 1; }
  .donde b { font: 900 20px/1.1 var(--marca); color: var(--c-papel); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .ahora .digitos { font-size: 40px; }
  .digitos.apagado { color: #5a524a; text-shadow: none; }
  .detalle { display: flex; flex-direction: column; gap: 6px; font: 700 13px/1.1 var(--marca); color: var(--gris-claro); }
  .detalle span:first-child::first-letter { text-transform: uppercase; }
  .mm { color: var(--gris); font-variant-numeric: tabular-nums; }
  .buscador { padding: 12px 12px 12px 20px; }
  .linea { display: flex; align-items: center; gap: 12px; }
  .linea input::-webkit-search-cancel-button { display: none; }
  .lugares { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 12px; padding-bottom: 6px; }
  .lugar { flex-direction: column; align-items: flex-start; gap: 6px; min-height: 60px; padding: 10px 14px; white-space: normal; text-align: left; }
  .lugar b { font: 900 14px/1.1 var(--marca); }
  .lugar small { font: 600 11px/1.2 var(--marca); letter-spacing: 0; text-transform: none; opacity: .7; }
  .nota { margin: 0; font: 800 11px/1 var(--marca); letter-spacing: .14em; text-transform: uppercase; color: rgba(26,23,20,.66); }
  @media (max-width: 560px) { .detalle { display: none; } }
</style>
