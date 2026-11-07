# SintéticaOS

El sistema de los bichos de Sintética Labs: la capa que hace que la cara nunca
sea una imagen estática y que cada dueño edite de punta a punta.

Esta es la **etapa 1**: el formato compartido de caras, el núcleo del firmware
(hitos M0, M1 y M2 de `docs/especificacion.md`) y un simulador que corre el
mismo núcleo en el navegador.

| | |
| --- | --- |
| `shared/formato/` | JSON Schema de la configuración, las 16 caras base y las referencias del manual |
| `firmware/nucleo/` | El núcleo en C++17 autocontenido: render, motor de vida, gestos, reglas, protocolo BLE |
| `firmware/perfiles/dev-147b/` | El perfil para la Waveshare ESP32-S3-LCD-1.47B (PlatformIO + LovyanGFX + NimBLE) |
| `firmware/perfiles/sim/` | El perfil del simulador (WebAssembly) |
| `firmware/test/` | Tests en la PC y tests de oro contra el manual de marca |
| `sim/` | La página del simulador y su test de punta a punta |
| `docs/` | La especificación y el protocolo BLE |

## Requisitos

- Node 22, Python 3 con Pillow, NumPy y OpenCV (`opencv-python-headless`, para los
  tests de oro y del QR), CMake, clang y
  wasm-ld (para el simulador; no hace falta Emscripten).
- PlatformIO para el firmware: `pip install platformio`.
- Playwright con Chromium solo para regenerar las referencias de oro.

## Comandos

```sh
npm install
npm test               # formato + núcleo + oro + simulador
npm run sim            # simulador en http://localhost:8147
npm run empaquetar     # sim/dist/simulador.html: un solo archivo, con el wasm adentro
npm run caras          # regenerar las caras desde las medidas del manual
cd firmware && pio run -e dev-147b                 # compilar el firmware
cd firmware && pio run -e dev-147b -t upload -t monitor   # grabarlo en la placa
```

## Probar en la placa

1. Conectar la 1.47B por USB-C y correr `pio run -e dev-147b -t upload -t monitor`.
2. La cara neutral tiene que parpadear cada 3 a 7 s. El botón BOOT hace de
   sensor táctil: un toque lo pone contento, dos lo ponen en sospecha.
3. Sacudirlo lo marea (si el monitor dice "no encontré el QMI8658", revisar los
   pines del IMU en `firmware/perfiles/dev-147b/pines.h`).
4. Con nRF Connect, seguir los pasos de `docs/protocolo-ble.md`.

Si la imagen sale corrida o con colores raros, los primeros sospechosos son
`LCD_DESFASE_X` y `invert` en el perfil. Todo lo que falta verificar en el
hardware está listado al final de `docs/especificacion.md`.

Todos los derechos reservados. Ver `LICENSE`.
