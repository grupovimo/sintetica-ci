<script lang="ts">
  // Integraciones: el robot conectado al mundo desde la nube. Telegram (le
  // escribís y te avisa), webhooks a tus APIs, y automatizaciones: cuando pasa
  // algo en el robot, hace cosas.
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import type { Bicho } from '$lib/tipos';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();

  type Accion =
    | { tipo: 'telegram'; texto: string }
    | { tipo: 'webhook'; url: string }
    | { tipo: 'robot'; bicho: string; expresion: string };
  type Automatizacion = { id: string; activa: boolean; cuando: string; valor?: string; hacer: Accion[] };
  type Ejecucion = { automatizacion: string; evento: string; accion: string; ok: boolean; detalle?: string; en: string };

  // Lo que la nube detecta de verdad (la API manda la lista; estos son los nombres).
  const NOMBRES_CUANDO: Record<string, string> = {
    recordatorio: 'Muestra un recordatorio', pomodoro: 'Termina una tanda de pomodoro', 'bateria-baja': 'Se queda con poca batería',
    desconectado: 'Se desconecta', conectado: 'Se conecta', despierta: 'Se despierta', duerme: 'Se va a dormir', expresion: 'Cambia de cara',
    tacto: 'Lo tocan', 'doble-toque': 'Doble toque', caricia: 'Una caricia', 'muchos-toques': 'Muchos toques', levantado: 'Lo levantan',
    sacudida: 'Lo sacuden', 'sacudida-suave': 'Sacudida suave', 'sacudida-fuerte': 'Sacudida fuerte', 'boca-abajo': 'Lo ponen boca abajo',
    inclinado: 'Lo inclinan', caida: 'Se cae', voz: 'Le hablan', nombre: 'Dicen su nombre', ruido: 'Un ruido fuerte', musica: 'Suena música',
    cargando: 'Lo enchufan', desenchufado: 'Lo desenchufan', cargado: 'Termina de cargar', volvio: 'Volvés después de un rato', hito: 'Un hito (cumple, días juntos)',
  };
  let cuandos = $state<string[]>(['recordatorio', 'bateria-baja', 'desconectado', 'conectado', 'despierta', 'duerme', 'expresion']);
  const CUANDO = $derived(cuandos.map((k) => [k, NOMBRES_CUANDO[k] ?? k] as [string, string]));
  const ACCIONES: [Accion['tipo'], string][] = [['telegram', 'Mandarte un Telegram'], ['webhook', 'Llamar a una URL'], ['robot', 'Mover a otro robot']];
  const CARAS = ['contento', 'sorpresa', 'enamorado', 'guino', 'triste', 'pensando', 'dormido'];
  const PLANTILLAS = '{robot} {bateria} {expresion} {texto} {hora}';

  type Telegram = { activo: boolean; bot?: string; vinculado?: boolean; nombre?: string };
  let telegram = $state<Telegram | null>(null);
  let enlace = $state('');
  let qr = $state('');
  $effect(() => {
    if (!enlace) { qr = ''; return; }
    QRCode.toString(enlace, { type: 'svg', margin: 1, color: { dark: '#1a1714', light: '#f3eee8' } }).then((s) => (qr = s));
  });
  let autos = $state<Automatizacion[]>([]);
  let historial = $state<Ejecucion[]>([]);
  let otros = $state<Bicho[]>([]);
  let secreto = $state('');
  let verSecreto = $state(false);
  let error = $state('');
  let sinNube = $state(false);
  let guardando = $state<'listo' | 'guardando' | 'guardado'>('listo');
  let probado = $state<Record<number, string>>({});

  const texto = (e: unknown, por: string) => (e instanceof ErrorApi ? `${e.message}${e.detalle ? ': ' + e.detalle : ''}` : por);

  async function cargar() {
    try {
      telegram = await api<Telegram>('/v1/telegram');
      const r = await api<{ automatizaciones: Automatizacion[]; historial?: Ejecucion[]; cuandos?: string[] }>(`/v1/bichos/${b.id}/automatizaciones`);
      autos = r.automatizaciones ?? [];
      historial = r.historial ?? [];
      if (r.cuandos?.length) cuandos = r.cuandos;
      sinNube = false;
    } catch (e) {
      if (e instanceof ErrorApi && e.estado === 404) sinNube = true;
      else error = texto(e, 'No pude cargar las integraciones.');
    }
    api<{ bichos: Bicho[] }>('/v1/bichos').then((r) => (otros = r.bichos.filter((x) => x.id !== b.id))).catch(() => {});
  }
  onMount(() => {
    cargar();
    return escuchar((e) => {
      const t = (e as { tipo: string; bicho?: string }).tipo;
      if (t === 'telegram' || (t === 'automatizacion' && e.bicho === b.id)) cargar();
    });
  });

  async function vincular() {
    error = '';
    try {
      enlace = (await api<{ enlace: string }>('/v1/telegram/vincular', { metodo: 'POST' })).enlace;
      // Hasta que vuelva el evento, se fija cada tanto.
      const t = setInterval(async () => {
        telegram = await api<Telegram>('/v1/telegram').catch(() => telegram);
        if (telegram?.vinculado) { clearInterval(t); enlace = ''; }
      }, 3000);
      setTimeout(() => clearInterval(t), 15 * 60e3);
    } catch (e) {
      error = texto(e, 'No pude generar el enlace.');
    }
  }
  async function desvincular() {
    await api('/v1/telegram', { metodo: 'DELETE' }).catch(() => {});
    await cargar();
  }

  let reloj: ReturnType<typeof setTimeout> | null = null;
  function guardar() {
    guardando = 'guardando';
    if (reloj) clearTimeout(reloj);
    reloj = setTimeout(async () => {
      try {
        await api(`/v1/bichos/${b.id}/automatizaciones`, { metodo: 'PUT', cuerpo: { automatizaciones: $state.snapshot(autos) } });
        guardando = 'guardado';
        error = '';
      } catch (e) {
        guardando = 'listo';
        error = texto(e, 'No pude guardar.');
      }
    }, 700);
  }
  function nueva() {
    autos.push({ id: 'a' + Date.now().toString(36), activa: true, cuando: 'recordatorio', hacer: [{ tipo: 'telegram', texto: '{robot}: {texto}' }] });
    guardar();
  }
  function accionNueva(tipo: Accion['tipo']): Accion {
    if (tipo === 'telegram') return { tipo, texto: '{robot}: {evento}' };
    if (tipo === 'webhook') return { tipo, url: 'https://' };
    return { tipo, bicho: otros[0]?.id ?? '', expresion: 'contento' };
  }
  async function probar(i: number) {
    probado[i] = 'Probando…';
    try {
      await api(`/v1/bichos/${b.id}/automatizaciones/${i}/probar`, { metodo: 'POST' });
      probado[i] = 'Listo, fijate';
    } catch (e) {
      probado[i] = texto(e, 'Falló');
    }
    setTimeout(() => delete probado[i], 4000);
  }
  async function cargarSecreto(rotar = false) {
    try {
      secreto = (await api<{ secreto: string }>('/v1/webhooks/secreto', rotar ? { metodo: 'POST' } : {})).secreto;
      verSecreto = true;
    } catch (e) {
      error = texto(e, 'No pude traer el secreto.');
    }
  }
  const nombreDe = (id: string) => { const o = otros.find((x) => x.id === id); return o ? o.apodo || o.estado?.nombre || o.id : id; };
  const hora = (iso: string) => new Date(iso).toLocaleString('es-AR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
</script>

<div class="integraciones">
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}
  {#if sinNube}
    <p class="aviso"><span class="led ambar"></span>Las integraciones todavía no están en esta nube.</p>
  {:else}
    <div class="telegram bandeja">
      <div class="logo" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="30" height="30"><path fill="#f3eee8" d="M9.8 15.3 9.6 19c.4 0 .6-.2.8-.4l2-1.9 4 3c.7.4 1.3.2 1.5-.7l2.7-12.6c.3-1.1-.4-1.6-1.1-1.3L3.6 10.9c-1.1.4-1.1 1.1-.2 1.4l4 1.2 9.3-5.9c.4-.3.8-.1.5.2z" /></svg>
      </div>
      <div class="que">
        <span class="grabado">Telegram{telegram?.bot ? ` · @${telegram.bot}` : ''}</span>
        {#if !telegram}<b>Buscando…</b>
        {:else if !telegram.activo}<b>Apagado en esta nube</b>
        {:else if telegram.vinculado}<b>Vinculado{telegram.nombre ? ` con ${telegram.nombre}` : ''}</b>
          <p class="texto"><code>/robots</code> <code>/estado</code> <code>/cara</code></p>
        {:else}<b>Que te hable por Telegram</b>
          <p class="texto">{enlace ? 'Escaneá y tocá Iniciar.' : 'Te avisa y le escribís.'}</p>{/if}
      </div>
      {#if enlace && qr && !telegram?.vinculado}<div class="qr" role="img" aria-label="Código QR para vincular Telegram">{@html qr}</div>{/if}
      {#if telegram?.activo}
        {#if telegram.vinculado}<button class="tecla negra chica" onclick={desvincular}>Desvincular</button>
        {:else if enlace}<a class="tecla naranja" href={enlace} target="_blank" rel="noopener">Abrir Telegram</a>
        {:else}<button class="tecla naranja" onclick={vincular}>Vincular</button>{/if}
      {/if}
    </div>

    <div class="titulo">
      <span class="grabado">Automatizaciones{guardando === 'guardando' ? ' · guardando…' : guardando === 'guardado' ? ' · guardadas' : ''}</span>
      <button class="tecla negra" disabled={autos.length >= 20} onclick={nueva}>+ Nueva</button>
    </div>

    {#each autos as a, i (a.id)}
      <div class="auto bandeja" class:apagada={!a.activa}>
        <div class="cuando">
          <span class="grabado">Cuando {nombreDe(b.id)}…</span>
          <select class="sel" value={a.cuando} onchange={(e) => { a.cuando = e.currentTarget.value; if (a.cuando !== 'expresion') delete a.valor; guardar(); }}>
            {#each CUANDO as [k, t]}<option value={k}>{t}</option>{/each}
          </select>
          {#if a.cuando === 'expresion'}
            <select class="sel" value={a.valor ?? ''} onchange={(e) => { a.valor = e.currentTarget.value || undefined; guardar(); }}>
              <option value="">a cualquiera</option>{#each CARAS as c}<option value={c}>a {c}</option>{/each}
            </select>
          {/if}
        </div>
        <div class="hacer">
          {#each a.hacer as h, j}
            <div class="accion">
              <span class="flecha" aria-hidden="true">→</span>
              {#if h.tipo === 'telegram'}
                <label class="visor"><span class="grabado">Te manda por Telegram</span>
                  <input class="campo claro" value={h.texto} maxlength="300" onchange={(e) => { h.texto = e.currentTarget.value; guardar(); }} /></label>
              {:else if h.tipo === 'webhook'}
                <label class="visor"><span class="grabado">Llama a (POST, firmado)</span>
                  <input class="campo claro" type="url" value={h.url} placeholder="https://tu-api.com/gancho" onchange={(e) => { h.url = e.currentTarget.value.trim(); guardar(); }} /></label>
              {:else}
                <div class="visor"><span class="grabado">Otro robot pone una cara</span>
                  <div class="dos">
                    <select class="campo claro" value={h.bicho} onchange={(e) => { h.bicho = e.currentTarget.value; guardar(); }}>
                      {#each otros as o}<option value={o.id}>{nombreDe(o.id)}</option>{:else}<option value="">No tenés otro robot</option>{/each}
                    </select>
                    <select class="campo claro" value={h.expresion} onchange={(e) => { h.expresion = e.currentTarget.value; guardar(); }}>
                      {#each CARAS as c}<option value={c}>{c}</option>{/each}
                    </select>
                  </div>
                </div>
              {/if}
              <button class="tecla negra cuadrada" aria-label="Quitar esta acción" disabled={a.hacer.length === 1} onclick={() => { a.hacer.splice(j, 1); guardar(); }}>✕</button>
            </div>
          {/each}
          <div class="mas">
            {#each ACCIONES as [t, n]}
              <button class="tecla chica" disabled={a.hacer.length >= 4 || (t === 'robot' && !otros.length)} onclick={() => { a.hacer.push(accionNueva(t)); guardar(); }}>+ {n}</button>
            {/each}
          </div>
        </div>
        <div class="lado">
          <button class="llave" aria-pressed={a.activa} onclick={() => { a.activa = !a.activa; guardar(); }}><span class="led"></span>{a.activa ? 'Activa' : 'Apagada'}</button>
          <button class="tecla chica" onclick={() => probar(i)}>{probado[i] ?? 'Probar'}</button>
          <button class="tecla negra chica" onclick={() => { autos.splice(i, 1); guardar(); }}>Borrar</button>
        </div>
      </div>
    {:else}
      <div class="vacio bandeja"><p class="texto">Ej.: recordatorio → Telegram</p></div>
    {/each}
    <p class="ayuda">{PLANTILLAS}</p>

    <details class="bandeja webhooks">
      <summary class="grabado">Para desarrolladores: cómo verificar los webhooks</summary>
      <p class="texto">Cada llamada es un <code>POST</code> con JSON <code>{'{'}evento, robot, datos, en{'}'}</code> y la cabecera <code>X-Sintetica-Firma: sha256=…</code>, un HMAC del cuerpo con tu secreto.</p>
      <div class="secreto">
        <code>{verSecreto ? secreto : '••••••••••••••••••••••••'}</code>
        <button class="tecla chica" onclick={() => (verSecreto ? (verSecreto = false) : cargarSecreto())}>{verSecreto ? 'Ocultar' : 'Ver secreto'}</button>
        <button class="tecla negra chica" onclick={() => cargarSecreto(true)}>Rotar</button>
      </div>
    </details>

    {#if historial.length}
      <div class="historial">
        <span class="grabado">Últimas ejecuciones</span>
        {#each historial.slice(0, 6) as h}
          <div class="ejec"><span class="led" class:verde={h.ok} class:on={!h.ok}></span><span>{hora(h.en)}</span><span class="que-paso">{NOMBRES_CUANDO[h.evento] ?? h.evento} → {h.accion}{h.detalle ? ` · ${h.detalle}` : ''}</span></div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .integraciones { display: flex; flex-direction: column; gap: 14px; }
  .telegram { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 18px; align-items: center; }
  .qr { grid-column: 1 / -1; justify-self: center; width: 200px; padding: 10px; border-radius: 18px; background: #f3eee8; box-shadow: 0 6px 0 #a39c90; }
  .qr :global(svg) { display: block; width: 100%; height: auto; }
  .logo { display: grid; place-items: center; width: 56px; height: 56px; border-radius: 16px; background: linear-gradient(180deg, #3aa4e0, #1d7fbf);
    box-shadow: inset 0 1px 0 rgba(255,255,255,.4), 0 5px 0 #145a87; }
  .que { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .que b { font: 900 22px/1.05 var(--marca); font-stretch: 112%; text-transform: uppercase; letter-spacing: -.01em; }
  code { font-family: ui-monospace, monospace; font-size: .9em; padding: 1px 5px; border-radius: 5px; background: rgba(26,23,20,.12); }
  .titulo { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding-top: 6px; }
  .auto { display: grid; grid-template-columns: 210px minmax(0, 1fr) auto; gap: 14px; align-items: start; }
  .auto.apagada { opacity: .55; }
  .cuando { display: flex; flex-direction: column; gap: 8px; }
  .sel { height: 44px; padding: 0 10px; border: 0; border-radius: 12px; background: #0e0d0b; color: #f3eee8; font: 700 14px/1 var(--marca); cursor: pointer; }
  .hacer { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .accion { display: grid; grid-template-columns: 20px minmax(0, 1fr) 44px; gap: 8px; align-items: center; }
  .accion .visor { padding: 10px 14px; gap: 6px; }
  .flecha { font: 900 18px/1 var(--marca); color: rgba(26,23,20,.6); }
  .dos { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 10px; }
  .dos select { font-size: 16px; }
  .mas { display: flex; flex-wrap: wrap; gap: 8px; padding-left: 28px; padding-bottom: 4px; }
  .lado { display: flex; flex-direction: column; gap: 8px; }
  .ayuda { margin: 0; font: 700 12px/1.4 var(--marca); color: rgba(26,23,20,.7); }
  .webhooks summary { cursor: pointer; }
  .webhooks[open] { display: flex; flex-direction: column; gap: 12px; }
  .secreto { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .secreto code { flex: 1; min-width: 200px; padding: 12px; background: #0e0d0b; color: #ff8a5c; border-radius: 10px; overflow-wrap: anywhere; }
  .historial { display: flex; flex-direction: column; gap: 6px; }
  .ejec { display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 10px; background: #0e0d0b; color: #d9d2c6; font: 600 13px/1.3 var(--marca); }
  .que-paso { color: #8f877c; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
  @media (max-width: 860px) {
    .auto { grid-template-columns: 1fr; }
    .lado { flex-direction: row; flex-wrap: wrap; }
    .telegram { grid-template-columns: auto minmax(0, 1fr); }
    .telegram > :last-child { grid-column: 1 / -1; }
  }
</style>
