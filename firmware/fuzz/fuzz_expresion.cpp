// Blanco 4: el dibujo de una expresión paramétrica (sintetica_dibujar_expresion)
// con una config arbitraria: parser de la config + rostro.cpp + lienzo.cpp.
//
// Entrada: [nombre][ancho][alto][forma][t u16][json...]
//  - nombre: < 0x80 = la expresión de fábrica (índice % cantidad); si no, el
//    nombre va adelante del JSON, terminado en 0 (para las caras propias).
//  - ancho y alto: 16 + byte % 113 (hasta 128: más grande solo es más lento).
//  - forma: bit 0, pantalla redonda.
//  - t: segundos × 100.
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "comun.h"
#include "sintetica_c.h"

namespace {
uint8_t g_rgba[128 * 128 * 4];
}

extern "C" int LLVMFuzzerTestOneInput(const uint8_t* datos, size_t n) {
  Entrada e{datos, n};
  const uint8_t sel = e.u8();
  const int ancho = 16 + e.u8() % 113;
  const int alto = 16 + e.u8() % 113;
  const int redonda = e.u8() & 1;
  const float t = e.u16() / 100.0f;

  char nombre[64] = {};
  if (sel < 0x80) {
    const char* nb = nullptr;
    const int cant = sintetica_vida_base(0, 0, nullptr, nullptr);
    if (cant > 0) sintetica_vida_base(0, sel % cant, &nb, nullptr);
    if (nb) strncpy(nombre, nb, sizeof nombre - 1);
  } else {
    size_t k = 0;
    while (!e.vacia()) {
      const uint8_t c = e.u8();
      if (!c) break;
      if (k < sizeof nombre - 1) nombre[k++] = static_cast<char>(c);
    }
  }

  const uint8_t* resto = nullptr;
  const size_t m = e.tomar(e.n, &resto);
  char* json = static_cast<char*>(malloc(m ? m : 1));
  if (m) memcpy(json, resto, m);

  const size_t px = static_cast<size_t>(ancho) * alto * 4;
  memset(g_rgba, 0xA5, sizeof g_rgba);
  char detalle[96];
  const int r = sintetica_dibujar_expresion(m ? json : nullptr, m, nombre, ancho, alto, redonda, t, g_rgba, detalle,
                                            sizeof detalle);
  if (!memchr(detalle, 0, sizeof detalle)) abort();
  if (r == 0) {
    // Dibujó todos los píxeles (opacos) y ni uno más.
    for (size_t i = 3; i < px; i += 4)
      if (g_rgba[i] != 255) abort();
    for (size_t i = px; i < sizeof g_rgba; i++)
      if (g_rgba[i] != 0xA5) abort();
  }
  free(json);
  return 0;
}
