# Protocolo BLE de SintéticaOS (v1.2)

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
| Imagen | `5157e71c-a000-4b1c-8a5e-000000000006` | write, write without response, notify (v1.2) |

El nombre BLE del bicho es el que eligió el dueño. El anuncio lleva, además del
UUID del servicio, **datos de fabricante** `FF FF` + los 6 bytes de la MAC: es
el id del bicho, el mismo que trae el QR, así la app encuentra el bicho
correcto sin conectarse a todos. (`0xFFFF` es el id reservado para pruebas del
Bluetooth SIG; si Sintética Labs se registra como empresa, se cambia.)

## Vínculo y autenticación

El bicho muestra un QR con `SINTETICA:V1/<id>/<código>`: el id es la MAC en 12
caracteres hexadecimales y el código, 6 caracteres de `23456789ABCDEFGHJKMNPQRSTUVWXYZ`
que se generan en el primer arranque y no cambian.

- Muestra el QR solo en dos casos: cuando ningún celu está vinculado, y durante
  2 minutos después de mantener el botón 5 s.
- Leer Estado es libre. Todo lo demás (Config, Comando, OTA) responde `0x09` hasta
  que la conexión se autentica.
- **Vincular** (`0x13` + el código en 6 bytes) funciona solo mientras muestra el
  QR: responde `[0x93][0x00][clave de 16 bytes]`, deja la conexión autenticada,
  deja de mostrar el QR y guiña. La app guarda la clave.
- **Autenticar** (`0x14` + la clave) en cada conexión nueva. El bicho guarda
  hasta 4 claves (4 celus); la quinta reemplaza a la más vieja.
- **Desvincular** (`0x15`) olvida todas las claves y vuelve a mostrar el QR.
- 5 intentos fallidos seguidos bloquean vincular y autenticar por 30 s.
- Límite de esta versión: la clave viaja sin cifrar por BLE. LE Secure
  Connections (cifrado del enlace) queda para M8.

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
| `0x09` | No autorizado (falta vincular o autenticar, o el código o la clave no coinciden) |
| `0x0A` | No está mostrando el QR (mantené el botón 5 s; en el broche, 8 s) |
| `0x0B` | Imagen inválida (no es un QOI del tamaño de la pantalla) |
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

## Imagen (v1.2)

El bicho guarda hasta 8 imágenes (espacios 0 a 7), en
[QOI](https://qoiformat.org) con 3 canales y **exactamente del tamaño de su
pantalla** (lo dice el estado v3). Pesan hasta 240 KB. La app recorta la imagen
a la forma de la pantalla (en el broche, redonda: lo de afuera del círculo en
`#0B0A09`). Las imágenes se muestran desde la lista de la config
(`"lista": ["imagen:0", …]`).

| Op | Datos | Qué hace |
| --- | --- | --- |
| `0x20` Inicio | u8 espacio, u32 largo, u32 CRC-32 | Empieza a recibir; borra lo que había en ese espacio. Responde. |
| `0x21` Bloque | u32 desplazamiento, bytes | Se escribe directo a la flash. Solo responde si hay error (y entonces la transferencia se aborta). |
| `0x22` Fin | — | Verifica largo, CRC y la cabecera QOI (tamaño de la pantalla). Responde. |
| `0x23` Borrar | u8 espacio | Responde. |
| `0x24` Listar | — | Responde `[0xA4][0x00][máscara de ocupados]` + por cada espacio ocupado `[espacio][u32 largo][u32 CRC-32]`. |

Los bloques conviene mandarlos sin respuesta (write without response): es varias
veces más rápido.

## Comando

| Op | Datos | Qué hace |
| --- | --- | --- |
| `0x10` Expresión | nombre UTF-8 (vacío = automático) | Deja fija esa expresión. Se guarda. |
| `0x11` Nombre | UTF-8, de 1 a 24 bytes | Cambia el nombre. Se guarda. |
| `0x12` Evento | u8 evento | Simula un evento como si viniera de un sensor. |
| `0x13` Vincular | el código del QR (6 bytes) | Ver "Vínculo y autenticación". No pide autenticación. |
| `0x14` Autenticar | la clave (16 bytes) | Autentica esta conexión. No pide autenticación. |
| `0x15` Desvincular | — | Olvida todas las claves y vuelve a mostrar el QR. |

Eventos: `0` tacto, `1` doble toque, `2` levantado, `3` sacudida, `4` boca abajo,
`5` vinculado, `6` app abierta, `7` despertado.

## Estado

Lectura o notificación (se notifica cuando cambia algo que no sea el humor):

```
[versión = 3][batería u8, 255 = no se sabe][humor i8, -100..100][modo u8]
[flags u8: bit 0 dormido, 1 expresión fija, 2 mostrando el QR, 3 esta conexión autenticada]
[n u8][expresión visible][n u8][nombre][n u8][número de unidad][n u8][id]
[u16 ancho][u16 alto][forma u8: 0 rectangular, 1 redonda]
```

(Textos en UTF-8. La versión 2 no tenía la pantalla; la 1 tampoco el número
de unidad, el id ni los bits 2 y 3.)

Modos: `0` mascota, `1` reloj, `2` pomodoro, `3` clima, `4` notificaciones.

## Taller

Por USB serie (115200 baudios), `serie 001` graba el número de unidad. Al
arrancar, el bicho imprime su id, su número y si ya está vinculado.

## Probarlo con nRF Connect

1. Conectarse al bicho y abrir el servicio `…0001`.
2. Activar las notificaciones de Comando (`…0003`) y Estado (`…0004`).
3. Escribir en Comando `13` + el código del QR en ASCII (por ejemplo, para
   `K7Q2XM`: `13 4B 37 51 32 58 4D`): llega `93 00` + la clave. Guardala si
   vas a reconectar (`14` + la clave).
4. Escribir en Comando el hex `10 65 6E 61 6D 6F 72 61 64 6F` ("enamorado"):
   llega `90 00` y la cara cambia.
5. Escribir `11 54 6F 74 6F` ("Toto"): llega `91 00`.
6. Reiniciar el bicho y reconectar: autenticar con `14` + la clave. Sigue
   enamorado y se llama Toto.
7. Escribir `10` solo: vuelve al comportamiento automático.
