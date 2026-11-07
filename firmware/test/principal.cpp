#include <cstring>

#include "prueba.h"

// Uso: pruebas [parte del nombre] (sin argumento, corre todas).
int main(int argc, char** argv) {
  int n = 0;
  for (prueba::Caso* c = prueba::lista(); c; c = c->siguiente) {
    if (argc > 1 && !std::strstr(c->nombre, argv[1])) continue;
    int antes = prueba::fallas();
    c->fn();
    std::printf("%s %s\n", prueba::fallas() == antes ? "ok  " : "MAL ", c->nombre);
    std::fflush(stdout);
    n++;
  }
  std::printf("\n%d pruebas, %d fallas\n", n, prueba::fallas());
  return prueba::fallas() ? 1 : 0;
}
