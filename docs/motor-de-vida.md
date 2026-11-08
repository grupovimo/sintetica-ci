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
- **Partículas**: zzz, corazón, lágrima, gota, chispa, ?, !, nota, estrella,
  burbuja y oveja, como SDF chiquitas (lo que sueña va adentro de la burbuja:
  `Particula::enBurbuja`, sube con ella); nacen, flotan, se balancean y se apagan. Nunca
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
  El reposo es: la fija, la cara de una regla de estado de humor, la de la
  lista, dormido o la neutral. Las reglas de **batería** (`bateria<15`) no son
  el reposo: son una **capa del ánimo** (35 %; enchufado, 12 %), la pantalla
  baja al 85 % y, sin enchufar, la cara se asoma entera 2,5 s cada 1 a 2 min.
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
`particula:x`, `sonido:x`, `led:#RRGGBB`, `animacion:x` y `decir:<situación>`
(pide una frase: ver "Habla").

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
- `expresiones`: con `formas` es un **dibujo** (v1); con `sprites`, una **cara
  de imagen** (abajo); si no, una **expresión
  paramétrica**: `ojo` (los dos), `izq`, `der`, los parámetros de la cara,
  `movimiento`, `particula`, `parpadeo` y `extiende`. Una con el nombre de una
  base se aplica **encima** de la base (se guarda solo lo que cambia); una
  nueva parte de la neutral (o de la que extiende).
- `cara.expresion`: la **cara de siempre** (en vez de la neutral): una
  paramétrica o de imagen que exista. No es una cara fija: sigue durmiéndose
  solo, eligiendo comportamientos y reaccionando (encima de ella).
- `comportamientos`: `tipo`, `cuando`, `necesidad`, `base`, `probabilidad`,
  `enfriamiento`, `interrumpible`, `afinidad` y hasta 6 `pasos` (`cara`, `dura`,
  `veces`, `mirada`, `parpadeo`, `particula`, `cuantas`, `peso`, `sonido`, `estado`,
  `decir`: una situación de frases.json, y `dentro`: con `"particula": "burbuja"`,
  la partícula que va adentro, o `sueno` = al azar entre corazón, estrella, nota y oveja).
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
- `modo`: `mascota` (la cara), `reloj`, `pomodoro` y `clima` (abajo).
  `notificaciones` necesita datos de la nube que todavía no llegan: hasta
  entonces se comporta como `mascota`.
- `habla` (v1.6): `frecuencia` 0 a 1 (0,5; 0 = nunca habla solo) y `propias`
  `{situación: [frases de 1 a 60 bytes]}`, hasta 12 en total (`Habla`, 746 B).
- `despertador` (v1.6): `hora` (obligatoria), `dias`, `activo`, `sonido`,
  `texto` (1 a 60 bytes; sin texto, una frase de "despertador") y `posponer`
  (1 a 30 min, 9). Uno solo (`Despertador`, 70 B). Se decidió una sección
  propia y no `recordatorios` con `alarma: true`: el despertador despierta, suena
  creciente y se pospone, tiene su visor en la consola (Su día) y no le saca
  lugar ni el límite de 40 bytes a los recordatorios.
- `ubicacion` (v1.6): `lat`, `lon`, `ciudad`: la usa la nube para el clima; el
  núcleo solo la valida.

Límites (iguales en todos lados porque valida el mismo núcleo): 48 expresiones
paramétricas (8 de ellas pueden ser de imagen, con 12 sprites), 64
comportamientos, 24 reglas propias (+ las base), 12 recordatorios, 16 KB.

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

## Caras de imagen

"La cara tiene que ser cualquiera que quiera crear la IA": unos ojitos de
anime, de gato, un visor… Una **cara de imagen** es una expresión v2 que arma
los ojos y la boca con **sprites** en vez de con la SDF, y el motor la anima
como a cualquier cara.

### El contrato (config)

```json
"cara": { "expresion": "anime" },
"expresiones": {
  "anime": {
    "sprites": { "ojo": 0, "ojo_cerrado": 1, "boca": 2 },
    "tamano": 0.46, "boca_y": -0.42, "espejar_der": true,
    "sep": 0.3, "ey": 0.02, "blush": 0.5, "movimiento": "rebote"
  }
}
```

- `sprites` (obligatorio, con `ojo`): qué sprite (0 a 11) va en cada parte:
  `ojo` (el izquierdo abierto, mirando al frente), `ojo_cerrado`, `ojo_der`,
  `ojo_der_cerrado` y `boca`. Sin `ojo_der`, el derecho es el izquierdo
  espejado (`espejar_der: false` para no espejarlo).
- `tamano` (0,08 a 0,8; 0,36): el ancho de cada ojo en unidades del rostro.
  Las otras partes van **a la misma escala** que el ojo (en píxeles del
  archivo), salvo la boca con `boca_tamano` (0,04 a 0,9). `boca_y` (−0,7 a 0,3;
  −0,36): la altura de la boca, como la de la SDF.
- Valen todos los parámetros de una expresión paramétrica (`sep`, `ey`, `gx`,
  `ftilt`, `blush`, `ojo.lt`…, `movimiento`, `particula`, `parpadeo`,
  `extiende`): mueven los sprites igual que a los ojos de la SDF. Una que
  extiende una de imagen hereda sus sprites.
- Límites (los valida el mismo núcleo en la placa, la API por cgo y la web por
  wasm): 12 sprites, 8 caras de imagen por config. Los sprites viajan aparte de
  la config (protocolo: espacios 8 a 19 de la característica Imagen, ver
  [protocolo-ble.md](protocolo-ble.md#sprites-v15)); si falta el del ojo, la cara
  se ve con la SDF hasta que llegue.

### Cómo la anima el motor

Es una `CaraVida` más (con `imagen` = su índice en `DatosVida::imagenes`), así
que sirve en todos lados: `reflejo:anime`, `expresion:anime`, un paso
`"cara": "anime"`, la lista (`cara:anime`), un recordatorio, la fija y la de
siempre. `MotorVida::caraImagen()` dice cuál se dibuja: la de la capa (un
reflejo o un paso) si es de imagen; si no, la de la "persona" (fija > estado >
base de la lista > `cara.expresion`; una fija o una base paramétrica la corta).
Las demás caras **la mueven con sus parámetros**: sorpresa agranda los ojos,
dormido los cierra, hablar abre la boca. En `nombreVisible()` gana la de imagen.

`dibujarSprites` (`nucleo/sprites.cpp`) toma la geometría de cada cuadro:

| Del rostro | En los sprites |
| --- | --- |
| `sep`, `gx`, `gy`, `ey`, órbita | dónde va cada ojo (la mirada los desplaza; no hace falta pupila) |
| `w`, `h`, `squash`, latido, pulso | el tamaño (relativo a la neutral) |
| `lt`, `lb` (y el parpadeo, que los lleva a 0,5) | cuánto está abierto: con `ojo_cerrado`, el abierto se aplasta un poco y se funde con el cerrado (entre 22 % y 50 % abierto); sin él, se aplasta hasta una rayita. `smile` también lo cierra (los ojitos felices ^ ^) |
| `tilt`, `ftilt`, `fscale`, respiración, anticipación, deriva, `shake` | el giro y la escala (la misma transformación que la SDF) |
| `glitch` | las bandas corridas |
| `dim`, `hue`, el brillo de la noche y del ritmo | el brillo y el tinte |
| `mw`, `mo` | el ancho y la apertura de la boca |

Los cachetes y las partículas se dibujan igual que siempre (las partículas,
encima). El dibujo es un muestreo inverso afín con interpolación bilineal en
punto fijo (16.16, sin FPU), recortado a la franja: da lo mismo por franjas
que de una vez (test).

### Nunca un corte

`Sistema::avanzarSprites` funde la SDF y los sprites en 200 ms: de la SDF a una
de imagen, los ojos de la SDF se apagan (`ocultarOjos`) mientras los sprites se
prenden; de una de imagen a otra, la vieja se apaga, se cargan los sprites de la
nueva y se prende. Cada cara de imagen tiene una firma (sus partes, su
geometría y los CRC de sus sprites): si se sube un sprite nuevo para la cara
que se ve, se va y vuelve con un fundido (los sprites viejos siguen en la RAM
mientras se va). Si no se pueden leer, vuelve la SDF (también con fundido).

### La memoria (medida)

No se decodifica nada por cuadro: al empezar a mostrar una cara de imagen, sus
partes se decodifican **una vez**, achicadas a lo que se ven en esa pantalla
(nunca más grandes que el archivo), en un buffer fijo (`CacheSprites`). Se
guardan en **RGB565 premultiplicado** por la transparencia y se dibujan
"aclarando" (el máximo por canal): sobre la pantalla casi negra (`#0B0A09`) da
lo mismo que mezclar con la transparencia y ocupa **2 bytes por píxel en vez
de 3** (RGB565 + alfa). Medido (`sizeof`, test "sprites: entran en el buffer
fijo…"):

| | Bytes |
| --- | --- |
| `CacheSprites` (1.47B, simulador, apps, API: 16 384 píxeles) | 32 852 |
| `CacheSprites` en el broche (`-DSINTETICA_CACHE_SPRITES_PX=6144`) | 12 372 |
| Decodificar (`sprites.cpp`: el decodificador QOI, una fila RGBA, las sumas de una fila) | ~3 100 |
| `CaraImagen` × 8 en `Config` + `nImagenes` + `reposo` | 98 |
| `CaraVida` (el índice de imagen entró en el relleno) | +0 (144) |
| Los metadatos de los 12 sprites en `Sistema` + la firma y el fundido | 108 |

En la 1.47B el `Sistema` está en la PSRAM. En el broche suman ~15,6 KB de RAM
interna (del 68 % a ~73 %, a confirmar con `pio run -e broche-c3` en CI); si una
cara no entra, sus partes se achican por igual hasta entrar (con 12 KB, tres
partes de ~45 × 45 que se ven de ~55 px).

### La IA y la web

- MCP `dibujar_cara` (ver [nube.md](nube.md#herramientas)): cada parte en SVG,
  URL o base64 → sprites a la misma escala → la expresión; devuelve la cara
  abierta y parpadeando dibujada por el núcleo. La guía (`sintetica://guia/caras`)
  explica cómo dibujar para la pantalla, con un ejemplo de ojitos de anime.
- Web: Laboratorio → Expresiones → **De imagen** (subir o pegar el SVG de cada
  parte, perillas, la cara viva en el núcleo entero y "Siempre") y, en la
  consola, el módulo Cara elige la cara de siempre.

## Habla, despertador, clima y la noche (v1.6)

### El globo (`globo.cpp`, `Sistema::decir`)

Lo que dice el bicho (el comando Decir, sus frases, el despertador) va en un
**globo** claro, con la cara achicada arriba (el mismo panel del recordatorio:
fundido de 400 ms, nunca un corte) y la colita hacia la boca. El texto (hasta
60 bytes) se ajusta como el del recordatorio: Fraunces en 2 líneas (3 en la
redonda, al ancho del círculo) y, si no entra, mono mayúscula. Dura 2,5 s +
60 ms por byte (de 3 a 10 s) o hasta un toque (que no hace nada más). Un
recordatorio y el despertador le ganan; uno nuevo espera que el anterior se
vaya. La expresión visible del estado es `globo`.

### Que hable solo (`habla.cpp`, `shared/formato/vida/frases.json`)

- **Las frases son datos**: `frases.json` tiene 20 situaciones (`despertar`,
  `buenas-noches`, `volviste`, `te-extrane`, `aburrido`, `sueno`,
  `bateria-baja`, `cargando`, `caricia`, `saludo`, `hito`, `cumple-dueno`,
  `cumple-robot`, `recordatorio-hecho`, `despertador`, `posponer`,
  `clima-frio`, `clima-calor`, `clima-lluvia`, `clima-lindo`), cada una con
  `probabilidad`, `enfriamiento` (s), `siempre` y frases para `todos` y por
  preset; `{dueno}` y `{robot}` se reemplazan (sin dueño, "¡Hola, {dueno}!" →
  "¡Hola!"). `embeber_caras.py` las pasa a una **tabla** en `vida_base.h`
  (flash, sin parsear: no ocupan RAM).
- **Quién las pide**: los datos. La acción `decir:<situación>` en las reglas y
  el campo `"decir"` en los pasos: los comportamientos de fábrica la usan
  (`despertar` → despertar, `contar-ovejas` → buenas-noches, `bostezar` →
  sueno, `llamar-la-atencion` → aburrido, `saludar-al-volver` → volviste,
  `volver-ofendido` → te-extrane, `cansancio` → bateria-baja, `alivio` →
  cargando, `caricia` → caricia, `saludar` → saludo). Los hitos piden `hito`, o
  `cumple-dueno`/`cumple-robot` si es el cumple; cerrar un recordatorio con un
  toque, `recordatorio-hecho`; el clima en modo clima, `clima-*`.
- **Quién decide** (`Hablador`): la probabilidad de la situación × la
  frecuencia (0,5 = ×1), su enfriamiento, 15 min entre frases (÷ frecuencia) y
  10 por día (× frecuencia). Las `siempre` (cumple, despertador, posponer) no
  cuentan ni dependen de la frecuencia. Si ya hay algo en pantalla, la
  espontánea no espera: no se dice.
- **Elegir**: si el usuario tiene propias para la situación, solo esas; si no,
  las de `todos` (peso 1) y las de su preset (peso 2), con el preset más
  parecido a sus rasgos (`presetCercano`).

### El cumple

Al primer contacto del día del cumple (`dueno.cumple` o `cumple`, el hito de
la memoria, una vez por año) festeja (comportamiento `festejar`, estrellas y
corazones) y dice "¡Feliz cumple, {dueno}!" o "¡Hoy cumplo años!". La nube,
además, le manda un Telegram al dueño (docs/nube.md).

### El despertador (`Sistema::revisarDespertador`, `avanzarAlarma`)

A su hora (hasta 1 min tarde, una vez por día, en sus días): si duerme, lo
despierta con la rutina `despertar` (sin mal humor); si no, la empieza. El globo
dice el `texto` o una frase de `despertador`, más el clima si hay ("¡Buen día,
Nahuel! 11° y despejado"), y abajo cinco puntitos que laten con el volumen.
Suena `despertador:1` cada 4 s y sube un nivel cada 10 s hasta `despertador:5`
(el perfil elige el volumen de cada nivel; `voz.volumen` 0 lo silencia). Un
toque (o el botón del broche) **pospone** `posponer` minutos (y dice una frase
de `posponer`); mantenerlo 1,5 s, levantarlo o una sacudida fuerte lo
**apagan**; a los 10 min se apaga solo. Evento `33 despertador` (al empezar a
sonar a su hora), con aviso a la nube. El broche programa el despertador del
sueño profundo también para la alarma y para la pospuesta. La expresión
visible es `despertador`.

### El modo clima (`clima.cpp`)

Con `modo: clima`, el panel muestra el ícono (sol o luna, algo nublado,
nublado, niebla, llovizna, lluvia, nieve, tormenta; dibujados con el lienzo y
animados: las gotas caen, el sol gira, la niebla se corre), la temperatura en
Fraunces grande y `MÁX · MÍN` en mono. Sin datos (o con más de 3 h), "--°". Un
toque muestra la cara 15 s. La cara reacciona al llegar el dato y cada 40 s:
tormenta → `miedo`; menos de 8° → `frio` (expresión nueva: tiembla); lluvia →
`triste`; nieve → `emocionado`; niebla → `confundido`; nublado → `pensando`;
más de 30° → `cansado`; sol de día → `contento`. La expresión visible es
`clima`.

### Una noche que se ve (`MotorVida::vidaDormido`)

- **zzz** que suben cada 3,5 a 6,5 s (soñando, cada 8 a 14).
- **Sueños** cada 10 a 25 min (`sonar`, o `pesadilla` con estrés): REM (la
  mirada salta y los párpados tiemblan), burbujas de pensamiento con algo
  adentro (corazón, estrella, nota u oveja; en la pesadilla, un "!").
- **Postura**: cada 2 a 6 min gira de lado o se acomoda (por los resortes).
- **Sonrisas y murmullos** en sueños cada tanto (más seguido soñando, más
  sonrisas si te quiere), con `sonido("murmullo")`.
- **Ronquido suave**: `sonido("ronquido")` cada 25 a 60 s (no mientras sueña).
- **Lo que lo despierta de noche**: tocarlo, una sacudida fuerte o una caída;
  su **nombre** lo despierta suave (sin mal humor). La **voz** (6 %) y el
  **ruido** (3 %) casi nunca, y cada vez menos (se divide por 1 + 0,5 por cada
  uno de esa noche); si no, la mitad de las veces se mueve o murmura en sueños.
  La música y el silencio, nunca.

## Acompañar el día (v1.7, `acompanar.cpp` y `MotorVida::rutinaConVos`)

Un bicho que es parte del día y no un accesorio. Las frases son de
`frases.json` (situaciones nuevas: `buen-dia`, `comiste`, `a-dormir`,
`tardaste`, `agua`, `pausa`, `como-estas`, `animo-bien|medio|mal`, `jugar`,
`gane`, `perdi`, `empate`, `anio-nuevo`, `navidad`, `dia-del-amigo`,
`dia-del-nino`, `aniversario`) y se pueden reemplazar con `habla.propias`.

- **El día con vos**: la primera interacción del día (de 2 h antes de
  despertarse a las 14) es un "buen día" con el clima (`{clima}`: "11° y
  despejado"); si llueve o hace frío, el consejo ("Llevá paraguas"). A la tarde
  (13 a 15:30), "¿comiste?" (una vez). En los 45 min antes de dormir, bosteza y
  te dice "Ya es tarde, ¿nos vamos a dormir?". Aprende a qué hora aparecés
  (`Memoria::horas`), te espera (queda en `hoyEspero`) y, si volvés entre 1,5 y
  6 h más tarde que lo de siempre, "¡Tardaste!".
- **Cuidarte** (`cuidado`): agua cada `agua` horas mientras está despierto;
  pausa activa (se estira y te lo dice) si pasan `pausa` minutos sin tocarlo en
  el horario de `trabajo`. "¿Cómo estás?" una vez por día, cuando te vio hace
  menos de 2 min (no en los primeros 10 min después de prenderse): toque =
  bien, doble toque = más o menos, mantener 0,8 s = mal (o el comando `0x45`,
  que la nube manda desde los botones de Telegram). Queda en la memoria
  (`hoyAnimo`), en el registro (tipo 6) y avisa el evento `35 animo`; responde
  con empatía (mal: cariñoso, con corazones, y una frase linda).
- **Juegos** (`0x43` Jugar, la consola y `/jugar` de Telegram): piedra, papel o
  tijera (las tres van pasando; tu toque frena tu elección y él elige al
  azar), reflejos ("tocá en el verde": antes, perdiste; después, los ms; ganás
  con menos de 450 ms; el récord queda en `mejorReflejo`), seguí la mirada
  (cinco rondas: mira a un lado y tenés 2,5 s para inclinarlo hacia ahí; ganás
  con 4), dado y moneda (sacudilo o tocalo). Los toques son del juego; si nadie
  juega 30 s, se termina. Cada partida: `juegosJugados`, `juegosGanados`,
  `hoyJuegos`, registro tipo 5 y evento `34 juego`, y él dice "¡Gané!",
  "¡Ganaste!" o "¡Empate!".
- **Fechas** (hitos, una vez por año, con festejo y frase): año nuevo (1/1),
  Navidad (25/12), el día del amigo (20/7), el día del niño en la Argentina
  (tercer domingo de agosto) y el aniversario (el día del primer encuentro).
- **Te extraña**: si no lo ves en un día, se va poniendo triste (hasta la mitad
  del ánimo de fondo a las 72 h); cuando volvés después de más de 24 h,
  festejo grande (`reflejo:sorpresa` + `festejar` + "Te extrañé", en
  `reglas.json`); después de 3 días, primero se ofende. La nube te manda "te
  extraño" por Telegram (docs/nube.md).
- **Avisos del celu** (`avisos` y `0x44`): según la app y el contacto, una
  cara, un comportamiento, un sonido y un globo ("{contacto} te escribe").
- **Entre robots** y **su día contado** (el resumen de la noche) los hace la
  nube con lo que guarda la memoria (docs/nube.md).
- **Voz** (M7): contrato en protocolo-ble.md ("Voz"); `0x46` Mirar ya lo
  simula el núcleo.

RAM: `Cuidado` 8 B, `Avisos` 769 B (8 avisos), el juego y la pregunta ~70 B.

## Compañía (v1.9, `compania.cpp`)

Lo que llega de afuera (los amigos, el calendario, el celu, Spotify) y los
hábitos. Las frases son de `frases.json` (situaciones `toque-recibido`,
`toque-devuelto`, `agenda`, `agenda-manana`, `te-vas`, `habito-hecho`,
`habito-racha`, `habito-cortado`, `musica`, `pago`, `mail`, `github`,
`pedido`, `aviso`, con variantes por preset) y las plantillas nuevas `{de}`,
`{titulo}`, `{minutos}`, `{cantidad}`, `{racha}`, `{habito}` y `{artista}`
(`Plantillas` en `completarFrase`/`elegirFrase`). Una frase que usa una
plantilla que vino vacía no se elige ("Un mail de {de}." sin de quién). Son
pedidos (de la nube o del dueño): no pasan por el Hablador (`decirFrase`),
salvo la de la música.

- **Toques** (`0x47`): lo despierta suave, una cara según el tipo (toque:
  contento; abrazo: `verguenza`, que se achica y se pone colorado; beso:
  `enamorado`, con corazones; carita: la expresión que vino), el sonido
  (`toque`, `abrazo`, `beso`) y el globo con `{de}`. Durante 30 s, un toque (o
  una caricia, aunque llegue como evento) **lo devuelve**: evento `40
  devolver-toque` y el aviso `0x9E` con la entrada de tipo 7 (el tipo de
  toque), que la nube le manda al que lo mandó. Registro: evento 39 + tipo 7.
- **Agenda** (`0x48`): de 0 a 15 min, el comportamiento `apurado` (mira para
  los costados, parpadea rápido, la partícula `reloj` y el sonido `agenda`),
  evento `41 agenda` y el globo (`{titulo}` en `{minutos}`). Con más de 15 min,
  nada. `minutos = 0xFFFF` es el resumen de mañana (`agenda-manana`, con
  `{cantidad}` = la duración del marco).
- **Presencia** (`0x49`): te fuiste → evento 43 y, si `presencia.preguntar`, el
  globo de `presencia.texto` (o una frase de `te-vas`); 15 min después se
  aburre y le da sueño (`MotorVida::empujarEstado`). Llegaste → evento 42 y el
  "¡volviste!" de verdad (o "te extrañé" después de 20 h), con la cara según
  cuánto tardaste (un ratito: guiño; horas: emocionado; un día: enamorado).
- **Hábitos** (`habitos`, `0x4A`): a la hora del hábito (y sus días; si estaba
  ocupado, hasta 10 min después) pregunta "¿{habito}?" con el sonido `habito`;
  el globo se queda mientras espera (1 min). Un toque = hecho; mantener 0,8 s =
  todavía no (pregunta una vez más en 1 h). Hecho: comportamiento `orgulloso`
  (cara orgullosa, estrellas, sonido `habito`), evento `44 habito`, registro
  tipo 8 (el índice) y, si fue en el robot, el aviso `0x9E`. Con racha de 3 o
  más, `habito-racha`; si la racha se cortó (ayer no), la pregunta es la de
  `habito-cortado`. La racha la manda la nube en el `0x4A`; marcado en el
  robot, cuenta "ayer + 1".
- **Música** (`0x4B`): mueve la cabeza al bpm (100 si no se sabe): de costado
  cada dos pulsos y un saltito en cada uno, con notas. Es una capa del motor
  (`MotorVida::musica`) con un peso que entra y sale en ~0,7 s, así que
  nunca corta; dormido no baila. Al cambiar de tema, `bailar-ritmo` y quizás
  un globo (`musica`). Sin el bit de sonando (o sin noticias en 20 min), deja
  de bailar.
- **Avisos de afuera** (`0x44` tipos 2 a 6): el evento `notificacion` (y su
  regla), el sonido (`pago` o `notificacion`; con un pago, además, emocionado)
  y el globo de su situación con `{titulo}` y `{de}`.

RAM: `Habitos` 98 B y `Presencia` 42 B en `Config`; el estado en `Sistema`
~60 B; el Hablador suma 14 situaciones (28 B). Los comportamientos de base
pasaron a 58 (`MAX_COMPORTAMIENTOS` 67: el usuario sigue con 9 propios).

### Sonidos (los nombres que recibe `Hal::sonido`)

`despertador:1` a `despertador:5`, `ronquido`, `murmullo` (v1.6), `dado`,
`notificacion`, `llamada` (v1.7), `toque`, `abrazo`, `beso`, `agenda`, `pago`,
`habito` (v1.9), más los de
siempre (`recordatorio`, `pomodoro` y los `sonido` de los pasos). Todos pasan
por `Sistema::sonar` (`voz.volumen` 0 = silencio).

### RAM

`Config` suma `Habla` (746 B) y `Despertador` (70 B); `Sistema`, el globo, el
texto del despertador, `DatosClima` (40 B) y el `Hablador`: unos 2,3 KB en
total. El broche quedó en 74 % de RAM (con las caras de imagen).

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
- El modo `notificaciones` (necesita que la nube mande los datos).
- La app todavía no muestra el diario ni tiene los editores de expresiones y
  comportamientos (están en la web); sí la personalidad, los presets y el horario.
- iOS sigue sin compilarse en este entorno (no hay Xcode): revisar en la Mac.
