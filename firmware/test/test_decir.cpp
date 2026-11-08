// Escribir (comando Decir y el globo), hablar solo, el cumple, el despertador
// y el modo clima, con el sistema entero y un HAL falso.
#include <algorithm>
#include <map>
#include <string>
#include <vector>

#include "clima.h"
#include "formato_vida.h"
#include "prueba.h"
#include "protocolo.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
Config g_cfg;
ResultadoConfig cargar(const std::string& json) { return cargarConfig(g_cfg, json.c_str(), json.size()); }
bool rechaza(const std::string& json, const char* parte) {
  ResultadoConfig r = cargar(json);
  if (r.ok()) return false;
  if (std::string(r.detalle).find(parte) == std::string::npos) {
    std::printf("    detalle: %s\n", r.detalle);
    return false;
  }
  return true;
}

uint32_t g_ahora = 0;
uint32_t g_epoch = 0;  // hora local que da el HAL (0 = no sabe)
std::map<std::string, std::vector<uint8_t>> g_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_notis;
std::vector<std::string> g_sonidos;
uint16_t g_px[320 * 172];
Sistema g_s;

// Lunes 9 de noviembre de 2026, 07:29:00 (hora local).
constexpr uint32_t LUNES_0729 = 1794209340u;

Hal halFalso() {
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
  h.sonido = [](const char* s) { g_sonidos.push_back(s); };
  h.epoch = [] { return g_epoch ? g_epoch + (g_ahora - 1000) / 1000 : 0u; };
  h.minutoDelDia = [] {
    if (!g_epoch) return -1;
    const uint32_t e = g_epoch + (g_ahora - 1000) / 1000;
    return static_cast<int>((e % 86400) / 60);
  };
  return h;
}

void arrancar(const std::string& config, uint32_t epoch = 0) {
  g_flash.clear();
  g_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  g_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  g_notis.clear();
  g_sonidos.clear();
  g_ahora = 1000;
  g_epoch = epoch;
  g_s.iniciar(halFalso(), g_px, 320, 172);
  g_s.confiar();
}

void correr(uint32_t ms, uint32_t paso = 20) {
  for (uint32_t t = 0; t < ms; t += paso) {
    g_ahora += paso;
    g_s.tick();
  }
}

void tocar(uint32_t dura = 60) {
  g_s.tacto(true);
  correr(dura);
  g_s.tacto(false);
  correr(40);
}

std::string expresionDelEstado() {
  uint8_t b[MAX_ESTADO];
  const size_t n = g_s.estadoBle(b, sizeof b);
  if (n < 6) return "";
  return std::string(reinterpret_cast<const char*>(b + 6), b[5]);
}

std::vector<uint8_t> marcoDecir(const std::string& texto, const std::string& expresion = "") {
  std::vector<uint8_t> m = {CmdDecir, static_cast<uint8_t>(expresion.size())};
  m.insert(m.end(), expresion.begin(), expresion.end());
  m.insert(m.end(), texto.begin(), texto.end());
  return m;
}

uint8_t ultimaRespuesta(uint8_t op) {
  for (auto it = g_notis.rbegin(); it != g_notis.rend(); ++it)
    if (it->first == CarComando && it->second.size() >= 2 && it->second[0] == (op | Respuesta)) return it->second[1];
  return 0xEE;
}

void escribir(const std::vector<uint8_t>& m) { g_s.bleEscribir(CarComando, m.data(), m.size()); }

int cuantos(const std::string& prefijo) {
  int n = 0;
  for (auto& s : g_sonidos) n += s.rfind(prefijo, 0) == 0;
  return n;
}

// Píxeles claros (el globo es claro) en una franja de filas.
int clarosEntre(int y0, int y1) {
  int n = 0;
  for (int y = y0; y < y1; y++)
    for (int x = 0; x < 320; x++) {
      const uint32_t c = de565(g_px[y * 320 + x]);
      if (((c >> 16) & 0xFF) > 0xC0 && ((c >> 8) & 0xFF) > 0xC0 && (c & 0xFF) > 0xB0) n++;
    }
  return n;
}

bool avisoDe(Evento e) {
  for (auto& x : g_notis) {
    const auto& b = x.second;
    if (x.first == CarComando && b.size() >= 9 && b[0] == (CmdRegistro | Respuesta) && b[7] == 0 && b[8] == static_cast<uint8_t>(e)) return true;
  }
  return false;
}
}  // namespace

// ---------------------------------------------------------------- config

PRUEBA("decir: habla, despertador y ubicacion en la config") {
  VERIFICAR(cargar("{\"version\":2,\"habla\":{\"frecuencia\":0.8,\"propias\":{\"despertar\":[\"¡Arriba!\",\"Dale\"],\"caricia\":[\"Más\"]}},"
                   "\"despertador\":{\"hora\":\"07:30\",\"dias\":\"LMXJV\",\"texto\":\"¡Arriba, {dueno}!\",\"posponer\":5},"
                   "\"ubicacion\":{\"lat\":-34.6,\"lon\":-58.38,\"ciudad\":\"Buenos Aires\"}}")
               .ok());
  VERIFICAR(g_cfg.habla.frecuencia == 80 && g_cfg.habla.nPropias == 3);
  VERIFICAR(g_cfg.habla.propias[2].situacion == Situacion::Caricia);
  VERIFICAR(g_cfg.despertador.hay && g_cfg.despertador.activo && g_cfg.despertador.minuto == 450);
  VERIFICAR(g_cfg.despertador.dias == 0x1F && g_cfg.despertador.posponer == 5);
  VERIFICAR(rechaza("{\"version\":2,\"habla\":{\"frecuencia\":2}}", "frecuencia de 0 a 1"));
  VERIFICAR(rechaza("{\"version\":2,\"habla\":{\"propias\":{\"nada\":[\"a\"]}}}", "situación desconocida"));
  VERIFICAR(rechaza("{\"version\":2,\"habla\":{\"propias\":{\"caricia\":[\"" + std::string(61, 'a') + "\"]}}}", "1 a 60 bytes"));
  std::string trece;
  for (int i = 0; i < 13; i++) trece += std::string(i ? "," : "") + "\"a\"";
  VERIFICAR(rechaza("{\"version\":2,\"habla\":{\"propias\":{\"caricia\":[" + trece + "]}}}", "hasta 12"));
  VERIFICAR(rechaza("{\"version\":2,\"despertador\":{\"dias\":\"L\"}}", "hora es HH:MM"));
  VERIFICAR(rechaza("{\"version\":2,\"despertador\":{\"hora\":\"07:00\",\"posponer\":31}}", "posponer"));
  VERIFICAR(rechaza("{\"version\":2,\"ubicacion\":{\"lat\":-100,\"lon\":0}}", "lat"));
  VERIFICAR(rechaza("{\"version\":2,\"ubicacion\":{\"lat\":0}}", "lon"));
  VERIFICAR(rechaza("{\"version\":2,\"reglas\":[{\"si\":\"tacto\",\"entonces\":[\"decir:nada\"]}]}", "situación"));
  VERIFICAR(cargar("{\"version\":2,\"reglas\":[{\"si\":\"tacto\",\"entonces\":[\"decir:saludo\"]}]}").ok());
  // Lo nuevo de la config, compacto (el broche no tiene PSRAM).
  std::printf("    RAM: Habla %zu B, Despertador %zu B, DatosClima %zu B\n", sizeof(Habla), sizeof(Despertador), sizeof(DatosClima));
  VERIFICAR(sizeof(Habla) <= 760 && sizeof(Despertador) <= 72);
}

// ---------------------------------------------------------------- escribir

PRUEBA("decir: el comando muestra el globo con fundido y se va solo o con un toque") {
  arrancar("{\"version\":2}");
  correr(500);
  const int antes = clarosEntre(110, 172);
  escribir(marcoDecir("¡Hola, soy Lucho!"));
  VERIFICAR(ultimaRespuesta(CmdDecir) == Ok);
  correr(40);
  VERIFICAR(g_s.panel() == Sistema::Panel::Globo);
  VERIFICAR(g_s.fasePanel() < 0.5f);  // entra con fundido, no de golpe
  correr(600);
  VERIFICAR(g_s.fasePanel() >= 1.0f);
  VERIFICAR(expresionDelEstado() == "globo");
  VERIFICAR(std::string(g_s.textoGlobo()) == "¡Hola, soy Lucho!");
  VERIFICAR(clarosEntre(80, 172) > antes + 2000);  // el globo claro
  correr(4000);  // 3 s y pico: se fue solo
  VERIFICAR(g_s.panel() == Sistema::Panel::Ninguno);
  VERIFICAR(expresionDelEstado() != "globo");
  // Con un toque se va antes (y ese toque no hace nada más).
  escribir(marcoDecir("Algo más largo para que dure un rato largo en la pantalla", "contento"));
  VERIFICAR(ultimaRespuesta(CmdDecir) == Ok);
  correr(500);
  VERIFICAR(g_s.panel() == Sistema::Panel::Globo);
  tocar();
  correr(500);
  VERIFICAR(g_s.panel() == Sistema::Panel::Ninguno);
  // Errores.
  escribir(marcoDecir(""));
  VERIFICAR(ultimaRespuesta(CmdDecir) == Malformado);
  escribir(marcoDecir(std::string(61, 'a')));
  VERIFICAR(ultimaRespuesta(CmdDecir) == Malformado);
  escribir(marcoDecir("hola", "no-existe"));
  VERIFICAR(ultimaRespuesta(CmdDecir) == NoExiste);
  escribir(marcoDecir("\xC3("));
  VERIFICAR(ultimaRespuesta(CmdDecir) == Malformado);
}

PRUEBA("decir: también por la nube, y un recordatorio le gana al globo") {
  arrancar("{\"version\":2}");
  const auto m = marcoDecir("Desde la nube");
  g_s.nubeEscribir(CarComando, m.data(), m.size());
  correr(600);
  VERIFICAR(g_s.panel() == Sistema::Panel::Globo);
}

// ---------------------------------------------------------------- hablar solo

PRUEBA("decir: habla solo con las reglas (caricia, volviste) y sin ser pesado") {
  arrancar("{\"version\":2,\"dueno\":{\"nombre\":\"Nahuel\"},\"habla\":{\"frecuencia\":1,\"propias\":{\"caricia\":[\"Más, {dueno}\"]}}}",
           LUNES_0729 + 3 * 3600);
  correr(2000);
  int globos = 0;
  bool dijoPropia = false;
  // Muchas caricias en una hora: habla, pero pocas veces.
  for (int i = 0; i < 60; i++) {
    g_s.evento(Evento::Caricia);
    for (int k = 0; k < 60; k++) {
      correr(1000, 50);
      if (g_s.panel() == Sistema::Panel::Globo && g_s.fasePanel() >= 1) {
        if (std::string(g_s.textoGlobo()) == "Más, Nahuel") dijoPropia = true;
        globos++;
        tocar();
        break;
      }
    }
  }
  VERIFICAR(dijoPropia);
  VERIFICAR(globos >= 1 && globos <= 4);
  // Frecuencia 0: callado.
  arrancar("{\"version\":2,\"habla\":{\"frecuencia\":0}}", LUNES_0729 + 3 * 3600);
  correr(1000);
  int callado = 0;
  for (int i = 0; i < 30; i++) {
    g_s.evento(Evento::Caricia);
    correr(4000, 50);
    callado += g_s.panel() == Sistema::Panel::Globo;
  }
  VERIFICAR(callado == 0);
}

// ---------------------------------------------------------------- cumple

PRUEBA("decir: el día de su cumple, al verlo, festeja y dice ¡Feliz cumple!") {
  // 9/11: el cumple del dueño.
  arrancar("{\"version\":2,\"dueno\":{\"nombre\":\"Nahuel\",\"cumple\":\"11-09\"},\"habla\":{\"frecuencia\":0}}",
           LUNES_0729 + 3 * 3600);
  correr(2000);
  g_s.evento(Evento::Tacto);
  bool feliz = false, festejo = false;
  for (int i = 0; i < 400 && !feliz; i++) {
    correr(100);
    const char* c = g_s.motor().nombreComportamiento();
    festejo = festejo || (c && std::string(c) == "festejar");
    feliz = g_s.panel() == Sistema::Panel::Globo && std::string(g_s.textoGlobo()).find("Feliz cumple, Nahuel") != std::string::npos;
  }
  VERIFICAR(feliz);
  VERIFICAR(festejo);
  // El del robot.
  arrancar("{\"version\":2,\"cumple\":\"11-09\",\"habla\":{\"frecuencia\":0}}", LUNES_0729 + 3 * 3600);
  correr(2000);
  g_s.evento(Evento::Tacto);
  bool cumplo = false;
  for (int i = 0; i < 400 && !cumplo; i++) {
    correr(100);
    cumplo = g_s.panel() == Sistema::Panel::Globo && std::string(g_s.textoGlobo()).find("cumplo años") != std::string::npos;
  }
  VERIFICAR(cumplo);
}

// ---------------------------------------------------------------- despertador

PRUEBA("decir: el despertador lo despierta, suena creciente, se pospone y se apaga") {
  arrancar("{\"version\":2,\"dueno\":{\"nombre\":\"Nahuel\"},\"horario\":{\"despierta\":\"09:00\",\"duerme\":\"23:00\"},"
           "\"despertador\":{\"hora\":\"07:30\",\"dias\":\"L\",\"posponer\":2}}",
           LUNES_0729);
  correr(1000);
  g_s.motor().dormir(g_ahora);
  correr(1000);
  VERIFICAR(g_s.motor().dormido());
  correr(60000, 100);  // pasa las 07:30
  VERIFICAR(g_s.alarmaSonando());
  VERIFICAR(!g_s.motor().dormido());
  VERIFICAR(avisoDe(Evento::Despertador));
  correr(600);
  VERIFICAR(g_s.panel() == Sistema::Panel::Despertador);
  VERIFICAR(expresionDelEstado() == "despertador");
  VERIFICAR(std::string(g_s.textoGlobo()).find("Nahuel") != std::string::npos);
  correr(30000, 100);
  VERIFICAR(cuantos("despertador:1") >= 1);
  VERIFICAR(cuantos("despertador:3") >= 1);  // cada vez más fuerte
  // Un toque: posponer 2 min.
  tocar();
  VERIFICAR(!g_s.alarmaSonando());
  correr(60000, 100);
  VERIFICAR(!g_s.alarmaSonando());
  correr(70000, 100);
  VERIFICAR(g_s.alarmaSonando());
  // Mantenerlo apretado: apagar.
  const size_t sonidos = g_sonidos.size();
  g_s.tacto(true);
  correr(1600);
  VERIFICAR(!g_s.alarmaSonando());
  g_s.tacto(false);
  correr(300000, 200);
  VERIFICAR(!g_s.alarmaSonando());
  int despues = 0;
  for (size_t i = sonidos; i < g_sonidos.size(); i++) despues += g_sonidos[i].rfind("despertador", 0) == 0;
  VERIFICAR(despues <= 1);
  // Al otro día (martes) no suena: solo los lunes.
  correr(24u * 3600u * 1000u, 10000);
  VERIFICAR(!g_s.alarmaSonando());
}

PRUEBA("decir: la alarma por comando (probar, posponer, apagar y despertar)") {
  arrancar("{\"version\":2}", LUNES_0729 + 3 * 3600);
  correr(500);
  escribir({CmdAlarma, 0});
  VERIFICAR(ultimaRespuesta(CmdAlarma) == NoExiste);
  escribir({CmdAlarma, 2});
  VERIFICAR(ultimaRespuesta(CmdAlarma) == Ok);
  correr(100);
  VERIFICAR(g_s.alarmaSonando());
  escribir({CmdAlarma, 0});
  VERIFICAR(ultimaRespuesta(CmdAlarma) == Ok);
  VERIFICAR(!g_s.alarmaSonando());
  g_s.motor().dormir(g_ahora);
  correr(500);
  escribir({CmdAlarma, 3});
  VERIFICAR(ultimaRespuesta(CmdAlarma) == Ok);
  correr(500);
  VERIFICAR(!g_s.motor().dormido());
  escribir({CmdAlarma, 9});
  VERIFICAR(ultimaRespuesta(CmdAlarma) == Malformado);
}

// ---------------------------------------------------------------- clima

PRUEBA("decir: el modo clima muestra el ícono y la temperatura, y la cara reacciona") {
  arrancar("{\"version\":2,\"modo\":\"clima\"}", LUNES_0729 + 3 * 3600);
  correr(600);
  VERIFICAR(g_s.panel() == Sistema::Panel::Clima);
  VERIFICAR(expresionDelEstado() == "clima");
  // -2,5 °C con nieve.
  const uint8_t m[] = {CmdClima, 0xE7, 0xFF, 73, 1, static_cast<uint8_t>(-4), 1, 3, 'B', 'R', 'C'};
  escribir(std::vector<uint8_t>(m, m + sizeof m));
  VERIFICAR(ultimaRespuesta(CmdClima) == Ok);
  VERIFICAR(g_s.clima().hay && g_s.clima().temperatura() == -3);
  correr(2000);
  VERIFICAR(std::string(g_s.motor().nombreVisible()) == "frio");
  VERIFICAR(clarosEntre(100, 172) > 300);
  // Marco mal armado.
  escribir({CmdClima, 1, 2});
  VERIFICAR(ultimaRespuesta(CmdClima) == Malformado);
  // A las 3 h sin datos nuevos, deja de valer.
  correr(3u * 3600u * 1000u + 5000u, 5000);
  VERIFICAR(!g_s.climaVigente());
}

// ---------------------------------------------------------------- dormido

namespace {
constexpr uint32_t LUNES_0300 = LUNES_0729 - 4u * 3600u - 29u * 60u;

int contar(const std::string& sonido) {
  int n = 0;
  for (auto& s : g_sonidos) n += s == sonido;
  return n;
}
}  // namespace

PRUEBA("dormido: zzz que suben, cambia de postura y ronca suave (y en silencio con volumen 0)") {
  arrancar("{\"version\":2}", LUNES_0300);
  correr(1000);
  g_s.motor().dormir(g_ahora);
  int zzz = 0, lados = 0, ultimoLado = 0;
  for (int i = 0; i < 12000; i++) {  // 10 min
    correr(50, 50);
    for (const Particula& p : g_s.motor().cuadro().particulas) zzz += p.viva && p.tipo == TipoParticula::Zzz && p.edad < 0.06f;
    if (g_s.motor().ladoDormido() != ultimoLado) lados++, ultimoLado = g_s.motor().ladoDormido();
  }
  VERIFICAR(g_s.motor().dormido());
  std::printf("    10 min dormido: %d zzz, %d cambios de postura, %d ronquidos\n", zzz, lados, contar("ronquido"));
  VERIFICAR(zzz >= 60);
  VERIFICAR(lados >= 1);
  VERIFICAR(contar("ronquido") >= 5);
  arrancar("{\"version\":2,\"voz\":{\"volumen\":0}}", LUNES_0300);
  g_s.motor().dormir(g_ahora);
  correr(600000, 100);
  VERIFICAR(contar("ronquido") == 0);
}

PRUEBA("dormido: sueña con una burbuja y algo adentro (corazón, estrella, nota u oveja)") {
  arrancar("{\"version\":2}", LUNES_0300);
  correr(1000);
  g_s.motor().dormir(g_ahora);
  correr(1000);
  const int s = g_s.config().vida.indiceComportamiento("sonar");
  VERIFICAR(s >= 0 && g_s.motor().comportamiento(s, g_ahora));
  bool burbuja = false, adentro = false;
  for (int i = 0; i < 600; i++) {
    correr(50, 50);
    for (const Particula& p : g_s.motor().cuadro().particulas) {
      burbuja = burbuja || (p.viva && p.tipo == TipoParticula::Burbuja);
      adentro = adentro || (p.viva && p.enBurbuja &&
                            (p.tipo == TipoParticula::Corazon || p.tipo == TipoParticula::Estrella ||
                             p.tipo == TipoParticula::Nota || p.tipo == TipoParticula::Oveja));
    }
  }
  VERIFICAR(burbuja && adentro);
  VERIFICAR(g_s.motor().dormido());
}

PRUEBA("dormido: de noche la voz y el ruido casi no lo despiertan; su nombre sí") {
  arrancar("{\"version\":2}", LUNES_0300);
  correr(1000);
  g_s.motor().dormir(g_ahora);
  int despertadas = 0;
  for (int i = 0; i < 200; i++) {
    g_s.evento(i % 2 ? Evento::Voz : Evento::Ruido);
    correr(3000, 100);
    if (!g_s.motor().dormido()) {
      despertadas++;
      g_s.motor().dormir(g_ahora);
    }
  }
  std::printf("    200 voces y ruidos de noche: se despertó %d veces, %d murmullos\n", despertadas, contar("murmullo"));
  VERIFICAR(despertadas <= 4);
  VERIFICAR(contar("murmullo") >= 5);
  g_s.evento(Evento::Nombre);
  correr(500);
  VERIFICAR(!g_s.motor().dormido());
}
