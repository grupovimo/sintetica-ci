# SintéticaOS en el VPS de Grupo Vimo

Corre en `/opt/sintetica` (usuario `deploy`). Las imágenes las compila GitHub
Actions (`.github/workflows/imagenes.yml`) y se bajan de GHCR. Lo compartido
del VPS (Caddy, UFW, puertos, copias) está anotado en `/opt/VPS.md`.

| Qué | Dónde |
| --- | --- |
| La web | https://sintetica.grupovimo.com.ar → 127.0.0.1:3600 |
| La API | https://api-sintetica.grupovimo.com.ar → 127.0.0.1:4400 |
| MQTT (bichos) | mqtts://mqtt-sintetica.grupovimo.com.ar:8883 (sin Cloudflare) |
| Panel de EMQX | `ssh -N -L 18083:127.0.0.1:18083 deploy@187.41.102.190` → http://localhost:18083 |
| Los mails (hasta tener SMTP) | `ssh -N -L 8025:127.0.0.1:8025 deploy@187.41.102.190` → http://localhost:8025 |

## La carpeta

```
/opt/sintetica/
  compose.yml          (de produccion/compose.yml)
  emqx.conf            (de produccion/emqx.conf)
  .env                 (600; de produccion/.env.ejemplo, con las claves)
  secretos/emqx-api-claves.txt   (600; "sintetica:<clave>")
  claves/              ota-privada.pem (la firma de las OTA) y ota-publica.pem
  certs/               mqtt.crt y mqtt.key (los renueva el timer sintetica-cert)
  pg/                  los datos de Postgres
  emqx-datos/
```

## Primera vez

1. `docker login ghcr.io` con un token de solo lectura de paquetes
   (`read:packages`): el repo es privado.
2. Copiar `compose.yml`, `emqx.conf` y armar `.env` y `secretos/` con claves
   nuevas (ver `.env.ejemplo`).
3. La clave de firma de las OTA, una sola vez:
   ```sh
   docker run --rm -u root -v /opt/sintetica/claves:/claves \
     --entrypoint sintetica-api ghcr.io/nawesan12/sintetica-api claves /claves
   chown -R 10001 claves && chmod 600 claves/ota-privada.pem
   ```
   **Perderla obliga a reflashear por USB todos los bichos**: guardar una copia
   fuera del VPS. La pública (`ota-publica.pem`) es la que se embebe en el
   firmware de producción.
4. `docker compose pull && docker compose up -d`.
5. `curl -s https://api-sintetica.grupovimo.com.ar/salud`.

## Actualizar

```sh
cd /opt/sintetica
docker compose exec postgres pg_dump -U sintetica sintetica | gzip > ~/sintetica-$(date +%F).sql.gz
VERSION=<sha> docker compose pull && VERSION=<sha> docker compose up -d
```

Cuando el timer renueva el certificado de MQTT deja `certs/renovado`:
`docker compose restart emqx` y borrar el archivo.

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
