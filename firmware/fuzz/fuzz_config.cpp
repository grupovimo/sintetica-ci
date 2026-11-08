// Blanco 1: la config v1/v2 (JSON → Config): formato.cpp + formato_vida.cpp +
// json.cpp. La entrada es el JSON tal cual, sin terminador.
//
// La API valida con sintetica_validar_config → cargarConfig = cargarBase +
// aplicarSobre. cargarBase (leer todo lo de fábrica del motor de vida) es lo
// que más tarda y no depende de la entrada: acá se carga una vez y cada entrada
// se aplica sobre una copia (aplicarConfig, que hace eso mismo). Una de cada 16
// entradas va además por la API entera, y las dos tienen que dar lo mismo.
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "formato.h"
#include "sintetica_c.h"

using namespace sintetica;

namespace {
Config g_base, g_trabajo;  // grandes: fuera de la pila
bool g_listo = false;
}  // namespace

extern "C" int LLVMFuzzerTestOneInput(const uint8_t* d, size_t n) {
  if (!g_listo) {
    if (!cargarBase(g_base).ok()) abort();
    g_listo = true;
  }
  // Una copia del tamaño justo: si el parser lee un byte de más, ASan lo ve.
  char* json = static_cast<char*>(malloc(n ? n : 1));
  if (n) memcpy(json, d, n);

  g_trabajo = g_base;
  const ResultadoConfig r = aplicarConfig(g_trabajo, json, n);
  if (r.detalle && strlen(r.detalle) > 255) abort();  // y que esté terminado

  uint32_t h = 2166136261u;
  for (size_t i = 0; i < n; i++) h = (h ^ d[i]) * 16777619u;
  if ((h & 15) == 0) {
    char detalle[128];
    const int e = sintetica_validar_config(json, n, detalle, sizeof detalle);
    if (!memchr(detalle, 0, sizeof detalle)) abort();
    if (e != static_cast<int>(r.error)) abort();  // la API y este atajo no coinciden
  }
  free(json);
  return 0;
}
