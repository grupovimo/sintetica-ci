<script lang="ts">
  // Recordatorios: a la hora, el robot lo muestra en su pantalla (con una cara y
  // un sonido) hasta que lo tocás. Si está apagado y tenés Telegram vinculado,
  // te llega por ahí.
  import { usarBicho } from '$lib/bicho.svelte';
  import { cortarBytes, MAX_RECORDATORIOS, type Recordatorio } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const DIAS = [['L', 'Lunes'], ['M', 'Martes'], ['X', 'Miércoles'], ['J', 'Jueves'], ['V', 'Viernes'], ['S', 'Sábado'], ['D', 'Domingo']];
  const CARAS: [string, string][] = [['sorpresa', 'Sorpresa'], ['contento', 'Contento'], ['pensando', 'Pensando'], ['guino', 'Guiño'], ['enamorado', 'Enamorado'], ['escuchando', 'Atento']];
  const lista = $derived(b.config.recordatorios ?? []);
  const conSonido = $derived(modelo.capacidades.has('parlante'));

  function cambiar(i: number, f: (r: Recordatorio) => void) {
    g.cambiar((c) => {
      const l = [...(c.recordatorios ?? [])];
      const r = { ...l[i] };
      f(r);
      l[i] = r;
      c.recordatorios = l;
    });
  }
  function nuevo() {
    g.cambiar((c) => {
      const h = new Date(Date.now() + 3600e3);
      c.recordatorios = [...(c.recordatorios ?? []), { hora: `${String(h.getHours()).padStart(2, '0')}:00`, texto: 'Tomar agua', dias: '' }].slice(0, MAX_RECORDATORIOS);
    });
  }
  function borrar(i: number) {
    g.cambiar((c) => {
      c.recordatorios = (c.recordatorios ?? []).filter((_, j) => j !== i);
      if (!c.recordatorios.length) delete c.recordatorios;
    });
  }
  function dia(i: number, letra: string) {
    cambiar(i, (r) => {
      const d = new Set((r.dias ?? '').split('').filter(Boolean));
      if (d.size === 0) DIAS.forEach(([l]) => d.add(l)); // vacío = todos: se arranca de todos
      if (d.has(letra)) d.delete(letra); else d.add(letra);
      const orden = DIAS.map(([l]) => l).filter((l) => d.has(l)).join('');
      r.dias = orden.length === 7 ? '' : orden;
    });
  }
  const tiene = (r: Recordatorio, l: string) => !r.dias || r.dias.includes(l);
  const ordenados = $derived(lista.map((r, i) => ({ r, i })).sort((a, z) => a.r.hora.localeCompare(z.r.hora)));
</script>

<div class="recordatorios">
  <div class="titulo">
    <span class="grabado">{lista.length} de {MAX_RECORDATORIOS}</span>
    <button class="tecla negra" disabled={lista.length >= MAX_RECORDATORIOS} onclick={nuevo}>+ Nuevo</button>
  </div>

  {#each ordenados as { r, i } (i)}
    <div class="recordatorio bandeja" class:apagado={r.activo === false}>
      <label class="visor hora">
        <span class="grabado">A las</span>
        <input class="campo digitos chicos" type="time" value={r.hora} onchange={(e) => e.currentTarget.value && cambiar(i, (x) => (x.hora = e.currentTarget.value))} />
      </label>
      <div class="medio">
        <label class="visor texto-r">
          <span class="grabado">Te dice</span>
          <input class="campo claro" type="text" maxlength="40" value={r.texto} placeholder="Tomar la pastilla"
            oninput={(e) => { const v = cortarBytes(e.currentTarget.value, 40); if (v !== e.currentTarget.value) e.currentTarget.value = v; }}
            onchange={(e) => { const v = cortarBytes(e.currentTarget.value.trim(), 40); if (v) cambiar(i, (x) => (x.texto = v)); else e.currentTarget.value = r.texto; }} />
        </label>
        <div class="dias" role="group" aria-label="Días">
          {#each DIAS as [l, n]}
            <button class="dia" aria-pressed={tiene(r, l)} aria-label={n} title={n} onclick={() => dia(i, l)}>{l}</button>
          {/each}
          <select class="cara-sel" aria-label="Cara" value={r.expresion ?? 'sorpresa'} onchange={(e) => cambiar(i, (x) => (x.expresion = e.currentTarget.value === 'sorpresa' ? undefined : e.currentTarget.value))}>
            {#each CARAS as [k, t]}<option value={k}>Cara: {t}</option>{/each}
          </select>
        </div>
      </div>
      <div class="lado">
        <button class="llave" aria-pressed={r.activo !== false} onclick={() => cambiar(i, (x) => (x.activo = x.activo === false ? undefined : false))}><span class="led"></span>{r.activo === false ? 'Apagado' : 'Activo'}</button>
        {#if conSonido}<button class="llave" aria-pressed={r.sonido !== false} onclick={() => cambiar(i, (x) => (x.sonido = x.sonido === false ? undefined : false))}><span class="led"></span>Suena</button>{/if}
        <button class="tecla negra chica" aria-label="Borrar el recordatorio de las {r.hora}" onclick={() => borrar(i)}>Borrar</button>
      </div>
    </div>
  {:else}
    <div class="vacio bandeja">
      <span class="digitos">--:--</span>
      <p class="texto">Ej.: 08:30 · Tomar la pastilla</p>
    </div>
  {/each}
</div>

<style>
  .recordatorios { display: flex; flex-direction: column; gap: 14px; }
  .titulo { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
  .recordatorio { display: grid; grid-template-columns: 150px minmax(0, 1fr) auto; gap: 14px; align-items: stretch; transition: opacity .2s; }
  .recordatorio.apagado { opacity: .55; }
  .hora { justify-content: center; }
  .hora input { width: 100%; color-scheme: dark; }
  .hora input::-webkit-calendar-picker-indicator { filter: invert(.6); }
  .medio { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .texto-r { padding: 12px 16px; gap: 8px; }
  .dias { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
  .dia { width: 36px; height: 36px; border: 0; border-radius: 10px; cursor: pointer; text-align: center; font: 900 13px/1 var(--marca); color: rgba(26,23,20,.7);
    background: linear-gradient(180deg, #fbf9f5, #e6e0d6); box-shadow: 0 4px 0 #8f887c; transition: transform .08s, box-shadow .08s; }
  .dia[aria-pressed='true'] { transform: translateY(3px); color: #f3eee8; background: linear-gradient(180deg, #2a2622, #151310); box-shadow: 0 1px 0 #000; }
  .cara-sel { margin-left: auto; height: 36px; padding: 0 10px; border: 0; border-radius: 10px; background: #0e0d0b; color: #f3eee8; font: 700 13px/1 var(--marca); cursor: pointer; }
  .lado { display: flex; flex-direction: column; gap: 8px; justify-content: center; }
  .vacio { display: flex; align-items: center; gap: 20px; }
  .vacio .digitos { color: rgba(26,23,20,.4); text-shadow: none; }
  @media (max-width: 760px) {
    .recordatorio { grid-template-columns: 1fr; }
    .lado { flex-direction: row; flex-wrap: wrap; }
    .cara-sel { margin-left: 0; }
  }
</style>
