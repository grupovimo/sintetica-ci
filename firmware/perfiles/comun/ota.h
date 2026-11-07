// OTA firmada con vuelta atrás, para todos los perfiles ESP32.
//
// - Por BLE (la app manda el binario en bloques) o, con WiFi, bajándolo por
//   HTTP(S) desde la URL que manda la nube.
// - Antes de aceptar un byte verifica la firma ECDSA P-256 del manifiesto con
//   la clave pública embebida (clave_ota.h); al final verifica el SHA-256 de lo
//   que escribió. Recién ahí marca la partición nueva y reinicia.
// - El firmware nuevo arranca "a prueba": si no se confirma (otaAtender) antes
//   de 2 minutos, o si se cuelga y reinicia antes, el bootloader vuelve solo a
//   la versión anterior.
#pragma once

#include "sistema.h"

namespace comun {

// Completa otaEmpezar/Escribir/Terminar/Cancelar (y otaDescargar si el perfil
// tiene WiFi: conDescarga = true).
void otaCompletarHal(sintetica::Hal& h, bool conDescarga);

// En cada vuelta del loop: pasa el progreso de la descarga al sistema,
// reinicia después de instalar y confirma el firmware nuevo cuando anda bien.
// `sano` = el perfil considera que funciona (por ejemplo, conectado a la nube
// si tiene WiFi configurado).
void otaAtender(sintetica::Sistema& s, bool sano);

// La versión que corre está a prueba (recién instalada por OTA).
bool otaAPrueba();
// La confirma ya (por ejemplo, antes del sueño profundo del broche).
void otaConfirmar();

}  // namespace comun
