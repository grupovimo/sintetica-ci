#!/bin/sh
# El CI corre en el espejo público grupovimo/sintetica-ci (los minutos de
# Actions de la organización) mientras el repo de nawesan12 no tenga minutos.
# Empuja HEAD a la misma rama allá; el workflow Pruebas arranca solo.
# Uso: sh herramientas/publicar-ci.sh   ·   ver: gh run list -R grupovimo/sintetica-ci
set -eu
cd "$(dirname "$0")/.."
token="$(gh auth token -u grupovimo)"
rama="$(git rev-parse --abbrev-ref HEAD)"
git push -q "https://grupovimo:$token@github.com/grupovimo/sintetica-ci.git" "HEAD:refs/heads/$rama"
echo "publicado $(git rev-parse --short HEAD) en grupovimo/sintetica-ci ($rama)"
