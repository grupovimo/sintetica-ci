// La charla en el robot (conversar.cpp): el comando 0x4C, las acciones, la
// alarma y el recordatorio de una vez, los gustos guardados y la voz propia
// por Hal::audio.
#include <algorithm>
#include <cstdlib>
#include <cstring>
#include <map>
#include <string>
#include <vector>

#include "prueba.h"
#include "protocolo.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
uint32_t g_ahora = 0;
std::map<std::string, std::vector<uint8_t>> g_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_notis, g_notisNube;
std::vector<std::string> g_sonidos;
size_t g_muestras = 0;
int g_pico = 0;
uint16_t g_px[320 * 172];
Sistema g_s;

// Lunes 9 de noviembre de 2026, 14:00 (hora local).
constexpr uint32_t LUNES_1400 = 1794182400u + 14 * 3600;

Hal halFalso(bool parlante) {
  Hal h;
  h.ahoraMs = [] { return g_ahora; };
  h.semilla = [] { return 7u; };
  h.identificador = [] { return "A1B2C3D4E5F6"; };
  h.guardar = [](const char* k, const uint8_t* d, size_t n) {
    g_flash[k] = std::vector<uint8_t>(d, d + n);
    return true;
  };
  h.leer = [](const char* k, uint8_t* d, size_t cap) {
    auto it = g_flash.find(k);
    if (it == g_flash.end() || it->second.size() > cap) return -1;
    std::copy(it->second.begin(), it->second.end(), d);
    return static_cast<int>(it->second.size());
  };
  h.notificar = [](uint8_t c, const uint8_t* d, size_t n) { g_notis.push_back({c, std::vector<uint8_t>(d, d + n)}); };
  h.notificarNube = [](uint8_t c, const uint8_t* d, size_t n) { g_notisNube.push_back({c, std::vector<uint8_t>(d, d + n)}); };
  h.sonido = [](const char* s) { g_sonidos.push_back(s); };
  h.epoch = [] { return LUNES_1400 + (g_ahora - 1000) / 1000; };
  h.minutoDelDia = [] { return static_cast<int>(((LUNES_1400 + (g_ahora - 1000) / 1000) % 86400) / 60); };
  if (parlante)
    h.audio = [](const int16_t* m, size_t n) {
      g_muestras += n;
      for (size_t i = 0; i < n; i++) g_pico = std::max(g_pico, std::abs(static_cast<int>(m[i])));
    };
  return h;
}

void arrancar(bool parlante = true, const std::string& config = "") {
  g_flash.clear();
  if (!config.empty()) g_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  g_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  g_notis.clear();
  g_notisNube.clear();
  g_sonidos.clear();
  g_muestras = 0;
  g_pico = 0;
  g_ahora = 1000;
  g_s.iniciar(halFalso(parlante), g_px, 320, 172);
  g_s.confiar();
}

void correr(uint32_t ms, uint32_t paso = 20) {
  for (uint32_t t = 0; t < ms; t += paso) {
    g_ahora += paso;
    g_s.tick();
  }
}

bool charlar(const char* t, RespuestaCharla* r = nullptr) {
  RespuestaCharla x;
  const bool ok = g_s.charlar(t, std::strlen(t), r ? r : &x);
  correr(100);
  return ok;
}

std::vector<uint8_t> marcoCharlar(const std::string& t) {
  std::vector<uint8_t> m = {CmdCharlar, static_cast<uint8_t>(t.size())};
  m.insert(m.end(), t.begin(), t.end());
  return m;
}

const std::vector<uint8_t>* respuestaCharla(const std::vector<std::pair<uint8_t, std::vector<uint8_t>>>& notis) {
  for (auto it = notis.rbegin(); it != notis.rend(); ++it)
    if (it->first == CarComando && it->second.size() >= 2 && it->second[0] == (CmdCharlar | Respuesta)) return &it->second;
  return nullptr;
}
}  // namespace

PRUEBA("conversar: el comando 0x4C contesta con su globo y avisa qué entendió (0xCC)") {
  arrancar();
  const auto m = marcoCharlar("hola, ¿qué hora es?");
  g_s.bleEscribir(CarComando, m.data(), m.size());
  correr(600);
  const auto* r = respuestaCharla(g_notis);
  VERIFICAR(r && (*r)[1] == Ok && ((*r)[2] & 1) == 0);
  if (r) {
    const size_t n = (*r)[4];
    VERIFICAR(std::string(reinterpret_cast<const char*>(r->data() + 5), n) == "hora");
  }
  VERIFICAR(g_s.globoVisible() && std::strstr(g_s.textoGlobo(), "14:00"));
  // Mal armado.
  const std::vector<uint8_t> malo = {CmdCharlar, 5, 'h', 'o'};
  g_notis.clear();
  g_s.bleEscribir(CarComando, malo.data(), malo.size());
  VERIFICAR(!g_notis.empty() && g_notis.back().second[1] == Malformado);
}

PRUEBA("conversar: lo dice con su voz propia si hay parlante, y no si no hay") {
  arrancar(true);
  charlar("contame un chiste");
  correr(8000);
  VERIFICAR(g_muestras > 16000 / 2 && g_pico > 1000 && g_pico <= 30000);
  VERIFICAR(!g_s.hablandoEnVozAlta());
  arrancar(false);
  charlar("contame un chiste");
  correr(3000);
  VERIFICAR(g_muestras == 0);
  // Sin volumen, tampoco.
  arrancar(true, R"({"version":2,"voz":{"volumen":0}})");
  charlar("contame un chiste");
  correr(3000);
  VERIFICAR(g_muestras == 0);
}

PRUEBA("conversar: la alarma y el recordatorio de una vez suenan a su hora") {
  arrancar();
  VERIFICAR(charlar("poneme una alarma en 2 minutos"));
  VERIFICAR(g_s.alarmaCharla() != 0 && !g_s.alarmaSonando());
  // Avisó la acción por BLE y por la nube (para que la guarde).
  correr(60000, 50);
  VERIFICAR(!g_s.alarmaSonando());
  correr(70000, 50);
  VERIFICAR(g_s.alarmaSonando());
  RespuestaCharla r;
  VERIFICAR(charlar("ya me desperté", &r) && r.accion == AccionCharla::ApagarAlarma && !g_s.alarmaSonando());

  arrancar();
  VERIFICAR(charlar("recordame sacar la ropa en 1 minuto"));
  VERIFICAR(std::string(g_s.textoRecordatorioCharla()) == "Sacar la ropa");
  correr(70000, 50);
  VERIFICAR(g_s.recordatorioCharla() == 0);
  VERIFICAR(std::find(g_sonidos.begin(), g_sonidos.end(), "recordatorio") != g_sonidos.end());
}

PRUEBA("conversar: un despertador de todos los días queda en la config; los gustos, en la flash") {
  arrancar();
  VERIFICAR(charlar("despertame todos los días a las 7"));
  VERIFICAR(g_s.config().despertador.hay && g_s.config().despertador.minuto == 7 * 60 && g_s.config().despertador.dias == 0x7F);
  VERIFICAR(charlar("me gusta el mate"));
  VERIFICAR(charlar("me gustan los perros"));
  char gs[LARGO_LISTA_GUSTOS + 1];
  g_s.gustos(gs, sizeof gs);
  VERIFICAR(std::string(gs) == "el mate\nlos perros");
  RespuestaCharla r;
  correr(9000);
  VERIFICAR(charlar("¿qué me gusta?", &r) && std::strstr(r.frase, "el mate y los perros"));
  VERIFICAR(charlar("no me gusta el mate"));
  g_s.gustos(gs, sizeof gs);
  VERIFICAR(std::string(gs) == "los perros");
}

PRUEBA("conversar: si no entiende y hay alguien más que conteste, se queda pensando") {
  arrancar();
  RespuestaCharla r;
  VERIFICAR(!g_s.charlar("asdf qwer zxcv", 14, &r, false));
  correr(600);
  VERIFICAR(r.noEntendi && r.frase[0] && !g_s.globoVisible());
  // Con el comando sí dice que no entendió.
  const auto m = marcoCharlar("asdf qwer zxcv");
  g_s.bleEscribir(CarComando, m.data(), m.size());
  correr(600);
  const auto* x = respuestaCharla(g_notis);
  VERIFICAR(x && ((*x)[2] & 1) == 1 && g_s.globoVisible());
}

PRUEBA("conversar: jugar, cambiar de modo, callarse y dormir") {
  arrancar();
  VERIFICAR(charlar("tirá un dado"));
  VERIFICAR(g_s.juego() == Sistema::Juego::Dado);
  arrancar();
  VERIFICAR(charlar("modo reloj"));
  VERIFICAR(g_s.config().modo == Modo::Reloj);
  VERIFICAR(charlar("modo mascota"));
  VERIFICAR(charlar("andá a dormir"));
  correr(12000);
  VERIFICAR(g_s.motor().dormido());
}
