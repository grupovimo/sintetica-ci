// Las funciones del HAL de la API en C, que llaman a Go (nucleo.go). cgo no
// deja definir funciones de C en un archivo de Go con //export, así que van acá.
#include <stdlib.h>

extern "C" {
#include "_cgo_export.h"
}

namespace {
uint32_t ahora(void*) { return goAhoraMs(); }
uint32_t semilla(void*) { return goSemilla(); }
int minuto(void*) { return goMinuto(); }
int bateria(void*) { return goBateria(); }
int guardar(void*, const char* c, const uint8_t* d, size_t n) {
  return goGuardar(const_cast<char*>(c), const_cast<uint8_t*>(d), n);
}
int leer(void*, const char* c, uint8_t* d, size_t cap) { return goLeer(const_cast<char*>(c), d, cap); }
int escribirTrozo(void*, const char* c, uint32_t off, const uint8_t* d, size_t n) {
  return goEscribirTrozo(const_cast<char*>(c), off, const_cast<uint8_t*>(d), n);
}
int leerTrozo(void*, const char* c, uint32_t off, uint8_t* d, size_t n) { return goLeerTrozo(const_cast<char*>(c), off, d, n); }
int borrar(void*, const char* c) { return goBorrar(const_cast<char*>(c)); }
void notificar(void*, int car, const uint8_t* d, size_t n) { goNotificar(car, const_cast<uint8_t*>(d), n); }
void notificarNube(void*, int car, const uint8_t* d, size_t n) { goNotificarNube(car, const_cast<uint8_t*>(d), n); }
void redCambio(void*) { goRedCambio(); }
int otaDescargar(void*, const sintetica_manifiesto* m, const char* url) {
  return goOtaDescargar(const_cast<sintetica_manifiesto*>(m), const_cast<char*>(url));
}
void otaCancelar(void*) { goOtaCancelar(); }
void registrar(void*, const char* m) { goRegistrar(const_cast<char*>(m)); }
}  // namespace

extern "C" void sintetica_go_iniciar(const char* id, const char* perfil, const char* version, int ancho, int alto,
                                     int redonda, int broche) {
  sintetica_hal h = {};
  h.ahora_ms = ahora;
  h.semilla = semilla;
  h.minuto_del_dia = minuto;
  h.bateria = bateria;
  h.guardar = guardar;
  h.leer = leer;
  h.notificar = notificar;
  h.registrar = registrar;
  h.identificador = id;
  h.notificar_nube = notificarNube;
  h.red_cambio = redCambio;
  h.ota_descargar = otaDescargar;
  h.ota_cancelar = otaCancelar;
  h.perfil = perfil;
  h.version = version;
  h.escribir_trozo = escribirTrozo;
  h.leer_trozo = leerTrozo;
  h.borrar = borrar;
  sintetica_iniciar_perfil(&h, ancho, alto, redonda, broche);
}
