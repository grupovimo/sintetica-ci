<script lang="ts">
  import { usarBicho } from '$lib/bicho.svelte';
  import { clonar, EVENTOS, NOMBRES_EVENTO, type Config, type Regla } from '$lib/formato';

  const b = usarBicho();
  let reglas = $state<Regla[]>(clonar($state.snapshot(b.config.reglas ?? [])) as Regla[]);
  let error = $state('');
  let ok = $state('');
  let guardando = $state(false);
  const sucio = $derived(JSON.stringify(reglas) !== JSON.stringify(b.config.reglas ?? []));

  // Las reglas del manual (van después de las propias: las propias ganan).
  const DEL_MANUAL = [
    ['tacto', 'expresion:contento'], ['doble-toque', 'expresion:sospecha'], ['levantado', 'expresion:sorpresa'],
    ['sacudida', 'expresion:mareado'], ['boca-abajo', 'expresion:triste'], ['vinculado', 'expresion:guino'],
    ['app-abierta', 'expresion:enamorado'], ['despertado', 'expresion:enojado'], ['bateria<15', 'expresion:bateria-baja'],
  ];

  type Disparador = { tipo: 'evento' | 'bateria' | 'humor'; evento: string; op: '<' | '>'; valor: number };
  function leerSi(si: string): Disparador {
    const m = /^(bateria|humor)([<>])(-?[0-9.]+)$/.exec(si);
    if (m) return { tipo: m[1] as 'bateria' | 'humor', evento: 'tacto', op: m[2] as '<' | '>', valor: +m[3] };
    return { tipo: 'evento', evento: si, op: '<', valor: 15 };
  }
  function escribirSi(d: Disparador) {
    return d.tipo === 'evento' ? d.evento : `${d.tipo}${d.op}${d.valor}`;
  }
  const TIPOS_ACCION = [
    { tipo: 'expresion', nombre: 'Poner la cara' },
    { tipo: 'animacion', nombre: 'Reproducir la animación' },
    { tipo: 'sonido', nombre: 'Hacer el sonido' },
    { tipo: 'led', nombre: 'Prender el LED de' },
  ];
  const partir = (a: string) => {
    const i = a.indexOf(':');
    return [a.slice(0, i), a.slice(i + 1)];
  };

  function cambiarSi(i: number, cambio: Partial<Disparador>) {
    reglas[i] = { ...reglas[i], si: escribirSi({ ...leerSi(reglas[i].si), ...cambio }) };
  }
  function cambiarAccion(i: number, k: number, tipo: string, arg: string) {
    const entonces = [...reglas[i].entonces];
    entonces[k] = `${tipo}:${arg}`;
    reglas[i] = { ...reglas[i], entonces };
  }
  function argumentoPorDefecto(tipo: string) {
    return tipo === 'expresion' ? 'contento' : tipo === 'animacion' ? (b.nombresAnimaciones[0] ?? '') : tipo === 'led' ? '#FF6A33' : 'boing';
  }

  async function guardar() {
    guardando = true;
    error = ok = '';
    const c = clonar($state.snapshot(b.config)) as Config;
    if (reglas.length) c.reglas = $state.snapshot(reglas);
    else delete c.reglas;
    const e = await b.guardar(c);
    guardando = false;
    if (e) error = e;
    else ok = 'Guardadas.';
  }
</script>

<div class="grilla" style="max-width: 900px">
  <p class="nota" style="margin: 0">Cuando pasa algo, el bicho hace hasta cuatro cosas. Tus reglas van antes que las del manual: si las dos responden a lo mismo, gana la tuya.</p>
  {#each reglas as r, i}
    {@const d = leerSi(r.si)}
    <div class="tarjeta regla grilla">
      <div class="fila">
        <span class="etiqueta">Cuando</span>
        <select value={d.tipo} onchange={(e) => cambiarSi(i, { tipo: e.currentTarget.value as Disparador['tipo'] })} style="width: auto">
          <option value="evento">pasa algo</option><option value="bateria">la batería</option><option value="humor">el humor</option>
        </select>
        {#if d.tipo === 'evento'}
          <select value={d.evento} onchange={(e) => cambiarSi(i, { evento: e.currentTarget.value })} style="width: auto">
            {#each EVENTOS as ev}<option value={ev}>{NOMBRES_EVENTO[ev]}</option>{/each}
          </select>
        {:else}
          <select value={d.op} onchange={(e) => cambiarSi(i, { op: e.currentTarget.value as '<' | '>' })} style="width: auto"><option value="<">baja de</option><option value=">">pasa de</option></select>
          <input type="number" value={d.valor} step={d.tipo === 'humor' ? 0.1 : 1} min={d.tipo === 'humor' ? -1 : 0} max={d.tipo === 'humor' ? 1 : 100}
            onchange={(e) => cambiarSi(i, { valor: +e.currentTarget.value })} style="width: 100px" />
          <span class="nota">{d.tipo === 'bateria' ? '%' : '(de −1 a 1)'}</span>
        {/if}
        <button class="chico peligro" style="margin-left: auto" onclick={() => (reglas = reglas.filter((_, k) => k !== i))}>Quitar</button>
      </div>
      {#each r.entonces as a, k}
        {@const [tipo, arg] = partir(a)}
        <div class="fila accion">
          <span class="etiqueta">{k === 0 ? 'entonces' : 'y'}</span>
          <select value={tipo} onchange={(e) => cambiarAccion(i, k, e.currentTarget.value, argumentoPorDefecto(e.currentTarget.value))} style="width: auto">
            {#each TIPOS_ACCION as t}<option value={t.tipo}>{t.nombre}</option>{/each}
          </select>
          {#if tipo === 'expresion'}
            <select value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: auto">{#each b.nombresCaras as n}<option value={n}>{n}</option>{/each}</select>
          {:else if tipo === 'animacion'}
            <select value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: auto">
              {#each b.nombresAnimaciones as n}<option value={n}>{n}</option>{:else}<option value="">(no hay animaciones)</option>{/each}
            </select>
          {:else if tipo === 'led'}
            <input type="color" value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value.toUpperCase())} />
          {:else}
            <input value={arg} maxlength="24" pattern="[a-z0-9-]+" onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: 160px" />
          {/if}
          {#if r.entonces.length > 1}<button class="icono peligro" aria-label="Quitar la acción" onclick={() => (reglas[i] = { ...r, entonces: r.entonces.filter((_, j) => j !== k) })}>×</button>{/if}
        </div>
      {/each}
      {#if r.entonces.length < 4}<button class="chico" style="justify-self: start" onclick={() => (reglas[i] = { ...r, entonces: [...r.entonces, 'expresion:contento'] })}>Y además…</button>{/if}
    </div>
  {/each}
  <div class="fila">
    <button onclick={() => (reglas = [...reglas, { si: 'doble-toque', entonces: ['expresion:enamorado'] }])} disabled={reglas.length >= 15}>Regla nueva</button>
    <button class="primario" onclick={guardar} disabled={!sucio || guardando}>{guardando ? 'Guardando…' : 'Guardar'}</button>
    {#if sucio}<button onclick={() => (reglas = clonar($state.snapshot(b.config.reglas ?? [])) as Regla[])}>Descartar</button>{/if}
  </div>
  {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
  {#if ok && !sucio}<p class="aviso-caja ok">{ok}</p>{/if}

  <details class="tarjeta">
    <summary class="etiqueta">Las reglas del manual</summary>
    <ul class="manual">{#each DEL_MANUAL as [si, ent]}<li><span class="mono">{si}</span> → <span class="mono">{ent}</span></li>{/each}</ul>
  </details>
</div>

<style>
  .regla { box-shadow: none; padding: 16px; }
  .accion { padding-left: 12px; border-left: 2px solid var(--linea); }
  .manual { margin: 10px 0 0; padding-left: 18px; display: grid; gap: 4px; }
  summary { cursor: pointer; }
</style>
