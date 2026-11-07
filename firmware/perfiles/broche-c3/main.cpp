// Perfil broche-c3: SintéticaOS en el broche (ESP32-C3 SuperMini + GC9A01).
// Sin PSRAM: la pantalla se dibuja de a franjas de 40 filas (19 KB).
// Pulsador: toque = lo siguiente de la lista, mantener 2 s = dormir (sueño
// profundo), un toque lo despierta, mantener 8 s = QR.
#include <Arduino.h>
#include <LovyanGFX.hpp>
#include <esp_sleep.h>

#include "comun.h"
#include "ota.h"
#include "pines.h"
#include "sistema.h"

using namespace sintetica;

namespace {

class Pantalla : public lgfx::LGFX_Device {
  lgfx::Panel_GC9A01 panel_;
  lgfx::Bus_SPI bus_;
  lgfx::Light_PWM luz_;

 public:
  Pantalla() {
    auto b = bus_.config();
    b.spi_host = SPI2_HOST;
    b.spi_mode = 0;
    b.freq_write = 40000000;
    b.pin_sclk = pines::LCD_SCLK;
    b.pin_mosi = pines::LCD_MOSI;
    b.pin_miso = -1;
    b.pin_dc = pines::LCD_DC;
    bus_.config(b);
    panel_.setBus(&bus_);
    auto p = panel_.config();
    p.pin_cs = pines::LCD_CS;
    p.pin_rst = pines::LCD_RST;
    p.panel_width = 240;
    p.panel_height = 240;
    p.invert = true;
    p.readable = false;
    panel_.config(p);
    auto l = luz_.config();
    l.pin_bl = pines::LCD_BL;
    l.freq = 12000;
    l.pwm_channel = 0;
    luz_.config(l);
    panel_.setLight(&luz_);
    setPanel(&panel_);
  }
};

Pantalla g_lcd;
constexpr int LADO = 240, FILAS = 40;
uint16_t g_franja[LADO * FILAS];
Sistema g_sistema;
bool g_ignorarHastaSoltar = false;  // el toque que lo despertó no cuenta como "siguiente"
uint32_t g_ultimoCuadro = 0;

Hal armarHal() {
  Hal h;
  comun::completarHal(h);
  h.ahoraMs = [] { return static_cast<uint32_t>(millis()); };
  h.presentarFranja = [](const uint16_t* px, int ancho, int y0, int filas) {
    g_lcd.pushImage(0, y0, ancho, filas, reinterpret_cast<const lgfx::rgb565_t*>(px));
  };
  h.bateria = [] {
    const uint32_t mv = analogReadMilliVolts(pines::BATERIA_ADC) * pines::BATERIA_DIVISOR;
    if (mv < 2500) return -1;  // sin batería o sin el divisor
    return static_cast<int>(constrain((static_cast<int>(mv) - 3300) * 100 / (4200 - 3300), 0, 100));
  };
  comun::otaCompletarHal(h, false);  // sin WiFi: la OTA llega por BLE, desde la app
  return h;
}

void dormir() {
  // Al despertar arranca de nuevo: si el firmware estaba a prueba, el
  // bootloader volvería al anterior. Si llegó a dormirse, anda.
  comun::otaConfirmar();
  Serial.println("[sintetica] a dormir; un toque lo despierta");
  Serial.flush();
  g_lcd.setBrightness(0);
  g_lcd.sleep();  // la pantalla también se duerme (unos µA)
  comun::bleApagar();
  // Despertar con el pulsador (va a GND: nivel bajo) y, si se durmió de
  // noche, solo a la hora de despertarse (el reloj sigue andando).
  esp_deep_sleep_enable_gpio_wakeup(1ULL << pines::BOTON, ESP_GPIO_WAKEUP_GPIO_LOW);
  const int segundos = g_sistema.segundosHastaDespertar();
  if (segundos > 0 && g_sistema.motor().dormido())
    esp_sleep_enable_timer_wakeup(static_cast<uint64_t>(segundos) * 1000000ULL);
  esp_deep_sleep_start();
}

}  // namespace

void setup() {
  Serial.begin(115200);
  pinMode(pines::BOTON, INPUT_PULLUP);
  analogReadResolution(12);
  g_ignorarHastaSoltar = esp_sleep_get_wakeup_cause() == ESP_SLEEP_WAKEUP_GPIO;
  comun::almacenIniciar();

  g_lcd.init();
  g_lcd.setBrightness(110);
  g_lcd.fillScreen(0);

  Perfil p;
  p.ancho = LADO, p.alto = LADO, p.filasFranja = FILAS, p.redonda = true, p.boton = Perfil::Boton::Broche;
  p.nombre = "broche-c3";
  p.version = SINTETICA_VERSION;
  g_sistema.iniciar(armarHal(), g_franja, p);
  comun::bleIniciar(g_sistema, g_sistema.nombre());
  comun::tallerSaludo(g_sistema);
}

void loop() {
  comun::bleAtender(g_sistema);
  comun::otaAtender(g_sistema, true);
  comun::tallerAtender(g_sistema);

  const bool apretado = digitalRead(pines::BOTON) == LOW;
  if (g_ignorarHastaSoltar) {
    if (!apretado) g_ignorarHastaSoltar = false;
  } else {
    g_sistema.tacto(apretado);
  }

  const uint32_t ahora = millis();
  // Los modos de energía: 30 cuadros por segundo despierto, 5 dormido.
  if (ahora - g_ultimoCuadro >= 1000u / static_cast<uint32_t>(g_sistema.fpsSugerido())) {
    g_ultimoCuadro = ahora;
    g_sistema.tick();
    if (g_sistema.quiereDormir()) dormir();
  }
  delay(1);
}
