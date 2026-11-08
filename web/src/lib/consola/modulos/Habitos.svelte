<script lang="ts">
  // Hábitos: hasta 4. Cada uno con su ícono, la hora a la que pregunta (si
  // tiene) y sus días. La tecla grande lo marca hecho hoy; abajo, la racha y
  // los últimos 60 días en puntitos.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { cortarBytes, ICONOS_HABITO, MAX_HABITOS, MAX_NOMBRE_HABITO, type Habito } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import { ICONO_HABITO, svg } from '../iconos';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  type Vista = { indice: number; nombre: string; icono: string; hora?: string; racha: number; hoy: boolean; dias: Record<string, boolean> };
  const habitos = $derived(b.config.habitos ?? []);
  let vistas = $state<Vista[]>([]);
  let aviso = $state('');
  const DIAS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  const EJEMPLOS: Habito[] = [
    { nombre: 'Agua', icono: 'gota' }, { nombre: 'Leer', icono: 'libro', hora: '22:00' },
    { nombre: 'Moverme', icono: 'pesa', hora: '18:00', dias: 'LMXJV' }, { nombre: 'Meditar', icono: 'zen', hora: '08:00' },
  ];

  async function cargar() {
    try { vistas = (await api<Vista[]>(`/v1/bichos/${b.id}/habitos`)) ?? []; } catch { /* sin nube: solo la config */ }
  }
  onMount(cargar);
  // Cuando se guarda un cambio (un nombre nuevo empieza de cero), se vuelve a mirar.
  $effect(() => { if (g.estado === 'guardado') cargar(); });

  const vista = (i: number) => vistas.find((v) => v.indice === i && v.nombre === habitos[i]?.nombre);

  function poner(i: number, f: (h: Habito) => void) {
    g.cambiar((c) => {
      const l = [...(c.habitos ?? [])];
      const h = { ...l[i] };
      f(h);
      if (!h.hora) delete h.hora;
      if (h.dias !== undefined && (h.dias === '' || h.dias.length === 7)) delete h.dias;
      l[i] = h;
      c.habitos = l;
    });
  }
  function sumar() {
    const usados = new Set(habitos.map((h) => h.icono));
    const e = EJEMPLOS.find((x) => !usados.has(x.icono)) ?? { nombre: 'Hábito', icono: 'estrella' };
    g.cambiar((c) => { c.habitos = [...(c.habitos ?? []), { ...e }]; });
  }
  function quitar(i: number) {
    g.cambiar((c) => { const l = [...(c.habitos ?? [])]; l.splice(i, 1); if (l.length) c.habitos = l; else delete c.habitos; });
  }
  function dia(i: number, l: string) {
    poner(i, (h) => {
      const ya = h.dias || DIAS.join('');
      const nuevo = DIAS.filter((d) => (d === l ? !ya.includes(d) : ya.includes(d))).join('');
      if (nuevo) h.dias = nuevo;
    });
  }

  async function marcar(i: number) {
    const v = vista(i);
    const hecho = !v?.hoy;
    try {
      const r = await api<{ habito: Vista; enviado: boolean }>(`/v1/bichos/${b.id}/habitos/${i}`, { cuerpo: { hecho } });
      const k = vistas.findIndex((x) => x.indice === i);
      if (k >= 0) vistas[k] = r.habito; else vistas.push(r.habito);
      aviso = hecho ? (r.habito.racha >= 3 ? `¡${r.habito.racha} días seguidos!` : '¡Bien ahí!') : 'Desmarcado';
    } catch (e) {
      aviso = e instanceof ErrorApi && e.estado === 404 ? 'Guardando… probá en un segundo' : 'No pude';
    }
    setTimeout(() => (aviso = ''), 2500);
  }

  // Los últimos 60 días, del más viejo al de hoy (en la hora de acá).
  const clave = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  const ULTIMOS = Array.from({ length: 60 }, (_, k) => { const d = new Date(); d.setDate(d.getDate() - (59 - k)); return clave(d); });
</script>

<div class="habitos">
  <div class="grilla">
    {#each habitos as h, i (i)}
      {@const v = vista(i)}
      <div class="habito bandeja">
        <div class="arriba">
          <button class="marcar" class:hecho={v?.hoy} aria-pressed={!!v?.hoy} aria-label="{h.nombre}: {v?.hoy ? 'hecho hoy' : 'marcar hecho'}" onclick={() => marcar(i)}>
            {@html svg(ICONO_HABITO[h.icono ?? 'estrella'], 40)}
            <span class="led" class:on={v?.hoy}></span>
          </button>
          <div class="nombre">
            <label class="visor">
              <span class="solo-lectores">Nombre</span>
              <input class="campo" value={h.nombre} maxlength="16"
                onchange={(e) => { const t = cortarBytes(e.currentTarget.value.trim(), MAX_NOMBRE_HABITO); if (t) poner(i, (x) => (x.nombre = t)); else e.currentTarget.value = h.nombre; }} />
            </label>
            <div class="racha visor" aria-label="Racha: {v?.racha ?? 0} días">
              <span class="digitos chicos">{v?.racha ?? 0}</span><span class="grabado">{(v?.racha ?? 0) === 1 ? 'día' : 'días'}</span>
            </div>
          </div>
          <button class="tecla negra cuadrada chica-c" aria-label="Quitar {h.nombre}" onclick={() => quitar(i)}>✕</button>
        </div>

        <div class="iconos" role="radiogroup" aria-label="Ícono">
          {#each ICONOS_HABITO as ic}
            <button class="icono" role="radio" aria-checked={(h.icono ?? 'estrella') === ic} aria-label={ic} title={ic} onclick={() => poner(i, (x) => (x.icono = ic))}>{@html svg(ICONO_HABITO[ic], 18)}</button>
          {/each}
        </div>

        <div class="cuando">
          <button class="llave" aria-pressed={!!h.hora} onclick={() => poner(i, (x) => (x.hora = x.hora ? undefined : '09:00'))}><span class="led"></span>Pregunta</button>
          {#if h.hora}<input class="hora" type="time" aria-label="Hora" value={h.hora} onchange={(e) => { const t = e.currentTarget.value; if (t) poner(i, (x) => (x.hora = t)); }} />{/if}
          <div class="dias" role="group" aria-label="Días">
            {#each DIAS as l}<button class="dia" aria-pressed={(h.dias || DIAS.join('')).includes(l)} onclick={() => dia(i, l)}>{l}</button>{/each}
          </div>
        </div>

        <div class="puntos" role="img" aria-label="Últimos 60 días: {ULTIMOS.filter((d) => v?.dias?.[d]).length} hechos">
          {#each ULTIMOS as d, k}<span class:si={v?.dias?.[d]} class:hoy={k === 59}></span>{/each}
        </div>
      </div>
    {/each}
    {#if habitos.length < MAX_HABITOS}
      <button class="nuevo" onclick={sumar}><span>+</span><b>Hábito</b></button>
    {/if}
  </div>
  {#if aviso}<p class="aviso" aria-live="polite"><span class="led on"></span>{aviso}</p>{/if}
</div>

<style>
  .habitos { display: flex; flex-direction: column; gap: 16px; }
  .grilla { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .habito { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .arriba { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 12px; align-items: start; }
  .marcar { position: relative; display: grid; place-items: center; width: 84px; height: 84px; border-radius: 50%; cursor: pointer; color: #1a1714;
    background: radial-gradient(circle at 40% 30%, #fbf9f5, #ddd6ca 75%);
    box-shadow: inset 0 1px 0 #fff, inset 0 -3px 0 rgba(0,0,0,.12), 0 6px 0 #8f887c, 0 10px 16px -6px rgba(80,20,0,.55); transition: transform .08s, box-shadow .08s; }
  .marcar:active { transform: translateY(5px); box-shadow: 0 1px 0 #8f887c; }
  .marcar.hecho { color: #ff8a5c; background: radial-gradient(circle at 40% 30%, #3a3632, #151310 75%);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.12), 0 2px 0 #000, 0 0 22px rgba(255,106,51,.45); transform: translateY(4px); }
  .marcar .led { position: absolute; top: 10px; right: 14px; }
  .nombre { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .nombre .visor { padding: 10px 14px; }
  .nombre .campo { font-size: 22px; }
  .racha { flex-direction: row; align-items: baseline; gap: 8px; padding: 8px 14px; }
  .chica-c { width: 40px; min-height: 40px; font-size: 14px; }
  .iconos { display: grid; grid-template-columns: repeat(8, minmax(0, 1fr)); gap: 4px; }
  .icono { display: grid; place-items: center; height: 34px; border-radius: 9px; cursor: pointer; background: #0e0d0b; color: #5a524a; box-shadow: inset 0 2px 4px rgba(0,0,0,.8); }
  .icono[aria-checked='true'] { color: #ff8a5c; filter: drop-shadow(0 0 6px rgba(255,106,51,.6)); }
  .cuando { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .hora { flex: none; width: 150px; height: 44px; padding: 0 10px; border: 0; border-radius: 12px; background: #0e0d0b; color: #ff8a5c; font: 800 16px/1 var(--marca); font-variant-numeric: tabular-nums; color-scheme: dark; }
  .dias { flex: 1 0 100%; display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 4px; }
  .dia { display: grid; place-items: center; height: 32px; border-radius: 8px; cursor: pointer; background: #0e0d0b; color: #5a524a; font: 800 11px/1 var(--marca); box-shadow: inset 0 2px 4px rgba(0,0,0,.8); }
  .dia[aria-pressed='true'] { color: #ff8a5c; text-shadow: 0 0 8px rgba(255,106,51,.6); }
  .puntos { display: grid; grid-template-columns: repeat(20, minmax(0, 1fr)); gap: 5px; padding: 12px; border-radius: 14px; background: #0e0d0b; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); }
  .puntos span { aspect-ratio: 1; border-radius: 50%; background: #2c2925; }
  .puntos span.si { background: #ff6a33; box-shadow: 0 0 6px rgba(255,106,51,.7); }
  .puntos span.hoy { outline: 2px solid #f3eee8; outline-offset: 1px; }
  .nuevo { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; min-height: 160px; border-radius: 22px; cursor: pointer;
    color: rgba(26,23,20,.7); background: rgba(26,23,20,.08); box-shadow: inset 0 0 0 2px rgba(26,23,20,.25); font-family: var(--marca); }
  .nuevo:hover { color: #1a1714; }
  .nuevo span { font: 900 44px/1 var(--marca); }
  .nuevo b { font: 900 12px/1 var(--marca); text-transform: uppercase; letter-spacing: .1em; }
  @media (max-width: 1100px) { .grilla { grid-template-columns: 1fr; } }
  @media (max-width: 480px) {
    .marcar { width: 68px; height: 68px; }
    .marcar :global(svg) { width: 32px; height: 32px; }
    .puntos { gap: 3px; padding: 10px; }
  }
</style>
