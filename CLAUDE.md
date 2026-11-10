# SintéticaOS — reglas para trabajar en este repo

La especificación completa está en `docs/especificacion.md`; el motor de vida,
en `docs/motor-de-vida.md` (cómo está hecho) y
`docs/motor-de-vida-especificacion.md` (lo que pide el doc). Lo esencial:

- **Caras, comportamientos, reglas y personalidades son datos, nunca código.**
  Viven en `shared/formato/` (JSON Schema v2, los dibujos base en `caras/` y el
  motor de vida en `vida/`). El firmware los embebe con
  `firmware/herramientas/embeber_caras.py` (`caras_base.h`, `vida_base.h`); no
  se escriben a mano en C++.
- **Una sola cara viva, la misma en todos lados**: el rostro paramétrico
  (`firmware/nucleo/rostro.cpp`, el port del "Motor de caras v0" de
  `docs/referencia/`) dibujado por el mismo núcleo en el bicho, el simulador,
  la web (WebAssembly) y las apps (NDK / C). Los dibujos de formas
  (`nucleo/cara.cpp`) siguen para las caras propias del usuario y los tests de
  oro contra el manual (`firmware/test/oro`).
- **Nunca un corte**: todo cambio visible pasa por los resortes del motor
  (`nucleo/motor_vida.cpp`) o por un fundido.
- **Núcleo y perfiles de hardware separados.** `firmware/nucleo/` es C++17
  autocontenido: sin STL, sin memoria dinámica, sin libc (compila a
  WebAssembly sin Emscripten). Todo lo que toca hardware va en
  `firmware/perfiles/<perfil>/` y entra al núcleo por `nucleo/hal.h`.
- Cada hito termina con una demo (simulador o placa) y tests de lo que se pueda
  probar fuera del hardware.
- Nada público (web, app, repos públicos) revela qué placas o componentes se
  usan. Este repo es privado.
- Código, comentarios, claves JSON y textos en español rioplatense.

## Comandos

- `npm test`: formato + núcleo (tests y oro) + simulador.
- `npm run caras`: regenerar las caras desde las medidas del manual y embeber
  caras y datos del motor de vida (correrlo después de tocar `shared/formato/vida/`).
- `npm run sim`: compilar el simulador y servirlo en http://localhost:8147.
- `cd firmware && pio run -e dev-147b` (y `-e broche-c3`): el firmware.
- `./build/nucleo/pruebas "vida:"`: solo los tests que tengan ese texto (después
  de `npm run test:nucleo` o `cmake --build build/nucleo`).
- `node shared/protocolo/generar_vectores.mjs`: regenerar los vectores del
  protocolo después de tocar `sim/web/protocolo.js`.
- `node shared/formato/paridad/generar.mjs`: regenerar el set de paridad de la
  config (casos borde y límites que corren núcleo, cgo, wasm, web, Kotlin y
  Swift) después de tocar un límite o un detalle de error del núcleo.
- La nube: `docker compose up --build` (ver `docs/local.md`; la web corre con
  Vite en :5173, así que en la Mac `docker compose up -d --build --wait api
  bicho-virtual bicho-broche`); `SINTETICA_PRUEBAS_DB=postgres://sintetica:sintetica@localhost:5433/pruebas
  go test -p 1 ./api/... ./firmware/api/cgo/`; `go test -tags e2e -count=1 ./api/e2e/`
  (con `ADMIN_EMAILS=@ejemplo.com` en el compose para la parte de la OTA).
- La web: `cd web && npm run check && npm run build`.
- Android: `cd android && ./gradlew testDebugUnitTest`.
- iOS a TestFlight: `sh herramientas/publicar-ios.sh` (publica HEAD en `grupovimo/sintetica-ios`; compila Actions).
- La landing a producción: `sh herramientas/desplegar-landing.sh` (HEAD → sintetica.grupovimo.com.ar).
- La nube completa a producción: `sh herramientas/desplegar-nube.sh` (HEAD → api-sintetica, sintetica y mqtt-sintetica; desde el 8/11 reemplaza a la landing sola). El bot de Telegram corre solo en producción (long polling): el `.env` local va sin `TELEGRAM_TOKEN`.

## Memoria del proyecto (estado al 10/11/2026)

Rama de trabajo: `claude/pensive-tesla-8fe2fg` (sin PR). El detalle de cada
decisión está en `docs/especificacion.md` ("Decisiones de la etapa 6"), y lo
pendiente, en `docs/pendientes.md`.

- **Hecho antes**: M0–M6 y el motor de vida v2 (ver `docs/motor-de-vida.md`).
- **Etapa 6 (8 y 9/11)**:
  - **Web**: la "consola" (`web/src/lib/consola/`, rutas `/robots`; `/bichos` redirige) con sus módulos que guardan solos, el encendido y el Laboratorio. Tiempo real por SSE (`/v1/eventos`). Poco texto: ver la memoria `poco-texto`. La web es la referencia de las apps.
  - **Núcleo**: recordatorios, reloj, pomodoro, protector, el paquete de vida (v1.6/v1.7: decir, despertador, clima, juegos, cuidado, avisos del celu), la compañía (v1.9: toques de amigos, agenda, presencia, hábitos, música, avisos de afuera), el **motor de charla sin IA** (`charla.cpp`, datos en `shared/formato/charla/`, 297 casos reales en `casos.json`), la **voz propia** (balbuceo, `voz_propia.cpp`; modo 2 = solo sin texto) y el **motor de vida v3** (único por semilla, etapas, recuerdos y diario, juega solo, empatía, rutinas; datos en `shared/formato/vida/caracter.json`). `cara.tamano` ya agranda la cara.
  - **Nube**: Telegram (@sinteticalabs_bot; también se ENTRA con Telegram), automatizaciones, MCP con OAuth (`/mcp`), amigos y toques, agenda iCal, Spotify (sin claves, apagado), presencia y horarios, hábitos, avisos entrantes (GitHub, Mercado Pago), la **charla** (primero el motor propio por cgo; si no entiende, la IA: la del usuario con su clave, `/v1/ia`, o la del servidor, Anthropic o compatible con OpenAI por `CHARLA_URL`), la **voz** (ElevenLabs con la clave de cada usuario, conversación en tiempo real; Edge TTS gratis; paquete de voz sin internet). Broker MQTT embebido (Mochi) en la API.
  - **Producción** (desde el 8/11, 20:30 UTC): la nube completa en el VPS de Grupo Vimo (`sh herramientas/desplegar-nube.sh`): https://sintetica.grupovimo.com.ar, https://api-sintetica.grupovimo.com.ar, mqtts://mqtt-sintetica.grupovimo.com.ar:8883. Sin SMTP: se entra con Telegram. Desplegado hasta `abb1fd4` (10/11: v3 del motor de vida, /admin, preventa, modo galería). Sin usuarios todavía. **Desplegar y publicar iOS sin preguntar** (memoria `desplegar-sin-preguntar`).
  - **La primera placa real** (10/11): la Waveshare 1.47B de Nahuel (`dev-147b`, id `441BF686559D`) anda con SintéticaOS, vinculada al iPhone, con WiFi y en su cuenta de producción. Se graba desde la Mac (venv `~/.venvs/sintetica-firmware`; detalle en la memoria `placa-147b`). Arreglados: la pila de 8 KB (temporales de `Config` y `DatosVida`: ahora `EnSuLugar`, `formato.h`), el QMI8658 (reinicio por software), el espacio libre antes de subir imágenes (`Hal::espacioLibre`). **Modo galería** (`"modo":"galeria"`, `"galeria":{"segundos"}`) en núcleo, web e iOS; subidas de imágenes más rápidas (16 bits y bloques sin respuesta en iOS).
  - **Control del dueño** (10/11): https://sintetica.grupovimo.com.ar/admin (unidades, componentes, compras, ventas, preventa; dólar oficial de DolarApi). **Preventa del cubito** con seña por Mercado Pago: apagada hasta cargar `MP_ACCESS_TOKEN` en `/opt/sintetica/.env` (ver `docs/pendientes.md`).
  - **iOS**: el CI compila en `macos-15-intel` (la cola arm64 de GitHub no daba máquina el 10/11); la build con galería (`c13beb2`) estaba compilando al cerrar. Antes, build 15 en TestFlight (`herramientas/publicar-ios.sh`, compila en `grupovimo/sintetica-ios`), apunta a producción. **Android**: APK en el release `android-debug` de este repo.
  - **El robot-celu** (Moto E4, `android-bicho/`, de Nahuel): anda contra la nube LOCAL; se pasa a producción con `python3 android-bicho/herramientas/sumar.py`. Quedó en silencio y sin recordatorios ni avisos (config v73) la noche del 8/11: volver a prenderlos desde el historial.
  - **CI**: el repo `nawesan12/sintetica` se quedó sin minutos de Actions (pago o límite). Hay un espejo `grupovimo/sintetica-ci` (creado privado) y `herramientas/publicar-ci.sh`; hacerlo público y empujar lo tiene que correr Nahuel (el permiso de Claude lo bloquea). La v3 del motor de vida y la robustez de la nube **no pasaron CI**: falta medir la RAM del broche (`pio run -e broche-c3`).
- **Sin verificar**: placas reales salvo la 1.47B (el broche y el cubito, no); claude.ai/ChatGPT conectados de verdad; ElevenLabs, Spotify, GitHub y Mercado Pago reales; las apps en un celu real.

## Trampas conocidas

- `docker build` desde Claude se cuelga en "load metadata" (el ayudante de
  credenciales `desktop` de Docker en segundo plano): desplegar con
  `DOCKER_CONFIG` apuntando a una config sin `credsStore` (solo
  `{"currentContext":"desktop-linux"}` y un enlace a `~/.docker/contexts`).

- **Archivos nuevos del núcleo**: además de `firmware/nucleo/`, hay que crear un
  enlace en `firmware/api/cgo/` (`ln -s ../../nucleo/x.cpp x.cpp`); el test
  `TestEnlaces` avisa. cgo no ve los cambios de headers de afuera del paquete.
- `Config` es grande (~110 KB): en la 1.47B el `Sistema` vive en la PSRAM
  (`perfiles/dev-147b/main.cpp`); en el broche no hay PSRAM y la RAM está al
  68 %: cualquier tabla nueva en `Config` hay que medirla con `pio run -e broche-c3`.
- Los límites de la config (cantidad de expresiones, comportamientos, reglas)
  tienen que ser iguales en todos lados: valida el mismo núcleo en el bicho, la
  API (cgo) y la web (wasm). No usar `#ifdef ARDUINO` para límites que cambien
  qué config se acepta (solo para buffers internos, como la máscara de trazos).
- El motor corre la lógica a 60 Hz fijos y "salta" si entre ticks pasan más de
  2 s: los tests que simulan días avanzan de a 10 s.
- Los nombres del JSON son ASCII (`grunon`, `guino`, `bateria-baja`); los
  textos para el usuario, con tildes.
- El broker vive y muere con la API: cada reinicio de la API reconecta a todos
  los bichos (en ~5 s). Mochi tiene una carrera interna entre un retenido nuevo
  y una suscripción con comodines (la ve `-race`): los bichos no usan comodines.
- `.sandbox/` (excluido por `.git/info/exclude`) tiene Dockerfiles con el
  certificado del proxy de la sesión en la nube; en la Mac no hace falta.
