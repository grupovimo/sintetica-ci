// La v1.9 con el sistema entero y un HAL falso: los toques de los amigos (y
// devolverlos), la agenda, la presencia, los hábitos (la pregunta a su hora),
// la música y los avisos de afuera.
#include <algorithm>
#include <map>
#include <string>
#include <vector>

#include "prueba.h"
#include "protocolo.h"
#include "rostro.h"
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
uint32_t g_epoch = 0;
std::map<std::string, std::vector<uint8_t>> g_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> g_notis;
std::vector<std::string> g_sonidos;
uint16_t g_px[320 * 172];
Sistema g_s;

// Lunes 9 de noviembre de 2026, 07:29:00 (hora local).
constexpr uint32_t LUNES_0729 = 1794209340u;
constexpr uint32_t LUNES_1000 = LUNES_0729 + 151 * 60;

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

// Sin el "¿cómo estás?" ni frases espontáneas: solo lo de la v1.9.
void arrancar(const std::string& extra = "", uint32_t epoch = LUNES_0729 + 3 * 3600) {
  const std::string config = "{\"version\":2,\"cuidado\":{\"animo\":false},\"habla\":{\"frecuencia\":0}" + extra + "}";
  g_flash.clear();
  g_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  g_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  g_notis.clear();
  g_sonidos.clear();
  g_ahora = 1000;
  g_epoch = epoch;
  g_s.iniciar(halFalso(), g_px, 320, 172);
  g_s.confiar();
  for (int i = 0; i < 25; i++) g_ahora += 20, g_s.tick();
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

void escribir(const std::vector<uint8_t>& m) { g_s.bleEscribir(CarComando, m.data(), m.size()); }

uint8_t ultimaRespuesta(uint8_t op) {
  for (auto it = g_notis.rbegin(); it != g_notis.rend(); ++it)
    if (it->first == CarComando && it->second.size() >= 2 && it->second[0] == (op | Respuesta)) return it->second[1];
  return 0xEE;
}

void conTexto(std::vector<uint8_t>& m, const std::string& s) {
  m.push_back(static_cast<uint8_t>(s.size()));
  m.insert(m.end(), s.begin(), s.end());
}

std::vector<uint8_t> marcoToque(uint8_t tipo, const std::string& de, const std::string& ex = "") {
  std::vector<uint8_t> m = {CmdToque, tipo};
  conTexto(m, de);
  conTexto(m, ex);
  return m;
}

std::vector<uint8_t> marcoAgenda(uint16_t minutos, uint16_t duracion, const std::string& titulo) {
  std::vector<uint8_t> m = {CmdAgenda, static_cast<uint8_t>(minutos), static_cast<uint8_t>(minutos >> 8),
                            static_cast<uint8_t>(duracion), static_cast<uint8_t>(duracion >> 8)};
  conTexto(m, titulo);
  return m;
}

std::vector<uint8_t> marcoMusica(uint8_t bpm, bool sonando, const std::string& titulo, const std::string& artista) {
  std::vector<uint8_t> m = {CmdMusica, bpm, static_cast<uint8_t>(sonando ? 1 : 0)};
  conTexto(m, titulo);
  conTexto(m, artista);
  return m;
}

bool globoDice(const std::string& parte, int ms = 3000) {
  for (int i = 0; i < ms / 50; i++) {
    correr(50, 50);
    if (g_s.panel() == Sistema::Panel::Globo && std::string(g_s.textoGlobo()).find(parte) != std::string::npos) return true;
  }
  std::printf("    globo: \"%s\" (esperaba \"%s\")\n", g_s.textoGlobo(), parte.c_str());
  return false;
}

int cuantos(const std::string& nombre) {
  int n = 0;
  for (auto& s : g_sonidos) n += s == nombre;
  return n;
}

// El aviso [0x9E][0][2][evento][tipo con su dato] que el robot manda solo.
bool avisoCon(Evento e, uint8_t tipo, int dato = -1) {
  for (auto& x : g_notis) {
    const auto& b = x.second;
    if (x.first == CarComando && b.size() == 15 && b[0] == (CmdRegistro | Respuesta) && b[2] == 2 &&
        b[8] == static_cast<uint8_t>(e) && b[13] == tipo && (dato < 0 || b[14] == dato))
      return true;
  }
  return false;
}

bool enRegistro(uint8_t tipo, int dato) {
  EntradaRegistro r[MAX_REGISTRO];
  const int n = g_s.motor().registro(r, MAX_REGISTRO);
  for (int i = 0; i < n; i++)
    if (r[i].tipo == tipo && r[i].dato == dato) return true;
  return false;
}

bool comportamiento(const char* nombre, int ms = 500) {
  for (int i = 0; i < ms / 20; i++) {
    if (std::string(g_s.motor().nombreComportamiento()) == nombre) return true;
    correr(20);
  }
  return false;
}
}  // namespace

PRUEBA("compañía: habitos y presencia en la config (validados por el núcleo)") {
  VERIFICAR(cargar("{\"version\":2,\"habitos\":[{\"nombre\":\"Tomar agua\",\"icono\":\"gota\",\"hora\":\"10:00\",\"dias\":\"LMXJV\"},"
                   "{\"nombre\":\"Leer\",\"icono\":\"libro\"}],\"presencia\":{\"preguntar\":false,\"texto\":\"¿Y el paraguas?\"}}")
                .ok());
  VERIFICAR(g_cfg.habitos.n == 2 && g_cfg.habitos.lista[0].conHora && g_cfg.habitos.lista[0].minuto == 600);
  VERIFICAR(g_cfg.habitos.lista[0].dias == 0x1F && g_cfg.habitos.lista[0].icono == IconoHabito::Gota);
  VERIFICAR(!g_cfg.habitos.lista[1].conHora && g_cfg.habitos.lista[1].icono == IconoHabito::Libro);
  VERIFICAR(!g_cfg.presencia.preguntar && std::string(g_cfg.presencia.texto) == "¿Y el paraguas?");
  VERIFICAR(cargar("{\"version\":2}").ok() && g_cfg.presencia.preguntar && g_cfg.habitos.n == 0);
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"a\"},{\"nombre\":\"b\"},{\"nombre\":\"c\"},{\"nombre\":\"d\"},{\"nombre\":\"e\"}]}",
                    "habitos: hasta 4"));
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"\"}]}", "habitos: nombre de 1 a 16 bytes"));
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"áááááááá1\"}]}", "habitos: nombre de 1 a 16 bytes"));
  VERIFICAR(cargar("{\"version\":2,\"habitos\":[{\"nombre\":\"áááááááá\"}]}").ok());
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"a\",\"icono\":\"perro\"}]}", "habitos: icono"));
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"a\",\"hora\":\"25:00\"}]}", "habitos: hora es HH:MM"));
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"a\",\"dias\":\"LL\"}]}", "habitos: dias"));
  VERIFICAR(rechaza("{\"version\":2,\"habitos\":[{\"nombre\":\"a\",\"color\":1}]}", "habitos: claves válidas"));
  VERIFICAR(rechaza("{\"version\":2,\"presencia\":{\"preguntar\":1}}", "presencia: preguntar es true o false"));
  VERIFICAR(rechaza("{\"version\":2,\"presencia\":{\"texto\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}}", "presencia: texto de hasta 40 bytes"));
  VERIFICAR(rechaza("{\"version\":2,\"presencia\":[]}", "presencia: no es un objeto"));
  std::printf("    RAM: Habitos %zu B, Presencia %zu B\n", sizeof(Habitos), sizeof(Presencia));
  VERIFICAR(sizeof(Habitos) + sizeof(Presencia) <= 140);
}

PRUEBA("compañía: un toque de un amigo lo despierta, reacciona según el tipo y se devuelve en 30 s") {
  arrancar();
  g_s.motor().dormir(g_ahora);
  correr(200);
  escribir(marcoToque(1, "Ana"));
  VERIFICAR(ultimaRespuesta(CmdToque) == Ok);
  VERIFICAR(!g_s.motor().dormido());
  VERIFICAR(cuantos("abrazo") == 1);
  VERIFICAR(globoDice("Ana"));
  VERIFICAR(std::string(g_s.motor().nombreVisible()) == "verguenza");  // se achica y se pone colorado
  VERIFICAR(enRegistro(7, 1) && enRegistro(0, static_cast<int>(Evento::ToqueRecibido)));
  VERIFICAR(g_s.toqueAbierto());
  // Un toque lo devuelve: el aviso 0x9E (evento 40 + la entrada de tipo 7) y un globo.
  g_notis.clear();
  tocar();
  VERIFICAR(avisoCon(Evento::DevolverToque, 7, 1));
  VERIFICAR(!g_s.toqueAbierto());
  VERIFICAR(globoDice("Ana"));
  // Pasados los 30 s ya no se devuelve.
  escribir(marcoToque(2, "Martín"));
  VERIFICAR(cuantos("beso") == 1);
  correr(31000, 50);
  VERIFICAR(!g_s.toqueAbierto());
  g_notis.clear();
  tocar();
  VERIFICAR(!avisoCon(Evento::DevolverToque, 7));
  correr(1000);
  // Una caricia (aunque llegue como evento) también lo devuelve.
  escribir(marcoToque(0, "Ana"));
  correr(500);
  g_notis.clear();
  escribir({CmdEvento, static_cast<uint8_t>(Evento::Caricia)});
  VERIFICAR(avisoCon(Evento::DevolverToque, 7, 0));
  // Carita: con esa expresión (si existe).
  correr(3000);
  escribir(marcoToque(3, "Ana", "enamorado"));
  VERIFICAR(ultimaRespuesta(CmdToque) == Ok);
  correr(800);
  VERIFICAR(std::string(g_s.motor().nombreVisible()) == "enamorado");
  escribir(marcoToque(3, "Ana", "no-existe"));
  VERIFICAR(ultimaRespuesta(CmdToque) == NoExiste);
  // Mal armados.
  escribir(marcoToque(4, "Ana"));
  VERIFICAR(ultimaRespuesta(CmdToque) == Malformado);
  escribir(marcoToque(0, ""));
  VERIFICAR(ultimaRespuesta(CmdToque) == Malformado);
  escribir({CmdToque, 0, 30, 'A'});
  VERIFICAR(ultimaRespuesta(CmdToque) == Malformado);
}

PRUEBA("compañía: la agenda (apurado de 0 a 15 min) y el resumen de mañana") {
  arrancar();
  escribir(marcoAgenda(10, 30, "Reunión"));
  VERIFICAR(ultimaRespuesta(CmdAgenda) == Ok);
  VERIFICAR(comportamiento("apurado"));
  VERIFICAR(globoDice("Reunión"));
  VERIFICAR(std::string(g_s.textoGlobo()).find("10") != std::string::npos);
  VERIFICAR(enRegistro(0, static_cast<int>(Evento::Agenda)));
  VERIFICAR(cuantos("agenda") == 1);  // el sonido es del comportamiento (datos)
  // Con más de 15 minutos no hace nada.
  correr(12000, 50);
  escribir(marcoAgenda(40, 30, "Dentista"));
  VERIFICAR(ultimaRespuesta(CmdAgenda) == Ok);
  correr(1000);
  VERIFICAR(std::string(g_s.textoGlobo()).find("Dentista") == std::string::npos);
  // El resumen de mañana: minutos = 0xFFFF y la cantidad en la duración.
  escribir(marcoAgenda(0xFFFF, 3, "Dentista"));
  VERIFICAR(globoDice("Dentista"));
  escribir({CmdAgenda, 1, 0, 1, 0, 0});
  VERIFICAR(ultimaRespuesta(CmdAgenda) == Malformado);
}

PRUEBA("compañía: presencia: te vas (¿llevás las llaves?) y llegaste") {
  arrancar();
  escribir({CmdPresencia, 1});
  VERIFICAR(ultimaRespuesta(CmdPresencia) == Ok);
  VERIFICAR(enRegistro(0, static_cast<int>(Evento::TeFuiste)));
  VERIFICAR(globoDice(""));
  const float antes = g_s.motor().variable(VarVida::Aburrimiento);
  correr(16 * 60000, 200);  // un rato después, se aburre
  VERIFICAR(g_s.motor().variable(VarVida::Aburrimiento) > antes);
  escribir({CmdPresencia, 0});
  VERIFICAR(enRegistro(0, static_cast<int>(Evento::Llegaste)));
  VERIFICAR(globoDice(""));
  // El texto propio, y sin preguntar no dice nada.
  arrancar(",\"presencia\":{\"texto\":\"¿Y el paraguas?\"}");
  escribir({CmdPresencia, 1});
  VERIFICAR(globoDice("paraguas"));
  arrancar(",\"presencia\":{\"preguntar\":false}");
  escribir({CmdPresencia, 1});
  correr(1500);
  VERIFICAR(g_s.panel() != Sistema::Panel::Globo);
  escribir({CmdPresencia, 2});
  VERIFICAR(ultimaRespuesta(CmdPresencia) == Malformado);
}

PRUEBA("compañía: el hábito a su hora: toque = hecho (aviso 44), mantener = todavía no") {
  const std::string habitos = ",\"habitos\":[{\"nombre\":\"agua\",\"icono\":\"gota\",\"hora\":\"10:00\"},{\"nombre\":\"Leer\"}]";
  arrancar(habitos, LUNES_1000 - 30);
  VERIFICAR(g_s.habitoPreguntando() < 0);
  correr(31000, 50);
  VERIFICAR(g_s.habitoPreguntando() == 0);
  VERIFICAR(globoDice("\xC2\xBF" "agua?"));
  // El globo se queda mientras espera la respuesta.
  correr(15000, 50);
  VERIFICAR(g_s.panel() == Sistema::Panel::Globo && g_s.habitoPreguntando() == 0);
  g_notis.clear();
  tocar();
  VERIFICAR(g_s.habitoPreguntando() < 0);
  VERIFICAR(avisoCon(Evento::Habito, 8, 0));
  VERIFICAR(comportamiento("orgulloso"));
  VERIFICAR(cuantos("habito") >= 1);
  VERIFICAR(globoDice(""));
  // Hecho: no vuelve a preguntar hoy.
  correr(5 * 60000, 200);
  VERIFICAR(g_s.habitoPreguntando() < 0);

  // Mantener = todavía no; pregunta otra vez en una hora.
  arrancar(habitos, LUNES_1000);
  correr(1000);
  VERIFICAR(g_s.habitoPreguntando() == 0);
  correr(500);
  g_notis.clear();
  tocar(1000);
  VERIFICAR(g_s.habitoPreguntando() < 0 && !avisoCon(Evento::Habito, 8));
  correr(59 * 60000, 500);
  VERIFICAR(g_s.habitoPreguntando() < 0);
  correr(90000, 500);
  VERIFICAR(g_s.habitoPreguntando() == 0);
  // Sin respuesta, se olvida al minuto.
  correr(61000, 100);
  VERIFICAR(g_s.habitoPreguntando() < 0);

  // Por comando (la consola, Telegram): con racha de 3 o más lo festeja; no avisa (ya lo sabe la nube).
  arrancar(habitos, LUNES_0729);
  g_notis.clear();
  escribir({CmdHabito, 1, 0, 5});
  VERIFICAR(ultimaRespuesta(CmdHabito) == Ok);
  VERIFICAR(enRegistro(8, 1) && !avisoCon(Evento::Habito, 8));
  VERIFICAR(globoDice("5"));
  escribir({CmdHabito, 1, 1, 4});
  VERIFICAR(ultimaRespuesta(CmdHabito) == Ok);
  escribir({CmdHabito, 2, 0, 0});
  VERIFICAR(ultimaRespuesta(CmdHabito) == NoExiste);
  escribir({CmdHabito, 4, 0, 0});
  VERIFICAR(ultimaRespuesta(CmdHabito) == Malformado);
}

PRUEBA("compañía: la música mueve la cabeza al ritmo, entra y sale suave") {
  arrancar();
  escribir(marcoMusica(120, true, "Muchachos", "La Mosca"));
  VERIFICAR(ultimaRespuesta(CmdMusica) == Ok);
  VERIFICAR(g_s.motor().ritmoMusica() == 120);
  VERIFICAR(comportamiento("bailar-ritmo"));
  correr(2000);
  VERIFICAR(g_s.motor().pesoRitmo() > 0.99f);
  // La inclinación va y viene con el ritmo.
  float lo = 1, hi = -1;
  for (int i = 0; i < 60; i++) {
    correr(20);
    const float t = g_s.motor().parametro(param::FTILT);
    lo = std::min(lo, t), hi = std::max(hi, t);
  }
  VERIFICAR(hi - lo > 0.06f);
  // Sin el bit de sonando: deja de bailar, de a poco (nunca un corte).
  escribir(marcoMusica(0, false, "", ""));
  correr(100);
  VERIFICAR(g_s.motor().pesoRitmo() > 0.5f && g_s.motor().pesoRitmo() < 1);
  correr(1500);
  VERIFICAR(g_s.motor().pesoRitmo() == 0);
  // bpm 0: no se sabe, baila a 100.
  escribir(marcoMusica(0, true, "", ""));
  VERIFICAR(g_s.motor().ritmoMusica() == 100);
  escribir({CmdMusica, 100});
  VERIFICAR(ultimaRespuesta(CmdMusica) == Malformado);
}

PRUEBA("compañía: los avisos de afuera (pago, mail, github…) con su frase y su sonido") {
  arrancar();
  std::vector<uint8_t> m = {CmdAviso, 2};
  conTexto(m, "$12.000");
  conTexto(m, "Ana");
  escribir(m);
  VERIFICAR(ultimaRespuesta(CmdAviso) == Ok);
  VERIFICAR(globoDice("$12.000"));
  VERIFICAR(cuantos("pago") == 1);
  correr(12000, 50);
  m = {CmdAviso, 4};
  conTexto(m, "PR #12");
  conTexto(m, "");
  escribir(m);
  VERIFICAR(globoDice("PR #12"));
  VERIFICAR(cuantos("notificacion") == 1);
  escribir({CmdAviso, 7, 0, 0});
  VERIFICAR(ultimaRespuesta(CmdAviso) == Malformado);
  // La partícula del reloj (la del apuro) existe.
  TipoParticula t;
  VERIFICAR(particulaPorNombre("reloj", &t) && t == TipoParticula::Reloj);
}
