# Contrato de la v1.9: amigos, agenda, presencia, hábitos, avisos, música y charla

Lo que se reparte entre las partes (núcleo, nube, web, apps y el robot-celu)
para trabajar en paralelo. Cuando esté todo hecho, esto pasa a
`docs/protocolo-ble.md`, `docs/nube.md` y `docs/motor-de-vida.md`, y este
archivo se borra.

## Comandos nuevos (característica Comando y `bicho/{id}/cmd`)

| Comando | Cuerpo | Qué hace en el robot |
| --- | --- | --- |
| `0x47` Toque | `[u8 tipo: 0 toque, 1 abrazo, 2 beso, 3 carita][n][de: 1 a 24 bytes][n][expresión, 0 a 24 bytes]` | Un toque de un amigo: lo despierta suave, reacciona según el tipo (abrazo: se achica y se pone colorado; beso: corazones; carita: pone esa expresión) y un globo con la frase de `toque-recibido` (`{de}`). Durante 30 s, una caricia o un toque **lo devuelve** (evento `devolver-toque`). |
| `0x48` Agenda | `[u16 minutos que faltan][u16 duración en minutos][n][título: 1 a 40 bytes]` | El próximo evento del calendario. Con 0 a 15 minutos, globo de `agenda` (`{titulo}`, `{minutos}`) y cara de apuro (comportamiento base `apurado`, con la partícula de un reloj si entra). El mismo marco con `minutos = 0xFFFF` es el resumen de mañana: globo de `agenda-manana` (`{titulo}` = el primero, `{cantidad}`). |
| `0x49` Presencia | `[u8 0 llegaste, 1 te fuiste]` | Del celu (por cercanía de Bluetooth) o de la nube. Llegaste: el "¡volviste!" de verdad (situación `volviste`, con el ánimo de cuánto tardaste). Te fuiste: globo de `te-vas` ("¿Llevás las llaves?"; el texto propio está en `presencia.texto`) y, un rato después, se aburre o se duerme antes. |
| `0x4A` Hábito | `[u8 índice 0-3][u8 0 hecho, 1 deshacer][u8 racha en días]` | Marca un hábito de `habitos` desde la consola, la app, Telegram o la IA. Hecho: se pone orgulloso (situación `habito-hecho`; con racha de 3 o más, `habito-racha` con `{racha}`). Deshacer: nada visible. |
| `0x4B` Música | `[u8 bpm (0 = no se sabe)][u8 flags: bit 0 está sonando][n][título, 0 a 40 bytes][n][artista, 0 a 24 bytes]` | Lo que suena (Spotify o el celu). Sonando: mueve la cabeza al ritmo (el bpm, o 100 si es 0), notas de música y, al cambiar de tema, un globo de `musica` (`{titulo}`, `{artista}`). Sin el bit: deja de bailar. |
| `0x44` Aviso (amplía la v1.7) | `[u8 tipo][n][app][n][contacto]` | Tipos nuevos: `2` pago, `3` mail, `4` github, `5` pedido, `6` otro. Para estos, `app` es el título corto (por ejemplo "$12.000" o "PR #12") y `contacto` quién (por ejemplo "Ana"). Cada uno tiene su situación (`pago`, `mail`, `github`, `pedido`, `aviso`) y su sonido (`pago`, `notificacion`). |

Los textos se cortan por bytes UTF-8 sin partir letras, como el resto.
Errores como siempre: `0x07` si no aplica, `0x08` si está mal armado.

## Eventos nuevos (al final del enum, después del 38 `mano-cerca`)

| | |
| --- | --- |
| `39` toque-recibido | llegó un `0x47` |
| `40` devolver-toque | lo devolviste con una caricia o un toque (el robot lo avisa con `0x9E`; la nube se lo manda al que lo mandó) |
| `41` agenda | llegó un `0x48` con 0 a 15 minutos |
| `42` llegaste | presencia 0 |
| `43` te-fuiste | presencia 1 |
| `44` habito | se marcó un hábito (el dato del registro = índice) |

Registro (`0x1E`): tipo `7` toque (dato = tipo de toque), tipo `8` hábito
(dato = índice).

## Situaciones nuevas de `frases.json`

`toque-recibido`, `toque-devuelto`, `agenda`, `agenda-manana`, `te-vas`,
`habito-hecho`, `habito-racha`, `habito-cortado`, `musica`, `pago`, `mail`,
`github`, `pedido`, `aviso`. Plantillas nuevas: `{de}`, `{titulo}`,
`{minutos}`, `{cantidad}`, `{racha}`, `{habito}`, `{artista}`.

## Config v2 nueva

```json
"habitos": [ { "nombre": "agua", "icono": "gota", "hora": "10:00", "dias": "LMXJV" } ],
"presencia": { "preguntar": true, "texto": "¿Llevás las llaves?" }
```

- `habitos`: hasta 4. `nombre` de 1 a 16 bytes (el texto que ve el usuario,
  con tildes); `icono` uno de `gota`, `libro`, `pesa`, `zen`, `pastilla`,
  `cama`, `fruta`, `estrella`; `hora` opcional (HH:MM: a esa hora pregunta
  "¿{habito}?" y un toque lo marca hecho, mantener = todavía no); `dias` como
  en los recordatorios.
- `presencia.preguntar` (por defecto `true`) y `presencia.texto` (0 a 40 bytes;
  vacío = las frases de `te-vas`).
- Medir el broche: todo lo nuevo en `Config` tiene que entrar (hoy 73,9 %).

## Comportamientos y sonidos base nuevos

- Comportamientos: `apurado` (mira para los costados, parpadea rápido),
  `orgulloso` (se infla, ojos felices), `bailar-ritmo` (el que usa la música; el
  ritmo lo pone el núcleo).
- Partículas, si entran: `reloj`, `nota` (ya está para los sueños).
- Sonidos (`Hal::sonido`): `toque`, `abrazo`, `beso`, `agenda`, `pago`,
  `habito`.

## Nube (rutas nuevas)

Amigos y toques:
- `POST /v1/amigos/invitacion` → `{codigo, link, vence}` (un código de 8
  letras, 48 h, un solo uso).
- `POST /v1/amigos {codigo}` acepta; `GET /v1/amigos`; `DELETE /v1/amigos/{id}`.
- `PUT /v1/amigos/{id} {robots: [ids míos que reciben sus toques]}` (por
  defecto, todos los míos).
- `POST /v1/amigos/{id}/toque {tipo, expresion?, desde?: id de mi robot}` →
  el `0x47` a sus robots habilitados (con `de` = mi nombre o el de mi robot).
  Tope: 30 por hora por par.
- El evento `devolver-toque` de un robot se lo manda de vuelta al último que le
  tocó (en 10 min). Si el amigo tiene Telegram vinculado y ningún robot
  conectado, le llega por Telegram.
- Eventos SSE: `toque` (`{de, tipo, robot}`).

Agenda:
- `PUT /v1/agenda {ics: "https://…ics"}` (la "dirección secreta en formato
  iCal" de Google Calendar, o cualquier `.ics`; se valida como los webhooks:
  https, sin IPs privadas). `GET /v1/agenda` (los próximos 10), `DELETE`.
- Se lee cada 5 min; RRULE diarias, semanales y mensuales con BYDAY, COUNT,
  UNTIL y EXDATE. Avisa `config.agenda.aviso` minutos antes (por defecto 10;
  se guarda del lado de la nube) a todos los robots conectados del usuario. A
  la hora de `a-dormir` (o 22:00), el resumen de mañana.
- Cuando nuevos para automatizaciones: `agenda`.

Presencia:
- `POST /v1/bichos/{id}/presencia {llegaste: bool}` (las apps, por cercanía
  de Bluetooth del celu; el robot-celu, por su cuenta).
- La nube anota llegadas y salidas y aprende tus horarios: `GET
  /v1/bichos/{id}/horarios` → `{llegas: "19:10", salis: "08:20", dormis:
  "00:30", seguridad: 0..1}` (mediana de los últimos 14 días).
- Cuandos nuevos: `llegaste`, `te-fuiste`.

Hábitos:
- `POST /v1/bichos/{id}/habitos/{i} {hecho: bool}` → el `0x4A` con la racha
  que calcula la nube.
- `GET /v1/bichos/{id}/habitos` → `[{nombre, icono, racha, dias: {"YYYY-MM-DD": true}}]`
  (los últimos 60 días, para el calendario de puntitos).
- Cuando nuevo: `habito`. Telegram `/habito`; MCP `habitos`.

Avisos entrantes (sin cuentas de terceros):
- `POST /v1/entrantes/{token}` (un token secreto por usuario: `GET/POST
  /v1/entrantes` lo muestra o lo regenera). Acepta:
  - JSON propio `{tipo: "pago|mail|github|pedido|otro", titulo, de, robot?}`;
  - un webhook de GitHub tal cual (`X-GitHub-Event`: PR, issue, review,
    estrellas, CI fallido), firmado con el mismo token como secreto;
  - una notificación de Mercado Pago (`type: payment`): sin token de MP dice
    "¡Te pagaron!"; con `MP_ACCESS_TOKEN` del usuario guardado, el monto.
- Manda el `0x44` con el tipo nuevo a sus robots y el evento SSE `entrante`.

Música:
- Spotify (OAuth con `SPOTIFY_CLIENT_ID`/`SPOTIFY_CLIENT_SECRET` en el `.env`;
  sin esas claves, apagado y la consola no lo ofrece): cada 20 s mira qué suena
  y manda el `0x4B` al cambiar (bpm de la API de audio de Spotify si está; si
  no, 0).
- `POST /v1/bichos/{id}/musica {sonando, titulo, artista, bpm?}` para el celu
  (Android lee la sesión de medios con el NotificationListener).

Charla con IA:
- `POST /v1/bichos/{id}/charla {texto}` → la respuesta del robot (hasta 60
  bytes, en su personalidad, con su nombre, el dueño, el clima y su memoria) y
  la manda con `0x40` Decir con una expresión que elige. Con
  `ANTHROPIC_API_KEY` en el `.env` (modelo `claude-haiku-5-5`); sin la clave,
  `503` y la consola no lo ofrece. Tope: 60 por día por usuario.
- Telegram: un mensaje libre (que no sea comando) a @sinteticalabs_bot es una
  charla con tu robot.
- El robot-celu (Moto E4): voz → texto en el celu → `charla` → globo y voz.

MCP: herramientas nuevas `tocar` (a un amigo), `amigos`, `agenda`,
`habitos`, `horarios`, `musica`, `entrantes`.
