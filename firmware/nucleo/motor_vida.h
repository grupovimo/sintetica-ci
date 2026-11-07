// El motor de vida: lo que hace que el bicho nunca sea una imagen estática.
// Parpadea, mira, respira, cambia de humor, se duerme y cada tanto sorprende.
// Todo depende del reloj y del azar que le pasan, así los tests son exactos.
#pragma once

#include "cara.h"
#include "formato.h"
#include "mate.h"

namespace sintetica {

namespace tiempos {
constexpr uint32_t PARPADEO_MS = 160;          // manual: 160 ms ease-in-out
constexpr uint32_t PARPADEO_MIN_MS = 3000;     // manual: cada 3 a 7 s, al azar
constexpr uint32_t PARPADEO_MAX_MS = 7000;
constexpr uint32_t TRANSICION_MS = 120;        // manual: 120 ms con un parpadeo en el medio
constexpr uint32_t MIRADA_MS = 200;
constexpr uint32_t RESPIRACION_MS = 4200;
constexpr uint32_t REACCION_MS = 3000;
constexpr uint32_t SIESTA_MS = 10u * 60u * 1000u;           // manual: 10 min sin interacción
constexpr uint32_t ABANDONO_MS = 24u * 60u * 60u * 1000u;   // manual: 24 h sin uso
constexpr uint32_t DESPIERTO_POR_TOQUE_MS = 60u * 1000u;    // si lo despiertan de noche
}  // namespace tiempos

struct Contexto {
  int minutoDelDia = -1;  // -1 si no se sabe la hora
  int expresionEstado = -1;  // la que piden las reglas de estado (batería baja, etc.)
};

class MotorVida {
 public:
  void iniciar(const Config* config, uint32_t semilla, uint32_t ahora);
  void tick(uint32_t ahora, const Contexto& ctx);

  // Muestra una expresión por un rato (una reacción) y vuelve sola.
  void reaccionar(int expresion, uint32_t ahora, uint32_t duracion = tiempos::REACCION_MS);
  // Alguien lo tocó, lo movió o abrió la app: lo despierta y lo alegra.
  void interaccion(uint32_t ahora);
  // Deja una expresión fija (-1 vuelve al comportamiento automático).
  void fijar(int expresion);

  int expresionVisible() const { return actual_; }
  int expresionFija() const { return fija_; }
  bool dormido() const { return dormido_; }
  float humor() const { return humor_; }
  const EstadoCara& estado() const { return estado_; }
  EstadoCara& estado() { return estado_; }

  // Para los tests.
  uint32_t proximoParpadeo() const { return proximoParpadeo_; }
  bool parpadeando(uint32_t ahora) const { return ahora - inicioParpadeo_ < tiempos::PARPADEO_MS; }

 private:
  int elegirObjetivo(uint32_t ahora, const Contexto& ctx);
  bool enHorarioDeSueno(int minuto) const;
  void programarParpadeo(uint32_t ahora);
  void programarMirada(uint32_t ahora);
  void programarSorpresa(uint32_t ahora);
  int indice(const char* nombre) const { return config_->indiceExpresion(nombre); }

  const Config* config_ = nullptr;
  Azar azar_;
  EstadoCara estado_;

  int actual_ = 0;
  int destino_ = -1;          // expresión a la que se está pasando
  uint32_t inicioTransicion_ = 0;
  int fija_ = -1;
  int reaccion_ = -1;
  uint32_t reaccionHasta_ = 0;

  uint32_t proximoParpadeo_ = 0;
  uint32_t inicioParpadeo_ = 0xFFFF0000u;
  bool dobleParpadeo_ = false;

  uint32_t proximaMirada_ = 0;
  uint32_t inicioMirada_ = 0;
  float miradaDesdeX_ = 0, miradaDesdeY_ = 0, miradaHaciaX_ = 0, miradaHaciaY_ = 0;

  uint32_t proximaSorpresa_ = 0;
  uint32_t ultimaInteraccion_ = 0;
  uint32_t despiertoHasta_ = 0;
  uint32_t ultimoTick_ = 0;
  bool dormido_ = false;
  float humor_ = 0.2f;
  uint32_t inicio_ = 0;
};

}  // namespace sintetica
