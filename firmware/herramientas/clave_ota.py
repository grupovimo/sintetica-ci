#!/usr/bin/env python3
"""Genera perfiles/comun/clave_ota.h con la clave pública de las OTA.

Lee claves/ota-publica.pem en la raíz del repo (la crea `npm run claves`, junto
con la privada que usa la API para firmar). Si no está, el firmware queda sin
clave y rechaza todas las OTA (código 0x0C), que es lo seguro.

Sin dependencias: una clave pública P-256 en SPKI DER son siempre 91 bytes, 26
de cabecera fija y el punto sin comprimir (65 bytes).

PlatformIO lo corre antes de compilar (extra_scripts); también se puede
correr a mano: python3 firmware/herramientas/clave_ota.py
"""
import base64
import os
import sys

CABECERA_SPKI_P256 = bytes.fromhex("3059301306072a8648ce3d020106082a8648ce3d030107034200")


def leer_punto(pem: str) -> bytes:
    cuerpo = "".join(l for l in pem.splitlines() if l and not l.startswith("-----"))
    der = base64.b64decode(cuerpo)
    if len(der) != 91 or not der.startswith(CABECERA_SPKI_P256) or der[26] != 4:
        raise ValueError("no es una clave pública P-256 (SPKI)")
    return der[26:]


def main(raiz: str) -> int:
    pem_ruta = os.path.join(raiz, "claves", "ota-publica.pem")
    destino = os.path.join(raiz, "firmware", "perfiles", "comun", "clave_ota.h")
    punto = b""
    if os.path.exists(pem_ruta):
        with open(pem_ruta) as f:
            punto = leer_punto(f.read())
    else:
        print("[clave_ota] no está claves/ota-publica.pem (npm run claves): el firmware rechaza toda OTA")
    filas = ",\n  ".join(", ".join(f"0x{b:02x}" for b in punto[i : i + 13]) for i in range(0, len(punto), 13))
    texto = (
        "// Generado por firmware/herramientas/clave_ota.py desde claves/ota-publica.pem. No editar.\n"
        "#pragma once\n#include <stdint.h>\n\n"
        "// Punto público P-256 sin comprimir (0x04 || X || Y). Vacío = sin clave.\n"
        f"constexpr unsigned CLAVE_OTA_LARGO = {len(punto)};\n"
        f"static const uint8_t CLAVE_OTA[{max(len(punto), 1)}] = {{\n  {filas or '0'}\n}};\n"
    )
    previo = open(destino).read() if os.path.exists(destino) else ""
    if previo != texto:
        with open(destino, "w") as f:
            f.write(texto)
    return 0


try:
    Import("env")  # noqa: F821 (lo define PlatformIO)
    main(os.path.abspath(os.path.join(env["PROJECT_DIR"], "..")))  # noqa: F821
except NameError:
    if __name__ == "__main__":
        sys.exit(main(os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))))
