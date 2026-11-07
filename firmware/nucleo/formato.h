// El formato de configuración en memoria: caras, reglas y personalidad.
// Es la traducción directa de shared/formato/config.schema.json, con límites
// fijos para no usar memoria dinámica.
#pragma once

#include "json.h"
#include "mate.h"

namespace sintetica {

constexpr int MAX_PUNTOS = 12;
constexpr int MAX_BANDAS = 6;
constexpr int MAX_FORMAS = 12;
constexpr int MAX_EXPRESIONES = 24;
constexpr int MAX_REGLAS = 24;
constexpr int MAX_ACCIONES = 4;
constexpr int LARGO_NOMBRE = 25;  // 24 bytes + terminador
constexpr int MAX_TOKENS_JSON = 1200;
constexpr size_t MAX_CONFIG_BYTES = 8192;

// Paleta de la marca, tema Pantalla (manual de marca, tokens.json).
namespace paleta {
constexpr uint32_t NARANJA = 0xFF6A33;
constexpr uint32_t NARANJA_PROFUNDO = 0xFF8A5C;
constexpr uint32_t PANTALLA = 0x0B0A09;
constexpr uint32_t TINTA = 0xF5F2EC;
constexpr uint32_t TINTA_SUAVE = 0xA39E96;
constexpr uint32_t LINEA = 0x2C2925;
}  // namespace paleta

enum class TipoForma : uint8_t { Pildora, Corte, Circulo, Arco, Corazon, Espiral, Poligono, Franjas };

struct Punto {
  float x, y;
};

struct Banda {
  float y, h, dx;
};

struct Forma {
  TipoForma tipo = TipoForma::Pildora;
  uint32_t color = paleta::NARANJA;
  bool acento = true;     // usa el color de ojos que elija el dueño
  bool fijo = false;      // no sigue mirada, respiración ni parpadeo
  bool relleno = true;
  bool progreso = false;
  int8_t audio = -1;
  float x = 0, y = 0, w = 0, h = 0, r = -1;  // r < 0: la mitad del lado corto
  float izq = 0, der = 0;
  float trazo = 0;
  float tamano = 1;
  float r0 = 0, paso = 0, giro = 0;
  uint8_t nPuntos = 0;
  union {
    Punto puntos[MAX_PUNTOS];
    Banda bandas[MAX_BANDAS];
  };
  Forma() : puntos{} {}
};

struct Expresion {
  char nombre[LARGO_NOMBRE] = {};
  bool parpadea = true;
  uint8_t nFormas = 0;
  Forma formas[MAX_FORMAS];
};

enum class Evento : uint8_t {
  Tacto,
  DobleToque,
  Levantado,
  Sacudida,
  BocaAbajo,
  Vinculado,
  AppAbierta,
  Despertado,
  Cantidad
};
const char* nombreEvento(Evento e);
bool eventoPorNombre(const char* nombre, Evento* salida);

enum class Variable : uint8_t { Bateria, Humor };

struct Condicion {
  enum class Tipo : uint8_t { Evento, Menor, Mayor } tipo = Tipo::Evento;
  Evento evento = Evento::Tacto;
  Variable variable = Variable::Bateria;
  float valor = 0;
};

struct Accion {
  enum class Tipo : uint8_t { Expresion, Sonido, Led } tipo = Tipo::Expresion;
  char argumento[LARGO_NOMBRE] = {};
};

struct Regla {
  Condicion si;
  uint8_t nAcciones = 0;
  Accion entonces[MAX_ACCIONES];
};

enum class Modo : uint8_t { Mascota, Reloj, Pomodoro, Clima, Notificaciones };

struct Config {
  int version = 1;
  char nombre[LARGO_NOMBRE] = {};
  bool hayColorOjos = false;
  uint32_t colorOjos = paleta::NARANJA;
  float tamano = 1.0f;
  float energia = 0.5f;
  float timidez = 0.3f;
  int16_t suenoDesde = -1;  // minutos desde la medianoche; -1 = sin horario
  int16_t suenoHasta = -1;
  Modo modo = Modo::Mascota;
  uint8_t nExpresiones = 0;
  Expresion expresiones[MAX_EXPRESIONES];
  uint8_t nReglas = 0;
  Regla reglas[MAX_REGLAS];

  int indiceExpresion(const char* nombre) const;
};

enum class ErrorConfig : uint8_t {
  Ninguno = 0,
  JsonInvalido,
  Version,
  Campo,          // un campo con tipo o valor fuera de rango
  Forma,          // una forma desconocida o incompleta
  Demasiadas,     // superó un límite (formas, expresiones, reglas)
  Regla,
};

struct ResultadoConfig {
  ErrorConfig error = ErrorConfig::Ninguno;
  const char* detalle = "";
  bool ok() const { return error == ErrorConfig::Ninguno; }
};

// Carga las 16 caras base embebidas y las reglas del manual.
ResultadoConfig cargarBase(Config& c);

// Aplica una expresión suelta (un archivo de shared/formato/caras).
ResultadoConfig aplicarExpresion(Config& c, const char* json, size_t largo);

// Aplica una configuración de usuario encima de la base. Si falla, `c` queda
// como estaba (se trabaja sobre una copia temporal).
ResultadoConfig aplicarConfig(Config& c, const char* json, size_t largo);

// La base más una configuración de usuario, de cero. Si falla, `destino`
// queda como estaba. Usa la misma memoria de trabajo que aplicarConfig.
ResultadoConfig cargarConfig(Config& destino, const char* json, size_t largo);

// Interpreta "#RRGGBB" o un token de la paleta. Devuelve false si no lo conoce.
bool interpretarColor(const char* s, uint32_t* rgb, bool* acento);

}  // namespace sintetica
