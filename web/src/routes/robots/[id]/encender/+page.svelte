<script lang="ts">
  // Encender un robot: el onboarding. Cuatro pasos (nombre, carácter, su día y
  // una primera imagen), con el robot al lado reaccionando a cada elección.
  import { goto } from '$app/navigation';
  import { api, conBinario, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { clonar, NOMBRES_PRESET, PRESETS, type Config } from '$lib/formato';
  import { prepararImagen } from '$lib/imagenes';
  import { pantallaDe } from '$lib/tipos';
  import Barra from '$lib/consola/Barra.svelte';
  import Producto from '$lib/consola/Producto.svelte';
  import Dial, { DICE_PRESET } from '$lib/consola/Dial.svelte';
  import { aV2 } from '$lib/consola/guardado.svelte';
  import { modeloDe } from '$lib/consola/modelo';

  const b = usarBicho();
  const PASOS = ['Nombre', 'Carácter', 'Su día', 'Foto'];
  const MESES = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];

  let paso = $state(1);
  let nombre = $state('');
  let tuNombre = $state('');
  let dia = $state(0);
  let mes = $state(0);
  let preset = $state('picaro');
  let despierta = $state('08:00');
  let duerme = $state('23:30');
  let foto = $state<File | null>(null);
  let vistaFoto = $state('');
  let protectorFoto = $state(false);
  let error = $state('');
  let encendiendo = $state(false);
  let arranco = false;

  // Cuando carga el robot, se arranca de lo que ya tenga.
  $effect(() => {
    if (arranco || !b.bicho) return;
    arranco = true;
    nombre = b.bicho.apodo || b.config.nombre || '';
    tuNombre = b.config.dueno?.nombre ?? '';
    preset = b.config.personalidad?.preset ?? 'picaro';
    despierta = b.config.horario?.despierta ?? '08:00';
    duerme = b.config.horario?.duerme ?? '23:30';
    if (b.config.dueno?.cumple) { mes = +b.config.dueno.cumple.slice(0, 2); dia = +b.config.dueno.cumple.slice(3, 5); }
  });

  const modelo = $derived(b.bicho ? modeloDe(b.bicho) : null);
  const quien = $derived(nombre.trim() || 'Tu robot');
  // Lo que se ve en la pantalla de al lado: la config que se está armando.
  const borrador = $derived.by(() => {
    const c = clonar($state.snapshot(b.config)) as Config;
    aV2(c);
    if (nombre.trim()) c.nombre = nombre.trim().slice(0, 24);
    c.personalidad = { preset, ...(PRESETS[preset] ?? {}) };
    c.horario = { despierta, duerme };
    return c;
  });
  const cara = $derived(['', 'contento', preset === 'dormilon' ? 'cansado' : preset === 'grunon' ? 'grunon' : preset === 'timido' ? 'verguenza' : preset === 'carinoso' ? 'enamorado' : preset === 'electrico' ? 'emocionado' : preset === 'curioso' ? 'curioso' : 'guino', 'dormido', 'enamorado'][paso]);
  const dice = $derived([
    '',
    `${quien} se está despertando`,
    `${quien} prueba ser ${NOMBRES_PRESET[preset].toLowerCase()}`,
    `De noche, ${quien} duerme`,
    `${quien} ya te quiere`,
  ][paso]);

  const aMin = (h: string) => +h.slice(0, 2) * 60 + +h.slice(3, 5);
  const aHora = (m: number) => { const x = ((m % 1440) + 1440) % 1440; return `${String(Math.floor(x / 60)).padStart(2, '0')}:${String(x % 60).padStart(2, '0')}`; };
  function mover(cual: 'despierta' | 'duerme', d: number) {
    if (cual === 'despierta') { const n = aHora(aMin(despierta) + d); if (n !== duerme) despierta = n; }
    else { const n = aHora(aMin(duerme) + d); if (n !== despierta) duerme = n; }
  }
  function elegirFoto(f: File | undefined) {
    if (!f) return;
    foto = f;
    if (vistaFoto) URL.revokeObjectURL(vistaFoto);
    vistaFoto = URL.createObjectURL(f);
  }

  function marcarVisto() {
    try { localStorage.setItem(`encendido:${b.id}`, '1'); } catch { /* sin almacenamiento */ }
  }
  function despues() {
    marcarVisto();
    goto(`/robots/${b.id}`);
  }

  async function encender() {
    encendiendo = true;
    error = '';
    try {
      const c = $state.snapshot(borrador) as Config;
      if (tuNombre.trim() || (dia && mes)) {
        c.dueno = { ...(c.dueno ?? {}) };
        if (tuNombre.trim()) c.dueno.nombre = tuNombre.trim().slice(0, 24);
        if (dia && mes) c.dueno.cumple = `${String(mes).padStart(2, '0')}-${String(dia).padStart(2, '0')}`;
      }
      if (nombre.trim()) {
        const r = await api<{ apodo: string }>(`/v1/bichos/${b.id}`, { metodo: 'PATCH', cuerpo: { apodo: nombre.trim().slice(0, 24) } });
        if (b.bicho) b.bicho.apodo = r.apodo;
      }
      if (foto && b.bicho) {
        const p = pantallaDe(b.bicho);
        const { qoi } = await prepararImagen(foto, p.ancho, p.alto, p.redonda);
        await conBinario(`/v1/bichos/${b.id}/imagenes/0`, 'PUT', qoi);
        const l = c.lista?.length ? c.lista : ['cara:neutral'];
        if (protectorFoto) c.protector = { imagen: 0 };
        else if (!l.includes('imagen:0')) c.lista = [...l, 'imagen:0'];
      }
      const e = await b.guardar(c);
      if (e) throw new Error(e);
      marcarVisto();
      goto(`/robots/${b.id}`);
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : e instanceof Error ? e.message : 'No pude encenderlo.';
      encendiendo = false;
    }
  }
</script>

<Barra>
  {#snippet medio()}
    <div class="progreso" role="progressbar" aria-label="Paso {paso} de 4" aria-valuemin={1} aria-valuemax={4} aria-valuenow={paso}>
      {#each PASOS as p, i}<span class="paso"><span class="led" class:on={i < paso}></span><span class="grabado" class:actual={i + 1 === paso}>{p}</span></span>{/each}
    </div>
  {/snippet}
</Barra>

<main class="caja">
  {#if modelo}
    <div class="robot">
      <Producto {modelo} config={borrador} visible={cara} etiqueta={dice} />
    </div>
  {/if}

  <div class="pasos">
    <span class="grabado" style="color: var(--brasa)">Paso {paso} de 4</span>

    {#if paso === 1}
      <h1 class="titular">Ponele<br />nombre.</h1>
      <label class="visor"><span class="grabado">Se llama</span>
        <!-- svelte-ignore a11y_autofocus -->
        <input class="campo grande" type="text" maxlength="24" bind:value={nombre} placeholder="Lucho" autofocus /></label>
      <div class="dos">
        <label class="visor"><span class="grabado">Vos te llamás</span><input class="campo claro" type="text" maxlength="24" bind:value={tuNombre} placeholder="Tu nombre" /></label>
        <div class="visor"><span class="grabado">Tu cumple (te lo festeja)</span>
          <div class="fecha">
            <select class="campo claro" aria-label="Día" bind:value={dia}><option value={0}>—</option>{#each Array.from({ length: 31 }, (_, i) => i + 1) as d}<option value={d}>{d}</option>{/each}</select>
            <select class="campo claro" aria-label="Mes" bind:value={mes}><option value={0}>—</option>{#each MESES as m, i}<option value={i + 1}>{m}</option>{/each}</select>
          </div>
        </div>
      </div>
    {:else if paso === 2}
      <h1 class="titular">¿Cómo<br />es?</h1>
      <div class="placa-naranja caracter">
        <Dial valor={preset} alCambiar={(p) => (preset = p)} />
        <div class="dice">
          <span class="grabado">Carácter</span>
          <b>{NOMBRES_PRESET[preset]}</b>
          <p class="texto">{DICE_PRESET[preset]}</p>
        </div>
      </div>
      <p class="texto">Después lo ajustás rasgo por rasgo, cuando lo conozcas.</p>
    {:else if paso === 3}
      <h1 class="titular">Su día.</h1>
      <div class="dos">
        {#each [['despierta', 'Se despierta', despierta], ['duerme', 'Se duerme', duerme]] as [k, t, h]}
          <div class="visor reloj">
            <div class="lectura"><span class="grabado">{t}</span><span class="digitos">{h}</span></div>
            <div class="mas-menos">
              <button class="tecla negra cuadrada" aria-label="{t}: más tarde" onclick={() => mover(k as 'despierta', 15)}>+</button>
              <button class="tecla negra cuadrada" aria-label="{t}: más temprano" onclick={() => mover(k as 'despierta', -15)}>−</button>
            </div>
          </div>
        {/each}
      </div>
      <p class="texto">De noche cuenta ovejas y sueña; a la mañana se despierta con vos.</p>
    {:else}
      <h1 class="titular">¿Una foto?</h1>
      <label class="bahia" class:llena={!!vistaFoto}>
        <span class="grabado num">Bahía 01</span>
        {#if vistaFoto && foto?.type.startsWith('video/')}<video src={vistaFoto} autoplay muted loop playsinline></video>
        {:else if vistaFoto}<img src={vistaFoto} alt="La foto elegida" />
        {:else}<span class="mas" aria-hidden="true">+</span><span class="leyenda">Una imagen, un GIF o un video</span>{/if}
        <input class="solo-lectores" type="file" accept="image/*,.gif,video/*" onchange={(e) => elegirFoto(e.currentTarget.files?.[0])} />
      </label>
      <label class="llave" aria-pressed={protectorFoto}><input class="solo-lectores" type="checkbox" bind:checked={protectorFoto} /><span class="led"></span>Que sea su protector de pantalla</label>
      <p class="texto">{protectorFoto ? 'Se ve siempre que no pase nada; lo tocás y aparece su cara.' : 'Se va turnando con su cara.'} Es opcional: podés sumar hasta ocho después.</p>
    {/if}

    {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
    <div class="botones">
      {#if paso > 1}<button class="tecla" onclick={() => paso--}>Atrás</button>{/if}
      {#if paso < 4}<button class="tecla naranja ancha" onclick={() => paso++}>Siguiente</button>
      {:else}<button class="tecla naranja ancha" disabled={encendiendo} onclick={encender}>{encendiendo ? 'Encendiendo…' : `Encender a ${quien}`}</button>{/if}
      <button class="tecla negra" onclick={despues}>Más tarde</button>
    </div>
  </div>
</main>

<style>
  main { display: grid; grid-template-columns: minmax(0, 420px) minmax(0, 1fr); gap: 56px; align-items: center; padding-top: 24px; padding-bottom: 56px; min-height: calc(100vh - 100px); }
  .progreso { display: flex; align-items: center; gap: 18px; padding: 12px 18px; border-radius: 14px; background: var(--chapa); box-shadow: inset 0 1px 0 rgba(255,255,255,.06); }
  .paso { display: flex; align-items: center; gap: 8px; }
  .grabado.actual { color: var(--c-papel); }
  .pasos { display: flex; flex-direction: column; gap: 26px; min-width: 0; }
  .campo.grande { font-size: 44px; }
  .dos { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .fecha { display: flex; gap: 10px; }
  .fecha select:first-child { width: 3.4em; flex: none; }
  .caracter { display: grid; grid-template-columns: 300px minmax(0, 1fr); gap: 28px; align-items: center; padding: 20px; }
  .dice { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .dice b { font: 900 clamp(34px, 4vw, 52px)/.95 var(--marca); font-stretch: 120%; letter-spacing: -.04em; text-transform: uppercase; }
  .reloj { flex-direction: row; align-items: center; justify-content: space-between; }
  .lectura { display: flex; flex-direction: column; gap: 10px; }
  .mas-menos { display: flex; flex-direction: column; gap: 10px; }
  .bahia { position: relative; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; min-height: 240px; border-radius: 28px;
    cursor: pointer; overflow: hidden; background: var(--hundido); box-shadow: inset 0 3px 12px rgba(0,0,0,.9), 0 1px 0 rgba(255,255,255,.08); }
  .bahia:focus-within { box-shadow: inset 0 3px 12px rgba(0,0,0,.9), 0 0 0 2px var(--brasa); }
  .bahia img, .bahia video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; }
  .num { position: absolute; left: 20px; top: 18px; z-index: 1; }
  .llena .num { background: rgba(14,13,11,.8); padding: 5px 7px; border-radius: 6px; }
  .mas { display: grid; place-items: center; width: 64px; height: 64px; border-radius: 50%; background: linear-gradient(180deg, #ff8a5c, #f2541a); color: #1a1714;
    font: 700 34px/1 var(--marca); box-shadow: 0 5px 0 #9a3209; }
  .leyenda { font: 800 17px/1.2 var(--marca); }
  .botones { display: flex; gap: 14px; flex-wrap: wrap; }
  .ancha { min-width: 240px; }
  @media (max-width: 1000px) {
    main { grid-template-columns: 1fr; gap: 28px; min-height: 0; }
    .robot { max-width: 320px; width: 100%; margin: 0 auto; }
    .progreso .grabado { display: none; }
  }
  @media (max-width: 640px) {
    .dos, .caracter { grid-template-columns: 1fr; justify-items: center; }
    .dice { text-align: center; align-items: center; }
    .ancha { min-width: 0; flex: 1; }
  }
</style>
