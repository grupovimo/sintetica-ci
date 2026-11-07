# Motor de vida (especificación)

> Exportado de la pestaña "Motor de vida" del doc *Sintética Labs — Producto y
> SintéticaOS* (7/11/2026). La fuente de verdad es ese doc; cómo está
> implementado, en [motor-de-vida.md](motor-de-vida.md).

El bicho es una sola cara viva que nunca se corta: no hay imágenes ni animaciones pregrabadas, solo parámetros que se mueven con física, empujados por un estado interno que cambia con el tiempo y con lo que le pasa. Referencia viva: Motor de caras v0.

## Principios
- Nunca quieto: ningún frame se repite idéntico más de 2 segundos mientras está despierto.
- Nunca un corte: todo cambio visible es una interpolación de parámetros con resortes. Prohibido reemplazar un dibujo por otro.
- Todo es dato: expresiones, comportamientos, reglas y personalidad viven en JSON editable desde la app; el código solo interpreta.
- Causa visible: cada cambio de humor tiene un motivo que el dueño puede entender (lo tocaste, lo ignoraste, es de noche, tiene poca batería).
- Nunca predecible: azar con límites, enfriamientos y anti-repetición; dos bichos con la misma personalidad no hacen lo mismo al mismo tiempo.
- Dentro de la marca: el usuario edita todo, pero los parámetros tienen topes para que siempre se vea Sintética.
- Mismo resultado en todos lados: el bicho, la app y la web usan el mismo motor y deben dar el mismo frame con la misma semilla.

## Arquitectura en capas
Cada capa solo habla con la de abajo; se ejecutan en este orden en cada tick (60 Hz la lógica, el render a lo que dé la pantalla).
- Sensores: tacto, IMU, micrófono, botón, batería, reloj, BLE, WiFi, notificaciones de la app. Depende del perfil de hardware.
- Percepción: convierte señales crudas en eventos con intensidad: `toque`, `caricia`, `sacudida`, `inclinado`, `boca_abajo`, `ruido_fuerte`, `voz`, `silencio_largo`, `notificacion`, `bateria_baja`, `cargando`, `bicho_cerca`, `app_abierta`, etc.
- Estado interno: variables continuas (energía, ánimo, afecto, aburrimiento, curiosidad, sueño, estrés) que los eventos empujan y el tiempo hace decaer.
- Decisión: elige qué comportamiento ejecutar según el estado, la personalidad, el contexto y el azar (puntaje de utilidad, ver abajo).
- Comportamiento: una secuencia corta de objetivos de expresión, mirada, partículas y sonido, con duración y posibilidad de interrupción.
- Micro-vida: capa siempre activa que suma parpadeo, mirada, respiración y temblores encima de cualquier comportamiento.
- Mezclador de parámetros: combina expresión base + capas aditivas + micro-vida, y lo pasa por resortes.
- Render: dibuja la cara por funciones de distancia (SDF) con la misma matemática en C++, Swift, Kotlin y GLSL/TS.

## Modelo de cara
La cara son dos ojos, una boca opcional y una capa de partículas, en coordenadas normalizadas (−1 a 1, el lado corto de la pantalla). Cada ojo tiene sus propios parámetros, así puede ser asimétrica; los ángulos se espejan solos en el ojo derecho.

### Parámetros por ojo
| Parámetro | Rango | Qué hace |
| --- | --- | --- |
| `w`, `h` | 0,03 a 0,2 / 0,03 a 0,32 | Medio ancho y medio alto |
| `r` | 0 a 1 | Redondez de las esquinas (1 = píldora) |
| `tilt` | −0,6 a 0,6 rad | Inclinación del ojo |
| `lt`, `lta` | 0 a 0,5 / −0,8 a 0,8 | Párpado de arriba: cuánto tapa y su ángulo (positivo = baja hacia adentro, enojo) |
| `lb`, `lba` | 0 a 0,5 / −0,8 a 0,8 | Párpado de abajo: cuánto tapa y su ángulo |
| `smile` | 0 a 1 | Recorte de abajo que lo vuelve arco feliz (∩) |
| `heart` | 0 a 1 | Mezcla hacia forma de corazón |
| `star`, `cross`, `ring` | 0 a 1 | Mezclas hacia estrella (emoción), cruz (KO) y anillo (cargando) |
| `pupil` | 0 a 1 | Pupila oscura dentro del ojo, para miradas intensas |
| `squash` | 0,7 a 1,3 | Aplastamiento propio del ojo (rebotes) |
Cuando `lt + lb` llega a 1 el ojo se dibuja como una línea; si además tiene `smile`, la línea se curva (^). Así un ojo cerrado sigue siendo el mismo ojo.

### Parámetros globales
| Parámetro | Qué hace |
| --- | --- |
| `gx`, `gy` | Mirada: corre los dos ojos juntos |
| `ey`, `sep` | Altura de la cara y separación entre ojos |
| `ftilt`, `fscale` | Inclinación y escala de toda la cara |
| `spin` | Remolino que retuerce los ojos (mareo) |
| `orbit` | Los ojos giran alrededor del centro (cargando, pensando fuerte) |
| `pulse` | Los ojos laten al ritmo del sonido (escuchando, hablando) |
| `beat` | Latido doble tipo corazón |
| `glitch` | Cortes horizontales y separación de color |
| `dim`, `hue` | Brillo y corrimiento de color (de naranja a rojo) |
| `mo`, `mw`, `mcurve` | Boca: apertura, ancho y curva (sonrisa o puchero) |
| `blush` | Cachetes encendidos bajo los ojos |
| `shake` | Temblor rápido (frío, miedo, risa) |

### Partículas
Pequeñas formas que nacen, flotan y se desvanecen, también como SDF: `zzz` (durmiendo), `corazoncitos`, `lágrima`, `gota de sudor`, `chispas`, `signo ?`, `signo !`, `notas musicales`, `estrellitas` (mareado), `burbuja de pensamiento`. Cada una tiene emisor, ritmo, física simple (subir, caer, oscilar) y vida. Nunca hay más de 6 en pantalla.

### Biblioteca de expresiones
Una expresión es solo un destino de parámetros más, opcionalmente, partículas y una capa de movimiento propia.
| Expresión | Claves | Capa de movimiento |
| --- | --- | --- |
| neutral | Píldoras | Solo micro-vida |
| contento | `smile` 0,85 | Rebote suave |
| emocionado | `star` 0,8, `fscale` 1,08 | Saltitos, chispas |
| risa | `smile` 1, ojos cerrados en ^ | `shake` rítmico, boca abierta |
| sospecha | `lt` 0,42, `lb` 0,22, mirada al costado | Mirada lenta de lado a lado |
| sorpresa | Ojos redondos y grandes, boca "o" | Salto hacia arriba, signo ! |
| miedo | Ojos chicos, `pupil` 1, `shake` | Se encoge (`fscale` 0,9) |
| dormido | Cerrados | Respiración lenta, zzz |
| bostezo | `lt` alto, boca abierta grande | Una sola vez, 2 s |
| enojado | `lt` 0,36 con `lta` 0,55 | Temblor corto, `hue` rojo |
| gruñón | Enojado leve, mirada hacia abajo | Resoplido |
| triste | `lt` con `lta` negativo, mirada abajo | Lágrima ocasional |
| vergüenza | Mirada al costado y abajo, `blush` 1 | Se achica un poco |
| guiño | Un ojo cerrado en ^ | Inclinación pícara |
| enamorado | `heart` 1 | `beat`, corazoncitos |
| mareado | `spin` 1, ojos desparejos | Bamboleo, estrellitas |
| escuchando | Ojos altos | `pulse` con el volumen |
| hablando | Neutral | Boca y `pulse` con la voz |
| pensando | Mirada arriba al costado, un ojo entrecerrado | Burbuja, `orbit` leve |
| confundido | Ojos desparejos, `tilt` opuesto | Signo ? |
| curioso | Un ojo grande, cabeza inclinada | Se acerca (`fscale` 1,1) |
| aburrido | `lt` 0,3, mirada perdida | Suspiros |
| orgulloso | `lb` 0,2, mirada arriba | — |
| concentrado | `lt` 0,2, `lb` 0,15, `pupil` | Quieto, parpadea menos |
| batería baja | Chicos, `dim` 0,55, `hue` | Cansancio, cabeceo |
| cargando | Contento leve | `ring` en un ojo que se llena |
| actualizando | Puntos | `orbit` 1 |
| sin conexión | Neutral, `cross` leve | Busca con la mirada |
| KO | `cross` 1 | Se cae de costado (`ftilt`) |
| glitch | `glitch` 1 | Saltos de color |
Las 16 originales siguen existiendo; las nuevas se suman y el usuario puede crear las suyas en el editor.

## Transiciones
Cada parámetro es un resorte amortiguado (masa, rigidez `k`, fricción `c`); la expresión solo mueve el objetivo. Así toda transición anticipa, se pasa un poquito y se asienta.
| Grupo | `k` | `c` | Sensación |
| --- | --- | --- | --- |
| Forma de ojos | 170 | 17 | Elástica, con leve rebote |
| Párpados | 260 | 24 | Rápidos, sin rebote |
| Mirada (saccade) | 380 | 28 | Salto seco |
| Mirada (seguimiento) | 90 | 18 | Suave |
| Cara entera (escala, inclinación) | 120 | 11 | Gelatinosa |
| Color y brillo | 60 | 15 | Lenta |
| Parpadeo | 900 | 40 | Instantáneo |
- Anticipación: todo cambio de expresión le da un impulso negativo a `fscale` vertical (se aplasta y rebota). La fuerza depende de la intensidad del cambio y de la energía del bicho.
- Escalonado: los párpados llegan antes que la forma, y la mirada antes que todo (primero mira, después siente).
- Interrupciones: un comportamiento nuevo nunca resetea los resortes; cambia los objetivos desde donde están. Prioridades: reflejo > reacción > comportamiento > ánimo de fondo.
- Capas aditivas: el ánimo de fondo pone una base (por ejemplo, un 30 % de triste), el comportamiento se suma encima y la micro-vida encima de todo. El mezclador hace `base + Σ peso·delta` y lo limita a los rangos de la marca.
- Velocidad según personalidad y energía: un bicho cansado tiene resortes más blandos; uno eléctrico, más rígidos.

## Micro-vida
Capa siempre activa, también durante cualquier expresión; es lo que hace que nunca parezca una imagen.
| Elemento | Comportamiento |
| --- | --- |
| Parpadeo | Cada 2 a 6 s con distribución log-normal; 15 % de las veces es doble. Cerrar dura unos 70 ms, abrir unos 120 ms. Más frecuente cuando está cansado o nervioso; menos cuando está concentrado o sorprendido. Siempre parpadea al terminar un movimiento grande de mirada. |
| Saccades | Cada 0,7 a 3 s la mirada salta a un punto nuevo, cerca del centro, con preferencia por donde vino el último estímulo. |
| Micro-saccades | Temblores de 0,005 a 0,01 cada 150 a 400 ms entre saccades. |
| Seguimiento | Si hay algo que seguir (dedo en la pantalla táctil, el celu inclinado, la dirección del sonido en el cubito) la mirada lo persigue suave. |
| Respiración | Escala vertical de ±1,2 % a unas 16 respiraciones por minuto; dormido, ±3,5 % y más lento; nervioso, más rápido. |
| Deriva | La cara flota ±0,012 con ruido suave, nunca en un ciclo exacto. |
| Ajustes de postura | Cada 20 a 60 s un pequeño cambio de inclinación o altura, como quien se acomoda. |
| Ruido de forma | Variaciones de ±2 % en `w` y `h` con ruido muy lento, para que ningún frame sea idéntico. |
Todo usa ruido determinista con semilla por bicho: dos bichos iguales nunca parpadean al mismo tiempo, y el mismo bicho con la misma semilla se puede reproducir en tests.

## Estado interno
Siete variables de 0 a 1 que nunca se ven directamente, pero deciden todo. Los eventos las empujan, el tiempo las devuelve a su punto de reposo (que depende de la personalidad) y se guardan en flash cada 5 minutos para sobrevivir a un reinicio.
| Variable | Sube con | Baja con | Vuelve al reposo en |
| --- | --- | --- | --- |
| Energía | Dormir, cargar, mañana | Actividad, horas despierto, batería baja | Sigue el ritmo diario |
| Ánimo | Caricias, que le hablen, juegos, el nombre | Ignorarlo, sacudidas fuertes, ruidos, batería baja | unas 2 h |
| Afecto | Atención repetida a lo largo de los días | Abandono de varios días (muy lento) | Casi no decae: es la relación |
| Aburrimiento | Tiempo sin estímulos | Cualquier interacción o novedad | Se resetea al interactuar |
| Curiosidad | Estímulos nuevos o raros, movimiento cerca | Repetición del mismo estímulo | unos 10 min |
| Sueño | Horas despierto, noche, poca luz | Dormir | Ritmo diario |
| Estrés | Sacudidas, caídas, ruidos fuertes, boca abajo | Calma, caricias | unos 5 min |
De estas variables sale el ánimo de fondo: una mezcla continua de expresiones que se aplica siempre (por ejemplo, poca energía + mucho sueño = 40 % de cansado en todo lo que hace). El estado también modula la micro-vida: estrés alto = parpadeo y respiración más rápidos; ánimo bajo = mirada más baja.

## Personalidad
Seis rasgos de 0 a 1 que el dueño elige en la app (o un preset) y que cambian puntos de reposo, velocidades y gustos.
| Rasgo | Alto | Bajo |
| --- | --- | --- |
| Energía | Se mueve mucho, resortes rápidos, duerme poco | Tranquilo, lento, siestero |
| Sociabilidad | Busca atención, se aburre rápido solo | Independiente |
| Picardía | Guiños, bromas, finge dormir | Serio y predecible |
| Timidez | Se asusta y se avergüenza fácil | Valiente |
| Curiosidad | Investiga todo, se distrae | Se concentra |
| Mal genio | Se enoja rápido y se le pasa lento | Paciente |
Presets: tímido, pícaro, dormilón, cariñoso, curioso, gruñón y eléctrico. Además, con el tiempo la personalidad deriva un 10 % como máximo según cómo lo traten: un bicho muy acariciado se vuelve más cariñoso.

## Cómo elige qué hacer
Un selector por utilidad, evaluado cada 250 ms y cada vez que llega un evento:
- Cada comportamiento disponible calcula un puntaje: `necesidad del estado × afinidad de la personalidad × contexto (hora, batería, perfil de hardware) − penalización por enfriamiento − penalización por repetición + ruido`.
- Se elige por sorteo ponderado entre los 3 mejores, no siempre el primero.
- Un comportamiento en curso solo se interrumpe si el nuevo lo supera por un margen o si es un reflejo.
- Los reflejos (sacudida, caída, toque) saltean el selector y responden en menos de 50 ms.
- Anti-repetición: ningún comportamiento ocioso se repite dentro de sus últimos 5, y cada uno tiene su enfriamiento propio.

## Comportamientos autónomos
Lo que hace solo, sin que nadie lo toque. Cada uno es una secuencia de datos con duración, enfriamiento y condiciones.
| Comportamiento | Cuándo | Qué hace | Enfriamiento |
| --- | --- | --- | --- |
| Mirar alrededor | Curiosidad media | Recorre la pantalla con saccades largos, se detiene en un punto | 30 s |
| Investigar | Curiosidad alta | Mira fijo a un lado, se acerca, inclina la cabeza, signo ? | 2 min |
| Bostezar | Sueño > 0,5 | Bostezo completo, parpadeo lento después | 3 min |
| Cabecear | Sueño > 0,75 | Se le cierran los ojos, se despierta de golpe, mira si alguien vio | 2 min |
| Suspirar | Aburrimiento > 0,6 | Respiración grande, mirada abajo | 2 min |
| Llamar la atención | Sociabilidad alta y aburrido | Mira al frente, pestañea rápido, saltito, sonido | 5 min |
| Ofenderse | Lo ignoraron mucho tiempo | Se da vuelta (ojos al borde), mira de reojo, vuelve si lo tocás | 10 min |
| Tararear | Ánimo alto | Ojos felices, balanceo, notas musicales | 4 min |
| Estornudar | Azar, muy raro | Anticipación de 1 s, estornudo con `shake`, cara de sorprendido | 30 min |
| Hipo | Azar, muy raro | 3 a 6 saltitos irregulares | 1 h |
| Soñar despierto | Tranquilo y sin estímulos | Mirada perdida arriba, burbuja de pensamiento | 5 min |
| Practicar caras | Picardía alta | Prueba expresiones rápido, como frente a un espejo | 15 min |
| Fingir dormir | Picardía alta | Cierra los ojos, espia con uno, guiño al ser descubierto | 20 min |
| Glitch espontáneo | Azar, raro | 300 ms de glitch, después cara de "¿qué fue eso?" | 10 min |
| Acomodarse | Siempre, cada tanto | Cambio de postura y parpadeo | 30 s |
| Contar ovejas | Antes de dormir | Ojos siguiendo algo que salta, se le cierran de a poco | 1 vez por noche |
| Saludar al volver | Primera interacción tras una ausencia | Sorpresa, alegría proporcional a cuánto te extrañó, corazoncitos si el afecto es alto | Cada ausencia |

## Reacciones a estímulos
Disponibles según el perfil: el broche tiene solo el botón y el reloj; la 1.47B suma IMU y tacto; el cubito suma pantalla táctil, micrófonos y parlante.
| Estímulo | Reacción base | Varía con |
| --- | --- | --- |
| Toque corto | Mira hacia el toque, parpadea, contento leve | Si estaba dormido: sorpresa; tímido: vergüenza |
| Caricia (toque sostenido o deslizar) | Ojos felices que se cierran de a poco, ronroneo con vibración | Afecto alto: corazoncitos; mal genio y enojado: se calma |
| Muchos toques seguidos | Primero se ríe, después se molesta | Mal genio |
| Sacudida suave | Bamboleo y risa | Energía |
| Sacudida fuerte | Mareado con estrellitas, después gruñón | Timidez: miedo |
| Inclinarlo | Los ojos "caen" hacia abajo con la gravedad | — |
| Boca abajo | Miedo, ¡! y pide ayuda | Si sigue, se resigna y se duerme |
| Caída libre | Reflejo: sorpresa máxima, después KO 2 s | — |
| Levantarlo | Mira para todos lados, curioso | — |
| Voz | Escuchando, mira hacia el sonido | Si dicen su nombre: alegría y mira al frente |
| Ruido fuerte | Susto y parpadeo rápido | Timidez |
| Música | Se mueve al ritmo detectado | Ánimo |
| Silencio largo | Aburrimiento sube, empieza a dormitar | Sociabilidad |
| Notificación | Signo !, mira al dueño, muestra el ícono de la app | Modo no molestar |
| Llamada entrante | Sorpresa insistente al ritmo del tono | — |
| Batería < 20 % | Cansado, bostezos | Con < 5 %: KO lento y se apaga "durmiéndose" |
| Enchufarlo | Alivio, ojo con anillo que se llena | Si estaba en KO: se despierta de a poco |
| Carga completa | Contento y chispas | — |
| Sin WiFi | Busca con la mirada, cruz leve | — |
| App abierta | Mira al frente, saluda | — |
| Cambio de cara desde la app | Se mira en el "espejo" y prueba la cara nueva | — |
| Otro bicho cerca (BLE) | Lo busca con la mirada, sorpresa, saludo; si se conocen, alegría | Sociabilidad y timidez |
Los dos bichos que se encuentran pueden sincronizarse: uno inicia una "conversación" de miradas y guiños por BLE y el otro responde.

## Ritmo diario y sueño
El bicho vive en el horario del dueño (configurable en la app, por defecto 8:00 a 23:30).
| Momento | Qué pasa |
| --- | --- |
| Despertar | Rutina de 10 a 20 s: un ojo, el otro, bostezo, estirarse (escala), mirar alrededor |
| Mañana | Energía alta, más curiosidad |
| Después de comer | Bajón: más bostezos, siesta corta si nadie lo toca |
| Tarde | Estado normal |
| Noche | Sueño sube, brillo baja solo, resortes más blandos |
| Ir a dormir | Contar ovejas, se le cierran los ojos de a poco |
| Dormido | Respiración lenta, zzz cada tanto, brillo mínimo |
| Sueños | Cada 20 a 40 min, una fase de 30 s: ojos que se mueven bajo los párpados, una sonrisa o un puchero, hablar dormido |
| Despertar forzado | Si lo tocan dormido: sobresalto y gruñón un rato, salvo afecto alto |
En el broche, dormir de noche usa el sueño profundo del chip: antes de dormir hace su rutina y se despierta solo a la mañana o con el botón.

## Memoria y relación
El bicho recuerda, y la app lo muestra como un diario.
- El dueño: su nombre, el horario en que suele aparecer (aprende la rutina y lo espera: a esa hora mira más al frente).
- La relación: días juntos, caricias totales, racha de días seguidos, afecto acumulado.
- Lo que le gusta: qué estímulos le subieron más el ánimo; los busca o los pide.
- Hitos: primer día, 7 días, 30 días, 100 caricias, su cumpleaños (fecha de armado) y el del dueño, con una animación especial.
- Rencores chiquitos: si lo sacudieron fuerte, la próxima vez que lo levantan duda un instante. Se olvida en un día.
- Ausencias: cuanto más tiempo sin verte, más grande el saludo, hasta un tope; después de varios días, primero se hace el ofendido.
Todo se guarda en el bicho y se sincroniza con la cuenta cuando hay conexión; si cambia de placa, el bicho "se muda" con su memoria.

## Configuración
Todo lo anterior es un documento JSON validado por un JSON Schema compartido. Ejemplo resumido:
```
{
  "version": 2,
  "semilla": 48213,
  "nombre": "Toto",
  "personalidad": { "energia": 0.6, "sociabilidad": 0.8, "picardia": 0.7, "timidez": 0.3, "curiosidad": 0.6, "malGenio": 0.2 },
  "horario": { "despierta": "08:00", "duerme": "23:30" },
  "expresiones": {
    "contento": { "ojo": { "w": 0.13, "h": 0.2, "smile": 0.85 }, "gy": 0.04, "movimiento": "rebote" }
  },
  "comportamientos": {
    "bostezar": {
      "cuando": { "sueno": ">0.5" },
      "pasos": [
        { "expresion": "bostezo", "dura": 2.0 },
        { "expresion": "neutral", "parpadeo": "lento" }
      ],
      "enfriamiento": 180
    }
  },
  "reglas": [
    { "si": "sacudida.fuerte", "entonces": ["reflejo:mareado", "estado.estres+0.4", "sonido:boing"] },
    { "si": "nombre_escuchado", "entonces": ["comportamiento:saludar", "estado.animo+0.2"] }
  ],
  "limites_marca": { "color": "#FF6A33", "brillo_max": 0.9 }
}
```

La app ofrece tres niveles de edición: fácil (preset y sliders de personalidad), creativo (editor visual de caras y comportamientos con vista previa en vivo) y avanzado (el JSON directo y reglas "si pasa X, hacé Y").

## Rendimiento en el ESP32
- Dibujar solo lo que cambia: la cara ocupa poco de la pantalla; se calcula la caja de cada ojo, la boca y las partículas, y solo ahí se evalúa la SDF. El resto es color de fondo.
- SDF solo en el borde: dentro de la forma, relleno sólido; afuera, fondo; la SDF fina solo en la franja del antialias, recorriendo por filas.
- Doble buffer en PSRAM con DMA: mientras la pantalla recibe un frame, el segundo núcleo dibuja el siguiente. Lógica en el núcleo 0, render en el 1.
- Metas: 30 fps en la 1.47B y el cubito, 20 fps en el broche C3; la lógica de vida usa menos del 5 % de CPU.
- Modos de energía: despierto (fps completos), tranquilo (15 fps si nada cambia mucho), dormido (5 fps y brillo mínimo), sueño profundo (broche, apagado hasta el botón o la hora).
- Matemática liviana: floats de precisión simple (el S3 tiene FPU), tablas para seno y coseno, sin `sqrt` donde alcance el cuadrado.

## Herramientas para desarrollar
- Simulador web: el mismo motor en TypeScript, con controles para disparar eventos, ver el estado interno en barras y acelerar el tiempo (un día en 2 minutos).
- Superposición de depuración en el bicho (desde la app): comportamiento actual, puntajes del selector, estado interno y fps.
- Tests de igualdad visual: frames de referencia con semilla fija; C++, Swift, Kotlin y TS tienen que dar la misma imagen con una diferencia menor al 1 %.
- Tests de vida: simular 24 h aceleradas y medir que se cumplan los criterios de abajo.
- Registro de eventos con marca de tiempo, exportable desde la app para reportar bugs.

## Criterios de aceptación
- Despierto, ningún frame se repite idéntico más de 2 s.
- Entre dos expresiones cualesquiera no hay saltos: la diferencia entre frames consecutivos nunca supera un umbral fijo.
- Parpadea entre 10 y 20 veces por minuto despierto, con 10 a 20 % de dobles.
- En una hora sin tocarlo hace al menos 8 comportamientos distintos y ninguno se repite seguido.
- Ignorado 2 h se ve aburrido; ignorado 3 días, se hace el ofendido al volver.
- Los reflejos responden en menos de 50 ms.
- Dos bichos con la misma personalidad y distinta semilla no hacen lo mismo al mismo tiempo.
- Todo lo configurable se puede cambiar desde la app sin regrabar el firmware.
- La cara se ve igual en el bicho, la app y la web con la misma semilla.
- El broche dura al menos 6 h animado y se duerme solo de noche.
