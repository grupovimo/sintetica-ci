#!/usr/bin/env python3
"""Arma el corpus semilla de cada blanco (firmware/fuzz/corpus/<blanco>/) a
partir de lo que ya está en el repo: las configs de ejemplo, los datos del
motor de vida, los vectores del protocolo y QOI chicos válidos.

    python3 firmware/fuzz/generar_semillas.py

Se commitea el resultado; correrlo de nuevo solo hace falta si cambian esos
archivos. Los formatos de entrada de cada arnés están en fuzz_<blanco>.cpp."""

import json
import os
import shutil
import struct
import zlib

AQUI = os.path.dirname(os.path.abspath(__file__))
RAIZ = os.path.normpath(os.path.join(AQUI, "..", ".."))
FORMATO = os.path.join(RAIZ, "shared", "formato")
VECTORES = os.path.join(RAIZ, "shared", "protocolo", "vectores.json")


def leer_json(*partes):
    with open(os.path.join(*partes), encoding="utf-8") as f:
        return json.load(f)


def bytes_hex(h):
    return bytes(int(x, 16) for x in h.split())


def compacto(o):
    return json.dumps(o, ensure_ascii=False, separators=(",", ":")).encode()


# --- QOI ----------------------------------------------------------------------

def qoi(ancho, alto, pixeles, canales=3):
    """Un codificador QOI mínimo (y correcto) para las semillas."""
    out = bytearray(b"qoif" + struct.pack(">II", ancho, alto) + bytes([canales, 0]))
    indice = [(0, 0, 0, 0)] * 64
    prev = (0, 0, 0, 255)
    racha = 0
    for i, px in enumerate(pixeles):
        if px == prev:
            racha += 1
            if racha == 62 or i == len(pixeles) - 1:
                out.append(0xC0 | (racha - 1))
                racha = 0
            continue
        if racha:
            out.append(0xC0 | (racha - 1))
            racha = 0
        r, g, b, a = px
        h = (r * 3 + g * 5 + b * 7 + a * 11) % 64
        if indice[h] == px:
            out.append(h)
        else:
            indice[h] = px
            if a == prev[3]:
                dr, dg, db = (r - prev[0] + 128) % 256 - 128, (g - prev[1] + 128) % 256 - 128, (b - prev[2] + 128) % 256 - 128
                dgr, dgb = dr - dg, db - dg
                if -2 <= dr <= 1 and -2 <= dg <= 1 and -2 <= db <= 1:
                    out.append(0x40 | (dr + 2) << 4 | (dg + 2) << 2 | (db + 2))
                elif -32 <= dg <= 31 and -8 <= dgr <= 7 and -8 <= dgb <= 7:
                    out += bytes([0x80 | (dg + 32), (dgr + 8) << 4 | (dgb + 8)])
                else:
                    out += bytes([0xFE, r, g, b])
            else:
                out += bytes([0xFF, r, g, b, a])
        prev = px
    return bytes(out + b"\x00" * 7 + b"\x01")


def degradado(ancho, alto, alfa=False):
    return [((x * 255) // max(1, ancho - 1), (y * 255) // max(1, alto - 1), (x ^ y) & 0xFF,
             (x * 16) & 0xFF if alfa else 255) for y in range(alto) for x in range(ancho)]


def liso(ancho, alto, c=(245, 80, 26, 255)):
    return [c] * (ancho * alto)


def sani(ancho, alto, cuadros):
    """Una animación "sani": cabecera, tabla y los QOI uno detrás del otro."""
    tabla = 10 + 10 * len(cuadros)
    cab = bytearray(b"sani" + bytes([1, len(cuadros)]) + struct.pack("<HH", ancho, alto))
    cuerpo = bytearray()
    for ms, q in cuadros:
        cab += struct.pack("<HII", ms, tabla + len(cuerpo), len(q))
        cuerpo += q
    return bytes(cab + cuerpo)


# --- Protocolo (formato de fuzz_protocolo.cpp) ---------------------------------

def crudo(car, datos, nube=False):
    return bytes([1 if nube else 0, car]) + struct.pack("<H", len(datos)) + datos


def tick(ms16=1):
    return bytes([2, ms16])


def config(datos):
    return bytes([3]) + struct.pack("<H", len(datos)) + datos


def imagen(espacio, datos):
    return bytes([4, espacio]) + struct.pack("<H", len(datos)) + datos


def escribir(carpeta, nombre, datos):
    with open(os.path.join(carpeta, nombre), "wb") as f:
        f.write(datos)


def main():
    corpus = os.path.join(AQUI, "corpus")
    for b in ("config", "protocolo", "qoi", "expresion"):
        d = os.path.join(corpus, b)
        shutil.rmtree(d, ignore_errors=True)
        os.makedirs(d)
    vec = leer_json(VECTORES)

    # --- config
    cfgs = {}
    for n in ("config-ejemplo.json", "config-ejemplo-v2.json"):
        with open(os.path.join(FORMATO, n), "rb") as f:
            cfgs[n] = f.read()
    cfgs["vectores.json"] = vec["config"]["json"].encode()
    exprs = leer_json(FORMATO, "vida", "expresiones.json")
    comps = leer_json(FORMATO, "vida", "comportamientos.json")
    reglas = leer_json(FORMATO, "vida", "reglas.json")
    pers = leer_json(FORMATO, "vida", "personalidades.json")
    cfgs["vida-expresiones.json"] = compacto({"version": 2, "expresiones": dict(list(exprs.items())[:12])})
    cfgs["vida-comportamientos.json"] = compacto({"version": 2, "comportamientos": dict(list(comps.items())[:12])
                                                  if isinstance(comps, dict) else comps})
    cfgs["vida-reglas.json"] = compacto({"version": 2, "reglas": reglas[:12] if isinstance(reglas, list) else reglas})
    if isinstance(pers, dict):
        for k in list(pers)[:3]:
            cfgs[f"preset-{k}.json"] = compacto({"version": 2, "personalidad": {"preset": k}})
    for n in sorted(os.listdir(os.path.join(FORMATO, "animaciones"))):
        a = leer_json(FORMATO, "animaciones", n)
        nombre = a.pop("nombre", n[:-5])
        cfgs[f"anim-{n}"] = compacto({"version": 1, "animaciones": {nombre: a}, "lista": [f"animacion:{nombre}", "cara:neutral", "imagen:0"]})
    cfgs["modo-reloj.json"] = b'{"version":2,"modo":"reloj"}'
    cfgs["modo-pomodoro.json"] = b'{"version":2,"modo":"pomodoro","pomodoro":{"trabajo":1,"descanso":1}}'
    cfgs["lista-imagen.json"] = b'{"version":2,"lista":["imagen:0","imagen:1","cara:neutral"],"rotar":1}'
    cfgs["minima.json"] = b'{"version":2}'
    for n, d in cfgs.items():
        escribir(os.path.join(corpus, "config"), n, d)

    # --- qoi: [control][archivo]
    qois = {
        "liso-12x7": qoi(12, 7, liso(12, 7)),
        "degradado-16x9": qoi(16, 9, degradado(16, 9)),
        "rgba-8x8": qoi(8, 8, degradado(8, 8, alfa=True), canales=4),
        "uno-1x1": qoi(1, 1, liso(1, 1)),
    }
    qv = bytes_hex(vec["qoi"]["hex"])
    qois["vectores"] = qv
    anim = sani(12, 7, [(80, qoi(12, 7, liso(12, 7))), (120, qoi(12, 7, degradado(12, 7)))])
    for n, d in qois.items():
        escribir(os.path.join(corpus, "qoi"), n, bytes([0x00]) + d)
        escribir(os.path.join(corpus, "qoi"), n + "-trozos", bytes([0xC3]) + d)
    escribir(os.path.join(corpus, "qoi"), "sani-2", bytes([0x05]) + anim)
    escribir(os.path.join(corpus, "qoi"), "sani-2-rgba", bytes([0x41]) + anim)

    # --- protocolo: [control][registros]
    p = os.path.join(corpus, "protocolo")
    comandos = [bytes_hex(c["hex"]) for c in vec["comandos"]]
    escribir(p, "comandos", bytes([0]) + b"".join(crudo(2, c) + tick() for c in comandos))
    escribir(p, "comandos-nube", bytes([1]) + b"".join(crudo(2, c, nube=True) for c in comandos))
    escribir(p, "sin-autenticar", bytes([4]) + b"".join(crudo(2, c) for c in comandos[:8]))
    # Las escrituras de los vectores (config, qoi, ota), tal cual.
    for clave, car in (("config", 1), ("qoi", 5), ("ota", 4)):
        esc = vec[clave].get("escrituras", [])
        datos = b"".join(crudo(car, bytes_hex(e if isinstance(e, str) else e["hex"])) for e in esc)
        escribir(p, f"vectores-{clave}", bytes([0]) + datos + tick() + crudo(1, b"\x04"))
    escribir(p, "config-v2", bytes([0]) + config(cfgs["config-ejemplo-v2.json"]) + tick(4) + crudo(1, b"\x04"))
    escribir(p, "config-v1-broche", bytes([1]) + config(cfgs["config-ejemplo.json"]) + tick() + tick(200))
    # Imágenes del tamaño de cada pantalla, en la lista, y unos cuadros para verlas.
    lista = config(cfgs["lista-imagen.json"])
    for nombre, control, w, h in (("147b", 0, 320, 172), ("broche", 1, 240, 240)):
        img = qoi(w, h, liso(w, h, (10, 200, 90, 255)))
        an = sani(w, h, [(40, qoi(w, h, liso(w, h))), (40, img)])
        escribir(p, f"imagen-{nombre}", bytes([control]) + imagen(0, img) + lista + b"".join(tick(255) for _ in range(6)) +
                 crudo(5, b"\x24") + crudo(5, b"\x25\x00"))
        escribir(p, f"animacion-{nombre}", bytes([control]) + imagen(1, an) + lista + b"".join(tick(255) for _ in range(6)))
    escribir(p, "ticks", bytes([0]) + b"".join(tick(255) for _ in range(10)) + bytes([5, 1, 2, 4, 5, 0, 6, 3, 7, 0, 7, 1]))
    escribir(p, "vista-previa", bytes([2]) + config(cfgs["config-ejemplo-v2.json"]) + tick() + tick())

    # --- expresion: [nombre][ancho][alto][forma][t u16][json]
    e = os.path.join(corpus, "expresion")
    for i in range(0, 40, 7):
        escribir(e, f"fabrica-{i}", bytes([i, 48, 32, i & 1]) + struct.pack("<H", 150 * i))
    escribir(e, "propia-contento", bytes([0x80, 64, 40, 0, 10, 0]) + b"contento\0" + cfgs["config-ejemplo-v2.json"])
    escribir(e, "propia-picaro", bytes([0x80, 40, 40, 1, 200, 1]) + b"picaro\0" + cfgs["config-ejemplo-v2.json"])
    escribir(e, "fabrica-con-config", bytes([3, 80, 50, 0, 0, 2]) + cfgs["vida-expresiones.json"])

    # Los diccionarios: claves y valores de los JSON del repo.
    palabras = set()

    def recorrer(o):
        if isinstance(o, dict):
            for k, v in o.items():
                palabras.add(json.dumps(k, ensure_ascii=False))
                recorrer(v)
        elif isinstance(o, list):
            for v in o:
                recorrer(v)
        elif isinstance(o, str) and len(o) <= 40:
            palabras.add(json.dumps(o, ensure_ascii=False))

    for d in cfgs.values():
        recorrer(json.loads(d))
    recorrer(exprs)
    recorrer(comps)
    recorrer(reglas)
    recorrer(leer_json(FORMATO, "config.schema.json").get("properties", {}))
    for t in ("{", "}", "[", "]", ":", ",", "true", "false", "null", "-0.5", "1e9", "\\u00e1", "\\ud83d\\ude00"):
        palabras.add(json.dumps(t))

    def dic(nombre, entradas):
        with open(os.path.join(corpus, nombre + ".dict"), "w", encoding="utf-8") as f:
            f.write(f"# Diccionario de fuzz_{nombre} (generado por generar_semillas.py)\n")
            for x in sorted(entradas):
                f.write(escapar(x) + "\n")

    def escapar(x):
        # libFuzzer solo entiende ASCII: lo demás (y las comillas de adentro), con \xNN.
        if x.startswith('"') and x.endswith('"') and "\\x" in x:
            return x
        cuerpo = x[1:-1].encode()
        return '"' + "".join(chr(c) if 32 <= c < 127 and c not in (34, 92) else f"\\x{c:02X}" for c in cuerpo) + '"'

    def binario(b):
        return '"' + "".join(f"\\x{c:02X}" for c in b) + '"'

    dic("config", palabras)
    dic("expresion", palabras)
    dic("qoi", {binario(b"qoif"), binario(b"sani\x01"), binario(b"\x00" * 7 + b"\x01"), binario(b"\xFE"), binario(b"\xFF"),
                binario(b"\x00\x00\x01\x40"), binario(b"\x00\x00\x00\xAC"), binario(b"\xFF\xFF\xFF\xFF")})
    ops = {binario(bytes([o])) for o in list(range(0x01, 0x05)) + list(range(0x10, 0x20)) + list(range(0x20, 0x26)) +
           list(range(0x30, 0x36))}
    dic("protocolo", ops | {binario(b"qoif"), binario(b"sani\x01"), binario(struct.pack("<H", 320)),
                            binario(struct.pack("<H", 172)), binario(struct.pack("<H", 240))} |
        {x for x in palabras if len(x) < 24})


if __name__ == "__main__":
    main()
