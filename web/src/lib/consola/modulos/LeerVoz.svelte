<script lang="ts">
  // Con qué voz lee en voz alta: la gratis (Edge) o la de tu cuenta de
  // ElevenLabs (tu clave, tus topes). La llave prende o apaga la lectura.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import Perilla from '../Perilla.svelte';
  import Medidor from '../Medidor.svelte';
  import { escucharPrueba, miles, type VozEdge, type VozEleven } from '../voz';

  const b = usarBicho();
  const nombre = $derived(b.bicho?.apodo || b.config.nombre || 'tu robot');

  let edge = $state<VozEdge | null>(null);
  let eleven = $state<VozEleven | null>(null);
  let pestana = $state<'edge' | 'eleven'>('edge');
  let clave = $state('');
  let ocupado = $state('');
  let error = $state('');

  const texto = (e: unknown, por: string) => (e instanceof ErrorApi || e instanceof Error ? e.message : por);
  const leyendoEleven = $derived(!!eleven?.conectado && !!eleven.leer);
  const leyendo = $derived(leyendoEleven || !!(edge?.disponible && edge.prendida));

  async function cargar(primera = false) {
    const [e, x] = await Promise.all([api<VozEdge>('/v1/voz/edge').catch(() => null), api<VozEleven>('/v1/voz/elevenlabs').catch(() => null)]);
    edge = e;
    eleven = x;
    if (primera) pestana = x?.conectado && (x.leer || !e?.disponible) ? 'eleven' : e?.disponible ? 'edge' : 'eleven';
  }
  onMount(() => {
    cargar(true);
    return escuchar((e) => { if (e.tipo === 'voz') cargar(); });
  });

  async function ponerEdge(c: { prendida?: boolean; voz?: string }) {
    error = '';
    try { edge = await api<VozEdge>('/v1/voz/edge', { metodo: 'PUT', cuerpo: c }); } catch (e) { error = texto(e, 'No pude cambiarla.'); }
  }
  async function ponerEleven(c: Record<string, unknown>) {
    error = '';
    try { eleven = await api<VozEleven>('/v1/voz/elevenlabs', { metodo: 'PUT', cuerpo: c }); return true; } catch (e) { error = texto(e, 'No pude cambiarla.'); return false; }
  }

  async function elegir(p: 'edge' | 'eleven') {
    pestana = p;
    if (p === 'edge' && leyendoEleven) {
      await ponerEleven({ leer: false });
      if (edge?.disponible && !edge.prendida) await ponerEdge({ prendida: true });
    }
    if (p === 'eleven' && eleven?.conectado && !eleven.leer) await ponerEleven({ leer: true });
  }
  async function llave() {
    if (leyendo) {
      if (leyendoEleven) await ponerEleven({ leer: false });
      if (edge?.prendida) await ponerEdge({ prendida: false });
    } else if (pestana === 'eleven' && eleven?.conectado) await ponerEleven({ leer: true });
    else if (edge?.disponible) await ponerEdge({ prendida: true });
  }

  async function conectar() {
    const c = clave.trim();
    if (!c) return;
    ocupado = 'conectar';
    if (await ponerEleven({ clave: c })) clave = '';
    ocupado = '';
  }
  async function desconectar() {
    ocupado = 'desconectar';
    try { eleven = await api<VozEleven>('/v1/voz/elevenlabs', { metodo: 'DELETE' }); } catch (e) { error = texto(e, 'No pude desconectarla.'); }
    ocupado = '';
  }

  async function probar(cual: 'edge' | 'elevenlabs') {
    error = '';
    ocupado = 'probar';
    try { await escucharPrueba(`/v1/voz/${cual}/probar`, `¡Hola! Soy ${nombre}.`); } catch (e) { error = texto(e, 'No sonó.'); }
    ocupado = '';
  }

  // Los topes van en escalones (una perilla de 0 a 200 000 de a uno no se puede girar).
  const LETRAS = [0, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000];
  const MINUTOS = [0, 5, 10, 15, 20, 30, 45, 60, 90, 120, 240];
  const escalon = (l: readonly number[], v: number) => l.reduce((m, x, i) => (Math.abs(x - v) < Math.abs(l[m] - v) ? i : m), 0);
  let reloj: ReturnType<typeof setTimeout> | null = null;
  function tope(c: { tope?: number; tope_minutos?: number }) {
    if (!eleven) return;
    Object.assign(eleven, c);
    if (reloj) clearTimeout(reloj);
    reloj = setTimeout(() => ponerEleven(c), 700);
  }

  // Las voces de Edge, por país: "Elena · AR".
  const vocesEdge = $derived((edge?.voces ?? []).map((v) => ({ id: v.id, nombre: `${v.nombre.split(' ')[0]} · ${v.idioma.slice(3)}` })));
</script>

<div class="leer bandeja">
  <div class="cabeza">
    <button class="llave" aria-pressed={leyendo} onclick={llave}><span class="led"></span>Leer en voz alta</button>
    <div class="fuentes" role="radiogroup" aria-label="Con qué voz">
      {#if edge?.disponible}
        <button role="radio" aria-checked={pestana === 'edge'} class="fuente" onclick={() => elegir('edge')}>
          <span class="led" class:verde={pestana === 'edge' && !!edge.prendida && !leyendoEleven}></span>Gratis
        </button>
      {/if}
      <button role="radio" aria-checked={pestana === 'eleven'} class="fuente" onclick={() => elegir('eleven')}>
        <span class="led" class:verde={leyendoEleven} class:ambar={!!eleven?.conectado && !eleven.leer}></span>ElevenLabs
      </button>
    </div>
  </div>

  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  {#if pestana === 'edge' && edge?.disponible}
    <div class="fila">
      <label class="visor voz">
        <span class="grabado">Voz</span>
        <select class="campo claro" value={edge.voz ?? ''} onchange={(e) => ponerEdge({ voz: e.currentTarget.value })}>
          {#each vocesEdge as v (v.id)}<option value={v.id}>{v.nombre}</option>{/each}
        </select>
      </label>
      <button class="tecla naranja probar" disabled={ocupado === 'probar'} onclick={() => probar('edge')}><span aria-hidden="true">▶</span>{ocupado === 'probar' ? '…' : 'Probar'}</button>
    </div>
    <Medidor nombre="Letras hoy" valor={edge.usados_hoy ?? 0} de={edge.tope ?? 0} mostrar={miles} />
  {:else if eleven && !eleven.conectado}
    <form class="fila" onsubmit={(e) => { e.preventDefault(); conectar(); }}>
      <label class="visor voz">
        <span class="grabado">Clave de ElevenLabs</span>
        <input class="campo claro" type="password" autocomplete="off" spellcheck="false" placeholder="sk_…" bind:value={clave} />
      </label>
      <button class="tecla naranja probar" disabled={!clave.trim() || ocupado === 'conectar'}>{ocupado === 'conectar' ? '…' : 'Conectar'}</button>
    </form>
    <a class="sacar grabado" href="https://elevenlabs.io/app/settings/api-keys" target="_blank" rel="noopener">Sacar una clave ↗</a>
  {:else if eleven?.conectado}
    <div class="fila">
      <label class="visor voz">
        <span class="grabado">Voz de tu cuenta</span>
        <select class="campo claro" value={eleven.voz ?? ''} onchange={(e) => ponerEleven({ voz: e.currentTarget.value })}>
          {#each eleven.voces ?? [] as v (v.id)}<option value={v.id}>{v.nombre}</option>{/each}
        </select>
      </label>
      <button class="tecla naranja probar" disabled={ocupado === 'probar'} onclick={() => probar('elevenlabs')}><span aria-hidden="true">▶</span>{ocupado === 'probar' ? '…' : 'Probar'}</button>
    </div>
    <div class="topes">
      <Perilla nombre="Letras/día" min={0} max={LETRAS.length - 1} paso={1} valor={escalon(LETRAS, eleven.tope ?? 5000)}
        mostrar={(i) => miles(LETRAS[i])} alCambiar={(i) => tope({ tope: LETRAS[i] })} />
      <Perilla nombre="Min/día" min={0} max={MINUTOS.length - 1} paso={1} valor={escalon(MINUTOS, eleven.tope_minutos ?? 10)}
        mostrar={(i) => (MINUTOS[i] ? String(MINUTOS[i]) : 'Off')} alCambiar={(i) => tope({ tope_minutos: MINUTOS[i] })} />
      <div class="medidores">
        <Medidor nombre="Letras hoy" valor={eleven.usados_hoy ?? 0} de={eleven.tope ?? 0} mostrar={miles} />
        <Medidor nombre="Min hoy" valor={Math.round((eleven.segundos_hoy ?? 0) / 60)} de={eleven.tope_minutos ?? 0} />
      </div>
    </div>
    <button class="tecla negra chica salir" disabled={ocupado === 'desconectar'} onclick={desconectar}>Desconectar</button>
  {/if}
</div>

<style>
  .leer { display: flex; flex-direction: column; gap: 14px; }
  .cabeza { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
  .fuentes { display: inline-flex; gap: 4px; padding: 4px; border-radius: 13px; background: var(--hundido); box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .fuente { display: inline-flex; align-items: center; gap: 8px; min-height: 38px; padding: 0 14px; border-radius: 10px; cursor: pointer;
    font: 800 11px/1 var(--marca); letter-spacing: .1em; text-transform: uppercase; color: var(--gris); }
  .fuente[aria-checked='true'] { color: #1a1714; background: linear-gradient(180deg, #fbf9f5, #e6e0d6); box-shadow: 0 3px 0 #8f887c; }
  .fila { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: stretch; }
  .voz { padding: 12px 16px; gap: 6px; }
  .probar { min-height: 100%; }
  .topes { display: flex; flex-wrap: wrap; align-items: center; gap: 24px; padding: 6px 4px 0; }
  .medidores { flex: 1; min-width: 200px; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  .sacar { align-self: flex-start; text-decoration: none; color: rgba(26,23,20,.7); }
  .salir { align-self: flex-start; }
  @media (max-width: 560px) { .topes { justify-content: space-around; } .medidores { grid-template-columns: 1fr 1fr; min-width: 100%; } }
</style>
