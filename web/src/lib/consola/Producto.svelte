<script lang="ts">
  // La foto real del robot con su pantalla viva encima: el mismo núcleo que
  // corre en el robot, encastrado donde cae la pantalla en la foto.
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import type { Modelo } from './modelo';

  let {
    modelo,
    config,
    visible = '',
    apagado = false,
    etiqueta = '',
  }: { modelo: Modelo; config: unknown; visible?: string; apagado?: boolean; etiqueta?: string } = $props();

  const e = $derived(modelo.encuadre);
</script>

<div class="ventana" class:apagado style="background: {e.fondo}">
  <img src={e.src} alt={e.alt} style="width: {e.foto.ancho}%; left: {e.foto.izq}%; top: {e.foto.arriba}%" />
  <div class="pantalla" class:redonda={modelo.redonda}
    style="left: {e.pantalla.izq}%; top: {e.pantalla.arriba}%; width: {e.pantalla.ancho}%; height: {e.pantalla.alto}%">
    <div class="encastre" class:apaisada={!modelo.redonda && e.pantalla.alto * 5 > e.pantalla.ancho * 4}>
      <Pantalla {config} {visible} redonda={modelo.redonda} desnuda etiqueta="Pantalla de {etiqueta || 'tu robot'}, en vivo (tocala)" />
    </div>
  </div>
  {#if etiqueta}<span class="cartel"><span class="led"></span>{etiqueta}</span>{/if}
</div>

<style>
  .ventana { position: relative; width: 100%; aspect-ratio: 4 / 5; border-radius: var(--radio-ventana, 36px); overflow: hidden;
    box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 40px 60px -30px rgba(0,0,0,.9); isolation: isolate; }
  img { position: absolute; max-width: none; height: auto; user-select: none; pointer-events: none; }
  .pantalla { position: absolute; border-radius: 10px; overflow: hidden; background: #000; display: flex; align-items: center; }
  .pantalla.redonda { border-radius: 50%; }
  .encastre { width: 100%; height: 100%; }
  /* Una pantalla apaisada en una caja de pie: se agranda para que la cara
     llene el ancho (los costados, que suelen estar vacíos, quedan afuera). */
  .encastre.apaisada { position: absolute; left: 50%; top: 50%; width: 185%; height: auto; aspect-ratio: 320 / 172; transform: translate(-50%, -50%); }
  .apagado img { filter: grayscale(.7) brightness(.62); }
  .apagado .pantalla { opacity: .25; }
  .cartel { position: absolute; left: 16px; right: 16px; bottom: 16px; display: flex; align-items: center; gap: 10px; min-height: 40px;
    padding: 0 14px; border-radius: 12px; background: rgba(14,13,11,.85); font: 700 14px/1.2 var(--marca); color: #f3eee8; }
  .cartel .led { width: 9px; height: 9px; border-radius: 50%; background: #ff6a33; box-shadow: 0 0 10px #ff6a33; flex: none; }
</style>
