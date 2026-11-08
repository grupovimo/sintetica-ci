<script lang="ts">
  // Expresiones: una galería con las caras de verdad (las dibuja el núcleo), la
  // elegida grande y viva, y perillas agrupadas por parte de la cara. Se guarda
  // solo lo que cambia respecto de la original.
  import { onMount, untrack } from 'svelte';
  import { usarBicho } from '$lib/bicho.svelte';
  import { foto } from '$lib/foto.svelte';
  import { nucleoEditor, pintar, type Simulador } from '$lib/nucleo';
  import {
    clonar, EXPRESIONES_BASE, esDibujo, esImagen, MOVIMIENTOS, nombreValido, PARAMS_CARA, PARAMS_OJO, PARPADEOS, PARTICULAS, GLIFO_PARTICULA,
    type Config, type ExpresionVida,
  } from '$lib/formato';
  import DeImagen from './DeImagen.svelte';
  import { pantallaDe } from '$lib/tipos';
  import Perilla from '$lib/consola/Perilla.svelte';
  import Pad from '$lib/consola/Pad.svelte';

  const b = usarBicho();
  let sim = $state<Simulador | null>(null);
  let nombres = $state<string[]>([]);
  let miniaturas = $state<Record<string, string>>({});
  let elegida = $state('');
  let nombre = $state('');
  let valores = $state<Float32Array | null>(null);
  let referencia: Float32Array | null = null;
  let extra = $state({ movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' });
  let extraRef = { movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' };
  let lado = $state<'ojo' | 'izq' | 'der'>('ojo');
  let grupo = $state('ojos');
  let error = $state('');
  let aviso = $state('');
  let guardando = $state(false);
  let canvas = $state<HTMLCanvasElement>();
  let indices: Record<string, number> = {};
  // Una cara de imagen abierta: null = el editor de perillas; '' = una nueva.
  let imagen = $state<string | null>(null);
  const deImagen = (n: string) => esImagen(b.config.expresiones?.[n] as ExpresionVida | undefined);
  // La miniatura de una cara de imagen: su ojo.
  function miniImagen(n: string): string {
    const e = b.config.expresiones?.[n];
    const s = esImagen(e) ? e.sprites.ojo : undefined;
    return s !== undefined && b.sprites[s] ? `/api/v1/bichos/${b.id}/sprites/${s}?png=1&v=${b.sprites[s].crc}` : '';
  }
  const POR_OJO = PARAMS_OJO.length;

  const propias = $derived(Object.entries(b.config.expresiones ?? {}).filter(([, e]) => !esDibujo(e)).map(([n]) => n));
  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });
  const LIMITES = Object.fromEntries([...PARAMS_OJO, ...PARAMS_CARA].map(([p, min, max]) => [p, [min, max]])) as Record<string, [number, number]>;

  // Los grupos: cada uno con su ícono y sus perillas (clave, nombre corto).
  const GRUPOS: { clave: string; nombre: string; icono: string; ojo?: boolean; perillas: [string, string][] }[] = [
    { clave: 'ojos', nombre: 'Ojos', icono: 'M4 12c3-5 13-5 16 0-3 5-13 5-16 0Z M12 9.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5Z', ojo: true,
      perillas: [['w', 'Ancho'], ['h', 'Alto'], ['r', 'Redondez'], ['tilt', 'Giro'], ['squash', 'Aplastado']] },
    { clave: 'parpados', nombre: 'Párpados', icono: 'M3 13c4-6 14-6 18 0 M6 9l-1.5-2 M12 7.5V5 M18 9l1.5-2', ojo: true,
      perillas: [['lt', 'Arriba'], ['lta', 'Ángulo'], ['lb', 'Abajo'], ['lba', 'Ángulo']] },
    { clave: 'adornos', nombre: 'Adornos', icono: 'M12 20s-7-4.5-7-10a4 4 0 0 1 7-2.5A4 4 0 0 1 19 10c0 5.5-7 10-7 10Z', ojo: true,
      perillas: [['smile', '◠ Feliz'], ['heart', '♥ Corazón'], ['star', '★ Estrella'], ['cross', '✕ Cruz'], ['ring', '○ Anillo'], ['pupil', '● Pupila']] },
    { clave: 'mirada', nombre: 'Mirada', icono: 'M12 3v4 M12 17v4 M3 12h4 M17 12h4 M12 9a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z',
      perillas: [['ey', 'Altura'], ['sep', 'Separación'], ['fscale', 'Tamaño'], ['ftilt', 'Giro']] },
    { clave: 'boca', nombre: 'Boca', icono: 'M5 11c3 5 11 5 14 0 M8 16.5c2 1 6 1 8 0',
      perillas: [['mcurve', 'Sonrisa'], ['mo', 'Abierta'], ['mw', 'Ancha'], ['blush', 'Cachetes']] },
    { clave: 'efectos', nombre: 'Efectos', icono: 'M13 2 4 14h7l-1 8 9-12h-7l1-8Z',
      perillas: [['pulse', 'Pulso'], ['beat', 'Latido'], ['shake', 'Temblor'], ['spin', 'Remolino'], ['orbit', 'Órbita'], ['glitch', 'Glitch'], ['dim', 'Apagado'], ['hue', 'Rojo']] },
    { clave: 'vida', nombre: 'Gestos', icono: 'M4 17c2-6 5-6 8 0s6 6 8 0 M8 7h.01 M16 7h.01', perillas: [] },
  ];
  const actual = $derived(GRUPOS.find((g) => g.clave === grupo)!);
  const GLIFO = GLIFO_PARTICULA;

  function configDelEditor(): Config {
    const c = clonar(foto(b.config)) as Config;
    c.version = 2;
    return c;
  }

  function vectorDe(n: string): Float32Array | null {
    if (!sim) return null;
    const i = sim.editor.indiceCara(n);
    return i < 0 ? null : sim.editor.parametrosCara(i);
  }

  // Una miniatura de cada cara, dibujada una vez por el núcleo.
  function hacerMiniaturas() {
    if (!sim) return;
    const c = document.createElement('canvas');
    const m: Record<string, string> = {};
    for (const n of nombres) {
      const v = vectorDe(n);
      if (!v) continue;
      const { rgba, ancho, alto } = sim.editor.rostro(v, 0.4, 0, -1);
      pintar(c, rgba, ancho, alto);
      m[n] = c.toDataURL('image/jpeg', 0.8);
    }
    miniaturas = m;
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
      hacerMiniaturas();
      elegir(elegida || 'contento');
      const t0 = performance.now();
      const cuadro = () => {
        if (!vivo) return;
        if (sim && valores && canvas) {
          const t = (performance.now() - t0) / 1000;
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

  // La config llega (o cambia) después de abrir la página: el editor la vuelve a cargar.
  $effect(() => {
    void b.versionConfig;
    const s = sim;
    if (!s) return;
    untrack(() => {
      const r = s.editor.config(configDelEditor());
      if (r.ok) {
        nombres = s.editor.caras();
        hacerMiniaturas();
      }
    });
  });

  function elegir(n: string) {
    error = aviso = '';
    elegida = nombre = n;
    imagen = deImagen(n) ? n : null;
    if (imagen !== null) return;
    valores = vectorDe(n);
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
    let n = 'mi-cara', k = 2;
    while (nombres.includes(n)) n = `mi-cara-${k++}`;
    elegir(elegida || 'neutral');
    elegida = '';
    nombre = n;
    referencia = vectorDe('neutral');
    extraRef = { movimiento: 'ninguno', particula: 'ninguna', parpadeo: 'normal' };
  }

  const valor = (clave: string) => (valores ? valores[indices[clave]] : 0);
  const claveDe = (p: string, ojo: boolean) => (ojo ? (lado === 'der' ? `der.${p}` : `izq.${p}`) : p);
  function poner(p: string, v: number, ojo: boolean) {
    if (!valores) return;
    const nuevo = new Float32Array(valores);
    if (!ojo) nuevo[indices[p]] = v;
    else {
      if (lado !== 'der') nuevo[indices[`izq.${p}`]] = v;
      if (lado !== 'izq') nuevo[indices[`der.${p}`]] = v;
    }
    valores = nuevo;
  }

  function documento(): ExpresionVida {
    const d: ExpresionVida = {};
    const r = (x: number) => Math.round(x * 1000) / 1000;
    const distinto = (a: number, z: number) => Math.abs(a - z) > 0.0005;
    if (!valores || !referencia) return d;
    const ojo: Record<string, number> = {}, izq: Record<string, number> = {}, der: Record<string, number> = {};
    for (const [p] of PARAMS_OJO) {
      const a = valores[indices[`izq.${p}`]], z = valores[indices[`der.${p}`]];
      const ra = referencia[indices[`izq.${p}`]], rz = referencia[indices[`der.${p}`]];
      if (!distinto(a, z) && (distinto(a, ra) || distinto(z, rz))) ojo[p] = r(a);
      else {
        if (distinto(a, ra)) izq[p] = r(a);
        if (distinto(z, rz)) der[p] = r(z);
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
  const cambios = $derived.by(() => { void valores; void extra.movimiento; void extra.particula; void extra.parpadeo; return Object.keys(documento()).length; });

  async function guardar() {
    error = aviso = '';
    if (!nombreValido(nombre)) { error = 'El nombre va en minúsculas, sin espacios.'; return; }
    const c = configDelEditor();
    const doc = documento();
    if (EXPRESIONES_BASE[nombre] && !Object.keys(doc).length) { if (c.expresiones) delete c.expresiones[nombre]; }
    else c.expresiones = { ...c.expresiones, [nombre]: doc };
    if (c.expresiones && !Object.keys(c.expresiones).length) delete c.expresiones;
    guardando = true;
    const e = await b.guardar(c);
    guardando = false;
    if (e) { error = e; return; }
    sim?.editor.config(configDelEditor());
    nombres = sim?.editor.caras() ?? nombres;
    hacerMiniaturas();
    elegida = nombre;
    aviso = 'Guardada';
    setTimeout(() => (aviso = ''), 2500);
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
    hacerMiniaturas();
    elegir(nombres.includes(elegida) ? elegida : 'neutral');
  }

  function nuevaImagen() {
    elegida = '';
    imagen = '';
  }
  function imagenLista(n: string) {
    sim?.editor.config(configDelEditor());
    nombres = sim?.editor.caras() ?? nombres;
    hacerMiniaturas();
    if (n) { elegida = n; imagen = n; } else elegir('neutral');
  }

  async function probarEnRobot() {
    error = (await b.comando({ expresion: elegida })) ?? '';
    if (!error) setTimeout(() => b.comando({ expresion: '' }), 4000);
  }
</script>

<div class="expresiones">
  <aside class="galeria" aria-label="Caras">
    <button class="cara nueva" onclick={nueva} disabled={!sim}><span class="mas">+</span><span class="n">Nueva</span></button>
    <button class="cara nueva" class:elegida={imagen === ''} onclick={nuevaImagen} disabled={!sim} title="Una cara dibujada: SVG, PNG, JPEG o GIF">
      <span class="mas"><svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 5h16v14H4Z M4 16l5-5 4 4 3-3 4 4 M15.5 8.5h.01" /></svg></span>
      <span class="n">De imagen</span>
    </button>
    {#each nombres as n}
      {@const img = deImagen(n)}
      <button class="cara" class:elegida={n === elegida} aria-pressed={n === elegida} onclick={() => elegir(n)} title={n}>
        <span class="mini" class:redonda={pantalla.redonda} class:dibujo={img}>{#if img}{#if miniImagen(n)}<img src={miniImagen(n)} alt="" />{/if}{:else if miniaturas[n]}<img src={miniaturas[n]} alt="" />{/if}</span>
        <span class="n">{n}{#if propias.includes(n)}<span class="tuya" title="Tuya o cambiada"></span>{/if}</span>
      </button>
    {/each}
  </aside>

  {#if imagen !== null}
    {#key imagen}<DeImagen inicial={imagen} alListo={imagenLista} />{/key}
  {:else}
  <section class="mesa">
    <div class="pantalla" class:redonda={pantalla.redonda}><canvas bind:this={canvas} aria-label="La cara, en vivo"></canvas></div>

    <div class="barra">
      <label class="visor nombre"><span class="grabado">Nombre</span>
        <input class="campo" bind:value={nombre} maxlength="24" disabled={!!elegida && !!EXPRESIONES_BASE[elegida]} /></label>
      <button class="tecla negra" onclick={probarEnRobot} disabled={!elegida || !b.bicho?.conectado} title="La muestra 4 s en el robot">▶ En el robot</button>
      {#if elegida && propias.includes(elegida)}
        <button class="tecla negra" onclick={volverABase} title={EXPRESIONES_BASE[elegida] ? 'Volver a la original' : 'Borrarla'}>{EXPRESIONES_BASE[elegida] ? '↺ Original' : 'Borrar'}</button>
      {/if}
      <button class="tecla naranja" onclick={guardar} disabled={guardando || !valores || (!cambios && !!elegida)}>{guardando ? 'Guardando…' : aviso || (cambios ? `Guardar (${cambios})` : 'Guardar')}</button>
    </div>
    {#if error}<p class="aviso error" role="alert"><span class="led"></span>{error}</p>{/if}

    <div class="placa-naranja tablero">
      <div class="solapas" role="tablist" aria-label="Partes de la cara">
        {#each GRUPOS as g}
          <button role="tab" class="solapa" class:abajo={grupo === g.clave} aria-selected={grupo === g.clave} onclick={() => (grupo = g.clave)}>
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={g.icono} /></svg>
            <span>{g.nombre}</span>
          </button>
        {/each}
      </div>

      {#if valores}
        <div class="bandeja controles">
          {#if actual.ojo}
            <div class="ojos" role="radiogroup" aria-label="Qué ojo">
              {#each [['ojo', 'Los dos'], ['izq', 'Izquierdo'], ['der', 'Derecho']] as [k, t]}
                <button role="radio" aria-checked={lado === k} class="ojo" class:sel={lado === k} onclick={() => (lado = k as 'ojo')}>
                  <span class="pastillas"><i class:on={k !== 'der'}></i><i class:on={k !== 'izq'}></i></span>{t}
                </button>
              {/each}
            </div>
          {/if}
          {#if grupo === 'mirada'}
            <div class="mirada">
              <div class="pad"><Pad nombre="Hacia dónde mira" x={valor('gx')} y={valor('gy')} minX={-0.3} maxX={0.3} minY={-0.25} maxY={0.25}
                alCambiar={(x, y) => { poner('gx', x, false); poner('gy', y, false); }} /></div>
              <div class="perillas">
                {#each actual.perillas as [p, t]}
                  <Perilla nombre={t} valor={valor(p)} min={LIMITES[p][0]} max={LIMITES[p][1]} paso={0.01} tamano={58} alCambiar={(v) => poner(p, v, false)} />
                {/each}
              </div>
            </div>
          {:else if grupo === 'vida'}
            <div class="gestos">
              <span class="grabado">Se mueve</span>
              <div class="chips">{#each MOVIMIENTOS as m}<button class="tecla chica" aria-pressed={extra.movimiento === m} onclick={() => (extra.movimiento = m)}>{m}</button>{/each}</div>
              <span class="grabado">Le sale</span>
              <div class="chips">
                <button class="tecla chica" aria-pressed={extra.particula === 'ninguna'} onclick={() => (extra.particula = 'ninguna')}>—</button>
                {#each PARTICULAS as m}<button class="tecla chica glifo" aria-pressed={extra.particula === m} title={m} onclick={() => (extra.particula = m)}>{GLIFO[m] ?? m}</button>{/each}
              </div>
              <span class="grabado">Parpadea</span>
              <div class="chips">{#each PARPADEOS as m}<button class="tecla chica" aria-pressed={extra.parpadeo === m} onclick={() => (extra.parpadeo = m)}>{m}</button>{/each}</div>
            </div>
          {:else}
            <div class="perillas">
              {#each actual.perillas as [p, t]}
                <Perilla nombre={t} valor={valor(claveDe(p, !!actual.ojo))} min={LIMITES[p][0]} max={LIMITES[p][1]} paso={0.01} tamano={58} alCambiar={(v) => poner(p, v, !!actual.ojo)} />
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </section>
  {/if}
</div>

<style>
  .expresiones { display: grid; grid-template-columns: 236px minmax(0, 1fr); gap: 22px; align-items: start; }
  .galeria { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; max-height: calc(100vh - 140px); overflow-y: auto; padding: 4px 6px 4px 2px;
    position: sticky; top: 16px; scrollbar-width: thin; }
  .cara { display: flex; flex-direction: column; gap: 6px; padding: 6px; border-radius: 14px; cursor: pointer; text-align: left; color: var(--gris-claro); }
  .cara:hover { background: rgba(255,255,255,.05); }
  .cara.elegida { background: var(--chapa); box-shadow: 0 0 0 2px var(--brasa); color: var(--c-papel); }
  .mini { display: block; aspect-ratio: 320 / 172; border-radius: 9px; overflow: hidden; background: #050505; box-shadow: inset 0 0 0 1px rgba(255,255,255,.06); }
  .mini.redonda { aspect-ratio: 1; border-radius: 50%; }
  .mini img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .mini.dibujo { display: grid; place-items: center; }
  .mini.dibujo img { width: auto; height: auto; max-width: 70%; max-height: 80%; object-fit: contain; }
  .n { display: flex; align-items: center; gap: 6px; font: 700 11.5px/1.1 var(--marca); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tuya { width: 7px; height: 7px; border-radius: 50%; background: var(--led); box-shadow: 0 0 6px var(--led); flex: none; }
  .nueva .mas { display: grid; place-items: center; aspect-ratio: 320 / 172; border-radius: 9px; border: 2px dashed #3a3632; font: 700 26px/1 var(--marca); color: var(--brasa); }
  .mesa { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
  .pantalla { padding: 14px; border-radius: 30px; background: linear-gradient(180deg, #2c2925, #1c1a17); box-shadow: inset 0 1px 0 rgba(255,255,255,.1), 0 30px 50px -30px rgba(0,0,0,.9); }
  .pantalla canvas { display: block; width: 100%; aspect-ratio: 320 / 172; border-radius: 18px; background: #050505; }
  .pantalla.redonda { width: min(100%, 420px); align-self: center; border-radius: 50%; }
  .pantalla.redonda canvas { aspect-ratio: 1; border-radius: 50%; }
  .barra { display: flex; gap: 12px; align-items: stretch; flex-wrap: wrap; }
  .nombre { flex: 1; min-width: 200px; padding: 10px 16px; gap: 6px; }
  .nombre .campo { font-size: 22px; }
  .barra .tecla { align-self: center; }
  .tablero { display: flex; flex-direction: column; gap: 16px; padding: 18px; }
  .solapas { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 10px; }
  .solapa { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px 4px 10px; border-radius: 14px; cursor: pointer; color: #1a1714;
    font: 900 10.5px/1 var(--marca); letter-spacing: .04em; text-transform: uppercase; text-align: center;
    background: linear-gradient(180deg, #fbf9f5, #e6e0d6); box-shadow: inset 0 1px 0 #fff, 0 5px 0 #8f887c, 0 9px 14px -6px rgba(80,20,0,.5); transition: transform .08s, box-shadow .08s; }
  .solapa.abajo { transform: translateY(4px); color: var(--c-papel); background: linear-gradient(180deg, #2a2622, #151310); box-shadow: 0 1px 0 #000; }
  .solapa.abajo svg { color: var(--brasa); }
  .controles { display: flex; flex-direction: column; gap: 18px; padding: 20px; }
  .ojos { display: flex; gap: 8px; flex-wrap: wrap; }
  .ojo { display: inline-flex; align-items: center; gap: 10px; min-height: 40px; padding: 0 14px; border-radius: 11px; cursor: pointer; background: #0e0d0b; color: var(--gris);
    font: 800 11px/1 var(--marca); letter-spacing: .08em; text-transform: uppercase; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .ojo.sel { color: var(--c-papel); box-shadow: inset 0 2px 6px rgba(0,0,0,.8), 0 0 0 2px var(--brasa); }
  .pastillas { display: inline-flex; gap: 3px; }
  .pastillas i { width: 6px; height: 12px; border-radius: 3px; background: #3a3632; }
  .pastillas i.on { background: var(--led); box-shadow: 0 0 6px var(--led); }
  .perillas { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: 22px 10px; }
  .mirada { display: grid; grid-template-columns: 200px minmax(0, 1fr); gap: 24px; align-items: center; }
  .gestos { display: flex; flex-direction: column; gap: 10px; }
  .chips { display: flex; flex-wrap: wrap; gap: 10px 8px; padding-bottom: 6px; }
  .glifo { min-width: 40px; font-size: 16px; letter-spacing: 0; }
  @media (max-width: 1100px) {
    .expresiones { grid-template-columns: 1fr; }
    .galeria { position: static; display: flex; overflow-x: auto; max-height: none; }
    .cara { flex: none; width: 120px; }
    .solapas { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  }
  @media (max-width: 560px) { .mirada { grid-template-columns: 1fr; } }
</style>
