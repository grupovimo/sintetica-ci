# SintéticaOS

El sistema de los bichos de Sintética Labs: la capa que hace que la cara nunca
sea una imagen estática y que cada dueño edite de punta a punta.

**El motor de vida v2** ([docs/motor-de-vida.md](docs/motor-de-vida.md)): una
sola cara viva, por parámetros y resortes (el "Motor de caras v0" portado al
núcleo), con estado interno, personalidad, comportamientos, ritmo diario,
sueños y memoria de la relación.

Hasta ahora (hitos M0 a M6 de `docs/especificacion.md`, más el motor de vida): el formato compartido
de caras, animaciones y reglas; el núcleo del firmware con WiFi, MQTT y OTA
firmada; un simulador que corre el mismo núcleo en el navegador; las apps
nativas; el broche; y la nube: una API en Go con cuentas por link mágico,
el broker MQTT adentro, Postgres, la web con los editores y bichos virtuales,
todo en Docker.

**Para levantarlo en tu Mac: [docs/local.md](docs/local.md)** (`docker compose up --build`).

| | |
| --- | --- |
| `shared/formato/` | JSON Schema de la configuración (v2), las 16 caras base (dibujos), las referencias del manual y, en `vida/`, las expresiones, comportamientos, reglas y personalidades del motor de vida |
| `firmware/nucleo/` | El núcleo en C++17 autocontenido: el rostro paramétrico (`rostro.*`), el motor de vida (`motor_vida.*`), los dibujos (`cara.*`), gestos, reglas, protocolo |
| `firmware/perfiles/dev-147b/` | El perfil para la Waveshare ESP32-S3-LCD-1.47B (PlatformIO + LovyanGFX + NimBLE) |
| `firmware/perfiles/broche-c3/` | El broche: ESP32-C3 SuperMini + GC9A01 redonda; ver su README |
| `firmware/perfiles/comun/` | Lo que comparten las placas ESP32 (BLE, almacenamiento, taller) |
| `firmware/perfiles/sim/` | El perfil del simulador (WebAssembly) |
| `firmware/api/` | El núcleo detrás de una API en C, para las apps; `cgo/`, el mismo para Go |
| `api/` | La API en Go (cuentas, bichos, MQTT, sincronización, OTA) y el bicho virtual; ver `docs/nube.md` |
| `web/` | La web (SvelteKit): Mis bichos y los editores de caras, animaciones y reglas |
| `docker-compose.yml` | Todo junto en local: API (con el broker MQTT), web, Postgres, Mailpit y dos bichos virtuales |
| `shared/protocolo/` | Vectores de prueba del protocolo BLE, compartidos por todas las implementaciones |
| `android/` | App de Android (Kotlin + Compose + NDK) |
| `ios/` | App de iOS (SwiftUI + CoreBluetooth); ver `ios/README.md` |
| `firmware/test/` | Tests en la PC y tests de oro contra el manual de marca |
| `sim/` | La página del simulador y su test de punta a punta |
| `docs/` | La especificación, el motor de vida, el protocolo, la nube y cómo levantarlo en local; `docs/referencia/`, el motor de caras v0; `docs/empresa/`, papeles |

## Requisitos

- Node 22, Python 3 con Pillow, NumPy y OpenCV (`opencv-python-headless`, para los
  tests de oro y del QR), CMake, clang y
  wasm-ld (para el simulador; no hace falta Emscripten).
- PlatformIO para el firmware: `pip install platformio`.
- Playwright con Chromium solo para regenerar las referencias de oro.
- Android: JDK 17+, el SDK con `platforms;android-35`, `ndk;27.2.12479018` y
  `cmake;3.22.1` (`sdk.dir` en `android/local.properties`).
- iOS: una Mac con Xcode 15 o más nuevo y XcodeGen.
- La nube: Docker (Docker Desktop en la Mac). Fuera de Docker, Go 1.24 (con un
  compilador de C++ para cgo) y Postgres 16.

## Comandos

```sh
npm install
npm test               # formato + núcleo + oro + simulador
npm run sim            # simulador en http://localhost:8147
npm run empaquetar     # sim/dist/simulador.html: un solo archivo, con el wasm adentro
npm run caras          # regenerar las caras desde las medidas del manual
npm run claves         # el par de claves de firma de las OTA (en claves/, fuera del repo)
docker compose up --build                         # la nube en local (ver docs/local.md)
go test ./api/... ./firmware/api/cgo/             # la API (los de la base: SINTETICA_PRUEBAS_DB=postgres://…)
go test -tags e2e ./api/e2e/                      # de punta a punta, contra el compose levantado
cd web && npm install && npm run check            # la web
cd firmware && pio run -e dev-147b                 # compilar el firmware
cd firmware && pio run -e dev-147b -t upload -t monitor   # grabarlo en la placa
cd firmware && pio run -e broche-c3 -t upload -t monitor  # el broche
cd android && ./gradlew testDebugUnitTest assembleDebug   # app de Android (APK en app/build/outputs)
cd ios && xcodegen && open Sintetica.xcodeproj            # app de iOS
```

## Probar en la placa

1. Conectar la 1.47B por USB-C y correr `pio run -e dev-147b -t upload -t monitor`.
2. La cara neutral tiene que parpadear cada 3 a 7 s. El botón BOOT hace de
   sensor táctil: un toque lo pone contento, dos lo ponen en sospecha.
3. Sacudirlo lo marea (si el monitor dice "no encontré el QMI8658", revisar los
   pines del IMU en `firmware/perfiles/dev-147b/pines.h`).
4. La primera vez muestra un QR. Con la app (o el simulador) se vincula; con
   nRF Connect, seguir los pasos de `docs/protocolo-ble.md`.
5. Por el monitor serie, `serie 001` graba el número de unidad.

Si la imagen sale corrida o con colores raros, los primeros sospechosos son
`LCD_DESFASE_X` y `invert` en el perfil. Todo lo que falta verificar en el
hardware está listado al final de `docs/especificacion.md`.

Todos los derechos reservados. Ver `LICENSE`.
