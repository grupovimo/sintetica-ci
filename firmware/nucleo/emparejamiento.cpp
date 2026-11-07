#include "emparejamiento.h"

#include "formato.h"
#include "fuentes.h"
#include "qr.h"
#include "texto.h"

namespace sintetica {

namespace {
Qr g_qr;
}

void dibujarEmparejamiento(Lienzo& l, const char* serie, const char* contenidoQr, const char* estado) {
  l.limpiar(paleta::PANTALLA);
  // Medidas del manual, para la pantalla de 320 × 172.
  constexpr int PANEL_X = 20, PANEL_Y = 22, PANEL = 128;
  l.pildora(PANEL_X + PANEL / 2.0f, PANEL_Y + PANEL / 2.0f, PANEL, PANEL, 8, paleta::TINTA);

  if (g_qr.codificar(contenidoQr)) {
    const int lado = g_qr.lado();
    const int modulo = maxi(1, (PANEL - 16) / lado);  // deja al menos 2 módulos de margen
    const int x0 = PANEL_X + (PANEL - lado * modulo) / 2;
    const int y0 = PANEL_Y + (PANEL - lado * modulo) / 2;
    for (int y = 0; y < lado; y++)
      for (int x = 0; x < lado; x++)
        if (g_qr.oscuro(x, y)) l.rectangulo(x0 + x * modulo, y0 + y * modulo, modulo, modulo, paleta::PANTALLA);
  }

  char unidad[32];
  copiarTexto(unidad, sizeof unidad, "UNIDAD #");
  size_t n = largo(unidad);
  copiarTexto(unidad + n, sizeof unidad - n, (serie && serie[0]) ? serie : "—");
  constexpr float TEXTO_X = 164;
  dibujarTexto(l, fuentes::MONO_13, unidad, TEXTO_X, 44, paleta::NARANJA);
  dibujarTexto(l, fuentes::VOZ_20, "Escaneame", TEXTO_X, 82, paleta::TINTA);
  dibujarTexto(l, fuentes::VOZ_20, "con la app.", TEXTO_X, 106, paleta::TINTA);
  dibujarTexto(l, fuentes::MONO_9, estado, TEXTO_X, 142, paleta::TINTA_SUAVE);
}

}  // namespace sintetica
