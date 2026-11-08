// Blanco 2: el protocolo BLE (y la nube) contra un sistema entero, iniciado por
// la API C como lo hacen las apps y el bicho virtual (sintetica_iniciar con un
// hal propio). Cubre los marcos de config en bloques, los comandos, la subida y
// la lectura de imágenes, y lo que se dibuja con eso (ticks).
//
// Entrada: [control] y después registros [tipo][...]:
//   control: bit 0-1 perfil (0 1.47B, 1 broche redondo, 2 vista previa de la
//            app 640×344, 3 1.47B), bit 2 sin autenticar.
//   tipo % 8:
//     0 [car][u16 n][n bytes]  escritura BLE cruda (car % 6)
//     1 [car][u16 n][n bytes]  lo mismo por la nube
//     2 [ms/16]                avanza el reloj y dibuja un cuadro (hasta 24)
//     3 [u16 n][n bytes]       sube esa config bien armada (inicio, bloques, fin)
//     4 [espacio][u16 n][n]    sube esa imagen bien armada (con su CRC)
//     5 [b]                    tacto (apretado = b & 1)
//     6 [evento]               un evento
//     7 [b]                    conexión BLE (b & 1)
// Los tipos 3 y 4 existen porque el fuzzer no adivina un CRC-32.
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include <string>
#include <vector>

#include "comun.h"
#include "protocolo.h"
#include "sintetica_c.h"

using namespace sintetica;

namespace {
// El almacenamiento del anfitrión, que se vacía en cada entrada (así un crash
// se reproduce con la entrada sola).
struct Guardado {
  std::string clave;
  std::vector<uint8_t> datos;
};
std::vector<Guardado> g_disco;
uint32_t g_ahora = 0;
int g_ticks = 0;

Guardado* buscar(const char* clave, bool crear) {
  for (auto& g : g_disco)
    if (g.clave == clave) return &g;
  if (!crear || g_disco.size() >= 64) return nullptr;
  g_disco.push_back({clave, {}});
  return &g_disco.back();
}

uint32_t ahoraMs(void*) { return g_ahora; }
uint32_t semilla(void*) { return 0x5157E71Cu; }
int minutoDelDia(void*) { return static_cast<int>((g_ahora / 60000u) % 1440u); }
int bateria(void*) { return 77; }
uint32_t epoch(void*) { return 1762560000u + g_ahora / 1000u; }

int guardar(void*, const char* clave, const uint8_t* d, size_t n) {
  Guardado* g = buscar(clave, true);
  if (!g) return 0;
  g->datos.assign(d, d + n);
  return 1;
}
int leer(void*, const char* clave, uint8_t* d, size_t cap) {
  const Guardado* g = buscar(clave, false);
  if (!g || g->datos.size() > cap) return -1;
  if (!g->datos.empty()) memcpy(d, g->datos.data(), g->datos.size());
  return static_cast<int>(g->datos.size());
}
int escribirTrozo(void*, const char* clave, uint32_t off, const uint8_t* d, size_t n) {
  Guardado* g = buscar(clave, true);
  if (!g || (off != 0 && off != g->datos.size()) || off + n > (1u << 22)) return 0;
  g->datos.resize(off);
  g->datos.insert(g->datos.end(), d, d + n);
  return 1;
}
int leerTrozo(void*, const char* clave, uint32_t off, uint8_t* d, size_t n) {
  const Guardado* g = buscar(clave, false);
  if (!g) return -1;
  if (off >= g->datos.size()) return 0;
  size_t m = g->datos.size() - off;
  if (m > n) m = n;
  memcpy(d, g->datos.data() + off, m);
  return static_cast<int>(m);
}
int borrar(void*, const char* clave) {
  for (size_t i = 0; i < g_disco.size(); i++)
    if (g_disco[i].clave == clave) g_disco.erase(g_disco.begin() + static_cast<long>(i));
  return 1;
}

// Las notificaciones nunca pasan del MTU.
void notificar(void*, int, const uint8_t* d, size_t n) {
  if (n > protocolo::MAX_NOTIFICACION || (n && !d)) abort();
  // Que ASan vea si apunta a algo que no existe.
  volatile uint8_t x = 0;
  for (size_t i = 0; i < n; i++) x = x ^ d[i];
  (void)x;
}

void escribir(int car, const uint8_t* d, size_t n) {
  // Un buffer del tamaño justo: un byte leído de más es un error de ASan.
  uint8_t* b = static_cast<uint8_t*>(malloc(n ? n : 1));
  if (n) memcpy(b, d, n);
  sintetica_ble_escribir(car, b, n);
  free(b);
}

void vincular() {
  // El código va al final del QR: "SINTETICA:V1/<id>/<código>".
  const char* qr = sintetica_qr();
  const size_t l = strlen(qr);
  if (l < 6) return;
  uint8_t m[7] = {protocolo::CmdVincular};
  memcpy(m + 1, qr + l - 6, 6);
  escribir(protocolo::CarComando, m, sizeof m);
}

void subirConfig(const uint8_t* d, size_t n) {
  using namespace protocolo;
  uint8_t ini[9] = {ConfigInicio};
  escribir32(ini + 1, static_cast<uint32_t>(n));
  escribir32(ini + 5, crc32(d, n));
  escribir(CarConfig, ini, sizeof ini);
  uint8_t b[5 + 180];
  for (size_t off = 0; off < n; off += 180) {
    const size_t k = n - off < 180 ? n - off : 180;
    b[0] = ConfigBloque;
    escribir32(b + 1, static_cast<uint32_t>(off));
    memcpy(b + 5, d + off, k);
    escribir(CarConfig, b, 5 + k);
  }
  const uint8_t fin = ConfigFin;
  escribir(CarConfig, &fin, 1);
}

void subirImagen(uint8_t espacio, const uint8_t* d, size_t n) {
  using namespace protocolo;
  uint8_t ini[10] = {ImgInicio, espacio};
  escribir32(ini + 2, static_cast<uint32_t>(n));
  escribir32(ini + 6, crc32(d, n));
  escribir(CarImagen, ini, sizeof ini);
  uint8_t b[5 + 180];
  for (size_t off = 0; off < n; off += 180) {
    const size_t k = n - off < 180 ? n - off : 180;
    b[0] = ImgBloque;
    escribir32(b + 1, static_cast<uint32_t>(off));
    memcpy(b + 5, d + off, k);
    escribir(CarImagen, b, 5 + k);
  }
  const uint8_t fin = ImgFin;
  escribir(CarImagen, &fin, 1);
}
}  // namespace

extern "C" int LLVMFuzzerTestOneInput(const uint8_t* datos, size_t n) {
  Entrada e{datos, n};
  const uint8_t control = e.u8();
  g_disco.clear();
  g_ahora = 1000;
  g_ticks = 0;

  sintetica_hal hal{};
  hal.ahora_ms = ahoraMs;
  hal.semilla = semilla;
  hal.minuto_del_dia = minutoDelDia;
  hal.bateria = bateria;
  hal.guardar = guardar;
  hal.leer = leer;
  hal.notificar = notificar;
  hal.notificar_nube = notificar;
  hal.identificador = "A1B2C3D4E5F6";
  hal.escribir_trozo = escribirTrozo;
  hal.leer_trozo = leerTrozo;
  hal.borrar = borrar;
  hal.epoch = epoch;
  hal.perfil = "virtual";
  switch (control & 3) {
    case 1: sintetica_iniciar_perfil(&hal, 240, 240, 1, 1); break;
    case 2: sintetica_iniciar(&hal, 640, 344); break;
    default: sintetica_iniciar(&hal, 320, 172); break;
  }
  sintetica_ble_conexion(1);
  if (!(control & 4)) vincular();

  while (!e.vacia()) {
    const uint8_t tipo = e.u8() % 8;
    const uint8_t* p = nullptr;
    size_t m = 0;
    switch (tipo) {
      case 0:
      case 1: {
        const int car = e.u8() % 6;
        m = e.tomar(e.u16(), &p);
        if (tipo == 0) {
          escribir(car, p, m);
        } else {
          uint8_t* b = static_cast<uint8_t*>(malloc(m ? m : 1));
          if (m) memcpy(b, p, m);
          sintetica_nube_escribir(car, b, m);
          free(b);
        }
        break;
      }
      case 2:
        g_ahora += 1u + e.u8() * 16u;
        // Dibujar es lo caro: unos pocos cuadros por entrada alcanzan.
        if (g_ticks++ < 24) sintetica_tick();
        break;
      case 3:
        m = e.tomar(e.u16(), &p);
        subirConfig(p, m);
        break;
      case 4: {
        const uint8_t espacio = e.u8() % 9;  // 8 = fuera de rango
        m = e.tomar(e.u16(), &p);
        subirImagen(espacio, p, m);
        break;
      }
      case 5: sintetica_tacto(e.u8() & 1); break;
      case 6: sintetica_evento(e.u8()); break;
      case 7: sintetica_ble_conexion(e.u8() & 1); break;
    }
  }
  // Un último cuadro con lo que quedó, y el estado.
  g_ahora += 17;
  sintetica_tick();
  uint8_t estado[protocolo::MAX_ESTADO];
  if (sintetica_ble_estado(estado, sizeof estado) > sizeof estado) abort();
  (void)sintetica_expresion_visible();
  return 0;
}
