<script lang="ts">
  // Reglas: cuando le pasa algo, qué hace. Se elige el evento en la botonera,
  // se ve qué responde (las tuyas y las de fábrica) y se prueba en el robot
  // vivo de al lado. Las tuyas van antes que las de fábrica.
  import { onDestroy, onMount } from 'svelte';
  import { usarBicho } from '$lib/bicho.svelte';
  import { foto } from '$lib/foto.svelte';
  import { pintar, vistaPrevia, type VistaPrevia } from '$lib/nucleo';
  import {
    clonar, COMPORTAMIENTOS_BASE, ESTADO, EVENTOS, NOMBRES_ESTADO, NOMBRES_EVENTO, PARTICULAS, REGLAS_BASE, type Config, type Regla,
  } from '$lib/formato';
  import { pantallaDe } from '$lib/tipos';
  import { miniaturasDeCaras } from '$lib/consola/miniaturas';

  const b = usarBicho();
  let reglas = $state<Regla[]>(clonar($state.snapshot(b.config.reglas ?? [])) as Regla[]);
  let evento = $state('tacto');
  let eligiendo = $state<{ regla: number; tipo: string } | null>(null);
  let error = $state('');
  let aviso = $state('');
  let guardando = $state(false);
  let caras = $state<Record<string, string>>({});
  let canvas = $state<HTMLCanvasElement>();
  let sim: VistaPrevia | null = null;
  let cuadro = 0;
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });
  const sucio = $derived(JSON.stringify(reglas) !== JSON.stringify(b.config.reglas ?? []));
  const comportamientos = $derived([...new Set([...Object.keys(COMPORTAMIENTOS_BASE), ...Object.keys(b.config.comportamientos ?? {})])]);

  // Los eventos, agrupados, con su ícono.
  const I = {
    toque: 'M9 11V5a2 2 0 1 1 4 0v6 M13 10a2 2 0 1 1 4 0v3a7 7 0 0 1-7 7 5 5 0 0 1-4.5-3L4 13a1.5 1.5 0 0 1 2.5-1.5L9 14',
    mueve: 'M5 9l-3 3 3 3 M19 9l3 3-3 3 M2 12h20 M12 2v20',
    oye: 'M3 10v4 M7 7v10 M11 4v16 M15 8v8 M19 11v2',
    vida: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21.2l8.8-8.8a5.5 5.5 0 0 0 0-7.8Z',
    sistema: 'M3 7h15v10H3z M21 10v4',
  };
  const GRUPO: Record<string, keyof typeof I> = {
    tacto: 'toque', 'doble-toque': 'toque', caricia: 'toque', 'muchos-toques': 'toque', despertado: 'toque',
    levantado: 'mueve', sacudida: 'mueve', 'sacudida-suave': 'mueve', 'sacudida-fuerte': 'mueve', 'boca-abajo': 'mueve', inclinado: 'mueve', caida: 'mueve',
    voz: 'oye', nombre: 'oye', ruido: 'oye', musica: 'oye', silencio: 'oye',
    volvio: 'vida', hito: 'vida', 'bicho-cerca': 'vida', 'cambio-de-cara': 'vida', vinculado: 'vida', 'app-abierta': 'vida',
  };
  const iconoDe = (e: string) => I[GRUPO[e] ?? 'sistema'];
  const corto = (e: string) => (NOMBRES_EVENTO[e] ?? e).replace(/ \(.*\)$/, '');
  const cuantas = (e: string) => reglas.filter((r) => r.si === e).length;

  const GLIFO: Record<string, string> = { zzz: 'z', corazon: '♥', lagrima: '💧', gota: '·', chispa: '✦', pregunta: '?', exclamacion: '!', nota: '♪', estrella: '★', burbuja: '○' };
  const ACCIONES: { tipo: string; nombre: string }[] = [
    { tipo: 'reflejo', nombre: 'Cara ya' }, { tipo: 'expresion', nombre: 'Cara 3 s' }, { tipo: 'comportamiento', nombre: 'Hace' },
    { tipo: 'particula', nombre: 'Le sale' }, { tipo: 'estado', nombre: 'Siente' }, { tipo: 'led', nombre: 'Luz' },
  ];
  const LEDS = ['#FF6A33', '#FFB347', '#F3EEE8', '#4ADE80', '#3DE0FF', '#A78BFA', '#FF5FA2'];
  const partir = (a: string): [string, string] => (a.startsWith('estado.') ? ['estado', a.slice(7)] : [a.slice(0, a.indexOf(':')), a.slice(a.indexOf(':') + 1)]);
  const unir = (tipo: string, arg: string) => (tipo === 'estado' ? `estado.${arg}` : `${tipo}:${arg}`);
  const partirEstado = (arg: string) => { const m = /^([a-z]+)([+-])([0-9.]+)$/.exec(arg); return m ? { v: m[1], s: m[2], n: +m[3] } : { v: 'animo', s: '+', n: 0.1 }; };

  const propias = $derived(reglas.map((r, i) => ({ r, i })).filter(({ r }) => r.si === evento));
  const deFabrica = $derived(REGLAS_BASE.filter((r) => r.si === evento));

  onMount(async () => {
    sim = await vistaPrevia({ redonda: pantalla.redonda, alCuadro: (rgba, w, h) => canvas && pintar(canvas, rgba, w, h) });
    aplicar();
    let ultimo = 0;
    const vuelta = (t: number) => {
      if (t - ultimo >= 33) { ultimo = t; sim?.tick(); }
      cuadro = requestAnimationFrame(vuelta);
    };
    cuadro = requestAnimationFrame(vuelta);
    caras = await miniaturasDeCaras({ ...foto(b.config), version: 2 }, pantalla);
  });
  onDestroy(() => cancelAnimationFrame(cuadro));

  function borrador(): Config {
    const c = clonar(foto(b.config)) as Config;
    if (reglas.length) c.reglas = $state.snapshot(reglas).map((r) => (r.y ? r : { si: r.si, entonces: r.entonces }));
    else delete c.reglas;
    c.version = 2;
    return c;
  }
  function aplicar() {
    if (!sim) return;
    const e = sim.aplicar(JSON.stringify(borrador()));
    error = e ?? '';
  }
  function simular() {
    if (!sim) return;
    aplicar();
    const i = (sim.eventos() as string[]).indexOf(evento);
    if (i >= 0) sim.evento(i);
  }

  function nueva(desde?: Regla) {
    if (reglas.length >= 24) return;
    reglas = [...reglas, desde ? clonar(desde) : { si: evento, entonces: ['reflejo:contento'] }];
  }
  function quitar(i: number) {
    reglas = reglas.filter((_, j) => j !== i);
    eligiendo = null;
  }
  function sumarAccion(i: number, tipo: string, arg: string) {
    const r = reglas[i];
    if (r.entonces.length >= 6) return;
    reglas[i] = { ...r, entonces: [...r.entonces, unir(tipo, arg)] };
    eligiendo = null;
  }
  function quitarAccion(i: number, k: number) {
    const r = reglas[i];
    if (r.entonces.length <= 1) return quitar(i);
    reglas[i] = { ...r, entonces: r.entonces.filter((_, j) => j !== k) };
  }

  async function guardar() {
    guardando = true;
    error = '';
    const e = await b.guardar(borrador());
    guardando = false;
    if (e) error = e;
    else { aviso = 'Guardado'; setTimeout(() => (aviso = ''), 2500); }
  }
</script>

{#snippet ficha(a: string)}
  {@const [tipo, arg] = partir(a)}
  {#if tipo === 'reflejo' || tipo === 'expresion'}
    <span class="mini" class:redonda={pantalla.redonda}>{#if caras[arg]}<img src={caras[arg]} alt={arg} />{/if}</span>
    <span class="t">{arg}{tipo === 'expresion' ? ' · 3 s' : ''}</span>
  {:else if tipo === 'comportamiento'}
    <span class="icono">▶</span><span class="t">{arg.replaceAll('-', ' ')}</span>
  {:else if tipo === 'particula'}
    <span class="icono glifo">{GLIFO[arg] ?? arg}</span><span class="t">{arg}</span>
  {:else if tipo === 'estado'}
    {@const e = partirEstado(arg)}
    <span class="icono">{e.s === '+' ? '▲' : '▼'}</span><span class="t">{NOMBRES_ESTADO[e.v] ?? e.v}</span>
  {:else if tipo === 'led'}
    <span class="icono"><span class="lampara" style="--c: {arg}"></span></span><span class="t">Luz</span>
  {:else}
    <span class="icono">♪</span><span class="t">{arg}</span>
  {/if}
{/snippet}

<div class="reglas">
  <aside class="eventos" aria-label="Cuando…">
    <span class="grabado">Cuando…</span>
    <div class="grilla-ev">
      {#each EVENTOS as e}
        <button class="ev" class:sel={evento === e} aria-pressed={evento === e} onclick={() => { evento = e; eligiendo = null; }} title={NOMBRES_EVENTO[e]}>
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={iconoDe(e)} /></svg>
          <span>{corto(e)}</span>
          {#if cuantas(e)}<span class="cuenta">{cuantas(e)}</span>{/if}
        </button>
      {/each}
    </div>
  </aside>

  <section class="mesa">
    <div class="escenario">
      <div class="pantalla" class:redonda={pantalla.redonda}><canvas bind:this={canvas} aria-label="El robot, en vivo"></canvas></div>
      <button class="tecla naranja simular" onclick={simular}>
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={iconoDe(evento)} /></svg>
        {corto(evento)}
      </button>
    </div>

    <div class="barra">
      <h2 class="titulo">{corto(evento)}</h2>
      <button class="tecla negra" disabled={reglas.length >= 24} onclick={() => nueva()}>+ Respuesta</button>
      <button class="tecla naranja" onclick={guardar} disabled={!sucio || guardando}>{guardando ? 'Guardando…' : aviso || 'Guardar'}</button>
    </div>
    {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}

    <div class="placa-naranja tablero">
      {#each propias as { r, i } (i)}
        <div class="regla bandeja">
          <div class="fichas">
            {#each r.entonces as a, k}
              <button class="ficha" title="Quitar" onclick={() => quitarAccion(i, k)}>{@render ficha(a)}<span class="x" aria-hidden="true">✕</span></button>
            {/each}
            {#if r.entonces.length < 6}
              <div class="sumar">
                {#each ACCIONES as ac}
                  <button class="tecla chica" aria-pressed={eligiendo?.regla === i && eligiendo.tipo === ac.tipo}
                    onclick={() => (eligiendo = eligiendo?.regla === i && eligiendo.tipo === ac.tipo ? null : { regla: i, tipo: ac.tipo })}>+ {ac.nombre}</button>
                {/each}
              </div>
            {/if}
          </div>
          {#if eligiendo?.regla === i}
            <div class="selector">
              {#if eligiendo.tipo === 'reflejo' || eligiendo.tipo === 'expresion'}
                {#each Object.keys(caras) as c}
                  <button class="cara" onclick={() => sumarAccion(i, eligiendo!.tipo, c)} title={c}><span class="mini" class:redonda={pantalla.redonda}><img src={caras[c]} alt={c} /></span></button>
                {/each}
              {:else if eligiendo.tipo === 'comportamiento'}
                {#each comportamientos as c}<button class="tecla chica" onclick={() => sumarAccion(i, 'comportamiento', c)}>{c.replaceAll('-', ' ')}</button>{/each}
              {:else if eligiendo.tipo === 'particula'}
                {#each PARTICULAS as p}<button class="tecla chica glifo" title={p} onclick={() => sumarAccion(i, 'particula', p)}>{GLIFO[p] ?? p}</button>{/each}
              {:else if eligiendo.tipo === 'estado'}
                {#each ESTADO as v}
                  <span class="estado"><b>{NOMBRES_ESTADO[v]}</b>
                    <button class="tecla chica" onclick={() => sumarAccion(i, 'estado', `${v}+0.2`)}>▲</button>
                    <button class="tecla negra chica" onclick={() => sumarAccion(i, 'estado', `${v}-0.2`)}>▼</button></span>
                {/each}
              {:else if eligiendo.tipo === 'led'}
                {#each LEDS as c}<button class="led-b" style="--c: {c}" aria-label="Luz {c}" onclick={() => sumarAccion(i, 'led', c.toLowerCase())}></button>{/each}
              {/if}
            </div>
          {/if}
          <div class="pie">
            <label class="visor si"><span class="grabado">Solo si</span>
              <input class="campo claro" value={r.y ?? ''} placeholder="—" spellcheck="false"
                onchange={(e) => { const v = e.currentTarget.value.trim(); reglas[i] = v ? { ...r, y: v } : { si: r.si, entonces: r.entonces }; }} /></label>
            <button class="tecla negra chica" onclick={() => quitar(i)}>Borrar</button>
          </div>
        </div>
      {/each}

      {#each deFabrica as r}
        <div class="regla fabrica">
          <span class="grabado">De fábrica{r.y ? ` · si ${r.y}` : ''}</span>
          <div class="fichas">{#each r.entonces as a}<span class="ficha quieta">{@render ficha(a)}</span>{/each}</div>
          <button class="tecla chica" disabled={reglas.length >= 24} onclick={() => nueva(r)}>Copiar y cambiar</button>
        </div>
      {/each}

      {#if !propias.length && !deFabrica.length}
        <div class="vacio"><span class="grabado">Sin respuesta</span><button class="tecla negra" onclick={() => nueva()}>+ Respuesta</button></div>
      {/if}
    </div>
  </section>
</div>

<style>
  .reglas { display: grid; grid-template-columns: 236px minmax(0, 1fr); gap: 22px; align-items: start; }
  .eventos { display: flex; flex-direction: column; gap: 10px; position: sticky; top: 16px; max-height: calc(100vh - 140px); overflow-y: auto; padding: 2px; scrollbar-width: thin; }
  .grilla-ev { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
  .ev { position: relative; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; padding: 10px; border-radius: 12px; cursor: pointer; text-align: left;
    background: var(--chapa); color: var(--gris-claro); font: 700 12px/1.15 var(--marca); }
  .ev svg { color: var(--gris); }
  .ev.sel { color: var(--c-papel); box-shadow: 0 0 0 2px var(--brasa); }
  .ev.sel svg { color: var(--brasa); }
  .cuenta { position: absolute; right: 8px; top: 8px; min-width: 18px; height: 18px; padding: 0 5px; border-radius: 9px; background: var(--led); color: #1a1714;
    font: 900 10px/18px var(--marca); text-align: center; }
  .mesa { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
  .escenario { position: relative; }
  .pantalla { padding: 14px; border-radius: 30px; background: linear-gradient(180deg, #2c2925, #1c1a17); box-shadow: inset 0 1px 0 rgba(255,255,255,.1), 0 30px 50px -30px rgba(0,0,0,.9); }
  .pantalla canvas { display: block; width: 100%; aspect-ratio: 320 / 172; border-radius: 18px; background: #050505; }
  .pantalla.redonda { width: min(100%, 420px); margin: 0 auto; border-radius: 50%; }
  .pantalla.redonda canvas { aspect-ratio: 1; border-radius: 50%; }
  .simular { position: absolute; right: 26px; bottom: 26px; }
  .barra { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
  .titulo { flex: 1; min-width: 0; font: 900 34px/1 var(--marca); font-stretch: 120%; letter-spacing: -.03em; text-transform: uppercase; }
  .tablero { display: flex; flex-direction: column; gap: 14px; padding: 18px; }
  .regla { display: flex; flex-direction: column; gap: 14px; padding: 16px; }
  .fichas { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
  .ficha { position: relative; display: flex; flex-direction: column; align-items: center; gap: 6px; width: 96px; padding: 8px; border-radius: 14px; cursor: pointer;
    background: #0e0d0b; color: var(--c-papel); box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .ficha .mini, .ficha .icono { width: 100%; height: 44px; }
  .ficha .t { font: 700 11px/1.1 var(--marca); text-align: center; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; color: #d9d2c6; }
  .ficha .x { position: absolute; right: 6px; top: 6px; font-size: 10px; color: var(--gris); opacity: 0; transition: opacity .15s; }
  .ficha:hover .x { opacity: 1; }
  .ficha.quieta { cursor: default; opacity: .8; }
  .icono { display: grid; place-items: center; border-radius: 8px; background: #050505; color: var(--brasa); font: 900 18px/1 var(--marca); }
  .glifo { font-size: 18px; letter-spacing: 0; }
  .lampara { width: 18px; height: 18px; border-radius: 50%; background: var(--c); box-shadow: 0 0 12px var(--c); }
  .mini { display: block; aspect-ratio: 320 / 172; border-radius: 8px; overflow: hidden; background: #050505; }
  .mini.redonda { aspect-ratio: 1; border-radius: 50%; }
  .mini img { width: 100%; height: 100%; object-fit: contain; display: block; }
  .sumar { display: flex; flex-wrap: wrap; gap: 8px; padding-bottom: 6px; }
  .selector { display: flex; flex-wrap: wrap; gap: 8px; padding: 12px; border-radius: 14px; background: rgba(26,23,20,.15); max-height: 220px; overflow-y: auto; }
  .selector .cara { width: 80px; padding: 2px; border-radius: 10px; cursor: pointer; }
  .selector .cara:hover { box-shadow: 0 0 0 2px #1a1714; }
  .estado { display: inline-flex; align-items: center; gap: 6px; padding: 6px 8px 10px; border-radius: 12px; background: rgba(26,23,20,.12); }
  .estado b { font: 800 12px/1 var(--marca); color: #1a1714; }
  .led-b { width: 40px; height: 40px; border-radius: 50%; cursor: pointer; background: #0e0d0b; box-shadow: inset 0 0 0 10px #0e0d0b, inset 0 0 0 20px var(--c), 0 0 12px var(--c); }
  .pie { display: flex; gap: 10px; align-items: center; }
  .si { flex: 1; flex-direction: row; align-items: center; gap: 12px; padding: 8px 14px; }
  .si .campo { font-family: ui-monospace, monospace; font-size: 14px; }
  .fabrica { display: flex; flex-direction: column; gap: 10px; align-items: flex-start; padding: 14px 16px 18px; border-radius: 22px; border: 2px dashed rgba(26,23,20,.3); }
  .vacio { display: flex; justify-content: space-between; align-items: center; padding: 8px 4px 12px; }
  @media (max-width: 1100px) {
    .reglas { grid-template-columns: 1fr; }
    .eventos { position: static; max-height: 280px; }
    .grilla-ev { grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); }
  }
</style>
