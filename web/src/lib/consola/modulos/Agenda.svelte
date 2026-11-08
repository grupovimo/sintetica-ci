<script lang="ts">
  // Agenda: tu calendario (la dirección secreta iCal de Google Calendar o
  // cualquier .ics). Unos minutos antes de cada evento, te avisa con su cara de
  // apuro; a la noche, lo de mañana. Es de tu cuenta: les avisa a todos tus robots.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { escuchar } from '$lib/eventos.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';

  let { modelo: _ }: { modelo: Modelo } = $props();

  type Evento = { uid: string; titulo: string; inicio: string; fin: string; todoElDia: boolean };
  type Agenda = { conectada: boolean; ics?: string; aviso: number; proximos: Evento[]; leida?: string; error?: string };
  let agenda = $state<Agenda | null>(null);
  let ics = $state('');
  let error = $state('');
  let conectando = $state(false);

  async function cargar() {
    try { agenda = await api<Agenda>('/v1/agenda'); } catch (e) { error = e instanceof ErrorApi ? e.message : 'No pude traer la agenda.'; }
  }
  onMount(() => {
    cargar();
    return escuchar((e) => { if (e.tipo === 'agenda') cargar(); });
  });

  async function conectar() {
    const u = ics.trim();
    if (!u || conectando) return;
    conectando = true;
    error = '';
    try {
      agenda = await api<Agenda>('/v1/agenda', { metodo: 'PUT', cuerpo: { ics: u } });
      ics = '';
    } catch (e) {
      error = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude conectarlo.';
    }
    conectando = false;
  }
  async function desconectar() {
    await api('/v1/agenda', { metodo: 'DELETE' }).catch(() => {});
    await cargar();
  }
  let reloj: ReturnType<typeof setTimeout> | null = null;
  function ponerAviso(v: number) {
    if (!agenda) return;
    agenda.aviso = v;
    if (reloj) clearTimeout(reloj);
    reloj = setTimeout(() => api('/v1/agenda', { metodo: 'PUT', cuerpo: { aviso: v } }).catch(() => {}), 600);
  }

  const SEMANA = ['dom', 'lun', 'mar', 'mié', 'jue', 'vie', 'sáb'];
  const hora = (iso: string) => new Date(iso).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit', hour12: false });
  const esHoy = (iso: string) => new Date(iso).toDateString() === new Date().toDateString();
  // Los de día entero vienen como fecha (medianoche UTC): se leen como el día de esa fecha.
  const fecha = (e: Evento) => (e.todoElDia ? new Date(e.inicio.slice(0, 10) + 'T12:00:00') : new Date(e.inicio));
</script>

<div class="agenda">
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  {#if agenda && !agenda.conectada}
    <ol class="pasos" aria-label="De dónde sale">
      <li class="bandeja">
        <span class="n">1</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9 7 7M17 17l2.1 2.1M4.9 19.1 7 17M17 7l2.1-2.1" /></svg>
        <b>Configuración</b>
      </li>
      <li class="bandeja">
        <span class="n">2</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="3" /><path d="M3 10h18M8 3v4M16 3v4" /></svg>
        <b>Tu calendario</b>
      </li>
      <li class="bandeja">
        <span class="n">3</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg>
        <b>Dirección secreta iCal</b>
      </li>
    </ol>
    <a class="tecla negra chica abrir" href="https://calendar.google.com/calendar/r/settings" target="_blank" rel="noopener">Abrir Google Calendar ↗</a>
    <form class="pegar visor" onsubmit={(e) => { e.preventDefault(); conectar(); }}>
      <span class="grabado">Pegala acá</span>
      <div class="linea">
        <input class="campo claro" type="url" aria-label="Dirección iCal" placeholder="https://calendar.google.com/…/basic.ics" bind:value={ics} />
        <button class="tecla naranja chica" disabled={!ics.trim() || conectando}>{conectando ? '…' : 'Conectar'}</button>
      </div>
    </form>
  {:else if agenda}
    <div class="conectada">
      <div class="bandeja estado">
        <span class="led" class:verde={!agenda.error} class:ambar={!!agenda.error}></span>
        <div class="que">
          <span class="grabado">{agenda.error ? 'No la pude leer' : 'Conectada'}</span>
          <code>{agenda.ics}</code>
        </div>
        <button class="tecla negra chica" onclick={desconectar}>Sacar</button>
      </div>
      <div class="bandeja perilla">
        <Perilla nombre="Avisa" valor={agenda.aviso} min={0} max={15} paso={1} tamano={64} mostrar={(v) => (v ? `${v}′ antes` : 'A la hora')} alCambiar={ponerAviso} />
      </div>
    </div>

    <div class="proximos">
      {#each agenda.proximos as e (e.uid + e.inicio)}
        {@const d = fecha(e)}
        <div class="evento visor" class:hoy={esHoy(d.toISOString())}>
          <div class="fecha"><span class="grabado">{SEMANA[d.getDay()]}</span><span class="digitos chicos">{d.getDate()}</span></div>
          <b>{e.titulo}</b>
          <span class="hora">{e.todoElDia ? 'Todo el día' : hora(e.inicio)}</span>
        </div>
      {:else}
        <div class="visor vacio"><span class="grabado">Nada en dos semanas</span></div>
      {/each}
    </div>
  {:else}
    <p class="grabado">Buscando…</p>
  {/if}
</div>

<style>
  .agenda { display: flex; flex-direction: column; gap: 14px; }
  .pasos { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 0; padding: 0; list-style: none; }
  .pasos li { position: relative; display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 18px 10px; text-align: center; }
  .pasos .n { position: absolute; top: 10px; left: 12px; font: 900 13px/1 var(--marca); color: rgba(26,23,20,.55); }
  .pasos svg { width: 40px; height: 40px; fill: none; stroke: #1a1714; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  .pasos b { font: 900 12px/1.15 var(--marca); text-transform: uppercase; letter-spacing: .03em; }
  .abrir { align-self: flex-start; }
  .pegar { padding: 12px 12px 12px 18px; }
  .linea { display: flex; align-items: center; gap: 12px; }
  .linea .campo { font-size: 16px; }
  .conectada { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 14px; }
  .estado { display: flex; align-items: center; gap: 14px; min-width: 0; }
  .que { flex: 1; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .que code { font: 700 13px/1.3 ui-monospace, monospace; color: rgba(26,23,20,.8); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .perilla { display: grid; place-items: center; padding: 18px 26px; }
  .proximos { display: flex; flex-direction: column; gap: 8px; }
  .evento { flex-direction: row; align-items: center; gap: 16px; padding: 10px 18px 10px 12px; }
  .evento.hoy { box-shadow: inset 0 3px 10px rgba(0,0,0,.8), inset 3px 0 0 #ff6a33; }
  .fecha { display: flex; flex-direction: column; align-items: center; gap: 4px; width: 44px; flex: none; }
  .evento b { flex: 1; min-width: 0; font: 800 17px/1.2 var(--marca); color: var(--c-papel); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .hora { flex: none; font: 800 14px/1 var(--marca); color: var(--brasa); font-variant-numeric: tabular-nums; }
  .vacio { align-items: center; padding: 22px; }
  @media (max-width: 560px) {
    .pasos { gap: 8px; }
    .pasos li { padding: 14px 6px; }
    .pasos svg { width: 30px; height: 30px; }
    .pasos b { font-size: 10px; }
    .conectada { grid-template-columns: minmax(0, 1fr); }
    .linea { flex-direction: column; align-items: stretch; }
  }
</style>
