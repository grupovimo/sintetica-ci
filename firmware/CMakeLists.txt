# Compila el núcleo de SintéticaOS en la PC: tests y herramientas.
# El firmware real se compila con PlatformIO (platformio.ini); el simulador,
# con sim/compilar.sh (clang → WebAssembly).
cmake_minimum_required(VERSION 3.16)
project(sinteticaos_nucleo CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
if(NOT CMAKE_BUILD_TYPE)
  set(CMAKE_BUILD_TYPE RelWithDebInfo)
endif()
add_compile_options(-Wall -Wextra -Wno-missing-field-initializers)

file(GLOB NUCLEO_FUENTES CONFIGURE_DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/nucleo/*.cpp)
add_library(nucleo STATIC ${NUCLEO_FUENTES})
target_include_directories(nucleo PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/nucleo)

file(GLOB TEST_FUENTES CONFIGURE_DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/test/test_*.cpp)
add_executable(pruebas ${TEST_FUENTES} test/principal.cpp)
target_link_libraries(pruebas nucleo)
target_compile_definitions(pruebas PRIVATE RAIZ_REPO="${CMAKE_CURRENT_SOURCE_DIR}/..")

add_executable(render_caras herramientas/render_caras.cpp)
target_link_libraries(render_caras nucleo)

enable_testing()
add_test(NAME nucleo COMMAND pruebas)
add_test(NAME caras_embebidas COMMAND python3 ${CMAKE_CURRENT_SOURCE_DIR}/herramientas/embeber_caras.py --verificar)
add_test(NAME fuentes_generadas COMMAND python3 ${CMAKE_CURRENT_SOURCE_DIR}/herramientas/generar_fuentes.py --verificar)
add_test(NAME qr_legible COMMAND python3 ${CMAKE_CURRENT_SOURCE_DIR}/test/oro/leer_qr.py $<TARGET_FILE:render_caras>)
add_test(NAME caras_de_oro
         COMMAND python3 ${CMAKE_CURRENT_SOURCE_DIR}/test/oro/comparar.py $<TARGET_FILE:render_caras>)
