# SintéticaOS en el VPS de Grupo Vimo

Corre en `/opt/sintetica` (usuario `deploy`). Las imágenes las compila GitHub
Actions (`.github/workflows/imagenes.yml`) y se bajan de GHCR. Lo compartido
del VPS (Caddy, UFW, puertos, copias) está anotado en `/opt/VPS.md`.

| Qué | Dónde |
| --- | --- |
| La web | https://sintetica.grupovimo.com.ar → 127.0.0.1:3600 |
| La API | https://api-sintetica.grupovimo.com.ar → 127.0.0.1:4400 |
| MQTT (bichos) | mqtts://mqtt-sintetica.grupovimo.com.ar:8883 (sin Cloudflare): el broker está adentro de la API |
| Mirar el MQTT (el superusuario `api`) | `mosquitto_sub -h mqtt-sintetica.grupovimo.com.ar -p 8883 --capath /etc/ssl/certs -u api -P <MQTT_CLAVE_API> -t 'bicho/#' -v` |
| Los mails (hasta tener SMTP) | `ssh -N -L 8025:127.0.0.1:8025 deploy@187.41.102.190` → http://localhost:8025 |

## La carpeta

```
/opt/sintetica/
  compose.yml          (de produccion/compose.yml)
  .env                 (600; de produccion/.env.ejemplo, con las claves)
  claves/              ota-privada.pem (la firma de las OTA) y ota-publica.pem
  certs/               mqtt.crt y mqtt.key (los renueva el timer sintetica-cert;
                       la clave legible por el uid 10001, el de la API)
  pg/                  los datos de Postgres
```

## Primera vez

1. `docker login ghcr.io` (como `deploy`, queda en
   `/home/deploy/.docker/config.json`) con un token que crea Nahuel con **solo**
   `read:packages` (PAT clásico o fine-grained): el repo es privado.
   `mkdir -p pg claves` como `deploy`.
2. Copiar `compose.yml` y armar `.env` con claves nuevas (ver `.env.ejemplo`).
3. La clave de firma de las OTA, una sola vez:
   ```sh
   docker run --rm -u root -v /opt/sintetica/claves:/claves \
     --entrypoint sintetica-api ghcr.io/nawesan12/sintetica-api claves /claves
   chown -R 10001 claves && chmod 600 claves/ota-privada.pem
   ```
   **Perderla obliga a reflashear por USB todos los bichos**: hay una copia en
   la Mac de Nahuel (`~/Trabajo/vimo/credenciales-sintetica/`) y otra cifrada
   en el backup de `vimo-copia`. La pública (`ota-publica.pem`) es la que se embebe en el
   firmware de producción.
4. `docker compose pull && docker compose up -d`.
5. `curl -s https://api-sintetica.grupovimo.com.ar/salud`.

## Actualizar

```sh
cd /opt/sintetica
docker compose exec postgres pg_dump -U sintetica sintetica | gzip > ~/sintetica-$(date +%F).sql.gz
VERSION=<sha> docker compose pull && VERSION=<sha> docker compose up -d
```

El certificado de MQTT lo renueva el timer `sintetica-cert` (lego, DNS-01) en
`certs/`. La API lo relee sola en el próximo handshake (mira la fecha de los
archivos): el timer **no tiene que reiniciar nada**, solo dejar
`certs/mqtt.key` legible por el uid 10001 (`chown 10001 certs/mqtt.key`,
`chmod 600`). Si el archivo nuevo está roto, la API sigue con el anterior y lo
anota en el log.

### Pasar de EMQX al broker de la API (una vez)

Desde el 8/11/2026 el broker MQTT está adentro de la API (ver `docs/nube.md`,
"El broker"); el servicio `emqx` ya no existe.

1. Copiar el `compose.yml` nuevo.
2. `chown 10001 certs/mqtt.key && chmod 600 certs/mqtt.key` (antes lo leía el
   usuario de EMQX).
3. En el timer `sintetica-cert`: sacar el reinicio de EMQX (y lo de
   `certs/renovado`) y sumar el `chown 10001` de la clave después de cada
   renovación.
4. `docker rm -f sintetica-emqx-1` (libera el 8883) y enseguida
   `VERSION=<sha> docker compose up -d --remove-orphans`. Los bichos se
   reconectan solos (reintentan cada 5 s).
5. Borrar `emqx.conf`, `emqx-datos/` y `secretos/emqx-api-claves.txt`, y en el
   `.env`, `EMQX_CLAVE_PANEL`, `EMQX_COOKIE` y `EMQX_API_CLAVE`. Ya no hay túnel
   al 18083. UFW sigue igual (8883 abierto).

`VERSION` en el `.env` va siempre fijada a un sha probado (nunca `latest`), así
un `pull` no sube nada sin querer y volver atrás es cambiar el sha.

## Caddy (lo maneja `/etc/caddy/sitios/sintetica.caddy`)

La API toma la IP del usuario de `X-Forwarded-For` (para sus límites). Detrás de
Cloudflare hay que reescribirla con la IP real, para que no sea la del proxy ni
se pueda falsificar:

```
reverse_proxy 127.0.0.1:4400 {
	header_up X-Forwarded-For {http.request.header.CF-Connecting-IP}
	flush_interval -1
}
```

La web hace lo mismo con `ADDRESS_HEADER=CF-Connecting-IP` (en el compose).

## Lo que falta

- SMTP de verdad (cuando esté el dominio de Sintética): hasta entonces el
  código para entrar se lee en Mailpit por el túnel.
- Sumar `sintetica-postgres-1` a `vimo-copia`.
