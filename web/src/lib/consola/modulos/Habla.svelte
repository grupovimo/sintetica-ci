<script lang="ts">
  // Habla: cuánto habla solo (en su pantalla, con un globo) y sus frases
  // propias para cada situación. No hace falta parlante.
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { bytes, cortarBytes, MAX_FRASES, MAX_TEXTO, NOMBRES_SITUACION, type Situacion } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';
  import fabrica from '$formato/vida/frases.json';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const EJEMPLO: Partial<Record<Situacion, string>> = {
    despertar: '¡Arriba, {dueno}!', 'buenas-noches': 'Hasta mañana, {dueno}', volviste: '¡Volviste!', 'te-extrane': 'Te extrañé',
    aburrido: 'Qué embole...', caricia: 'Más, más', saludo: 'Hola, soy {robot}', despertador: '¡Arriba!', 'clima-lluvia': 'Llevá paraguas',
  };

  // Agrupadas, así la lista se lee de un vistazo.
  const GRUPOS: [string, Situacion[]][] = [
    ['Su día', ['despertar', 'buen-dia', 'comiste', 'tardaste', 'a-dormir', 'buenas-noches', 'despertador', 'posponer', 'sueno', 'agenda', 'agenda-manana']],
    ['Con vos', ['saludo', 'volviste', 'te-vas', 'te-extrane', 'caricia', 'aburrido', 'como-estas', 'animo-bien', 'animo-medio', 'animo-mal', 'musica']],
    ['Amigos', ['toque-recibido', 'toque-devuelto']],
    ['Cuidarte', ['agua', 'pausa', 'recordatorio-hecho', 'habito-hecho', 'habito-racha', 'habito-cortado', 'bateria-baja', 'cargando']],
    ['Avisos', ['pago', 'mail', 'github', 'pedido', 'aviso']],
    ['Clima', ['clima-frio', 'clima-calor', 'clima-lluvia', 'clima-lindo']],
    ['Jugar', ['jugar', 'gane', 'perdi', 'empate']],
    ['Fechas', ['cumple-dueno', 'cumple-robot', 'aniversario', 'anio-nuevo', 'navidad', 'dia-del-amigo', 'dia-del-nino', 'hito']],
  ];

  // Las fichas de cada situación: las de siempre y las que usan sus frases de fábrica ({de}, {titulo}, {racha}…).
  const SIEMPRE = ['{dueno}', '{robot}', '{clima}'];
  const situaciones = (fabrica as { situaciones: Record<string, unknown> }).situaciones;
  const fichasDe = (s: Situacion) =>
    [...SIEMPRE, ...new Set([...JSON.stringify(situaciones[s] ?? '').matchAll(/\{[a-z]+\}/g)].map((m) => m[0]).filter((f) => !SIEMPRE.includes(f)))];
  // De ejemplo, la primera de fábrica si no hay una propia.
  const deFabrica = (s: Situacion) => (situaciones[s] as { frases?: { todos?: string[] } } | undefined)?.frases?.todos?.[0];

  const frecuencia = $derived(b.config.habla?.frecuencia ?? 0.5);
  const propias = $derived(b.config.habla?.propias ?? {});
  const total = $derived(Object.values(propias).reduce((n, l) => n + (l?.length ?? 0), 0));
  let situacion = $state<Situacion>('despertar');
  const frases = $derived(propias[situacion] ?? []);
  const nombre = $derived(b.bicho?.apodo || b.config.nombre || 'tu robot');
  const conectado = $derived(b.bicho?.conectado ?? false);

  const tono = (v: number) => (v <= 0.05 ? 'Callado' : v >= 0.8 ? 'Charlatán' : 'Normal');
  const ponerFrecuencia = (v: number) => g.cambiar((c) => { c.habla = { ...(c.habla ?? {}), frecuencia: v }; });

  function cambiarFrases(f: (l: string[]) => void) {
    g.cambiar((c) => {
      const p = { ...(c.habla?.propias ?? {}) };
      const l = [...(p[situacion] ?? [])];
      f(l);
      if (l.length) p[situacion] = l; else delete p[situacion];
      c.habla = { ...(c.habla ?? {}), propias: p };
      if (!Object.keys(p).length) delete c.habla.propias;
    });
  }

  let nueva = $state('');
  let campo: HTMLInputElement;
  const usados = $derived(bytes(nueva));
  function agregar() {
    const t = nueva.trim();
    if (!t || total >= MAX_FRASES) return;
    cambiarFrases((l) => l.push(t));
    nueva = '';
  }
  // Las fichas se meten donde está el cursor.
  function ficha(f: string) {
    const i = campo?.selectionStart ?? nueva.length, j = campo?.selectionEnd ?? i;
    nueva = cortarBytes(nueva.slice(0, i) + f + nueva.slice(j), MAX_TEXTO);
    queueMicrotask(() => { campo?.focus(); campo?.setSelectionRange(i + f.length, i + f.length); });
  }

  let oido = $state('');
  async function probar(t: string) {
    const texto = cortarBytes(t.replaceAll('{dueno}', b.config.dueno?.nombre ?? '').replaceAll('{robot}', nombre).replaceAll('{clima}', '').replace(/\s+/g, ' ').trim(), MAX_TEXTO);
    try {
      await api(`/v1/bichos/${b.id}/decir`, { cuerpo: { texto } });
      oido = 'Lo dijo';
    } catch (e) {
      oido = e instanceof ErrorApi && e.estado === 409 ? 'No está conectado' : 'No pude';
    }
    setTimeout(() => (oido = ''), 2000);
  }
</script>

<div class="habla">
  <div class="arriba">
    <div class="bandeja perilla">
      <Perilla nombre="Cuánto" valor={frecuencia} alCambiar={ponerFrecuencia} mostrar={tono} tamano={72} />
    </div>
    <label class="visor situacion">
      <span class="grabado">Cuando</span>
      <select class="campo claro" value={situacion} onchange={(e) => (situacion = e.currentTarget.value as Situacion)}>
        {#each GRUPOS as [titulo, lista]}
          <optgroup label={titulo}>
            {#each lista as s}<option value={s}>{NOMBRES_SITUACION[s]}{propias[s]?.length ? ` · ${propias[s]!.length}` : ''}</option>{/each}
          </optgroup>
        {/each}
      </select>
      <span class="total" class:lleno={total >= MAX_FRASES}>{total}/{MAX_FRASES}</span>
    </label>
  </div>

  <div class="lista">
    {#each frases as f, i (i + f)}
      <div class="frase visor">
        <span class="globo">“{f}”</span>
        <button class="tecla chica" disabled={!conectado} aria-label="Que lo diga" title="Que lo diga" onclick={() => probar(f)}>▶</button>
        <button class="tecla negra chica" aria-label="Borrar" onclick={() => cambiarFrases((l) => l.splice(i, 1))}>✕</button>
      </div>
    {:else}
      <p class="vacio">Usa las suyas</p>
    {/each}
    {#if oido}<p class="aviso" aria-live="polite"><span class="led on"></span>{oido}</p>{/if}
  </div>

  <form class="nueva visor" onsubmit={(e) => { e.preventDefault(); agregar(); }}>
    <div class="linea">
      <input bind:this={campo} class="campo claro" aria-label="Frase nueva" placeholder={EJEMPLO[situacion] ?? deFabrica(situacion) ?? 'Frase nueva'} disabled={total >= MAX_FRASES}
        value={nueva} oninput={(e) => { const v = cortarBytes(e.currentTarget.value, MAX_TEXTO); if (v !== e.currentTarget.value) e.currentTarget.value = v; nueva = v; }} />
      <span class="cuenta" class:lleno={usados >= MAX_TEXTO}>{usados}/{MAX_TEXTO}</span>
      <button class="tecla naranja chica" disabled={!nueva.trim() || total >= MAX_FRASES}>+ Sumar</button>
    </div>
    <div class="fichas">
      {#each fichasDe(situacion) as f}<button type="button" class="ficha" onclick={() => ficha(f)}>{f}</button>{/each}
    </div>
  </form>
</div>

<style>
  .habla { display: flex; flex-direction: column; gap: 16px; }
  .arriba { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 16px; }
  .perilla { display: grid; place-items: center; padding: 20px 28px; }
  .situacion { justify-content: center; position: relative; }
  .situacion select { font-size: 26px; padding-right: 64px; }
  .total { position: absolute; right: 20px; bottom: 18px; font: 800 11px/1 var(--marca); color: var(--gris); font-variant-numeric: tabular-nums; }
  .total.lleno, .cuenta.lleno { color: var(--brasa); }
  .lista { display: flex; flex-direction: column; gap: 8px; }
  .frase { flex-direction: row; align-items: center; gap: 8px; padding: 8px 8px 8px 18px; }
  .globo { flex: 1; min-width: 0; font: 800 18px/1.25 var(--marca); color: var(--c-papel); overflow-wrap: anywhere; }
  .frase .tecla { width: 40px; padding: 0; }
  .vacio { margin: 0; padding: 14px 4px; font: 800 11px/1 var(--marca); letter-spacing: .14em; text-transform: uppercase; color: rgba(26,23,20,.6); }
  .nueva { padding: 12px 12px 12px 18px; }
  .linea { display: flex; align-items: center; gap: 12px; }
  .linea .campo:disabled { cursor: not-allowed; }
  .cuenta { flex: none; font: 800 11px/1 var(--marca); color: #5a524a; font-variant-numeric: tabular-nums; }
  .fichas { display: flex; flex-wrap: wrap; gap: 8px; }
  .ficha { padding: 7px 10px; border-radius: 8px; cursor: pointer; background: var(--chapa); color: var(--brasa);
    font: 800 12px/1 var(--marca); letter-spacing: .02em; box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 3px 0 #000; }
  .ficha:active { transform: translateY(2px); box-shadow: 0 1px 0 #000; }
  @media (max-width: 560px) { .arriba { grid-template-columns: 1fr; } .situacion select { font-size: 22px; } }
</style>
