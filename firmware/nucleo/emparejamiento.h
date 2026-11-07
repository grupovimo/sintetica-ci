// La pantalla de emparejamiento del manual (pantalla-emparejamiento.svg):
// el QR sobre papel claro a la izquierda; a la derecha la unidad en mono
// naranja, "Escaneame / con la app." en Fraunces y el estado en mono gris.
#pragma once

#include "lienzo.h"

namespace sintetica {

// Contenido del QR: "SINTETICA:V1/<id>/<código>", todo alfanumérico para que
// entre en un QR versión 2 (25 × 25 módulos).
constexpr const char* PREFIJO_QR = "SINTETICA:V1/";

void dibujarEmparejamiento(Lienzo& l, const char* serie, const char* contenidoQr, const char* estado);

}  // namespace sintetica
