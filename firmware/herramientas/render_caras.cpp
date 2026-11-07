// Dibuja las 16 caras base en una pantalla de 320 × 172 y las guarda como PPM.
// Lo usan los tests de oro (test/oro/comparar.py) y sirve para mirar a ojo.
//
// Uso: render_caras <carpeta> [cierre]
#include <cstdio>
#include <cstdlib>
#include <string>

#include "cara.h"
#include "formato.h"

using namespace sintetica;

static Config g_config;
static uint16_t g_pixeles[320 * 172];

int main(int argc, char** argv) {
  if (argc < 2) {
    std::fprintf(stderr, "uso: %s <carpeta> [cierre]\n", argv[0]);
    return 2;
  }
  const std::string carpeta = argv[1];
  ResultadoConfig r = cargarBase(g_config);
  if (!r.ok()) {
    std::fprintf(stderr, "no cargó la base: %s\n", r.detalle);
    return 1;
  }
  Lienzo lienzo;
  lienzo.iniciar(g_pixeles, 320, 172);
  const Vista vista = Vista::para(320, 172);
  EstadoCara estado;
  if (argc > 2) estado.cierre = static_cast<float>(std::atof(argv[2]));
  for (int i = 0; i < g_config.nExpresiones; i++) {
    const Expresion& e = g_config.expresiones[i];
    lienzo.limpiar(paleta::PANTALLA);
    dibujarExpresion(lienzo, vista, e, estado);
    char nombre[64];
    std::snprintf(nombre, sizeof nombre, "%02d-%s.ppm", i + 1, e.nombre);
    FILE* f = std::fopen((carpeta + "/" + nombre).c_str(), "wb");
    if (!f) return 1;
    std::fprintf(f, "P6\n320 172\n255\n");
    for (int p = 0; p < 320 * 172; p++) {
      uint32_t c = de565(g_pixeles[p]);
      unsigned char rgb[3] = {static_cast<unsigned char>(c >> 16), static_cast<unsigned char>(c >> 8),
                              static_cast<unsigned char>(c)};
      std::fwrite(rgb, 1, 3, f);
    }
    std::fclose(f);
  }
  std::printf("%d caras en %s\n", g_config.nExpresiones, carpeta.c_str());
  return 0;
}
