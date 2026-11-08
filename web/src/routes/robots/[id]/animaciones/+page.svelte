<script lang="ts">
  import { foto } from '$lib/foto.svelte';
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { formatoBase } from '$lib/base';
  import { clonar, nombreValido, type Animacion, type Config } from '$lib/formato';
  import type { Pieza } from '$lib/tipos';
  import EditorAnimacion from '$lib/componentes/EditorAnimacion.svelte';
  import Miniatura from '$lib/componentes/Miniatura.svelte';

  const b = usarBicho();
  let ejemplos = $state<{ nombre: string; documento: Animacion }[]>([]);
  let biblioteca = $state<Pieza[]>([]);
  let editando = $state<{ nombre: string; animacion: Animacion; nueva: boolean } | null>(null);
  let errorEditor = $state('');
  let aviso = $state('');
  let guardando = $state(false);

  onMount(async () => {
    ejemplos = (await formatoBase()).animaciones;
    biblioteca = (await api<{ piezas: Pieza[] }>('/v1/biblioteca?tipo=animacion')).piezas;
  });

  const propias = $derived(b.config.animaciones ?? {});
  const contexto = $derived({ version: 1 as const, cara: b.config.cara, expresiones: b.config.expresiones });

  function nombreLibre(base: string) {
    let n = base, k = 2;
    while (propias[n]) n = `${base}-${k++}`;
    return n;
  }
  function nueva(desde?: { nombre: string; documento: Animacion }) {
    editando = {
      nombre: nombreLibre(desde?.nombre ?? 'mi-animacion'),
      animacion: clonar(desde?.documento ?? { duracion: 1200, repetir: true, sobre: 'neutral', pistas: [{ forma: { forma: 'corazon', x: 0, y: -0.36, tamano: 0.5 }, claves: [{ t: 0, escala: 0.6 }, { t: 600, escala: 1.1 }, { t: 1200, escala: 0.6 }] }] }),
      nueva: true,
    };
  }

  async function guardarEnBicho() {
    if (!editando) return;
    guardando = true;
    const c = clonar(foto(b.config)) as Config;
    c.animaciones = { ...c.animaciones, [editando.nombre]: foto(editando.animacion) };
    const e = await b.guardar(c);
    guardando = false;
    if (e) errorEditor = e;
    else { aviso = `"${editando.nombre}" está en el bicho. Ponela en la lista o en una regla, o reproducila desde Resumen.`; editando = null; }
  }
  async function guardarEnBiblioteca() {
    if (!editando) return;
    try {
      await api('/v1/biblioteca', { cuerpo: { tipo: 'animacion', nombre: editando.nombre, documento: foto(editando.animacion) } });
      biblioteca = (await api<{ piezas: Pieza[] }>('/v1/biblioteca?tipo=animacion')).piezas;
      aviso = `"${editando.nombre}" quedó en tu biblioteca.`;
    } catch (e) {
      errorEditor = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude guardarla.';
    }
  }
  async function borrar(n: string) {
    const c = clonar(foto(b.config)) as Config;
    delete c.animaciones?.[n];
    if (c.animaciones && !Object.keys(c.animaciones).length) delete c.animaciones;
    aviso = (await b.guardar(c)) ?? `"${n}" se borró.`;
  }
  async function reproducir(n: string) {
    aviso = (await b.comando({ reproducir: n })) ?? `Reproduciendo "${n}" en el bicho.`;
  }
</script>

{#if editando}
  <div class="grilla">
    <div class="fila" style="justify-content: space-between">
      <h2>{editando.nueva ? 'Animación nueva' : `Editando "${editando.nombre}"`}</h2>
      <div class="fila">
        <button onclick={() => (editando = null)}>Cancelar</button>
        <button onclick={guardarEnBiblioteca} disabled={!!errorEditor || !nombreValido(editando.nombre)}>A mi biblioteca</button>
        <button class="primario" onclick={guardarEnBicho} disabled={guardando || !!errorEditor || !nombreValido(editando.nombre)}>{guardando ? 'Guardando…' : 'Guardar en el bicho'}</button>
      </div>
    </div>
    <EditorAnimacion bind:animacion={editando.animacion} bind:nombre={editando.nombre} nombreFijo={!editando.nueva} {contexto} caras={b.nombresCaras} bind:error={errorEditor} />
  </div>
{:else}
  <div class="grilla">
    <div class="fila" style="justify-content: space-between">
      <p class="nota" style="margin: 0">Formas que se mueven, escalan, giran y cambian de color en el tiempo. Hasta 6 por bicho.</p>
      <button class="primario" onclick={() => nueva()} disabled={Object.keys(propias).length >= 6}>Animación nueva</button>
    </div>
    {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
    {#if Object.keys(propias).length}
      <div class="lista">
        {#each Object.entries(propias) as [n, a]}
          <div class="item tarjeta">
            <button class="mini" onclick={() => (editando = { nombre: n, animacion: clonar(a), nueva: false })} aria-label="Editar {n}"><Miniatura animacion={a} {contexto} ancho={200} /></button>
            <div class="fila" style="justify-content: space-between">
              <span class="mono">{n} · {a.duracion} ms</span>
              <span class="fila"><button class="chico" disabled={!b.bicho?.conectado} onclick={() => reproducir(n)}>▶</button><button class="chico peligro" onclick={() => borrar(n)}>Borrar</button></span>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <p class="nota">Este bicho todavía no tiene animaciones.</p>
    {/if}
    <h3>Para empezar</h3>
    <div class="lista">
      {#each [...ejemplos.map((e) => ({ ...e, origen: 'ejemplo' })), ...biblioteca.map((p) => ({ nombre: p.nombre, documento: p.documento as Animacion, origen: 'biblioteca' }))] as e}
        <div class="item tarjeta">
          <Miniatura animacion={e.documento} {contexto} ancho={200} />
          <div class="fila" style="justify-content: space-between">
            <span class="mono">{e.nombre}</span><span class="fila"><span class="chip">{e.origen}</span><button class="chico" onclick={() => nueva(e)}>Usar</button></span>
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .lista { display: grid; gap: 14px; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); }
  .item { padding: 12px; display: grid; gap: 10px; box-shadow: none; }
  .mini { padding: 0; border: 0; background: none; justify-self: center; }
</style>
