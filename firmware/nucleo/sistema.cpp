#include "sistema.h"

#include "emparejamiento.h"

namespace sintetica {

namespace {
constexpr const char* CLAVE_CONFIG = "config";
constexpr const char* CLAVE_AJUSTES = "ajustes";
constexpr const char* CLAVE_SERIE = "serie";
constexpr const char* CLAVE_CODIGO = "codigo";
constexpr const char* CLAVE_CLAVES = "claves";
constexpr const char* CLAVE_IMAGENES = "imagenes";

void claveImagen(char* destino, int espacio) {
  copiarTexto(destino, 6, "img0");
  destino[3] = static_cast<char>('0' + espacio);
}

uint16_t g_filaImagen[MAX_ANCHO];
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

void Sistema::iniciar(const Hal& hal, uint16_t* pixeles, const Perfil& perfil) {
  hal_ = hal;
  perfil_ = perfil;
  vistaPrevia_ = false;
  apagado_ = false;
  botonApretado_ = false;
  ignorarSoltar_ = false;
  posLista_ = 0;
  largoUltimoEstado_ = 0;  // que se notifique el estado del arranque
  receptor_.reiniciar();
  autenticado_ = false;
  fallos_ = 0;
  leerIdentidad();
  lienzo_.iniciar(pixeles, perfil.ancho, perfil.alto, perfil.filasFranja);
  vista_ = Vista::para(perfil.ancho, perfil.alto, perfil.redonda);

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
  leerImagenes();
  azar_.sembrar(hal_.semilla ? hal_.semilla() : ahora);
  subiendo_ = -1;
  mostrandoImagen_ = -1;
  imagenSucia_ = true;
  proximaVueltaCara_ = vueltaCaraHasta_ = 0;
  ultimaRotacion_ = ahora;
  aplicarLista(ahora);
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
  autenticado_ = vistaPrevia_;
  receptor_.reiniciar();
  log(conectado ? "celu conectado" : "celu desconectado");
}

void Sistema::leerAjustes() {
  nombre_[0] = 0;
  fijaGuardada_[0] = 0;
  uint8_t b[64];
  int n = hal_.leer ? hal_.leer(CLAVE_AJUSTES, b, sizeof b) : -1;
  // [versión=2][n][nombre][n][expresión fija][posición en la lista] (la v1 no tenía la posición)
  if (n < 3 || (b[0] != 1 && b[0] != 2)) return;
  size_t i = 1;
  size_t nn = b[i++];
  if (i + nn >= static_cast<size_t>(n)) return;
  copiarTexto(nombre_, sizeof nombre_, reinterpret_cast<const char*>(b + i), nn);
  i += nn;
  size_t ne = b[i++];
  if (i + ne > static_cast<size_t>(n)) return;
  copiarTexto(fijaGuardada_, sizeof fijaGuardada_, reinterpret_cast<const char*>(b + i), ne);
  i += ne;
  motor_.fijar(config_.indiceExpresion(fijaGuardada_));
  if (b[0] == 2 && i < static_cast<size_t>(n)) posLista_ = b[i];
}

void Sistema::guardarAjustes() {
  if (!hal_.guardar) return;
  uint8_t b[64];
  size_t i = 0;
  b[i++] = 2;
  size_t nn = largo(nombre_), ne = largo(fijaGuardada_);
  b[i++] = static_cast<uint8_t>(nn);
  copiarBytes(b + i, nombre_, nn);
  i += nn;
  b[i++] = static_cast<uint8_t>(ne);
  copiarBytes(b + i, fijaGuardada_, ne);
  i += ne;
  b[i++] = posLista_;
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

void Sistema::leerImagenes() {
  for (auto& m : imagenes_) m = MetaImagen();
  uint8_t b[MAX_IMAGENES * 8];
  const int n = hal_.leer ? hal_.leer(CLAVE_IMAGENES, b, sizeof b) : -1;
  if (n != static_cast<int>(sizeof b)) return;
  for (int i = 0; i < MAX_IMAGENES; i++) {
    imagenes_[i].largo = protocolo::leer32(b + i * 8);
    imagenes_[i].crc = protocolo::leer32(b + i * 8 + 4);
  }
}

void Sistema::guardarImagenes() {
  if (!hal_.guardar) return;
  uint8_t b[MAX_IMAGENES * 8];
  for (int i = 0; i < MAX_IMAGENES; i++) {
    protocolo::escribir32(b + i * 8, imagenes_[i].largo);
    protocolo::escribir32(b + i * 8 + 4, imagenes_[i].crc);
  }
  hal_.guardar(CLAVE_IMAGENES, b, sizeof b);
}

int Sistema::leerParaQoi(void* contexto, uint32_t desplazamiento, uint8_t* destino, size_t n) {
  auto* s = static_cast<Sistema*>(contexto);
  if (!s->hal_.leerTrozo) return -1;
  char clave[6];
  claveImagen(clave, s->imagenLeyendo_);
  return s->hal_.leerTrozo(clave, desplazamiento, destino, n);
}

// Decodifica la imagen en la franja actual del lienzo. false si no se puede leer.
bool Sistema::dibujarImagen(int espacio) {
  const int y0 = lienzo_.franja();
  if (y0 == 0) {
    imagenLeyendo_ = espacio;
    if (!qoi_.empezar(leerParaQoi, this) || qoi_.ancho() != static_cast<uint32_t>(lienzo_.ancho()) ||
        qoi_.alto() != static_cast<uint32_t>(lienzo_.alto()))
      return false;
  }
  for (int y = y0; y < y0 + lienzo_.filasFranja(); y++) {
    if (!qoi_.siguienteFila(g_filaImagen)) return false;
    lienzo_.fila(y, g_filaImagen);
  }
  return true;
}

// La imagen que corresponde mostrar ahora, o -1 si va una cara.
int Sistema::imagenParaMostrar(uint32_t ahora) {
  if (apagado_ || emparejando_ || motor_.expresionFija() >= 0 || largoLista() == 0) return -1;
  const ElementoLista e = elementoLista(posLista_);
  if (e.tipo != ElementoLista::Tipo::Imagen || imagenes_[e.indice].largo == 0 || subiendo_ == e.indice) return -1;
  // Cada tanto vuelve la cara un par de segundos: nunca una imagen quieta del todo.
  if (proximaVueltaCara_ == 0)
    proximaVueltaCara_ = ahora + azar_.entre(tiemposImagen::VUELTA_MIN_MS, tiemposImagen::VUELTA_MAX_MS);
  if (static_cast<int32_t>(ahora - proximaVueltaCara_) >= 0) {
    vueltaCaraHasta_ = ahora + tiemposImagen::VUELTA_DURA_MS;
    proximaVueltaCara_ = vueltaCaraHasta_ + azar_.entre(tiemposImagen::VUELTA_MIN_MS, tiemposImagen::VUELTA_MAX_MS);
  }
  if (static_cast<int32_t>(ahora - vueltaCaraHasta_) < 0) return -1;
  return e.indice;
}

void Sistema::bleImagen(uint8_t op, const uint8_t* d, size_t n) {
  using namespace protocolo;
  const uint8_t car = CarImagen;
  char clave[6];
  switch (op) {
    case ImgInicio: {
      if (n != 10 || d[1] >= MAX_IMAGENES) return responder(car, op, Malformado);
      const uint32_t largo = leer32(d + 2);
      if (largo < 22 || largo > MAX_IMAGEN_BYTES) return responder(car, op, MuyGrande);
      subiendo_ = d[1];
      subidaLargo_ = largo;
      subidaCrc_ = leer32(d + 6);
      subidaRecibido_ = 0;
      subidaCrcAcum_ = 0;
      imagenes_[subiendo_] = MetaImagen();
      guardarImagenes();
      imagenSucia_ = true;
      return responder(car, op, Ok);
    }
    case ImgBloque: {
      if (subiendo_ < 0) return responder(car, op, SinInicio);
      if (n < 5) return responder(car, op, Malformado);
      const uint32_t off = leer32(d + 1);
      const size_t m = n - 5;
      claveImagen(clave, subiendo_);
      if (off != subidaRecibido_ || subidaRecibido_ + m > subidaLargo_ || !hal_.escribirTrozo ||
          !hal_.escribirTrozo(clave, off, d + 5, m)) {
        if (hal_.borrar) hal_.borrar(clave);
        subiendo_ = -1;
        return responder(car, op, off != subidaRecibido_ ? FueraDeOrden : MuyGrande);
      }
      subidaCrcAcum_ = crc32(d + 5, m, subidaCrcAcum_);
      subidaRecibido_ += static_cast<uint32_t>(m);
      return;  // los bloques buenos no se confirman uno por uno
    }
    case ImgFin: {
      if (subiendo_ < 0) return responder(car, op, SinInicio);
      const int espacio = subiendo_;
      subiendo_ = -1;
      claveImagen(clave, espacio);
      if (subidaRecibido_ != subidaLargo_ || subidaCrcAcum_ != subidaCrc_) {
        if (hal_.borrar) hal_.borrar(clave);
        return responder(car, op, subidaRecibido_ != subidaLargo_ ? FueraDeOrden : CrcDistinto);
      }
      uint8_t cab[14];
      uint32_t w = 0, h = 0;
      const bool cabeceraOk = hal_.leerTrozo && hal_.leerTrozo(clave, 0, cab, sizeof cab) == 14 &&
                              DecodificadorQoi::leerCabecera(cab, sizeof cab, &w, &h);
      if (!cabeceraOk || w != static_cast<uint32_t>(perfil_.ancho) || h != static_cast<uint32_t>(perfil_.alto)) {
        if (hal_.borrar) hal_.borrar(clave);
        return responder(car, op, ImagenInvalida, "tiene que ser un QOI del tamaño de la pantalla");
      }
      imagenes_[espacio].largo = subidaLargo_;
      imagenes_[espacio].crc = subidaCrc_;
      guardarImagenes();
      imagenSucia_ = true;
      log("imagen guardada");
      return responder(car, op, Ok);
    }
    case ImgBorrar: {
      if (n != 2 || d[1] >= MAX_IMAGENES) return responder(car, op, Malformado);
      claveImagen(clave, d[1]);
      if (hal_.borrar) hal_.borrar(clave);
      imagenes_[d[1]] = MetaImagen();
      guardarImagenes();
      imagenSucia_ = true;
      return responder(car, op, Ok);
    }
    case ImgListar: {
      if (!hal_.notificar) return;
      uint8_t b[3 + MAX_IMAGENES * 9];
      size_t i = 3;
      uint8_t mascara = 0;
      for (int k = 0; k < MAX_IMAGENES; k++) {
        if (!imagenes_[k].largo) continue;
        mascara |= static_cast<uint8_t>(1u << k);
        b[i++] = static_cast<uint8_t>(k);
        escribir32(b + i, imagenes_[k].largo);
        escribir32(b + i + 4, imagenes_[k].crc);
        i += 8;
      }
      b[0] = op | Respuesta;
      b[1] = Ok;
      b[2] = mascara;
      hal_.notificar(car, b, i);
      return;
    }
    default:
      return responder(car, op, Malformado);
  }
}

int Sistema::largoLista() const {
  // Sin lista propia, el broche recorre las caras; la mascota no tiene lista.
  if (config_.nLista > 0) return config_.nLista;
  return perfil_.boton == Perfil::Boton::Broche ? config_.nExpresiones : 0;
}

ElementoLista Sistema::elementoLista(int i) const {
  if (config_.nLista > 0) return config_.lista[i % config_.nLista];
  ElementoLista e;
  e.indice = static_cast<uint8_t>(i % config_.nExpresiones);
  return e;
}

void Sistema::aplicarLista(uint32_t ahora) {
  const int n = largoLista();
  if (n == 0) {
    motor_.base(-1);
    return;
  }
  if (posLista_ >= n) posLista_ = 0;
  const ElementoLista e = elementoLista(posLista_);
  // Las imágenes se dibujan aparte; mientras tanto la cara de base es la neutral.
  motor_.base(e.tipo == ElementoLista::Tipo::Cara ? e.indice : -1);
  (void)ahora;
}

void Sistema::siguiente() {
  const int n = largoLista();
  if (n == 0) return;
  posLista_ = static_cast<uint8_t>((posLista_ + 1) % n);
  ultimaRotacion_ = hal_.ahoraMs();
  aplicarLista(ultimaRotacion_);
  guardarAjustes();
}

void Sistema::dormir() {
  apagado_ = true;
  guardarAjustes();
  log("a dormir");
}

void Sistema::tacto(bool apretado) {
  const uint32_t ahora = hal_.ahoraMs();
  if (perfil_.boton == Perfil::Boton::Mascota) {
    gestos_.tacto(apretado, ahora);
    return;
  }
  botonBroche(apretado, ahora);
}

void Sistema::botonBroche(bool apretado, uint32_t ahora) {
  if (apretado == botonApretado_) return;
  botonApretado_ = apretado;
  if (apretado) {
    botonDesde_ = ahora;
    qrPorBoton_ = false;
    // Dormido, un toque lo despierta (en la placa, arranca de nuevo).
    ignorarSoltar_ = apagado_;
    if (apagado_) {
      apagado_ = false;
      motor_.interaccion(ahora);
      log("despierto");
    }
    return;
  }
  if (ignorarSoltar_) {
    ignorarSoltar_ = false;
    return;
  }
  const uint32_t dur = ahora - botonDesde_;
  if (qrPorBoton_) return;
  if (dur < tiemposBroche::TOQUE_MS) {
    motor_.interaccion(ahora);
    siguiente();
  } else if (dur >= tiemposBroche::DORMIR_MS) {
    dormir();
  }
}

void Sistema::dibujarPantalla(uint32_t ahora) {
  (void)ahora;
  if (apagado_) {
    lienzo_.limpiar(0x000000);
    return;
  }
  if (emparejando_) {
    dibujarEmparejamiento(lienzo_, serie_, qr_, "ESPERANDO VÍNCULO", perfil_.redonda);
    return;
  }
  lienzo_.limpiar(paleta::PANTALLA);
  int visible = motor_.expresionVisible();
  // Manteniendo el botón del broche para dormirlo: muestra que se va a dormir.
  if (perfil_.boton == Perfil::Boton::Broche && botonApretado_ && !ignorarSoltar_ &&
      ahora - botonDesde_ >= tiemposBroche::DORMIR_MS) {
    const int d = config_.indiceExpresion("dormido");
    if (d >= 0) visible = d;
  }
  dibujarExpresion(lienzo_, vista_, config_.expresiones[visible], motor_.estado());
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

  // Mantener el botón (5 s la mascota, 8 s el broche) muestra el QR para vincular otro celu.
  bool qr = gestos_.sacarMantenido();
  if (perfil_.boton == Perfil::Boton::Broche && botonApretado_ && !ignorarSoltar_ && !qrPorBoton_ &&
      ahora - botonDesde_ >= tiemposBroche::QR_MS) {
    qrPorBoton_ = true;
    qr = true;
  }
  if (qr) {
    emparejando_ = true;
    emparejarHasta_ = ahora + QR_POR_BOTON_MS;
  }
  // La lista rota sola cada tantos minutos.
  if (config_.rotarMinutos && largoLista() > 1 &&
      ahora - ultimaRotacion_ >= static_cast<uint32_t>(config_.rotarMinutos) * 60000u)
    siguiente();
  if (emparejando_ && emparejarHasta_ && nClaves_ > 0 && static_cast<int32_t>(ahora - emparejarHasta_) >= 0)
    emparejando_ = false;

  if (vistaPrevia_) emparejando_ = false;
  const int imagen = imagenParaMostrar(ahora);
  if (imagen >= 0 && imagen == mostrandoImagen_ && !imagenSucia_) {
    // La imagen ya está en la pantalla: no hace falta redibujarla.
    notificarEstadoSiCambio();
    return;
  }
  mostrandoImagen_ = imagen;
  imagenSucia_ = false;
  bool imagenOk = imagen >= 0;
  // De a una franja: en placas sin PSRAM no entra la pantalla entera.
  for (int y0 = 0; y0 < lienzo_.alto(); y0 += lienzo_.filasPorFranja()) {
    lienzo_.empezarFranja(y0);
    if (imagenOk) imagenOk = dibujarImagen(imagen);
    if (!imagenOk) dibujarPantalla(ahora);
    if (hal_.presentarFranja) hal_.presentarFranja(lienzo_.pixeles(), lienzo_.ancho(), y0, lienzo_.filasFranja());
  }
  if (!hal_.presentarFranja && hal_.presentar && lienzo_.filasPorFranja() >= lienzo_.alto())
    hal_.presentar(lienzo_.pixeles(), lienzo_.ancho(), lienzo_.alto());
  if (imagen >= 0 && !imagenOk) {
    // No se pudo leer: se marca vacía para no intentarlo en cada cuadro.
    imagenes_[imagen] = MetaImagen();
    mostrandoImagen_ = -1;
    log("no pude leer la imagen; la saco");
  }
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
  p.ancho = static_cast<uint16_t>(perfil_.ancho);
  p.alto = static_cast<uint16_t>(perfil_.alto);
  p.redonda = perfil_.redonda;
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
        aplicarLista(hal_.ahoraMs());
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

  if (car == CarImagen) return bleImagen(op, d, n);
  if (car == CarOta) return responder(car, op, NoImplementado);
}

}  // namespace sintetica
