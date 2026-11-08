<script lang="ts">
  // Hablarle en tiempo real desde la web: una tecla de micrófono que abre la
  // conversación con el agente de ElevenLabs de este robot (la cuenta del
  // dueño). Sus herramientas (poner_expresion y mirar) mueven la cara de acá y,
  // si el robot está conectado, también la de él. Corta sola al silencio.
  import { onDestroy, onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { Car, comando } from '$compartido/protocolo.js';
  import type { VozEleven } from './voz';

  let { alCara, alMirar }: { alCara: (e: string | null) => void; alMirar: (x: number, y: number) => void } = $props();
  const b = usarBicho();

  type Sesion = { endSession: () => Promise<void> };
  type Pedido = { url: string; segundos: number; silencio: number; quedan: number; expresiones?: string[] };
  let disponible = $state(false);
  let estado = $state<'quieto' | 'conectando' | 'escucha' | 'habla'>('quieto');
  let quedan = $state(0);
  let aviso = $state('');
  let sesion: Sesion | null = null;
  let reloj: ReturnType<typeof setInterval> | null = null;
  let puso = false; // si le cambiamos la cara al robot (para soltarla al cortar)
  let expresion: string | null = null;

  async function mirarDisponible() {
    const v = await api<VozEleven>('/v1/voz/elevenlabs').catch(() => null);
    disponible = !!v?.conectado && (v.tope_minutos ?? 0) > 0;
  }
  onMount(() => {
    mirarDisponible();
    return escuchar((e) => { if (e.tipo === 'voz') mirarDisponible(); });
  });
  onDestroy(() => { cortar(); });

  const conectado = () => !!b.bicho?.conectado;
  function avisar(t: string) {
    aviso = t;
    setTimeout(() => (aviso = ''), 3000);
  }

  async function empezar() {
    if (estado !== 'quieto') return cortar();
    estado = 'conectando';
    try {
      // El permiso primero: así un "no" se ve como tal y no como un error de la conversación.
      const mic = await navigator.mediaDevices.getUserMedia({ audio: true });
      mic.getTracks().forEach((t) => t.stop());
    } catch {
      estado = 'quieto';
      return avisar('Sin micrófono');
    }
    let c: Pedido;
    try {
      c = await api<Pedido>(`/v1/bichos/${b.id}/conversacion`);
    } catch (e) {
      estado = 'quieto';
      if (e instanceof ErrorApi && e.estado === 404) disponible = false;
      return avisar(e instanceof ErrorApi && e.estado === 429 ? 'Por hoy ya hablaron' : e instanceof ErrorApi ? e.message : 'No pude');
    }
    const caras = new Set(c.expresiones ?? []);
    try {
      const { Conversation } = await import('@elevenlabs/client');
      sesion = await Conversation.startSession({
        signedUrl: c.url,
        connectionType: 'websocket',
        clientTools: {
          poner_expresion: ({ nombre }: { nombre?: string }) => {
            if (!nombre || (caras.size && !caras.has(nombre))) return 'no la tengo';
            expresion = nombre;
            alCara(nombre);
            if (conectado()) { puso = true; b.comando({ expresion: nombre }); }
            return 'ok';
          },
          mirar: ({ x = 0, y = 0 }: { x?: number; y?: number }) => {
            const fx = Math.max(-1, Math.min(1, x / 100)), fy = Math.max(-1, Math.min(1, y / 100));
            alMirar(fx, fy);
            if (conectado()) b.marco(Car.comando, comando.mirar(fx, fy));
            return 'ok';
          },
        },
        onModeChange: ({ mode }) => {
          estado = mode === 'speaking' ? 'habla' : 'escucha';
          // Escuchando, pone cara de escuchar (si el agente no eligió otra).
          alCara(expresion ?? (mode === 'listening' ? 'escuchando' : null));
        },
        onDisconnect: () => terminar(),
        onError: (m) => avisar(typeof m === 'string' && m.length < 40 ? m : 'Se cortó'),
      });
      estado = 'escucha';
      quedan = c.segundos;
      reloj = setInterval(() => { if (--quedan <= 0) cortar(); }, 1000);
    } catch {
      terminar();
      avisar('No pude conectar');
    }
  }

  async function cortar() {
    const s = sesion;
    sesion = null;
    if (s) await s.endSession().catch(() => {});
    terminar();
  }
  function terminar() {
    if (reloj) clearInterval(reloj);
    reloj = null;
    sesion = null;
    estado = 'quieto';
    expresion = null;
    alCara(null);
    alMirar(0, 0);
    if (puso && conectado()) b.comando({ expresion: '' });
    puso = false;
  }
  const mmss = (s: number) => `${Math.floor(s / 60)}:${String(Math.max(0, s) % 60).padStart(2, '0')}`;
  const ETIQUETA = { quieto: 'Hablale', conectando: '…', escucha: 'Te escucha', habla: 'Habla' } as const;
</script>

{#if disponible}
  <div class="hablar">
    <button class="mic {estado}" aria-pressed={estado !== 'quieto'} aria-label={estado === 'quieto' ? 'Hablarle' : 'Cortar'} onclick={empezar}>
      {#if estado === 'quieto' || estado === 'conectando'}
        <svg viewBox="0 0 24 24" width="30" height="30" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="3" width="6" height="11" rx="3" /><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21" /></svg>
      {:else}
        <svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true"><rect x="6" y="6" width="12" height="12" rx="3" fill="currentColor" /></svg>
      {/if}
    </button>
    <div class="estado visor" aria-live="polite">
      <span class="grabado"><span class="led" class:verde={estado === 'escucha'} class:on={estado === 'habla'} class:ambar={estado === 'conectando'}></span>{aviso || ETIQUETA[estado]}</span>
      {#if estado === 'escucha' || estado === 'habla'}
        <span class="ondas" class:habla={estado === 'habla'} aria-hidden="true"><i></i><i></i><i></i><i></i><i></i></span>
        <span class="tiempo">{mmss(quedan)}</span>
      {/if}
    </div>
  </div>
{/if}

<style>
  .hablar { display: flex; align-items: center; gap: 18px; padding-left: 8px; min-width: 0; }
  .mic { display: grid; place-items: center; width: 76px; height: 76px; flex: none; border-radius: 50%; cursor: pointer; color: #1a1714;
    background: radial-gradient(circle at 35% 28%, #ffa27c, #f2541a 70%);
    box-shadow: inset 0 2px 0 rgba(255,255,255,.45), inset 0 -4px 0 rgba(0,0,0,.2), 0 7px 0 #9a3209, 0 14px 22px -6px rgba(0,0,0,.6), 0 0 0 6px #1c1a17, 0 0 0 8px #2c2925;
    transition: transform .08s, box-shadow .08s; }
  .mic:active, .mic[aria-pressed='true'] { transform: translateY(5px);
    box-shadow: inset 0 2px 0 rgba(255,255,255,.45), 0 2px 0 #9a3209, 0 0 0 6px #1c1a17, 0 0 0 8px var(--led), 0 0 22px 6px rgba(255,106,51,.45); }
  .mic.escucha { box-shadow: inset 0 2px 0 rgba(255,255,255,.45), 0 2px 0 #9a3209, 0 0 0 6px #1c1a17, 0 0 0 8px var(--verde), 0 0 22px 6px rgba(74,222,128,.35); }
  .mic.conectando { animation: late 1s ease-in-out infinite; }
  @keyframes late { 50% { filter: brightness(.8); } }
  .estado { flex-direction: row; align-items: center; gap: 14px; padding: 12px 16px; min-height: 52px; flex: 1; min-width: 0; max-width: 360px; }
  .estado .grabado { display: inline-flex; align-items: center; gap: 8px; color: var(--c-papel); white-space: nowrap; }
  .ondas { display: inline-flex; align-items: center; gap: 3px; height: 20px; flex: 1; }
  .ondas i { width: 4px; height: 30%; border-radius: 2px; background: var(--verde); animation: onda .8s ease-in-out infinite alternate; }
  .ondas.habla i { background: var(--led); }
  .ondas i:nth-child(2) { animation-delay: -.2s; } .ondas i:nth-child(3) { animation-delay: -.5s; } .ondas i:nth-child(4) { animation-delay: -.1s; } .ondas i:nth-child(5) { animation-delay: -.65s; }
  @keyframes onda { to { height: 100%; } }
  .tiempo { font: 900 15px/1 var(--marca); font-variant-numeric: tabular-nums; color: var(--brasa); margin-left: auto; }
</style>
