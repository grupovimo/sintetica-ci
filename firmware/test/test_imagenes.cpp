// Imágenes: el decodificador QOI y la característica Imagen del protocolo v1.2.
#include <cstring>
#include <fstream>
#include <map>
#include <sstream>
#include <string>
#include <vector>

#include "json.h"
#include "prueba.h"
#include "qoi.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
uint32_t g_t = 0;
std::map<std::string, std::vector<uint8_t>> g_mem;     // claves chicas
std::map<std::string, std::vector<uint8_t>> g_archivos;  // archivos por trozos
std::vector<std::vector<uint8_t>> g_notis;
std::vector<uint16_t> g_pantalla;
int g_presentadas = 0;
uint16_t g_franja[240 * 40];
Sistema g_s;

std::vector<uint8_t> deHex(const std::string& h) {
  std::vector<uint8_t> v;
  std::istringstream s(h);
  std::string par;
  while (s >> par) v.push_back(static_cast<uint8_t>(std::stoul(par, nullptr, 16)));
  return v;
}

// Codificador QOI mínimo (solo RGB y rachas) para armar imágenes de prueba.
std::vector<uint8_t> qoi(int w, int h, uint32_t (*color)(int x, int y)) {
  std::vector<uint8_t> o = {'q', 'o', 'i', 'f'};
  auto be = [&](uint32_t v) { for (int k = 3; k >= 0; k--) o.push_back(static_cast<uint8_t>(v >> (8 * k))); };
  be(static_cast<uint32_t>(w));
  be(static_cast<uint32_t>(h));
  o.push_back(3);
  o.push_back(0);
  uint32_t prev = 0;
  int racha = 0;
  for (int i = 0; i < w * h; i++) {
    uint32_t c = color(i % w, i / w);
    if (c == prev) {
      racha++;
      if (racha == 62 || i == w * h - 1) o.push_back(static_cast<uint8_t>(0xC0 | (racha - 1))), racha = 0;
      continue;
    }
    if (racha) o.push_back(static_cast<uint8_t>(0xC0 | (racha - 1))), racha = 0;
    o.push_back(0xFE), o.push_back(c >> 16), o.push_back(c >> 8), o.push_back(c);
    prev = c;
  }
  for (int i = 0; i < 7; i++) o.push_back(0);
  o.push_back(1);
  return o;
}

Hal hal() {
  Hal h;
  h.ahoraMs = [] { return g_t; };
  h.semilla = [] { return 99u; };
  h.guardar = [](const char* k, const uint8_t* d, size_t n) { g_mem[k].assign(d, d + n); return true; };
  h.leer = [](const char* k, uint8_t* d, size_t cap) {
    auto it = g_mem.find(k);
    if (it == g_mem.end() || it->second.size() > cap) return -1;
    std::memcpy(d, it->second.data(), it->second.size());
    return static_cast<int>(it->second.size());
  };
  h.escribirTrozo = [](const char* k, uint32_t off, const uint8_t* d, size_t n) {
    auto& a = g_archivos[k];
    if (off == 0) a.clear();
    if (off != a.size()) return false;
    a.insert(a.end(), d, d + n);
    return true;
  };
  h.leerTrozo = [](const char* k, uint32_t off, uint8_t* d, size_t n) {
    auto it = g_archivos.find(k);
    if (it == g_archivos.end()) return -1;
    if (off >= it->second.size()) return 0;
    size_t m = std::min(n, it->second.size() - off);
    std::memcpy(d, it->second.data() + off, m);
    return static_cast<int>(m);
  };
  h.borrar = [](const char* k) { g_archivos.erase(k); return true; };
  h.notificar = [](uint8_t, const uint8_t* d, size_t n) { g_notis.emplace_back(d, d + n); };
  h.presentarFranja = [](const uint16_t* px, int ancho, int y0, int filas) {
    g_presentadas++;
    std::memcpy(&g_pantalla[y0 * ancho], px, sizeof(uint16_t) * ancho * filas);
  };
  return h;
}

void arrancar(bool borrar = true) {
  if (borrar) g_mem.clear(), g_archivos.clear();
  g_t = 1000;
  g_pantalla.assign(240 * 240, 0);
  Perfil p;
  p.ancho = 240, p.alto = 240, p.filasFranja = 40, p.redonda = true, p.boton = Perfil::Boton::Broche;
  g_s.iniciar(hal(), g_franja, p);
  g_s.modoVistaPrevia();  // confía en quien escribe
  g_notis.clear();
}
void correr(uint32_t ms) {
  for (uint32_t i = 0; i < ms; i += 20) g_t += 20, g_s.tick();
}
void escribir(const std::vector<uint8_t>& b) { g_s.bleEscribir(CarImagen, b.data(), b.size()); }
std::vector<uint8_t> u32(uint32_t v) { return {uint8_t(v), uint8_t(v >> 8), uint8_t(v >> 16), uint8_t(v >> 24)}; }

uint8_t subir(int espacio, const std::vector<uint8_t>& datos, uint32_t crc, size_t bloque = 175) {
  std::vector<uint8_t> ini = {ImgInicio, uint8_t(espacio)};
  for (uint8_t x : u32(static_cast<uint32_t>(datos.size()))) ini.push_back(x);
  for (uint8_t x : u32(crc)) ini.push_back(x);
  g_notis.clear();
  escribir(ini);
  for (size_t off = 0; off < datos.size(); off += bloque) {
    std::vector<uint8_t> b = {ImgBloque};
    for (uint8_t x : u32(static_cast<uint32_t>(off))) b.push_back(x);
    b.insert(b.end(), datos.begin() + off, datos.begin() + std::min(datos.size(), off + bloque));
    escribir(b);
  }
  escribir({ImgFin});
  return g_notis.back()[1];
}

uint32_t naranja(int, int) { return 0xF5501A; }
uint32_t degrade(int x, int y) {
  return (static_cast<uint32_t>(x & ~7) << 16) | (static_cast<uint32_t>(y & ~7) << 8) | 0x40;
}
}  // namespace

PRUEBA("qoi: el vector compartido decodifica igual que en JS") {
  std::ifstream f(std::string(RAIZ_REPO) + "/shared/protocolo/vectores.json");
  std::stringstream ss;
  ss << f.rdbuf();
  std::string texto = ss.str();
  static TokenJson toks[4000];
  Json j;
  VERIFICAR(j.parsear(texto.data(), texto.size(), toks, 4000) > 0);
  int q = j.buscar(0, "qoi");
  char hex[8000];
  j.texto(j.buscar(q, "hex"), hex, sizeof hex);
  static std::vector<uint8_t> datos;
  datos = deHex(hex);
  DecodificadorQoi d;
  auto leer = [](void*, uint32_t off, uint8_t* dst, size_t n) -> int {
    if (off >= datos.size()) return 0;
    size_t m = std::min<size_t>(n, 5);  // de a poquito, para probar el buffer
    m = std::min(m, datos.size() - off);
    std::memcpy(dst, datos.data() + off, m);
    return static_cast<int>(m);
  };
  VERIFICAR(d.empezar(leer, nullptr));
  VERIFICAR(d.ancho() == 12 && d.alto() == 7);
  int esperado = j.buscar(q, "rgb565");
  uint16_t fila[12];
  bool igual = true;
  for (int y = 0; y < 7; y++) {
    VERIFICAR(d.siguienteFila(fila));
    for (int x = 0; x < 12; x++)
      igual = igual && fila[x] == static_cast<uint16_t>(j.numero(j.elemento(esperado, y * 12 + x), -1));
  }
  VERIFICAR(igual);
  VERIFICAR(!d.siguienteFila(fila));
}

PRUEBA("imagen: se sube, se lista, se muestra y se borra") {
  arrancar();
  auto img = qoi(240, 240, degrade);
  VERIFICAR(img.size() < MAX_IMAGEN_BYTES);
  VERIFICAR(subir(3, img, crc32(img.data(), img.size())) == Ok);
  VERIFICAR(g_s.largoImagen(3) == img.size());
  g_notis.clear();
  escribir({ImgListar});
  VERIFICAR(g_notis.size() == 1 && g_notis[0][2] == (1 << 3) && g_notis[0][3] == 3);

  // Una lista con la imagen: se ve, con los píxeles de la imagen.
  const char* cfg = R"({"version":1,"lista":["imagen:3","cara:neutral"]})";
  g_mem["config"].assign(cfg, cfg + std::strlen(cfg));
  arrancar(false);
  correr(40);
  VERIFICAR(g_s.imagenVisible() == 3);
  VERIFICAR(g_pantalla[100 * 240 + 30] == static_cast<uint16_t>(((24 & 0xF8) << 8) | ((96 & 0xFC) << 3) | (0x40 >> 3)));
  // Ya dibujada, no se vuelve a presentar en cada cuadro.
  g_presentadas = 0;
  correr(1000);
  VERIFICAR(g_presentadas == 0);

  // Borrarla: vuelve la cara.
  escribir({ImgBorrar, 3});
  VERIFICAR(g_notis.back()[1] == Ok);
  correr(40);
  VERIFICAR(g_s.imagenVisible() == -1);
  VERIFICAR(g_archivos.find("img3") == g_archivos.end());
}

PRUEBA("imagen: cada 1 a 3 minutos vuelve la cara un par de segundos") {
  arrancar();
  auto img = qoi(240, 240, naranja);
  VERIFICAR(subir(0, img, crc32(img.data(), img.size())) == Ok);
  const char* cfg = R"({"version":1,"lista":["imagen:0"]})";
  g_mem["config"].assign(cfg, cfg + std::strlen(cfg));
  arrancar(false);
  int vueltas = 0;
  bool antes = true;
  uint32_t cara = 0;
  for (int i = 0; i < 10 * 60 * 50; i++) {  // 10 minutos
    g_t += 20;
    g_s.tick();
    const bool imagen = g_s.imagenVisible() == 0;
    if (!imagen) cara += 20;
    if (!imagen && antes) vueltas++;
    antes = imagen;
  }
  VERIFICAR(vueltas >= 3 && vueltas <= 10);
  VERIFICAR(cara >= vueltas * 2400u && cara <= vueltas * 2600u);
}

PRUEBA("galeria: solo las imágenes, rotando con fundido; un toque pasa a la siguiente") {
  arrancar();
  auto a = qoi(240, 240, naranja), b = qoi(240, 240, degrade);
  VERIFICAR(subir(1, a, crc32(a.data(), a.size())) == Ok);
  VERIFICAR(subir(4, b, crc32(b.data(), b.size())) == Ok);
  const char* cfg = R"({"version":1,"modo":"galeria","galeria":{"segundos":10}})";
  g_mem["config"].assign(cfg, cfg + std::strlen(cfg));
  // La mascota (sin botón de broche): el toque es Tacto.
  g_t = 1000;
  Perfil p;
  p.ancho = 240, p.alto = 240, p.filasFranja = 40, p.redonda = true;
  g_s.iniciar(hal(), g_franja, p);
  g_s.modoVistaPrevia();
  correr(40);
  VERIFICAR(g_s.imagenVisible() == 1);
  // 9 minutos: nunca vuelve la cara salvo en el fundido, y pasan las dos.
  bool vio4 = false;
  uint32_t cara = 0;
  for (int i = 0; i < 9 * 60 * 50; i++) {
    g_t += 20;
    g_s.tick();
    if (g_s.imagenVisible() == 4) vio4 = true;
    if (g_s.imagenVisible() < 0) cara += 20;
  }
  VERIFICAR(vio4);
  VERIFICAR(cara == 0);  // en el fundido la cara se ve debajo, pero la imagen sigue puesta
  // Un toque pasa a la otra.
  const int antes = g_s.imagenVisible();
  g_s.tacto(true);
  correr(60);
  g_s.tacto(false);
  correr(2000);
  VERIFICAR(g_s.imagenVisible() >= 0 && g_s.imagenVisible() != antes);
  // Sin imágenes, la cara de siempre.
  escribir({ImgBorrar, 1});
  escribir({ImgBorrar, 4});
  correr(2000);
  VERIFICAR(g_s.imagenVisible() == -1);
  // Los segundos se validan.
  const char* malo = R"({"version":1,"modo":"galeria","galeria":{"segundos":2}})";
  static Config c;
  VERIFICAR(!aplicarConfig(c, malo, std::strlen(malo)).ok());
}

PRUEBA("imagen: errores (sin autenticar, CRC, tamaño, orden, espacio)") {
  arrancar();
  auto img = qoi(240, 240, naranja);
  VERIFICAR(subir(1, img, 12345) == CrcDistinto);
  VERIFICAR(g_s.largoImagen(1) == 0 && g_archivos.find("img1") == g_archivos.end());
  auto chica = qoi(100, 100, naranja);  // no es del tamaño de la pantalla
  VERIFICAR(subir(1, chica, crc32(chica.data(), chica.size())) == ImagenInvalida);
  std::vector<uint8_t> basura(500, 7);
  VERIFICAR(subir(1, basura, crc32(basura.data(), basura.size())) == ImagenInvalida);
  // Un bloque fuera de orden aborta.
  escribir({ImgInicio, 1, 100, 0, 0, 0, 0, 0, 0, 0});
  escribir({ImgBloque, 50, 0, 0, 0, 1, 2, 3});
  VERIFICAR(g_notis.back()[0] == (ImgBloque | Respuesta) && g_notis.back()[1] == FueraDeOrden);
  escribir({ImgFin});
  VERIFICAR(g_notis.back()[1] == SinInicio);
  escribir({ImgInicio, 20, 100, 0, 0, 0, 0, 0, 0, 0});  // 8 a 19 son los sprites
  VERIFICAR(g_notis.back()[1] == Malformado);
  escribir({ImgInicio, 1, 0, 0, 0x10, 0, 0, 0, 0, 0});  // 1 MB
  VERIFICAR(g_notis.back()[1] == MuyGrande);
  // El límite es el peor caso de QOI para esta pantalla: 240 × 240 × 4 + 22.
  std::vector<uint8_t> justo = {ImgInicio, 1};
  for (uint8_t x : u32(limiteImagen(240, 240))) justo.push_back(x);
  for (int i = 0; i < 4; i++) justo.push_back(0);
  escribir(justo);
  VERIFICAR(g_notis.back()[1] == Ok);
  justo[2] += 1;
  escribir(justo);
  VERIFICAR(g_notis.back()[1] == MuyGrande);
  // Sin lugar: se rechaza de entrada (lo que ocupa ese espacio cuenta como libre).
  static int64_t s_libre = 0;
  Hal conPoco = hal();
  conPoco.espacioLibre = [] { return s_libre; };
  Perfil chico;
  chico.ancho = 240, chico.alto = 240, chico.filasFranja = 40, chico.redonda = true, chico.boton = Perfil::Boton::Broche;
  g_archivos.clear(), g_mem.clear();
  g_s.iniciar(conPoco, g_franja, chico);
  g_s.modoVistaPrevia();
  s_libre = 1000000;
  auto entra = qoi(240, 240, degrade);
  VERIFICAR(subir(2, entra, crc32(entra.data(), entra.size())) == Ok);
  s_libre = 1000;
  escribir({ImgInicio, 3, 0, 0x10, 0, 0, 0, 0, 0, 0});  // 4 KB más el margen: no entra
  VERIFICAR(g_notis.back()[1] == MuyGrande);
  s_libre = 16 * 1024;  // reemplazar la misma imagen sí entra: la vieja se libera
  VERIFICAR(subir(2, entra, crc32(entra.data(), entra.size())) == Ok);
  arrancar();
  // Sin autenticar.
  Perfil p;
  p.ancho = 240, p.alto = 240, p.filasFranja = 40, p.redonda = true;
  g_s.iniciar(hal(), g_franja, p);
  g_s.bleConexion(true);
  g_notis.clear();
  escribir({ImgListar});
  VERIFICAR(g_notis.back()[1] == NoAutorizado);
}

PRUEBA("imagen: el estado trae el tamaño y la forma de la pantalla") {
  arrancar();
  uint8_t b[160];
  size_t n = g_s.estadoBle(b, sizeof b);
  VERIFICAR(b[0] == 4);
  // Después de la pantalla: [n][versión][n][perfil][u32 crc].
  size_t i = 5;
  for (int k = 0; k < 4; k++) i += 1 + b[i];
  VERIFICAR(i + 9 < n && b[i] == 240 && b[i + 1] == 0 && b[i + 2] == 240 && b[i + 3] == 0 && b[i + 4] == 1);
}

// --- Imágenes animadas ("sani") ---

namespace {
uint32_t azul(int, int) { return 0x1A50F5; }
// Arma una animación con estos cuadros (QOI) y duraciones.
std::vector<uint8_t> sani(int w, int h, const std::vector<std::vector<uint8_t>>& cuadros, const std::vector<uint16_t>& ms) {
  std::vector<uint8_t> o = {'s', 'a', 'n', 'i', 1, uint8_t(cuadros.size()), uint8_t(w), uint8_t(w >> 8), uint8_t(h), uint8_t(h >> 8)};
  uint32_t off = static_cast<uint32_t>(10 + 10 * cuadros.size());
  for (size_t i = 0; i < cuadros.size(); i++) {
    o.push_back(uint8_t(ms[i])), o.push_back(uint8_t(ms[i] >> 8));
    for (uint8_t x : u32(off)) o.push_back(x);
    for (uint8_t x : u32(static_cast<uint32_t>(cuadros[i].size()))) o.push_back(x);
    off += static_cast<uint32_t>(cuadros[i].size());
  }
  for (const auto& c : cuadros) o.insert(o.end(), c.begin(), c.end());
  return o;
}
}  // namespace

PRUEBA("imagen animada: pasa de cuadro con la duración de cada uno, y vuelve a empezar") {
  arrancar();
  auto anim = sani(240, 240, {qoi(240, 240, naranja), qoi(240, 240, azul)}, {100, 300});
  VERIFICAR(subir(2, anim, crc32(anim.data(), anim.size())) == Ok);
  VERIFICAR(g_s.largoImagen(2) == anim.size());
  const char* cfg = R"({"version":1,"lista":["imagen:2"]})";
  g_mem["config"].assign(cfg, cfg + std::strlen(cfg));
  arrancar(false);
  correr(40);
  VERIFICAR(g_s.imagenVisible() == 2);
  VERIFICAR(g_pantalla[120 * 240 + 120] == a565(0xF5501A));
  correr(100);  // pasaron los 100 ms del primero
  VERIFICAR(g_pantalla[120 * 240 + 120] == a565(0x1A50F5));
  correr(200);  // el segundo dura 300: sigue
  VERIFICAR(g_pantalla[120 * 240 + 120] == a565(0x1A50F5));
  correr(120);  // y vuelve al primero
  VERIFICAR(g_pantalla[120 * 240 + 120] == a565(0xF5501A));
}

PRUEBA("imagen animada: una sola vez por cuadro (no redibuja de más)") {
  arrancar();
  auto anim = sani(240, 240, {qoi(240, 240, naranja), qoi(240, 240, azul)}, {500, 500});
  VERIFICAR(subir(0, anim, crc32(anim.data(), anim.size())) == Ok);
  const char* cfg = R"({"version":1,"lista":["imagen:0"]})";
  g_mem["config"].assign(cfg, cfg + std::strlen(cfg));
  arrancar(false);
  correr(40);
  g_presentadas = 0;
  correr(2000);  // 4 cuadros nuevos, de 6 franjas cada uno
  VERIFICAR(g_presentadas == 4 * 6);
}

PRUEBA("imagen animada: se rechaza si está mal armada") {
  arrancar();
  auto bien = qoi(240, 240, naranja);
  auto chica = qoi(100, 100, naranja);
  auto rechaza = [](const std::vector<uint8_t>& a) { return subir(1, a, crc32(a.data(), a.size())) == ImagenInvalida; };
  VERIFICAR(rechaza(sani(240, 240, {bien, chica}, {100, 100})));  // un cuadro de otro tamaño
  VERIFICAR(rechaza(sani(100, 100, {chica}, {100})));             // otro tamaño de pantalla
  VERIFICAR(rechaza(sani(240, 240, {bien}, {5})));                 // menos de 20 ms
  auto corrida = sani(240, 240, {bien, bien}, {100, 100});
  corrida[10 + 10 + 2] += 50;  // el segundo cuadro apunta afuera del archivo
  VERIFICAR(rechaza(corrida));
  auto version = sani(240, 240, {bien}, {100});
  version[4] = 2;
  VERIFICAR(rechaza(version));
  auto ninguno = sani(240, 240, {}, {});  // ni llega a empezar: es más chica que cualquier imagen
  VERIFICAR(subir(1, ninguno, crc32(ninguno.data(), ninguno.size())) != Ok);
  VERIFICAR(g_s.largoImagen(1) == 0 && g_archivos.find("img1") == g_archivos.end());
}
