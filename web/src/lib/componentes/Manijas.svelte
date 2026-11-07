<script lang="ts">
  // Lo que se arrastra sobre la pantalla en los editores: cada forma se puede
  // elegir y mover; la elegida muestra manijas para el tamaño o sus puntos.
  // Coordenadas normalizadas de la pantalla de referencia (x de −1 a 1).
  import { cajaDe, mover, MEDIO_ALTO, type Forma, type Punto } from '$lib/formato';

  let {
    formas,
    seleccionada = -1,
    alCambiar,
    alElegir,
    desplazamiento = [0, 0],
  }: {
    formas: Forma[];
    seleccionada?: number;
    alCambiar: (i: number, f: Forma) => void;
    alElegir: (i: number) => void;
    desplazamiento?: [number, number]; // para mostrar las manijas donde está la forma en un cuadro de animación
  } = $props();

  let svg: SVGSVGElement;
  type Arrastre = { tipo: 'mover' | 'tamano' | 'punto' | 'radio'; i: number; punto?: number; desde: Punto; original: Forma };
  let arrastre: Arrastre | null = null;

  const r = (v: number) => Math.round(v * 1000) / 1000;

  function aNormal(e: PointerEvent): Punto {
    const c = svg.getBoundingClientRect();
    return [((e.clientX - c.left) / c.width) * 2 - 1, ((e.clientY - c.top) / c.height) * 2 * MEDIO_ALTO - MEDIO_ALTO];
  }

  function empezar(e: PointerEvent, tipo: Arrastre['tipo'], i: number, punto?: number) {
    e.stopPropagation();
    e.preventDefault();
    alElegir(i);
    arrastre = { tipo, i, punto, desde: aNormal(e), original: JSON.parse(JSON.stringify(formas[i])) as Forma };
    svg.setPointerCapture(e.pointerId);
  }

  function moverPuntero(e: PointerEvent) {
    if (!arrastre) return;
    const [x, y] = aNormal(e);
    const dx = x - arrastre.desde[0], dy = y - arrastre.desde[1];
    const o = arrastre.original;
    let f: Forma;
    if (arrastre.tipo === 'mover') {
      f = mover(o, dx, dy);
    } else if (arrastre.tipo === 'punto' && o.puntos && arrastre.punto !== undefined) {
      f = JSON.parse(JSON.stringify(o));
      f.puntos![arrastre.punto] = [r(o.puntos[arrastre.punto][0] + dx), r(o.puntos[arrastre.punto][1] + dy)];
    } else if (arrastre.tipo === 'radio') {
      f = { ...o, r: r(Math.max(0.005, (o.r ?? 0.1) + dx)) };
    } else {
      // Tamaño desde la esquina de abajo a la derecha, con el centro fijo.
      f = { ...o, w: r(Math.max(0.01, (o.w ?? 0.1) + dx * 2)), h: r(Math.max(0.01, (o.h ?? 0.1) + dy * 2)) };
    }
    alCambiar(arrastre.i, f);
  }

  function soltar(e: PointerEvent) {
    if (arrastre) svg.releasePointerCapture(e.pointerId);
    arrastre = null;
  }
</script>

<svg bind:this={svg} viewBox="-1 {-MEDIO_ALTO} 2 {2 * MEDIO_ALTO}" preserveAspectRatio="none"
  onpointermove={moverPuntero} onpointerup={soltar} onpointercancel={soltar}
  onpointerdown={() => alElegir(-1)} role="application" aria-label="Formas: arrastrá para mover">
  <g transform="translate({desplazamiento[0]} {desplazamiento[1]})">
    {#each formas as f, i}
      {@const [x0, y0, x1, y1] = cajaDe(f)}
      {@const pad = 0.012}
      <rect class="caja" class:elegida={i === seleccionada} x={x0 - pad} y={y0 - pad} width={x1 - x0 + 2 * pad} height={Math.max(y1 - y0, 0.01) + 2 * pad}
        rx="0.01" role="button" tabindex="-1" aria-label="Forma {i + 1}: {f.forma}" onpointerdown={(e) => empezar(e, 'mover', i)} />
      {#if i === seleccionada}
        {#if f.forma === 'pildora' || f.forma === 'corte'}
          <circle class="manija" cx={x1} cy={y1} r="0.022" role="button" tabindex="-1" aria-label="Tamaño" onpointerdown={(e) => empezar(e, 'tamano', i)} />
        {:else if f.forma === 'circulo'}
          <circle class="manija" cx={x1} cy={f.y ?? 0} r="0.022" role="button" tabindex="-1" aria-label="Radio" onpointerdown={(e) => empezar(e, 'radio', i)} />
        {/if}
        {#each f.puntos ?? [] as p, k}
          <circle class="manija punto" cx={p[0]} cy={p[1]} r="0.02" role="button" tabindex="-1" aria-label="Punto {k + 1}" onpointerdown={(e) => empezar(e, 'punto', i, k)} />
        {/each}
      {/if}
    {/each}
  </g>
</svg>

<style>
  svg { position: absolute; inset: 0; width: 100%; height: 100%; touch-action: none; }
  .caja { fill: transparent; stroke: rgba(245, 242, 236, 0.18); stroke-width: 0.004; stroke-dasharray: 0.012 0.01; cursor: move; vector-effect: none; }
  .caja:hover { stroke: rgba(245, 242, 236, 0.5); }
  .caja.elegida { stroke: #ff6a33; stroke-dasharray: none; stroke-width: 0.005; }
  .manija { fill: #f5f2ec; stroke: #ff6a33; stroke-width: 0.006; cursor: nwse-resize; }
  .manija.punto { cursor: grab; fill: #ff6a33; stroke: #0b0a09; }
</style>
