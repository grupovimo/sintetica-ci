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
  import Ajustes from '$lib/consola/modulos/Ajustes.svelte';

  const b = usarBicho();
  const g = usarGuardado();

  const MODULOS = [
    { clave: 'caracter', nombre: 'Carácter', modulo: Caracter },
    { clave: 'cara', nombre: 'Cara', modulo: Cara },
    { clave: 'imagenes', nombre: 'Imágenes', modulo: Imagenes },
    { clave: 'dia', nombre: 'Su día', modulo: Dia },
    { clave: 'recordatorios', nombre: 'Recordatorios', modulo: Recordatorios },
    { clave: 'modos', nombre: 'Modos', modulo: Modos },
    { clave: 'reflejos', nombre: 'Reflejos', modulo: Reflejos },
    { clave: 'integraciones', nombre: 'Integraciones', modulo: Integraciones },
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
    <span class="guardado" class:error={g.estado === 'error'} aria-live="polite"><span class="led" class:on={g.estado === 'guardando'} class:verde={g.estado === 'guardado'}></span>{textoGuardado}</span>
  {/snippet}
</Barra>

{#if b.error}
  <main class="caja"><p class="aviso error"><span class="led"></span>{b.error}</p></main>
{:else if b.bicho && modelo}
  <main class="caja">
    {#if b.aviso}
      <div class="aviso-robot" role="status"><span class="campana" aria-hidden="true">{b.aviso.icono}</span><b>{b.aviso.texto}</b><button class="tecla negra chica" onclick={() => (b.aviso = null)}>OK</button></div>
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
          {#if b.bicho.estado?.fija && !/^(recordatorio|pomodoro|reloj):/.test(b.bicho.estado.expresion)}<button class="tecla negra" disabled={!conectado} onclick={() => hacer({ expresion: '' }, 'Vuelve a ser él')}>Soltar la cara</button>{/if}
        </div>
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
    .teclado { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .modulo .n { display: none; }
  }
</style>
