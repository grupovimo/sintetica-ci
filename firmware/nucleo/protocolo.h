// Protocolo BLE de SintéticaOS (versión 1). Documentado en docs/protocolo-ble.md.
// Lo mismo viaja por BLE en la placa y por el panel del simulador.
#pragma once

#include "formato.h"
#include "mate.h"

namespace sintetica {
namespace protocolo {

// UUID base: 5157e71c-a000-4b1c-8a5e-0000000000XX (XX = el id de abajo).
constexpr const char* UUID_SERVICIO = "5157e71c-a000-4b1c-8a5e-000000000001";
constexpr const char* UUID_CONFIG = "5157e71c-a000-4b1c-8a5e-000000000002";
constexpr const char* UUID_COMANDO = "5157e71c-a000-4b1c-8a5e-000000000003";
constexpr const char* UUID_ESTADO = "5157e71c-a000-4b1c-8a5e-000000000004";
constexpr const char* UUID_OTA = "5157e71c-a000-4b1c-8a5e-000000000005";
constexpr const char* UUID_IMAGEN = "5157e71c-a000-4b1c-8a5e-000000000006";

enum Caracteristica : uint8_t { CarConfig = 1, CarComando = 2, CarEstado = 3, CarOta = 4, CarImagen = 5 };

// El peor caso de QOI (3 canales, ruido) es ancho × alto × 4 + 22 bytes: el
// límite de cada bicho depende de su pantalla (en el broche, 230 KB; una foto
// común ocupa entre 80 y 130 KB). Este es el de la pantalla más grande en placa.
constexpr uint32_t MAX_IMAGEN_BYTES = 320 * 240 * 4 + 22;
inline uint32_t limiteImagen(int ancho, int alto) { return static_cast<uint32_t>(ancho) * static_cast<uint32_t>(alto) * 4u + 22u; }

// Operaciones (primer byte de cada escritura).
enum Op : uint8_t {
  // Config
  ConfigInicio = 0x01,   // u32 largo, u32 crc32
  ConfigBloque = 0x02,   // u32 desplazamiento, datos
  ConfigFin = 0x03,      // (nada)
  ConfigPedir = 0x04,    // (nada) → el bicho manda su config en notificaciones
  // Comando
  CmdExpresion = 0x10,   // nombre UTF-8; vacío = volver a automático
  CmdNombre = 0x11,      // nombre UTF-8, 1 a 24 bytes
  CmdEvento = 0x12,      // u8 evento
  CmdVincular = 0x13,    // código del QR (6 bytes) → responde con una clave de 16 bytes
  CmdAutenticar = 0x14,  // clave de 16 bytes (en cada conexión nueva)
  CmdDesvincular = 0x15, // olvida todas las claves y vuelve a mostrar el QR
  // Imagen
  ImgInicio = 0x20,      // u8 espacio (0-7), u32 largo, u32 crc32
  ImgBloque = 0x21,      // u32 desplazamiento, datos
  ImgFin = 0x22,         // (nada)
  ImgBorrar = 0x23,      // u8 espacio
  ImgListar = 0x24,      // → [0xA4][0][máscara u8] + por espacio ocupado [espacio][u32 largo][u32 crc]
  // Respuestas (notificación): op | 0x80, código, [detalle]
  Respuesta = 0x80,
  ConfigTrozo = 0x84,    // u32 desplazamiento, datos (respuesta a ConfigPedir)
  ConfigCierre = 0x85,   // u32 largo, u32 crc32 (fin de ConfigPedir)
};

enum Codigo : uint8_t {
  Ok = 0,
  MuyGrande = 1,
  FueraDeOrden = 2,
  CrcDistinto = 3,
  JsonInvalido = 4,
  ConfigInvalida = 5,
  SinInicio = 6,
  NoExiste = 7,
  Malformado = 8,
  NoAutorizado = 9,      // falta vincular o autenticar, o el código/clave no coincide
  NoEmparejando = 10,    // para vincular, el bicho tiene que estar mostrando el QR
  ImagenInvalida = 11,   // no es un QOI del tamaño de la pantalla
  NoImplementado = 0xFF,
};

constexpr size_t MAX_NOTIFICACION = 180;  // entra con un MTU de 185

uint32_t crc32(const uint8_t* datos, size_t n, uint32_t crc = 0);

inline uint32_t leer32(const uint8_t* p) {
  return static_cast<uint32_t>(p[0]) | static_cast<uint32_t>(p[1]) << 8 |
         static_cast<uint32_t>(p[2]) << 16 | static_cast<uint32_t>(p[3]) << 24;
}
inline void escribir32(uint8_t* p, uint32_t v) {
  p[0] = static_cast<uint8_t>(v), p[1] = static_cast<uint8_t>(v >> 8);
  p[2] = static_cast<uint8_t>(v >> 16), p[3] = static_cast<uint8_t>(v >> 24);
}

// Arma la configuración que llega por partes.
class Receptor {
 public:
  Codigo inicio(uint32_t largo, uint32_t crc);
  Codigo bloque(uint32_t desplazamiento, const uint8_t* datos, size_t n);
  Codigo fin();  // verifica largo y CRC
  const char* texto() const { return reinterpret_cast<const char*>(buf_); }
  size_t largo() const { return recibido_; }
  void reiniciar() { activo_ = false, recibido_ = 0; }

 private:
  uint8_t buf_[MAX_CONFIG_BYTES + 1];
  uint32_t esperado_ = 0, crc_ = 0, recibido_ = 0;
  bool activo_ = false;
};

// El estado público que se lee o se notifica.
struct EstadoPublico {
  uint8_t bateria = 255;  // 0-100; 255 = no se sabe
  int8_t humor = 0;       // -100..100
  uint8_t modo = 0;
  bool dormido = false;
  bool fija = false;
  bool emparejando = false;
  bool autenticado = false;
  const char* expresion = "";
  const char* nombre = "";
  const char* serie = "";
  const char* id = "";
  uint16_t ancho = 0, alto = 0;
  bool redonda = false;
};
constexpr uint8_t VERSION_ESTADO = 3;
// [versión=3][batería][humor][modo][flags][n][expresión][n][nombre][n][serie][n][id]
// [u16 ancho][u16 alto][forma: 0 rectangular, 1 redonda]
// flags: bit 0 dormido, 1 expresión fija, 2 mostrando el QR, 3 esta conexión autenticada
size_t codificarEstado(const EstadoPublico& e, uint8_t* salida, size_t cap);

}  // namespace protocolo
}  // namespace sintetica
