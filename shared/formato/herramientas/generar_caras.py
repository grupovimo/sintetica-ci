#!/usr/bin/env python3
"""Genera shared/formato/caras/*.json a partir de las medidas del manual de marca.

Las medidas están en píxeles de la pantalla de referencia (320 × 172), tal como
aparecen en `expresiones-set.svg` del manual. Se pasan a coordenadas
normalizadas con escala uniforme:

    x = (px - 160) / 160      y = (py - 86) / 160      tamaños / 160

Así el centro de la pantalla es (0, 0), el ancho va de -1 a 1 y las formas no
se deforman en pantallas de otra proporción (la redonda de 240 × 240, etc.).

Uso:  python3 shared/formato/herramientas/generar_caras.py
"""
import json
import pathlib

U = 160.0
CX, CY = 160.0, 86.0
DESTINO = pathlib.Path(__file__).resolve().parent.parent / "caras"


def n(v):
    return round(v / U, 5)


def nx(px):
    return round((px - CX) / U, 5)


def ny(py):
    return round((py - CY) / U, 5)


def pildora(x, y, w, h, r=None, color=None, **extra):
    """Rectángulo redondeado dado por su caja (x, y arriba a la izquierda)."""
    f = {"forma": "pildora", "x": nx(x + w / 2), "y": ny(y + h / 2), "w": n(w), "h": n(h)}
    if r is not None and abs(r - min(w, h) / 2) > 1e-6:
        f["r"] = n(r)
    if color:
        f["color"] = color
    f.update(extra)
    return f


def corte(x, top, w, bottom, izq=0.0, der=0.0, color=None):
    """Ojo con el corte: arriba una recta inclinada, abajo medio círculo.

    `top` es el punto más alto, `bottom` el más bajo (incluye el medio círculo).
    `izq` y `der` cuánto baja la recta desde `top` en cada borde.
    """
    h = bottom - top
    f = {
        "forma": "corte",
        "x": nx(x + w / 2),
        "y": ny(top + h / 2),
        "w": n(w),
        "h": n(h),
        "izq": n(izq),
        "der": n(der),
    }
    if color:
        f["color"] = color
    return f


def circulo(cx, cy, r, color=None, **extra):
    f = {"forma": "circulo", "x": nx(cx), "y": ny(cy), "r": n(r)}
    if color:
        f["color"] = color
    f.update(extra)
    return f


def arco(x0, y0, qx, qy, x1, y1, trazo):
    return {
        "forma": "arco",
        "puntos": [[nx(x0), ny(y0)], [nx(qx), ny(qy)], [nx(x1), ny(y1)]],
        "trazo": n(trazo),
    }


def poligono(puntos, color=None, **extra):
    f = {"forma": "poligono", "puntos": [[nx(x), ny(y)] for x, y in puntos]}
    if color:
        f["color"] = color
    f.update(extra)
    return f


# Las Z de "zZ" de dormido, tal como las dibuja el manual (texto pasado a curvas).
Z_GRANDE = [(247.404, 64.0), (247.404, 61.966), (252.57, 56.08), (247.566, 56.08),
            (247.566, 54.1), (255.198, 54.1), (255.198, 56.134), (249.978, 62.02),
            (255.396, 62.02), (255.396, 64.0)]
Z_CHICA = [(263.014, 44.0), (263.014, 42.531), (266.745, 38.28), (263.131, 38.28),
           (263.131, 36.85), (268.643, 36.85), (268.643, 38.319), (264.873, 42.57),
           (268.786, 42.57), (268.786, 44.0)]

# Las dos pastillas neutrales, que se repiten en varias caras.
NEUTRAL = [pildora(92, 50, 36, 72), pildora(192, 50, 36, 72)]

CARAS = [
    ("neutral", "En reposo", {"formas": NEUTRAL}),
    ("contento", "Caricia en el sensor táctil", {"formas": [
        arco(94, 104, 110, 58, 126, 104, 16),
        arco(194, 104, 210, 58, 226, 104, 16),
    ]}),
    ("sospecha", "Doble toque", {"formas": [
        pildora(92, 70, 36, 52),
        corte(192, 52, 36, 122, izq=0, der=18),
    ]}),
    ("sorpresa", "Lo levantan (IMU)", {"formas": [
        circulo(110, 86, 30), circulo(210, 86, 30),
        circulo(120, 74, 8, color="pantalla"), circulo(220, 74, 8, color="pantalla"),
    ]}),
    ("dormido", "10 min sin interacción", {"parpadea": False, "formas": [
        pildora(92, 92, 36, 10), pildora(192, 92, 36, 10),
        poligono(Z_GRANDE, color="tinta-suave", fijo=True),
        poligono(Z_CHICA, color="tinta-suave", fijo=True),
    ]}),
    ("enojado", "Lo despiertan de la siesta", {"formas": [
        corte(92, 50, 36, 122, izq=0, der=22),
        corte(192, 50, 36, 122, izq=22, der=0),
    ]}),
    ("triste", "Boca abajo o 24 h sin uso", {"formas": [
        corte(92, 54, 36, 122, izq=18, der=0),
        corte(192, 54, 36, 122, izq=0, der=18),
    ]}),
    ("guino", "Vínculo exitoso con la app", {"formas": [
        pildora(92, 50, 36, 72),
        arco(194, 96, 210, 66, 226, 96, 14),
    ]}),
    ("enamorado", "Su dueño abre la app", {"formas": [
        {"forma": "corazon", "x": nx(110), "y": ny(86), "tamano": 1.0},
        {"forma": "corazon", "x": nx(210), "y": ny(86), "tamano": 1.0},
    ]}),
    ("mareado", "Sacudida fuerte (IMU)", {"parpadea": False, "formas": [
        # r(θ) = 3 + 1,41·θ px, de θ = 0 a 16,34 rad (2,6 vueltas), trazo 7.
        {"forma": "espiral", "x": nx(110), "y": ny(86), "r0": n(3), "paso": n(1.41),
         "giro": 16.34, "trazo": n(7)},
        {"forma": "espiral", "x": nx(210), "y": ny(86), "r0": n(3), "paso": n(1.41),
         "giro": 16.34, "trazo": n(7)},
    ]}),
    ("escuchando", "Palabra de activación", {"formas": [
        pildora(92, 36, 36, 56), pildora(192, 36, 36, 56),
        pildora(140, 128, 5, 8, fijo=True, audio=0),
        pildora(149, 123, 5, 18, fijo=True, audio=1),
        pildora(158, 118, 5, 28, fijo=True, audio=2),
        pildora(167, 124, 5, 16, fijo=True, audio=3),
        pildora(176, 127, 5, 10, fijo=True, audio=4),
    ]}),
    ("pensando", "Esperando la respuesta de la IA", {"formas": [
        pildora(106, 34, 32, 54), pildora(206, 34, 32, 54),
        circulo(240, 140, 4, color="tinta-suave", fijo=True),
        circulo(254, 140, 4, color="tinta-suave", fijo=True),
        circulo(268, 140, 4, color="tinta-suave", fijo=True),
    ]}),
    ("curioso", "Gira la cabeza (servo)", {"formas": [
        pildora(92, 46, 36, 80), pildora(196, 64, 28, 44),
    ]}),
    ("bateria-baja", "Menos de 15% de batería", {"formas": [
        corte(92, 82, 36, 122), corte(192, 82, 36, 122),
        pildora(266, 20, 28, 14, r=3, color="tinta-suave", fijo=True, relleno=False, trazo=n(2)),
        pildora(295, 24, 3, 6, r=1, color="tinta-suave", fijo=True),
        pildora(269, 23, 5, 8, r=1, fijo=True),
    ]}),
    ("actualizando", "Firmware OTA en curso", {"formas": [
        pildora(92, 70, 36, 10), pildora(192, 70, 36, 10),
        pildora(92, 112, 136, 8, color="linea", fijo=True),
        # El relleno se dibuja a lo ancho de la barra por el progreso (en el manual, 84 de 136: 62%).
        pildora(92, 112, 136, 8, fijo=True, progreso=True),
    ]}),
    ("glitch", "Error de sistema", {"parpadea": False, "formas": NEUTRAL + [
        {"forma": "franjas", "bandas": [
            {"y": ny(40), "h": n(28), "dx": n(-5)},
            {"y": ny(68), "h": n(18), "dx": n(7)},
            {"y": ny(86), "h": n(18), "dx": n(-3)},
            {"y": ny(104), "h": n(28), "dx": n(4)},
        ]},
        pildora(64, 95, 56, 2, fijo=True),
        pildora(214, 95, 40, 2, fijo=True),
    ]}),
]


def main():
    DESTINO.mkdir(parents=True, exist_ok=True)
    for viejo in DESTINO.glob("*.json"):
        viejo.unlink()
    indice = []
    for i, (nombre, disparador, cuerpo) in enumerate(CARAS, start=1):
        doc = {"nombre": nombre, "numero": i, "disparador": disparador}
        doc.update(cuerpo)
        archivo = DESTINO / f"{i:02d}-{nombre}.json"
        archivo.write_text(json.dumps(doc, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        indice.append(archivo.name)
    print(f"{len(indice)} caras en {DESTINO}")


if __name__ == "__main__":
    main()
