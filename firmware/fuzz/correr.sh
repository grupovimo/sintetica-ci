#!/usr/bin/env bash
# Compila y corre los blancos de fuzzing del núcleo: firmware/fuzz/correr.sh [segundos]
#
#   segundos   por blanco (60 si no se dice)
#   BLANCOS    los que se corren (por defecto: config protocolo qoi expresion)
#   PROCESOS   procesos de libFuzzer por blanco (-fork; 1 = sin fork)
#   JUNTOS=1   corre todos los blancos a la vez
#   CXX        el compilador. En Linux, el clang del sistema (trae libFuzzer).
#              En la Mac, el clang de Apple con el libFuzzer del llvm de Homebrew
#              (el ASan de Homebrew no anda en todas las versiones de macOS).
#
# El corpus que va creciendo queda en build/fuzz/corpus/<blanco> (las semillas
# de firmware/fuzz/corpus/<blanco> se leen pero no se tocan); lo que rompe, en
# build/fuzz/fallas/<blanco>/. Si algo rompe: reproducirlo con
# build/fuzz/fuzz_<blanco> <archivo>, arreglarlo y copiar el archivo a
# firmware/fuzz/regresiones/<blanco>/ (corre en ctest --test-dir build/fuzz).
set -euo pipefail

SEGUNDOS="${1:-60}"
AQUI="$(cd "$(dirname "$0")" && pwd)"
RAIZ="$(cd "$AQUI/../.." && pwd)"
SALIDA="$RAIZ/build/fuzz"
BLANCOS="${BLANCOS:-config protocolo qoi expresion}"
PROCESOS="${PROCESOS:-1}"

args=(-DCMAKE_BUILD_TYPE=RelWithDebInfo)
if [[ "$(uname)" == "Darwin" && -z "${CXX:-}" ]]; then
  llvm="$(brew --prefix llvm 2>/dev/null || true)"
  lib="$(ls "$llvm"/lib/clang/*/lib/darwin/libclang_rt.fuzzer_osx.a 2>/dev/null | head -1 || true)"
  if [[ -z "$lib" ]]; then
    echo "falta libFuzzer: brew install llvm" >&2
    exit 1
  fi
  args+=(-DCMAKE_CXX_COMPILER=/usr/bin/clang++ "-DSINTETICA_LIBFUZZER=$lib"
         "-DSINTETICA_LIBFUZZER_LDFLAGS=-L$llvm/lib/c++ -Wl,-rpath,$llvm/lib/c++")
else
  args+=("-DCMAKE_CXX_COMPILER=${CXX:-clang++}")
fi

cmake -S "$AQUI" -B "$SALIDA" "${args[@]}" >/dev/null
cmake --build "$SALIDA" -j "$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)" >/dev/null

# Primero las regresiones y las semillas: tienen que pasar siempre.
ctest --test-dir "$SALIDA" --output-on-failure

largo_maximo() {
  case "$1" in
    config | expresion) echo 20000 ;;
    protocolo) echo 24000 ;;
    *) echo 8192 ;;
  esac
}

correr() {
  local b="$1"
  local corpus="$SALIDA/corpus/$b" fallas="$SALIDA/fallas/$b"
  mkdir -p "$corpus" "$fallas"
  local extra=()
  [[ "$PROCESOS" -gt 1 ]] && extra+=("-fork=$PROCESOS" -ignore_crashes=0)
  echo "== $b: $SEGUNDOS s"
  "$SALIDA/fuzz_$b" "$corpus" "$AQUI/corpus/$b" \
    -dict="$AQUI/corpus/$b.dict" \
    -max_total_time="$SEGUNDOS" -max_len="$(largo_maximo "$b")" \
    -timeout=20 -rss_limit_mb=2560 -print_final_stats=1 \
    -artifact_prefix="$fallas/" "${extra[@]+"${extra[@]}"}" \
    >"$SALIDA/$b.log" 2>&1
}

export ASAN_OPTIONS="${ASAN_OPTIONS:-abort_on_error=1:detect_stack_use_after_return=1}"
export UBSAN_OPTIONS="${UBSAN_OPTIONS:-print_stacktrace=1:halt_on_error=1}"

estado=0
pids=()
for b in $BLANCOS; do
  if [[ "${JUNTOS:-0}" == 1 ]]; then
    correr "$b" &
    pids+=("$!:$b")
  else
    correr "$b" || { echo "!! $b: falló (ver $SALIDA/$b.log)"; estado=1; }
  fi
done
for p in "${pids[@]+"${pids[@]}"}"; do
  wait "${p%%:*}" || { echo "!! ${p#*:}: falló (ver $SALIDA/${p#*:}.log)"; estado=1; }
done

for b in $BLANCOS; do
  # Una línea de resumen por blanco: ejecuciones y cobertura.
  echo "$b: $(grep -E '^(#[0-9]+.*(DONE|pulse|NEW|REDUCE)|stat::number_of_executed_units)' "$SALIDA/$b.log" | tail -1)"
  if compgen -G "$SALIDA/fallas/$b/*" >/dev/null; then
    echo "   fallas en $SALIDA/fallas/$b/"
    estado=1
  fi
done
exit $estado
