<script lang="ts">
  // Cuidado: que te recuerde tomar agua, que te haga hacer una pausa en el
  // horario de trabajo, que te pregunte cómo estás, y por Telegram que te
  // extrañe y te cuente su día.
  import { usarBicho } from '$lib/bicho.svelte';
  import { CUIDADO_BASE, type Cuidado } from '$lib/formato';
  import { usarGuardado } from '../guardado.svelte';
  import type { Modelo } from '../modelo';
  import Perilla from '../Perilla.svelte';

  let { modelo: _ }: { modelo: Modelo } = $props();
  const b = usarBicho();
  const g = usarGuardado();

  const c = $derived({ ...CUIDADO_BASE, ...(b.config.cuidado ?? {}), trabajo: { ...CUIDADO_BASE.trabajo, ...(b.config.cuidado?.trabajo ?? {}) } });

  function poner(f: (x: typeof c) => void) {
    g.cambiar((cfg) => {
      const x = { ...CUIDADO_BASE, ...(cfg.cuidado ?? {}), trabajo: { ...CUIDADO_BASE.trabajo, ...(cfg.cuidado?.trabajo ?? {}) } };
      f(x);
      cfg.cuidado = x as Cuidado;
    });
  }

  const aMin = (h: string) => +h.slice(0, 2) * 60 + +h.slice(3, 5);
  const aHora = (m: number) => { const x = ((m % 1440) + 1440) % 1440; return `${String(Math.floor(x / 60)).padStart(2, '0')}:${String(x % 60).padStart(2, '0')}`; };
  function mover(cual: 'desde' | 'hasta', d: number) {
    poner((x) => {
      const nuevo = aHora(aMin(x.trabajo[cual]) + d);
      if (nuevo !== x.trabajo[cual === 'desde' ? 'hasta' : 'desde']) x.trabajo[cual] = nuevo;
    });
  }
  const DIAS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  function dia(l: string) {
    poner((x) => {
      const nuevo = DIAS.filter((d) => (d === l ? !x.trabajo.dias.includes(d) : x.trabajo.dias.includes(d))).join('');
      if (nuevo) x.trabajo.dias = nuevo;
    });
  }
  // La pausa salta de "no" a 30 minutos (menos no tiene sentido).
  const ponerPausa = (v: number) => poner((x) => (x.pausa = v <= 0 ? 0 : Math.max(30, v)));
</script>

<div class="cuidado">
  <div class="perillas">
    <div class="bandeja perilla" class:apagada={!c.agua}>
      <Perilla nombre="Agua" valor={c.agua} min={0} max={8} paso={1} tamano={72} mostrar={(v) => (v ? `${v} h` : 'No')}
        alCambiar={(v) => poner((x) => (x.agua = v))} />
    </div>
    <div class="bandeja perilla" class:apagada={!c.pausa}>
      <Perilla nombre="Pausa" valor={c.pausa} min={0} max={240} paso={15} tamano={72} mostrar={(v) => (v ? `${v}′` : 'No')}
        alCambiar={ponerPausa} />
    </div>
  </div>

  <div class="trabajo bandeja">
    <span class="grabado">Trabajo</span>
    <div class="visores">
      {#each [['desde', 'Desde'], ['hasta', 'Hasta']] as [k, t]}
        <div class="visor reloj">
          <div class="lectura"><span class="grabado">{t}</span><span class="digitos">{c.trabajo[k as 'desde']}</span></div>
          <div class="mas-menos">
            <button class="tecla negra cuadrada" aria-label="{t}: 30 minutos más tarde" onclick={() => mover(k as 'desde', 30)}>+</button>
            <button class="tecla negra cuadrada" aria-label="{t}: 30 minutos más temprano" onclick={() => mover(k as 'desde', -30)}>−</button>
          </div>
        </div>
      {/each}
    </div>
    <div class="dias" role="group" aria-label="Días de trabajo">
      {#each DIAS as l}<button class="tecla chica dia-t" class:naranja={c.trabajo.dias.includes(l)} aria-pressed={c.trabajo.dias.includes(l)} onclick={() => dia(l)}>{l}</button>{/each}
    </div>
  </div>

  <div class="llaves">
    <button class="llave" aria-pressed={c.animo} onclick={() => poner((x) => (x.animo = !x.animo))}><span class="led"></span>¿Cómo estás?</button>
    <div class="extrana visor">
      <button class="llave" aria-pressed={c.extranar} onclick={() => poner((x) => (x.extranar = !x.extranar))}><span class="led"></span>Te extraña</button>
      <button class="tecla negra cuadrada chica-c" aria-label="Te extraña: un día menos" disabled={!c.extranar} onclick={() => poner((x) => (x.extranarDias = Math.max(1, x.extranarDias - 1)))}>−</button>
      <span class="digitos chicos" class:off={!c.extranar} aria-label="{c.extranarDias} días">{c.extranarDias}<small>d</small></span>
      <button class="tecla negra cuadrada chica-c" aria-label="Te extraña: un día más" disabled={!c.extranar} onclick={() => poner((x) => (x.extranarDias = Math.min(7, x.extranarDias + 1)))}>+</button>
    </div>
    <button class="llave" aria-pressed={c.resumen} onclick={() => poner((x) => (x.resumen = !x.resumen))}><span class="led"></span>Resumen</button>
  </div>
</div>

<style>
  .cuidado { display: flex; flex-direction: column; gap: 16px; }
  .perillas { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .perilla { display: grid; place-items: center; padding: 22px 16px; transition: opacity .2s; }
  .perilla.apagada { opacity: .7; }
  .trabajo { display: flex; flex-direction: column; gap: 12px; }
  .visores { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  .reloj { flex-direction: row; align-items: center; justify-content: space-between; }
  .lectura { display: flex; flex-direction: column; gap: 10px; }
  .mas-menos { display: flex; flex-direction: column; gap: 10px; }
  .dias { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 8px; padding-bottom: 6px; }
  .dia-t { padding: 0; font-size: 13px; }
  .llaves { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .extrana { flex-direction: row; align-items: center; gap: 8px; padding: 6px 8px 10px 6px; border-radius: 16px; }
  .extrana .llave { background: none; box-shadow: none; }
  .extrana .digitos { min-width: 2.4ch; text-align: center; }
  .extrana .digitos.off { color: #5a524a; text-shadow: none; }
  .extrana small { font-size: 13px; }
  .chica-c { width: 40px; min-height: 40px; }
  @media (max-width: 560px) { .visores { grid-template-columns: 1fr; } .digitos { font-size: 40px; } }
</style>
