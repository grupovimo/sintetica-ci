<script lang="ts">
  // Las expresiones del motor de vida: cada una es un destino de parámetros
  // (los dos ojos, la cara, la boca) más una capa de movimiento y una partícula.
  // Se editan con controles y se ven con el mismo núcleo del bicho.
  import { onMount } from 'svelte';
  import { usarBicho } from '$lib/bicho.svelte';
  import { foto } from '$lib/foto.svelte';
  import { nucleoEditor, pintar, type Simulador } from '$lib/nucleo';
  import {
    clonar, EXPRESIONES_BASE, esDibujo, MOVIMIENTOS, nombreValido, PARAMS_CARA, PARAMS_OJO, PARPADEOS, PARTICULAS,
    type Config, type ExpresionVida,
  } from '$lib/formato';
  import { pantallaDe } from '$lib/tipos';

  const b = usarBicho();
  let sim = $state<Simulador | null>(null);
  let nombres = $state<string[]>([]);
  let elegida = $state('');
  let nombre = $state('');
  let valores = $state<Float32Array | null>(null);  // el vector entero (N parámetros)
  let referencia: Float32Array | null = null;       // contra qué se guarda la diferencia
  let extra = $state({ movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' });
  let extraRef = { movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' };
  let lado = $state<'ojo' | 'izq' | 'der'>('ojo');
  let error = $state('');
  let aviso = $state('');
  let guardando = $state(false);
  let canvas = $state<HTMLCanvasElement>();
  let indices: Record<string, number> = {};
  const POR_OJO = PARAMS_OJO.length;

  const propias = $derived(Object.entries(b.config.expresiones ?? {}).filter(([, e]) => !esDibujo(e)).map(([n]) => n));
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });

  function configDelEditor(): Config {
    const c = clonar(foto(b.config)) as Config;
    c.version = 2;
    return c;
  }

  onMount(() => {
    let vivo = true;
    nucleoEditor().then((s) => {
      sim = s;
      s.editor.iniciar(pantalla.ancho * 2, pantalla.alto * 2, pantalla.redonda);
      const r = s.editor.config(configDelEditor());
      if (!r.ok) error = r.detalle;
      nombres = s.editor.caras();
      s.parametros().forEach((p: { nombre: string }, i: number) => {
        indices[i < POR_OJO ? `izq.${p.nombre}` : i < 2 * POR_OJO ? `der.${p.nombre}` : p.nombre] = i;
      });
      elegir(elegida || 'contento');
      const t0 = performance.now();
      const cuadro = () => {
        if (!vivo) return;
        if (sim && valores && canvas) {
          const t = (performance.now() - t0) / 1000;
          // Un parpadeo cada 3 s, para ver cómo queda con los ojos cerrándose.
          const f = t % 3;
          const parpadeo = f < 0.09 ? f / 0.09 : f < 0.2 ? 1 - (f - 0.09) / 0.11 : 0;
          const p = extra.particula === 'ninguna' ? -1 : PARTICULAS.indexOf(extra.particula as (typeof PARTICULAS)[number]);
          const { rgba, ancho, alto } = sim.editor.rostro(valores, t, parpadeo, p);
          pintar(canvas, rgba, ancho, alto);
        }
        requestAnimationFrame(cuadro);
      };
      requestAnimationFrame(cuadro);
    });
    return () => { vivo = false; };
  });

  function vectorDe(n: string): Float32Array | null {
    if (!sim) return null;
    const i = sim.editor.indiceCara(n);
    return i < 0 ? null : sim.editor.parametrosCara(i);
  }

  function elegir(n: string) {
    error = aviso = '';
    elegida = nombre = n;
    valores = vectorDe(n);
    // La referencia: la base con el mismo nombre (así se guarda solo lo que cambia) o la neutral.
    const base = EXPRESIONES_BASE[n];
    if (base && sim) {
      const r = sim.editor.config({ version: 2 });
      referencia = vectorDe(n);
      sim.editor.config(configDelEditor());
      if (!r.ok) referencia = null;
    } else {
      referencia = vectorDe('neutral');
    }
    const doc = (b.config.expresiones?.[n] as ExpresionVida | undefined) ?? base ?? {};
    const deBase = base ?? {};
    extra = { movimiento: (doc.movimiento ?? deBase.movimiento ?? 'ninguno') as string, particula: (doc.particula ?? deBase.particula ?? 'ninguna') as string, parpadeo: (doc.parpadeo ?? deBase.parpadeo ?? 'normal') as string };
    extraRef = { movimiento: (deBase.movimiento ?? 'ninguno') as string, particula: (deBase.particula ?? 'ninguna') as string, parpadeo: (deBase.parpadeo ?? 'normal') as string };
  }

  function nueva() {
    let n = 'mi-expresion', k = 2;
    while (nombres.includes(n)) n = `mi-expresion-${k++}`;
    const desde = elegida;
    elegir(desde);
    elegida = '';
    nombre = n;
    referencia = vectorDe('neutral');
    extraRef = { movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' };
  }

  const valor = (clave: string) => (valores ? valores[indices[clave]] : 0);
  function poner(p: string, v: number) {
    if (!valores) return;
    const nuevo = new Float32Array(valores);
    if (lado === 'ojo' || lado === 'izq') nuevo[indices[`izq.${p}`]] = v;
    if (lado === 'ojo' || lado === 'der') nuevo[indices[`der.${p}`]] = v;
    valores = nuevo;
  }
  function ponerCara(p: string, v: number) {
    if (!valores) return;
    const nuevo = new Float32Array(valores);
    nuevo[indices[p]] = v;
    valores = nuevo;
  }

  // Lo que se guarda: solo lo que cambia respecto de la referencia.
  function documento(): ExpresionVida {
    const d: ExpresionVida = {};
    const r = (x: number) => Math.round(x * 1000) / 1000;
    const distinto = (a: number, b: number) => Math.abs(a - b) > 0.0005;
    if (!valores || !referencia) return d;
    const ojo: Record<string, number> = {}, izq: Record<string, number> = {}, der: Record<string, number> = {};
    for (const [p] of PARAMS_OJO) {
      const a = valores[indices[`izq.${p}`]], bb = valores[indices[`der.${p}`]];
      const ra = referencia[indices[`izq.${p}`]], rb = referencia[indices[`der.${p}`]];
      if (!distinto(a, bb) && (distinto(a, ra) || distinto(bb, rb))) ojo[p] = r(a);
      else {
        if (distinto(a, ra)) izq[p] = r(a);
        if (distinto(bb, rb)) der[p] = r(bb);
      }
    }
    if (Object.keys(ojo).length) d.ojo = ojo;
    if (Object.keys(izq).length) d.izq = izq;
    if (Object.keys(der).length) d.der = der;
    for (const [p] of PARAMS_CARA) if (distinto(valores[indices[p]], referencia[indices[p]])) d[p] = r(valores[indices[p]]);
    if (extra.movimiento !== extraRef.movimiento) d.movimiento = extra.movimiento;
    if (extra.particula !== extraRef.particula) d.particula = extra.particula;
    if (extra.parpadeo !== extraRef.parpadeo) d.parpadeo = extra.parpadeo;
    return d;
  }

  async function guardar() {
    error = aviso = '';
    if (!nombreValido(nombre)) { error = 'El nombre va en minúsculas, sin espacios (letras, números y guiones).'; return; }
    const c = configDelEditor();
    const doc = documento();
    const esBase = !!EXPRESIONES_BASE[nombre];
    if (esBase && !Object.keys(doc).length) {
      if (c.expresiones) delete c.expresiones[nombre];
    } else {
      c.expresiones = { ...c.expresiones, [nombre]: doc };
    }
    if (c.expresiones && !Object.keys(c.expresiones).length) delete c.expresiones;
    guardando = true;
    const e = await b.guardar(c);
    guardando = false;
    if (e) { error = e; return; }
    sim?.editor.config(configDelEditor());
    nombres = sim?.editor.caras() ?? nombres;
    elegida = nombre;
    aviso = b.bicho?.conectado ? 'Guardada: ya la tiene el bicho.' : 'Guardada: le llega cuando se conecte.';
  }

  async function volverABase() {
    const c = configDelEditor();
    if (!c.expresiones) return;
    delete c.expresiones[elegida];
    if (!Object.keys(c.expresiones).length) delete c.expresiones;
    const e = await b.guardar(c);
    if (e) { error = e; return; }
    sim?.editor.config(configDelEditor());
    nombres = sim?.editor.caras() ?? nombres;
    if (nombres.includes(elegida)) elegir(elegida); else elegir('neutral');
    aviso = EXPRESIONES_BASE[elegida] ? 'Volvió a la original.' : 'Borrada.';
  }

  async function probarEnBicho() {
    error = (await b.comando({ expresion: elegida })) ?? '';
    if (!error) setTimeout(() => b.comando({ expresion: '' }), 4000);
  }
</script>

<div class="dos">
  <section class="tarjeta grilla">
    <div class="fila">
      <h2 style="margin-right: auto">Expresiones</h2>
      <button class="primario chico" onclick={nueva} disabled={!sim}>Nueva</button>
    </div>
    <p class="nota">Las que usa el motor de vida. Las tuyas están marcadas; cambiar una de las originales guarda solo lo que cambiaste.</p>
    <div class="lista">
      {#each nombres as n}
        <button class="chico" aria-pressed={n === elegida} onclick={() => elegir(n)}>{n}{propias.includes(n) ? ' ★' : ''}</button>
      {/each}
    </div>
  </section>

  <section class="tarjeta grilla">
    <canvas bind:this={canvas} class="vista" class:redonda={pantalla.redonda} aria-label="Vista previa"></canvas>
    <div class="fila">
      <label style="flex: 1"><span class="etiqueta">Nombre</span><input bind:value={nombre} maxlength="24" disabled={!!elegida && !!EXPRESIONES_BASE[elegida]} /></label>
      <button onclick={probarEnBicho} disabled={!elegida || !b.bicho?.conectado} title="La pone 4 s en el bicho">Probar en el bicho</button>
    </div>

    <div class="fila" role="group" aria-label="Qué ojo">
      <button class="chico" aria-pressed={lado === 'ojo'} onclick={() => (lado = 'ojo')}>Los dos ojos</button>
      <button class="chico" aria-pressed={lado === 'izq'} onclick={() => (lado = 'izq')}>Izquierdo</button>
      <button class="chico" aria-pressed={lado === 'der'} onclick={() => (lado = 'der')}>Derecho</button>
    </div>
    {#if valores}
      <div class="controles">
        {#each PARAMS_OJO as [p, min, max, texto]}
          {@const v = valor(lado === 'der' ? `der.${p}` : `izq.${p}`)}
          <label><span class="etiqueta">{texto} · {v.toFixed(2)}</span>
            <input type="range" {min} {max} step="0.005" value={v} oninput={(e) => poner(p, +e.currentTarget.value)} /></label>
        {/each}
      </div>
      <h3>La cara</h3>
      <div class="controles">
        {#each PARAMS_CARA as [p, min, max, texto]}
          {@const v = valor(p)}
          <label><span class="etiqueta">{texto} · {v.toFixed(2)}</span>
            <input type="range" {min} {max} step="0.005" value={v} oninput={(e) => ponerCara(p, +e.currentTarget.value)} /></label>
        {/each}
      </div>
      <div class="controles">
        <label><span class="etiqueta">Movimiento</span>
          <select bind:value={extra.movimiento}>{#each MOVIMIENTOS as m}<option value={m}>{m}</option>{/each}</select></label>
        <label><span class="etiqueta">Partícula</span>
          <select bind:value={extra.particula}><option value="ninguna">ninguna</option>{#each PARTICULAS as m}<option value={m}>{m}</option>{/each}</select></label>
        <label><span class="etiqueta">Parpadeo</span>
          <select bind:value={extra.parpadeo}>{#each PARPADEOS as m}<option value={m}>{m}</option>{/each}</select></label>
      </div>
    {/if}
    <div class="fila">
      <button class="primario" onclick={guardar} disabled={guardando || !valores}>{guardando ? 'Guardando…' : 'Guardar en el bicho'}</button>
      {#if elegida && propias.includes(elegida)}
        <button class="peligro" onclick={volverABase}>{EXPRESIONES_BASE[elegida] ? 'Volver a la original' : 'Borrar'}</button>
      {/if}
    </div>
    {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
    {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
  </section>
</div>

<style>
  .dos { display: grid; grid-template-columns: minmax(220px, 1fr) 2fr; gap: 16px; align-items: start; }
  @media (max-width: 900px) { .dos { grid-template-columns: 1fr; } }
  .lista { display: flex; flex-wrap: wrap; gap: 6px; }
  .vista { width: 100%; max-width: 640px; border-radius: 18px; background: #0b0a09; }
  .vista.redonda { max-width: 360px; border-radius: 50%; justify-self: center; }
  .controles { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px 16px; }
  h3 { margin: 4px 0 0; font-size: 15px; }
</style>
