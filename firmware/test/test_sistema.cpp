// El sistema completo con un HAL falso: reloj manual, almacenamiento en
// memoria y las notificaciones BLE guardadas para revisarlas.
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
int g_bateria = -1;
std::map<std::string, std::vector<uint8_t>> g_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_notis;
std::vector<std::string> g_sonidos;
uint16_t g_px[320 * 172];
Sistema g_s;

Hal halFalso() {
  Hal h;
  h.ahoraMs = [] { return g_ahora; };
  h.semilla = [] {
    static uint32_t x = 42;
    x = x * 1664525u + 1013904223u;
    return x;
  };
  h.identificador = [] { return "A1B2C3D4E5F6"; };
  h.bateria = [] { return g_bateria; };
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
  return h;
}

std::vector<uint8_t> g_clave;  // la que dio el último vínculo

void arrancar(bool autenticar = true) {
  g_ahora = 1000;
  g_notis.clear();
  g_s.iniciar(halFalso(), g_px, 320, 172);
  g_s.bleConexion(true);
  if (!autenticar) return;
  if (g_s.emparejando()) {
    std::vector<uint8_t> v{CmdVincular};
    v.insert(v.end(), g_s.codigo(), g_s.codigo() + 6);
    g_s.bleEscribir(CarComando, v.data(), v.size());
    for (auto& n : g_notis)
      if (n.first == CarComando && n.second[0] == (CmdVincular | Respuesta) && n.second[1] == Ok)
        g_clave.assign(n.second.begin() + 2, n.second.end());
    for (int i = 0; i < 160; i++) g_ahora += 20, g_s.tick();  // que pase el guiño del vínculo
  } else {
    std::vector<uint8_t> v{CmdAutenticar};
    v.insert(v.end(), g_clave.begin(), g_clave.end());
    g_s.bleEscribir(CarComando, v.data(), v.size());
  }
  g_notis.clear();
}

void correr(uint32_t ms) {
  for (uint32_t t = 0; t < ms; t += 20) {
    g_ahora += 20;
    g_s.tick();
  }
}

std::vector<uint8_t> bytes(std::initializer_list<uint8_t> l) { return l; }

void escribir(uint8_t car, const std::vector<uint8_t>& d) { g_s.bleEscribir(car, d.data(), d.size()); }

std::vector<uint8_t> conTexto(uint8_t op, const std::string& s) {
  std::vector<uint8_t> v{op};
  v.insert(v.end(), s.begin(), s.end());
  return v;
}

// Manda una config como lo haría la app: inicio, bloques de 100 bytes, fin.
uint8_t mandarConfig(const std::string& json) {
  g_notis.clear();
  std::vector<uint8_t> ini(9);
  ini[0] = ConfigInicio;
  escribir32(&ini[1], static_cast<uint32_t>(json.size()));
  escribir32(&ini[5], crc32(reinterpret_cast<const uint8_t*>(json.data()), json.size()));
  escribir(CarConfig, ini);
  for (size_t off = 0; off < json.size(); off += 100) {
    std::vector<uint8_t> b(5);
    b[0] = ConfigBloque;
    escribir32(&b[1], static_cast<uint32_t>(off));
    b.insert(b.end(), json.begin() + off, json.begin() + std::min(json.size(), off + 100));
    escribir(CarConfig, b);
  }
  escribir(CarConfig, bytes({ConfigFin}));
  for (auto& n : g_notis)
    if (n.first == CarConfig && n.second[0] == (ConfigFin | Respuesta)) return n.second[1];
  return 0xEE;
}

std::string expresionActual() { return g_s.config().expresiones[g_s.motor().expresionVisible()].nombre; }
}  // namespace

PRUEBA("sistema: arranca en neutral y dibuja") {
  g_flash.clear();
  arrancar(false);
  correr(200);
  VERIFICAR(expresionActual() == "neutral");
  VERIFICAR(std::string(g_s.nombre()) == "bicho");
  // Se notificó el estado del arranque.
  bool hubo = false;
  for (auto& n : g_notis) hubo = hubo || n.first == CarEstado;
  VERIFICAR(hubo);
}

PRUEBA("sistema: el botón dispara las reglas del manual") {
  g_flash.clear();
  arrancar();
  correr(100);
  g_s.tacto(true);
  correr(100);
  g_s.tacto(false);
  correr(600);
  VERIFICAR(expresionActual() == "contento");
  correr(3500);
  VERIFICAR(expresionActual() == "neutral");
}

PRUEBA("sistema: batería baja muestra su cara") {
  g_flash.clear();
  arrancar();
  g_bateria = 10;
  correr(300);
  VERIFICAR(expresionActual() == "bateria-baja");
  g_bateria = 80;
  correr(300);
  VERIFICAR(expresionActual() == "neutral");
  g_bateria = -1;
}

PRUEBA("protocolo: crc32 estándar") {
  const char* t = "123456789";
  VERIFICAR(crc32(reinterpret_cast<const uint8_t*>(t), 9) == 0xCBF43926u);
}

PRUEBA("protocolo: comando de expresión y nombre, y sobreviven a un reinicio") {
  g_flash.clear();
  arrancar();
  g_notis.clear();
  escribir(CarComando, conTexto(CmdExpresion, "enamorado"));
  VERIFICAR(!g_notis.empty() && g_notis.back().second == bytes({CmdExpresion | Respuesta, Ok}));
  escribir(CarComando, conTexto(CmdNombre, "Toto"));
  VERIFICAR(g_notis.back().second == bytes({CmdNombre | Respuesta, Ok}));
  escribir(CarComando, conTexto(CmdExpresion, "no-existe"));
  VERIFICAR(g_notis.back().second == bytes({CmdExpresion | Respuesta, NoExiste}));
  correr(300);
  VERIFICAR(expresionActual() == "enamorado");

  arrancar();  // reinicio: el flash queda
  correr(300);
  VERIFICAR(expresionActual() == "enamorado");
  VERIFICAR(std::string(g_s.nombre()) == "Toto");

  uint8_t est[64];
  size_t n = g_s.estadoBle(est, sizeof est);
  VERIFICAR(n > 6 && est[0] == 2 && (est[4] & 2));
  VERIFICAR(std::string(reinterpret_cast<char*>(est + 6), est[5]) == "enamorado");

  escribir(CarComando, bytes({CmdExpresion}));  // vacío = automático
  correr(300);
  VERIFICAR(expresionActual() == "neutral");
}

PRUEBA("protocolo: evento por comando") {
  g_flash.clear();
  arrancar();
  escribir(CarComando, bytes({CmdEvento, static_cast<uint8_t>(Evento::Sacudida)}));
  correr(300);
  VERIFICAR(expresionActual() == "mareado");
  escribir(CarComando, bytes({CmdEvento, 99}));
  VERIFICAR(g_notis.back().second == bytes({CmdEvento | Respuesta, Malformado}));
}

PRUEBA("protocolo: config por bloques, se aplica, se guarda y se puede pedir") {
  g_flash.clear();
  arrancar();
  const std::string cfg =
      R"({"version":1,"nombre":"Pochi","cara":{"color":"#33CCFF"},)"
      R"("reglas":[{"si":"sacudida","entonces":["expresion:enojado","sonido:grr"]}]})";
  VERIFICAR(mandarConfig(cfg) == Ok);
  VERIFICAR(std::string(g_s.nombre()) == "Pochi");
  VERIFICAR(g_s.config().hayColorOjos && g_s.config().colorOjos == 0x33CCFF);
  g_s.evento(Evento::Sacudida);
  correr(300);
  VERIFICAR(expresionActual() == "enojado");
  VERIFICAR(!g_sonidos.empty() && g_sonidos.back() == "grr");

  arrancar();
  VERIFICAR(std::string(g_s.nombre()) == "Pochi");

  // Pedirla de vuelta: trozos + cierre con el mismo CRC.
  g_notis.clear();
  escribir(CarConfig, bytes({ConfigPedir}));
  std::string armada;
  uint32_t crc = 0, largoTotal = 0;
  for (auto& n : g_notis) {
    if (n.first != CarConfig) continue;
    if (n.second[0] == ConfigTrozo) armada.append(n.second.begin() + 5, n.second.end());
    if (n.second[0] == ConfigCierre) largoTotal = leer32(&n.second[1]), crc = leer32(&n.second[5]);
  }
  VERIFICAR(armada == cfg);
  VERIFICAR(largoTotal == cfg.size());
  VERIFICAR(crc == crc32(reinterpret_cast<const uint8_t*>(cfg.data()), cfg.size()));
}

PRUEBA("protocolo: errores de config no rompen nada") {
  g_flash.clear();
  arrancar();
  VERIFICAR(mandarConfig(R"({"version":1,"cara":{"tamano":9}})") == ConfigInvalida);
  VERIFICAR(mandarConfig(R"({"version":1,)") == JsonInvalido);
  // CRC mal.
  g_notis.clear();
  std::vector<uint8_t> ini(9);
  ini[0] = ConfigInicio;
  escribir32(&ini[1], 2);
  escribir32(&ini[5], 1234);
  escribir(CarConfig, ini);
  escribir(CarConfig, bytes({ConfigBloque, 0, 0, 0, 0, '{', '}'}));
  escribir(CarConfig, bytes({ConfigFin}));
  VERIFICAR(g_notis.back().second == bytes({ConfigFin | Respuesta, CrcDistinto}));
  // Bloque fuera de orden.
  escribir32(&ini[5], 0);
  escribir(CarConfig, ini);
  escribir(CarConfig, bytes({ConfigBloque, 1, 0, 0, 0, '}'}));
  VERIFICAR(g_notis.back().second == bytes({ConfigBloque | Respuesta, FueraDeOrden}));
  // Sin inicio.
  escribir(CarConfig, bytes({ConfigFin}));
  VERIFICAR(g_notis.back().second[1] == SinInicio);
  // OTA todavía no.
  escribir(CarOta, bytes({1}));
  VERIFICAR(g_notis.back().second[1] == NoImplementado);
  VERIFICAR(std::string(g_s.nombre()) == "bicho");
  VERIFICAR(g_flash.find("config") == g_flash.end());
}

PRUEBA("sistema: una config guardada rota no impide arrancar") {
  g_flash.clear();
  const std::string basura = "{\"version\":1,\"nombre\":";
  g_flash["config"] = std::vector<uint8_t>(basura.begin(), basura.end());
  arrancar();
  correr(200);
  VERIFICAR(expresionActual() == "neutral");
  VERIFICAR(std::string(g_s.nombre()) == "bicho");
}

// --- Vínculo y autenticación (protocolo v1.1) ---------------------------------

namespace {
uint8_t ultimaRespuesta(uint8_t op) {
  for (auto it = g_notis.rbegin(); it != g_notis.rend(); ++it)
    if (it->first == CarComando && it->second[0] == (op | Respuesta)) return it->second[1];
  return 0xEE;
}
std::vector<uint8_t> conBytes(uint8_t op, const char* s, size_t n) {
  std::vector<uint8_t> v{op};
  v.insert(v.end(), s, s + n);
  return v;
}
}  // namespace

PRUEBA("vínculo: el primer arranque muestra el QR con id y código") {
  g_flash.clear();
  arrancar(false);
  VERIFICAR(g_s.emparejando());
  VERIFICAR(std::string(g_s.identificador()) == "A1B2C3D4E5F6");
  VERIFICAR(std::string(g_s.codigo()).size() == 6);
  VERIFICAR(std::string(g_s.contenidoQr()) == std::string("SINTETICA:V1/A1B2C3D4E5F6/") + g_s.codigo());
  std::string codigo = g_s.codigo();
  arrancar(false);  // el código no cambia entre reinicios
  VERIFICAR(codigo == g_s.codigo());
}

PRUEBA("vínculo: sin autenticar no se escribe, pero el estado se lee") {
  g_flash.clear();
  arrancar(false);
  escribir(CarComando, conTexto(CmdNombre, "Intruso"));
  VERIFICAR(ultimaRespuesta(CmdNombre) == NoAutorizado);
  escribir(CarConfig, bytes({ConfigPedir}));
  VERIFICAR(g_notis.back().second[1] == NoAutorizado);
  VERIFICAR(std::string(g_s.nombre()) == "bicho");
  uint8_t est[96];
  VERIFICAR(g_s.estadoBle(est, sizeof est) > 0 && (est[4] & 4) && !(est[4] & 8));
}

PRUEBA("vínculo: código malo, código bueno, y guiño al vincular") {
  g_flash.clear();
  arrancar(false);
  escribir(CarComando, conBytes(CmdVincular, "XXXXXX", 6));
  VERIFICAR(ultimaRespuesta(CmdVincular) == NoAutorizado);
  VERIFICAR(g_s.emparejando());
  g_notis.clear();
  escribir(CarComando, conBytes(CmdVincular, g_s.codigo(), 6));
  VERIFICAR(ultimaRespuesta(CmdVincular) == Ok);
  VERIFICAR(g_notis.back().second.size() == 18);
  VERIFICAR(!g_s.emparejando());
  VERIFICAR(g_s.cantidadDeClaves() == 1);
  correr(200);
  VERIFICAR(expresionActual() == "guino");
  // Ya vinculado, el código solo no alcanza: hay que mostrar el QR de nuevo.
  escribir(CarComando, conBytes(CmdVincular, g_s.codigo(), 6));
  VERIFICAR(ultimaRespuesta(CmdVincular) == NoEmparejando);
}

PRUEBA("vínculo: cada conexión se autentica, y la clave sobrevive a un reinicio") {
  g_flash.clear();
  arrancar();  // vincula y guarda g_clave
  escribir(CarComando, conTexto(CmdNombre, "Toto"));
  VERIFICAR(ultimaRespuesta(CmdNombre) == Ok);
  g_s.bleConexion(false);
  g_s.bleConexion(true);
  escribir(CarComando, conTexto(CmdNombre, "Otro"));
  VERIFICAR(ultimaRespuesta(CmdNombre) == NoAutorizado);
  std::vector<uint8_t> mala(17, 7);
  mala[0] = CmdAutenticar;
  escribir(CarComando, mala);
  VERIFICAR(ultimaRespuesta(CmdAutenticar) == NoAutorizado);
  arrancar();  // reinicio + autenticar con la clave guardada
  VERIFICAR(!g_s.emparejando());
  escribir(CarComando, conTexto(CmdNombre, "Pochi"));
  VERIFICAR(ultimaRespuesta(CmdNombre) == Ok);
  VERIFICAR(std::string(g_s.nombre()) == "Pochi");
}

PRUEBA("vínculo: 5 intentos fallidos bloquean 30 s") {
  g_flash.clear();
  arrancar(false);
  for (int i = 0; i < 5; i++) escribir(CarComando, conBytes(CmdVincular, "ZZZZZZ", 6));
  escribir(CarComando, conBytes(CmdVincular, g_s.codigo(), 6));
  VERIFICAR(ultimaRespuesta(CmdVincular) == NoAutorizado);
  g_ahora += 31000;
  escribir(CarComando, conBytes(CmdVincular, g_s.codigo(), 6));
  VERIFICAR(ultimaRespuesta(CmdVincular) == Ok);
}

PRUEBA("vínculo: mantener el botón 5 s muestra el QR por 2 min") {
  g_flash.clear();
  arrancar();
  VERIFICAR(!g_s.emparejando());
  g_s.tacto(true);
  correr(5100);
  g_s.tacto(false);
  correr(100);
  VERIFICAR(g_s.emparejando());
  correr(121000);
  VERIFICAR(!g_s.emparejando());
}

PRUEBA("vínculo: desvincular olvida las claves y vuelve al QR") {
  g_flash.clear();
  arrancar();
  escribir(CarComando, bytes({CmdDesvincular}));
  VERIFICAR(ultimaRespuesta(CmdDesvincular) == Ok);
  VERIFICAR(g_s.emparejando());
  VERIFICAR(g_s.cantidadDeClaves() == 0);
  escribir(CarComando, conTexto(CmdNombre, "x"));
  VERIFICAR(ultimaRespuesta(CmdNombre) == NoAutorizado);
}

PRUEBA("vínculo: el número de serie se guarda y viaja en el estado") {
  g_flash.clear();
  arrancar();
  g_s.fijarSerie("001");
  arrancar();
  VERIFICAR(std::string(g_s.serie()) == "001");
  uint8_t est[96];
  size_t n = g_s.estadoBle(est, sizeof est);
  std::string todo(reinterpret_cast<char*>(est), n);
  VERIFICAR(todo.find("001") != std::string::npos);
  VERIFICAR(todo.find("A1B2C3D4E5F6") != std::string::npos);
}
