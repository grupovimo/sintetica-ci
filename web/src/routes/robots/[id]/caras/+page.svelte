<script lang="ts">
  import { foto } from '$lib/foto.svelte';
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { BASE, usarBicho } from '$lib/bicho.svelte';
  import { formatoBase } from '$lib/base';
  import { clonar, esDibujo, nombreValido, type Config, type Expresion } from '$lib/formato';
  import type { Pieza } from '$lib/tipos';
  import EditorCara from '$lib/componentes/EditorCara.svelte';
  import Miniatura from '$lib/componentes/Miniatura.svelte';

  const b = usarBicho();
  let base = $state<Record<string, Expresion>>({});
  let biblioteca = $state<Pieza[]>([]);
  let editando = $state<{ nombre: string; expresion: Expresion; nueva: boolean } | null>(null);
  let errorEditor = $state('');
  let aviso = $state('');
  let guardando = $state(false);

  onMount(async () => {
    const f = await formatoBase();
    base = Object.fromEntries(f.caras.map((c) => [c.nombre, c.documento]));
    biblioteca = (await api<{ piezas: Pieza[] }>('/v1/biblioteca?tipo=cara')).piezas;
  });

  // Solo los dibujos (expresiones con formas); las paramétricas van en Expresiones.
  const propias = $derived(Object.fromEntries(Object.entries(b.config.expresiones ?? {}).filter(([, e]) => esDibujo(e))) as Record<string, Expresion>);
  const nombresDibujos = $derived([...new Set([...BASE, ...Object.keys(propias)])]);
  const docDe = (n: string): Expresion | null => propias[n] ?? base[n] ?? null;

  function editar(n: string) {
    const doc = docDe(n);
    if (doc) editando = { nombre: n, expresion: clonar(doc), nueva: false };
  }
  function nueva(desde?: Expresion) {
    let n = 'mi-cara', k = 2;
    while (b.nombresCaras.includes(n) || nombresDibujos.includes(n)) n = `mi-cara-${k++}`;
    editando = { nombre: n, expresion: clonar(desde ?? base['neutral'] ?? { formas: [{ forma: 'pildora', x: -0.3125, y: 0, w: 0.15, h: 0.4 }] }), nueva: true };
  }

  async function guardarEnBicho() {
    if (!editando) return;
    guardando = true;
    const c = clonar(foto(b.config)) as Config;
    c.expresiones = { ...c.expresiones, [editando.nombre]: foto(editando.expresion) };
    const e = await b.guardar(c);
    guardando = false;
    if (e) errorEditor = e;
    else {
      aviso = `"${editando.nombre}" está en el robot.`;
      editando = null;
    }
  }

  async function guardarEnBiblioteca() {
    if (!editando) return;
    try {
      await api('/v1/biblioteca', { cuerpo: { tipo: 'cara', nombre: editando.nombre, documento: foto(editando.expresion) } });
      biblioteca = (await api<{ piezas: Pieza[] }>('/v1/biblioteca?tipo=cara')).piezas;
      aviso = `"${editando.nombre}" quedó en tu biblioteca.`;
    } catch (e) {
      errorEditor = e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : 'No pude guardarla.';
    }
  }

  async function volverALaBase(n: string) {
    const c = clonar(foto(b.config)) as Config;
    if (!c.expresiones) return;
    delete c.expresiones[n];
    if (!Object.keys(c.expresiones).length) delete c.expresiones;
    // Si alguien la usa (lista, reglas, animaciones) y no es base, el núcleo avisa.
    aviso = (await b.guardar(c)) ?? (BASE.includes(n) ? `"${n}" volvió a la del manual.` : `"${n}" se borró.`);
  }

  async function traer(p: Pieza) {
    const c = clonar(foto(b.config)) as Config;
    c.expresiones = { ...c.expresiones, [p.nombre]: p.documento };
    aviso = (await b.guardar(c)) ?? `"${p.nombre}" está en el robot.`;
  }
</script>

{#if editando}
  <div class="grilla">
    <div class="fila" style="justify-content: space-between">
      <h2>{editando.nueva ? 'Cara nueva' : `Editando "${editando.nombre}"`}</h2>
      <div class="fila">
        <button onclick={() => (editando = null)}>Cancelar</button>
        <button onclick={guardarEnBiblioteca} disabled={!!errorEditor || !nombreValido(editando.nombre)}>A mi biblioteca</button>
        <button class="primario" onclick={guardarEnBicho} disabled={guardando || !!errorEditor || !nombreValido(editando.nombre)}>{guardando ? 'Guardando…' : 'Guardar'}</button>
      </div>
    </div>
    <EditorCara bind:expresion={editando.expresion} bind:nombre={editando.nombre} nombreFijo={!editando.nueva} contexto={{ version: 1, cara: b.config.cara }} bind:error={errorEditor} />
  </div>
{:else}
  <div class="grilla">
    <div class="fila" style="justify-content: space-between">
      <span class="etiqueta">Dibujos</span>
      <button class="primario" onclick={() => nueva()} disabled={Object.keys(propias).length >= 24}>Cara nueva</button>
    </div>
    {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
    <div class="caras">
      {#each nombresDibujos as n}
        {@const doc = docDe(n)}
        <div class="cara tarjeta">
          <button class="mini" onclick={() => editar(n)} aria-label="Editar {n}">
            {#if doc}<Miniatura expresion={doc} contexto={{ version: 1, cara: b.config.cara }} ancho={200} />{/if}
          </button>
          <div class="fila" style="justify-content: space-between">
            <span class="mono">{n}</span>
            {#if propias[n]}
              <span class="fila"><span class="chip">{BASE.includes(n) ? 'cambiada' : 'propia'}</span>
                <button class="chico" onclick={() => volverALaBase(n)}>{BASE.includes(n) ? 'Original' : 'Borrar'}</button></span>
            {/if}
          </div>
        </div>
      {/each}
    </div>

    {#if biblioteca.length}
      <h3>De tu biblioteca</h3>
      <div class="caras">
        {#each biblioteca as p}
          <div class="cara tarjeta">
            <Miniatura expresion={p.documento} contexto={{ version: 1, cara: b.config.cara }} ancho={200} />
            <div class="fila" style="justify-content: space-between">
              <span class="mono">{p.nombre}</span>
              <span class="fila"><button class="chico" onclick={() => nueva(p.documento)}>Copiar</button><button class="chico" onclick={() => traer(p)}>Al robot</button></span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .caras { display: grid; gap: 14px; grid-template-columns: repeat(auto-fill, minmax(230px, 1fr)); }
  .cara { padding: 12px; display: grid; gap: 10px; box-shadow: none; }
  .mini { padding: 0; border: 0; background: none; border-radius: 8px; justify-self: center; }
</style>
