# SintéticaOS — reglas para trabajar en este repo

La especificación completa está en `docs/especificacion.md`. Lo esencial:

- **Caras, animaciones y reglas son datos, nunca código.** Viven en
  `shared/formato/` (JSON Schema + caras base). El firmware las embebe con
  `firmware/herramientas/embeber_caras.py`; no se escriben a mano en C++.
- **Un solo formato de cara**: el bicho, la app y la web lo dibujan igual. El
  renderer de referencia es `firmware/nucleo/cara.cpp`; los tests de oro lo
  comparan contra el manual de marca (`firmware/test/oro`).
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
- `npm run caras`: regenerar las caras desde las medidas del manual y embeberlas.
- `npm run sim`: compilar el simulador y servirlo en http://localhost:8147.
- `cd firmware && pio run -e dev-147b`: firmware para la Waveshare 1.47B.
