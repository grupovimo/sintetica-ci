// M4: render por franjas, pantalla redonda, el pulsador del broche, dormir y la lista.
#include <cstring>
#include <map>
#include <string>
#include <vector>

#include "cara.h"
#include "emparejamiento.h"
#include "prueba.h"
#include "sistema.h"

using namespace sintetica;

namespace {
uint32_t g_t = 0;
std::map<std::string, std::vector<uint8_t>> g_mem;
std::vector<uint16_t> g_pantalla;  // lo que "muestra" la pantalla, armado franja por franja
int g_franjas = 0;
Config g_cfg;
uint16_t g_franja[240 * 40];

Hal halBroche() {
  Hal h;
  h.ahoraMs = [] { return g_t; };
  h.semilla = [] { return 7u; };
  h.identificador = [] { return "C3C3C3C3C3C3"; };
  h.guardar = [](const char* k, const uint8_t* d, size_t n) {
    g_mem[k] = std::vector<uint8_t>(d, d + n);
    return true;
  };
  h.leer = [](const char* k, uint8_t* d, size_t cap) {
    auto it = g_mem.find(k);
    if (it == g_mem.end() || it->second.size() > cap) return -1;
    std::memcpy(d, it->second.data(), it->second.size());
    return static_cast<int>(it->second.size());
  };
  h.presentarFranja = [](const uint16_t* px, int ancho, int y0, int filas) {
    g_franjas++;
    std::memcpy(&g_pantalla[y0 * ancho], px, sizeof(uint16_t) * ancho * filas);
  };
  return h;
}

Perfil perfilBroche() {
  Perfil p;
  p.ancho = 240, p.alto = 240, p.filasFranja = 40, p.redonda = true, p.boton = Perfil::Boton::Broche;
  return p;
}

Sistema g_s;
void arrancarBroche(bool borrar = true) {
  if (borrar) g_mem.clear();
  g_t = 1000;
  g_pantalla.assign(240 * 240, 0);
  g_s.iniciar(halBroche(), g_franja, perfilBroche());
  g_s.modoVistaPrevia();  // sin QR: estos tests no son del vínculo
}
void correr(uint32_t ms) {
  for (uint32_t i = 0; i < ms; i += 20) g_t += 20, g_s.tick();
}
void tocar(uint32_t ms) {
  g_s.tacto(true);
  correr(ms);
  g_s.tacto(false);
  correr(40);
}
std::string visible() { return g_s.config().expresiones[g_s.motor().expresionVisible()].nombre; }
}  // namespace

PRUEBA("franjas: dibujar de a 40 filas da los mismos píxeles que de una vez") {
  cargarBase(g_cfg);
  for (bool redonda : {false, true}) {
    const int w = redonda ? 240 : 320, h = redonda ? 240 : 172;
    static uint16_t completo[320 * 240], franja[320 * 40], armado[320 * 240];
    const Vista v = Vista::para(w, h, redonda);
    EstadoCara e;
    e.cierre = 0.3f, e.miradaX = 0.02f;
    for (int i = 0; i < g_cfg.nExpresiones; i++) {
      Lienzo a;
      a.iniciar(completo, w, h);
      a.limpiar(paleta::PANTALLA);
      dibujarExpresion(a, v, g_cfg.expresiones[i], e);
      Lienzo b;
      b.iniciar(franja, w, h, 40);
      for (int y0 = 0; y0 < h; y0 += 40) {
        b.empezarFranja(y0);
        b.limpiar(paleta::PANTALLA);
        dibujarExpresion(b, v, g_cfg.expresiones[i], e);
        std::memcpy(&armado[y0 * w], franja, sizeof(uint16_t) * w * b.filasFranja());
      }
      const bool igual = std::memcmp(completo, armado, sizeof(uint16_t) * w * h) == 0;
      VERIFICAR(igual);
      if (!igual) std::printf("    distinta: %s (%s)\n", g_cfg.expresiones[i].nombre, redonda ? "redonda" : "rectangular");
    }
    // También la pantalla de emparejamiento (texto y QR).
    Lienzo a, b;
    a.iniciar(completo, w, h);
    dibujarEmparejamiento(a, "001", "SINTETICA:V1/A1B2C3D4E5F6/K7Q2XM", "ESPERANDO VÍNCULO", redonda);
    b.iniciar(franja, w, h, 40);
    for (int y0 = 0; y0 < h; y0 += 40) {
      b.empezarFranja(y0);
      dibujarEmparejamiento(b, "001", "SINTETICA:V1/A1B2C3D4E5F6/K7Q2XM", "ESPERANDO VÍNCULO", redonda);
      std::memcpy(&armado[y0 * w], franja, sizeof(uint16_t) * w * b.filasFranja());
    }
    VERIFICAR(std::memcmp(completo, armado, sizeof(uint16_t) * w * h) == 0);
  }
}

PRUEBA("redonda: los ojos más grandes y todo dentro del círculo") {
  cargarBase(g_cfg);
  static uint16_t px[240 * 240];
  for (int i = 0; i < g_cfg.nExpresiones; i++) {
    Lienzo l;
    l.iniciar(px, 240, 240);
    l.limpiar(paleta::PANTALLA);
    dibujarExpresion(l, Vista::para(240, 240, true), g_cfg.expresiones[i], EstadoCara());
    int fuera = 0;
    for (int y = 0; y < 240; y++)
      for (int x = 0; x < 240; x++) {
        const float dx = x + 0.5f - 120, dy = y + 0.5f - 120;
        if (dx * dx + dy * dy > 118 * 118 && de565(px[y * 240 + x]) != de565(a565(paleta::PANTALLA))) fuera++;
      }
    VERIFICAR(fuera == 0);
    if (fuera) std::printf("    %s: %d píxeles fuera del círculo\n", g_cfg.expresiones[i].nombre, fuera);
  }
  // La neutral: 72 px de alto en la referencia, acá 72 × 1,5 × (120 / 160) = 81.
  Lienzo l;
  l.iniciar(px, 240, 240);
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(240, 240, true), g_cfg.expresiones[0], EstadoCara());
  int y0 = -1, y1 = -1;
  for (int y = 0; y < 240; y++)
    if ((de565(px[y * 240 + 64]) >> 16) > 128) y1 = y, y0 = y0 < 0 ? y : y0;
  VERIFICAR(y1 - y0 + 1 >= 80 && y1 - y0 + 1 <= 82);  // 81 px con los bordes al 50%
  if (y1 - y0 + 1 < 80 || y1 - y0 + 1 > 82) std::printf("    alto del ojo: %d (y %d a %d)\n", y1 - y0 + 1, y0, y1);
}

PRUEBA("broche: presenta en 6 franjas de 40 filas") {
  arrancarBroche();
  g_franjas = 0;
  correr(20);
  VERIFICAR(g_franjas == 6);
  VERIFICAR((de565(g_pantalla[120 * 240 + 64]) >> 16) > 200);  // un ojo naranja en su lugar
}

PRUEBA("broche: un toque pasa a la cara siguiente y la posición se guarda") {
  arrancarBroche();
  correr(300);
  VERIFICAR(visible() == "neutral");
  tocar(100);
  correr(300);
  VERIFICAR(visible() == "contento");
  tocar(100);
  correr(300);
  VERIFICAR(visible() == "sospecha");
  arrancarBroche(false);  // reinicio
  correr(300);
  VERIFICAR(visible() == "sospecha");
  // Con la cara de base no se duerme sola a los 10 min.
  correr(11 * 60 * 1000);
  VERIFICAR(visible() == "sospecha");
}

PRUEBA("broche: mantener 2 s lo duerme, un toque lo despierta") {
  arrancarBroche();
  correr(200);
  g_s.tacto(true);
  correr(2200);
  VERIFICAR(visible() == "dormido");  // mientras se mantiene se le cierran los ojos
  VERIFICAR(!g_s.quiereDormir());
  g_s.tacto(false);
  correr(40);
  VERIFICAR(g_s.quiereDormir());
  VERIFICAR(g_pantalla[120 * 240 + 64] == 0);  // pantalla negra
  tocar(100);
  VERIFICAR(!g_s.quiereDormir());
  correr(200);
  VERIFICAR(visible() == "neutral");  // despertar no cuenta como "siguiente"
}

PRUEBA("broche: mantener 8 s muestra el QR y no lo duerme") {
  arrancarBroche();
  g_s.iniciar(halBroche(), g_franja, perfilBroche());  // sin vista previa: con QR
  // Ya vinculado (si no, el QR está siempre).
  g_mem["claves"] = std::vector<uint8_t>(16, 1);
  g_s.iniciar(halBroche(), g_franja, perfilBroche());
  correr(100);
  VERIFICAR(!g_s.emparejando());
  g_s.tacto(true);
  correr(8100);
  VERIFICAR(g_s.emparejando());
  g_s.tacto(false);
  correr(40);
  VERIFICAR(!g_s.quiereDormir());
}

PRUEBA("lista: caras e imágenes, siguiente y rotación") {
  arrancarBroche();
  const char* cfg = R"({"version":1,"lista":["cara:enamorado","imagen:2","cara:guino"],"rotar":1})";
  VERIFICAR(cargarConfig(g_cfg, cfg, std::strlen(cfg)).ok());
  VERIFICAR(g_cfg.nLista == 3 && g_cfg.rotarMinutos == 1);
  VERIFICAR(g_cfg.lista[1].tipo == ElementoLista::Tipo::Imagen && g_cfg.lista[1].indice == 2);
  const char* malas[] = {R"({"version":1,"lista":["cara:inventada"]})", R"({"version":1,"lista":["imagen:9"]})",
                         R"({"version":1,"lista":["foto"]})", R"({"version":1,"rotar":5000})"};
  for (const char* m : malas) VERIFICAR(!cargarConfig(g_cfg, m, std::strlen(m)).ok());

  g_mem["config"] = std::vector<uint8_t>(cfg, cfg + std::strlen(cfg));
  arrancarBroche(false);
  correr(300);
  VERIFICAR(visible() == "enamorado");
  correr(61 * 1000);  // rota sola al minuto
  VERIFICAR(g_s.posicionLista() == 1);
  tocar(100);
  correr(300);
  VERIFICAR(visible() == "guino");
}

PRUEBA("mascota: sin lista, el comportamiento automático de siempre") {
  g_mem.clear();
  static uint16_t px[320 * 172];
  Sistema s;
  Hal h = halBroche();
  h.presentarFranja = nullptr;
  g_t = 1000;
  s.iniciar(h, px, 320, 172);
  s.modoVistaPrevia();
  for (int i = 0; i < 30; i++) g_t += 20, s.tick();
  s.siguiente();  // no hace nada
  VERIFICAR(s.posicionLista() == 0);
  VERIFICAR(!s.motor().base().existe());
}
