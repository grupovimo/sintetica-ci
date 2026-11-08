package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Lo que cada módulo de la consola le cambia a la config (los mismos cambios
 * que los módulos de web/src/lib/consola). Todo puro: config en, config afuera.
 */
object Editar {
    const val MAX_LISTA = 16
    const val MAX_PALABRA = 16
    /** Nombres (del robot, del dueño, de expresiones y comportamientos), en bytes. */
    const val MAX_NOMBRE = 24

    // --- Carácter ---
    val RASGOS = listOf("energia", "sociabilidad", "picardia", "timidez", "curiosidad", "malGenio")
    val NOMBRES_RASGO = mapOf(
        "energia" to "Energía", "sociabilidad" to "Social", "picardia" to "Picardía",
        "timidez" to "Timidez", "curiosidad" to "Curiosidad", "malGenio" to "Mal genio",
    )
    /** El orden del dial (web: Dial.svelte). */
    val PRESETS_DIAL = listOf("picaro", "timido", "carinoso", "curioso", "dormilon", "grunon", "electrico", "equilibrado")
    val NOMBRES_PRESET = mapOf(
        "equilibrado" to "Equilibrado", "timido" to "Tímido", "picaro" to "Pícaro", "dormilon" to "Dormilón",
        "carinoso" to "Cariñoso", "curioso" to "Curioso", "grunon" to "Gruñón", "electrico" to "Eléctrico",
    )
    /** shared/formato/vida/personalidades.json, en el orden de RASGOS. */
    val PRESETS: Map<String, List<Double>> = mapOf(
        "equilibrado" to listOf(0.5, 0.5, 0.45, 0.3, 0.55, 0.25),
        "timido" to listOf(0.4, 0.35, 0.2, 0.9, 0.5, 0.15),
        "picaro" to listOf(0.65, 0.6, 0.95, 0.15, 0.6, 0.3),
        "dormilon" to listOf(0.15, 0.45, 0.3, 0.35, 0.3, 0.3),
        "carinoso" to listOf(0.5, 0.95, 0.4, 0.3, 0.45, 0.05),
        "curioso" to listOf(0.65, 0.5, 0.45, 0.2, 0.95, 0.2),
        "grunon" to listOf(0.45, 0.25, 0.3, 0.2, 0.35, 0.9),
        "electrico" to listOf(0.95, 0.7, 0.65, 0.1, 0.75, 0.35),
    )

    fun preset(c: JsonObject): String = c.obj("personalidad")?.texto("preset") ?: "equilibrado"

    fun rasgo(c: JsonObject, r: String): Double =
        c.obj("personalidad")?.numero(r) ?: PRESETS[preset(c)]?.get(RASGOS.indexOf(r)) ?: 0.5

    fun ponerPreset(c: JsonObject, p: String): JsonObject = c.con("personalidad", buildJsonObject {
        put("preset", p)
        PRESETS[p]?.forEachIndexed { i, v -> put(RASGOS[i], v) }
    })

    fun ponerRasgo(c: JsonObject, r: String, v: Double) = c.en("personalidad") { it.con(r, redondo(v)) }

    /** ¿Algún rasgo se movió del preset? */
    fun aMedida(c: JsonObject): Boolean {
        val p = PRESETS[preset(c)] ?: return false
        return RASGOS.withIndex().any { (i, r) -> Math.abs(rasgo(c, r) - p[i]) > 0.001 }
    }

    // --- Cara ---
    const val NARANJA = "#FF6A33"
    val COLORES = listOf(
        "#FF6A33" to "Naranja", "#FFB347" to "Ámbar", "#F3EEE8" to "Blanco", "#4ADE80" to "Verde",
        "#3DE0FF" to "Cian", "#A78BFA" to "Violeta", "#FF5FA2" to "Rosa", "#FF3B3B" to "Rojo",
    )
    val RAPIDAS = listOf(
        "contento" to "Contento", "enamorado" to "Enamorado", "guino" to "Guiño", "sorpresa" to "Sorpresa",
        "sospecha" to "Sospecha", "pensando" to "Pensando", "triste" to "Triste", "enojado" to "Enojado",
        "mareado" to "Mareado", "dormido" to "Dormido", "curioso" to "Curioso", "glitch" to "Glitch",
    )

    fun color(c: JsonObject) = (c.obj("cara")?.texto("color") ?: NARANJA).uppercase()
    fun tamano(c: JsonObject) = c.obj("cara")?.numero("tamano") ?: 1.0

    /** El naranja es el de fábrica: no se escribe. */
    fun ponerColor(c: JsonObject, hex: String) = c.en("cara") { if (hex.uppercase() == NARANJA) it.sin("color") else it.con("color", hex) }
    fun ponerTamano(c: JsonObject, v: Double) = c.en("cara") { it.con("tamano", redondo(v)) }

    // --- Su día ---
    fun aMin(h: String) = (h.take(2).toIntOrNull() ?: 0) * 60 + (h.drop(3).take(2).toIntOrNull() ?: 0)
    fun aHora(m: Int): String {
        val x = ((m % 1440) + 1440) % 1440
        return "%02d:%02d".format(x / 60, x % 60)
    }

    fun despierta(c: JsonObject) = c.obj("horario")?.texto("despierta") ?: "08:00"
    fun duerme(c: JsonObject) = c.obj("horario")?.texto("duerme") ?: "23:30"

    /** Mueve una hora de a 15 min; no deja que se despierte y se duerma a la misma hora. */
    fun moverHora(c: JsonObject, cual: String, minutos: Int): JsonObject {
        val d = despierta(c)
        val z = duerme(c)
        val nuevo = aHora(aMin(if (cual == "despierta") d else z) + minutos)
        if (nuevo == (if (cual == "despierta") z else d)) return c
        return c.con("horario", buildJsonObject {
            put("despierta", if (cual == "despierta") nuevo else d)
            put("duerme", if (cual == "duerme") nuevo else z)
        })
    }

    /** Dónde está despierto en la barra de 24 h (fracciones de 0 a 1; puede cruzar la medianoche). */
    fun tramos(despierta: String, duerme: String): List<Pair<Float, Float>> {
        val a = aMin(despierta) / 1440f
        val z = aMin(duerme) / 1440f
        return if (a < z) listOf(a to z) else listOf(0f to z, a to 1f)
    }

    val MESES = listOf("ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic")
    private val DIAS_MES = listOf(31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31)

    /** "MM-DD" o null (falta el mes o el día); el día no se pasa del mes. */
    fun mmdd(mes: Int, dia: Int): String? = if (mes in 1..12 && dia > 0) "%02d-%02d".format(mes, minOf(dia, DIAS_MES[mes - 1])) else null

    fun partes(v: String?): Pair<Int, Int> = if (v == null || v.length < 5) 0 to 0 else (v.take(2).toIntOrNull() ?: 0) to (v.drop(3).take(2).toIntOrNull() ?: 0)

    fun ponerCumple(c: JsonObject, deRobot: Boolean, mes: Int, dia: Int): JsonObject {
        val v = mmdd(mes, dia)
        return if (deRobot) c.con("cumple", v) else c.en("dueno") { it.con("cumple", v) }
    }

    fun ponerDueno(c: JsonObject, nombre: String): JsonObject {
        val v = cortarBytes(nombre.trim(), MAX_NOMBRE).ifEmpty { null }
        return c.en("dueno") { it.con("nombre", v) }
    }

    // --- Modos ---
    fun modo(c: JsonObject) = c.texto("modo") ?: "mascota"
    fun ponerModo(c: JsonObject, m: String) = if (m == "mascota") c.sin("modo") else c.con("modo", m)
    fun trabajo(c: JsonObject) = c.obj("pomodoro")?.numero("trabajo")?.toInt() ?: 25
    fun descanso(c: JsonObject) = c.obj("pomodoro")?.numero("descanso")?.toInt() ?: 5

    fun moverPomodoro(c: JsonObject, cual: String, d: Int): JsonObject {
        var t = trabajo(c)
        var x = descanso(c)
        if (cual == "trabajo") t = (t + d).coerceIn(1, 120) else x = (x + d).coerceIn(1, 60)
        return c.en("pomodoro") { it.con("trabajo", t).con("descanso", x) }
    }

    // --- Imágenes ---
    val ESPERAS = listOf(3, 5, 10, 30, 60, 300, 900, 3600)
    fun textoEspera(s: Int) = if (s < 60) "$s s" else "${s / 60} min"

    fun enLista(c: JsonObject, espacio: Int) = "imagen:$espacio" in c.textos("lista")

    /** Sumarla a lo que muestra: si la lista estaba vacía, primero va su cara. */
    fun mostrar(c: JsonObject, espacio: Int, si: Boolean): JsonObject {
        val l = c.textos("lista").ifEmpty { listOf("cara:neutral") }
        val k = "imagen:$espacio"
        val nueva = if (si) (if (k in l) l else (l + k).take(MAX_LISTA)) else l.filter { it != k }
        return if (nueva == listOf("cara:neutral")) c.sin("lista") else c.con("lista", textosJson(nueva))
    }

    fun protector(c: JsonObject): Int? = c.obj("protector")?.numero("imagen")?.toInt()
    fun espera(c: JsonObject): Int = c.obj("protector")?.numero("espera")?.toInt() ?: 10

    fun ponerProtector(c: JsonObject, espacio: Int?): JsonObject {
        if (espacio == null) return c.sin("protector")
        val e = c.obj("protector")?.numero("espera")?.toInt()
        return c.con("protector", buildJsonObject { put("imagen", espacio); if (e != null) put("espera", e) })
    }

    fun moverEspera(c: JsonObject, d: Int): JsonObject {
        if (c.obj("protector") == null) return c
        val i = ESPERAS.indexOfFirst { it >= espera(c) }.coerceAtLeast(0)
        val n = ESPERAS[(i + d).coerceIn(0, ESPERAS.size - 1)]
        return c.en("protector") { it.con("espera", n) }
    }

    /** Al borrar una imagen: sale de la lista y deja de ser el protector. */
    fun olvidarImagen(c: JsonObject, espacio: Int): JsonObject {
        var r = if (enLista(c, espacio)) mostrar(c, espacio, false) else c
        if (protector(r) == espacio) r = ponerProtector(r, null)
        return r
    }

    // --- Voz ---
    fun voz(c: JsonObject) = c.obj("voz") ?: VACIO
    fun ponerVoz(c: JsonObject, f: (JsonObject) -> JsonObject) = c.en("voz", f)

    // --- Ajustes ---
    fun brillo(c: JsonObject) = c.obj("limites_marca")?.numero("brillo_max") ?: 0.9
    fun ponerBrillo(c: JsonObject, v: Double) = c.en("limites_marca") { it.con("brillo_max", redondo(v)) }

    /** La consola siempre escribe la v2: la v1 decía cuándo duerme ("sueno": "HH:MM-HH:MM"). */
    fun aV2(c: JsonObject): JsonObject {
        var r = c
        val p = r.obj("personalidad")
        val sueno = p?.texto("sueno")
        if (p != null && sueno != null && sueno.length >= 11) {
            if (r.obj("horario") == null) r = r.con("horario", buildJsonObject { put("despierta", sueno.substring(6)); put("duerme", sueno.take(5)) })
            r = r.con("personalidad", p.sin("sueno"))
        }
        return r.con("version", 2)
    }

    /** El onboarding lo arma de una: nombre, carácter, horario, vos. */
    fun encendido(c: JsonObject, nombre: String, preset: String, despierta: String, duerme: String, tuNombre: String, mes: Int, dia: Int): JsonObject {
        var r = ponerPreset(aV2(c), preset)
        if (nombre.isNotBlank()) r = r.con("nombre", cortarBytes(nombre.trim(), 24))
        r = r.con("horario", buildJsonObject { put("despierta", despierta); put("duerme", duerme) })
        if (tuNombre.isNotBlank()) r = ponerDueno(r, tuNombre)
        if (mmdd(mes, dia) != null) r = ponerCumple(r, false, mes, dia)
        return r
    }

    /** La primera vez que se abre un robot sin bautizar, se enciende. */
    fun sinEncender(c: JsonObject) = c.obj("dueno") == null && c.obj("personalidad")?.texto("preset") == null
}

/**
 * Reflejos: un patchbay. A la izquierda lo que le pasa, a la derecha cómo
 * reacciona; un cable los une. Se guardan como reglas simples
 * ({si, entonces: ["reflejo:x"]}); las más complejas no se tocan.
 */
object Reflejos {
    val ENTRADAS = listOf("tacto", "caricia", "doble-toque", "muchos-toques", "levantado", "sacudida-fuerte", "boca-abajo", "caida", "volvio")
    val NOMBRES_ENTRADA = mapOf(
        "tacto" to "Lo tocan", "caricia" to "Caricia", "doble-toque" to "Doble toque", "muchos-toques" to "Muchos toques",
        "levantado" to "Lo levantan", "sacudida-fuerte" to "Sacudón", "boca-abajo" to "Boca abajo", "caida" to "Se cae", "volvio" to "Volvés",
    )
    val SALIDAS = listOf(
        "contento" to "Contento", "enamorado" to "Enamorado", "risa" to "Se ríe", "guino" to "Guiña", "sorpresa" to "Sorpresa",
        "verguenza" to "Vergüenza", "mareado" to "Mareado", "enojado" to "Se enoja", "ofendido" to "Ofendido",
    )
    val DE_MOVIMIENTO = setOf("levantado", "sacudida-fuerte", "boca-abajo", "caida")
    const val MAX_REGLAS = 24

    private fun simple(r: JsonObject): Boolean {
        val si = r.texto("si") ?: return false
        val entonces = r.textos("entonces")
        return si in ENTRADAS && r["y"] == null && entonces.size == 1 && entonces[0].startsWith("reflejo:") &&
            SALIDAS.any { "reflejo:${it.first}" == entonces[0] }
    }

    /** entrada → salida, de las reglas simples. */
    fun cables(c: JsonObject): Map<String, String> = c.lista("reglas").orEmpty().mapNotNull { it as? JsonObject }.filter { simple(it) }
        .associate { it.texto("si")!! to it.textos("entonces")[0].removePrefix("reflejo:") }

    fun conectar(c: JsonObject, entrada: String, salida: String?): JsonObject {
        val otras = c.lista("reglas").orEmpty().filter { !((it as? JsonObject)?.let { r -> simple(r) && r.texto("si") == entrada } ?: false) }
        val nuevas = if (salida != null) (otras + buildJsonObject {
            put("si", entrada)
            put("entonces", JsonArray(listOf(JsonPrimitive("reflejo:$salida"))))
        }).take(MAX_REGLAS) else otras
        return if (nuevas.isEmpty()) c.sin("reglas") else c.con("reglas", JsonArray(nuevas))
    }

    /** Tocar una salida con una entrada elegida: conecta, o desconecta si ya iba ahí. */
    fun tocar(c: JsonObject, entrada: String, salida: String): JsonObject = conectar(c, entrada, if (cables(c)[entrada] == salida) null else salida)
}
