#!/bin/sh
# Despliega la nube completa (API con el broker MQTT, web y Postgres) en el VPS
# de Grupo Vimo: arma las imágenes desde lo commiteado (HEAD), las manda por ssh
# (sin registro) y las levanta con produccion/compose.yml en /opt/sintetica.
# Los secretos (.env, claves/, certs/) ya están en el VPS. Antes de migrar, copia
# de la base. Uso: sh herramientas/desplegar-nube.sh
set -eu
raiz="$(cd "$(dirname "$0")/.." && pwd)"
vps="${SINTETICA_VPS:-deploy@187.41.102.190}"
cd "$raiz"
version="$(git rev-parse --short HEAD)"

if ! git diff --quiet HEAD -- api web sim firmware/nucleo firmware/api shared produccion; then
  echo "ojo: hay cambios sin commitear; se publica HEAD ($version)"
fi

echo "armando sintetica-api:$version y sintetica-web:$version"
git archive HEAD | docker build -q --target api -f api/Dockerfile -t "sintetica-api:$version" - >/dev/null
git archive HEAD | docker build -q -f web/Dockerfile -t "sintetica-web:$version" - >/dev/null

echo "mandándolas al VPS"
docker save "sintetica-api:$version" "sintetica-web:$version" | gzip -1 | ssh "$vps" 'gunzip | docker load' >/dev/null
scp -q produccion/compose.yml "$vps:/opt/sintetica/compose.yml"

echo "copia de la base y levantando"
ssh "$vps" "set -e; cd /opt/sintetica
  if docker compose ps -q postgres 2>/dev/null | grep -q .; then
    mkdir -p copias
    docker compose exec -T postgres pg_dump -U sintetica sintetica | gzip > copias/antes-$version.sql.gz
    gzip -t copias/antes-$version.sql.gz
    ls -1t copias/antes-*.sql.gz | tail -n +8 | xargs -r rm -f
  fi
  # La primera vez, la landing sola le deja el 3600 a la web completa.
  if [ -f landing.compose.yml ] && docker compose -f landing.compose.yml ps -q 2>/dev/null | grep -q .; then
    docker compose -f landing.compose.yml down >/dev/null 2>&1
  fi
  sed -i \"s/^VERSION=.*/VERSION=$version/\" .env
  docker compose up -d --remove-orphans >/dev/null 2>&1
  for i in \$(seq 1 60); do curl -sf -o /dev/null http://127.0.0.1:4400/salud && break; sleep 1; done
  for i in \$(seq 1 30); do curl -sf -o /dev/null http://127.0.0.1:3600/ && break; sleep 1; done
  docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -E '^sintetica-(api|web):' | grep -v \":$version\\\$\" | xargs -r docker rmi >/dev/null 2>&1 || true"

for u in https://sintetica.grupovimo.com.ar/ https://api-sintetica.grupovimo.com.ar/salud; do
  echo "$u → $(curl -s -o /dev/null -w '%{http_code}' "$u")"
done
echo "listo (versión $version)"
