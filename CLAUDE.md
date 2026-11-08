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
- La nube: `docker compose up --build` (ver `docs/local.md`; la web corre con
  Vite en :5173, así que en la Mac `docker compose up -d --build --wait api
  bicho-virtual bicho-broche`); `SINTETICA_PRUEBAS_DB=postgres://sintetica:sintetica@localhost:5433/pruebas
  go test -p 1 ./api/... ./firmware/api/cgo/`; `go test -tags e2e -count=1 ./api/e2e/`
  (con `ADMIN_EMAILS=@ejemplo.com` en el compose para la parte de la OTA).
- La web: `cd web && npm run check && npm run build`.
- Android: `cd android && ./gradlew testDebugUnitTest`.
- iOS a TestFlight: `sh herramientas/publicar-ios.sh` (publica HEAD en `grupovimo/sintetica-ios`; compila Actions).
- La landing a producción: `sh herramientas/desplegar-landing.sh` (HEAD → sintetica.grupovimo.com.ar).

## Memoria del proyecto (estado al 8/11/2026)

Rama de trabajo: `claude/pensive-tesla-8fe2fg` (sin PR).

- **Hecho antes**: M0–M6 y el motor de vida v2 (ver `docs/motor-de-vida.md`).
- **Etapa 6 (8/11)**:
  - **Web**: la "consola" (`web/src/lib/consola/`, rutas `/robots`; `/bichos` redirige): Mis robots (dock con la foto real y la cara en vivo), la consola de cada robot con 10 módulos que guardan solos, el encendido (onboarding) y el Laboratorio (expresiones, comportamientos, reglas, vida, dibujos, animaciones, firmware, historial) en el mismo lenguaje. Tiempo real por SSE (`/v1/eventos`). Poco texto: ver la memoria `poco-texto`.
  - **Núcleo**: recordatorios (se muestran a la hora; aviso 0x9E por BLE y nube), modo reloj y pomodoro, protector de pantalla, voz (solo config). Eventos 31 `recordatorio` y 32 `pomodoro`.
  - **Nube**: bot oficial de Telegram (@sinteticalabs_bot, token en `.env`, nunca en el repo), automatizaciones (cuando X → Telegram / webhook firmado / otro robot), y un **servidor MCP con OAuth** en `/mcp` para que cada uno conecte su Claude o ChatGPT (17 herramientas; ver `docs/nube.md`).
  - **iOS**: compila y sube a TestFlight desde `grupovimo/sintetica-ios` (`herramientas/publicar-ios.sh`); nunca en la Mac.
  - **Broker MQTT embebido** (reemplaza a EMQX): Mochi MQTT adentro de la API (`api/internal/broker`), auth y ACL por ganchos con la lógica de `servidor/mqtt.go`, tcp :1883 (local) y TLS :8883 (producción, el certificado se relee solo). Ver `docs/nube.md`, "El broker", y en `produccion/README.md` los pasos para pasar el VPS.
- **Sin verificar**: placas reales; claude.ai/ChatGPT conectados de verdad (necesitan la nube con HTTPS público).
- **Lo próximo posible**: caras de imagen (sprites que el motor anima), apps iOS y Android con la consola, subir la nube al VPS de Grupo Vimo.

## Trampas conocidas

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
