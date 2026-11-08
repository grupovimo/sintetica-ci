# Levantar SintéticaOS en tu Mac

Todo corre en Docker: la API (con el broker MQTT adentro), la web, Postgres, Mailpit
(los mails) y dos bichos virtuales (una 1.47B y un broche). Nada sale de tu
máquina.

## Lo que hace falta

- **Docker Desktop** (con Apple Silicon o Intel, da igual).
- Para el firmware de una placa real: [PlatformIO](https://platformio.org)
  (`brew install platformio`).
- Para las apps: Xcode (iOS) o Android Studio.
- Node 22 y Go 1.24 solo si querés correr los tests o la web fuera de Docker.

## Primera vez

```sh
cp .env.example .env
docker compose up --build
```

La primera vez tarda unos minutos (compila la API, la web y el núcleo a
WebAssembly). Cuando termina:

| Qué | Dónde |
| --- | --- |
| La web | http://localhost:5173 |
| Los mails (los links para entrar) | http://localhost:8025 |
| Bicho virtual (1.47B) | http://localhost:8090 |
| Bicho virtual (broche) | http://localhost:8091 |
| El broker MQTT | tcp://localhost:1883 (y la IP de la Mac en la red de la casa, para las placas y el celu); para mirar: `mosquitto_sub -h localhost -u api -P cambiame-api -t 'bicho/#' -v` |
| La API | http://localhost:8080 (`/salud` dice si anda) |

El servicio `claves` genera, una sola vez, el par de claves de firma de las OTA
en `claves/` (la privada nunca va al repo). El firmware embebe la pública al
compilar.

## Probarlo de punta a punta

1. Abrí la web, poné cualquier email y tocá **Mandame el link**.
2. En Mailpit (localhost:8025) abrí el mail y tocá **Entrar** (o escribí el código).
3. **Sumar un bicho** → **Bichos virtuales** → **Sumar**. En unos segundos
   aparece en Mis bichos, conectado.
4. Abrí http://localhost:8090 al lado: es la pantalla del bicho virtual (se
   puede tocar). Cambiá algo en la web (Config, una cara, una animación, una
   regla) y mirá cómo le llega.
5. **El "otro celu"**: abrí la web en otra ventana privada (o en el celu, ver
   abajo) con el mismo email, cambiá algo, y el bicho se actualiza solo.
6. **OTA**: poné tu email en `ADMIN_EMAILS` (en `.env`; también sirve
   `@tudominio.com`), reiniciá la API (`docker compose up -d api`) y en
   **Firmwares** subí cualquier archivo para el perfil `virtual` con una
   versión nueva. En el bicho → **Firmware** → **Instalar**: lo baja, verifica
   la firma, reinicia y queda confirmado a los 30 s. Un archivo que empieza con
   `FALLAR` simula un firmware roto: el bicho vuelve solo a la versión anterior.

7. **El motor de vida**: en el bicho, la pestaña **Vida** muestra lo que siente
   y lo que está haciendo (en vivo), deja mandarle eventos (una caricia, una
   sacudida fuerte…) y prender la superposición de depuración en su pantalla;
   abajo, su diario (lo trae cada 5 min). **Config** tiene la personalidad
   (presets y seis rasgos) y su horario; **Expresiones** y **Comportamientos**,
   los editores (con vista previa y "probar en el bicho"). Para verlo vivir
   rápido, el simulador (`npm run sim`) tiene "un día en 2 minutos".

Hay una prueba automática que hace todo eso (y la memoria y la mudanza):

```sh
ADMIN_EMAILS=@ejemplo.com docker compose up -d --build
go test -tags e2e -count=1 -v ./api/e2e/
```

## Con el celu y una placa de verdad

El celu y la placa tienen que llegar a tu Mac por la red de tu casa.

1. Buscá la IP de la Mac (Ajustes del Sistema → Red, algo como `192.168.1.10`)
   y ponela en `.env`: `IP_LOCAL=192.168.1.10` (y `IP_WEB` si vas a abrir la
   web desde el celu). `docker compose up -d` de nuevo.
2. **Firmware** (la 1.47B):
   ```sh
   cd firmware && pio run -e dev-147b -t upload
   ```
   Compila con la clave pública de `claves/`. Para una versión nueva:
   `PLATFORMIO_BUILD_FLAGS='-DSINTETICA_VERSION=\"0.2.1\"' pio run -e dev-147b`
   y subí `firmware/.pio/build/dev-147b/firmware.bin` en Firmwares.
3. **La app** (Android o iOS): **Entrar a tu cuenta** → servidor
   `http://192.168.1.10:8080` → tu email → el código del mail (o el link, si
   abrís el mail en el celu).
4. **Vincular un bicho** (el QR de la placa) → en el bicho, **WiFi y nube**:
   pasale tu WiFi y tocá **Sumar a mi cuenta**. La app pide a la API una clave
   para el bicho, se la pasa por BLE y espera a que se conecte. Desde ese
   momento lo manejás desde la web o desde cualquier celu con tu cuenta.

El broche no tiene WiFi: se maneja por BLE desde la app y sus OTA van por BLE
(la app baja el firmware firmado de la API y se lo manda).

## Comandos útiles

```sh
docker compose logs -f api            # lo que pasa (los links de los mails también salen acá)
docker compose up -d --build api web  # después de cambiar código
docker compose down                   # apagar (los datos quedan)
docker compose down -v                # apagar y borrar todo (cuentas, bichos)
```

Si un puerto está ocupado en la Mac, cambialo en `docker-compose.yml` (el de
la izquierda en `ports`). Postgres queda en `localhost:5433` por si lo querés
mirar (`psql postgres://sintetica:sintetica@localhost:5433/sintetica`).

## Fuera de Docker (para desarrollar)

```sh
docker compose up -d postgres mailpit claves bicho-virtual
# La API, con el broker MQTT adentro (en :1883: pará la del compose antes,
# docker compose stop api)
MQTT_CLAVE=cambiame-api DATABASE_URL=postgres://sintetica:sintetica@localhost:5433/sintetica?sslmode=disable \
  CLAVE_FIRMA=claves/ota-privada.pem DESARROLLO=1 go run ./api/cmd/sintetica-api
# La web, con recarga en vivo
cd web && npm install && API_INTERNA=http://localhost:8080 npm run dev
```

Los tests: `npm test` (formato, núcleo, simulador), `go test ./api/... ./firmware/api/cgo/`
(con `SINTETICA_PRUEBAS_DB` apuntando a un Postgres vacío para los de la base),
`cd web && npm run check`, y en `android/`, `./gradlew testDebugUnitTest`.
