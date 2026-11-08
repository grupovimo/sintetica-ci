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
IMPORTAR(js_epoch) uint32_t js_epoch();
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
// El editor de la web: dibuja una expresión o un cuadro de una animación sin
// el sistema, con una config propia y al tamaño que pida (hasta 640 × 480).
uint16_t g_editorPx[640 * 480];
Lienzo g_editorLienzo;
Vista g_editorVista;
Config g_editorConfig;
char g_detalle[128];  // como el de cgo: entran todos los detalles del núcleo
char g_visible[LARGO_NOMBRE + 12];

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
  h.epoch = js_epoch;  // la hora local (el simulador la puede acelerar)
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

// Animaciones de la config (para el panel).
__attribute__((export_name("sim_cantidad_animaciones"))) int sim_cantidad_animaciones() {
  return g_sistema.config().nAnimaciones;
}
__attribute__((export_name("sim_nombre_animacion"))) const char* sim_nombre_animacion(int i) {
  return (i >= 0 && i < g_sistema.config().nAnimaciones) ? g_sistema.config().animaciones[i].nombre : "";
}
// Lo que se ve: el nombre de la expresión o "animacion:<nombre>".
__attribute__((export_name("sim_visible"))) const char* sim_visible() {
  const int a = g_sistema.animacionVisible();
  if (a < 0) return g_sistema.motor().nombreVisible();
  const size_t k = copiarTexto(g_visible, sizeof g_visible, "animacion:");
  copiarTexto(g_visible + k, sizeof g_visible - k, g_sistema.config().animaciones[a].nombre);
  return g_visible;
}
// Lo que llega por la nube (el bicho virtual de la web lo usa para probar).
__attribute__((export_name("sim_nube_escribir"))) void sim_nube_escribir(int car, int n) {
  if (n > 0 && n <= static_cast<int>(sizeof g_intercambio)) g_sistema.nubeEscribir(static_cast<uint8_t>(car), g_intercambio, n);
}
// Vista previa (la web y las apps): nunca muestra el QR y confía en quien escribe.
__attribute__((export_name("sim_vista_previa"))) void sim_vista_previa() { g_sistema.modoVistaPrevia(); }
__attribute__((export_name("sim_crc_config"))) uint32_t sim_crc_config() { return g_sistema.crcConfig(); }

// --- Editor ---
// Valida la config que está en el intercambio (n bytes). 0 = válida; si no, el
// código de error (ErrorConfig) y el detalle en sim_detalle().
__attribute__((export_name("sim_validar"))) int sim_validar(int n) {
  static Config c;
  if (n <= 0 || n > static_cast<int>(MAX_CONFIG_BYTES)) {
    copiarTexto(g_detalle, sizeof g_detalle, "config: más de 16 KB");
    return static_cast<int>(ErrorConfig::Demasiadas);
  }
  ResultadoConfig r = cargarConfig(c, reinterpret_cast<const char*>(g_intercambio), static_cast<size_t>(n));
  copiarTexto(g_detalle, sizeof g_detalle, r.detalle ? r.detalle : "");
  return static_cast<int>(r.error);
}
__attribute__((export_name("sim_detalle"))) const char* sim_detalle() { return g_detalle; }

// El tamaño del lienzo del editor (por ejemplo, 640 × 344 para la 1.47B al doble).
__attribute__((export_name("sim_editor_iniciar"))) void sim_editor_iniciar(int ancho, int alto, int redonda) {
  ancho = mini(maxi(ancho, 16), 640);
  alto = mini(maxi(alto, 16), 480);
  g_editorLienzo.iniciar(g_editorPx, ancho, alto);
  g_editorVista = Vista::para(ancho, alto, redonda != 0);
}
// Carga la config del editor (como sim_validar). Si falla, queda la anterior.
__attribute__((export_name("sim_editor_config"))) int sim_editor_config(int n) {
  if (n <= 0 || n > static_cast<int>(MAX_CONFIG_BYTES)) return static_cast<int>(ErrorConfig::Demasiadas);
  static Config nueva;
  ResultadoConfig r = cargarConfig(nueva, reinterpret_cast<const char*>(g_intercambio), static_cast<size_t>(n));
  copiarTexto(g_detalle, sizeof g_detalle, r.detalle ? r.detalle : "");
  if (r.ok()) g_editorConfig = nueva;
  return static_cast<int>(r.error);
}
// Índices por nombre (el nombre va en el intercambio, terminado en 0).
__attribute__((export_name("sim_editor_indice_expresion"))) int sim_editor_indice_expresion() {
  return g_editorConfig.indiceExpresion(reinterpret_cast<const char*>(g_intercambio));
}
__attribute__((export_name("sim_editor_indice_animacion"))) int sim_editor_indice_animacion() {
  return g_editorConfig.indiceAnimacion(reinterpret_cast<const char*>(g_intercambio));
}
__attribute__((export_name("sim_editor_duracion"))) int sim_editor_duracion(int i) {
  return (i >= 0 && i < g_editorConfig.nAnimaciones) ? g_editorConfig.animaciones[i].duracion : 0;
}
// Dibuja la expresión i con un parpadeo (cierre 0-1) y una mirada.
__attribute__((export_name("sim_editor_expresion"))) void sim_editor_expresion(int i, float cierre, float mx, float my) {
  g_editorLienzo.limpiar(paleta::PANTALLA);
  if (i < 0 || i >= g_editorConfig.nExpresiones) return;
  EstadoCara e;
  e.cierre = cierre, e.miradaX = mx, e.miradaY = my;
  e.tamano = g_editorConfig.tamano;
  e.hayColorOjos = g_editorConfig.hayColorOjos, e.colorOjos = g_editorConfig.colorOjos;
  dibujarExpresion(g_editorLienzo, g_editorVista, g_editorConfig.expresiones[i], e);
}
// Dibuja el cuadro t (ms) de la animación i, con su expresión de abajo.
__attribute__((export_name("sim_editor_animacion"))) void sim_editor_animacion(int i, int t) {
  g_editorLienzo.limpiar(paleta::PANTALLA);
  if (i < 0 || i >= g_editorConfig.nAnimaciones) return;
  const Animacion& a = g_editorConfig.animaciones[i];
  EstadoCara e;
  e.tamano = g_editorConfig.tamano;
  e.hayColorOjos = g_editorConfig.hayColorOjos, e.colorOjos = g_editorConfig.colorOjos;
  if (a.sobre >= 0) dibujarExpresion(g_editorLienzo, g_editorVista, g_editorConfig.expresiones[a.sobre], e);
  dibujarAnimacion(g_editorLienzo, g_editorVista, a, t < 0 ? 0u : static_cast<uint32_t>(t), e);
}
__attribute__((export_name("sim_editor_pixeles"))) const uint16_t* sim_editor_pixeles() { return g_editorPx; }

// --- Motor de vida ---
// Los parámetros del rostro (para armar los controles del editor).
__attribute__((export_name("sim_cantidad_parametros"))) int sim_cantidad_parametros() { return N_PARAMETROS; }
__attribute__((export_name("sim_parametro_nombre"))) const char* sim_parametro_nombre(int i) {
  return i >= 0 && i < N_PARAMETROS ? infoParametro(i).nombre : "";
}
__attribute__((export_name("sim_parametro_min"))) float sim_parametro_min(int i) { return i >= 0 && i < N_PARAMETROS ? infoParametro(i).minimo : 0; }
__attribute__((export_name("sim_parametro_max"))) float sim_parametro_max(int i) { return i >= 0 && i < N_PARAMETROS ? infoParametro(i).maximo : 0; }
__attribute__((export_name("sim_parametro_defecto"))) float sim_parametro_defecto(int i) {
  return i >= 0 && i < N_PARAMETROS ? infoParametro(i).defecto : 0;
}
// Las expresiones paramétricas del editor.
__attribute__((export_name("sim_editor_cantidad_caras"))) int sim_editor_cantidad_caras() { return g_editorConfig.vida.nCaras; }
__attribute__((export_name("sim_editor_nombre_cara"))) const char* sim_editor_nombre_cara(int i) {
  return i >= 0 && i < g_editorConfig.vida.nCaras ? g_editorConfig.vida.caras[i].nombre : "";
}
__attribute__((export_name("sim_editor_indice_cara"))) int sim_editor_indice_cara() {
  return g_editorConfig.vida.indiceCara(reinterpret_cast<const char*>(g_intercambio));
}
// Los parámetros completos de la cara i (neutral + sus ajustes) al intercambio, como float32.
__attribute__((export_name("sim_editor_parametros_cara"))) int sim_editor_parametros_cara(int i) {
  Rostro r;
  const int n = g_editorConfig.vida.indiceCara("neutral");
  if (n >= 0) g_editorConfig.vida.caras[n].aplicar(r);
  if (i >= 0 && i < g_editorConfig.vida.nCaras) g_editorConfig.vida.caras[i].aplicar(r);
  copiarBytes(g_intercambio, r.v, sizeof r.v);
  return N_PARAMETROS;
}
// Dibuja un rostro con los parámetros del intercambio (N_PARAMETROS float32),
// con un parpadeo y una mirada, y las partículas de muestra que se pidan.
__attribute__((export_name("sim_editor_rostro"))) void sim_editor_rostro(float parpadeo, float t, int particula) {
  CuadroRostro c;
  copiarBytes(c.r.v, g_intercambio, sizeof c.r.v);
  c.r.limitar();
  const float bl = limitar(parpadeo, 0, 1);
  for (int lado = 0; lado < 2; lado++) {
    const int b = lado * param::POR_OJO;
    c.r[b + param::LT] += (0.5f - c.r[b + param::LT]) * bl;
    c.r[b + param::LB] += (0.5f - c.r[b + param::LB]) * bl;
  }
  c.tiempo = t;
  c.faseSpin = t * 3.2f * c.r[param::SPIN];
  c.faseOrbita = t * 4.5f * c.r[param::ORBIT];
  const float ph = t * 1.15f - piso(t * 1.15f);
  auto g = [](float x) { return exponencial(-x * x); };
  c.latido = g((ph - 0.1f) * 22) + 0.65f * g((ph - 0.3f) * 22);
  c.pulso = 1 + c.r[param::PULSE] * (0.09f + 0.09f * seno(t * 9) * seno(t * 2.3f));
  c.glitch = c.r[param::GLITCH] * (seno(t * 23) > 0.6f ? 1.0f : 0.35f);
  c.semillaGlitch = piso(t * 8);
  c.progreso = t * 0.15f - piso(t * 0.15f);
  c.colorOjos = g_editorConfig.hayColorOjos ? g_editorConfig.colorOjos : paleta::NARANJA;
  c.brillo = g_editorConfig.vida.brilloMax;
  if (particula >= 0 && particula < static_cast<int>(TipoParticula::Cantidad)) {
    Particula& p = c.particulas[0];
    p.viva = true;
    p.tipo = static_cast<TipoParticula>(particula);
    p.x = 0.33f, p.y = 0.3f, p.vida = 2, p.edad = 0.6f, p.tamano = 0.06f;
  }
  const VistaRostro v = VistaRostro::para(g_editorLienzo.ancho(), g_editorLienzo.alto(), g_editorVista.redonda);
  dibujarRostro(g_editorLienzo, v, c);
}

// El estado del motor del simulador (las barras del panel).
__attribute__((export_name("sim_vida_estado"))) float sim_vida_estado(int i) {
  return i >= 0 && i < N_ESTADO ? g_sistema.motor().estadoInterno()[i] : 0;
}
__attribute__((export_name("sim_vida_rasgo"))) float sim_vida_rasgo(int i) {
  return i >= 0 && i < N_RASGOS ? g_sistema.motor().rasgos()[i] : 0;
}
__attribute__((export_name("sim_vida_comportamiento"))) const char* sim_vida_comportamiento() {
  return g_sistema.motor().nombreComportamiento();
}
__attribute__((export_name("sim_vida_dormido"))) int sim_vida_dormido() {
  return (g_sistema.motor().dormido() ? 1 : 0) | (g_sistema.motor().sonando() ? 2 : 0);
}
__attribute__((export_name("sim_vida_candidato"))) const char* sim_vida_candidato(int i) {
  const char* n[3];
  float p[3];
  const int k = g_sistema.motor().candidatos(n, p, 3);
  return i >= 0 && i < k ? n[i] : "";
}
__attribute__((export_name("sim_vida_puntaje"))) float sim_vida_puntaje(int i) {
  const char* n[3];
  float p[3];
  const int k = g_sistema.motor().candidatos(n, p, 3);
  return i >= 0 && i < k ? p[i] : 0;
}
__attribute__((export_name("sim_vida_comportamientos_hechos"))) int sim_vida_comportamientos_hechos() {
  return static_cast<int>(g_sistema.motor().comportamientosEmpezados());
}
__attribute__((export_name("sim_vida_parpadeos"))) int sim_vida_parpadeos() { return static_cast<int>(g_sistema.motor().parpadeos()); }
// Las expresiones paramétricas de la config del bicho.
__attribute__((export_name("sim_cantidad_caras"))) int sim_cantidad_caras() { return g_sistema.config().vida.nCaras; }
__attribute__((export_name("sim_nombre_cara"))) const char* sim_nombre_cara(int i) {
  return i >= 0 && i < g_sistema.config().vida.nCaras ? g_sistema.config().vida.caras[i].nombre : "";
}
__attribute__((export_name("sim_cantidad_eventos"))) int sim_cantidad_eventos() { return static_cast<int>(Evento::Cantidad); }
__attribute__((export_name("sim_nombre_evento"))) const char* sim_nombre_evento(int i) {
  return i >= 0 && i < static_cast<int>(Evento::Cantidad) ? nombreEvento(static_cast<Evento>(i)) : "";
}
// Empieza un comportamiento por nombre (el nombre en el intercambio).
__attribute__((export_name("sim_vida_comportamiento_forzar"))) int sim_vida_comportamiento_forzar() {
  const int i = g_sistema.config().vida.indiceComportamiento(reinterpret_cast<const char*>(g_intercambio));
  return g_sistema.motor().comportamiento(i, 0) ? 1 : 0;
}
__attribute__((export_name("sim_seguir"))) void sim_seguir(float x, float y) { g_sistema.seguir(x, y); }
__attribute__((export_name("sim_depurar"))) void sim_depurar(int si) { g_sistema.depurar(si != 0); }
}
