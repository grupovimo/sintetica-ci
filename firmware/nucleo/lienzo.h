// El lienzo: un framebuffer RGB565 y las tres operaciones con antialias que
// alcanzan para dibujar todas las caras (píldora, polígono y trazo redondo),
// más el corrimiento de franjas del glitch.
#pragma once

#include "formato.h"
#include "mate.h"

namespace sintetica {

constexpr int MAX_ANCHO = 320;
constexpr int MAX_ALTO = 320;

inline uint16_t a565(uint32_t rgb) {
  return static_cast<uint16_t>(((rgb >> 8) & 0xF800) | ((rgb >> 5) & 0x07E0) | ((rgb >> 3) & 0x001F));
}
inline uint32_t de565(uint16_t c) {
  uint32_t r = (c >> 11) & 0x1F, g = (c >> 5) & 0x3F, b = c & 0x1F;
  return ((r << 3 | r >> 2) << 16) | ((g << 2 | g >> 4) << 8) | (b << 3 | b >> 2);
}

class Lienzo {
 public:
  // `pixeles` debe tener ancho*alto elementos; lo da quien llama (en la placa,
  // memoria con DMA; en el simulador, memoria del módulo WebAssembly).
  void iniciar(uint16_t* pixeles, int ancho, int alto);

  int ancho() const { return ancho_; }
  int alto() const { return alto_; }
  uint16_t* pixeles() { return px_; }
  const uint16_t* pixeles() const { return px_; }

  void limpiar(uint32_t rgb);

  // Rectángulo redondeado relleno, en píxeles (centro, tamaño y radio).
  void pildora(float cx, float cy, float w, float h, float r, uint32_t rgb);
  // Solo el contorno, con el trazo hacia adentro.
  void pildoraContorno(float cx, float cy, float w, float h, float r, float trazo, uint32_t rgb);
  // Polígono relleno (par-impar).
  void poligono(const Punto* p, int n, uint32_t rgb);
  // Línea poligonal con puntas y uniones redondas.
  void trazo(const Punto* p, int n, float grosor, uint32_t rgb);
  // Mezcla un píxel con una cobertura de 0 a 1 (lo usan el texto y el QR).
  void cubrir(int x, int y, float cobertura, uint32_t rgb) { mezclarPixel(x, y, cobertura, rgb); }
  // Rectángulo lleno alineado a píxeles, sin antialias.
  void rectangulo(int x, int y, int w, int h, uint32_t rgb);
  // Corre en horizontal las filas [y0, y1) dx píxeles, rellenando con `fondo`.
  void correrFilas(int y0, int y1, int dx, uint32_t fondo);

 private:
  void mezclarPixel(int x, int y, float cobertura, uint32_t rgb);

  uint16_t* px_ = nullptr;
  int ancho_ = 0, alto_ = 0;
};

}  // namespace sintetica
