#!/usr/bin/env python3
"""Embebe shared/formato/caras/*.json en firmware/nucleo/caras_base.h y
shared/formato/vida/*.json (expresiones paramétricas, comportamientos, reglas,
personalidades y frases del motor de vida) en firmware/nucleo/vida_base.h y
shared/formato/charla/intenciones.json (la charla sin IA) en
firmware/nucleo/charla_base.h.

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
CHARLA = RAIZ / "shared" / "formato" / "charla" / "intenciones.json"
DESTINO_CHARLA = RAIZ / "firmware" / "nucleo" / "charla_base.h"


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


# ---------------------------------------------------------------- la charla

# El orden es el de AccionCharla (firmware/nucleo/charla.h); un test lo verifica.
ACCIONES_CHARLA = ["ninguna", "alarma", "cancelar-alarma", "posponer", "apagar-alarma", "recordatorio", "habito",
                   "jugar", "modo", "gusto", "olvidar-gusto", "toque", "musica", "dormir", "despertar", "dueno",
                   "volumen", "callar", "animo"]
NECESITA_CHARLA = {"hora": 1, "texto": 2, "juego": 4, "habito": 8, "quien": 16}
REQUIERE_CHARLA = {"": 0, "alarma": 1, "habitos": 2}
MARCAS_CHARLA = {"sigue": 1, "ia": 2, "manana": 4, "mayuscula": 8}
PLANTILLAS_CHARLA = {"dueno", "robot", "hora", "fecha", "dia", "clima", "cuando", "cuando_dia", "a_las", "minutos",
                     "texto", "quien", "toque", "juego", "habito", "habitos", "gustos", "agenda", "musica", "dias",
                     "resultado"}
PRESETS_CHARLA = {"todos", "timido", "picaro", "dormilon", "carinoso", "curioso", "grunon", "electrico"}
ANIMOS_CHARLA = ["bien", "medio", "mal"]
# Las acciones que cambian algo: si no está seguro, pregunta antes («confirmar»).
CONFIRMAN_CHARLA = {"alarma", "cancelar-alarma", "recordatorio", "gusto", "olvidar-gusto", "toque", "modo", "volumen",
                    "dueno"}


def plegar(t: str) -> str:
    """Como normaliza el núcleo: minúsculas ASCII, sin tildes."""
    import unicodedata
    t = unicodedata.normalize("NFD", t.lower())
    return "".join(c for c in t if unicodedata.category(c) != "Mn")


def patron_valido(p: str) -> bool:
    import re
    toks = p.removeprefix("~").split(" ")
    return all(re.fullmatch(r"[a-z0-9]+\*?(\|[a-z0-9]+\*?)*\??", t) for t in toks) and \
        any(not t.endswith("?") for t in toks) and len(toks) <= 8


def generar_charla() -> str:
    import re
    datos = json.loads(CHARLA.read_text(encoding="utf-8"))
    expresiones = set(json.loads((VIDA / "expresiones.json").read_text(encoding="utf-8")))
    comportamientos = set(json.loads((VIDA / "comportamientos.json").read_text(encoding="utf-8")))
    patrones, respuestas = [], []

    def mal(donde, que):
        sys.exit(f"intenciones.json: {donde}: {que}")

    def sumar_patrones(lista, donde):
        primero = len(patrones)
        for p in lista:
            q = plegar(p)
            if not patron_valido(q):
                mal(donde, f"patrón inválido {p!r}")
            patrones.append(q)
        return primero, len(patrones) - primero

    def sumar_respuestas(grupo, donde, falta=False):
        primero = len(respuestas)
        for preset, lista in (grupo or {}).items():
            # En "falta", también por lo que falta ("hora", "texto"): "@hora".
            if falta and preset in NECESITA_CHARLA:
                preset = "@" + preset
            elif preset not in PRESETS_CHARLA:
                mal(donde, f"preset desconocido {preset!r}")
            for t in lista:
                n = len(t.encode("utf-8"))
                if not 1 <= n <= LARGO_FRASE:
                    mal(donde, f"{t!r} tiene {n} bytes (de 1 a {LARGO_FRASE})")
                for clave in re.findall(r"\{([^}]*)\}", t):
                    if clave not in PLANTILLAS_CHARLA:
                        mal(donde, f"plantilla desconocida {{{clave}}} en {t!r}")
                respuestas.append(f'    {{{c_texto("" if preset == "todos" else preset)}, {c_texto(t)}}},')
        return primero, len(respuestas) - primero

    def grupos(nombre, lista):
        filas = []
        for g in lista:
            pp, np_ = sumar_patrones(g["patrones"], f"{nombre}/{g['nombre']}")
            filas.append(f'    {{"{g["nombre"]}", {c_texto(g.get("decir", g["nombre"]))}, {pp}, {np_}}},')
        return filas

    juegos = grupos("juegos", datos["juegos"])
    modos = grupos("modos", datos["modos"])
    toques = grupos("toques", datos["toques"])
    nombres = {"juegos": [g["nombre"] for g in datos["juegos"]], "modos": [g["nombre"] for g in datos["modos"]],
               "toques": [g["nombre"] for g in datos["toques"]]}
    ne = sumar_respuestas(datos["no_entendi"], "no_entendi")
    ca = sumar_respuestas(datos.get("cancelado"), "cancelado")
    if not ca[1]:
        mal("cancelado", "no tiene respuestas")
    filas = []
    for nombre, it in datos["intenciones"].items():
        accion = it.get("accion", "ninguna")
        if accion not in ACCIONES_CHARLA:
            mal(nombre, f"acción desconocida {accion!r}")
        dato = it.get("dato", -1)
        if isinstance(dato, str):
            tabla = {"jugar": nombres["juegos"], "modo": nombres["modos"], "toque": nombres["toques"],
                     "animo": ANIMOS_CHARLA}.get(accion)
            if not tabla or dato not in tabla:
                mal(nombre, f"dato {dato!r} no sirve para {accion}")
            dato = tabla.index(dato)
        necesita = 0
        for n in it.get("necesita", []):
            if n not in NECESITA_CHARLA:
                mal(nombre, f"necesita desconocido {n!r}")
            necesita |= NECESITA_CHARLA[n]
        requiere = it.get("requiere", "")
        if requiere not in REQUIERE_CHARLA:
            mal(nombre, f"requiere desconocido {requiere!r}")
        marcas = sum(v for k, v in MARCAS_CHARLA.items() if it.get(k))
        expresion = it.get("expresion", "")
        if expresion and expresion not in expresiones:
            mal(nombre, f"la expresión {expresion!r} no existe")
        comp = it.get("comportamiento", "")
        if comp and comp not in comportamientos:
            mal(nombre, f"el comportamiento {comp!r} no existe")
        texto = it.get("texto", "")
        if len(texto.encode("utf-8")) > 40:
            mal(nombre, "el texto por defecto pasa los 40 bytes")
        pp, np_ = sumar_patrones(it["patrones"], nombre)
        r = sumar_respuestas(it.get("respuestas"), nombre)
        if not r[1]:
            mal(nombre, "no tiene respuestas")
        f = sumar_respuestas(it.get("falta"), nombre, True)
        if necesita and not f[1]:
            mal(nombre, "necesita algo pero no tiene «falta»")
        s = sumar_respuestas(it.get("sin"), nombre)
        co = sumar_respuestas(it.get("confirmar"), nombre)
        if accion in CONFIRMAN_CHARLA and not co[1]:
            mal(nombre, "cambia algo pero no tiene «confirmar»")
        if co[1] and accion not in CONFIRMAN_CHARLA:
            mal(nombre, "tiene «confirmar» pero no cambia nada")
        no_empieza = [plegar(w) for w in it.get("no_empieza", [])]
        if any(not re.fullmatch(r"[a-z0-9]+", w) for w in no_empieza):
            mal(nombre, "no_empieza: palabras sueltas, sin tildes")
        filas.append(f'    {{"{nombre}", {ACCIONES_CHARLA.index(accion)}, {int(dato)}, {necesita}, '
                     f'{REQUIERE_CHARLA[requiere]}, {marcas}, {c_texto(expresion)}, {c_texto(comp)}, {c_texto(texto)}, '
                     f'{c_texto("|".join(no_empieza))}, {pp}, {np_}, '
                     f'{{{{{r[0]}, {r[1]}}}, {{{f[0]}, {f[1]}}}, {{{s[0]}, {s[1]}}}, {{{co[0]}, {co[1]}}}}}}},')
    relleno = sorted({plegar(w) for w in datos["relleno"]})
    abrev = [(plegar(k), plegar(v)) for k, v in datos["abreviaturas"].items()]
    partes = [
        "// GENERADO por firmware/herramientas/embeber_caras.py desde shared/formato/charla/intenciones.json.",
        "// No editar a mano. Las tablas van a la flash (const): no ocupan RAM.",
        "#pragma once",
        "",
        "#include <stdint.h>",
        "",
        "namespace sintetica {",
        "",
        "struct IntencionCharla {",
        "  const char* nombre;",
        "  uint8_t accion;        // AccionCharla",
        "  int8_t dato;           // el juego, el modo, el toque, el ánimo o el volumen (-1 = ninguno)",
        "  uint8_t necesita;      // bits: 1 hora, 2 texto, 4 juego, 8 hábito, 16 quién",
        "  uint8_t requiere;      // 1 la alarma sonando, 2 tener hábitos",
        "  uint8_t marcas;        // 1 sigue, 2 mejor la IA, 4 hora de la mañana, 8 texto con mayúscula",
        "  const char* expresion;",
        "  const char* comportamiento;",
        "  const char* texto;     // el texto por defecto (temporizador)",
        "  const char* noEmpieza; // tu|que|cuando: si el texto libre empieza así, no es esta",
        "  uint16_t patron, nPatrones;  // en PATRONES_CHARLA (~ adelante: dudoso, pregunta antes)",
        "  uint16_t grupo[4][2];  // respuestas, falta, sin y confirmar: primera y cantidad en RESPUESTAS_CHARLA",
        "};",
        "",
        "struct RespuestaCharlaBase {",
        '  const char* preset;  // "" = para todos',
        "  const char* texto;",
        "};",
        "",
        "struct GrupoCharla {",
        "  const char* nombre;",
        "  const char* decir;",
        "  uint16_t patron, nPatrones;",
        "};",
        "",
        "struct AbreviaturaCharla {",
        "  const char* de;",
        "  const char* a;",
        "};",
        "",
        f"constexpr int CHARLA_UMBRAL = {round(float(datos['umbral']) * 100)};  // centésimos",
        f"constexpr int CHARLA_UMBRAL_ACCION = {round(float(datos.get('umbral_accion', datos['umbral'])) * 100)};"
        "  // para hacer sin preguntar lo que cambia algo",
        f"constexpr int CHARLA_CONTEXTO_S = {int(datos['contexto'])};",
        "",
        f"constexpr int CANTIDAD_ACCIONES_CHARLA = {len(ACCIONES_CHARLA)};",
        "inline const char* const ACCIONES_CHARLA[CANTIDAD_ACCIONES_CHARLA] = {"
        + ", ".join(f'"{a}"' for a in ACCIONES_CHARLA) + "};",
        "",
        f"constexpr int CANTIDAD_PATRONES_CHARLA = {len(patrones)};",
        "inline const char* const PATRONES_CHARLA[CANTIDAD_PATRONES_CHARLA] = {",
        *[f"    {c_texto(p)}," for p in patrones],
        "};",
        "",
        f"constexpr int CANTIDAD_RESPUESTAS_CHARLA = {len(respuestas)};",
        "inline const RespuestaCharlaBase RESPUESTAS_CHARLA[CANTIDAD_RESPUESTAS_CHARLA] = {",
        *respuestas,
        "};",
        f"constexpr int NO_ENTENDI_PRIMERA = {ne[0]}, NO_ENTENDI_CANTIDAD = {ne[1]};",
        f"constexpr int CANCELADO_PRIMERA = {ca[0]}, CANCELADO_CANTIDAD = {ca[1]};",
        "",
        f"constexpr int CANTIDAD_INTENCIONES_CHARLA = {len(filas)};",
        "inline const IntencionCharla INTENCIONES_CHARLA[CANTIDAD_INTENCIONES_CHARLA] = {",
        *filas,
        "};",
        "",
        f"constexpr int CANTIDAD_JUEGOS_CHARLA = {len(juegos)};",
        "inline const GrupoCharla JUEGOS_CHARLA[CANTIDAD_JUEGOS_CHARLA] = {", *juegos, "};",
        f"constexpr int CANTIDAD_MODOS_CHARLA = {len(modos)};",
        "inline const GrupoCharla MODOS_CHARLA[CANTIDAD_MODOS_CHARLA] = {", *modos, "};",
        f"constexpr int CANTIDAD_TOQUES_CHARLA = {len(toques)};",
        "inline const GrupoCharla TOQUES_CHARLA[CANTIDAD_TOQUES_CHARLA] = {", *toques, "};",
        "",
        f"constexpr int CANTIDAD_RELLENO_CHARLA = {len(relleno)};",
        "inline const char* const RELLENO_CHARLA[CANTIDAD_RELLENO_CHARLA] = {"
        + ", ".join(c_texto(w) for w in relleno) + "};",
        f"constexpr int CANTIDAD_ABREVIATURAS_CHARLA = {len(abrev)};",
        "inline const AbreviaturaCharla ABREVIATURAS_CHARLA[CANTIDAD_ABREVIATURAS_CHARLA] = {",
        *[f"    {{{c_texto(a)}, {c_texto(b)}}}," for a, b in abrev],
        "};",
        "",
        "}  // namespace sintetica",
        "",
    ]
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
    for destino, texto in ((DESTINO, generar()), (DESTINO_VIDA, generar_vida()), (DESTINO_CHARLA, generar_charla())):
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
