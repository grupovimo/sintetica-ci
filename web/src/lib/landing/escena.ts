// La landing: el bicho en 3D que el scroll gira, abre y pinta. La pantalla del
// modelo la dibuja el núcleo de verdad (el canvas `pantalla`), así la cara de
// la landing es la misma que la del bicho.
import * as THREE from 'three';
import { RoundedBoxGeometry } from 'three/examples/jsm/geometries/RoundedBoxGeometry.js';
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js';
import Lenis from 'lenis';

export const COLORES: [string, string, string][] = [
  ['naranja', 'Naranja', '#F5501A'],
  ['carbon', 'Carbón', '#26221F'],
  ['hueso', 'Hueso', '#E6E0D3'],
];

// Lo que se ve en la pantalla en cada escena: una expresión del núcleo, o el QR
// de emparejamiento (que en la vista previa no aparece solo).
export type Cara = string;

export type Opciones = {
  raiz: HTMLElement;
  canvas: HTMLCanvasElement;
  pantalla: HTMLCanvasElement | null; // el núcleo (320 × 172); null mientras carga
  alCambiarCara: (cara: Cara) => void;
  alCaricia: () => void;
  alColor: (i: number) => void;
  colorElegido: () => number; // -1 = el que va con el scroll
};

type Clave = {
  x: number; y: number; ry: number; rx: number; rz: number; s: number; cz: number; cy: number; ex: number;
  cara: Cara; sacudir?: 1; hablar?: 1; colores?: 1;
};
function K(x: number, y: number, ry: number, cara: Cara, o: Partial<Clave> = {}): Clave {
  return Object.assign({ x, y, ry, rx: 0, rz: 0, s: 1, cz: 13, cy: 0.4, ex: 0, cara }, o);
}
function plan(): Clave[] {
  if (innerWidth < 760)
    return [
      K(0, 0.95, -0.3, 'neutral', { s: 0.8, cz: 15 }),
      K(0, 1.2, 0.45, 'contento', { s: 0.72, rz: 0.1, cz: 15 }),
      K(0, 1.2, -0.4, 'mareado', { s: 0.72, sacudir: 1, cz: 15 }),
      K(0, 1.2, 0.3, 'escuchando', { s: 0.72, hablar: 1, cz: 15 }),
      K(0, 1.25, -0.75, 'curioso', { s: 0.62, rx: 0.18, ex: 1, cz: 15 }),
      K(0, 1.2, 0.25, 'qr', { s: 0.72, cz: 15 }),
      K(0, 1.2, -0.2, 'actualizando', { s: 0.72, cz: 15 }),
      K(0, 1.25, -0.5, 'enamorado', { s: 0.72, colores: 1, cz: 15 }),
      K(0, 1.2, Math.PI - 0.35, 'neutral', { s: 0.72, cz: 15 }),
    ];
  return [
    K(1.8, 0.35, -0.4, 'neutral', { cz: 11.5, cy: 0.5, rx: 0.1 }),
    K(-2.3, 0.2, 0.55, 'contento', { rz: 0.1, rx: 0.12 }),
    K(2.3, 0.2, -0.5, 'mareado', { sacudir: 1 }),
    K(-2.3, 0.2, 0.4, 'escuchando', { hablar: 1, rx: -0.08 }),
    K(1.6, 0.25, -0.9, 'curioso', { s: 0.9, rx: 0.2, ex: 1, cz: 13.5 }),
    K(-2.3, 0.2, 0.35, 'qr', { rx: 0.08 }),
    K(2.0, 0.2, -0.35, 'actualizando', { cz: 12.5 }),
    K(-2.1, 0.25, 0.55, 'enamorado', { colores: 1, cz: 12.2, rx: 0.1 }),
    K(2.1, 0.2, Math.PI - 0.45, 'neutral'),
  ];
}
const FONDOS = ['#FBFAF7', '#FBFAF7', '#141210', '#FBFAF7', '#141210', '#EDE9E1', '#141210', '#FBFAF7', '#EDE9E1'];
const CARICIAS = ['contento', 'enamorado', 'sorpresa', 'guino', 'sospecha'];
const HABLANDO = ['escuchando', 'hablando', 'pensando'];

const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
const suave = (a: number, b: number, x: number) => {
  const t = Math.min(Math.max((x - a) / (b - a), 0), 1);
  return t * t * (3 - 2 * t);
};

// El QR de emparejamiento, como lo muestra el bicho (dibujo, no un QR real).
function dibujarQR(x: CanvasRenderingContext2D, W: number, H: number) {
  const k = W / 320;
  x.save();
  x.scale(k, H / 172);
  x.fillStyle = '#F5F2EC';
  x.beginPath();
  x.roundRect(20, 22, 128, 128, 8);
  x.fill();
  x.fillStyle = '#0B0A09';
  for (const [fx, fy] of [[28, 30], [108, 30], [28, 110]]) {
    x.fillRect(fx, fy, 32, 32);
    x.fillStyle = '#F5F2EC';
    x.fillRect(fx + 6, fy + 6, 20, 20);
    x.fillStyle = '#0B0A09';
    x.fillRect(fx + 12, fy + 12, 8, 8);
  }
  for (const [a, b, c, d] of [[68,30,8,8],[84,30,8,16],[68,46,16,8],[92,54,8,8],[68,70,8,8],[28,70,16,8],[52,78,8,16],[84,70,24,8],[116,70,8,16],[132,78,8,8],[36,94,8,8],[68,86,8,24],[84,94,16,8],[108,94,8,8],[124,102,16,8],[76,118,16,8],[100,110,8,16],[116,118,8,8],[132,126,8,16],[68,134,8,8],[92,134,24,8]])
    x.fillRect(a, b, c, d);
  x.font = "700 15px 'JetBrains Mono', monospace";
  x.fillStyle = '#FF6A33';
  x.fillText('UNIDAD #001', 166, 70);
  x.font = "700 13px 'JetBrains Mono', monospace";
  x.fillStyle = '#F5F2EC';
  x.fillText('ESCANEAME', 166, 100);
  x.fillText('CON LA APP', 166, 120);
  x.restore();
}

export function montar(op: Opciones): () => void {
  const { raiz } = op;
  const $ = <T extends Element = HTMLElement>(s: string) => raiz.querySelector(s) as T;
  const reducir = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const escenas = [...raiz.querySelectorAll<HTMLElement>('.escena')];
  const N = escenas.length;
  const limpiar: (() => void)[] = [];
  const escuchar = <K extends keyof WindowEventMap>(t: K, f: (e: WindowEventMap[K]) => void, o?: AddEventListenerOptions) => {
    addEventListener(t, f, o);
    limpiar.push(() => removeEventListener(t, f));
  };

  // ---------- scroll suave ----------
  let lenis: Lenis | null = null;
  if (!reducir) {
    lenis = new Lenis({ lerp: 0.075, smoothWheel: true, wheelMultiplier: 0.9, touchMultiplier: 1.4 });
    limpiar.push(() => lenis?.destroy());
    raiz.querySelectorAll<HTMLAnchorElement>('a[href^="#"]').forEach((a) => {
      const f = (e: Event) => {
        const el = raiz.querySelector(a.getAttribute('href')!);
        if (!el) return;
        e.preventDefault();
        lenis!.scrollTo(el as HTMLElement, { duration: 2.2, easing: (t) => 1 - Math.pow(1 - t, 4) });
      };
      a.addEventListener('click', f);
      limpiar.push(() => a.removeEventListener('click', f));
    });
  }

  // ---------- three ----------
  let renderer: THREE.WebGLRenderer;
  try {
    renderer = new THREE.WebGLRenderer({ canvas: op.canvas, antialias: true, alpha: true });
  } catch {
    raiz.classList.add('sin-gl');
    return () => limpiar.forEach((f) => f());
  }
  limpiar.push(() => renderer.dispose());
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.05;

  const scene = new THREE.Scene();
  const pmrem = new THREE.PMREMGenerator(renderer);
  scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;
  const camera = new THREE.PerspectiveCamera(28, 1, 0.1, 100);
  const key = new THREE.DirectionalLight(0xffffff, 1.6);
  key.position.set(4, 6, 6);
  scene.add(key);
  const rim = new THREE.DirectionalLight(0xffd2bf, 1.3);
  rim.position.set(-6, 2, -4);
  scene.add(rim);
  scene.add(new THREE.HemisphereLight(0xffffff, 0x2a2420, 0.5));

  const shellMat = new THREE.MeshPhysicalMaterial({ color: 0xf5501a, roughness: 0.62, clearcoat: 0.25, clearcoatRoughness: 0.6, sheen: 0.6, sheenRoughness: 0.5, sheenColor: new THREE.Color(0xffb08a) });
  const accent = new THREE.MeshPhysicalMaterial({ color: 0xf5501a, roughness: 0.4, clearcoat: 0.6 });
  const black = new THREE.MeshPhysicalMaterial({ color: 0x151311, roughness: 0.55, metalness: 0.1, clearcoat: 0.3 });
  const pcbMat = new THREE.MeshStandardMaterial({ color: 0x1d1a17, roughness: 0.7 });
  const chipMat = new THREE.MeshStandardMaterial({ color: 0x2a2724, roughness: 0.35, metalness: 0.4 });
  const metal = new THREE.MeshStandardMaterial({ color: 0xbfb8ad, roughness: 0.3, metalness: 0.85 });
  const glass = new THREE.MeshPhysicalMaterial({ color: 0x050404, roughness: 0.08, clearcoat: 1, transparent: true, opacity: 0.22 });

  const root = new THREE.Group();
  scene.add(root);
  const body = new THREE.Group();
  root.add(body);
  const head = new THREE.Group();
  body.add(head);

  // grano "soft-touch" de la carcasa
  const gC = document.createElement('canvas');
  gC.width = gC.height = 512;
  const gX = gC.getContext('2d')!;
  const gI = gX.createImageData(512, 512);
  for (let i = 0; i < gI.data.length; i += 4) {
    const v = 118 + Math.random() * 40;
    gI.data[i] = gI.data[i + 1] = gI.data[i + 2] = v;
    gI.data[i + 3] = 255;
  }
  gX.putImageData(gI, 0, 0);
  const grainTex = new THREE.CanvasTexture(gC);
  grainTex.wrapS = grainTex.wrapT = THREE.RepeatWrapping;
  grainTex.repeat.set(3, 3);
  shellMat.bumpMap = grainTex;
  shellMat.bumpScale = 0.6;

  const DW = 3.5, DH = 2.2, DD = 0.95;
  head.add(new THREE.Mesh(new RoundedBoxGeometry(DW, DH, DD, 10, 0.44), shellMat));

  const face = new THREE.Group();
  head.add(face);
  const bezel = new THREE.Mesh(new RoundedBoxGeometry(2.92, 1.7, 0.1, 6, 0.28), black);
  bezel.position.z = 0.43;
  face.add(bezel);
  const SW = 640, SH = 344;
  const sc = document.createElement('canvas');
  sc.width = SW;
  sc.height = SH;
  const sx = sc.getContext('2d')!;
  const screenTex = new THREE.CanvasTexture(sc);
  screenTex.colorSpace = THREE.SRGBColorSpace;
  screenTex.anisotropy = 8;
  const screenGeo = new THREE.PlaneGeometry(2.62, (2.62 * SH) / SW);
  const screen = new THREE.Mesh(screenGeo, new THREE.MeshBasicMaterial({ map: screenTex, toneMapped: false }));
  screen.position.z = 0.485;
  face.add(screen);
  const cover = new THREE.Mesh(screenGeo, glass);
  cover.position.z = 0.49;
  face.add(cover);

  // la espalda, con el número grabado
  const bc = document.createElement('canvas');
  bc.width = 1024;
  bc.height = 512;
  const bx = bc.getContext('2d')!;
  const backTex = new THREE.CanvasTexture(bc);
  backTex.colorSpace = THREE.SRGBColorSpace;
  backTex.anisotropy = 8;
  const backMat = new THREE.MeshStandardMaterial({ map: backTex, transparent: true, roughness: 0.6 });
  const back = new THREE.Mesh(new THREE.PlaneGeometry(2.7, 1.35), backMat);
  back.position.z = -0.482;
  back.rotation.y = Math.PI;
  head.add(back);
  let backInk = '#141210';
  function drawBack() {
    bx.clearRect(0, 0, 1024, 512);
    bx.fillStyle = backInk;
    bx.font = "700 44px 'JetBrains Mono', monospace";
    bx.fillText('SINTÉTICA LABS', 80, 120);
    bx.font = "700 150px 'JetBrains Mono', monospace";
    bx.fillText('#001', 72, 290);
    bx.font = "400 36px 'JetBrains Mono', monospace";
    bx.fillText('LOTE 26-11 · FW 0.1.0', 80, 370);
    bx.fillText('HECHO A MANO EN MAR DEL PLATA', 80, 424);
    backTex.needsUpdate = true;
  }
  drawBack();
  document.fonts?.ready.then(drawBack);

  // detalles: botón de arriba, USB-C, agujeritos del parlante
  const antenna = new THREE.Group();
  head.add(antenna);
  const topBtn = new THREE.Mesh(new RoundedBoxGeometry(0.62, 0.16, 0.3, 5, 0.075), accent);
  topBtn.position.set(1.0, DH / 2 + 0.02, 0);
  antenna.add(topBtn);
  const usb = new THREE.Mesh(new RoundedBoxGeometry(0.06, 0.36, 0.15, 3, 0.03), new THREE.MeshStandardMaterial({ color: 0x0b0a09, roughness: 0.4, metalness: 0.6 }));
  usb.position.set(-DW / 2 + 0.01, 0, 0);
  head.add(usb);
  for (let i = 0; i < 9; i++) {
    const d = new THREE.Mesh(new THREE.CircleGeometry(0.035, 16), new THREE.MeshBasicMaterial({ color: 0x141210 }));
    d.position.set(1.25 - (i % 3) * 0.12, -0.55 + Math.floor(i / 3) * 0.12, 0.001);
    back.add(d);
  }

  // por dentro (se ve al abrirlo)
  const inner = new THREE.Group();
  head.add(inner);
  const pcb = new THREE.Mesh(new RoundedBoxGeometry(2.6, 1.5, 0.06, 2, 0.03), pcbMat);
  pcb.position.z = 0.22;
  inner.add(pcb);
  const mcu = new THREE.Mesh(new THREE.BoxGeometry(0.62, 0.62, 0.08), chipMat);
  mcu.position.set(0.35, 0.1, 0.28);
  inner.add(mcu);
  const mcuCan = new THREE.Mesh(new THREE.BoxGeometry(0.5, 0.36, 0.05), metal);
  mcuCan.position.set(0.35, 0.12, 0.33);
  inner.add(mcuCan);
  const imu = new THREE.Mesh(new THREE.BoxGeometry(0.18, 0.18, 0.06), accent);
  imu.position.set(-0.55, -0.4, 0.27);
  inner.add(imu);
  for (let i = 0; i < 6; i++) {
    const c = new THREE.Mesh(new THREE.BoxGeometry(0.12, 0.06, 0.04), chipMat);
    c.position.set(-0.9 + i * 0.16, 0.5, 0.26);
    inner.add(c);
  }
  const battery = new THREE.Mesh(new RoundedBoxGeometry(2.4, 1.0, 0.18, 3, 0.06), metal);
  battery.position.set(0.1, -0.1, -0.15);
  inner.add(battery);
  const speaker = new THREE.Group();
  speaker.position.set(-1.1, 0.3, -0.05);
  inner.add(speaker);
  const spk = new THREE.Mesh(new THREE.CylinderGeometry(0.28, 0.28, 0.12, 40), black);
  spk.rotation.x = Math.PI / 2;
  speaker.add(spk);
  const spkRing = new THREE.Mesh(new THREE.TorusGeometry(0.2, 0.03, 12, 40), accent);
  spkRing.position.z = 0.07;
  speaker.add(spkRing);

  // sombra
  const shC = document.createElement('canvas');
  shC.width = shC.height = 256;
  const shX = shC.getContext('2d')!;
  const gr = shX.createRadialGradient(128, 128, 0, 128, 128, 128);
  gr.addColorStop(0, 'rgba(0,0,0,.32)');
  gr.addColorStop(1, 'rgba(0,0,0,0)');
  shX.fillStyle = gr;
  shX.fillRect(0, 0, 256, 256);
  const shadow = new THREE.Mesh(new THREE.PlaneGeometry(4.4, 1.5), new THREE.MeshBasicMaterial({ map: new THREE.CanvasTexture(shC), transparent: true, depthWrite: false }));
  shadow.rotation.x = -Math.PI / 2;
  shadow.position.y = -1.75;
  root.add(shadow);

  // partículas de fondo
  const PN = 420;
  const pg = new THREE.BufferGeometry();
  const pp = new Float32Array(PN * 3);
  for (let i = 0; i < PN; i++) {
    pp[i * 3] = (Math.random() - 0.5) * 26;
    pp[i * 3 + 1] = (Math.random() - 0.5) * 14;
    pp[i * 3 + 2] = (Math.random() - 0.5) * 14 - 2;
  }
  pg.setAttribute('position', new THREE.BufferAttribute(pp, 3));
  const dotC = document.createElement('canvas');
  dotC.width = dotC.height = 64;
  const dx = dotC.getContext('2d')!;
  dx.beginPath();
  dx.arc(32, 32, 28, 0, 7);
  dx.fillStyle = '#fff';
  dx.fill();
  const pMat = new THREE.PointsMaterial({ size: 0.06, map: new THREE.CanvasTexture(dotC), transparent: true, opacity: 0.35, depthWrite: false, color: 0x141210 });
  const points = new THREE.Points(pg, pMat);
  scene.add(points);

  // ---------- la pantalla ----------
  let caraActual: Cara = '';
  let arranque = reducir ? 9 : 0;
  const ponerCara = (c: Cara) => {
    if (c === caraActual) return;
    caraActual = c;
    if (c !== 'qr') op.alCambiarCara(c);
  };
  function dibujarPantalla() {
    sx.fillStyle = '#0B0A09';
    sx.fillRect(0, 0, SW, SH);
    sx.save();
    if (arranque < 1.6) {
      // el arranque: una línea, el nombre, y aparece la cara
      sx.shadowColor = 'rgba(255,106,51,.6)';
      sx.shadowBlur = 24;
      sx.fillStyle = '#FF6A33';
      if (arranque < 0.6) {
        const w = SW * Math.min(arranque / 0.45, 1) * 0.8;
        sx.fillRect((SW - w) / 2, SH / 2 - 2, w, 4);
      } else {
        sx.font = "700 34px 'JetBrains Mono', monospace";
        sx.textAlign = 'center';
        sx.fillText('SINTÉTICA LABS', SW / 2, SH / 2 - 6);
        sx.font = "400 22px 'JetBrains Mono', monospace";
        sx.fillStyle = '#A39E96';
        sx.fillText('UNIDAD #001 · DESPERTANDO' + '.'.repeat(1 + (Math.floor(arranque * 4) % 3)), SW / 2, SH / 2 + 34);
      }
    } else if (caraActual === 'qr') {
      dibujarQR(sx, SW, SH);
    } else if (op.pantalla) {
      const k = arranque < 2.2 ? (arranque - 1.6) / 0.6 : 1;
      sx.imageSmoothingEnabled = true;
      sx.shadowColor = 'rgba(255,106,51,.45)';
      sx.shadowBlur = 18;
      sx.drawImage(op.pantalla, 0, (SH - SH * k) / 2, SW, SH * k);
    }
    sx.restore();
    sx.fillStyle = 'rgba(255,255,255,.025)';
    for (let y = 0; y < SH; y += 4) sx.fillRect(0, y, SW, 1);
    screenTex.needsUpdate = true;
  }

  // ---------- etiquetas del despiece ----------
  const capaEtiquetas = $('.etiquetas-3d');
  const ETIQUETAS = (
    [
      ['Cara', 'pantalla a color', screen, [-0.9, 0.75, 0]],
      ['Cerebro', 'WiFi + Bluetooth', mcuCan, [0, 0, 0]],
      ['Equilibrio', 'siente el movimiento', imu, [-0.2, -0.45, 0]],
      ['Voz', 'habla y escucha', speaker, [0, 0.25, 0]],
      ['Batería', 'recargable', battery, [0.9, -0.1, 0]],
      ['Puerto', 'USB-C, carga y datos', usb, [0, 0, 0]],
    ] as [string, string, THREE.Object3D, [number, number, number]][]
  ).map(([a, b, obj, off]) => {
    const el = document.createElement('div');
    el.className = 'lbl';
    const s = document.createElement('span');
    s.textContent = a;
    const em = document.createElement('em');
    em.textContent = b;
    s.appendChild(em);
    el.appendChild(s);
    capaEtiquetas.appendChild(el);
    limpiar.push(() => el.remove());
    return { el, obj, off: new THREE.Vector3(...off) };
  });
  const v3 = new THREE.Vector3();

  // ---------- entrada ----------
  const mirar = { x: 0, y: 0 }, mirarA = { x: 0, y: 0 };
  escuchar('pointermove', (e) => {
    mirarA.x = (e.clientX / innerWidth) * 2 - 1;
    mirarA.y = (e.clientY / innerHeight) * 2 - 1;
  }, { passive: true });
  let caricia = 0, cariciaHasta = 0;
  const rayo = new THREE.Raycaster(), ndc = new THREE.Vector2();
  const clock = new THREE.Clock();
  escuchar('click', (e) => {
    if ((e.target as Element).closest('a,button,input,textarea,select,label,.solida')) return;
    ndc.set((e.clientX / innerWidth) * 2 - 1, -(e.clientY / innerHeight) * 2 + 1);
    rayo.setFromCamera(ndc, camera);
    if (rayo.intersectObject(root, true).length) {
      caricia = 1;
      op.alCaricia();
      ponerCara(CARICIAS[Math.floor(Math.random() * CARICIAS.length)]);
      cariciaHasta = clock.elapsedTime + 2.2;
    }
  });

  let P = plan();
  function medir() {
    renderer.setSize(innerWidth, innerHeight, false);
    camera.aspect = innerWidth / innerHeight;
    camera.updateProjectionMatrix();
    P = plan();
  }
  escuchar('resize', medir);
  medir();

  // ---------- progreso ----------
  const riel = $('.riel');
  escenas.forEach(() => riel.appendChild(document.createElement('i')));
  const textos = escenas.map((s) => s.querySelector<HTMLElement>('.txt')!);
  function progreso() {
    const vh = innerHeight;
    let f = 0;
    for (let i = 0; i < N; i++) {
      const r = escenas[i].getBoundingClientRect();
      if (r.top <= 0.001) f = i + Math.min(Math.max(-r.top / (r.height - vh), 0), 1);
    }
    return f;
  }
  const tira1 = $('.tira-1'), tira2 = $('.tira-2'), seccionCaras = $('.caras');
  const contador = $('.contador');

  const cA = new THREE.Color(), cB = new THREE.Color(), cFondo = new THREE.Color();
  const colorActual = new THREE.Color(COLORES[0][2]);
  const colorDestino = new THREE.Color();
  let ultimaClave: Clave | null = null, hablarT = 0, indice = -1, oscuro: boolean | null = null, colorIdx = -1;
  let cuadro = 0;

  function tick(time: number) {
    lenis?.raf(time);
    const dt = Math.min(clock.getDelta(), 0.05), t = clock.elapsedTime;
    if (arranque < 9) arranque += dt;

    const f = Math.min(progreso(), N - 1 + 0.999);
    const k = Math.floor(f), u = f - k;
    const tr = k < N - 1 ? suave(0.55, 1, u) : 0;
    const A = P[k], B = P[Math.min(k + 1, N - 1)];
    const v = (c: 'x' | 'y' | 'ry' | 'rx' | 'rz' | 's' | 'cz' | 'cy' | 'ex') => lerp(A[c], B[c], tr);

    textos.forEach((el, i) => {
      let entrada = 0;
      if (i === k) entrada = (i === 0 ? 1 : suave(0.02, 0.2, u)) * (i === N - 1 ? 1 : 1 - suave(0.5, 0.7, u));
      el.style.setProperty('--in', entrada.toFixed(3));
    });

    cA.set(FONDOS[k]);
    cB.set(FONDOS[Math.min(k + 1, N - 1)]);
    cFondo.copy(cA).lerp(cB, tr);
    raiz.style.backgroundColor = '#' + cFondo.getHexString();
    const esOscuro = cFondo.r * 0.2126 + cFondo.g * 0.7152 + cFondo.b * 0.0722 < 0.4;
    if (esOscuro !== oscuro) {
      oscuro = esOscuro;
      raiz.style.setProperty('--fg', esOscuro ? '#FBFAF7' : '#141210');
      pMat.color.set(esOscuro ? 0xfbfaf7 : 0x141210);
    }
    const idx = tr > 0.5 ? Math.min(k + 1, N - 1) : k;
    if (idx !== indice) {
      indice = idx;
      [...riel.children].forEach((el, i) => el.classList.toggle('on', i === idx));
      contador.textContent = String(idx + 1).padStart(2, '0') + ' / ' + String(N).padStart(2, '0');
    }

    // la cara de cada escena
    const sk = tr > 0.5 ? B : A;
    const acariciado = cariciaHasta > t;
    if (sk !== ultimaClave) {
      ultimaClave = sk;
      hablarT = 0;
      if (!acariciado) ponerCara(sk.cara);
    }
    if (cariciaHasta && !acariciado) {
      cariciaHasta = 0;
      ponerCara(sk.cara);
    }
    if (sk.hablar && !acariciado) {
      hablarT += dt;
      ponerCara(HABLANDO[Math.floor(hablarT / 1.6) % 3]);
    }

    // el color de la carcasa
    const elegido = op.colorElegido();
    let ci = elegido >= 0 ? elegido : 0;
    if (sk.colores && elegido < 0) ci = Math.min(2, Math.floor(suave(0.05, 0.5, u) * 2.999));
    if (ci !== colorIdx) {
      colorIdx = ci;
      op.alColor(ci);
      const tinta = ci === 1 ? '#E6E0D3' : '#141210';
      if (tinta !== backInk) {
        backInk = tinta;
        drawBack();
      }
    }
    colorActual.lerp(colorDestino.set(COLORES[ci][2]), Math.min(1, dt * 6));
    shellMat.color.copy(colorActual);

    camera.position.set(0, v('cy'), v('cz'));
    camera.lookAt(0, 0.2, 0);

    mirar.x += (mirarA.x - mirar.x) * Math.min(1, dt * 4);
    mirar.y += (mirarA.y - mirar.y) * Math.min(1, dt * 4);
    root.position.set(v('x'), v('y'), 0);
    root.scale.setScalar(v('s'));
    root.rotation.y = v('ry') + (reducir ? 0 : Math.sin(t * 0.35) * 0.04);
    root.rotation.x = v('rx');
    const flota = reducir ? 0 : Math.sin(t * 1.4) * 0.07;
    body.position.y = flota;
    shadow.scale.setScalar(1 - flota * 0.8);
    let sacudida = 0;
    if (sk.sacudir && !reducir) {
      const ph = t % 2.4;
      if (ph < 0.7) sacudida = Math.sin(ph * 42) * 0.11 * (1 - ph / 0.7);
    }
    caricia = Math.max(0, caricia - dt * 2.2);
    const aplasta = reducir ? 0 : Math.sin(caricia * Math.PI) * 0.06;
    head.scale.set(1 + aplasta, 1 - aplasta, 1 + aplasta);
    const ex = v('ex');
    head.rotation.y = mirar.x * 0.32 * (1 - ex);
    head.rotation.x = mirar.y * 0.16 * (1 - ex) + (sk.hablar && !reducir ? Math.sin(t * 3) * 0.04 : 0);
    head.rotation.z = v('rz') + sacudida + (reducir ? 0 : Math.sin(t * 0.9) * 0.02) * (1 - ex);

    // el despiece
    const e = ex * ex * (3 - 2 * ex);
    face.position.z = e * 1.4;
    back.position.z = -0.482 - e * 1.0;
    antenna.position.y = e * 0.6;
    inner.visible = e > 0.01;
    battery.position.z = -0.15 - e * 0.55;
    speaker.position.x = -1.1 - e * 0.35;
    mcuCan.position.z = 0.33 + e * 0.25;
    mcu.position.z = 0.28 + e * 0.15;
    const transparente = e > 0.001;
    if (transparente !== shellMat.transparent) {
      shellMat.transparent = transparente;
      shellMat.needsUpdate = true;
    }
    shellMat.opacity = 1 - e * 0.86;
    shellMat.depthWrite = e < 0.5;
    backMat.opacity = 1 - e * 0.6;

    ETIQUETAS.forEach((L, i) => {
      const o = suave(0.55 + i * 0.05, 0.85 + i * 0.03, e);
      if (o <= 0.001) {
        L.el.style.opacity = '0';
        return;
      }
      L.obj.getWorldPosition(v3);
      v3.add(L.off).project(camera);
      const x = (v3.x * 0.5 + 0.5) * innerWidth, y = (-v3.y * 0.5 + 0.5) * innerHeight;
      L.el.style.opacity = o.toFixed(3);
      L.el.style.transform = `translate(${(x - 5).toFixed(1)}px,${(y - 12).toFixed(1)}px)`;
    });

    points.rotation.y = f * 0.35 + t * 0.01;
    points.position.y = -f * 0.6;
    root.visible = progreso() < N - 0.02 || escenas[N - 1].getBoundingClientRect().bottom > 0;

    const fr = seccionCaras.getBoundingClientRect();
    if (fr.top < innerHeight && fr.bottom > 0) {
      const p = (innerHeight - fr.top) / (innerHeight + fr.height);
      tira1.style.transform = `translateX(${-p * 40}%)`;
      tira2.style.transform = `translateX(${-33 + p * 30}%)`;
    }

    dibujarPantalla();
    renderer.render(scene, camera);
    cuadro = requestAnimationFrame(tick);
  }
  cuadro = requestAnimationFrame(tick);
  limpiar.push(() => cancelAnimationFrame(cuadro));

  return () => {
    limpiar.forEach((f) => f());
    pmrem.dispose();
  };
}
