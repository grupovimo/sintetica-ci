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
  `automatizacion`, `telegram` (ver "Integraciones") y `mcp` (ver "MCP").

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
por bicho y los recordatorios que la nube manda cuando el bicho no está.

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
  inline con las caras; `auto` vuelve a la automática), `/decir <texto> [nombre]`,
  `/ayuda`. Con un solo robot el nombre no hace falta; con varios, si no se
  dice, aparece un teclado para elegirlo. El nombre es el apodo (o el nombre
  del bicho, o su id), sin importar mayúsculas ni tildes.
- **Pendiente: `/decir`**. El núcleo todavía no tiene un comando para mostrar
  un texto que llega de afuera. Por ahora le manda el evento `notificacion` (el
  bicho reacciona) y contesta que el texto en pantalla llega con los
  recordatorios.

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
  `valor` (opcional) solo con `"cuando": "expresion"`: cuál.
- **Plantillas** en los textos: `{robot}` (apodo, nombre o id), `{bateria}`,
  `{expresion}`, `{evento}`, `{texto}` (el del recordatorio), `{hora}` (HH:MM,
  hora del bicho).
- **`telegram`**: le escribe al chat vinculado del dueño (texto de 1 a 1000 bytes).
- **`robot`**: solo a bichos de la misma cuenta (se valida al guardar y al
  correr); le pone esa expresión (el mismo comando que la web).
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
| `tacto`, `doble-toque`, `caricia`, `muchos-toques`, `levantado`, `sacudida`, `sacudida-suave`, `sacudida-fuerte`, `boca-abajo`, `inclinado`, `caida`, `voz`, `nombre`, `ruido`, `musica`, `cargando` (= `enchufado`), `desenchufado`, `cargado`, `volvio` | el registro del motor de vida (`0x1E`, tipo 0) |
| `hito` | el registro (tipo 3) |
| `recordatorio` | el aviso que el bicho manda solo al mostrar uno (ver abajo) |
| `pomodoro` | el aviso que el bicho manda solo al terminar un tramo del pomodoro |

Los del registro: la API le pide el registro (`0x1E`) cada 10 s **solo** a los
bichos conectados que tienen alguna automatización activa que lo necesita, y
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
| `automatizaciones` | Las de Integraciones: listar / guardar (la lista entera) / probar |
| `mandar_telegram` | Le escribe al usuario si vinculó Telegram |

Además, el recurso **`sintetica://guia/caras`** (qué hace cada parámetro, sus
límites, consejos y ejemplos, armado desde el schema, y la sección "Caras de
imagen": cómo dibujar para la pantalla, con un ejemplo de ojitos de anime en
SVG) y el prompt `inventar_cara`.

## Variables de la API

| Variable | Para qué | En local |
| --- | --- | --- |
| `DATABASE_URL` | Postgres | el del compose |
| `MQTT_URL`, `MQTT_USUARIO`, `MQTT_CLAVE` | la conexión de la API al broker | `tcp://emqx:1883`, `api`, `MQTT_CLAVE_API` |
| `MQTT_URL_PUBLICA` | la que se les da a los bichos | `mqtt://IP_LOCAL:1883` |
| `MQTT_URL_INTERNA` | la que se le da al bicho virtual | `mqtt://emqx:1883` |
| `API_URL_PUBLICA`, `API_URL_INTERNA` | de dónde bajan las OTA (placa y virtual); la pública es también el issuer de OAuth y la base del MCP (`/mcp`) | `http://IP_LOCAL:8080`, `http://api:8080` |
| `WEB_URL` | para los links de los mails | `http://localhost:5173` |
| `SMTP_SERVIDOR`, `SMTP_USUARIO`, `SMTP_CLAVE`, `SMTP_DE` | el correo | Mailpit, sin usuario |
| `CLAVE_FIRMA` | la clave privada de las OTA | `/claves/ota-privada.pem` |
| `SECRETO` | firma los links de descarga | uno local |
| `ADMIN_EMAILS` | quién sube firmwares | vacío |
| `DESARROLLO` | bichos virtuales y los links en el log | `1` |
| `BICHOS_VIRTUALES` | dónde están los virtuales | los dos del compose |
| `EMQX_API_URL`, `EMQX_API_CLAVE` | para echar a un bicho | `http://emqx:18083`, la de `emqx/api-claves.txt` |
| `COOKIE_SEGURA` | cookie solo por HTTPS | `0` |
| `TELEGRAM_TOKEN` | el token del bot (de @BotFather); vacío = Telegram apagado | el de `.env` (nunca en el repo) |
| `TELEGRAM_BOT` | el usuario del bot, sin `@` | `sinteticalabs_bot` |
| `TELEGRAM_API_URL` | la API de Telegram (para tests) | vacío = `https://api.telegram.org` |

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
- MCP: `API_URL_PUBLICA` con el `https://` público de la API (es el issuer y el
  recurso) y `WEB_URL` con el de la web (la pantalla de permitir pasa por su
  `/api`). Caddy tiene que dejar pasar `/.well-known/*`, `/oauth/*` y `/mcp`.
