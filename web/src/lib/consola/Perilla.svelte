<script lang="ts">
  // Una perilla: se gira arrastrando para arriba o abajo, con la rueda o con las
  // flechas. Muestra el valor en un visor. El valor va de `min` a `max`.
  let {
    valor,
    nombre,
    min = 0,
    max = 1,
    paso = 0.05,
    mostrar = (v: number) => String(Math.round(((v - min) / (max - min)) * 100)),
    tamano = 64,
    alCambiar,
  }: {
    valor: number;
    nombre: string;
    min?: number;
    max?: number;
    paso?: number;
    mostrar?: (v: number) => string;
    tamano?: number;
    alCambiar: (v: number) => void;
  } = $props();

  const fraccion = $derived((valor - min) / (max - min));
  let desde: { y: number; v: number } | null = null;

  const ajustar = (v: number) => {
    const r = Math.round(Math.min(max, Math.max(min, v)) / paso) * paso;
    alCambiar(+r.toFixed(4));
  };
  function bajar(e: PointerEvent) {
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    desde = { y: e.clientY, v: valor };
  }
  function mover(e: PointerEvent) {
    if (!desde) return;
    ajustar(desde.v + ((desde.y - e.clientY) / 160) * (max - min));
  }
  function tecla(e: KeyboardEvent) {
    const d = { ArrowUp: 1, ArrowRight: 1, ArrowDown: -1, ArrowLeft: -1, PageUp: 4, PageDown: -4 }[e.key];
    if (d) { e.preventDefault(); ajustar(valor + d * paso); }
    if (e.key === 'Home') ajustar(min);
    if (e.key === 'End') ajustar(max);
  }
</script>

<div class="perilla">
  <div class="cuerpo" role="slider" tabindex="0" aria-label={nombre} aria-valuemin={min} aria-valuemax={max} aria-valuenow={valor} aria-valuetext={mostrar(valor)}
    style="width: {tamano}px; height: {tamano}px"
    onpointerdown={bajar} onpointermove={mover} onpointerup={() => (desde = null)} onpointercancel={() => (desde = null)} onkeydown={tecla}
    onwheel={(e) => { e.preventDefault(); ajustar(valor + (e.deltaY < 0 ? paso : -paso)); }}>
    <svg class="escala" viewBox="0 0 100 100" aria-hidden="true">
      {#each Array.from({ length: 11 }, (_, i) => i) as i}
        {@const a = ((-135 + i * 27) * Math.PI) / 180}
        <line x1={50 + Math.sin(a) * 44} y1={50 - Math.cos(a) * 44} x2={50 + Math.sin(a) * 49} y2={50 - Math.cos(a) * 49}
          class:prendida={i / 10 <= fraccion + 0.001} />
      {/each}
    </svg>
    <div class="tapa" style="transform: rotate({-135 + fraccion * 270}deg)"><span class="marca"></span></div>
  </div>
  <span class="grabado">{nombre}</span>
  <span class="valor">{mostrar(valor)}</span>
</div>

<style>
  .perilla { display: flex; flex-direction: column; align-items: center; gap: 8px; min-width: 0; }
  .cuerpo { position: relative; border-radius: 50%; cursor: ns-resize; touch-action: none; }
  .escala { position: absolute; inset: -9px; width: calc(100% + 18px); height: calc(100% + 18px); overflow: visible; }
  .escala line { stroke: rgba(26,23,20,.35); stroke-width: 3; stroke-linecap: round; }
  .escala line.prendida { stroke: #1a1714; }
  .tapa { position: absolute; inset: 0; border-radius: 50%; background: radial-gradient(circle at 35% 30%, #3e3a35, #161412 72%);
    box-shadow: 0 5px 0 #000, 0 10px 14px -6px rgba(80,20,0,.7), inset 0 1px 0 rgba(255,255,255,.15); transition: transform .12s; }
  .marca { position: absolute; left: 50%; top: 7%; width: 5px; height: 26%; margin-left: -2.5px; border-radius: 3px; background: #ff8a5c; box-shadow: 0 0 8px #ff6a33; }
  .valor { min-width: 44px; text-align: center; padding: 5px 8px; border-radius: 7px; background: #0e0d0b; color: #ff8a5c;
    font: 900 15px/1 var(--marca); font-variant-numeric: tabular-nums; box-shadow: inset 0 2px 5px rgba(0,0,0,.8); }
  .grabado { text-align: center; }
</style>
