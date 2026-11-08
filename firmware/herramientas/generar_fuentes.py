#!/usr/bin/env python3
"""Genera firmware/nucleo/fuentes.h: atlas de glifos con antialias de 4 bits.

Las fuentes son las de la marca (OFL, en ./fuentes): JetBrains Mono para lo
técnico y Fraunces para la voz del bicho. Los tamaños, pesos y tracking están
ajustados para que la pantalla de emparejamiento coincida con
pantalla-emparejamiento.svg del manual (320 × 172).

Cada glifo se rasteriza con el origen en la línea de base. El núcleo los dibuja
en posiciones enteras y avanza con el ancho real (en 1/16 de píxel) más el
tracking del estilo.

Uso:  python3 firmware/herramientas/generar_fuentes.py [--verificar]
"""
import pathlib
import sys

import numpy as np
from PIL import Image, ImageDraw, ImageFont

AQUI = pathlib.Path(__file__).resolve().parent
DESTINO = AQUI.parent / "nucleo" / "fuentes.h"

MAYUS = "ABCDEFGHIJKLMNOPQRSTUVWXYZÁÉÍÓÚÜÑ"
DIGITOS = "0123456789"
TECNICO = MAYUS + DIGITOS + " #·-—:./%+()°"
VOZ = MAYUS + MAYUS.lower() + DIGITOS + " .,:;!¡?¿-—'()°"

ESTILOS = [
    # nombre, archivo, tamaño px, ejes variables, tracking px, juego de caracteres
    ("MONO_13", "JetBrainsMono.ttf", 13, [700], 1.1, TECNICO),
    ("MONO_9", "JetBrainsMono.ttf", 9.5, [400], 0.62, TECNICO),
    # Fraunces: opsz, wght, SOFT, WONK
    ("VOZ_20", "Fraunces.ttf", 20, [20, 700, 100, 0], 0.2, VOZ),
]


def rasterizar(archivo, tamano, ejes, caracteres):
    fuente = ImageFont.truetype(str(AQUI / "fuentes" / archivo), tamano)
    fuente.set_variation_by_axes(ejes)
    ascenso, descenso = fuente.getmetrics()
    glifos = []
    for ch in sorted(set(caracteres), key=ord):
        avance = fuente.getlength(ch)
        x0, y0, x1, y1 = fuente.getbbox(ch, anchor="ls")
        w, h = max(0, x1 - x0), max(0, y1 - y0)
        alfa = b""
        if w and h:
            im = Image.new("L", (w, h), 0)
            ImageDraw.Draw(im).text((-x0, -y0), ch, font=fuente, fill=255, anchor="ls")
            a = (np.asarray(im, dtype=np.uint16) * 15 + 127) // 255  # a 4 bits
            plano = a.flatten().tolist()
            if len(plano) % 2:
                plano.append(0)
            alfa = bytes((plano[i] << 4) | plano[i + 1] for i in range(0, len(plano), 2))
        glifos.append((ord(ch), x0, y0, w, h, round(avance * 16), alfa))
    return glifos, ascenso, descenso


def generar():
    out = [
        "// GENERADO por firmware/herramientas/generar_fuentes.py. No editar a mano.",
        "// Fuentes: JetBrains Mono y Fraunces (SIL Open Font License 1.1,",
        "// ver firmware/herramientas/fuentes/OFL-*.txt).",
        "#pragma once",
        "",
        '#include "texto.h"',
        "",
        "namespace sintetica {",
        "namespace fuentes {",
        "",
    ]
    for nombre, archivo, tamano, ejes, tracking, caracteres in ESTILOS:
        glifos, asc, desc = rasterizar(archivo, tamano, ejes, caracteres)
        datos = bytearray()
        filas = []
        for cp, x0, y0, w, h, av, alfa in glifos:
            filas.append(f"    {{0x{cp:04X}, {x0}, {y0}, {w}, {h}, {av}, {len(datos)}}},")
            datos += alfa
        hexa = ", ".join(f"0x{b:02X}" for b in datos)
        out += [
            f"// {archivo} {tamano} px, ejes {ejes}, tracking {tracking} px",
            f"inline const Glifo GLIFOS_{nombre}[] = {{",
            *filas,
            "};",
            f"inline const uint8_t DATOS_{nombre}[] = {{{hexa}}};",
            f"inline const Fuente {nombre} = {{GLIFOS_{nombre}, {len(glifos)}, DATOS_{nombre}, "
            f"{tracking}f, {asc}, {desc}}};",
            "",
        ]
    out += ["}  // namespace fuentes", "}  // namespace sintetica", ""]
    return "\n".join(out)


def glifos_de(texto: str) -> list:
    """Los códigos de cada fuente (lo que no depende de cómo rasteriza FreeType)."""
    import re

    return [(n, re.findall(r"\{0x([0-9A-F]{4}),", cuerpo))
            for n, cuerpo in re.findall(r"GLIFOS_(\w+)\[\] = \{(.*?)\};", texto, re.S)]


def main():
    texto = generar()
    if "--verificar" in sys.argv:
        actual = DESTINO.read_text(encoding="utf-8") if DESTINO.exists() else ""
        if actual == texto:
            print("fuentes.h al día")
            return
        # Otra versión de Pillow/FreeType rasteriza un poco distinto (la Mac y
        # Linux no dan los mismos bytes): ahí alcanza con que estén las mismas
        # letras en cada fuente. Lo que se dibuja lo cuidan los tests de oro.
        if actual and glifos_de(actual) == glifos_de(texto):
            print("fuentes.h al día (las mismas letras; el rasterizado de esta máquina es otro)")
            return
        print("fuentes.h está desactualizado: correr firmware/herramientas/generar_fuentes.py")
        sys.exit(1)
    DESTINO.write_text(texto, encoding="utf-8")
    print(f"escrito {DESTINO} ({len(texto) // 1024} KB de fuente)")


if __name__ == "__main__":
    main()
