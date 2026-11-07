// v1.3: animaciones con claves, el canal de la nube, WiFi y OTA.
#include <algorithm>
#include <cstring>
#include <map>
#include <string>
#include <vector>

#include "cara.h"
#include "prueba.h"
#include "protocolo.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
uint32_t g_ahora = 0;
std::map<std::string, std::vector<uint8_t>> g_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_ble, g_nube;
int g_redCambios = 0;
uint16_t g_px[320 * 172];
Sistema g_s;

// OTA falsa: guarda lo que llega y "verifica" con un SHA de mentira (la suma de los bytes).
std::vector<uint8_t> g_otaBin;
ManifiestoOta g_otaMan;
bool g_otaFirmaOk = true;
std::string g_otaUrl;
bool g_otaTerminada = false;

Hal hal() {
  Hal h;
  h.ahoraMs = [] { return g_ahora; };
  h.semilla = [] {
    static uint32_t x = 7;
    x = x * 1664525u + 1013904223u;
    return x;
  };
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
  h.escribirTrozo = [](const char* k, uint32_t off, const uint8_t* d, size_t n) {
    auto& v = g_flash[k];
    if (off == 0) v.clear();
    if (off != v.size()) return false;
    v.insert(v.end(), d, d + n);
    return true;
  };
  h.leerTrozo = [](const char* k, uint32_t off, uint8_t* d, size_t n) {
    auto it = g_flash.find(k);
    if (it == g_flash.end()) return -1;
    if (off >= it->second.size()) return 0;
    size_t m = std::min(n, it->second.size() - off);
    std::copy(it->second.begin() + off, it->second.begin() + off + m, d);
    return static_cast<int>(m);
  };
  h.borrar = [](const char* k) { return g_flash.erase(k) > 0; };
  h.notificar = [](uint8_t c, const uint8_t* d, size_t n) { g_ble.push_back({c, std::vector<uint8_t>(d, d + n)}); };
  h.notificarNube = [](uint8_t c, const uint8_t* d, size_t n) { g_nube.push_back({c, std::vector<uint8_t>(d, d + n)}); };
  h.redCambio = [] { g_redCambios++; };
  h.otaEmpezar = [](const ManifiestoOta& m) -> uint8_t {
    if (!g_otaFirmaOk) return FirmaInvalida;
    g_otaMan = m;
    g_otaBin.clear();
    return Ok;
  };
  h.otaEscribir = [](uint32_t off, const uint8_t* d, size_t n) -> uint8_t {
    if (off != g_otaBin.size()) return FalloOta;
    g_otaBin.insert(g_otaBin.end(), d, d + n);
    return Ok;
  };
  h.otaTerminar = []() -> uint8_t {
    uint8_t suma = 0;
    for (uint8_t b : g_otaBin) suma = static_cast<uint8_t>(suma + b);
    if (suma != g_otaMan.sha256[0]) return HashDistinto;
    g_otaTerminada = true;
    return Ok;
  };
  h.otaDescargar = [](const ManifiestoOta& m, const char* url) -> uint8_t {
    g_otaMan = m;
    g_otaUrl = url;
    return Ok;
  };
  return h;
}

Perfil perfil() {
  Perfil p;
  p.nombre = "dev-147b";
  p.version = "0.2.0";
  return p;
}

void arrancar() {
  g_ahora = 1000;
  g_flash.clear();
  g_ble.clear();
  g_nube.clear();
  g_redCambios = 0;
  g_otaTerminada = false;
  g_otaFirmaOk = true;
  g_s.iniciar(hal(), g_px, perfil());
  g_s.bleConexion(true);
  std::vector<uint8_t> v{CmdVincular};
  v.insert(v.end(), g_s.codigo(), g_s.codigo() + 6);
  g_s.bleEscribir(CarComando, v.data(), v.size());
  for (int i = 0; i < 160; i++) g_ahora += 20, g_s.tick();
  g_ble.clear();
  g_nube.clear();
}

void correr(uint32_t ms) {
  for (uint32_t t = 0; t < ms; t += 20) {
    g_ahora += 20;
    g_s.tick();
  }
}

std::vector<uint8_t> v(std::initializer_list<uint8_t> l) { return l; }
std::vector<uint8_t> conTexto(uint8_t op, const std::string& s) {
  std::vector<uint8_t> r{op};
  r.insert(r.end(), s.begin(), s.end());
  return r;
}

void ble(uint8_t car, const std::vector<uint8_t>& b) { g_s.bleEscribir(car, b.data(), b.size()); }
void nube(uint8_t car, const std::vector<uint8_t>& b) { g_s.nubeEscribir(car, b.data(), b.size()); }

// Manda una config por el canal que diga y devuelve el código de ConfigFin.
int mandarConfig(const std::string& json, bool porNube = false) {
  auto& destino = porNube ? g_nube : g_ble;
  destino.clear();
  std::vector<uint8_t> ini{ConfigInicio, 0, 0, 0, 0, 0, 0, 0, 0};
  escribir32(&ini[1], static_cast<uint32_t>(json.size()));
  escribir32(&ini[5], crc32(reinterpret_cast<const uint8_t*>(json.data()), json.size()));
  auto enviar = [&](const std::vector<uint8_t>& b) { porNube ? nube(CarConfig, b) : ble(CarConfig, b); };
  enviar(ini);
  for (size_t off = 0; off < json.size(); off += 150) {
    std::vector<uint8_t> b{ConfigBloque, 0, 0, 0, 0};
    escribir32(&b[1], static_cast<uint32_t>(off));
    b.insert(b.end(), json.begin() + off, json.begin() + std::min(json.size(), off + 150));
    enviar(b);
  }
  enviar(v({ConfigFin}));
  return destino.empty() ? -1 : destino.back().second[1];
}

const char* CONFIG_ANIMADA = R"({"version":1,
  "animaciones":{
    "latido":{"duracion":1000,"repetir":true,"sobre":"neutral","pistas":[
      {"forma":{"forma":"corazon","x":0,"y":-0.3,"tamano":0.5,"color":"naranja"},
       "claves":[{"t":0,"escala":0.5,"opacidad":0,"curva":"salida"},{"t":500,"escala":1,"opacidad":1},{"t":1000,"escala":0.5,"opacidad":0}]}]},
    "giro":{"duracion":800,"pistas":[
      {"forma":{"forma":"pildora","x":0,"y":0,"w":0.4,"h":0.1},"centro":[0,0],
       "claves":[{"t":0,"rotacion":0,"curva":"lineal","color":"#FF0000"},{"t":800,"rotacion":180,"color":"#0000FF"}]}]}},
  "reglas":[{"si":"doble-toque","entonces":["animacion:giro"]}],
  "lista":["animacion:latido","cara:contento"]})";

std::string visible() {
  uint8_t b[160];
  size_t n = g_s.estadoBle(b, sizeof b);
  return n > 6 ? std::string(reinterpret_cast<char*>(b + 6), b[5]) : "";
}
}  // namespace

PRUEBA("animación: curvas y claves") {
  VERIFICAR(aplicarCurva(Curva::Lineal, 0.25f) == 0.25f);
  VERIFICAR(aplicarCurva(Curva::Paso, 0.99f) == 0.0f && aplicarCurva(Curva::Paso, 1.0f) == 1.0f);
  VERIFICAR(aplicarCurva(Curva::Salida, 0.5f) > 0.5f && aplicarCurva(Curva::Entrada, 0.5f) < 0.5f);
  VERIFICAR(aplicarCurva(Curva::Rebote, 0.7f) > 1.0f);  // se pasa y vuelve
  Pista p;
  p.nClaves = 2;
  p.claves[0].t = 100, p.claves[0].x = 0, p.claves[0].curva = Curva::Lineal;
  p.claves[1].t = 300, p.claves[1].x = 1, p.claves[1].hayColor = true, p.claves[1].color = 0x0000FF;
  p.claves[0].hayColor = true, p.claves[0].color = 0xFF0000;
  VERIFICAR(cuadroDePista(p, 0).x == 0);       // antes de la primera, la primera
  VERIFICAR(std::fabs(cuadroDePista(p, 200).x - 0.5f) < 1e-5f);
  VERIFICAR(cuadroDePista(p, 999).x == 1);     // después de la última, la última
  Cuadro q = cuadroDePista(p, 200);
  VERIFICAR(q.hayColor && ((q.color >> 16) & 255) > 120 && ((q.color >> 16) & 255) < 136 && (q.color & 255) > 120);
}

PRUEBA("animación: la config se valida") {
  Config c;
  VERIFICAR(cargarConfig(c, CONFIG_ANIMADA, std::strlen(CONFIG_ANIMADA)).ok());
  VERIFICAR(c.nAnimaciones == 2 && c.indiceAnimacion("giro") == 1);
  VERIFICAR(c.animaciones[0].sobre == c.indiceExpresion("neutral") && c.animaciones[0].repetir);
  VERIFICAR(c.lista[0].tipo == ElementoLista::Tipo::Animacion);
  VERIFICAR(c.reglas[0].entonces[0].tipo == Accion::Tipo::Animacion);
  const char* malas[] = {
      R"({"version":1,"animaciones":{"a":{"duracion":0,"pistas":[{"forma":{"forma":"circulo","x":0,"y":0,"r":0.1},"claves":[{"t":0}]}]}}})",
      R"({"version":1,"animaciones":{"a":{"duracion":500,"pistas":[{"forma":{"forma":"circulo","x":0,"y":0,"r":0.1},"claves":[{"t":300},{"t":100}]}]}}})",
      R"({"version":1,"animaciones":{"a":{"duracion":500,"pistas":[{"forma":{"forma":"circulo","x":0,"y":0,"r":0.1},"claves":[{"t":600}]}]}}})",
      R"({"version":1,"animaciones":{"a":{"duracion":500,"pistas":[{"forma":{"forma":"circulo","x":0,"y":0,"r":0.1},"claves":[{"t":0,"curva":"loca"}]}]}}})",
      R"({"version":1,"animaciones":{"a":{"duracion":500,"sobre":"nada","pistas":[{"forma":{"forma":"circulo","x":0,"y":0,"r":0.1},"claves":[{"t":0}]}]}}})",
      R"({"version":1,"lista":["animacion:nada"]})",
      R"({"version":1,"reglas":[{"si":"tacto","entonces":["animacion:nada"]}]})",
  };
  for (const char* m : malas) VERIFICAR(!cargarConfig(c, m, std::strlen(m)).ok());
}

PRUEBA("animación: se dibuja girada, con color y opacidad") {
  Config c;
  VERIFICAR(cargarConfig(c, CONFIG_ANIMADA, std::strlen(CONFIG_ANIMADA)).ok());
  Lienzo l;
  l.iniciar(g_px, 320, 172);
  Vista vi = Vista::para(320, 172);
  EstadoCara e;
  const Animacion& giro = c.animaciones[1];
  auto pintado = [&](int x, int y) { return g_px[y * 320 + x] != a565(paleta::PANTALLA); };
  // t = 0: barra horizontal roja (0.4 × 0.1 → 64 × 16 px en el centro).
  l.limpiar(paleta::PANTALLA);
  dibujarAnimacion(l, vi, giro, 0, e);
  VERIFICAR(pintado(160 + 25, 86) && !pintado(160, 86 - 25));
  VERIFICAR((de565(g_px[86 * 320 + 160]) >> 16) > 200);
  // A mitad (90°): vertical, entre rojo y azul.
  l.limpiar(paleta::PANTALLA);
  dibujarAnimacion(l, vi, giro, 400, e);
  VERIFICAR(!pintado(160 + 25, 86) && pintado(160, 86 - 25));
  // Opacidad 0: no dibuja nada.
  const Animacion& latido = c.animaciones[0];
  l.limpiar(paleta::PANTALLA);
  dibujarAnimacion(l, vi, latido, 0, e);
  int pintados = 0;
  for (int i = 0; i < 320 * 172; i++) pintados += g_px[i] != a565(paleta::PANTALLA);
  VERIFICAR(pintados == 0);
  l.limpiar(paleta::PANTALLA);
  dibujarAnimacion(l, vi, latido, 500, e);
  for (int i = 0; i < 320 * 172; i++) pintados += g_px[i] != a565(paleta::PANTALLA);
  VERIFICAR(pintados > 200);
}

PRUEBA("animación: en la lista, por regla y por comando") {
  arrancar();
  VERIFICAR(mandarConfig(CONFIG_ANIMADA) == Ok);
  correr(200);
  VERIFICAR(visible() == "animacion:latido");  // lo primero de la lista
  VERIFICAR(g_s.animacionVisible() == 0);
  // Una regla: el doble toque la gira una vez y vuelve.
  g_s.evento(Evento::DobleToque);
  correr(100);
  VERIFICAR(visible() == "animacion:giro");
  correr(900);
  VERIFICAR(visible() == "animacion:latido");
  // Por comando.
  ble(CarComando, conTexto(CmdReproducir, "giro"));
  VERIFICAR(g_ble.back().second == v({CmdReproducir | Respuesta, Ok}));
  correr(100);
  VERIFICAR(visible() == "animacion:giro");
  ble(CarComando, conTexto(CmdReproducir, "nada"));
  VERIFICAR(g_ble.back().second[1] == NoExiste);
  ble(CarComando, v({CmdReproducir}));  // parar
  correr(100);
  VERIFICAR(visible() == "animacion:latido");
  // Una expresión fija gana sobre la lista.
  ble(CarComando, conTexto(CmdExpresion, "sorpresa"));
  correr(400);
  VERIFICAR(visible() == "sorpresa");
  // Siguiente en la lista: la cara.
  ble(CarComando, v({CmdExpresion}));
  g_s.siguiente();
  correr(3000);  // (antes se mira en el espejo por la cara que le pusieron)
  VERIFICAR(visible() == "contento");
}

PRUEBA("nube: autorizada por el broker, responde por la nube") {
  arrancar();
  g_s.bleConexion(false);
  g_s.bleConexion(true);  // BLE sin autenticar
  g_ble.clear();
  ble(CarComando, conTexto(CmdExpresion, "enamorado"));
  VERIFICAR(g_ble.back().second[1] == NoAutorizado);
  nube(CarComando, conTexto(CmdExpresion, "enamorado"));
  VERIFICAR(!g_nube.empty() && g_nube.back().first == CarComando && g_nube.back().second == v({CmdExpresion | Respuesta, Ok}));
  VERIFICAR(g_ble.size() == 1);  // la respuesta no fue por BLE
  // La nube no puede cambiar ni el WiFi ni la nube.
  nube(CarComando, v({CmdRed, 1, 'x', 0}));
  VERIFICAR(g_nube.back().second[1] == NoAutorizado);
  nube(CarComando, v({CmdNube, 1, 'x', 1, 'y', 1, 'z'}));
  VERIFICAR(g_nube.back().second[1] == NoAutorizado);
  // La config por la nube, y pedirla.
  VERIFICAR(mandarConfig(CONFIG_ANIMADA, true) == Ok);
  VERIFICAR(g_s.crcConfig() == crc32(reinterpret_cast<const uint8_t*>(CONFIG_ANIMADA), std::strlen(CONFIG_ANIMADA)));
  g_nube.clear();
  nube(CarConfig, v({ConfigPedir}));
  std::string armada;
  for (auto& n : g_nube)
    if (n.second[0] == ConfigTrozo) armada.append(n.second.begin() + 5, n.second.end());
  VERIFICAR(armada == CONFIG_ANIMADA);
  VERIFICAR(g_nube.size() < 25);  // trozos grandes por la nube
  // El estado va por los dos lados, con el CRC de la config.
  g_ble.clear();
  g_nube.clear();
  g_s.conexiones(true, true);
  correr(40);
  VERIFICAR(!g_nube.empty() && g_nube.back().first == CarEstado && (g_nube.back().second[4] & 48) == 48);
  VERIFICAR(!g_ble.empty() && g_ble.back().first == CarEstado);
  const auto& e = g_nube.back().second;
  VERIFICAR(leer32(e.data() + e.size() - 8) == g_s.crcConfig());
  // Los segundos encendido no disparan notificaciones (con una expresión fija,
  // para que el motor de vida no cambie nada).
  nube(CarComando, conTexto(CmdExpresion, "neutral"));
  correr(500);
  g_nube.clear();
  correr(3000);
  VERIFICAR(g_nube.empty());
  uint8_t est[160];
  size_t n = g_s.estadoBle(est, sizeof est);
  VERIFICAR(leer32(est + n - 4) >= 6);
}

PRUEBA("red: WiFi y nube por BLE, avisan al perfil") {
  arrancar();
  std::vector<uint8_t> red{CmdRed, 4, 'C', 'a', 's', 'a', 3, 'a', 'b', 'c'};
  ble(CarComando, red);
  VERIFICAR(g_ble.back().second == v({CmdRed | Respuesta, Ok}) && g_redCambios == 1);
  char a[161], b[65], c[129];
  VERIFICAR(g_s.red(a, 33, b, 65) && std::string(a) == "Casa" && std::string(b) == "abc");
  ble(CarComando, v({CmdRed, 9, 'C'}));  // largo mentiroso
  VERIFICAR(g_ble.back().second[1] == Malformado);
  std::string url = "mqtt://10.0.0.2:1883";
  std::vector<uint8_t> nu{CmdNube, static_cast<uint8_t>(url.size())};
  nu.insert(nu.end(), url.begin(), url.end());
  nu.insert(nu.end(), {2, 'i', 'd', 3, 'k', 'e', 'y'});
  ble(CarComando, nu);
  VERIFICAR(g_ble.back().second[1] == Ok && g_redCambios == 2);
  VERIFICAR(g_s.nube(a, sizeof a, b, sizeof b, c, sizeof c) && url == a && std::string(b) == "id" && std::string(c) == "key");
  std::vector<uint8_t> http{CmdNube, 4, 'h', 't', 't', 'p', 1, 'i', 1, 'k'};
  ble(CarComando, http);
  VERIFICAR(g_ble.back().second[1] == Malformado);
  // Sobrevive a un reinicio.
  g_s.iniciar(hal(), g_px, perfil());
  VERIFICAR(g_s.red(a, 33, b, 65));
  uint8_t est[160];
  g_s.estadoBle(est, sizeof est);
  VERIFICAR(est[0] == 4 && (est[4] & 64));
  // Olvidar (también se puede desde la nube).
  nube(CarComando, v({CmdOlvidarRed}));
  VERIFICAR(g_nube.back().second[1] == Ok);
  VERIFICAR(!g_s.red(a, 33, b, 65) && !g_s.nube(a, sizeof a, b, sizeof b, c, sizeof c));
}

PRUEBA("imagen: pedirla de vuelta") {
  arrancar();
  // Una imagen chica no es del tamaño de la pantalla, pero se guarda como bytes crudos para la prueba.
  std::vector<uint8_t> datos(3000);
  for (size_t i = 0; i < datos.size(); i++) datos[i] = static_cast<uint8_t>(i * 13);
  g_flash["img4"] = datos;
  uint8_t meta[MAX_IMAGENES * 8] = {};
  escribir32(meta + 4 * 8, static_cast<uint32_t>(datos.size()));
  escribir32(meta + 4 * 8 + 4, crc32(datos.data(), datos.size()));
  g_flash["imagenes"] = std::vector<uint8_t>(meta, meta + sizeof meta);
  g_s.iniciar(hal(), g_px, perfil());
  g_s.bleConexion(true);
  g_s.confiar();
  g_ble.clear();
  ble(CarImagen, v({ImgPedir, 4}));
  VERIFICAR(g_ble.front().second == v({ImgPedir | Respuesta, Ok}));
  std::vector<uint8_t> armada;
  for (auto& n : g_ble)
    if (n.second[0] == ImgTrozo) {
      VERIFICAR(leer32(n.second.data() + 1) == armada.size());
      armada.insert(armada.end(), n.second.begin() + 5, n.second.end());
    }
  VERIFICAR(armada == datos);
  const auto& cierre = g_ble.back().second;
  VERIFICAR(cierre[0] == ImgCierre && cierre[1] == 4 && leer32(cierre.data() + 2) == datos.size() &&
            leer32(cierre.data() + 6) == crc32(datos.data(), datos.size()));
  ble(CarImagen, v({ImgPedir, 1}));
  VERIFICAR(g_ble.back().second == v({ImgPedir | Respuesta, NoExiste}));
}

namespace {
std::vector<uint8_t> manifiesto(const std::vector<uint8_t>& bin, const char* perfilOta) {
  ManifiestoOta m;
  m.tamano = static_cast<uint32_t>(bin.size());
  uint8_t suma = 0;
  for (uint8_t b : bin) suma = static_cast<uint8_t>(suma + b);
  m.sha256[0] = suma;
  copiarTexto(m.version, sizeof m.version, "0.2.1");
  copiarTexto(m.perfil, sizeof m.perfil, perfilOta);
  uint8_t b[200];
  size_t n = escribirManifiesto(m, b, sizeof b);
  return std::vector<uint8_t>(b, b + n);
}
}  // namespace

PRUEBA("ota: por BLE, por bloques, con progreso") {
  arrancar();
  std::vector<uint8_t> bin(5000);
  for (size_t i = 0; i < bin.size(); i++) bin[i] = static_cast<uint8_t>(i * 31 + 7);
  auto man = manifiesto(bin, "dev-147b");
  std::vector<uint8_t> ini{OtaInicio};
  ini.insert(ini.end(), man.begin(), man.end());
  ble(CarOta, ini);
  VERIFICAR(g_ble.back().second == v({OtaInicio | Respuesta, Ok}));
  VERIFICAR(g_s.actualizando());
  ble(CarOta, ini);  // ya hay una en curso
  VERIFICAR(g_ble.back().second[1] == Ocupado);
  int progresos = 0;
  for (size_t off = 0; off < bin.size(); off += 180) {
    std::vector<uint8_t> b{OtaBloque, 0, 0, 0, 0};
    escribir32(&b[1], static_cast<uint32_t>(off));
    b.insert(b.end(), bin.begin() + off, bin.begin() + std::min(bin.size(), off + 180));
    ble(CarOta, b);
  }
  for (auto& n : g_ble) progresos += n.second[0] == (OtaProgreso | Respuesta);
  VERIFICAR(progresos > 10);
  ble(CarOta, v({OtaFin}));
  VERIFICAR(g_ble.back().second == v({OtaFin | Respuesta, Ok}) && g_otaTerminada && g_otaBin == bin);
  VERIFICAR(!g_s.actualizando());
}

PRUEBA("ota: rechaza otro perfil, otra firma, bloques fuera de orden y hash distinto") {
  arrancar();
  std::vector<uint8_t> bin(500, 3);
  auto otro = manifiesto(bin, "broche-c3");
  std::vector<uint8_t> ini{OtaInicio};
  ini.insert(ini.end(), otro.begin(), otro.end());
  ble(CarOta, ini);
  VERIFICAR(g_ble.back().second[1] == PerfilDistinto);
  auto man = manifiesto(bin, "dev-147b");
  ini = {OtaInicio};
  ini.insert(ini.end(), man.begin(), man.end());
  g_otaFirmaOk = false;
  ble(CarOta, ini);
  VERIFICAR(g_ble.back().second[1] == FirmaInvalida && !g_s.actualizando());
  g_otaFirmaOk = true;
  ble(CarOta, ini);
  ble(CarOta, v({OtaBloque, 10, 0, 0, 0, 1, 2}));
  VERIFICAR(g_ble.back().second[1] == FueraDeOrden && !g_s.actualizando());
  // Hash distinto: llegan otros bytes.
  ble(CarOta, ini);
  std::vector<uint8_t> b{OtaBloque, 0, 0, 0, 0};
  b.insert(b.end(), 500, 4);
  ble(CarOta, b);
  ble(CarOta, v({OtaFin}));
  VERIFICAR(g_ble.back().second[1] == HashDistinto && !g_otaTerminada);
}

PRUEBA("ota: descarga por WiFi desde la nube") {
  arrancar();
  std::vector<uint8_t> bin(500, 3);
  auto man = manifiesto(bin, "dev-147b");
  std::string url = "http://10.0.0.2:8080/v1/firmware/1/binario?t=x";
  std::vector<uint8_t> d{OtaDescargar};
  d.insert(d.end(), man.begin(), man.end());
  d.push_back(static_cast<uint8_t>(url.size()));
  d.push_back(0);
  d.insert(d.end(), url.begin(), url.end());
  nube(CarOta, d);
  VERIFICAR(g_nube.back().second[1] == SinRed);
  g_s.conexiones(true, true);
  nube(CarOta, d);
  VERIFICAR(g_nube.back().second == v({OtaDescargar | Respuesta, Ok}) && g_otaUrl == url);
  VERIFICAR(g_s.actualizando());
  g_ble.clear();
  g_nube.clear();
  g_s.otaProgreso(Ok, 40);
  VERIFICAR(g_nube.back().second == v({OtaProgreso | Respuesta, Ok, 40}) && g_ble.back().second == g_nube.back().second);
  g_s.otaProgreso(FalloOta, 41, "se cortó");
  VERIFICAR(!g_s.actualizando());
  VERIFICAR(std::string(g_nube.back().second.begin() + 3, g_nube.back().second.end()) == "se cortó");
}

#include <dirent.h>
#include <fstream>
#include <sstream>

PRUEBA("animación: las de ejemplo (shared/formato/animaciones) las acepta el núcleo") {
  const std::string dir = std::string(RAIZ_REPO) + "/shared/formato/animaciones";
  std::string config = "{\"version\":1,\"animaciones\":{";
  DIR* d = opendir(dir.c_str());
  VERIFICAR(d != nullptr);
  int n = 0;
  while (dirent* e = d ? readdir(d) : nullptr) {
    std::string f = e->d_name;
    if (f.size() < 6 || f.substr(f.size() - 5) != ".json") continue;
    std::ifstream in(dir + "/" + f);
    std::stringstream s;
    s << in.rdbuf();
    // {"nombre": "x", ...resto} → "x": {...resto}
    std::string t = s.str();
    size_t a = t.find("\"nombre\"");
    size_t q1 = t.find('"', t.find(':', a) + 1), q2 = t.find('"', q1 + 1);
    std::string nombre = t.substr(q1 + 1, q2 - q1 - 1);
    size_t coma = t.find(',', q2);
    config += (n++ ? "," : "") + std::string("\"") + nombre + "\":{" + t.substr(coma + 1);
  }
  if (d) closedir(d);
  config += "}}";
  static Config c;
  ResultadoConfig r = cargarConfig(c, config.data(), config.size());
  if (!r.ok()) std::printf("    %s\n", r.detalle);
  VERIFICAR(r.ok() && n >= 3 && c.nAnimaciones == n);
}

PRUEBA("config: si la nueva es mala, vuelve la anterior (leída de la flash, sin copia en RAM)") {
  arrancar();
  VERIFICAR(mandarConfig(CONFIG_ANIMADA) == Ok);
  const uint32_t crc = g_s.crcConfig();
  VERIFICAR(mandarConfig(R"({"version":1,"lista":["animacion:nada"]})") == ConfigInvalida);
  VERIFICAR(g_s.config().nAnimaciones == 2 && g_s.crcConfig() == crc);
  // Y pedirla devuelve la buena.
  g_ble.clear();
  ble(CarConfig, v({ConfigPedir}));
  std::string armada;
  for (auto& n : g_ble)
    if (n.second[0] == ConfigTrozo) armada.append(n.second.begin() + 5, n.second.end());
  VERIFICAR(armada == CONFIG_ANIMADA);
  // Pedirla en medio de una transferencia la aborta y la manda igual.
  std::vector<uint8_t> ini{ConfigInicio, 10, 0, 0, 0, 0, 0, 0, 0};
  ble(CarConfig, ini);
  g_ble.clear();
  ble(CarConfig, v({ConfigPedir}));
  VERIFICAR(g_ble.back().second[0] == ConfigCierre);
  ble(CarConfig, v({ConfigFin}));
  VERIFICAR(g_ble.back().second[1] == SinInicio);
}
