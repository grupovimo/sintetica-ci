<script lang="ts">
  import { usarBicho } from '$lib/bicho.svelte';
  import { clonar, COMPORTAMIENTOS_BASE, ESTADO, EVENTOS, NOMBRES_ESTADO, NOMBRES_EVENTO, PARTICULAS, REGLAS_BASE, type Config, type Regla } from '$lib/formato';

  const b = usarBicho();
  let reglas = $state<Regla[]>(clonar($state.snapshot(b.config.reglas ?? [])) as Regla[]);
  let error = $state('');
  let ok = $state('');
  let guardando = $state(false);
  const sucio = $derived(JSON.stringify(reglas) !== JSON.stringify(b.config.reglas ?? []));

  // Las reglas base (van después de las propias: las propias ganan).
  const DEL_MANUAL = REGLAS_BASE.map((r) => [r.si + (r.y ? ` (y ${r.y})` : ''), r.entonces.join(', ')]);
  const comportamientos = $derived([...new Set([...Object.keys(COMPORTAMIENTOS_BASE), ...Object.keys(b.config.comportamientos ?? {})])]);

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
    { tipo: 'comportamiento', nombre: 'Hacer' },
    { tipo: 'reflejo', nombre: 'Reflejo (ya, < 50 ms)' },
    { tipo: 'estado', nombre: 'Empujar el estado' },
    { tipo: 'particula', nombre: 'Soltar' },
    { tipo: 'expresion', nombre: 'Poner la cara' },
    { tipo: 'animacion', nombre: 'Reproducir la animación' },
    { tipo: 'sonido', nombre: 'Hacer el sonido' },
    { tipo: 'led', nombre: 'Prender el LED de' },
  ];
  // "estado.animo+0.2" es una acción de tipo "estado" con argumento "animo+0.2".
  const partir = (a: string) => {
    if (a.startsWith('estado.')) return ['estado', a.slice(7)];
    const i = a.indexOf(':');
    return [a.slice(0, i), a.slice(i + 1)];
  };
  const unir = (tipo: string, arg: string) => (tipo === 'estado' ? `estado.${arg}` : `${tipo}:${arg}`);
  const partirEstado = (arg: string) => {
    const m = /^([a-z]+)([+-])([0-9.]+)$/.exec(arg);
    return m ? { v: m[1], s: m[2], n: +m[3] } : { v: 'animo', s: '+', n: 0.1 };
  };

  function cambiarSi(i: number, cambio: Partial<Disparador>) {
    reglas[i] = { ...reglas[i], si: escribirSi({ ...leerSi(reglas[i].si), ...cambio }) };
  }
  function cambiarAccion(i: number, k: number, tipo: string, arg: string) {
    const entonces = [...reglas[i].entonces];
    entonces[k] = unir(tipo, arg);
    reglas[i] = { ...reglas[i], entonces };
  }
  function argumentoPorDefecto(tipo: string) {
    return tipo === 'expresion' || tipo === 'reflejo' ? 'contento' : tipo === 'animacion' ? (b.nombresAnimaciones[0] ?? '')
      : tipo === 'led' ? '#FF6A33' : tipo === 'comportamiento' ? 'toque' : tipo === 'estado' ? 'animo+0.1' : tipo === 'particula' ? 'corazon' : 'boing';
  }

  async function guardar() {
    guardando = true;
    error = ok = '';
    const c = clonar($state.snapshot(b.config)) as Config;
    if (reglas.length) c.reglas = $state.snapshot(reglas).map((r) => (r.y ? r : { si: r.si, entonces: r.entonces }));
    else delete c.reglas;
    c.version = 2;
    const e = await b.guardar(c);
    guardando = false;
    if (e) error = e;
    else ok = 'Guardadas.';
  }
</script>

<div class="grilla" style="max-width: 900px">
  <p class="nota" style="margin: 0">Cuando pasa algo, el bicho hace hasta seis cosas. Tus reglas van antes que las base: si las dos responden a lo mismo (y se cumple el "si además"), gana la tuya.</p>
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
      <div class="fila">
        <span class="etiqueta">si además</span>
        <input class="mono" value={r.y ?? ''} placeholder="afecto>0.6, rasgo.timidez>0.5, dormido, noche…" style="flex: 1"
          onchange={(e) => (reglas[i] = { ...r, y: e.currentTarget.value.replace(/\s+/g, '') || undefined })} />
      </div>
      {#each r.entonces as a, k}
        {@const [tipo, arg] = partir(a)}
        <div class="fila accion">
          <span class="etiqueta">{k === 0 ? 'entonces' : 'y'}</span>
          <select value={tipo} onchange={(e) => cambiarAccion(i, k, e.currentTarget.value, argumentoPorDefecto(e.currentTarget.value))} style="width: auto">
            {#each TIPOS_ACCION as t}<option value={t.tipo}>{t.nombre}</option>{/each}
          </select>
          {#if tipo === 'expresion' || tipo === 'reflejo'}
            <select value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: auto">{#each b.nombresCaras as n}<option value={n}>{n}</option>{/each}</select>
          {:else if tipo === 'comportamiento'}
            <select value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: auto">{#each comportamientos as n}<option value={n}>{n}</option>{/each}</select>
          {:else if tipo === 'particula'}
            <select value={arg} onchange={(e) => cambiarAccion(i, k, tipo, e.currentTarget.value)} style="width: auto">{#each PARTICULAS as n}<option value={n}>{n}</option>{/each}</select>
          {:else if tipo === 'estado'}
            {@const es = partirEstado(arg)}
            <select value={es.v} onchange={(e) => cambiarAccion(i, k, tipo, `${e.currentTarget.value}${es.s}${es.n}`)} style="width: auto">{#each ESTADO as v}<option value={v}>{NOMBRES_ESTADO[v]}</option>{/each}</select>
            <select value={es.s} onchange={(e) => cambiarAccion(i, k, tipo, `${es.v}${e.currentTarget.value}${es.n}`)} style="width: auto"><option value="+">sube</option><option value="-">baja</option></select>
            <input type="number" min="0" max="1" step="0.05" value={es.n} onchange={(e) => cambiarAccion(i, k, tipo, `${es.v}${es.s}${+e.currentTarget.value}`)} style="width: 90px" />
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
      {#if r.entonces.length < 6}<button class="chico" style="justify-self: start" onclick={() => (reglas[i] = { ...r, entonces: [...r.entonces, 'expresion:contento'] })}>Y además…</button>{/if}
    </div>
  {/each}
  <div class="fila">
    <button onclick={() => (reglas = [...reglas, { si: 'doble-toque', entonces: ['expresion:enamorado'] }])} disabled={reglas.length >= 24}>Regla nueva</button>
    <button class="primario" onclick={guardar} disabled={!sucio || guardando}>{guardando ? 'Guardando…' : 'Guardar'}</button>
    {#if sucio}<button onclick={() => (reglas = clonar($state.snapshot(b.config.reglas ?? [])) as Regla[])}>Descartar</button>{/if}
  </div>
  {#if error}<p class="aviso-caja error" role="alert">{error}</p>{/if}
  {#if ok && !sucio}<p class="aviso-caja ok">{ok}</p>{/if}

  <details class="tarjeta">
    <summary class="etiqueta">Las reglas base</summary>
    <ul class="manual">{#each DEL_MANUAL as [si, ent]}<li><span class="mono">{si}</span> → <span class="mono">{ent}</span></li>{/each}</ul>
  </details>
</div>

<style>
  .regla { box-shadow: none; padding: 16px; }
  .accion { padding-left: 12px; border-left: 2px solid var(--linea); }
  .manual { margin: 10px 0 0; padding-left: 18px; display: grid; gap: 4px; }
  summary { cursor: pointer; }
</style>
