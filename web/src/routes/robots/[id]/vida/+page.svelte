<script lang="ts">
  // El motor de vida en vivo (lo que siente y lo que está haciendo) y su diario
  // (lo que recuerda: días juntos, caricias, hitos, a qué hora te espera).
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { ESTADO, EVENTOS, NOMBRES_ESTADO, NOMBRES_EVENTO, NOMBRES_RASGO, RASGOS } from '$lib/formato';
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

  async function evento(n: string) {
    error = (await b.comando({ evento: n })) ?? '';
  }

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

<div class="dos">
  <div class="grilla">
    <section class="tarjeta grilla" aria-live="polite">
      <div class="fila">
        <h2 style="margin-right: auto">Ahora</h2>
        <button class="chico" aria-pressed={superposicion} onclick={cambiarSuperposicion} disabled={!b.bicho?.conectado}>
          Superposición en la pantalla
        </button>
      </div>
      {#if !b.bicho?.conectado}
        <p class="nota">El bicho no está conectado: el estado en vivo aparece cuando vuelva.</p>
      {:else if !b.vida}
        <p class="nota">Preguntándole…</p>
      {:else}
        <p>
          <b>{b.vida.dormido ? (b.vida.sonando ? 'Soñando' : 'Durmiendo') : b.vida.comportamiento || 'Nada en especial'}</b>
          <span class="nota">· se ve <span class="mono">{b.vida.visible}</span>{b.vida.fps ? ` · ${b.vida.fps} fps` : ''}{vieja ? ' · (hace un rato)' : ''}</span>
        </p>
        <div class="barras">
          {#each ESTADO as v}
            <div class="barra"><span>{NOMBRES_ESTADO[v]}</span><i><b style="width: {Math.round((b.vida.estado[v] ?? 0) * 100)}%" class:estres={v === 'estres'}></b></i><span class="mono">{(b.vida.estado[v] ?? 0).toFixed(2)}</span></div>
          {/each}
        </div>
        {#if b.vida.candidatos.length}
          <p class="nota">Lo último que pensó hacer: {b.vida.candidatos.map((c) => `${c.nombre} (${c.puntaje.toFixed(2)})`).join(', ')}</p>
        {/if}
      {/if}
    </section>

    <section class="tarjeta grilla">
      <h2>Probar</h2>
      <p class="nota">Le manda el evento como si pasara de verdad (las reglas deciden qué hace).</p>
      <div class="eventos">
        {#each EVENTOS.filter((e) => e !== 'volvio' && e !== 'hito') as e}
          <button class="chico" onclick={() => evento(e)} disabled={!b.bicho?.conectado} title={e}>{NOMBRES_EVENTO[e] ?? e}</button>
        {/each}
      </div>
    </section>
  </div>

  <div class="grilla">
    <section class="tarjeta grilla">
      <h2>Diario</h2>
      {#if !memoria}
        <p class="nota">Todavía no me mandó su memoria (la pido cada 5 minutos mientras está conectado).</p>
      {:else}
        <dl class="datos">
          <div><dt class="etiqueta">Días juntos</dt><dd>{memoria.dias}</dd></div>
          <div><dt class="etiqueta">Racha</dt><dd>{memoria.racha} {memoria.racha === 1 ? 'día' : 'días'} (récord {memoria.rachaMax})</dd></div>
          <div><dt class="etiqueta">Caricias</dt><dd>{memoria.caricias}</dd></div>
          <div><dt class="etiqueta">Toques</dt><dd>{memoria.toques}</dd></div>
          <div><dt class="etiqueta">Desde</dt><dd>{fecha(memoria.nacimiento)}</dd></div>
          <div><dt class="etiqueta">Afecto</dt><dd>{(memoria.estado.afecto ?? 0).toFixed(2)}</dd></div>
        </dl>
        {#if memoria.hitos.length}
          <p>Hitos: {memoria.hitos.map((h) => HITOS[h] ?? h).join(' · ')}</p>
        {/if}
        {#if horaPreferida !== null && horaPreferida >= 0}
          <p>Te espera a eso de las <b>{horaPreferida}:00</b> (a esa hora mira más al frente).</p>
        {/if}
        <div class="horas" aria-label="A qué hora aparecés">
          {#each memoria.horas as v, h}
            <i title="{h}:00 · {v}" style="height: {Math.min(100, v * 8)}%"></i>
          {/each}
        </div>
        {#if gustos.length}<p>Le gusta: {gustos.map(([g]) => NOMBRES_EVENTO[g] ?? g).join(', ')}.</p>{/if}
        {#if memoria.ausenciaMaxima}<p class="nota">Lo más que estuvo sin verte: {memoria.ausenciaMaxima} h.</p>{/if}
        {#if Object.values(memoria.deriva).some((x) => x !== 0)}
          <p class="nota">Cómo lo cambió el trato:
            {RASGOS.filter((r) => memoria!.deriva[r]).map((r) => `${NOMBRES_RASGO[r][0]} ${memoria!.deriva[r] > 0 ? '+' : ''}${Math.round(memoria!.deriva[r] * 100)} %`).join(', ')}</p>
        {/if}
        <p class="nota">Actualizado {actualizada ? new Date(actualizada).toLocaleString('es-AR') : '—'}</p>
      {/if}
    </section>

    <section class="tarjeta grilla">
      <h2>Mudanza</h2>
      <p class="nota">Si cambiaste de placa, el bicho "se muda": su memoria y su config pasan a esta.</p>
      {#if mudanzaPendiente}<p class="aviso-caja">Hay una mudanza en camino: se completa cuando se conecte.</p>{/if}
      <div class="fila">
        <select bind:value={desde} style="flex: 1">
          <option value="">Elegí el bicho viejo…</option>
          {#each otros as o}<option value={o.id}>{o.apodo || o.estado?.nombre || o.id} · {o.id}</option>{/each}
        </select>
        <button onclick={mudar} disabled={!desde}>Mudar</button>
      </div>
      {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
      {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
    </section>
  </div>
</div>

<style>
  .dos { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; align-items: start; }
  @media (max-width: 900px) { .dos { grid-template-columns: 1fr; } }
  .barras { display: grid; gap: 6px; }
  .barra { display: grid; grid-template-columns: 8em 1fr 3em; gap: 8px; align-items: center; font-size: 13px; }
  .barra i { display: block; height: 8px; border-radius: 4px; background: var(--linea, #2c2925); overflow: hidden; }
  .barra i b { display: block; height: 100%; background: #ff6a33; transition: width 0.6s; }
  .barra i b.estres { background: #ff3820; }
  .eventos { display: flex; flex-wrap: wrap; gap: 6px; }
  .datos { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin: 0; }
  .datos dd { margin: 0; font-size: 18px; }
  .horas { display: grid; grid-template-columns: repeat(24, 1fr); align-items: end; gap: 2px; height: 48px; }
  .horas i { display: block; background: #ff6a33; min-height: 2px; border-radius: 2px; }
</style>
