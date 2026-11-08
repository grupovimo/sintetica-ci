<script lang="ts">
  // La landing comercial ("Tu propio robot"): diseño en el lienzo
  // https://claude.ai/artifact/17XSJuSjqfZf35xCUdQKVA (B). Las caras y la
  // pantalla en vivo las dibuja el mismo núcleo del robot.
  import { onDestroy, onMount } from 'svelte';
  import Lenis from 'lenis';
  import { api, ErrorApi } from '$lib/api';
  import Pantalla from '$lib/componentes/Pantalla.svelte';
  import Foto from '$lib/landing/Foto.svelte';
  import { fotos } from '$lib/landing/caras';

  let { data } = $props();

  // Los links de las tiendas, cuando las apps estén publicadas.
  const APP_STORE = '';
  const GOOGLE_PLAY = '';

  const PERSONALIDADES = [
    ['picaro', 'Pícaro'],
    ['timido', 'Tímido'],
    ['carinoso', 'Cariñoso'],
    ['curioso', 'Curioso'],
    ['dormilon', 'Dormilón'],
    ['grunon', 'Gruñón'],
    ['electrico', 'Eléctrico'],
    ['equilibrado', 'Equilibrado'],
  ] as const;
  const COLORES = [
    ['naranja', 'Naranja', 'radial-gradient(circle at 30% 25%, #ffb08a, #ff5a1f 50%, #b83a0a)'],
    ['negro', 'Negro', 'radial-gradient(circle at 30% 25%, #666, #1a1a1a 50%, #000)'],
    ['blanco', 'Blanco', 'radial-gradient(circle at 30% 25%, #fff, #ebe8e2 50%, #b9b4ab)'],
  ] as const;

  let personalidad = $state('picaro');
  const configVivo = $derived({ version: 2, personalidad: { preset: personalidad } });
  let caras = $state<{ nombre: string; url: string }[]>([]);

  // La reserva
  let pieza = $state<'unidad' | 'broche'>('unidad');
  let color = $state('naranja');
  let nombre = $state('');
  let contacto = $state('');
  let sitio = $state(''); // trampa para bots: la persona no lo ve
  let enviando = $state(false);
  let reservado = $state(false);
  let errorReserva = $state('');

  async function reservar(e: Event) {
    e.preventDefault();
    enviando = true;
    errorReserva = '';
    try {
      await api('/v1/reservas', { cuerpo: { nombre, pieza, contacto, color, mensaje: '', sitio } });
      reservado = true;
    } catch (err) {
      errorReserva = err instanceof ErrorApi ? err.message : 'No pude mandarla. Probá de nuevo.';
    } finally {
      enviando = false;
    }
  }

  function elegirPieza(p: 'unidad' | 'broche') {
    pieza = p;
  }

  // Movimiento: scroll suave, parallax y apariciones. Nada si la persona
  // pidió menos movimiento.
  let raiz: HTMLElement;
  let lenis: Lenis | null = null;
  let cuadro = 0;
  let observador: IntersectionObserver | null = null;

  onMount(() => {
    fotos(['contento', 'enamorado', 'mareado', 'sospecha'])
      .then((f) => (caras = f))
      .catch(() => {});
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) {
      raiz.classList.add('quieto');
      return;
    }
    lenis = new Lenis({ lerp: 0.09, smoothWheel: true });
    raiz.querySelectorAll<HTMLAnchorElement>('a[href^="#"]').forEach((a) =>
      a.addEventListener('click', (e) => {
        const destino = raiz.querySelector(a.getAttribute('href')!);
        if (!destino) return;
        e.preventDefault();
        lenis?.scrollTo(destino as HTMLElement, { offset: -16, duration: 1.4 });
      }),
    );
    const moviles = [...raiz.querySelectorAll<HTMLElement>('[data-parallax]')];
    const vuelta = (t: number) => {
      lenis?.raf(t);
      const mitad = innerHeight / 2;
      for (const el of moviles) {
        const r = el.getBoundingClientRect();
        if (r.bottom < -200 || r.top > innerHeight + 200) continue;
        const d = (r.top + r.height / 2 - mitad) * Number(el.dataset.parallax);
        el.style.setProperty('--py', `${d.toFixed(1)}px`);
      }
      cuadro = requestAnimationFrame(vuelta);
    };
    cuadro = requestAnimationFrame(vuelta);
    observador = new IntersectionObserver(
      (es) => es.forEach((e) => e.isIntersecting && (e.target.classList.add('visto'), observador?.unobserve(e.target))),
      { rootMargin: '0px 0px -10% 0px' },
    );
    raiz.querySelectorAll('.aparece').forEach((el) => observador!.observe(el));
  });

  onDestroy(() => {
    cancelAnimationFrame(cuadro);
    lenis?.destroy();
    observador?.disconnect();
  });
</script>

<svelte:head>
  <title>Sintética · Tu propio robot</title>
  <meta name="description" content="Robots y joyería electrónica, hechos a mano en Mar del Plata. Ya es normal tener uno." />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Archivo:wdth,wght@62..125,400..900&display=swap" />
</svelte:head>

<div class="landing" bind:this={raiz}>
  <div class="grano" aria-hidden="true"></div>

  <!-- 1 · Portada -->
  <section class="portada">
    <div class="puntos" data-parallax="0.08" aria-hidden="true"></div>
    <header class="contenedor nav">
      <a class="marca" href="#inicio" aria-label="Sintética, inicio">
        <svg width="40" height="24" viewBox="0 0 40 26" aria-hidden="true"><rect x="4" y="3" width="11" height="20" rx="5.5" fill="currentColor" /><path d="M25 8 L36 3 V18 A5.5 5.5 0 0 1 25 18 Z" fill="currentColor" /></svg>
        <span>SINTÉTICA</span>
      </a>
      <nav aria-label="Secciones">
        <a class="ficha" href="#piezas">Unidades</a>
        <a class="ficha" href="#app">App</a>
        <a class="ficha" href={data.usuario ? '/bichos' : '/entrar'}>{data.usuario ? 'Mis robots' : 'Entrar'}</a>
      </nav>
      <a class="inflado" href="#reservar">Reservar</a>
    </header>

    <div id="inicio" class="contenedor inicio">
      <h1 class="t-display aparece">Tu propio<br /><span class="cromo">robot.</span></h1>
      <div class="dos">
        <div class="aparece" data-parallax="-0.04" style="transform: translateY(var(--py, 0))">
          <Foto alt="el Broche en una campera, de noche" proporcion="16 / 10" lineas />
        </div>
        <div class="tarjeta-negra aparece">
          <p class="t-lead">Robots y joyería electrónica.<br /><span class="naranja">Ya es normal tener uno.</span></p>
          <div class="pila">
            <div class="fila"><span class="ficha">Lote 01</span><span class="ficha">Hecho a mano</span><span class="ficha">Nº grabado</span></div>
            <div class="fila">
              <a class="inflado naranja" href="#reservar">Reservar el tuyo</a>
              <a class="inflado contorno" href="#facil">Cómo funciona</a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- 2 · Así de fácil -->
  <section id="facil" class="hueso seccion">
    <div class="contenedor angosto pila-48">
      <h2 class="t-h2 aparece">Así de fácil.</h2>
      <ol class="grilla-3">
        {#each [['1', 'Sacalo de la caja', 'caja'], ['2', 'Escaneá su QR', 'qr'], ['3', 'Es tuyo', 'corazon']] as [n, texto, icono], i}
          <li class="paso aparece" style:transition-delay={`${i * 80}ms`}>
            <div class="paso-arriba">
              <span class="inflado naranja numero" aria-hidden="true">{n}</span>
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#111" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                {#if icono === 'caja'}<path d="M3 8l9-5 9 5v8l-9 5-9-5z" /><path d="M3 8l9 5 9-5M12 13v8" />
                {:else if icono === 'qr'}<rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><path d="M14 14h3v3h-3zM20 14v.01M14 20h.01M17 20h4v-3" />
                {:else}<path d="M12 21s-7-4.5-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 11c0 5.5-7 10-7 10z" />{/if}
              </svg>
            </div>
            <span class="t-h4">{texto}</span>
          </li>
        {/each}
      </ol>
    </div>
  </section>

  <!-- 3 · Las piezas -->
  <section id="piezas" class="hueso seccion sin-arriba">
    <div class="contenedor angosto grilla-2">
      <article class="pieza aparece">
        <Foto alt="la Unidad en el escritorio" clara />
        <div class="pie-pieza">
          <div><h3 class="t-h3">Unidad</h3><span class="detalle">Robot de escritorio · [PRECIO]</span></div>
          <a class="inflado" href="#reservar" onclick={() => elegirPieza('unidad')}>Reservar</a>
        </div>
      </article>
      <article class="pieza aparece" style:transition-delay="80ms">
        <Foto src="/landing/hebilla.jpg" alt="El Broche como hebilla naranja de cinturón, con una carita guiñando en la pantalla" />
        <div class="pie-pieza">
          <div><h3 class="t-h3">Broche</h3><span class="detalle">Joyería electrónica · [PRECIO]</span></div>
          <a class="inflado" href="#reservar" onclick={() => elegirPieza('broche')}>Reservar</a>
        </div>
      </article>
    </div>
  </section>

  <!-- 3b · Joyería electrónica -->
  <section id="joyeria" class="crema seccion">
    <div class="contenedor angosto app">
      <div class="pila-40">
        <div class="pila">
          <span class="ficha arriba-sola">Joyería electrónica</span>
          <h2 class="t-h2 aparece">Llevalo<br />puesto.</h2>
        </div>
        <div class="fila chica">
          {#each ['Hebilla', 'Colgante', 'Alfiler', 'Hebilla de pelo', 'Broche'] as tipo}<span class="ficha">{tipo}</span>{/each}
        </div>
        <div class="fila"><a class="inflado" href="#reservar" onclick={() => elegirPieza('broche')}>Reservar un Broche</a></div>
      </div>
      <div class="collage">
        <div class="aparece" data-parallax="0.04" style="transform: translateY(var(--py, 0px))">
          <Foto src="/landing/coleccion.jpg" alt="La colección: hebilla de cinturón, colgante, broche translúcido, alfiler y hebilla de pelo, cada uno con una carita luminosa y su número" radio="32px" />
        </div>
        <div class="aparece baja" data-parallax="-0.04" style="transform: translateY(var(--py, 0px))">
          <Foto src="/landing/hebilla.jpg" alt="El Broche como hebilla de cinturón, puesto" radio="32px" />
        </div>
      </div>
    </div>
  </section>

  <!-- 4 · Hacelo tuyo -->
  <section id="tuyo" class="negra seccion">
    <div class="contenedor angosto pila-48">
      <h2 class="t-h2 naranja aparece">Hacelo<br />tuyo.</h2>
      <div class="grilla-3">
        <figure class="opcion aparece"><Foto alt="imagen vintage en pantalla" proporcion="16 / 10" radio="28px" /><figcaption class="t-h4">Tu imagen</figcaption></figure>
        <figure class="opcion aparece" style:transition-delay="80ms"><Foto alt="mini GIF en el Broche" proporcion="16 / 10" radio="28px" video /><figcaption class="t-h4">Un GIF</figcaption></figure>
        <figure class="opcion aparece" style:transition-delay="160ms">
          <div class="caras">
            {#each caras as c}<img src={c.url} alt={'Cara ' + c.nombre} width="320" height="172" />{/each}
          </div>
          <figcaption class="t-h4">Tus caras</figcaption>
        </figure>
      </div>
    </div>
  </section>

  <!-- 5 · Personalidad -->
  <section class="negra seccion sin-arriba">
    <div class="contenedor angosto personalidad">
      <div class="pila-32">
        <h2 class="t-h2 aparece">Elegí<br />cómo es.</h2>
        <div class="fila" role="group" aria-label="Personalidades">
          {#each PERSONALIDADES as [clave, etiqueta]}
            <button type="button" class={personalidad === clave ? 'inflado naranja' : 'inflado contorno'} aria-pressed={personalidad === clave} onclick={() => (personalidad = clave)}>{etiqueta}</button>
          {/each}
        </div>
      </div>
      <div class="vivo aparece">
        <Pantalla config={configVivo} ancho={640} etiqueta="La cara de un robot de verdad, en vivo, con la personalidad elegida" />
        <span class="ficha"><i class="punto"></i>En vivo</span>
      </div>
    </div>
  </section>

  <!-- 6 · La app -->
  <section id="app" class="hueso seccion">
    <div class="contenedor angosto app">
      <div class="pila-40">
        <h2 class="t-h2 aparece">Todo<br />desde<br />el celu.</h2>
        <ul class="funciones">
          <li class="aparece"><span class="inflado naranja icono" aria-hidden="true"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#111" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="18" height="12" rx="3" /><path d="M9 10v4M15 10v4" /></svg></span><span class="t-h4">Tus robots, en vivo</span></li>
          <li class="aparece"><span class="inflado naranja icono" aria-hidden="true"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#111" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" /></svg></span><span class="t-h4">Su personalidad</span></li>
          <li class="aparece"><span class="inflado naranja icono" aria-hidden="true"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#111" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="3" /><circle cx="9" cy="9" r="2" /><path d="M21 15l-5-5L5 21" /></svg></span><span class="t-h4">Imágenes y GIFs</span></li>
        </ul>
        <div class="fila">
          <a class="tienda" href={APP_STORE || '#app'} aria-label="Descárgalo en el App Store">
            <svg width="30" height="30" viewBox="0 0 24 24" fill="#fff" aria-hidden="true"><path d="M16.37 1.43c0 1.14-.49 2.27-1.18 3.08-.74.9-1.99 1.57-2.99 1.57-.12 0-.23-.02-.3-.03-.01-.06-.04-.22-.04-.39 0-1.15.57-2.27 1.21-2.98.8-.94 2.14-1.64 3.25-1.68.03.13.05.28.05.43zm4.56 15.71c-.03.07-.46 1.58-1.52 3.12-.94 1.34-1.94 2.71-3.43 2.71-1.52 0-1.9-.88-3.63-.88-1.7 0-2.3.91-3.67.91-1.38 0-2.33-1.26-3.43-2.8C4 18.38 2.96 15.57 2.96 12.92c0-4.28 2.8-6.55 5.55-6.55 1.45 0 2.68.95 3.6.95.87 0 2.22-1.01 3.9-1.01.61 0 2.89.06 4.37 2.19-.13.09-2.38 1.37-2.38 4.19 0 3.26 2.85 4.42 2.96 4.45z" /></svg>
            <span class="tienda-texto apple"><small>Descárgalo en el</small><b>App Store</b></span>
          </a>
          <a class="tienda" href={GOOGLE_PLAY || '#app'} aria-label="Disponible en Google Play">
            <svg width="28" height="30" viewBox="0 0 22 24" aria-hidden="true"><path d="M1.1.5C.8.8.6 1.3.6 1.9v20.2c0 .6.2 1.1.5 1.4l.1.1L12.5 12.3v-.3L1.2.4z" fill="#4285F4" /><path d="M16.3 16.1l-3.8-3.8v-.3l3.8-3.8.1.1 4.5 2.6c1.3.7 1.3 1.9 0 2.6l-4.5 2.6z" fill="#FBBC04" /><path d="M16.4 16.1L12.5 12.2 1.1 23.5c.4.4 1.1.5 1.9.1l13.4-7.5" fill="#EA4335" /><path d="M16.4 8.3L3 .8C2.2.3 1.5.4 1.1.8l11.4 11.4z" fill="#34A853" /></svg>
            <span class="tienda-texto"><small>DISPONIBLE EN</small><b>Google Play</b></span>
          </a>
        </div>
      </div>

      <div class="escenario aparece">
        <div class="puntos" aria-hidden="true"></div>
        <div class="sombra-piso" aria-hidden="true"></div>
        <div class="telefono izq" data-parallax="0.05" style="transform: translateY(var(--py, 0px)) rotate(-8deg)">
          <i class="boton b1"></i><i class="boton b2"></i><i class="boton b3"></i>
          <div class="bisel"><div class="pantalla-tel" role="img" aria-label="La app: Mis robots, con la Unidad conectada y el Broche cerca">
            <span class="isla"></span>
            <div class="tel-cuerpo">
              <span class="tel-titulo">Mis robots</span>
              <div class="tel-tarjeta col">
                <svg viewBox="0 0 160 86" class="tel-cara" aria-hidden="true"><rect x="52" y="22" width="14" height="40" rx="7" fill="#ff6a33" /><rect x="94" y="22" width="14" height="40" rx="7" fill="#ff6a33" /></svg>
                <div class="entre"><b>Unidad #001</b><small class="ok">● Conectada</small></div>
              </div>
              <div class="tel-tarjeta">
                <svg viewBox="0 0 60 60" width="44" height="44" aria-hidden="true"><circle cx="30" cy="30" r="29" fill="#0b0a09" /><rect x="19" y="20" width="7" height="20" rx="3.5" fill="#ff6a33" /><rect x="34" y="20" width="7" height="20" rx="3.5" fill="#ff6a33" /></svg>
                <div class="col"><b>Broche #014</b><small class="cerca">● Cerca</small></div>
              </div>
            </div>
            <div class="pestanas"><span class:activa={true}><i></i>Robots</span><span class:activa={false}><i></i>Caras</span><span class:activa={false}><i></i>Imágenes</span><span class:activa={false}><i></i>Cuenta</span></div>
            <span class="reflejo"></span>
          </div></div>
        </div>
        <div class="telefono der" data-parallax="0.05" style="transform: translateY(var(--py, 0px)) rotate(8deg)">
          <i class="boton b1"></i><i class="boton b2"></i><i class="boton b3"></i>
          <div class="bisel"><div class="pantalla-tel" role="img" aria-label="La app: imágenes y GIFs para la pantalla del robot">
            <span class="isla"></span>
            <div class="tel-cuerpo">
              <span class="tel-titulo">Imágenes</span>
              <div class="miniaturas">
                <span style="background: linear-gradient(135deg, #e8c39e, #8b5e3c)"></span>
                <span style="background: linear-gradient(135deg, #7bd3ff, #3d2c8d)"><em>GIF</em></span>
                <span style="background: linear-gradient(135deg, #b6ff5a, #1f6b3a)"></span>
                <span style="background: linear-gradient(135deg, #ffd1f4, #c2410c)"></span>
                <span class="sumar">+</span>
              </div>
            </div>
            <div class="pestanas"><span class:activa={false}><i></i>Robots</span><span class:activa={false}><i></i>Caras</span><span class:activa={true}><i></i>Imágenes</span><span class:activa={false}><i></i>Cuenta</span></div>
            <span class="reflejo"></span>
          </div></div>
        </div>
        <div class="telefono centro" data-parallax="-0.04" style="transform: translateY(var(--py, 0px))">
          <i class="boton b1"></i><i class="boton b2"></i><i class="boton b3"></i>
          <div class="bisel"><div class="pantalla-tel" role="img" aria-label="La app: la Unidad, con su cara en vivo y la personalidad Pícaro">
            <span class="isla"></span>
            <div class="tel-cuerpo">
              <div class="entre"><span class="tel-titulo chico">Unidad #001</span><small class="vivo-chip">● En vivo</small></div>
              <div class="marco-cara"><svg viewBox="0 0 160 86" class="tel-cara" aria-hidden="true"><path d="M50 52 Q59 26 68 52 M92 52 Q101 26 110 52" fill="none" stroke="#ff6a33" stroke-width="8" stroke-linecap="round" /></svg></div>
              <div class="fila chica"><span class="pildora oscura">Contento</span><span class="pildora">Jugando</span></div>
              <b class="tel-etiqueta">Personalidad</b>
              <div class="fila chica"><span class="pildora activa">Pícaro</span><span class="pildora">Tímido</span><span class="pildora">Curioso</span></div>
              <div class="tel-tarjeta col barras">
                {#each [['Energía', 72], ['Curiosidad', 58]] as [n, v]}
                  <div class="col"><small>{n}</small><span class="barra" style:--v={`${v}%`}></span></div>
                {/each}
              </div>
            </div>
            <div class="pestanas"><span class:activa={true}><i></i>Robots</span><span class:activa={false}><i></i>Caras</span><span class:activa={false}><i></i>Imágenes</span><span class:activa={false}><i></i>Cuenta</span></div>
            <span class="reflejo"></span>
          </div></div>
        </div>
      </div>
    </div>
  </section>

  <!-- 7 · Reserva -->
  <section id="reservar" class="seccion reserva">
    <div class="puntos" data-parallax="0.08" aria-hidden="true"></div>
    <div class="contenedor angosto pila-48">
      <div class="entre abajo">
        <h2 class="t-display aparece">Reservá<br />el tuyo.</h2>
        <div class="fila chica"><span class="ficha">Lote 01</span><span class="ficha">Sin pago ahora</span></div>
      </div>
      {#if reservado}
        <div class="formulario listo">
          <p class="t-lead">¡Listo, {nombre.split(' ')[0]}!</p>
          <p class="detalle-claro">Te escribimos a <b>{contacto}</b> para coordinar tu {pieza === 'broche' ? 'Broche' : 'Unidad'}.</p>
        </div>
      {:else}
        <form class="formulario banda" onsubmit={reservar}>
          <label>Nombre<input bind:value={nombre} required maxlength="80" autocomplete="name" /></label>
          <label>Email o WhatsApp<input bind:value={contacto} required minlength="5" maxlength="120" autocomplete="email" /></label>
          <fieldset>
            <legend>Pieza y color</legend>
            <div class="selector">
              <button type="button" class={pieza === 'unidad' ? 'inflado blanco mini' : 'inflado plano mini'} aria-pressed={pieza === 'unidad'} onclick={() => (pieza = 'unidad')}>Unidad</button>
              <button type="button" class={pieza === 'broche' ? 'inflado blanco mini' : 'inflado plano mini'} aria-pressed={pieza === 'broche'} onclick={() => (pieza = 'broche')}>Broche</button>
              <span class="relleno"></span>
              {#each COLORES as [clave, etiqueta, fondo]}
                <button type="button" class="muestra" style:background={fondo} aria-label={etiqueta} aria-pressed={color === clave} onclick={() => (color = clave)}></button>
              {/each}
            </div>
          </fieldset>
          <label class="trampa" aria-hidden="true">Sitio<input bind:value={sitio} tabindex="-1" autocomplete="off" /></label>
          <button class="inflado naranja enviar" disabled={enviando}>{enviando ? 'Mandando…' : 'Reservar'}</button>
          {#if errorReserva}<p class="error" role="alert">{errorReserva}</p>{/if}
        </form>
      {/if}
    </div>
  </section>

  <footer class="pie">
    <div class="contenedor angosto entre">
      <span>SINTÉTICA LABS · MAR DEL PLATA</span>
      <a href="https://instagram.com/sintetica.labs" target="_blank" rel="noopener">@sintetica.labs</a>
    </div>
  </footer>
</div>

<style>
  .landing { position: relative; overflow-x: clip; background: #ff5a1f; color: #111; font-family: 'Archivo', system-ui, sans-serif; -webkit-font-smoothing: antialiased; color-scheme: light; }
  .landing :global(*) { box-sizing: border-box; }
  .landing a { text-decoration: none; }
  .landing a:not(.inflado):not(.tienda) { color: inherit; }
  .landing :global(:focus-visible) { outline: 3px solid #fff; outline-offset: 3px; }

  /* Textura: grano que se mueve y la trama de puntos. */
  .grano { position: fixed; inset: -50%; z-index: 60; pointer-events: none; opacity: 0.07; mix-blend-mode: multiply; background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='200' height='200'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='3' stitchTiles='stitch'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>"); animation: grano 1s steps(5) infinite; }
  @keyframes grano { 0% { transform: translate(0, 0); } 25% { transform: translate(-2%, 3%); } 50% { transform: translate(3%, -2%); } 75% { transform: translate(-3%, -3%); } 100% { transform: translate(0, 0); } }
  .puntos { position: absolute; inset: -80px 0; pointer-events: none; opacity: 0.5; background-image: radial-gradient(rgba(0, 0, 0, 0.16) 1.2px, transparent 1.6px); background-size: 8px 8px; transform: translateY(var(--py, 0)); }

  /* Escala tipográfica (200 / 128 / 64 / 36) y grilla de 8 px. */
  .t-display { margin: 0; font: 900 clamp(72px, 11.7vw, 168px) / 0.9 'Archivo', sans-serif; font-stretch: 125%; letter-spacing: -0.05em; text-transform: uppercase; }
  .t-h2 { margin: 0; font: 900 clamp(56px, 8.9vw, 128px) / 0.92 'Archivo', sans-serif; font-stretch: 125%; letter-spacing: -0.05em; text-transform: uppercase; }
  .t-h3 { margin: 0; font: 900 clamp(40px, 4.4vw, 64px) / 0.9 'Archivo', sans-serif; font-stretch: 125%; letter-spacing: -0.04em; text-transform: uppercase; }
  .t-h4 { margin: 0; font: 900 clamp(28px, 2.5vw, 36px) / 0.95 'Archivo', sans-serif; font-stretch: 120%; letter-spacing: -0.03em; text-transform: uppercase; }
  .t-lead { margin: 0; font: 800 clamp(28px, 3vw, 44px) / 1.02 'Archivo', sans-serif; font-stretch: 110%; letter-spacing: -0.03em; }
  .naranja { color: #ff5a1f; }
  .cromo { background: linear-gradient(100deg, #fff 0%, #c8f1ff 22%, #ffd1f4 44%, #fff6c2 62%, #fff 80%); background-size: 200% 100%; -webkit-background-clip: text; background-clip: text; color: transparent; filter: drop-shadow(0 4px 0 #111); animation: cromo 6s ease-in-out infinite alternate; }
  @keyframes cromo { to { background-position: 100% 0; } }

  .contenedor { position: relative; z-index: 1; max-width: 1312px; margin: 0 auto; padding: 0 40px; }
  .contenedor.angosto { max-width: 1312px; }
  .seccion { position: relative; padding: 128px 0; }
  .seccion.sin-arriba { padding-top: 0; }
  .hueso { background: #efece6; }
  .crema { background: #f6f3ee; }
  .arriba-sola { align-self: flex-start; }
  .collage { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; align-items: start; }
  .collage .baja { margin-top: 64px; }
  .collage :global(.foto) { box-shadow: 0 32px 48px -32px rgba(0, 0, 0, 0.45); }
  .negra { background: #111; color: #fff; }
  .pila { display: flex; flex-direction: column; gap: 24px; }
  .pila-32 { display: flex; flex-direction: column; gap: 32px; }
  .pila-48 { display: flex; flex-direction: column; gap: 48px; }
  .pila-64 { display: flex; flex-direction: column; gap: 64px; }
  .fila { display: flex; gap: 16px; flex-wrap: wrap; align-items: center; }
  .fila.chica { gap: 8px; }
  .col { display: flex; flex-direction: column; gap: 6px; }
  .entre { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
  .grilla-2 { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 480px), 1fr)); gap: 24px; }
  .grilla-2.centrado { align-items: center; gap: 48px; }
  .grilla-3 { margin: 0; padding: 0; list-style: none; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr)); gap: 24px; }

  /* Botones inflados y fichas */
  .inflado { display: inline-flex; align-items: center; justify-content: center; gap: 10px; min-height: 56px; padding: 0 32px; border-radius: 999px; border: 0; font: 800 16px/1 'Archivo', sans-serif; font-stretch: 112%; text-transform: uppercase; letter-spacing: 0.02em; cursor: pointer; color: #fff; background: radial-gradient(130% 150% at 30% 0%, #4a4a4a 0%, #161616 45%, #000 100%); box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.35), inset 0 -10px 16px rgba(0, 0, 0, 0.6), 0 16px 24px -12px rgba(0, 0, 0, 0.6), 0 3px 0 #000; transition: transform 0.25s cubic-bezier(0.3, 1.7, 0.45, 1); }
  .inflado:hover { transform: translateY(-2px) scale(1.03); }
  .inflado:active { transform: translateY(2px) scale(0.97); }
  .inflado:disabled { opacity: 0.6; cursor: progress; }
  .inflado.blanco { color: #111; background: radial-gradient(130% 150% at 30% 0%, #fff 0%, #f1efea 45%, #c9c5bd 100%); box-shadow: inset 0 2px 1px #fff, inset 0 -10px 16px rgba(0, 0, 0, 0.18), 0 16px 24px -12px rgba(0, 0, 0, 0.45), 0 3px 0 rgba(0, 0, 0, 0.3); }
  .inflado.naranja { color: #111; background: radial-gradient(130% 150% at 30% 0%, #ffb08a 0%, #ff5a1f 45%, #c2410c 100%); box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.6), inset 0 -10px 16px rgba(110, 30, 0, 0.45), 0 16px 24px -12px rgba(194, 65, 12, 0.7), 0 3px 0 rgba(0, 0, 0, 0.3); }
  .inflado.contorno { box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.25), 0 0 0 2px #3a3a3a; }
  .inflado.chico { min-height: 48px; padding: 0 24px; font-size: 14px; }
  .ficha { display: inline-flex; align-items: center; gap: 8px; min-height: 40px; padding: 0 18px; border-radius: 999px; font: 700 15px/1 'Archivo', sans-serif; font-stretch: 110%; color: #111; background: radial-gradient(130% 150% at 30% 0%, #fff 0%, #f4f2ee 60%, #dcd8d0 100%); box-shadow: inset 0 2px 1px #fff, inset 0 -6px 10px rgba(0, 0, 0, 0.12), 0 8px 16px -10px rgba(0, 0, 0, 0.5); }
  a.ficha:hover { transform: translateY(-1px); }

  /* Apariciones al scrollear */
  .aparece { opacity: 0; translate: 0 32px; transition: opacity 0.7s ease, translate 0.9s cubic-bezier(0.2, 0.8, 0.2, 1); }
  .aparece:global(.visto) { opacity: 1; translate: 0 0; }
  .landing:global(.quieto) .aparece { opacity: 1; translate: none; transition: none; }

  /* 1 · Portada */
  .portada { position: relative; overflow: hidden; }
  .nav { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; padding-top: 24px; padding-bottom: 24px; }
  .marca { display: flex; align-items: center; gap: 8px; font: 900 22px/1 'Archivo', sans-serif; font-stretch: 125%; letter-spacing: -0.02em; }
  .nav .ficha, .nav .inflado { min-height: 48px; padding: 0 24px; font-size: 15px; }
  .nav nav { flex: 1; display: flex; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
  .inicio { display: flex; flex-direction: column; gap: 32px; padding-top: 32px; padding-bottom: 128px; }
  .dos { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 400px), 1fr)); gap: 24px; }
  .tarjeta-negra { display: flex; flex-direction: column; justify-content: space-between; gap: 32px; padding: 40px; border-radius: 40px; background: #111; color: #fff; box-shadow: 0 32px 48px -32px rgba(0, 0, 0, 0.7); }

  /* 2 y 3 */
  .paso { display: flex; flex-direction: column; gap: 24px; padding: 32px; border-radius: 40px; background: #fff; box-shadow: inset 0 -10px 16px rgba(0, 0, 0, 0.06), 0 24px 40px -28px rgba(0, 0, 0, 0.5); }
  .paso-arriba { display: flex; justify-content: space-between; align-items: center; }
  .numero { width: 64px; min-height: 64px; padding: 0; font-size: 24px; }
  .pieza { display: flex; flex-direction: column; gap: 24px; }
  .pie-pieza { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; flex-wrap: wrap; }
  .detalle { display: block; margin-top: 8px; font: 700 18px/1.2 'Archivo', sans-serif; color: #4a4741; }

  /* 4 y 5 */
  .opcion { margin: 0; display: flex; flex-direction: column; gap: 16px; padding: 16px; border-radius: 40px; background: #1d1d1d; box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08); }
  .opcion figcaption { padding: 0 8px 8px; }
  .caras { aspect-ratio: 16 / 10; border-radius: 28px; background: #000; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; padding: 8px; }
  .caras img { width: 100%; height: 100%; object-fit: cover; border-radius: 20px; }
  .personalidad { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 480px), 1fr)); gap: 48px; align-items: center; padding-top: 128px; border-top: 1px solid #2a2a2a; }
  .vivo { position: relative; padding: 16px; border-radius: 40px; background: #1d1d1d; box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08); }
  .vivo :global(.placa) { width: 100% !important; max-width: none; }
  .vivo :global(canvas) { width: 100%; height: auto; border-radius: 28px; display: block; }
  .vivo .ficha { position: absolute; left: 32px; top: 32px; }
  .punto { width: 8px; height: 8px; border-radius: 50%; background: #e5402b; box-shadow: 0 0 10px #e5402b; }

  /* 6 · La app */
  .app { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 480px), 1fr)); gap: 64px; align-items: center; }
  .pila-40 { display: flex; flex-direction: column; gap: 40px; }
  /* En media columna el título va un escalón más chico: si no, no entra. */
  .app .t-h2, .personalidad .t-h2 { font-size: clamp(48px, 5.6vw, 80px); }
  .funciones { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 12px; }
  .funciones li { display: flex; align-items: center; gap: 16px; }
  .funciones .t-h4 { font-size: 28px; }
  .icono { width: 48px; min-height: 48px; padding: 0; }
  .tienda { display: inline-flex; align-items: center; gap: 12px; min-height: 64px; padding: 0 24px 0 18px; border-radius: 16px; background: #000; color: #fff !important; box-shadow: inset 0 0 0 1px #a6a6a6; transition: transform 0.2s; }
  .tienda:hover { transform: translateY(-2px); }
  .tienda-texto { display: flex; flex-direction: column; gap: 2px; font-family: Roboto, Arial, sans-serif; }
  .tienda-texto.apple { font-family: -apple-system, 'Helvetica Neue', Arial, sans-serif; }
  .tienda-texto small { font-size: 11px; line-height: 1; letter-spacing: 0.02em; }
  .tienda-texto b { font-size: 24px; font-weight: 600; line-height: 1; letter-spacing: -0.02em; }
  .escenario { position: relative; height: 720px; border-radius: 48px; overflow: hidden; background: radial-gradient(90% 70% at 50% 40%, #ff7a45 0%, #ff5a1f 45%, #d9440f 100%); box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.4), inset 0 -24px 40px rgba(110, 30, 0, 0.35); }
  .escenario .puntos { inset: 0; opacity: 0.35; transform: none; }
  .sombra-piso { position: absolute; left: 15%; right: 15%; bottom: 40px; height: 40px; border-radius: 50%; background: rgba(0, 0, 0, 0.35); filter: blur(24px); }
  .telefono { position: absolute; width: 236px; height: 500px; padding: 10px; border-radius: 40px; top: 150px; z-index: 1; background: linear-gradient(135deg, #8d8a84 0%, #e9e6df 18%, #6f6c66 45%, #c9c5bd 72%, #5f5c57 100%); box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.5), 0 60px 80px -40px rgba(0, 0, 0, 0.65), 0 20px 30px -20px rgba(0, 0, 0, 0.5); }
  .telefono.izq { left: 6%; }
  .telefono.der { right: 6%; }
  .telefono.centro { width: 280px; height: 590px; border-radius: 48px; top: 60px; left: 50%; margin-left: -140px; z-index: 2; }
  .boton { position: absolute; width: 3px; background: #77736d; }
  .b1 { left: -3px; top: 120px; height: 34px; border-radius: 2px 0 0 2px; }
  .b2 { left: -3px; top: 170px; height: 56px; border-radius: 2px 0 0 2px; }
  .b3 { right: -3px; top: 150px; height: 80px; border-radius: 0 2px 2px 0; }
  .bisel { width: 100%; height: 100%; padding: 4px; border-radius: 32px; background: #050505; }
  .telefono.centro .bisel { border-radius: 40px; }
  .pantalla-tel { position: relative; width: 100%; height: 100%; border-radius: 28px; overflow: hidden; background: #f5f2ec; color: #1a1714; display: flex; flex-direction: column; }
  .telefono.centro .pantalla-tel { border-radius: 36px; }
  .isla { position: absolute; z-index: 3; top: 10px; left: 50%; width: 92px; height: 26px; margin-left: -46px; border-radius: 999px; background: #000; }
  .reflejo { position: absolute; z-index: 4; inset: 0; pointer-events: none; background: linear-gradient(115deg, rgba(255, 255, 255, 0.22) 0%, rgba(255, 255, 255, 0) 32%); }
  .tel-cuerpo { flex: 1; display: flex; flex-direction: column; gap: 12px; padding: 48px 14px 12px; }
  .tel-titulo { font: 900 24px/1 'Archivo', sans-serif; font-stretch: 115%; letter-spacing: -0.03em; }
  .tel-titulo.chico { font-size: 22px; }
  .tel-tarjeta { display: flex; gap: 10px; align-items: center; padding: 10px; border-radius: 22px; background: #fff; box-shadow: 0 8px 16px -12px rgba(0, 0, 0, 0.4); }
  .tel-tarjeta.col { flex-direction: column; align-items: stretch; }
  .tel-tarjeta b, .tel-etiqueta { font: 800 14px/1 'Archivo', sans-serif; }
  .tel-etiqueta { color: #5c5650; font-size: 13px; }
  .pantalla-tel small { font: 700 10px/1 'Archivo', sans-serif; }
  .ok { color: #2f7d5b; }
  .cerca { color: #a8650a; }
  .vivo-chip { padding: 6px 10px; border-radius: 999px; background: #e3f2ea; color: #2f7d5b; }
  .tel-cara { width: 100%; display: block; border-radius: 18px; background: #0b0a09; }
  .marco-cara { padding: 10px; border-radius: 26px; background: linear-gradient(160deg, #2b2b2b, #0c0c0c); box-shadow: 0 16px 24px -16px rgba(0, 0, 0, 0.6); }
  .pildora { padding: 8px 12px; border-radius: 999px; background: #fff; font: 700 12px/1 'Archivo', sans-serif; }
  .pildora.oscura { background: #1a1714; color: #fff; }
  .pildora.activa { background: radial-gradient(130% 150% at 30% 0%, #ffb08a, #ff5a1f 45%, #c2410c); box-shadow: inset 0 2px 1px rgba(255, 255, 255, 0.6); font-weight: 800; }
  .barras { gap: 10px; padding: 14px; }
  .barra { height: 8px; border-radius: 999px; background: linear-gradient(90deg, #ff6a33 var(--v), #ece7de var(--v)); }
  .miniaturas { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .miniaturas span { position: relative; aspect-ratio: 16 / 10; border-radius: 12px; }
  .miniaturas em { position: absolute; right: 5px; bottom: 5px; padding: 3px 6px; border-radius: 999px; background: #000; color: #fff; font: 800 9px/1 'Archivo', sans-serif; font-style: normal; }
  .miniaturas .sumar { border: 2px dashed #b8b2a8; display: flex; align-items: center; justify-content: center; font: 800 18px/1 'Archivo', sans-serif; color: #8a847c; }
  .pestanas { display: flex; justify-content: space-around; align-items: center; padding: 10px 12px 18px; background: rgba(255, 255, 255, 0.85); border-top: 1px solid #e2ddd4; }
  .pestanas span { display: flex; flex-direction: column; align-items: center; gap: 4px; font: 700 10px/1 'Archivo', sans-serif; color: #8a847c; }
  .pestanas i { width: 22px; height: 22px; border-radius: 8px; background: #d6d1c8; }
  .pestanas span.activa { color: #c2410c; }
  .pestanas span.activa i { background: #ff5a1f; }
  .relleno { flex: 1; }

  /* 7 · Reserva */
  .reserva { overflow: hidden; }
  .abajo { align-items: flex-end; }
  .abajo .fila { padding-bottom: 16px; }
  .formulario { display: flex; flex-direction: column; gap: 16px; padding: 32px; border-radius: 40px; background: #111; color: #fff; box-shadow: 0 32px 48px -32px rgba(0, 0, 0, 0.7); }
  .formulario.banda { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto minmax(180px, 220px); align-items: end; padding: 24px; }
  .formulario label, .formulario legend { display: flex; flex-direction: column; gap: 8px; font: 700 15px 'Archivo', sans-serif; color: #c4c4c4; }
  .formulario legend { padding: 0 0 8px; }
  .formulario input { min-height: 56px; padding: 0 24px; border-radius: 999px; border: 0; background: #262626; color: #fff; font: 600 18px 'Archivo', sans-serif; }
  .formulario fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
  .selector { display: flex; gap: 8px; align-items: center; min-height: 56px; padding: 4px 12px 4px 4px; border-radius: 999px; background: #262626; }
  .inflado.mini { min-height: 48px; padding: 0 18px; font-size: 13px; }
  .inflado.plano { background: none; box-shadow: none; }
  .muestra { flex: none; width: 32px; height: 32px; border-radius: 50%; border: 0; cursor: pointer; box-shadow: 0 0 0 1px #555; }
  .muestra[aria-pressed='true'] { box-shadow: 0 0 0 2px #262626, 0 0 0 4px #fff; }
  .enviar { width: 100%; font-size: 17px; }
  .error { grid-column: 1 / -1; margin: 0; color: #ffb4a1; font-weight: 700; }
  .detalle-claro { margin: 0; color: #c4c4c4; font-size: 18px; line-height: 1.4; }
  .trampa { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .pie { background: #111; color: #a3a3a3; padding: 32px 0; font: 700 15px 'Archivo', sans-serif; }
  .pie a { color: #e5e5e5; }

  @media (max-width: 720px) {
    .contenedor { padding: 0 16px; }
    .seccion { padding: 96px 0; }
    .tarjeta-negra, .paso, .formulario { padding: 24px; }
    .formulario.banda { grid-template-columns: 1fr; }
    .nav nav .ficha:not(:last-child) { display: none; }
    .escenario { height: 600px; }
    .telefono.izq, .telefono.der { display: none; }
  }
  @media (prefers-reduced-motion: reduce) {
    .grano, .cromo { animation: none; }
    .inflado { transition: none; }
  }
</style>
