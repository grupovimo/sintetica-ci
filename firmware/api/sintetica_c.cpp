#include "sintetica_c.h"

#include "sistema.h"

using namespace sintetica;

namespace {
sintetica_hal g_hal;
Sistema g_sistema;
uint16_t g_pixeles[640 * 640];
int g_ancho = 0, g_alto = 0;
}  // namespace

extern "C" {

void sintetica_iniciar(const sintetica_hal* hal, int ancho, int alto) {
  g_hal = hal ? *hal : sintetica_hal{};
  g_ancho = mini(maxi(ancho, 16), MAX_ANCHO);
  g_alto = mini(maxi(alto, 16), MAX_ALTO);
  Hal h;
  h.ahoraMs = [] { return g_hal.ahora_ms ? g_hal.ahora_ms(g_hal.contexto) : 0u; };
  h.identificador = [] { return g_hal.identificador; };
  h.semilla = [] { return g_hal.semilla ? g_hal.semilla(g_hal.contexto) : 0x5157E71Cu; };
  h.minutoDelDia = [] { return g_hal.minuto_del_dia ? g_hal.minuto_del_dia(g_hal.contexto) : -1; };
  h.bateria = [] { return g_hal.bateria ? g_hal.bateria(g_hal.contexto) : -1; };
  h.guardar = [](const char* c, const uint8_t* d, size_t n) {
    return g_hal.guardar ? g_hal.guardar(g_hal.contexto, c, d, n) != 0 : false;
  };
  h.leer = [](const char* c, uint8_t* d, size_t cap) { return g_hal.leer ? g_hal.leer(g_hal.contexto, c, d, cap) : -1; };
  h.notificar = [](uint8_t car, const uint8_t* d, size_t n) {
    if (g_hal.notificar) g_hal.notificar(g_hal.contexto, car, d, n);
  };
  h.registrar = [](const char* m) {
    if (g_hal.registrar) g_hal.registrar(g_hal.contexto, m);
  };
  g_sistema.iniciar(h, g_pixeles, g_ancho, g_alto);
}

void sintetica_vista_previa(void) { g_sistema.modoVistaPrevia(); }
void sintetica_tick(void) { g_sistema.tick(); }
const uint16_t* sintetica_pixeles(void) { return g_pixeles; }
int sintetica_ancho(void) { return g_ancho; }
int sintetica_alto(void) { return g_alto; }

void sintetica_pixeles_rgba(uint8_t* s) {
  for (int i = 0; i < g_ancho * g_alto; i++) {
    uint32_t c = de565(g_pixeles[i]);
    s[i * 4] = static_cast<uint8_t>(c >> 16);
    s[i * 4 + 1] = static_cast<uint8_t>(c >> 8);
    s[i * 4 + 2] = static_cast<uint8_t>(c);
    s[i * 4 + 3] = 255;
  }
}

void sintetica_tacto(int apretado) { g_sistema.tacto(apretado != 0); }
void sintetica_imu(float ax, float ay, float az) { g_sistema.imu(ax, ay, az); }
void sintetica_evento(int e) {
  if (e >= 0 && e < static_cast<int>(Evento::Cantidad)) g_sistema.evento(static_cast<Evento>(e));
}

void sintetica_ble_conexion(int conectado) { g_sistema.bleConexion(conectado != 0); }
void sintetica_ble_escribir(int car, const uint8_t* d, size_t n) { g_sistema.bleEscribir(static_cast<uint8_t>(car), d, n); }
size_t sintetica_ble_estado(uint8_t* s, size_t cap) { return g_sistema.estadoBle(s, cap); }

int sintetica_cantidad_expresiones(void) { return g_sistema.config().nExpresiones; }
const char* sintetica_nombre_expresion(int i) {
  return (i >= 0 && i < g_sistema.config().nExpresiones) ? g_sistema.config().expresiones[i].nombre : "";
}
const char* sintetica_expresion_visible(void) {
  return g_sistema.config().expresiones[g_sistema.motor().expresionVisible()].nombre;
}
const char* sintetica_qr(void) { return g_sistema.contenidoQr(); }
const char* sintetica_version(void) { return VERSION_SINTETICAOS; }
}
