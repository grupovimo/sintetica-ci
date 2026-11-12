// El motor de vida v2: los criterios de aceptación de la especificación
// (docs/motor-de-vida-especificacion.md, "Criterios de aceptación") y lo que
// los sostiene.
#include <algorithm>
#include <cstring>
#include <string>
#include <vector>

#include "formato.h"
#include "formato_vida.h"
#include "motor_vida.h"
#include "prueba.h"
#include "rostro.h"

using namespace sintetica;

namespace {

// Un día cualquiera a las 9:00 (hora local).
constexpr uint32_t DIA = 20764;
constexpr uint32_t EPOCH_9 = DIA * 86400u + 9 * 3600u;

struct Sim {
  static Config& config() {
    static Config c;
    return c;
  }
  MotorVida m;
  uint32_t t = 1000;
  uint32_t epoch0 = EPOCH_9;
  bool conHora = true;
  int bateria = -1;

  explicit Sim(uint32_t semilla, bool base = true) {
    if (base) cargarBase(config());
    m.iniciar(&config(), semilla, t);
  }
  Contexto ctx() const {
    Contexto x;
    if (!conHora) return x;
    x.epoch = epoch0 + (t - 1000) / 1000;
    x.minutoDelDia = static_cast<int>((x.epoch % 86400u) / 60u);
    x.bateria = bateria;
    return x;
  }
  void correr(uint32_t ms, uint32_t paso = 33) {
    const uint32_t fin = t + ms;
    while (t < fin) {
      t += paso;
      m.tick(t, ctx());
    }
  }
  std::string visible() const { return m.nombreVisible(); }
  std::string comportamiento() const { return m.nombreComportamiento(); }
};

float distancia(const CuadroRostro& a, const CuadroRostro& b) {
  float d = 0;
  for (int i = 0; i < N_PARAMETROS; i++) {
    const InfoParametro& inf = infoParametro(i);
    d = std::max(d, std::fabs(a.r[i] - b.r[i]) / (inf.maximo - inf.minimo));
  }
  return d;
}

}  // namespace

PRUEBA("vida: la base trae las expresiones, los comportamientos y las reglas") {
  static Config c;
  VERIFICAR(cargarBase(c).ok());
  VERIFICAR(c.vida.nCaras >= 30);
  VERIFICAR(c.vida.nComportamientos >= 40);
  VERIFICAR(c.nReglas >= 30);
  // Las 16 del manual existen como paramétricas.
  for (const char* n : {"neutral", "contento", "sospecha", "sorpresa", "dormido", "enojado", "triste", "guino",
                        "enamorado", "mareado", "escuchando", "pensando", "curioso", "bateria-baja", "actualizando", "glitch"})
    VERIFICAR(c.vida.indiceCara(n) >= 0);
  // Sin dibujos propios, una cara del manual se resuelve a la paramétrica.
  VERIFICAR(c.resolverCara("contento").vida >= 0);
  VERIFICAR(c.resolverCara("contento").dibujo < 0);
  VERIFICAR(!c.resolverCara("no-existe").existe());
  // Los presets.
  Personalidad p;
  VERIFICAR(presetPersonalidad("dormilon", &p));
  VERIFICAR(p.energia < 0.3f);
  VERIFICAR(!presetPersonalidad("inventado", &p));
}

PRUEBA("vida: config v2 completa") {
  static Config c;
  const char* json = R"({
    "version": 2, "semilla": 48213, "nombre": "Toto",
    "personalidad": { "preset": "picaro", "timidez": 0.7 },
    "horario": { "despierta": "07:30", "duerme": "22:45" },
    "dueno": { "nombre": "Nahuel", "cumple": "03-14" },
    "cumple": "11-07",
    "expresiones": {
      "contento": { "ojo": { "w": 0.13, "h": 0.2, "smile": 0.85 }, "gy": 0.04, "movimiento": "rebote" },
      "mia": { "extiende": "sorpresa", "izq": { "h": 0.3 }, "particula": "chispa" }
    },
    "comportamientos": {
      "bostezar": { "cuando": "sueno>0.5", "pasos": [ { "cara": "bostezo", "dura": 2.0 }, { "cara": "neutral", "parpadeo": "lento" } ], "enfriamiento": 180 },
      "saludar-mio": { "tipo": "reaccion", "pasos": [ { "cara": "mia", "dura": [1, 2], "veces": [2, 3] } ] }
    },
    "reglas": [
      { "si": "sacudida.fuerte", "entonces": ["reflejo:mareado", "estado.estres+0.4", "sonido:boing"] },
      { "si": "nombre_escuchado", "y": "afecto>0.2", "entonces": ["comportamiento:saludar-mio", "estado.animo+0.2"] }
    ],
    "limites_marca": { "color": "#FF6A33", "brillo_max": 0.9 }
  })";
  ResultadoConfig r = cargarConfig(c, json, std::strlen(json));
  if (!r.ok()) std::printf("    %s\n", r.detalle);
  VERIFICAR(r.ok());
  VERIFICAR(c.version == 2);
  VERIFICAR(c.vida.semilla == 48213);
  VERIFICAR_CERCA(c.vida.personalidad.picardia, 0.95, 1e-4);
  VERIFICAR_CERCA(c.vida.personalidad.timidez, 0.7, 1e-4);
  VERIFICAR(c.vida.despierta == 7 * 60 + 30 && c.vida.duerme == 22 * 60 + 45);
  VERIFICAR(std::string(c.vida.dueno) == "Nahuel" && c.vida.cumpleDueno == 314 && c.vida.cumpleBicho == 1107);
  const int mia = c.vida.indiceCara("mia");
  VERIFICAR(mia >= 0 && c.vida.caras[mia].propia);
  // "extiende": tiene lo de la sorpresa más lo suyo.
  Rostro r1;
  c.vida.caras[mia].aplicar(r1);
  VERIFICAR_CERCA(r1[param::W], 0.15, 1e-4);
  VERIFICAR_CERCA(r1[param::H], 0.3, 1e-4);
  VERIFICAR_CERCA(r1[param::DER + param::H], 0.17, 1e-4);
  VERIFICAR(c.vida.caras[mia].particula == static_cast<int>(TipoParticula::Chispa));
  // Las reglas del usuario van primero.
  VERIFICAR(c.reglas[0].si.evento == Evento::SacudidaFuerte);
  VERIFICAR(c.reglas[0].entonces[0].tipo == Accion::Tipo::Reflejo);
  VERIFICAR(c.reglas[0].entonces[1].tipo == Accion::Tipo::Estado && c.reglas[0].entonces[1].variable == 6);
  VERIFICAR(c.reglas[1].si.evento == Evento::Nombre && c.reglas[1].nY == 1);
  const int b = c.vida.indiceComportamiento("saludar-mio");
  Paso pasos[MAX_PASOS];
  VERIFICAR(b >= 0 && pasosDe(c, c.vida.comportamientos[b], pasos) > 0 && pasos[0].vecesMax == 3);
  VERIFICAR(c.vida.brilloMax < 0.95f);
}

PRUEBA("vida: la config v2 se valida (límites de la marca, referencias)") {
  static Config c;
  const char* malos[] = {
      R"({"version":2,"expresiones":{"x":{"ojo":{"w":0.5}}}})",                    // w fuera de rango
      R"({"version":2,"expresiones":{"x":{"ojo":{"gx":0.1}}}})",                   // gx no es del ojo
      R"({"version":2,"expresiones":{"x":{"movimiento":"volar"}}})",
      R"({"version":2,"comportamientos":{"x":{"pasos":[{"cara":"no-existe"}]}}})",
      R"({"version":2,"comportamientos":{"x":{"cuando":"humedad>3","pasos":[{"dura":1}]}}})",
      R"({"version":2,"reglas":[{"si":"tacto","entonces":["comportamiento:no-existe"]}]})",
      R"({"version":2,"reglas":[{"si":"tacto","entonces":["estado.felicidad+0.2"]}]})",
      R"({"version":2,"personalidad":{"preset":"robot"}})",
      R"({"version":2,"horario":{"despierta":"25:00"}})",
      R"({"version":2,"limites_marca":{"brillo_max":0.05}})",
      R"({"version":3})",
  };
  for (const char* m : malos) {
    ResultadoConfig r = cargarConfig(c, m, std::strlen(m));
    if (r.ok()) std::printf("    debería fallar: %s\n", m);
    VERIFICAR(!r.ok());
  }
  // La v1 sigue andando, y su "sueno" pasa al horario.
  const char* v1 = R"({"version":1,"personalidad":{"energia":0.9,"sueno":"23:00-07:00"}})";
  VERIFICAR(cargarConfig(c, v1, std::strlen(v1)).ok());
  VERIFICAR(c.vida.duerme == 23 * 60 && c.vida.despierta == 7 * 60);
  VERIFICAR_CERCA(c.vida.personalidad.energia, 0.9, 1e-4);
}

PRUEBA("vida: la memoria va y vuelve (y rechaza basura)") {
  Memoria m;
  m.nacimiento = 1, m.ultimaInteraccion = 123456, m.dias = 42, m.racha = 3, m.caricias = 77;
  m.horas[9] = 5, m.gustos[0] = -12, m.hitos = 0x0B, m.deriva[1] = 7, m.estado[2] = 200, m.afectoHoy = 9;
  uint8_t b[LARGO_MEMORIA];
  VERIFICAR(codificarMemoria(m, b, sizeof b) == LARGO_MEMORIA);
  Memoria n;
  VERIFICAR(decodificarMemoria(b, sizeof b, &n));
  VERIFICAR(n.ultimaInteraccion == 123456 && n.dias == 42 && n.caricias == 77 && n.horas[9] == 5);
  VERIFICAR(n.gustos[0] == -12 && n.hitos == 0x0B && n.deriva[1] == 7 && n.estado[2] == 200 && n.afectoHoy == 9);
  b[10] ^= 1;
  VERIFICAR(!decodificarMemoria(b, sizeof b, &n));
}

PRUEBA("vida: misma semilla, mismos cuadros; otra semilla, otra vida") {
  Sim a(77), b(77), c(78);
  bool igual = true, distinto = false;
  for (int i = 0; i < 60 * 30; i++) {
    a.correr(33), b.correr(33), c.correr(33);
    igual = igual && std::memcmp(&a.m.cuadro().r, &b.m.cuadro().r, sizeof(Rostro)) == 0;
    distinto = distinto || distancia(a.m.cuadro(), c.m.cuadro()) > 0.01f;
  }
  VERIFICAR(igual);
  VERIFICAR(distinto);
  VERIFICAR(a.m.parpadeos() == b.m.parpadeos());
}

PRUEBA("vida: parpadea de 10 a 20 veces por minuto, con 10 a 20 % de dobles") {
  Sim s(1234);
  s.correr(30 * 60 * 1000);
  const float porMinuto = s.m.parpadeos() / 30.0f;
  const float dobles = static_cast<float>(s.m.parpadeosDobles()) / std::max(1u, s.m.parpadeos());
  std::printf("    %.1f parpadeos/min, %.0f %% dobles\n", porMinuto, dobles * 100);
  VERIFICAR(porMinuto >= 10 && porMinuto <= 20);
  VERIFICAR(dobles >= 0.10f && dobles <= 0.20f);
}

PRUEBA("vida: despierto, ningún cuadro se repite más de 2 s") {
  Sim s(99);
  CuadroRostro antes = s.m.cuadro();
  uint32_t igualDesde = s.t, peor = 0;
  for (int i = 0; i < 20 * 60 * 30; i++) {
    s.correr(33);
    const CuadroRostro& c = s.m.cuadro();
    const bool igual = std::memcmp(&c.r, &antes.r, sizeof(Rostro)) == 0 && c.escalaY == antes.escalaY &&
                       c.derivaX == antes.derivaX && c.derivaY == antes.derivaY;
    if (!igual) {
      peor = std::max(peor, s.t - igualDesde);
      igualDesde = s.t;
      antes = c;
    }
  }
  VERIFICAR(!s.m.dormido());
  VERIFICAR(peor < 2000);
}

PRUEBA("vida: entre expresiones no hay saltos") {
  Sim s(5);
  const DatosVida& v = Sim::config().vida;
  Azar azar;
  azar.sembrar(9);
  float peor = 0;
  float antes[N_PARAMETROS];
  for (int i = 0; i < N_PARAMETROS; i++) antes[i] = s.m.parametro(i);
  for (int k = 0; k < 600; k++) {
    // Una cara cualquiera cada 300 a 900 ms, sin dar tiempo a llegar.
    Config::Cara c;
    c.vida = static_cast<int8_t>(azar.entre(0, v.nCaras - 1));
    s.m.reaccionar(c, s.t, 2000);
    const uint32_t dura = azar.entre(300, 900);
    for (uint32_t ms = 0; ms < dura; ms += 16) {
      s.t += 16;
      s.m.tick(s.t, s.ctx());
      for (int i = 0; i < N_PARAMETROS; i++) {
        const InfoParametro& inf = infoParametro(i);
        peor = std::max(peor, std::fabs(s.m.parametro(i) - antes[i]) / (inf.maximo - inf.minimo));
        antes[i] = s.m.parametro(i);
      }
    }
  }
  std::printf("    el mayor cambio en un paso de 16 ms: %.3f del rango\n", peor);
  VERIFICAR(peor < 0.2f);  // nada recorre todo su rango en menos de 5 pasos (80 ms)
}

PRUEBA("vida: en una hora sin tocarlo hace 8 comportamientos distintos, sin repetir seguido") {
  for (uint32_t semilla : {11u, 22u, 33u}) {
    Sim s(semilla);
    std::vector<std::string> hechos;
    std::string antes;
    for (int i = 0; i < 60 * 60 * 10; i++) {
      s.correr(100, 100);
      const int a = s.m.comportamientoActual();
      const std::string n = a >= 0 ? s.comportamiento() : "";
      if (!n.empty() && n != antes && Sim::config().vida.comportamientos[a].tipo == TipoComportamiento::Ocioso)
        hechos.push_back(n);
      antes = n;
    }
    std::vector<std::string> distintos;
    bool repetido = false;
    for (size_t i = 0; i < hechos.size(); i++) {
      if (std::find(distintos.begin(), distintos.end(), hechos[i]) == distintos.end()) distintos.push_back(hechos[i]);
      if (i && hechos[i] == hechos[i - 1]) repetido = true;
    }
    std::printf("    semilla %u: %zu comportamientos, %zu distintos\n", semilla, hechos.size(), distintos.size());
    VERIFICAR(distintos.size() >= 8);
    VERIFICAR(!repetido);
  }
}

PRUEBA("vida: dos bichos con la misma personalidad y distinta semilla no hacen lo mismo") {
  Sim a(1001), b(2002);
  int iguales = 0, total = 0;
  for (int i = 0; i < 20 * 60 * 10; i++) {
    a.correr(100, 100), b.correr(100, 100);
    if (a.m.comportamientoActual() >= 0 || b.m.comportamientoActual() >= 0) {
      total++;
      iguales += a.m.comportamientoActual() == b.m.comportamientoActual();
    }
  }
  VERIFICAR(total > 0);
  VERIFICAR(iguales < total / 2);
}

PRUEBA("vida: ignorado 2 h se ve aburrido") {
  Sim s(321);
  s.correr(2 * 3600 * 1000, 100);
  // Esperar un momento sin comportamiento para ver el ánimo de fondo.
  for (int i = 0; i < 600 && s.m.comportamientoActual() != -1; i++) s.correr(100, 100);
  std::printf("    aburrimiento %.2f, se ve %s\n", s.m.estadoInterno()[3], s.visible().c_str());
  VERIFICAR(s.m.estadoInterno()[3] > 0.6f);
  VERIFICAR(s.visible() == "aburrido");
}

PRUEBA("vida: ignorado 3 días, se hace el ofendido al volver") {
  Sim s(4242);
  s.correr(1000);
  s.m.evento(Evento::Tacto, s.t);
  s.correr(10 * 1000);
  // Tres días sin nadie (de a 10 s: el motor salta sin resortes).
  for (int i = 0; i < 3 * 86400 / 10; i++) s.correr(10000, 10000);
  // Lo despertamos de día.
  s.correr(1000);
  s.m.evento(Evento::Tacto, s.t);
  s.correr(100);
  std::printf("    al volver: %s\n", s.comportamiento().c_str());
  VERIFICAR(s.comportamiento() == "volver-ofendido");
  // Unas horas no alcanzan para ofenderse: saluda.
  Sim t(4243);
  t.m.evento(Evento::Tacto, t.t);
  t.correr(4 * 3600 * 1000, 200);
  t.m.evento(Evento::Tacto, t.t);
  t.correr(50);
  VERIFICAR(t.comportamiento() == "saludar-al-volver");
}

PRUEBA("vida: los reflejos responden en menos de 50 ms") {
  Sim s(8);
  s.correr(5000);
  const CuadroRostro antes = s.m.cuadro();
  s.m.evento(Evento::SacudidaFuerte, s.t);
  float cambio = 0;
  for (int ms = 0; ms < 48; ms += 16) {
    s.t += 16;
    s.m.tick(s.t, s.ctx());
    cambio = std::max(cambio, distancia(antes, s.m.cuadro()));
  }
  std::printf("    cambio en 48 ms: %.3f del rango\n", cambio);
  VERIFICAR(cambio > 0.05f);
  VERIFICAR(s.visible() == "mareado");
}

PRUEBA("vida: de noche cuenta ovejas, se duerme, sueña y se despierta a la mañana") {
  Sim s(55);
  s.epoch0 = DIA * 86400u + 22 * 3600u;  // 22:00
  s.m.iniciar(&Sim::config(), 55, s.t);
  bool ovejas = false, durmio = false, sueno = false, desperto = false;
  for (int i = 0; i < 11 * 3600; i++) {
    s.correr(1000, 50);
    ovejas = ovejas || s.comportamiento() == "contar-ovejas";
    durmio = durmio || s.m.dormido();
    sueno = sueno || s.m.sonando();
    if (durmio && !s.m.dormido()) desperto = true;
    if (desperto) break;
  }
  const Contexto c = s.ctx();
  std::printf("    se despertó a las %02d:%02d\n", c.minutoDelDia / 60, c.minutoDelDia % 60);
  VERIFICAR(ovejas);
  VERIFICAR(durmio);
  VERIFICAR(sueno);
  VERIFICAR(desperto);
  VERIFICAR(c.minutoDelDia >= 8 * 60 && c.minutoDelDia < 8 * 60 + 2);
  VERIFICAR(s.comportamiento() == "despertar");
}

PRUEBA("vida: si lo despiertan de noche, sobresalto y gruñón (salvo que lo quiera mucho)") {
  Sim s(56);
  s.epoch0 = DIA * 86400u + 2 * 3600u;  // 2:00
  s.m.iniciar(&Sim::config(), 56, s.t);
  s.m.dormir(s.t);
  s.correr(2000);
  VERIFICAR(s.m.dormido());
  s.m.evento(Evento::Tacto, s.t);
  s.correr(50);
  VERIFICAR(!s.m.dormido());
  VERIFICAR(s.visible() == "sorpresa");
  VERIFICAR(s.comportamiento() == "despertar-forzado");
  // Si nadie lo toca, a los minutos se vuelve a dormir.
  s.correr(15 * 60 * 1000, 100);
  VERIFICAR(s.m.dormido());
}

PRUEBA("vida: las reglas eligen la variante según el estado y la personalidad") {
  static Config c;
  const char* json = R"({"version":2,"personalidad":{"timidez":0.9}})";
  VERIFICAR(cargarConfig(c, json, std::strlen(json)).ok());
  MotorVida m;
  m.iniciar(&c, 3, 0);
  Contexto x;
  m.tick(500, x);
  m.evento(Evento::Tacto, 500);
  VERIFICAR(std::string(m.nombreComportamiento()) == "toque-timido");
  // Una caricia sube el ánimo y cuenta en la memoria.
  const float antes = m.estadoInterno()[1];
  m.evento(Evento::Caricia, 600);
  VERIFICAR(m.estadoInterno()[1] > antes);
  VERIFICAR(m.memoria().caricias == 1);
  // Sacudida fuerte con timidez alta: miedo.
  m.evento(Evento::SacudidaFuerte, 700);
  m.tick(720, x);
  VERIFICAR(std::string(m.nombreVisible()) == "miedo");
  VERIFICAR(m.estadoInterno()[6] > 0.4f);
}

PRUEBA("vida: una cara fija gana; la base de la lista no se duerme sola") {
  Sim s(9);
  VERIFICAR(s.m.fijar("enamorado"));
  s.correr(2000);
  VERIFICAR(s.visible() == "enamorado");
  VERIFICAR(!s.m.fijar("no-existe"));
  s.m.fijar("");
  Config::Cara b = Sim::config().resolverCara("contento");
  s.m.base(b);
  s.correr(3 * 3600 * 1000, 200);
  VERIFICAR(!s.m.dormido());
}

PRUEBA("vida: batería baja (regla de estado) y el rostro con poca luz de noche") {
  Sim s(10);
  s.bateria = 10;
  s.correr(3000);
  // Una capa del ánimo: más apagado, pero no tapa la cara.
  VERIFICAR(s.visible() != "bateria-baja");
  VERIFICAR(s.m.brillo() < 0.9f);
  s.bateria = 80;
  s.epoch0 = DIA * 86400u + 23 * 3600u;  // 23:00, media hora antes de dormir
  s.correr(3000);
  VERIFICAR(s.m.brillo() < 0.9f);
}

PRUEBA("vida: rostro: el dibujo por bloques evalúa la SDF fina solo en los bordes y da igual por franjas") {
  static uint16_t entero[320 * 172], franjas[320 * 172], tira[320 * 20];
  CuadroRostro c;
  Lienzo l;
  l.iniciar(entero, 320, 172);
  const VistaRostro v = VistaRostro::para(320, 172, false);
  dibujarRostro(l, v, c);
  const uint32_t finos = pixelesFinosUltimoDibujo();
  std::printf("    %u píxeles finos de %d\n", finos, 320 * 172);
  VERIFICAR(finos < 320 * 172 / 4);
  Lienzo f;
  f.iniciar(tira, 320, 172, 20);
  for (int y = 0; y < 172; y += 20) {
    f.empezarFranja(y);
    dibujarRostro(f, v, c);
    std::memcpy(franjas + y * 320, tira, sizeof(uint16_t) * 320 * f.filasFranja());
  }
  VERIFICAR(std::memcmp(entero, franjas, sizeof entero) == 0);
  // Los ojos de la neutral miden 72 px de alto (el manual) en la 1.47B.
  int arriba = -1, abajo = -1;
  for (int y = 0; y < 172; y++) {
    const uint32_t px = de565(entero[y * 320 + 160 - 47]);
    if (((px >> 16) & 0xFF) > 200) {
      if (arriba < 0) arriba = y;
      abajo = y;
    }
  }
  VERIFICAR(std::abs((abajo - arriba + 1) - 72) <= 2);
}

PRUEBA("vida: rostro: el halo de los ojos con párpados es redondo, no una caja") {
  // Los párpados cortan el ojo con un semiplano: con el máximo de las dos
  // distancias, afuera las curvas de nivel (y el resplandor, que sale de
  // ahí) quedaban con esquinas, como cajas. Afuera tiene que ser la distancia
  // de verdad a la forma (la que se mide a mano, contra los puntos de adentro).
  Config& cfg = Sim::config();
  cargarBase(cfg);
  const DatosVida& vida = cfg.vida;
  for (const char* n : {"sospecha", "dormido", "guino", "pensando", "bateria-baja", "neutral"}) {
    const int i = vida.indiceCara(n);
    VERIFICAR(i >= 0);
    CuadroRostro c;
    vida.caras[vida.indiceCara("neutral")].aplicar(c.r);
    vida.caras[i].aplicar(c.r);
    c.r.limitar();
    std::vector<std::pair<float, float>> adentro;
    const float paso = 0.004f;
    for (float y = -0.7f; y <= 0.7f; y += paso)
      for (float x = -1.0f; x <= 1.0f; x += paso)
        if (distanciaRostro(c, x, y) <= 0) adentro.push_back({x, y});
    VERIFICAR(!adentro.empty());
    float peor = 0;
    for (float y = -0.7f; y <= 0.7f; y += 0.015f)
      for (float x = -1.0f; x <= 1.0f; x += 0.015f) {
        const float d = distanciaRostro(c, x, y);
        if (d < 0.02f || d > 0.2f) continue;  // donde llega el resplandor
        float real = 9;
        for (const auto& q : adentro) real = std::min(real, std::hypot(x - q.first, y - q.second));
        if (real > 0.25f) continue;
        peor = std::max(peor, std::fabs(real - d));
      }
    std::printf("    %s: lo peor del halo, %.3f\n", n, peor);
    VERIFICAR(peor < 0.012f);
  }
}
