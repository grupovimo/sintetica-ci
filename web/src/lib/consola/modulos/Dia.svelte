<script lang="ts">
  // Su día: a qué hora se despierta y se duerme (fuera de eso duerme), y los
  // cumpleaños que festeja.
  import { usarBicho } from '$lib/bicho.svelte';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const aMin = (h: string) => +h.slice(0, 2) * 60 + +h.slice(3, 5);
  const aHora = (m: number) => { const x = ((m % 1440) + 1440) % 1440; return `${String(Math.floor(x / 60)).padStart(2, '0')}:${String(x % 60).padStart(2, '0')}`; };
  const despierta = $derived(b.config.horario?.despierta ?? '08:00');
  const duerme = $derived(b.config.horario?.duerme ?? '23:30');

  function mover(cual: 'despierta' | 'duerme', d: number) {
    g.cambiar((c) => {
      const h = { despierta, duerme, ...(c.horario ?? {}) };
      const nuevo = aHora(aMin(h[cual]!) + d);
      if (nuevo === (cual === 'despierta' ? h.duerme : h.despierta)) return;
      c.horario = { ...h, [cual]: nuevo };
    });
  }

  // La barra de 24 h: dónde está despierto (de despierta a duerme, puede cruzar la medianoche).
  const tramos = $derived.by(() => {
    const a = aMin(despierta) / 1440, z = aMin(duerme) / 1440;
    return a < z ? [[a, z]] : [[0, z], [a, 1]];
  });
  let ahora = $state(new Date());
  $effect(() => { const t = setInterval(() => (ahora = new Date()), 60000); return () => clearInterval(t); });
  const enAhora = $derived((ahora.getHours() * 60 + ahora.getMinutes()) / 1440);

  const MESES = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
  const partes = (mmdd?: string) => (mmdd ? { m: +mmdd.slice(0, 2), d: +mmdd.slice(3, 5) } : { m: 0, d: 0 });
  function ponerCumple(de: 'dueno' | 'robot', m: number, d: number) {
    g.cambiar((c) => {
      const v = m && d ? `${String(m).padStart(2, '0')}-${String(Math.min(d, [31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][m - 1])).padStart(2, '0')}` : undefined;
      if (de === 'robot') { if (v) c.cumple = v; else delete c.cumple; }
      else { c.dueno = { ...(c.dueno ?? {}), cumple: v }; if (!v) delete c.dueno.cumple; }
    });
  }
  const cumpleDueno = $derived(partes(b.config.dueno?.cumple));
  const cumpleRobot = $derived(partes(b.config.cumple));
</script>

<div class="dia">
  <div class="visores">
    {#each [['despierta', 'Se despierta', despierta], ['duerme', 'Se duerme', duerme]] as [k, t, h]}
      <div class="visor reloj">
        <div class="lectura"><span class="grabado">{t}</span><span class="digitos">{h}</span></div>
        <div class="mas-menos">
          <button class="tecla negra cuadrada" aria-label="{t}: 15 minutos más tarde" onclick={() => mover(k as 'despierta', 15)}>+</button>
          <button class="tecla negra cuadrada" aria-label="{t}: 15 minutos más temprano" onclick={() => mover(k as 'despierta', -15)}>−</button>
        </div>
      </div>
    {/each}
  </div>

  <div class="bandeja">
    <div class="barra" role="img" aria-label="Despierto de {despierta} a {duerme}">
      {#each tramos as [a, z]}<span class="despierto" style="left: {a * 100}%; width: {(z - a) * 100}%"></span>{/each}
      <span class="ahora" style="left: {enAhora * 100}%" title="Ahora"></span>
    </div>
    <div class="horas">{#each [0, 6, 12, 18, 24] as h}<span>{String(h).padStart(2, '0')}</span>{/each}</div>
    <p class="texto">De noche cuenta ovejas y sueña; a la mañana se despierta con vos. Si lo despertás antes, se queja un poco.</p>
  </div>

  <div class="cumples">
    <div class="visor">
      <span class="grabado">Vos te llamás</span>
      <input class="campo claro" type="text" maxlength="24" placeholder="Tu nombre" value={b.config.dueno?.nombre ?? ''}
        onchange={(e) => { const v = e.currentTarget.value.trim(); g.cambiar((c) => { c.dueno = { ...(c.dueno ?? {}), nombre: v || undefined }; if (!v) delete c.dueno.nombre; }); }} />
    </div>
    {#each [['dueno', 'Tu cumple', cumpleDueno], ['robot', 'Su cumple', cumpleRobot]] as [de, t, cu]}
      {@const p = cu as { m: number; d: number }}
      <div class="visor">
        <span class="grabado">{t}</span>
        <div class="fecha">
          <select class="campo claro dia-n" aria-label="{t}: día" value={p.d} onchange={(e) => ponerCumple(de as 'dueno', p.m || 1, +e.currentTarget.value)}>
            <option value={0}>—</option>{#each Array.from({ length: 31 }, (_, i) => i + 1) as d}<option value={d}>{d}</option>{/each}
          </select>
          <select class="campo claro" aria-label="{t}: mes" value={p.m} onchange={(e) => ponerCumple(de as 'dueno', +e.currentTarget.value, p.d || 1)}>
            <option value={0}>—</option>{#each MESES as m, i}<option value={i + 1}>{m}</option>{/each}
          </select>
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  .dia { display: flex; flex-direction: column; gap: 16px; }
  .visores { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .reloj { flex-direction: row; align-items: center; justify-content: space-between; }
  .lectura { display: flex; flex-direction: column; gap: 10px; }
  .mas-menos { display: flex; flex-direction: column; gap: 10px; }
  .barra { position: relative; height: 34px; border-radius: 10px; background: #0e0d0b; box-shadow: inset 0 2px 6px rgba(0,0,0,.8); overflow: hidden; }
  .despierto { position: absolute; top: 5px; bottom: 5px; border-radius: 6px; background: repeating-linear-gradient(90deg, #ff8a5c 0 6px, #f2541a 6px 8px); box-shadow: 0 0 12px rgba(255,106,51,.6); }
  .ahora { position: absolute; top: 0; bottom: 0; width: 3px; margin-left: -1.5px; background: #f3eee8; box-shadow: 0 0 8px #fff; }
  .horas { display: flex; justify-content: space-between; font: 800 11px/1 var(--marca); color: rgba(26,23,20,.66); font-variant-numeric: tabular-nums; }
  .bandeja { display: flex; flex-direction: column; gap: 10px; }
  .cumples { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
  .fecha { display: flex; gap: 8px; }
  .dia-n { width: 3.2em; flex: none; }
  @media (max-width: 760px) {
    .visores, .cumples { grid-template-columns: 1fr; }
  }
</style>
