// La frontera con el hardware. Cada perfil (dev-147b, el simulador, los tests)
// llena esta tabla; el núcleo no sabe nada de pines, pantallas ni radios.
#pragma once

#include <stddef.h>
#include <stdint.h>

namespace sintetica {

struct Hal {
  // Milisegundos desde que arrancó (puede dar la vuelta; el núcleo lo tolera).
  uint32_t (*ahoraMs)() = nullptr;
  // Identificador único del bicho, 12 caracteres hexadecimales en mayúscula
  // (en la placa, la MAC). nullptr = "000000000000".
  const char* (*identificador)() = nullptr;
  // Una semilla para el azar (ruido del ADC, RNG de hardware, etc.).
  uint32_t (*semilla)() = nullptr;
  // Muestra el framebuffer RGB565 completo.
  void (*presentar)(const uint16_t* pixeles, int ancho, int alto) = nullptr;
  // Minuto del día (0-1439) o -1 si no se sabe la hora.
  int (*minutoDelDia)() = nullptr;
  // Batería 0-100, o -1 si no hay o no se sabe.
  int (*bateria)() = nullptr;
  // Almacenamiento persistente por clave (NVS, localStorage, memoria).
  bool (*guardar)(const char* clave, const uint8_t* datos, size_t n) = nullptr;
  int (*leer)(const char* clave, uint8_t* datos, size_t cap) = nullptr;  // -1 si no está
  // Notificación BLE en una característica (protocolo::Caracteristica).
  void (*notificar)(uint8_t caracteristica, const uint8_t* datos, size_t n) = nullptr;
  // Salidas opcionales.
  void (*sonido)(const char* nombre) = nullptr;
  void (*led)(uint32_t rgb) = nullptr;
  void (*registrar)(const char* mensaje) = nullptr;
};

}  // namespace sintetica
