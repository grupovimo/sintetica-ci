// Matemática mínima y autocontenida: el núcleo no depende de libc para poder
// compilarse igual en el ESP32, en la PC y en WebAssembly sin biblioteca estándar.
#pragma once

#include <stddef.h>
#include <stdint.h>

namespace sintetica {

constexpr float PI = 3.14159265358979f;
constexpr float DOS_PI = 6.28318530717959f;

inline float absf(float x) { return x < 0 ? -x : x; }
inline float minf(float a, float b) { return a < b ? a : b; }
inline float maxf(float a, float b) { return a > b ? a : b; }
inline int mini(int a, int b) { return a < b ? a : b; }
inline int maxi(int a, int b) { return a > b ? a : b; }
inline float limitar(float x, float lo, float hi) { return x < lo ? lo : (x > hi ? hi : x); }
inline float mezclar(float a, float b, float t) { return a + (b - a) * t; }
inline float raiz(float x) { return x <= 0 ? 0 : __builtin_sqrtf(x); }
inline float piso(float x) { return __builtin_floorf(x); }
inline float techo(float x) { return __builtin_ceilf(x); }

// Seno con reducción de rango y polinomio de Taylor hasta x^9 (error < 1e-5).
inline float seno(float x) {
  x = x - DOS_PI * piso((x + PI) / DOS_PI);  // a [-π, π)
  if (x > PI / 2) x = PI - x;
  if (x < -PI / 2) x = -PI - x;
  const float x2 = x * x;
  return x * (1.0f - x2 / 6.0f * (1.0f - x2 / 20.0f * (1.0f - x2 / 42.0f * (1.0f - x2 / 72.0f))));
}
inline float coseno(float x) { return seno(x + PI / 2); }

// Curva ease-in-out cúbica, la que pide el manual para el parpadeo.
inline float suave(float t) {
  t = limitar(t, 0.0f, 1.0f);
  if (t < 0.5f) return 4.0f * t * t * t;
  const float u = -2.0f * t + 2.0f;
  return 1.0f - u * u * u / 2.0f;
}

// Generador xorshift32: determinista con la misma semilla (los tests lo usan).
struct Azar {
  uint32_t estado = 0x5157E71Cu;
  void sembrar(uint32_t s) { estado = s ? s : 0x5157E71Cu; }
  uint32_t siguiente() {
    uint32_t x = estado;
    x ^= x << 13;
    x ^= x >> 17;
    x ^= x << 5;
    estado = x;
    return x;
  }
  // Entero en [lo, hi] incluido.
  uint32_t entre(uint32_t lo, uint32_t hi) { return lo + siguiente() % (hi - lo + 1); }
  // Real en [0, 1).
  float real() { return (siguiente() >> 8) * (1.0f / 16777216.0f); }
};

// Utilidades de bytes y texto sin libc.
inline void copiarBytes(void* dst, const void* src, size_t n) {
  auto* d = static_cast<uint8_t*>(dst);
  auto* s = static_cast<const uint8_t*>(src);
  for (size_t i = 0; i < n; i++) d[i] = s[i];
}
inline void llenarBytes(void* dst, uint8_t v, size_t n) {
  auto* d = static_cast<uint8_t*>(dst);
  for (size_t i = 0; i < n; i++) d[i] = v;
}
inline size_t largo(const char* s) {
  size_t n = 0;
  while (s && s[n]) n++;
  return n;
}
inline bool iguales(const char* a, const char* b) {
  if (!a || !b) return a == b;
  while (*a && *a == *b) a++, b++;
  return *a == *b;
}
// Copia con terminador; devuelve cuántos bytes copió (sin el terminador).
inline size_t copiarTexto(char* dst, size_t cap, const char* src, size_t n) {
  if (cap == 0) return 0;
  if (n > cap - 1) n = cap - 1;
  for (size_t i = 0; i < n; i++) dst[i] = src[i];
  dst[n] = 0;
  return n;
}
inline size_t copiarTexto(char* dst, size_t cap, const char* src) {
  return copiarTexto(dst, cap, src, largo(src));
}

}  // namespace sintetica
