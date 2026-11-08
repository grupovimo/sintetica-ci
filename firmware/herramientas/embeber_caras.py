#!/usr/bin/env python3
"""Embebe shared/formato/caras/*.json en firmware/nucleo/caras_base.h y
shared/formato/vida/*.json (expresiones paramétricas, comportamientos, reglas,
personalidades y frases del motor de vida) en firmware/nucleo/vida_base.h.

Las frases van como una tabla (no como JSON): el núcleo las lee de la flash
sin parsear nada, así no ocupan RAM en el broche.

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
VIDA = RAIZ / "shared" / "formato" / "vida"
DESTINO_VIDA = RAIZ / "firmware" / "nucleo" / "vida_base.h"


def compacto(v) -> str:
    return json.dumps(v, ensure_ascii=False, separators=(",", ":"))


LARGO_FRASE = 60  # bytes UTF-8 (lo mismo que el globo del comando Decir)


def c_texto(t: str) -> str:
    """Un texto UTF-8 como literal de C++."""
    return '"' + t.replace("\\", "\\\\").replace('"', '\\"') + '"'


def tabla_frases(frases) -> list:
    sit = frases["situaciones"]
    lim = frases["limites"]
    partes = [
        "struct SituacionBase {",
        "  const char* nombre;",
        "  int probabilidad;   // centésimos",
        "  int enfriamiento;   // segundos",
        "  bool siempre;       // no cuenta para el límite diario ni depende de la frecuencia",
        "  int primera, cantidad;  // en FRASES_BASE",
        "};",
        "",
        "struct FraseBase {",
        '  const char* preset;  // "" = para todos',
        "  const char* texto;",
        "};",
        "",
        f"constexpr int FRASES_POR_DIA = {int(lim['por_dia'])};",
        f"constexpr int FRASES_SEPARACION_S = {int(lim['separacion'])};",
        "",
    ]
    filas, tabla = [], []
    for nombre, s in sit.items():
        primera = len(tabla)
        for preset, lista in s["frases"].items():
            for t in lista:
                n = len(t.encode("utf-8"))
                if not 1 <= n <= LARGO_FRASE:
                    sys.exit(f"frases.json: {nombre}/{preset}: {t!r} tiene {n} bytes (de 1 a {LARGO_FRASE})")
                tabla.append(f'    {{{c_texto("" if preset == "todos" else preset)}, {c_texto(t)}}},')
        filas.append(f'    {{"{nombre}", {round(float(s["probabilidad"]) * 100)}, {int(s["enfriamiento"])}, '
                     f'{"true" if s.get("siempre") else "false"}, {primera}, {len(tabla) - primera}}},')
    partes += [f"constexpr int CANTIDAD_SITUACIONES_BASE = {len(filas)};",
               "inline const SituacionBase SITUACIONES_BASE[CANTIDAD_SITUACIONES_BASE] = {", *filas, "};", "",
               f"constexpr int CANTIDAD_FRASES_BASE = {len(tabla)};",
               "inline const FraseBase FRASES_BASE[CANTIDAD_FRASES_BASE] = {", *tabla, "};", ""]
    return partes


def generar_vida() -> str:
    def leer(nombre):
        return json.loads((VIDA / nombre).read_text(encoding="utf-8"))

    expresiones = leer("expresiones.json")
    comportamientos = leer("comportamientos.json")
    partes = [
        "// GENERADO por firmware/herramientas/embeber_caras.py desde shared/formato/vida.",
        "// No editar a mano.",
        "#pragma once",
        "",
        "namespace sintetica {",
        "",
        "struct DatoVidaBase {",
        "  const char* nombre;",
        "  const char* json;",
        "};",
        "",
        f"constexpr int CANTIDAD_CARAS_VIDA_BASE = {len(expresiones)};",
        "inline const DatoVidaBase CARAS_VIDA_BASE[CANTIDAD_CARAS_VIDA_BASE] = {",
    ]
    for k, v in expresiones.items():
        partes.append(f'    {{"{k}", R"json({compacto(v)})json"}},')
    partes += ["};", "", f"constexpr int CANTIDAD_COMPORTAMIENTOS_BASE = {len(comportamientos)};",
               "inline const DatoVidaBase COMPORTAMIENTOS_BASE[CANTIDAD_COMPORTAMIENTOS_BASE] = {"]
    for k, v in comportamientos.items():
        partes.append(f'    {{"{k}", R"json({compacto(v)})json"}},')
    partes += ["};", "",
               f'inline const char* const REGLAS_VIDA_BASE = R"json({compacto(leer("reglas.json"))})json";',
               "",
               f'inline const char* const PERSONALIDADES_BASE = R"json({compacto(leer("personalidades.json"))})json";',
               "", *tabla_frases(leer("frases.json")), "}  // namespace sintetica", ""]
    return "\n".join(partes)


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
    for destino, texto in ((DESTINO, generar()), (DESTINO_VIDA, generar_vida())):
        if "--verificar" in sys.argv:
            if not destino.exists() or destino.read_text(encoding="utf-8") != texto:
                print(f"{destino.name} está desactualizado: correr firmware/herramientas/embeber_caras.py")
                sys.exit(1)
            print(f"{destino.name} al día")
            continue
        destino.write_text(texto, encoding="utf-8")
        print(f"escrito {destino.relative_to(RAIZ)}")


if __name__ == "__main__":
    main()
