#include "comun.h"

#include <LittleFS.h>
#include <NimBLEDevice.h>
#include <Preferences.h>
#include <esp_mac.h>

using namespace sintetica;

namespace comun {

namespace {
Preferences g_prefs;
bool g_hayLittleFs = false;
char g_id[13];

// car 0 = cambio de conexión (datos[0]: 1 conectado, 0 desconectado).
struct Escritura {
  uint8_t car;
  uint16_t n;
  uint8_t datos[512];
};
QueueHandle_t g_cola = nullptr;
NimBLECharacteristic* g_car[6] = {};
Sistema* g_sistema = nullptr;

void encolar(uint8_t car, const uint8_t* d, size_t n) {
  static Escritura e;
  if (n > sizeof e.datos) return;
  e.car = car;
  e.n = static_cast<uint16_t>(n);
  memcpy(e.datos, d, n);
  xQueueSend(g_cola, &e, pdMS_TO_TICKS(50));
}

class AlEscribir : public NimBLECharacteristicCallbacks {
  uint8_t car_;

 public:
  explicit AlEscribir(uint8_t car) : car_(car) {}
  void onWrite(NimBLECharacteristic* c) override {
    const std::string& v = c->getValue();
    if (!v.empty()) encolar(car_, reinterpret_cast<const uint8_t*>(v.data()), v.size());
  }
};

class AlLeerEstado : public NimBLECharacteristicCallbacks {
  void onRead(NimBLECharacteristic* c) override {
    uint8_t b[96];
    c->setValue(b, g_sistema ? g_sistema->estadoBle(b, sizeof b) : 0);
  }
};

class AlConectar : public NimBLEServerCallbacks {
  void onConnect(NimBLEServer*) override {
    uint8_t uno = 1;
    encolar(0, &uno, 1);
  }
  void onDisconnect(NimBLEServer*) override {
    uint8_t cero = 0;
    encolar(0, &cero, 1);
  }
};

String ruta(const char* clave) { return String("/") + clave; }
}  // namespace

const char* identificador() {
  if (!g_id[0]) {
    uint8_t mac[6];
    esp_read_mac(mac, ESP_MAC_BT);
    snprintf(g_id, sizeof g_id, "%02X%02X%02X%02X%02X%02X", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);
  }
  return g_id;
}

void almacenIniciar() {
  g_prefs.begin("sintetica", false);
  // La partición "spiffs" de la tabla de particiones, con LittleFS.
  g_hayLittleFs = LittleFS.begin(true);
  if (!g_hayLittleFs) Serial.println("[sintetica] no pude montar LittleFS: sin imágenes");
}

bool guardar(const char* clave, const uint8_t* d, size_t n) { return g_prefs.putBytes(clave, d, n) == n; }

int leer(const char* clave, uint8_t* d, size_t cap) {
  size_t n = g_prefs.getBytesLength(clave);
  if (n == 0 || n > cap) return -1;
  return static_cast<int>(g_prefs.getBytes(clave, d, cap));
}

bool escribirTrozo(const char* clave, uint32_t off, const uint8_t* d, size_t n) {
  if (!g_hayLittleFs) return false;
  File f = LittleFS.open(ruta(clave), off == 0 ? "w" : "a");
  if (!f) return false;
  if (f.size() != off) {
    f.close();
    return false;
  }
  const bool ok = f.write(d, n) == n;  // si se llena, write escribe menos
  f.close();
  return ok;
}

int leerTrozo(const char* clave, uint32_t off, uint8_t* d, size_t n) {
  if (!g_hayLittleFs) return -1;
  File f = LittleFS.open(ruta(clave), "r");
  if (!f) return -1;
  int r = 0;
  if (off < f.size() && f.seek(off)) r = static_cast<int>(f.read(d, n));
  f.close();
  return r;
}

bool borrar(const char* clave) { return !g_hayLittleFs || !LittleFS.exists(ruta(clave)) || LittleFS.remove(ruta(clave)); }

void completarHal(Hal& h) {
  h.identificador = identificador;
  h.semilla = [] { return static_cast<uint32_t>(esp_random()); };
  h.guardar = guardar;
  h.leer = leer;
  h.escribirTrozo = escribirTrozo;
  h.leerTrozo = leerTrozo;
  h.borrar = borrar;
  h.notificar = bleNotificar;
  h.registrar = [](const char* m) { Serial.printf("[sintetica] %s\n", m); };
}

void bleIniciar(Sistema& s, const char* nombre) {
  g_sistema = &s;
  g_cola = xQueueCreate(8, sizeof(Escritura));
  NimBLEDevice::init(nombre);
  NimBLEDevice::setMTU(517);
  NimBLEServer* servidor = NimBLEDevice::createServer();
  static AlConectar conexiones;
  servidor->setCallbacks(&conexiones);
  NimBLEService* srv = servidor->createService(protocolo::UUID_SERVICIO);
  using P = NIMBLE_PROPERTY;
  g_car[protocolo::CarConfig] = srv->createCharacteristic(protocolo::UUID_CONFIG, P::WRITE | P::WRITE_NR | P::NOTIFY);
  g_car[protocolo::CarComando] = srv->createCharacteristic(protocolo::UUID_COMANDO, P::WRITE | P::NOTIFY);
  g_car[protocolo::CarEstado] = srv->createCharacteristic(protocolo::UUID_ESTADO, P::READ | P::NOTIFY);
  g_car[protocolo::CarOta] = srv->createCharacteristic(protocolo::UUID_OTA, P::WRITE | P::NOTIFY);
  g_car[protocolo::CarImagen] = srv->createCharacteristic(protocolo::UUID_IMAGEN, P::WRITE | P::WRITE_NR | P::NOTIFY);
  static AlEscribir config(protocolo::CarConfig), cmd(protocolo::CarComando), ota(protocolo::CarOta),
      imagen(protocolo::CarImagen);
  static AlLeerEstado estado;
  g_car[protocolo::CarConfig]->setCallbacks(&config);
  g_car[protocolo::CarComando]->setCallbacks(&cmd);
  g_car[protocolo::CarOta]->setCallbacks(&ota);
  g_car[protocolo::CarImagen]->setCallbacks(&imagen);
  g_car[protocolo::CarEstado]->setCallbacks(&estado);
  srv->start();
  NimBLEAdvertising* a = NimBLEDevice::getAdvertising();
  a->addServiceUUID(protocolo::UUID_SERVICIO);
  // El id del bicho en los datos de fabricante (0xFFFF = id de pruebas del Bluetooth SIG).
  uint8_t mac[6];
  esp_read_mac(mac, ESP_MAC_BT);
  std::string datos("\xFF\xFF", 2);
  datos.append(reinterpret_cast<const char*>(mac), 6);
  a->setManufacturerData(datos);
  a->setScanResponse(true);
  a->start();
  servidor->advertiseOnDisconnect(true);
}

void bleAtender(Sistema& s) {
  static Escritura e;
  while (g_cola && xQueueReceive(g_cola, &e, 0) == pdTRUE) {
    if (e.car == 0) s.bleConexion(e.datos[0] != 0);
    else s.bleEscribir(e.car, e.datos, e.n);
  }
}

void bleNotificar(uint8_t car, const uint8_t* d, size_t n) {
  if (car < 1 || car > 5 || !g_car[car]) return;
  g_car[car]->setValue(d, n);
  g_car[car]->notify();
}

void bleApagar() { NimBLEDevice::deinit(true); }

void tallerAtender(Sistema& s) {
  static char linea[48];
  static size_t n = 0;
  while (Serial.available()) {
    char c = static_cast<char>(Serial.read());
    if (c != '\n' && c != '\r') {
      if (n < sizeof linea - 1) linea[n++] = c;
      continue;
    }
    linea[n] = 0;
    if (n > 6 && strncmp(linea, "serie ", 6) == 0 && n - 6 <= 12) {
      s.fijarSerie(linea + 6);
      Serial.printf("[taller] unidad #%s grabada\n", s.serie());
    } else if (n > 0) {
      Serial.println("[taller] comandos: serie <número, hasta 12 caracteres>");
    }
    n = 0;
  }
}

void tallerSaludo(Sistema& s) {
  Serial.printf("[taller] id %s · unidad #%s · %s · %s\n", s.identificador(), s.serie()[0] ? s.serie() : "(sin grabar)",
                s.emparejando() ? "esperando vínculo" : "ya vinculado", g_hayLittleFs ? "con LittleFS" : "sin LittleFS");
}

}  // namespace comun
