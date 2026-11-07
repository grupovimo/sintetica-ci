#include "prueba.h"

int main() {
  int n = 0;
  for (prueba::Caso* c = prueba::lista(); c; c = c->siguiente) {
    int antes = prueba::fallas();
    c->fn();
    std::printf("%s %s\n", prueba::fallas() == antes ? "ok  " : "MAL ", c->nombre);
    n++;
  }
  std::printf("\n%d pruebas, %d fallas\n", n, prueba::fallas());
  return prueba::fallas() ? 1 : 0;
}
