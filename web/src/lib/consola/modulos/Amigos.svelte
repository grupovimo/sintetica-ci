<script lang="ts">
  // Amigos: invitar con un código (o el QR), aceptar el de otro, y mandarle
  // toques a sus robots. Cada amigo elige cuáles de mis robots reciben los suyos.
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { TIPOS_TOQUE, type TipoToque } from '$lib/formato';
  import type { Bicho } from '$lib/tipos';
  import { ICONO_TOQUE, svg } from '../iconos';
  import { miniaturasDeCaras } from '../miniaturas';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();

  type Amigo = { id: number; nombre: string; robots: string[]; todos: boolean; conectado: boolean; creada: string };
  type Invitacion = { codigo: string; link: string; telegram?: string; vence: string };

  const NOMBRE_TOQUE: Record<TipoToque, string> = { toque: 'Toque', abrazo: 'Abrazo', beso: 'Beso', carita: 'Carita' };
  const CARITAS = ['contento', 'risa', 'enamorado', 'guino', 'sorpresa', 'triste', 'orgulloso', 'pensando'];

  let amigos = $state<Amigo[]>([]);
  let mios = $state<Bicho[]>([]);
  let invitacion = $state<Invitacion | null>(null);
  let qr = $state('');
  let codigo = $state('');
  let error = $state('');
  let aviso = $state('');
  let copiado = $state(false);
  let caras = $state<Record<string, string>>({});
  let carita = $state<Record<number, string>>({});
  let mandando = $state<string>('');

  const texto = (e: unknown, por: string) => (e instanceof ErrorApi ? e.message : por);
  const decir = (t: string) => { aviso = t; setTimeout(() => (aviso = ''), 2500); };

  async function cargar() {
    try {
      amigos = (await api<{ amigos: Amigo[] }>('/v1/amigos')).amigos ?? [];
    } catch (e) { error = texto(e, 'No pude traer tus amigos.'); }
  }
  onMount(() => {
    cargar();
    api<{ bichos: Bicho[] }>('/v1/bichos').then((r) => (mios = r.bichos)).catch(() => {});
    miniaturasDeCaras({ version: 2 }, { ancho: 160, alto: 86, redonda: false }).then((m) => (caras = m)).catch(() => {});
    return escuchar((e) => { if (e.tipo === 'toque') cargar(); });
  });

  $effect(() => {
    if (!invitacion) { qr = ''; return; }
    QRCode.toString(invitacion.link, { type: 'svg', margin: 1, color: { dark: '#1a1714', light: '#f3eee8' } }).then((s) => (qr = s));
  });

  async function invitar() {
    error = '';
    try { invitacion = await api<Invitacion>('/v1/amigos/invitacion', { metodo: 'POST' }); }
    catch (e) { error = texto(e, 'No pude crear el código.'); }
  }
  async function copiar() {
    if (!invitacion) return;
    try { await navigator.clipboard.writeText(invitacion.link); copiado = true; setTimeout(() => (copiado = false), 1800); } catch { /* sin portapapeles */ }
  }
  async function aceptar() {
    const c = codigo.replace(/[\s-]/g, '').toUpperCase();
    if (!c) return;
    error = '';
    try {
      const a = await api<Amigo>('/v1/amigos', { cuerpo: { codigo: c } });
      codigo = '';
      decir(`¡Ya son amigos con ${a.nombre}!`);
      await cargar();
    } catch (e) {
      error = e instanceof ErrorApi ? (e.estado === 409 ? 'Ya son amigos' : e.estado === 400 ? 'Ese código no anda' : e.message) : 'No pude.';
    }
  }
  async function tocar(a: Amigo, tipo: TipoToque) {
    if (mandando) return;
    mandando = `${a.id}:${tipo}`;
    try {
      const cuerpo: Record<string, string> = { tipo, desde: b.id };
      if (tipo === 'carita') cuerpo.expresion = carita[a.id] ?? 'contento';
      const r = await api<{ robots: number; telegram: boolean }>(`/v1/amigos/${a.id}/toque`, { cuerpo });
      decir(r.robots ? '¡Le llegó!' : r.telegram ? 'Le llegó por Telegram' : 'Le llega cuando se conecte');
    } catch (e) {
      decir(e instanceof ErrorApi && e.estado === 429 ? 'Muchos toques: esperá un rato' : texto(e, 'No pude'));
    }
    setTimeout(() => (mandando = ''), 300);
  }
  async function robots(a: Amigo, id: string | null) {
    // null = todos los míos; si no, prende o apaga ese.
    let l: string[] | null;
    if (id === null) l = a.todos ? [] : null;
    else {
      const ya = a.todos ? mios.map((m) => m.id) : a.robots;
      l = ya.includes(id) ? ya.filter((x) => x !== id) : [...ya, id];
      if (l.length === mios.length) l = null;
    }
    try { await api(`/v1/amigos/${a.id}`, { metodo: 'PUT', cuerpo: { robots: l } }); await cargar(); }
    catch (e) { error = texto(e, 'No pude cambiarlo.'); }
  }
  async function borrar(a: Amigo) {
    if (!confirm(`¿Dejar de ser amigos con ${a.nombre}?`)) return;
    await api(`/v1/amigos/${a.id}`, { metodo: 'DELETE' }).catch(() => {});
    await cargar();
  }
  const recibe = (a: Amigo, id: string) => a.todos || a.robots.includes(id);
  const nombreMio = (r: Bicho) => r.apodo || r.estado?.nombre || r.id.slice(-4);
  const vence = (iso: string) => Math.max(0, Math.round((Date.parse(iso) - Date.now()) / 3600e3));
</script>

<div class="amigos">
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  <div class="arriba">
    <div class="invitar bandeja">
      {#if invitacion}
        {#if qr}<div class="qr" role="img" aria-label="QR para sumarte de amigo">{@html qr}</div>{/if}
        <div class="codigo">
          <span class="grabado">Tu código · {vence(invitacion.vence)} h</span>
          <span class="digitos" aria-label="Código {invitacion.codigo}">{invitacion.codigo.slice(0, 4)}<i>-</i>{invitacion.codigo.slice(4)}</span>
          <div class="fila">
            <button class="tecla naranja chica" onclick={copiar}>{copiado ? 'Copiado' : 'Copiar link'}</button>
            {#if invitacion.telegram}<a class="tecla negra chica" href={invitacion.telegram} target="_blank" rel="noopener">Telegram</a>{/if}
          </div>
        </div>
      {:else}
        <div class="vacio-inv">
          <span class="corazones" aria-hidden="true">{@html svg(ICONO_TOQUE.abrazo, 40)}</span>
          <button class="tecla naranja" onclick={invitar}>Invitar</button>
        </div>
      {/if}
    </div>

    <form class="aceptar visor" onsubmit={(e) => { e.preventDefault(); aceptar(); }}>
      <span class="grabado">Código de un amigo</span>
      <input class="campo" aria-label="Código de un amigo" placeholder="ABCD-EFGH" maxlength="12" autocomplete="off" autocapitalize="characters" bind:value={codigo} />
      <button class="tecla naranja chica" disabled={!codigo.trim()}>Aceptar</button>
    </form>
  </div>

  {#if aviso}<p class="aviso" aria-live="polite"><span class="led on"></span>{aviso}</p>{/if}

  {#each amigos as a (a.id)}
    <div class="amigo bandeja">
      <div class="cabeza">
        <span class="led" class:verde={a.conectado}></span>
        <b>{a.nombre}</b>
        <button class="tecla negra cuadrada chica-c" aria-label="Dejar de ser amigos" onclick={() => borrar(a)}>✕</button>
      </div>
      <div class="toques" role="group" aria-label="Mandarle a {a.nombre}">
        {#each TIPOS_TOQUE as t}
          <button class="toque" class:abajo={mandando === `${a.id}:${t}`} aria-label={NOMBRE_TOQUE[t]} onclick={() => tocar(a, t)}>
            {@html svg(ICONO_TOQUE[t], 40)}
            <b>{NOMBRE_TOQUE[t]}</b>
          </button>
        {/each}
      </div>
      <div class="caritas" role="radiogroup" aria-label="Qué carita">
        {#each CARITAS as c}
          <button class="carita" role="radio" aria-checked={(carita[a.id] ?? 'contento') === c} aria-label={c} title={c} onclick={() => (carita[a.id] = c)}>
            {#if caras[c]}<img src={caras[c]} alt="" />{:else}<span>{c}</span>{/if}
          </button>
        {/each}
      </div>
      {#if mios.length}
        <div class="reciben">
          <span class="grabado">Le llega a</span>
          {#each mios as r (r.id)}
            <button class="llave" aria-pressed={recibe(a, r.id)} onclick={() => robots(a, r.id)}><span class="led"></span>{nombreMio(r)}</button>
          {/each}
          {#if mios.length > 1}<button class="llave" aria-pressed={a.todos} onclick={() => robots(a, null)}><span class="led"></span>Todos</button>{/if}
        </div>
      {/if}
    </div>
  {:else}
    <div class="bandeja sin"><span class="grabado">Sin amigos todavía</span></div>
  {/each}
</div>

<style>
  .amigos { display: flex; flex-direction: column; gap: 16px; }
  .arriba { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); gap: 16px; }
  .invitar { display: flex; align-items: center; gap: 18px; }
  .qr { flex: none; width: 132px; padding: 8px; border-radius: 16px; background: #f3eee8; box-shadow: 0 6px 0 #a39c90; }
  .qr :global(svg) { display: block; width: 100%; height: auto; }
  .codigo { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .codigo .digitos { font-size: 40px; letter-spacing: .04em; color: #1a1714; text-shadow: none; }
  .codigo i { font-style: normal; opacity: .4; }
  .fila { display: flex; flex-wrap: wrap; gap: 8px; padding-bottom: 6px; }
  .vacio-inv { display: flex; align-items: center; gap: 18px; width: 100%; justify-content: space-between; padding: 6px 4px 10px; }
  .corazones { color: #1a1714; }
  .corazones :global(.lleno) { fill: #1a1714; }
  .aceptar { justify-content: center; }
  .aceptar .campo { text-transform: uppercase; letter-spacing: .08em; }
  .aceptar .tecla { align-self: flex-start; }
  .amigo { display: flex; flex-direction: column; gap: 14px; }
  .cabeza { display: flex; align-items: center; gap: 10px; }
  .cabeza b { flex: 1; font: 900 24px/1 var(--marca); font-stretch: 112%; text-transform: uppercase; letter-spacing: -.01em; overflow-wrap: anywhere; }
  .chica-c { width: 40px; min-height: 40px; font-size: 14px; }
  .toques { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; padding-bottom: 6px; }
  .toque { display: flex; flex-direction: column; align-items: center; gap: 8px; min-height: 104px; padding: 16px 6px 12px; border-radius: 18px; cursor: pointer;
    color: #1a1714; font-family: var(--marca); background: linear-gradient(180deg, #fbf9f5 0%, #e6e0d6 100%);
    box-shadow: inset 0 1px 0 #fff, inset 0 -3px 0 rgba(0,0,0,.1), 0 6px 0 #8f887c, 0 10px 16px -6px rgba(80,20,0,.55); transition: transform .08s, box-shadow .08s; }
  .toque:active, .toque.abajo { transform: translateY(5px); box-shadow: inset 0 1px 0 #fff, 0 1px 0 #8f887c; }
  .toque :global(.lleno) { fill: #f2541a; stroke: none; }
  .toque:hover { color: #f2541a; }
  .toque b { font: 900 11px/1 var(--marca); text-transform: uppercase; letter-spacing: .04em; }
  .caritas { display: grid; grid-template-columns: repeat(8, minmax(0, 1fr)); gap: 6px; }
  .carita { aspect-ratio: 16 / 9; border-radius: 9px; overflow: hidden; cursor: pointer; background: #050505; box-shadow: inset 0 0 0 2px #2c2925; opacity: .6; }
  .carita[aria-checked='true'] { opacity: 1; box-shadow: 0 0 0 3px #1a1714; }
  .carita img { display: block; width: 100%; height: 100%; object-fit: cover; }
  .carita span { font: 700 9px/1 var(--marca); color: var(--gris); }
  .reciben { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .sin { padding: 22px; text-align: center; }
  @media (max-width: 760px) {
    .arriba { grid-template-columns: 1fr; }
    .caritas { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  }
  @media (max-width: 480px) {
    .toques { gap: 8px; }
    .toque { min-height: 88px; }
    .toque :global(svg) { width: 32px; height: 32px; }
    .toque b { font-size: 9px; }
    .invitar { flex-direction: column; align-items: stretch; }
    .qr { align-self: center; }
  }
</style>
