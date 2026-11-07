#ifdef SINTETICA_CON_WIFI

#include "nube.h"

#include <Arduino.h>
#include <WiFi.h>
#include <mqtt_client.h>
#include <time.h>

// Los certificados raíz que trae ESP-IDF (para https:// y mqtts://). El
// esp_crt_bundle.h de Arduino tapa al de IDF, así que se declara acá.
extern "C" esp_err_t esp_crt_bundle_attach(void* conf);

using namespace sintetica;

namespace comun {

namespace {

constexpr size_t MAX_MENSAJE = 2048;  // la API manda marcos de hasta ~1 KB

struct Mensaje {
  uint16_t n;
  uint8_t datos[MAX_MENSAJE];
};

QueueHandle_t g_entrada = nullptr;
esp_mqtt_client_handle_t g_cliente = nullptr;
volatile bool g_wifi = false, g_conectado = false, g_rechazado = false;
uint32_t g_reconfigurarEn = 1;  // 1 = configurar en el primer nubeAtender
char g_id[13];
char g_topicoCmd[40], g_topicoResp[40], g_topicoEstado[40], g_topicoConexion[44];
char g_url[161], g_usuario[65], g_clave[129];
bool g_horaPedida = false;

// Lo que llega partido (mensajes más grandes que el buffer del cliente) se arma
// acá: el primer pedazo trae el tópico y los siguientes no.
Mensaje g_armando;
int g_armadoTotal = -1;

void alLlegar(const esp_mqtt_event_handle_t e) {
  if (e->current_data_offset == 0) {
    const bool esCmd = e->topic_len == static_cast<int>(strlen(g_topicoCmd)) && strncmp(e->topic, g_topicoCmd, e->topic_len) == 0;
    g_armadoTotal = esCmd && e->total_data_len <= static_cast<int>(MAX_MENSAJE) ? e->total_data_len : -1;
    if (esCmd && g_armadoTotal < 0) Serial.printf("[nube] mensaje de %d bytes: muy grande, lo ignoro\n", e->total_data_len);
  }
  if (g_armadoTotal < 0 || e->current_data_offset + e->data_len > g_armadoTotal) return;
  memcpy(g_armando.datos + e->current_data_offset, e->data, e->data_len);
  if (e->current_data_offset + e->data_len < g_armadoTotal) return;
  g_armando.n = static_cast<uint16_t>(g_armadoTotal);
  g_armadoTotal = -1;
  // Si el loop está ocupado (una imagen grande), esperar: TCP frena al que manda.
  if (xQueueSend(g_entrada, &g_armando, pdMS_TO_TICKS(5000)) != pdTRUE) Serial.println("[nube] se llenó la cola");
}

void alEvento(void*, esp_event_base_t, int32_t id, void* datos) {
  auto* e = static_cast<esp_mqtt_event_handle_t>(datos);
  switch (static_cast<esp_mqtt_event_id_t>(id)) {
    case MQTT_EVENT_CONNECTED:
      g_rechazado = false;
      esp_mqtt_client_subscribe(g_cliente, g_topicoCmd, 1);
      break;
    case MQTT_EVENT_SUBSCRIBED:
      // Se anuncia recién con la suscripción confirmada: si no, lo primero
      // que le manda la API se pierde.
      esp_mqtt_client_publish(g_cliente, g_topicoConexion, "1", 1, 1, 1);
      g_conectado = true;
      Serial.println("[nube] conectado");
      break;
    case MQTT_EVENT_DISCONNECTED:
      if (g_conectado) Serial.println("[nube] desconectado");
      g_conectado = false;
      break;
    case MQTT_EVENT_DATA:
      alLlegar(e);
      break;
    case MQTT_EVENT_ERROR:
      if (e->error_handle && e->error_handle->error_type == MQTT_ERROR_TYPE_CONNECTION_REFUSED) {
        if (!g_rechazado) Serial.println("[nube] el broker no me acepta: hay que sumarme a una cuenta de nuevo");
        g_rechazado = true;
      }
      break;
    default:
      break;
  }
}

void pararMqtt() {
  if (!g_cliente) return;
  esp_mqtt_client_stop(g_cliente);
  esp_mqtt_client_destroy(g_cliente);
  g_cliente = nullptr;
  g_conectado = false;
}

void arrancarMqtt() {
  if (g_cliente || !g_url[0]) return;
  esp_mqtt_client_config_t cfg = {};
  cfg.uri = g_url;
  cfg.client_id = g_id;
  cfg.username = g_usuario;
  cfg.password = g_clave;
  cfg.lwt_topic = g_topicoConexion;
  cfg.lwt_msg = "0";
  cfg.lwt_msg_len = 1;
  cfg.lwt_qos = 1;
  cfg.lwt_retain = 1;
  cfg.keepalive = 30;
  cfg.buffer_size = 1536;
  cfg.out_buffer_size = 1536;
  cfg.reconnect_timeout_ms = 5000;
  cfg.task_stack = 6144;
  if (strncmp(g_url, "mqtts://", 8) == 0) cfg.crt_bundle_attach = esp_crt_bundle_attach;
  g_cliente = esp_mqtt_client_init(&cfg);
  if (!g_cliente) return;
  esp_mqtt_client_register_event(g_cliente, MQTT_EVENT_ANY, alEvento, nullptr);
  esp_mqtt_client_start(g_cliente);
  Serial.printf("[nube] conectando a %s como %s\n", g_url, g_usuario);
}

void configurar(Sistema& s) {
  pararMqtt();
  char ssid[33], clave[65];
  const bool hayRed = s.red(ssid, sizeof ssid, clave, sizeof clave);
  if (!s.nube(g_url, sizeof g_url, g_usuario, sizeof g_usuario, g_clave, sizeof g_clave)) g_url[0] = 0;
  WiFi.disconnect(true);
  if (!hayRed) {
    WiFi.mode(WIFI_OFF);
    Serial.println("[nube] sin WiFi configurado");
    return;
  }
  WiFi.mode(WIFI_STA);
  WiFi.setAutoReconnect(true);
  WiFi.setHostname(g_id);
  WiFi.begin(ssid, clave[0] ? clave : nullptr);
  Serial.printf("[nube] WiFi: conectando a \"%s\"\n", ssid);
}

}  // namespace

void nubeIniciar(Sistema& s) {
  strlcpy(g_id, s.identificador(), sizeof g_id);
  snprintf(g_topicoCmd, sizeof g_topicoCmd, "bicho/%s/cmd", g_id);
  snprintf(g_topicoResp, sizeof g_topicoResp, "bicho/%s/resp", g_id);
  snprintf(g_topicoEstado, sizeof g_topicoEstado, "bicho/%s/state", g_id);
  snprintf(g_topicoConexion, sizeof g_topicoConexion, "bicho/%s/conexion", g_id);
  if (!g_entrada) g_entrada = xQueueCreate(12, sizeof(Mensaje));
  WiFi.onEvent([](WiFiEvent_t, WiFiEventInfo_t) { g_wifi = true; }, ARDUINO_EVENT_WIFI_STA_GOT_IP);
  WiFi.onEvent([](WiFiEvent_t, WiFiEventInfo_t) { g_wifi = false; }, ARDUINO_EVENT_WIFI_STA_DISCONNECTED);
  g_reconfigurarEn = millis() + 1;
}

void nubeReconfigurar() { g_reconfigurarEn = millis() + 600; }  // que salga la respuesta antes

void nubeAtender(Sistema& s) {
  if (g_reconfigurarEn && static_cast<int32_t>(millis() - g_reconfigurarEn) >= 0) {
    g_reconfigurarEn = 0;
    configurar(s);
  }
  if (g_wifi && !g_cliente) arrancarMqtt();
  if (g_wifi && !g_horaPedida) {
    configTzTime("<-03>3", "pool.ntp.org", "time.google.com");  // Argentina, sin horario de verano
    g_horaPedida = true;
  }
  if (!g_wifi && g_cliente && g_conectado) g_conectado = false;
  if (s.wifiConectado() != g_wifi || s.nubeConectada() != g_conectado) s.conexiones(g_wifi, g_conectado);

  static Mensaje m;
  while (g_entrada && xQueueReceive(g_entrada, &m, 0) == pdTRUE) {
    if (m.n >= 2) s.nubeEscribir(m.datos[0], m.datos + 1, m.n - 1);
  }
}

void nubeNotificar(uint8_t car, const uint8_t* d, size_t n) {
  if (!g_cliente || !g_conectado) return;
  if (car == protocolo::CarEstado) {
    esp_mqtt_client_publish(g_cliente, g_topicoEstado, reinterpret_cast<const char*>(d), static_cast<int>(n), 1, 1);
    return;
  }
  static uint8_t b[1 + 1100];
  if (n + 1 > sizeof b) return;
  b[0] = car;
  memcpy(b + 1, d, n);
  // QoS 0: el protocolo ya tiene sus respuestas; QoS 1 llenaría la memoria con
  // los trozos de una imagen.
  esp_mqtt_client_publish(g_cliente, g_topicoResp, reinterpret_cast<const char*>(b), static_cast<int>(n + 1), 0, 0);
}

bool nubeSana(const Sistema& s) { return !s.redConfigurada() || g_conectado; }

int nubeMinutoDelDia() {
  const time_t t = time(nullptr);
  if (t < 1700000000) return -1;
  struct tm local;
  localtime_r(&t, &local);
  return local.tm_hour * 60 + local.tm_min;
}

}  // namespace comun

#endif  // SINTETICA_CON_WIFI
