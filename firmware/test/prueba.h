// Mini framework de tests (sin dependencias externas).
#pragma once
#include <cmath>
#include <cstdio>

namespace prueba {
struct Caso {
  const char* nombre;
  void (*fn)();
  Caso* siguiente;
};
inline Caso*& lista() {
  static Caso* l = nullptr;
  return l;
}
inline int& fallas() {
  static int f = 0;
  return f;
}
struct Registro {
  Registro(Caso* c) {
    c->siguiente = lista();
    lista() = c;
  }
};
}  // namespace prueba

#define PRUEBA_CONCAT2(a, b) a##b
#define PRUEBA_CONCAT(a, b) PRUEBA_CONCAT2(a, b)
#define PRUEBA(nombre)                                                                  \
  static void PRUEBA_CONCAT(prueba_fn_, __LINE__)();                                   \
  static prueba::Caso PRUEBA_CONCAT(prueba_caso_, __LINE__) = {nombre, PRUEBA_CONCAT(prueba_fn_, __LINE__), nullptr}; \
  static prueba::Registro PRUEBA_CONCAT(prueba_reg_, __LINE__)(&PRUEBA_CONCAT(prueba_caso_, __LINE__)); \
  static void PRUEBA_CONCAT(prueba_fn_, __LINE__)()

#define VERIFICAR(cond)                                                              \
  do {                                                                               \
    if (!(cond)) {                                                                   \
      std::printf("    FALLA %s:%d: %s\n", __FILE__, __LINE__, #cond);                \
      prueba::fallas()++;                                                            \
    }                                                                                \
  } while (0)

#define VERIFICAR_CERCA(a, b, tol)                                                   \
  do {                                                                               \
    double _a = (a), _b = (b);                                                       \
    if (std::fabs(_a - _b) > (tol)) {                                                \
      std::printf("    FALLA %s:%d: %s = %g, se esperaba %g (±%g)\n", __FILE__, __LINE__, #a, _a, _b, (double)(tol)); \
      prueba::fallas()++;                                                            \
    }                                                                                \
  } while (0)
