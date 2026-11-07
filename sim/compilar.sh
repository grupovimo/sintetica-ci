#!/bin/sh
# Compila el núcleo de SintéticaOS + el perfil "sim" a WebAssembly con clang y
# wasm-ld (sin Emscripten ni libc). Deja sim/dist/sinteticaos.wasm.
set -eu
cd "$(dirname "$0")/.."
CLANG="${CLANG:-clang}"
mkdir -p sim/dist build/sim
OBJS=""
for f in firmware/nucleo/*.cpp firmware/perfiles/sim/sim.cpp; do
  o="build/sim/$(basename "$f" .cpp).o"
  "$CLANG" --target=wasm32 -std=c++17 -O2 -ffreestanding -nostdlib -fno-exceptions -fno-rtti \
    -fno-threadsafe-statics -Wall -Wextra -Wno-missing-field-initializers \
    -Ifirmware/nucleo -c "$f" -o "$o"
  OBJS="$OBJS $o"
done
# shellcheck disable=SC2086
wasm-ld --no-entry --export=__wasm_call_ctors -z stack-size=262144 \
  --initial-memory=8388608 $OBJS -o sim/dist/sinteticaos.wasm
echo "sim/dist/sinteticaos.wasm ($(wc -c < sim/dist/sinteticaos.wasm) bytes)"
