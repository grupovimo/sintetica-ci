// SintéticaOS: junta la configuración, el motor de vida, los gestos, las
// reglas y el protocolo. Cada perfil de hardware solo tiene que llamar a
// iniciar() una vez y a tick() en cada vuelta, y pasarle las entradas.
#pragma once

#include "cara.h"
#include "formato.h"
#include "gestos.h"
#include "hal.h"
#include "lienzo.h"
#include "motor_vida.h"
#include "protocolo.h"

namespace sintetica {

constexpr const char* VERSION_SINTETICAOS = "0.1.0";

// Cómo es la placa: la pantalla y qué hace su pulsador.
struct Perfil {
  int ancho = 320, alto = 172;
  int filasFranja = 0;     // 0 = la pantalla de una vez; si no, de a tantas filas
  bool redonda = false;
  // Mascota (1.47B): toque = caricia, doble toque, mantener 5 s = QR.
  // Broche: toque = lo siguiente de la lista, mantener 2 s = dormir, 8 s = QR.
  enum class Boton : uint8_t { Mascota, Broche } boton = Boton::Mascota;
};

namespace tiemposBroche {
constexpr uint32_t TOQUE_MS = 600;
constexpr uint32_t DORMIR_MS = 2000;
constexpr uint32_t QR_MS = 8000;
}  // namespace tiemposBroche

class Sistema {
 public:
  // `pixeles` = ancho × filasFranja RGB565 (o ancho × alto si se dibuja de una
  // vez). Lee la configuración guardada (o usa la base).
  void iniciar(const Hal& hal, uint16_t* pixeles, const Perfil& perfil);
  void iniciar(const Hal& hal, uint16_t* pixeles, int ancho, int alto) {
    Perfil p;
    p.ancho = ancho, p.alto = alto;
    iniciar(hal, pixeles, p);
  }
  // Avanza el tiempo, dibuja y presenta un cuadro.
  void tick();

  // Entradas.
  void tacto(bool apretado);
  void imu(float ax, float ay, float az) { gestos_.imu(ax, ay, az, hal_.ahoraMs()); }
  void evento(Evento e);
  // Se conectó o desconectó un celu: cada conexión empieza sin autenticar.
  void bleConexion(bool conectado);
  // Una escritura en una característica BLE.
  void bleEscribir(uint8_t caracteristica, const uint8_t* datos, size_t n);
  // Lo que devuelve una lectura de la característica de estado.
  size_t estadoBle(uint8_t* salida, size_t cap) const;

  const Config& config() const { return config_; }
  const MotorVida& motor() const { return motor_; }
  MotorVida& motor() { return motor_; }
  const char* nombre() const { return nombre_[0] ? nombre_ : config_.nombre; }
  const Lienzo& lienzo() const { return lienzo_; }

  // Identidad y vínculo.
  void fijarSerie(const char* serie);  // lo graba el taller al armar la unidad
  const char* serie() const { return serie_; }
  const char* identificador() const { return id_; }
  const char* codigo() const { return codigo_; }
  const char* contenidoQr() const { return qr_; }
  bool emparejando() const { return emparejando_; }
  int cantidadDeClaves() const { return nClaves_; }
  // Da por autenticada la conexión actual (vista previa local en las apps).
  void confiar() { autenticado_ = true; }
  // Pasa a lo siguiente de la lista (en el broche, un toque).
  void siguiente();
  int posicionLista() const { return posLista_; }
  // El broche pidió dormir (mantener 2 s): el perfil apaga la pantalla y entra
  // en sueño profundo; al despertar, la placa arranca de nuevo. En el
  // simulador, la pantalla queda negra hasta el próximo toque.
  bool quiereDormir() const { return apagado_; }
  void dormir();
  const Perfil& perfil() const { return perfil_; }

  // Vista previa de las apps: nunca muestra el QR y confía en quien escribe.
  void modoVistaPrevia() {
    vistaPrevia_ = true;
    emparejando_ = false;
    autenticado_ = true;
  }

 private:
  void responder(uint8_t car, uint8_t op, protocolo::Codigo c, const char* detalle = nullptr);
  void aplicarAcciones(const Regla& r);
  int expresionDeEstado();
  void guardarAjustes();
  void leerAjustes();
  void enviarConfig();
  void notificarEstadoSiCambio();
  void leerIdentidad();
  void guardarClaves();
  void armarQr();
  void dibujarPantalla(uint32_t ahora);
  void aplicarLista(uint32_t ahora);
  int largoLista() const;
  ElementoLista elementoLista(int i) const;
  void botonBroche(bool apretado, uint32_t ahora);
  bool bloqueado(uint32_t ahora) const;
  void falla(uint32_t ahora);
  void log(const char* m) {
    if (hal_.registrar) hal_.registrar(m);
  }

  Hal hal_;
  Perfil perfil_;
  Lienzo lienzo_;
  Vista vista_{};
  Config config_;
  MotorVida motor_;
  Gestos gestos_;
  protocolo::Receptor receptor_;

  char nombre_[LARGO_NOMBRE] = {};       // nombre puesto por comando (pisa al de la config)
  char fijaGuardada_[LARGO_NOMBRE] = {}; // expresión fija guardada
  char serie_[13] = {};
  char id_[13] = {};
  char codigo_[7] = {};
  char qr_[40] = {};
  uint8_t claves_[4][16] = {};
  uint8_t nClaves_ = 0;
  bool emparejando_ = false;
  uint32_t emparejarHasta_ = 0;  // 0 = hasta que se vincule
  bool autenticado_ = false;
  bool vistaPrevia_ = false;
  uint8_t fallos_ = 0;
  uint32_t bloqueoHasta_ = 0;

  // Lista y pulsador del broche.
  uint8_t posLista_ = 0;
  uint32_t ultimaRotacion_ = 0;
  bool apagado_ = false;
  bool botonApretado_ = false;
  bool ignorarSoltar_ = false;
  bool qrPorBoton_ = false;
  uint32_t botonDesde_ = 0;

  uint8_t ultimoEstado_[96] = {};
  size_t largoUltimoEstado_ = 0;
};

}  // namespace sintetica
