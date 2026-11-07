# El núcleo de SintéticaOS (el mismo C++ del firmware) + el puente JNI.
cmake_minimum_required(VERSION 3.22.1)
project(sintetica_nucleo CXX)

set(RAIZ ${CMAKE_CURRENT_SOURCE_DIR}/../../../../..)
file(GLOB NUCLEO ${RAIZ}/firmware/nucleo/*.cpp)
add_library(sintetica SHARED ${NUCLEO} ${RAIZ}/firmware/api/sintetica_c.cpp puente.cpp)
target_include_directories(sintetica PRIVATE ${RAIZ}/firmware/nucleo ${RAIZ}/firmware/api)
target_compile_options(sintetica PRIVATE -O2 -Wall -Wno-missing-field-initializers)
target_link_libraries(sintetica log)
