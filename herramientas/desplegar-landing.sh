#!/bin/sh
# Despliega la landing sola en sintetica.grupovimo.com.ar: arma la imagen de la
# web desde lo commiteado (HEAD), la manda al VPS por ssh (sin registro) y la
# levanta con produccion/landing.compose.yml. Los secretos (landing.env) ya
# están en el VPS. Uso: sh herramientas/desplegar-landing.sh
set -eu
raiz="$(cd "$(dirname "$0")/.." && pwd)"
vps="${SINTETICA_VPS:-deploy@187.41.102.190}"
cd "$raiz"
version="$(git rev-parse --short HEAD)"
imagen="sintetica-web:landing-$version"

if ! git diff --quiet HEAD -- web sim firmware/nucleo shared; then
  echo "ojo: hay cambios sin commitear en web/sim/núcleo; se publica HEAD ($version)"
fi

echo "armando $imagen"
git archive HEAD | docker build -q -f web/Dockerfile -t "$imagen" - >/dev/null

echo "mandándola al VPS"
docker save "$imagen" | gzip -1 | ssh "$vps" 'gunzip | docker load' >/dev/null
scp -q produccion/landing.compose.yml "$vps:/opt/sintetica/landing.compose.yml"

echo "levantándola"
ssh "$vps" "cd /opt/sintetica && VERSION=$version docker compose -f landing.compose.yml up -d >/dev/null 2>&1 &&
  for i in \$(seq 1 30); do curl -sf -o /dev/null http://127.0.0.1:3600/ && break; sleep 1; done &&
  docker image ls --format '{{.Repository}}:{{.Tag}}' | grep '^sintetica-web:landing-' | grep -v ':landing-$version\$' | xargs -r docker rmi >/dev/null 2>&1 || true"

estado="$(curl -s -o /dev/null -w '%{http_code}' https://sintetica.grupovimo.com.ar/)"
echo "listo: https://sintetica.grupovimo.com.ar/ responde $estado (versión $version)"
