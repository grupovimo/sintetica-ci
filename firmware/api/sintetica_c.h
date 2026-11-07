/* API en C del núcleo de SintéticaOS, para las apps (Swift la importa con un
 * module map; Kotlin, por JNI) y cualquier otro anfitrión.
 *
 * Hay UNA instancia por proceso: el núcleo usa memoria estática. Todas las
 * funciones se llaman desde el mismo hilo.
 *
 * Dos usos:
 *  - Vista previa idéntica: sintetica_iniciar + sintetica_vista_previa, se le
 *    manda la config del bicho (protocolo Config) y la expresión, y se dibuja
 *    con sintetica_tick. La geometría es la misma que en la placa; el ancho
 *    puede ser el doble (640 × 344) para que se vea nítido en el celu.
 *  - Bicho virtual ("probar sin bicho"): lo mismo, pero la app le habla con los
 *    bytes del protocolo (sintetica_ble_escribir) y recibe las notificaciones
 *    por hal.notificar, como si fuera BLE.
 */
#ifndef SINTETICA_C_H
#define SINTETICA_C_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct sintetica_hal {
  void* contexto; /* se pasa tal cual a cada función */
  uint32_t (*ahora_ms)(void* contexto);
  uint32_t (*semilla)(void* contexto);
  int (*minuto_del_dia)(void* contexto); /* -1 si no se sabe */
  int (*bateria)(void* contexto);        /* 0-100, -1 si no hay */
  int (*guardar)(void* contexto, const char* clave, const uint8_t* datos, size_t n); /* 1 = ok */
  int (*leer)(void* contexto, const char* clave, uint8_t* datos, size_t cap);        /* -1 si no está */
  void (*notificar)(void* contexto, int caracteristica, const uint8_t* datos, size_t n);
  void (*registrar)(void* contexto, const char* mensaje);
  const char* identificador; /* 12 caracteres hexadecimales; NULL = "000000000000" */
} sintetica_hal;

/* Arranca el núcleo. ancho ≤ 640, alto ≤ 640. Cualquier función de la tabla puede ser NULL. */
void sintetica_iniciar(const sintetica_hal* hal, int ancho, int alto);
void sintetica_vista_previa(void);

void sintetica_tick(void);
const uint16_t* sintetica_pixeles(void); /* RGB565, ancho × alto, fila por fila */
int sintetica_ancho(void);
int sintetica_alto(void);
/* Convierte el último cuadro a RGBA de 8 bits (ancho × alto × 4 bytes). */
void sintetica_pixeles_rgba(uint8_t* salida);

void sintetica_tacto(int apretado);
void sintetica_imu(float ax, float ay, float az);
void sintetica_evento(int evento);

void sintetica_ble_conexion(int conectado);
void sintetica_ble_escribir(int caracteristica, const uint8_t* datos, size_t n);
size_t sintetica_ble_estado(uint8_t* salida, size_t cap);

int sintetica_cantidad_expresiones(void);
const char* sintetica_nombre_expresion(int indice);
const char* sintetica_expresion_visible(void);
const char* sintetica_qr(void);
const char* sintetica_version(void);

#ifdef __cplusplus
}
#endif

#endif
