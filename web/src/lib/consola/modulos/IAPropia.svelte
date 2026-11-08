<script lang="ts">
  // La IA de la charla: la del servidor, la tuya (tu clave de Anthropic o de
  // un servicio compatible, tu tope y tu factura) o ninguna. Tres LEDs dicen
  // cuál está usando.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { escuchar } from '$lib/eventos.svelte';
  import Perilla from '../Perilla.svelte';
  import Medidor from '../Medidor.svelte';
  import { SERVICIOS, type IA } from '../voz';

  let ia = $state<IA | null>(null);
  let servicio = $state<(typeof SERVICIOS)[number]['clave']>('groq');
  let url = $state<string>(SERVICIOS[1].url);
  let modelo = $state<string>(SERVICIOS[1].modelo);
  let clave = $state('');
  let abierto = $state(false);
  let ocupado = $state(false);
  let error = $state('');

  const s = $derived(SERVICIOS.find((x) => x.clave === servicio)!);
  const cual = $derived(ia?.propia ? 'tuya' : ia?.servidor ? 'servidor' : 'ninguna');

  async function cargar() { ia = await api<IA>('/v1/ia').catch(() => null); }
  onMount(() => {
    cargar();
    return escuchar((e) => { if (e.tipo === 'ia') cargar(); });
  });

  function elegir(k: typeof servicio) {
    servicio = k;
    const x = SERVICIOS.find((y) => y.clave === k)!;
    url = x.url;
    modelo = x.modelo;
  }
  const texto = (e: unknown, por: string) => (e instanceof ErrorApi ? e.message : por);

  async function conectar() {
    if (!clave.trim()) return;
    error = '';
    ocupado = true;
    try {
      const cuerpo: Record<string, unknown> = { proveedor: s.proveedor, clave: clave.trim() };
      if (modelo.trim()) cuerpo.modelo = modelo.trim();
      if (s.proveedor === 'compatible') cuerpo.url = url.trim();
      ia = await api<IA>('/v1/ia', { metodo: 'PUT', cuerpo });
      clave = '';
      abierto = false;
    } catch (e) { error = texto(e, 'No pude conectarla.'); }
    ocupado = false;
  }
  async function cambiar(c: { modelo?: string; tope?: number }) {
    error = '';
    try { ia = await api<IA>('/v1/ia', { metodo: 'PUT', cuerpo: c }); } catch (e) { error = texto(e, 'No pude cambiarla.'); cargar(); }
  }
  async function borrar() {
    error = '';
    try { await api('/v1/ia', { metodo: 'DELETE' }); } catch (e) { error = texto(e, 'No pude borrarla.'); }
    await cargar();
  }

  // El tope en escalones; 0 es sin tope.
  const TOPES = [0, 10, 20, 30, 50, 100, 200, 500, 1000, 2000, 5000, 10000];
  const escalon = (v: number) => TOPES.reduce((m, x, i) => (Math.abs(x - v) < Math.abs(TOPES[m] - v) ? i : m), 0);
  let reloj: ReturnType<typeof setTimeout> | null = null;
  function tope(i: number) {
    if (!ia) return;
    ia.tope = TOPES[i];
    if (reloj) clearTimeout(reloj);
    reloj = setTimeout(() => cambiar({ tope: TOPES[i] }), 700);
  }
  const servicioDe = (x: IA) => (x.proveedor === 'anthropic' ? 'Anthropic' : SERVICIOS.find((y) => y.url && x.url_pista && y.url.includes(x.url_pista))?.nombre ?? x.url_pista ?? 'Compatible');
</script>

{#if ia}
  <div class="ia-propia bandeja">
    <div class="logo" aria-hidden="true">
      <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="#ff8a5c" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6.5a2.5 2.5 0 0 1 2.5-2.5h11A2.5 2.5 0 0 1 20 6.5v7a2.5 2.5 0 0 1-2.5 2.5H10l-4 4v-4H6.5A2.5 2.5 0 0 1 4 13.5z" /><path d="M9 10h.01M12 10h.01M15 10h.01" /></svg>
    </div>
    <div class="que">
      <span class="grabado">Charla · con qué IA</span>
      <div class="cual" role="img" aria-label="Usa {cual === 'tuya' ? 'tu IA' : cual === 'servidor' ? 'la del servidor' : 'ninguna IA'}">
        <span class:prendido={cual === 'servidor'}><span class="led" class:verde={cual === 'servidor'}></span>Servidor</span>
        <span class:prendido={cual === 'tuya'}><span class="led" class:verde={cual === 'tuya'}></span>Tuya</span>
        <span class:prendido={cual === 'ninguna'}><span class="led" class:ambar={cual === 'ninguna'}></span>Ninguna</span>
      </div>
    </div>
    {#if !ia.propia && !abierto}<button class="tecla naranja chica" onclick={() => (abierto = true)}>+ Tu clave</button>{/if}

    {#if error}<p class="aviso error todo"><span class="led"></span>{error}</p>{/if}

    {#if ia.propia}
      <div class="todo detalle">
        <div class="visor"><span class="grabado">Servicio</span><b class="dato">{servicioDe(ia)}</b></div>
        <label class="visor"><span class="grabado">Modelo</span>
          <input class="campo claro" value={ia.modelo ?? ''} spellcheck="false" onchange={(e) => { const m = e.currentTarget.value.trim(); if (m && m !== ia?.modelo) cambiar({ modelo: m }); }} /></label>
        <Perilla nombre="Tope/día" min={0} max={TOPES.length - 1} paso={1} valor={escalon(ia.tope ?? 0)} mostrar={(i) => (TOPES[i] ? String(TOPES[i]) : '∞')} alCambiar={tope} tamano={56} />
        <Medidor nombre="Hoy" valor={ia.usadas_hoy ?? 0} de={ia.tope ?? 0} />
        <button class="tecla negra chica borrar" onclick={borrar}>Borrar</button>
      </div>
    {:else if abierto}
      <form class="todo forma" onsubmit={(e) => { e.preventDefault(); conectar(); }}>
        <div class="servicios" role="group" aria-label="Servicio">
          {#each SERVICIOS as x (x.clave)}<button type="button" class="tecla chica" aria-pressed={servicio === x.clave} onclick={() => elegir(x.clave)}>{x.nombre}</button>{/each}
        </div>
        {#if s.proveedor === 'compatible'}
          <label class="visor"><span class="grabado">URL</span><input class="campo claro" type="url" spellcheck="false" placeholder="https://…/v1" bind:value={url} /></label>
        {/if}
        <div class="dos">
          <label class="visor"><span class="grabado">Modelo</span><input class="campo claro" spellcheck="false" bind:value={modelo} /></label>
          <label class="visor"><span class="grabado">Clave</span><input class="campo claro" type="password" autocomplete="off" spellcheck="false" bind:value={clave} /></label>
        </div>
        <div class="botones">
          <button class="tecla naranja chica" disabled={ocupado || !clave.trim() || (s.proveedor === 'compatible' && (!url.trim() || !modelo.trim()))}>{ocupado ? '…' : 'Conectar'}</button>
          <button type="button" class="tecla negra chica" onclick={() => (abierto = false)}>Cancelar</button>
          {#if s.claves}<a class="grabado sacar" href={s.claves} target="_blank" rel="noopener">Sacar una clave ↗</a>{/if}
        </div>
      </form>
    {:else if ia.servidor}
      <div class="todo"><Medidor nombre="Hoy" valor={ia.usadas_hoy ?? 0} de={ia.tope ?? 0} /></div>
    {/if}
  </div>
{/if}

<style>
  .ia-propia { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 14px 18px; align-items: center; }
  .logo { display: grid; place-items: center; width: 56px; height: 56px; border-radius: 16px; background: linear-gradient(180deg, #2a2622, #151310);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.15), 0 5px 0 #000; }
  .que { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .cual { display: flex; flex-wrap: wrap; gap: 6px; }
  .cual > span { display: inline-flex; align-items: center; gap: 7px; min-height: 30px; padding: 0 10px; border-radius: 9px; background: var(--hundido);
    font: 800 11px/1 var(--marca); letter-spacing: .1em; text-transform: uppercase; color: #5a524a; box-shadow: inset 0 2px 4px rgba(0,0,0,.8); }
  .cual > span.prendido { color: var(--c-papel); }
  .todo { grid-column: 1 / -1; }
  .detalle { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr) auto minmax(0, 1fr); gap: 12px; align-items: center; }
  .detalle .visor { padding: 12px 14px; gap: 6px; }
  .detalle .campo { font-size: 16px; }
  .dato { font: 900 16px/1.1 var(--marca); color: var(--c-papel); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .borrar { grid-column: 1 / -1; justify-self: start; }
  .forma { display: flex; flex-direction: column; gap: 10px; }
  .forma .visor { padding: 12px 14px; gap: 6px; }
  .forma .campo { font-size: 16px; }
  .servicios { display: flex; flex-wrap: wrap; gap: 10px 8px; padding-bottom: 6px; }
  .dos { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 10px; }
  .botones { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; padding-top: 4px; }
  .sacar { margin-left: auto; text-decoration: none; }
  @media (max-width: 700px) {
    .ia-propia { grid-template-columns: auto minmax(0, 1fr); }
    .ia-propia > .tecla { grid-column: 1 / -1; justify-self: start; }
    .detalle { grid-template-columns: auto minmax(0, 1fr); }
    .detalle > .visor { grid-column: 1 / -1; }
    .dos { grid-template-columns: 1fr; }
  }
</style>
