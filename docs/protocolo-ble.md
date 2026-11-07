# Protocolo BLE de SintéticaOS (v1)

Un servicio GATT propio. Todos los enteros van en little-endian. La
implementación de referencia es `firmware/nucleo/protocolo.*` y
`firmware/nucleo/sistema.cpp`; el panel del simulador manda exactamente estos
mismos bytes.

## Servicio y características

| Característica | UUID | Propiedades |
| --- | --- | --- |
| Servicio | `5157e71c-a000-4b1c-8a5e-000000000001` | |
| Config | `5157e71c-a000-4b1c-8a5e-000000000002` | write, write without response, notify |
| Comando | `5157e71c-a000-4b1c-8a5e-000000000003` | write, notify |
| Estado | `5157e71c-a000-4b1c-8a5e-000000000004` | read, notify |
| OTA | `5157e71c-a000-4b1c-8a5e-000000000005` | write, notify (reservada, todavía no implementada) |

El nombre BLE del bicho es el que eligió el dueño.

## Respuestas

Cada operación que se puede confirmar responde con una notificación en la misma
característica: `[op | 0x80][código][detalle opcional en UTF-8]`.

| Código | Significado |
| --- | --- |
| `0x00` | Ok |
| `0x01` | Muy grande (más de 8 KB) |
| `0x02` | Fuera de orden (un bloque con un desplazamiento inesperado; aborta la transferencia) |
| `0x03` | CRC distinto |
| `0x04` | JSON inválido (el detalle dice por qué) |
| `0x05` | Config inválida (el detalle dice qué campo) |
| `0x06` | Sin inicio |
| `0x07` | No existe (una expresión desconocida) |
| `0x08` | Malformado |
| `0xFF` | No implementado |

## Config

La configuración (el JSON de `shared/formato/config.schema.json`) se manda
entera, partida en bloques que entren en el MTU.

| Op | Datos | Qué hace |
| --- | --- | --- |
| `0x01` Inicio | u32 largo, u32 CRC-32 (IEEE) del JSON | Empieza una transferencia. Responde. |
| `0x02` Bloque | u32 desplazamiento, bytes | Agrega un bloque. Solo responde si hay error. |
| `0x03` Fin | — | Verifica largo y CRC, valida y aplica la config, la guarda en flash. Responde. |
| `0x04` Pedir | — | El bicho manda su config actual: varios `0x84` y un `0x85`. |

- `0x84` Trozo: `[0x84][u32 desplazamiento][bytes]`
- `0x85` Cierre: `[0x85][u32 largo][u32 CRC-32]`

Si la config no es válida, el bicho sigue con la anterior. Una config nueva
borra el nombre puesto por comando (gana el de la config).

## Comando

| Op | Datos | Qué hace |
| --- | --- | --- |
| `0x10` Expresión | nombre UTF-8 (vacío = automático) | Deja fija esa expresión. Se guarda. |
| `0x11` Nombre | UTF-8, de 1 a 24 bytes | Cambia el nombre. Se guarda. |
| `0x12` Evento | u8 evento | Simula un evento como si viniera de un sensor. |

Eventos: `0` tacto, `1` doble toque, `2` levantado, `3` sacudida, `4` boca abajo,
`5` vinculado, `6` app abierta, `7` despertado.

## Estado

Lectura o notificación (se notifica cuando cambia algo que no sea el humor):

```
[versión = 1][batería u8, 255 = no se sabe][humor i8, -100..100][modo u8]
[flags u8: bit 0 dormido, bit 1 expresión fija]
[n u8][expresión visible, UTF-8][n u8][nombre, UTF-8]
```

Modos: `0` mascota, `1` reloj, `2` pomodoro, `3` clima, `4` notificaciones.

## Probarlo con nRF Connect

1. Conectarse al bicho y abrir el servicio `…0001`.
2. Activar las notificaciones de Comando (`…0003`) y Estado (`…0004`).
3. Escribir en Comando el hex `10 65 6E 61 6D 6F 72 61 64 6F` ("enamorado"):
   llega `90 00` y la cara cambia.
4. Escribir `11 54 6F 74 6F` ("Toto"): llega `91 00`.
5. Reiniciar el bicho: sigue enamorado y se llama Toto.
6. Escribir `10` solo: vuelve al comportamiento automático.
