// Corre un blanco sin libFuzzer, como test normal: le pasa cada archivo de las
// carpetas (o archivos) que recibe. Así las entradas de regresiones/ y el
// corpus semilla se prueban con cualquier compilador, con ASan y UBSan.
#include <dirent.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>

#include <string>
#include <vector>

extern "C" int LLVMFuzzerTestOneInput(const uint8_t* d, size_t n);

namespace {
int correrArchivo(const std::string& ruta) {
  FILE* f = fopen(ruta.c_str(), "rb");
  if (!f) {
    fprintf(stderr, "no se puede abrir %s\n", ruta.c_str());
    return 1;
  }
  std::vector<uint8_t> b;
  uint8_t tmp[65536];
  size_t k;
  while ((k = fread(tmp, 1, sizeof tmp, f)) > 0) b.insert(b.end(), tmp, tmp + k);
  fclose(f);
  // Copia del tamaño justo, como libFuzzer.
  uint8_t* d = static_cast<uint8_t*>(malloc(b.size() ? b.size() : 1));
  if (!b.empty()) memcpy(d, b.data(), b.size());
  LLVMFuzzerTestOneInput(d, b.size());
  free(d);
  return 0;
}
}  // namespace

int main(int argc, char** argv) {
  int archivos = 0, fallas = 0;
  for (int i = 1; i < argc; i++) {
    struct stat st;
    if (stat(argv[i], &st) != 0) continue;  // una carpeta que todavía no existe
    if (!S_ISDIR(st.st_mode)) {
      fallas += correrArchivo(argv[i]), archivos++;
      continue;
    }
    DIR* dir = opendir(argv[i]);
    if (!dir) continue;
    std::vector<std::string> nombres;
    while (dirent* e = readdir(dir))
      if (e->d_name[0] != '.') nombres.push_back(e->d_name);
    closedir(dir);
    for (const auto& n : nombres) {
      const std::string ruta = std::string(argv[i]) + "/" + n;
      if (stat(ruta.c_str(), &st) == 0 && S_ISREG(st.st_mode)) fallas += correrArchivo(ruta), archivos++;
    }
  }
  printf("%d entradas, %d sin poder leer\n", archivos, fallas);
  return fallas ? 1 : 0;
}
