<script lang="ts">
  // Reflejos: un patchbay. A la izquierda lo que le pasa, a la derecha cómo
  // reacciona; un cable los une. Se guardan como reglas propias simples
  // ({si, entonces: ["reflejo:x"]}), que se suman a las de fábrica. Las reglas
  // más complejas se editan en el Laboratorio y acá no se tocan.
  // Abajo, los avisos del celu: cómo reacciona a una notificación o llamada
  // según la app y el contacto (config.avisos).
  import { usarBicho } from '$lib/bicho.svelte';
  import {
    bytes, COMPORTAMIENTOS_BASE, cortarBytes, EXPRESIONES_BASE, MAX_AVISOS, MAX_DECIR_AVISO, MAX_NOMBRE_AVISO, MAX_REGLAS, NOMBRES_EVENTO,
    type Aviso, type Regla,
  } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const ENTRADAS = ['tacto', 'caricia', 'doble-toque', 'muchos-toques', 'levantado', 'sacudida-fuerte', 'boca-abajo', 'caida', 'volvio'];
  const SALIDAS: [string, string][] = [
    ['contento', 'Contento'], ['enamorado', 'Enamorado'], ['risa', 'Se ríe'], ['guino', 'Guiña'], ['sorpresa', 'Sorpresa'],
    ['verguenza', 'Vergüenza'], ['mareado', 'Mareado'], ['enojado', 'Se enoja'], ['ofendido', 'Ofendido'],
  ];
  const FILA = 52;
  const ALTO = ENTRADAS.length * FILA;

  const simple = (r: Regla) => ENTRADAS.includes(r.si) && !r.y && r.entonces.length === 1 && r.entonces[0].startsWith('reflejo:') &&
    SALIDAS.some(([s]) => `reflejo:${s}` === r.entonces[0]);
  const cables = $derived(
    Object.fromEntries((b.config.reglas ?? []).filter(simple).map((r) => [r.si, r.entonces[0].slice(8)])) as Record<string, string>,
  );
  const sinMovimiento = $derived(!modelo.capacidades.has('movimiento'));
  let elegida = $state<string | null>(null);

  function conectar(entrada: string, salida: string | null) {
    g.cambiar((c) => {
      const otras = (c.reglas ?? []).filter((r) => !(simple(r) && r.si === entrada));
      c.reglas = salida ? [...otras, { si: entrada, entonces: [`reflejo:${salida}`] }].slice(0, MAX_REGLAS) : otras;
      if (!c.reglas.length) delete c.reglas;
    });
    elegida = null;
  }
  function tocarSalida(s: string) {
    if (!elegida) return;
    conectar(elegida, cables[elegida] === s ? null : s);
  }
  const y = (i: number) => i * FILA + FILA / 2;
  const necesitaMovimiento = (e: string) => ['levantado', 'sacudida-fuerte', 'boca-abajo', 'caida'].includes(e);

  // Avisos del celu.
  const avisos = $derived(b.config.avisos ?? []);
  const caras = $derived([...new Set([...Object.keys(EXPRESIONES_BASE), ...Object.keys(b.config.expresiones ?? {})])].sort());
  const comportamientos = $derived([...new Set([...Object.keys(COMPORTAMIENTOS_BASE), ...Object.keys(b.config.comportamientos ?? {})])].sort());
  function ponerAviso(i: number, f: (a: Aviso) => void) {
    g.cambiar((c) => {
      const l = [...(c.avisos ?? [])];
      const a = { ...l[i] };
      f(a);
      for (const k of ['app', 'contacto', 'expresion', 'comportamiento', 'decir'] as const) if (!a[k]) delete a[k];
      if (a.sonido !== false) delete a.sonido;
      l[i] = a;
      c.avisos = l;
    });
  }
  function sumarAviso() {
    g.cambiar((c) => { c.avisos = [...(c.avisos ?? []), { tipo: 'notificacion' as const, expresion: 'sorpresa', decir: '{app}' }].slice(0, MAX_AVISOS); });
  }
  function borrarAviso(i: number) {
    g.cambiar((c) => { c.avisos = (c.avisos ?? []).filter((_, j) => j !== i); if (!c.avisos.length) delete c.avisos; });
  }
  const nombre = (v: string) => cortarBytes(v.trim(), MAX_NOMBRE_AVISO);
  const decir = (v: string) => cortarBytes(v.replace(/\s+/g, ' ').trim(), MAX_DECIR_AVISO);
</script>

<div class="reflejos">
  <span class="grabado">{elegida ? `${NOMBRES_EVENTO[elegida]} → ¿qué hace?` : 'Uní un punto con otro'}</span>
  <div class="bahia">
    <div class="columna" role="listbox" aria-label="Cuando…">
      {#each ENTRADAS as e}
        {@const off = sinMovimiento && necesitaMovimiento(e)}
        <button class="jack izq" role="option" aria-selected={elegida === e} class:elegida={elegida === e} class:conectada={!!cables[e]} disabled={off}
          onclick={() => (elegida = elegida === e ? null : e)}>
          <span class="nombre">{NOMBRES_EVENTO[e]}</span><span class="toma"></span>
        </button>
      {/each}
    </div>
    <svg class="cables" viewBox="0 0 100 {ALTO}" preserveAspectRatio="none" aria-hidden="true" style="height: {ALTO}px">
      {#each ENTRADAS as e, i}
        {#if cables[e]}
          {@const j = SALIDAS.findIndex(([s]) => s === cables[e])}
          <path d="M0 {y(i)} C 45 {y(i) + 26}, 55 {y(j) + 26}, 100 {y(j)}" class:viva={elegida === e} vector-effect="non-scaling-stroke" />
        {/if}
      {/each}
    </svg>
    <div class="columna" role="listbox" aria-label="…reacciona">
      {#each SALIDAS as [s, t]}
        <button class="jack der" role="option" aria-selected={!!elegida && cables[elegida] === s} class:conectada={Object.values(cables).includes(s)}
          disabled={!elegida} onclick={() => tocarSalida(s)}>
          <span class="toma"></span><span class="nombre">{t}</span>
        </button>
      {/each}
    </div>
  </div>
  {#if sinMovimiento}<p class="texto">Este modelo no siente el movimiento.</p>{/if}

  <div class="celu">
    <div class="titulo">
      <span class="grabado">Del celu · {avisos.length}/{MAX_AVISOS}</span>
      <button class="tecla negra chica" disabled={avisos.length >= MAX_AVISOS} onclick={sumarAviso}>+ Aviso</button>
    </div>
    {#each avisos as a, i (i)}
      <div class="aviso-celu visor">
        <div class="fila">
          <div class="tipo" role="group" aria-label="Tipo">
            {#each [['notificacion', 'Mensaje'], ['llamada', 'Llamada']] as [t, n]}
              <button class="tecla chica" class:naranja={a.tipo === t} aria-pressed={a.tipo === t} onclick={() => ponerAviso(i, (x) => (x.tipo = t as Aviso['tipo']))}>{n}</button>
            {/each}
          </div>
          <input class="campo claro" aria-label="App" placeholder="App" value={a.app ?? ''} onchange={(e) => { const v = nombre(e.currentTarget.value); e.currentTarget.value = v; ponerAviso(i, (x) => (x.app = v)); }} />
          <input class="campo claro" aria-label="Contacto" placeholder="Contacto" value={a.contacto ?? ''} onchange={(e) => { const v = nombre(e.currentTarget.value); e.currentTarget.value = v; ponerAviso(i, (x) => (x.contacto = v)); }} />
          <button class="tecla negra cuadrada" aria-label="Borrar este aviso" onclick={() => borrarAviso(i)}>✕</button>
        </div>
        <div class="fila">
          <span class="flecha" aria-hidden="true">→</span>
          <select class="campo claro" aria-label="Cara" value={a.expresion ?? ''} onchange={(e) => { const v = e.currentTarget.value; ponerAviso(i, (x) => (x.expresion = v)); }}>
            <option value="">Su cara</option>{#each caras as c}<option value={c}>{c}</option>{/each}
          </select>
          <select class="campo claro" aria-label="Qué hace" value={a.comportamiento ?? ''} onchange={(e) => { const v = e.currentTarget.value; ponerAviso(i, (x) => (x.comportamiento = v)); }}>
            <option value="">Nada más</option>{#each comportamientos as c}<option value={c}>{c.replaceAll('-', ' ')}</option>{/each}
          </select>
          <button class="llave" aria-pressed={a.sonido !== false} onclick={() => ponerAviso(i, (x) => (x.sonido = x.sonido === false))}><span class="led"></span>Sonido</button>
        </div>
        <div class="fila">
          <span class="flecha" aria-hidden="true">“</span>
          <input class="campo claro" aria-label="Qué dice" placeholder="{'{contacto}'} te escribe" value={a.decir ?? ''}
            oninput={(e) => { const v = cortarBytes(e.currentTarget.value, MAX_DECIR_AVISO); if (v !== e.currentTarget.value) e.currentTarget.value = v; }}
            onchange={(e) => { const v = decir(e.currentTarget.value); ponerAviso(i, (x) => (x.decir = v)); }} />
          <span class="cuenta">{bytes(a.decir ?? '')}/{MAX_DECIR_AVISO}</span>
          {#each ['{app}', '{contacto}'] as f}
            <button type="button" class="ficha" disabled={bytes((a.decir ?? '') + f) > MAX_DECIR_AVISO} onclick={() => ponerAviso(i, (x) => (x.decir = decir(`${x.decir ?? ''} ${f}`)))}>{f}</button>
          {/each}
        </div>
      </div>
    {:else}
      <p class="vacio">Mamá en WhatsApp → enamorado</p>
    {/each}
  </div>
</div>

<style>
  .reflejos { display: flex; flex-direction: column; gap: 14px; }
  .bahia { display: grid; grid-template-columns: minmax(0, 1fr) minmax(60px, 1fr) minmax(0, 1fr); align-items: start; padding: 16px;
    border-radius: 22px; background: #1c1a17; box-shadow: inset 0 3px 12px rgba(0,0,0,.8); }
  .columna { display: flex; flex-direction: column; }
  .jack { display: flex; align-items: center; gap: 10px; height: 52px; padding: 0 4px; border: 0; background: none; cursor: pointer;
    color: #b9b1a6; font: 700 14px/1.15 var(--marca); text-align: left; }
  .jack.izq { justify-content: flex-end; text-align: right; }
  .jack:disabled { cursor: default; }
  .jack.izq:disabled { opacity: .35; }
  .jack:not(:disabled):hover .nombre, .jack.elegida .nombre, .jack.conectada .nombre { color: #f3eee8; }
  .nombre { min-width: 0; }
  .toma { flex: none; width: 26px; height: 26px; border-radius: 50%; background: radial-gradient(circle, #050505 34%, #6b645b 36%, #3a3632 62%, #24211e 64%);
    box-shadow: 0 2px 0 rgba(255,255,255,.08), inset 0 1px 2px rgba(0,0,0,.8); transition: box-shadow .15s; }
  .conectada .toma { background: radial-gradient(circle, #ff8a5c 30%, #6b645b 34%, #3a3632 62%, #24211e 64%); }
  .elegida .toma { box-shadow: 0 0 0 3px #ff8a5c, 0 0 16px #ff6a33; }
  .jack.der:not(:disabled) .toma { box-shadow: 0 0 0 2px rgba(255,138,92,.35); }
  .cables { width: 100%; overflow: visible; }
  .cables path { fill: none; stroke: #ff6a33; stroke-width: 5; stroke-linecap: round; filter: drop-shadow(0 4px 3px rgba(0,0,0,.6)); }
  .cables path.viva { stroke: #ffb347; }
  .celu { display: flex; flex-direction: column; gap: 10px; padding-top: 8px; }
  .titulo { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  .aviso-celu { gap: 10px; padding: 12px 12px 12px 16px; }
  .fila { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .fila .campo { flex: 1 1 120px; min-width: 0; font-size: 17px; border-bottom: 1px solid #2c2925; }
  .tipo { display: flex; gap: 6px; padding-bottom: 4px; }
  .flecha { width: 18px; color: var(--brasa); font: 900 18px/1 var(--marca); text-align: center; }
  .cuenta { flex: none; font: 800 11px/1 var(--marca); color: #5a524a; font-variant-numeric: tabular-nums; }
  .ficha { padding: 7px 10px; border: 0; border-radius: 8px; cursor: pointer; background: var(--chapa); color: var(--brasa);
    font: 800 12px/1 var(--marca); box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 3px 0 #000; }
  .ficha:active { transform: translateY(2px); box-shadow: 0 1px 0 #000; }
  .ficha:disabled { opacity: .4; cursor: default; }
  .vacio { margin: 0; padding: 10px 4px; font: 800 11px/1 var(--marca); letter-spacing: .14em; text-transform: uppercase; color: rgba(26,23,20,.6); }
  @media (max-width: 560px) { .jack { font-size: 12px; } .bahia { padding: 10px 6px; grid-template-columns: minmax(0, 1fr) 36px minmax(0, 1fr); } }
</style>
