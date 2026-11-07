#!/usr/bin/env python3
"""Embebe shared/formato/caras/*.json en firmware/nucleo/caras_base.h.

Las caras son datos: el firmware las lee con el mismo parser que usa para la
configuración que manda la app. Este header es generado; no editarlo a mano.

Uso:  python3 firmware/herramientas/embeber_caras.py [--verificar]
"""
import json
import pathlib
import sys

RAIZ = pathlib.Path(__file__).resolve().parents[2]
CARAS = RAIZ / "shared" / "formato" / "caras"
DESTINO = RAIZ / "firmware" / "nucleo" / "caras_base.h"


def generar() -> str:
    archivos = sorted(CARAS.glob("*.json"))
    partes = [
        "// GENERADO por firmware/herramientas/embeber_caras.py desde shared/formato/caras.",
        "// No editar a mano.",
        "#pragma once",
        "",
        "namespace sintetica {",
        "",
        f"constexpr int CANTIDAD_CARAS_BASE = {len(archivos)};",
        "",
        "inline const char* const CARAS_BASE[CANTIDAD_CARAS_BASE] = {",
    ]
    for a in archivos:
        compacto = json.dumps(json.loads(a.read_text(encoding="utf-8")), ensure_ascii=False,
                              separators=(",", ":"))
        partes.append(f'    // {a.name}\n    R"json({compacto})json",')
    partes += ["};", "", "}  // namespace sintetica", ""]
    return "\n".join(partes)


def main():
    texto = generar()
    if "--verificar" in sys.argv:
        if not DESTINO.exists() or DESTINO.read_text(encoding="utf-8") != texto:
            print("caras_base.h está desactualizado: correr firmware/herramientas/embeber_caras.py")
            sys.exit(1)
        print("caras_base.h al día")
        return
    DESTINO.write_text(texto, encoding="utf-8")
    print(f"escrito {DESTINO.relative_to(RAIZ)}")


if __name__ == "__main__":
    main()
