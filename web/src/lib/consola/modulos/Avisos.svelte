<script lang="ts">
  // Avisos de afuera: una dirección secreta para pegar en GitHub, Mercado Pago o
  // donde quieras. Cuando llega algo, tus robots te avisan con su cara.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { escuchar } from '$lib/eventos.svelte';
  import { TIPOS_ENTRANTE, type TipoEntrante } from '$lib/formato';
  import { ICONO_ENTRANTE, svg } from '../iconos';
  import { miniaturasDeCaras } from '../miniaturas';
  import type { Modelo } from '../modelo';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();

  type Entrantes = { url: string | null; token: string | null; mercadoPago: boolean; creado: string; regenerar?: boolean };
  let e = $state<Entrantes | null>(null);
  let error = $state('');
  let copiado = $state('');
  let seguro = $state(false);
  let mp = $state('');
  let ultimo = $state<{ tipo: string; titulo: string; de: string } | null>(null);
  let probado = $state<Record<string, string>>({});
  let caras = $state<Record<string, string>>({});

  // Lo que hace con cada uno (el núcleo: compania.cpp). Pago: cara de emocionado y su sonido; los demás, un globo.
  const TIPOS: Record<TipoEntrante, { nombre: string; cara?: string; ejemplo: { titulo: string; de: string } }> = {
    pago: { nombre: 'Pago', cara: 'emocionado', ejemplo: { titulo: '$12.000', de: 'Ana' } },
    mail: { nombre: 'Mail', ejemplo: { titulo: 'Hola', de: 'Ana' } },
    github: { nombre: 'GitHub', ejemplo: { titulo: 'PR #12', de: 'ana' } },
    pedido: { nombre: 'Pedido', ejemplo: { titulo: 'Pedido #8', de: 'La tienda' } },
    otro: { nombre: 'Otro', ejemplo: { titulo: 'Aviso', de: 'Vos' } },
  };

  const texto = (x: unknown, por: string) => (x instanceof ErrorApi ? `${x.message}${x.detalle ? ': ' + x.detalle : ''}` : por);
  async function cargar() {
    try { e = await api<Entrantes>('/v1/entrantes'); } catch (x) { error = texto(x, 'No pude traer la dirección.'); }
  }
  onMount(() => {
    cargar();
    miniaturasDeCaras(b.config, { ancho: 160, alto: 86, redonda: false }).then((m) => (caras = m)).catch(() => {});
    return escuchar((x) => { if (x.tipo === 'entrante') ultimo = x.datos; });
  });

  async function copiar(t: string | null, que: string) {
    if (!t) return;
    try { await navigator.clipboard.writeText(t); copiado = que; setTimeout(() => (copiado = ''), 1800); } catch { /* sin portapapeles */ }
  }
  async function regenerar() {
    if (!seguro) { seguro = true; setTimeout(() => (seguro = false), 4000); return; }
    seguro = false;
    try { e = await api<Entrantes>('/v1/entrantes', { metodo: 'POST' }); } catch (x) { error = texto(x, 'No pude.'); }
  }
  async function guardarMP() {
    const t = mp.trim();
    if (!t) return;
    try { await api('/v1/entrantes/mercadopago', { metodo: 'PUT', cuerpo: { accessToken: t } }); mp = ''; await cargar(); }
    catch (x) { error = texto(x, 'No pude guardarlo.'); }
  }
  async function sacarMP() {
    await api('/v1/entrantes/mercadopago', { metodo: 'DELETE' }).catch(() => {});
    await cargar();
  }
  async function probar(t: TipoEntrante) {
    if (!e?.token) return;
    probado[t] = '…';
    try {
      const r = await api<{ robots: number }>(`/v1/entrantes/${e.token}`, { cuerpo: { tipo: t, ...TIPOS[t].ejemplo, robot: b.id } });
      probado[t] = r.robots ? '¡Llegó!' : 'Desconectado';
    } catch (x) { probado[t] = texto(x, 'No pude'); }
    setTimeout(() => delete probado[t], 2500);
  }
</script>

<div class="avisos">
  {#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

  <div class="direccion bandeja">
    <span class="grabado">Tu dirección secreta</span>
    {#if e?.url}
      <button class="url" onclick={() => copiar(e!.url, 'url')} title="Copiar"><code>{e.url}</code><span class="grabado">{copiado === 'url' ? 'Copiada' : 'Copiar'}</span></button>
    {:else if e?.regenerar}
      <p class="aviso"><span class="led ambar"></span>Hay que hacer una nueva</p>
    {/if}
    <button class="tecla negra chica" onclick={regenerar}>{seguro ? '¿Seguro? La vieja deja de andar' : 'Regenerar'}</button>
  </div>

  <div class="servicios">
    <div class="servicio bandeja">
      <div class="logo gh" aria-hidden="true">{@html svg(ICONO_ENTRANTE.github, 28)}</div>
      <ol class="pasos">
        <li><span class="grabado">Webhooks</span><b>Add webhook</b></li>
        <li><span class="grabado">Content type</span><b>application/json</b></li>
        <li><span class="grabado">Secret</span><button class="tecla chica" disabled={!e?.token} onclick={() => copiar(e!.token, 'token')}>{copiado === 'token' ? 'Copiado' : 'Copiar'}</button></li>
      </ol>
    </div>
    <div class="servicio bandeja">
      <div class="logo mp" aria-hidden="true">{@html svg(ICONO_ENTRANTE.pago, 28)}</div>
      <ol class="pasos">
        <li><span class="grabado">Mercado Pago</span><b>Webhooks · Pagos</b></li>
        {#if e?.mercadoPago}
          <li><span class="grabado">Monto</span><span class="si"><span class="led verde"></span>Con access token</span><button class="tecla negra chica" onclick={sacarMP}>Sacar</button></li>
        {:else}
          <li class="token">
            <input class="campo-mp" type="password" autocomplete="off" aria-label="Access token de Mercado Pago (opcional)" placeholder="Access token" bind:value={mp} />
            <button class="tecla chica" disabled={!mp.trim()} onclick={guardarMP}>Guardar</button>
          </li>
        {/if}
      </ol>
    </div>
  </div>

  <div class="tipos">
    {#each TIPOS_ENTRANTE as t}
      {@const x = TIPOS[t]}
      <div class="tipo visor" class:ultimo={ultimo?.tipo === t}>
        <span class="icono">{@html svg(ICONO_ENTRANTE[t], 26)}</span>
        <div class="pantallita" aria-label={x.cara ? `Pone cara de ${x.cara}` : 'Lo dice en un globo'}>
          {#if x.cara && caras[x.cara]}<img src={caras[x.cara]} alt="" />
          {:else}<span class="globo">{x.ejemplo.titulo}</span>{/if}
        </div>
        <b>{x.nombre}</b>
        <button class="tecla chica" disabled={!e?.token} onclick={() => probar(t)}>{probado[t] ?? 'Probar'}</button>
      </div>
    {/each}
  </div>
</div>

<style>
  .avisos { display: flex; flex-direction: column; gap: 14px; }
  .direccion { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; }
  .url { display: flex; align-items: center; justify-content: space-between; gap: 10px; width: 100%; padding: 12px 14px; border-radius: 12px; cursor: pointer;
    background: #0e0d0b; text-align: left; overflow: hidden; }
  .url code { font: 700 13px/1.3 ui-monospace, monospace; color: #ff8a5c; overflow-wrap: anywhere; min-width: 0; }
  .url .grabado { flex: none; }
  .servicios { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
  .servicio { display: flex; gap: 14px; align-items: flex-start; min-width: 0; }
  .logo { flex: none; display: grid; place-items: center; width: 52px; height: 52px; border-radius: 15px; color: #f3eee8; }
  .logo.gh { background: linear-gradient(180deg, #3a3632, #151310); box-shadow: inset 0 1px 0 rgba(255,255,255,.15), 0 5px 0 #000; }
  .logo.mp { background: linear-gradient(180deg, #4fb4ff, #1f7fd6); box-shadow: inset 0 1px 0 rgba(255,255,255,.4), 0 5px 0 #145a99; }
  .pasos { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; margin: 0; padding: 0; list-style: none; }
  .pasos li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; min-height: 40px; }
  .pasos b { font: 800 14px/1.2 var(--marca); color: #1a1714; overflow-wrap: anywhere; }
  .si { display: inline-flex; align-items: center; gap: 8px; font: 800 13px/1 var(--marca); }
  .token { flex-wrap: nowrap !important; }
  .campo-mp { flex: 1; min-width: 0; height: 40px; padding: 0 12px; border: 0; border-radius: 11px; background: #0e0d0b; color: #f3eee8; font: 600 13px/1 var(--marca); }
  .campo-mp::placeholder { color: #5a524a; }
  .tipos { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
  .tipo { align-items: center; gap: 10px; padding: 14px 10px; text-align: center; }
  .tipo.ultimo { box-shadow: inset 0 3px 10px rgba(0,0,0,.8), 0 0 0 2px #ff6a33; }
  .icono { color: var(--brasa); }
  .pantallita { display: grid; place-items: center; width: 100%; aspect-ratio: 16 / 9; border-radius: 9px; overflow: hidden; background: #050505; box-shadow: 0 0 0 2px #2c2925; }
  .pantallita img { width: 100%; height: 100%; object-fit: cover; }
  .globo { padding: 4px 8px; border-radius: 9px; background: #f3eee8; color: #1a1714; font: 800 11px/1.1 var(--marca); max-width: 90%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tipo b { font: 900 12px/1 var(--marca); text-transform: uppercase; letter-spacing: .06em; color: var(--c-papel); }
  .tipo .tecla { width: 100%; padding: 0 6px; }
  @media (max-width: 860px) {
    .servicios { grid-template-columns: 1fr; }
    .tipos { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  }
  @media (max-width: 480px) { .tipos { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
