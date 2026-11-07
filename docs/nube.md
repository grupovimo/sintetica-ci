# La nube de SintéticaOS (M5 y M6)

Cómo se conectan los bichos, las apps y la web. Para levantarlo en local, ver
[local.md](local.md); el protocolo de bytes está en
[protocolo-ble.md](protocolo-ble.md).

```
            BLE (cerca)                         HTTP + SSE
  App ───────────────────► Bicho          Web / App ◄──────────► API (Go) ◄──► Postgres
   │                        │  ▲                                   │  ▲
   │  POST /v1/bichos/      │  │ MQTT (bicho/{id}/…)               │  │ HTTP (autenticar y autorizar)
   └──── reclamar ────► API │  └──────────────► EMQX ◄─────────────┘  │
                            └──────────────────►  │  ─────────────────┘
```

## Piezas

| Pieza | Dónde | Qué hace |
| --- | --- | --- |
| API | `api/` (Go) | Cuentas, bichos, configs, imágenes, biblioteca, firmwares; habla con los bichos por MQTT y con EMQX por HTTP |
| EMQX | `emqx/emqx.conf` | El broker. No guarda usuarios: le pregunta a la API |
| Postgres | `api/internal/almacen/migraciones` | Todo lo que se guarda (las migraciones se aplican solas al arrancar) |
| Web | `web/` (SvelteKit) | Mis bichos, editores de caras, animaciones y reglas; su servidor hace de proxy a la API |
| Bicho virtual | `api/cmd/bicho-virtual` | El núcleo entero en un contenedor, con MQTT y OTA (para probar sin placa) |
| El núcleo en Go | `firmware/api/cgo` | El mismo C++ por cgo: la API valida las configs con él |

## Cuentas

- **Link mágico**: `POST /v1/sesion/enlace {email, cliente}` manda un mail con
  un link (web: `/entrar/confirmar?t=…`, app: `sintetica://entrar?t=…` a
  través de `/entrar/app`) y un código de 8 caracteres para tipear. Valen 15
  minutos y una sola vez; el código, 5 intentos. Se guardan solo sus hashes.
- `POST /v1/sesion/canjear {token}` o `{email, codigo}` abre la sesión: a la
  web le deja una cookie `HttpOnly` (30 días, se renueva con el uso); a la app
  le devuelve un token (180 días) que va en `Authorization: Bearer`.
- Con cookie, todo lo que cambia algo exige el encabezado `X-Sintetica` (un
  sitio ajeno no lo puede mandar: CSRF).
- Límites: 5 links por email por hora, 20 por IP por hora.
- La cuenta se crea sola la primera vez. `ADMIN_EMAILS` (emails o `@dominio`)
  marca administradores: pueden subir firmwares.

## Sumar un bicho a una cuenta

1. La app ya está vinculada por BLE con el bicho (o sea, lo tiene en la mano:
   leyó el QR y tiene su clave).
2. `POST /v1/bichos/reclamar {id, perfil}` → la API crea una **credencial
   pendiente** (vence en 30 min) y devuelve `{url, usuario: id, clave}`.
3. La app se la pasa al bicho por BLE (comando `0x17`). El bicho se conecta a
   EMQX con esa clave; EMQX le pregunta a la API; la API ve que es la pendiente
   y **recién ahí** el bicho pasa a la cuenta. Si era de otra cuenta, se le
   revocan las claves viejas y se borran los datos del dueño anterior: tener el
   bicho en la mano gana.
4. `GET /v1/bichos/{id}/reclamo?esperar=40` le dice a la app cuándo pasó.

Sacarlo de la cuenta (`DELETE /v1/bichos/{id}`) le pide que olvide el WiFi y la
nube, revoca sus claves y lo echa del broker.

## MQTT

| Tópico | Quién publica | Qué |
| --- | --- | --- |
| `bicho/{id}/cmd` | la API | `[característica][marco]`, los mismos bytes que por BLE (QoS 1) |
| `bicho/{id}/resp` | el bicho | `[característica][respuesta]` (QoS 0) |
| `bicho/{id}/state` | el bicho | el estado v4, retenido (QoS 1) |
| `bicho/{id}/conexion` | el bicho | `1` al conectarse; `0` es su testamento (retenido) |

Cada bicho entra con usuario = client id = su id y su clave; solo puede
publicar en sus tres tópicos y suscribirse a su `cmd` (ACL en
`/emqx/autorizar`). La API entra como superusuario (`MQTT_USUARIO`/`MQTT_CLAVE`).
Los endpoints de EMQX (`/emqx/*`) escuchan en otro puerto (8081) que no se
publica fuera de Docker.

## Sincronización

La config y las imágenes que el dueño quiere están en Postgres (la config, con
todas sus versiones). Un trabajador por bicho (`api/internal/nube`) las
mantiene iguales a las del bicho con **tres vías**: compara lo que tiene el
bicho (el CRC de su config en el estado; sus imágenes con `0x24`), lo que
quiere la API y lo último en lo que estuvieron de acuerdo:

- Iguales: nada.
- Cambió la API (desde la web u otro celu): se lo manda (config en marcos de
  1 KB, imágenes igual) apenas está conectado.
- Cambió el bicho (por BLE, con una app cerca): la API se lo trae (`0x04`,
  `0x25`) y queda como versión nueva (autor "bicho").
- El bicho volvió a fábrica: se le manda la última.

Si el bicho rechaza una config (no debería: la API la valida con el mismo
núcleo), no se reintenta hasta que cambie.

## Clientes lejos del bicho

- **Comandos sueltos**: `POST /v1/bichos/{id}/comando {expresion | nombre | evento | reproducir}`.
- **El protocolo crudo**: `POST /v1/bichos/{id}/marcos {car, datos}` acepta los
  mismos marcos que BLE (así las apps reusan su cliente). Lo que cambia la
  config o las imágenes no va directo: se arma, se valida y se guarda como lo
  deseado (y responde la API); pedir la config o las imágenes lo responde la API
  desde la base; los comandos van al bicho y su respuesta llega por los eventos.
- **Eventos en vivo** (SSE): `GET /v1/eventos` (todos los bichos de la cuenta) o
  `GET /v1/bichos/{id}/eventos`: `estado` (con los bytes crudos en `crudo`),
  `conexion`, `sincro`, `config`, `imagenes`, `ota`, `marco`, `memoria` y `bicho`.

## El motor de vida (v1.4)

- **La hora**: a cada bicho que se conecta (y cada 6 h) la API le manda el
  comando Hora (UTC y la zona de Argentina). Las apps hacen lo mismo por BLE.
- **La memoria** (lo que el bicho recuerda: días juntos, caricias, hitos, a qué
  hora aparecés): la API se la pide cada 5 minutos mientras está conectado y la
  guarda (`memorias`, migración `002_memoria.sql`).
  `GET /v1/bichos/{id}/memoria` la devuelve decodificada (el diario de la web).
- **La mudanza** (cambio de placa): `POST /v1/bichos/{id}/mudar {desde}` copia
  la memoria y la config de otro bicho de la cuenta a este; la memoria queda
  pendiente (`deseada`) hasta que el bicho la confirma con el comando
  Restaurar. Si el bicho pasa a otra cuenta, su memoria guardada en la API se
  borra con el resto de sus datos (la del bicho, no: es suya).
- El relevo de marcos deja pasar Hora, Vida, Memoria, Restaurar, Registro y
  Comportamiento, así la web y las apps lejos del bicho los usan igual que por
  BLE (la respuesta llega por los eventos).

## OTA

1. Un administrador sube el `firmware.bin` de un perfil y una versión
   (`POST /v1/firmwares`). La API arma el manifiesto y lo **firma** con su
   clave privada (ECDSA P-256; `claves/ota-privada.pem`, generada por
   `npm run claves` o el servicio `claves`).
2. Con WiFi: `POST /v1/bichos/{id}/actualizar {firmware}` le manda al bicho el
   manifiesto y un link firmado (HMAC, una hora). El bicho verifica la firma
   con la clave pública que trae embebida **antes** de bajar nada, baja,
   verifica el SHA-256, instala en la otra partición y reinicia.
3. Sin WiFi (el broche): la app baja el manifiesto y el binario de la API y se
   los manda por BLE (`0x30`–`0x32`).
4. El firmware nuevo arranca a prueba: se confirma solo a los 20 s si anda
   (y, si tiene WiFi configurado, si se conectó a la nube). Si no, a los 2 min
   (o si se cuelga antes) el bootloader vuelve a la versión anterior. La API lo
   sigue con el estado (`encendido` le dice que reinició): `instalada` →
   `confirmada` (30 s andando con la versión nueva) o `fallida` (volvió con la
   anterior).

## Variables de la API

| Variable | Para qué | En local |
| --- | --- | --- |
| `DATABASE_URL` | Postgres | el del compose |
| `MQTT_URL`, `MQTT_USUARIO`, `MQTT_CLAVE` | la conexión de la API al broker | `tcp://emqx:1883`, `api`, `MQTT_CLAVE_API` |
| `MQTT_URL_PUBLICA` | la que se les da a los bichos | `mqtt://IP_LOCAL:1883` |
| `MQTT_URL_INTERNA` | la que se le da al bicho virtual | `mqtt://emqx:1883` |
| `API_URL_PUBLICA`, `API_URL_INTERNA` | de dónde bajan las OTA (placa y virtual) | `http://IP_LOCAL:8080`, `http://api:8080` |
| `WEB_URL` | para los links de los mails | `http://localhost:5173` |
| `SMTP_SERVIDOR`, `SMTP_USUARIO`, `SMTP_CLAVE`, `SMTP_DE` | el correo | Mailpit, sin usuario |
| `CLAVE_FIRMA` | la clave privada de las OTA | `/claves/ota-privada.pem` |
| `SECRETO` | firma los links de descarga | uno local |
| `ADMIN_EMAILS` | quién sube firmwares | vacío |
| `DESARROLLO` | bichos virtuales y los links en el log | `1` |
| `BICHOS_VIRTUALES` | dónde están los virtuales | los dos del compose |
| `EMQX_API_URL`, `EMQX_API_CLAVE` | para echar a un bicho | `http://emqx:18083`, la de `emqx/api-claves.txt` |
| `COOKIE_SEGURA` | cookie solo por HTTPS | `0` |

## Antes de ponerlo en el VPS

Esto está pensado para correr en local. Para producción falta (y está
anotado acá a propósito):

- **TLS** en todo: la web y la API detrás de un proxy con HTTPS
  (`COOKIE_SEGURA=1`), y MQTT por `mqtts://` (EMQX con su certificado; el
  firmware ya acepta `mqtts://` con los certificados raíz de ESP-IDF).
- Cambiar **todas** las claves: `MQTT_CLAVE_API`, `SECRETO`, la de Postgres,
  el panel de EMQX y la de su API REST (`emqx/api-claves.txt`).
- Un SMTP de verdad (con SPF y DKIM para el dominio).
- Guardar `claves/ota-privada.pem` fuera del servidor de la web, con copia
  segura: perderla obliga a reflashear por USB todos los bichos.
- Sacar `DESARROLLO=1`, los bichos virtuales y, en las apps, el permiso de
  HTTP sin cifrar (`res/xml/red.xml` en Android, `NSAllowsLocalNetworking` en iOS).
- El cifrado del enlace BLE y secure boot quedan para M8.
