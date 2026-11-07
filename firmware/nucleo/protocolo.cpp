#include "protocolo.h"

namespace sintetica {
namespace protocolo {

uint32_t crc32(const uint8_t* datos, size_t n, uint32_t crc) {
  crc = ~crc;
  for (size_t i = 0; i < n; i++) {
    crc ^= datos[i];
    for (int k = 0; k < 8; k++) crc = (crc >> 1) ^ (0xEDB88320u & (0u - (crc & 1u)));
  }
  return ~crc;
}

Codigo Receptor::inicio(uint32_t largo, uint32_t crc) {
  activo_ = false;
  recibido_ = 0;
  if (largo == 0 || largo > MAX_CONFIG_BYTES) return MuyGrande;
  esperado_ = largo;
  crc_ = crc;
  activo_ = true;
  return Ok;
}

Codigo Receptor::bloque(uint32_t desplazamiento, const uint8_t* datos, size_t n) {
  if (!activo_) return SinInicio;
  // Un bloque malo aborta la transferencia: la app tiene que empezar de nuevo.
  if (desplazamiento != recibido_) return activo_ = false, FueraDeOrden;
  if (recibido_ + n > esperado_) return activo_ = false, MuyGrande;
  copiarBytes(buf_ + recibido_, datos, n);
  recibido_ += static_cast<uint32_t>(n);
  return Ok;
}

Codigo Receptor::fin() {
  if (!activo_) return SinInicio;
  activo_ = false;
  if (recibido_ != esperado_) return FueraDeOrden;
  if (crc32(buf_, recibido_) != crc_) return CrcDistinto;
  buf_[recibido_] = 0;
  return Ok;
}

size_t codificarEstado(const EstadoPublico& e, uint8_t* s, size_t cap) {
  const size_t ne = largo(e.expresion), nn = largo(e.nombre), ns = largo(e.serie), ni = largo(e.id);
  const size_t total = 5 + 1 + ne + 1 + nn + 1 + ns + 1 + ni;
  if (cap < total || ne > 255 || nn > 255 || ns > 255 || ni > 255) return 0;
  size_t i = 0;
  s[i++] = VERSION_ESTADO;
  s[i++] = e.bateria;
  s[i++] = static_cast<uint8_t>(e.humor);
  s[i++] = e.modo;
  s[i++] = static_cast<uint8_t>((e.dormido ? 1 : 0) | (e.fija ? 2 : 0) | (e.emparejando ? 4 : 0) |
                                (e.autenticado ? 8 : 0));
  s[i++] = static_cast<uint8_t>(ne);
  copiarBytes(s + i, e.expresion, ne);
  i += ne;
  s[i++] = static_cast<uint8_t>(nn);
  copiarBytes(s + i, e.nombre, nn);
  i += nn;
  s[i++] = static_cast<uint8_t>(ns);
  copiarBytes(s + i, e.serie, ns);
  i += ns;
  s[i++] = static_cast<uint8_t>(ni);
  copiarBytes(s + i, e.id, ni);
  i += ni;
  return i;
}

}  // namespace protocolo
}  // namespace sintetica
