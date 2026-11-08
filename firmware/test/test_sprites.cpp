// Caras de imagen: el formato, los sprites (caché y dibujo), el motor y el
// sistema (los espacios 8 a 19 de la característica Imagen y el fundido).
#include <cstdio>
#include <cstring>
#include <map>
#include <string>
#include <vector>

#include "prueba.h"
#include "sintetica_c.h"
#include "sistema.h"
#include "sprites.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {

// Un QOI de 4 canales (todo con QOI_OP_RGBA y rachas), para armar sprites de prueba.
std::vector<uint8_t> qoiRgba(int w, int h, uint32_t (*color)(int x, int y, int w, int h)) {
  std::vector<uint8_t> o = {'q', 'o', 'i', 'f'};
  auto be = [&](uint32_t v) {
    for (int k = 3; k >= 0; k--) o.push_back(static_cast<uint8_t>(v >> (8 * k)));
  };
  be(static_cast<uint32_t>(w));
  be(static_cast<uint32_t>(h));
  o.push_back(4);
  o.push_back(0);
  uint32_t prev = 0x000000FF;  // r g b a: el "anterior" del QOI (0, 0, 0, 255)
  int racha = 0;
  for (int i = 0; i < w * h; i++) {
    const uint32_t c = color(i % w, i / w, w, h);  // 0xRRGGBBAA
    if (c == prev) {
      racha++;
      if (racha == 62 || i == w * h - 1) o.push_back(static_cast<uint8_t>(0xC0 | (racha - 1))), racha = 0;
      continue;
    }
    if (racha) o.push_back(static_cast<uint8_t>(0xC0 | (racha - 1))), racha = 0;
    o.push_back(0xFF), o.push_back(c >> 24), o.push_back(c >> 16), o.push_back(c >> 8), o.push_back(c);
    prev = c;
  }
  for (int i = 0; i < 7; i++) o.push_back(0);
  o.push_back(1);
  return o;
}

// Un ojo cian redondo (con transparencia afuera), uno cerrado magenta (una
// rayita) y una boca verde.
uint32_t ojoCian(int x, int y, int w, int h) {
  const float dx = (x + 0.5f - w / 2.0f) / (w / 2.0f), dy = (y + 0.5f - h / 2.0f) / (h / 2.0f);
  return dx * dx + dy * dy < 0.9f ? 0x00FFFFFFu : 0x00000000u;
}
uint32_t cerradoMagenta(int, int y, int, int h) { return (y > h / 3 && y < 2 * h / 3) ? 0xFF00FFFFu : 0u; }
uint32_t bocaVerde(int, int, int, int) { return 0x00FF00FFu; }

const char* CONFIG_ANIME =
    "{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0,\"ojo_cerrado\":1,\"boca\":2},\"tamano\":0.4,"
    "\"boca_y\":-0.35,\"sep\":0.3}}}";
const char* CONFIG_SIN_CERRADO =
    "{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0},\"tamano\":0.4}}}";

// Cuántos píxeles de un color (cerca) hay en un RGBA.
int contar(const std::vector<uint8_t>& px, int r, int g, int b) {
  int n = 0;
  for (size_t i = 0; i < px.size(); i += 4)
    if (std::abs(px[i] - r) < 90 && std::abs(px[i + 1] - g) < 90 && std::abs(px[i + 2] - b) < 90) n++;
  return n;
}

std::vector<uint8_t> dibujar(const char* cfg, const char* nombre, float cierre, int* error = nullptr) {
  std::vector<uint8_t> px(320 * 172 * 4);
  char detalle[128];
  const int e = sintetica_dibujar_expresion_cierre(cfg, std::strlen(cfg), nombre, 320, 172, 0, 0.3f, cierre, px.data(),
                                                   detalle, sizeof detalle);
  if (error) *error = e;
  if (e != 0) std::printf("    (dibujar: %d %s)\n", e, detalle);
  return px;
}

void ponerSueltos(bool cerrado = true) {
  const auto ojo = qoiRgba(64, 64, ojoCian), cer = qoiRgba(64, 20, cerradoMagenta), boca = qoiRgba(40, 12, bocaVerde);
  VERIFICAR(sintetica_sprite_suelto(0, ojo.data(), ojo.size()) == 0);
  VERIFICAR(sintetica_sprite_suelto(1, cerrado ? cer.data() : nullptr, cerrado ? cer.size() : 0) == 0);
  VERIFICAR(sintetica_sprite_suelto(2, boca.data(), boca.size()) == 0);
}

ResultadoConfig cargar(const char* s) {
  static Config c;
  return cargarConfig(c, s, std::strlen(s));
}

}  // namespace

PRUEBA("sprites: una expresión de imagen se lee y valida igual en todos lados") {
  static Config c;
  VERIFICAR(cargarConfig(c, CONFIG_ANIME, std::strlen(CONFIG_ANIME)).ok());
  const int i = c.vida.indiceCara("anime");
  VERIFICAR(i >= 0 && c.vida.caras[i].imagen == 0 && c.vida.nImagenes == 1);
  const CaraImagen& im = c.vida.imagenes[0];
  VERIFICAR(im.sprite[0] == 0 && im.sprite[1] == 1 && im.sprite[4] == 2 && im.sprite[2] == -1);
  VERIFICAR(im.tamano == 4000 && im.bocaY == -3500 && im.espejar);
  VERIFICAR(c.vida.reposo == -1);
  // Errores: sin ojo, sprite fuera de rango, parte desconocida, medidas sin sprites o fuera de rango.
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"boca\":2}}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":12}}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":1.5}}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":0,\"nariz\":1}}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"tamano\":0.4,\"gx\":0.1}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":0},\"tamano\":2}}}").ok());
  VERIFICAR(!cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":0},\"espejar_der\":1}}}").ok());
  // Con parámetros de la cara (se aplican como en cualquier expresión) y extiende.
  VERIFICAR(cargar("{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":0},\"gx\":0.1,\"movimiento\":\"rebote\","
                   "\"ojo\":{\"lt\":0.2}},\"b\":{\"extiende\":\"a\",\"ey\":0.05}}}")
               .ok());
  {
    static Config d;
    const char* s = "{\"version\":2,\"expresiones\":{\"a\":{\"sprites\":{\"ojo\":3}},\"b\":{\"extiende\":\"a\",\"tamano\":0.3}}}";
    VERIFICAR(cargarConfig(d, s, std::strlen(s)).ok());
    const int b = d.vida.indiceCara("b");
    VERIFICAR(b >= 0 && d.vida.caras[b].imagen >= 0 && d.vida.imagenes[d.vida.caras[b].imagen].sprite[0] == 3 &&
              d.vida.imagenes[d.vida.caras[b].imagen].tamano == 3000);
  }
  // Hasta 8 caras de imagen.
  std::string muchas = "{\"version\":2,\"expresiones\":{";
  for (int k = 0; k < 9; k++) muchas += (k ? "," : "") + std::string("\"i") + char('a' + k) + "\":{\"sprites\":{\"ojo\":0}}";
  muchas += "}}";
  VERIFICAR(cargar(muchas.c_str()).error == ErrorConfig::Demasiadas);
  // La cara de siempre: tiene que existir.
  VERIFICAR(!cargar("{\"version\":2,\"cara\":{\"expresion\":\"no-existe\"}}").ok());
  const char* conReposo =
      "{\"version\":2,\"cara\":{\"expresion\":\"anime\"},\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0}}}}";
  VERIFICAR(cargarConfig(c, conReposo, std::strlen(conReposo)).ok() && c.vida.reposo == c.vida.indiceCara("anime"));
  VERIFICAR(cargar("{\"version\":2,\"cara\":{\"expresion\":\"contento\"}}").ok());
  // El mismo validador que usan la API (cgo) y la web (wasm).
  char detalle[96];
  VERIFICAR(sintetica_validar_config(CONFIG_ANIME, std::strlen(CONFIG_ANIME), detalle, sizeof detalle) == 0);
}

PRUEBA("sprites: dibuja la cara de imagen con y sin ojo_cerrado, y parpadea") {
  ponerSueltos(true);
  int e = -9;
  const auto abierta = dibujar(CONFIG_ANIME, "anime", 0, &e);
  VERIFICAR(e == 0);
  const int cian = contar(abierta, 0, 255, 255), verde = contar(abierta, 0, 255, 0);
  // Dos ojos de 0,4 unidades (~63 px) con un 70 % de círculo: unos 2 × 2700 píxeles.
  VERIFICAR(cian > 3500 && cian < 9000);
  VERIFICAR(verde > 200);
  // Sin los ojos naranjas de la SDF.
  VERIFICAR(contar(abierta, 255, 106, 51) < 50);
  // Parpadeando: el ojo cerrado (magenta) y casi nada del abierto.
  const auto cerrada = dibujar(CONFIG_ANIME, "anime", 1);
  VERIFICAR(contar(cerrada, 255, 0, 255) > 500);
  VERIFICAR(contar(cerrada, 0, 255, 255) < cian / 10);
  // Sin ojo_cerrado: el abierto se aplasta.
  ponerSueltos(false);
  const auto aplastada = dibujar(CONFIG_SIN_CERRADO, "anime", 1);
  const int cianAplastado = contar(aplastada, 0, 255, 255);
  VERIFICAR(cianAplastado > 0 && cianAplastado < cian / 8);
  VERIFICAR(contar(aplastada, 255, 0, 255) == 0);
  // A medio cerrar, más o menos la mitad.
  const int medio = contar(dibujar(CONFIG_SIN_CERRADO, "anime", 0.5f), 0, 255, 255);
  VERIFICAR(medio > cian / 4 && medio < cian * 3 / 4);
  // Sin el sprite del ojo, se dibuja con la SDF (naranja).
  VERIFICAR(sintetica_sprite_suelto(0, nullptr, 0) == 0);
  VERIFICAR(contar(dibujar(CONFIG_ANIME, "anime", 0), 255, 106, 51) > 2000);
  // Un sprite que no es válido no se guarda.
  const auto grande = qoiRgba(129, 10, bocaVerde);
  VERIFICAR(sintetica_sprite_suelto(0, grande.data(), grande.size()) == -1);
  VERIFICAR(sintetica_sprite_suelto(12, grande.data(), 20) == -1);
}

PRUEBA("sprites: la mirada mueve los ojos y el espejo da vuelta el derecho") {
  // Un ojo con un punto a la izquierda: espejado, el derecho lo tiene a la derecha.
  struct L {
    static uint32_t ojo(int x, int y, int w, int h) {
      if (x < w / 4 && y > h / 3 && y < 2 * h / 3) return 0xFF0000FFu;  // rojo a la izquierda
      return ojoCian(x, y, w, h);
    }
  };
  const auto ojo = qoiRgba(64, 64, L::ojo);
  VERIFICAR(sintetica_sprite_suelto(0, ojo.data(), ojo.size()) == 0);
  VERIFICAR(sintetica_sprite_suelto(1, nullptr, 0) == 0);
  VERIFICAR(sintetica_sprite_suelto(2, nullptr, 0) == 0);
  auto centroX = [](const std::vector<uint8_t>& px, int r, int g, int b, int desde, int hasta) {
    double s = 0;
    int n = 0;
    for (int y = 0; y < 172; y++)
      for (int x = desde; x < hasta; x++) {
        const uint8_t* p = &px[(y * 320 + x) * 4];
        if (std::abs(p[0] - r) < 90 && std::abs(p[1] - g) < 90 && std::abs(p[2] - b) < 90) s += x, n++;
      }
    return n ? s / n : -1.0;
  };
  const char* quieta = "{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0},\"tamano\":0.3}}}";
  const char* mirando = "{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0},\"tamano\":0.3,\"gx\":0.2}}}";
  const auto a = dibujar(quieta, "anime", 0), b = dibujar(mirando, "anime", 0);
  const double ca = centroX(a, 0, 255, 255, 0, 320), cb = centroX(b, 0, 255, 255, 0, 320);
  // gx = 0,2 unidades ≈ 31 px a la derecha.
  VERIFICAR(cb - ca > 25 && cb - ca < 38);
  // El rojo del izquierdo está a la izquierda de su ojo; el del derecho (espejado), a la derecha.
  const double rojoIzq = centroX(a, 255, 0, 0, 0, 160), cianIzq = centroX(a, 0, 255, 255, 0, 160);
  const double rojoDer = centroX(a, 255, 0, 0, 160, 320), cianDer = centroX(a, 0, 255, 255, 160, 320);
  VERIFICAR(rojoIzq > 0 && rojoIzq < cianIzq);
  VERIFICAR(rojoDer > cianDer);
  // Sin espejo, el derecho también lo tiene a la izquierda.
  const char* sinEspejo = "{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0},\"tamano\":0.3,\"espejar_der\":false}}}";
  const auto c = dibujar(sinEspejo, "anime", 0);
  VERIFICAR(centroX(c, 255, 0, 0, 160, 320) < centroX(c, 0, 255, 255, 160, 320));
}

PRUEBA("sprites: entran en el buffer fijo y la RAM que suman está medida") {
  // El buffer (los píxeles en RGB565) y casi nada más.
  VERIFICAR(sizeof(CacheSprites) <= CACHE_SPRITES_PX * 2 + 128);
  VERIFICAR(sizeof(CaraImagen) <= 12);
  VERIFICAR(sizeof(CaraVida) <= 144);
  std::printf("    RAM: CacheSprites %zu B (%d px), CaraImagen %zu B × %d, CaraVida %zu B, Config %zu B, Sistema %zu B\n",
              sizeof(CacheSprites), CACHE_SPRITES_PX, sizeof(CaraImagen), MAX_CARAS_IMAGEN, sizeof(CaraVida), sizeof(Config),
              sizeof(Sistema));
  // Cinco partes de 128 × 128 no entran enteras: se achican hasta entrar.
  struct M {
    static std::vector<uint8_t>& g() {
      static std::vector<uint8_t> v = qoiRgba(128, 128, ojoCian);
      return v;
    }
    static int leer(void*, int, uint32_t off, uint8_t* d, size_t n) {
      auto& v = g();
      if (off >= v.size()) return 0;
      const size_t m = std::min(n, v.size() - off);
      std::memcpy(d, v.data() + off, m);
      return static_cast<int>(m);
    }
  };
  static CacheSprites c;
  CaraImagen im;
  for (int i = 0; i < N_PARTES_SPRITE; i++) im.sprite[i] = static_cast<int8_t>(i);
  im.tamano = 8000;
  VERIFICAR(c.cargar(im, 400, M::leer, nullptr));
  VERIFICAR(c.usados() <= static_cast<uint32_t>(CACHE_SPRITES_PX) && c.usados() > CACHE_SPRITES_PX / 2);
  VERIFICAR(c.parte(ParteSprite::Ojo).ancho < 128 && c.parte(ParteSprite::Boca).hay());
  // Chicos en la pantalla: no se guardan más grandes de lo que se ven.
  im.tamano = 1000;
  VERIFICAR(c.cargar(im, 156, M::leer, nullptr));
  VERIFICAR(c.parte(ParteSprite::Ojo).ancho <= 18);
  // La misma imagen en dos partes, una vez.
  for (int i = 0; i < N_PARTES_SPRITE; i++) im.sprite[i] = 0;
  im.tamano = 3000;
  VERIFICAR(c.cargar(im, 156, M::leer, nullptr));
  VERIFICAR(c.usados() == static_cast<uint32_t>(c.parte(ParteSprite::Ojo).ancho) * c.parte(ParteSprite::Ojo).alto);
}

// --- El sistema: subir sprites por la característica Imagen y el fundido ---

namespace {
uint32_t g_t = 0;
std::map<std::string, std::vector<uint8_t>> g_mem, g_archivos;
std::vector<std::vector<uint8_t>> g_notis;
uint16_t g_px[320 * 172];
Sistema g_s;

Hal halSprites() {
  Hal h;
  h.ahoraMs = [] { return g_t; };
  h.semilla = [] { return 7u; };
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
    const size_t m = std::min(n, it->second.size() - off);
    std::memcpy(d, it->second.data() + off, m);
    return static_cast<int>(m);
  };
  h.borrar = [](const char* k) { g_archivos.erase(k); return true; };
  h.notificar = [](uint8_t, const uint8_t* d, size_t n) { g_notis.emplace_back(d, d + n); };
  return h;
}

std::vector<uint8_t> u32(uint32_t v) { return {uint8_t(v), uint8_t(v >> 8), uint8_t(v >> 16), uint8_t(v >> 24)}; }

uint8_t subir(int espacio, const std::vector<uint8_t>& datos) {
  std::vector<uint8_t> ini = {ImgInicio, uint8_t(espacio)};
  for (uint8_t x : u32(static_cast<uint32_t>(datos.size()))) ini.push_back(x);
  for (uint8_t x : u32(crc32(datos.data(), datos.size()))) ini.push_back(x);
  g_notis.clear();
  g_s.bleEscribir(CarImagen, ini.data(), ini.size());
  for (size_t off = 0; off < datos.size(); off += 170) {
    std::vector<uint8_t> b = {ImgBloque};
    for (uint8_t x : u32(static_cast<uint32_t>(off))) b.push_back(x);
    b.insert(b.end(), datos.begin() + off, datos.begin() + std::min(datos.size(), off + 170));
    g_s.bleEscribir(CarImagen, b.data(), b.size());
  }
  const uint8_t fin = ImgFin;
  g_s.bleEscribir(CarImagen, &fin, 1);
  return g_notis.back()[1];
}

uint8_t mandarConfig(const std::string& s) {
  std::vector<uint8_t> ini = {ConfigInicio};
  for (uint8_t x : u32(static_cast<uint32_t>(s.size()))) ini.push_back(x);
  for (uint8_t x : u32(crc32(reinterpret_cast<const uint8_t*>(s.data()), s.size()))) ini.push_back(x);
  g_notis.clear();
  g_s.bleEscribir(CarConfig, ini.data(), ini.size());
  std::vector<uint8_t> b = {ConfigBloque, 0, 0, 0, 0};
  b.insert(b.end(), s.begin(), s.end());
  g_s.bleEscribir(CarConfig, b.data(), b.size());
  const uint8_t fin = ConfigFin;
  g_s.bleEscribir(CarConfig, &fin, 1);
  return g_notis.back()[1];
}

// Cuánto cian se ve: la suma de su intensidad (en píxeles enteros), que
// cambia de a poco si la cara se funde y de golpe si hay un corte.
int cianEnPantalla() {
  uint32_t n = 0;
  for (uint16_t c : g_px) {
    const uint32_t rgb = de565(c);
    const uint32_t r = (rgb >> 16) & 0xFF, g = (rgb >> 8) & 0xFF, b = rgb & 0xFF;
    const int k = static_cast<int>(g < b ? g : b) - static_cast<int>(r) - 16;  // el fondo (#0B0A09) no cuenta
    if (k > 0) n += static_cast<uint32_t>(k);
  }
  return static_cast<int>(n / 255);
}
}  // namespace

PRUEBA("sprites: se suben a los espacios 8 a 19, se listan y la cara entra y sale sin cortes") {
  g_mem.clear(), g_archivos.clear();
  g_t = 1000;
  Perfil p;
  g_s.iniciar(halSprites(), g_px, p);
  g_s.modoVistaPrevia();
  const auto ojo = qoiRgba(64, 64, ojoCian), cer = qoiRgba(64, 20, cerradoMagenta);
  VERIFICAR(subir(8, ojo) == Ok);
  VERIFICAR(subir(9, cer) == Ok);
  VERIFICAR(g_s.largoSprite(0) == ojo.size() && g_s.largoSprite(1) == cer.size());
  VERIFICAR(g_archivos.count("spr0") && g_archivos.count("spr1"));
  // Un sprite más grande que 128, o que no es QOI: no.
  VERIFICAR(subir(10, qoiRgba(130, 4, bocaVerde)) == ImagenInvalida);
  VERIFICAR(subir(19, std::vector<uint8_t>(40, 3)) == ImagenInvalida);
  // Listar los sprites: [0xA6][0][máscara u16][espacio, largo, crc]...
  const uint8_t listar = ImgListarSprites;
  g_notis.clear();
  g_s.bleEscribir(CarImagen, &listar, 1);
  VERIFICAR(g_notis.size() == 1 && g_notis[0][0] == (ImgListarSprites | Respuesta) && g_notis[0][1] == Ok);
  VERIFICAR(g_notis[0][2] == 0x03 && g_notis[0][3] == 0 && g_notis[0].size() == 4 + 2 * 9 && g_notis[0][4] == 8);
  // La de las imágenes no los muestra.
  const uint8_t listarImg = ImgListar;
  g_notis.clear();
  g_s.bleEscribir(CarImagen, &listarImg, 1);
  VERIFICAR(g_notis[0][2] == 0 && g_notis[0].size() == 3);
  // Pedirlo de vuelta.
  const uint8_t pedir[2] = {ImgPedir, 8};
  g_notis.clear();
  g_s.bleEscribir(CarImagen, pedir, 2);
  VERIFICAR(g_notis.size() >= 3 && g_notis.back()[0] == ImgCierre && g_notis.back()[1] == 8);

  // La cara de siempre es la de imagen: entra con un fundido.
  g_s.tick();
  VERIFICAR(cianEnPantalla() == 0);
  VERIFICAR(mandarConfig("{\"version\":2,\"cara\":{\"expresion\":\"anime\"},\"expresiones\":{\"anime\":{\"sprites\":"
                         "{\"ojo\":0,\"ojo_cerrado\":1},\"tamano\":0.4}}}") == Ok);
  int antes = 0, maxSalto = 0;
  float opAntes = 0;
  float maxPaso = 0;
  for (int i = 0; i < 40; i++) {
    g_t += 16;
    g_s.tick();
    const int ahora = cianEnPantalla();
    maxSalto = std::max(maxSalto, std::abs(ahora - antes));
    maxPaso = std::max(maxPaso, std::abs(g_s.opacidadSprites() - opAntes));
    antes = ahora, opAntes = g_s.opacidadSprites();
  }
  VERIFICAR(g_s.opacidadSprites() > 0.99f && g_s.cacheSprites().listo());
  int lleno = cianEnPantalla();
  for (int i = 0; i < 120; i++) g_t += 16, g_s.tick(), lleno = std::max(lleno, cianEnPantalla());
  VERIFICAR(lleno > 2500);
  // Nunca un corte: ningún cuadro cambia más de un 15 % de golpe.
  VERIFICAR(maxPaso <= 0.1f);
  VERIFICAR(maxSalto < lleno * 0.25);
  VERIFICAR(std::strcmp(g_s.motor().nombreVisible(), "anime") == 0);
  // Vive: en 10 s parpadea (el cian baja mucho en algún cuadro) y no es siempre igual.
  int minimo = lleno, distintos = 0, previo = lleno;
  for (int i = 0; i < 600; i++) {
    g_t += 16;
    g_s.tick();
    const int n = cianEnPantalla();
    minimo = std::min(minimo, n);
    if (n != previo) distintos++;
    previo = n;
  }
  VERIFICAR(minimo < lleno / 3);
  VERIFICAR(distintos > 100);
  // Un reflejo con una cara paramétrica la deforma (sorpresa agranda los ojos) sin sacarla.
  g_s.motor().reaccionar(g_s.config().resolverCara("sorpresa"), g_t, 2000);
  for (int i = 0; i < 30; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.motor().caraImagen() == g_s.config().vida.indiceCara("anime"));
  VERIFICAR(g_s.opacidadSprites() > 0.99f);
  for (int i = 0; i < 200; i++) g_t += 16, g_s.tick();
  // Una fija paramétrica (la eligió el dueño): sale la de imagen, con fundido.
  VERIFICAR(g_s.motor().fijar("contento"));
  maxPaso = 0, opAntes = g_s.opacidadSprites();
  for (int i = 0; i < 30; i++) {
    g_t += 16;
    g_s.tick();
    maxPaso = std::max(maxPaso, std::abs(g_s.opacidadSprites() - opAntes));
    opAntes = g_s.opacidadSprites();
  }
  VERIFICAR(g_s.opacidadSprites() < 0.01f && maxPaso <= 0.1f && g_s.motor().caraImagen() == -1);
  VERIFICAR(cianEnPantalla() < 50);
  g_s.motor().fijar("");
  for (int i = 0; i < 30; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.opacidadSprites() > 0.99f);
  // Borrar el sprite del ojo: vuelve la SDF, con fundido.
  const uint8_t borrar[2] = {ImgBorrar, 8};
  g_s.bleEscribir(CarImagen, borrar, 2);
  VERIFICAR(g_s.largoSprite(0) == 0 && !g_archivos.count("spr0"));
  g_t += 16, g_s.tick();
  VERIFICAR(g_s.opacidadSprites() > 0.8f);  // todavía se está yendo
  for (int i = 0; i < 30; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.opacidadSprites() < 0.01f && cianEnPantalla() < 50);
  // Al reiniciar, los sprites guardados siguen (los metadatos en "sprites").
  VERIFICAR(subir(8, ojo) == Ok);
  g_s.iniciar(halSprites(), g_px, p);
  g_s.modoVistaPrevia();
  int maximo = 0;
  for (int i = 0; i < 120; i++) g_t += 16, g_s.tick(), maximo = std::max(maximo, cianEnPantalla());
  VERIFICAR(g_s.largoSprite(0) == ojo.size() && g_s.opacidadSprites() > 0.99f && maximo > 2500);
}

PRUEBA("sprites: un reflejo o un paso con la cara de imagen la muestran un rato") {
  g_mem.clear(), g_archivos.clear();
  g_t = 1000;
  Perfil p;
  g_s.iniciar(halSprites(), g_px, p);
  g_s.modoVistaPrevia();
  VERIFICAR(subir(8, qoiRgba(48, 48, ojoCian)) == Ok);
  VERIFICAR(mandarConfig("{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0}}},"
                         "\"reglas\":[{\"si\":\"tacto\",\"entonces\":[\"reflejo:anime\"]}]}") == Ok);
  for (int i = 0; i < 20; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.motor().caraImagen() == -1 && cianEnPantalla() == 0);
  g_s.evento(Evento::Tacto);
  for (int i = 0; i < 20; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.motor().caraImagen() >= 0 && g_s.opacidadSprites() > 0.9f && cianEnPantalla() > 1000);
  // Se va sola (el reflejo dura poco).
  for (int i = 0; i < 300; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.motor().caraImagen() == -1 && g_s.opacidadSprites() < 0.01f);
  // Como paso de un comportamiento propio.
  VERIFICAR(mandarConfig("{\"version\":2,\"expresiones\":{\"anime\":{\"sprites\":{\"ojo\":0}}},\"comportamientos\":"
                         "{\"saludo-anime\":{\"tipo\":\"reaccion\",\"pasos\":[{\"cara\":\"anime\",\"dura\":2}]}}}") == Ok);
  VERIFICAR(g_s.motor().comportamiento(g_s.config().vida.indiceComportamiento("saludo-anime"), g_t));
  for (int i = 0; i < 30; i++) g_t += 16, g_s.tick();
  VERIFICAR(g_s.motor().caraImagen() >= 0 && g_s.opacidadSprites() > 0.9f);
}

PRUEBA("sprites: dibujar de a franjas da lo mismo que de una vez (con glitch y temblor)") {
  struct M {
    static std::vector<uint8_t>& g() {
      static std::vector<uint8_t> v = qoiRgba(60, 60, ojoCian);
      return v;
    }
    static int leer(void*, int, uint32_t off, uint8_t* d, size_t n) {
      auto& v = g();
      if (off >= v.size()) return 0;
      const size_t m = std::min(n, v.size() - off);
      std::memcpy(d, v.data() + off, m);
      return static_cast<int>(m);
    }
  };
  static CacheSprites c;
  CaraImagen im;
  im.sprite[0] = 0, im.sprite[4] = 0;
  im.tamano = 4000;
  const VistaRostro v = VistaRostro::para(240, 240, true, false);
  VERIFICAR(c.cargar(im, v.unidad, M::leer, nullptr));
  CuadroRostro q;
  q.r[param::GLITCH] = 0.8f, q.glitch = 0.8f, q.semillaGlitch = 3;
  q.r[param::SHAKE] = 0.5f, q.r[param::FTILT] = 0.2f, q.tiempo = 1.3f;
  q.r[param::TILT] = 0.3f;
  static uint16_t entera[240 * 240], franja[240 * 40], porFranjas[240 * 240];
  Lienzo a;
  a.iniciar(entera, 240, 240);
  dibujarRostro(a, v, q, &c, 1, 1);
  Lienzo b;
  b.iniciar(franja, 240, 240, 40);
  for (int y0 = 0; y0 < 240; y0 += 40) {
    b.empezarFranja(y0);
    dibujarRostro(b, v, q, &c, 1, 1);
    std::memcpy(porFranjas + y0 * 240, franja, sizeof franja);
  }
  VERIFICAR(std::memcmp(entera, porFranjas, sizeof entera) == 0);
  int encendidos = 0;
  for (uint16_t px : entera) encendidos += (de565(px) & 0xFF) > 150;
  VERIFICAR(encendidos > 1500);
}
