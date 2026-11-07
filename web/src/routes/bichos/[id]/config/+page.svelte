<script lang="ts">
  import { usarBicho } from '$lib/bicho.svelte';
  import { clonar, MODOS, NOMBRES_PRESET, NOMBRES_RASGO, PRESETS, RASGOS, type Config } from '$lib/formato';
  import { nucleoEditor } from '$lib/nucleo';
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import { pantallaDe } from '$lib/tipos';

  const b = usarBicho();
  let c = $state<Config>(clonar($state.snapshot(b.config)) as Config);
  let version = $state(b.versionConfig);
  let error = $state('');
  let ok = $state('');
  let guardando = $state(false);
  let avanzado = $state(false);
  let json = $state('');
  let errorJson = $state('');

  // Si llega una versión nueva (de otro celu, o del bicho) y no hay cambios sin guardar, se toma.
  $effect(() => {
    if (b.versionConfig !== version && !sucio) {
      c = clonar($state.snapshot(b.config)) as Config;
      version = b.versionConfig;
    }
  });
  const sucio = $derived(JSON.stringify(c) !== JSON.stringify(b.config));

  // Horario (v2). Una config v1 con "sueno" (HH:MM-HH:MM, cuándo duerme) se pasa al horario.
  const sueno0 = b.config.personalidad?.sueno;
  let horario = $state({
    despierta: b.config.horario?.despierta ?? (sueno0 ? sueno0.slice(6) : '08:00'),
    duerme: b.config.horario?.duerme ?? (sueno0 ? sueno0.slice(0, 5) : '23:30'),
  });
  function ponerHorario() {
    c.horario = { despierta: horario.despierta, duerme: horario.duerme };
    if (c.personalidad?.sueno) { const { sueno: _, ...resto } = c.personalidad; c.personalidad = resto; }
  }
  const rasgo = (r: string) => (c.personalidad?.[r as keyof typeof PRESETS.equilibrado] as number | undefined) ??
    (PRESETS[c.personalidad?.preset ?? 'equilibrado']?.[r as keyof typeof PRESETS.equilibrado] as number) ?? 0.5;
  function ponerPreset(p: string) {
    // Un preset pone los seis rasgos (después se pueden ajustar uno por uno).
    c.personalidad = { ...(p ? { preset: p } : {}), ...(PRESETS[p] ?? {}) };
  }

  const opcionesLista = $derived([
    ...b.nombresCaras.map((n) => ({ valor: `cara:${n}`, texto: `Cara: ${n}` })),
    ...Object.keys(c.animaciones ?? {}).map((n) => ({ valor: `animacion:${n}`, texto: `Animación: ${n}` })),
    ...Array.from({ length: 8 }, (_, i) => ({ valor: `imagen:${i}`, texto: `Imagen del espacio ${i}` })),
  ]);
  let agregar = $state('cara:neutral');

  function mover(i: number, d: number) {
    const l = [...(c.lista ?? [])];
    const j = i + d;
    if (j < 0 || j >= l.length) return;
    [l[i], l[j]] = [l[j], l[i]];
    c.lista = l;
  }

  async function guardar() {
    guardando = true;
    error = ok = '';
    const limpia = clonar($state.snapshot(c)) as Config;
    if (limpia.lista && limpia.lista.length === 0) delete limpia.lista;
    // La web guarda siempre en v2 (el motor de vida); la v1 se sigue leyendo.
    limpia.version = 2;
    if (limpia.dueno && !limpia.dueno.nombre && !limpia.dueno.cumple) delete limpia.dueno;
    if (limpia.dueno) for (const k of ['nombre', 'cumple'] as const) if (!limpia.dueno[k]) delete limpia.dueno[k];
    const e = await b.guardar(limpia);
    guardando = false;
    if (e) error = e;
    else {
      version = b.versionConfig;
      ok = b.bicho?.conectado ? 'Guardada: ya se la estoy mandando al bicho.' : 'Guardada: le llega cuando se conecte.';
    }
  }

  function abrirAvanzado() {
    json = JSON.stringify(c, null, 2);
    errorJson = '';
    avanzado = true;
  }

  async function aplicarJson() {
    try {
      const nueva = JSON.parse(json);
      const v = (await nucleoEditor()).validar(nueva);
      if (!v.ok) { errorJson = v.detalle; return; }
      c = nueva;
      avanzado = false;
    } catch (e) {
      errorJson = 'JSON inválido: ' + (e as Error).message;
    }
  }

  const pantalla = $derived(b.bicho ? pantallaDe(b.bicho) : { ancho: 320, alto: 172, redonda: false });
  let errorVista = $state<string | null>(null);
</script>

<div class="dos">
  <div class="grilla">
    <section class="tarjeta grilla">
      <h2>General</h2>
      <div class="campos">
        <label><span class="etiqueta">Nombre</span><input maxlength="24" bind:value={c.nombre} placeholder="bicho" /></label>
        <label><span class="etiqueta">Modo</span>
          <select bind:value={c.modo}>{#each MODOS as m}<option value={m}>{m}</option>{/each}</select>
        </label>
      </div>
    </section>

    <section class="tarjeta grilla">
      <h2>Ojos</h2>
      <div class="campos">
        <label><span class="etiqueta">Color</span>
          <div class="fila"><input type="color" value={c.cara?.color ?? '#FF6A33'} oninput={(e) => (c.cara = { ...c.cara, color: e.currentTarget.value.toUpperCase() })} />
            <span class="mono">{c.cara?.color ?? 'el de cada cara'}</span>
            {#if c.cara?.color}<button class="chico" onclick={() => { const { color: _, ...resto } = c.cara ?? {}; c.cara = resto; }}>Quitar</button>{/if}
          </div>
        </label>
        <label><span class="etiqueta">Tamaño · {(c.cara?.tamano ?? 1).toFixed(2)}</span>
          <input type="range" min="0.5" max="1.5" step="0.05" value={c.cara?.tamano ?? 1} oninput={(e) => (c.cara = { ...c.cara, tamano: +e.currentTarget.value })} />
        </label>
      </div>
    </section>

    <section class="tarjeta grilla">
      <h2>Personalidad</h2>
      <p class="nota">Cambia qué le da ganas de hacer, qué tan rápido se mueve y cómo reacciona. Con el tiempo, el trato lo cambia un poco (hasta un 10 %).</p>
      <div class="presets" role="group" aria-label="Presets">
        {#each Object.keys(PRESETS) as p}
          <button class="chico" aria-pressed={c.personalidad?.preset === p} onclick={() => ponerPreset(p)}>{NOMBRES_PRESET[p] ?? p}</button>
        {/each}
      </div>
      <div class="campos">
        {#each RASGOS as r}
          <label><span class="etiqueta">{NOMBRES_RASGO[r][0]} · {rasgo(r).toFixed(2)}</span>
            <input type="range" min="0" max="1" step="0.05" value={rasgo(r)}
              oninput={(e) => (c.personalidad = { ...c.personalidad, [r]: +e.currentTarget.value })} />
            <span class="nota">Alto: {NOMBRES_RASGO[r][1].toLowerCase()}. Bajo: {NOMBRES_RASGO[r][2].toLowerCase()}.</span>
          </label>
        {/each}
      </div>
    </section>

    <section class="tarjeta grilla">
      <h2>Su día</h2>
      <p class="nota">Vive en tu horario: fuera de él, cuenta ovejas y duerme (y sueña). A la mañana se despierta solo.</p>
      <div class="campos">
        <label><span class="etiqueta">Se despierta</span><input type="time" bind:value={horario.despierta} onchange={ponerHorario} /></label>
        <label><span class="etiqueta">Se duerme</span><input type="time" bind:value={horario.duerme} onchange={ponerHorario} /></label>
        <label><span class="etiqueta">Tu nombre</span><input maxlength="24" value={c.dueno?.nombre ?? ''} placeholder="(opcional)"
          oninput={(e) => (c.dueno = { ...c.dueno, nombre: e.currentTarget.value || undefined })} /></label>
        <label><span class="etiqueta">Tu cumpleaños (MM-DD)</span><input maxlength="5" pattern="[0-1][0-9]-[0-3][0-9]" value={c.dueno?.cumple ?? ''} placeholder="03-14"
          oninput={(e) => (c.dueno = { ...c.dueno, cumple: e.currentTarget.value || undefined })} /></label>
        <label><span class="etiqueta">El cumpleaños del bicho (MM-DD)</span><input maxlength="5" value={c.cumple ?? ''} placeholder="11-07"
          oninput={(e) => (c.cumple = e.currentTarget.value || undefined)} /></label>
        <label><span class="etiqueta">Brillo máximo · {Math.round((c.limites_marca?.brillo_max ?? 1) * 100)} %</span>
          <input type="range" min="0.2" max="1" step="0.05" value={c.limites_marca?.brillo_max ?? 1}
            oninput={(e) => (c.limites_marca = { ...c.limites_marca, brillo_max: +e.currentTarget.value })} /></label>
      </div>
    </section>

    <section class="tarjeta grilla">
      <h2>Lista</h2>
      <p class="nota">Lo que muestra, en orden: en el broche, un toque pasa al siguiente. Sin lista, la mascota se maneja sola y el broche recorre las caras.</p>
      <ol class="lista">
        {#each c.lista ?? [] as el, i}
          <li class="fila">
            <span class="mono">{el}</span>
            <span class="fila acciones">
              <button class="icono" aria-label="Subir" onclick={() => mover(i, -1)} disabled={i === 0}>↑</button>
              <button class="icono" aria-label="Bajar" onclick={() => mover(i, 1)} disabled={i === (c.lista?.length ?? 0) - 1}>↓</button>
              <button class="icono peligro" aria-label="Quitar" onclick={() => (c.lista = c.lista?.filter((_, k) => k !== i))}>×</button>
            </span>
          </li>
        {/each}
      </ol>
      <div class="fila">
        <select bind:value={agregar} style="flex: 1">{#each opcionesLista as o}<option value={o.valor}>{o.texto}</option>{/each}</select>
        <button onclick={() => (c.lista = [...(c.lista ?? []), agregar])} disabled={(c.lista?.length ?? 0) >= 16}>Agregar</button>
      </div>
      <label><span class="etiqueta">Pasa sola al siguiente cada (minutos, 0 = nunca)</span>
        <input type="number" min="0" max="1440" value={c.rotar ?? 0} oninput={(e) => (c.rotar = Math.max(0, Math.min(1440, Math.round(+e.currentTarget.value))))} />
      </label>
    </section>
  </div>

  <aside class="lateral">
    <div class="tarjeta grilla">
      <p class="etiqueta">Así queda</p>
      <Pantalla config={c} redonda={pantalla.redonda} ancho={pantalla.redonda ? 200 : 300} alError={(e) => (errorVista = e)} />
      {#if errorVista}<p class="aviso-caja error">{errorVista}</p>{/if}
      <div class="fila">
        <button class="primario" onclick={guardar} disabled={guardando || !sucio}>{guardando ? 'Guardando…' : 'Guardar'}</button>
        <button onclick={() => (c = clonar($state.snapshot(b.config)) as Config)} disabled={!sucio}>Descartar</button>
        <button onclick={abrirAvanzado}>JSON</button>
      </div>
      {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
      {#if ok && !sucio}<p class="aviso-caja ok">{ok}</p>{/if}
      <p class="nota">v{b.versionConfig}{b.autorConfig ? ` · la última la hizo ${b.autorConfig === 'bicho' ? 'el bicho (cambió cerca, por BLE)' : b.autorConfig}` : ''}</p>
    </div>
  </aside>
</div>

{#if avanzado}
  <div class="fondo" role="presentation" onclick={(e) => e.target === e.currentTarget && (avanzado = false)}>
    <div class="tarjeta dialogo grilla" role="dialog" aria-modal="true" aria-label="Editar el JSON">
      <h2>La config en JSON</h2>
      <p class="nota">Es el documento de <code>shared/formato/config.schema.json</code>. Se valida con el mismo núcleo del bicho.</p>
      <textarea bind:value={json} spellcheck="false" rows="22"></textarea>
      {#if errorJson}<p class="aviso-caja error">{errorJson}</p>{/if}
      <div class="fila" style="justify-content: flex-end"><button onclick={() => (avanzado = false)}>Cancelar</button><button class="primario" onclick={aplicarJson}>Usar este</button></div>
    </div>
  </div>
{/if}

<style>
  .dos { display: grid; grid-template-columns: 1fr 360px; gap: 20px; align-items: start; }
  .lateral { position: sticky; top: 84px; }
  .lista { margin: 0; padding: 0; list-style: none; display: grid; gap: 6px; counter-reset: n; }
  .lista li { justify-content: space-between; border: 1px solid var(--linea); border-radius: 10px; padding: 6px 6px 6px 12px; }
  .acciones { gap: 4px; }
  .fondo { position: fixed; inset: 0; background: rgba(11, 10, 9, 0.5); display: grid; place-items: center; padding: 16px; z-index: 50; }
  .dialogo { max-width: 760px; width: 100%; max-height: 92vh; overflow: auto; }
  textarea { font-size: 13px; min-height: 420px; }
  @media (max-width: 960px) { .dos { grid-template-columns: 1fr; } .lateral { position: static; } }
  .presets { display: flex; flex-wrap: wrap; gap: 6px; }
</style>
