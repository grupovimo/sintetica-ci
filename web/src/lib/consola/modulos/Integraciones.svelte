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
  import { cortarBytes, JUEGOS, MAX_TEXTO } from '$lib/formato';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();

  type Accion =
    | { tipo: 'telegram'; texto: string }
    | { tipo: 'webhook'; url: string }
    | { tipo: 'robot'; bicho: string; expresion: string }
    | { tipo: 'decir'; texto: string; expresion?: string }
    | { tipo: 'cara'; expresion: string }
    | { tipo: 'modo'; modo: string }
    | { tipo: 'despertar' }
    | { tipo: 'jugar'; juego: string };
  // cuando "hora": a esa hora (HH:MM) los días que diga (L M X J V S D; vacío = todos).
  type Automatizacion = { id: string; activa: boolean; cuando: string; valor?: string; hora?: string; dias?: string; hacer: Accion[] };
  type Ejecucion = { automatizacion: string; evento: string; accion: string; ok: boolean; detalle?: string; en: string };

  // Lo que la nube detecta de verdad (la API manda la lista; estos son los nombres).
  const NOMBRES_CUANDO: Record<string, string> = {
    hora: 'A una hora', despertador: 'Suena el despertador',
    recordatorio: 'Muestra un recordatorio', pomodoro: 'Termina una tanda de pomodoro', 'bateria-baja': 'Se queda con poca batería',
    desconectado: 'Se desconecta', conectado: 'Se conecta', despierta: 'Se despierta', duerme: 'Se va a dormir', expresion: 'Cambia de cara',
    tacto: 'Lo tocan', 'doble-toque': 'Doble toque', caricia: 'Una caricia', 'muchos-toques': 'Muchos toques', levantado: 'Lo levantan',
    sacudida: 'Lo sacuden', 'sacudida-suave': 'Sacudida suave', 'sacudida-fuerte': 'Sacudida fuerte', 'boca-abajo': 'Lo ponen boca abajo',
    inclinado: 'Lo inclinan', caida: 'Se cae', voz: 'Le hablan', nombre: 'Dicen su nombre', ruido: 'Un ruido fuerte', musica: 'Suena música',
    cargando: 'Lo enchufan', desenchufado: 'Lo desenchufan', cargado: 'Termina de cargar', volvio: 'Volvés después de un rato', hito: 'Un hito (cumple, días juntos)',
    juego: 'Termina un juego', animo: 'Te pregunta cómo estás',
    llegaste: 'Llegás a casa', 'te-fuiste': 'Te vas', habito: 'Marcás un hábito', agenda: 'Se viene un evento',
  };
  let cuandos = $state<string[]>(['recordatorio', 'bateria-baja', 'desconectado', 'conectado', 'despierta', 'duerme', 'expresion']);
  const CUANDO = $derived([...new Set(['hora', 'despertador', ...cuandos])].map((k) => [k, NOMBRES_CUANDO[k] ?? k] as [string, string]));
  const ACCIONES: [Accion['tipo'], string][] = [
    ['decir', 'Decir'], ['cara', 'Cara'], ['modo', 'Modo'], ['despertar', 'Despertar'], ['jugar', 'Jugar'], ['telegram', 'Telegram'], ['webhook', 'URL'], ['robot', 'Otro robot'],
  ];
  const MODOS_ACCION: [string, string][] = [['mascota', 'Compañía'], ['reloj', 'Reloj'], ['pomodoro', 'Pomodoro'], ['clima', 'Clima']];
  const DIAS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  function dia(a: Automatizacion, l: string) {
    const ya = a.dias || DIAS.join('');
    const nuevo = DIAS.filter((d) => (d === l ? !ya.includes(d) : ya.includes(d))).join('');
    if (!nuevo) return;
    if (nuevo.length === 7) delete a.dias; else a.dias = nuevo;
    guardar();
  }
  const CARAS = ['contento', 'sorpresa', 'enamorado', 'guino', 'triste', 'pensando', 'dormido'];
  const PLANTILLAS = ['{robot}', '{dueno}', '{texto}', '{hora}', '{clima}', '{temp}', '{max}', '{min}', '{ciudad}', '{bateria}', '{expresion}'];
  // Las que solo tienen sentido con un cuando (agenda: {texto} es el título del evento).
  const PLANTILLAS_CUANDO: Record<string, string[]> = { juego: ['{juego}', '{resultado}'], animo: ['{animo}'], habito: ['{habito}', '{racha}'] };

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
  const plantillas = $derived([...new Set([...PLANTILLAS, ...autos.flatMap((a) => PLANTILLAS_CUANDO[a.cuando] ?? [])])]);
  let otros = $state<Bicho[]>([]);
  let secreto = $state('');
  let verSecreto = $state(false);
  let error = $state('');
  let sinNube = $state(false);
  let guardando = $state<'listo' | 'guardando' | 'guardado'>('listo');
  let probado = $state<Record<number, string>>({});

  // Música: Spotify (solo si la nube tiene las claves).
  type Suena = { sonando: boolean; titulo: string; artista: string; bpm?: number };
  type Musica = { disponible: boolean; conectada?: boolean; nombre?: string; suena?: Suena | null };
  let musica = $state<Musica | null>(null);
  async function cargarMusica() { musica = await api<Musica>('/v1/musica').catch(() => null); }
  async function conectarSpotify() {
    try { location.href = (await api<{ url: string }>('/v1/musica/spotify')).url; } catch (e) { error = texto(e, 'No pude abrir Spotify.'); }
  }
  async function desconectarSpotify() { await api('/v1/musica/spotify', { metodo: 'DELETE' }).catch(() => {}); await cargarMusica(); }

  // Tu IA (MCP): las IAs conectadas por OAuth y los tokens para Claude Code.
  type Cliente = { id: string; cliente: string; creada: string; usada: string | null };
  type TokenIA = { id: string; nombre: string; creado: string; usado: string | null };
  let mcpUrl = $state('');
  let clientes = $state<Cliente[]>([]);
  let tokens = $state<TokenIA[]>([]);
  let tokenNuevo = $state('');
  let copiado = $state('');
  async function cargarIA() {
    try {
      const c = await api<{ clientes: Cliente[]; url: string }>('/v1/mcp/clientes');
      clientes = c.clientes ?? [];
      mcpUrl = c.url;
      tokens = (await api<{ tokens: TokenIA[] }>('/v1/mcp/tokens')).tokens ?? [];
    } catch { /* sin MCP en esta nube */ }
  }
  async function crearToken() {
    try {
      tokenNuevo = (await api<{ token: string }>('/v1/mcp/tokens', { metodo: 'POST', cuerpo: { nombre: 'Claude Code' } })).token;
      await cargarIA();
    } catch (e) { error = texto(e, 'No pude crear el token.'); }
  }
  async function borrarToken(id: string) { await api(`/v1/mcp/tokens/${id}`, { metodo: 'DELETE' }).catch(() => {}); await cargarIA(); }
  async function desconectar(id: string) { await api(`/v1/mcp/clientes/${id}`, { metodo: 'DELETE' }).catch(() => {}); await cargarIA(); }
  async function copiar(t: string, que: string) {
    try { await navigator.clipboard.writeText(t); copiado = que; setTimeout(() => (copiado = ''), 1800); } catch { /* sin portapapeles */ }
  }
  const comandoClaude = $derived(`claude mcp add --transport http sintetica ${mcpUrl} --header "Authorization: Bearer ${tokenNuevo}"`);

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
    cargarIA();
    cargarMusica();
    return escuchar((e) => {
      const t = (e as { tipo: string; bicho?: string }).tipo;
      if (t === 'telegram' || (t === 'automatizacion' && e.bicho === b.id)) cargar();
      if (t === 'mcp') cargarIA();
      if (e.tipo === 'musica' && musica) {
        const d = e.datos;
        if (d.conectada !== undefined) cargarMusica();
        else if (d.titulo !== undefined) musica.suena = { sonando: !!d.sonando, titulo: d.titulo, artista: d.artista ?? '', bpm: d.bpm };
      }
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
    if (tipo === 'decir') return { tipo, texto: 'Buen día, {dueno}: {clima}' };
    if (tipo === 'cara') return { tipo, expresion: 'contento' };
    if (tipo === 'modo') return { tipo, modo: 'reloj' };
    if (tipo === 'despertar') return { tipo };
    if (tipo === 'jugar') return { tipo, juego: 'dado' };
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
    <div class="ia bandeja">
      <div class="logo ia-logo" aria-hidden="true">✦</div>
      <div class="que">
        <span class="grabado">Tu IA · Claude o ChatGPT</span>
        <b>{clientes.length ? `${clientes.length} conectada${clientes.length > 1 ? 's' : ''}` : 'Que la maneje tu IA'}</b>
        {#if mcpUrl}
          <button class="url" onclick={() => copiar(mcpUrl, 'url')} title="Copiar">{mcpUrl}<span class="grabado">{copiado === 'url' ? 'Copiada' : 'Copiar'}</span></button>
          <p class="texto">Agregala como conector.</p>
        {/if}
      </div>
      <button class="tecla negra chica" onclick={crearToken} title="Token para Claude Code">+ Token</button>
      {#if tokenNuevo}
        <div class="token-nuevo">
          <span class="grabado">Se ve una sola vez</span>
          <button class="url" onclick={() => copiar(comandoClaude, 'cmd')}><code>{comandoClaude}</code><span class="grabado">{copiado === 'cmd' ? 'Copiado' : 'Copiar'}</span></button>
        </div>
      {/if}
      {#if clientes.length || tokens.length}
        <div class="conectadas">
          {#each clientes as c (c.id)}
            <div class="ejec"><span class="led verde"></span><span>{c.cliente}</span><span class="que-paso">{c.usada ? `usó ${hora(c.usada)}` : 'sin usar'}</span><button class="tecla negra chica" onclick={() => desconectar(c.id)}>Desconectar</button></div>
          {/each}
          {#each tokens as t (t.id)}
            <div class="ejec"><span class="led"></span><span>{t.nombre}</span><span class="que-paso">{t.usado ? `usó ${hora(t.usado)}` : 'sin usar'}</span><button class="tecla negra chica" onclick={() => borrarToken(t.id)}>Borrar</button></div>
          {/each}
        </div>
      {/if}
    </div>

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

    {#if musica?.disponible}
      <div class="musica bandeja">
        <div class="logo spotify" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="30" height="30" fill="none" stroke="#0e0d0b" stroke-width="2.2" stroke-linecap="round"><path d="M6 9.5c4-1.3 8.5-1 12 1M7 13c3.3-1 6.8-.7 9.5.8M8 16.3c2.6-.7 5-.5 7 .6" /></svg>
        </div>
        <div class="que">
          <span class="grabado">Spotify{musica.nombre ? ` · ${musica.nombre}` : ''}</span>
          {#if !musica.conectada}<b>Que baile tu música</b>
          {:else if musica.suena?.titulo}
            <div class="suena" class:quieto={!musica.suena.sonando}>
              <span class="eq" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
              <span class="tema"><b>{musica.suena.titulo}</b><small>{musica.suena.artista}</small></span>
            </div>
          {:else}<b>Conectado</b>{/if}
        </div>
        {#if musica.conectada}<button class="tecla negra chica" onclick={desconectarSpotify}>Desconectar</button>
        {:else}<button class="tecla naranja" onclick={conectarSpotify}>Conectar</button>{/if}
      </div>
    {/if}

    <div class="titulo">
      <span class="grabado">Automatizaciones{guardando === 'guardando' ? ' · guardando…' : guardando === 'guardado' ? ' · guardadas' : ''}</span>
      <button class="tecla negra" disabled={autos.length >= 20} onclick={nueva}>+ Nueva</button>
    </div>

    {#each autos as a, i (a.id)}
      <div class="auto bandeja" class:apagada={!a.activa}>
        <div class="cuando">
          <span class="grabado">Cuando {b.bicho?.apodo || b.config.nombre || 'él'}…</span>
          <select class="sel" value={a.cuando} onchange={(e) => {
            a.cuando = e.currentTarget.value;
            if (a.cuando !== 'expresion') delete a.valor;
            if (a.cuando === 'hora') a.hora ??= '08:00'; else { delete a.hora; delete a.dias; }
            guardar();
          }}>
            {#each CUANDO as [k, t]}<option value={k}>{t}</option>{/each}
          </select>
          {#if a.cuando === 'expresion'}
            <select class="sel" value={a.valor ?? ''} onchange={(e) => { a.valor = e.currentTarget.value || undefined; guardar(); }}>
              <option value="">a cualquiera</option>{#each CARAS as c}<option value={c}>a {c}</option>{/each}
            </select>
          {:else if a.cuando === 'hora'}
            <input class="sel hora" type="time" aria-label="Hora" value={a.hora ?? '08:00'} onchange={(e) => { if (e.currentTarget.value) { a.hora = e.currentTarget.value; guardar(); } }} />
            <div class="dias" role="group" aria-label="Días">
              {#each DIAS as l}<button class="dia" aria-pressed={(a.dias || DIAS.join('')).includes(l)} onclick={() => dia(a, l)}>{l}</button>{/each}
            </div>
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
              {:else if h.tipo === 'decir'}
                <div class="visor"><span class="grabado">Dice</span>
                  <div class="decir">
                    <input class="campo claro" value={h.texto} aria-label="Qué dice" onchange={(e) => { h.texto = cortarBytes(e.currentTarget.value.trim(), MAX_TEXTO) || '¡Hola!'; e.currentTarget.value = h.texto; guardar(); }} />
                    <select class="campo claro chico" aria-label="Con qué cara" value={h.expresion ?? ''} onchange={(e) => { h.expresion = e.currentTarget.value || undefined; guardar(); }}>
                      <option value="">su cara</option>{#each CARAS as c}<option value={c}>{c}</option>{/each}
                    </select>
                  </div>
                </div>
              {:else if h.tipo === 'cara'}
                <label class="visor"><span class="grabado">Pone la cara</span>
                  <select class="campo claro" value={h.expresion} onchange={(e) => { h.expresion = e.currentTarget.value; guardar(); }}>
                    {#each CARAS as c}<option value={c}>{c}</option>{/each}
                  </select></label>
              {:else if h.tipo === 'modo'}
                <label class="visor"><span class="grabado">Pasa a</span>
                  <select class="campo claro" value={h.modo} onchange={(e) => { h.modo = e.currentTarget.value; guardar(); }}>
                    {#each MODOS_ACCION as [k, t]}<option value={k}>{t}</option>{/each}
                  </select></label>
              {:else if h.tipo === 'despertar'}
                <div class="visor"><span class="grabado">Lo despierta</span><b class="fijo">¡Buen día!</b></div>
              {:else if h.tipo === 'jugar'}
                <label class="visor"><span class="grabado">Te invita a</span>
                  <select class="campo claro" value={h.juego} onchange={(e) => { h.juego = e.currentTarget.value; guardar(); }}>
                    {#each JUEGOS as [k, t]}<option value={k}>{t}</option>{/each}
                  </select></label>
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
    <div class="ayuda" aria-label="Plantillas">{#each plantillas as p}<code>{p}</code>{/each}</div>

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
  .ia { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 14px 18px; align-items: center; }
  .ia-logo { background: linear-gradient(180deg, #2a2622, #151310) !important; color: #ff8a5c; font-size: 26px; box-shadow: inset 0 1px 0 rgba(255,255,255,.15), 0 5px 0 #000 !important; }
  .url { display: flex; align-items: center; justify-content: space-between; gap: 10px; width: 100%; max-width: 100%; padding: 10px 12px; border-radius: 10px; cursor: pointer;
    background: #0e0d0b; color: #ff8a5c; font: 700 13px/1.3 ui-monospace, monospace; text-align: left; overflow: hidden; }
  .url code { background: none; padding: 0; color: #f3eee8; overflow-wrap: anywhere; font-size: 12px; }
  .url .grabado { flex: none; }
  .token-nuevo, .conectadas { grid-column: 1 / -1; display: flex; flex-direction: column; gap: 6px; }
  .conectadas .ejec { display: grid; grid-template-columns: auto auto minmax(0, 1fr) auto; }
  .musica { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 18px; align-items: center; }
  .spotify { background: linear-gradient(180deg, #3ee07a, #1db954) !important; box-shadow: inset 0 1px 0 rgba(255,255,255,.4), 0 5px 0 #13863c !important; }
  .suena { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 12px; background: #0e0d0b; min-width: 0; }
  .tema { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
  .tema b { font: 800 16px/1.15 var(--marca) !important; text-transform: none !important; color: var(--c-papel); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tema small { font: 700 12px/1 var(--marca); color: var(--gris); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .eq { display: flex; align-items: flex-end; gap: 3px; height: 22px; flex: none; }
  .eq i { width: 4px; height: 100%; border-radius: 2px; background: #ff6a33; box-shadow: 0 0 6px rgba(255,106,51,.7); animation: eq .9s ease-in-out infinite alternate; transform-origin: bottom; }
  .eq i:nth-child(2) { animation-delay: -.3s; } .eq i:nth-child(3) { animation-delay: -.6s; } .eq i:nth-child(4) { animation-delay: -.15s; }
  .quieto .eq i { animation: none; transform: scaleY(.25); }
  @keyframes eq { from { transform: scaleY(.2); } to { transform: scaleY(1); } }
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
  .ayuda { display: flex; flex-wrap: wrap; gap: 6px; font: 700 12px/1.4 var(--marca); color: rgba(26,23,20,.75); }
  .hora { width: 100%; font-variant-numeric: tabular-nums; color-scheme: dark; }
  .dias { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 4px; }
  .dia { display: grid; place-items: center; height: 32px; border-radius: 8px; cursor: pointer; background: #0e0d0b; color: #5a524a; font: 800 11px/1 var(--marca); box-shadow: inset 0 2px 4px rgba(0,0,0,.8); }
  .dia[aria-pressed='true'] { color: #ff8a5c; text-shadow: 0 0 8px rgba(255,106,51,.6); }
  .decir { display: flex; flex-direction: column; gap: 6px; }
  .decir .chico { font-size: 13px; color: var(--gris-claro); }
  .fijo { font: 900 18px/1.1 var(--marca); color: var(--c-papel); }
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
    .telegram > :last-child, .musica > :last-child { grid-column: 1 / -1; }
    .musica { grid-template-columns: auto minmax(0, 1fr); }
  }
</style>
