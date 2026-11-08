<script lang="ts">
  // La consola de un robot: arriba él (la foto real con su cara en vivo), abajo
  // una placa con un teclado de módulos. Todo se guarda solo.
  import { onMount } from 'svelte';
  import { goto, replaceState } from '$app/navigation';
  import { page } from '$app/state';
  import { usarBicho } from '$lib/bicho.svelte';
  import { usarGuardado } from '$lib/consola/guardado.svelte';
  import Barra from '$lib/consola/Barra.svelte';
  import Producto from '$lib/consola/Producto.svelte';
  import { animoDe, hace, modeloDe, NOMBRES_CAPACIDAD, numeroDe, type Capacidad } from '$lib/consola/modelo';
  import Caracter from '$lib/consola/modulos/Caracter.svelte';
  import Cara from '$lib/consola/modulos/Cara.svelte';
  import Imagenes from '$lib/consola/modulos/Imagenes.svelte';
  import Dia from '$lib/consola/modulos/Dia.svelte';
  import Recordatorios from '$lib/consola/modulos/Recordatorios.svelte';
  import Modos from '$lib/consola/modulos/Modos.svelte';
  import Reflejos from '$lib/consola/modulos/Reflejos.svelte';
  import Integraciones from '$lib/consola/modulos/Integraciones.svelte';
  import Voz from '$lib/consola/modulos/Voz.svelte';
  import Habla from '$lib/consola/modulos/Habla.svelte';
  import Jugar from '$lib/consola/modulos/Jugar.svelte';
  import Cuidado from '$lib/consola/modulos/Cuidado.svelte';
  import { api, ErrorApi } from '$lib/api';
  import { bytes, cortarBytes, MAX_TEXTO } from '$lib/formato';
  import Ajustes from '$lib/consola/modulos/Ajustes.svelte';
  import Amigos from '$lib/consola/modulos/Amigos.svelte';
  import Habitos from '$lib/consola/modulos/Habitos.svelte';
  import Agenda from '$lib/consola/modulos/Agenda.svelte';
  import Avisos from '$lib/consola/modulos/Avisos.svelte';
  import { escuchar } from '$lib/eventos.svelte';

  const b = usarBicho();
  const g = usarGuardado();

  const MODULOS = [
    { clave: 'caracter', nombre: 'Carácter', modulo: Caracter },
    { clave: 'cara', nombre: 'Cara', modulo: Cara },
    { clave: 'imagenes', nombre: 'Imágenes', modulo: Imagenes },
    { clave: 'dia', nombre: 'Su día', modulo: Dia },
    { clave: 'cuidado', nombre: 'Cuidado', modulo: Cuidado },
    { clave: 'recordatorios', nombre: 'Recordatorios', modulo: Recordatorios },
    { clave: 'habitos', nombre: 'Hábitos', modulo: Habitos },
    { clave: 'agenda', nombre: 'Agenda', modulo: Agenda },
    { clave: 'modos', nombre: 'Modos', modulo: Modos },
    { clave: 'reflejos', nombre: 'Reflejos', modulo: Reflejos },
    { clave: 'integraciones', nombre: 'Integraciones', modulo: Integraciones },
    { clave: 'amigos', nombre: 'Amigos', modulo: Amigos },
    { clave: 'avisos', nombre: 'Avisos', modulo: Avisos },
    { clave: 'habla', nombre: 'Habla', modulo: Habla },
    { clave: 'jugar', nombre: 'Jugar', modulo: Jugar },
    { clave: 'voz', nombre: 'Voz', modulo: Voz, necesita: ['microfono', 'parlante'] as Capacidad[] },
    { clave: 'ajustes', nombre: 'Ajustes', modulo: Ajustes },
  ];
  let actual = $state(MODULOS.some((m) => m.clave === page.url.searchParams.get('m')) ? page.url.searchParams.get('m')! : 'caracter');
  function elegir(clave: string) {
    actual = clave;
    const u = new URL(page.url);
    u.searchParams.set('m', clave);
    replaceState(u, {});
  }

  const modelo = $derived(b.bicho ? modeloDe(b.bicho) : null);
  const nombre = $derived(b.bicho?.apodo || b.config.nombre || b.bicho?.estado?.nombre || 'Tu robot');
  const conectado = $derived(b.bicho?.conectado ?? false);
  const bloqueado = (m: (typeof MODULOS)[number]) => !!m.necesita && !!modelo && !m.necesita.some((c) => modelo.capacidades.has(c));
  const Modulo = $derived(MODULOS.find((m) => m.clave === actual)!.modulo);

  let aviso = $state('');
  async function hacer(c: Parameters<typeof b.comando>[0], texto: string) {
    const e = await b.comando(c);
    aviso = e ?? texto;
    setTimeout(() => (aviso = ''), 2500);
  }

  // Decile algo: lo muestra en un globo con su cara.
  let frase = $state('');
  let diciendo = $state(false);
  const usados = $derived(bytes(frase));
  async function decir() {
    const t = frase.trim();
    if (!t || diciendo || !conectado) return;
    diciendo = true;
    try {
      await api(`/v1/bichos/${b.id}/decir`, { cuerpo: { texto: t } });
      frase = '';
      aviso = 'Lo dijo';
    } catch (e) {
      aviso = e instanceof ErrorApi ? (e.estado === 409 ? 'No está conectado' : e.estado === 404 ? 'Todavía no sabe hablar' : e.detalle || e.message) : 'No pude mandarlo';
    }
    diciendo = false;
    setTimeout(() => (aviso = ''), 2500);
  }
  // Charla con IA (si la nube la tiene): le escribís y contesta él, en su pantalla y acá.
  let charla = $state<{ disponible: boolean; quedan: number; porDia: number } | null>(null);
  let charlando = $state(false);
  let respuesta = $state<{ yo: string; texto: string; expresion?: string } | null>(null);
  async function charlar() {
    const t = frase.trim();
    if (!t || diciendo || !charla) return;
    diciendo = true;
    try {
      const r = await api<{ texto: string; expresion?: string; enviado: boolean; quedan: number }>(`/v1/bichos/${b.id}/charla`, { cuerpo: { texto: t } });
      respuesta = { yo: t, texto: r.texto, expresion: r.expresion };
      charla.quedan = r.quedan;
      frase = '';
    } catch (e) {
      aviso = e instanceof ErrorApi ? (e.estado === 429 ? 'Por hoy ya charlaron mucho' : e.estado === 503 ? 'Sin charla en esta nube' : e.message) : 'No contestó';
      setTimeout(() => (aviso = ''), 2500);
    }
    diciendo = false;
  }

  // Lo que suena (Spotify o el celu), para verlo al lado de él.
  let suena = $state<{ titulo: string; artista: string } | null>(null);
  onMount(() => {
    api<{ disponible: boolean; quedan: number; porDia: number }>('/v1/charla').then((c) => (charla = c.disponible ? c : null)).catch(() => {});
    api<{ suena?: { sonando: boolean; titulo: string; artista: string } | null }>('/v1/musica').then((m) => (suena = m.suena?.sonando ? m.suena : null)).catch(() => {});
    return escuchar((e) => {
      if (e.tipo !== 'musica' || (e.bicho && e.bicho !== b.id) || e.datos.titulo === undefined) return;
      suena = e.datos.sonando ? { titulo: e.datos.titulo, artista: e.datos.artista ?? '' } : null;
    });
  });
  const MAX_CHARLA = 200;

  async function alarma(accion: 'apagar' | 'posponer') {
    b.aviso = null;
    try { await api(`/v1/bichos/${b.id}/alarma`, { cuerpo: { accion } }); } catch { /* ya se apagó o no está */ }
  }

  // La primera vez que se abre un robot sin bautizar, se enciende (onboarding).
  onMount(() => {
    const t = setInterval(() => {
      if (!b.bicho) return;
      clearInterval(t);
      let visto = false;
      try { visto = localStorage.getItem(`encendido:${b.id}`) === '1'; } catch { /* sin almacenamiento */ }
      if (!visto && !b.config.dueno && !b.config.personalidad?.preset) goto(`/robots/${b.id}/encender`, { replaceState: true });
    }, 100);
    return () => clearInterval(t);
  });

  const textoGuardado = $derived({ listo: 'Al día', guardando: 'Guardando…', guardado: conectado ? 'Guardado y enviado' : 'Guardado · le llega al volver', error: 'No se guardó' }[g.estado]);
</script>

<Barra>
  {#snippet izquierda()}<a class="tecla negra chica" href="/robots">← Mis robots</a>{/snippet}
  {#snippet medio()}
    <span class="guardado" class:error={g.estado === 'error'} aria-live="polite" title={textoGuardado}><span class="led" class:on={g.estado === 'guardando'} class:verde={g.estado === 'guardado'}></span><span>{textoGuardado}</span></span>
  {/snippet}
</Barra>

{#if b.error}
  <main class="caja"><p class="aviso error"><span class="led"></span>{b.error}</p></main>
{:else if b.bicho && modelo}
  <main class="caja">
    {#if b.aviso}
      <div class="aviso-robot" role="status"><span class="campana" aria-hidden="true">{b.aviso.icono}</span><b>{b.aviso.texto}</b>
        {#if b.aviso.despertador && conectado}<button class="tecla chica" onclick={() => alarma('posponer')}>Posponer</button><button class="tecla negra chica" onclick={() => alarma('apagar')}>Apagar</button>
        {:else}<button class="tecla negra chica" onclick={() => (b.aviso = null)}>OK</button>{/if}</div>
    {/if}
    <section class="heroe">
      <div class="foto">
        <Producto {modelo} config={b.config} visible={b.bicho.estado?.expresion ?? ''} apagado={!conectado}
          etiqueta={conectado ? `${nombre} · en vivo` : `Así se ve ${nombre}`} />
      </div>
      <div class="info">
        <span class="grabado"><span class="led" class:verde={conectado}></span>{conectado ? 'Conectado' : `Desconectado · visto ${hace(b.bicho.ultimaConexion)}`} · {modelo.nombre} {numeroDe(b.id)}</span>
        <h1 class="titular">{nombre}</h1>
        <p class="estado">Está <b>{animoDe(b.bicho).toLowerCase()}</b>{#if b.bicho.estado?.bateria != null} · {b.bicho.estado.bateria}% de batería{/if}.</p>
        {#if suena}<p class="suena" aria-label="Suena {suena.titulo}"><span class="eq" aria-hidden="true"><i></i><i></i><i></i></span><b>{suena.titulo}</b>{#if suena.artista}<span>{suena.artista}</span>{/if}</p>{/if}
        <div class="capacidades" aria-label="Lo que trae">
          <span class="grabado">Lo que trae</span>
          <div class="chips">
            {#each Object.entries(NOMBRES_CAPACIDAD) as [c, n]}
              <span class="cap" class:no={!modelo.capacidades.has(c as Capacidad)}><span class="led" class:on={modelo.capacidades.has(c as Capacidad)}></span>{n}</span>
            {/each}
          </div>
        </div>
        <div class="acciones">
          <button class="tecla" disabled={!conectado} onclick={() => hacer({ evento: 'caricia' }, 'Le gustó')}>Acariciar</button>
          <button class="tecla" disabled={!conectado} onclick={() => hacer({ evento: 'nombre' }, 'Te escuchó')}>Llamarlo</button>
          <button class="tecla naranja" disabled={!conectado} onclick={() => hacer({ evento: 'muchos-toques' }, 'Se rió')}>Cosquillas</button>
          <button class="tecla" disabled={!conectado} onclick={() => { elegir('jugar'); document.querySelector('.tablero')?.scrollIntoView({ behavior: 'smooth', block: 'start' }); }}>Jugar</button>
          {#if b.bicho.estado?.fija && !/^(recordatorio|pomodoro|reloj):/.test(b.bicho.estado.expresion)}<button class="tecla negra" disabled={!conectado} onclick={() => hacer({ expresion: '' }, 'Vuelve a ser él')}>Soltar la cara</button>{/if}
        </div>
        <form class="decir visor" class:charla={charlando} onsubmit={(e) => { e.preventDefault(); if (charlando) charlar(); else decir(); }}>
          <div class="cabeza">
            {#if charla}
              <div class="modo" role="radiogroup" aria-label="Modo">
                <button type="button" role="radio" aria-checked={!charlando} onclick={() => (charlando = false)}>Decile algo</button>
                <button type="button" role="radio" aria-checked={charlando} onclick={() => { charlando = true; frase = cortarBytes(frase, MAX_CHARLA); }}>Charlar</button>
              </div>
              {#if charlando}<span class="grabado quedan" aria-label="Quedan {charla.quedan} de {charla.porDia} hoy">{charla.quedan}/{charla.porDia} hoy</span>{/if}
            {:else}<span class="grabado">Decile algo</span>{/if}
          </div>
          {#if charlando && respuesta}
            <div class="charla-ver" aria-live="polite">
              <span class="yo">{respuesta.yo}</span>
              <span class="el">“{respuesta.texto}”</span>
            </div>
          {/if}
          <div class="linea">
            <input class="campo" aria-label={charlando ? 'Charlale' : 'Decile algo'} placeholder={charlando ? '¿Cómo andás?' : conectado ? '¡Hola!' : 'Desconectado'} disabled={!conectado && !charlando} value={frase}
              oninput={(e) => { const v = cortarBytes(e.currentTarget.value, charlando ? MAX_CHARLA : MAX_TEXTO); if (v !== e.currentTarget.value) e.currentTarget.value = v; frase = v; }} />
            {#if !charlando}<span class="cuenta" class:lleno={usados >= MAX_TEXTO} aria-label="{usados} de {MAX_TEXTO}">{usados}/{MAX_TEXTO}</span>{/if}
            <button class="tecla naranja chica" disabled={(!conectado && !charlando) || diciendo || !frase.trim() || (charlando && !charla?.quedan)}>{charlando ? (diciendo ? '…' : 'Charlar') : 'Decir'}</button>
          </div>
        </form>
        {#if aviso}<p class="aviso" aria-live="polite"><span class="led on"></span>{aviso}</p>{/if}
        {#if !conectado}<p class="texto chico">Le llega cuando vuelva.</p>{/if}
      </div>
    </section>

    <section class="placa-naranja tablero">
      <div class="teclado" role="tablist" aria-label="Módulos">
        {#each MODULOS as m, i}
          <button role="tab" class="modulo" class:abajo={actual === m.clave} aria-selected={actual === m.clave} onclick={() => elegir(m.clave)}>
            <span class="arriba"><span class="n">{String(i + 1).padStart(2, '0')}</span><span class="led" class:on={actual === m.clave} class:candado={bloqueado(m)}></span></span>
            <b>{m.nombre}</b>
          </button>
        {/each}
      </div>
      <div class="panel" role="tabpanel">
        {#if g.estado === 'error'}<p class="aviso error"><span class="led"></span>{g.error}</p>{/if}
        {#key actual}<Modulo {modelo} />{/key}
      </div>
    </section>
  </main>
{:else}
  <main class="caja"><p class="grabado">Cargando…</p></main>
{/if}

<style>
  main { display: flex; flex-direction: column; gap: 40px; padding-top: 4px; padding-bottom: 64px; }
  .aviso-robot { display: flex; align-items: center; gap: 14px; padding: 14px 16px 14px 20px; border-radius: 18px; color: #1a1714;
    background: linear-gradient(180deg, #ff8a5c, #f2541a); box-shadow: 0 6px 0 #9a3209, 0 20px 30px -16px rgba(0,0,0,.8); animation: entra .4s cubic-bezier(.3,1.5,.5,1); }
  .aviso-robot b { flex: 1; font: 900 22px/1.1 var(--marca); font-stretch: 112%; text-transform: uppercase; }
  .campana { font-size: 24px; }
  @keyframes entra { from { transform: translateY(-12px) scale(.96); opacity: 0; } }
  .guardado { display: inline-flex; align-items: center; gap: 8px; font: 800 11px/1 var(--marca); letter-spacing: .12em; text-transform: uppercase; color: var(--gris); }
  .guardado.error { color: #ff7a7a; }
  .heroe { display: grid; grid-template-columns: minmax(0, 400px) minmax(0, 1fr); gap: 48px; align-items: center; }
  .info { display: flex; flex-direction: column; gap: 20px; min-width: 0; }
  .info > .grabado { display: inline-flex; align-items: center; gap: 8px; color: var(--gris-claro); }
  .estado { margin: 0; font: 600 20px/1.35 var(--marca); color: #d9d2c6; }
  .estado b { color: var(--brasa); font-weight: 800; }
  .capacidades { display: flex; flex-direction: column; gap: 10px; }
  .chips { display: flex; flex-wrap: wrap; gap: 8px; }
  .cap { display: inline-flex; align-items: center; gap: 8px; min-height: 32px; padding: 0 12px; border-radius: 9px; background: var(--chapa);
    font: 800 11px/1 var(--marca); letter-spacing: .1em; text-transform: uppercase; color: #d9d2c6; box-shadow: inset 0 1px 0 rgba(255,255,255,.06); }
  .cap.no { color: #5a524a; background: transparent; box-shadow: inset 0 0 0 1px #2c2925; }
  .acciones { display: flex; flex-wrap: wrap; gap: 12px; padding-top: 4px; }
  .chico { font-size: 14px; }
  .decir { padding: 12px 12px 12px 18px; gap: 8px; max-width: 520px; }
  .decir .linea { display: flex; align-items: center; gap: 12px; }
  .decir .campo { font-size: 22px; }
  .decir .campo:disabled { cursor: not-allowed; }
  .decir .cabeza { display: flex; align-items: center; justify-content: space-between; gap: 10px; min-height: 22px; }
  .modo { display: inline-flex; gap: 4px; padding: 3px; border-radius: 10px; background: #1c1a17; box-shadow: inset 0 1px 3px rgba(0,0,0,.8); }
  .modo button { padding: 6px 10px; border-radius: 7px; cursor: pointer; font: 800 11px/1 var(--marca); letter-spacing: .12em; text-transform: uppercase; color: var(--gris); }
  .modo button[aria-checked='true'] { color: #1a1714; background: linear-gradient(180deg, #ff8a5c, #f2541a); box-shadow: 0 2px 0 #9a3209; }
  .quedan { font-variant-numeric: tabular-nums; }
  .charla-ver { display: flex; flex-direction: column; gap: 8px; padding: 4px 0; }
  .charla-ver .yo { align-self: flex-end; max-width: 80%; padding: 6px 12px; border-radius: 12px 12px 4px 12px; background: #2c2925; font: 700 14px/1.3 var(--marca); color: var(--gris-claro); overflow-wrap: anywhere; }
  .charla-ver .el { align-self: flex-start; max-width: 90%; padding: 10px 14px; border-radius: 14px 14px 14px 4px; background: #f3eee8; color: #1a1714; font: 800 18px/1.25 var(--marca); overflow-wrap: anywhere; }
  .suena { display: flex; align-items: center; gap: 10px; margin: -8px 0 0; font: 700 14px/1.2 var(--marca); color: var(--gris); min-width: 0; }
  .suena b { color: var(--c-papel); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
  .suena span:not(.eq) { flex: none; max-width: 40%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .eq { display: inline-flex; align-items: flex-end; gap: 2px; height: 14px; flex: none; }
  .eq i { width: 3px; height: 100%; border-radius: 2px; background: var(--led); animation: eq .9s ease-in-out infinite alternate; transform-origin: bottom; }
  .eq i:nth-child(2) { animation-delay: -.3s; } .eq i:nth-child(3) { animation-delay: -.6s; }
  @keyframes eq { from { transform: scaleY(.2); } to { transform: scaleY(1); } }
  .cuenta { flex: none; font: 800 11px/1 var(--marca); color: #5a524a; font-variant-numeric: tabular-nums; }
  .cuenta.lleno { color: var(--brasa); }

  .tablero { display: grid; grid-template-columns: 320px minmax(0, 1fr); gap: 28px; align-items: start; padding: 24px; }
  .teclado { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .modulo { display: flex; flex-direction: column; justify-content: space-between; align-items: flex-start; gap: 14px; min-height: 76px; padding: 11px 12px;
    border: 0; border-radius: 16px; cursor: pointer; text-align: left; color: #1a1714; font-family: var(--marca);
    background: linear-gradient(180deg, #fbf9f5 0%, #e6e0d6 100%);
    box-shadow: inset 0 1px 0 #fff, inset 0 -3px 0 rgba(0,0,0,.1), 0 6px 0 #8f887c, 0 10px 16px -6px rgba(80,20,0,.55); transition: transform .08s, box-shadow .08s; }
  .modulo:active { transform: translateY(5px); box-shadow: inset 0 1px 0 #fff, 0 1px 0 #8f887c; }
  .modulo.abajo { transform: translateY(5px); color: var(--c-papel); background: linear-gradient(180deg, #2a2622 0%, #151310 100%);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.12), 0 1px 0 #000; }
  .modulo .arriba { display: flex; width: 100%; justify-content: space-between; align-items: center; }
  .modulo .n { font: 900 11px/1 var(--marca); opacity: .55; font-variant-numeric: tabular-nums; }
  .modulo b { font: 900 12px/1.1 var(--marca); font-stretch: 105%; letter-spacing: .01em; overflow-wrap: anywhere; text-transform: uppercase; }
  .modulo .led { background: #b9b1a6; }
  .modulo .led.on { background: var(--led); }
  .modulo .led.candado { background: transparent; box-shadow: inset 0 0 0 2px currentColor; opacity: .5; }
  .panel { min-width: 0; display: flex; flex-direction: column; gap: 16px; }

  @media (max-width: 1000px) {
    .heroe { grid-template-columns: 1fr; gap: 28px; }
    .foto { max-width: 420px; width: 100%; margin: 0 auto; }
    .tablero { grid-template-columns: 1fr; padding: 16px; border-radius: 28px; }
    .teclado { grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
    .modulo { min-height: 64px; padding: 9px; gap: 10px; }
    .modulo b { font-size: 10px; }
  }
  @media (max-width: 560px) {
    /* En el celu, el teclado es una tira que se desliza: cada tecla con su nombre entero. */
    .teclado { display: flex; overflow-x: auto; gap: 8px; margin: -6px -16px 0; padding: 6px 16px 12px; scroll-snap-type: x mandatory; scrollbar-width: none; }
    .teclado::-webkit-scrollbar { display: none; }
    .modulo { flex: none; min-width: 112px; scroll-snap-align: start; }
    .modulo b { font-size: 11px; overflow-wrap: normal; white-space: nowrap; }
    .guardado span:not(.led) { display: none; }
  }
</style>
