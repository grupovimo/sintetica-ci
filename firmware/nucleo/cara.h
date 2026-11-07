// Dibuja una expresión en el lienzo con el estado vivo del momento: cuánto
// está cerrado el parpadeo, hacia dónde mira, la respiración, el color y el
// tamaño que eligió el dueño.
#pragma once

#include "formato.h"
#include "lienzo.h"

namespace sintetica {

// Cómo se pasa de coordenadas normalizadas a píxeles en una pantalla.
struct Vista {
  float escala;    // píxeles por unidad normalizada (media pantalla de ancho = 1)
  float origenX;   // píxel del centro (0, 0)
  float origenY;

  // La pantalla de referencia del manual (320 × 172) y cualquier otra con el
  // mismo ancho por unidad: escala = ancho / 2.
  static Vista para(int ancho, int alto) {
    return {ancho / 2.0f, ancho / 2.0f, alto / 2.0f};
  }
};

struct EstadoCara {
  float cierre = 0;          // 0 abierto, 1 cerrado (parpadeo)
  float miradaX = 0;         // corrimiento de los ojos, en unidades normalizadas
  float miradaY = 0;
  float respiracion = 0;     // -1..1, se aplica como una escala muy chica
  float tamano = 1;
  bool hayColorOjos = false;
  uint32_t colorOjos = paleta::NARANJA;
  float audio[8] = {1, 1, 1, 1, 1, 1, 1, 1};
  float progreso = 0.6176f;  // el valor del manual (84 de 136)
};

// El alto al que llega un ojo cerrado: 8 px en la pantalla de referencia.
constexpr float ALTO_CERRADO = 8.0f / 160.0f;
constexpr float AMPLITUD_RESPIRACION = 0.015f;

void dibujarExpresion(Lienzo& lienzo, const Vista& vista, const Expresion& e, const EstadoCara& estado);

// Caja de una forma en coordenadas normalizadas (para agrupar los ojos).
void cajaDeForma(const Forma& f, float* x0, float* y0, float* x1, float* y1);

}  // namespace sintetica
