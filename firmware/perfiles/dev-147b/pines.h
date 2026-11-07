// Perfil dev-147b: Waveshare ESP32-S3-LCD-1.47B.
//
// Pantalla: wiki de Waveshare (docs.waveshare.com/ESP32-S3-LCD-1.47B).
// IMU y batería: tomados de un proyecto de la comunidad para esta placa
// (github.com/f0xx/esp32-s3-good-vibes, docs/zephyr-hardware.md).
// TODO verificar al grabar: IMU, batería, LED, desfase de columna y rotación.
#pragma once

namespace pines {
// ST7789, 172 × 320, SPI.
constexpr int LCD_MOSI = 45;
constexpr int LCD_SCLK = 40;
constexpr int LCD_CS = 42;
constexpr int LCD_DC = 41;
constexpr int LCD_RST = 39;
constexpr int LCD_BL = 46;
constexpr int LCD_DESFASE_X = 34;  // el ST7789 tiene 240 columnas; el panel usa 172 centradas

// QMI8658 por I2C.
constexpr int IMU_SDA = 48;
constexpr int IMU_SCL = 47;
constexpr int IMU_DIRECCION = 0x6B;

// Batería: ADC1 canal 0, divisor 200k/100k (×3).
constexpr int BATERIA_ADC = 1;

// LED RGB de la placa (tipo sin confirmar: se asume direccionable, 1 LED).
constexpr int LED_RGB = 38;

// BOOT hace de sensor táctil hasta que llegue el TTP223.
constexpr int BOTON_BOOT = 0;
constexpr int TTP223 = -1;  // -1 = no conectado todavía
}  // namespace pines
