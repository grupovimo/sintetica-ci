#include "cara.h"
#include "formato.h"
#include "prueba.h"

using namespace sintetica;

static Config g_c;
static uint16_t g_px[320 * 172];

static int contarNaranja(const Lienzo& l, int x0, int x1) {
  int n = 0;
  for (int y = 0; y < l.alto(); y++)
    for (int x = x0; x < x1; x++) {
      uint32_t c = de565(l.pixeles()[y * l.ancho() + x]);
      if (((c >> 16) & 0xFF) > 200) n++;
    }
  return n;
}

static void altoDelOjo(const Lienzo& l, int x, int* y0, int* y1) {
  *y0 = -1, *y1 = -1;
  for (int y = 0; y < l.alto(); y++) {
    uint32_t c = de565(l.pixeles()[y * l.ancho() + x]);
    if (((c >> 16) & 0xFF) > 128) {
      if (*y0 < 0) *y0 = y;
      *y1 = y;
    }
  }
}

PRUEBA("render: la neutral mide lo del manual (36 × 72 en x 110 / 210)") {
  cargarBase(g_c);
  Lienzo l;
  l.iniciar(g_px, 320, 172);
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(320, 172), g_c.expresiones[0], EstadoCara());
  int y0, y1;
  altoDelOjo(l, 110, &y0, &y1);
  VERIFICAR(y0 == 50 && y1 == 121);
  altoDelOjo(l, 210, &y0, &y1);
  VERIFICAR(y0 == 50 && y1 == 121);
  VERIFICAR(contarNaranja(l, 0, 160) == contarNaranja(l, 160, 320));
}

PRUEBA("render: parpadeo a mitad y cerrado (72 → 40 → 8 px)") {
  cargarBase(g_c);
  Lienzo l;
  l.iniciar(g_px, 320, 172);
  EstadoCara e;
  int y0, y1;
  e.cierre = 0.5f;
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(320, 172), g_c.expresiones[0], e);
  altoDelOjo(l, 110, &y0, &y1);
  VERIFICAR(y1 - y0 + 1 == 40);
  e.cierre = 1.0f;
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(320, 172), g_c.expresiones[0], e);
  altoDelOjo(l, 110, &y0, &y1);
  VERIFICAR(y1 - y0 + 1 == 8);
  VERIFICAR(y0 == 82);
}

PRUEBA("render: la mirada corre los ojos y no los indicadores") {
  cargarBase(g_c);
  Lienzo l;
  l.iniciar(g_px, 320, 172);
  EstadoCara e;
  e.miradaX = 10.0f / 160;
  const Expresion& pensando = g_c.expresiones[g_c.indiceExpresion("pensando")];
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(320, 172), pensando, e);
  // El ojo izquierdo (x 106..138) ahora va de 116 a 148.
  uint32_t c = de565(l.pixeles()[61 * 320 + 146]);
  VERIFICAR(((c >> 16) & 0xFF) > 200);
  // Los puntos siguen en x 240, y 140.
  c = de565(l.pixeles()[140 * 320 + 240]);
  VERIFICAR(((c >> 16) & 0xFF) > 140 && ((c >> 16) & 0xFF) < 200);
}

PRUEBA("render: color de ojos del dueño") {
  cargarBase(g_c);
  Lienzo l;
  l.iniciar(g_px, 320, 172);
  EstadoCara e;
  e.hayColorOjos = true;
  e.colorOjos = 0x33CCFF;
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(320, 172), g_c.expresiones[0], e);
  VERIFICAR(de565(l.pixeles()[86 * 320 + 110]) == de565(a565(0x33CCFF)));
}

PRUEBA("render: escala a una pantalla redonda de 240 × 240") {
  cargarBase(g_c);
  static uint16_t px[240 * 240];
  Lienzo l;
  l.iniciar(px, 240, 240);
  l.limpiar(paleta::PANTALLA);
  dibujarExpresion(l, Vista::para(240, 240), g_c.expresiones[0], EstadoCara());
  // Ojo de 36 × 72 a escala 120/160: 27 × 54, centrado en x 82,5 y 120.
  uint32_t c = de565(px[120 * 240 + 82]);
  VERIFICAR(((c >> 16) & 0xFF) > 200);
  c = de565(px[(120 - 30) * 240 + 82]);
  VERIFICAR(((c >> 16) & 0xFF) < 40);
}
