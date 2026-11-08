# Xiaozhi: qué tomamos y cómo (fase 1: investigación)

Estado al 8/11/2026. Todo lo de acá lo leí en el código y en las docs de los
repos (links al final). Lo que no pude comprobar dice **a verificar**.

## La recomendación

> **Una sola placa y un solo producto** (decisión de Nahuel, 8/11): el cubito
> lleva **nuestro firmware** (la cara viva, el motor de vida, todo). De xiaozhi
> se toma solo el **protocolo de voz** (WebSocket + Opus contra nuestra nube) y
> piezas del firmware (audio, AEC). Flashearle xiaozhi tal cual es solo una
> prueba de un día del audio, en la misma placa, y después va el nuestro. La
> "puerta xiaozhi" para placas ajenas es un extra opcional, no algo que el
> producto necesite.

1. **Sí a una "puerta xiaozhi" en nuestra API, solo por WebSocket.** La nube
   de Sintética habla el protocolo de xiaozhi y cualquier placa con el
   firmware de xiaozhi (más de 50 fabricantes, más de 100 placas) se vincula como un
   **robot de voz** de Sintética: un código de 6 dígitos en su pantalla y se
   tipea en la consola. Oído: Whisper de Groq (capa gratis). Cerebro: la
   charla que ya tenemos (el motor propio y, si hay, la IA). Voz: Edge TTS.
   No hace falta ningún servidor extra. Es **aditivo**: los bichos de
   siempre siguen igual.
2. **La cara viva no se pierde.** En una placa con el firmware de xiaozhi la
   pantalla muestra los emojis de xiaozhi (le mandamos la emoción más
   parecida a la expresión que elige la charla), pero el robot existe en la
   nube con su personalidad, su memoria y su cara viva en la consola y las
   apps. Más adelante se le puede correr un **gemelo del núcleo** en la API
   (ya está por cgo) para que tenga motor de vida de verdad.
3. **Nuestro cubito usa el mismo protocolo para la voz.** La placa elegida
   para el cubito **ya está soportada por xiaozhi**
   (`main/boards/waveshare/esp32-s3-touch-lcd-1.54`, con AEC en el chip
   activado). El contrato M7 de voz (Opus de 20 ms por `bicho/{id}/audio`)
   todavía no tiene hardware: propongo reemplazar **solo la parte de voz en
   tiempo real** por el protocolo de xiaozhi, así hay un solo servidor de voz
   para las placas de xiaozhi y para el cubito. Todo lo demás (comandos,
   estado, OTA firmada, leer en voz alta tipo 2) sigue por MQTT como hoy.
4. **El primer día con el cubito, flashearle xiaozhi tal cual** (el binario de
   sus Releases) y apuntarlo a nuestra API: prueba el audio de la placa y la
   puerta sin escribir una línea de firmware. Después se porta lo que sirva a
   `firmware/perfiles/cubito`.
5. **xiaozhi.me no.** Ni para el producto ni para los usuarios (ver abajo).

Costo: cero mientras alcance la capa gratis de Groq y Edge TTS siga andando.
Mantenimiento: un paquete de Go chico (`api/internal/xiaozhi`), sin Python, sin
modelos locales, sin puertos UDP nuevos.

## Lo que hace falta de Nahuel

- Decidir lo del punto 3 (que la voz en tiempo real del cubito use el
  protocolo de xiaozhi en vez del contrato M7 de `protocolo-ble.md`).
- Una clave de Groq (gratis, console.groq.com) en el `.env` para el oído. Si
  ya está la de la charla (`CHARLA_URL` de Groq), sirve la misma.
- Cuando llegue el cubito: aceptar flashearle el firmware de xiaozhi para la
  prueba (se borra después con el nuestro por USB).
- Nada público dice "xiaozhi" ni nombra placas: en la web y las apps es
  "robot de voz" y punto.

## Cómo funciona xiaozhi (resumen técnico)

### 1. Arranque: la "OTA" que en realidad es la configuración

Cada vez que prende, el aparato hace `POST` a la URL de OTA
(`CONFIG_OTA_URL`, por defecto `https://api.tenclass.net/xiaozhi/ota/`, o la
que tenga guardada en NVS `wifi/ota_url`), con estos encabezados:
`Device-Id` (la MAC), `Client-Id` (un UUID que genera al flashear),
`Activation-Version` (1 sin número de serie, 2 con serie y HMAC en eFuse),
`User-Agent` (placa/versión), `Accept-Language`, y el JSON de la placa en el
cuerpo. La respuesta (`main/ota.cc`) puede traer:

```json
{
  "server_time": { "timestamp": 1762600000000, "timezone_offset": -180 },
  "firmware":   { "version": "2.0.3", "url": "" },
  "websocket":  { "url": "wss://api-…/xiaozhi/v1/", "token": "…", "version": 1 },
  "mqtt":       { "endpoint": "…", "client_id": "…", "username": "…", "password": "…",
                  "publish_topic": "…", "keepalive": 240 },
  "activation": { "code": "123456", "message": "Tipeá 123456 en la consola",
                  "challenge": "…", "timeout_ms": 30000 }
}
```

- Lo que venga en `websocket` o `mqtt` se guarda tal cual en NVS. Si hay
  `mqtt`, usa MQTT+UDP; si no, WebSocket.
- Con `activation`, muestra el código en pantalla y hace `POST
  {ota_url}activate` hasta que el servidor conteste 200 (202 = "todavía no").
  Con la versión 1 el cuerpo es `{}`: **el vínculo lo decide el servidor** (el
  usuario tipea el código). Es exactamente nuestro "sumar un bicho".
- Si `firmware.version` es más nueva (o `force: 1`), baja `firmware.url` y se
  reflashea **sin verificar firma**: un aparato con xiaozhi confía en el
  servidor que tenga configurado. Nosotros nunca mandamos `url` (salvo, algún
  día, nuestro propio firmware, con el permiso explícito del dueño).

**Cómo se apunta a un servidor propio sin compilar**: en el portal de wifi
(el hotspot `Xiaozhi-XXXX`, http://192.168.4.1), pestaña **Avanzado**, se
guarda la URL de OTA (`esp-wifi-connect`, `wifi_configuration_ap.cc`). **A
verificar** que esa pestaña aparezca en el firmware de fábrica de cada placa
(el campo depende de `show_ota_config`); si no, se compila con otro
`CONFIG_OTA_URL` (ESP-IDF 6.x).

### 2. WebSocket (lo que vamos a hablar)

Doc: `docs/websocket.md`; código: `main/protocols/websocket_protocol.cc`.

- El aparato abre el WebSocket **cuando hay que hablar** (palabra, botón), no
  todo el tiempo. Encabezados: `Authorization: Bearer <token de la OTA>`,
  `Protocol-Version`, `Device-Id`, `Client-Id`.
- `hello` del aparato:
  `{"type":"hello","version":1,"transport":"websocket","features":{"mcp":true,"aec":true},"audio_params":{"format":"opus","sample_rate":16000,"channels":1,"frame_duration":60}}`.
  El servidor tiene 10 s para contestar
  `{"type":"hello","transport":"websocket","session_id":"…","audio_params":{"format":"opus","sample_rate":24000,"channels":1,"frame_duration":60}}`.
  El aparato **usa la duración de cuadro que dice el servidor** para
  decodificar la bajada (`audio_service.cc`, `SetDecodeSampleRate`): podemos
  mandar cuadros de 20 ms (**a verificar en placa**).
- Aparato → servidor: `listen` (`state`: `start` / `stop` / `detect` con
  `text` = la palabra; `mode`: `auto` / `manual` / `realtime`), `abort`
  (`reason`), `mcp` (respuestas JSON-RPC), y **binario**: Opus mono 16 kHz
  de 60 ms (v1: el paquete Opus pelado; v2 y v3 le agregan un encabezado).
- Servidor → aparato: `stt` (`text`: lo que entendió), `llm` (`emotion`: la
  cara), `tts` (`start` / `sentence_start` con `text` / `stop`), `mcp`,
  `system` (`reboot`), `alert` (`status`, `message`, `emotion`), `notify`
  (abajo) y **binario**: Opus para el parlante.
- Modos: en `manual` el aparato manda `listen stop` (botón soltado); en
  `auto` **el servidor decide cuándo terminó la frase** (VAD del lado del
  servidor) y responde con `tts start`; en `realtime` (con AEC) se puede
  interrumpir hablando. Después de `tts stop`, en `auto` vuelve a escuchar;
  en `manual`, a reposo.
- Emociones de `llm` (las 21 que usa el servidor de xinnan,
  `core/utils/textUtils.py`, y que conocen los emojis del firmware): `neutral`, `happy`,
  `laughing`, `funny`, `sad`, `angry`, `crying`, `loving`, `embarrassed`,
  `surprised`, `shocked`, `thinking`, `winking`, `cool`, `relaxed`,
  `delicious`, `kissy`, `confident`, `sleepy`, `silly`, `confused`.
- `notify` (`docs/notify.md`): con el aparato en reposo, el servidor manda
  `{"type":"notify","audio_url":"https://…ogg","subtitles":[{start_ms,text}]}`
  y el aparato baja un **Ogg Opus mono** y lo reproduce sin abrir el
  micrófono. Es nuestro "leer en voz alta" de recordatorios y despertador.

### 3. MQTT + UDP (lo que no vamos a hablar, por ahora)

Doc: `docs/mqtt-udp.md`. El control va por MQTT (los mismos JSON) y el audio
por **UDP cifrado con AES-128-CTR** (la clave y el nonce los da el `hello`
del servidor; encabezado de 16 bytes con ssrc, timestamp y secuencia). Menos
latencia, pero hay que abrir un puerto UDP en el VPS, hacer el cifrado y la
sesión, y no pasa por Cloudflare. El servidor oficial lo usa con un gateway
aparte (`78/xiaozhi-mqtt-gateway`). Para nosotros: WebSocket y listo.

### 4. MCP del lado del aparato

Doc: `docs/mcp-protocol.md`, `docs/mcp-usage.md`; código: `main/mcp_server.cc`.
**El aparato es el servidor MCP y la nube es el cliente**, por el mismo canal
(`type: "mcp"`, `payload` JSON-RPC 2.0). La nube manda `initialize` (puede
pasar `capabilities.vision {url, token}` para que suba fotos), `tools/list`
(paginado con `cursor`; `withUserTools: true` agrega las privilegiadas) y
`tools/call`. Herramientas de fábrica: `self.get_device_status`,
`self.audio_speaker.set_volume`, `self.screen.set_brightness`,
`self.screen.set_theme`, `self.camera.take_photo` (si tiene cámara); solo
para el usuario: `self.reboot`, `self.upgrade_firmware`,
`self.screen.snapshot`, `self.screen.preview_image`, etc. Cada placa suma las
suyas.

Ojo con el pedido original: **nuestras** acciones (cara, recordatorios,
hábitos…) no son herramientas del aparato: son herramientas de nuestro
cerebro (la charla), que ya existen en el MCP de la API. Las del aparato
(volumen, brillo, foto) se las sumamos a la charla y a la consola.

### 5. El audio en el chip (para nuestro firmware)

`main/audio/README.md`, `main/audio/engines/afe_audio_engine.cc`:

- En S3/P4 con PSRAM: **un solo AFE de ESP-SR** con AEC (`FD_LOW_COST`,
  `AEC_NLP_LEVEL_VERYAGGR`) + VAD, y WakeNet adentro (o MultiNet para una
  palabra propia). Sin supresión de ruido (no traen el modelo NSNet). En C3/C6:
  `LiteAudioEngine`, PCM crudo y WakeNet suelto.
- AEC en el aparato (`CONFIG_USE_DEVICE_AEC`) solo en las placas con
  referencia del parlante; **la del cubito la tiene activada**.
- Guarda los últimos 2 s en un anillo de 64 KB en PSRAM para mandar la
  palabra al servidor (por si quiere verificar la voz).
- Códecs: `es8311`, `es8374`, `es8388`, `es8389` y `box_audio_codec`
  (ES8311 + ES7210, el del cubito) sobre `esp_codec_dev` de Espressif.
- Requiere **ESP-IDF 6.0.1 o más** (recomiendan `release/v6.1`); ESP-IDF 5.x
  no. Dependencias por el gestor de componentes de IDF (`esp-sr` 2.4.7,
  `esp_audio_codec` 2.5, `esp_codec_dev` 1.6, LVGL 9.5…).

## Las opciones

### a) La nube habla xiaozhi (recomendada)

| | |
| --- | --- |
| Pros | 100+ placas baratas ya andan; el vínculo con código en pantalla encaja con el nuestro; WebSocket pasa por Caddy y Cloudflare (`wss://`); el cubito puede usar lo mismo; todo en Go, en el mismo proceso |
| Contras | La cara del aparato es la de xiaozhi (emojis), no la nuestra; la marca xiaozhi aparece en el hotspot y el arranque; el protocolo cambia seguido (32 900 estrellas, commits todos los días) y la doc avisa que "puede no coincidir con el código"; la identidad v1 es MAC + UUID (el token de la OTA es la credencial) |
| Costo | Cero: Groq gratis (oído), Edge TTS (voz), la charla de siempre |
| Mantenimiento | Bajo: ~6 mensajes JSON + Opus. Fijar una versión de firmware probada y tener tests con mensajes grabados |

**Oído (ASR) sin costo y sin cargar el VPS**: Whisper en Groq
(`whisper-large-v3-turbo`, API compatible con OpenAI,
`/openai/v1/audio/transcriptions`, `language=es`). Acepta `ogg`: **no hace
falta decodificar para transcribir**, se meten los paquetes Opus tal cual en
un contenedor Ogg (puro Go, unas 100 líneas o el `oggwriter` de Pion) y se
manda. Límites que publica Groq para Whisper: 20 pedidos por minuto, 2000
por día, 7200 s de audio por hora y 28 800 por día (la tabla es del plan
Developer; los del gratis se ven en la cuenta, **a verificar**); cada pedido
cuenta como mínimo 10 s. Si algún día se paga: USD 0,04 la hora. Para que un
usuario no se coma la cuota de todos: un tope por usuario (como los 60 de la
charla). Configurable con `OIDO_URL`/`OIDO_CLAVE`/`OIDO_MODELO` (cualquier
servicio compatible: Groq, OpenAI, un whisper propio).

**Decodificar Opus en Go** (solo hace falta para el VAD del modo `auto`):

- **`github.com/pion/opus`** (MIT, puro Go, v0.1.0 de junio de 2026):
  decodifica SILK, CELT e híbrido con tests de conformidad; el codificador
  solo acepta 48 kHz y 20 ms (CELT). Sin cgo: no cambia el Dockerfile.
- `github.com/hraban/opus` (MIT, cgo con libopus): lo más probado; suma
  `libopus-dev` a la imagen. Ya usamos cgo para el núcleo, así que es
  posible, pero es otra dependencia del sistema.
- CPU: decodificar 16 kHz mono es muy barato (libopus, una fracción de 1 %
  de un núcleo por conversación; Pion, varias veces más, igual en pocos %).
  Esto es **estimado**: no pude correr el benchmark en esta tanda. Medirlo
  en la fase 1 y poner un tope de conversaciones a la vez (por ejemplo 4).
- VAD: con PCM decodificado alcanza uno de energía con piso de ruido
  adaptativo + 700 ms de silencio + tope de 15 s. Arrancar con el modo
  `manual` (botón o toque) y sumar el `auto` después.

**Voz (TTS)**: Edge TTS (lo está sumando otro agente). Si puede pedir
`ogg-24khz-16bit-mono-opus` (**a verificar**: el servicio no es oficial y
rechaza algunos formatos), los paquetes Opus se sacan del Ogg y se mandan
tal cual: cero CPU. Si solo da mp3: mp3 → PCM (`hajimehoshi/go-mp3`, puro Go)
→ 48 kHz → `pion/opus` a 20 ms. Riesgo: Edge TTS es un servicio de Microsoft
sin API pública ni términos para esto; ya se rompió una vez (el token
`Sec-MS-GEC`). Si se cae: el texto sale igual en la pantalla (`tts
sentence_start`) y quien tenga ElevenLabs usa su voz (pedirle Opus si lo
ofrece, **a verificar**; si no, mp3 y el mismo camino).

**Cerebro**: lo mismo que `POST /v1/bichos/{id}/charla` (`datosCharla`, el
tope de 60 por día, la personalidad de ese robot). La `expresion` que elige se
traduce a una emoción de xiaozhi (tabla fija en el código) y va en `llm`.

### b) Tomar del firmware de xiaozhi para el nuestro

| Qué | Sirve | Cómo |
| --- | --- | --- |
| Pines y armado de la placa del cubito (`config.h`, `.cc`) | Sí, mucho | Copiar con el aviso MIT a `firmware/perfiles/cubito/` |
| Arranque de ES8311 + ES7210 | Sí | Con `esp_codec_dev` (Espressif) o portando sus registros |
| AFE de ESP-SR (AEC + VAD) | Sí, es lo que hace andar la charla con el parlante sonando | Copiar la configuración de `afe_audio_engine.cc` |
| WakeNet / MultiNet | Con cuidado | WakeNet no tiene español; MultiNet solo inglés y chino (el nombre del robot "aproximado" en fonemas ingleses). Una palabra propia la entrena Espressif (pago). Arrancar con **tocar para hablar** |
| La interfaz LVGL y los emojis | No | La cara es la nuestra (`nucleo/rostro.cpp`) |
| Su OTA y su activación | No | Las nuestras son firmadas |

- **Licencias**: xiaozhi es **MIT** (se puede usar comercialmente; hay que
  dejar el aviso de copyright en lo copiado). ESP-SR es "ESPRESSIF MIT": libre
  pero **solo en chips de Espressif** (nos va bien), y los modelos y parte de
  `esp_audio_codec` vienen compilados. Nada de esto obliga a abrir nuestro
  código.
- **Nuestro build**: `espressif32@6.9.0` es Arduino 2.0.17 sobre ESP-IDF 4.4.
  El código de xiaozhi pide IDF 6 (C++23, `std::expected`) y no compila tal
  cual. Opciones, de menos a más trabajo: (1) para el perfil del cubito,
  pasar a Arduino 3.x (plataforma de la comunidad `pioarduino`, IDF 5.x), que
  trae la librería `ESP_SR` y `esp_codec_dev`, sin tocar la 1.47B ni el
  broche; (2) el cubito en ESP-IDF puro con el núcleo como componente (el
  núcleo no depende de Arduino). **Decidirlo cuando esté la placa**, no antes.
- Un "fork de xiaozhi con nuestro núcleo como pantalla" daría la cara viva en
  100+ placas, pero sería mantener un fork de un repo que cambia todos los
  días. Por ahora no.

### c) Usar xiaozhi.me (el servidor oficial)

Honestamente, **no conviene** para un producto:

- **Privacidad**: el audio de la casa iría a un servidor en China (lo opera
  la empresa del autor; el dominio por defecto es `api.tenclass.net`). Los
  modelos son Qwen y DeepSeek.
- **Dependencia y control**: es gratis "para usuarios personales" sin
  compromiso de que siga así; no encontré términos de uso para integrarlo en
  un producto comercial ni una política de privacidad publicada en inglés o
  español. Y el servidor al que apunta un aparato puede reflashearlo (ver la
  OTA sin firma).
- Lo único que tendría sentido: usarlo **en el banco de pruebas** para
  comparar cómo se comporta un aparato contra el oficial mientras armamos la
  puerta. Su "punto de acceso MCP" (que un servidor MCP nuestro se conecte a
  la cuenta de xiaozhi.me de un usuario) es posible y gratis, pero suma poco y
  manda los datos allá: no lo haría.

## El plan, por fases

**Fase 1 — la puerta, chica (una semana de trabajo)**, `api/internal/xiaozhi`:

1. `POST /xiaozhi/ota/` y `POST /xiaozhi/ota/activate`: `server_time` (hora de
   Argentina), `firmware` = la misma versión que trae (nunca actualiza), y
   `activation {code, message}` hasta que el dueño tipee el código en la
   consola; después, `websocket {url, token}` (el token: un secreto por
   aparato, solo su hash en la base, como las claves MQTT).
2. `GET /xiaozhi/v1/` (WebSocket; `gorilla/websocket` ya está en `go.sum`):
   `hello`, `listen` en modo `manual`, Opus → Ogg → Groq → `stt` → charla →
   `llm` + `tts sentence_start` → Edge TTS → Opus → `tts stop`. `abort`
   corta. Tope de sesiones a la vez y por usuario.
3. El robot de voz en la base: un bicho con perfil `voz` (sin núcleo en el
   aparato, sin MQTT). La consola lo muestra con su cara viva en la web.
4. Tests: mensajes grabados (`hello`, `listen`, Opus de `testdata`), un
   aparato falso en Go para el e2e, y el benchmark de CPU de Pion en el VPS.
5. Demo: el cubito (o cualquier placa con xiaozhi) con la URL de OTA de la
   API en local.

**Fase 2 — que se sienta Sintética**:

- Modo `auto` (VAD en la nube) y `realtime` si la placa tiene AEC.
- MCP del aparato: `initialize` + `tools/list` al conectar; volumen y brillo
  en la consola y como herramientas de la charla.
- `notify` para recordatorios, despertador y avisos: la API sirve el Ogg Opus
  (con Edge TTS o ElevenLabs) y la placa lo dice sola.
- El gemelo del núcleo en la API para los robots de voz (ánimo, memoria,
  sueño): su expresión manda la emoción de `llm`. Medir la CPU antes.

**Fase 3 — el cubito con nuestro firmware y la misma voz**:

- Perfil `cubito` con los pines, los códecs y el AFE tomados de xiaozhi (MIT,
  con el aviso), en Arduino 3.x o ESP-IDF según lo que se decida.
- La voz en tiempo real por el mismo WebSocket de la puerta: un solo
  servidor de voz para todos.

## Links

- Firmware: https://github.com/78/xiaozhi-esp32 (MIT): `docs/websocket.md`,
  `docs/mqtt-udp.md`, `docs/mcp-protocol.md`, `docs/mcp-usage.md`,
  `docs/notify.md`, `docs/glyph-push.md`, `docs/custom-board.md`,
  `main/ota.cc`, `main/application.cc`, `main/mcp_server.cc`,
  `main/audio/README.md`, `main/boards/waveshare/esp32-s3-touch-lcd-1.54/`.
- Portal de wifi: https://github.com/78/esp-wifi-connect (MIT).
- Servidores abiertos: https://github.com/xinnan-tech/xiaozhi-esp32-server
  (MIT, Python; su `core/api/ota_handler.py` arma la respuesta de la OTA),
  https://github.com/hackers365/xiaozhi-esp32-server-golang (MIT, Go: para
  leer cómo resolvieron lo mismo), https://github.com/78/xiaozhi-mqtt-gateway.
- Opus en Go: https://github.com/pion/opus, https://github.com/hraban/opus.
- Groq: https://console.groq.com/docs/speech-to-text,
  https://console.groq.com/docs/rate-limits.
- ESP-SR: https://github.com/espressif/esp-sr (ESPRESSIF MIT).
- Componentes de Espressif: https://components.espressif.com/components/espressif/esp_codec_dev,
  https://components.espressif.com/components/espressif/esp_audio_codec.
