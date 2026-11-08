<script lang="ts">
  import Pagina from '$lib/consola/Pagina.svelte';
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';

  type Reserva = { id: number; nombre: string; pieza: string; contacto: string; color: string; mensaje: string; creada: string; atendida: boolean };

  let { data } = $props();
  let reservas = $state<Reserva[]>([]);
  let error = $state('');
  let soloPendientes = $state(true);

  const COLORES: Record<string, string> = { naranja: 'Naranja', negro: 'Negro', blanco: 'Blanco', carbon: 'Carbón', hueso: 'Hueso' };
  const visibles = $derived(soloPendientes ? reservas.filter((r) => !r.atendida) : reservas);

  async function cargar() {
    try {
      reservas = (await api<{ reservas: Reserva[] }>('/v1/reservas')).reservas;
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer las reservas.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function marcar(r: Reserva) {
    try {
      await api(`/v1/reservas/${r.id}`, { metodo: 'PATCH', cuerpo: { atendida: !r.atendida } });
      r.atendida = !r.atendida;
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude marcarla.';
    }
  }
</script>

<Pagina titulo="Reservas">
<div>
  {#if !data.usuario?.admin}
    <p class="aviso-caja">Solo los administradores (ADMIN_EMAILS) ven las reservas.</p>
  {:else}
    <label class="fila"><input type="checkbox" bind:checked={soloPendientes} /> Solo las que falta atender</label>
    {#if error}<p class="aviso-caja error">{error}</p>{/if}
    {#if visibles.length === 0}
      <p class="nota">No hay reservas {soloPendientes ? 'pendientes' : 'todavía'}.</p>
    {/if}
    {#each visibles as r (r.id)}
      <div class="tarjeta reserva" class:atendida={r.atendida}>
        <div>
          <h2>{r.nombre}</h2>
          <p><a href={r.contacto.includes('@') ? `mailto:${r.contacto}` : `https://wa.me/${r.contacto.replace(/\D/g, '')}`} target="_blank" rel="noopener">{r.contacto}</a></p>
          <p class="etiqueta">{r.pieza === 'broche' ? 'Broche' : 'Unidad'} · {COLORES[r.color] ?? r.color} · {new Date(r.creada).toLocaleString('es-AR')}</p>
          {#if r.mensaje}<p>{r.mensaje}</p>{/if}
        </div>
        <button class="chico" onclick={() => marcar(r)}>{r.atendida ? 'Volver a pendiente' : 'Atendida'}</button>
      </div>
    {/each}
  {/if}
</div>
</Pagina>

<style>
  .reserva { display: flex; justify-content: space-between; gap: 16px; align-items: start; }
  .reserva h2 { margin: 0 0 4px; }
  .reserva p { margin: 4px 0; }
  .atendida { opacity: 0.55; }
  .fila { display: flex; gap: 8px; align-items: center; }
</style>
