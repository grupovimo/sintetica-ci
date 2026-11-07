#include "formato.h"

#include "caras_base.h"

namespace sintetica {

namespace {

const char* const NOMBRES_EVENTO[] = {"tacto",    "doble-toque", "levantado",   "sacudida",
                                      "boca-abajo", "vinculado", "app-abierta", "despertado"};

// Memoria de trabajo compartida (no reentrante: el núcleo corre en un solo hilo).
TokenJson g_tokens[MAX_TOKENS_JSON];
Config g_copia;

ResultadoConfig error(ErrorConfig e, const char* detalle) {
  ResultadoConfig r;
  r.error = e;
  r.detalle = detalle;
  return r;
}

bool esNumero(const Json& j, int i) { return j.valido(i) && j.tipo(i) == TipoJson::Numero; }

// Lee un número obligatorio dentro de [lo, hi].
bool numeroEn(const Json& j, int obj, const char* clave, float lo, float hi, float* salida) {
  int i = j.buscar(obj, clave);
  if (!esNumero(j, i)) return false;
  float v = j.numero(i, 0);
  if (v < lo || v > hi) return false;
  *salida = v;
  return true;
}

// Lee un número opcional; si está mal, falla.
bool numeroOpcional(const Json& j, int obj, const char* clave, float lo, float hi, float* salida) {
  int i = j.buscar(obj, clave);
  if (i < 0) return true;
  return numeroEn(j, obj, clave, lo, hi, salida);
}

bool leerPunto(const Json& j, int arr, Punto* p) {
  if (!j.valido(arr) || j.tipo(arr) != TipoJson::Arreglo || j.cantidad(arr) != 2) return false;
  int a = j.elemento(arr, 0), b = j.elemento(arr, 1);
  if (!esNumero(j, a) || !esNumero(j, b)) return false;
  p->x = j.numero(a, 0);
  p->y = j.numero(b, 0);
  return absf(p->x) <= 2 && absf(p->y) <= 2;
}

bool leerPuntos(const Json& j, int obj, int minimo, int maximo, Forma* f) {
  int arr = j.buscar(obj, "puntos");
  if (!j.valido(arr) || j.tipo(arr) != TipoJson::Arreglo) return false;
  int n = j.cantidad(arr);
  if (n < minimo || n > maximo) return false;
  for (int k = 0; k < n; k++)
    if (!leerPunto(j, j.elemento(arr, k), &f->puntos[k])) return false;
  f->nPuntos = static_cast<uint8_t>(n);
  return true;
}

ResultadoConfig leerForma(const Json& j, int obj, Forma* f) {
  const auto MAL = [](const char* d) { return error(ErrorConfig::Forma, d); };
  if (!j.valido(obj) || j.tipo(obj) != TipoJson::Objeto) return MAL("forma: no es un objeto");
  *f = Forma();
  int t = j.buscar(obj, "forma");
  char tipo[16];
  if (j.texto(t, tipo, sizeof tipo) < 0) return MAL("forma: falta \"forma\"");

  // Atributos comunes.
  int c = j.buscar(obj, "color");
  if (c >= 0) {
    char col[LARGO_NOMBRE];
    if (j.texto(c, col, sizeof col) < 0 || !interpretarColor(col, &f->color, &f->acento))
      return MAL("forma: color desconocido");
  }
  f->fijo = j.booleano(j.buscar(obj, "fijo"), false);
  f->progreso = j.booleano(j.buscar(obj, "progreso"), false);
  float audio = -1;
  if (!numeroOpcional(j, obj, "audio", 0, 7, &audio)) return MAL("forma: audio fuera de rango");
  f->audio = static_cast<int8_t>(audio);

  const bool centro = numeroEn(j, obj, "x", -2, 2, &f->x) && numeroEn(j, obj, "y", -2, 2, &f->y);

  if (iguales(tipo, "pildora") || iguales(tipo, "corte")) {
    f->tipo = iguales(tipo, "pildora") ? TipoForma::Pildora : TipoForma::Corte;
    if (!centro || !numeroEn(j, obj, "w", 0, 2, &f->w) || !numeroEn(j, obj, "h", 0, 2, &f->h))
      return MAL("pildora/corte: faltan x, y, w o h");
    if (f->tipo == TipoForma::Pildora) {
      if (!numeroOpcional(j, obj, "r", 0, 2, &f->r)) return MAL("pildora: r fuera de rango");
      f->relleno = j.booleano(j.buscar(obj, "relleno"), true);
      if (!numeroOpcional(j, obj, "trazo", 0, 2, &f->trazo)) return MAL("pildora: trazo");
      if (!f->relleno && f->trazo <= 0) return MAL("pildora: sin relleno necesita trazo");
    } else {
      if (!numeroOpcional(j, obj, "izq", 0, 2, &f->izq) ||
          !numeroOpcional(j, obj, "der", 0, 2, &f->der))
        return MAL("corte: izq/der fuera de rango");
    }
  } else if (iguales(tipo, "circulo")) {
    f->tipo = TipoForma::Circulo;
    if (!centro || !numeroEn(j, obj, "r", 0, 2, &f->r)) return MAL("circulo: faltan x, y o r");
  } else if (iguales(tipo, "arco")) {
    f->tipo = TipoForma::Arco;
    if (!leerPuntos(j, obj, 3, 3, f)) return MAL("arco: necesita 3 puntos");
    if (!numeroEn(j, obj, "trazo", 0, 2, &f->trazo)) return MAL("arco: falta trazo");
  } else if (iguales(tipo, "corazon")) {
    f->tipo = TipoForma::Corazon;
    if (!centro) return MAL("corazon: faltan x o y");
    if (!numeroOpcional(j, obj, "tamano", 0, 2, &f->tamano)) return MAL("corazon: tamano");
  } else if (iguales(tipo, "espiral")) {
    f->tipo = TipoForma::Espiral;
    if (!centro || !numeroEn(j, obj, "r0", 0, 2, &f->r0) ||
        !numeroEn(j, obj, "paso", 0, 2, &f->paso) || !numeroEn(j, obj, "giro", 0, 40, &f->giro) ||
        !numeroEn(j, obj, "trazo", 0, 2, &f->trazo))
      return MAL("espiral: faltan campos");
  } else if (iguales(tipo, "poligono")) {
    f->tipo = TipoForma::Poligono;
    if (!leerPuntos(j, obj, 3, MAX_PUNTOS, f)) return MAL("poligono: de 3 a 12 puntos");
  } else if (iguales(tipo, "franjas")) {
    f->tipo = TipoForma::Franjas;
    int arr = j.buscar(obj, "bandas");
    if (!j.valido(arr) || j.tipo(arr) != TipoJson::Arreglo || j.cantidad(arr) > MAX_BANDAS)
      return MAL("franjas: hasta 6 bandas");
    for (int k = 0; k < j.cantidad(arr); k++) {
      int b = j.elemento(arr, k);
      Banda& bd = f->bandas[k];
      if (!numeroEn(j, b, "y", -2, 2, &bd.y) || !numeroEn(j, b, "h", 0, 2, &bd.h) ||
          !numeroEn(j, b, "dx", -2, 2, &bd.dx))
        return MAL("franjas: banda incompleta");
    }
    f->nPuntos = static_cast<uint8_t>(j.cantidad(arr));
  } else {
    return MAL("forma desconocida");
  }
  return {};
}

ResultadoConfig leerExpresion(const Json& j, int obj, Expresion* e) {
  if (!j.valido(obj) || j.tipo(obj) != TipoJson::Objeto)
    return error(ErrorConfig::Campo, "expresion: no es un objeto");
  e->parpadea = j.booleano(j.buscar(obj, "parpadea"), true);
  int arr = j.buscar(obj, "formas");
  if (!j.valido(arr) || j.tipo(arr) != TipoJson::Arreglo || j.cantidad(arr) < 1)
    return error(ErrorConfig::Forma, "expresion: falta \"formas\"");
  if (j.cantidad(arr) > MAX_FORMAS) return error(ErrorConfig::Demasiadas, "expresion: más de 12 formas");
  e->nFormas = 0;
  for (int k = 0; k < j.cantidad(arr); k++) {
    ResultadoConfig r = leerForma(j, j.elemento(arr, k), &e->formas[k]);
    if (!r.ok()) return r;
    e->nFormas++;
  }
  return {};
}

bool nombreValido(const char* s) {
  size_t n = largo(s);
  if (n == 0 || n > 24 || s[0] < 'a' || s[0] > 'z') return false;
  for (size_t i = 0; i < n; i++) {
    char c = s[i];
    if (!((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-')) return false;
  }
  return true;
}

// Guarda la expresión en `c`: reemplaza la del mismo nombre o la agrega.
ResultadoConfig guardarExpresion(Config& c, const Expresion& e) {
  int i = c.indiceExpresion(e.nombre);
  if (i < 0) {
    if (c.nExpresiones >= MAX_EXPRESIONES) return error(ErrorConfig::Demasiadas, "más de 24 expresiones");
    i = c.nExpresiones++;
  }
  c.expresiones[i] = e;
  return {};
}

bool leerHora(const char* s, int16_t* minutos) {
  auto d = [](char c) { return c >= '0' && c <= '9'; };
  if (!d(s[0]) || !d(s[1]) || s[2] != ':' || !d(s[3]) || !d(s[4])) return false;
  int h = (s[0] - '0') * 10 + (s[1] - '0');
  int m = (s[3] - '0') * 10 + (s[4] - '0');
  if (h > 23 || m > 59) return false;
  *minutos = static_cast<int16_t>(h * 60 + m);
  return true;
}

// "bateria<15", "humor>0.5" o un nombre de evento.
bool leerCondicion(const char* s, Condicion* c) {
  if (eventoPorNombre(s, &c->evento)) {
    c->tipo = Condicion::Tipo::Evento;
    return true;
  }
  const char* op = s;
  while (*op && *op != '<' && *op != '>') op++;
  if (!*op) return false;
  size_t n = static_cast<size_t>(op - s);
  char var[16];
  copiarTexto(var, sizeof var, s, n);
  if (iguales(var, "bateria")) c->variable = Variable::Bateria;
  else if (iguales(var, "humor")) c->variable = Variable::Humor;
  else return false;
  c->tipo = *op == '<' ? Condicion::Tipo::Menor : Condicion::Tipo::Mayor;
  // Reusar el lector de números del JSON: el resto es un número JSON válido.
  const char* num = op + 1;
  Json jn;
  TokenJson t[1];
  if (jn.parsear(num, largo(num), t, 1) != 1 || jn.tipo(0) != TipoJson::Numero) return false;
  c->valor = jn.numero(0, 0);
  return true;
}

bool leerAccion(const char* s, Accion* a) {
  const char* dos = s;
  while (*dos && *dos != ':') dos++;
  if (!*dos || !dos[1]) return false;
  char tipo[16];
  copiarTexto(tipo, sizeof tipo, s, static_cast<size_t>(dos - s));
  if (iguales(tipo, "expresion")) a->tipo = Accion::Tipo::Expresion;
  else if (iguales(tipo, "sonido")) a->tipo = Accion::Tipo::Sonido;
  else if (iguales(tipo, "led")) a->tipo = Accion::Tipo::Led;
  else return false;
  return copiarTexto(a->argumento, sizeof a->argumento, dos + 1) > 0;
}

void reglaBase(Config& c, const char* si, const char* expresion) {
  Regla& r = c.reglas[c.nReglas++];
  r = Regla();
  leerCondicion(si, &r.si);
  r.nAcciones = 1;
  r.entonces[0].tipo = Accion::Tipo::Expresion;
  copiarTexto(r.entonces[0].argumento, LARGO_NOMBRE, expresion);
}

}  // namespace

const char* nombreEvento(Evento e) {
  auto i = static_cast<int>(e);
  return i < static_cast<int>(Evento::Cantidad) ? NOMBRES_EVENTO[i] : "?";
}

bool eventoPorNombre(const char* nombre, Evento* salida) {
  for (int i = 0; i < static_cast<int>(Evento::Cantidad); i++) {
    if (iguales(nombre, NOMBRES_EVENTO[i])) {
      *salida = static_cast<Evento>(i);
      return true;
    }
  }
  return false;
}

int Config::indiceExpresion(const char* n) const {
  for (int i = 0; i < nExpresiones; i++)
    if (iguales(expresiones[i].nombre, n)) return i;
  return -1;
}

bool interpretarColor(const char* s, uint32_t* rgb, bool* acento) {
  struct Token {
    const char* nombre;
    uint32_t valor;
  };
  static const Token tokens[] = {
      {"naranja", paleta::NARANJA},       {"naranja-profundo", paleta::NARANJA_PROFUNDO},
      {"pantalla", paleta::PANTALLA},     {"tinta", paleta::TINTA},
      {"tinta-suave", paleta::TINTA_SUAVE}, {"linea", paleta::LINEA},
  };
  for (const auto& t : tokens) {
    if (iguales(s, t.nombre)) {
      *rgb = t.valor;
      *acento = t.valor == paleta::NARANJA;
      return true;
    }
  }
  if (s[0] != '#' || largo(s) != 7) return false;
  uint32_t v = 0;
  for (int i = 1; i < 7; i++) {
    char c = s[i];
    int d = (c >= '0' && c <= '9') ? c - '0'
            : (c >= 'a' && c <= 'f') ? c - 'a' + 10
            : (c >= 'A' && c <= 'F') ? c - 'A' + 10
                                     : -1;
    if (d < 0) return false;
    v = v * 16 + static_cast<uint32_t>(d);
  }
  *rgb = v;
  *acento = false;
  return true;
}

ResultadoConfig aplicarExpresion(Config& c, const char* texto, size_t n) {
  Json j;
  if (j.parsear(texto, n, g_tokens, MAX_TOKENS_JSON) <= 0)
    return error(ErrorConfig::JsonInvalido, "expresion: JSON inválido");
  static Expresion e;
  e = Expresion();
  if (j.texto(j.buscar(0, "nombre"), e.nombre, sizeof e.nombre) < 0 || !nombreValido(e.nombre))
    return error(ErrorConfig::Campo, "expresion: nombre inválido");
  ResultadoConfig r = leerExpresion(j, 0, &e);
  if (!r.ok()) return r;
  return guardarExpresion(c, e);
}

ResultadoConfig cargarBase(Config& c) {
  c = Config();
  copiarTexto(c.nombre, sizeof c.nombre, "bicho");
  for (int i = 0; i < CANTIDAD_CARAS_BASE; i++) {
    ResultadoConfig r = aplicarExpresion(c, CARAS_BASE[i], largo(CARAS_BASE[i]));
    if (!r.ok()) return r;
  }
  // Disparadores del manual de marca.
  reglaBase(c, "tacto", "contento");
  reglaBase(c, "doble-toque", "sospecha");
  reglaBase(c, "levantado", "sorpresa");
  reglaBase(c, "sacudida", "mareado");
  reglaBase(c, "boca-abajo", "triste");
  reglaBase(c, "vinculado", "guino");
  reglaBase(c, "app-abierta", "enamorado");
  reglaBase(c, "despertado", "enojado");
  reglaBase(c, "bateria<15", "bateria-baja");
  return {};
}

namespace {
// Aplica la config de usuario sobre `c` (que es memoria de trabajo: si falla,
// queda a medio aplicar y el que llama la descarta).
ResultadoConfig aplicarSobre(Config& c, const char* texto, size_t n) {
  if (n > MAX_CONFIG_BYTES) return error(ErrorConfig::Demasiadas, "config: más de 8 KB");
  Json j;
  if (j.parsear(texto, n, g_tokens, MAX_TOKENS_JSON) <= 0 || j.tipo(0) != TipoJson::Objeto)
    return error(ErrorConfig::JsonInvalido, "config: JSON inválido");

  float version = 0;
  if (!numeroEn(j, 0, "version", 1, 1, &version)) return error(ErrorConfig::Version, "config: version debe ser 1");

  int nom = j.buscar(0, "nombre");
  if (nom >= 0) {
    int len = j.texto(nom, c.nombre, sizeof c.nombre);
    if (len <= 0) return error(ErrorConfig::Campo, "config: nombre inválido");
  }

  int cara = j.buscar(0, "cara");
  if (cara >= 0) {
    if (j.tipo(cara) != TipoJson::Objeto) return error(ErrorConfig::Campo, "cara: no es un objeto");
    int col = j.buscar(cara, "color");
    if (col >= 0) {
      char hex[LARGO_NOMBRE];
      bool acento;
      if (j.texto(col, hex, sizeof hex) != 7 || hex[0] != '#' ||
          !interpretarColor(hex, &c.colorOjos, &acento))
        return error(ErrorConfig::Campo, "cara: color debe ser #RRGGBB");
      c.hayColorOjos = true;
    }
    if (!numeroOpcional(j, cara, "tamano", 0.5f, 1.5f, &c.tamano))
      return error(ErrorConfig::Campo, "cara: tamano entre 0.5 y 1.5");
  }

  int per = j.buscar(0, "personalidad");
  if (per >= 0) {
    if (j.tipo(per) != TipoJson::Objeto) return error(ErrorConfig::Campo, "personalidad: no es un objeto");
    if (!numeroOpcional(j, per, "energia", 0, 1, &c.energia) ||
        !numeroOpcional(j, per, "timidez", 0, 1, &c.timidez))
      return error(ErrorConfig::Campo, "personalidad: valores entre 0 y 1");
    int s = j.buscar(per, "sueno");
    if (s >= 0) {
      char h[16];
      if (j.texto(s, h, sizeof h) != 11 || h[5] != '-' || !leerHora(h, &c.suenoDesde) ||
          !leerHora(h + 6, &c.suenoHasta))
        return error(ErrorConfig::Campo, "personalidad: sueno es HH:MM-HH:MM");
    }
  }

  int exs = j.buscar(0, "expresiones");
  if (exs >= 0) {
    if (j.tipo(exs) != TipoJson::Objeto) return error(ErrorConfig::Campo, "expresiones: no es un objeto");
    static Expresion e;
    int k = exs + 1;
    for (int i = 0; i < j.cantidad(exs); i++) {
      e = Expresion();
      if (j.texto(k, e.nombre, sizeof e.nombre) < 0 || !nombreValido(e.nombre))
        return error(ErrorConfig::Campo, "expresiones: nombre inválido");
      ResultadoConfig r = leerExpresion(j, k + 1, &e);
      if (!r.ok()) return r;
      r = guardarExpresion(c, e);
      if (!r.ok()) return r;
      k = j.saltar(k + 1);
    }
  }

  int reg = j.buscar(0, "reglas");
  if (reg >= 0) {
    if (j.tipo(reg) != TipoJson::Arreglo) return error(ErrorConfig::Regla, "reglas: no es un arreglo");
    // Las reglas del usuario van primero: ganan sobre las del manual.
    static Regla propias[MAX_REGLAS];
    int nPropias = j.cantidad(reg);
    if (nPropias > MAX_REGLAS) return error(ErrorConfig::Demasiadas, "reglas: más de 24");
    for (int i = 0; i < nPropias; i++) {
      int r = j.elemento(reg, i);
      Regla& regla = propias[i];
      regla = Regla();
      char si[LARGO_NOMBRE];
      if (j.texto(j.buscar(r, "si"), si, sizeof si) <= 0 || !leerCondicion(si, &regla.si))
        return error(ErrorConfig::Regla, "regla: \"si\" desconocido");
      int ent = j.buscar(r, "entonces");
      if (!j.valido(ent) || j.tipo(ent) != TipoJson::Arreglo || j.cantidad(ent) < 1 ||
          j.cantidad(ent) > MAX_ACCIONES)
        return error(ErrorConfig::Regla, "regla: \"entonces\" de 1 a 4 acciones");
      for (int a = 0; a < j.cantidad(ent); a++) {
        char acc[48];
        if (j.texto(j.elemento(ent, a), acc, sizeof acc) <= 0 || !leerAccion(acc, &regla.entonces[a]))
          return error(ErrorConfig::Regla, "regla: acción inválida");
        if (regla.entonces[a].tipo == Accion::Tipo::Expresion &&
            c.indiceExpresion(regla.entonces[a].argumento) < 0)
          return error(ErrorConfig::Regla, "regla: expresión que no existe");
        regla.nAcciones++;
      }
    }
    int total = nPropias + c.nReglas;
    if (total > MAX_REGLAS) total = MAX_REGLAS;
    for (int i = total - 1; i >= nPropias; i--) c.reglas[i] = c.reglas[i - nPropias];
    for (int i = 0; i < nPropias; i++) c.reglas[i] = propias[i];
    c.nReglas = static_cast<uint8_t>(total);
  }

  int modo = j.buscar(0, "modo");
  if (modo >= 0) {
    static const char* const modos[] = {"mascota", "reloj", "pomodoro", "clima", "notificaciones"};
    bool ok = false;
    for (int i = 0; i < 5; i++) {
      if (j.textoIgual(modo, modos[i])) {
        c.modo = static_cast<Modo>(i);
        ok = true;
      }
    }
    if (!ok) return error(ErrorConfig::Campo, "modo desconocido");
  }
  return {};
}
}  // namespace

ResultadoConfig aplicarConfig(Config& destino, const char* texto, size_t n) {
  g_copia = destino;
  ResultadoConfig r = aplicarSobre(g_copia, texto, n);
  if (r.ok()) destino = g_copia;
  return r;
}

ResultadoConfig cargarConfig(Config& destino, const char* texto, size_t n) {
  ResultadoConfig r = cargarBase(g_copia);
  if (r.ok()) r = aplicarSobre(g_copia, texto, n);
  if (r.ok()) destino = g_copia;
  return r;
}

}  // namespace sintetica
