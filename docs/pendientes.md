# Pendientes (al 9/11/2026)

Lo que quedó abierto al cerrar la etapa 6. Ordenado por lo que más destraba.

## Para Nahuel

- **La preventa del cubito** (10/11): pasar el access token de producción de
  Mercado Pago (`APP_USR-…`) para ponerlo como `MP_ACCESS_TOKEN` en
  `/opt/sintetica/.env` (y `docker compose up -d api`). Después, en
  https://sintetica.grupovimo.com.ar/admin/preventa: precio, precio final, seña,
  cupos y entrega, y prenderla. El aviso de pago llega solo (cada preferencia
  manda su `notification_url`); no hace falta configurar webhooks en el panel de
  Mercado Pago. Probar una seña real chica antes de anunciarla.
- **Cargar el negocio** en /admin: componentes con costo, proveedor y link, la
  lista de materiales de la Unidad y el Broche, y las unidades que ya existen.

- **El CI**: arreglar el pago o el límite de Actions de `nawesan12`, o hacer
  público el espejo y empujar la rama:
  ```sh
  GH_TOKEN=$(gh auth token -u grupovimo) gh repo edit grupovimo/sintetica-ci --visibility public --accept-visibility-change-consequences
  sh herramientas/publicar-ci.sh
  ```
  (El permiso de Claude Code bloquea publicar código afuera; para que lo haga
  Claude, sumar un permiso para `sh herramientas/publicar-ci.sh`.)
- **Entrar a producción** con Telegram (https://sintetica.grupovimo.com.ar/entrar)
  y pasar su id de Telegram a `ADMIN_TELEGRAM` en `/opt/sintetica/.env` (lo
  saca Claude de la base) para ser administrador.
- **El Moto E4 a producción**: Moto y Mac en el mismo wifi,
  `python3 android-bicho/herramientas/sumar.py`, abrir el link de Telegram en
  el celu y Start; después aparece en la app de iOS (build 15).
- **El Moto E4 de noche**: quedó callado (`adb shell am broadcast -a
  sintetica.EVENT --es tipo silencio --ez si false` lo vuelve a hacer hablar) y
  con los recordatorios y avisos de cuidado apagados (config v73 de la nube
  local; el historial vuelve a la v72).
- **Probar en el iPhone** (build 15): la foto viva con el Moto, subir una
  imagen, dibujos, tamaño de la cara, entrar con Telegram, la charla y, si
  conecta ElevenLabs, la conversación (eco).
- **Claves opcionales**: Groq gratis (`CHARLA_URL`, `CHARLA_CLAVE`,
  `CHARLA_MODELO`) para la charla libre en producción; Spotify si se quiere.

## Para seguir

1. **CI de la v3** (motor de vida) y de la robustez de la nube: que pasen,
   sobre todo la RAM del broche (la v3 suma ~2,4 KB; el segmento DRAM está
   justo). Si no entra, achicar como se hizo con la Regla. Recién después,
   desplegar a producción (`sh herramientas/desplegar-nube.sh`).
2. **La v3 en la nube y las consolas**: guardar el diario (`0xCD`, pedir
   `0x4D` al conectarse), avisar el evento 45 `crecio`; en la web y las apps,
   el módulo Diario, la etapa y los días juntos, `fechas` en "Su día", y en el
   Laboratorio los comportamientos y variables nuevas (`etapa`, `dias`, `solo`,
   `bajon`, `lluvia`, `esperando`), las miradas `particula`/`puerta` y la
   partícula `garabato`.
3. **"Sumar mail"** y volver a leer la sesión cuando la cuenta se une, en la
   web y las apps (la API ya está: `POST /v1/sesion/email`).
4. **Editor de animaciones** en iOS (la web lo tiene).
5. **El perfil del cubito** cuando llegue la placa (`docs/xiaozhi.md`: una sola
   placa, nuestro firmware, el protocolo de voz de xiaozhi; el robot como caché
   con la biblioteca afuera).
6. **M8, el robot cerrado**: ESP-IDF, secure boot, flash cifrada, paquetes
   firmados (ver `docs/especificacion.md`).
7. **Marketplace** de rostros y personalidades (idea de negocio; diseño
   pendiente).
8. Detalles: la sonrisa de "contento" tiene el mismo corte de halo que se
   arregló en los ojos; el APK de Android pesa 96 MB (achicar a arm64 al ir a
   la Play Store); `assetlinks.json` para que los links abran la app (con la
   clave definitiva); `docs/contrato-v1.9.md` se puede borrar (ya está en los
   docs de siempre); el desborde del rótulo "Copiar" de la URL del MCP en el
   celu; la licencia GPL del port de `edgetts` si algún día se libera la nube.
