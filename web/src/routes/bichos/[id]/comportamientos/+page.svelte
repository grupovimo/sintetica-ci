<script lang="ts">
  // Los comportamientos: lo que hace solo (ociosos, los elige el selector),
  // lo que hace cuando le pasa algo (reacciones, las disparan las reglas) y sus
  // rutinas (despertarse, contar ovejas, soñar). Cada uno es una lista de pasos.
  import { onDestroy, onMount } from 'svelte';
  import { usarBicho } from '$lib/bicho.svelte';
  import { foto } from '$lib/foto.svelte';
  import { pintar, vistaPrevia, type VistaPrevia } from '$lib/nucleo';
  import { Car, comando } from '$compartido/protocolo.js';
  import {
    clonar, COMPORTAMIENTOS_BASE, EXPRESIONES_BASE, MIRADAS, nombreValido, NOMBRES_RASGO, PARPADEOS, PARTICULAS, RASGOS,
    type Comportamiento, type Config, type Paso,
  } from '$lib/formato';
  import { pantallaDe } from '$lib/tipos';

  const b = usarBicho();
  const propios = $derived(b.config.comportamientos ?? {});
  const todos = $derived([...new Set([...Object.keys(COMPORTAMIENTOS_BASE), ...Object.keys(propios)])]);
  const caras = $derived([...new Set([...Object.keys(EXPRESIONES_BASE), ...Object.keys(b.config.expresiones ?? {})])]);
  let filtro = $state<'ocioso' | 'reaccion' | 'rutina'>('ocioso');
  let elegido = $state('');
  let nombre = $state('');
  let doc = $state<Comportamiento | null>(null);
  let error = $state('');
  let aviso = $state('');
  let guardando = $state(false);
  let canvas = $state<HTMLCanvasElement>();
  let sim: VistaPrevia | null = null;
  let cuadro = 0;
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });

  const tipoDe = (n: string) => (propios[n] ?? COMPORTAMIENTOS_BASE[n])?.tipo ?? 'ocioso';
  const lista = $derived(todos.filter((n) => tipoDe(n) === filtro));

  onMount(async () => {
    sim = await vistaPrevia({ redonda: pantalla.redonda, alCuadro: (rgba, w, h) => canvas && pintar(canvas, rgba, w, h) });
    sim.aplicar(JSON.stringify({ ...foto(b.config), version: 2 }));
    let ultimo = 0;
    const vuelta = (t: number) => {
      if (t - ultimo >= 33) { ultimo = t; sim?.tick(); }
      cuadro = requestAnimationFrame(vuelta);
    };
    cuadro = requestAnimationFrame(vuelta);
    elegir('mirar-alrededor');
  });
  onDestroy(() => cancelAnimationFrame(cuadro));

  function elegir(n: string) {
    error = aviso = '';
    elegido = nombre = n;
    doc = clonar(propios[n] ?? COMPORTAMIENTOS_BASE[n] ?? { pasos: [{ cara: 'contento', dura: 1.5 }] });
  }
  function nuevo() {
    let n = 'mi-comportamiento', k = 2;
    while (todos.includes(n)) n = `mi-comportamiento-${k++}`;
    elegido = '';
    nombre = n;
    doc = { tipo: filtro, cuando: filtro === 'ocioso' ? 'aburrimiento>0.3' : undefined, enfriamiento: 120, pasos: [{ cara: 'contento', dura: [1, 2] }] };
  }

  const rango = (v: Paso['dura']): [number, number] => (Array.isArray(v) ? v : [v ?? 1, v ?? 1]);
  function ponerDura(p: Paso, i: 0 | 1, x: number) {
    const r = rango(p.dura);
    r[i] = x;
    if (r[1] < r[0]) r[i === 0 ? 1 : 0] = x;
    p.dura = r[0] === r[1] ? r[0] : [r[0], r[1]];
  }
  function moverPaso(i: number, d: number) {
    if (!doc) return;
    const j = i + d;
    if (j < 0 || j >= doc.pasos.length) return;
    [doc.pasos[i], doc.pasos[j]] = [doc.pasos[j], doc.pasos[i]];
  }
  function limpio(c: Comportamiento): Comportamiento {
    const x = clonar(foto(c));
    for (const p of x.pasos) for (const k of Object.keys(p) as (keyof Paso)[]) if (p[k] === '' || p[k] === undefined) delete p[k];
    if (!x.cuando) delete x.cuando;
    if (x.afinidad) { for (const r of Object.keys(x.afinidad)) if (!x.afinidad[r]) delete x.afinidad[r]; if (!Object.keys(x.afinidad).length) delete x.afinidad; }
    return x;
  }

  function configCon(n: string, c: Comportamiento | null): Config {
    const cfg = clonar(foto(b.config)) as Config;
    cfg.version = 2;
    const comps = { ...(cfg.comportamientos ?? {}) };
    if (c) comps[n] = limpio(c); else delete comps[n];
    if (Object.keys(comps).length) cfg.comportamientos = comps; else delete cfg.comportamientos;
    return cfg;
  }

  function probarAca() {
    if (!sim || !doc) return;
    const e = sim.aplicar(JSON.stringify(configCon(nombre, doc)));
    if (e) { error = e; return; }
    error = '';
    sim.bleEscribir(Car.comando, comando.comportamiento(nombre));
  }

  async function guardar() {
    if (!doc) return;
    error = aviso = '';
    if (!nombreValido(nombre)) { error = 'El nombre va en minúsculas, sin espacios.'; return; }
    guardando = true;
    const e = await b.guardar(configCon(nombre, doc));
    guardando = false;
    if (e) { error = e; return; }
    elegido = nombre;
    sim?.aplicar(JSON.stringify({ ...foto(b.config), version: 2 }));
    aviso = 'Guardado.';
  }
  async function volver() {
    const e = await b.guardar(configCon(elegido, null));
    if (e) { error = e; return; }
    aviso = COMPORTAMIENTOS_BASE[elegido] ? 'Volvió al original.' : 'Borrado.';
    elegir(COMPORTAMIENTOS_BASE[elegido] ? elegido : 'mirar-alrededor');
  }
  async function probarEnBicho() {
    error = (await b.marco(Car.comando, comando.comportamiento(elegido))) ?? '';
  }
</script>

<div class="dos">
  <section class="tarjeta grilla">
    <div class="fila" role="group" aria-label="Tipo">
      <button class="chico" aria-pressed={filtro === 'ocioso'} onclick={() => (filtro = 'ocioso')}>Solo</button>
      <button class="chico" aria-pressed={filtro === 'reaccion'} onclick={() => (filtro = 'reaccion')}>Reacciones</button>
      <button class="chico" aria-pressed={filtro === 'rutina'} onclick={() => (filtro = 'rutina')}>Rutinas</button>
      <button class="primario chico" style="margin-left: auto" onclick={nuevo}>Nuevo</button>
    </div>
    <p class="nota">
      {#if filtro === 'ocioso'}Lo que hace solo: cada 250 ms el selector puntúa los que cumplen su "cuándo" (necesidad × afinidad con la personalidad), sortea entre los 3 mejores y no repite ninguno de los últimos 5.
      {:else if filtro === 'reaccion'}Lo que hace cuando le pasa algo: lo dispara una regla (comportamiento:nombre).
      {:else}Las rutinas del día: despertarse, contar ovejas, soñar.{/if}
    </p>
    <div class="lista">
      {#each lista as n}
        <button class="chico" aria-pressed={n === elegido} onclick={() => elegir(n)}>{n}{propios[n] ? ' ★' : ''}</button>
      {/each}
    </div>
    <canvas bind:this={canvas} class="vista" class:redonda={pantalla.redonda} aria-label="Vista previa"></canvas>
  </section>

  {#if doc}
    <section class="tarjeta grilla">
      <div class="campos">
        <label><span class="etiqueta">Nombre</span><input bind:value={nombre} maxlength="24" disabled={!!elegido && !!COMPORTAMIENTOS_BASE[elegido]} /></label>
        <label><span class="etiqueta">Tipo</span>
          <select bind:value={doc.tipo}><option value="ocioso">solo (ocioso)</option><option value="reaccion">reacción</option><option value="rutina">rutina</option></select></label>
        {#if (doc.tipo ?? 'ocioso') === 'ocioso'}
          <label><span class="etiqueta">Cuándo</span><input bind:value={doc.cuando} placeholder="sueno>0.5,noche" class="mono" /></label>
          <label><span class="etiqueta">Enfriamiento (s)</span><input type="number" min="0" max="86400" bind:value={doc.enfriamiento} /></label>
          <label><span class="etiqueta">Sin condiciones, puntaje · {(doc.base ?? 0.3).toFixed(2)}</span>
            <input type="range" min="0" max="1" step="0.05" value={doc.base ?? 0.3} oninput={(e) => (doc!.base = +e.currentTarget.value)} /></label>
          <label><span class="etiqueta">Probabilidad (raro &lt; 1) · {(doc.probabilidad ?? 1).toFixed(2)}</span>
            <input type="range" min="0.01" max="1" step="0.01" value={doc.probabilidad ?? 1} oninput={(e) => (doc!.probabilidad = +e.currentTarget.value)} /></label>
        {/if}
      </div>
      <p class="nota">"Cuándo": condiciones separadas por coma sobre el estado (energia, animo, afecto, aburrimiento, curiosidad, sueno, estres), los rasgos (rasgo.picardia), ignorado (minutos), hora, bateria, noche, dormido.</p>
      {#if (doc.tipo ?? 'ocioso') === 'ocioso'}
        <h3>Afinidad con la personalidad</h3>
        <div class="campos">
          {#each RASGOS as r}
            <label><span class="etiqueta">{NOMBRES_RASGO[r][0]} · {(doc.afinidad?.[r] ?? 0).toFixed(2)}</span>
              <input type="range" min="-1" max="1" step="0.1" value={doc.afinidad?.[r] ?? 0}
                oninput={(e) => (doc!.afinidad = { ...doc!.afinidad, [r]: +e.currentTarget.value })} /></label>
          {/each}
        </div>
      {/if}

      <h3>Pasos</h3>
      <ol class="pasos">
        {#each doc.pasos as p, i}
          <li class="paso">
            <label><span class="etiqueta">Cara</span>
              <select bind:value={p.cara}><option value={undefined}>(la de reposo)</option>{#each caras as c}<option value={c}>{c}</option>{/each}</select></label>
            <label><span class="etiqueta">Dura (s)</span>
              <span class="fila"><input type="number" min="0.05" max="60" step="0.1" value={rango(p.dura)[0]} oninput={(e) => ponerDura(p, 0, +e.currentTarget.value)} />
              a <input type="number" min="0.05" max="60" step="0.1" value={rango(p.dura)[1]} oninput={(e) => ponerDura(p, 1, +e.currentTarget.value)} /></span></label>
            <label><span class="etiqueta">Mirada</span><select bind:value={p.mirada}><option value={undefined}>libre</option>{#each MIRADAS as m}<option value={m}>{m}</option>{/each}</select></label>
            <label><span class="etiqueta">Parpadeo</span><select bind:value={p.parpadeo}><option value={undefined}>normal</option>{#each PARPADEOS as m}<option value={m}>{m}</option>{/each}</select></label>
            <label><span class="etiqueta">Partícula</span><select bind:value={p.particula}><option value={undefined}>ninguna</option>{#each PARTICULAS as m}<option value={m}>{m}</option>{/each}</select></label>
            <label><span class="etiqueta">Peso · {(p.peso ?? 1).toFixed(2)}</span><input type="range" min="0" max="1" step="0.05" value={p.peso ?? 1} oninput={(e) => (p.peso = +e.currentTarget.value)} /></label>
            <label><span class="etiqueta">Veces</span><input type="number" min="1" max="20" value={Array.isArray(p.veces) ? p.veces[0] : (p.veces ?? 1)} oninput={(e) => (p.veces = +e.currentTarget.value > 1 ? +e.currentTarget.value : undefined)} /></label>
            <label><span class="etiqueta">Sonido</span><input maxlength="8" bind:value={p.sonido} placeholder="—" /></label>
            <label><span class="etiqueta">Estado</span><input maxlength="24" bind:value={p.estado} placeholder="animo+0.1" class="mono" /></label>
            <span class="fila acciones">
              <button class="icono" aria-label="Subir" onclick={() => moverPaso(i, -1)} disabled={i === 0}>↑</button>
              <button class="icono" aria-label="Bajar" onclick={() => moverPaso(i, 1)} disabled={i === doc.pasos.length - 1}>↓</button>
              <button class="icono peligro" aria-label="Quitar" onclick={() => (doc!.pasos = doc!.pasos.filter((_, k) => k !== i))} disabled={doc.pasos.length === 1}>×</button>
            </span>
          </li>
        {/each}
      </ol>
      <button class="chico" onclick={() => doc!.pasos.push({ cara: 'neutral', dura: 1 })} disabled={doc.pasos.length >= 6}>Otro paso</button>

      <div class="fila">
        <button onclick={probarAca}>Probar acá</button>
        <button onclick={probarEnBicho} disabled={!elegido || !b.bicho?.conectado}>Probar en el bicho</button>
        <button class="primario" onclick={guardar} disabled={guardando}>{guardando ? 'Guardando…' : 'Guardar'}</button>
        {#if elegido && propios[elegido]}<button class="peligro" onclick={volver}>{COMPORTAMIENTOS_BASE[elegido] ? 'Volver al original' : 'Borrar'}</button>{/if}
      </div>
      {#if aviso}<p class="aviso-caja ok">{aviso}</p>{/if}
      {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
    </section>
  {/if}
</div>

<style>
  .dos { display: grid; grid-template-columns: minmax(260px, 1fr) 2fr; gap: 16px; align-items: start; }
  @media (max-width: 900px) { .dos { grid-template-columns: 1fr; } }
  .lista { display: flex; flex-wrap: wrap; gap: 6px; }
  .vista { width: 100%; border-radius: 14px; background: #0b0a09; }
  .vista.redonda { border-radius: 50%; }
  .pasos { display: grid; gap: 10px; padding-left: 20px; margin: 0; }
  .paso { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 6px 12px; align-items: end; padding: 8px 0; border-bottom: 1px solid var(--linea, #2c2925); }
  .paso input[type='number'] { width: 5em; }
  h3 { margin: 4px 0 0; font-size: 15px; }
</style>
