// "cara.tamano": la cara paramétrica (y la de imagen) se dibuja de ese tamaño,
// cambia con un fundido (sin salto, sin depender de los fps) y nunca se sale
// de la pantalla, ni rectangular ni redonda.
#include <algorithm>
#include <cmath>
#include <map>
#include <string>
#include <vector>

#include "prueba.h"
#include "protocolo.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
uint32_t tm_ahora = 1000;
std::map<std::string, std::vector<uint8_t>> tm_flash;
std::vector<std::vector<uint8_t>> tm_notis;
uint16_t tm_px[320 * 240];
Sistema tm_s;

Hal halTamano() {
  Hal h;
  h.ahoraMs = [] { return tm_ahora; };
  h.semilla = [] { return 3u; };
  h.guardar = [](const char* k, const uint8_t* d, size_t n) {
    tm_flash[k] = std::vector<uint8_t>(d, d + n);
    return true;
  };
  h.leer = [](const char* k, uint8_t* d, size_t cap) {
    auto it = tm_flash.find(k);
    if (it == tm_flash.end() || it->second.size() > cap) return -1;
    std::copy(it->second.begin(), it->second.end(), d);
    return static_cast<int>(it->second.size());
  };
  h.notificar = [](uint8_t, const uint8_t* d, size_t n) { tm_notis.push_back(std::vector<uint8_t>(d, d + n)); };
  return h;
}

struct Pantalla {
  int ancho, alto;
  bool redonda;
  const char* nombre;
};
const Pantalla PANTALLAS[] = {{320, 172, false, "rectangular"}, {240, 240, true, "redonda"}, {240, 320, false, "vertical"}};
Pantalla g_pantalla = PANTALLAS[0];

void arrancarTamano(float tamano, bool redonda) {
  const std::string config = "{\"version\":2,\"habla\":{\"frecuencia\":0},\"cara\":{\"tamano\":" + std::to_string(tamano) + "}}";
  tm_flash.clear();
  tm_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  tm_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  tm_ahora = 1000;
  Perfil p;
  (void)redonda;
  p.ancho = g_pantalla.ancho, p.alto = g_pantalla.alto, p.redonda = g_pantalla.redonda;
  tm_s.iniciar(halTamano(), tm_px, p);
  tm_s.confiar();
}

void correrTamano(uint32_t ms, uint32_t paso = 33) {
  for (uint32_t t = 0; t < ms; t += paso) tm_ahora += paso, tm_s.tick();
}

// El color más común: el fondo.
uint16_t fondoDe(int W, int H) {
  std::map<uint16_t, int> c;
  for (int i = 0; i < W * H; i++) c[tm_px[i]]++;
  uint16_t m = 0;
  int n = -1;
  for (auto& x : c)
    if (x.second > n) n = x.second, m = x.first;
  return m;
}

int encendidos(int W, int H, uint16_t fondo) {
  int n = 0;
  for (int i = 0; i < W * H; i++) n += tm_px[i] != fondo;
  return n;
}

// Lo brillante que se ve en el borde (rectangular: el marco de 2 px; redonda: fuera del 96 % del radio).
int enElBorde(int W, int H, bool redonda, uint16_t fondo) {
  int n = 0;
  for (int y = 0; y < H; y++)
    for (int x = 0; x < W; x++) {
      bool borde;
      if (redonda) {
        const float dx = x + 0.5f - W / 2.0f, dy = y + 0.5f - H / 2.0f;
        borde = dx * dx + dy * dy > (0.96f * W / 2) * (0.96f * W / 2) && dx * dx + dy * dy < (W / 2.0f) * (W / 2.0f);
      } else {
        borde = x < 2 || y < 2 || x >= W - 2 || y >= H - 2;
      }
      // La cara de verdad (los ojos, la boca): brillante; el resplandor es tenue.
      n += borde && tm_px[y * W + x] != fondo && (tm_px[y * W + x] >> 11) >= 16;
    }
  return n;
}

void mandarConfigTamano(const std::string& json) {
  std::vector<uint8_t> ini{ConfigInicio, 0, 0, 0, 0, 0, 0, 0, 0};
  escribir32(&ini[1], static_cast<uint32_t>(json.size()));
  escribir32(&ini[5], crc32(reinterpret_cast<const uint8_t*>(json.data()), json.size()));
  tm_s.bleEscribir(CarConfig, ini.data(), ini.size());
  std::vector<uint8_t> b{ConfigBloque, 0, 0, 0, 0};
  b.insert(b.end(), json.begin(), json.end());
  tm_s.bleEscribir(CarConfig, b.data(), b.size());
  const uint8_t fin[] = {ConfigFin};
  tm_s.bleEscribir(CarConfig, fin, 1);
}
}  // namespace

PRUEBA("tamaño: cara.tamano agranda o achica la cara de verdad (0,65 y 1,4 no son la misma)") {
  for (const Pantalla& pa : PANTALLAS) {
    g_pantalla = pa;
    const bool redonda = pa.redonda;
    const int W = pa.ancho, H = pa.alto;
    arrancarTamano(0.65f, redonda);
    correrTamano(500);
    const uint16_t fondo = fondoDe(W, H);
    const int chica = encendidos(W, H, fondo);
    arrancarTamano(1.4f, redonda);
    correrTamano(500);
    const int grande = encendidos(W, H, fondo);
    std::printf("    %s: con 0,65 %d píxeles, con 1,4 %d (tamaño visto %.2f)\n", pa.nombre, chica, grande, tm_s.tamanoVisto());
    VERIFICAR(grande > chica * 2);
    // En la redonda y en un celu parado entra entero; en la rectangular (baja), hasta donde entra.
    VERIFICAR(redonda || pa.alto > pa.ancho ? tm_s.tamanoVisto() > 1.39f : tm_s.tamanoVisto() >= 0.99f);
  }
}

// Lo peor que toca el borde en un repaso de caras grandes (sorpresa, emocionado…).
int peorBorde(float tamano, bool redonda) {
  const int W = g_pantalla.ancho, H = g_pantalla.alto;
  arrancarTamano(tamano, redonda);
  correrTamano(300);
  const uint16_t fondo = fondoDe(W, H);
  int peor = 0;
  for (const char* cara : {"neutral", "sorpresa", "contento", "curioso", "emocionado", "escuchando"}) {
    tm_s.motor().reaccionar(tm_s.config().resolverCara(cara), tm_ahora, 3000);
    int esta = 0;
    for (int k = 0; k < 40; k++) {
      correrTamano(50, 50);
      esta = std::max(esta, enElBorde(W, H, redonda, fondo));
    }
    peor = std::max(peor, esta);
  }
  return peor;
}

PRUEBA("tamaño: con 1,4 no se sale de la pantalla más que con 1 (rectangular ni redonda)") {
  for (const Pantalla& pa : PANTALLAS) {
    g_pantalla = pa;
    const bool redonda = pa.redonda;
    const int base = peorBorde(1.0f, redonda);
    const int grande = peorBorde(1.4f, redonda);
    std::printf("    %s: tamaño %.2f, píxeles en el borde %d (con 1: %d)\n", pa.nombre, tm_s.tamanoVisto(), grande, base);
    VERIFICAR(grande <= base + 8);  // las chispas que vuelan pueden rozar el borde igual que con 1
  }
}

PRUEBA("tamaño: cambia con un fundido, sin salto, igual a 30 y a 60 cuadros por segundo") {
  float despues[2];
  g_pantalla = PANTALLAS[2];
  for (int i = 0; i < 2; i++) {
    arrancarTamano(1.0f, false);
    correrTamano(300);
    VERIFICAR(std::fabs(tm_s.tamanoVisto() - 1.0f) < 0.001f);
    mandarConfigTamano("{\"version\":2,\"habla\":{\"frecuencia\":0},\"cara\":{\"tamano\":1.3}}");
    VERIFICAR(std::fabs(tm_s.config().tamano - 1.3f) < 0.001f);
    const uint32_t paso = i ? 16 : 33;
    float antes = tm_s.tamanoVisto(), mayor = 0;
    for (uint32_t t = 0; t < 150; t += paso) {
      tm_ahora += paso, tm_s.tick();
      mayor = std::max(mayor, tm_s.tamanoVisto() - antes);
      antes = tm_s.tamanoVisto();
    }
    despues[i] = tm_s.tamanoVisto();
    VERIFICAR(mayor < 0.1f);                   // ningún salto grande entre cuadros
    VERIFICAR(despues[i] > 1.1f && despues[i] < 1.28f);  // a mitad de camino
    correrTamano(1500, paso);
    VERIFICAR(std::fabs(tm_s.tamanoVisto() - 1.3f) < 0.01f);
  }
  VERIFICAR(std::fabs(despues[0] - despues[1]) < 0.03f);  // no depende de los fps
}
