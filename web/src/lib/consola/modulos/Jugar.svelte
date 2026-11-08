<script lang="ts">
  // Jugar: cinco juegos en su pantalla (los arranca la nube con 0x43) y el
  // marcador que guarda en su memoria.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { JUEGOS, type Juego } from '$lib/formato';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const conectado = $derived(b.bicho?.conectado ?? false);

  const CORTO: Record<Juego, string> = { ppt: 'Piedra papel tijera', reflejos: 'Reflejos', mirada: 'Mirada', dado: 'Dado', moneda: 'Moneda' };

  // El marcador: la memoria decodificada (si la nube todavía no trae los campos, queda en —).
  let marcador = $state<{ jugados?: number; ganados?: number; reflejo?: number; hoy?: number }>({});
  const num = (m: Record<string, unknown>, ...k: string[]) => { for (const x of k) if (typeof m[x] === 'number') return m[x] as number; return undefined; };
  onMount(async () => {
    try {
      const r = await api<{ memoria: Record<string, unknown> | null }>(`/v1/bichos/${b.id}/memoria`);
      const m = r.memoria ?? {};
      marcador = { jugados: num(m, 'juegosJugados', 'jugados'), ganados: num(m, 'juegosGanados', 'ganados'), reflejo: num(m, 'mejorReflejo'), hoy: num(m, 'juegosHoy') };
    } catch { /* sin memoria todavía */ }
  });
  const reflejo = $derived(marcador.reflejo ? (marcador.reflejo / 1000).toFixed(2).replace('.', ',') : '—');

  let jugando = $state<Juego | null>(null);
  let aviso = $state('');
  async function jugar(juego: Juego) {
    if (jugando || !conectado) return;
    jugando = juego;
    try {
      await api(`/v1/bichos/${b.id}/jugar`, { cuerpo: { juego } });
      aviso = '¡A jugar!';
    } catch (e) {
      aviso = e instanceof ErrorApi ? (e.estado === 409 ? 'Desconectado' : e.estado === 404 ? 'Todavía no sabe' : e.detalle || e.message) : 'No pude';
    }
    jugando = null;
    setTimeout(() => (aviso = ''), 2500);
  }
</script>

<div class="jugar">
  <div class="juegos" role="group" aria-label="Juegos">
    {#each JUEGOS as [j, n]}
      <button class="juego" class:abajo={jugando === j} disabled={!conectado} aria-label={n} title={n} onclick={() => jugar(j)}>
        <svg viewBox="0 0 40 40" aria-hidden="true">
          {#if j === 'ppt'}
            <circle cx="12" cy="29" r="5" /><circle cx="28" cy="29" r="5" /><path d="M15 25 29 6M25 25 11 6" />
          {:else if j === 'reflejos'}
            <path class="lleno" d="M23 3 9 23h10l-3 14 15-21H21z" />
          {:else if j === 'mirada'}
            <path d="M3 20s6-11 17-11 17 11 17 11-6 11-17 11S3 20 3 20z" /><circle class="lleno" cx="24" cy="20" r="5" />
          {:else if j === 'dado'}
            <rect x="6" y="6" width="28" height="28" rx="7" />
            {#each [[13, 13], [27, 13], [20, 20], [13, 27], [27, 27]] as [x, y]}<circle class="lleno" cx={x} cy={y} r="2.6" />{/each}
          {:else}
            <circle cx="20" cy="20" r="15" /><circle cx="20" cy="20" r="9.5" /><path d="M20 15v10" />
          {/if}
        </svg>
        <b>{CORTO[j]}</b>
      </button>
    {/each}
  </div>
  {#if aviso}<p class="aviso" aria-live="polite"><span class="led on"></span>{aviso}</p>{/if}

  <div class="marcador">
    {#each [['Jugados', marcador.jugados ?? '—'], ['Ganó', marcador.ganados ?? '—'], ['Hoy', marcador.hoy ?? '—'], ['Reflejo', reflejo]] as [t, v]}
      <div class="visor"><span class="grabado">{t}</span><span class="digitos">{v}{#if t === 'Reflejo' && marcador.reflejo}<small>s</small>{/if}</span></div>
    {/each}
  </div>
</div>

<style>
  .jugar { display: flex; flex-direction: column; gap: 16px; }
  .juegos { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; padding-bottom: 6px; }
  .juego { display: flex; flex-direction: column; align-items: center; justify-content: space-between; gap: 10px; min-height: 120px; padding: 16px 8px 12px;
    border: 0; border-radius: 18px; cursor: pointer; color: #1a1714; font-family: var(--marca);
    background: linear-gradient(180deg, #fbf9f5 0%, #e6e0d6 100%);
    box-shadow: inset 0 1px 0 #fff, inset 0 -3px 0 rgba(0,0,0,.1), 0 6px 0 #8f887c, 0 10px 16px -6px rgba(80,20,0,.55); transition: transform .08s, box-shadow .08s; }
  .juego:not(:disabled):active, .juego.abajo { transform: translateY(5px); box-shadow: inset 0 1px 0 #fff, 0 1px 0 #8f887c; }
  .juego:disabled { cursor: not-allowed; opacity: .55; }
  .juego svg { width: 44px; height: 44px; fill: none; stroke: currentColor; stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
  .juego svg .lleno { fill: #f2541a; stroke: none; }
  .juego:hover:not(:disabled) svg { color: #f2541a; }
  .juego b { font: 900 11px/1.1 var(--marca); text-transform: uppercase; letter-spacing: .02em; text-align: center; }
  .marcador { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
  .marcador .digitos { font-size: 40px; }
  .marcador small { font-size: 18px; margin-left: 2px; }
  @media (max-width: 760px) {
    .juegos { grid-template-columns: repeat(3, minmax(0, 1fr)); }
    .marcador { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
</style>
