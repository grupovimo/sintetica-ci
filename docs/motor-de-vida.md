# El motor de vida (cómo está hecho)

La especificación está en [motor-de-vida-especificacion.md](motor-de-vida-especificacion.md)
(exportada de la pestaña "Motor de vida" del doc). Acá está **cómo** se
implementó, dónde vive cada cosa y qué decisiones se tomaron donde la
especificación dejaba lugar. La referencia visual es el prototipo
[Motor de caras v0](referencia/motor-de-caras-v0/index.html) (WebGL): el núcleo
porta sus parámetros, sus funciones de distancia y sus resortes tal cual.

## En una línea por capa

| Capa (spec) | Dónde | Qué hace |
| --- | --- | --- |
| Sensores | `firmware/perfiles/*` → `Sistema::tacto`, `imu`, `Hal::bateria`, `cargando`, `epoch` | Lo crudo de cada placa |
| Percepción | `nucleo/gestos.cpp`, `Sistema::vigilar` | Toque, doble toque, caricia, muchos toques, sacudida suave/fuerte, caída libre, inclinado, boca abajo; batería baja/crítica, enchufado, cargado, sin WiFi |
| Estado interno | `MotorVida::actualizarEstado` | 7 variables que suben con los eventos y vuelven a su reposo con el tiempo |
| Decisión | `MotorVida::elegirComportamiento`, `reglaPara` | Selector por utilidad cada 250 ms; las reglas para los eventos |
| Comportamiento | `MotorVida::comportamiento`, `empezarPaso`, `avanzarComportamiento` | Secuencias de pasos (cara, mirada, parpadeo, partículas, sonido, empujón al estado) |
| Micro-vida | `MotorVida::microVida` | Parpadeo, saccades, micro-saccades, respiración, deriva, postura, ruido de forma |
| Mezclador | `MotorVida::calcularObjetivos` + resortes en `paso` | Reposo + ánimo de fondo + capa, escalonado y limitado a la marca |
| Render | `nucleo/rostro.cpp` | SDF por bloques en la CPU: el mismo código en el bicho, el simulador, la web y las apps |

## El rostro (`firmware/nucleo/rostro.h/.cpp`)

- **Parámetros**: 15 por ojo (`w h r tilt lt lta lb lba smile heart star cross
  ring pupil squash`) y 18 de la cara (`gx gy ey sep ftilt fscale spin orbit
  pulse beat glitch dim hue mo mw mcurve blush shake`), en un arreglo plano de
  48 floats (`param::`). La tabla `INFO_OJO`/`INFO_GLOBAL` tiene, para cada uno,
  el nombre del JSON, los límites de la marca, el valor de la neutral (los del
  v0) y su grupo de resorte. Es la única fuente: el esquema JSON, la web
  (`PARAMS_OJO`/`PARAMS_CARA`) y el simulador (`sim.parametros()`) la repiten o
  la leen.
- **Coordenadas**: como el v0, (0, 0) en el centro y **y hacia arriba**. La
  unidad es `VistaRostro::unidad` píxeles: en las pantallas rectangulares, el
  49 % del ancho (así la neutral, `h = 0,23`, mide 72 px de alto en la 1.47B,
  como en el manual; hay un test que lo mide); en la redonda, el 60 %.
- **SDF**: el ojo del v0 (rectángulo redondeado + mezcla a corazón + párpados
  como semiplanos inclinados + recorte del arco feliz + la línea curva del ojo
  cerrado), más lo que pide la especificación: estrella (`sdStar5`), cruz,
  anillo (con la parte que falta cargar apagada) y pupila (un disco oscuro que
  sigue la mirada). La boca es la elipse del v0 más una curva para `mcurve`
  (sonrisa o puchero). Los cachetes son dos manchas suaves. El glitch es el del
  v0: bandas corridas y separación de color (tres evaluaciones).
- **Por bloques de 4 × 4** (para el C3, sin FPU): se evalúa la SDF en el centro
  del bloque; lejos afuera es fondo, bien adentro es relleno parejo, y solo cerca
  del borde se evalúa píxel por píxel. El resplandor del v0 se calcula en las
  cuatro esquinas del bloque y se interpola (una SDF cada 16 píxeles), y llega a
  cero justo en su alcance para que no quede un escalón. La grilla de bloques es
  absoluta, así el resultado es el mismo dibujando de una vez o por franjas (hay
  un test). En la neutral de la 1.47B, menos del 10 % de los píxeles necesita la
  SDF fina.
- **Partículas**: zzz, corazón, lágrima, gota, chispa, ?, !, nota, estrella y
  burbuja, como SDF chiquitas; nacen, flotan, se balancean y se apagan. Nunca
  más de 6 (si no hay lugar, reemplaza a la más vieja).
- El broche no dibuja el resplandor (`VistaRostro::halo = false`), por costo.

## El motor (`firmware/nucleo/motor_vida.h/.cpp`)

### Tiempo y determinismo

- La lógica corre a **60 Hz fijos** (`PASO_MS`), sin importar los cuadros por
  segundo; el estado, el ritmo y el selector, cada 250 ms (`LENTO_MS`).
- Si entre dos ticks pasan más de 2 s (sueño profundo, un test que salta días,
  el simulador acelerado), **salta**: avanza el estado, el ritmo y los
  comportamientos en tramos de un minuto y deja los resortes en sus objetivos.
- Todo el azar sale de un `Azar` sembrado con `config.semilla` (o la del
  hardware): misma semilla y mismas entradas, mismos cuadros (test). El ruido
  suave (deriva, ruido de forma) es ruido de valor con hash de la semilla.

### Resortes y capas

- Un resorte por parámetro con la tabla de la especificación (forma 170/17,
  párpados 260/24, mirada 380/28, cara 120/11, color 60/15). La mirada de la
  micro-vida usa 90/18 cuando sigue algo (un dedo, la inclinación, las ovejas).
  El parpadeo es otro resorte (900/40), como el v0, y cierra los párpados hacia
  el medio. La anticipación es el resorte `aplastar` del v0 (260/11): cada
  cambio de cara le da un impulso proporcional a cuánto cambia y a la energía.
- La rigidez se multiplica por `0,8 + 0,4 · energía` (cansado = más blando;
  dormido, un 25 % más) y un reflejo la sube hasta 2,5 veces por un momento.
- **Mezcla**: `T = reposo + Σ pesoᵢ · (ánimoᵢ − neutral)`; después la capa
  (reflejo o comportamiento) interpola desde ahí: `T += peso · (cara − T)` y el
  ánimo sigue sumando encima (así "un 40 % de cansado en todo lo que hace").
  Al final, la capa de movimiento, la postura y el ruido de forma, y todo se
  limita a la marca.
- **Escalonado**: al cambiar la capa, la mirada va ya, los párpados a los 40 ms,
  la cara entera a los 60 y la forma a los 90 ("primero mira, después siente").
  Un reflejo no se escalona.
- **Prioridades**: fija > (reflejo > comportamiento/reacción > ánimo de fondo).
  El reposo es: la fija, la cara de una regla de estado (`bateria<15`), la de la
  lista, dormido o la neutral.
- **Dibujos de formas** (las caras v1 propias del usuario): si la cara que toca
  es un dibujo, el `Sistema` lo dibuja encima del rostro con un fundido de
  200 ms, nunca con un corte (`MotorVida::dibujo`, `opacidadDibujo`).

### Micro-vida

| Elemento | Implementación |
| --- | --- |
| Parpadeo | Log-normal (mediana 3,3 s, σ 0,38) entre 2 y 6 s; 15 % doble; cierra en 85 ms y abre con el resorte. Más seguido con sueño o estrés; las caras y los pasos pueden pedir `menos`, `mas`, `no`, `lento`, `rapido` o `doble`. Siempre parpadea después de un salto grande de mirada. En 30 min da ~17/min con ~13 % de dobles (test). |
| Saccades | Cada 0,7 a 3 s (más seguido con energía); cerca del centro, con preferencia por el último estímulo; más abajo con ánimo bajo o timidez; al frente si a esa hora suele aparecer el dueño. Los pasos eligen otras: frente, arriba, abajo, lado, alrededor, perdida, reojo, estímulo, REM (soñando), ovejas. |
| Micro-saccades | 0,005 a 0,01 cada 150 a 400 ms (casi nada en una cara "quieta"). |
| Respiración | ±1,2 % a 16 por minuto (más rápida con estrés); dormido, ±3,5 % a 8,6. |
| Deriva | ±0,012 con ruido suave (nunca un ciclo). |
| Postura | Cada 20 a 60 s, un poco de inclinación y altura. |
| Ruido de forma | ±2 % en `w` y `h`, con ruido muy lento, distinto por ojo. |

### Estado interno (`actualizarEstado`)

| Variable | Reposo y tiempo | Empujones |
| --- | --- | --- |
| Energía | 0,55 ± personalidad; +0,15 las 3 h después de despertar, −0,12 de 13:30 a 15:30, −0,2 en las 2 h antes de dormir, −0,25 con batería < 20 %; τ 1 h (dormido, sube a 1 con τ 3 h) | cada interacción gasta un poco; reglas |
| Ánimo | 0,5 + 0,15·(afecto − 0,3), menos si está aburrido o con poca batería; τ 2 h | reglas (`estado.animo+0.08` en una caricia, etc.) |
| Afecto | casi no decae; después de 48 h sin verte, −0,01 por día | cada interacción, con tope diario (`afectoHoy`) |
| Aburrimiento | sube sin estímulos con τ = 1,6 h·(1,25 − 0,5·sociabilidad); a las 2 h ignorado pasa 0,6 (test) | cualquier interacción lo multiplica por 0,2 |
| Curiosidad | 0,2 + 0,5·rasgo, τ 10 min; cada 3 a 9 min "algo le llama la atención" | lo nuevo sube; lo repetido cada vez menos (habituación por evento) |
| Sueño | presión que sube despierto (más rápido con poca energía) y baja durmiendo, + el ritmo del día (+0,35 en la hora y media antes de dormir, +0,45 de noche, +0,12 después de comer, un poco al despertar) | — |
| Estrés | vuelve a 0 con τ 5 min·(1 + mal genio) | sacudidas, caídas, ruidos, boca abajo; las caricias lo bajan |

El **ánimo de fondo** sale de ahí: cansado, triste, contento, aburrido, gruñón,
miedo, curioso y ofendido, con pesos que se normalizan si pasan de 1. El estado
se guarda cada 5 minutos con la memoria.

### Personalidad

Seis rasgos (`energia`, `sociabilidad`, `picardia`, `timidez`, `curiosidad`,
`malGenio`) y los presets de `shared/formato/vida/personalidades.json`
(equilibrado, tímido, pícaro, dormilón, cariñoso, curioso, gruñón, eléctrico).
Cambian los reposos, las velocidades, las afinidades del selector y las
variantes de las reglas (`"y": "rasgo.timidez>0.65"`). La **deriva** (±10 % como
mucho, en `Memoria::deriva`) la mueve el trato: las caricias suben la
sociabilidad y bajan el mal genio; las sacudidas fuertes suben la timidez.

### Selector y comportamientos

- Cada 250 ms, si está despierto, sin cara fija ni de lista y libre: para cada
  comportamiento **ocioso** fuera de su enfriamiento y que no esté en los
  últimos 5, si se cumplen sus condiciones (`cuando`), puntaje =
  `necesidad × afinidad × contexto − repetición + ruido(±0,12)`; la necesidad
  sale de cuánto se pasan las condiciones (o de `base` si no tiene); los raros
  tiran un dado (`probabilidad`). Se sortea entre los 3 mejores. Si pasan 45 s
  sin candidatos, la anti-repetición se afloja a los últimos 2.
- Entre un comportamiento y otro hay una pausa de 3 a 12 s (más corta con
  energía): mientras tanto, solo micro-vida y ánimo.
- Los **reflejos** (`reflejo:x` en una regla) saltean todo: cambian la capa en
  el mismo tick y endurecen los resortes; en 48 ms la cara ya se movió el 80 %
  del rango (test < 50 ms).
- Los 55 comportamientos base están en `shared/formato/vida/comportamientos.json`:
  los ociosos de la tabla de la especificación (mirar alrededor, investigar,
  bostezar, cabecear, suspirar, llamar la atención, ofenderse, tararear,
  estornudar, hipo, soñar despierto, practicar caras, fingir dormir, glitch,
  acomodarse; más pensar y estirarse), las rutinas (despertar, contar ovejas,
  soñar, pesadilla) y las reacciones a estímulos.
- Con la personalidad por defecto y sin tocarlo, en una hora hace 8 o más
  distintos sin repetir seguido (test con tres semillas: 10).

### Reglas

`shared/formato/vida/reglas.json`: la primera que coincide gana; las del usuario
van antes. Un evento sin regla prueba con su "padre" (sacudida-fuerte →
sacudida, caricia → tacto, caída → levantado, nombre → voz), así las reglas v1
siguen andando. Tocarlo dormido es `despertado`. Las acciones:
`expresion:x` (3 s), `reflejo:x`, `comportamiento:x`, `estado.var±n`,
`particula:x`, `sonido:x`, `led:#RRGGBB`, `animacion:x`.

### Ritmo diario

- Con hora (`Contexto::minutoDelDia`/`epoch`): en el horario de dormir y 5 min
  quieto, cuenta ovejas (una vez por noche) y se duerme. Despierta solo a la
  hora con la rutina "despertar". Siesta si no es de noche, 20 min quieto y
  sueño > 0,6 (15 a 30 min). Soñando cada 20 a 40 min (pesadilla si hay estrés).
  Brillo: −30 % en la última hora antes de dormir; dormido, 40 %; nunca más que
  `limites_marca.brillo_max`.
- Sin hora: duerme si lleva 10 min quieto con mucho sueño, y despierta cuando
  baja la presión.
- **Despertar forzado**: sobresalto, gruñón de 2 a 5 min (salvo afecto > 0,7) y,
  si sigue siendo de noche, a los 3 min sin tocarlo se vuelve a dormir.
- Con una cara elegida (fija o de la lista) no se duerme solo (como en la v1).
- **El broche**: de noche, dormido hace más de un minuto, apaga todo
  (`quiereSuenoProfundo`) y el perfil programa el despertador del RTC a la hora
  de levantarse (`segundosHastaDespertar`); el botón lo despierta antes.

### Memoria y relación (`Memoria`, 112 bytes con CRC)

Días juntos, racha, caricias, toques, sacudidas, a qué hora aparece el dueño
(24 contadores; lo espera a esa hora), gustos (cuánto le subió el ánimo cada
estímulo), hitos (primer día, 7, 30 y 100 días, 100 y 1000 caricias, los dos
cumpleaños, una vez por año), rencor (después de una sacudida fuerte, duda al
levantarlo durante un día), ausencias (desde 3 h, saluda al volver; desde
3 días, primero se hace el ofendido: test), la deriva y el estado. Se guarda en
la flash (`"vida"`) cada 5 minutos y en los hitos, y viaja por el protocolo
(`0x1C`/`0x1D`). Los hitos se festejan cuando está libre (no pisan la reacción
a lo que pasó).

## La config v2

Ver `shared/formato/config.schema.json` y `shared/formato/config-ejemplo-v2.json`.
Una config v1 sigue siendo válida (su `personalidad.sueno` pasa al horario).

- `semilla`, `personalidad` (rasgos o `preset`), `horario {despierta, duerme}`,
  `dueno {nombre, cumple}`, `cumple` (el del bicho), `limites_marca {color, brillo_max}`.
- `expresiones`: con `formas` es un **dibujo** (v1); si no, una **expresión
  paramétrica**: `ojo` (los dos), `izq`, `der`, los parámetros de la cara,
  `movimiento`, `particula`, `parpadeo` y `extiende`. Una con el nombre de una
  base se aplica **encima** de la base (se guarda solo lo que cambia); una
  nueva parte de la neutral (o de la que extiende).
- `comportamientos`: `tipo`, `cuando`, `necesidad`, `base`, `probabilidad`,
  `enfriamiento`, `interrumpible`, `afinidad` y hasta 6 `pasos` (`cara`, `dura`,
  `veces`, `mirada`, `parpadeo`, `particula`, `cuantas`, `peso`, `sonido`, `estado`).
- `reglas`: `si`, `y` (condiciones extra) y hasta 6 acciones.

- `recordatorios` (hasta 12): `hora` `"HH:MM"` (obligatoria), `dias` (letras
  de `L M X J V S D`, lunes a domingo, sin repetir y en cualquier orden; vacío
  u omitido = todos los días), `texto` (obligatorio, de 1 a 40 **bytes** UTF-8:
  una tilde ocupa 2, como el nombre del dueño), `expresion` (una que exista; por
  defecto `sorpresa`), `sonido` y `activo` (por defecto `true`). En la RAM van
  compactos (`Recordatorio`: minuto del día en `uint16`, días en una máscara
  `uint8`, el texto en `char[41]`, la cara ya resuelta).
- `voz` (todo opcional): `volumen` y `microfono` de 0 a 1 (por defecto 0,7 y
  0,6; se guardan en centésimos), `tipo` `grave|media|aguda|robot` (`media`),
  `palabra` de 1 a 16 bytes (vacía = su nombre) y `conversar` (`false`). Hoy
  solo se guarda y se valida (los modelos con parlante llegan después);
  `volumen` 0 silencia todos los sonidos (`Sistema::sonar`).
- `pomodoro`: `trabajo` (1 a 120 minutos, 25) y `descanso` (1 a 60, 5).
- `protector`: `imagen` (el espacio, 0 a 7, obligatorio) y `espera` (3 a 3600
  segundos, 10).
- `modo`: `mascota` (la cara), `reloj` y `pomodoro` (abajo). `clima` y
  `notificaciones` necesitan datos de la nube que todavía no llegan: hasta
  entonces se comportan como `mascota`.

Límites (iguales en todos lados porque valida el mismo núcleo): 48 expresiones
paramétricas, 64 comportamientos, 24 reglas propias (+ las base), 12
recordatorios, 16 KB.

### Recordatorios, reloj y pomodoro en el bicho (`Sistema`)

Los tres usan el mismo **panel**: la cara se achica (al 45 %) y sube, y abajo
aparece el texto, con un fundido de 400 ms (`tiemposPanel::FUNDIDO_MS`). Si
cambia lo que se muestra (llega un recordatorio en modo reloj), primero se va
lo que había y después entra lo nuevo: nunca un corte. El texto usa las fuentes
de `texto.h`: el del recordatorio va centrado en Fraunces 20 en hasta 2 líneas
(3 en la redonda, con el ancho del círculo a esa altura) y, si no entra, en mono
mayúscula; la hora y la cuenta del pomodoro, en Fraunces agrandada
(`dibujarTextoEscalado`).

- **Recordatorio**: con la hora local (la del motor: `Hal::minutoDelDia`/`epoch`
  o la del comando Hora), cuando entra el minuto de uno activo de un día que
  corresponde (hasta 1 minuto tarde) y no se disparó hoy: muestra el texto,
  fija la expresión pedida (por los resortes), `sonido("recordatorio")` si
  `sonido`, y el evento `recordatorio` (registro, reglas y aviso a la nube,
  ver protocolo-ble.md). Queda hasta que lo tocan (ese toque no hace nada más)
  o 2 minutos. Si estaba dormido, se despierta; el broche programa el
  despertador del sueño profundo para el próximo. Sin fecha (solo el minuto del
  día) se disparan solo los de todos los días. Dos a la vez: el segundo espera.
- **Reloj**: `HH:MM` grande (`--:--` sin hora) con los ojos chiquitos arriba,
  que siguen vivos. Un toque muestra la cara 15 s y vuelve al reloj.
- **Pomodoro**: `MM:SS`, una barra de progreso y `TRABAJO`/`DESCANSO`/`PAUSA`.
  Empieza en pausa; un toque arranca o pausa (en el broche, el botón). Al
  terminar cada tramo: `sorpresa` (terminó el trabajo) o `contento` (terminó el
  descanso), `sonido("pomodoro")`, el evento `pomodoro` y sigue solo con el
  otro tramo. Cambiar el modo o los tramos lo reinicia.
- **Protector de pantalla** (`Sistema::avanzarProtector`): si la imagen del
  espacio existe, después de `espera` segundos sin que pase nada se muestra en
  pantalla completa (QOI o animación, en loop: la misma vía de las imágenes de
  la lista, sin las "vueltas de la cara") con un fundido de 400 ms
  (`Lienzo::filaMezclada` sobre la cara). Cualquier toque, evento (movimiento,
  levantado, sacudida, pomodoro, recordatorio…), comando de la app o reacción
  del motor vuelve a la cara con fundido, y la cuenta empieza de nuevo. Dormido
  duerme normal; un recordatorio siempre le gana; sin la imagen, se ignora.
  Pensado para la joyería: una animación que se ve siempre y, al tocarla, la
  carita.

Los datos base se embeben con `npm run caras` (`firmware/herramientas/embeber_caras.py`
genera `vida_base.h`); cada uno se parsea por separado al arrancar.

## Protocolo v1.4

Ver [protocolo-ble.md](protocolo-ble.md): eventos nuevos (índices 8 a 30) y los
comandos Hora (`0x1A`), Vida (`0x1B`, también prende la superposición de
depuración), Memoria (`0x1C`), Restaurar (`0x1D`), Registro (`0x1E`) y
Comportamiento (`0x1F`). Los vectores de `shared/protocolo/vectores.json` los
prueban en C++, JS, Go, Kotlin y Swift.

## Memoria RAM y rendimiento

- `Config` pasó de ~64 a ~110 KB. En la 1.47B el `Sistema` entero va en la
  PSRAM (RAM interna: 38 %); en el broche (sin PSRAM) se compactaron los datos
  (ajustes en diezmilésimos, pasos de 6, la máscara de trazos más chica) y queda
  en 68 % (sin WiFi). Si hiciera falta más, el próximo paso es dejar los datos
  base en la flash y copiar solo los del usuario.
- Modos de energía: 30 cuadros por segundo despierto, 5 dormido
  (`Sistema::fpsSugerido`), 1 con la pantalla apagada. Falta medir en la placa
  los 30 fps de la 1.47B y los 20 del C3 (el cálculo por bloques está pensado
  para eso) y las 6 h del broche.

## Herramientas

- **Simulador** (`npm run sim`): panel "Motor de vida" con el estado y los
  rasgos en barras, el comportamiento y el último sorteo del selector, todos
  los eventos, el tiempo a ×60 o un día en 2 minutos, saltar a la noche o a la
  mañana, la mirada que sigue al mouse y la superposición.
- **Superposición en el bicho** (comando Vida con `1`, o el botón de la pestaña
  Vida de la web): comportamiento, estado en barras y fps.
- **Registro** de eventos con marca de tiempo (comando `0x1E`, 28 entradas).
- **Web**: pestañas Vida (en vivo, diario, mudanza), Expresiones (editor
  paramétrico con vista previa del núcleo), Comportamientos (pasos, probar acá o
  en el bicho), Config (personalidad, presets, horario) y Reglas v2.

## Criterios de aceptación ↔ tests

| Criterio | Test |
| --- | --- |
| Ningún cuadro idéntico más de 2 s despierto | `vida: despierto, ningún cuadro se repite más de 2 s` (20 min) |
| Sin saltos entre expresiones | `vida: entre expresiones no hay saltos` (600 cambios: ningún parámetro recorre más del 20 % de su rango en 16 ms) |
| 10 a 20 parpadeos por minuto, 10 a 20 % dobles | `vida: parpadea de 10 a 20…` (30 min) y el del simulador |
| 8 comportamientos distintos por hora sin repetir seguido | `vida: en una hora sin tocarlo…` (3 semillas) |
| Aburrido a las 2 h; ofendido a los 3 días | `vida: ignorado 2 h…`, `vida: ignorado 3 días…` |
| Reflejos < 50 ms | `vida: los reflejos responden en menos de 50 ms` |
| Distinta semilla, distinta vida | `vida: misma semilla, mismos cuadros…`, `vida: dos bichos con la misma personalidad…` |
| Todo configurable desde la app | config v2 (tests de formato) + editores de la web + apps |
| La misma cara en todos lados | el mismo núcleo en todas partes; `vida: rostro: … da igual por franjas` |
| Noche y sueños | `vida: de noche cuenta ovejas, se duerme, sueña y se despierta a la mañana` |
| Memoria en la nube y mudanza | `TestMemoriaYMudanza` (e2e en Docker) |
| Broche ≥ 6 h y se duerme de noche | pendiente de medir en la placa (el código del sueño profundo con despertador está) |

## Lo que queda (no entra sin hardware o es de otra etapa)

- Medir fps y consumo en las placas; ajustar el tamaño de bloque si hace falta.
- Micrófono y parlante (el cubito): voz, nombre, ruido, música y "hablando"
  existen como eventos y caras, pero hoy solo llegan por comando.
- Bicho cerca por BLE y la "conversación" entre dos bichos.
- Los modos `clima` y `notificaciones` (necesitan que la nube mande los datos).
- La app todavía no muestra el diario ni tiene los editores de expresiones y
  comportamientos (están en la web); sí la personalidad, los presets y el horario.
- iOS sigue sin compilarse en este entorno (no hay Xcode): revisar en la Mac.
