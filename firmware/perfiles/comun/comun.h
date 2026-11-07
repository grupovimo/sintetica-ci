// Lo que comparten todos los perfiles ESP32 con Arduino: BLE (NimBLE),
// almacenamiento (Preferences para lo chico, LittleFS para las imágenes), el
// identificador por MAC y los comandos de taller por USB.
#pragma once

#include <Arduino.h>

#include "sistema.h"

namespace comun {

const char* identificador();  // la MAC de Bluetooth, 12 caracteres hexadecimales

// Almacenamiento: llamar una vez antes de iniciar el sistema.
void almacenIniciar();
bool guardar(const char* clave, const uint8_t* datos, size_t n);
int leer(const char* clave, uint8_t* datos, size_t cap);
bool escribirTrozo(const char* clave, uint32_t desplazamiento, const uint8_t* datos, size_t n);
int leerTrozo(const char* clave, uint32_t desplazamiento, uint8_t* datos, size_t n);
bool borrar(const char* clave);

// Completa en el HAL lo que sale de acá (almacenamiento, BLE, registro, id).
void completarHal(sintetica::Hal& h);

// BLE: el servicio de docs/protocolo-ble.md. Las escrituras llegan en otra
// tarea y se encolan; bleAtender las pasa al sistema desde el loop.
void bleIniciar(sintetica::Sistema& s, const char* nombre);
void bleAtender(sintetica::Sistema& s);
void bleNotificar(uint8_t caracteristica, const uint8_t* datos, size_t n);
void bleApagar();

// "serie 001" por USB graba el número de unidad.
void tallerAtender(sintetica::Sistema& s);
void tallerSaludo(sintetica::Sistema& s);

}  // namespace comun
