<script lang="ts">
  // El editor de caras: el núcleo real dibuja, encima se arrastran las formas.
  import { onMount } from 'svelte';
  import { nucleoEditor, pintar } from '$lib/nucleo';
  import { clonar, espejar, nombreValido, TIPOS_FORMA, type Config, type Expresion, type Forma } from '$lib/formato';
  import Manijas from './Manijas.svelte';
  import CamposForma from './CamposForma.svelte';

  let {
    expresion = $bindable(),
    nombre = $bindable(),
    contexto = { version: 1 },
    nombreFijo = false,
    error = $bindable(''),
  }: { expresion: Expresion; nombre: string; contexto?: Config; nombreFijo?: boolean; error?: string } = $props();

  let canvas: HTMLCanvasElement;
  let sel = $state(-1);
  let cierre = $state(0);
  let miradaX = $state(0);
  let miradaY = $state(0);
  let redonda = $state(false);
  let animarParpadeo = $state(false);
  let listo = $state(false);

  async function dibujar() {
    const n = await nucleoEditor();
    const w = redonda ? 480 : 640, h = redonda ? 480 : 344;
    n.editor.iniciar(w, h, redonda);
    const clave = nombreValido(nombre) ? nombre : 'edicion';
    const r = n.editor.config({ ...$state.snapshot(contexto), version: 1, expresiones: { [clave]: $state.snapshot(expresion) } });
    error = r.ok ? '' : r.detalle;
    const i = n.editor.indiceExpresion(clave);
    const c = n.editor.expresion(i, cierre, miradaX, miradaY);
    pintar(canvas, c.rgba, c.ancho, c.alto);
    listo = true;
  }

  $effect(() => {
    void JSON.stringify(expresion);
    void [cierre, miradaX, miradaY, redonda, nombre];
    dibujar();
  });

  // Parpadeo de prueba: 160 ms ida y vuelta, como el motor de vida.
  onMount(() => {
    let t0 = 0, id = 0;
    const vuelta = (t: number) => {
      if (animarParpadeo) {
        if (!t0) t0 = t;
        const f = ((t - t0) % 1400) / 160;
        cierre = f < 1 ? Math.sin(f * Math.PI) : 0;
      } else t0 = 0;
      id = requestAnimationFrame(vuelta);
    };
    id = requestAnimationFrame(vuelta);
    return () => cancelAnimationFrame(id);
  });

  function cambiar(i: number, f: Forma) {
    const formas = [...expresion.formas];
    formas[i] = f;
    expresion = { ...expresion, formas };
  }
  function agregar(t: (typeof TIPOS_FORMA)[number]) {
    expresion = { ...expresion, formas: [...expresion.formas, t.nueva()] };
    sel = expresion.formas.length - 1;
  }
  function quitar(i: number) {
    expresion = { ...expresion, formas: expresion.formas.filter((_, k) => k !== i) };
    sel = Math.min(sel, expresion.formas.length - 1);
  }
  function duplicar(i: number, conEspejo: boolean) {
    const f = conEspejo ? espejar(expresion.formas[i]) : clonar(expresion.formas[i]);
    expresion = { ...expresion, formas: [...expresion.formas.slice(0, i + 1), f, ...expresion.formas.slice(i + 1)] };
    sel = i + 1;
  }
  function correr(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= expresion.formas.length) return;
    const formas = [...expresion.formas];
    [formas[i], formas[j]] = [formas[j], formas[i]];
    expresion = { ...expresion, formas };
    sel = j;
  }
  let tipoNuevo = $state(TIPOS_FORMA[0].tipo);
</script>

<div class="editor">
  <div class="escenario">
    <div class="placa" class:redonda>
      <div class="lienzo" style="aspect-ratio: {redonda ? '1 / 1' : '320 / 172'}">
        <canvas bind:this={canvas} width="640" height="344" aria-label="Vista previa de la cara"></canvas>
        {#if listo && !redonda}
          <Manijas formas={expresion.formas} seleccionada={sel} alCambiar={cambiar} alElegir={(i) => (sel = i)} />
        {/if}
      </div>
    </div>
    <div class="controles campos">
      <label><span class="etiqueta">Parpadeo · {cierre.toFixed(2)}</span><input type="range" min="0" max="1" step="0.01" bind:value={cierre} disabled={animarParpadeo} /></label>
      <label><span class="etiqueta">Mirada x</span><input type="range" min="-0.08" max="0.08" step="0.005" bind:value={miradaX} /></label>
      <label><span class="etiqueta">Mirada y</span><input type="range" min="-0.05" max="0.05" step="0.005" bind:value={miradaY} /></label>
      <div class="fila">
        <button class="chico" aria-pressed={animarParpadeo} onclick={() => (animarParpadeo = !animarParpadeo)}>Parpadear</button>
        <button class="chico" aria-pressed={redonda} onclick={() => (redonda = !redonda)}>Broche</button>
        <button class="chico" onclick={() => { cierre = 0; miradaX = 0; miradaY = 0; }}>Centrar</button>
      </div>
    </div>
    {#if redonda}<p class="nota">En el broche los ojos se ven 1,5× y los indicadores, más cerca del centro. Para mover formas, volvé a la pantalla rectangular.</p>{/if}
    {#if error}<p class="aviso-caja error" role="alert">El robot no la aceptaría: {error}</p>{/if}
  </div>

  <div class="panel grilla">
    <div class="campos">
      <label><span class="etiqueta">Nombre</span>
        <input bind:value={nombre} disabled={nombreFijo} maxlength="24" placeholder="mi-cara" aria-invalid={!nombreValido(nombre)} />
      </label>
      <label class="fila" style="align-self: end"><input type="checkbox" checked={expresion.parpadea !== false}
        onchange={(e) => (expresion = { ...expresion, parpadea: e.currentTarget.checked ? undefined : false })} /> <span class="etiqueta">parpadea</span></label>
    </div>
    {#if !nombreValido(nombre)}<p class="nota">El nombre va en minúsculas, números y guiones, y empieza con una letra.</p>{/if}

    <div>
      <p class="etiqueta">Formas ({expresion.formas.length}/12)</p>
      <ol class="formas">
        {#each expresion.formas as f, i}
          <li class:elegida={i === sel}>
            <button class="nombre" onclick={() => (sel = i)}>{i + 1}. {TIPOS_FORMA.find((t) => t.tipo === f.forma)?.nombre}{f.fijo ? ' · fijo' : ''}</button>
            <span class="fila acciones">
              <button class="icono" title="Subir" aria-label="Subir" onclick={() => correr(i, -1)}>↑</button>
              <button class="icono" title="Bajar" aria-label="Bajar" onclick={() => correr(i, 1)}>↓</button>
              <button class="icono" title="Duplicar en el otro ojo (espejado)" aria-label="Espejar" disabled={expresion.formas.length >= 12} onclick={() => duplicar(i, true)}>⇋</button>
              <button class="icono" title="Duplicar" aria-label="Duplicar" disabled={expresion.formas.length >= 12} onclick={() => duplicar(i, false)}>⧉</button>
              <button class="icono peligro" title="Quitar" aria-label="Quitar" disabled={expresion.formas.length <= 1} onclick={() => quitar(i)}>×</button>
            </span>
          </li>
        {/each}
      </ol>
      <div class="fila">
        <select bind:value={tipoNuevo} style="flex: 1">{#each TIPOS_FORMA as t}<option value={t.tipo}>{t.nombre}</option>{/each}</select>
        <button disabled={expresion.formas.length >= 12} onclick={() => agregar(TIPOS_FORMA.find((t) => t.tipo === tipoNuevo)!)}>Agregar</button>
      </div>
    </div>

    {#if sel >= 0 && expresion.formas[sel]}
      <div class="tarjeta medida">
        <p class="etiqueta">Forma {sel + 1}</p>
        <CamposForma forma={expresion.formas[sel]} alCambiar={(f) => cambiar(sel, f)} />
        {#if expresion.formas[sel].puntos}<p class="nota">Arrastrá los puntos sobre la pantalla.</p>{/if}
      </div>
    {:else}
      <p class="nota">Tocá una forma en la pantalla (o en la lista) para moverla y cambiarle las medidas. ⇋ la copia espejada para el otro ojo.</p>
    {/if}
  </div>
</div>

<style>
  .editor { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(300px, 1fr); gap: 24px; align-items: start; }
  .escenario { display: grid; gap: 14px; position: sticky; top: 84px; }
  .placa { width: 100%; }
  .placa.redonda { max-width: 420px; justify-self: center; }
  .lienzo { position: relative; width: 100%; }
  .lienzo canvas { width: 100%; height: 100%; }
  .formas { list-style: none; margin: 8px 0; padding: 0; display: grid; gap: 4px; }
  .formas li { display: flex; justify-content: space-between; align-items: center; border: 1px solid var(--linea); border-radius: 10px; padding: 3px 4px 3px 4px; }
  .formas li.elegida { border-color: var(--naranja); }
  .formas .nombre { border: 0; background: none; text-transform: none; letter-spacing: 0; font: 15px var(--serif); padding: 6px 8px; }
  .acciones { gap: 2px; }
  .acciones .icono { border: 0; background: none; padding: 6px; }
  .medida { box-shadow: none; padding: 14px; }
  @media (max-width: 900px) { .editor { grid-template-columns: 1fr; } .escenario { position: static; } }
</style>
