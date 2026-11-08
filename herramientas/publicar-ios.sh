#!/bin/sh
# Publica la app de iOS en el repo público grupovimo/sintetica-ios, donde
# GitHub Actions la compila y la sube a TestFlight (en macOS, Actions solo es
# gratis en repos públicos). Copia la app, lo que necesita del núcleo y el
# workflow; nada más. Uso: sh herramientas/publicar-ios.sh
set -eu
raiz="$(cd "$(dirname "$0")/.." && pwd)"
destino="${SINTETICA_IOS:-$raiz/../sintetica-ios}"
token="$(gh auth token -u grupovimo)"
if [ ! -d "$destino/.git" ]; then
  git clone -q "https://x-access-token:$token@github.com/grupovimo/sintetica-ios.git" "$destino"
fi
cd "$destino"
git pull -q --rebase 2>/dev/null || true
rm -rf ios firmware shared .github
mkdir -p firmware shared/protocolo shared/formato .github/workflows
rsync -a --exclude 'Sintetica.xcodeproj' --exclude '.DS_Store' "$raiz/ios/" ios/
rsync -a --exclude '*.md' "$raiz/firmware/nucleo/" firmware/nucleo/
rsync -a --exclude 'cgo' "$raiz/firmware/api/" firmware/api/
cp "$raiz/shared/protocolo/vectores.json" shared/protocolo/
cp "$raiz/shared/formato/config-ejemplo.json" shared/formato/
cp "$raiz/herramientas/ios-ci/testflight.yml" .github/workflows/
printf '# Sintética · iOS\n\nCopia de la app de iOS para compilarla y subirla a TestFlight. No se edita acá.\n' > README.md
git add -A
origen="$(git -C "$raiz" rev-parse --short HEAD)"
if git diff --cached --quiet; then echo "sin cambios"; exit 0; fi
git -c user.name="Sintética" -c user.email="noreply@grupovimo.com.ar" commit -q -m "iOS desde sintetica@$origen"
git push -q "https://x-access-token:$token@github.com/grupovimo/sintetica-ios.git" HEAD:main
echo "publicado sintetica@$origen"
