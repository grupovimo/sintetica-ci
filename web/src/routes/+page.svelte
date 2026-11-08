<script lang="ts">
  // La landing comercial: un viaje de producto con el bicho en 3D. La cara que
  // se ve en su pantalla y en la tira la dibuja el mismo núcleo del bicho.
  import { onDestroy, onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { vistaPrevia, fijar, pintar, type VistaPrevia } from '$lib/nucleo';
  import { Car, comando } from '$compartido/protocolo.js';
  import { COLORES, montar } from '$lib/landing/escena';
  import { NOMBRES, fotos } from '$lib/landing/caras';

  let { data } = $props();

  let raiz: HTMLElement;
  let lienzo: HTMLCanvasElement;
  let pantalla: HTMLCanvasElement | null = null;
  let sim: VistaPrevia | null = null;
  let desmontar: (() => void) | null = null;
  let vuelta = 0;

  let colorElegido = $state(-1);
  let colorVisible = $state(0);
  let caras = $state<{ nombre: string; url: string }[]>([]);
  const mitad = (par: boolean) => caras.filter((_, i) => i % 2 === (par ? 0 : 1));

  // La reserva
  let nombre = $state('');
  let contacto = $state('');
  let mensaje = $state('');
  let sitio = $state(''); // trampa para bots: la persona no lo ve
  let enviando = $state(false);
  let reservado = $state(false);
  let errorReserva = $state('');
  const colorReserva = $derived(COLORES[colorElegido >= 0 ? colorElegido : colorVisible][0]);

  async function reservar(e: Event) {
    e.preventDefault();
    enviando = true;
    errorReserva = '';
    try {
      await api('/v1/reservas', { cuerpo: { nombre, contacto, color: colorReserva, mensaje, sitio } });
      reservado = true;
    } catch (err) {
      errorReserva = err instanceof ErrorApi ? err.message : 'No pude mandarla. Probá de nuevo.';
    } finally {
      enviando = false;
    }
  }

  onMount(() => {
    desmontar = montar({
      raiz,
      canvas: lienzo,
      get pantalla() {
        return pantalla;
      },
      alCambiarCara: (c) => sim && fijar(sim, c),
      alCaricia: () => sim?.bleEscribir(Car.comando, comando.evento('caricia')),
      alColor: (i) => (colorVisible = i),
      colorElegido: () => colorElegido,
    });
    // El núcleo de verdad para la pantalla del bicho 3D.
    const c = document.createElement('canvas');
    vistaPrevia({ alCuadro: (rgba, w, h) => pintar(c, rgba, w, h) })
      .then((s) => {
        sim = s;
        pantalla = c;
        fijar(s, 'neutral');
        let ultimo = 0;
        const paso = (t: number) => {
          if (t - ultimo >= 33) {
            ultimo = t;
            sim?.tick();
          }
          vuelta = requestAnimationFrame(paso);
        };
        vuelta = requestAnimationFrame(paso);
      })
      .catch(() => {});
    fotos()
      .then((f) => (caras = f))
      .catch(() => {});
  });

  onDestroy(() => {
    cancelAnimationFrame(vuelta);
    desmontar?.();
    sim = null;
  });
</script>

<svelte:head>
  <title>Sintética Labs</title>
  <meta name="description" content="Una cara de bolsillo. Un bicho con cara, hecho a mano en Mar del Plata." />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght,SOFT,WONK@9..144,300..900,0..100,0..1&family=JetBrains+Mono:wght@400;700&display=swap" />
</svelte:head>

<div class="landing" bind:this={raiz}>
  <canvas class="escenario" bind:this={lienzo} aria-label="El bicho de Sintética Labs en 3D. Tocalo para que reaccione." role="img"></canvas>
  <div class="respaldo" aria-hidden="true">
    <svg viewBox="0 0 600 470"><rect x="250" y="372" width="100" height="44" rx="8" fill="#141210" /><rect x="190" y="410" width="220" height="40" rx="20" fill="#141210" /><rect x="70" y="40" width="460" height="340" rx="72" fill="#F5501A" /><rect x="104" y="74" width="392" height="272" rx="42" fill="#141210" /><rect x="226" y="148" width="40" height="124" rx="20" fill="#FF6A33" /><path d="M338 166L378 146V252A20 20 0 0 1 338 252Z" fill="#FF6A33" /></svg>
  </div>
  <div class="etiquetas-3d" aria-hidden="true"></div>

  <header class="nav">
    <a class="marca" href="#s0" aria-label="Sintética Labs, inicio">
      <svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="16" fill="#F5501A" /><path fill="#141210" d="M23 20A5 5 0 0 1 28 25V39A5 5 0 0 1 18 39V25A5 5 0 0 1 23 20ZM36 26L46 21V39A5 5 0 0 1 36 39Z" /></svg>
      <b>sintética</b>
    </a>
    <div class="acciones">
      <a class="puff" href={data.usuario ? '/bichos' : '/entrar'}>{data.usuario ? 'Mis bichos' : 'Entrar'}</a>
      <a class="pill" href="#reservar">Reservar</a>
    </div>
  </header>
  <nav class="riel" aria-hidden="true"></nav>
  <span class="contador mono" aria-hidden="true">01 / 09</span>

  <main>
    <section class="escena hero" id="s0">
      <div class="pin"><div class="txt" style="--in:1">
        <h1 class="big"><span class="ln"><span>Una cara</span></span><span class="ln"><span>de bolsillo.</span></span></h1>
        <div class="lado">
          <div class="puffs"><span class="puff"><i class="d"></i>Lote 01 abierto</span><span class="puff">Hecho a mano · MDP</span></div>
          <span class="mono cue"><i></i>Scrolleá</span>
        </div>
      </div></div>
    </section>

    <section class="escena r">
      <div class="pin"><div class="txt">
        <span class="mono tag">Tacto</span>
        <h2 class="big"><span class="ln"><span>Acaricia-</span></span><span class="ln"><span>me.</span></span></h2>
        <p class="sub">Pasale el dedo por arriba y pone esa cara.</p>
        <div class="puffs"><span class="puff">Tacto capacitivo</span><span class="puff">Sin botones</span></div>
      </div></div>
    </section>

    <section class="escena">
      <div class="pin"><div class="txt">
        <span class="mono tag">Equilibrio</span>
        <h2 class="big"><span class="ln"><span>No me</span></span><span class="ln"><span>sacudas.</span></span></h2>
        <p class="sub">En serio. Se marea.</p>
        <div class="puffs"><span class="puff">Sacudir</span><span class="puff">Levantar</span><span class="puff">Dar vuelta</span></div>
      </div></div>
    </section>

    <section class="escena r">
      <div class="pin"><div class="txt">
        <span class="mono tag">Voz</span>
        <h2 class="big"><span class="ln"><span>Hablame.</span></span></h2>
        <p class="sub">Te escucha, lo piensa y te contesta.</p>
      </div></div>
    </section>

    <section class="escena">
      <div class="pin"><div class="txt">
        <span class="mono tag">Hecho a mano, pieza por pieza</span>
        <h2 class="big"><span class="ln"><span>Por</span></span><span class="ln"><span>dentro.</span></span></h2>
        <p class="sub">Cada capa soldada, impresa y armada a mano en Mar del Plata.</p>
      </div></div>
    </section>

    <section class="escena r">
      <div class="pin"><div class="txt">
        <span class="mono tag">App propia</span>
        <h2 class="big"><span class="ln"><span>Escanea-</span></span><span class="ln"><span>me.</span></span></h2>
        <p class="sub">Se presenta solo. Un QR en su cara y es tuyo. Personalizalo desde la app como se te cante.</p>
      </div></div>
    </section>

    <section class="escena">
      <div class="pin"><div class="txt">
        <span class="mono tag">Actualizaciones por WiFi</span>
        <h2 class="big"><span class="ln"><span>Se actualiza</span></span><span class="ln"><span>solo.</span></span></h2>
        <p class="sub">Caras nuevas, gestos nuevos, trucos nuevos. Le llegan mientras duerme.</p>
        <div class="puffs"><span class="puff"><i class="d"></i>FW 0.2.0 disponible</span></div>
      </div></div>
    </section>

    <section class="escena r">
      <div class="pin"><div class="txt">
        <span class="mono tag">Elegí su carcasa</span>
        <h2 class="big"><span class="ln"><span>Tu color.</span></span></h2>
        <div class="fila-colores">
          <div class="muestras">
            {#each COLORES as [, etiqueta, hex], i}
              <button type="button" class="muestra" style:background={hex} aria-label={'Carcasa ' + etiqueta} aria-pressed={colorVisible === i} onclick={() => (colorElegido = i)}></button>
            {/each}
          </div>
          <span class="mono">{COLORES[colorVisible][1]}</span>
        </div>
      </div></div>
    </section>

    <section class="escena">
      <div class="pin"><div class="txt">
        <span class="mono tag">Serie grabada</span>
        <h2 class="big"><span class="ln"><span>Uno</span></span><span class="ln"><span>de uno.</span></span></h2>
        <p class="sub">Cada unidad lleva su número grabado en la espalda.</p>
      </div></div>
    </section>

    <section class="solida caras" aria-label={`Las ${NOMBRES.length} caras`}>
      <div class="cabeza">
        <h2 class="big" style="font-size:clamp(56px,9vw,150px)">{NOMBRES.length} caras.</h2>
        <span class="mono tag">Cada una con un motivo</span>
      </div>
      <div class="tira tira-1" aria-hidden="true">
        {#each [0, 1, 2] as _}{#each mitad(true) as c}<img src={c.url} alt="" width="320" height="172" />{/each}{/each}
      </div>
      <div class="tira tira-2" aria-hidden="true">
        {#each [0, 1, 2] as _}{#each mitad(false) as c}<img src={c.url} alt="" width="320" height="172" />{/each}{/each}
      </div>
    </section>

    <section class="solida nums" aria-label="En números">
      <div class="num"><b>{NOMBRES.length}+</b><span class="mono">Caras de fábrica, y contando</span></div>
      <div class="num"><b>∞</b><span class="mono">Actualizaciones, sin cables</span></div>
      <div class="num"><b>#001</b><span class="mono">Serie única grabada</span></div>
      <div class="num"><b>MDP</b><span class="mono">Hecho a mano en Mar del Plata</span></div>
    </section>

    <section class="solida final" id="reservar">
      <span class="mono tag">Lote 01 · por pedido · hecho a mano</span>
      <div class="final-fila">
        <h2 class="big">Reservá<br />el tuyo.</h2>
        {#if reservado}
          <div class="reserva listo">
            <p class="mono">¡Listo!</p>
            <p>Te escribimos a <b>{contacto}</b> para coordinar tu bicho {COLORES.find((c) => c[0] === colorReserva)?.[1].toLowerCase()}.</p>
          </div>
        {:else}
          <form class="reserva" onsubmit={reservar}>
            <label><span class="mono">Tu nombre</span><input bind:value={nombre} required maxlength="80" autocomplete="name" /></label>
            <label><span class="mono">Email o WhatsApp</span><input bind:value={contacto} required minlength="5" maxlength="120" autocomplete="email" /></label>
            <div class="mono">Carcasa</div>
            <div class="muestras">
              {#each COLORES as [, etiqueta, hex], i}
                <button type="button" class="muestra" style:background={hex} aria-label={'Carcasa ' + etiqueta} aria-pressed={colorReserva === COLORES[i][0]} onclick={() => (colorElegido = i)}></button>
              {/each}
            </div>
            <label><span class="mono">Algo más (opcional)</span><textarea bind:value={mensaje} maxlength="500" rows="2"></textarea></label>
            <label class="trampa" aria-hidden="true">Sitio<input bind:value={sitio} tabindex="-1" autocomplete="off" /></label>
            {#if errorReserva}<p class="error">{errorReserva}</p>{/if}
            <div class="botones">
              <button class="pill dark" disabled={enviando}>{enviando ? 'Mandando…' : 'Reservar'}</button>
              <a class="puff" href="https://instagram.com/sintetica.labs" target="_blank" rel="noopener">@sintetica.labs ↗</a>
            </div>
          </form>
        {/if}
      </div>
    </section>
    <footer class="solida pie"><span class="mono">Sintética Labs · Mar del Plata</span><span class="mono">2026</span></footer>
  </main>
</div>

<style>
  .landing {
    --papel: #fbfaf7;
    --hueso: #ede9e1;
    --tinta: #141210;
    --suave: #6b6660;
    --naranja: #f5501a;
    --display: 'Fraunces', Georgia, serif;
    --mono: 'JetBrains Mono', ui-monospace, Menlo, monospace;
    --g: clamp(18px, 4.5vw, 64px);
    --fg: #141210;
    --puff-hi: inset 0 2px 1px rgba(255, 255, 255, 0.55), inset 0 -8px 14px rgba(0, 0, 0, 0.16), inset 0 8px 14px rgba(255, 255, 255, 0.18);
    --puff-drop: 0 14px 28px -12px rgba(20, 18, 16, 0.45), 0 3px 0 rgba(20, 18, 16, 0.12);
    position: relative;
    min-height: 100vh;
    background: var(--papel);
    color: var(--fg);
    font-family: var(--display);
    font-variation-settings: 'SOFT' 100, 'WONK' 0;
    overflow-x: clip;
    -webkit-font-smoothing: antialiased;
    color-scheme: light;
  }
  .landing::after { content: ''; position: fixed; inset: -50%; z-index: 50; pointer-events: none; opacity: 0.11; mix-blend-mode: multiply; background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='220' height='220'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='.85' numOctaves='3' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1.1 0'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>"); animation: grano 1.2s steps(6) infinite; }
  @keyframes grano { 0% { transform: translate(0, 0); } 20% { transform: translate(-3%, 2%); } 40% { transform: translate(2%, -3%); } 60% { transform: translate(-2%, -1%); } 80% { transform: translate(3%, 3%); } 100% { transform: translate(0, 0); } }
  .landing :global(*) { box-sizing: border-box; }
  .landing a { color: inherit; text-decoration: none; }
  .landing button { font: inherit; color: inherit; }
  .landing :focus-visible { outline: 3px solid var(--naranja); outline-offset: 3px; border-radius: 8px; }

  .escenario { position: fixed; inset: 0; width: 100%; height: 100%; z-index: 0; display: block; }
  .respaldo { position: fixed; inset: 0; display: none; place-items: center; z-index: 0; pointer-events: none; }
  .respaldo svg { width: min(520px, 80vw); }
  :global(.sin-gl) .respaldo { display: grid; }
  .etiquetas-3d { position: fixed; inset: 0; z-index: 1; pointer-events: none; }
  .etiquetas-3d :global(.lbl) { position: absolute; left: 0; top: 0; display: flex; align-items: center; gap: 10px; color: var(--papel); opacity: 0; will-change: transform, opacity; white-space: nowrap; }
  .etiquetas-3d :global(.lbl::before) { content: ''; width: 9px; height: 9px; border-radius: 50%; background: var(--naranja); box-shadow: 0 0 0 4px rgba(245, 80, 26, 0.25); flex: none; }
  .etiquetas-3d :global(.lbl span) { font-family: var(--mono); font-size: 12px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; padding: 6px 10px; border-radius: 999px; background: rgba(20, 18, 16, 0.72); border: 1px solid rgba(251, 250, 247, 0.18); backdrop-filter: blur(8px); }
  .etiquetas-3d :global(.lbl em) { font-style: normal; opacity: 0.6; margin-left: 8px; font-weight: 400; }

  .mono { font-family: var(--mono); font-variation-settings: normal; font-size: 12px; font-weight: 700; letter-spacing: 0.14em; text-transform: uppercase; }

  .nav { position: fixed; top: 0; left: 0; right: 0; z-index: 10; display: flex; justify-content: space-between; align-items: center; padding: 18px var(--g); pointer-events: none; color: var(--fg); }
  .nav > * { pointer-events: auto; }
  .acciones { display: flex; gap: 10px; align-items: center; }
  .marca { display: flex; align-items: center; gap: 10px; }
  .marca svg { width: 32px; height: 32px; }
  .marca b { font-weight: 800; font-size: 24px; letter-spacing: -0.03em; font-variation-settings: 'SOFT' 100, 'opsz' 144; }
  .pill { display: inline-flex; align-items: center; gap: 10px; height: 52px; padding: 0 26px; border-radius: 999px; background: var(--naranja); color: var(--tinta); font-family: var(--mono); font-size: 12px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; border: 0; cursor: pointer; box-shadow: var(--puff-hi), var(--puff-drop), 0 0 0 1px rgba(184, 60, 12, 0.35); transition: transform 0.3s cubic-bezier(0.3, 1.7, 0.45, 1); }
  .pill:hover { transform: translateY(-2px) scale(1.04); }
  .pill:active { transform: translateY(2px) scale(0.97); }
  .pill:disabled { opacity: 0.6; cursor: progress; }
  .pill.dark { background: var(--tinta); color: var(--papel); box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.18), inset 0 -8px 14px rgba(0, 0, 0, 0.4), var(--puff-drop); }
  .puff { display: inline-flex; align-items: center; gap: 8px; height: 40px; padding: 0 18px; border-radius: 999px; background: var(--papel); color: var(--tinta); font-family: var(--mono); font-size: 11px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; box-shadow: var(--puff-hi), 0 8px 18px -10px rgba(20, 18, 16, 0.5), 0 0 0 1px rgba(20, 18, 16, 0.08); }
  .puff .d { width: 8px; height: 8px; border-radius: 50%; background: var(--naranja); box-shadow: inset 0 1px 1px rgba(255, 255, 255, 0.6), 0 0 0 3px rgba(245, 80, 26, 0.18); }
  .puffs { display: flex; flex-wrap: wrap; gap: 10px; }
  .r .puffs { justify-content: flex-end; }

  .riel { position: fixed; right: var(--g); top: 50%; transform: translateY(-50%); z-index: 10; display: grid; gap: 10px; color: var(--fg); }
  .riel :global(i) { width: 6px; height: 6px; border-radius: 6px; background: currentColor; opacity: 0.25; transition: height 0.35s, opacity 0.35s, background 0.35s; display: block; }
  .riel :global(i.on) { height: 28px; opacity: 1; background: var(--naranja); }
  .contador { position: fixed; left: var(--g); bottom: 22px; z-index: 10; color: var(--fg); opacity: 0.6; font-variant-numeric: tabular-nums; }

  main { position: relative; z-index: 2; padding: 0; }
  .escena { height: 180vh; position: relative; }
  .escena:first-child { height: 160vh; }
  .pin { position: sticky; top: 0; height: 100vh; height: 100svh; display: grid; padding: 110px var(--g) 64px; pointer-events: none; }
  .pin :global(a), .pin :global(button) { pointer-events: auto; }
  .txt { --in: 0; align-self: end; display: grid; gap: 18px; max-width: 44vw; color: var(--fg); opacity: var(--in); transform: translateY(calc((1 - var(--in)) * 40px)); }
  .r .txt { justify-self: end; text-align: right; justify-items: end; }
  .big { margin: 0; font-weight: 800; font-size: clamp(64px, 11vw, 190px); line-height: 0.86; letter-spacing: -0.05em; font-variation-settings: 'SOFT' 100, 'WONK' 0, 'opsz' 144; }
  .ln { display: block; overflow: hidden; padding-bottom: 0.06em; margin-bottom: -0.06em; }
  .ln > span { display: block; transform: translateY(calc((1 - var(--in)) * 105%)); }
  .ln:nth-child(2) > span { transform: translateY(calc((1 - var(--in)) * 160%)); }
  .sub { margin: 0; font-size: clamp(17px, 1.5vw, 21px); line-height: 1.45; max-width: 30ch; opacity: 0.72; }
  .tag { display: inline-flex; gap: 10px; align-items: center; }
  .tag::before { content: ''; width: 24px; height: 2px; background: var(--naranja); }
  .r .tag { flex-direction: row-reverse; }

  .hero .txt { max-width: none; width: 100%; grid-template-columns: 1fr auto; align-items: end; }
  .hero .big { font-size: clamp(56px, 9vw, 150px); }
  .lado { display: grid; gap: 16px; justify-items: end; text-align: right; }
  .cue { display: inline-flex; align-items: center; gap: 10px; }
  .cue i { width: 2px; height: 28px; background: currentColor; display: block; animation: cue 1.6s ease-in-out infinite; transform-origin: top; }
  @keyframes cue { 0% { transform: scaleY(0); } 50% { transform: scaleY(1); } 100% { transform: scaleY(0); transform-origin: bottom; } }

  .fila-colores { display: flex; gap: 18px; align-items: center; flex-wrap: wrap; }
  .muestras { display: flex; gap: 10px; }
  .muestra { width: 52px; height: 52px; border-radius: 50%; border: 0; cursor: pointer; box-shadow: var(--puff-hi), var(--puff-drop); transition: transform 0.3s cubic-bezier(0.3, 1.7, 0.45, 1); }
  .muestra:hover { transform: translateY(-2px); }
  .muestra[aria-pressed='true'] { transform: scale(1.14); box-shadow: var(--puff-hi), var(--puff-drop), 0 0 0 4px var(--papel), 0 0 0 6px var(--tinta); }

  .solida { position: relative; z-index: 3; }
  .caras { background: var(--tinta); color: var(--papel); padding: clamp(80px, 12vw, 160px) 0; overflow: hidden; }
  .cabeza { padding: 0 var(--g); display: flex; justify-content: space-between; align-items: end; gap: 24px; flex-wrap: wrap; margin-bottom: 48px; }
  .tira { display: flex; width: max-content; gap: 16px; will-change: transform; min-height: 120px; }
  .tira + .tira { margin-top: 16px; }
  .tira img { width: clamp(180px, 22vw, 300px); height: auto; border-radius: 26px; display: block; flex: none; background: #0b0a09; image-rendering: auto; box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.06), 0 18px 30px -18px rgba(0, 0, 0, 0.9); }

  .nums { background: var(--papel); color: var(--tinta); padding: clamp(80px, 12vw, 160px) var(--g); display: grid; grid-template-columns: repeat(4, 1fr); gap: 32px; }
  .num { display: grid; gap: 12px; align-content: start; padding: 28px; border-radius: 32px; background: var(--hueso); box-shadow: var(--puff-hi), 0 18px 34px -22px rgba(20, 18, 16, 0.45); }
  .num b { font-weight: 800; font-size: clamp(56px, 8vw, 128px); line-height: 0.85; letter-spacing: -0.05em; font-variation-settings: 'SOFT' 100, 'opsz' 144; }
  .num .mono { color: var(--suave); }

  .final { background: var(--naranja); color: var(--tinta); min-height: 100vh; min-height: 100svh; padding: clamp(100px, 12vw, 160px) var(--g) 48px; display: grid; align-content: space-between; gap: 48px; clip-path: polygon(0 0, calc(100% - min(280px, 34vw)) 0, 100% min(140px, 17vw), 100% 100%, 0 100%); }
  .final .big { font-size: clamp(72px, 14vw, 240px); }
  .final-fila { display: flex; justify-content: space-between; align-items: end; gap: 32px; flex-wrap: wrap; }
  .reserva { display: grid; gap: 12px; width: min(420px, 100%); padding: 24px; border-radius: 28px; background: rgba(251, 250, 247, 0.92); box-shadow: var(--puff-hi), var(--puff-drop); }
  .reserva label { display: grid; gap: 6px; }
  .reserva input, .reserva textarea { font: 17px var(--display); padding: 12px 14px; border-radius: 14px; border: 1px solid rgba(20, 18, 16, 0.15); background: #fff; color: var(--tinta); }
  .reserva .muestra { width: 40px; height: 40px; }
  .reserva .botones { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .reserva .error { margin: 0; color: #a3260f; }
  .reserva.listo p { margin: 0; font-size: 19px; }
  .trampa { position: absolute; left: -9999px; width: 1px; height: 1px; overflow: hidden; }
  .pie { background: var(--naranja); color: var(--tinta); padding: 0 var(--g) 28px; display: flex; justify-content: space-between; flex-wrap: wrap; gap: 12px; }

  @media (max-width: 760px) {
    .pin { padding-bottom: 44px; }
    .txt, .r .txt { max-width: none; justify-self: start; text-align: left; justify-items: start; }
    .r .tag { flex-direction: row; }
    .r .puffs { justify-content: flex-start; }
    .hero .txt { grid-template-columns: 1fr; }
    .lado { justify-items: start; text-align: left; }
    .riel, .contador { display: none; }
    .nums { grid-template-columns: 1fr 1fr; gap: 48px 24px; }
    .marca b { font-size: 21px; }
    .acciones .puff { display: none; }
    .etiquetas-3d :global(.lbl span) { font-size: 10px; padding: 5px 8px; }
    .etiquetas-3d :global(.lbl em) { display: none; }
  }
  @media (prefers-reduced-motion: reduce) {
    .landing :global(*), .landing::after { animation: none !important; transition: none !important; }
  }
</style>
