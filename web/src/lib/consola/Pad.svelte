<script lang="ts">
  // Un pad 2D: se arrastra un punto (para la mirada: x a los costados, y arriba/abajo).
  let {
    x, y, minX, maxX, minY, maxY, nombre, alCambiar,
  }: { x: number; y: number; minX: number; maxX: number; minY: number; maxY: number; nombre: string; alCambiar: (x: number, y: number) => void } = $props();

  let caja: HTMLDivElement;
  let arrastrando = false;
  const fx = $derived((x - minX) / (maxX - minX));
  const fy = $derived(1 - (y - minY) / (maxY - minY));

  function mover(e: PointerEvent) {
    if (!arrastrando) return;
    const r = caja.getBoundingClientRect();
    const a = Math.min(1, Math.max(0, (e.clientX - r.left) / r.width));
    const b = Math.min(1, Math.max(0, (e.clientY - r.top) / r.height));
    alCambiar(+(minX + a * (maxX - minX)).toFixed(3), +(minY + (1 - b) * (maxY - minY)).toFixed(3));
  }
  function tecla(e: KeyboardEvent) {
    const p = 0.02;
    const d = { ArrowLeft: [-p, 0], ArrowRight: [p, 0], ArrowUp: [0, p], ArrowDown: [0, -p] }[e.key];
    if (!d) return;
    e.preventDefault();
    alCambiar(Math.min(maxX, Math.max(minX, x + d[0])), Math.min(maxY, Math.max(minY, y + d[1])));
  }
</script>

<div class="pad" bind:this={caja} role="slider" tabindex="0" aria-label={nombre} aria-valuenow={x} aria-valuetext="{x.toFixed(2)}, {y.toFixed(2)}"
  onpointerdown={(e) => { (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId); arrastrando = true; mover(e); }}
  onpointermove={mover} onpointerup={() => (arrastrando = false)} onpointercancel={() => (arrastrando = false)} onkeydown={tecla}
  ondblclick={() => alCambiar(0, 0)}>
  <span class="cruz h"></span><span class="cruz v"></span>
  <span class="punto" style="left: {fx * 100}%; top: {fy * 100}%"></span>
</div>

<style>
  .pad { position: relative; width: 100%; aspect-ratio: 1; border-radius: 18px; background: #0e0d0b; cursor: crosshair; touch-action: none;
    box-shadow: inset 0 3px 10px rgba(0,0,0,.8); background-image: radial-gradient(rgba(255,255,255,.07) 1px, transparent 1.4px); background-size: 14px 14px; }
  .cruz { position: absolute; background: rgba(255,255,255,.1); }
  .cruz.h { left: 10%; right: 10%; top: 50%; height: 1px; }
  .cruz.v { top: 10%; bottom: 10%; left: 50%; width: 1px; }
  .punto { position: absolute; width: 26px; height: 26px; margin: -13px 0 0 -13px; border-radius: 50%; background: radial-gradient(circle at 35% 30%, #ffb08a, #f2541a);
    box-shadow: 0 0 16px #ff6a33, 0 3px 0 #9a3209; pointer-events: none; }
</style>
