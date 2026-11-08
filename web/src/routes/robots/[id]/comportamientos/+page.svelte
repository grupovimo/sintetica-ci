<script lang="ts">
  // Comportamientos: lo que hace solo, cuando le pasa algo, y sus rutinas.
  // Cada uno es una tira de pasos (una cara un rato). Se prueba en el robot
  // vivo de arriba, con la config que se está armando.
  import { onDestroy, onMount } from 'svelte';
  import { usarBicho } from '$lib/bicho.svelte';
  import { foto } from '$lib/foto.svelte';
  import { pintar, vistaPrevia, type VistaPrevia } from '$lib/nucleo';
  import { Car, comando } from '$compartido/protocolo.js';
  import {
    clonar, COMPORTAMIENTOS_BASE, MIRADAS, nombreValido, NOMBRES_RASGO, PARPADEOS, PARTICULAS, GLIFO_PARTICULA, RASGOS,
    type Comportamiento, type Config, type Paso,
  } from '$lib/formato';
  import { pantallaDe } from '$lib/tipos';
  import { miniaturasDeCaras } from '$lib/consola/miniaturas';
  import Perilla from '$lib/consola/Perilla.svelte';

  const b = usarBicho();
  const propios = $derived(b.config.comportamientos ?? {});
  const todos = $derived([...new Set([...Object.keys(COMPORTAMIENTOS_BASE), ...Object.keys(propios)])]);
  let tipo = $state<'ocioso' | 'reaccion' | 'rutina'>('ocioso');
  let elegido = $state('');
  let nombre = $state('');
  let doc = $state<Comportamiento | null>(null);
  let paso = $state(0);
  let vista = $state<'pasos' | 'cuando'>('pasos');
  let error = $state('');
  let aviso = $state('');
  let guardando = $state(false);
  let caras = $state<Record<string, string>>({});
  let canvas = $state<HTMLCanvasElement>();
  let sim: VistaPrevia | null = null;
  let cuadro = 0;
  let original = '';
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });

  const TIPOS: [typeof tipo, string, string][] = [
    ['ocioso', 'Solo', 'M12 3v3 M12 18v3 M3 12h3 M18 12h3 M12 8a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z'],
    ['reaccion', 'Reacciones', 'M13 2 4 14h7l-1 8 9-12h-7l1-8Z'],
    ['rutina', 'Rutinas', 'M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z'],
  ];
  const GLIFO = GLIFO_PARTICULA;
  const tipoDe = (n: string) => (propios[n] ?? COMPORTAMIENTOS_BASE[n])?.tipo ?? 'ocioso';
  const lista = $derived(todos.filter((n) => tipoDe(n) === tipo));
  const primeraCara = (n: string) => (propios[n] ?? COMPORTAMIENTOS_BASE[n])?.pasos.find((p) => p.cara)?.cara ?? 'neutral';

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
    caras = await miniaturasDeCaras({ ...foto(b.config), version: 2 }, pantalla);
  });
  onDestroy(() => cancelAnimationFrame(cuadro));

  function elegir(n: string) {
    error = aviso = '';
    elegido = nombre = n;
    doc = clonar(propios[n] ?? COMPORTAMIENTOS_BASE[n] ?? { pasos: [{ cara: 'contento', dura: 1.5 }] });
    original = JSON.stringify(doc);
    paso = 0;
  }
  function nuevo() {
    let n = 'mi-rutina', k = 2;
    while (todos.includes(n)) n = `mi-rutina-${k++}`;
    elegido = '';
    nombre = n;
    doc = { tipo, ...(tipo === 'ocioso' ? { cuando: 'aburrimiento>0.3', enfriamiento: 120 } : {}), pasos: [{ cara: 'contento', dura: 1.5 }, { cara: 'guino', dura: 0.8 }] };
    original = '';
    paso = 0;
  }
  const cambiado = $derived(!!doc && JSON.stringify(doc) !== original);

  const rango = (v: Paso['dura']): [number, number] => (Array.isArray(v) ? v : [v ?? 1, v ?? 1]);
  const media = (v: Paso['dura']) => { const r = rango(v); return (r[0] + r[1]) / 2; };
  const total = $derived(doc ? doc.pasos.reduce((a, p) => a + media(p.dura), 0) : 0);
  function ponerDura(p: Paso, i: 0 | 1, x: number) {
    const r = rango(p.dura);
    r[i] = Math.round(Math.min(60, Math.max(0.1, x)) * 10) / 10;
    if (r[1] < r[0]) r[i === 0 ? 1 : 0] = r[i];
    p.dura = r[0] === r[1] ? r[0] : [r[0], r[1]];
  }
  function moverPaso(i: number, d: number) {
    if (!doc) return;
    const j = i + d;
    if (j < 0 || j >= doc.pasos.length) return;
    [doc.pasos[i], doc.pasos[j]] = [doc.pasos[j], doc.pasos[i]];
    paso = j;
  }
  function sumarPaso() {
    if (!doc || doc.pasos.length >= 16) return;
    doc.pasos.splice(paso + 1, 0, { cara: doc.pasos[paso]?.cara ?? 'contento', dura: 1 });
    paso++;
  }
  function quitarPaso() {
    if (!doc || doc.pasos.length <= 1) return;
    doc.pasos.splice(paso, 1);
    paso = Math.max(0, paso - 1);
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
    original = JSON.stringify(doc);
    sim?.aplicar(JSON.stringify({ ...foto(b.config), version: 2 }));
    aviso = 'Guardado';
    setTimeout(() => (aviso = ''), 2500);
  }
  async function volver() {
    const e = await b.guardar(configCon(elegido, null));
    if (e) { error = e; return; }
    elegir(COMPORTAMIENTOS_BASE[elegido] ? elegido : 'mirar-alrededor');
  }
  async function probarEnRobot() {
    error = (await b.marco(Car.comando, comando.comportamiento(elegido))) ?? '';
  }
  const p = $derived(doc?.pasos[paso]);
</script>

<div class="comportamientos">
  <aside class="lista">
    <div class="tipos" role="tablist" aria-label="Tipo">
      {#each TIPOS as [k, t, icono]}
        <button role="tab" class="tipo" class:sel={tipo === k} aria-selected={tipo === k} onclick={() => (tipo = k)}>
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={icono} /></svg>{t}
        </button>
      {/each}
    </div>
    <button class="item nuevo" onclick={nuevo}><span class="mas">+</span>Nuevo</button>
    {#each lista as n}
      <button class="item" class:sel={n === elegido} onclick={() => elegir(n)}>
        <span class="mini" class:redonda={pantalla.redonda}>{#if caras[primeraCara(n)]}<img src={caras[primeraCara(n)]} alt="" />{/if}</span>
        <span class="n">{n.replaceAll('-', ' ')}</span>{#if propios[n]}<span class="tuyo"></span>{/if}
      </button>
    {/each}
  </aside>

  {#if doc}
    <section class="mesa">
      <div class="escenario">
        <div class="pantalla" class:redonda={pantalla.redonda}><canvas bind:this={canvas} aria-label="El robot, en vivo"></canvas></div>
        <button class="tecla naranja probar" onclick={probarAca} title="Probarlo acá">▶</button>
      </div>

      <div class="barra">
        <label class="visor nombre"><span class="grabado">Nombre</span>
          <input class="campo" bind:value={nombre} maxlength="24" disabled={!!elegido && !!COMPORTAMIENTOS_BASE[elegido]} /></label>
        <button class="tecla negra" onclick={probarEnRobot} disabled={!elegido || !b.bicho?.conectado}>▶ En el robot</button>
        {#if elegido && propios[elegido]}<button class="tecla negra" onclick={volver}>{COMPORTAMIENTOS_BASE[elegido] ? '↺ Original' : 'Borrar'}</button>{/if}
        <button class="tecla naranja" onclick={guardar} disabled={guardando || (!cambiado && !!elegido)}>{guardando ? 'Guardando…' : aviso || 'Guardar'}</button>
      </div>
      {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}

      <div class="placa-naranja tablero">
        <div class="cabeza">
          <div class="solapas" role="group">
            <button class="tecla chica" aria-pressed={vista === 'pasos'} onclick={() => (vista = 'pasos')}>Pasos</button>
            {#if (doc.tipo ?? 'ocioso') === 'ocioso'}<button class="tecla chica" aria-pressed={vista === 'cuando'} onclick={() => (vista = 'cuando')}>Cuándo</button>{/if}
          </div>
          <span class="grabado">{doc.pasos.length} pasos · {total.toFixed(1)} s</span>
        </div>

        {#if vista === 'pasos' || (doc.tipo ?? 'ocioso') !== 'ocioso'}
          <div class="tira" role="listbox" aria-label="Pasos">
            {#each doc.pasos as q, i}
              <button role="option" aria-selected={i === paso} class="bloque" class:sel={i === paso} style="flex-grow: {Math.max(0.6, media(q.dura))}" onclick={() => (paso = i)}>
                <span class="mini" class:redonda={pantalla.redonda}>{#if q.cara && caras[q.cara]}<img src={caras[q.cara]} alt="" />{:else}<span class="reposo">·</span>{/if}</span>
                <span class="dura">{rango(q.dura)[0] === rango(q.dura)[1] ? rango(q.dura)[0] : `${rango(q.dura)[0]}–${rango(q.dura)[1]}`} s{q.particula ? ` ${GLIFO[q.particula] ?? ''}` : ''}</span>
              </button>
            {/each}
            <button class="bloque agregar" disabled={doc.pasos.length >= 16} onclick={sumarPaso} aria-label="Sumar un paso">+</button>
          </div>

          {#if p}
            <div class="bandeja editor">
              <div class="herramientas">
                <button class="tecla negra cuadrada" aria-label="Antes" disabled={paso === 0} onclick={() => moverPaso(paso, -1)}>←</button>
                <button class="tecla negra cuadrada" aria-label="Después" disabled={paso === doc.pasos.length - 1} onclick={() => moverPaso(paso, 1)}>→</button>
                <div class="visor dura-v">
                  <span class="grabado">Dura</span>
                  <div class="mm">
                    <button class="tecla negra cuadrada" aria-label="Menos" onclick={() => { ponerDura(p, 0, rango(p.dura)[0] - 0.5); ponerDura(p, 1, rango(p.dura)[1] - 0.5); }}>−</button>
                    <span class="digitos chicos">{media(p.dura).toFixed(1)} s</span>
                    <button class="tecla negra cuadrada" aria-label="Más" onclick={() => { ponerDura(p, 1, rango(p.dura)[1] + 0.5); ponerDura(p, 0, rango(p.dura)[0] + 0.5); }}>+</button>
                  </div>
                </div>
                <button class="tecla negra chica quitar" disabled={doc.pasos.length <= 1} onclick={quitarPaso}>Quitar paso</button>
              </div>

              <span class="grabado">Cara</span>
              <div class="caras">
                <button class="cara" class:sel={!p.cara} onclick={() => (p.cara = undefined)} title="La de reposo"><span class="mini vacia">·</span></button>
                {#each Object.keys(caras) as c}
                  <button class="cara" class:sel={p.cara === c} onclick={() => (p.cara = c)} title={c}><span class="mini" class:redonda={pantalla.redonda}><img src={caras[c]} alt={c} /></span></button>
                {/each}
              </div>

              <span class="grabado">Mira</span>
              <div class="chips">
                <button class="tecla chica" aria-pressed={!p.mirada} onclick={() => (p.mirada = undefined)}>libre</button>
                {#each MIRADAS.filter((m) => m !== 'libre') as m}<button class="tecla chica" aria-pressed={p.mirada === m} onclick={() => (p.mirada = m)}>{m}</button>{/each}
              </div>
              <span class="grabado">Le sale</span>
              <div class="chips">
                <button class="tecla chica" aria-pressed={!p.particula} onclick={() => (p.particula = undefined)}>—</button>
                {#each PARTICULAS as m}<button class="tecla chica glifo" aria-pressed={p.particula === m} title={m} onclick={() => (p.particula = m)}>{GLIFO[m] ?? m}</button>{/each}
              </div>
              <span class="grabado">Parpadea</span>
              <div class="chips">
                <button class="tecla chica" aria-pressed={!p.parpadeo} onclick={() => (p.parpadeo = undefined)}>normal</button>
                {#each PARPADEOS.filter((m) => m !== 'normal') as m}<button class="tecla chica" aria-pressed={p.parpadeo === m} onclick={() => (p.parpadeo = m)}>{m}</button>{/each}
              </div>
            </div>
          {/if}
        {:else}
          <div class="bandeja cuando">
            <div class="perillas">
              <Perilla nombre="Ganas" valor={doc.base ?? 0.3} alCambiar={(v) => (doc!.base = v)} />
              <Perilla nombre="Seguido" valor={doc.probabilidad ?? 1} min={0.01} max={1} paso={0.01} alCambiar={(v) => (doc!.probabilidad = v)} />
              <Perilla nombre="Espera" valor={doc.enfriamiento ?? 120} min={0} max={3600} paso={30} mostrar={(v) => (v < 60 ? `${v}s` : `${Math.round(v / 60)}m`)} alCambiar={(v) => (doc!.enfriamiento = v)} />
            </div>
            <span class="grabado">Le gusta más si es…</span>
            <div class="perillas">
              {#each RASGOS as r}
                <Perilla nombre={NOMBRES_RASGO[r][0]} valor={doc.afinidad?.[r] ?? 0} min={-1} max={1} paso={0.1} tamano={52}
                  mostrar={(v) => (v > 0 ? `+${Math.round(v * 10)}` : `${Math.round(v * 10)}`)} alCambiar={(v) => (doc!.afinidad = { ...doc!.afinidad, [r]: v })} />
              {/each}
            </div>
            <label class="visor condicion"><span class="grabado">Solo si</span>
              <input class="campo claro" bind:value={doc.cuando} placeholder="aburrimiento>0.3" spellcheck="false" /></label>
          </div>
        {/if}
      </div>
    </section>
  {/if}
</div>

<style>
  .comportamientos { display: grid; grid-template-columns: 236px minmax(0, 1fr); gap: 22px; align-items: start; }
  .lista { display: flex; flex-direction: column; gap: 6px; max-height: calc(100vh - 140px); overflow-y: auto; overflow-x: hidden; position: sticky; top: 16px; padding: 2px 6px 2px 2px; scrollbar-width: thin; }
  .tipos { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; margin-bottom: 8px; }
  .tipo { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 2px; border-radius: 12px; cursor: pointer; color: var(--gris);
    font: 800 10px/1 var(--marca); letter-spacing: .04em; text-transform: uppercase; background: var(--chapa); }
  .tipo.sel { color: var(--c-papel); box-shadow: 0 0 0 2px var(--brasa); }
  .tipo.sel svg { color: var(--brasa); }
  .item { display: flex; align-items: center; gap: 10px; padding: 6px; border-radius: 12px; cursor: pointer; color: var(--gris-claro); text-align: left; }
  .item:hover { background: rgba(255,255,255,.05); }
  .item.sel { background: var(--chapa); color: var(--c-papel); box-shadow: 0 0 0 2px var(--brasa); }
  .item .mini { width: 64px; flex: none; }
  .n { flex: 1; min-width: 0; font: 700 13px/1.15 var(--marca); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tuyo { width: 7px; height: 7px; border-radius: 50%; background: var(--led); box-shadow: 0 0 6px var(--led); flex: none; }
  .nuevo { color: var(--brasa); font: 800 13px/1 var(--marca); }
  .nuevo .mas { display: grid; place-items: center; width: 64px; aspect-ratio: 320 / 172; border-radius: 8px; border: 2px dashed #3a3632; font-size: 20px; }
  .mini { display: block; aspect-ratio: 320 / 172; border-radius: 8px; overflow: hidden; background: #050505; }
  .mini.redonda { aspect-ratio: 1; border-radius: 50%; }
  .mini img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .mesa { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
  .escenario { position: relative; }
  .pantalla { padding: 14px; border-radius: 30px; background: linear-gradient(180deg, #2c2925, #1c1a17); box-shadow: inset 0 1px 0 rgba(255,255,255,.1), 0 30px 50px -30px rgba(0,0,0,.9); }
  .pantalla canvas { display: block; width: 100%; aspect-ratio: 320 / 172; border-radius: 18px; background: #050505; }
  .pantalla.redonda { width: min(100%, 420px); margin: 0 auto; border-radius: 50%; }
  .pantalla.redonda canvas { aspect-ratio: 1; border-radius: 50%; }
  .probar { position: absolute; right: 26px; bottom: 26px; width: 64px; min-height: 64px; padding: 0; border-radius: 50%; font-size: 22px; }
  .barra { display: flex; gap: 12px; align-items: stretch; flex-wrap: wrap; }
  .nombre { flex: 1; min-width: 200px; padding: 10px 16px; gap: 6px; }
  .nombre .campo { font-size: 22px; }
  .barra .tecla { align-self: center; }
  .tablero { display: flex; flex-direction: column; gap: 16px; padding: 18px; }
  .cabeza { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  .solapas { display: flex; gap: 8px; padding-bottom: 6px; }
  .tira { display: flex; gap: 6px; align-items: stretch; padding: 10px; border-radius: 18px; background: #1c1a17; box-shadow: inset 0 3px 10px rgba(0,0,0,.8); overflow-x: auto; }
  .bloque { flex: 1 1 0; min-width: 76px; display: flex; flex-direction: column; gap: 6px; padding: 6px; border-radius: 12px; cursor: pointer; background: #2a2622; color: var(--gris-claro);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.08); }
  .bloque.sel { background: #3a3632; color: var(--c-papel); box-shadow: 0 0 0 2px var(--brasa); }
  .bloque .mini { width: 100%; height: 54px; aspect-ratio: auto; }
  .bloque .mini img { object-fit: contain; }
  .reposo { display: grid; place-items: center; height: 100%; color: #5a524a; font-size: 24px; }
  .dura { font: 800 11px/1 var(--marca); font-variant-numeric: tabular-nums; text-align: center; }
  .agregar { flex: 0 0 56px; min-width: 56px; display: grid; place-items: center; font: 700 24px/1 var(--marca); color: var(--brasa); background: transparent; border: 2px dashed #3a3632; }
  .editor { display: flex; flex-direction: column; gap: 12px; padding: 18px; }
  .herramientas { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; padding-bottom: 6px; }
  .dura-v { flex-direction: row; align-items: center; gap: 14px; padding: 8px 12px; }
  .mm { display: flex; align-items: center; gap: 10px; }
  .mm .digitos { min-width: 70px; text-align: center; }
  .quitar { margin-left: auto; }
  .caras { display: grid; grid-template-columns: repeat(auto-fill, minmax(70px, 1fr)); gap: 6px; max-height: 190px; overflow-y: auto; padding: 4px; }
  .cara { padding: 3px; border-radius: 10px; cursor: pointer; }
  .cara.sel { box-shadow: 0 0 0 3px #1a1714; }
  .vacia { display: grid; place-items: center; color: #5a524a; font-size: 22px; }
  .chips { display: flex; flex-wrap: wrap; gap: 10px 8px; padding-bottom: 6px; }
  .glifo { min-width: 40px; font-size: 16px; letter-spacing: 0; }
  .cuando { display: flex; flex-direction: column; gap: 18px; padding: 20px; }
  .perillas { display: grid; grid-template-columns: repeat(auto-fill, minmax(88px, 1fr)); gap: 20px 10px; }
  .condicion { padding: 10px 16px; gap: 6px; }
  .condicion .campo { font-family: ui-monospace, monospace; font-size: 16px; }
  @media (max-width: 1100px) {
    .comportamientos { grid-template-columns: 1fr; }
    .lista { position: static; max-height: 260px; }
  }
</style>
