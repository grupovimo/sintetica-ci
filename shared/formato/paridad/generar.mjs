// Genera shared/formato/paridad/: un set de configs (válidas e inválidas) en
// los límites y casos borde de la config, y el índice paridad.json que dice,
// para cada una, si el núcleo la acepta y, si no, con qué código y detalle.
// Las corren el núcleo (firmware/test/test_paridad.cpp), cgo
// (firmware/api/cgo/paridad_test.go), el WebAssembly (sim/test/paridad.test.mjs)
// y, para lo que validan del lado de la app (cortes de bytes y topes de
// listas), la web, Kotlin y Swift. Así todos aceptan y rechazan lo mismo.
//
// Uso: node shared/formato/paridad/generar.mjs [--verificar]
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = path.dirname(fileURLToPath(import.meta.url));
const dir = path.join(aqui, 'configs');
const verificar = process.argv.includes('--verificar');

// Los códigos de ErrorConfig (firmware/nucleo/formato.h).
const E = { json: 1, version: 2, campo: 3, forma: 4, demasiadas: 5, regla: 6 };

// Los límites del núcleo que las apps también cuidan (formato.h, habla.h,
// vida_datos.h). El test del núcleo comprueba que coincidan con las constantes.
const limites = {
  configBytes: 16384,
  expresiones: 24, // dibujos en total (16 de base)
  expresionesBase: 16,
  carasVida: 48, // paramétricas en total (38 de base)
  carasVidaBase: 38,
  comportamientos: 67, // en total (58 de base)
  comportamientosBase: 58,
  carasImagen: 8,
  sprites: 12,
  imagenes: 8,
  reglasUsuario: 24,
  acciones: 6,
  lista: 16,
  animaciones: 6,
  recordatorios: 12,
  bytesRecordatorio: 40,
  frasesPropias: 12,
  bytesFrase: 60, // el globo (habla.propias, despertador.texto)
  avisos: 8,
  bytesDecirAviso: 40,
  bytesAppAviso: 24, // app y contacto de un aviso
  habitos: 4, // v1.9
  bytesHabito: 16, // el nombre de un hábito
  bytesPresencia: 40, // presencia.texto
  bytesPalabra: 16, // voz.palabra
  bytesCiudad: 40,
  bytesNombre: 24, // nombres de expresiones, comportamientos y animaciones
};

const casos = [];
// acepta: true, o el código y el comienzo del detalle del núcleo.
// esquema: lo que dice config.schema.json, cuando no es lo mismo que el núcleo
// (el schema no sabe cuántas trae la base ni corta como el núcleo).
function caso(archivo, config, acepta, extra = {}) {
  const texto = typeof config === 'string' ? config : JSON.stringify(config, null, 1) + '\n';
  casos.push({ archivo, texto, ...(acepta === true ? { acepta: true } : { acepta: false, codigo: acepta[0], detalle: acepta[1] }), ...extra });
}

const v2 = (o = {}) => ({ version: 2, ...o });
const n = (k, f) => Array.from({ length: k }, (_, i) => f(i));
const objeto = (k, nombre, valor) => Object.fromEntries(n(k, (i) => [nombre(i), valor(i)]));
const bytes = (s) => new TextEncoder().encode(s).length;
// Un texto de exactamente `b` bytes con tildes (2 bytes) y, si hace falta, una letra suelta.
const conTildes = (b) => 'á'.repeat(Math.floor(b / 2)) + (b % 2 ? 'a' : '');
// Uno de `b` bytes con emoji (4 bytes): "😀" × k y el resto en ASCII.
const conEmoji = (b) => '😀'.repeat(Math.floor(b / 4)) + 'x'.repeat(b % 4);
for (const [s, b] of [[conTildes(60), 60], [conTildes(61), 61], [conEmoji(60), 60], [conEmoji(41), 41]]) {
  if (bytes(s) !== b) throw new Error(`mal armado: ${s}`);
}

// --- Lo básico ---
caso('minima-v2.json', v2(), true);
caso('minima-v1.json', { version: 1 }, true);
caso('version-3.json', { version: 3 }, [E.version, 'config: version debe ser 1 o 2']);
caso('sin-version.json', { nombre: 'Toto' }, [E.version, 'config: version debe ser 1 o 2']);
caso('json-roto.json', '{"version": 2, "nombre": "Toto",}\n', [E.json, 'config: JSON inválido']);
caso('no-es-objeto.json', '[{"version": 2}]\n', [E.json, 'config: JSON inválido']);
// Más de 16 KB (sin espacios: compactada sigue siendo grande).
caso('mas-de-16kb.json', JSON.stringify(v2({ nota: 'x'.repeat(16400) })), [E.demasiadas, 'config: más de 16 KB']);
caso('casi-16kb.json', JSON.stringify(v2({ nota: 'x'.repeat(16384 - 32) })), true, { esquema: false });
// Más tokens de los que tiene el parser (2400) en menos de 16 KB.
caso('demasiados-tokens.json', JSON.stringify(v2({ nota: n(2400, () => 0) })), [E.json, 'config: JSON inválido'], { esquema: false });

// --- Nombres ---
caso('nombre-24-bytes.json', v2({ nombre: 'Ñandú'.padEnd(22, 'x') }), true);
caso('nombre-vacio.json', v2({ nombre: '' }), [E.campo, 'config: nombre inválido']);
caso('expresion-nombre-mayuscula.json', v2({ expresiones: { Feliz: { gy: 0.02 } } }), [E.campo, 'expresiones: nombre inválido']);
caso('expresion-nombre-numero.json', v2({ expresiones: { '1feliz': { gy: 0.02 } } }), [E.campo, 'expresiones: nombre inválido']);
caso('expresion-nombre-tilde.json', v2({ expresiones: { 'pícaro': { gy: 0.02 } } }), [E.campo, 'expresiones: nombre inválido']);
caso('expresion-nombre-24.json', v2({ expresiones: { ['a'.repeat(24)]: { gy: 0.02 } } }), true);
caso('comportamiento-nombre-guion-bajo.json', v2({ comportamientos: { saludar_mucho: { pasos: [{ cara: 'contento', dura: 1 }] } } }),
  [E.campo, 'comportamientos: nombre inválido']);
caso('animacion-nombre-espacio.json',
  v2({ animaciones: { 'un salto': { duracion: 1000, pistas: [{ forma: { forma: 'circulo', x: 0, y: 0, r: 0.1 }, claves: [{ t: 0 }, { t: 1000, y: 0.1 }] }] } } }),
  [E.campo, 'animaciones: nombre inválido']);

// --- Expresiones: dibujos (24 en total, 16 de base) y paramétricas (48, 38 de base) ---
const dibujo = (i) => ({ formas: [{ forma: 'circulo', x: -0.3 + i * 0.05, y: 0, r: 0.1 }] });
caso('expresiones-dibujo-8.json', v2({ expresiones: objeto(8, (i) => `dibujo-${i}`, dibujo) }), true);
caso('expresiones-dibujo-9.json', v2({ expresiones: objeto(9, (i) => `dibujo-${i}`, dibujo) }), [E.demasiadas, 'más de 24 expresiones'], { esquema: true });
caso('expresion-13-formas.json', v2({ expresiones: { muchas: { formas: n(13, (i) => dibujo(i).formas[0]) } } }),
  [E.demasiadas, 'expresion: más de 12 formas']);
caso('expresion-forma-desconocida.json', v2({ expresiones: { rara: { formas: [{ forma: 'estrella', x: 0, y: 0 }] } } }), [E.forma, '']);
const parametrica = (i) => ({ extiende: 'neutral', gy: 0.01 * i });
caso('expresiones-vida-10.json', v2({ expresiones: objeto(10, (i) => `propia-${i}`, parametrica) }), true);
caso('expresiones-vida-11.json', v2({ expresiones: objeto(11, (i) => `propia-${i}`, parametrica) }),
  [E.demasiadas, 'demasiadas expresiones paramétricas'], { esquema: true });
// Reemplazar una de base no gasta lugar: 10 nuevas más 2 de base reescritas entran.
caso('expresiones-vida-10-y-reemplazos.json', v2({ expresiones: { ...objeto(10, (i) => `propia-${i}`, parametrica), contento: { gy: 0.03 }, triste: { extiende: 'neutral', gy: -0.02 } } }), true);
caso('expresion-extiende-inexistente.json', v2({ expresiones: { mia: { extiende: 'no-existe' } } }), [E.campo, 'expresion: extiende una expresión que no existe'], { esquema: true });

// --- Caras de imagen (sprites 0 a 11, hasta 8 caras) ---
const imagen = (i) => ({ sprites: { ojo: i % 12, ojo_cerrado: (i + 1) % 12, boca: (i + 2) % 12 }, tamano: 0.4, boca_y: -0.3 });
caso('imagen-una.json', v2({ cara: { expresion: 'anime' }, expresiones: { anime: { ...imagen(0), ojo: { lt: 0.1 }, movimiento: 'rebote' } } }), true);
caso('imagen-8.json', v2({ expresiones: objeto(8, (i) => `imagen-${i}`, imagen) }), true);
caso('imagen-9.json', v2({ expresiones: objeto(9, (i) => `imagen-${i}`, imagen) }), [E.demasiadas, 'expresiones: hasta 8 caras de imagen'], { esquema: true });
caso('imagen-sprite-11.json', v2({ expresiones: { a: { sprites: { ojo: 11, ojo_der: 0, ojo_der_cerrado: 1 }, espejar_der: false } } }), true);
caso('imagen-sprite-12.json', v2({ expresiones: { a: { sprites: { ojo: 12 } } } }),
  [E.campo, 'expresion: cada parte de sprites es el número de un sprite, de 0 a 11']);
caso('imagen-sin-ojo.json', v2({ expresiones: { a: { sprites: { boca: 1 } } } }), [E.campo, 'expresion: una cara de imagen necesita al menos sprites.ojo']);
caso('imagen-parte-desconocida.json', v2({ expresiones: { a: { sprites: { ojo: 0, nariz: 1 } } } }),
  [E.campo, 'expresion: las partes de sprites son ojo, ojo_cerrado, ojo_der, ojo_der_cerrado y boca']);
caso('imagen-tamano-sin-sprites.json', v2({ expresiones: { a: { tamano: 0.4, gx: 0.1 } } }),
  [E.campo, 'expresion: tamano, boca_tamano, boca_y y espejar_der van solo en una cara de imagen']);
caso('imagen-tamano-grande.json', v2({ expresiones: { a: { sprites: { ojo: 0 }, tamano: 0.9 } } }), [E.campo, 'expresion: tamano de 0,08 a 0,8']);

// --- Comportamientos (67 en total, 58 de base) ---
const comp = (i) => ({ tipo: 'reaccion', pasos: [{ cara: 'contento', dura: 1 + i * 0.1 }] });
caso('comportamientos-9.json', v2({ comportamientos: objeto(9, (i) => `propio-${i}`, comp) }), true);
caso('comportamientos-10.json', v2({ comportamientos: objeto(10, (i) => `propio-${i}`, comp) }), [E.demasiadas, 'demasiados comportamientos'], { esquema: true });

// --- Reglas del usuario (24) ---
const regla = (i) => ({ si: ['tacto', 'caricia', 'sacudida', 'levantado'][i % 4], entonces: [`expresion:${['contento', 'sorpresa', 'mareado'][i % 3]}`] });
caso('reglas-24.json', v2({ reglas: n(24, regla) }), true);
caso('reglas-25.json', v2({ reglas: n(25, regla) }), [E.demasiadas, 'reglas: más de 24']);
caso('regla-6-acciones.json', v2({ reglas: [{ si: 'tacto', entonces: n(6, () => 'estado.animo+0.1') }] }), true);
caso('regla-7-acciones.json', v2({ reglas: [{ si: 'tacto', entonces: n(7, () => 'estado.animo+0.1') }] }), [E.regla, 'regla: "entonces" de 1 a 6 acciones']);
caso('regla-si-desconocido.json', v2({ reglas: [{ si: 'estornudo', entonces: ['expresion:contento'] }] }), [E.regla, 'regla: "si" desconocido'], { esquema: true });
caso('regla-y-invalido.json', v2({ reglas: [{ si: 'tacto', y: 'afecto>>0.6', entonces: ['expresion:contento'] }] }), [E.regla, 'regla: "y" inválido']);
caso('regla-decir-desconocido.json', v2({ reglas: [{ si: 'tacto', entonces: ['decir:chiste'] }] }), [E.regla, 'regla: decir:<situación>'], { esquema: true });
caso('regla-decir.json', v2({ reglas: [{ si: 'caricia', y: 'afecto>0.6,noche', entonces: ['decir:caricia', 'estado.afecto+0.05'] }] }), true);

// --- Lista ---
caso('lista-16.json', v2({ lista: n(16, (i) => (i < 8 ? `imagen:${i}` : 'cara:contento')) }), true);
caso('lista-17.json', v2({ lista: n(17, () => 'cara:contento') }), [E.campo, 'lista: hasta 16 elementos']);
caso('lista-imagen-8.json', v2({ lista: ['imagen:8'] }), [E.campo, 'lista: usá cara:<nombre>, animacion:<nombre> o imagen:<0-7>']);
caso('lista-cara-inexistente.json', v2({ lista: ['cara:no-existe'] }), [E.campo, 'lista: una cara que no existe'], { esquema: true });

// --- Recordatorios (12, texto de 1 a 40 bytes) ---
const recordatorio = (i, texto = `Recordatorio ${i}`) => ({ hora: `${String(8 + (i % 12)).padStart(2, '0')}:${String(i * 5).padStart(2, '0')}`, dias: 'LMXJV', texto });
caso('recordatorios-12.json', v2({ recordatorios: n(12, (i) => recordatorio(i)) }), true);
caso('recordatorios-13.json', v2({ recordatorios: n(13, (i) => recordatorio(i)) }), [E.demasiadas, 'recordatorios: hasta 12']);
caso('recordatorio-40-tildes.json', v2({ recordatorios: [recordatorio(0, conTildes(40))] }), true);
caso('recordatorio-41-tildes.json', v2({ recordatorios: [recordatorio(0, conTildes(41))] }), [E.campo, 'recordatorios: texto de 1 a 40 bytes'], { esquema: true });
caso('recordatorio-40-emoji.json', v2({ recordatorios: [recordatorio(0, conEmoji(40))] }), true);
caso('recordatorio-41-emoji.json', v2({ recordatorios: [recordatorio(0, conEmoji(41))] }), [E.campo, 'recordatorios: texto de 1 a 40 bytes'], { esquema: true });
caso('recordatorio-texto-vacio.json', v2({ recordatorios: [recordatorio(0, '')] }), [E.campo, 'recordatorios: texto de 1 a 40 bytes']);
caso('recordatorio-hora-mala.json', v2({ recordatorios: [{ hora: '24:00', texto: 'Tarde' }] }), [E.campo, 'recordatorios: hora es HH:MM']);
caso('recordatorio-dias-repetidos.json', v2({ recordatorios: [{ hora: '08:00', dias: 'LL', texto: 'Dos veces' }] }), [E.campo, 'recordatorios: dias son letras']);
caso('recordatorio-todos-los-dias.json', v2({ recordatorios: [{ hora: '23:59', dias: 'LMXJVSD', texto: 'Todos' }, { hora: '00:00', dias: '', texto: 'Vacío' }] }), true);
caso('recordatorio-clave-de-mas.json', v2({ recordatorios: [{ hora: '08:00', texto: 'Hola', color: 'rojo' }] }), [E.campo, 'recordatorios: claves válidas']);

// --- Habla: frases propias (12 en total, de 1 a 60 bytes) ---
const sit = ['despertar', 'caricia', 'saludo', 'buen-dia'];
const propias = (k, f = (i) => `Frase ${i}`) => {
  const o = {};
  for (let i = 0; i < k; i++) (o[sit[i % sit.length]] ??= []).push(f(i));
  return o;
};
caso('frases-12.json', v2({ habla: { frecuencia: 0.5, propias: propias(12) } }), true);
caso('frases-13.json', v2({ habla: { propias: propias(13) } }), [E.campo, 'habla: hasta 12 frases propias en total'], { esquema: true });
caso('frase-60-tildes.json', v2({ habla: { propias: { despertar: [conTildes(60)] } } }), true);
caso('frase-61-tildes.json', v2({ habla: { propias: { despertar: [conTildes(61)] } } }), [E.campo, 'habla: cada frase de 1 a 60 bytes'], { esquema: true });
caso('frase-60-emoji.json', v2({ habla: { propias: { caricia: [conEmoji(60)] } } }), true);
caso('frase-61-emoji.json', v2({ habla: { propias: { caricia: [conEmoji(61)] } } }), [E.campo, 'habla: cada frase de 1 a 60 bytes'], { esquema: true });
// Los mismos 60 bytes, pero escritos con escapes \uXXXX (como los manda Python): 15 emoji en pares sustitutos.
caso('frase-60-emoji-escapado.json', `{"version": 2, "habla": {"propias": {"caricia": ["${'\\ud83d\\ude00'.repeat(15)}"]}}}\n`, true);
caso('frase-60-tildes-escapado.json', `{"version": 2, "habla": {"propias": {"despertar": ["${'\\u00e1'.repeat(30)}"]}}}\n`, true);
caso('frase-vacia.json', v2({ habla: { propias: { despertar: [''] } } }), [E.campo, 'habla: cada frase de 1 a 60 bytes']);
caso('habla-situacion-desconocida.json', v2({ habla: { propias: { chiste: ['Ja'] } } }), [E.campo, 'habla: situación desconocida']);
caso('habla-frecuencia-1-5.json', v2({ habla: { frecuencia: 1.5 } }), [E.campo, 'habla: frecuencia de 0 a 1']);

// --- Despertador ---
caso('despertador-ok.json', v2({ despertador: { hora: '07:30', dias: 'LMXJV', activo: true, sonido: true, texto: conTildes(60), posponer: 30 } }), true);
caso('despertador-hora-24.json', v2({ despertador: { hora: '24:00' } }), [E.campo, 'despertador: hora es HH:MM']);
caso('despertador-hora-sin-cero.json', v2({ despertador: { hora: '7:30' } }), [E.campo, 'despertador: hora es HH:MM']);
caso('despertador-minuto-60.json', v2({ despertador: { hora: '07:60' } }), [E.campo, 'despertador: hora es HH:MM']);
caso('despertador-sin-hora.json', v2({ despertador: { dias: 'LMXJV' } }), [E.campo, 'despertador: hora es HH:MM']);
caso('despertador-texto-61.json', v2({ despertador: { hora: '07:30', texto: conTildes(61) } }), [E.campo, 'despertador: texto de 1 a 60 bytes'], { esquema: true });
caso('despertador-posponer-31.json', v2({ despertador: { hora: '07:30', posponer: 31 } }), [E.campo, 'despertador: posponer en minutos, de 1 a 30']);

// --- Ubicación ---
caso('ubicacion-borde.json', v2({ ubicacion: { lat: -90, lon: 180, ciudad: conTildes(40) } }), true);
caso('ubicacion-lat-90-5.json', v2({ ubicacion: { lat: 90.5, lon: 0 } }), [E.campo, 'ubicacion: lat de -90 a 90']);
caso('ubicacion-lon-181.json', v2({ ubicacion: { lat: 0, lon: -181 } }), [E.campo, 'ubicacion: lon de -180 a 180']);
caso('ubicacion-sin-lat.json', v2({ ubicacion: { lon: 0, ciudad: 'Rosario' } }), [E.campo, 'ubicacion: lat de -90 a 90']);
caso('ubicacion-ciudad-41.json', v2({ ubicacion: { lat: 0, lon: 0, ciudad: conTildes(41) } }), [E.campo, 'ubicacion: ciudad de 1 a 40 bytes'], { esquema: true });

// --- Avisos (8; app y contacto de 24 bytes, decir de 40) ---
const aviso = (i) => ({ tipo: i % 2 ? 'llamada' : 'notificacion', app: `App ${i}`, expresion: 'sorpresa', decir: '{app}' });
caso('avisos-8.json', v2({ avisos: n(8, aviso) }), true);
caso('avisos-9.json', v2({ avisos: n(9, aviso) }), [E.campo, 'avisos: hasta 8']);
caso('aviso-borde.json', v2({ avisos: [{ tipo: 'llamada', contacto: conTildes(24), comportamiento: 'investigar', decir: conEmoji(40), sonido: false }] }), true);
caso('aviso-app-25.json', v2({ avisos: [{ tipo: 'notificacion', app: conTildes(25) }] }), [E.campo, 'avisos: app y contacto de hasta 24 bytes'], { esquema: true });
caso('aviso-decir-41.json', v2({ avisos: [{ tipo: 'notificacion', decir: conTildes(41) }] }), [E.campo, 'avisos: decir de 1 a 40 bytes'], { esquema: true });
caso('aviso-tipo-malo.json', v2({ avisos: [{ tipo: 'mensaje' }] }), [E.campo, 'avisos: tipo es notificacion o llamada']);
caso('aviso-expresion-inexistente.json', v2({ avisos: [{ tipo: 'llamada', expresion: 'no-existe' }] }), [E.campo, 'avisos: una expresion que no existe'], { esquema: true });

// --- Hábitos (4; nombre de 16 bytes) y presencia (texto de 40), v1.9 ---
const habito = (i) => ({ nombre: `Hábito ${i}`, icono: ['gota', 'libro', 'pesa', 'zen'][i % 4], hora: `0${i + 6}:30`, dias: 'LMXJV' });
caso('habitos-4.json', v2({ habitos: n(4, habito) }), true);
caso('habitos-5.json', v2({ habitos: n(5, habito) }), [E.campo, 'habitos: hasta 4']);
caso('habito-borde.json', v2({ habitos: [{ nombre: conTildes(16), icono: 'estrella' }, { nombre: conEmoji(16) }] }), true);
caso('habito-nombre-17.json', v2({ habitos: [{ nombre: conTildes(17) }] }), [E.campo, 'habitos: nombre de 1 a 16 bytes'], { esquema: true });
caso('habito-sin-nombre.json', v2({ habitos: [{ icono: 'gota' }] }), [E.campo, 'habitos: nombre de 1 a 16 bytes']);
caso('habito-icono-malo.json', v2({ habitos: [{ nombre: 'Agua', icono: 'perro' }] }), [E.campo, 'habitos: icono es gota']);
caso('habito-hora-mala.json', v2({ habitos: [{ nombre: 'Agua', hora: '24:00' }] }), [E.campo, 'habitos: hora es HH:MM']);
caso('habito-dias-repetidos.json', v2({ habitos: [{ nombre: 'Agua', dias: 'LL' }] }), [E.campo, 'habitos: dias son letras']);
caso('presencia-borde.json', v2({ presencia: { preguntar: false, texto: conEmoji(40) } }), true);
caso('presencia-vacia.json', v2({ presencia: { texto: '' } }), true);
caso('presencia-texto-41.json', v2({ presencia: { texto: conTildes(41) } }), [E.campo, 'presencia: texto de hasta 40 bytes'], { esquema: true });
caso('presencia-clave-mala.json', v2({ presencia: { preguntar: true, avisar: true } }), [E.campo, 'presencia: claves válidas']);

// --- Voz, pomodoro, protector, cuidado, modo ---
caso('voz-palabra-16.json', v2({ voz: { volumen: 0, tipo: 'robot', palabra: conTildes(16), conversar: true, microfono: 1 } }), true);
caso('voz-palabra-17.json', v2({ voz: { palabra: conTildes(17) } }), [E.campo, 'voz: palabra de 1 a 16 bytes'], { esquema: true });
caso('voz-tipo-malo.json', v2({ voz: { tipo: 'susurro' } }), [E.campo, 'voz: tipo es grave, media, aguda o robot']);
caso('pomodoro-borde.json', v2({ pomodoro: { trabajo: 120, descanso: 60 } }), true);
caso('pomodoro-121.json', v2({ pomodoro: { trabajo: 121 } }), [E.campo, 'pomodoro: trabajo en minutos, de 1 a 120']);
caso('protector-imagen-8.json', v2({ protector: { imagen: 8 } }), [E.campo, 'protector: imagen es el espacio de una imagen, de 0 a 7']);
caso('protector-espera-2.json', v2({ protector: { imagen: 0, espera: 2 } }), [E.campo, 'protector: espera en segundos, de 3 a 3600']);
caso('cuidado-agua-9.json', v2({ cuidado: { agua: 9 } }), [E.campo, 'cuidado: agua cada 1 a 8 horas']);
caso('cuidado-trabajo-al-reves.json', v2({ cuidado: { trabajo: { desde: '18:00', hasta: '09:00' } } }), [E.campo, 'cuidado: trabajo.desde antes que trabajo.hasta'], { esquema: true });
caso('modo-desconocido.json', v2({ modo: 'radio' }), [E.campo, 'modo desconocido']);
caso('modo-pomodoro.json', v2({ modo: 'pomodoro' }), true);
caso('horario-igual.json', v2({ horario: { despierta: '08:00', duerme: '08:00' } }), [E.campo, 'horario: despierta y duerme no pueden ser iguales'], { esquema: true });
caso('color-malo.json', v2({ cara: { color: '#FF6A3' } }), [E.campo, 'cara: color debe ser #RRGGBB']);

// Los cortes de bytes que hacen las apps (cortarBytes en la web, Kotlin y
// Swift): se corta sin partir una letra (ni un emoji ni un par sustituto).
function cortar(t, max) {
  let r = '';
  for (const letra of t) {
    if (bytes(r + letra) > max) break;
    r += letra;
  }
  return r;
}
const cortes = [
  ['Tomar la pastilla', 40],
  [conTildes(41), 40],
  ['a' + 'á'.repeat(30), 60], // 61 bytes: la última tilde no entra entera
  ['😀'.repeat(11), 40], // 44 bytes: entran 10
  ['x😀'.repeat(14), 60], // 70 bytes: el emoji 13 quedaría partido
  ['Ñandú', 5],
  ['Ñandú', 6],
  ['abc', 0],
  ['ñ'.repeat(13), 24], // 26 bytes
  ['👍🏽 listo', 4], // el emoji con tono son dos letras para JS (4 + 4 bytes)
].map(([texto, max]) => ({ texto, max, resultado: cortar(texto, max), bytes: bytes(cortar(texto, max)) }));

const indice = {
  version: 1,
  nota: 'Generado por shared/formato/paridad/generar.mjs. No editar a mano. codigo: el ErrorConfig del núcleo (1 JSON, 2 versión, 3 campo, 4 forma, 5 demasiadas, 6 regla); detalle: el comienzo del detalle del núcleo; esquema: lo que dice config.schema.json cuando no coincide con el núcleo.',
  limites,
  cortes,
  casos: casos.map(({ texto, ...c }) => c),
};

const salida = new Map(casos.map((c) => [path.join(dir, c.archivo), c.texto]));
salida.set(path.join(aqui, 'paridad.json'), JSON.stringify(indice, null, 2) + '\n');
if (verificar) {
  let mal = 0;
  for (const [f, t] of salida) {
    if (!fs.existsSync(f) || fs.readFileSync(f, 'utf8') !== t) {
      console.log(`desactualizado: ${path.relative(aqui, f)}`);
      mal++;
    }
  }
  const sobran = fs.readdirSync(dir).filter((f) => !salida.has(path.join(dir, f)));
  for (const f of sobran) console.log(`sobra: configs/${f}`), mal++;
  if (mal) process.exit(1);
  console.log(`paridad: ${casos.length} casos al día`);
} else {
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
  for (const [f, t] of salida) fs.writeFileSync(f, t);
  console.log(`paridad: ${casos.length} casos en ${path.relative(process.cwd(), dir)}`);
}
