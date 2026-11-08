# Fuzzing de los parsers del núcleo con libFuzzer (ver correr.sh).
#
#   cmake -S firmware/fuzz -B build/fuzz -DCMAKE_CXX_COMPILER=clang++
#   cmake --build build/fuzz -j && ctest --test-dir build/fuzz
#
# Por cada blanco (fuzz_<blanco>.cpp) arma:
#  - fuzz_<blanco>: con libFuzzer y ASan + UBSan. Si el compilador trae
#    libFuzzer (el clang de Ubuntu), con -fsanitize=fuzzer; si no (el de Apple),
#    con -DSINTETICA_LIBFUZZER=<libclang_rt.fuzzer_osx.a> y la cobertura a mano
#    (correr.sh lo arma con el libFuzzer del llvm de Homebrew).
#  - regresion_<blanco>: el mismo arnés sin libFuzzer, con ASan y UBSan, que
#    corre como test normal las entradas de regresiones/<blanco>/ y el corpus
#    semilla de corpus/<blanco>/.
# Compila el núcleo como lo compila la API (cgo): sin SINTETICA_JSON_GRANDE.
# SINTETICA_HERRAMIENTAS solo agrega aplicarConfig (lo usa fuzz_config).
cmake_minimum_required(VERSION 3.16)
project(sinteticaos_fuzz CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE RelWithDebInfo)
endif()

set(FIRMWARE ${CMAKE_CURRENT_SOURCE_DIR}/..)
file(GLOB NUCLEO_FUENTES CONFIGURE_DEPENDS ${FIRMWARE}/nucleo/*.cpp)
set(API_FUENTES ${FIRMWARE}/api/sintetica_c.cpp)

set(SANITIZADORES -fsanitize=address,undefined -fno-sanitize-recover=all -fno-omit-frame-pointer)
add_compile_options(-g -Wall -Wextra -Wno-missing-field-initializers)

include(CheckCXXSourceCompiles)
set(CMAKE_REQUIRED_FLAGS "-fsanitize=fuzzer")
check_cxx_source_compiles(
  "#include <stddef.h>\n#include <stdint.h>\nextern \"C\" int LLVMFuzzerTestOneInput(const uint8_t*, size_t) { return 0; }"
  HAY_LIBFUZZER)
unset(CMAKE_REQUIRED_FLAGS)

# variante: "fuzz" (instrumentado para libFuzzer) o "san" (solo sanitizadores).
function(nucleo_para variante)
  add_library(nucleo_${variante} STATIC ${NUCLEO_FUENTES} ${API_FUENTES})
  target_include_directories(nucleo_${variante} PUBLIC ${FIRMWARE}/nucleo ${FIRMWARE}/api)
  target_compile_definitions(nucleo_${variante} PUBLIC SINTETICA_HERRAMIENTAS)
  target_compile_options(nucleo_${variante} PUBLIC ${SANITIZADORES})
  target_link_options(nucleo_${variante} PUBLIC ${SANITIZADORES})
  if(variante STREQUAL "fuzz")
    target_compile_options(nucleo_${variante} PUBLIC ${COBERTURA})
  endif()
endfunction()

set(SINTETICA_LIBFUZZER "" CACHE FILEPATH "libFuzzer aparte (libclang_rt.fuzzer_*.a), para compiladores que no lo traen")
set(SINTETICA_LIBFUZZER_LDFLAGS "" CACHE STRING "Lo que necesita ese libFuzzer para enlazar (su libc++)")
if(HAY_LIBFUZZER)
  set(COBERTURA -fsanitize=fuzzer-no-link)
  set(ENLACE_FUZZER -fsanitize=fuzzer)
elseif(SINTETICA_LIBFUZZER)
  # Lo mismo que -fsanitize=fuzzer-no-link.
  set(COBERTURA -fsanitize-coverage=inline-8bit-counters,indirect-calls,trace-cmp,pc-table)
  separate_arguments(ENLACE_FUZZER UNIX_COMMAND "${SINTETICA_LIBFUZZER} ${SINTETICA_LIBFUZZER_LDFLAGS}")
endif()

nucleo_para(san)
if(COBERTURA)
  nucleo_para(fuzz)
else()
  message(STATUS "Este compilador no trae libFuzzer: solo se arman los tests de regresión.")
endif()

enable_testing()
set(BLANCOS config protocolo qoi expresion)
foreach(b ${BLANCOS})
  add_executable(regresion_${b} fuzz_${b}.cpp regresiones.cpp)
  target_link_libraries(regresion_${b} nucleo_san)
  add_test(NAME regresion_${b}
           COMMAND regresion_${b} ${CMAKE_CURRENT_SOURCE_DIR}/regresiones/${b} ${CMAKE_CURRENT_SOURCE_DIR}/corpus/${b})
  if(COBERTURA)
    add_executable(fuzz_${b} fuzz_${b}.cpp)
    target_link_libraries(fuzz_${b} nucleo_fuzz)
    if(HAY_LIBFUZZER)
      target_compile_options(fuzz_${b} PRIVATE -fsanitize=fuzzer)
    endif()
    target_link_options(fuzz_${b} PRIVATE ${ENLACE_FUZZER})
  endif()
endforeach()
