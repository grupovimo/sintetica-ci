<script lang="ts">
  // Su día: a qué hora se despierta y se duerme (fuera de eso duerme), el
  // despertador y los cumpleaños que festeja.
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { onMount } from 'svelte';
  import { cortarBytes, MAX_NOMBRE, MAX_TEXTO, MAX_TEXTO_PRESENCIA, PRESENCIA_BASE, type Despertador } from '$lib/formato';
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

  // El despertador: uno solo. Sin config, se ve apagado; tocar algo lo prende.
  const DIAS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  const POR_DEFECTO: Despertador = { hora: '07:30', dias: 'LMXJV', activo: true, sonido: true, posponer: 9 };
  const alarma = $derived<Despertador>(b.config.despertador ?? { ...POR_DEFECTO, activo: false });
  const dias = $derived(alarma.dias || DIAS.join(''));
  function ponerAlarma(f: (d: Despertador) => void) {
    g.cambiar((c) => {
      const d = { ...(c.despertador ?? POR_DEFECTO) };
      f(d);
      if (d.dias !== undefined && d.dias.length === 7) delete d.dias;
      if (!d.texto) delete d.texto;
      c.despertador = d;
    });
  }
  const moverAlarma = (d: number) => ponerAlarma((x) => (x.hora = aHora(aMin(x.hora) + d)));
  function dia(l: string) {
    ponerAlarma((x) => {
      const ya = x.dias || DIAS.join('');
      const nuevo = DIAS.filter((d) => (d === l ? !ya.includes(d) : ya.includes(d))).join('');
      if (nuevo) x.dias = nuevo;
    });
  }
  let probando = $state('');
  async function probar() {
    probando = '…';
    try {
      await api(`/v1/bichos/${b.id}/alarma`, { cuerpo: { accion: 'probar' } });
      probando = 'Sonando';
    } catch (e) {
      probando = e instanceof ErrorApi && e.estado === 409 ? 'Desconectado' : 'No pude';
    }
    setTimeout(() => (probando = ''), 3000);
  }
  const conectado = $derived(b.bicho?.conectado ?? false);

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

  // Presencia: los horarios que aprendió la nube (cuándo salís, llegás y dormís) y qué te dice al irte.
  type Horarios = { llegas: string | null; salis: string | null; dormis: string | null; seguridad: number; dias: number };
  let horarios = $state<Horarios | null>(null);
  onMount(() => { api<Horarios>(`/v1/bichos/${b.id}/horarios`).then((h) => (horarios = h)).catch(() => {}); });
  const presencia = $derived({ ...PRESENCIA_BASE, ...(b.config.presencia ?? {}) });
  const ponerPresencia = (f: (p: typeof presencia) => void) =>
    g.cambiar((c) => { const p = { ...PRESENCIA_BASE, ...(c.presencia ?? {}) }; f(p); if (!p.texto) delete (p as { texto?: string }).texto; c.presencia = p; });
  const segmentos = $derived(Math.round((horarios?.seguridad ?? 0) * 5));
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
      {#if alarma.activo !== false && b.config.despertador}<span class="campana" style="left: {(aMin(alarma.hora) / 1440) * 100}%" title="Despertador {alarma.hora}"></span>{/if}
    </div>
    <div class="horas">{#each [0, 6, 12, 18, 24] as h}<span>{String(h).padStart(2, '0')}</span>{/each}</div>
  </div>

  <div class="alarma bandeja" class:apagada={alarma.activo === false}>
    <div class="visor reloj grande">
      <div class="lectura"><span class="grabado">Despertador</span><span class="digitos">{alarma.hora}</span></div>
      <div class="pares">
        {#each [[60, 'una hora'], [5, '5 minutos']] as [d, t]}
          <div class="mas-menos">
            <button class="tecla negra cuadrada" aria-label="Despertador: {t} más tarde" onclick={() => moverAlarma(+d)}>+</button>
            <button class="tecla negra cuadrada" aria-label="Despertador: {t} más temprano" onclick={() => moverAlarma(-d)}>−</button>
          </div>
        {/each}
      </div>
    </div>
    <div class="controles">
      <div class="dias" role="group" aria-label="Días">
        {#each DIAS as l}<button class="tecla chica dia-t" class:naranja={dias.includes(l)} aria-pressed={dias.includes(l)} onclick={() => dia(l)}>{l}</button>{/each}
      </div>
      <div class="fila">
        <button class="llave" aria-pressed={alarma.activo !== false} onclick={() => { const on = alarma.activo !== false; ponerAlarma((x) => (x.activo = !on)); }}><span class="led"></span>Activo</button>
        <button class="llave" aria-pressed={alarma.sonido !== false} onclick={() => ponerAlarma((x) => (x.sonido = x.sonido === false))}><span class="led"></span>Sonido</button>
        <div class="posponer">
          <span class="grabado">Posponer</span>
          <button class="tecla negra cuadrada chica-c" aria-label="Posponer: menos" onclick={() => ponerAlarma((x) => (x.posponer = Math.max(1, (x.posponer ?? 9) - 1)))}>−</button>
          <span class="digitos chicos">{alarma.posponer ?? 9}<small>′</small></span>
          <button class="tecla negra cuadrada chica-c" aria-label="Posponer: más" onclick={() => ponerAlarma((x) => (x.posponer = Math.min(30, (x.posponer ?? 9) + 1)))}>+</button>
        </div>
      </div>
      <div class="fila">
        <label class="visor texto-alarma">
          <span class="solo-lectores">Qué dice</span>
          <input class="campo claro" placeholder="¡Buen día!" value={alarma.texto ?? ''}
            onchange={(e) => { const t = cortarBytes(e.currentTarget.value.trim(), MAX_TEXTO); ponerAlarma((x) => (x.texto = t || undefined)); }} />
        </label>
        <button class="tecla naranja" disabled={!conectado} onclick={probar}>{probando || 'Probar'}</button>
      </div>
    </div>
  </div>

  <div class="presencia bandeja">
    <div class="aprendidos">
      {#each [['salis', 'Salís', 'M14 4h5v16h-5M10 8l-4 4 4 4M6 12h10'], ['llegas', 'Llegás', 'M3 11l9-7 9 7M5 9.5V20h14V9.5M10 20v-5h4v5'], ['dormis', 'Dormís', 'M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z']] as [k, t, d]}
        {@const h = horarios?.[k as 'salis']}
        <div class="visor aprendido" class:sin={!h}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d={d} /></svg>
          <span class="grabado">{t}</span>
          <span class="digitos chicos">{h ?? '--:--'}</span>
        </div>
      {/each}
      <div class="visor seguridad" role="meter" aria-label="Seguridad" aria-valuemin={0} aria-valuemax={1} aria-valuenow={horarios?.seguridad ?? 0}>
        <span class="grabado">Seguro</span>
        <span class="medidor">{#each [0, 1, 2, 3, 4] as i}<span class:on={i < segmentos}></span>{/each}</span>
        <span class="grabado">{horarios?.dias ?? 0} d</span>
      </div>
    </div>
    <div class="al-irte">
      <button class="llave" aria-pressed={presencia.preguntar} onclick={() => ponerPresencia((p) => (p.preguntar = !p.preguntar))}><span class="led"></span>Al irte</button>
      <label class="visor texto-irte" class:off={!presencia.preguntar}>
        <span class="solo-lectores">Qué te dice al irte</span>
        <input class="campo claro" placeholder="¿Llevás las llaves?" disabled={!presencia.preguntar} value={presencia.texto}
          onchange={(e) => { const t = cortarBytes(e.currentTarget.value.trim(), MAX_TEXTO_PRESENCIA); ponerPresencia((p) => (p.texto = t)); }} />
      </label>
    </div>
  </div>

  <div class="cumples">
    <div class="visor">
      <span class="grabado">Vos te llamás</span>
      <input class="campo claro" type="text" maxlength="24" placeholder="Tu nombre" value={b.config.dueno?.nombre ?? ''}
        onchange={(e) => { const v = cortarBytes(e.currentTarget.value.trim(), MAX_NOMBRE); g.cambiar((c) => { c.dueno = { ...(c.dueno ?? {}), nombre: v || undefined }; if (!v) delete c.dueno.nombre; }); }} />
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
  .campana { position: absolute; top: 2px; bottom: 2px; width: 8px; margin-left: -4px; border-radius: 4px; background: #ffb347; box-shadow: 0 0 10px #ffb347; }
  .alarma { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 16px; align-items: stretch; transition: opacity .2s; }
  .alarma.apagada .digitos { color: #5a524a; text-shadow: none; }
  .grande { gap: 20px; }
  .grande .digitos { font-size: 64px; }
  .pares { display: flex; gap: 8px; }
  .controles { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .dias { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 8px; padding-bottom: 6px; }
  .dia-t { padding: 0; font-size: 13px; }
  .fila { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .posponer { display: flex; align-items: center; gap: 10px; margin-left: auto; padding-bottom: 6px; }
  .posponer .digitos { min-width: 2.2ch; text-align: center; }
  .posponer small { font-size: 14px; }
  .chica-c { width: 40px; min-height: 40px; }
  .texto-alarma { flex: 1; min-width: 160px; padding: 10px 16px; }
  .presencia { gap: 12px; }
  .aprendidos { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)) auto; gap: 10px; }
  .aprendido { align-items: flex-start; gap: 8px; padding: 12px 16px; }
  .aprendido svg { width: 22px; height: 22px; fill: none; stroke: var(--brasa); stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  .aprendido .digitos { font-size: 30px; }
  .aprendido.sin .digitos, .aprendido.sin svg { color: #5a524a; stroke: #5a524a; text-shadow: none; }
  .seguridad { align-items: center; justify-content: space-between; gap: 8px; padding: 12px 14px; }
  .medidor { display: flex; flex-direction: column-reverse; gap: 4px; }
  .medidor span { width: 26px; height: 7px; border-radius: 3px; background: #2c2925; }
  .medidor span.on { background: var(--verde); box-shadow: 0 0 6px var(--verde); }
  .al-irte { display: flex; align-items: center; gap: 10px; }
  .texto-irte { flex: 1; min-width: 0; padding: 10px 16px; }
  .texto-irte.off { opacity: .5; }
  .texto-irte .campo { font-size: 18px; }
  .cumples { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
  .fecha { display: flex; gap: 8px; }
  .dia-n { width: 3.2em; flex: none; }
  @media (max-width: 760px) {
    .visores, .cumples, .alarma { grid-template-columns: 1fr; }
    .aprendidos { grid-template-columns: repeat(3, minmax(0, 1fr)); }
    .seguridad { grid-column: 1 / -1; flex-direction: row; }
    .medidor { flex-direction: row; }
    .aprendido .digitos { font-size: 22px; }
    .grande .digitos { font-size: 52px; }
  }
</style>
