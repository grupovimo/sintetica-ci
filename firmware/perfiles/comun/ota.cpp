#include "ota.h"

#include <Arduino.h>
#include <esp_ota_ops.h>
#include <esp_system.h>
#include <mbedtls/ecdsa.h>
#include <mbedtls/sha256.h>
#include <mbedtls/version.h>

#include "clave_ota.h"

#ifdef SINTETICA_CON_WIFI
#include <esp_http_client.h>
#endif

// Los certificados raíz que trae ESP-IDF (para https:// y mqtts://). El
// esp_crt_bundle.h de Arduino tapa al de IDF, así que se declara acá.
extern "C" esp_err_t esp_crt_bundle_attach(void* conf);

using namespace sintetica;
using namespace sintetica::protocolo;

// El arranque de Arduino no confirma el firmware nuevo: lo confirma otaAtender
// cuando el bicho anda bien (si no, vuelve a la versión anterior).
bool verifyRollbackLater() { return true; }

namespace comun {

namespace {

#if MBEDTLS_VERSION_MAJOR >= 3
#define SHA_EMPEZAR(c) mbedtls_sha256_starts((c), 0)
#define SHA_SUMAR(c, d, n) mbedtls_sha256_update((c), (d), (n))
#define SHA_TERMINAR(c, h) mbedtls_sha256_finish((c), (h))
#define SHA_DE_UNA(d, n, h) mbedtls_sha256((d), (n), (h), 0)
#else
#define SHA_EMPEZAR(c) mbedtls_sha256_starts_ret((c), 0)
#define SHA_SUMAR(c, d, n) mbedtls_sha256_update_ret((c), (d), (n))
#define SHA_TERMINAR(c, h) mbedtls_sha256_finish_ret((c), (h))
#define SHA_DE_UNA(d, n, h) mbedtls_sha256_ret((d), (n), (h), 0)
#endif

struct EstadoOta {
  bool activa = false;
  esp_ota_handle_t handle = 0;
  const esp_partition_t* particion = nullptr;
  mbedtls_sha256_context sha;
  ManifiestoOta m;
  uint32_t escrito = 0;
} g_ota;

uint32_t g_reiniciarEn = 0;
int g_aPrueba = -1;  // -1 = no se sabe todavía

struct Progreso {
  uint8_t codigo, porcentaje;
  char detalle[48];
};
QueueHandle_t g_progreso = nullptr;

bool firmaValida(const ManifiestoOta& m) {
  if (CLAVE_OTA_LARGO != 65) {
    Serial.println("[ota] este firmware no tiene clave pública: rechazo toda OTA");
    return false;
  }
  char texto[160];
  const size_t n = mensajeOta(m, texto, sizeof texto);
  if (!n) return false;
  uint8_t hash[32];
  if (SHA_DE_UNA(reinterpret_cast<const uint8_t*>(texto), n, hash) != 0) return false;
  mbedtls_ecp_group grupo;
  mbedtls_ecp_point q;
  mbedtls_mpi r, s;
  mbedtls_ecp_group_init(&grupo);
  mbedtls_ecp_point_init(&q);
  mbedtls_mpi_init(&r);
  mbedtls_mpi_init(&s);
  const bool ok = mbedtls_ecp_group_load(&grupo, MBEDTLS_ECP_DP_SECP256R1) == 0 &&
                  mbedtls_ecp_point_read_binary(&grupo, &q, CLAVE_OTA, CLAVE_OTA_LARGO) == 0 &&
                  mbedtls_mpi_read_binary(&r, m.firma, 32) == 0 && mbedtls_mpi_read_binary(&s, m.firma + 32, 32) == 0 &&
                  mbedtls_ecdsa_verify(&grupo, hash, sizeof hash, &q, &r, &s) == 0;
  mbedtls_mpi_free(&s);
  mbedtls_mpi_free(&r);
  mbedtls_ecp_point_free(&q);
  mbedtls_ecp_group_free(&grupo);
  return ok;
}

void abortar() {
  if (g_ota.activa) {
    esp_ota_abort(g_ota.handle);
    mbedtls_sha256_free(&g_ota.sha);
  }
  g_ota.activa = false;
}

uint8_t empezar(const ManifiestoOta& m) {
  if (g_ota.activa) return Ocupado;
  if (!firmaValida(m)) return FirmaInvalida;
  const esp_partition_t* p = esp_ota_get_next_update_partition(nullptr);
  if (!p) return FalloOta;
  if (m.tamano == 0 || m.tamano > p->size) return MuyGrande;
  Serial.printf("[ota] %s %s, %u bytes → %s (borrando…)\n", m.perfil, m.version, static_cast<unsigned>(m.tamano), p->label);
  esp_ota_handle_t h = 0;
  if (esp_ota_begin(p, m.tamano, &h) != ESP_OK) return FalloOta;
  g_ota.activa = true;
  g_ota.handle = h;
  g_ota.particion = p;
  g_ota.m = m;
  g_ota.escrito = 0;
  mbedtls_sha256_init(&g_ota.sha);
  SHA_EMPEZAR(&g_ota.sha);
  return Ok;
}

uint8_t escribir(uint32_t off, const uint8_t* d, size_t n) {
  if (!g_ota.activa) return SinInicio;
  if (off != g_ota.escrito || g_ota.escrito + n > g_ota.m.tamano) {
    abortar();
    return FueraDeOrden;
  }
  if (esp_ota_write(g_ota.handle, d, n) != ESP_OK) {
    abortar();
    return FalloOta;
  }
  SHA_SUMAR(&g_ota.sha, d, n);
  g_ota.escrito += static_cast<uint32_t>(n);
  return Ok;
}

uint8_t terminar() {
  if (!g_ota.activa) return SinInicio;
  uint8_t hash[32];
  SHA_TERMINAR(&g_ota.sha, hash);
  mbedtls_sha256_free(&g_ota.sha);
  g_ota.activa = false;
  if (g_ota.escrito != g_ota.m.tamano || memcmp(hash, g_ota.m.sha256, 32) != 0) {
    esp_ota_abort(g_ota.handle);
    Serial.println("[ota] el SHA-256 no coincide con el firmado");
    return HashDistinto;
  }
  // esp_ota_end además valida la cabecera y el checksum de la imagen.
  if (esp_ota_end(g_ota.handle) != ESP_OK || esp_ota_set_boot_partition(g_ota.particion) != ESP_OK) return FalloOta;
  Serial.printf("[ota] instalada %s; reinicio y queda a prueba\n", g_ota.m.version);
  return Ok;
}

void avisar(uint8_t codigo, uint8_t pct, const char* detalle = nullptr) {
  if (!g_progreso) return;
  Progreso p{codigo, pct, {}};
  if (detalle) strlcpy(p.detalle, detalle, sizeof p.detalle);
  xQueueSend(g_progreso, &p, pdMS_TO_TICKS(100));
}

#ifdef SINTETICA_CON_WIFI
struct Descarga {
  ManifiestoOta m;
  char url[256];
} g_descarga;
volatile bool g_cancelar = false;
TaskHandle_t g_tarea = nullptr;

void tareaDescarga(void*) {
  uint8_t c = empezar(g_descarga.m);
  if (c != Ok) {
    avisar(c, 0, c == FirmaInvalida ? "la firma no es de Sintética Labs" : nullptr);
    g_tarea = nullptr;
    vTaskDelete(nullptr);
    return;
  }
  esp_http_client_config_t cfg = {};
  cfg.url = g_descarga.url;
  cfg.timeout_ms = 15000;
  cfg.buffer_size = 4096;
  if (strncmp(g_descarga.url, "https://", 8) == 0) cfg.crt_bundle_attach = esp_crt_bundle_attach;
  esp_http_client_handle_t cli = esp_http_client_init(&cfg);
  const char* error = nullptr;
  static uint8_t buf[4096];
  uint8_t pctAvisado = 0;
  if (!cli || esp_http_client_open(cli, 0) != ESP_OK) {
    error = "no me pude conectar";
  } else {
    const int largo = esp_http_client_fetch_headers(cli);
    const int estado = esp_http_client_get_status_code(cli);
    if (estado != 200) error = "el servidor no lo dio (HTTP distinto de 200)";
    else if (largo >= 0 && static_cast<uint32_t>(largo) != g_descarga.m.tamano) error = "otro tamaño que el firmado";
    while (!error && !g_cancelar && g_ota.escrito < g_descarga.m.tamano) {
      const int n = esp_http_client_read(cli, reinterpret_cast<char*>(buf), sizeof buf);
      if (n < 0) error = "se cortó la descarga";
      else if (n == 0) break;
      else if (escribir(g_ota.escrito, buf, static_cast<size_t>(n)) != Ok) error = "no pude escribir la flash";
      const uint8_t pct = static_cast<uint8_t>(static_cast<uint64_t>(g_ota.escrito) * 99u / g_descarga.m.tamano);
      if (!error && pct >= pctAvisado + 2) {
        pctAvisado = pct;
        avisar(Ok, pct);
      }
    }
  }
  if (cli) {
    esp_http_client_close(cli);
    esp_http_client_cleanup(cli);
  }
  if (g_cancelar && !error) error = "cancelada";
  if (error) {
    abortar();
    avisar(FalloOta, pctAvisado, error);
  } else {
    c = terminar();
    avisar(c, c == Ok ? 100 : pctAvisado);
  }
  g_tarea = nullptr;
  vTaskDelete(nullptr);
}
#endif

}  // namespace

void otaCompletarHal(Hal& h, bool conDescarga) {
  if (!g_progreso) g_progreso = xQueueCreate(8, sizeof(Progreso));
  h.otaEmpezar = empezar;
  h.otaEscribir = escribir;
  h.otaTerminar = [] {
    const uint8_t c = terminar();
    if (c == Ok) g_reiniciarEn = millis() + 1500;  // que llegue la respuesta antes
    return c;
  };
  h.otaCancelar = [] {
#ifdef SINTETICA_CON_WIFI
    if (g_tarea) {
      g_cancelar = true;
      return;
    }
#endif
    abortar();
  };
#ifdef SINTETICA_CON_WIFI
  if (conDescarga) {
    h.otaDescargar = [](const ManifiestoOta& m, const char* url) -> uint8_t {
      if (g_tarea || g_ota.activa) return Ocupado;
      g_descarga.m = m;
      strlcpy(g_descarga.url, url, sizeof g_descarga.url);
      g_cancelar = false;
      // La verificación de la firma y el borrado de la partición tardan: van en la tarea.
      if (xTaskCreate(tareaDescarga, "ota", 8192, nullptr, 3, &g_tarea) != pdPASS) return FalloOta;
      return Ok;
    };
  }
#else
  (void)conDescarga;
#endif
}

bool otaAPrueba() {
  if (g_aPrueba < 0) {
    esp_ota_img_states_t estado;
    g_aPrueba = esp_ota_get_state_partition(esp_ota_get_running_partition(), &estado) == ESP_OK &&
                estado == ESP_OTA_IMG_PENDING_VERIFY;
    if (g_aPrueba) Serial.println("[ota] firmware nuevo a prueba: si no anda bien, vuelvo al anterior");
  }
  return g_aPrueba == 1;
}

void otaConfirmar() {
  if (!otaAPrueba()) return;
  esp_ota_mark_app_valid_cancel_rollback();
  g_aPrueba = 0;
  Serial.println("[ota] firmware nuevo confirmado");
}

void otaAtender(Sistema& s, bool sano) {
  Progreso p;
  while (g_progreso && xQueueReceive(g_progreso, &p, 0) == pdTRUE) {
    s.otaProgreso(p.codigo, p.porcentaje, p.detalle[0] ? p.detalle : nullptr);
    if (p.codigo == Ok && p.porcentaje >= 100) g_reiniciarEn = millis() + 1500;
  }
  const uint32_t ahora = millis();
  if (g_reiniciarEn && static_cast<int32_t>(ahora - g_reiniciarEn) >= 0) esp_restart();
  if (otaAPrueba()) {
    if (ahora > 20000 && sano) {
      otaConfirmar();
    } else if (ahora > 120000) {
      Serial.println("[ota] el firmware nuevo no anduvo bien: vuelvo al anterior");
      esp_ota_mark_app_invalid_rollback_and_reboot();
    }
  }
}

}  // namespace comun
