<script lang="ts">
  import { foto } from '$lib/foto.svelte';
  // El editor de animaciones: pistas de formas con claves en el tiempo.
  import { onMount } from 'svelte';
  import { nucleoEditor, pintar } from '$lib/nucleo';
  import { clonar, CURVAS, cuadro, NOMBRES_CURVA, nombreValido, TIPOS_FORMA, type Animacion, type Clave, type Config, type Forma } from '$lib/formato';
  import Manijas from './Manijas.svelte';
  import CamposForma from './CamposForma.svelte';

  let {
    animacion = $bindable(),
    nombre = $bindable(),
    contexto = { version: 1 },
    caras = [] as string[],
    nombreFijo = false,
    error = $bindable(''),
  }: { animacion: Animacion; nombre: string; contexto?: Config; caras?: string[]; nombreFijo?: boolean; error?: string } = $props();

  let canvas: HTMLCanvasElement;
  let t = $state(0);
  let reproduciendo = $state(true);
  let pista = $state(0);
  let clave = $state(-1); // índice dentro de las claves de la pista elegida
  let redonda = $state(false);
  let editarForma = $state(false);

  async function dibujar() {
    const n = await nucleoEditor();
    n.editor.iniciar(redonda ? 480 : 640, redonda ? 480 : 344, redonda);
    const clave = nombreValido(nombre) ? nombre : 'edicion';
    const r = n.editor.config({ ...foto(contexto), version: 1, animaciones: { [clave]: foto(animacion) } });
    error = r.ok ? '' : r.detalle;
    if (!r.ok) return;
    const c = n.editor.animacion(n.editor.indiceAnimacion(clave), Math.round(t));
    pintar(canvas, c.rgba, c.ancho, c.alto);
  }
  $effect(() => {
    void JSON.stringify(animacion);
    void [t, redonda, nombre];
    dibujar();
  });

  onMount(() => {
    let antes = 0, id = 0;
    const vuelta = (ahora: number) => {
      if (reproduciendo && antes) {
        t += ahora - antes;
        if (t > animacion.duracion) t = t % Math.max(1, animacion.duracion);
      }
      antes = ahora;
      id = requestAnimationFrame(vuelta);
    };
    id = requestAnimationFrame(vuelta);
    return () => cancelAnimationFrame(id);
  });

  const p = $derived(animacion.pistas[pista]);
  const ordenadas = $derived(p ? p.claves.map((c, i) => ({ c, i })).sort((a, b) => a.c.t - b.c.t) : []);

  function cambiarPista(f: (p: Animacion['pistas'][number]) => Animacion['pistas'][number]) {
    const pistas = [...animacion.pistas];
    pistas[pista] = f(clonar(foto(pistas[pista])));
    animacion = { ...animacion, pistas };
  }
  function cambiarClave(cambio: Partial<Clave>) {
    cambiarPista((q) => {
      const c = { ...q.claves[clave], ...cambio };
      for (const k of Object.keys(c) as (keyof Clave)[]) if (c[k] === undefined || Number.isNaN(c[k])) delete c[k];
      c.t = Math.max(0, Math.min(animacion.duracion, Math.round(c.t ?? 0)));
      q.claves[clave] = c;
      return q;
    });
  }
  function agregarClave() {
    if (!p || p.claves.length >= 8) return;
    const ahora = cuadro(p, t);
    const r = (v: number) => Math.round(v * 1000) / 1000;
    cambiarPista((q) => {
      q.claves.push({ t: Math.round(t), x: r(ahora.x), y: r(ahora.y), escala: r(ahora.escala), rotacion: r(ahora.rotacion), opacidad: r(ahora.opacidad) });
      q.claves.sort((a, b) => a.t - b.t);
      return q;
    });
    clave = animacion.pistas[pista].claves.findIndex((c) => c.t === Math.round(t));
  }
  function quitarClave(i: number) {
    if (!p || p.claves.length <= 1) return;
    cambiarPista((q) => ({ ...q, claves: q.claves.filter((_, k) => k !== i) }));
    clave = -1;
  }
  function agregarPista(tipo: Forma['forma']) {
    if (animacion.pistas.length >= 6) return;
    const forma = TIPOS_FORMA.find((x) => x.tipo === tipo)!.nueva();
    animacion = { ...animacion, pistas: [...animacion.pistas, { forma, claves: [{ t: 0 }, { t: animacion.duracion, escala: 1.5 }] }] };
    pista = animacion.pistas.length - 1;
    clave = -1;
  }
  function quitarPista(i: number) {
    if (animacion.pistas.length <= 1) return;
    animacion = { ...animacion, pistas: animacion.pistas.filter((_, k) => k !== i) };
    pista = Math.max(0, pista - 1);
    clave = -1;
  }

  // Arrastrar en la línea de tiempo.
  let pistaDeTiempo: HTMLDivElement | null = $state(null);
  let arrastrando: { i: number } | null = null;
  function tiempoDe(e: PointerEvent) {
    const r = pistaDeTiempo!.getBoundingClientRect();
    return Math.round(Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)) * animacion.duracion);
  }
  function arrastrarClave(e: PointerEvent, i: number) {
    e.stopPropagation();
    clave = i;
    arrastrando = { i };
    (e.currentTarget as Element).setPointerCapture(e.pointerId);
    reproduciendo = false;
  }
  function moverClave(e: PointerEvent) {
    if (!arrastrando) return;
    const nt = tiempoDe(e);
    t = nt;
    cambiarClave({ t: nt });
  }

  // Manijas: con una clave elegida, mover cambia su x/y; si no, mueve la forma.
  const ahora = $derived(p ? cuadro(p, t) : { x: 0, y: 0 });
  function moverForma(_: number, f: Forma) {
    if (clave >= 0) {
      const base = p.forma;
      const [bx, by] = [base.x ?? base.puntos?.[0][0] ?? 0, base.y ?? base.puntos?.[0][1] ?? 0];
      const [nx, ny] = [f.x ?? f.puntos?.[0][0] ?? 0, f.y ?? f.puntos?.[0][1] ?? 0];
      const c = p.claves[clave];
      cambiarClave({ x: Math.round(((c.x ?? 0) + nx - bx) * 1000) / 1000, y: Math.round(((c.y ?? 0) + ny - by) * 1000) / 1000 });
    } else {
      cambiarPista((q) => ({ ...q, forma: f }));
    }
  }
  let tipoPista = $state<Forma['forma']>('corazon');
</script>

<div class="editor">
  <div class="escenario grilla">
    <div class="placa" class:redonda>
      <div class="lienzo" style="aspect-ratio: {redonda ? '1 / 1' : '320 / 172'}">
        <canvas bind:this={canvas} width="640" height="344" aria-label="Vista previa de la animación"></canvas>
        {#if p && !redonda && !reproduciendo}
          <Manijas formas={[p.forma]} seleccionada={0} alCambiar={moverForma} alElegir={() => {}} desplazamiento={[ahora.x, ahora.y]} />
        {/if}
      </div>
    </div>
    <div class="fila">
      <button class="chico" onclick={() => (reproduciendo = !reproduciendo)}>{reproduciendo ? 'Pausa' : 'Reproducir'}</button>
      <span class="mono">{Math.round(t)} / {animacion.duracion} ms</span>
      <button class="chico" aria-pressed={redonda} onclick={() => (redonda = !redonda)}>Broche</button>
    </div>

    <div class="tiempo tarjeta">
      <input type="range" min="0" max={animacion.duracion} step="1" value={Math.round(t)} oninput={(e) => { reproduciendo = false; t = +e.currentTarget.value; }} aria-label="Instante" />
      {#each animacion.pistas as q, i}
        <div class="carril" class:elegido={i === pista}>
          <button class="nombre-carril" onclick={() => { pista = i; clave = -1; }}>{i + 1}. {TIPOS_FORMA.find((x) => x.tipo === q.forma.forma)?.nombre}</button>
          <div class="barra" bind:this={pistaDeTiempo} onpointermove={moverClave} onpointerup={() => (arrastrando = null)} role="presentation">
            <div class="cabezal" style="left: {(t / animacion.duracion) * 100}%"></div>
            {#each q.claves as c, k}
              <button class="clave" class:elegida={i === pista && k === clave} style="left: {(c.t / animacion.duracion) * 100}%"
                aria-label="Clave en {c.t} ms" onpointerdown={(e) => { pista = i; arrastrarClave(e, k); }} onpointerup={() => (arrastrando = null)}></button>
            {/each}
          </div>
        </div>
      {/each}
      <p class="nota">Arrastrá los rombos para moverlos en el tiempo. En pausa, arrastrá la forma en la pantalla: con una clave elegida movés la clave; si no, la forma.</p>
    </div>
    {#if error}<p class="aviso-caja error" role="alert">El robot no la aceptaría: {error}</p>{/if}
  </div>

  <div class="panel grilla">
    <div class="campos">
      <label><span class="etiqueta">Nombre</span><input bind:value={nombre} disabled={nombreFijo} maxlength="24" aria-invalid={!nombreValido(nombre)} /></label>
      <label><span class="etiqueta">Duración (ms)</span>
        <input type="number" min="100" max="60000" step="50" value={animacion.duracion}
          oninput={(e) => { const d = Math.max(100, Math.min(60000, Math.round(+e.currentTarget.value || 0))); animacion = { ...animacion, duracion: d, pistas: animacion.pistas.map((q) => ({ ...q, claves: q.claves.map((c) => ({ ...c, t: Math.min(c.t, d) })) })) }; }} />
      </label>
      <label><span class="etiqueta">Sobre la cara</span>
        <select value={animacion.sobre ?? ''} onchange={(e) => { const { sobre: _, ...resto } = animacion; animacion = e.currentTarget.value ? { ...resto, sobre: e.currentTarget.value } : resto; }}>
          <option value="">ninguna (pantalla negra)</option>
          {#each caras as c}<option value={c}>{c}</option>{/each}
        </select>
      </label>
      <label class="fila" style="align-self: end"><input type="checkbox" checked={!!animacion.repetir} onchange={(e) => (animacion = { ...animacion, repetir: e.currentTarget.checked || undefined })} /> <span class="etiqueta">repetir</span></label>
    </div>

    <div>
      <p class="etiqueta">Pistas ({animacion.pistas.length}/6)</p>
      <div class="fila">
        <select bind:value={tipoPista} style="flex: 1">{#each TIPOS_FORMA.filter((x) => x.tipo !== 'franjas') as x}<option value={x.tipo}>{x.nombre}</option>{/each}</select>
        <button disabled={animacion.pistas.length >= 6} onclick={() => agregarPista(tipoPista)}>Agregar pista</button>
      </div>
    </div>

    {#if p}
      <div class="tarjeta medida grilla">
        <div class="fila" style="justify-content: space-between">
          <p class="etiqueta">Pista {pista + 1}</p>
          <span class="fila">
            <button class="chico" aria-pressed={editarForma} onclick={() => (editarForma = !editarForma)}>Forma</button>
            <button class="chico peligro" disabled={animacion.pistas.length <= 1} onclick={() => quitarPista(pista)}>Quitar</button>
          </span>
        </div>
        {#if editarForma}
          <CamposForma forma={p.forma} sinFijo alCambiar={(f) => cambiarPista((q) => ({ ...q, forma: f }))} />
          <label class="fila"><input type="checkbox" checked={!!p.centro} onchange={(e) => cambiarPista((q) => { const { centro: _, ...r } = q; return e.currentTarget.checked ? { ...r, centro: [0, 0] } : r; })} />
            <span class="etiqueta">centro propio para girar y escalar</span></label>
          {#if p.centro}
            <div class="campos">
              <label><span class="etiqueta">centro x</span><input type="number" step="0.01" value={p.centro[0]} oninput={(e) => cambiarPista((q) => ({ ...q, centro: [+e.currentTarget.value, q.centro?.[1] ?? 0] }))} /></label>
              <label><span class="etiqueta">centro y</span><input type="number" step="0.01" value={p.centro[1]} oninput={(e) => cambiarPista((q) => ({ ...q, centro: [q.centro?.[0] ?? 0, +e.currentTarget.value] }))} /></label>
            </div>
          {/if}
        {/if}
        <div class="fila">
          <span class="etiqueta">Claves ({p.claves.length}/8)</span>
          <button class="chico" disabled={p.claves.length >= 8} onclick={agregarClave}>Clave en {Math.round(t)} ms</button>
        </div>
        <div class="fila claves">
          {#each ordenadas as { c, i }}
            <button class="chico" aria-pressed={i === clave} onclick={() => { clave = i; t = c.t; reproduciendo = false; }}>{c.t} ms</button>
          {/each}
        </div>
        {#if clave >= 0 && p.claves[clave]}
          {@const c = p.claves[clave]}
          <div class="campos">
            <label><span class="etiqueta">t (ms)</span><input type="number" min="0" max={animacion.duracion} value={c.t} oninput={(e) => cambiarClave({ t: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">x</span><input type="number" step="0.01" min="-4" max="4" value={c.x ?? 0} oninput={(e) => cambiarClave({ x: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">y</span><input type="number" step="0.01" min="-4" max="4" value={c.y ?? 0} oninput={(e) => cambiarClave({ y: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">escala</span><input type="number" step="0.05" min="0" max="8" value={c.escala ?? 1} oninput={(e) => cambiarClave({ escala: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">giro (°)</span><input type="number" step="5" min="-3600" max="3600" value={c.rotacion ?? 0} oninput={(e) => cambiarClave({ rotacion: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">opacidad · {(c.opacidad ?? 1).toFixed(2)}</span><input type="range" step="0.05" min="0" max="1" value={c.opacidad ?? 1} oninput={(e) => cambiarClave({ opacidad: +e.currentTarget.value })} /></label>
            <label><span class="etiqueta">curva hasta la siguiente</span>
              <select value={c.curva ?? 'suave'} onchange={(e) => cambiarClave({ curva: e.currentTarget.value === 'suave' ? undefined : e.currentTarget.value })}>
                {#each CURVAS as k}<option value={k}>{NOMBRES_CURVA[k]}</option>{/each}
              </select>
            </label>
            <label><span class="etiqueta">color</span>
              <div class="fila">
                <input type="color" value={c.color?.startsWith('#') ? c.color : '#FF6A33'} oninput={(e) => cambiarClave({ color: e.currentTarget.value.toUpperCase() })} />
                {#if c.color}<button class="chico" onclick={() => cambiarClave({ color: undefined })}>Quitar</button>{/if}
              </div>
            </label>
          </div>
          <button class="chico peligro" style="justify-self: start" disabled={p.claves.length <= 1} onclick={() => quitarClave(clave)}>Quitar la clave</button>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .editor { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(300px, 1fr); gap: 24px; align-items: start; }
  .escenario { position: sticky; top: 84px; }
  .placa { width: 100%; }
  .placa.redonda { max-width: 420px; justify-self: center; }
  .lienzo { position: relative; width: 100%; }
  .lienzo canvas { width: 100%; height: 100%; }
  .tiempo { box-shadow: none; padding: 14px; display: grid; gap: 8px; }
  .carril { display: grid; grid-template-columns: 130px 1fr; gap: 10px; align-items: center; }
  .nombre-carril { border: 0; background: none; text-transform: none; letter-spacing: 0; font: 14px var(--serif); padding: 4px; justify-content: start; overflow: hidden; text-overflow: ellipsis; }
  .carril.elegido .nombre-carril { color: var(--naranja-tinta); }
  .barra { position: relative; height: 28px; border-radius: 8px; background: var(--papel-2); touch-action: none; }
  .cabezal { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--naranja); transform: translateX(-1px); pointer-events: none; }
  .clave { position: absolute; top: 50%; width: 14px; height: 14px; padding: 0; border-radius: 3px; border: 1px solid var(--tinta-suave); background: var(--tarjeta); transform: translate(-50%, -50%) rotate(45deg); cursor: ew-resize; }
  .clave.elegida { background: var(--naranja); border-color: var(--naranja); }
  .medida { box-shadow: none; padding: 14px; }
  .claves { gap: 4px; }
  @media (max-width: 900px) { .editor { grid-template-columns: 1fr; } .escenario { position: static; } }
</style>
