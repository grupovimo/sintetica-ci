# Protocolo de SintéticaOS (v1.6)

Un servicio GATT propio. Todos los enteros van en little-endian. La
implementación de referencia es `firmware/nucleo/protocolo.*` y
`firmware/nucleo/sistema.cpp`; el panel del simulador manda exactamente estos
mismos bytes. Los mismos marcos viajan también por la nube (MQTT, ver "Nube").

Los bytes de cada operación están en `shared/protocolo/vectores.json`: el
núcleo C++, el JS de referencia, Kotlin y Swift se prueban contra ese archivo.

## Servicio y características

| Característica | UUID | Propiedades |
| --- | --- | --- |
| Servicio | `5157e71c-a000-4b1c-8a5e-000000000001` | |
| Config | `5157e71c-a000-4b1c-8a5e-000000000002` | write, write without response, notify |
| Comando | `5157e71c-a000-4b1c-8a5e-000000000003` | write, notify |
| Estado | `5157e71c-a000-4b1c-8a5e-000000000004` | read, notify |
| OTA | `5157e71c-a000-4b1c-8a5e-000000000005` | write, write without response, notify (v1.3) |
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
| `0x01` | Muy grande (la config, más de 16 KB) |
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
| `0x0C` | Firma inválida (el firmware no lo firmó Sintética Labs) |
| `0x0D` | Hash distinto (lo que llegó no es lo que se firmó) |
| `0x0E` | Perfil distinto (es el firmware de otra placa) |
| `0x0F` | Sin WiFi (para descargar hace falta red) |
| `0x10` | Ocupado (ya hay una actualización o una transferencia en curso) |
| `0x11` | Falló la actualización (el detalle dice por qué) |
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

El bicho guarda hasta 8 imágenes (espacios 0 a 7; los espacios 8 a 19 son los
sprites de las caras de imagen, abajo), en
[QOI](https://qoiformat.org) con 3 canales y **exactamente del tamaño de su
pantalla** (lo dice el estado v3). Pesan como mucho ancho × alto × 4 + 22
bytes, el peor caso de QOI (en el broche, 230 KB; una foto común, 80 a 130 KB). La app recorta la imagen
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
| `0x25` Pedir | u8 espacio | Responde `[0xA5][0x00]` (o `[0xA5][0x07]` si está vacío), varios `[0xA7][u32 desplazamiento][bytes]` y `[0xA8][espacio][u32 largo][u32 CRC-32]` (v1.3). |
| `0x26` Listar sprites | — | Responde `[0xA6][0x00][u16 máscara de ocupados]` + por cada sprite ocupado `[espacio 8-19][u32 largo][u32 CRC-32]` (v1.5). Un firmware anterior responde `[0xA6][0x08]`. |

### Sprites (v1.5)

Las **caras de imagen** (ver [motor-de-vida.md](motor-de-vida.md#caras-de-imagen))
usan sprites: hasta **12** (0 a 11), aparte de las 8 imágenes, en los
**espacios 8 a 19** de esta misma característica (el sprite `n` es el espacio
`8 + n`) y con los mismos marcos: `0x20` Inicio, `0x21` Bloque, `0x22` Fin,
`0x23` Borrar y `0x25` Pedir. Lo que cambia:

- Un sprite es un **QOI de 3 o 4 canales** (con transparencia) de **1 a 128 por
  lado**, de cualquier tamaño dentro de eso (no el de la pantalla): pesa como
  mucho 128 × 128 × 5 + 22 = 81 942 bytes (si no, `0x01` Muy grande; si no es
  un QOI así, `0x0B` Imagen inválida).
- `0x24` Listar no los muestra (sigue siendo la máscara de un byte de las
  imágenes); se listan con `0x26`.
- En la flash van como `spr0` a `spr11` y sus largos y CRC en `sprites`.
- Si se sube o se borra el sprite de una cara que se está viendo, la cara se va
  con un fundido (los sprites ya decodificados siguen en la RAM) y vuelve
  cuando llega el nuevo: nunca un corte.

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
| `0x16` WiFi | `[n][ssid][n][clave]` (ssid de 1 a 32 bytes, clave de 0 a 64) | Lo guarda y se conecta. Solo por BLE (v1.3). |
| `0x17` Nube | `[n][url][n][usuario][n][clave]` (url `mqtt://` o `mqtts://`, hasta 160 bytes) | Las credenciales MQTT que da la API al sumar el bicho a una cuenta. Solo por BLE (v1.3). |
| `0x18` Olvidar red | — | Olvida el WiFi y la nube (v1.3). |
| `0x19` Reproducir | nombre de una animación (vacío = parar) | La reproduce; si la animación tiene `"repetir": true`, hasta que la paren (v1.3). |
| `0x1A` Hora | `[u32 epoch UTC][i16 zona en minutos]` (Argentina: −180) | La hora, para el ritmo diario y la memoria. Las apps la mandan al conectarse (el broche no tiene WiFi) y la API a cada bicho que se conecta y cada 6 h. El ESP32 la guarda en su reloj, que sigue andando en sueño profundo (v1.4). |
| `0x1B` Vida | — o `[u8 superposición]` | Responde el estado del motor de vida (abajo); con `1`/`0` prende o apaga la superposición de depuración en la pantalla (v1.4). |
| `0x1C` Memoria | — | Responde `[0x9C][0x00][memoria, 112 bytes]` (v1.4). |
| `0x1D` Restaurar | memoria (112 bytes, con su CRC) | La reemplaza: la mudanza a otra placa (v1.4). |
| `0x1E` Registro | — | Responde `[0x9E][0x00][n][n × (u32 segundos desde que arrancó, u8 tipo, u8 dato)]`: tipo 0 evento, 1 comportamiento, 2 dormir (1) / despertar (0), 3 hito (v1.4), 4 recordatorio (dato: su índice). |
| `0x1F` Comportamiento | nombre | Empieza ese comportamiento ya (para probarlo desde un editor). `0x07` si no existe (v1.4). |
| `0x40` Decir | `[n][expresión, n bytes (0 = ninguna)][texto UTF-8, 1 a 60 bytes]` | Lo muestra en un **globo** con la cara achicada arriba, con fundido; dura 2,5 s + 60 ms por byte (de 3 a 10 s) o hasta un toque. Con expresión, la pone ese rato; sin, mueve la boca (`hablando`). Si duerme, lo despierta. Si ya hay un globo, primero se va. `0x07` si la expresión no existe; `0x08` si el texto está vacío, es de más de 60 bytes o no es UTF-8 (v1.6). |
| `0x41` Clima | `[i16 temperatura en décimas de °C][u8 código WMO][i8 máx °C][i8 mín °C][u8 flags: bit 0 es de día][n][ciudad, 0 a 24 bytes]` | El tiempo que trae la nube (Open-Meteo) para el modo clima y el despertador. Vale 3 h. `0x08` si está mal armado (v1.6). |
| `0x42` Alarma | `[u8 acción]`: 0 apagar, 1 posponer, 2 sonar ya, 3 despertar | El despertador desde lejos. 2 lo hace sonar (para probarlo); 3 lo despierta sin alarma, con un "buen día". `0x07` si apagar o posponer y no está sonando (v1.6). |

Eventos (el índice del comando `0x12`; los nombres son los de las reglas):

| | | | |
| --- | --- | --- | --- |
| `0` tacto | `1` doble-toque | `2` levantado | `3` sacudida |
| `4` boca-abajo | `5` vinculado | `6` app-abierta | `7` despertado |
| `8` caricia | `9` muchos-toques | `10` sacudida-suave | `11` sacudida-fuerte |
| `12` inclinado | `13` caida | `14` voz | `15` nombre |
| `16` ruido | `17` musica | `18` silencio | `19` notificacion |
| `20` llamada | `21` bateria-baja | `22` bateria-critica | `23` enchufado |
| `24` desenchufado | `25` cargado | `26` sin-wifi | `27` cambio-de-cara |
| `28` bicho-cerca | `29` volvio | `30` hito | `31` recordatorio |
| `32` pomodoro | `33` despertador | | |

Los 8 primeros son de la v1; del 8 en adelante, del motor de vida (v1.4).
`volvio` y `hito` los genera el motor; `recordatorio`, `pomodoro` y
`despertador` (v1.6: empezó a sonar a su hora), el sistema (mandarlos sirve
para probar las reglas, pero no muestran nada). Los eventos nuevos van siempre
al final.

### Eventos que avisa el bicho solo

Cuando se dispara un recordatorio (`31`), termina un tramo del pomodoro
(`32`) o suena el despertador a su hora (`33`), además de anotarlo en el registro, el bicho lo avisa en el momento con
una notificación en Comando, **por BLE y por la nube** (`bicho/{id}/resp`),
con el mismo formato que la respuesta de Registro:

- pomodoro: `[0x9E][0x00][0x01][u32 segundos][0x00 = evento][0x20]`
- despertador: `[0x9E][0x00][0x01][u32 segundos][0x00][0x21]`
- recordatorio: `[0x9E][0x00][0x02][u32 segundos][0x00][0x1F][u32 segundos][0x04 = recordatorio][u8 índice]`

El registro (`0x1E`) guarda las mismas entradas: el tipo 4 es "mostró un
recordatorio" y su dato, la posición en `recordatorios` (desde 0). Quien no
pidió el registro y recibe un `0x9E` lo toma como un aviso.
Además, mientras el recordatorio está en pantalla la expresión visible del
estado es `recordatorio:<n>` (n = su posición en `recordatorios`, desde 0), así
que el estado también se notifica al empezar y al terminar. En modo pomodoro la
expresión visible es `pomodoro:trabajo`, `pomodoro:descanso` o
`pomodoro:pausa`.

### Vida (v1.4)

`[0x9B][0x00][7 × estado][6 × rasgos][flags][n][comportamiento][n][se ve][n candidatos]{[n][nombre][puntaje × 100]}[fps]`

- Estado (0–255 = 0–1): energía, ánimo, afecto, aburrimiento, curiosidad, sueño, estrés.
- Rasgos (0–255, con la deriva): energía, sociabilidad, picardía, timidez, curiosidad, mal genio.
- Flags: bit 0 dormido, 1 soñando, 2 superposición prendida.
- Candidatos: los tres mejores del último sorteo del selector.

### Memoria (v1.4)

112 bytes, little-endian: `[versión = 1]`, u32 nacimiento (epoch local),
u32 última interacción, u32 rencor hasta, u32 último día (epoch/86400),
u16 días juntos, u16 racha, u16 racha máxima, u32 caricias, u32 toques,
u32 sacudidas, 24 × u8 a qué hora aparece el dueño, 8 × i8 gustos (caricia,
tacto, voz, música, nombre, levantado, sacudida suave, app abierta), u16 hitos
(bits: primer día, 7 días, 30 días, 100 caricias, 100 días, 1000 caricias),
u16 año del último cumpleaños del bicho festejado, u16 ídem del dueño,
6 × i8 deriva de los rasgos (centésimos), 7 × u8 estado, u32 epoch del estado,
u16 ausencia máxima (horas), u8 afecto ganado hoy, ceros hasta el byte 108 y el
CRC-32 de los primeros 108 en los últimos 4. Implementaciones:
`codificarMemoria`/`decodificarMemoria` (C++), `leerMemoria`/`codificarMemoria`
(JS), `protocolo.LeerMemoria` (Go).

## Estado

Lectura o notificación (se notifica cuando cambia algo que no sea el humor):

```
[versión = 4][batería u8, 255 = no se sabe][humor i8, -100..100][modo u8]
[flags u8: bit 0 dormido, 1 expresión fija, 2 mostrando el QR, 3 esta conexión autenticada,
           4 WiFi conectado, 5 nube conectada, 6 WiFi configurado, 7 actualizando]
[n u8][expresión visible][n u8][nombre][n u8][número de unidad][n u8][id]
[u16 ancho][u16 alto][forma u8: 0 rectangular, 1 redonda]
[n u8][versión del firmware][n u8][perfil][u32 CRC-32 de la config guardada, 0 = la de fábrica]
[u32 segundos encendido]
```

(Ni el humor ni los segundos encendido disparan notificaciones solos.)

(Textos en UTF-8. Mientras se ve una animación, la expresión visible es
`animacion:<nombre>`; un recordatorio, `recordatorio:<n>`; en modo pomodoro,
`pomodoro:trabajo|descanso|pausa`; mientras suena el despertador,
`despertador`; con un globo (comando Decir o una frase suya), `globo`; en modo
clima, `clima`. La versión 3 no tenía la red, el firmware ni el CRC; la
2 tampoco la pantalla; la 1 tampoco el número de unidad, el id ni los bits 2 y
3.)

Modos: `0` mascota, `1` reloj, `2` pomodoro, `3` clima, `4` notificaciones.

## OTA (v1.3)

Las actualizaciones las firma la API con una clave ECDSA P-256 que solo tiene
el servidor; el firmware trae la clave pública embebida y rechaza todo lo que
no esté firmado con ella. El **manifiesto**:

```
[u32 tamaño][32 bytes SHA-256 del binario][64 bytes firma: r || s][n][versión][n][perfil]
```

La firma es sobre el SHA-256 de este texto (UTF-8, con `\n` al final de cada línea):

```
SINTETICA-OTA-V1
<perfil>
<versión>
<tamaño en decimal>
<SHA-256 en hex minúscula>
```

Así un binario firmado para el broche no se puede instalar en la 1.47B.

| Op | Datos | Qué hace |
| --- | --- | --- |
| `0x30` Inicio | manifiesto | Verifica perfil, firma y tamaño y prepara la otra partición. Responde. |
| `0x31` Bloque | u32 desplazamiento, bytes | Se escribe a la flash. Solo responde si hay error (y aborta). |
| `0x32` Fin | — | Verifica el SHA-256, marca la partición nueva y responde; después reinicia. |
| `0x33` Descargar | manifiesto, `[u16 n][url]` | Con WiFi: la baja él solo por HTTP(S), verifica e instala. Responde si arrancó. |
| `0x35` Cancelar | — | Responde. |

Mientras tanto notifica el progreso: `[0xB4][código][porcentaje][detalle]`. Un
código distinto de 0 es que falló y terminó.

Después de instalar, el firmware nuevo arranca "a prueba": si no llega a
confirmarse (20 s andando y, si tiene WiFi configurado, conectado a la nube),
el bootloader vuelve solo a la versión anterior.

## Nube (v1.3)

Con WiFi, el bicho se conecta al broker MQTT con las credenciales que le pasó
la app (`0x17`): el usuario es su id y la clave es suya (la API la genera al
sumarlo a una cuenta). Tópicos:

| Tópico | Sentido | Contenido |
| --- | --- | --- |
| `bicho/{id}/cmd` | la API → el bicho | `[característica][marco]`: los mismos bytes que por BLE |
| `bicho/{id}/resp` | el bicho → la API | `[característica][respuesta]` |
| `bicho/{id}/state` | el bicho → la API (retenido) | el estado v4 |
| `bicho/{id}/conexion` | el bicho → la API (retenido, con testamento) | `1` conectado, `0` desconectado |

La característica es el número de `Car` del núcleo: 1 Config, 2 Comando,
3 Estado, 4 OTA, 5 Imagen. Por la nube los trozos de config e imagen que manda
el bicho son de hasta 1 KB. El broker solo deja a cada bicho publicar y
suscribirse a sus propios tópicos; por la nube no se puede cambiar el WiFi ni
la nube (`0x16`, `0x17`: responde `0x09`).

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
