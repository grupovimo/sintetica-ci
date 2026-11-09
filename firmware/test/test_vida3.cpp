// El motor de vida v3 (docs/motor-de-vida.md, "v3"): la voz sin texto, los
// tics de la semilla, las etapas, la memoria de lo que pasó y el diario, jugar
// solo, la empatía, el clima, los feriados y las rutinas. Con el sistema
// entero y un HAL falso.
#include <algorithm>
#include <map>
#include <string>
#include <vector>

#include "prueba.h"
#include "protocolo.h"
#include "rostro.h"
#include "sistema.h"
#include "vida_base.h"

using namespace sintetica;
using namespace sintetica::protocolo;

namespace {
uint32_t v3_ahora = 0;
uint32_t v3_epoch = 0;
std::map<std::string, std::vector<uint8_t>> v3_flash;
std::vector<std::pair<uint8_t, std::vector<uint8_t>>> v3_notis;
std::vector<std::string> v3_sonidos;
size_t v3_muestras = 0;
const char* v3_id = "A1B2C3D4E5F6";
uint16_t v3_px[320 * 172];
Sistema v3_s;

// Lunes 9 de noviembre de 2026, 07:29:00 (hora local).
constexpr uint32_t LUNES_0729 = 1794209340u;

Hal halV3(bool parlante) {
  Hal h;
  h.ahoraMs = [] { return v3_ahora; };
  h.semilla = [] { return 7u; };
  h.identificador = [] { return v3_id; };
  h.guardar = [](const char* k, const uint8_t* d, size_t n) {
    v3_flash[k] = std::vector<uint8_t>(d, d + n);
    return true;
  };
  h.leer = [](const char* k, uint8_t* d, size_t cap) {
    auto it = v3_flash.find(k);
    if (it == v3_flash.end() || it->second.size() > cap) return -1;
    std::copy(it->second.begin(), it->second.end(), d);
    return static_cast<int>(it->second.size());
  };
  h.notificar = [](uint8_t c, const uint8_t* d, size_t n) { v3_notis.push_back({c, std::vector<uint8_t>(d, d + n)}); };
  h.sonido = [](const char* s) { v3_sonidos.push_back(s); };
  h.epoch = [] { return v3_epoch ? v3_epoch + (v3_ahora - 1000) / 1000 : 0u; };
  h.minutoDelDia = [] {
    if (!v3_epoch) return -1;
    return static_cast<int>(((v3_epoch + (v3_ahora - 1000) / 1000) % 86400) / 60);
  };
  if (parlante) h.audio = [](const int16_t*, size_t n) { v3_muestras += n; };
  return h;
}

void arrancarV3(bool parlante, const std::string& extra = "", uint32_t epoch = LUNES_0729 + 3 * 3600, bool borrar = true,
                const char* frecuencia = "0") {
  const std::string config = "{\"version\":2,\"cuidado\":{\"animo\":false},\"habla\":{\"frecuencia\":" +
                             std::string(frecuencia) + "}" + extra + "}";
  if (borrar) v3_flash.clear();
  v3_flash["config"] = std::vector<uint8_t>(config.begin(), config.end());
  v3_flash["claves"] = std::vector<uint8_t>(16, 0xAB);
  v3_notis.clear();
  v3_sonidos.clear();
  v3_muestras = 0;
  v3_ahora = 1000;
  v3_epoch = epoch;
  v3_s.iniciar(halV3(parlante), v3_px, 320, 172);
  v3_s.confiar();
  for (int i = 0; i < 25; i++) v3_ahora += 20, v3_s.tick();
}

void correrV3(uint32_t ms, uint32_t paso = 20) {
  for (uint32_t t = 0; t < ms; t += paso) {
    v3_ahora += paso;
    v3_s.tick();
  }
}

bool sono(const char* s) { return std::find(v3_sonidos.begin(), v3_sonidos.end(), s) != v3_sonidos.end(); }
bool sonoAlgunaVoz() {
  for (const std::string& s : v3_sonidos)
    if (s.rfind("voz:", 0) == 0) return true;
  return false;
}
void tararear() {
  const int t = v3_s.config().vida.indiceComportamiento("tararear");
  VERIFICAR(t >= 0);
  v3_s.motor().comportamiento(t, v3_ahora);
  correrV3(800);
}
}  // namespace

// ---------------------------------------------------------------- la voz sin texto

PRUEBA("v3: voz propia 2 (sin texto) balbucea los sonidos y nunca lee un globo") {
  arrancarV3(true);
  v3_s.vozPropia(2);
  VERIFICAR(v3_s.modoVozPropia() == 2);
  v3_s.decir("Hola, esto no lo lee");
  correrV3(3000);
  VERIFICAR(v3_s.globoVisible());
  VERIFICAR(v3_muestras == 0);
  correrV3(9000);
  // El tarareo: por la voz, no por el sonido.
  tararear();
  VERIFICAR(v3_muestras > 0);
  VERIFICAR(!sono("tarareo"));
  VERIFICAR(!sonoAlgunaVoz());
  // Una sorpresa corta ("¡oh!"): por la voz.
  v3_muestras = 0;
  correrV3(3000);
  v3_s.motor().reaccionar(v3_s.config().resolverCara("sorpresa"), v3_ahora, 1500);
  correrV3(600);
  VERIFICAR(v3_muestras > 0);
  // Lo que no está en "voz.sonidos" sigue por el sonido.
  v3_sonidos.clear();
  v3_s.toque(1, "Ana", "");
  correrV3(200);
  VERIFICAR(sono("abrazo"));
}

PRUEBA("v3: voz propia 1 lee los globos y también balbucea; 0, todo por el sonido") {
  arrancarV3(true);
  v3_s.vozPropia(1);  // el de siempre (la instancia lo guarda entre arranques)
  VERIFICAR(v3_s.modoVozPropia() == 1);
  v3_s.decir("Hola");
  correrV3(1500);
  VERIFICAR(v3_muestras > 0);
  correrV3(10000);
  v3_muestras = 0;
  tararear();
  VERIFICAR(v3_muestras > 0 && !sono("tarareo"));

  arrancarV3(true);
  v3_s.vozPropia(0);
  v3_muestras = 0;
  v3_s.decir("Hola");
  correrV3(1500);
  tararear();
  VERIFICAR(v3_muestras == 0);
  VERIFICAR(sono("tarareo"));
  VERIFICAR(!sonoAlgunaVoz());

  // Sin parlante (sin Hal::audio) es como hasta ahora: el sonido.
  arrancarV3(false);
  v3_s.vozPropia(2);
  tararear();
  VERIFICAR(sono("tarareo"));
  VERIFICAR(!sonoAlgunaVoz());
}

PRUEBA("v3: la tabla de la voz sin texto: qué pasa a la voz y qué no") {
  arrancarV3(true);
  v3_s.vozPropia(2);
  for (const char* s : {"murmullo", "risa", "tarareo", "hip", "achis", "ayuda", "llamar", "festejo", "ronroneo"})
    VERIFICAR(v3_s.vocalizacion(s) != nullptr);
  for (const char* s : {"ronquido", "despertador:1", "recordatorio", "pomodoro", "toque", "abrazo", "beso", "agenda", "pago",
                        "habito", "dado", "notificacion", "llamada"})
    VERIFICAR(v3_s.vocalizacion(s) == nullptr);
  VERIFICAR(v3_s.vocalizacion("voz:sorpresa") && v3_s.vocalizacion("voz:pensando") && v3_s.vocalizacion("voz:tacto"));
  v3_s.vozPropia(0);
  VERIFICAR(v3_s.vocalizacion("murmullo") == nullptr);
}

// ---------------------------------------------------------------- los datos

namespace {
constexpr uint32_t LUNES_1000 = 1794218400u;    // 9/11/2026 10:00
constexpr uint32_t MEMORIA_1000 = 1774346400u;  // 24/3/2026 10:00 (el Día de la Memoria)
constexpr uint32_t CARNAVAL_1000 = 1771236000u; // 16/2/2026 10:00 (pascua: 5/4)
constexpr uint32_t OCTUBRE_9 = 1791540000u;     // 9/10/2026 10:00 (un mes antes)

struct SoloMotor {
  static Config& config() {
    static Config c;
    return c;
  }
  MotorVida m;
  uint32_t t = 1000, epoch0 = LUNES_1000;
  explicit SoloMotor(uint32_t semilla = 5) {
    cargarBase(config());
    m.iniciar(&config(), semilla, t);
    m.identidad(semilla * 2654435761u);  // un robot de verdad: con sus tics
  }
  Contexto ctx() const {
    Contexto x;
    x.epoch = epoch0 + (t - 1000) / 1000;
    x.minutoDelDia = static_cast<int>((x.epoch % 86400u) / 60u);
    return x;
  }
  void correr(uint32_t ms, uint32_t paso = 33) {
    const uint32_t fin = t + ms;
    while (t < fin) t += paso, m.tick(t, ctx());
  }
};

uint16_t diaDe(uint32_t epoch) { return static_cast<uint16_t>((epoch - 4 * 3600) / 86400); }

void guardarRecuerdosEn(const Recuerdos& r) {
  uint8_t b[LARGO_RECUERDOS];
  const size_t n = codificarRecuerdos(r, b, sizeof b);
  v3_flash["recuerdos"] = std::vector<uint8_t>(b, b + n);
}

std::vector<std::string> v3_globos;
void correrAnotando(uint32_t ms, uint32_t paso = 50) {
  for (uint32_t t = 0; t < ms; t += paso) {
    v3_ahora += paso;
    v3_s.tick();
    if (v3_s.globoVisible() && (v3_globos.empty() || v3_globos.back() != v3_s.textoGlobo())) v3_globos.push_back(v3_s.textoGlobo());
  }
}
bool dijo(const char* parte) {
  for (const std::string& g : v3_globos)
    if (g.find(parte) != std::string::npos) return true;
  return false;
}
int indiceParametro(const char* n) {
  int lado = 0;
  if (std::string(n).rfind("der.", 0) == 0) lado = 1, n += 4;
  else if (std::string(n).rfind("izq.", 0) == 0) n += 4;
  bool ojo = false;
  const int i = parametroPorNombre(n, &ojo);
  return i < 0 ? -1 : i + (ojo && lado ? param::DER : 0);
}
}  // namespace

PRUEBA("v3: los episodios de caracter.json son los de TipoEpisodio, en orden") {
  static const char* const NOMBRES[] = {"gane", "perdi", "empate", "toque", "animo-bien", "animo-medio", "animo-mal", "tarde",
                                        "posponer", "habito", "caricias", "sacudida", "musica", "volviste", "solo", "crecio"};
  VERIFICAR(caracter::N_EPISODIOS == static_cast<int>(TipoEpisodio::Cantidad));
  for (int i = 0; i < caracter::N_EPISODIOS; i++) VERIFICAR(std::string(caracter::EPISODIOS[i].nombre) == NOMBRES[i]);
  // Cada sueño es una partícula; cada microgesto, un parámetro; cada favorito, un comportamiento.
  for (const EpisodioBase& e : caracter::EPISODIOS) {
    TipoParticula t;
    VERIFICAR(particulaPorNombre(e.sueno, &t));
  }
  for (const MicrogestoBase& g : caracter::MICROGESTOS) VERIFICAR(indiceParametro(g.param) >= 0);
  SoloMotor s;
  for (const char* f : caracter::FAVORITOS) VERIFICAR(SoloMotor::config().vida.indiceComportamiento(f) >= 0);
  std::printf("    RAM: Recuerdos %zu B (en la flash: %zu), Caracter %zu B, FechasTuyas %zu B, Comportamiento %zu B\n",
              sizeof(Recuerdos), LARGO_RECUERDOS, sizeof(Caracter), sizeof(FechasTuyas), sizeof(Comportamiento));
}

PRUEBA("v3: los recuerdos van y vuelven de la flash (y sin la clave, en cero)") {
  Recuerdos r;
  r.episodios[0] = {20000, 3, 2};
  r.nEpisodios = 1;
  r.diario[3] = {20001, DIARIO_TRANQUILO, 0};
  r.nDiario = 4, r.iniDiario = 0;
  r.animos[0] = 3, r.diaAnimos = 20001, r.minLlegada = 645, r.etapa = 2, r.feriados = 0x80000001u, r.juegos[3] = 9;
  uint8_t b[LARGO_RECUERDOS];
  VERIFICAR(codificarRecuerdos(r, b, sizeof b) == LARGO_RECUERDOS);
  Recuerdos x;
  VERIFICAR(decodificarRecuerdos(b, sizeof b, &x));
  VERIFICAR(x.episodios[0].dia == 20000 && x.episodios[0].dato == 2 && x.diario[3].tipo == DIARIO_TRANQUILO);
  VERIFICAR(x.minLlegada == 645 && x.minDormis == 0xFFFF && x.etapa == 2 && x.feriados == 0x80000001u && x.juegos[3] == 9);
  b[10] ^= 1;
  VERIFICAR(!decodificarRecuerdos(b, sizeof b, &x));
  // Un robot de antes (sin "recuerdos"): arranca en cero y sigue andando.
  arrancarV3(false);
  VERIFICAR(v3_s.motor().recuerdos().nEpisodios == 0 && v3_s.motor().etapa() == 0);
}

// ---------------------------------------------------------------- único

PRUEBA("v3: cada robot tiene sus tics (de su id) y son los mismos para siempre") {
  v3_id = "A1B2C3D4E5F6";
  arrancarV3(false);
  const Caracter a = v3_s.motor().caracter();
  VERIFICAR(a.hay);
  v3_id = "0C0FFEE00042";
  arrancarV3(false);
  const Caracter b = v3_s.motor().caracter();
  v3_id = "A1B2C3D4E5F6";
  arrancarV3(false);
  const Caracter c = v3_s.motor().caracter();
  VERIFICAR(a.parpadeo == c.parpadeo && a.favorito == c.favorito && a.microgesto == c.microgesto && a.puerta == c.puerta);
  VERIFICAR(a.parpadeo != b.parpadeo || a.microgesto != b.microgesto || a.favorito != b.favorito);
  VERIFICAR(a.parpadeo >= caracter::PARPADEO_MIN && a.parpadeo <= caracter::PARPADEO_MAX);
  // Distintas semillas: se reparten los microgestos.
  int micros[8] = {};
  SoloMotor s;
  for (uint32_t k = 1; k <= 64; k++) {
    s.m.identidad(k * 7919u);
    micros[s.m.caracter().microgesto]++;
  }
  int distintos = 0;
  for (int i = 0; i < caracter::N_MICROGESTOS; i++) distintos += micros[i] > 0;
  VERIFICAR(distintos >= caracter::N_MICROGESTOS - 1);
  std::printf("    tics de A1B2…: parpadeo ×%.2f, dobles %.0f %%, velocidad ×%.2f, favorito %s, microgesto %s\n", a.parpadeo,
              a.dobles * 100, a.velocidad, caracter::FAVORITOS[a.favorito], caracter::MICROGESTOS[a.microgesto].nombre);
}

PRUEBA("v3: el microgesto propio se ve (por los resortes) cada tanto") {
  SoloMotor s;
  s.m.identidad(12345);
  s.correr(1000);
  const MicrogestoBase& g = caracter::MICROGESTOS[s.m.caracter().microgesto];
  const int i = indiceParametro(g.param);
  // Sin un comportamiento encima: cuántas veces el parámetro se aparta de golpe.
  int veces = 0;
  for (int k = 0; k < 180 * 30; k++) {
    const float antes = s.m.objetivo(i);
    s.correr(33);
    if (std::fabs(s.m.objetivo(i) - antes) > std::fabs(g.valor) * 0.08f) veces++;
  }
  VERIFICAR(veces >= 1);
  std::printf("    microgesto %s (%s): %d cuadros con salto de objetivo en 3 min\n", g.nombre, g.param, veces);
}

// ---------------------------------------------------------------- crece

PRUEBA("v3: crece por etapas (días y afecto), nunca para atrás, y cambia su forma de ser") {
  SoloMotor s;
  const Personalidad base = SoloMotor::config().vida.personalidad;
  Memoria m;
  m.nacimiento = LUNES_1000 - 2 * 86400;
  m.dias = 2, m.ultimoDia = LUNES_1000 / 86400, m.ultimaInteraccion = LUNES_1000;
  s.m.restaurarMemoria(m);
  s.correr(1000);
  VERIFICAR(s.m.etapa() == 0);
  VERIFICAR(std::string(s.m.nombreEtapa()) == "recien-llegado");
  VERIFICAR(s.m.rasgos().timidez > base.timidez + 0.15f);  // tímido y curioso los primeros días
  VERIFICAR(s.m.rasgos().curiosidad > base.curiosidad + 0.1f);
  m.dias = 20, m.estado[2] = static_cast<uint8_t>(0.5f * 255), m.epochEstado = LUNES_1000;
  s.m.restaurarMemoria(m);
  s.correr(1000);
  VERIFICAR(s.m.etapa() == 2);
  VERIFICAR(std::fabs(s.m.rasgos().timidez - base.timidez) < 0.01f);
  VERIFICAR(s.m.ocurrio(diaDe(LUNES_1000), TipoEpisodio::Crecio));
  // Lo que destrabó queda: aunque la memoria vuelva a 1 día.
  m.dias = 1;
  s.m.restaurarMemoria(m);
  s.correr(1000);
  VERIFICAR(s.m.etapa() == 2);
  VERIFICAR(s.m.variable(VarVida::Etapa) > 1.5f);
  // El dibujito (etapa > 0.5) se destraba al crecer.
  const int d = SoloMotor::config().vida.indiceComportamiento("dibujito");
  VERIFICAR(d >= 0 && SoloMotor::config().vida.comportamientos[d].cuando[1].var == VarVida::Etapa);
}

PRUEBA("v3: fechas propias: «hoy hace un mes que nos conocimos»") {
  v3_flash.clear();
  Memoria m;
  m.nacimiento = OCTUBRE_9, m.dias = 12, m.ultimoDia = LUNES_1000 / 86400 - 1, m.ultimaInteraccion = LUNES_1000 - 20 * 3600;
  uint8_t b[LARGO_MEMORIA];
  codificarMemoria(m, b, sizeof b);
  v3_flash["vida"] = std::vector<uint8_t>(b, b + LARGO_MEMORIA);
  arrancarV3(false, ",\"dueno\":{\"nombre\":\"Nahuel\"}", LUNES_1000, false);
  v3_globos.clear();
  v3_s.evento(Evento::Tacto);
  correrAnotando(15000);
  for (const std::string& g : v3_globos) std::printf("    dijo: %s\n", g.c_str());
  VERIFICAR(dijo("un mes"));
  VERIFICAR(v3_s.motor().mesesJuntos() == 1);
}

// ---------------------------------------------------------------- recuerda

PRUEBA("v3: se acuerda de ayer («ayer me ganaste a los dados»)") {
  v3_flash.clear();
  Recuerdos r;
  r.episodios[0] = {static_cast<uint16_t>(diaDe(LUNES_1000) - 1), static_cast<uint8_t>(TipoEpisodio::Perdi), 3};
  r.episodios[1] = {static_cast<uint16_t>(diaDe(LUNES_1000) - 1), static_cast<uint8_t>(TipoEpisodio::Musica), 0};
  r.nEpisodios = 2;
  guardarRecuerdosEn(r);
  arrancarV3(false, "", LUNES_1000, false, "1");
  v3_globos.clear();
  // Un toque cada 2 min durante 26 min: el buen día primero, el recuerdo después.
  for (int k = 0; k < 13; k++) {
    v3_s.evento(Evento::Tacto);
    correrAnotando(120000, 100);
  }
  for (const std::string& g : v3_globos) std::printf("    dijo: %s\n", g.c_str());
  VERIFICAR(dijo("los dados"));
}

PRUEBA("v3: el diario: lo escribe y lo manda (0x4D y el aviso 0xCD)") {
  v3_flash.clear();
  arrancarV3(false, ",\"dueno\":{\"nombre\":\"Nahuel\"}", LUNES_1000);
  MotorVida& m = v3_s.motor();
  m.recordar(TipoEpisodio::Perdi, 0);
  m.recordar(TipoEpisodio::AnimoMal, 3);
  m.recordar(TipoEpisodio::Musica);
  m.recordar(TipoEpisodio::Caricias, 10);
  v3_notis.clear();
  VERIFICAR(m.escribirDiario(m.diaVida()) == 3);
  VERIFICAR(m.escribirDiario(m.diaVida()) == 0);  // una vez por día
  // Los avisos: tres 0xCD, los más importantes primero (ánimo mal, el juego).
  std::vector<std::string> lineas;
  std::vector<uint8_t> tipos;
  for (auto& n : v3_notis)
    if (n.second[0] == (CmdDiario | Respuesta)) {
      VERIFICAR(n.second[1] == Ok && n.second[4] == 0 && n.second[5] == 1);
      lineas.push_back(std::string(n.second.begin() + 9, n.second.end()));
      tipos.push_back(n.second[6]);
    }
  VERIFICAR(lineas.size() == 3);
  VERIFICAR(!tipos.empty() && tipos[0] == static_cast<uint8_t>(TipoEpisodio::AnimoMal));
  for (const std::string& l : lineas) {
    std::printf("    diario: %s\n", l.c_str());
    VERIFICAR(!l.empty() && l.size() <= 60);
  }
  // El registro: tipo 9.
  EntradaRegistro reg[4];
  const int nr = m.registro(reg, 4);
  VERIFICAR(nr >= 3 && reg[nr - 1].tipo == 9);
  // Pedírselo: 0x4D → las tres líneas, con su índice y el total.
  v3_notis.clear();
  const uint8_t pedido[] = {CmdDiario};
  v3_s.bleEscribir(CarComando, pedido, 1);
  int n = 0;
  for (auto& x : v3_notis)
    if (x.second[0] == (CmdDiario | Respuesta)) {
      VERIFICAR(x.second[4] == n && x.second[5] == 3);
      n++;
    }
  VERIFICAR(n == 3);
  // Queda en la flash: después de reiniciar, sigue (y dice lo mismo).
  arrancarV3(false, ",\"dueno\":{\"nombre\":\"Nahuel\"}", LUNES_1000 + 86400, false);
  VERIFICAR(v3_s.motor().recuerdos().nDiario == 3);
  char t[LARGO_FRASE];
  VERIFICAR(v3_s.textoEpisodio(v3_s.motor().recuerdos().diario[1], true, t, sizeof t) && lineas.size() == 3 && lineas[1] == t);
}

PRUEBA("v3: de noche escribe el diario solo; un día sin nada es «un día tranquilo»") {
  v3_flash.clear();
  // 23:20 de un lunes: duerme a las 23:30 (y antes jugó solo un rato).
  arrancarV3(false, "", LUNES_1000 + 13 * 3600 + 20 * 60);
  v3_notis.clear();
  correrV3(25 * 60 * 1000, 200);
  VERIFICAR(v3_s.motor().dormido());
  int lineas = 0;
  for (auto& n : v3_notis)
    if (n.second[0] == (CmdDiario | Respuesta)) {
      lineas++;
      std::printf("    diario: %s\n", std::string(n.second.begin() + 9, n.second.end()).c_str());
      VERIFICAR(n.second.size() > 9);
    }
  VERIFICAR(lineas >= 1);
  // Un día sin episodios.
  v3_notis.clear();
  VERIFICAR(v3_s.motor().escribirDiario(static_cast<uint16_t>(v3_s.motor().diaVida() + 1)) == 1);
  VERIFICAR(v3_notis.size() == 1 && v3_notis[0].second[6] == DIARIO_TRANQUILO && v3_notis[0].second.size() > 9);
}

PRUEBA("v3: sueña con lo que pasó en el día") {
  SoloMotor s;
  s.correr(500);
  s.m.recordar(TipoEpisodio::Toque, 1);  // un abrazo: sueña con corazones
  s.m.dormir(s.t);
  const int sonar = SoloMotor::config().vida.indiceComportamiento("sonar");
  int corazones = 0, otros = 0;
  for (int k = 0; k < 6; k++) {
    s.m.comportamiento(sonar, s.t);
    for (int f = 0; f < 600; f++) {
      s.correr(33);
      for (const Particula& p : s.m.cuadro().particulas)
        if (p.viva && p.enBurbuja && p.edad < 0.04f) (p.tipo == TipoParticula::Corazon ? corazones : otros)++;
    }
  }
  VERIFICAR(corazones > 0 && otros == 0);
}

// ---------------------------------------------------------------- juega solo

PRUEBA("v3: solo, juega por su cuenta; al volver, te propone algo") {
  // El motor solo: en una hora sin nadie, hace cosas suyas.
  SoloMotor s;
  s.m.identidad(77);
  s.m.presencia(false);
  std::vector<std::string> hechos;
  for (int k = 0; k < 3600; k++) {
    s.correr(1000, 50);
    const std::string c = s.m.nombreComportamiento();
    if (!c.empty() && (hechos.empty() || hechos.back() != c)) hechos.push_back(c);
  }
  int solos = 0;
  for (const std::string& c : hechos)
    for (const char* n : {"perseguir-estrella", "tararear-solo", "ordenar-cosas", "mirar-la-hora"}) solos += c == n;
  std::printf("    en una hora solo: %zu comportamientos, %d de jugar solo\n", hechos.size(), solos);
  VERIFICAR(solos >= 2);

  // Con el sistema: estuvo solo y volviste → propuesta (lo que más jugás).
  arrancarV3(false, "", LUNES_1000, true, "1");
  v3_s.motor().recordar(TipoEpisodio::Gane, 3);
  v3_s.motor().recordar(TipoEpisodio::Perdi, 3);
  v3_s.evento(Evento::Tacto);  // el buen día
  correrV3(9 * 60 * 1000, 200);
  v3_globos.clear();
  v3_s.motor().comportamiento(v3_s.config().vida.indiceComportamiento("perseguir-estrella"), v3_ahora);
  correrV3(8000, 50);
  v3_s.evento(Evento::Tacto);
  correrAnotando(3000);
  for (const std::string& g : v3_globos) std::printf("    dijo: %s\n", g.c_str());
  VERIFICAR(dijo("los dados") || dijo("juntos") || dijo("jugar") || dijo("Jugamos"));
}

PRUEBA("v3: la mirada sigue a la partícula (persigue la estrella)") {
  SoloMotor s;
  s.correr(500);
  s.m.comportamiento(SoloMotor::config().vida.indiceComportamiento("perseguir-estrella"), s.t);
  float minX = 1, maxX = -1;
  for (int k = 0; k < 60; k++) {
    s.correr(33);
    minX = std::min(minX, s.m.cuadro().r[param::GX]), maxX = std::max(maxX, s.m.cuadro().r[param::GX]);
  }
  VERIFICAR(maxX - minX > 0.05f);
}

// ---------------------------------------------------------------- empatía, clima

PRUEBA("v3: si estuviste bajoneado varios días, cambia (más suave, más cariñoso, menos pícaro)") {
  SoloMotor s;
  s.correr(500);
  const Personalidad antes = s.m.rasgos();
  VERIFICAR(s.m.bajon() < 0 && !s.m.empatico());
  Recuerdos r;
  const uint16_t hoy = s.m.diaVida();
  r.diaAnimos = hoy, r.animos[0] = 3, r.animos[1] = 3, r.animos[2] = 2;
  s.m.restaurarRecuerdos(r);
  s.correr(500);
  VERIFICAR(s.m.empatico());
  VERIFICAR(s.m.bajon() > 0.8f);
  VERIFICAR(s.m.rasgos().picardia < antes.picardia - 0.2f);
  VERIFICAR(s.m.rasgos().sociabilidad > antes.sociabilidad + 0.1f);
  VERIFICAR(s.m.variable(VarVida::Bajon) > 0.5f);
  // Un día bien no lo borra: ayer y antes de ayer, mal.
  s.m.anotarHoy(Hoy::Animo, 1);
  VERIFICAR(s.m.empatico());
}

PRUEBA("v3: la lluvia lo pone más tranquilo; la primavera, más activo") {
  SoloMotor s;  // noviembre: primavera
  s.correr(500);
  const float prima = s.m.rasgos().energia;
  s.m.clima(true);
  s.correr(500);
  VERIFICAR(s.m.rasgos().energia < prima - 0.05f);
  VERIFICAR(s.m.variable(VarVida::Lluvia) > 0.5f);
  SoloMotor j;
  j.epoch0 = LUNES_1000 - 150 * 86400;  // junio: invierno
  j.correr(500);
  VERIFICAR(j.m.rasgos().energia < prima - 0.08f);
}

// ---------------------------------------------------------------- fechas

PRUEBA("v3: los feriados argentinos (el Día de la Memoria, sin festejo; el Carnaval, de la pascua)") {
  v3_flash.clear();
  arrancarV3(false, "", MEMORIA_1000);
  v3_globos.clear();
  v3_s.evento(Evento::Tacto);
  bool festejo = false;
  for (int k = 0; k < 200; k++) {
    correrAnotando(50);
    festejo = festejo || std::string(v3_s.motor().nombreComportamiento()) == "festejar";
  }
  for (const std::string& g : v3_globos) std::printf("    dijo: %s\n", g.c_str());
  VERIFICAR(dijo("Memoria"));
  VERIFICAR(!festejo);
  v3_flash.clear();
  arrancarV3(false, "", CARNAVAL_1000);
  v3_globos.clear();
  v3_s.evento(Evento::Tacto);
  correrAnotando(10000);
  VERIFICAR(dijo("Carnaval"));
}

PRUEBA("v3: las fechas tuyas (config «fechas»): el día antes te avisa") {
  v3_flash.clear();
  arrancarV3(false, ",\"fechas\":[{\"fecha\":\"11-10\",\"texto\":\"el cumple de mamá\"}]", LUNES_1000);
  VERIFICAR(v3_s.config().fechas.n == 1 && v3_s.config().fechas.lista[0].fecha == 1110);
  v3_globos.clear();
  v3_s.evento(Evento::Tacto);
  correrAnotando(10000);
  for (const std::string& g : v3_globos) std::printf("    dijo: %s\n", g.c_str());
  VERIFICAR(dijo("es el cumple de mamá"));
  // Mal escritas: no.
  static Config c;
  const std::string mal = "{\"version\":2,\"fechas\":[{\"fecha\":\"13-01\",\"texto\":\"x\"}]}";
  VERIFICAR(!cargarConfig(c, mal.c_str(), mal.size()).ok());
}

// ---------------------------------------------------------------- rutinas

PRUEBA("v3: aprende a qué hora llegás, te espera mirando la puerta y se despierta antes") {
  SoloMotor s;
  s.m.identidad(31);
  s.correr(500);
  s.m.presencia(true);  // 10:00
  VERIFICAR(s.m.recuerdos().minLlegada == 600);
  s.correr(3600 * 1000, 500);
  s.m.presencia(true);  // 11:00 → se corre un cuarto
  VERIFICAR(s.m.recuerdos().minLlegada == 615);
  // Te espera: un rato antes de las 10:40, sin verte hace rato.
  Recuerdos r = s.m.recuerdos();
  r.minLlegada = 10 * 60 + 40;
  SoloMotor e;
  e.m.identidad(31);
  e.m.restaurarRecuerdos(r);
  e.correr(35 * 60 * 1000, 200);  // 10:35, 35 min sin nadie
  VERIFICAR(e.m.esperando());
  VERIFICAR(e.m.variable(VarVida::Esperando) > 0.5f);
  // Durmiendo una siesta, se despierta un poco antes de que llegues.
  SoloMotor d;
  d.epoch0 = LUNES_1000 + 32 * 60;  // 10:32
  d.m.restaurarRecuerdos(r);
  d.correr(500);
  d.m.siesta(d.t, 3600);
  VERIFICAR(d.m.dormido());
  d.correr(5000, 100);
  VERIFICAR(!d.m.dormido());
}

PRUEBA("v3: a la hora en que te dormís, baja la luz y bosteza") {
  Recuerdos r;
  r.minDormis = 22 * 60;
  SoloMotor a, b;
  a.epoch0 = LUNES_1000 + 10 * 3600;            // 20:00 (lejos)
  b.epoch0 = LUNES_1000 + 11 * 3600 + 50 * 60;  // 21:50 (10 min antes)
  a.m.restaurarRecuerdos(r), b.m.restaurarRecuerdos(r);
  a.correr(3000), b.correr(3000);
  VERIFICAR(b.m.brillo() < a.m.brillo() - 0.1f);
  VERIFICAR(b.m.variable(VarVida::Sueno) > a.m.variable(VarVida::Sueno) + 0.1f);
  bool bostezo = false;
  for (int k = 0; k < 100; k++) {
    b.correr(100);
    bostezo = bostezo || std::string(b.m.nombreComportamiento()) == "bostezar";
  }
  VERIFICAR(bostezo);
}
