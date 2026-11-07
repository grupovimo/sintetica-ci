// Perfil dev-147b: SintéticaOS en la Waveshare ESP32-S3-LCD-1.47B.
// Este archivo solo traduce el hardware al HAL del núcleo; todo el
// comportamiento vive en nucleo/ (el mismo código que el simulador).
#include <Arduino.h>
#include <LovyanGFX.hpp>
#include <NimBLEDevice.h>
#include <Preferences.h>
#include <Wire.h>

#include "pines.h"
#include "sistema.h"

using namespace sintetica;

namespace {

// --- Pantalla -----------------------------------------------------------------

class Pantalla : public lgfx::LGFX_Device {
  lgfx::Panel_ST7789 panel_;
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
    p.panel_width = 172;
    p.panel_height = 320;
    p.memory_width = 240;
    p.memory_height = 320;
    p.offset_x = pines::LCD_DESFASE_X;
    p.offset_y = 0;
    p.invert = true;
    p.readable = false;
    panel_.config(p);

    auto l = luz_.config();
    l.pin_bl = pines::LCD_BL;
    l.freq = 12000;
    l.pwm_channel = 7;
    luz_.config(l);
    panel_.setLight(&luz_);
    setPanel(&panel_);
  }
};

Pantalla g_lcd;
constexpr int ANCHO = 320, ALTO = 172;  // apaisada
uint16_t* g_pixeles = nullptr;  // en la PSRAM: no entra en la RAM interna junto con BLE
Sistema g_sistema;
Preferences g_prefs;

// --- IMU QMI8658 (registros mínimos para leer el acelerómetro) ------------------

bool g_hayImu = false;

void imuEscribir(uint8_t reg, uint8_t v) {
  Wire.beginTransmission(pines::IMU_DIRECCION);
  Wire.write(reg);
  Wire.write(v);
  Wire.endTransmission();
}

bool imuIniciar() {
  Wire.begin(pines::IMU_SDA, pines::IMU_SCL, 400000);
  Wire.beginTransmission(pines::IMU_DIRECCION);
  Wire.write(0x00);  // WHO_AM_I
  if (Wire.endTransmission(false) != 0 || Wire.requestFrom(pines::IMU_DIRECCION, 1) != 1) return false;
  if (Wire.read() != 0x05) return false;
  imuEscribir(0x02, 0x40);  // CTRL1: autoincremento de dirección
  imuEscribir(0x03, 0x16);  // CTRL2: acelerómetro ±4 g, ~125 Hz
  imuEscribir(0x08, 0x01);  // CTRL7: prender el acelerómetro
  return true;
}

bool imuLeer(float* ax, float* ay, float* az) {
  Wire.beginTransmission(pines::IMU_DIRECCION);
  Wire.write(0x35);  // AX_L
  if (Wire.endTransmission(false) != 0 || Wire.requestFrom(pines::IMU_DIRECCION, 6) != 6) return false;
  int16_t v[3];
  for (int i = 0; i < 3; i++) {
    uint8_t lo = Wire.read(), hi = Wire.read();
    v[i] = static_cast<int16_t>(lo | (hi << 8));
  }
  const float k = 4.0f / 32768.0f;
  *ax = v[0] * k, *ay = v[1] * k, *az = v[2] * k;
  return true;
}

// --- BLE --------------------------------------------------------------------------

struct Escritura {
  uint8_t car;
  uint16_t n;
  uint8_t datos[512];
};
QueueHandle_t g_colaBle = nullptr;
NimBLECharacteristic* g_car[5] = {};

class AlEscribir : public NimBLECharacteristicCallbacks {
  uint8_t car_;

 public:
  explicit AlEscribir(uint8_t car) : car_(car) {}
  void onWrite(NimBLECharacteristic* c) override {
    // Los callbacks de BLE corren en otra tarea: se encola y el loop lo procesa.
    static Escritura e;
    const std::string& v = c->getValue();
    if (v.size() == 0 || v.size() > sizeof e.datos) return;
    e.car = car_;
    e.n = static_cast<uint16_t>(v.size());
    memcpy(e.datos, v.data(), v.size());
    xQueueSend(g_colaBle, &e, 0);
  }
};

class AlLeerEstado : public NimBLECharacteristicCallbacks {
  void onRead(NimBLECharacteristic* c) override {
    uint8_t b[64];
    size_t n = g_sistema.estadoBle(b, sizeof b);
    c->setValue(b, n);
  }
};

void bleIniciar(const char* nombre) {
  g_colaBle = xQueueCreate(8, sizeof(Escritura));
  NimBLEDevice::init(nombre);
  NimBLEDevice::setMTU(517);
  NimBLEServer* servidor = NimBLEDevice::createServer();
  NimBLEService* s = servidor->createService(protocolo::UUID_SERVICIO);
  using P = NIMBLE_PROPERTY;
  g_car[protocolo::CarConfig] = s->createCharacteristic(protocolo::UUID_CONFIG, P::WRITE | P::WRITE_NR | P::NOTIFY);
  g_car[protocolo::CarComando] = s->createCharacteristic(protocolo::UUID_COMANDO, P::WRITE | P::NOTIFY);
  g_car[protocolo::CarEstado] = s->createCharacteristic(protocolo::UUID_ESTADO, P::READ | P::NOTIFY);
  g_car[protocolo::CarOta] = s->createCharacteristic(protocolo::UUID_OTA, P::WRITE | P::NOTIFY);
  static AlEscribir config(protocolo::CarConfig), cmd(protocolo::CarComando), ota(protocolo::CarOta);
  static AlLeerEstado estado;
  g_car[protocolo::CarConfig]->setCallbacks(&config);
  g_car[protocolo::CarComando]->setCallbacks(&cmd);
  g_car[protocolo::CarOta]->setCallbacks(&ota);
  g_car[protocolo::CarEstado]->setCallbacks(&estado);
  s->start();
  NimBLEAdvertising* a = NimBLEDevice::getAdvertising();
  a->addServiceUUID(protocolo::UUID_SERVICIO);
  a->setScanResponse(true);
  a->start();
  servidor->advertiseOnDisconnect(true);
}

// --- HAL --------------------------------------------------------------------------

Hal armarHal() {
  Hal h;
  h.ahoraMs = [] { return static_cast<uint32_t>(millis()); };
  h.semilla = [] { return static_cast<uint32_t>(esp_random()); };
  h.presentar = [](const uint16_t* px, int w, int alto) {
    g_lcd.pushImage(0, 0, w, alto, reinterpret_cast<const lgfx::rgb565_t*>(px));
  };
  h.minutoDelDia = [] { return -1; };  // sin hora hasta tener NTP o la app
  h.bateria = [] {
    const uint32_t mv = analogReadMilliVolts(pines::BATERIA_ADC) * 3;
    if (mv < 2500) return -1;  // sin batería conectada
    return static_cast<int>(constrain((static_cast<int>(mv) - 3300) * 100 / (4200 - 3300), 0, 100));
  };
  h.guardar = [](const char* clave, const uint8_t* d, size_t n) { return g_prefs.putBytes(clave, d, n) == n; };
  h.leer = [](const char* clave, uint8_t* d, size_t cap) -> int {
    size_t n = g_prefs.getBytesLength(clave);
    if (n == 0 || n > cap) return -1;
    return static_cast<int>(g_prefs.getBytes(clave, d, cap));
  };
  h.notificar = [](uint8_t car, const uint8_t* d, size_t n) {
    if (car < 1 || car > 4 || !g_car[car]) return;
    g_car[car]->setValue(d, n);
    g_car[car]->notify();
  };
  h.sonido = [](const char* s) { Serial.printf("[sonido] %s (sin parlante en este perfil)\n", s); };
  h.led = [](uint32_t rgb) { neopixelWrite(pines::LED_RGB, (rgb >> 16) & 0xFF, (rgb >> 8) & 0xFF, rgb & 0xFF); };
  h.registrar = [](const char* m) { Serial.printf("[sintetica] %s\n", m); };
  return h;
}

uint32_t g_ultimoImu = 0, g_ultimoCuadro = 0;

}  // namespace

void setup() {
  Serial.begin(115200);
  pinMode(pines::BOTON_BOOT, INPUT_PULLUP);
  if (pines::TTP223 >= 0) pinMode(pines::TTP223, INPUT);
  analogReadResolution(12);
  g_prefs.begin("sintetica", false);

  g_lcd.init();
  g_lcd.setRotation(1);  // apaisada: 320 × 172
  g_lcd.setBrightness(110);  // ~43%: Waveshare recomienda no pasar del 50%
  g_lcd.fillScreen(0);

  g_pixeles = static_cast<uint16_t*>(heap_caps_malloc(ANCHO * ALTO * 2, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT));
  if (!g_pixeles) g_pixeles = static_cast<uint16_t*>(heap_caps_malloc(ANCHO * ALTO * 2, MALLOC_CAP_8BIT));

  g_hayImu = imuIniciar();
  g_sistema.iniciar(armarHal(), g_pixeles, ANCHO, ALTO);
  if (!g_hayImu) Serial.println("[sintetica] no encontré el QMI8658: sin sacudidas");
  bleIniciar(g_sistema.nombre());
}

void loop() {
  static Escritura e;
  while (g_colaBle && xQueueReceive(g_colaBle, &e, 0) == pdTRUE) g_sistema.bleEscribir(e.car, e.datos, e.n);

  const bool apretado = digitalRead(pines::BOTON_BOOT) == LOW || (pines::TTP223 >= 0 && digitalRead(pines::TTP223) == HIGH);
  g_sistema.tacto(apretado);

  const uint32_t ahora = millis();
  if (g_hayImu && ahora - g_ultimoImu >= 20) {
    g_ultimoImu = ahora;
    float ax, ay, az;
    if (imuLeer(&ax, &ay, &az)) g_sistema.imu(ax, ay, az);
  }
  if (ahora - g_ultimoCuadro >= 33) {  // ~30 cuadros por segundo
    g_ultimoCuadro = ahora;
    g_sistema.tick();
  }
  delay(1);
}
