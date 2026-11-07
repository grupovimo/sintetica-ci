#!/usr/bin/env python3
"""Tests de oro: el renderer del núcleo contra las caras del manual de marca.

Dibuja las 16 caras con render_caras y compara píxel a píxel contra las PNG de
./referencia (rasterizadas del SVG del manual con rasterizar.cjs).

Criterio: menos del 0,6% de los píxeles con una diferencia mayor a 48 (de 255)
en algún canal. Las diferencias normales son el antialias del borde y la
cuantización a RGB565 de la pantalla.

Uso: comparar.py <ruta a render_caras> [--diferencias carpeta]
"""
import pathlib
import subprocess
import sys
import tempfile

import numpy as np
from PIL import Image

AQUI = pathlib.Path(__file__).resolve().parent
UMBRAL = 48
TOLERANCIA = 0.006
# La pantalla de emparejamiento: el QR del manual es ilustrativo, así que se
# compara todo menos su interior. El texto rasterizado tiene otro antialias que
# el del SVG, así que se compara después de un desenfoque de 3 × 3: tolera
# corrimientos de un píxel, pero no letras, pesos o posiciones equivocadas.
TOLERANCIA_TEXTO = 0.015


def desenfocar(img):
    p = np.pad(img, ((1, 1), (1, 1), (0, 0)), mode="edge").astype(np.float32)
    h, w = img.shape[:2]
    return sum(p[dy:dy + h, dx:dx + w] for dy in range(3) for dx in range(3)) / 9


def main():
    render = sys.argv[1]
    carpeta_dif = None
    if "--diferencias" in sys.argv:
        carpeta_dif = pathlib.Path(sys.argv[sys.argv.index("--diferencias") + 1])
        carpeta_dif.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmp:
        subprocess.run([render, tmp], check=True, stdout=subprocess.DEVNULL)
        malas = 0
        for ref in sorted((AQUI / "referencia").glob("*.png")):
            nuestro = pathlib.Path(tmp) / (ref.stem + ".ppm")
            a = np.asarray(Image.open(ref).convert("RGB"), dtype=np.int16)
            b = np.asarray(Image.open(nuestro).convert("RGB"), dtype=np.int16)
            dif = np.abs(a - b).max(axis=2)
            if ref.stem == "emparejamiento":
                dif = np.abs(desenfocar(a) - desenfocar(b)).max(axis=2)
                mascara = np.ones(dif.shape, dtype=bool)
                mascara[28:142, 28:142] = False  # el interior del QR
                frac = float((dif[mascara] > UMBRAL).mean())
                ok = frac < TOLERANCIA_TEXTO
            else:
                frac = float((dif > UMBRAL).mean())
                ok = frac < TOLERANCIA
            malas += not ok
            print(f"{'ok  ' if ok else 'MAL '} {ref.stem:16s} {frac * 100:6.3f}% píxeles distintos")
            if carpeta_dif is not None:
                lado = np.concatenate([a, b, np.repeat(dif[..., None], 3, axis=2)], axis=1)
                Image.fromarray(lado.clip(0, 255).astype(np.uint8)).save(carpeta_dif / f"{ref.stem}.png")
    sys.exit(1 if malas else 0)


if __name__ == "__main__":
    main()
