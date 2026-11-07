// Dibuja las 16 caras base en una pantalla de 320 × 172 y las guarda como PPM.
// Lo usan los tests de oro (test/oro/comparar.py) y sirve para mirar a ojo.
//
// Uso: render_caras <carpeta> [cierre]
//      render_caras --redonda <carpeta>   (240 × 240, como el broche)
#include <cstdio>
#include <cstdlib>
#include <string>

#include "cara.h"
#include "emparejamiento.h"
#include "formato.h"

using namespace sintetica;

static Config g_config;
static uint16_t g_pixeles[320 * 240];

int main(int argc, char** argv) {
  if (argc < 2) {
    std::fprintf(stderr, "uso: %s <carpeta> [cierre]\n", argv[0]);
    return 2;
  }
  const bool redonda = std::string(argv[1]) == "--redonda";
  if (redonda && argc < 3) return 2;
  const std::string carpeta = argv[redonda ? 2 : 1];
  const int W = redonda ? 240 : 320, H = redonda ? 240 : 172;
  ResultadoConfig r = cargarBase(g_config);
  if (!r.ok()) {
    std::fprintf(stderr, "no cargó la base: %s\n", r.detalle);
    return 1;
  }
  Lienzo lienzo;
  lienzo.iniciar(g_pixeles, W, H);
  const Vista vista = Vista::para(W, H, redonda);
  EstadoCara estado;
  if (!redonda && argc > 2) estado.cierre = static_cast<float>(std::atof(argv[2]));
  for (int i = 0; i < g_config.nExpresiones; i++) {
    const Expresion& e = g_config.expresiones[i];
    lienzo.limpiar(paleta::PANTALLA);
    dibujarExpresion(lienzo, vista, e, estado);
    char nombre[64];
    std::snprintf(nombre, sizeof nombre, "%02d-%s.ppm", i + 1, e.nombre);
    FILE* f = std::fopen((carpeta + "/" + nombre).c_str(), "wb");
    if (!f) return 1;
    std::fprintf(f, "P6\n%d %d\n255\n", W, H);
    for (int p = 0; p < W * H; p++) {
      uint32_t c = de565(g_pixeles[p]);
      unsigned char rgb[3] = {static_cast<unsigned char>(c >> 16), static_cast<unsigned char>(c >> 8),
                              static_cast<unsigned char>(c)};
      std::fwrite(rgb, 1, 3, f);
    }
    std::fclose(f);
  }
  // La pantalla de emparejamiento, con el número de serie del manual.
  dibujarEmparejamiento(lienzo, "001", "SINTETICA:V1/A1B2C3D4E5F6/K7Q2XM", "ESPERANDO VÍNCULO", redonda);
  FILE* f = std::fopen((carpeta + "/emparejamiento.ppm").c_str(), "wb");
  if (!f) return 1;
  std::fprintf(f, "P6\n%d %d\n255\n", W, H);
  for (int p = 0; p < W * H; p++) {
    uint32_t c = de565(g_pixeles[p]);
    unsigned char rgb[3] = {static_cast<unsigned char>(c >> 16), static_cast<unsigned char>(c >> 8),
                            static_cast<unsigned char>(c)};
    std::fwrite(rgb, 1, 3, f);
  }
  std::fclose(f);
  std::printf("%d caras y la pantalla de emparejamiento en %s\n", g_config.nExpresiones, carpeta.c_str());
  return 0;
}
