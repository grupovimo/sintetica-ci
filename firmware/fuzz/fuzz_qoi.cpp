// Blanco 3: QOI y animaciones "sani" (qoi.cpp): las cabeceras, la tabla de
// cuadros y el decodificador que lee por partes, como lo usa el sistema.
//
// Entrada: [byte de control][archivo]. El control elige de a cuántos bytes
// entrega la lectura (la placa lee por partes) y si se decodifica en RGB565 o
// en RGBA.
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "qoi.h"

using namespace sintetica;

namespace {
struct Archivo {
  const uint8_t* d;
  uint32_t n;
  uint32_t trozo;  // como mucho, de a cuánto entrega
};

int leer(void* contexto, uint32_t desplazamiento, uint8_t* destino, size_t m) {
  const auto* a = static_cast<const Archivo*>(contexto);
  if (desplazamiento >= a->n) return 0;
  size_t k = a->n - desplazamiento;
  if (k > m) k = m;
  if (k > a->trozo) k = a->trozo;
  memcpy(destino, a->d + desplazamiento, k);
  return static_cast<int>(k);
}

uint16_t g_fila565[4096];
uint8_t g_filaRgba[4096 * 4];

// Decodifica una imagen entera (o hasta que se corten los datos).
void decodificar(const Archivo& a, bool rgba, uint32_t saltar) {
  DecodificadorQoi q;
  if (!q.empezar(leer, const_cast<Archivo*>(&a))) return;
  uint32_t w = 0, h = 0;
  uint8_t cab[14] = {};
  memcpy(cab, a.d, a.n < 14 ? a.n : 14);
  // La cabecera suelta y la del decodificador dicen lo mismo.
  if (!DecodificadorQoi::leerCabecera(cab, 14, &w, &h) || w != q.ancho() || h != q.alto()) abort();
  if (q.ancho() > 4096 || q.canales() < 3 || q.canales() > 4) abort();
  if (saltar && !q.saltarFilas(saltar % (q.alto() + 1))) return;
  while (q.fila() < q.alto()) {
    const bool ok = rgba ? q.siguienteFilaRgba(g_filaRgba) : q.siguienteFila(g_fila565);
    if (!ok) return;
  }
  // Pasado el final no hay más filas.
  if (q.siguienteFila(g_fila565)) abort();
}
}  // namespace

extern "C" int LLVMFuzzerTestOneInput(const uint8_t* datos, size_t n) {
  if (n < 1) return 0;
  const uint8_t control = datos[0];
  // El archivo en un buffer del tamaño justo (ASan ve cualquier byte de más).
  const size_t m = n - 1;
  uint8_t* d = static_cast<uint8_t*>(malloc(m ? m : 1));
  if (m) memcpy(d, datos + 1, m);
  Archivo a{d, static_cast<uint32_t>(m), static_cast<uint32_t>(1u + (control & 0x3F) * 5u)};
  const bool rgba = control & 0x40;
  const uint32_t saltar = (control & 0x80) ? 3 : 0;

  uint32_t w = 0, h = 0;
  DecodificadorQoi::leerCabecera(d, m, &w, &h);

  int cuadros = 0;
  if (animacion::leerCabecera(d, m, &cuadros, &w, &h)) {
    if (cuadros < 1 || cuadros > animacion::MAX_CUADROS) abort();
    // Igual que Sistema::imagenValida: la tabla, y cada cuadro es un QOI.
    const uint32_t tabla = static_cast<uint32_t>(animacion::CABECERA + animacion::POR_CUADRO * cuadros);
    for (int i = 0; i < cuadros; i++) {
      const size_t p = animacion::CABECERA + animacion::POR_CUADRO * static_cast<size_t>(i);
      if (p + animacion::POR_CUADRO > m) break;
      const animacion::Cuadro c = animacion::leerCuadro(d + p);
      if (c.ms < animacion::MIN_MS || c.desplazamiento < tabla || c.largo < 22 || c.desplazamiento > m ||
          c.largo > m - c.desplazamiento)
        continue;
      Archivo cuadro{d + c.desplazamiento, c.largo, a.trozo};
      decodificar(cuadro, rgba, saltar);
    }
  } else {
    decodificar(a, rgba, saltar);
  }
  free(d);
  return 0;
}
