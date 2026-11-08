<script lang="ts">
  // Cara: el color y el tamaño de sus ojos, su cara de siempre (la de fábrica o
  // una de imagen) y ponerle una cara ya (o soltarla).
  import { usarBicho } from '$lib/bicho.svelte';
  import { esImagen } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const COLORES = [
    { hex: '#FF6A33', nombre: 'Naranja' },
    { hex: '#FFB347', nombre: 'Ámbar' },
    { hex: '#F3EEE8', nombre: 'Blanco' },
    { hex: '#4ADE80', nombre: 'Verde' },
    { hex: '#3DE0FF', nombre: 'Cian' },
    { hex: '#A78BFA', nombre: 'Violeta' },
    { hex: '#FF5FA2', nombre: 'Rosa' },
    { hex: '#FF3B3B', nombre: 'Rojo' },
  ];
  const RAPIDAS: [string, string][] = [
    ['contento', 'Contento'], ['enamorado', 'Enamorado'], ['guino', 'Guiño'], ['sorpresa', 'Sorpresa'],
    ['sospecha', 'Sospecha'], ['pensando', 'Pensando'], ['triste', 'Triste'], ['enojado', 'Enojado'],
    ['mareado', 'Mareado'], ['dormido', 'Dormido'], ['curioso', 'Curioso'], ['glitch', 'Glitch'],
  ];

  // Las caras de imagen (dibujadas) que puede tener de siempre, con la miniatura de su ojo.
  const deImagen = $derived(Object.entries(b.config.expresiones ?? {}).filter(([, e]) => esImagen(e)).map(([n, e]) => {
    const s = esImagen(e) ? e.sprites.ojo : undefined;
    return { n, src: s !== undefined && b.sprites[s] ? `/api/v1/bichos/${b.id}/sprites/${s}?png=1&v=${b.sprites[s].crc}` : '' };
  }));
  const siempre = $derived(b.config.cara?.expresion ?? '');
  const ponerSiempre = (n: string) => g.cambiar((c) => {
    c.cara = { ...(c.cara ?? {}) };
    if (n) c.cara.expresion = n;
    else delete c.cara.expresion;
  });
  const color = $derived((b.config.cara?.color ?? '#FF6A33').toUpperCase());
  const tamano = $derived(b.config.cara?.tamano ?? 1);
  const conectado = $derived(b.bicho?.conectado ?? false);
  const fija = $derived(b.bicho?.estado?.fija ?? false);
  const visible = $derived(b.bicho?.estado?.expresion ?? '');
  let error = $state('');

  const ponerColor = (hex: string) => g.cambiar((c) => {
    c.cara = { ...(c.cara ?? {}), color: hex };
    if (hex === '#FF6A33') delete c.cara.color;
  });
  const ponerTamano = (v: number) => g.cambiar((c) => (c.cara = { ...(c.cara ?? {}), tamano: v }));
  async function fijar(n: string) {
    error = (await b.comando({ expresion: n })) ?? '';
  }
</script>

<div class="cara">
  <div class="fila">
    <div class="bandeja colores">
      <span class="grabado">Color de los ojos</span>
      <div class="leds" role="radiogroup" aria-label="Color de los ojos">
        {#each COLORES as c}
          <button role="radio" aria-checked={color === c.hex} class="color" class:elegido={color === c.hex} style="--c: {c.hex}" onclick={() => ponerColor(c.hex)}>
            <span class="lampara"></span><span class="grabado">{c.nombre}</span>
          </button>
        {/each}
      </div>
    </div>
    <div class="bandeja tamano">
      <Perilla nombre="Tamaño" valor={tamano} min={0.5} max={1.5} paso={0.05} mostrar={(v) => `${Math.round(v * 100)}%`} tamano={76} alCambiar={ponerTamano} />
    </div>
  </div>

  {#if deImagen.length || siempre}
    <div class="bandeja">
      <span class="grabado">Su cara</span>
      <div class="siempre" role="radiogroup" aria-label="Su cara de siempre">
        <button role="radio" aria-checked={!siempre} class="opcion" class:elegido={!siempre} onclick={() => ponerSiempre('')}>
          <span class="ojitos"><i></i><i></i></span><span class="grabado">De fábrica</span>
        </button>
        {#each deImagen as d}
          <button role="radio" aria-checked={siempre === d.n} class="opcion" class:elegido={siempre === d.n} onclick={() => ponerSiempre(d.n)} title={d.n}>
            <span class="ojitos">{#if d.src}<img src={d.src} alt="" /><img class="espejo" src={d.src} alt="" />{/if}</span><span class="grabado">{d.n}</span>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <div class="bandeja">
    <div class="titulo">
      <span class="grabado">Ponele una cara ya</span>
      <button class="tecla negra chica" disabled={!conectado || !fija} onclick={() => fijar('')}>{fija ? 'Soltar (vuelve a ser él)' : 'Se maneja solo'}</button>
    </div>
    {#if !conectado}<p class="texto">Necesita estar en línea.</p>{/if}
    <div class="rapidas">
      {#each RAPIDAS as [n, t]}
        <button class="tecla chica" aria-pressed={fija && visible === n} disabled={!conectado} onclick={() => fijar(n)}>{t}</button>
      {/each}
    </div>
    {#if b.nombresAnimaciones.length}
      <span class="grabado">Tus animaciones</span>
      <div class="rapidas">
        {#each b.nombresAnimaciones as n}
          <button class="tecla negra chica" disabled={!conectado} onclick={async () => (error = (await b.comando({ reproducir: n })) ?? '')}>▶ {n}</button>
        {/each}
      </div>
    {/if}
    {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
  </div>
</div>

<style>
  .cara { display: flex; flex-direction: column; gap: 16px; }
  .fila { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 16px; }
  .colores { display: flex; flex-direction: column; gap: 16px; }
  .leds { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
  .color { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px 4px 10px; border: 0; border-radius: 14px; cursor: pointer;
    background: #0e0d0b; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .color .grabado { color: #8f877c; font-size: 10px; }
  .color.elegido .grabado { color: #f3eee8; }
  .lampara { width: 22px; height: 22px; border-radius: 50%; background: var(--c); opacity: .35; transition: opacity .2s, box-shadow .2s; }
  .color.elegido .lampara { opacity: 1; box-shadow: 0 0 14px var(--c), 0 0 0 3px rgba(255,255,255,.12); }
  .color.elegido { box-shadow: inset 0 2px 6px rgba(0,0,0,.8), 0 0 0 2px var(--c); }
  .tamano { display: grid; place-items: center; padding: 22px 28px; }
  .bandeja { display: flex; flex-direction: column; gap: 14px; }
  .titulo { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
  .rapidas { display: flex; flex-wrap: wrap; gap: 10px 8px; padding-bottom: 6px; }
  .siempre { display: grid; grid-template-columns: repeat(auto-fill, minmax(110px, 1fr)); gap: 10px; }
  .opcion { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px 6px 10px; border: 0; border-radius: 14px; cursor: pointer;
    background: #0e0d0b; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); min-width: 0; }
  .opcion .grabado { color: #8f877c; font-size: 10px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .opcion.elegido { box-shadow: inset 0 2px 6px rgba(0,0,0,.8), 0 0 0 2px var(--brasa); }
  .opcion.elegido .grabado { color: #f3eee8; }
  .ojitos { display: flex; gap: 8px; align-items: center; height: 30px; }
  .ojitos i { width: 9px; height: 22px; border-radius: 5px; background: var(--led); box-shadow: 0 0 8px var(--led); }
  .ojitos img { height: 28px; width: auto; max-width: 40px; object-fit: contain; }
  .ojitos img.espejo { transform: scaleX(-1); }
  @media (max-width: 640px) { .fila { grid-template-columns: 1fr; } }
</style>
