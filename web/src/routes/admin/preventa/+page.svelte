<script lang="ts">
  // Preventa del cubito: los números que ve la landing y las señas de Mercado Pago.
  import { onMount } from 'svelte';
  import { api, ErrorApi } from '$lib/api';
  import { fecha, plata, type Preventa, type Sena, type Unidad } from '$lib/control/control';

  let { data } = $props();
  let p = $state<Preventa | null>(null);
  let tomados = $state(0);
  let conMP = $state(false);
  let senas = $state<Sena[]>([]);
  let unidades = $state<Unidad[]>([]);
  let error = $state('');
  let guardado = $state(false);
  let filtro = $state<'aprobada' | 'pendiente' | 'todas'>('aprobada');

  const visibles = $derived(filtro === 'todas' ? senas : senas.filter((s) => s.estado === filtro));
  const quedan = $derived(p ? Math.max(p.cupos - tomados, 0) : 0);
  const ESTADO_SENA: Record<Sena['estado'], { nombre: string; led: string }> = {
    aprobada: { nombre: 'Pagada', led: 'verde' },
    pendiente: { nombre: 'Pagando', led: 'ambar' },
    rechazada: { nombre: 'Rechazada', led: '' },
    devuelta: { nombre: 'Devuelta', led: 'on' },
  };

  type Respuesta = { preventa: Preventa; tomados: number; conMercadoPago: boolean };
  function poner(r: Respuesta) {
    p = r.preventa;
    tomados = r.tomados;
    conMP = r.conMercadoPago;
  }

  async function cargar() {
    try {
      const [r, s, u] = await Promise.all([
        api<Respuesta>('/v1/admin/preventa'),
        api<{ senas: Sena[] }>('/v1/admin/senas'),
        api<{ unidades: Unidad[] }>('/v1/admin/unidades'),
      ]);
      poner(r);
      senas = s.senas;
      unidades = u.unidades.filter((x) => x.modelo === 'unidad');
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude traer la preventa.';
    }
  }
  onMount(() => {
    if (data.usuario?.admin) cargar();
  });

  async function guardar(campos: Partial<Preventa>) {
    if (!p) return;
    const n = { ...p, ...campos };
    try {
      poner(await api<Respuesta>('/v1/admin/preventa', {
        metodo: 'PUT',
        cuerpo: { activa: n.activa, precio: n.precio, precioFinal: n.precioFinal, sena: n.sena, cupos: n.cupos, entrega: n.entrega },
      }));
      guardado = true;
      setTimeout(() => (guardado = false), 1500);
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude guardarla.';
      p = { ...p };
    }
  }

  async function cambiarSena(s: Sena, cuerpo: { estado?: string; unidadId?: number }) {
    try {
      const n = await api<Sena>(`/v1/admin/senas/${s.id}`, { metodo: 'PATCH', cuerpo });
      senas = senas.map((x) => (x.id === n.id ? n : x));
      error = '';
    } catch (e) {
      error = e instanceof ErrorApi ? e.message : 'No pude cambiarla.';
    }
  }

  const numero = (e: Event) => Number((e.currentTarget as HTMLInputElement).value) || 0;
  const contacto = (c: string) => (c.includes('@') ? `mailto:${c}` : `https://wa.me/${c.replace(/\D/g, '')}`);
</script>

{#if error}<p class="aviso error"><span class="led"></span>{error}</p>{/if}

{#if p}
  <section class="placa-naranja tablero">
    <div class="cabeza">
      <button class="llave" aria-pressed={p.activa} disabled={!conMP} onclick={() => guardar({ activa: !p!.activa })}>
        <span class="led"></span>{p.activa ? 'Abierta en la landing' : 'Cerrada'}
      </button>
      <span class="estado oscuro"><span class="led" class:on={guardado}></span>{guardado ? 'Guardado' : 'Guarda solo'}</span>
    </div>
    {#if !conMP}
      <p class="aviso"><span class="led ambar"></span>Falta MP_ACCESS_TOKEN en el servidor</p>
    {/if}
    <div class="campos">
      <label class="visor"><span class="grabado">Precio preventa</span>
        <input class="campo" type="number" min="0" step="any" value={p.precio || ''} placeholder="$" onchange={(e) => guardar({ precio: numero(e) })} /></label>
      <label class="visor"><span class="grabado">Precio final</span>
        <input class="campo" type="number" min="0" step="any" value={p.precioFinal || ''} placeholder="$" onchange={(e) => guardar({ precioFinal: numero(e) })} /></label>
      <label class="visor"><span class="grabado">Seña</span>
        <input class="campo" type="number" min="0" step="any" value={p.sena || ''} placeholder="$" onchange={(e) => guardar({ sena: numero(e) })} /></label>
      <label class="visor"><span class="grabado">Cupos</span>
        <input class="campo" type="number" min="0" step="1" value={p.cupos || ''} onchange={(e) => guardar({ cupos: Math.round(numero(e)) })} /></label>
      <label class="visor"><span class="grabado">Entrega</span>
        <input class="campo" value={p.entrega} maxlength="60" placeholder="Marzo 2027" onchange={(e) => guardar({ entrega: e.currentTarget.value })} /></label>
    </div>
    <div class="visores">
      <div class="visor dato"><span class="grabado">Quedan</span><span class="digitos">{quedan}<small> / {p.cupos}</small></span></div>
      <div class="visor dato"><span class="grabado">Señado</span><span class="digitos chicos">{plata(senas.filter((s) => s.estado === 'aprobada').reduce((t, s) => t + s.monto, 0))}</span></div>
      {#if p.precioFinal > p.precio && p.precio > 0}
        <div class="visor dato"><span class="grabado">Descuento</span><span class="digitos chicos">{Math.round((1 - p.precio / p.precioFinal) * 100)}%</span></div>
      {/if}
    </div>
  </section>

  <section class="seccion">
    <div class="cabeza">
      <span class="grabado">Señas · {visibles.length}</span>
      <div class="modo" role="radiogroup" aria-label="Filtrar señas">
        <button role="radio" aria-checked={filtro === 'aprobada'} onclick={() => (filtro = 'aprobada')}>Pagadas</button>
        <button role="radio" aria-checked={filtro === 'pendiente'} onclick={() => (filtro = 'pendiente')}>Pagando</button>
        <button role="radio" aria-checked={filtro === 'todas'} onclick={() => (filtro = 'todas')}>Todas</button>
      </div>
    </div>
    {#if visibles.length === 0}<p class="vacio"><span class="led"></span>Ninguna</p>{/if}
    <div class="lista">
      {#each visibles as s (s.id)}
        <div class="fila sena">
          <div>
            <div class="titulo">{s.nombre}</div>
            <div class="sub"><a class="enlace" href={contacto(s.contacto)} target="_blank" rel="noopener">{s.contacto}</a> · {s.color} · {fecha(s.pagada ?? s.creada)}</div>
          </div>
          <span class="estado"><span class="led {ESTADO_SENA[s.estado].led}"></span>{ESTADO_SENA[s.estado].nombre}</span>
          <span class="num">{plata(s.monto)}</span>
          <div class="acciones">
            {#if s.estado === 'aprobada' || s.estado === 'devuelta'}
              <select class="tecla negra chica" value={s.unidadId ?? 0} aria-label="Unidad" onchange={(e) => cambiarSena(s, { unidadId: Number(e.currentTarget.value) })}>
                <option value={0}>Sin unidad</option>
                {#each unidades as u}<option value={u.id}>{u.serie}</option>{/each}
              </select>
              <button class="tecla negra chica" onclick={() => cambiarSena(s, { estado: s.estado === 'devuelta' ? 'aprobada' : 'devuelta' })}>
                {s.estado === 'devuelta' ? 'Deshacer' : 'Devuelta'}</button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </section>
{:else if !error}
  <p class="vacio"><span class="led on"></span>Cargando</p>
{/if}

<style>
  .tablero { display: flex; flex-direction: column; gap: 16px; }
  .estado.oscuro { color: #1a1714; }
  .digitos small { font-size: 18px; opacity: .6; }
  .sena { grid-template-columns: minmax(0, 1fr) 110px auto auto; }
  .sena select { appearance: none; }
  .llave:disabled { cursor: not-allowed; opacity: .6; }
  @media (max-width: 700px) {
    .sena { grid-template-columns: minmax(0, 1fr) auto; }
    .sena .acciones { grid-column: 1 / -1; }
  }
</style>
