#!/usr/bin/env python3
"""El QR de la pantalla de emparejamiento se lee de verdad (OpenCV).

Uso: leer_qr.py <ruta a render_caras>
"""
import subprocess
import sys
import tempfile

import cv2

ESPERADO = "SINTETICA:V1/A1B2C3D4E5F6/K7Q2XM"

with tempfile.TemporaryDirectory() as tmp:
    subprocess.run([sys.argv[1], tmp], check=True, stdout=subprocess.DEVNULL)
    img = cv2.imread(f"{tmp}/emparejamiento.ppm")
    # Como lo vería una cámara: más grande y un poco desenfocado.
    img = cv2.GaussianBlur(cv2.resize(img, None, fx=3, fy=3, interpolation=cv2.INTER_LINEAR), (5, 5), 0)
    texto = cv2.QRCodeDetector().detectAndDecode(img)[0]
    print(f"leído: {texto!r}")
    sys.exit(0 if texto == ESPERADO else 1)
