// Paridad de la config (shared/formato/paridad): el núcleo acepta o rechaza
// cada config del set con el código y el detalle del índice. Los mismos casos
// los corren cgo, el WebAssembly y (lo que validan las apps) la web, Kotlin y Swift.
#include <cstdio>
#include <cstring>
#include <fstream>
#include <sstream>
#include <string>

#include "formato.h"
#include "habla.h"
#include "json.h"
#include "prueba.h"
#include "vida_base.h"

using namespace sintetica;

namespace {
const std::string DIR = std::string(RAIZ_REPO) + "/shared/formato/paridad/";

std::string leer(const std::string& ruta) {
  std::ifstream f(ruta, std::ios::binary);
  std::stringstream s;
  s << f.rdbuf();
  return s.str();
}

std::string g_indice;
TokenJson g_toks[6000];
Json g_j;

bool cargarIndice() {
  if (!g_indice.empty()) return true;
  g_indice = leer(DIR + "paridad.json");
  return g_j.parsear(g_indice.data(), g_indice.size(), g_toks, 6000) > 0;
}

std::string texto(int t) {
  static char b[512];
  return g_j.texto(t, b, sizeof b) >= 0 ? std::string(b) : std::string();
}

int limite(const char* k) { return static_cast<int>(g_j.numero(g_j.buscar(g_j.buscar(0, "limites"), k), -1)); }

Config g_c;
}  // namespace

PRUEBA("paridad: los límites del índice son los del núcleo") {
  VERIFICAR(cargarIndice());
  VERIFICAR(limite("configBytes") == static_cast<int>(MAX_CONFIG_BYTES));
  VERIFICAR(limite("expresiones") == MAX_EXPRESIONES);
  VERIFICAR(limite("carasVida") == MAX_CARAS_VIDA);
  VERIFICAR(limite("carasVidaBase") == CANTIDAD_CARAS_VIDA_BASE);
  VERIFICAR(limite("comportamientos") == MAX_COMPORTAMIENTOS);
  VERIFICAR(limite("comportamientosBase") == CANTIDAD_COMPORTAMIENTOS_BASE);
  VERIFICAR(limite("carasImagen") == MAX_CARAS_IMAGEN);
  VERIFICAR(limite("sprites") == MAX_SPRITES);
  VERIFICAR(limite("imagenes") == MAX_IMAGENES);
  VERIFICAR(limite("reglasUsuario") == MAX_REGLAS_USUARIO);
  VERIFICAR(limite("acciones") == MAX_ACCIONES);
  VERIFICAR(limite("lista") == MAX_LISTA);
  VERIFICAR(limite("animaciones") == MAX_ANIMACIONES);
  VERIFICAR(limite("recordatorios") == MAX_RECORDATORIOS);
  VERIFICAR(limite("bytesRecordatorio") == LARGO_RECORDATORIO - 1);
  VERIFICAR(limite("frasesPropias") == MAX_FRASES_PROPIAS);
  VERIFICAR(limite("bytesFrase") == LARGO_FRASE - 1);
  VERIFICAR(limite("avisos") == MAX_AVISOS);
  VERIFICAR(limite("bytesDecirAviso") == LARGO_DECIR_AVISO - 1);
  VERIFICAR(limite("bytesAppAviso") == LARGO_APP - 1);
  VERIFICAR(limite("bytesPalabra") == LARGO_PALABRA - 1);
  VERIFICAR(limite("bytesNombre") == LARGO_NOMBRE - 1);
  VERIFICAR(limite("bytesCiudad") == 40);
  VERIFICAR(limite("habitos") == MAX_HABITOS);
  VERIFICAR(limite("bytesHabito") == LARGO_HABITO - 1);
  VERIFICAR(limite("bytesPresencia") == LARGO_PRESENCIA - 1);
  VERIFICAR(cargarBase(g_c).ok());
  VERIFICAR(limite("expresionesBase") == g_c.nExpresiones);
}

PRUEBA("paridad: cada config se acepta o se rechaza como dice el índice") {
  VERIFICAR(cargarIndice());
  const int casos = g_j.buscar(0, "casos");
  VERIFICAR(g_j.cantidad(casos) > 50);
  int mal = 0;
  for (int i = 0; i < g_j.cantidad(casos); i++) {
    const int c = g_j.elemento(casos, i);
    const std::string archivo = texto(g_j.buscar(c, "archivo"));
    const std::string t = leer(DIR + "configs/" + archivo);
    const bool acepta = g_j.booleano(g_j.buscar(c, "acepta"), false);
    const int codigo = static_cast<int>(g_j.numero(g_j.buscar(c, "codigo"), 0));
    const std::string detalle = texto(g_j.buscar(c, "detalle"));
    ResultadoConfig r = t.empty() ? ResultadoConfig{ErrorConfig::JsonInvalido, "no está"} : cargarConfig(g_c, t.data(), t.size());
    const bool bien = acepta ? r.ok() : (static_cast<int>(r.error) == codigo && std::strncmp(r.detalle, detalle.c_str(), detalle.size()) == 0);
    // El detalle entra entero en el de cgo y el del WebAssembly (128 bytes).
    if (!r.ok()) VERIFICAR(std::strlen(r.detalle) < 128);
    if (!bien) {
      std::printf("    %s: el núcleo da %d \"%s\"; el índice, %s %d \"%s\"\n", archivo.c_str(), static_cast<int>(r.error), r.detalle,
                  acepta ? "acepta" : "rechaza", codigo, detalle.c_str());
      mal++;
    }
  }
  VERIFICAR(mal == 0);
}

PRUEBA("paridad: los cortes de bytes de las apps no parten letras y entran") {
  VERIFICAR(cargarIndice());
  const int cortes = g_j.buscar(0, "cortes");
  VERIFICAR(g_j.cantidad(cortes) > 5);
  for (int i = 0; i < g_j.cantidad(cortes); i++) {
    const int c = g_j.elemento(cortes, i);
    const std::string original = texto(g_j.buscar(c, "texto")), resultado = texto(g_j.buscar(c, "resultado"));
    const size_t max = static_cast<size_t>(g_j.numero(g_j.buscar(c, "max"), 0));
    VERIFICAR(resultado.size() <= max);
    VERIFICAR(resultado.size() == static_cast<size_t>(g_j.numero(g_j.buscar(c, "bytes"), -1)));
    VERIFICAR(original.compare(0, resultado.size(), resultado) == 0);
    VERIFICAR(textoUtf8Valido(resultado.data(), resultado.size()));
    // Lo cortado a 40 entra como recordatorio y a 60 como frase.
    if (!resultado.empty() && (max == 40 || max == 60)) {
      const std::string cfg = max == 40 ? "{\"version\":2,\"recordatorios\":[{\"hora\":\"08:00\",\"texto\":\"" + resultado + "\"}]}"
                                        : "{\"version\":2,\"habla\":{\"propias\":{\"saludo\":[\"" + resultado + "\"]}}}";
      VERIFICAR(cargarConfig(g_c, cfg.data(), cfg.size()).ok());
    }
  }
}
