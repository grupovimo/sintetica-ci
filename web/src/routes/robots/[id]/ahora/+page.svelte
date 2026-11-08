<script lang="ts">
  import { goto } from '$app/navigation';
  import { api, ErrorApi } from '$lib/api';
  import { usarBicho } from '$lib/bicho.svelte';
  import { EVENTOS, NOMBRES_EVENTO } from '$lib/formato';

  const b = usarBicho();
  let aviso = $state('');
  let apodo = $state(b.bicho?.apodo ?? '');
  let nombreNuevo = $state('');
  let quitando = $state(false);

  async function mandar(c: Parameters<typeof b.comando>[0]) {
    aviso = (await b.comando(c)) ?? '';
  }

  async function guardarApodo(e: Event) {
    e.preventDefault();
    try {
      const r = await api<{ apodo: string }>(`/v1/bichos/${b.id}`, { metodo: 'PATCH', cuerpo: { apodo } });
      if (b.bicho) b.bicho.apodo = r.apodo;
      aviso = '';
    } catch (err) {
      aviso = err instanceof ErrorApi ? err.message : 'No pude guardar el apodo.';
    }
  }

  async function quitar() {
    if (!confirm('¿Sacar este robot de tu cuenta? Se olvida del WiFi y de la nube; para volver a sumarlo hace falta la app, cerca.')) return;
    quitando = true;
    try {
      await api(`/v1/bichos/${b.id}`, { metodo: 'DELETE' });
      goto('/robots');
    } catch (err) {
      aviso = err instanceof ErrorApi ? err.message : 'No pude sacarlo.';
      quitando = false;
    }
  }

  const conectado = $derived(b.bicho?.conectado ?? false);
  const fija = $derived(b.bicho?.estado?.fija ?? false);
  const visible = $derived(b.bicho?.estado?.expresion ?? '');
</script>

<div class="grilla dos">
  <section class="tarjeta grilla">
    <div class="fila" style="justify-content: space-between">
      <h2>Ahora</h2>
      <button class="chico" aria-pressed={!fija} disabled={!conectado} onclick={() => mandar({ expresion: '' })}>Automático</button>
    </div>
    {#if !conectado}<p class="aviso-caja">El robot no está conectado: los comandos sueltos necesitan que esté en línea. Los cambios de config le llegan cuando vuelva.</p>{/if}
    <p class="etiqueta">Dejar una cara fija</p>
    <div class="botones">
      {#each b.nombresCaras as n}
        <button class="chico" disabled={!conectado} aria-pressed={fija && visible === n} onclick={() => mandar({ expresion: n })}>{n}</button>
      {/each}
    </div>
    {#if b.nombresAnimaciones.length}
      <p class="etiqueta">Reproducir una animación</p>
      <div class="botones">
        {#each b.nombresAnimaciones as n}
          <button class="chico" disabled={!conectado} aria-pressed={visible === 'animacion:' + n} onclick={() => mandar({ reproducir: n })}>{n}</button>
        {/each}
        <button class="chico" disabled={!conectado} onclick={() => mandar({ reproducir: '' })}>Parar</button>
      </div>
    {/if}
    <p class="etiqueta">Hacer como si…</p>
    <div class="botones">
      {#each EVENTOS.filter((e) => e !== 'vinculado') as e}
        <button class="chico" disabled={!conectado} onclick={() => mandar({ evento: e })}>{NOMBRES_EVENTO[e]}</button>
      {/each}
    </div>
    {#if aviso}<p class="aviso-caja error">{aviso}</p>{/if}
    {#each b.respuestas.slice(0, 1) as r (r.cuando)}<p class="nota">{r.texto}</p>{/each}
  </section>

  <section class="grilla">
    <div class="tarjeta grilla">
      <h2>Cómo se llama</h2>
      <form class="fila" onsubmit={guardarApodo}>
        <label style="flex: 1"><span class="etiqueta">Apodo en tu lista</span><input bind:value={apodo} maxlength="40" placeholder={b.bicho?.estado?.nombre ?? ''} /></label>
        <button style="align-self: end">Guardar</button>
      </form>
      <form class="fila" onsubmit={(e) => { e.preventDefault(); mandar({ nombre: nombreNuevo }); nombreNuevo = ''; }}>
        <label style="flex: 1"><span class="etiqueta">Nombre del robot (comando, hasta que cambie la config)</span><input bind:value={nombreNuevo} maxlength="24" /></label>
        <button style="align-self: end" disabled={!conectado || !nombreNuevo}>Cambiar</button>
      </form>
      <p class="nota">Para que quede siempre, cambialo en <a href="/robots/{b.id}/config">Config</a>.</p>
    </div>
    <div class="tarjeta grilla">
      <h2>Cuenta</h2>
      <p class="nota">Está en tu cuenta desde el {b.bicho ? new Date(b.bicho.creado).toLocaleDateString('es-AR') : ''}. {b.bicho?.ultimaConexion ? `Se conectó por última vez el ${new Date(b.bicho.ultimaConexion).toLocaleString('es-AR')}.` : ''}</p>
      <button class="peligro" style="justify-self: start" onclick={quitar} disabled={quitando}>Sacar de mi cuenta</button>
    </div>
  </section>
</div>

<style>
  .dos { grid-template-columns: 1.3fr 1fr; align-items: start; }
  .botones { display: flex; flex-wrap: wrap; gap: 6px; }
  @media (max-width: 900px) { .dos { grid-template-columns: 1fr; } }
</style>
