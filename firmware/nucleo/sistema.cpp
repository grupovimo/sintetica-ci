#include "sistema.h"

#include "emparejamiento.h"

namespace sintetica {

namespace {
constexpr const char* CLAVE_CONFIG = "config";
constexpr const char* CLAVE_AJUSTES = "ajustes";
constexpr const char* CLAVE_SERIE = "serie";
constexpr const char* CLAVE_CODIGO = "codigo";
constexpr const char* CLAVE_CLAVES = "claves";
constexpr uint32_t QR_POR_BOTON_MS = 120000;  // el QR del botón dura 2 min
constexpr uint8_t FALLOS_MAXIMOS = 5;
constexpr uint32_t BLOQUEO_MS = 30000;
// Sin letras ni números que se confundan (0/O, 1/I/L).
const char ALFABETO_CODIGO[] = "23456789ABCDEFGHJKMNPQRSTUVWXYZ";

// Memoria grande, fuera de la pila.
char g_textoConfig[MAX_CONFIG_BYTES + 1];
size_t g_largoConfig = 0;

const char CONFIG_VACIA[] = "{\"version\":1}";
}  // namespace

void Sistema::iniciar(const Hal& hal, uint16_t* pixeles, int ancho, int alto) {
  hal_ = hal;
  largoUltimoEstado_ = 0;  // que se notifique el estado del arranque
  receptor_.reiniciar();
  autenticado_ = false;
  fallos_ = 0;
  leerIdentidad();
  lienzo_.iniciar(pixeles, ancho, alto);
  vista_ = Vista::para(ancho, alto);

  ResultadoConfig r = cargarBase(config_);
  if (!r.ok()) log(r.detalle);

  // Configuración guardada.
  g_largoConfig = 0;
  int n = hal_.leer ? hal_.leer(CLAVE_CONFIG, reinterpret_cast<uint8_t*>(g_textoConfig), MAX_CONFIG_BYTES) : -1;
  if (n > 0) {
    ResultadoConfig rc = aplicarConfig(config_, g_textoConfig, static_cast<size_t>(n));
    if (rc.ok()) {
      g_largoConfig = static_cast<size_t>(n);
    } else {
      log("config guardada inválida; uso la base");
      log(rc.detalle);
    }
  }
  if (g_largoConfig == 0) {
    copiarTexto(g_textoConfig, sizeof g_textoConfig, CONFIG_VACIA);
    g_largoConfig = largo(CONFIG_VACIA);
  }
  g_textoConfig[g_largoConfig] = 0;

  const uint32_t ahora = hal_.ahoraMs();
  motor_.iniciar(&config_, hal_.semilla ? hal_.semilla() : ahora, ahora);
  leerAjustes();
  log("SintéticaOS listo");
}

void Sistema::leerIdentidad() {
  const char* id = hal_.identificador ? hal_.identificador() : nullptr;
  copiarTexto(id_, sizeof id_, id && id[0] ? id : "000000000000");

  serie_[0] = 0;
  int n = hal_.leer ? hal_.leer(CLAVE_SERIE, reinterpret_cast<uint8_t*>(serie_), sizeof serie_ - 1) : -1;
  serie_[n > 0 ? n : 0] = 0;

  // El código del QR se genera una vez, en el primer arranque, y queda fijo.
  n = hal_.leer ? hal_.leer(CLAVE_CODIGO, reinterpret_cast<uint8_t*>(codigo_), 6) : -1;
  if (n != 6) {
    for (int i = 0; i < 6; i++) {
      uint32_t r = hal_.semilla ? hal_.semilla() : hal_.ahoraMs() * 2654435761u + static_cast<uint32_t>(i);
      codigo_[i] = ALFABETO_CODIGO[r % (sizeof ALFABETO_CODIGO - 1)];
    }
    if (hal_.guardar) hal_.guardar(CLAVE_CODIGO, reinterpret_cast<const uint8_t*>(codigo_), 6);
  }
  codigo_[6] = 0;

  uint8_t b[4 * 16];
  n = hal_.leer ? hal_.leer(CLAVE_CLAVES, b, sizeof b) : -1;
  nClaves_ = n > 0 ? static_cast<uint8_t>(n / 16) : 0;
  copiarBytes(claves_, b, nClaves_ * 16u);
  emparejando_ = nClaves_ == 0;  // sin ningún celu vinculado, muestra el QR
  emparejarHasta_ = 0;
  armarQr();
}

void Sistema::armarQr() {
  copiarTexto(qr_, sizeof qr_, PREFIJO_QR);
  size_t n = largo(qr_);
  n += copiarTexto(qr_ + n, sizeof qr_ - n, id_);
  n += copiarTexto(qr_ + n, sizeof qr_ - n, "/");
  copiarTexto(qr_ + n, sizeof qr_ - n, codigo_);
}

void Sistema::fijarSerie(const char* serie) {
  copiarTexto(serie_, sizeof serie_, serie);
  if (hal_.guardar) hal_.guardar(CLAVE_SERIE, reinterpret_cast<const uint8_t*>(serie_), largo(serie_));
}

void Sistema::guardarClaves() {
  if (hal_.guardar) hal_.guardar(CLAVE_CLAVES, &claves_[0][0], nClaves_ * 16u);
}

bool Sistema::bloqueado(uint32_t ahora) const {
  return fallos_ >= FALLOS_MAXIMOS && static_cast<int32_t>(ahora - bloqueoHasta_) < 0;
}

void Sistema::falla(uint32_t ahora) {
  if (fallos_ < 255) fallos_++;
  if (fallos_ >= FALLOS_MAXIMOS) bloqueoHasta_ = ahora + BLOQUEO_MS;
}

void Sistema::bleConexion(bool conectado) {
  autenticado_ = false;
  receptor_.reiniciar();
  log(conectado ? "celu conectado" : "celu desconectado");
}

void Sistema::leerAjustes() {
  nombre_[0] = 0;
  fijaGuardada_[0] = 0;
  uint8_t b[64];
  int n = hal_.leer ? hal_.leer(CLAVE_AJUSTES, b, sizeof b) : -1;
  // [versión=1][n][nombre][n][expresión fija]
  if (n < 3 || b[0] != 1) return;
  size_t i = 1;
  size_t nn = b[i++];
  if (i + nn >= static_cast<size_t>(n)) return;
  copiarTexto(nombre_, sizeof nombre_, reinterpret_cast<const char*>(b + i), nn);
  i += nn;
  size_t ne = b[i++];
  if (i + ne > static_cast<size_t>(n)) return;
  copiarTexto(fijaGuardada_, sizeof fijaGuardada_, reinterpret_cast<const char*>(b + i), ne);
  motor_.fijar(config_.indiceExpresion(fijaGuardada_));
}

void Sistema::guardarAjustes() {
  if (!hal_.guardar) return;
  uint8_t b[64];
  size_t i = 0;
  b[i++] = 1;
  size_t nn = largo(nombre_), ne = largo(fijaGuardada_);
  b[i++] = static_cast<uint8_t>(nn);
  copiarBytes(b + i, nombre_, nn);
  i += nn;
  b[i++] = static_cast<uint8_t>(ne);
  copiarBytes(b + i, fijaGuardada_, ne);
  i += ne;
  hal_.guardar(CLAVE_AJUSTES, b, i);
}

int Sistema::expresionDeEstado() {
  const int bat = hal_.bateria ? hal_.bateria() : -1;
  for (int i = 0; i < config_.nReglas; i++) {
    const Regla& r = config_.reglas[i];
    if (r.si.tipo == Condicion::Tipo::Evento) continue;
    float v;
    if (r.si.variable == Variable::Bateria) {
      if (bat < 0) continue;
      v = static_cast<float>(bat);
    } else {
      v = motor_.humor();
    }
    const bool cumple = r.si.tipo == Condicion::Tipo::Menor ? v < r.si.valor : v > r.si.valor;
    if (!cumple) continue;
    for (int a = 0; a < r.nAcciones; a++)
      if (r.entonces[a].tipo == Accion::Tipo::Expresion) return config_.indiceExpresion(r.entonces[a].argumento);
  }
  return -1;
}

void Sistema::aplicarAcciones(const Regla& r) {
  const uint32_t ahora = hal_.ahoraMs();
  for (int a = 0; a < r.nAcciones; a++) {
    const Accion& acc = r.entonces[a];
    switch (acc.tipo) {
      case Accion::Tipo::Expresion:
        motor_.reaccionar(config_.indiceExpresion(acc.argumento), ahora);
        break;
      case Accion::Tipo::Sonido:
        if (hal_.sonido) hal_.sonido(acc.argumento);
        break;
      case Accion::Tipo::Led: {
        uint32_t rgb;
        bool acento;
        if (hal_.led && interpretarColor(acc.argumento, &rgb, &acento)) hal_.led(rgb);
        break;
      }
    }
  }
}

void Sistema::evento(Evento e) {
  const uint32_t ahora = hal_.ahoraMs();
  // Tocarlo mientras duerme es despertarlo de la siesta.
  if (e == Evento::Tacto && motor_.dormido()) e = Evento::Despertado;
  motor_.interaccion(ahora);
  for (int i = 0; i < config_.nReglas; i++) {
    const Regla& r = config_.reglas[i];
    if (r.si.tipo == Condicion::Tipo::Evento && r.si.evento == e) {
      aplicarAcciones(r);
      break;
    }
  }
}

void Sistema::tick() {
  const uint32_t ahora = hal_.ahoraMs();
  gestos_.tick(ahora);
  Evento e;
  while (gestos_.sacar(&e)) evento(e);

  Contexto ctx;
  ctx.minutoDelDia = hal_.minutoDelDia ? hal_.minutoDelDia() : -1;
  ctx.expresionEstado = expresionDeEstado();
  motor_.tick(ahora, ctx);

  // Mantener el botón 5 s muestra el QR para vincular otro celu.
  if (gestos_.sacarMantenido()) {
    emparejando_ = true;
    emparejarHasta_ = ahora + QR_POR_BOTON_MS;
  }
  if (emparejando_ && emparejarHasta_ && nClaves_ > 0 && static_cast<int32_t>(ahora - emparejarHasta_) >= 0)
    emparejando_ = false;

  if (emparejando_) {
    dibujarEmparejamiento(lienzo_, serie_, qr_, "ESPERANDO VÍNCULO");
  } else {
    lienzo_.limpiar(paleta::PANTALLA);
    dibujarExpresion(lienzo_, vista_, config_.expresiones[motor_.expresionVisible()], motor_.estado());
  }
  if (hal_.presentar) hal_.presentar(lienzo_.pixeles(), lienzo_.ancho(), lienzo_.alto());
  notificarEstadoSiCambio();
}

size_t Sistema::estadoBle(uint8_t* salida, size_t cap) const {
  protocolo::EstadoPublico p;
  const int bat = hal_.bateria ? hal_.bateria() : -1;
  p.bateria = bat < 0 ? 255 : static_cast<uint8_t>(bat);
  p.humor = static_cast<int8_t>(motor_.humor() * 100);
  p.modo = static_cast<uint8_t>(config_.modo);
  p.dormido = motor_.dormido();
  p.fija = motor_.expresionFija() >= 0;
  p.emparejando = emparejando_;
  p.autenticado = autenticado_;
  p.expresion = config_.expresiones[motor_.expresionVisible()].nombre;
  p.nombre = nombre();
  p.serie = serie_;
  p.id = id_;
  return protocolo::codificarEstado(p, salida, cap);
}

void Sistema::notificarEstadoSiCambio() {
  if (!hal_.notificar) return;
  uint8_t b[96];
  size_t n = estadoBle(b, sizeof b);
  // El humor cambia de a poco: no notificar por eso solo.
  bool igual = n == largoUltimoEstado_;
  for (size_t i = 0; igual && i < n; i++) igual = i == 2 || b[i] == ultimoEstado_[i];
  if (igual) return;
  copiarBytes(ultimoEstado_, b, n);
  largoUltimoEstado_ = n;
  hal_.notificar(protocolo::CarEstado, b, n);
}

void Sistema::responder(uint8_t car, uint8_t op, protocolo::Codigo c, const char* detalle) {
  if (!hal_.notificar) return;
  uint8_t b[protocolo::MAX_NOTIFICACION];
  size_t i = 0;
  b[i++] = static_cast<uint8_t>(op | protocolo::Respuesta);
  b[i++] = c;
  if (detalle) {
    size_t n = largo(detalle);
    if (n > sizeof b - i) n = sizeof b - i;
    copiarBytes(b + i, detalle, n);
    i += n;
  }
  hal_.notificar(car, b, i);
}

void Sistema::enviarConfig() {
  if (!hal_.notificar) return;
  uint8_t b[protocolo::MAX_NOTIFICACION];
  const size_t porTrozo = protocolo::MAX_NOTIFICACION - 5;
  for (size_t off = 0; off < g_largoConfig; off += porTrozo) {
    size_t n = g_largoConfig - off < porTrozo ? g_largoConfig - off : porTrozo;
    b[0] = protocolo::ConfigTrozo;
    protocolo::escribir32(b + 1, static_cast<uint32_t>(off));
    copiarBytes(b + 5, g_textoConfig + off, n);
    hal_.notificar(protocolo::CarConfig, b, 5 + n);
  }
  b[0] = protocolo::ConfigCierre;
  protocolo::escribir32(b + 1, static_cast<uint32_t>(g_largoConfig));
  protocolo::escribir32(b + 5, protocolo::crc32(reinterpret_cast<const uint8_t*>(g_textoConfig), g_largoConfig));
  hal_.notificar(protocolo::CarConfig, b, 9);
}

void Sistema::bleEscribir(uint8_t car, const uint8_t* d, size_t n) {
  using namespace protocolo;
  if (n < 1) return;
  const uint8_t op = d[0];
  const uint32_t ahora = hal_.ahoraMs();

  // Vincular y autenticar son lo único que se puede hacer sin estar autenticado.
  if (car == CarComando && op == CmdVincular) {
    if (bloqueado(ahora)) return responder(car, op, NoAutorizado, "demasiados intentos: esperá 30 s");
    if (!emparejando_) return responder(car, op, NoEmparejando, "mantené el botón 5 s para ver el QR");
    if (n != 7 || !iguales(reinterpret_cast<const char*>(d + 1), codigo_, 6)) {
      falla(ahora);
      return responder(car, op, NoAutorizado, "el código no coincide");
    }
    // Clave nueva para este celu; si ya hay 4, se olvida la más vieja.
    if (nClaves_ == 4) {
      for (int i = 0; i < 3; i++) copiarBytes(claves_[i], claves_[i + 1], 16);
      nClaves_ = 3;
    }
    uint8_t* clave = claves_[nClaves_++];
    for (int i = 0; i < 16; i += 4) {
      uint32_t r = hal_.semilla ? hal_.semilla() : ahora * 2654435761u + static_cast<uint32_t>(i);
      for (int k = 0; k < 4; k++) clave[i + k] = static_cast<uint8_t>(r >> (8 * k));
    }
    guardarClaves();
    fallos_ = 0;
    autenticado_ = true;
    emparejando_ = false;
    uint8_t r[18] = {static_cast<uint8_t>(op | Respuesta), Ok};
    copiarBytes(r + 2, clave, 16);
    if (hal_.notificar) hal_.notificar(car, r, sizeof r);
    evento(Evento::Vinculado);
    log("celu vinculado");
    return;
  }
  if (car == CarComando && op == CmdAutenticar) {
    if (bloqueado(ahora)) return responder(car, op, NoAutorizado, "demasiados intentos: esperá 30 s");
    bool ok = false;
    for (int k = 0; n == 17 && k < nClaves_; k++) {
      uint8_t dif = 0;  // comparar todo, sin cortar en el primer byte distinto
      for (int i = 0; i < 16; i++) dif |= claves_[k][i] ^ d[1 + i];
      ok = ok || dif == 0;
    }
    if (!ok) {
      falla(ahora);
      return responder(car, op, NoAutorizado, "clave desconocida: hay que vincular de nuevo");
    }
    fallos_ = 0;
    autenticado_ = true;
    return responder(car, op, Ok);
  }
  if (!autenticado_) return responder(car, op, NoAutorizado, "vinculá o autenticá primero");

  if (car == CarConfig) {
    switch (op) {
      case ConfigInicio:
        if (n != 9) return responder(car, op, Malformado);
        return responder(car, op, receptor_.inicio(leer32(d + 1), leer32(d + 5)));
      case ConfigBloque:
        if (n < 5) return responder(car, op, Malformado);
        {
          Codigo c = receptor_.bloque(leer32(d + 1), d + 5, n - 5);
          // Los bloques bien recibidos no se confirman uno por uno.
          if (c != Ok) responder(car, op, c);
        }
        return;
      case ConfigFin: {
        Codigo c = receptor_.fin();
        if (c != Ok) return responder(car, op, c);
        ResultadoConfig r = cargarConfig(config_, receptor_.texto(), receptor_.largo());
        if (!r.ok()) {
          return responder(car, op, r.error == ErrorConfig::JsonInvalido ? JsonInvalido : ConfigInvalida, r.detalle);
        }
        g_largoConfig = receptor_.largo();
        copiarBytes(g_textoConfig, receptor_.texto(), g_largoConfig);
        g_textoConfig[g_largoConfig] = 0;
        if (hal_.guardar) hal_.guardar(CLAVE_CONFIG, reinterpret_cast<const uint8_t*>(g_textoConfig), g_largoConfig);
        // Un nombre nuevo en la config gana sobre el puesto por comando.
        nombre_[0] = 0;
        const int fija = config_.indiceExpresion(fijaGuardada_);
        if (fija < 0) fijaGuardada_[0] = 0;
        guardarAjustes();
        motor_.iniciar(&config_, hal_.semilla ? hal_.semilla() : hal_.ahoraMs(), hal_.ahoraMs());
        motor_.fijar(fija);
        log("config nueva aplicada");
        return responder(car, op, Ok);
      }
      case ConfigPedir:
        return enviarConfig();
      default:
        return responder(car, op, Malformado);
    }
  }

  if (car == CarComando) {
    switch (op) {
      case CmdExpresion: {
        char nombre[LARGO_NOMBRE];
        if (n - 1 >= LARGO_NOMBRE) return responder(car, op, NoExiste);
        copiarTexto(nombre, sizeof nombre, reinterpret_cast<const char*>(d + 1), n - 1);
        int idx = n == 1 ? -1 : config_.indiceExpresion(nombre);
        if (n > 1 && idx < 0) return responder(car, op, NoExiste);
        motor_.fijar(idx);
        copiarTexto(fijaGuardada_, sizeof fijaGuardada_, idx < 0 ? "" : nombre);
        guardarAjustes();
        return responder(car, op, Ok);
      }
      case CmdNombre: {
        if (n < 2 || n - 1 > LARGO_NOMBRE - 1) return responder(car, op, Malformado);
        copiarTexto(nombre_, sizeof nombre_, reinterpret_cast<const char*>(d + 1), n - 1);
        guardarAjustes();
        return responder(car, op, Ok);
      }
      case CmdDesvincular: {
        nClaves_ = 0;
        llenarBytes(claves_, 0, sizeof claves_);
        guardarClaves();
        emparejando_ = true;
        emparejarHasta_ = 0;
        responder(car, op, Ok);
        autenticado_ = false;
        return;
      }
      case CmdEvento: {
        if (n != 2 || d[1] >= static_cast<uint8_t>(Evento::Cantidad)) return responder(car, op, Malformado);
        evento(static_cast<Evento>(d[1]));
        return responder(car, op, Ok);
      }
      default:
        return responder(car, op, Malformado);
    }
  }

  if (car == CarOta) return responder(car, op, NoImplementado);
}

}  // namespace sintetica
