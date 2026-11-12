#!/bin/sh
# Publica la app de escritorio en el repo público grupovimo/sintetica-escritorio,
# donde GitHub Actions la compila para Mac y Windows (y aloja las descargas).
# Copia la app, el núcleo y el workflow; nada más. Lo commiteado (HEAD), nunca
# lo que haya a medio hacer en el disco.
# Uso: sh herramientas/publicar-escritorio.sh   ·   ver: gh run list -R grupovimo/sintetica-escritorio
set -eu
raiz="$(cd "$(dirname "$0")/.." && pwd)"
destino="${SINTETICA_ESCRITORIO:-$raiz/../sintetica-escritorio}"
token="$(gh auth token -u grupovimo)"
remoto="https://grupovimo:$token@github.com/grupovimo/sintetica-escritorio.git"
if ! GH_TOKEN="$token" gh repo view grupovimo/sintetica-escritorio >/dev/null 2>&1; then
  GH_TOKEN="$token" gh repo create grupovimo/sintetica-escritorio --public \
    --description "Sintética · el robot que vive en la compu (compilación y descargas)"
fi
if [ ! -d "$destino/.git" ]; then
  git clone -q "$remoto" "$destino" 2>/dev/null || { mkdir -p "$destino" && git -C "$destino" init -q -b main; }
fi
foto="$(mktemp -d)"
trap 'rm -rf "$foto"' EXIT
git -C "$raiz" archive HEAD escritorio firmware/nucleo firmware/api herramientas/escritorio-ci | tar -x -C "$foto"
cd "$destino"
git pull -q --rebase "$remoto" main 2>/dev/null || true
rm -rf escritorio firmware .github
mkdir -p firmware .github/workflows
rsync -a "$foto/escritorio/" escritorio/
rsync -a --exclude '*.md' "$foto/firmware/nucleo/" firmware/nucleo/
rsync -a --exclude 'cgo' "$foto/firmware/api/" firmware/api/
cp "$foto/herramientas/escritorio-ci/escritorio.yml" .github/workflows/
printf 'escritorio/src-tauri/target/\nescritorio/src-tauri/gen/\nescritorio/src-tauri/icons/\n' > .gitignore
printf '# Sintética · escritorio\n\nCopia de la app de escritorio para compilarla en Mac y Windows. No se edita acá.\n' > README.md
git add -A
origen="$(git -C "$raiz" rev-parse --short HEAD)"
if git diff --cached --quiet; then echo "sin cambios"; exit 0; fi
git -c user.name="Sintética" -c user.email="noreply@grupovimo.com.ar" commit -q -m "Escritorio desde sintetica@$origen"
git push -q "$remoto" HEAD:main
echo "publicado sintetica@$origen"
