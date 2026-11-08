// Recordatorios, voz, reloj y pomodoro: el parseo de la config v2 y lo que
// hace el sistema (con un HAL falso y la hora que manda la app).
#include <map>
#include <string>
#include <vector>

#include "formato_vida.h"
#include "prueba.h"
#include "protocolo.h"
#include "sistema.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
Config g_cfg;

ResultadoConfig cargar(const std::string& json) { return cargarConfig(g_cfg, json.c_str(), json.size()); }

std::string conRecordatorios(const std::string& lista) { return "{\"version\":2,\"recordatorios\":[" + lista + "]}"; }

bool rechaza(const std::string& json, const char* parteDelError) {
  ResultadoConfig r = cargar(json);
  if (r.ok()) return false;
  if (std::string(r.detalle).find(parteDelError) == std::string::npos) {
    std::printf("    detalle: %s\n", r.detalle);
    return false;
  }
  return true;
}

// El sistema con un HAL falso.
uint32_t g_ahora = 0;
std::map<std::string, std::vector<uint8_t>> g_flash;
std::map<std::string, std::vector<uint8_t>> g_archivos;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_notis;
std::vector<std::string> g_sonidos;
uint16_t g_px[320 * 172];
Sistema g_s;

// El lunes 9 de noviembre de 2026, 08:29:30 (hora local: la zona va en 0).
constexpr uint32_t LUNES_0829 = 1794212970u;

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
    std::copy(it->second.begin() + off, it->second.begin() + off + m, d);
    return static_cast<int>(m);
  };
  return h;
}

// Arranca con esta config guardada (y una clave vinculada, así no muestra el QR).
void arrancar(const std::string& config) {
  g_flash.clear();
  g_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  g_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  g_archivos.clear();
  g_notis.clear();
  g_sonidos.clear();
  g_ahora = 1000;
  g_s.iniciar(halFalso(), g_px, 320, 172);
}

void correr(uint32_t ms) {
  for (uint32_t t = 0; t < ms; t += 20) {
    g_ahora += 20;
    g_s.tick();
  }
}

void tocar() {
  g_s.tacto(true);
  correr(60);
  g_s.tacto(false);
}

std::string expresionDelEstado() {
  uint8_t b[MAX_ESTADO];
  const size_t n = g_s.estadoBle(b, sizeof b);
  if (n < 6) return "";
  return std::string(reinterpret_cast<const char*>(b + 6), b[5]);
}

// Cuántos avisos de este evento (una respuesta de Registro que llega sola) hubo.
// Un recordatorio trae además una entrada de tipo 4 con su índice.
int eventosNotificados(Evento e, int indice = -1) {
  int n = 0;
  for (auto& x : g_notis) {
    const auto& b = x.second;
    if (x.first != CarComando || b.size() < 9 || b[0] != (CmdRegistro | Respuesta) || b.size() != 3u + 6u * b[2]) continue;
    if (b[7] != 0 || b[8] != static_cast<uint8_t>(e)) continue;
    if (e == Evento::Recordatorio && (b[2] != 2 || b[13] != 4 || (indice >= 0 && b[14] != indice))) continue;
    n++;
  }
  return n;
}

int cuantos(const std::string& sonido) {
  int n = 0;
  for (auto& s : g_sonidos) n += s == sonido;
  return n;
}

// Píxeles claros (texto) en una franja de filas.
int clarosEntre(int y0, int y1) {
  int n = 0;
  for (int y = y0; y < y1; y++)
    for (int x = 0; x < 320; x++) {
      const uint32_t c = de565(g_px[y * 320 + x]);
      if (((c >> 16) & 0xFF) > 0xC0 && ((c >> 8) & 0xFF) > 0xC0 && (c & 0xFF) > 0xB0) n++;
    }
  return n;
}

const char* const UN_RECORDATORIO =
    "{\"version\":2,\"recordatorios\":[{\"hora\":\"08:30\",\"dias\":\"L\",\"texto\":\"Tomar la pastilla\"}]}";
}  // namespace

// ---------------------------------------------------------------- parseo

PRUEBA("recordatorios: se leen compactos, con los valores por defecto") {
  ResultadoConfig r = cargar(conRecordatorios(
      "{\"hora\":\"08:30\",\"dias\":\"LMXJV\",\"texto\":\"Tomar la pastilla\",\"expresion\":\"contento\",\"sonido\":false,"
      "\"activo\":false},"
      "{\"hora\":\"23:59\",\"texto\":\"Regá las plantas, ñandú\"},"
      "{\"hora\":\"00:00\",\"dias\":\"DSVJXML\",\"texto\":\"x\"},"
      "{\"hora\":\"12:00\",\"dias\":\"\",\"texto\":\"y\"}"));
  VERIFICAR(r.ok());
  VERIFICAR(g_cfg.nRecordatorios == 4);
  const Recordatorio& a = g_cfg.recordatorios[0];
  VERIFICAR(a.minuto == 8 * 60 + 30);
  VERIFICAR(a.dias == 0x1F);
  VERIFICAR(std::string(a.texto) == "Tomar la pastilla");
  VERIFICAR(!a.sonido && !a.activo);
  VERIFICAR(a.caraVida == g_cfg.resolverCara("contento").vida);
  const Recordatorio& b = g_cfg.recordatorios[1];
  VERIFICAR(b.minuto == 23 * 60 + 59 && b.dias == 0x7F && b.sonido && b.activo);
  VERIFICAR(std::string(b.texto) == "Regá las plantas, ñandú");
  VERIFICAR(b.caraVida == g_cfg.resolverCara("sorpresa").vida && b.caraVida >= 0);
  VERIFICAR(g_cfg.recordatorios[2].dias == 0x7F);
  VERIFICAR(g_cfg.recordatorios[3].dias == 0x7F);
  // 40 bytes con tildes entran justo (20 × "á").
  std::string t;
  for (int i = 0; i < 20; i++) t += "á";
  VERIFICAR(cargar(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"" + t + "\"}")).ok());
  // Compacto: el broche no tiene PSRAM.
  VERIFICAR(sizeof(Recordatorio) <= 48);
  VERIFICAR(sizeof(Voz) <= 24);
}

PRUEBA("recordatorios: los inválidos se rechazan con un error claro") {
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"8:30\",\"texto\":\"a\"}"), "hora es HH:MM"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"24:00\",\"texto\":\"a\"}"), "hora es HH:MM"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:60\",\"texto\":\"a\"}"), "hora es HH:MM"));
  VERIFICAR(rechaza(conRecordatorios("{\"texto\":\"a\"}"), "hora es HH:MM"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"dias\":\"LL\",\"texto\":\"a\"}"), "sin repetir"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"dias\":\"LMA\",\"texto\":\"a\"}"), "dias son letras"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"dias\":\"l\",\"texto\":\"a\"}"), "dias son letras"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\"}"), "texto de 1 a 40"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"\"}"), "texto de 1 a 40"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"" + std::string(41, 'a') + "\"}"), "texto de 1 a 40"));
  std::string t;
  for (int i = 0; i < 21; i++) t += "á";  // 21 letras, 42 bytes
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"" + t + "\"}"), "texto de 1 a 40"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"a\",\"expresion\":\"no-existe\"}"), "no existe"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"a\",\"sonido\":1}"), "true o false"));
  VERIFICAR(rechaza(conRecordatorios("{\"hora\":\"08:30\",\"texto\":\"a\",\"color\":\"#FFFFFF\"}"), "claves válidas"));
  VERIFICAR(rechaza("{\"version\":2,\"recordatorios\":{}}", "es una lista"));
  std::string trece;
  for (int i = 0; i < 13; i++) trece += std::string(i ? "," : "") + "{\"hora\":\"08:30\",\"texto\":\"a\"}";
  VERIFICAR(rechaza(conRecordatorios(trece), "hasta 12"));
  std::string doce;
  for (int i = 0; i < 12; i++) doce += std::string(i ? "," : "") + "{\"hora\":\"08:30\",\"texto\":\"a\"}";
  VERIFICAR(cargar(conRecordatorios(doce)).ok() && g_cfg.nRecordatorios == 12);
  // Si falla, queda la base: sin recordatorios.
  VERIFICAR(!cargar(conRecordatorios(trece)).ok() && g_cfg.nRecordatorios == 0);
  uint8_t m = 0;
  VERIFICAR(leerDias("", &m) && m == 0x7F);
  VERIFICAR(leerDias("SD", &m) && m == 0x60);
}

PRUEBA("voz: se valida y se guarda") {
  VERIFICAR(cargar("{\"version\":2,\"voz\":{\"volumen\":0.7,\"tipo\":\"robot\",\"palabra\":\"Lucho\",\"conversar\":true,"
                   "\"microfono\":0.6}}")
                .ok());
  VERIFICAR(g_cfg.voz.volumen == 70 && g_cfg.voz.microfono == 60 && g_cfg.voz.tipo == Voz::Tipo::Robot);
  VERIFICAR(g_cfg.voz.conversar && std::string(g_cfg.voz.palabra) == "Lucho");
  // Todo opcional; lo que no viene queda por defecto.
  VERIFICAR(cargar("{\"version\":2,\"voz\":{}}").ok());
  VERIFICAR(g_cfg.voz.volumen == 70 && g_cfg.voz.tipo == Voz::Tipo::Media && !g_cfg.voz.conversar && !g_cfg.voz.palabra[0]);
  VERIFICAR(cargar("{\"version\":2,\"voz\":{\"volumen\":0}}").ok() && g_cfg.voz.volumen == 0);
  VERIFICAR(cargar("{\"version\":2,\"voz\":{\"volumen\":0.001}}").ok() && g_cfg.voz.volumen == 1);  // 0 es solo 0
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"volumen\":1.5}}", "volumen de 0 a 1"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"microfono\":-0.1}}", "microfono de 0 a 1"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"tipo\":\"chillona\"}}", "grave, media, aguda o robot"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"palabra\":\"\"}}", "palabra de 1 a 16"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"palabra\":\"" + std::string(17, 'a') + "\"}}", "palabra de 1 a 16"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"conversar\":\"si\"}}", "conversar es true o false"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":{\"idioma\":\"es\"}}", "claves válidas"));
  VERIFICAR(rechaza("{\"version\":2,\"voz\":[]}", "no es un objeto"));
}

PRUEBA("pomodoro: los tramos se validan") {
  VERIFICAR(cargar("{\"version\":2}").ok() && g_cfg.pomodoro.trabajo == 25 && g_cfg.pomodoro.descanso == 5);
  VERIFICAR(cargar("{\"version\":2,\"pomodoro\":{\"trabajo\":50,\"descanso\":10}}").ok());
  VERIFICAR(g_cfg.pomodoro.trabajo == 50 && g_cfg.pomodoro.descanso == 10);
  VERIFICAR(rechaza("{\"version\":2,\"pomodoro\":{\"trabajo\":0}}", "trabajo en minutos"));
  VERIFICAR(rechaza("{\"version\":2,\"pomodoro\":{\"trabajo\":121}}", "trabajo en minutos"));
  VERIFICAR(rechaza("{\"version\":2,\"pomodoro\":{\"trabajo\":2.5}}", "trabajo en minutos"));
  VERIFICAR(rechaza("{\"version\":2,\"pomodoro\":{\"descanso\":61}}", "descanso en minutos"));
  VERIFICAR(rechaza("{\"version\":2,\"pomodoro\":{\"largo\":3}}", "claves válidas"));
}

// ---------------------------------------------------------------- en el sistema

PRUEBA("recordatorios: se dispara a la hora en el día que corresponde") {
  arrancar(UN_RECORDATORIO);
  g_s.hora(LUNES_0829, 0);
  correr(20000);  // 08:29:50
  VERIFICAR(g_s.recordatorioVisible() == -1);
  correr(15000);  // 08:30:05
  VERIFICAR(g_s.recordatorioVisible() == 0);
  VERIFICAR(cuantos("recordatorio") == 1);
  VERIFICAR(eventosNotificados(Evento::Recordatorio, 0) == 1);
  VERIFICAR(expresionDelEstado() == "recordatorio:0");
  VERIFICAR(std::string(g_s.motor().nombreVisible()) == "sorpresa");
  // Entra con un fundido, sin saltos.
  VERIFICAR(g_s.panel() == Sistema::Panel::Recordatorio && g_s.fasePanel() >= 0.999f);
  // El texto se ve abajo, debajo de la cara chiquita.
  VERIFICAR(clarosEntre(95, 165) > 150);
  // En el registro, como evento.
  EntradaRegistro r[MAX_REGISTRO];
  const int n = g_s.motor().registro(r, MAX_REGISTRO);
  bool hay = false, conIndice = false;
  for (int i = 0; i < n; i++) {
    hay = hay || (r[i].tipo == 0 && r[i].dato == static_cast<uint8_t>(Evento::Recordatorio));
    conIndice = conIndice || (r[i].tipo == 4 && r[i].dato == 0);
  }
  VERIFICAR(hay && conIndice);

  // El martes no ("dias": "L").
  arrancar(UN_RECORDATORIO);
  g_s.hora(LUNES_0829 + 86400, 0);
  correr(60000);
  VERIFICAR(g_s.recordatorioVisible() == -1);
  VERIFICAR(cuantos("recordatorio") == 0 && eventosNotificados(Evento::Recordatorio) == 0);
  // Ni uno apagado.
  arrancar("{\"version\":2,\"recordatorios\":[{\"hora\":\"08:30\",\"texto\":\"a\",\"activo\":false}]}");
  g_s.hora(LUNES_0829, 0);
  correr(60000);
  VERIFICAR(g_s.recordatorioVisible() == -1);
}

PRUEBA("recordatorios: un toque lo cierra con un fundido y no se repite el mismo día") {
  // Callado: si no, a veces festeja con un globo ("¡Bien ahí!").
  arrancar("{\"version\":2,\"habla\":{\"frecuencia\":0},\"recordatorios\":[{\"hora\":\"08:30\",\"dias\":\"L\",\"texto\":\"Tomar la pastilla\"}]}");
  g_s.hora(LUNES_0829, 0);
  correr(35000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  g_notis.clear();
  g_s.tacto(true);
  VERIFICAR(g_s.recordatorioVisible() == -1);
  // Sale de a poco: nunca un corte.
  float antes = g_s.fasePanel();
  bool suave = true;
  for (int i = 0; i < 40; i++) {
    g_ahora += 20;
    g_s.tick();
    suave = suave && antes - g_s.fasePanel() <= 0.06f;
    antes = g_s.fasePanel();
  }
  g_s.tacto(false);
  VERIFICAR(suave);
  VERIFICAR(g_s.panel() == Sistema::Panel::Ninguno && g_s.fasePanel() == 0);
  VERIFICAR(expresionDelEstado() != "recordatorio:0");
  // El toque que lo cerró no hizo nada más (ni caricia ni toque en el registro).
  correr(1000);
  // Si vuelven a poner la hora antes (la app la corrige), no se repite hoy.
  g_s.hora(LUNES_0829 + 20, 0);
  correr(60000);
  VERIFICAR(g_s.recordatorioVisible() == -1);
  VERIFICAR(cuantos("recordatorio") == 1);
  // Al otro lunes, sí.
  g_s.hora(LUNES_0829 + 7 * 86400, 0);
  correr(40000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  VERIFICAR(cuantos("recordatorio") == 2);
}

PRUEBA("recordatorios: sin tocarlo se va a los 2 minutos; volumen 0 lo silencia") {
  arrancar("{\"version\":2,\"recordatorios\":[{\"hora\":\"08:30\",\"texto\":\"Sacar la basura\"}],\"voz\":{\"volumen\":0}}");
  g_s.hora(LUNES_0829, 0);
  correr(35000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  VERIFICAR(cuantos("recordatorio") == 0);  // volumen 0
  VERIFICAR(eventosNotificados(Evento::Recordatorio) == 1);
  correr(110000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  correr(12000);
  VERIFICAR(g_s.recordatorioVisible() == -1);
  correr(1000);
  VERIFICAR(g_s.panel() == Sistema::Panel::Ninguno);
  // Dos a la misma hora: el segundo espera a que se vaya el primero.
  arrancar("{\"version\":2,\"recordatorios\":[{\"hora\":\"08:30\",\"texto\":\"Uno\"},{\"hora\":\"08:30\",\"texto\":\"Dos\","
           "\"sonido\":false}]}");
  g_s.hora(LUNES_0829, 0);
  correr(35000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  tocar();
  correr(1000);
  VERIFICAR(g_s.recordatorioVisible() == 1);
  VERIFICAR(eventosNotificados(Evento::Recordatorio) == 2);
  VERIFICAR(eventosNotificados(Evento::Recordatorio, 1) == 1);
  VERIFICAR(cuantos("recordatorio") == 1);  // el segundo, sin sonido
}

PRUEBA("recordatorios: un texto largo entra en la redonda y en la rectangular") {
  const std::string largo = "{\"version\":2,\"recordatorios\":[{\"hora\":\"08:30\",\"texto\":\"Llamar a la abuela por "
                            "su cumpleaños\"}]}";  // 37 bytes
  VERIFICAR(cargar(largo).ok());
  arrancar(largo);
  g_s.hora(LUNES_0829, 0);
  correr(35000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  // Nada en los bordes izquierdo y derecho (16 px).
  int bordes = 0;
  for (int y = 90; y < 172; y++)
    for (int x = 0; x < 320; x++)
      if ((x < 12 || x >= 308) && de565(g_px[y * 320 + x]) > 0x808080) bordes++;
  VERIFICAR(bordes == 0);
  VERIFICAR(clarosEntre(90, 170) > 200);

  static uint16_t px[240 * 240];
  g_flash["config"] = std::vector<uint8_t>(largo.begin(), largo.end());
  g_ahora = 1000;
  Perfil p;
  p.ancho = 240, p.alto = 240, p.redonda = true, p.boton = Perfil::Boton::Broche;
  g_s.iniciar(halFalso(), px, p);
  g_s.hora(LUNES_0829, 0);
  correr(35000);
  VERIFICAR(g_s.recordatorioVisible() == 0);
  // Todo lo claro queda dentro del círculo.
  int afuera = 0, adentro = 0;
  for (int y = 0; y < 240; y++)
    for (int x = 0; x < 240; x++) {
      const float dx = x + 0.5f - 120, dy = y + 0.5f - 120;
      if (de565(px[y * 240 + x]) <= 0x808080) continue;
      if (dx * dx + dy * dy > 118.0f * 118.0f) afuera++;
      else if (y > 110) adentro++;
    }
  VERIFICAR(afuera == 0);
  VERIFICAR(adentro > 200);
  // En el broche el botón también lo cierra (y no pasa a lo siguiente de la lista).
  const int pos = g_s.posicionLista();
  tocar();
  VERIFICAR(g_s.recordatorioVisible() == -1);
  VERIFICAR(g_s.posicionLista() == pos);
}

PRUEBA("recordatorios: el broche se despierta del sueño profundo para el próximo") {
  arrancar(UN_RECORDATORIO);
  g_s.hora(LUNES_0829 - 3600, 0);  // 07:29:30
  correr(100);
  const int s = g_s.segundosHastaDespertar();
  VERIFICAR(s > 0 && s <= 31 * 60);  // a las 08:00 se levanta; el recordatorio de las 08:30 no lo pasa
  g_s.hora(LUNES_0829, 0);         // 08:29:30
  correr(100);
  VERIFICAR(g_s.segundosHastaDespertar() <= 31);
}

// ---------------------------------------------------------------- modos

PRUEBA("modo reloj: la hora grande con la cara chiquita; --:-- sin hora; un toque muestra la cara") {
  arrancar("{\"version\":2,\"modo\":\"reloj\",\"habla\":{\"frecuencia\":0},\"cuidado\":{\"animo\":false}}");
  correr(1000);
  VERIFICAR(g_s.panel() == Sistema::Panel::Reloj && g_s.fasePanel() >= 0.999f);
  const int sinHora = clarosEntre(85, 150);
  VERIFICAR(sinHora > 50);
  std::vector<uint16_t> guiones(g_px, g_px + 320 * 172);
  g_s.hora(LUNES_0829, 0);
  correr(100);
  const int conHora = clarosEntre(85, 150);
  VERIFICAR(conHora > sinHora);  // "08:29" tiene más tinta que "--:--"
  int distintos = 0;
  for (int y = 85; y < 150; y++)
    for (int x = 0; x < 320; x++) distintos += g_px[y * 320 + x] != guiones[y * 320 + x];
  VERIFICAR(distintos > 100);
  // La cara sigue viva arriba (los ojos, naranjas).
  int naranja = 0;
  for (int y = 0; y < 80; y++)
    for (int x = 0; x < 320; x++) {
      const uint32_t c = de565(g_px[y * 320 + x]);
      if (((c >> 16) & 0xFF) > 0xC0 && ((c >> 8) & 0xFF) < 0xA0) naranja++;
    }
  VERIFICAR(naranja > 50);
  // Un toque: vuelve la cara un rato (con fundido) y regresa al reloj.
  tocar();
  correr(1000);
  VERIFICAR(g_s.panel() == Sistema::Panel::Ninguno);
  correr(15000);
  VERIFICAR(g_s.panel() == Sistema::Panel::Reloj);
  correr(1000);
  VERIFICAR(g_s.fasePanel() >= 0.999f);
}

PRUEBA("modo pomodoro: arranca con un toque, termina el tramo, pasa al descanso y avisa") {
  arrancar("{\"version\":2,\"modo\":\"pomodoro\",\"pomodoro\":{\"trabajo\":1,\"descanso\":2}}");
  correr(1000);
  VERIFICAR(g_s.panel() == Sistema::Panel::Pomodoro);
  VERIFICAR(!g_s.pomodoroCorriendo() && g_s.pomodoroRestanteMs() == 60000);
  VERIFICAR(expresionDelEstado() == "pomodoro:pausa");
  VERIFICAR(clarosEntre(80, 140) > 50);  // "01:00"
  tocar();
  correr(600);  // el toque se confirma al vencer la ventana del doble toque
  VERIFICAR(g_s.pomodoroCorriendo() && !g_s.pomodoroDescanso());
  VERIFICAR(expresionDelEstado() == "pomodoro:trabajo");
  correr(30000);
  VERIFICAR(g_s.pomodoroRestanteMs() < 31000 && g_s.pomodoroRestanteMs() > 28000);
  VERIFICAR(eventosNotificados(Evento::Pomodoro) == 0);
  correr(31000);
  VERIFICAR(g_s.pomodoroCorriendo() && g_s.pomodoroDescanso());
  VERIFICAR(g_s.pomodoroRestanteMs() > 110000 && g_s.pomodoroRestanteMs() <= 120000);
  VERIFICAR(eventosNotificados(Evento::Pomodoro) == 1);
  VERIFICAR(cuantos("pomodoro") == 1);
  VERIFICAR(expresionDelEstado() == "pomodoro:descanso");
  VERIFICAR(std::string(g_s.motor().nombreVisible()) == "sorpresa");
  // Pausa con otro toque.
  tocar();
  correr(600);
  VERIFICAR(!g_s.pomodoroCorriendo());
  const uint32_t quieto = g_s.pomodoroRestanteMs();
  correr(5000);
  VERIFICAR(g_s.pomodoroRestanteMs() == quieto);
  // Sigue y termina el descanso: contento, y vuelve al trabajo.
  tocar();
  correr(125000);
  VERIFICAR(!g_s.pomodoroDescanso() && g_s.pomodoroCorriendo());
  VERIFICAR(eventosNotificados(Evento::Pomodoro) == 2);
}

// ---------------------------------------------------------------- protector de pantalla

namespace {
constexpr uint32_t AZUL = 0x1A50F5;

// Un QOI de un solo color del tamaño de la pantalla, guardado en el espacio 0.
void guardarImagenAzul() {
  std::vector<uint8_t> o = {'q', 'o', 'i', 'f', 0, 0, 1, 64, 0, 0, 0, 172, 3, 0};  // 320 × 172
  o.push_back(0xFE), o.push_back(AZUL >> 16), o.push_back((AZUL >> 8) & 0xFF), o.push_back(AZUL & 0xFF);
  for (int resto = 320 * 172 - 1; resto > 0; resto -= 62) o.push_back(static_cast<uint8_t>(0xC0 | (std::min(resto, 62) - 1)));
  for (int i = 0; i < 7; i++) o.push_back(0);
  o.push_back(1);
  g_archivos["img0"] = o;
  std::vector<uint8_t> meta(8 * 8, 0);
  escribir32(meta.data(), static_cast<uint32_t>(o.size()));
  escribir32(meta.data() + 4, crc32(o.data(), o.size()));
  g_flash["imagenes"] = meta;
}

void arrancarConImagen(const std::string& config) {
  arrancar(config);
  guardarImagenAzul();
  g_s.iniciar(halFalso(), g_px, 320, 172);  // vuelve a leer las imágenes
}

bool centroAzul() { return de565(g_px[86 * 320 + 160]) == de565(a565(AZUL)); }
}  // namespace

PRUEBA("protector: se valida") {
  VERIFICAR(cargar("{\"version\":2,\"protector\":{\"imagen\":3}}").ok());
  VERIFICAR(g_cfg.protector.imagen == 3 && g_cfg.protector.espera == 10);
  VERIFICAR(cargar("{\"version\":2,\"protector\":{\"imagen\":0,\"espera\":3600}}").ok() && g_cfg.protector.espera == 3600);
  VERIFICAR(cargar("{\"version\":2}").ok() && g_cfg.protector.imagen == -1);
  VERIFICAR(rechaza("{\"version\":2,\"protector\":{}}", "imagen es el espacio"));
  VERIFICAR(rechaza("{\"version\":2,\"protector\":{\"imagen\":8}}", "imagen es el espacio"));
  VERIFICAR(rechaza("{\"version\":2,\"protector\":{\"imagen\":0,\"espera\":2}}", "espera en segundos"));
  VERIFICAR(rechaza("{\"version\":2,\"protector\":{\"imagen\":0,\"espera\":3601}}", "espera en segundos"));
  VERIFICAR(rechaza("{\"version\":2,\"protector\":{\"imagen\":0,\"color\":1}}", "claves válidas"));
}

PRUEBA("protector: entra tras la espera, sale con un toque (con fundido) y vuelve") {
  // (Callado: una frase suelta también cuenta como "que pase algo".)
  arrancarConImagen("{\"version\":2,\"habla\":{\"frecuencia\":0},\"protector\":{\"imagen\":0,\"espera\":5}}");
  correr(4000);
  VERIFICAR(g_s.faseProtector() == 0);
  VERIFICAR(!centroAzul());
  correr(1200);  // pasaron los 5 s: empieza a entrar
  VERIFICAR(g_s.faseProtector() > 0);
  correr(600);
  VERIFICAR(g_s.faseProtector() >= 1);
  VERIFICAR(g_s.imagenVisible() == 0 && centroAzul());
  // Un toque: vuelve la cara, de a poco.
  g_s.tacto(true);
  correr(100);
  g_s.tacto(false);
  VERIFICAR(g_s.faseProtector() > 0 && g_s.faseProtector() < 1);
  VERIFICAR(!centroAzul());  // a mitad de camino: mezcla
  correr(600);
  VERIFICAR(g_s.faseProtector() == 0 && g_s.imagenVisible() == -1);
  VERIFICAR(!centroAzul());
  // Tras la espera sin nada, vuelve.
  correr(6000);
  VERIFICAR(g_s.faseProtector() >= 1 && centroAzul());
  // Un comando de la app (una expresión fija) también lo saca.
  std::vector<uint8_t> v{CmdExpresion};
  const std::string n = "contento";
  v.insert(v.end(), n.begin(), n.end());
  g_s.bleEscribir(CarComando, v.data(), v.size());
  correr(600);
  VERIFICAR(g_s.faseProtector() == 0);
}

PRUEBA("protector: sin la imagen no hace nada; un recordatorio le gana") {
  arrancar("{\"version\":2,\"protector\":{\"imagen\":0,\"espera\":3}}");  // sin imagen guardada
  correr(10000);
  VERIFICAR(g_s.faseProtector() == 0 && g_s.imagenVisible() == -1);

  arrancarConImagen("{\"version\":2,\"protector\":{\"imagen\":0,\"espera\":3},\"recordatorios\":[{\"hora\":\"08:30\","
                    "\"texto\":\"Tomar agua\"}]}");
  g_s.hora(LUNES_0829, 0);
  correr(20000);  // 08:29:50: el protector ya está
  VERIFICAR(g_s.faseProtector() >= 1);
  correr(15000);  // 08:30:05
  VERIFICAR(g_s.recordatorioVisible() == 0);
  VERIFICAR(g_s.faseProtector() == 0 && g_s.panel() == Sistema::Panel::Recordatorio);
  correr(60000);  // mientras se ve el recordatorio, no vuelve
  VERIFICAR(g_s.faseProtector() == 0 && g_s.recordatorioVisible() == 0);
  tocar();
  correr(4000);
  VERIFICAR(g_s.faseProtector() >= 1);
}
