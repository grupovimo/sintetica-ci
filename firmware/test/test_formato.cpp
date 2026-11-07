#include <fstream>
#include <sstream>
#include <string>

#include "formato.h"
#include "json.h"
#include "prueba.h"

using namespace sintetica;

static Config g_c;

static std::string leerArchivo(const std::string& ruta) {
  std::ifstream f(ruta);
  std::stringstream s;
  s << f.rdbuf();
  return s.str();
}

PRUEBA("json: objetos, arreglos, escapes y números") {
  const char* t = R"({"a": [1, -2.5e1, true, null], "b": "hóla\n", "c": {}})";
  TokenJson toks[32];
  Json j;
  VERIFICAR(j.parsear(t, std::char_traits<char>::length(t), toks, 32) > 0);
  int a = j.buscar(0, "a");
  VERIFICAR(j.cantidad(a) == 4);
  VERIFICAR_CERCA(j.numero(j.elemento(a, 1), 0), -25, 1e-4);
  VERIFICAR(j.booleano(j.elemento(a, 2), false));
  char b[16];
  VERIFICAR(j.texto(j.buscar(0, "b"), b, sizeof b) == 6);
  VERIFICAR(std::string(b) == "h\xc3\xb3la\n");
  VERIFICAR(j.buscar(0, "c") >= 0 && j.cantidad(j.buscar(0, "c")) == 0);
  VERIFICAR(j.buscar(0, "z") == -1);
}

PRUEBA("json: rechaza basura y falta de lugar") {
  TokenJson toks[4];
  Json j;
  VERIFICAR(j.parsear("{\"a\":}", 6, toks, 4) < 0);
  VERIFICAR(j.parsear("[1,2,3,4,5]", 11, toks, 4) == static_cast<int>(ErrorJson::SinLugar));
  VERIFICAR(j.parsear("{} x", 4, toks, 4) < 0);
  VERIFICAR(j.parsear("", 0, toks, 4) < 0);
}

PRUEBA("formato: la base trae las 16 caras y las reglas del manual") {
  VERIFICAR(cargarBase(g_c).ok());
  VERIFICAR(g_c.nExpresiones == 16);
  const char* nombres[] = {"neutral", "contento", "sospecha", "sorpresa", "dormido", "enojado",
                           "triste", "guino", "enamorado", "mareado", "escuchando", "pensando",
                           "curioso", "bateria-baja", "actualizando", "glitch"};
  for (int i = 0; i < 16; i++) VERIFICAR(g_c.indiceExpresion(nombres[i]) == i);
  VERIFICAR(g_c.nReglas == 9);
  VERIFICAR(!g_c.expresiones[g_c.indiceExpresion("dormido")].parpadea);
}

PRUEBA("formato: el ejemplo del doc se aplica") {
  std::string t = leerArchivo(std::string(RAIZ_REPO) + "/shared/formato/config-ejemplo.json");
  VERIFICAR(!t.empty());
  VERIFICAR(cargarBase(g_c).ok());
  ResultadoConfig r = aplicarConfig(g_c, t.c_str(), t.size());
  if (!r.ok()) std::printf("    detalle: %s\n", r.detalle);
  VERIFICAR(r.ok());
  VERIFICAR(std::string(g_c.nombre) == "Toto");
  VERIFICAR(g_c.hayColorOjos && g_c.colorOjos == 0xFF6A33);
  VERIFICAR_CERCA(g_c.energia, 0.6, 1e-6);
  VERIFICAR(g_c.suenoDesde == 23 * 60 + 30 && g_c.suenoHasta == 8 * 60);
  // Las reglas propias van antes que las del manual.
  VERIFICAR(g_c.nReglas == 12);
  VERIFICAR(g_c.reglas[0].entonces[0].tipo == Accion::Tipo::Animacion);
  VERIFICAR(g_c.nAnimaciones == 1 && g_c.animaciones[0].sobre == g_c.indiceExpresion("contento"));
  VERIFICAR(g_c.reglas[1].nAcciones == 2);
  VERIFICAR(g_c.reglas[1].entonces[1].tipo == Accion::Tipo::Sonido);
  VERIFICAR(g_c.reglas[2].si.tipo == Condicion::Tipo::Menor);
  VERIFICAR_CERCA(g_c.reglas[2].si.valor, 15, 1e-6);
  // "contento" se reemplazó, no se duplicó.
  VERIFICAR(g_c.nExpresiones == 16);
}

PRUEBA("formato: una config mala no toca la actual") {
  VERIFICAR(cargarBase(g_c).ok());
  const char* malas[] = {
      R"({"version":2})",
      R"({"version":1,"cara":{"tamano":3}})",
      R"({"version":1,"expresiones":{"x":{"formas":[{"forma":"triangulo","x":0,"y":0}]}}})",
      R"({"version":1,"reglas":[{"si":"cosquillas","entonces":["expresion:contento"]}]})",
      R"({"version":1,"reglas":[{"si":"tacto","entonces":["expresion:noexiste"]}]})",
      R"({"version":1,"personalidad":{"sueno":"25:00-08:00"}})",
      R"({"version":1,"modo":"fiesta"})",
      R"({"version":1,)",
  };
  for (const char* m : malas) {
    ResultadoConfig r = aplicarConfig(g_c, m, std::char_traits<char>::length(m));
    VERIFICAR(!r.ok());
  }
  VERIFICAR(std::string(g_c.nombre) == "bicho");
  VERIFICAR(g_c.nReglas == 9);
}

PRUEBA("formato: expresión propia nueva") {
  VERIFICAR(cargarBase(g_c).ok());
  const char* t = R"({"version":1,"expresiones":{"pirata":{"formas":[
      {"forma":"pildora","x":-0.3,"y":0,"w":0.2,"h":0.4},
      {"forma":"poligono","puntos":[[0.2,-0.1],[0.4,-0.1],[0.4,0.1],[0.2,0.1]],"color":"tinta"}]}},
      "reglas":[{"si":"doble-toque","entonces":["expresion:pirata","led:#00FF00"]}]})";
  ResultadoConfig r = aplicarConfig(g_c, t, std::char_traits<char>::length(t));
  if (!r.ok()) std::printf("    detalle: %s\n", r.detalle);
  VERIFICAR(r.ok());
  int i = g_c.indiceExpresion("pirata");
  VERIFICAR(i == 16);
  VERIFICAR(g_c.expresiones[i].formas[1].color == paleta::TINTA);
  VERIFICAR(!g_c.expresiones[i].formas[1].acento);
}
