# El núcleo de SintéticaOS (el mismo C++ del firmware) + el puente JNI del bicho.
cmake_minimum_required(VERSION 3.22.1)
project(sintetica_bicho CXX)

set(RAIZ ${CMAKE_CURRENT_SOURCE_DIR}/../../../../..)
file(GLOB NUCLEO CONFIGURE_DEPENDS ${RAIZ}/firmware/nucleo/*.cpp)
add_library(bicho SHARED ${NUCLEO} ${RAIZ}/firmware/api/sintetica_c.cpp puente.cpp)
target_include_directories(bicho PRIVATE ${RAIZ}/firmware/nucleo ${RAIZ}/firmware/api)
target_compile_options(bicho PRIVATE -O2 -Wall -Wno-missing-field-initializers)
target_link_libraries(bicho log GLESv2)
