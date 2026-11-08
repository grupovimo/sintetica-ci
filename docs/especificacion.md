# SintéticaOS: cómo se construye

> Exportado de la pestaña "SintéticaOS" del doc *Sintética Labs — Producto y
> SintéticaOS* (7/11/2026). La fuente de verdad del producto sigue siendo ese
> doc; acá se agregan, al final, las decisiones de implementación.

Esta es la especificación para construir SintéticaOS con Claude Code: el
firmware, la app, la web y el backend, en hitos con criterio de "listo". La
regla de oro: caras, animaciones y reglas son **datos**, nunca código.

## Reglas para quien implemente

- Un solo formato de cara (JSON) es la fuente de verdad; el bicho, la app y la web lo dibujan igual.
- El firmware se separa en núcleo y **perfiles de hardware** (pines, pantalla, sensores), para correr igual en la 1.47B, el broche y el cubito.
- Cada hito termina con una demo en el hardware real y tests de lo que se pueda testear fuera de él.
- Nada público (repos, web, app) revela qué placas o componentes se usan.

## Stack

| Capa | Tecnología | Notas |
| --- | --- | --- |
| Firmware MVP | C++ con PlatformIO (framework Arduino) + LovyanGFX | Rápido para iterar en la 1.47B |
| Firmware producción | ESP-IDF | OTA con rollback, secure boot y flash encryption |
| App iOS | Swift + SwiftUI + CoreBluetooth | Nativa, nada de Expo |
| App Android | Kotlin + Jetpack Compose | Nativa |
| Web | Svelte + TypeScript | Editor de caras y animaciones en pantalla grande |
| Backend | API en Go (con el broker MQTT embebido, Mochi) + Postgres, en el VPS de Grupo Vimo | Cuentas, configuraciones, OTA firmada |
| IA | Clave de API del usuario + servidor MCP | Nosotros no pagamos el uso |

## Perfiles de hardware

| Perfil | Pantalla | Entradas | Conexión |
| --- | --- | --- | --- |
| `dev-147b` | IPS 172 × 320 (pines según la wiki de Waveshare) | IMU QMI8658, TTP223 | WiFi + BLE |
| `broche-c3` | GC9A01 redonda 240 × 240 (pines en la pestaña Hardware) | Pulsador GPIO5 | Solo BLE |
| `cubito-154` | Táctil 1,54" (pines según la wiki de Waveshare) | Táctil, 2 micrófonos, IMU | WiFi + BLE |

## Formato de configuración

Un bicho guarda un solo documento; la app y la web lo editan y se lo mandan
entero o por partes. El JSON Schema está en `shared/formato/config.schema.json`
y un ejemplo en `shared/formato/config-ejemplo.json`.

Las formas son primitivas vectoriales en coordenadas normalizadas (−1 a 1), así
escalan a cualquier pantalla. Las animaciones son pistas de formas con claves en
el tiempo (posición, escala, giro, color y opacidad) y una curva por tramo.

## Comunicación

- **Vinculación:** el bicho muestra un QR con su id y un código de prueba de posesión; la app lo escanea y se conecta por BLE (wifi_provisioning de Espressif para pasarle el WiFi).
- **BLE (todos):** un servicio GATT propio con características de configuración (escritura por bloques), comandos (cambiar expresión, reproducir), estado (notificaciones: batería, expresión actual) y OTA. Detalle en `docs/protocolo-ble.md`.
- **MQTT (con WiFi):** `bicho/{id}/cmd`, `bicho/{id}/state`, `bicho/{id}/config` y `bicho/{id}/ota`, con TLS y credenciales por dispositivo.
- **En la calle:** la app hace de puente entre el backend y el bicho por BLE.

## Plan de construcción

| Hito | Qué | Listo cuando | Estado |
| --- | --- | --- | --- |
| M0 | Firmware base en la 1.47B: PlatformIO, LovyanGFX, perfil `dev-147b` | La cara neutral parpadea al azar cada 3 a 7 s, fluida y sin parpadeos de pantalla | Listo en simulador; falta probar en la placa |
| M1 | Motor de vida y las 16 expresiones desde el JSON embebido | Mira a los costados, respira, cambia de expresión con transición; sacudirlo lo marea (IMU) | Listo en simulador; falta probar en la placa |
| M2 | BLE: servicio GATT y configuración guardada en flash | Desde nRF Connect se cambia la expresión y el nombre, y sobrevive a un reinicio | Listo en simulador; falta probar en la placa |
| M3 | Apps iOS y Android mínimas | Escanear el QR, vincular, elegir expresión con vista previa idéntica a la del bicho | Android compila y pasa sus tests; iOS escrita, falta compilarla en una Mac; falta probar las dos con la placa |
| M4 | Broche: perfil `broche-c3`, pulsador y sueño profundo | Un toque cambia la animación, mantenerlo lo duerme, otro toque lo despierta; dormido gasta menos de 0,3 mA | Listo en el simulador y compila; falta armarlo y medir el consumo dormido |
| M5 | Backend en el VPS: cuentas, configuraciones, MQTT y OTA firmada | Un bicho con WiFi recibe un cambio hecho desde otro celu y se actualiza solo | Listo en local con Docker y probado de punta a punta con el bicho virtual; falta la placa con WiFi y subirlo al VPS |
| M6 | Web: editor de caras, animaciones y reglas | Una cara creada en la web se ve igual en la app y en el bicho | Listo en local (el mismo núcleo en WebAssembly dibuja la web, las apps y el bicho); falta probarlo con la placa |
| V2 | Motor de vida v2 (pestaña "Motor de vida" del doc; [motor-de-vida.md](motor-de-vida.md)) | Una sola cara viva por parámetros y resortes, con estado interno, personalidad, comportamientos, ritmo diario y memoria; los criterios de aceptación de la spec | Listo en el núcleo con los criterios en tests (24 h simuladas), en el simulador, la API (memoria, mudanza, hora), la web (Vida, Expresiones, Comportamientos) y las apps (personalidad y horario); compila para las dos placas; falta medir fps y batería en la placa |
| M7 | Cubito: voz, IA con clave propia y servidor MCP | Le hablás, responde con voz y cara; el Claude del usuario le cambia la cara por MCP | — |
| M8 | Producción: paso a ESP-IDF, secure boot, flash encryption, rollback | Una OTA fallida vuelve sola a la versión anterior | — |

## Estructura del repo

```
sintetica/
  firmware/        núcleo + perfiles (dev-147b, broche-c3; después cubito-154)
  shared/formato/  JSON Schema de la configuración + caras base
  sim/             el simulador: el núcleo compilado a WebAssembly
  docs/            esta especificación
  api/             la API en Go y el bicho virtual
  web/             la web (SvelteKit)
  ios/, android/   las apps nativas
```

---

## Decisiones de la etapa 1 (base + M0–M2)

Tomadas el 7/11/2026 con Nahuel:

- **Simulador primero.** No hay placa conectada al entorno de desarrollo, así
  que M0–M2 se validan en un simulador: el mismo núcleo C++ compilado a
  WebAssembly. Lo que se ve en el simulador es exactamente lo que corre en la placa.
- **Repo privado**: se usan los nombres reales de placas y pines.
- **Formato en español**, como el ejemplo del doc. Nombre: SintéticaOS.
  Licencia propietaria. Tipografías: Fraunces + JetBrains Mono (como el manual).
- **Las 16 caras salen del manual de marca** (`expresiones-set.svg`): las
  medidas en píxeles están en `shared/formato/herramientas/generar_caras.py` y
  las referencias SVG en `shared/formato/referencia/`.

### Stack de las próximas etapas (decidido el 7/11/2026)

- Apps nativas: Swift + SwiftUI en iOS y Kotlin + Jetpack Compose en Android.
- Web: el frontend en Svelte + TypeScript; el backend en Go.
- Todo se despliega en el VPS de Grupo Vimo.

### Decisiones de la etapa 2 (M3)

- La vista previa de las apps la dibuja **el mismo núcleo C++** (Android por
  JNI, iOS por la API en C de `firmware/api`): no hay renderers paralelos.
- **Emparejamiento**: la pantalla del manual (QR, "UNIDAD #001", "Escaneame /
  con la app."). Aparece mientras ningún celu está vinculado y 2 min después de
  mantener el botón 5 s.
- **Sin el código no se escribe**: vincular da una clave por celu; cada conexión
  se autentica. Detalle en `docs/protocolo-ble.md` (v1.1).
- Número de unidad grabado en el taller por USB (`serie 001`).
- Apps `com.sinteticalabs.app`, iOS 17+ y Android 10+. Pantallas: escanear,
  vincular, el bicho (vista previa, estado, caras, nombre, ojos, personalidad,
  sueño) y "probar sin bicho".

### Decisiones de la etapa 3 (M4)

- **Pulsador del broche:** toque = lo siguiente de la lista; mantener 2 s =
  dormir (pantalla apagada y sueño profundo); un toque lo despierta; mantener
  8 s = QR.
- **Pantalla redonda:** las mismas caras con los ojos 1,5× y los indicadores
  acercados al centro; el emparejamiento lleva el QR centrado.
- **Imágenes** para todos los perfiles: hasta 8, en QOI del tamaño de la
  pantalla, mezcladas con las caras en la lista (`"lista"`, `"rotar"` en la
  config). Mientras se ve una imagen, cada 1 a 3 min vuelve la cara 2,5 s.
- **Render por franjas** para placas sin PSRAM (el C3 dibuja de a 40 filas).
- **Batería del broche:** divisor de 2 × 100 kΩ de B+ a GND, al GPIO0.

### Decisiones de la etapa 4 (M5 y M6)

Tomadas el 7/11/2026 con Nahuel. Todo corre en local con Docker; todavía no
se sube nada al VPS (ver [local.md](local.md) y [nube.md](nube.md)).

- **Cuentas con link mágico** por email (y un código para tipear). En local
  los mails van a Mailpit.
- **Broker MQTT**, que autentica y autoriza contra la API: cada bicho tiene su
  propia clave y solo ve sus tópicos. (Era EMQX; desde el 8/11/2026 es Mochi
  MQTT embebido en la API, ver `docs/nube.md`, "El broker".)
- **El bicho pasa a la cuenta cuando se conecta con la clave que le dio la app
  por BLE** (prueba de posesión). La app también le pasa el WiFi (comandos
  `0x16` y `0x17` del protocolo v1.3).
- **Firmware con WiFi y MQTT** en la 1.47B, y un **bicho virtual** en Docker
  con el mismo núcleo para probar sin placa.
- **OTA completa**: la API firma (ECDSA P-256), el bicho verifica antes de
  bajar e instala con vuelta atrás. El broche la recibe por BLE desde la app.
- **Sincronización de tres vías** de la config y las imágenes: si el bicho
  cambió por BLE, la nube se lo trae; si cambió en la nube, se lo manda.
- **Animaciones con claves**: cada pista es una forma que se mueve, escala,
  gira y cambia de color y opacidad con una curva por tramo. Se usan en la
  lista, en las reglas y con el comando Reproducir.
- **La web** (SvelteKit): Mis bichos en vivo, config, editores de caras,
  animaciones y reglas, imágenes, firmware, historial y una biblioteca de
  caras y animaciones por cuenta. Dibuja con el mismo núcleo en WebAssembly.
- **Las apps** entran a la cuenta, suman bichos, les pasan el WiFi, instalan
  firmware por BLE y manejan por la nube los bichos que están lejos.

### Decisiones de la etapa 5 (motor de vida v2, 7/11/2026)

- La cara del bicho pasa a ser **paramétrica** (el "Motor de caras v0" portado a
  la CPU, `nucleo/rostro.cpp`) y la mueven resortes. Las caras de formas (v1)
  siguen como **dibujos**: las del manual quedan para los tests de oro y como
  base de los dibujos propios; un dibujo propio del usuario se muestra encima
  del rostro con fundido. Las 16 del manual existen también como expresiones
  paramétricas con el mismo nombre (y 21 más).
- Config **v2** (la v1 sigue valiendo). Los datos base del motor (expresiones,
  comportamientos, reglas y personalidades) son JSON en `shared/formato/vida/`,
  embebidos y parseados uno por uno al arrancar.
- El motor no duerme a los 10 min sin interacción (eso era del manual v1): vive
  el horario del dueño (por defecto 8:00 a 23:30), con siestas, sueños y un
  despertar; sin hora, duerme cuando tiene mucho sueño.
- Protocolo **v1.4**: eventos nuevos (8 a 30) y los comandos Hora, Vida,
  Memoria, Restaurar, Registro y Comportamiento.
- La memoria del bicho viaja a la cuenta (la API la pide cada 5 min) y se muda
  de placa con `POST /v1/bichos/{id}/mudar`.
- RAM: en la 1.47B el `Sistema` va en la PSRAM; en el broche se compactaron los
  datos. Las referencias a "16 caras" del manual siguen valiendo para los dibujos.

### Decisiones de la etapa 6 (consola, utilidad e integraciones, 8/11/2026)

- **Producto**: los productos se llaman **Unidad** y **Broche** (más la joyería:
  hebilla, colgante, translúcido, alfiler, hebilla de pelo). "Bicho" queda solo
  en el código; en todo lo que ve la gente es "robot".
- **La consola**: el panel de cada robot es una consola de hardware (teclas
  mecánicas, LEDs, visores, placa naranja). Arriba, la foto real del producto
  con la cara en vivo encastrada; abajo, diez módulos que **guardan solos**
  (600 ms, validados con el núcleo). Lo avanzado vive en el **Laboratorio**.
  La primera vez, un **encendido** de cuatro pasos. Las tres apps (web, iOS,
  Android) se ven y se comportan igual; la web es la referencia.
- **Poco texto**: rótulos de una o dos palabras, íconos y el efecto visible en
  vivo en vez de explicaciones; renglones de ~40 caracteres como mucho.
- **Útil, no solo mascota**: recordatorios (en el robot, con aviso a la nube),
  modo reloj y pomodoro, protector de pantalla (una imagen, GIF o video corto
  que se va con cualquier interacción), voz en la config para los modelos con
  micrófono y parlante. En curso: globo de texto ("decir"), frases propias,
  cumpleaños, despertador, clima y rutinas por hora.
- **Integraciones en la nube**: un bot oficial de Telegram (cada usuario se
  vincula con un link), automatizaciones "cuando pasa X → Telegram / webhook
  firmado / otro robot" y un **servidor MCP con OAuth** (`/mcp`) para que cada
  uno conecte su Claude o su ChatGPT y la IA maneje sus robots (inventar caras
  que valida el núcleo y que vuelven como PNG, imágenes por URL, recordatorios,
  personalidad, reglas…). En curso: **caras de imagen** (sprites que dibuja la
  IA y el motor anima).
- **Tiempo real**: todo lo que cambia en la nube sale por SSE (`/v1/eventos`):
  estado, config, imágenes, automatizaciones, Telegram, MCP y los avisos del
  robot (0x9E). Las apps escuchan el mismo canal.
- **Robustez**: fuzzing con libFuzzer de config, protocolo, QOI y dibujo de
  expresiones (`firmware/fuzz/`, 60 s por blanco en CI).
- **Dónde se compila**: la web con Vite en la Mac; iOS solo en GitHub Actions
  del repo público `grupovimo/sintetica-ios` (publica lo commiteado) y de ahí a
  TestFlight; Android y firmware en el CI de este repo.
- **Producción**: por ahora solo la landing en `sintetica.grupovimo.com.ar`
  (modo `SOLO_LANDING=1`, reservas por Telegram). La nube completa espera el
  token de GHCR y el dominio propio.
- **Una sola placa por producto**: el cubito lleva nuestro firmware; de
  xiaozhi se toma el protocolo de voz y piezas (ver `docs/xiaozhi.md`).
- **El robot como caché, la biblioteca afuera** (decidido el 8/11): sin
  microSD, el cubito guarda en su flash (4–6 MB de datos) solo lo que muestra y
  lo próximo (imágenes, GIFs, videitos, el paquete de voz con las frases más
  usadas) y lo demás lo baja cuando le toca: por wifi de la nube, por wifi
  local del celu o por Bluetooth del celu. La memoria larga vive en la nube y
  en el celu. Se arma con el perfil del cubito.
- **Marketplace de rostros y personalidades** (idea de negocio, 8/11): como
  caras, comportamientos, frases, voces y animaciones son **datos** validados
  por el núcleo, se pueden empaquetar y vender (creadores con reparto, cobro
  por Mercado Pago). La biblioteca de afuera es lo que lo hace posible en los
  robots con poca memoria. Pendiente de diseño. Los paquetes del marketplace
  van **firmados por la nube** y el núcleo solo carga los que verifican.
- **El robot cerrado** (M8, pedido de Nahuel el 8/11): en los robots que se
  venden, solo arranca firmware firmado por nosotros (secure boot v2 de la
  ESP32, con eFuses quemados en fábrica: es irreversible), la flash va cifrada
  (no se puede leer ni copiar) y el JTAG/USB de depuración queda apagado. Las
  OTA ya se verifican con firma. La clave de firma vive fuera del servidor de la
  web, con copia segura: si se pierde, los robots vendidos no se actualizan más.
  Las placas de desarrollo quedan abiertas.

### Diferencias con el ejemplo del doc

- Las formas de una expresión van en `"formas"` (el ejemplo decía `"ojos"`),
  porque además de ojos hay indicadores: zZ, batería, barras de audio, puntos.
- Una forma es un objeto con `"forma"` y sus medidas: `pildora`, `corte`,
  `circulo`, `arco`, `corazon`, `espiral`, `poligono` y `franjas` (glitch).
  `"fijo": true` marca los indicadores, que no siguen la mirada ni el parpadeo.
- Las coordenadas usan una escala uniforme: el centro de la pantalla es (0, 0),
  el ancho va de −1 a 1 y el alto se mide con la misma unidad (en 320 × 172, de
  −0,5375 a 0,5375). Así un círculo sigue siendo un círculo en la pantalla redonda.

### Lo que falta probar en la placa

- Pines de la 1.47B: los de la pantalla salen de la wiki de Waveshare; los del
  IMU (SDA 48, SCL 47, dirección 0x6B) y la batería (GPIO1, divisor ×3) salen de
  un proyecto de la comunidad. Verificar con el esquemático.
- El desfase de columna del ST7789 de 172 px (se usa 34) y la rotación.
- Los ejes del IMU: el núcleo asume que z sale de la pantalla hacia el frente
  (para "boca abajo").
- Mientras no llegue el TTP223, el botón BOOT hace de sensor táctil.
- Consumo y autonomía.
