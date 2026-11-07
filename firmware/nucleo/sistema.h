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

class Sistema {
 public:
  // `pixeles` = ancho*alto RGB565. Lee la configuración guardada (o usa la base).
  void iniciar(const Hal& hal, uint16_t* pixeles, int ancho, int alto);
  // Avanza el tiempo, dibuja y presenta un cuadro.
  void tick();

  // Entradas.
  void tacto(bool apretado) { gestos_.tacto(apretado, hal_.ahoraMs()); }
  void imu(float ax, float ay, float az) { gestos_.imu(ax, ay, az, hal_.ahoraMs()); }
  void evento(Evento e);
  // Una escritura en una característica BLE.
  void bleEscribir(uint8_t caracteristica, const uint8_t* datos, size_t n);
  // Lo que devuelve una lectura de la característica de estado.
  size_t estadoBle(uint8_t* salida, size_t cap) const;

  const Config& config() const { return config_; }
  const MotorVida& motor() const { return motor_; }
  MotorVida& motor() { return motor_; }
  const char* nombre() const { return nombre_[0] ? nombre_ : config_.nombre; }
  const Lienzo& lienzo() const { return lienzo_; }

 private:
  void responder(uint8_t car, uint8_t op, protocolo::Codigo c, const char* detalle = nullptr);
  void aplicarAcciones(const Regla& r);
  int expresionDeEstado();
  void guardarAjustes();
  void leerAjustes();
  void enviarConfig();
  void notificarEstadoSiCambio();
  void log(const char* m) {
    if (hal_.registrar) hal_.registrar(m);
  }

  Hal hal_;
  Lienzo lienzo_;
  Vista vista_{};
  Config config_;
  MotorVida motor_;
  Gestos gestos_;
  protocolo::Receptor receptor_;

  char nombre_[LARGO_NOMBRE] = {};       // nombre puesto por comando (pisa al de la config)
  char fijaGuardada_[LARGO_NOMBRE] = {}; // expresión fija guardada
  uint8_t ultimoEstado_[64] = {};
  size_t largoUltimoEstado_ = 0;
};

}  // namespace sintetica
