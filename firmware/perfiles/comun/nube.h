// WiFi + MQTT para los perfiles con WiFi (la 1.47B; después, el cubito).
// Usa las credenciales que guardó el sistema (comandos 0x16 y 0x17 por BLE) y
// se reconecta solo. Por MQTT viajan los mismos marcos que por BLE (ver
// docs/protocolo-ble.md, "Nube").
#pragma once

#include "sistema.h"

namespace comun {

void nubeIniciar(sintetica::Sistema& s);
// En cada vuelta del loop: entrega al sistema lo que llegó y le avisa cómo
// está conectado.
void nubeAtender(sintetica::Sistema& s);
// Para el HAL: cambiaron las credenciales (se reconecta en un ratito).
void nubeReconfigurar();
// Para el HAL: publica una respuesta o el estado.
void nubeNotificar(uint8_t caracteristica, const uint8_t* datos, size_t n);
// Sin WiFi configurado, o conectado a la nube: para confirmar una OTA.
bool nubeSana(const sintetica::Sistema& s);
// Con WiFi la hora sale de NTP (hora de Argentina). -1 si todavía no se sabe.
int nubeMinutoDelDia();

}  // namespace comun
