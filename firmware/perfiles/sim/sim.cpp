// Perfil "sim": el núcleo de SintéticaOS corriendo en WebAssembly, sin libc.
// El navegador (sim/web/sim.js) da el reloj, la pantalla (un canvas), el
// almacenamiento (localStorage) y recibe las notificaciones BLE.
#include "sistema.h"

using namespace sintetica;

#define IMPORTAR(nombre) __attribute__((import_module("env"), import_name(#nombre)))

extern "C" {
IMPORTAR(js_ahora) uint32_t js_ahora();
IMPORTAR(js_identificador) const char* js_identificador();
IMPORTAR(js_semilla) uint32_t js_semilla();
IMPORTAR(js_presentar_franja) void js_presentar_franja(const uint16_t* pixeles, int ancho, int y0, int filas);
IMPORTAR(js_escribir_trozo) int js_escribir_trozo(const char* clave, uint32_t desplazamiento, const uint8_t* datos, size_t n);
IMPORTAR(js_leer_trozo) int js_leer_trozo(const char* clave, uint32_t desplazamiento, uint8_t* datos, size_t n);
IMPORTAR(js_borrar) int js_borrar(const char* clave);
IMPORTAR(js_minuto) int js_minuto();
IMPORTAR(js_bateria) int js_bateria();
IMPORTAR(js_guardar) int js_guardar(const char* clave, const uint8_t* datos, size_t n);
IMPORTAR(js_leer) int js_leer(const char* clave, uint8_t* datos, size_t cap);
IMPORTAR(js_notificar) void js_notificar(int caracteristica, const uint8_t* datos, size_t n);
IMPORTAR(js_sonido) void js_sonido(const char* nombre);
IMPORTAR(js_led) void js_led(uint32_t rgb);
IMPORTAR(js_registrar) void js_registrar(const char* mensaje);

// Lo que el compilador puede pedir aunque no haya libc.
void* memcpy(void* d, const void* s, size_t n) {
  copiarBytes(d, s, n);
  return d;
}
void* memmove(void* d, const void* s, size_t n) {
  auto* dd = static_cast<uint8_t*>(d);
  auto* ss = static_cast<const uint8_t*>(s);
  if (dd < ss) for (size_t i = 0; i < n; i++) dd[i] = ss[i];
  else for (size_t i = n; i > 0; i--) dd[i - 1] = ss[i - 1];
  return d;
}
void* memset(void* d, int v, size_t n) {
  llenarBytes(d, static_cast<uint8_t>(v), n);
  return d;
}
int memcmp(const void* a, const void* b, size_t n) {
  auto* x = static_cast<const uint8_t*>(a);
  auto* y = static_cast<const uint8_t*>(b);
  for (size_t i = 0; i < n; i++)
    if (x[i] != y[i]) return x[i] < y[i] ? -1 : 1;
  return 0;
}
void __cxa_pure_virtual() { __builtin_trap(); }
}

namespace {
// 1.47B: 320 × 172 de una vez. Broche: 240 × 240 en franjas de 40 filas, como en la placa.
uint16_t g_pixeles[320 * 240];
uint8_t g_intercambio[MAX_CONFIG_BYTES + 64];
Sistema g_sistema;
}  // namespace

extern "C" {

// perfil: 0 = la 1.47B (mascota), 1 = el broche.
__attribute__((export_name("sim_iniciar"))) void sim_iniciar(int perfil) {
  Hal h;
  h.ahoraMs = js_ahora;
  h.identificador = js_identificador;
  h.semilla = js_semilla;
  h.presentarFranja = js_presentar_franja;
  h.escribirTrozo = [](const char* c, uint32_t off, const uint8_t* d, size_t n) { return js_escribir_trozo(c, off, d, n) != 0; };
  h.leerTrozo = js_leer_trozo;
  h.borrar = [](const char* c) { return js_borrar(c) != 0; };
  h.minutoDelDia = js_minuto;
  h.bateria = js_bateria;
  h.guardar = [](const char* c, const uint8_t* d, size_t n) { return js_guardar(c, d, n) != 0; };
  h.leer = js_leer;
  h.notificar = [](uint8_t c, const uint8_t* d, size_t n) { js_notificar(c, d, n); };
  h.sonido = js_sonido;
  h.led = js_led;
  h.registrar = js_registrar;
  Perfil p;
  if (perfil == 1) {
    p.ancho = 240, p.alto = 240, p.filasFranja = 40, p.redonda = true, p.boton = Perfil::Boton::Broche;
  }
  g_sistema.iniciar(h, g_pixeles, p);
}

__attribute__((export_name("sim_tick"))) void sim_tick() { g_sistema.tick(); }
__attribute__((export_name("sim_tacto"))) void sim_tacto(int apretado) { g_sistema.tacto(apretado != 0); }
__attribute__((export_name("sim_imu"))) void sim_imu(float ax, float ay, float az) { g_sistema.imu(ax, ay, az); }
__attribute__((export_name("sim_evento"))) void sim_evento(int e) {
  if (e >= 0 && e < static_cast<int>(Evento::Cantidad)) g_sistema.evento(static_cast<Evento>(e));
}

// Memoria para pasar bytes desde JS (escrituras BLE, lecturas de estado).
__attribute__((export_name("sim_intercambio"))) uint8_t* sim_intercambio() { return g_intercambio; }
__attribute__((export_name("sim_intercambio_cap"))) int sim_intercambio_cap() { return sizeof g_intercambio; }
__attribute__((export_name("sim_ble_escribir"))) void sim_ble_escribir(int car, int n) {
  if (n > 0 && n <= static_cast<int>(sizeof g_intercambio)) g_sistema.bleEscribir(static_cast<uint8_t>(car), g_intercambio, n);
}
__attribute__((export_name("sim_ble_estado"))) int sim_ble_estado() {
  return static_cast<int>(g_sistema.estadoBle(g_intercambio, sizeof g_intercambio));
}

// Para la interfaz: la lista de expresiones y cuál se ve.
__attribute__((export_name("sim_cantidad_expresiones"))) int sim_cantidad_expresiones() {
  return g_sistema.config().nExpresiones;
}
__attribute__((export_name("sim_nombre_expresion"))) const char* sim_nombre_expresion(int i) {
  return (i >= 0 && i < g_sistema.config().nExpresiones) ? g_sistema.config().expresiones[i].nombre : "";
}
__attribute__((export_name("sim_expresion_visible"))) int sim_expresion_visible() {
  return g_sistema.motor().expresionVisible();
}
__attribute__((export_name("sim_nombre"))) const char* sim_nombre() { return g_sistema.nombre(); }
__attribute__((export_name("sim_cierre"))) float sim_cierre() { return g_sistema.motor().estado().cierre; }
__attribute__((export_name("sim_conexion"))) void sim_conexion(int conectado) { g_sistema.bleConexion(conectado != 0); }
__attribute__((export_name("sim_qr"))) const char* sim_qr() { return g_sistema.contenidoQr(); }
__attribute__((export_name("sim_emparejando"))) int sim_emparejando() { return g_sistema.emparejando() ? 1 : 0; }
__attribute__((export_name("sim_fijar_serie"))) void sim_fijar_serie(const char* s) { g_sistema.fijarSerie(s); }
__attribute__((export_name("sim_quiere_dormir"))) int sim_quiere_dormir() { return g_sistema.quiereDormir() ? 1 : 0; }
__attribute__((export_name("sim_imagen_visible"))) int sim_imagen_visible() { return g_sistema.imagenVisible(); }
__attribute__((export_name("sim_posicion_lista"))) int sim_posicion_lista() { return g_sistema.posicionLista(); }
__attribute__((export_name("sim_version"))) const char* sim_version() { return VERSION_SINTETICAOS; }
}
