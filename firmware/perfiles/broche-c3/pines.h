// Perfil broche-c3: ESP32-C3 SuperMini + pantalla redonda GC9A01 240 × 240.
// Cableado: pestaña "Hardware y armado" del doc (y firmware/perfiles/broche-c3/README.md).
// TODO verificar al armar: rotación, inversión de colores y frecuencia del SPI.
#pragma once

namespace pines {
// GC9A01 por SPI. Se evitan los pines de arranque del C3 (2, 8 y 9).
constexpr int LCD_SCLK = 4;
constexpr int LCD_MOSI = 6;
constexpr int LCD_DC = 10;
constexpr int LCD_CS = 7;
constexpr int LCD_RST = 3;
constexpr int LCD_BL = 1;  // luz de fondo con PWM

// Pulsador a GND con pull-up interno. GPIO5: el C3 solo se despierta del
// sueño profundo con los pines 0 a 5.
constexpr int BOTON = 5;

// Batería: divisor de 2 × 100 kΩ de B+ a GND, punto medio al GPIO0 (ADC1_CH0).
constexpr int BATERIA_ADC = 0;
constexpr int BATERIA_DIVISOR = 2;
}  // namespace pines
