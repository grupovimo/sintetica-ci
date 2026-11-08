# La nube de SintéticaOS (M5 y M6)

Cómo se conectan los bichos, las apps y la web. Para levantarlo en local, ver
[local.md](local.md); el protocolo de bytes está en
[protocolo-ble.md](protocolo-ble.md).

```
            BLE (cerca)                         HTTP + SSE
  App ───────────────────► Bicho          Web / App ◄──────────► API (Go) ◄──► Postgres
   │                        │  ▲                                ┌──────────┐
   │  POST /v1/bichos/      │  │ MQTT (bicho/{id}/…)            │  broker  │ (Mochi, en el
   └──── reclamar ────► API │  └──────────────────────────────► │  MQTT    │  mismo proceso)
                            └───────────────────────────────►   └──────────┘
```

## Piezas

| Pieza | Dónde | Qué hace |
| --- | --- | --- |
| API | `api/` (Go) | Cuentas, bichos, configs, imágenes, biblioteca, firmwares; habla con los bichos por MQTT |
| Broker MQTT | `api/internal/broker` | Mochi MQTT embebido en la API: tcp (:1883) y TLS (:8883). No guarda usuarios: le pregunta a la API, en el mismo proceso |
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
3. La app se la pasa al bicho por BLE (comando `0x17`). El bicho se conecta al
   broker con esa clave; el broker le pregunta a la API; la API ve que es la pendiente
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
| `bicho/{id}/conexion` | el bicho | `1` al conectarse; `0` es su testamento (retenido); el broker también publica el `0` cuando se va con DISCONNECT o lo echan |

Cada bicho entra con usuario = client id = su id y su clave; solo puede
publicar en sus tres tópicos y suscribirse a su `cmd`
(`servidor.AutorizarMQTT`; lo que no está, no). La API publica y se suscribe
en el mismo proceso; `MQTT_USUARIO`/`MQTT_CLAVE` es un superusuario para mirar
de afuera (`mosquitto_sub -u api -P … -t 'bicho/#'`).

### El broker

[Mochi MQTT](https://github.com/mochi-mqtt/server) embebido en la API
(`api/internal/broker`). Antes era EMQX aparte; se cambió porque Erlang comía
un núcleo y medio sin tráfico en la Mac y 384–512 MB en el VPS. Lo que hacía
EMQX y cómo se hace ahora:

- **MQTT 3.1.1 y 5**, QoS 0, 1 y 2, retenidos, keepalive, sesiones limpias y
  persistentes (las persistentes se borran a las 2 h sin volver, como el
  `session_expiry_interval` de EMQX), paquetes de hasta **1 MB**.
- **Autenticación y autorización**: ganchos de Mochi que llaman a
  `servidor.AutenticarMQTT` y `servidor.AutorizarMQTT` (la misma lógica que
  tenían `/emqx/autenticar` y `/emqx/autorizar`, sin HTTP). Autenticar tiene 5 s
  (el `request_timeout` de EMQX). La autorización no tiene caché (la de EMQX
  era de 1 min): es comparar textos. Publicar donde no se puede se ignora con
  su PUBACK (el `deny_action = ignore`); suscribirse, SUBACK `0x80`. La última
  voluntad de un bicho también se revisa (no puede ser en el tópico de otro).
- **Puertos**: `MQTT_TCP` (sin TLS, `:1883`; en producción solo
  `127.0.0.1` adentro del contenedor) con tope `MQTT_TCP_MAX` (64) y
  `MQTT_TLS` (`:8883`, TLS 1.2 y 1.3) con tope `MQTT_TLS_MAX` (2000). Pasado el
  tope, la conexión se corta en el acto; el que no manda su CONNECT en 15 s,
  también.
- **El certificado** (`MQTT_TLS_CERT`, `MQTT_TLS_CLAVE`) se relee solo: en
  cada handshake se mira la fecha y el tamaño de los archivos y, si cambiaron,
  se vuelve a cargar (si el nuevo está roto, sigue el anterior). Renovarlo no
  necesita reiniciar nada.
- **Echar a un bicho** (cuando sale de la cuenta) es una llamada directa al
  broker (antes, la API REST de EMQX).
- **El estado de conexión**: el broker publica el `0` retenido de un bicho
  apenas se va, aunque sea con DISCONNECT (sin testamento) o echado; si el
  bicho se reconecta antes de que se note el corte (misma client id), la
  sesión vieja no avisa nada (su `0` llegaría después del `1` nuevo).
- **Sin persistencia**: el broker arranca y se apaga con la API, así que los
  bichos se reconectan solos en cada reinicio (reintentan cada 5 s) y vuelven
  a publicar su `conexion` y su `state` retenidos; un retenido de antes del
  reinicio estaría viejo. Las colas de QoS 1 no hacen falta: la API solo manda
  comandos a bichos conectados. Al arrancar, la API anota quiénes figuraban
  conectados: su vuelta en el primer minuto no dispara la automatización
  "conectado", y los que no vuelven pasan a desconectados (lo que antes
  hacía el `0` retenido de EMQX). Al apagarse, la API no anota desconexiones.
- Lo que no está: el panel de EMQX (para mirar, el superusuario de arriba), el
  cluster y el WebSocket (`:8083`, que no usaba nadie).

## Sincronización

La config y las imágenes que el dueño quiere están en Postgres (la config, con
todas sus versiones). Un trabajador por bicho (`api/internal/nube`) las
mantiene iguales a las del bicho con **tres vías**: compara lo que tiene el
bicho (el CRC de su config en el estado; sus imágenes con `0x24` y sus
sprites con `0x26`, si el firmware los tiene), lo que
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
  `conexion`, `sincro`, `config`, `imagenes`, `sprites`, `ota`, `marco`, `memoria`, `bicho`,
  `automatizacion`, `telegram` (ver "Integraciones"), `mcp` (ver "MCP") y,
  de la v1.9, `toque` y `entrante` (ver "Amigos, presencia, hábitos y avisos
  entrantes").

## Sprites de las caras de imagen (v1.5)

Las caras de imagen (ver [motor-de-vida.md](motor-de-vida.md#caras-de-imagen))
usan hasta 12 sprites por robot, aparte de las 8 bahías de imágenes. En
Postgres van en la misma tabla `imagenes`, en los espacios 8 a 19 (el sprite
`n` es el espacio `8 + n`, como en el protocolo), y se sincronizan igual.

- `GET /v1/bichos/{id}/sprites`: `{sprites: [{sprite, largo, crc}], max: 12, sincro}`.
- `GET /v1/bichos/{id}/sprites/{n}`: el QOI (con `?png=1`, en PNG para mostrarlo).
- `PUT /v1/bichos/{id}/sprites/{n}`: el cuerpo es un **SVG**, PNG, JPEG o GIF
  (el primer cuadro) o un QOI ya listo. La API (`api/internal/imagenes/sprites.go`)
  rasteriza el SVG en Go puro (`oksvg` + `rasterx`), lo recorta a lo dibujado
  (lo no transparente; en una imagen sin transparencia, lo que no es casi
  negro) y lo achica a 128 × 128 como mucho, con la transparencia
  premultiplicada (bordes sin halo), en QOI de 4 canales. Responde
  `{sprite, largo, crc, ancho, alto}`.
- `DELETE /v1/bichos/{id}/sprites/{n}`.
- `GET /v1/bichos/{id}/imagenes` sigue mostrando solo las bahías 0 a 7.

Cada cambio sale por el evento `sprites` (la web vuelve a pedir la lista y la
pantalla en vivo los carga en su núcleo).

## El motor de vida (v1.4)

- **La hora**: a cada bicho que se conecta (y cada 6 h) la API le manda el
  comando Hora (UTC y la zona de Argentina). Las apps hacen lo mismo por BLE.
- **La memoria** (lo que el bicho recuerda: días juntos, caricias, hitos, a qué
  hora aparecés): la API se la pide cada 5 minutos mientras está conectado y la
  guarda (`memorias`, migración `002_memoria.sql`).
  `GET /v1/bichos/{id}/memoria` la devuelve decodificada (el diario de la web).
  Desde la v1.7 suma `juegosGanados`, `juegosJugados`, `mejorReflejo` (ms, 0
  = nunca), `juegosHoy`, `suenosHoy`, `sonoConHoy` (`corazon`, `estrella`,
  `nota`, `oveja`, `pesadilla`), `animoHoy` (0 sin respuesta, 1 bien, 2 más o
  menos, 3 mal), `esperoHoy` (la hora a la que te esperó, o `null`),
  `diaContadores`, `anioFechas`, `fechas` (`anio-nuevo`, `navidad`,
  `dia-del-amigo`, `dia-del-nino`, `aniversario`) y `aguaHoy`. Lo "de hoy" de
  una memoria de otro día (el día de los contadores no es hoy en la zona de
  los bichos) sale en 0, y las fechas de otro año, vacías.
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

## Integraciones

Lo que hace que el bicho sirva fuera de su pantalla (`api/internal/integraciones`,
migración `005_integraciones.sql`): el bot de Telegram, las automatizaciones
por bicho (y las rutinas a una hora), los recordatorios que la nube manda
cuando el bicho no está, los cumpleaños, el clima (`api/internal/clima`), que
el robot diga algo o lo despierten desde afuera (protocolo v1.6) y lo de un
bicho que acompaña el día (v1.7: el "¿Cómo estás?", el "te extraño", su día
contado, los juegos y el saludo entre robots).

### Telegram (@sinteticalabs_bot)

- **Long polling** (`getUpdates`, 30 s), no webhook entrante: anda igual en
  local y en el VPS sin exponer nada. Un solo trabajador en la API, con backoff
  (1 s → 1 min, o lo que diga `retry_after`); se apaga con el contexto. El
  offset se guarda en `telegram_offset` (un reinicio no repite comandos).
  Solo `net/http`; la URL base se puede cambiar (`TELEGRAM_API_URL`, para los
  tests). El token va en el camino de cada pedido y nunca se loguea (los errores
  de red se limpian de la URL).
- **Sin `TELEGRAM_TOKEN`** todo lo de Telegram queda apagado: las rutas
  responden `{"activo": false}` y el resto de la API anda igual.
- **Vincular**: `POST /v1/telegram/vincular` → `{"enlace":"https://t.me/sinteticalabs_bot?start=<código>","vence":"ISO"}`
  (un solo uso, 15 min, se guarda solo el hash; 10 por hora). Al recibir
  `/start <código>`, el bot guarda el chat para ese usuario, saluda y emite el
  evento `telegram` (`{vinculado: true, nombre}`). Un chat es de un solo
  usuario: vincularlo en otra cuenta lo pasa a esa.
  `GET /v1/telegram` → `{"activo","bot","vinculado","nombre"}` (`nombre`: el
  `@usuario` o el nombre de Telegram). `DELETE /v1/telegram` desvincula
  (evento `telegram` con `vinculado: false`) y responde como el GET.
- **Comandos** (solo chats privados y vinculados; a cualquier otro chat le
  explica cómo vincularse; 30 mensajes por minuto por chat):
  `/robots` (cada uno con conectado, batería, cara y ánimo, y botones para ver
  su estado), `/estado [nombre]`, `/cara <expresión> [nombre]` (el mismo camino
  que `POST /v1/bichos/{id}/comando {expresion}`; sin expresión, un teclado
  inline con las caras; `auto` vuelve a la automática), `/decir <texto> [nombre]`
  (el robot lo dice en un globo en su pantalla: el marco `0x40`, hasta 60 bytes;
  si es más largo, el bot pide acortarlo), `/clima [nombre]` (el clima donde
  está el robot, si tiene `ubicacion`: "En Buenos Aires: 18° y nublado (máx.
  22°, mín. 12°)."), `/jugar [dado|moneda|ppt|reflejos|mirada] [nombre]` (el
  marco `0x43`; sin juego, un teclado con los cinco), `/ayuda`. Con un solo robot el nombre no hace falta; con varios, si no se
  dice, aparece un teclado para elegirlo. El nombre es el apodo (o el nombre
  del bicho, o su id), sin importar mayúsculas ni tildes.
### Automatizaciones

"Cuando pasa X en el bicho → hacer Y". Rutas (el dueño):

- `GET /v1/bichos/{id}/automatizaciones` → `{"automatizaciones":[…],"historial":[…],"cuandos":[…]}`
  (`historial`: las últimas 50 ejecuciones, la más nueva primero:
  `{automatizacion, evento, accion, ok, detalle, en}`; `cuandos`: los eventos
  que se pueden usar).
- `PUT /v1/bichos/{id}/automatizaciones {"automatizaciones":[…]}` reemplaza la
  lista entera (máximo 20; inválida → 400 con el motivo en `detalle`).
- `POST /v1/bichos/{id}/automatizaciones/{n}/probar` la corre ya, como si
  hubiera pasado su evento (`{n}`: la posición desde 0 o el id). Responde
  `{"ok", "resultados":[{tipo, ok, detalle}]}`; 429 si corrió hace muy poco.

```json
{"automatizaciones":[
 {"id":"a1","activa":true,"cuando":"bateria-baja","hacer":[
   {"tipo":"telegram","texto":"{robot} tiene {bateria}% de batería"},
   {"tipo":"webhook","url":"https://ejemplo.com/gancho"},
   {"tipo":"robot","bicho":"<id de otro robot propio>","expresion":"sorpresa"}
 ]}
]}
```

- `id`: 1 a 40 de `[A-Za-z0-9_-]`, sin repetir. `hacer`: de 1 a 5 acciones.
  `valor` (opcional) solo con `"cuando": "expresion"`: cuál. `hora` (HH:MM,
  obligatoria) y `dias` (letras `LMXJVSD` sin repetir; vacío = todos) solo con
  `"cuando": "hora"` (ver "Rutinas").
- **Plantillas** en los textos (`telegram` y `decir`): `{robot}` (apodo, nombre
  o id), `{bateria}`, `{expresion}`, `{evento}`, `{texto}` (el del
  recordatorio), `{hora}` (HH:MM, hora del bicho), `{dueno}` (`dueno.nombre`
  de la config; vacío si no hay), `{ciudad}` (`ubicacion.ciudad`) y las del
  clima del lugar del robot: `{clima}` ("18° y nublado"), `{temp}` ("18°"),
  `{max}`, `{min}` (sin `ubicacion` o si Open-Meteo no contesta: "sin datos
  del clima" y "?"). El clima solo se pide si el texto lo usa (con el caché de
  30 min). Con `cuando: juego`, `{juego}` (`ppt`, `reflejos`, `mirada`,
  `dado`, `moneda`) y `{resultado}` (`gano`, `perdio`, `empate`,
  `sin-resultado`); con `cuando: animo`, `{animo}` (`bien`, `medio`, `mal`).
- **`telegram`**: le escribe al chat vinculado del dueño (texto de 1 a 1000 bytes).
- **`robot`**: solo a bichos de la misma cuenta (se valida al guardar y al
  correr); le pone esa expresión (el mismo comando que la web).
- **`decir`** `{"tipo":"decir","texto":"Buen día, {dueno}: {clima}","expresion"?}`:
  este robot lo dice en un globo (marco `0x40`). El texto es de 1 a 200 bytes
  con plantillas; ya expandido se corta a 60 bytes sin partir una letra.
- **`cara`** `{"tipo":"cara","expresion":"contento"}`: le pone esa cara a este
  robot (`auto` vuelve a la automática).
- **`modo`** `{"tipo":"modo","modo":"reloj"}` (`mascota`, `reloj`, `pomodoro`,
  `clima`, `notificaciones`): cambia `modo` en la config por el mismo camino que
  `PUT /v1/bichos/{id}/config` (validada por el núcleo, versión nueva con autor
  `automatizacion`, migración `008`) y se sincroniza como siempre.
- **`despertar`** `{"tipo":"despertar"}`: el marco `0x42` acción 3 (lo
  despierta con un "¡Buen día!", sin alarma).
- **`jugar`** `{"tipo":"jugar","juego":"dado"}` (`ppt`, `reflejos`, `mirada`,
  `dado`, `moneda`): el marco `0x43`, lo invita a jugar.
- `decir`, `cara`, `despertar` y `jugar` necesitan que el robot esté conectado (si no,
  la acción queda en el historial con `ok: false`).
- **`webhook`**: `POST` JSON `{"evento","robot":{"id","nombre"},"datos":{…},"en":"ISO"}`
  con `X-Sintetica-Firma: sha256=<HMAC-SHA256 del cuerpo en hex>` (y
  `X-Sintetica-Evento`). El secreto es por usuario: `GET /v1/webhooks/secreto`
  → `{"secreto":"whsec_…"}` (se crea la primera vez), `POST` lo rota. Solo
  `http`/`https`, sin usuario ni clave en la URL; 5 s de timeout; no sigue
  redirecciones ni usa proxies. **Destinos internos bloqueados** (salvo
  `DESARROLLO=1`): loopback, privadas, link-local (169.254.169.254 incluida),
  CGNAT, multicast, reservadas, de documentación, IPv6 con una IPv4 adentro
  (NAT64, 6to4), nombres sin punto (los de Docker: `api`, `postgres`…),
  `localhost`, `.local`, `.internal`, `.lan`, y el propio servicio (los hosts de
  `WEB_URL`, `API_URL_PUBLICA`, `API_URL_INTERNA` y las IPs a las que
  resuelven). El DNS se resuelve al conectar, se validan **todas** las IPs y la
  conexión va a la IP validada (contra DNS rebinding).
- **Límite de disparos**: cada automatización corre como mucho una vez cada
  10 s y 200 veces por día (en memoria; `probar` cuenta). Así no sirve de cañón
  ni se arman bucles entre robots.
- Cada vez que una corre: entradas en el historial (una por acción) y el
  evento `automatizacion` por SSE: `{id, cuando, evento, resultados, en}`.
- Si el bicho pasa a otra cuenta, sus automatizaciones y su historial se borran.

**Los `cuando` que se detectan de verdad:**

| `cuando` | De dónde sale |
| --- | --- |
| `conectado`, `desconectado` | `bicho/{id}/conexion` (no los retenidos que llegan cuando arranca la API) |
| `bateria-baja` | el estado: la batería cruza de ≥ 15 % a < 15 % |
| `expresion` (opcional `valor`) | el estado: cambió la cara visible (`datos.valor`, `datos.antes`) |
| `duerme`, `despierta` | el estado: el bit de dormido |
| `tacto`, `doble-toque`, `caricia`, `muchos-toques`, `levantado`, `sacudida`, `sacudida-suave`, `sacudida-fuerte`, `boca-abajo`, `inclinado`, `caida`, `voz`, `nombre`, `ruido`, `musica`, `cargando` (= `enchufado`), `desenchufado`, `cargado`, `volvio`, `bicho-cerca` | el registro del motor de vida (`0x1E`, tipo 0) |
| `hito` | el registro (tipo 3) |
| `recordatorio` | el aviso que el bicho manda solo al mostrar uno (ver abajo) |
| `pomodoro` | el aviso que el bicho manda solo al terminar un tramo del pomodoro |
| `despertador` | el aviso que el bicho manda solo al empezar a sonar la alarma (evento `33`: `[0x9E][0x00][0x01][u32 s][0x00][0x21]`) |
| `hora` (con `hora` y `dias`) | el reloj de la nube, cada minuto (ver "Rutinas") |
| `juego` | v1.7: el aviso que el bicho manda solo al terminar una partida (evento `34` + registro **tipo 5**, dato juego·16 + resultado: `datos.juego`, `datos.resultado`) |
| `animo` | v1.7: el aviso que el bicho manda solo cuando le contestan el "¿Cómo estás?" (evento `35` + registro **tipo 6**, dato 0 bien, 1 más o menos, 2 mal: `datos.valor`) |

Los del registro: la API le pide el registro (`0x1E`) cada 10 s **solo** a los
bichos conectados que tienen alguna automatización activa que lo necesita (o
cuyo dueño tiene otro robot conectado: para el saludo entre robots), y
avisa cada entrada nueva una vez (la primera lectura después de conectarse o
de un reinicio solo marca el punto de partida). Los avisos (un `0x9E` que el
bicho manda solo por `bicho/{id}/resp`, ver protocolo-ble.md, "Eventos que avisa
el bicho solo") se atienden siempre, sin pedir el registro, y no se repiten
cuando después aparecen en él. Si en 10 s pasan más de 28
cosas, las más viejas se pierden. `bateria-baja` del registro no se usa (sale
del estado, con el umbral de arriba).

### Recordatorios

- **El evento que espera la API**: el aviso `[0x9E][0x00][0x02][u32 s][0x00][0x1F][u32 s][0x04][u8 índice]`
  que el bicho manda solo al mostrar un recordatorio (evento `31` recordatorio +
  entrada de registro de **tipo 4** con su índice en `recordatorios`, desde 0).
  Cuenta como un solo `recordatorio` (sale de la entrada de tipo 4). Si el
  aviso se perdiera y la API está leyendo el registro, la entrada de tipo 4
  también lo dispara. El texto se busca en la config (`{texto}`; en el
  webhook, `datos.texto`, `datos.hora` y `datos.indice`).
- **Si el bicho no está conectado**: cada minuto la nube revisa los
  recordatorios activos de la config (`recordatorios`, arriba de todo o en
  `vida`: `[{hora:"08:30", dias:"LMXJV", texto, expresion, sonido, activo}]`;
  días L M X J V S D, vacío = todos) de los bichos desconectados de los
  usuarios con Telegram, y los manda por Telegram ("Recordatorio de Pipo
  (08:30): …"); quedan en el historial como `recordatorio-nube`. La hora es la
  de los bichos (`America/Argentina/Buenos_Aires`, la misma con la que la API
  les manda la hora). Si la API estuvo parada, se pone al día como mucho 3
  minutos. Ojo: un bicho que nunca se conecta a la nube (un broche sin WiFi)
  cuenta como desconectado.

### Rutinas (`cuando: hora`)

Una automatización con `"cuando": "hora"`, `"hora": "07:30"` y `"dias": "LMXJV"`
corre todos esos días a esa hora (la de los bichos: `nube.Zona()`,
`America/Argentina/Buenos_Aires`). La nube las revisa cada minuto, **ande o
no Telegram** (busca en Postgres los bichos con alguna activa:
`lista @> '[{"cuando":"hora","activa":true}]'`) y las encola como un evento
`hora` (`datos.hora`). Si la API estuvo parada, se pone al día como mucho 3
minutos. El límite de disparos es el mismo de las demás.

```json
{"id":"buen-dia","activa":true,"cuando":"hora","hora":"07:30","dias":"LMXJV","hacer":[
  {"tipo":"despertar"},
  {"tipo":"decir","texto":"Buen día, {dueno}: {clima}"},
  {"tipo":"modo","modo":"clima"}
]}
```

### Cumpleaños

Sin armar nada: el día de `dueno.cumple` ("MM-DD"), a la hora de
`horario.despierta` (08:00 si no hay) en la zona de los bichos, si el dueño
vinculó Telegram, el bot le escribe "🎂 ¡Feliz cumple, {dueno}! Te saluda
{robot}." (sin `dueno.nombre`: "🎂 ¡Feliz cumple! Te saluda {robot}."). Igual con
`cumple` (el del robot): "🎉 Hoy {robot} cumple años." Va **una vez por año**:
queda en el historial como `cumple-dueno` / `cumple-robot` (evento `cumple`,
acción `telegram`) y no se manda si ya hay uno desde el 1 de enero. El 29/2,
en los años sin, se festeja el 28. Lo revisa el mismo trabajador de los
recordatorios (cada minuto, solo con Telegram prendido), con el robot
conectado o no.

### Decir, despertador y clima (protocolo v1.6)

Rutas (el dueño; mismas reglas de sesión y `X-Sintetica` que las demás):

- `POST /v1/bichos/{id}/decir {"texto": "...", "expresion": "contento"?}` →
  `{"ok": true}`: el marco `0x40`, el robot lo muestra en un globo (2,5 s + 60 ms
  por byte, entre 3 y 10 s, o hasta un toque). 400 con `detalle` si el texto no
  es de 1 a 60 bytes de UTF-8 (o la expresión no es `[a-z][a-z0-9-]`, hasta 24);
  409 si el robot no está conectado; 30 por minuto por robot.
- `POST /v1/bichos/{id}/alarma {"accion": "apagar"|"posponer"|"probar"|"despertar"}`
  → `{"ok": true}`: el marco `0x42` con 0/1/2/3. 409 si no está conectado.
- `GET /v1/clima/buscar?q=córdoba` (con sesión) →
  `{"lugares":[{"nombre":"Córdoba","region":"Córdoba","pais":"Argentina","lat":-31.41,"lon":-64.18,"ciudad":"Córdoba, Argentina"}]}`:
  hasta 6, del geocoding de Open-Meteo en español (`lat`/`lon` redondeados a
  0,01; `ciudad`, lo que va en `ubicacion.ciudad`, hasta 40 bytes). `q` de 2
  a 60 bytes (si no, 400); 502 si Open-Meteo no contesta; 30 por minuto por usuario.
- `GET /v1/bichos/{id}/clima` → `{"ubicacion": {lat, lon, ciudad} | null,
  "clima": {"temp": 18.4, "codigo": 3, "descripcion": "nublado", "max": 22, "min": 12, "dia": true, "en": "ISO"} | null}`
  (`en`: cuándo se trajo; `clima` null sin ubicación o si Open-Meteo falla).

**El clima** (`api/internal/clima`): Open-Meteo, sin clave.
Pronóstico: `current=temperature_2m,weather_code,is_day&daily=temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=1`.
Timeout de 5 s; caché en memoria por lugar (lat/lon redondeados a 0,01) de 30
min; si falla y hay uno de menos de 3 h, se usa ese. Las descripciones salen
del código WMO: despejado, casi despejado, algo nublado, nublado, niebla,
llovizna, lluvia, lluvia fuerte, chaparrones, nieve, tormenta, tormenta con
granizo… Las URLs se pueden cambiar (`CLIMA_API_URL`, `GEOCODING_API_URL`,
para los tests).

**El trabajador del clima**: cada minuto mira los robots con dueño que están
conectados y tienen `ubicacion` en su última config; a cada uno le manda el
marco `0x41` (`[i16 décimas de °C][u8 WMO][i8 máx][i8 mín][u8 flags: bit 0 =
de día][n][ciudad, hasta 24 bytes]`) si nunca se lo mandó, si pasaron 30 min,
si se acaba de conectar (también apenas llega el evento `conectado`) o si
cambió la ubicación (se nota en la vuelta siguiente; el MCP, al ubicar, lo
manda ya). El robot lo da por bueno 3 h.

### Un bicho que acompaña el día (protocolo v1.7)

Código: `api/internal/integraciones/cuidado.go` (y `servidor/cuidado.go`,
`servidor/mcp_cuidado.go`); los marcos, en `api/internal/protocolo/cuidado.go`.
Todo lo de Telegram lo revisa el trabajador de los recordatorios (cada minuto,
solo con Telegram prendido y para los dueños que lo vincularon), en la zona de
los bichos (`nube.Zona()`), con la **última memoria** que mandó el bicho (sin
memoria no se manda nada) y respetando `cuidado` de su última config (por
defecto: `animo` y `extranar` prendidos, `extranarDias` 2, `resumen`
apagado; `horario` de 08:00 a 23:30). Para no repetir, cada envío queda en el
historial de automatizaciones (con el nombre de abajo) y se mira ahí antes de
mandar.

- **"¿Cómo estás?"**: con `cuidado.animo`, si la memoria dice que hoy no
  contestó (`animoHoy == 0`) y pasaron 2 h desde `horario.despierta` (y
  todavía no es `horario.duerme`), una vez por día: "{robot} pregunta: ¿Cómo
  estás hoy?" con tres botones (😀 Bien / 😐 Más o menos / 😞 Mal; datos
  `animo:<id>:0|1|2`). Historial `como-estas`. Al tocar uno, se sacan los
  botones, se le manda al bicho `0x45` con la respuesta (historial `animo`,
  acción `robot`) y el bot contesta con empatía ("¡Qué bueno! {robot} se puso
  contento de saberlo 😊", "Bueno, hay días así. {robot} te manda un abrazo
  🤗", "Uh, lo siento mucho. {robot} está con vos 💛" y, con "mal", una frase
  linda). Si el robot no está conectado, lo dice.
- **Te extraño**: con `cuidado.extranar`, si la última interacción de la
  memoria fue hace `extranarDias` días o más (y menos de 30: después se deja de
  insistir), en horario despierto, "Te extraño 🥺 — {robot}", como mucho una
  vez cada `extranarDias` días (historial `te-extrano`).
- **Su día contado**: con `cuidado.resumen`, a la hora de `horario.duerme`
  (si duerme antes de las 06:00, a las 23:59), si la memoria es de hoy, un
  Telegram con lo de hoy: "Hoy {robot} jugó 3 veces (ganó 2), soñó con ovejas,
  te esperó a las 19, te recordó tomar agua 4 veces y le dijiste que estabas
  bien." (sin nada: "Hoy {robot} tuvo un día tranquilo."). Una vez por día
  (historial `resumen`). Cuántas ganó no está en la memoria: la nube cuenta
  los avisos `juego` del día (en memoria; si se reinició y no las vio todas,
  no lo dice).
- `GET /v1/bichos/{id}/resumen` → `{"texto", "juegos", "ganados" (null si no
  se sabe), "suenos", "sonoCon": [...], "animo": "bien"|"medio"|"mal"|null,
  "espero": 19|null, "agua", "actualizada": ISO|null}` para la consola (Vida,
  el diario). Sin memoria: `texto` lo dice y lo demás en cero.
- **Entre robots**: cuando un robot avisa `bicho-cerca` (registro) y el dueño
  tiene otro robot conectado, los dos hacen el comportamiento `saludar`
  (`0x1F`) y dicen "¡Hola, {otro}!" (`0x40`, cara `contento`), como mucho una
  vez cada 10 min por par (en memoria y en el historial: `entre-<id del
  otro>`, evento `bicho-cerca`, acción `saludar`).
- `POST /v1/bichos/{id}/jugar {"juego": "dado"|"moneda"|"ppt"|"reflejos"|"mirada"|"terminar"}`
  → `{"ok": true}`: el marco `0x43` (0 ppt, 1 reflejos, 2 mirada, 3 dado, 4
  moneda, 255 terminar). 400 si no existe; 409 si no está conectado; 30 por
  minuto por robot.
- `POST /v1/bichos/{id}/aviso {"tipo": "notificacion"|"llamada", "app"?, "contacto"?}`
  → `{"ok": true}`: el marco `0x44` (`[u8 tipo][n][app][n][contacto]`; la app
  y el contacto se cortan a 24 bytes sin partir una letra). El robot reacciona
  según `avisos` de su config. 409 si no está conectado; 60 por minuto.
- Los marcos `0x45` Ánimo (`[u8 0 bien, 1 más o menos, 2 mal]`) y `0x46` Mirar
  (`[i8 x][i8 y]`, −100 a 100) están en `protocolo` (`Animo`, `Mirar`); el
  primero lo usa el bot, el segundo es para los modelos con dos micrófonos.

### Amigos, presencia, hábitos y avisos entrantes (protocolo v1.9)

Código: `api/internal/integraciones/{amigos,presencia,habitos}.go` (la lógica,
el bot y lo que llega de los robots), `api/internal/servidor/amigos.go` (las
rutas, con una sola línea en el router: `rutasCompania`),
`servidor/entrantes.go` y `servidor/mcp_compania.go`; las tablas, en las
migraciones `009_amigos.sql` y `010_presencia_habitos.sql`; los marcos, en
`api/internal/protocolo/compania.go`.

**Amigos y toques.** Los toques van solo entre amigos aceptados y de un amigo
se ve solo su nombre (el que puso en su cuenta; si no, "Tu amigo"): ni su
email ni los ids de sus robots.

- `POST /v1/amigos/invitacion` → `{codigo, link, telegram?, vence}`: 8 letras
  sin I ni O (24 posibles: ~10¹¹ códigos), 48 h, un solo uso; en la base solo
  el hash. `link` es `WEB_URL/robots?amigo=CODIGO`; `telegram`,
  `https://t.me/<bot>?start=amigo-CODIGO` (el bot lo acepta si el chat está
  vinculado y le avisa al que invitó). Hasta 20 por día por usuario.
- `POST /v1/amigos {codigo}` acepta (sin importar mayúsculas, espacios ni
  guiones) → el amigo. 400 si no existe, venció, se usó o es propio; 409 si ya
  son amigos (no gasta el código). 10 intentos por hora por usuario y 30 por
  IP.
- `GET /v1/amigos` → `{amigos: [{id, nombre, robots, todos, conectado, creada}]}`:
  `robots` son los míos que reciben sus toques, `todos` si es "todos los
  míos" (también los que sume después) y `conectado` si tiene algún robot
  conectado que reciba los míos.
- `PUT /v1/amigos/{id} {robots: [ids míos] | null}` (null = todos; `[]` =
  ninguno). `DELETE /v1/amigos/{id}`: dejan de ser amigos los dos. Un id que
  no es tuyo es 404.
- `POST /v1/amigos/{id}/toque {tipo: toque|abrazo|beso|carita, expresion?, desde?}`
  → `{robots, telegram}`: el `0x47` a sus robots conectados que reciben mis
  toques, con `de` = el nombre de mi robot `desde` (tiene que ser mío) o el mío
  ("Un amigo" si no puse). `expresion` solo con `carita` (y obligatoria). Si no
  llegó a ningún robot y vinculó Telegram: "Ana te mandó un abrazo 🤗". Tope:
  30 por hora por par, en cada sentido (en memoria). Evento SSE `toque`
  `{de, tipo, robot}` para el que lo recibe.
- **Devolver**: cuando un robot avisa `devolver-toque` (evento 40, lo avisa
  solo), la nube busca el último toque que le llegó (en memoria, 10 min, una
  sola vez) y se lo manda de vuelta al robot `desde` (si sigue conectado) o a
  los del que tocó que reciben los toques de este dueño, con `de` = el nombre
  del robot que lo devolvió; el evento `toque` lleva `devuelto: true` y, si no
  llega a ninguno, el Telegram dice "Pipo te devolvió el abrazo 🤗". Cuenta
  para el tope y no se vuelve a devolver (sin ping-pong).

**Presencia.** La detecta la app del celu (cercanía de Bluetooth con el robot
o una geocerca) o el robot mismo (eventos 42 `llegaste` y 43 `te-fuiste`, que
avisa solo).

- `POST /v1/bichos/{id}/presencia {llegaste: bool}` → `{ok, nueva, enviado}`.
  Si en los últimos 2 min ya se anotó lo mismo (por el robot o por la app), es
  la misma: `nueva: false` y no pasa nada más. Si es nueva, se anota, se le
  manda al robot el `0x49` (`enviado`: si estaba conectado) y corren los
  `cuando` `llegaste` / `te-fuiste`. 60 por hora por robot.
- La nube también anota cuándo se duerme el robot (el `duerme` del estado).
  Se guarda 60 días.
- `GET /v1/bichos/{id}/horarios` → `{llegas, salis, dormis, seguridad, dias}`
  (HH:MM o null): de los últimos 14 días, por día la ausencia más larga (de 30
  min a 20 h) da la salida y la llegada; por noche, la primera dormida después
  de las 20 (o antes de las 5). De cada uno, la mediana. `seguridad` (0 a 1)
  es el promedio de los tres de `min(1, días/7) · max(0, 1 − desvío/90 min)`
  (el desvío es la mediana de las distancias a la mediana).

**Hábitos.** Los de `habitos` de la config (hasta 4).

- `GET /v1/bichos/{id}/habitos` → `[{indice, nombre, icono, hora?, racha, hoy, dias: {"YYYY-MM-DD": true}}]`
  (los días de los últimos 60, para el calendario de puntitos).
- `POST /v1/bichos/{id}/habitos/{i} {hecho: bool}` → `{habito, enviado}`:
  anota (o borra) el de hoy, calcula la racha y le manda al robot el `0x4A`
  con ella. Si quedó hecho, corre el `cuando` `habito` (plantillas `{habito}`
  y `{racha}`). 404 si no hay un hábito en ese lugar; 60 por minuto.
- **La racha**: los días seguidos hechos hasta hoy (si hoy todavía no, hasta
  ayer); los días que no tocan según `dias` no la cortan (y si se hizo igual,
  suman). Los días se guardan con el nombre del hábito: si en ese lugar se
  pone otro, empieza de cero. Se guardan 400 días.
- Si lo marca el robot (a la hora del hábito, con un toque: registro tipo 8,
  que avisa solo), la nube lo anota y corre el `cuando`, pero no le manda el
  `0x4A` (ya lo festejó).
- Telegram: `/habito` (sin decir cuál, la lista con la racha y un botón por
  hábito: datos `habito:<id>:<i>`), `/habito agua [robot]` lo marca.

**Avisos entrantes.** Una dirección secreta por usuario,
`POST /v1/entrantes/{token}` (sin sesión: el token es la llave). El token
(`ent_` + 32 caracteres) es un HMAC del `SECRETO` del servidor con una
semilla al azar del usuario: en la base están solo la semilla y el hash del
token (se lo busca por hash), y así `GET` lo puede volver a mostrar. Si
cambia el `SECRETO`, la dirección vieja sigue andando pero no se puede
mostrar (`regenerar: true`).

- `GET /v1/entrantes` → `{url, token, mercadoPago, creado}` (la primera vez
  la crea). `POST /v1/entrantes` la regenera (la vieja deja de andar; 10 por
  hora). `PUT /v1/entrantes/mercadopago {accessToken}` guarda el access token
  de Mercado Pago (cifrado con AES-GCM, con una clave que sale del `SECRETO`);
  `DELETE` lo saca.
- Lo que acepta (hasta 2 MB; 120 por minuto por IP antes de buscar el token
  y 30 por minuto por usuario):
  - **JSON propio** `{tipo: pago|mail|github|pedido|otro, titulo (1 a 200
    bytes), de?, robot?}` (robot: el id o el apodo de uno suyo; si no, todos).
  - **GitHub** tal cual (con `X-GitHub-Event`): en el repo, Webhooks, la
    dirección con `application/json` y el **mismo token como secreto**; la
    firma `X-Hub-Signature-256` se compara en tiempo constante (sin firma o
    con otra, 401). `ping` → 200. PR abierto ("PR #12"), mergeado ("PR #12
    mergeado"), review ("PR #12 aprobado", "Cambios en PR #12"), issue
    ("Issue #5"), estrella ("★ repo") y CI fallido (`workflow_run` o
    `check_suite` con `failure`: "CI falló", de = el repo); `de` es el que lo
    hizo. Lo demás, 202 `{ignorado: true}`.
  - **Mercado Pago** (`type`/`topic` `payment`, en el cuerpo o en la URL):
    sin access token, "¡Te pagaron!" (de "Mercado Pago"), solo con
    `payment.created`; con el access token, la nube lee el pago
    (`GET api.mercadopago.com/v1/payments/{id}`, el id solo números) y avisa
    solo los aprobados con el monto ("$12.000") y el nombre del que pagó. Una
    vez por pago (48 h, en memoria).
- Manda el `0x44` con el tipo nuevo (`app` = el título corto, `contacto` =
  de) a los robots conectados del usuario → `{ok, robots}` (cuántos lo
  recibieron) y el evento SSE `entrante` `{tipo, titulo, de, origen: json|github|mercadopago, robots}`.

**Cuandos nuevos** para las automatizaciones: `llegaste`, `te-fuiste` y
`habito` (con `{habito}` y `{racha}`). Los avisa el robot solo o salen de las
rutas de arriba: no hace falta pedirle el registro.

### Agenda, música y charla (v1.9)

Código: `api/internal/integraciones/agenda.go`, `musica.go`, `charla.go` y
`v19.go` (el trabajador); las rutas en `servidor/agenda.go`, `musica.go` y
`charla.go`; el MCP en `servidor/mcp_agenda.go`; el .ics en
`api/internal/agenda`. Los secretos de terceros (la dirección del calendario y
el refresh token de Spotify) van **cifrados** en la base (`api/internal/cifra`:
AES-256-GCM con una clave que sale de `SECRETO`; si `SECRETO` cambia, hay que
volver a conectarlos) y nunca se devuelven.

**Agenda** (migración `011_agenda.sql`, un calendario por usuario):

- `PUT /v1/agenda {"ics"?: "https://…" | "webcal://…", "aviso"?: 0..15}`: la
  "dirección secreta en formato iCal" de Google Calendar (o cualquier `.ics`).
  Se valida como los webhooks (solo https —http en desarrollo—, sin usuario ni
  clave, sin IPs internas ni el propio servicio; el DNS se valida al conectar),
  se baja ya (tope 2 MB y 15 s, hasta 3 redirecciones validadas) y, si no es un
  calendario, 400 con el motivo. Con solo `aviso`, cambia cuántos minutos antes
  avisa (por defecto 10; 404 sin calendario). 20 cambios por hora.
- `GET /v1/agenda` → `{"conectada", "ics": "calendar.google.com/…/basic.ics"
  (solo la pista), "aviso", "proximos": [{uid, titulo, inicio, fin, todoElDia}]
  (los 10 próximos que no terminaron), "leida", "error"}`. `DELETE /v1/agenda`.
- El .ics: VEVENT con `DTSTART`/`DTEND`/`DURATION` (con `TZID` —también los
  nombres de Windows de Outlook y los prefijados—, en UTC, flotantes o de día
  entero), `RRULE` (DAILY, WEEKLY, MONTHLY y YEARLY con BYDAY, INTERVAL, COUNT
  y UNTIL; las de cada hora o menos se toman como una sola vez), `EXDATE`,
  `RECURRENCE-ID` (una repetición movida) y `STATUS:CANCELLED`. La expansión
  usa [`github.com/teambition/rrule-go`](https://github.com/teambition/rrule-go)
  (chica, sin dependencias, la que más se usa en Go para RRULE); el resto del
  .ics se lee a mano (desdoblar líneas, parámetros, escapes), que es poco y así
  se controla. Topes: 3000 VEVENT, 20 000 repeticiones miradas por evento, 500
  ocurrencias. Las zonas van embebidas (`time/tzdata`).
- **Cada minuto** (`revisarAgendas`): relee los que tienen más de 5 min (si
  falla, quedan los de antes y `error`), y guarda las ocurrencias de las
  próximas 2 semanas (`proximos`, jsonb). Lo que empieza dentro de `aviso`
  minutos (sin los de día entero) va **una sola vez** (las claves `uid@inicio`
  quedan en `avisados`, así un reinicio no repite) con el `0x48`
  (`protocolo.Agenda(minutos, duración, título)`) a todos los robots
  conectados del dueño, el evento SSE `agenda` (`{titulo, minutos, inicio,
  robots}`) y el **cuando `agenda`** de las automatizaciones de cada robot
  (`{texto}` = el título; datos `titulo` y `minutos`).
- **Resumen de mañana**: a la hora de dormir (`horario.duerme` del primer
  robot si es de 18:00 a 23:59; si no, 22:00; con media hora de margen por si
  la API estaba caída), si mañana hay algo, una vez por día (`resumen_dia`): el
  `0x48` con `0xFFFF` (`protocolo.ResumenManana(cantidad, primero)`) a los
  robots conectados y, con Telegram vinculado, "Mañana tenés 2 cosas:" y la
  lista.
- Telegram **`/agenda`**: lo de hoy y mañana. MCP **`agenda`** `{dias?: 1..14}`.

**Música** (`spotify_cuentas` en `012_musica_charla.sql`): con
`SPOTIFY_CLIENT_ID` y `SPOTIFY_CLIENT_SECRET` (una app en
developer.spotify.com con el redirect `API_URL_PUBLICA/v1/musica/spotify/vuelta`);
sin ellas, apagado y `GET /v1/musica` → `{"disponible": false}`.

- `GET /v1/musica/spotify` → `{"url"}`: a donde mandar al usuario (alcances
  `user-read-currently-playing user-read-playback-state`; el `state` lleva su
  id y vence en 10 min, firmado con HMAC). La vuelta (`GET
  /v1/musica/spotify/vuelta`, sin sesión: la autentica el state) canjea el
  código, guarda el refresh token cifrado y vuelve a la web con
  `WEB_URL/robots?spotify=ok|error|cancelado`.
- `GET /v1/musica` → `{"disponible", "conectada", "nombre", "suena":
  {sonando, titulo, artista, bpm} | null}`. `DELETE /v1/musica/spotify`.
- Cada 20 s, solo para los usuarios con algún robot conectado, mira qué suena
  (`/v1/me/player/currently-playing`; el access token vive en memoria y se
  renueva solo; si Spotify dice `invalid_grant`, se olvida la cuenta) y, si
  cambió el tema o se pausó, manda el `0x4B` (`protocolo.Musica`) a sus robots
  conectados y el SSE `musica`. El bpm sale de `/v1/audio-features` si la app
  tiene acceso (las apps nuevas de Spotify no: entonces 0, y el robot baila a
  100). Un robot que se conecta recibe lo que suena en la vuelta siguiente.
- `POST /v1/bichos/{id}/musica {"sonando", "titulo", "artista", "bpm"?}` para
  el celu (Android lo lee de la sesión de medios): el `0x4B` a ese robot (409 si
  no está conectado; 30 por minuto). MCP **`musica`** (`ver` / `mandar`).

**Charla con IA** (`charlas_uso` y `charla_robot`): con `ANTHROPIC_API_KEY`
(modelo `claude-haiku-5-5`, o `CHARLA_MODELO`), con el SDK oficial de Go
(`github.com/anthropics/anthropic-sdk-go`), **o** con cualquier servicio
compatible con la API de chat de OpenAI (gana sobre Anthropic): `CHARLA_URL`
(la base, por ejemplo `https://api.groq.com/openai/v1`, OpenRouter, Gemini o un
Ollama propio), `CHARLA_CLAVE` y `CHARLA_MODELO` (obligatorio en este modo). Se
pide `response_format: json_object` y el esquema va en el prompt
(`charla_compatible.go`). Para arrancar sin costo sirve una capa gratis (Groq);
el tope de 60 por día por usuario es el mismo. Sin ninguno, `GET /v1/charla` →
`{"disponible": false}` y charlar da 503.

- `POST /v1/bichos/{id}/charla {"texto"}` (1 a 500 bytes) → `{"texto",
  "expresion", "enviado", "quedan"}`: el robot contesta y lo dice en su
  pantalla (`0x40`) si está conectado. `GET /v1/charla` → `{"disponible",
  "quedan", "porDia": 60}`.
- El system prompt es corto y tiene solo lo de **ese** robot: su nombre, su
  personalidad (`personalidad.preset`), el nombre del dueño, la hora, la ciudad
  y el clima, y lo que recuerda (días juntos, racha, hitos y el resumen de
  hoy). El modelo no tiene herramientas: no hay cómo sacarle datos de otros. La
  respuesta es JSON por salida estructurada (`texto` y `expresion`, una de las
  de fábrica o de su config); el texto se deja en una línea y se corta a 60
  bytes sin partir letras; una expresión que no existe no se pone.
- Tope: **60 por día por usuario** (en la base, compartido entre la web, la
  app, Telegram y el robot), 10 por minuto por HTTP, 20 s de timeout.
- **Telegram**: un mensaje que no es comando es una charla. Con un robot, con
  ese; con varios, con el último con el que charló (o pregunta "¿Con cuál
  charlás?" con botones `charla:<id>`).
- **El robot-celu** (sin token HTTP, con sus credenciales MQTT) publica en
  **`bicho/{id}/charla`** `{"texto": "…"}` (hasta 200 bytes); la ACL deja que
  cada robot publique solo en el suyo. La nube contesta con el `0x40` por su
  `cmd` (con el tope del día: "Por hoy ya charlamos mucho. ¡Mañana más!"). 30
  por hora por robot, 8 a la vez.
- Lo mismo para la presencia que el robot-celu sabe por su cuenta (el wifi de
  casa o el Bluetooth del celu del dueño): publica en **`bicho/{id}/presencia`**
  `{"llegaste": bool}` y la nube solo la anota para los horarios y corre los
  cuandos (no le reenvía el `0x49`: ya se lo dijo a su núcleo).
- SSE `charla` (`{texto, expresion, de, enviado}`).

## MCP: que una IA maneje los robots

La API es un **servidor MCP remoto**: cada usuario conecta su Claude
(claude.ai, Claude Desktop, Claude Code) o su ChatGPT y la IA maneja sus
robots: inventa caras (y las ve), pone imágenes, recordatorios, personalidad…
Código: `api/internal/servidor/mcp*.go` y `oauth.go`; migración `006_mcp.sql`.

- **Transporte**: MCP "Streamable HTTP" en `POST /mcp` (SDK oficial
  `github.com/modelcontextprotocol/go-sdk` v1.8), **sin sesiones** (cada pedido
  trae su token: sobrevive reinicios y proxies; `GET` responde 405) y con
  respuestas JSON. Cuerpo de hasta 12 MB (una imagen en base64).
- **Límites**: 120 llamadas a herramientas por minuto por usuario (300 pedidos
  HTTP); 20 Telegrams por hora. Cada llamada queda en el log
  (`msg=mcp herramienta=… usuario=… cliente=… bicho=… ok=… ms=…`).
- **En vivo**: los cambios salen por los eventos de siempre (`config`,
  `imagenes`, y `estado` cuando el bicho reacciona) y, por cada llamada que
  cambia algo, un evento `mcp` `{cliente, herramienta, titulo, bicho, en}` (la
  consola puede mostrar "Claude cambió X").
- Todo cambio de config pasa por el mismo camino que `PUT /v1/bichos/{id}/config`:
  la valida el núcleo (cgo) y queda como versión nueva con autor
  `mcp:<cliente>` (por ejemplo `mcp:Claude`); un candado por bicho evita pisar
  dos cambios simultáneos. La config conserva el orden de sus claves.

### Autenticación

**OAuth 2.1** (lo que usan claude.ai, Claude Desktop y ChatGPT, solos):

| Ruta | Qué |
| --- | --- |
| `GET /.well-known/oauth-protected-resource` (y `…/mcp`) | RFC 9728: `resource` = `API_URL_PUBLICA/mcp`, `authorization_servers` = `[API_URL_PUBLICA]` |
| `GET /.well-known/oauth-authorization-server` | RFC 8414 (issuer = `API_URL_PUBLICA`) |
| `POST /oauth/registrar` | Registro dinámico (RFC 7591), JSON. Redirecciones: `https://…`, `http://localhost|127.0.0.1|[::1]` (cualquier puerto, RFC 8252) o un esquema propio de app (`cursor://…`); nunca `javascript:`, `data:`, etc. `token_endpoint_auth_method`: `none` (público) o `client_secret_post`/`_basic` (devuelve `client_secret`). 30 por hora por IP |
| `GET /oauth/autorizar` | Se publica como **`WEB_URL/api/oauth/autorizar`**: pasa por el proxy de la web para que llegue la cookie de sesión. Sin sesión → `WEB_URL/entrar?volver=/api/oauth/autorizar?…`. Con sesión: la pantalla "<cliente> quiere manejar tus robots" (Permitir / Cancelar), HTML servido por la API, sin scripts, `frame-ancestors 'none'` |
| `POST /oauth/autorizar` | El formulario: token anti-CSRF (HMAC con `SECRETO`, atado a la sesión y al pedido) y `Origin` de la web o la API. Devuelve `code` (5 min, un solo uso), `state` e `iss` |
| `POST /oauth/token` | `authorization_code` con **PKCE S256 obligatorio**; `refresh_token` con **rotación**. Acceso: 1 h; refresco: 60 días. Reusar un código o un refresco ya canjeado revoca toda la concesión. 60 por minuto por IP |
| `POST /oauth/revocar` | RFC 7009 (un refresco se lleva su concesión) |

Los tokens son opacos (`sin_at_…`, `sin_rt_…`, `sin_pat_…`) y solo se guardan
sus hashes. Sin token, `/mcp` responde 401 con
`WWW-Authenticate: Bearer resource_metadata="…"`. Los tokens de sesión de la
web y de las apps **no** sirven para el MCP, y los del MCP no sirven para el
resto de la API.

**Tokens personales** (Claude Code, pruebas), desde la web (con sesión):
`POST /v1/mcp/tokens {nombre}` → `{token, id, nombre, creado, url}` (el token
se ve **una sola vez**; hasta 20), `GET /v1/mcp/tokens`, `DELETE /v1/mcp/tokens/{id}`.
**IAs conectadas**: `GET /v1/mcp/clientes` → `{clientes:[{id, clienteId, cliente, uri, alcance, creada, usada}], url}`;
`DELETE /v1/mcp/clientes/{id}` la desconecta (todos sus tokens dejan de andar).

### Cómo se conecta cada cliente

La URL del servidor es **`API_URL_PUBLICA/mcp`** (en el VPS,
`https://api-sintetica.grupovimo.com.ar/mcp`). claude.ai y ChatGPT se
conectan desde sus servidores: necesitan **HTTPS público**; en local
(`http://localhost:8080/mcp`) solo anda Claude Code (o Desktop con un token).

- **claude.ai / Claude Desktop**: Configuración → Conectores → Agregar conector
  personalizado → la URL. Se registra solo, abre la web para entrar y
  permitir, y listo.
- **ChatGPT** (modo desarrollador / conectores): Nuevo conector → la URL,
  autenticación OAuth. Igual que Claude.
- **Claude Code**, con un token personal:
  `claude mcp add --transport http sintetica http://localhost:8080/mcp --header "Authorization: Bearer sin_pat_…"`
  (o sin `--header`, con la URL https pública, y `/mcp` para hacer el OAuth).

### Herramientas

| Herramienta | Qué hace |
| --- | --- |
| `listar_robots` | Los robots: conectado, batería, cara, ánimo, pantalla |
| `ver_robot` | Estado, capacidades y resumen de la config (personalidad, horario, expresiones y comportamientos propios, recordatorios, reflejos, imágenes, protector) |
| `poner_cara` | Fija una expresión (o `soltar`) — robot conectado. Con `siempre: true` la deja como su cara de siempre (`cara.expresion` en la config; con `soltar`, vuelve la neutral) |
| `disparar_evento` | Un evento del protocolo (caricia, sacudida, nombre…) — conectado |
| `reproducir_comportamiento` | Empieza un comportamiento (de fábrica o propio) — conectado |
| `listar_expresiones` | De fábrica y propias, con sus parámetros |
| `crear_expresion` | Expresión paramétrica (ojo/izq/der y los globales del schema, con sus límites); la valida el núcleo, la guarda y devuelve un **PNG** de cómo queda (dibujado por el núcleo: `sintetica_dibujar_expresion`). `solo_ver` para iterar sin guardar; `mostrar` para ponerla ya |
| `dibujar_cara` | Una **cara de imagen**, la que sea: `ojo` (obligatorio), `ojo_cerrado`, `boca`, `ojo_der`, `ojo_der_cerrado`, cada una como SVG (`<svg …>`), URL https (anti-SSRF) o base64. Las convierte juntas (misma escala: la más grande entra en 128 × 128), las guarda en sprites libres (o los que ya tenía esa cara), crea la expresión con `sprites` (+ `tamano`, `boca_tamano`, `boca_y`, `espejar_der`, `sep`, `ey`, `movimiento`…) y, con `reposo: true`, la deja como su cara de siempre. Devuelve **dos PNG** dibujados por el núcleo (`sintetica_dibujar_expresion_cierre`): abierta y parpadeando. `solo_ver` para iterar; `mostrar` para fijarla ya |
| `borrar_expresion` | Saca una propia (si es de imagen, también sus sprites que no use otra; si era la de siempre, vuelve la neutral) |
| `poner_imagen` | URL https (bajada con las protecciones anti-SSRF de los webhooks, 8 MB, 15 s, hasta 3 redirecciones validadas) o base64; PNG/JPEG/GIF → recortada llenando la pantalla, redonda en el broche, QOI de 3 canales o animación `sani` (≤ 64 cuadros); en una bahía 0–7; opcional `lista` y `protector` |
| `borrar_imagen` | Vacía una bahía (y la saca de la lista y el protector) |
| `recordatorios` | listar / crear / borrar (máx. 12; `{hora, dias, texto ≤ 40 bytes, expresion, sonido, activo}`) |
| `personalidad` | Preset, rasgos y `sueno` |
| `horario` | `despierta` / `duerme` |
| `modo` | mascota, reloj o pomodoro (+ `pomodoro {trabajo, descanso}`) |
| `reflejos` | Reglas `si: evento → reflejo:cara` (+ acciones extra); listar / agregar / borrar |
| `automatizaciones` | Las de Integraciones: listar / guardar (la lista entera, con las acciones y los `cuando` nuevos) / probar |
| `mandar_telegram` | Le escribe al usuario si vinculó Telegram |
| `decir` | `{robot, texto ≤ 60 bytes, expresion?}`: lo dice en un globo (marco `0x40`) — conectado |
| `despertador` | `ver` / `poner` (`hora`, `dias`, `texto`, `posponer`, `sonido`, `activo`; en `despertador` de la config) / `sacar`; y, conectado, `probar` / `apagar` / `posponer` (marco `0x42`) |
| `clima` | `ver` (ubicación y clima de ahora) / `buscar` (`q`, sin robot) / `ubicar` (`lat`, `lon`, `ciudad`, o `q` y el primer lugar): guarda `ubicacion` y le manda el clima ya |
| `rutinas` | Las automatizaciones `cuando: hora`: `listar` / `crear` (`hora`, `dias`, `hacer[]`, `id` opcional: `rutina-HHMM`) / `borrar` (`id`) |
| `habla` | `ver` / `frecuencia` (0 a 1) / `agregar` (`situacion`, `texto` ≤ 60 bytes; hasta 12 frases propias en total; también las situaciones de la v1.7: `buen-dia`, `comiste`, `a-dormir`, `tardaste`, `agua`, `pausa`, `como-estas`, `animo-bien`, `animo-medio`, `animo-mal`, `jugar`, `gane`, `perdi`, `empate`, `anio-nuevo`, `navidad`, `dia-del-amigo`, `dia-del-nino`, `aniversario`) / `borrar` (`situacion` y, opcional, `texto`), en `habla` de la config |
| `jugar` | `{robot, juego}`: `ppt`, `reflejos`, `mirada`, `dado`, `moneda` o `terminar` (marco `0x43`) — conectado |
| `cuidado` | `ver` (lo de la config y los valores por defecto) / `poner` (`agua` 0 a 8 h, `pausa` 0 o 30 a 240 min, `trabajo {desde, hasta, dias}`, `animo`, `extranar`, `extranarDias` 1 a 7, `resumen`), en `cuidado` de la config |
| `avisos` | `listar` / `agregar` (`tipo` notificacion o llamada, `app` y `contacto` ≤ 24 bytes, `expresion` y `comportamiento` que existan, `decir` ≤ 40 bytes con `{app}` y `{contacto}`, `sonido`; hasta 8) / `borrar` (`indice`), en `avisos` de la config |
| `resumen` | Cómo fue su día (lo mismo que `GET /v1/bichos/{id}/resumen`) |
| `amigos` | `listar` / `invitar` (el código y los links) / `aceptar` (`codigo`) / `borrar` (`amigo`: nombre o id) |
| `tocar` | `{amigo?, tipo, expresion? (carita), robot? (desde cuál)}`: un toque a los robots de un amigo (o por Telegram si no tiene ninguno conectado) |
| `habitos` | `ver` (con la racha y los días) / `marcar` / `deshacer` (`habito`: nombre o número) |
| `horarios` | Lo que aprendió la nube: `llegas`, `salis`, `dormis` y `seguridad` |
| `entrantes` | `ver` (la dirección secreta y cómo usarla con GitHub, Mercado Pago o un JSON) / `regenerar` |
| `agenda` | `{dias?: 1..14}`: los próximos eventos del calendario conectado (v1.9) |
| `musica` | `ver` (qué suena en su Spotify) / `mandar` (`robot`, `sonando`, `titulo`, `artista`, `bpm`): el `0x4B` — conectado (v1.9) |

Además, el recurso **`sintetica://guia/caras`** (qué hace cada parámetro, sus
límites, consejos y ejemplos, armado desde el schema, y la sección "Caras de
imagen": cómo dibujar para la pantalla, con un ejemplo de ojitos de anime en
SVG) y el prompt `inventar_cara`.

## Variables de la API

| Variable | Para qué | En local |
| --- | --- | --- |
| `DATABASE_URL` | Postgres | el del compose |
| `MQTT_USUARIO`, `MQTT_CLAVE` | el superusuario del broker (vacío = nadie) | `api`, `MQTT_CLAVE_API` |
| `MQTT_TCP`, `MQTT_TCP_MAX` | el puerto MQTT sin TLS (`no` = apagado) y su tope de conexiones | `:1883`, `64` |
| `MQTT_TLS`, `MQTT_TLS_MAX` | el puerto MQTT con TLS (vacío = apagado) y su tope | vacío, `2000` |
| `MQTT_TLS_CERT`, `MQTT_TLS_CLAVE` | el certificado y la clave (PEM) del TLS; se releen solos | vacío |
| `MQTT_URL_PUBLICA` | la que se les da a los bichos | `mqtt://IP_LOCAL:1883` |
| `MQTT_URL_INTERNA` | la que se le da al bicho virtual | `mqtt://api:1883` |
| `API_URL_PUBLICA`, `API_URL_INTERNA` | de dónde bajan las OTA (placa y virtual); la pública es también el issuer de OAuth y la base del MCP (`/mcp`) | `http://IP_LOCAL:8080`, `http://api:8080` |
| `WEB_URL` | para los links de los mails | `http://localhost:5173` |
| `SMTP_SERVIDOR`, `SMTP_USUARIO`, `SMTP_CLAVE`, `SMTP_DE` | el correo | Mailpit, sin usuario |
| `CLAVE_FIRMA` | la clave privada de las OTA | `/claves/ota-privada.pem` |
| `SECRETO` | firma los links de descarga y de él sale la clave con la que se cifran el calendario y Spotify | uno local |
| `ADMIN_EMAILS` | quién sube firmwares | vacío |
| `DESARROLLO` | bichos virtuales y los links en el log | `1` |
| `BICHOS_VIRTUALES` | dónde están los virtuales | los dos del compose |
| `COOKIE_SEGURA` | cookie solo por HTTPS | `0` |
| `TELEGRAM_TOKEN` | el token del bot (de @BotFather); vacío = Telegram apagado | el de `.env` (nunca en el repo) |
| `TELEGRAM_BOT` | el usuario del bot, sin `@` | `sinteticalabs_bot` |
| `TELEGRAM_API_URL` | la API de Telegram (para tests) | vacío = `https://api.telegram.org` |
| `CLIMA_API_URL`, `GEOCODING_API_URL` | Open-Meteo (pronóstico y búsqueda de lugares; para tests) | vacío = `https://api.open-meteo.com/v1/forecast` y `https://geocoding-api.open-meteo.com/v1/search` |
| `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` | la app de Spotify (la música); vacías = apagado | el de `.env` (nunca en el repo) |
| `ANTHROPIC_API_KEY` | la charla con IA; vacía = apagada | el de `.env` (nunca en el repo) |
| `CHARLA_MODELO` | el modelo de la charla | vacío = `claude-haiku-5-5` (con `CHARLA_URL`, obligatorio) |
| `CHARLA_URL`, `CHARLA_CLAVE` | un servicio compatible con OpenAI para la charla (Groq, OpenRouter, Gemini, Ollama) | vacías = Anthropic |

## Antes de ponerlo en el VPS

Esto está pensado para correr en local. Para producción falta (y está
anotado acá a propósito):

- **TLS** en todo: la web y la API detrás de un proxy con HTTPS
  (`COOKIE_SEGURA=1`), y MQTT por `mqtts://` (el broker de la API con su
  certificado en `MQTT_TLS_CERT`; el firmware ya acepta `mqtts://` con los
  certificados raíz de ESP-IDF).
- Cambiar **todas** las claves: `MQTT_CLAVE_API`, `SECRETO` y la de Postgres.
- Un SMTP de verdad (con SPF y DKIM para el dominio).
- Guardar `claves/ota-privada.pem` fuera del servidor de la web, con copia
  segura: perderla obliga a reflashear por USB todos los bichos.
- Sacar `DESARROLLO=1`, los bichos virtuales y, en las apps, el permiso de
  HTTP sin cifrar (`res/xml/red.xml` en Android, `NSAllowsLocalNetworking` en iOS).
- El cifrado del enlace BLE y secure boot quedan para M8.
- MCP: `API_URL_PUBLICA` con el `https://` público de la API (es el issuer y el
  recurso) y `WEB_URL` con el de la web (la pantalla de permitir pasa por su
  `/api`). Caddy tiene que dejar pasar `/.well-known/*`, `/oauth/*` y `/mcp`.
