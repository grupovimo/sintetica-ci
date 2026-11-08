// Los vectores compartidos (shared/protocolo/vectores.json) contra el núcleo:
// lo que codifican la app y el simulador es lo que entiende el bicho.
#include <cstring>
#include <fstream>
#include <map>
#include <sstream>
#include <string>
#include <vector>

#include "../api/sintetica_c.h"
#include "json.h"
#include "lienzo.h"
#include "motor_vida.h"
#include "protocolo.h"
#include "prueba.h"

using namespace sintetica;

namespace {
std::string g_texto;
TokenJson g_toks[12000];
Json g_j;
std::vector<std::vector<uint8_t>> g_notis;
std::vector<int> g_cars;
uint32_t g_t = 0;
std::map<std::string, std::vector<uint8_t>> g_guardado;

bool cargar() {
  if (!g_texto.empty()) return true;
  std::ifstream f(std::string(RAIZ_REPO) + "/shared/protocolo/vectores.json");
  std::stringstream s;
  s << f.rdbuf();
  g_texto = s.str();
  return g_j.parsear(g_texto.data(), g_texto.size(), g_toks, 12000) > 0;
}

std::string texto(int i) {
  static char b[20000];
  return g_j.texto(i, b, sizeof b) >= 0 ? std::string(b) : std::string();
}

std::vector<uint8_t> deHex(const std::string& h) {
  std::vector<uint8_t> v;
  std::istringstream s(h);
  std::string par;
  while (s >> par) v.push_back(static_cast<uint8_t>(std::stoul(par, nullptr, 16)));
  return v;
}

std::string aHex(const uint8_t* b, size_t n) {
  std::string s;
  char t[4];
  for (size_t i = 0; i < n; i++) std::snprintf(t, sizeof t, i ? " %02x" : "%02x", b[i]), s += t;
  return s;
}

// Un bicho en vista previa (confía en quien escribe) por la API C.
void bichoNuevo() {
  sintetica_hal h{};
  h.ahora_ms = [](void*) { return g_t; };
  h.notificar = [](void*, int c, const uint8_t* d, size_t n) {
    g_cars.push_back(c);
    g_notis.emplace_back(d, d + n);
  };
  h.identificador = "A1B2C3D4E5F6";
  h.guardar = [](void*, const char* k, const uint8_t* d, size_t n) {
    g_guardado[k].assign(d, d + n);
    return 1;
  };
  h.leer = [](void*, const char* k, uint8_t* d, size_t cap) {
    auto it = g_guardado.find(k);
    if (it == g_guardado.end() || it->second.size() > cap) return -1;
    std::copy(it->second.begin(), it->second.end(), d);
    return static_cast<int>(it->second.size());
  };
  g_guardado.clear();
  sintetica_iniciar(&h, 320, 172);
  sintetica_vista_previa();
  g_notis.clear();
  g_cars.clear();
}
}  // namespace

PRUEBA("vectores: los comandos los entiende el núcleo") {
  VERIFICAR(cargar());
  bichoNuevo();
  int arr = g_j.buscar(0, "comandos");
  for (int k = 0; k < g_j.cantidad(arr); k++) {
    int c = g_j.elemento(arr, k);
    std::string nombre = texto(g_j.buscar(c, "nombre"));
    auto b = deHex(texto(g_j.buscar(c, "hex")));
    if (nombre == "vincular" || nombre == "autenticar" || nombre == "desvincular" || nombre == "pedir config" ||
        nombre.rfind("imagen", 0) == 0 || nombre.rfind("ota", 0) == 0 || nombre == "reproducir saludo")
      continue;
    // v1.9: todavía no los entiende el núcleo (provisorio).
    if (nombre.rfind("toque", 0) == 0 || nombre.rfind("agenda", 0) == 0 || nombre.rfind("presencia", 0) == 0 ||
        nombre.rfind("habito", 0) == 0 || nombre.rfind("musica", 0) == 0 || nombre == "aviso pago" || nombre == "aviso github")
      continue;
    g_notis.clear();
    sintetica_ble_escribir(protocolo::CarComando, b.data(), b.size());
    VERIFICAR(!g_notis.empty() && g_notis.back()[1] == protocolo::Ok);
    if (g_notis.empty() || g_notis.back()[1] != protocolo::Ok) std::printf("    comando: %s\n", nombre.c_str());
  }
  // El nombre con tilde llegó entero.
  uint8_t est[160];
  size_t n = sintetica_ble_estado(est, sizeof est);
  VERIFICAR(std::string(reinterpret_cast<char*>(est), n).find("\xc3\x91" "and\xc3\xba") != std::string::npos);
}

PRUEBA("vectores: la config en bloques se aplica") {
  VERIFICAR(cargar());
  bichoNuevo();
  int cfg = g_j.buscar(0, "config");
  int esc = g_j.buscar(cfg, "escrituras");
  for (int k = 0; k < g_j.cantidad(esc); k++) {
    auto b = deHex(texto(g_j.elemento(esc, k)));
    sintetica_ble_escribir(protocolo::CarConfig, b.data(), b.size());
  }
  VERIFICAR(!g_notis.empty() && g_notis.back()[0] == (protocolo::ConfigFin | protocolo::Respuesta) &&
            g_notis.back()[1] == protocolo::Ok);
}

PRUEBA("vectores: el estado se codifica igual") {
  VERIFICAR(cargar());
  int arr = g_j.buscar(0, "estados");
  static const char* const modos[] = {"mascota", "reloj", "pomodoro", "clima", "notificaciones"};
  for (int k = 0; k < g_j.cantidad(arr); k++) {
    int e = g_j.elemento(arr, k);
    int c = g_j.buscar(e, "campos");
    protocolo::EstadoPublico p;
    int bat = g_j.buscar(c, "bateria");
    p.bateria = g_j.tipo(bat) == TipoJson::Nulo ? 255 : static_cast<uint8_t>(g_j.numero(bat, 0));
    p.humor = static_cast<int8_t>(g_j.numero(g_j.buscar(c, "humor"), 0) * 100 + (g_j.numero(g_j.buscar(c, "humor"), 0) < 0 ? -0.5f : 0.5f));
    std::string modo = texto(g_j.buscar(c, "modo"));
    for (int m = 0; m < 5; m++)
      if (modo == modos[m]) p.modo = static_cast<uint8_t>(m);
    p.dormido = g_j.booleano(g_j.buscar(c, "dormido"), false);
    p.fija = g_j.booleano(g_j.buscar(c, "fija"), false);
    p.emparejando = g_j.booleano(g_j.buscar(c, "emparejando"), false);
    p.autenticado = g_j.booleano(g_j.buscar(c, "autenticado"), false);
    std::string ex = texto(g_j.buscar(c, "expresion")), no = texto(g_j.buscar(c, "nombre")),
                se = texto(g_j.buscar(c, "serie")), id = texto(g_j.buscar(c, "id"));
    p.expresion = ex.c_str(), p.nombre = no.c_str(), p.serie = se.c_str(), p.id = id.c_str();
    p.ancho = static_cast<uint16_t>(g_j.numero(g_j.buscar(c, "ancho"), 0));
    p.alto = static_cast<uint16_t>(g_j.numero(g_j.buscar(c, "alto"), 0));
    p.redonda = g_j.booleano(g_j.buscar(c, "redonda"), false);
    p.wifi = g_j.booleano(g_j.buscar(c, "wifi"), false);
    p.nube = g_j.booleano(g_j.buscar(c, "nube"), false);
    p.redConfigurada = g_j.booleano(g_j.buscar(c, "redConfigurada"), false);
    p.actualizando = g_j.booleano(g_j.buscar(c, "actualizando"), false);
    std::string ve = texto(g_j.buscar(c, "version")), pe = texto(g_j.buscar(c, "perfil"));
    p.version = ve.c_str(), p.perfil = pe.c_str();
    p.crcConfig = static_cast<uint32_t>(std::stoul(texto(g_j.buscar(c, "crcConfigHex")), nullptr, 16));
    p.encendido = static_cast<uint32_t>(g_j.numero(g_j.buscar(c, "encendido"), 0));
    uint8_t b[160];
    size_t n = protocolo::codificarEstado(p, b, sizeof b);
    VERIFICAR(aHex(b, n) == texto(g_j.buscar(e, "hex")));
  }
}

PRUEBA("vectores: WiFi y nube se guardan y se leen") {
  VERIFICAR(cargar());
  bichoNuevo();
  int arr = g_j.buscar(0, "comandos");
  for (int k = 0; k < g_j.cantidad(arr); k++) {
    int c = g_j.elemento(arr, k);
    std::string nombre = texto(g_j.buscar(c, "nombre"));
    if (nombre != "red" && nombre != "nube") continue;
    auto b = deHex(texto(g_j.buscar(c, "hex")));
    sintetica_ble_escribir(protocolo::CarComando, b.data(), b.size());
    if (nombre == "red") {
      char ssid[33], clave[65];
      VERIFICAR(sintetica_red(ssid, sizeof ssid, clave, sizeof clave) == 1);
      VERIFICAR(std::string(ssid) == texto(g_j.buscar(c, "ssid")) && std::string(clave) == texto(g_j.buscar(c, "clave")));
    } else {
      char url[161], usuario[65], clave[129];
      VERIFICAR(sintetica_nube(url, sizeof url, usuario, sizeof usuario, clave, sizeof clave) == 1);
      VERIFICAR(std::string(url) == texto(g_j.buscar(c, "url")) && std::string(usuario) == texto(g_j.buscar(c, "usuario")) &&
                std::string(clave) == texto(g_j.buscar(c, "clave")));
    }
  }
  uint8_t est[160];
  size_t n = sintetica_ble_estado(est, sizeof est);
  VERIFICAR(n > 5 && (est[4] & 64));  // WiFi configurado
  uint8_t olvidar[] = {protocolo::CmdOlvidarRed};
  sintetica_ble_escribir(protocolo::CarComando, olvidar, 1);
  char a[161], b[65], c[129];
  VERIFICAR(sintetica_red(a, sizeof a, b, sizeof b) == 0 && sintetica_nube(a, sizeof a, b, sizeof b, c, sizeof c) == 0);
}

PRUEBA("vectores: OTA, el manifiesto y el texto que se firma") {
  VERIFICAR(cargar());
  int o = g_j.buscar(0, "ota");
  auto man = deHex(texto(g_j.buscar(o, "manifiesto")));
  ManifiestoOta m;
  VERIFICAR(protocolo::leerManifiesto(man.data(), man.size(), &m) == man.size());
  VERIFICAR(std::string(m.version) == texto(g_j.buscar(o, "version")) && std::string(m.perfil) == texto(g_j.buscar(o, "perfil")));
  VERIFICAR(aHex(m.sha256, 32) == texto(g_j.buscar(o, "sha256")));
  VERIFICAR(aHex(m.firma, 64) == texto(g_j.buscar(o, "firma")));
  char msg[200];
  size_t n = protocolo::mensajeOta(m, msg, sizeof msg);
  VERIFICAR(n > 0 && std::string(msg, n) == texto(g_j.buscar(o, "mensaje")));
  uint8_t otra[200];
  VERIFICAR(protocolo::escribirManifiesto(m, otra, sizeof otra) == man.size() && aHex(otra, man.size()) == aHex(man.data(), man.size()));
}

PRUEBA("vectores: crc32") {
  VERIFICAR(cargar());
  int arr = g_j.buscar(0, "crc32");
  for (int k = 0; k < g_j.cantidad(arr); k++) {
    int e = g_j.elemento(arr, k);
    std::string t = texto(g_j.buscar(e, "texto"));
    char crc[9];
    std::snprintf(crc, sizeof crc, "%08x", protocolo::crc32(reinterpret_cast<const uint8_t*>(t.data()), t.size()));
    VERIFICAR(texto(g_j.buscar(e, "crc")) == crc);
  }
}

PRUEBA("api C: vista previa al doble, sin QR") {
  bichoNuevo();
  sintetica_hal h{};
  h.ahora_ms = [](void*) { return g_t; };
  sintetica_iniciar(&h, 640, 344);
  sintetica_vista_previa();
  g_t += 50;
  sintetica_tick();
  VERIFICAR(sintetica_ancho() == 640 && sintetica_alto() == 344);
  VERIFICAR(std::string(sintetica_expresion_visible()) == "neutral");
  // Ojo izquierdo de 72 px de alto en la referencia: acá, de unos 144 (respira y flota un poco).
  int y0 = -1, y1 = -1;
  for (int y = 0; y < 344; y++) {
    uint32_t c = de565(sintetica_pixeles()[y * 640 + 220]);
    if (((c >> 16) & 0xFF) > 128) {
      if (y0 < 0) y0 = y;
      y1 = y;
    }
  }
  VERIFICAR(std::abs((y1 - y0 + 1) - 144) <= 8 && std::abs((y0 + y1) / 2 - 172) <= 10);
  std::vector<uint8_t> rgba(640 * 344 * 4);
  sintetica_pixeles_rgba(rgba.data());
  VERIFICAR(rgba[(172 * 640 + 220) * 4] == 0xFF && rgba[3] == 255);
  VERIFICAR(sintetica_cantidad_expresiones() == 16);
}

PRUEBA("vectores: la memoria del motor de vida se decodifica igual") {
  VERIFICAR(cargar());
  const int m = g_j.buscar(0, "memoria");
  auto b = deHex(texto(g_j.buscar(m, "hex")));
  Memoria mem;
  VERIFICAR(b.size() == LARGO_MEMORIA && decodificarMemoria(b.data(), b.size(), &mem));
  const int c = g_j.buscar(m, "campos");
  VERIFICAR(mem.dias == g_j.numero(g_j.buscar(c, "dias"), -1));
  VERIFICAR(mem.caricias == g_j.numero(g_j.buscar(c, "caricias"), -1));
  VERIFICAR(mem.toques == g_j.numero(g_j.buscar(c, "toques"), -1));
  VERIFICAR(mem.hitos == g_j.numero(g_j.buscar(c, "hitos"), -1));
  VERIFICAR(mem.horas[9] == g_j.numero(g_j.buscar(c, "hora9"), -1));
  VERIFICAR(mem.estado[2] == g_j.numero(g_j.buscar(c, "afecto"), -1));
  // Y vuelve a dar los mismos bytes.
  uint8_t otra[LARGO_MEMORIA];
  codificarMemoria(mem, otra, sizeof otra);
  VERIFICAR(std::memcmp(otra, b.data(), LARGO_MEMORIA) == 0);
}
