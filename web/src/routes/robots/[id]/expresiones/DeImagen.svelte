<script lang="ts">
  // Una cara de imagen: el ojo, el ojo cerrado y la boca son dibujos (SVG, PNG,
  // JPEG o GIF) que el robot anima vivos. Cada parte se sube al toque (la API la
  // recorta y la achica a 128 × 128 con su transparencia); la pantalla de al
  // lado es el núcleo entero, con la cara ya puesta de cara de siempre.
  import { usarBicho } from '$lib/bicho.svelte';
  import { api, ErrorApi } from '$lib/api';
  import { foto } from '$lib/foto.svelte';
  import { clonar, esImagen, MAX_SPRITES, nombreValido, spritesEnUso, type Config, type ExpresionImagen, type ParteSprite } from '$lib/formato';
  import { pantallaDe } from '$lib/tipos';
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import Perilla from '$lib/consola/Perilla.svelte';

  let { inicial = '', alListo = (_: string) => {} }: { inicial?: string; alListo?: (nombre: string) => void } = $props();
  const b = usarBicho();

  const PARTES: [ParteSprite, string, string][] = [
    ['ojo', 'Ojo', 'M3 12c3-5 15-5 18 0-3 5-15 5-18 0Z M12 9a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z'],
    ['ojo_cerrado', 'Cerrado', 'M3 11c4 5 14 5 18 0 M6 14.5l-1.5 2 M12 16v2.5 M18 14.5l1.5 2'],
    ['boca', 'Boca', 'M6 11c3 4 9 4 12 0'],
  ];
  const PERILLAS: [keyof ExpresionImagen & string, string, number, number, number][] = [
    ['tamano', 'Tamaño', 0.08, 0.8, 0.36], ['sep', 'Separación', 0.15, 0.45, 0.3], ['ey', 'Altura', -0.25, 0.25, 0], ['boca_y', 'Boca', -0.7, 0.3, -0.36],
  ];

  const original = b.config.expresiones?.[inicial];
  let doc = $state<ExpresionImagen>(esImagen(original) ? clonar(foto(original)) : { sprites: {}, tamano: 0.4 });
  let nombre = $state(inicial || libreNombre());
  let siempre = $state(!!inicial && b.config.cara?.expresion === inicial);
  let pegando = $state<ParteSprite | null>(null);
  let svg = $state('');
  let subiendo = $state<ParteSprite | null>(null);
  let error = $state('');
  let guardando = $state(false);
  let entrada = $state<HTMLInputElement>();
  let paraSubir: ParteSprite = 'ojo';

  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });
  // La vista previa: la config del robot con esta cara como la de siempre.
  const vista = $derived.by(() => {
    const c = clonar(foto(b.config)) as Config;
    c.version = 2;
    if (doc.sprites.ojo === undefined) return c;
    const n = nombreValido(nombre) ? nombre : 'vista-previa';
    c.expresiones = { ...c.expresiones, [n]: foto(doc) };
    c.cara = { ...c.cara, expresion: n };
    return c;
  });
  const cambio = $derived(JSON.stringify(foto(doc)) !== JSON.stringify(original ?? null) || nombre !== inicial || siempre !== (!!inicial && b.config.cara?.expresion === inicial));

  function libreNombre() {
    let n = 'mi-imagen', k = 2;
    while (b.config.expresiones?.[n]) n = `mi-imagen-${k++}`;
    return n;
  }

  // Un sprite libre: ni de otra cara de imagen ni de otra parte de esta.
  function libre(): number {
    const usados = spritesEnUso(b.config, inicial);
    for (const x of Object.values(doc.sprites)) if (typeof x === 'number') usados.add(x);
    for (let i = 0; i < MAX_SPRITES; i++) if (!usados.has(i)) return i;
    throw new Error('No hay lugar: borrá otra cara de imagen.');
  }

  async function subir(p: ParteSprite, datos: Blob) {
    error = '';
    subiendo = p;
    try {
      const n = doc.sprites[p] ?? libre();
      await api(`/v1/bichos/${b.id}/sprites/${n}`, { metodo: 'PUT', cuerpo: datos, crudo: true });
      doc.sprites = { ...doc.sprites, [p]: n };
      await b.cargarSprites();
    } catch (e) {
      error = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : (e as Error).message;
    }
    subiendo = null;
  }
  function elegirArchivo(p: ParteSprite) {
    paraSubir = p;
    entrada?.click();
  }
  async function alElegir(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0];
    (e.target as HTMLInputElement).value = '';
    if (f) await subir(paraSubir, f);
  }
  async function pegar() {
    if (!pegando || !svg.trim()) return;
    const p = pegando;
    pegando = null;
    await subir(p, new Blob([svg.trim()], { type: 'image/svg+xml' }));
    svg = '';
  }
  function quitar(p: ParteSprite) {
    const s = { ...doc.sprites };
    delete s[p];
    doc.sprites = s;
  }
  const valor = (k: string, d: number) => (typeof doc[k] === 'number' ? (doc[k] as number) : d);
  function poner(k: string, v: number, d: number) {
    const n = { ...doc } as ExpresionImagen;
    if (Math.abs(v - d) < 0.0005 && k !== 'tamano') delete n[k];
    else n[k] = v;
    doc = n;
  }

  // Los sprites que dejó de usar (y no usa otra cara), se borran.
  async function limpiar(c: Config) {
    const enUso = spritesEnUso(c);
    const antes = esImagen(original) ? Object.values(original.sprites) : [];
    for (const n of antes) if (typeof n === 'number' && !enUso.has(n)) await api(`/v1/bichos/${b.id}/sprites/${n}`, { metodo: 'DELETE' }).catch(() => {});
    await b.cargarSprites();
  }

  async function guardar() {
    error = '';
    if (!nombreValido(nombre)) { error = 'El nombre va en minúsculas, sin espacios.'; return; }
    if (doc.sprites.ojo === undefined) { error = 'Falta el ojo.'; return; }
    const c = clonar(foto(b.config)) as Config;
    c.version = 2;
    const ex = { ...c.expresiones };
    if (inicial && inicial !== nombre) delete ex[inicial];
    ex[nombre] = foto(doc);
    c.expresiones = ex;
    if (siempre) c.cara = { ...c.cara, expresion: nombre };
    else if (c.cara && (c.cara.expresion === inicial || c.cara.expresion === nombre)) delete c.cara.expresion;
    guardando = true;
    const e = await b.guardar(c);
    guardando = false;
    if (e) { error = e; return; }
    await limpiar(c);
    alListo(nombre);
  }

  async function borrar() {
    const c = clonar(foto(b.config)) as Config;
    if (c.expresiones) delete c.expresiones[inicial];
    if (c.cara?.expresion === inicial) delete c.cara.expresion;
    const e = await b.guardar(c);
    if (e) { error = e; return; }
    await limpiar(c);
    alListo('');
  }
</script>

<section class="mesa">
  <div class="pantalla" class:redonda={pantalla.redonda}>
    <Pantalla config={vista} redonda={pantalla.redonda} desnuda etiqueta="La cara de imagen, viva" />
  </div>

  <div class="barra">
    <label class="visor nombre"><span class="grabado">Nombre</span><input class="campo" bind:value={nombre} maxlength="24" /></label>
    <button class="tecla negra" aria-pressed={siempre} onclick={() => (siempre = !siempre)} title="Su cara de siempre">{siempre ? '★ Siempre' : '☆ Siempre'}</button>
    {#if inicial}<button class="tecla negra" onclick={borrar}>Borrar</button>{/if}
    <button class="tecla naranja" onclick={guardar} disabled={guardando || doc.sprites.ojo === undefined || !cambio}>{guardando ? 'Guardando…' : 'Guardar'}</button>
  </div>
  {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}

  <div class="placa-naranja tablero">
    <div class="partes">
      {#each PARTES as [p, t, icono]}
        {@const n = doc.sprites[p]}
        <div class="bandeja parte" class:falta={n === undefined}>
          <div class="muestra">
            {#if n !== undefined && b.sprites[n]}
              <img src="/api/v1/bichos/{b.id}/sprites/{n}?png=1&v={b.sprites[n].crc}" alt={t} />
            {:else}
              <svg viewBox="0 0 24 24" width="34" height="34" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d={icono} /></svg>
            {/if}
          </div>
          <span class="grabado">{t}</span>
          <div class="teclas">
            <button class="tecla chica" onclick={() => elegirArchivo(p)} disabled={!!subiendo} title="Subir SVG, PNG, JPEG o GIF">{subiendo === p ? '…' : '↑'}</button>
            <button class="tecla chica" aria-pressed={pegando === p} onclick={() => (pegando = pegando === p ? null : p)} disabled={!!subiendo} title="Pegar un SVG">&lt;/&gt;</button>
            {#if n !== undefined && p !== 'ojo'}<button class="tecla negra chica" onclick={() => quitar(p)} title="Sacar">✕</button>{/if}
          </div>
        </div>
      {/each}
    </div>
    {#if pegando}
      <div class="bandeja pegar">
        <textarea class="campo" bind:value={svg} rows="5" placeholder="<svg viewBox=&quot;0 0 100 100&quot;>…</svg>" aria-label="SVG"></textarea>
        <button class="tecla naranja chica" onclick={pegar} disabled={!svg.trim().startsWith('<')}>Listo</button>
      </div>
    {/if}
    <div class="bandeja perillas">
      {#each PERILLAS as [k, t, min, max, d]}
        <Perilla nombre={t} valor={valor(k, d)} {min} {max} paso={0.01} tamano={58} alCambiar={(v) => poner(k, v, d)} />
      {/each}
    </div>
  </div>
  <input type="file" accept=".svg,image/svg+xml,image/png,image/jpeg,image/gif" bind:this={entrada} onchange={alElegir} hidden />
</section>

<style>
  .mesa { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
  .pantalla { padding: 14px; border-radius: 30px; background: linear-gradient(180deg, #2c2925, #1c1a17); box-shadow: inset 0 1px 0 rgba(255,255,255,.1), 0 30px 50px -30px rgba(0,0,0,.9);
    aspect-ratio: 320 / 172; }
  .pantalla :global(canvas) { border-radius: 18px; }
  .pantalla.redonda { width: min(100%, 420px); align-self: center; border-radius: 50%; aspect-ratio: 1; }
  .barra { display: flex; gap: 12px; align-items: stretch; flex-wrap: wrap; }
  .nombre { flex: 1; min-width: 200px; padding: 10px 16px; gap: 6px; }
  .nombre .campo { font-size: 22px; }
  .barra .tecla { align-self: center; }
  .tablero { display: flex; flex-direction: column; gap: 16px; padding: 18px; }
  .partes { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
  .parte { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 16px 10px; }
  .muestra { display: grid; place-items: center; width: 100%; aspect-ratio: 1.4; border-radius: 12px; background: #050505; color: #3a3632;
    box-shadow: inset 0 2px 8px rgba(0,0,0,.9); overflow: hidden; }
  .muestra img { max-width: 80%; max-height: 80%; image-rendering: auto; }
  .parte.falta .muestra { outline: 2px dashed #3a3632; outline-offset: -8px; }
  .teclas { display: flex; gap: 8px; }
  .pegar { display: flex; flex-direction: column; gap: 10px; padding: 14px; }
  .pegar textarea { width: 100%; font: 12px/1.4 ui-monospace, monospace; resize: vertical; }
  .pegar .tecla { align-self: flex-end; }
  .perillas { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: 22px 10px; padding: 20px; }
  @media (max-width: 560px) { .partes { grid-template-columns: 1fr; } }
</style>
