<script lang="ts">
  // El motor de vida en vivo (lo que siente y lo que está haciendo) y su diario
  // (lo que recuerda: días juntos, caricias, hitos, a qué hora te espera).
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { ESTADO, NOMBRES_ESTADO, NOMBRES_EVENTO, NOMBRES_RASGO, RASGOS } from '$lib/formato';
  import type { Bicho } from '$lib/tipos';

  const b = usarBicho();
  type Memoria = {
    nacimiento: number; ultimaInteraccion: number; dias: number; racha: number; rachaMax: number;
    caricias: number; toques: number; sacudidas: number; horas: number[]; gustos: Record<string, number>;
    hitos: string[]; deriva: Record<string, number>; estado: Record<string, number>; ausenciaMaxima: number; rencorHasta: number;
  };
  let memoria = $state<Memoria | null>(null);
  let actualizada = $state<string | null>(null);
  let mudanzaPendiente = $state(false);
  let otros = $state<Bicho[]>([]);
  let desde = $state('');
  let aviso = $state('');
  let error = $state('');
  let superposicion = $state(false);

  const HITOS: Record<string, string> = {
    'primer-dia': 'El primer día', 'siete-dias': 'Una semana juntos', 'treinta-dias': 'Un mes juntos',
    'cien-caricias': '100 caricias', 'cien-dias': '100 días', 'mil-caricias': '1000 caricias',
  };

  async function cargarMemoria() {
    try {
      const r = await api<{ memoria: Memoria | null; actualizada: string | null; mudanzaPendiente: boolean }>(`/v1/bichos/${b.id}/memoria`);
      memoria = r.memoria;
      actualizada = r.actualizada;
      mudanzaPendiente = r.mudanzaPendiente;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude leer la memoria.';
    }
  }

  onMount(() => {
    cargarMemoria();
    api<{ bichos: Bicho[] }>('/v1/bichos').then((r) => (otros = r.bichos.filter((x) => x.id !== b.id))).catch(() => {});
    // El estado en vivo: cada 2 s mientras la pestaña está abierta.
    const pedir = () => { if (b.bicho?.conectado) b.pedirVida(); };
    pedir();
    const reloj = setInterval(pedir, 2000);
    const relojMemoria = setInterval(cargarMemoria, 60000);
    return () => { clearInterval(reloj); clearInterval(relojMemoria); };
  });

  async function cambiarSuperposicion() {
    superposicion = !superposicion;
    error = (await b.pedirVida(superposicion)) ?? '';
  }

  async function mudar() {
    aviso = error = '';
    try {
      await api(`/v1/bichos/${b.id}/mudar`, { cuerpo: { desde } });
      aviso = 'Listo: la memoria y la config se le mandan apenas se conecte.';
      mudanzaPendiente = true;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude mudarlo.';
    }
  }

  const fecha = (s: number) => (s ? new Date((s - new Date().getTimezoneOffset() * -60) * 1000).toLocaleDateString('es-AR') : '—');
  const horaPreferida = $derived.by(() => {
    if (!memoria) return null;
    let mejor = -1;
    memoria.horas.forEach((v, h) => { if (v >= 3 && (mejor < 0 || v > memoria!.horas[mejor])) mejor = h; });
    return mejor;
  });
  const gustos = $derived(memoria ? Object.entries(memoria.gustos).filter(([, v]) => v > 0).sort((a, b) => b[1] - a[1]).slice(0, 3) : []);
  const vieja = $derived(b.vidaEn > 0 && Date.now() - b.vidaEn > 8000);
</script>

<div class="vida">
  <section class="ahora chapa" aria-live="polite">
    <div class="cabeza">
      <span class="grabado"><span class="led" class:verde={!!b.vida && !vieja}></span>Ahora</span>
      <button class="llave" aria-pressed={superposicion} onclick={cambiarSuperposicion} disabled={!b.bicho?.conectado} title="Ver los datos en su pantalla"><span class="led"></span>En su pantalla</button>
    </div>
    {#if !b.bicho?.conectado}
      <b class="haciendo apagado">Desconectado</b>
    {:else if !b.vida}
      <b class="haciendo apagado">Escuchando…</b>
    {:else}
      <b class="haciendo">{b.vida.dormido ? (b.vida.sonando ? 'Soñando' : 'Durmiendo') : (b.vida.comportamiento || 'Tranqui').replaceAll('-', ' ')}</b>
      <div class="vumetros">
        {#each ESTADO as v}
          {@const x = b.vida.estado[v] ?? 0}
          <div class="vu" title="{NOMBRES_ESTADO[v]} {Math.round(x * 100)}">
            <div class="columna">{#each Array.from({ length: 12 }, (_, k) => 11 - k) as k}<i class:on={x * 12 > k} class:alto={k >= 9} class:estres={v === 'estres'}></i>{/each}</div>
            <span class="grabado">{NOMBRES_ESTADO[v]}</span>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <section class="placa-naranja diario">
    <span class="grabado">Diario</span>
    {#if !memoria}
      <p class="texto">Todavía sin recuerdos.</p>
    {:else}
      <div class="contadores">
        {#each [['Días juntos', memoria.dias], ['Racha', memoria.racha], ['Caricias', memoria.caricias], ['Toques', memoria.toques]] as [t, n]}
          <div class="visor"><span class="grabado">{t}</span><span class="digitos">{n}</span></div>
        {/each}
      </div>
      {#if memoria.hitos.length}
        <div class="hitos">{#each memoria.hitos as h}<span class="medalla">★ {HITOS[h] ?? h}</span>{/each}</div>
      {/if}
      <div class="bandeja horas-caja">
        <span class="grabado">Cuándo aparecés{horaPreferida !== null && horaPreferida >= 0 ? ` · te espera a las ${horaPreferida}:00` : ''}</span>
        <div class="horas" aria-label="A qué hora aparecés">
          {#each memoria.horas as v, h}<i class:pico={h === horaPreferida} title="{h}:00" style="height: {Math.max(4, Math.min(100, v * 8))}%"></i>{/each}
        </div>
        <div class="ejes">{#each [0, 6, 12, 18, 24] as h}<span>{h}</span>{/each}</div>
      </div>
      {#if gustos.length}
        <div class="fila-chips"><span class="grabado">Le gusta</span>{#each gustos as [g]}<span class="gusto">♥ {NOMBRES_EVENTO[g] ?? g}</span>{/each}</div>
      {/if}
      {#if RASGOS.some((r) => memoria!.deriva[r])}
        <div class="fila-chips"><span class="grabado">Lo cambiaste</span>
          {#each RASGOS.filter((r) => memoria!.deriva[r]) as r}<span class="gusto">{memoria.deriva[r] > 0 ? '▲' : '▼'} {NOMBRES_RASGO[r][0]}</span>{/each}
        </div>
      {/if}
    {/if}
  </section>

  <section class="mudanza chapa">
    <span class="grabado">Mudanza{mudanzaPendiente ? ' · en camino' : ''}</span>
    <div class="fila">
      <select class="sel" bind:value={desde} aria-label="Robot viejo">
        <option value="">Traer la memoria de…</option>
        {#each otros as o}<option value={o.id}>{o.apodo || o.estado?.nombre || o.id}</option>{/each}
      </select>
      <button class="tecla naranja chica" onclick={mudar} disabled={!desde}>Mudar</button>
    </div>
    {#if aviso}<p class="aviso"><span class="led verde"></span>{aviso}</p>{/if}
    {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}
  </section>
</div>

<style>
  .vida { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 20px; align-items: start; }
  .ahora { display: flex; flex-direction: column; gap: 18px; padding: 22px; }
  .cabeza { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  .cabeza .grabado { display: inline-flex; align-items: center; gap: 8px; }
  .haciendo { font: 900 clamp(30px, 4vw, 46px)/.95 var(--marca); font-stretch: 120%; letter-spacing: -.03em; text-transform: uppercase; color: var(--brasa); }
  .haciendo.apagado { color: #5a524a; }
  .vumetros { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 8px; padding: 16px 12px 12px; border-radius: 18px; background: var(--hundido);
    box-shadow: inset 0 3px 10px rgba(0,0,0,.8); }
  .vu { display: flex; flex-direction: column; align-items: center; gap: 10px; min-width: 0; }
  .columna { display: flex; flex-direction: column; gap: 3px; width: 100%; max-width: 26px; }
  .columna i { height: 9px; border-radius: 2px; background: #2a2622; transition: background .3s, box-shadow .3s; }
  .columna i.on { background: var(--led); box-shadow: 0 0 6px var(--led); }
  .columna i.on.alto { background: #ffb347; box-shadow: 0 0 6px #ffb347; }
  .columna i.on.estres { background: #ff3b3b; box-shadow: 0 0 6px #ff3b3b; }
  .vu .grabado { font-size: 8.5px; letter-spacing: .06em; text-align: center; overflow: hidden; text-overflow: ellipsis; max-width: 100%; }
  .diario { grid-row: span 2; display: flex; flex-direction: column; gap: 16px; }
  .contadores { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .contadores .digitos { font-size: 40px; }
  .hitos, .fila-chips { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .medalla, .gusto { display: inline-flex; align-items: center; gap: 6px; min-height: 32px; padding: 0 12px; border-radius: 10px; background: #1a1714; color: var(--c-papel);
    font: 800 12px/1 var(--marca); }
  .medalla { color: #ffb347; }
  .horas-caja { display: flex; flex-direction: column; gap: 10px; }
  .horas { display: grid; grid-template-columns: repeat(24, 1fr); align-items: end; gap: 3px; height: 80px; }
  .horas i { display: block; border-radius: 3px 3px 1px 1px; background: rgba(26,23,20,.55); }
  .horas i.pico { background: #1a1714; box-shadow: 0 -3px 0 #fff inset; }
  .ejes { display: flex; justify-content: space-between; font: 800 11px/1 var(--marca); color: rgba(26,23,20,.66); }
  .mudanza { display: flex; flex-direction: column; gap: 12px; padding: 18px 20px; }
  .fila { display: flex; gap: 10px; }
  .sel { flex: 1; min-width: 0; height: 44px; padding: 0 12px; border: 0; border-radius: 12px; background: var(--hundido); color: var(--c-papel); font: 700 14px/1 var(--marca); }
  @media (max-width: 1000px) { .vida { grid-template-columns: 1fr; } .diario { grid-row: auto; } }
</style>
