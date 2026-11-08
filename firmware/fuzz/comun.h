// Lo compartido por los blancos de fuzzing: leer enteros de la entrada sin
// pasarse. Los arneses (no el núcleo) pueden usar la libc y la STL.
#pragma once

#include <stddef.h>
#include <stdint.h>

// Consume la entrada de a poco; cuando se acaba devuelve ceros.
struct Entrada {
  const uint8_t* d;
  size_t n;

  bool vacia() const { return n == 0; }
  uint8_t u8() {
    if (!n) return 0;
    n--;
    return *d++;
  }
  uint16_t u16() {
    const uint16_t a = u8();
    return static_cast<uint16_t>(a | u8() << 8);
  }
  // Hasta `m` bytes (menos si no hay): devuelve cuántos tomó.
  size_t tomar(size_t m, const uint8_t** p) {
    if (m > n) m = n;
    *p = d;
    d += m, n -= m;
    return m;
  }
};
