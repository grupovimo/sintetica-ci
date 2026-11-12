# Plan: qué se vende y en qué orden (12/11/2026)

## Contexto

La etapa 6 dejó mucho hecho (núcleo, motor de vida v3, nube en producción, web,
iOS en TestFlight, la 1.47B andando, /admin con preventa), pero el plan quedó
desparramado entre `docs/pendientes.md`, `docs/especificacion.md`, `docs/xiaozhi.md`
y memorias. Nahuel quiere **vender asistentes**. Decidido hoy (12/11):

- **Lo primero que se vende: el asistente de escritorio** (Mac y Windows). Es
  software: sin stock ni envíos, se vende en todo el mundo y no depende de placas.
  La preventa de la Unidad sigue en paralelo (solo falta el token de MP).
- **Cobro: pago único** (licencia por cuenta, con actualizaciones).
- **La v1 del escritorio trae**: cara viva flotante, charla y voz, ayuda en la
  compu, y el control de los robots físicos (la consola adentro).

## Las líneas de producto (una sola cosa por debajo: el núcleo)

| Producto | Qué es | Estado | Cómo se cobra |
|---|---|---|---|
| **Escritorio** (nuevo) | La cara viva en la compu + asistente | A construir (fase 1) | Pago único |
| **Unidad** (1.47B) | El robot de escritorio físico | Anda; falta carcasa y token MP | Preventa con seña (MP) |
| **Cubito** (S3 1.54, voz) | El asistente de hardware con voz | Placa recién llegada | Preventa después |
| **Broche** (C3) | El de llevar puesto | Sin probar en placa | Después |
| **Robot-celu** | Un Android viejo como robot | Anda en local | Gratis (embudo) o extra |
| **Marketplace** | Caras, personalidades, voces | Diseño pendiente | Por paquete, con reparto |

Todo es **el mismo robot en la nube**: el escritorio se suma como un robot más
(como el Moto con `android-bicho/`), así recordatorios, Telegram, MCP, charla,
amigos y la consola andan sin rehacer nada.

## Fase 0 — Destrabar (antes de cualquier cosa, esta semana)

1. **CI** (decidido 12/11): la cuenta personal `nawesan12` no se toca. Lo que
   necesita runners (macOS, Windows) va en **repos públicos de `grupovimo` con
   solo lo que se compila**, como `herramientas/publicar-ios.sh` →
   `grupovimo/sintetica-ios`; el escritorio, en `grupovimo/sintetica-escritorio`
   (que además aloja las descargas). El repo entero no se hace público: los
   perfiles y los docs dicen qué placas se usan. Las pruebas rápidas (núcleo,
   Go, web) corren en la Mac.
2. Pasar el CI de la v3 (RAM del broche) y desplegar.
3. Token de Mercado Pago en `/opt/sintetica/.env` → preventa de la Unidad viva.
4. Ordenar los docs: este plan pasa a `docs/plan.md` como única hoja de ruta;
   `docs/pendientes.md` queda solo para lo chico; se borra `docs/contrato-v1.9.md`.

## Fase 1 — El asistente de escritorio (lo que se vende primero)

**Tecnología: Tauri v2** (Rust + la webview del sistema). Motivo: una sola base
para Mac y Windows, reusa **tal cual** la web (SvelteKit, la consola) y el núcleo
en WebAssembly (`web/src/lib/nucleo.ts`, `sim/web/sim.js` → `crearSimulador`), que
ya corre un robot entero. Instaladores chicos (~10 MB), auto-actualización firmada,
bandeja, atajos globales y arranque con el sistema vienen de fábrica.

Piezas:

1. **Robot de software en la nube** (`api/internal/servidor/bichos.go`): un
   reclamo para robots sin BLE (hoy solo existe `reclamar-virtual`, de
   desarrollo). La app entra con Telegram (como `android-bicho/herramientas/sumar.py`),
   reclama un robot perfil `escritorio` y recibe sus credenciales MQTT. El perfil
   nuevo declara "lo que trae" (pantalla, micrófono, parlante; sin IMU) para la
   consola.
2. **El anfitrión** (`escritorio/`, nuevo, como `android-bicho/`):
   - Ventana transparente sin bordes, siempre arriba, arrastrable; tamaño chico
     / mediano / se esconde en la bandeja. El núcleo dibuja en un canvas.
   - Hal del escritorio en Rust: hora, guardado (config, memoria, diario) en la
     carpeta de la app, MQTT con TLS (`rumqttc`) contra `mqtt-sintetica`.
   - **Sentidos de la compu** → eventos del núcleo (datos, no código): actividad
     de teclado/mouse (presencia, `esperando`), hora de dormir, pantalla
     bloqueada, batería, la ventana activa (solo el nombre de la app),
     notificaciones (Windows: `UserNotificationListener`; Mac: lo que permita).
3. **Charla y voz**: atajo global para hablar o escribir; el motor propio por
   cgo en la nube y, si no entiende, la IA (la del usuario o la del servidor);
   la voz con Edge TTS / ElevenLabs / voz propia, ya existentes. El oído:
   Whisper de Groq (como en `docs/xiaozhi.md`).
4. **Ayuda en la compu**: recordatorios, agenda, pomodoro, hábitos y resumen del
   día ya existen en el núcleo y la nube; se suman acciones locales que la charla
   o la IA disparan (abrir una app o un link, buscar un archivo, copiar), con una
   lista cerrada de acciones y confirmación.
5. **La consola adentro**: la web en modo app para manejar este robot y los
   físicos; pasarle el WiFi a una placa por Bluetooth desde la compu (después de
   la v1 si complica).
6. **Venta y licencia (pago único)**:
   - Mercado Pago (Argentina) ya integrado en `/admin` (preventa) → reusar para
     la compra; afuera, **Lemon Squeezy** (cobra impuestos por nosotros, sin costo fijo).
   - La licencia queda en la cuenta de la nube (tabla nueva junto a
     `020_control.sql`); la app la chequea al entrar. Sin licencia: prueba de 7
     días con todo.
   - Página de compra y descarga en la landing; las ventas aparecen en /admin.
7. **Firmar los instaladores**: Mac con el Developer ID de la cuenta de Apple
   (ya existe para TestFlight) + notarización; **Windows sin firma** al
   principio (decidido 12/11: SmartScreen avisa; Azure Trusted Signing, ~USD
   10/mes, cuando haya ventas).

Listo cuando: alguien en Windows y en Mac lo compra, lo instala, entra con
Telegram, la cara vive en el escritorio, le habla y contesta, y le pone un
recordatorio desde Telegram que suena en la compu.

## Fase 2 — El cubito (el asistente de hardware, en paralelo a la fase 1)

**La placa llegó (12/11)**: la Waveshare ESP32-S3-Touch-LCD-1.54 de
MercadoLibre (ARS 60.900, vendedor INNOMAKEBRIGHT). Confirmado en la
publicación: S3R8 (8 MB PSRAM, 16 MB flash), 240×240 ST7789 + táctil CST816,
ES8311 + ES7210 con cancelación de eco, 2 micrófonos, NS4150B + parlante,
QMI8658, ranura TF, **3 botones**, carga de batería (MX1.25) y — lo importante —
**viene en un gabinete cúbico blanco de 46×46×22,5 mm con batería incluida**.
O sea: el cubito es casi un producto terminado sin diseñar carcasa; se puede
vender antes que la Unidad. Cargarla en /admin (componente con costo y link).

En orden (`docs/xiaozhi.md`):
0. Enchufarla, sacar su id (MAC) y backup de la flash de fábrica con esptool
   (por si hay que volver).
1. Día 1: flashear xiaozhi tal cual y apuntarlo a la nube para probar audio.
2. La "puerta xiaozhi" en la API (`api/internal/xiaozhi`, WebSocket + Opus) —
   sirve también para la voz del escritorio.
3. Perfil `firmware/perfiles/cubito-154/` con nuestro núcleo; voz con el
   protocolo de xiaozhi.
4. Preventa del cubito con la misma maquinaria de la Unidad.

## Fase 3 — Lo que hace crecer la venta

- Marketplace de caras y personalidades (paquetes firmados por la nube).
- M8: el robot cerrado (secure boot, flash cifrada) antes de vender hardware en serie.
- Las apps al día con la v3 (diario, etapa) y "Sumar mail".

## Verificación

- Fase 1: `npm test` y `go test` como siempre; CI que arma `.dmg` y `.msi`;
  probar la compra con una venta real chica (MP y Lemon Squeezy en modo prueba);
  instalar en la Mac de Nahuel y en una Windows (VM o compu prestada).
- Fase 2: la placa con el firmware nuestro, demo de voz con cara.
