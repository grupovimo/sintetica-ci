<script lang="ts">
  // Una foto del producto en la landing. Mientras no esté la foto real (en
  // static/landing/), se ve un lugar marcado con lo que tiene que mostrar.
  let {
    src = '',
    alt,
    proporcion = '4 / 5',
    clara = false,
    lineas = false,
    video = false,
    radio = '40px',
    encuadre = 'center',
    prioridad = false,
  }: { src?: string; alt: string; proporcion?: string; clara?: boolean; lineas?: boolean; video?: boolean; radio?: string; encuadre?: string; prioridad?: boolean } = $props();
</script>

<div class="foto" class:clara style:aspect-ratio={proporcion} style:border-radius={radio}>
  {#if src}
    <img {src} {alt} loading={prioridad ? 'eager' : 'lazy'} fetchpriority={prioridad ? 'high' : 'auto'} decoding="async" style:object-position={encuadre} />
  {:else}
    <div class="marca" role="img" aria-label={'Lugar para ' + (video ? 'un video' : 'una foto') + ': ' + alt}>
      <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        {#if video}<rect x="3" y="5" width="18" height="14" rx="3"></rect><path d="M10 9.5v5l4.5-2.5z"></path>{:else}<path d="M4 7h3l2-3h6l2 3h3v13H4z"></path><circle cx="12" cy="13" r="4"></circle>{/if}
      </svg>
    </div>
    <span class="ficha">{video ? 'Video' : 'Foto'} · {alt}</span>
  {/if}
  {#if lineas}<div class="lineas" aria-hidden="true"></div>{/if}
</div>

<style>
  .foto { position: relative; overflow: hidden; display: flex; align-items: flex-end; background: radial-gradient(110% 90% at 30% 20%, #2b2b2b, #0c0c0c 70%); color: #555; }
  .foto.clara { background: radial-gradient(110% 90% at 30% 20%, #dedad2, #b7b2a8 70%); color: #8d887f; }
  img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
  .marca { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; }
  .ficha { position: relative; margin: 24px; display: inline-flex; align-items: center; min-height: 40px; padding: 0 18px; border-radius: 999px; font: 700 15px/1 'Archivo', sans-serif; font-stretch: 110%; color: #111; background: radial-gradient(130% 150% at 30% 0%, #fff 0%, #f4f2ee 60%, #dcd8d0 100%); box-shadow: inset 0 2px 1px #fff, inset 0 -6px 10px rgba(0, 0, 0, 0.12), 0 8px 16px -10px rgba(0, 0, 0, 0.5); }
  .lineas { position: absolute; inset: 0; pointer-events: none; background: repeating-linear-gradient(0deg, rgba(255, 255, 255, 0.05) 0 1px, transparent 1px 4px); }
</style>
