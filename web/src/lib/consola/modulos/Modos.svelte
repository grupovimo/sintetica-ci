<script lang="ts">
  // Modos: qué es el robot ahora. Mascota (se maneja solo), reloj, pomodoro,
  // el clima de su ciudad o la galería de sus imágenes. Las notificaciones
  // llegan con la nube.
  import { usarBicho } from '$lib/bicho.svelte';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Ubicacion from '../Ubicacion.svelte';

  let { modelo }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const MODOS = [
    { clave: 'mascota', nombre: 'Compañía', dice: 'Vive su vida: duerme, juega, te busca.', listo: true },
    { clave: 'reloj', nombre: 'Reloj', dice: 'La hora grande, y él mirando de reojo.', listo: true },
    { clave: 'pomodoro', nombre: 'Pomodoro', dice: 'Tandas de foco y descanso. Un toque arranca o pausa.', listo: true },
    { clave: 'clima', nombre: 'Clima', dice: 'El tiempo de tu ciudad, con su cara.', listo: true },
    { clave: 'galeria', nombre: 'Galería', dice: 'Tus imágenes, una tras otra. Un toque pasa.', listo: true },
    { clave: 'notificaciones', nombre: 'Avisos', dice: 'Lo que le mandes desde tus integraciones.', listo: false },
  ];
  const modo = $derived(b.config.modo ?? 'mascota');
  const trabajo = $derived(b.config.pomodoro?.trabajo ?? 25);
  const descanso = $derived(b.config.pomodoro?.descanso ?? 5);
  const segundos = $derived(b.config.galeria?.segundos ?? 30);
  const CADA = [[10, '10 s'], [30, '30 s'], [60, '1 min'], [300, '5 min']] as const;
  const ponerSegundos = (s: number) => g.cambiar((c) => { if (s === 30) delete c.galeria; else c.galeria = { segundos: s }; });

  const ponerModo = (m: string) => g.cambiar((c) => { if (m === 'mascota') delete c.modo; else c.modo = m; });
  function pomo(cual: 'trabajo' | 'descanso', d: number) {
    g.cambiar((c) => {
      const p = { trabajo, descanso, ...(c.pomodoro ?? {}) };
      p[cual] = Math.min(cual === 'trabajo' ? 120 : 60, Math.max(1, p[cual]! + d));
      c.pomodoro = p;
    });
  }
</script>

<div class="modos">
  <div class="teclas" role="radiogroup" aria-label="Modo">
    {#each MODOS as m}
      <button role="radio" aria-checked={modo === m.clave} class="modo" class:elegido={modo === m.clave} disabled={!m.listo} onclick={() => ponerModo(m.clave)}>
        <span class="arriba"><span class="led" class:on={modo === m.clave}></span>{#if !m.listo}<span class="pronto">Pronto</span>{:else if m.clave === 'clima' && !b.config.ubicacion}<span class="pronto">Sin ciudad</span>{/if}</span>
        <b>{m.nombre}</b>
        <small>{m.dice}</small>
      </button>
    {/each}
  </div>

  {#if modo === 'clima'}
    <Ubicacion />
  {/if}

  {#if modo === 'galeria'}
    <div class="cada" role="radiogroup" aria-label="Cada cuánto cambia">
      <span class="grabado">Cada</span>
      {#each CADA as [s, t]}
        <button role="radio" aria-checked={segundos === s} class="tecla chica" class:negra={segundos !== s} class:naranja={segundos === s} onclick={() => ponerSegundos(s)}>{t}</button>
      {/each}
    </div>
  {/if}

  {#if modo === 'pomodoro'}
    <div class="visores">
      {#each [['trabajo', 'Foco', trabajo, 5], ['descanso', 'Descanso', descanso, 1]] as [k, t, v, paso]}
        <div class="visor reloj">
          <div class="lectura"><span class="grabado">{t}</span><span class="digitos">{String(v).padStart(2, '0')}<small> min</small></span></div>
          <div class="mas-menos">
            <button class="tecla negra cuadrada" aria-label="{t}: más" onclick={() => pomo(k as 'trabajo', +paso)}>+</button>
            <button class="tecla negra cuadrada" aria-label="{t}: menos" onclick={() => pomo(k as 'trabajo', -paso)}>−</button>
          </div>
        </div>
      {/each}
    </div>
    <p class="texto">Un toque arranca o pausa.</p>
  {/if}
</div>

<style>
  .modos { display: flex; flex-direction: column; gap: 16px; }
  .teclas { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 12px; }
  .cada { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .modo { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; min-height: 150px; padding: 14px; border: 0; border-radius: 18px; cursor: pointer;
    text-align: left; color: #1a1714; font-family: var(--marca); background: linear-gradient(180deg, #fbf9f5, #e6e0d6);
    box-shadow: inset 0 1px 0 #fff, 0 6px 0 #8f887c, 0 10px 16px -6px rgba(80,20,0,.55); transition: transform .08s, box-shadow .08s; }
  .modo.elegido { transform: translateY(5px); color: #f3eee8; background: linear-gradient(180deg, #2a2622, #151310); box-shadow: 0 1px 0 #000; }
  .modo:disabled { cursor: not-allowed; opacity: .55; }
  .arriba { display: flex; width: 100%; justify-content: space-between; align-items: center; }
  .pronto { font: 800 9px/1 var(--marca); letter-spacing: .12em; text-transform: uppercase; padding: 4px 6px; border-radius: 5px; background: rgba(26,23,20,.12); }
  .modo b { font: 900 17px/1 var(--marca); font-stretch: 115%; text-transform: uppercase; letter-spacing: .01em; }
  .modo small { font: 600 13px/1.3 var(--marca); opacity: .75; }
  .visores { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .reloj { flex-direction: row; align-items: center; justify-content: space-between; }
  .lectura { display: flex; flex-direction: column; gap: 10px; }
  .digitos small { font-size: 18px; color: #8f877c; text-shadow: none; }
  .mas-menos { display: flex; flex-direction: column; gap: 10px; }
  @media (max-width: 1100px) { .teclas { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
  @media (max-width: 900px) { .teclas { grid-template-columns: repeat(2, minmax(0, 1fr)); } .modo { min-height: 120px; } }
  @media (max-width: 560px) { .visores { grid-template-columns: 1fr; } }
</style>
