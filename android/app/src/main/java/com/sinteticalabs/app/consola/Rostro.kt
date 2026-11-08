package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.put

/** Un parámetro del rostro (firmware/nucleo/rostro.cpp): clave, mínimo, máximo y de fábrica. */
data class Parametro(val clave: String, val min: Double, val max: Double, val defecto: Double)

val PARAMS_OJO = listOf(
    Parametro("w", 0.03, 0.2, 0.105), Parametro("h", 0.03, 0.32, 0.23), Parametro("r", 0.0, 1.0, 1.0), Parametro("tilt", -0.6, 0.6, 0.0),
    Parametro("lt", 0.0, 0.5, 0.0), Parametro("lta", -0.8, 0.8, 0.0), Parametro("lb", 0.0, 0.5, 0.0), Parametro("lba", -0.8, 0.8, 0.0),
    Parametro("smile", 0.0, 1.0, 0.0), Parametro("heart", 0.0, 1.0, 0.0), Parametro("star", 0.0, 1.0, 0.0), Parametro("cross", 0.0, 1.0, 0.0),
    Parametro("ring", 0.0, 1.0, 0.0), Parametro("pupil", 0.0, 1.0, 0.0), Parametro("squash", 0.7, 1.3, 1.0),
)
val PARAMS_CARA = listOf(
    Parametro("gx", -0.3, 0.3, 0.0), Parametro("gy", -0.25, 0.25, 0.0), Parametro("ey", -0.25, 0.25, 0.0), Parametro("sep", 0.15, 0.45, 0.3),
    Parametro("ftilt", -0.6, 0.6, 0.0), Parametro("fscale", 0.7, 1.3, 1.0), Parametro("spin", 0.0, 1.0, 0.0), Parametro("orbit", 0.0, 1.0, 0.0),
    Parametro("pulse", 0.0, 1.0, 0.0), Parametro("beat", 0.0, 1.0, 0.0), Parametro("glitch", 0.0, 1.0, 0.0), Parametro("dim", 0.0, 1.0, 0.0),
    Parametro("hue", 0.0, 1.0, 0.0), Parametro("mo", 0.0, 0.2, 0.0), Parametro("mw", 0.02, 0.2, 0.07), Parametro("mcurve", -1.0, 1.0, 0.0),
    Parametro("blush", 0.0, 1.0, 0.0), Parametro("shake", 0.0, 1.0, 0.0),
)
val LIMITES: Map<String, Parametro> = (PARAMS_OJO + PARAMS_CARA).associateBy { it.clave }

val MOVIMIENTOS = listOf("ninguno", "rebote", "saltitos", "risa", "mirada-lado", "salto", "encogerse", "dormir", "temblor", "resoplido",
    "lagrima", "achicarse", "picaro", "latido", "bamboleo", "escuchar", "hablar", "pensar", "duda", "acercarse", "suspiro",
    "quieto", "cabeceo", "carga", "orbita", "buscar", "caer", "glitch", "estirarse")
val PARTICULAS = listOf("zzz", "corazon", "lagrima", "gota", "chispa", "pregunta", "exclamacion", "nota", "estrella", "burbuja")
val PARPADEOS = listOf("normal", "menos", "mas", "no", "lento", "rapido", "doble")
val MIRADAS = listOf("libre", "frente", "arriba", "abajo", "izquierda", "derecha", "lado", "alrededor", "perdida", "reojo", "estimulo", "rem", "ovejas")
val GLIFO = mapOf("zzz" to "z", "corazon" to "♥", "lagrima" to "💧", "gota" to "·", "chispa" to "✦", "pregunta" to "?",
    "exclamacion" to "!", "nota" to "♪", "estrella" to "★", "burbuja" to "○")

/** Los nombres del JSON: minúsculas, números y guiones (hasta 24). */
fun nombreValido(n: String) = Regex("[a-z][a-z0-9-]{0,23}").matches(n)

/**
 * Los valores de una cara: "izq.w", "der.w" para los ojos y "gx" para los
 * globales. Como Rostro de iOS y el editor de la web.
 */
object Rostro {
    val neutral: Map<String, Double> = buildMap {
        for (p in PARAMS_OJO) { put("izq.${p.clave}", p.defecto); put("der.${p.clave}", p.defecto) }
        for (p in PARAMS_CARA) put(p.clave, p.defecto)
    }

    private fun num(o: JsonObject?, k: String) = (o?.get(k) as? JsonPrimitive)?.doubleOrNull

    /** Aplica un documento encima (como el núcleo: primero los globales, después ojo, izq y der). */
    fun aplicar(doc: JsonObject, v: Map<String, Double>): Map<String, Double> {
        val r = v.toMutableMap()
        for (p in PARAMS_CARA) num(doc, p.clave)?.let { r[p.clave] = it }
        for ((lado, destinos) in listOf("ojo" to listOf("izq", "der"), "izq" to listOf("izq"), "der" to listOf("der"))) {
            val o = doc.obj(lado) ?: continue
            for (p in PARAMS_OJO) { val n = num(o, p.clave) ?: continue; for (d in destinos) r["$d.${p.clave}"] = n }
        }
        return r
    }

    /** Los valores de una cara de fábrica (con lo que extienda). */
    fun deFabrica(nombre: String, base: Map<String, JsonObject>, profundidad: Int = 0): Map<String, Double> {
        val doc = base[nombre] ?: return neutral
        if (profundidad > 5) return neutral
        val desde = doc.texto("extiende")?.let { deFabrica(it, base, profundidad + 1) } ?: neutral
        return aplicar(doc, desde)
    }

    private fun red(x: Double) = Math.round(x * 1000) / 1000.0
    private fun distinto(a: Double, z: Double) = Math.abs(a - z) > 0.0005

    /** Lo que difiere de la referencia, como documento (lo que se guarda). */
    fun documento(v: Map<String, Double>, ref: Map<String, Double>): JsonObject {
        val ojo = mutableMapOf<String, JsonElement>()
        val izq = mutableMapOf<String, JsonElement>()
        val der = mutableMapOf<String, JsonElement>()
        for (p in PARAMS_OJO) {
            val a = v["izq.${p.clave}"] ?: p.defecto
            val z = v["der.${p.clave}"] ?: p.defecto
            val ra = ref["izq.${p.clave}"] ?: p.defecto
            val rz = ref["der.${p.clave}"] ?: p.defecto
            if (!distinto(a, z) && (distinto(a, ra) || distinto(z, rz))) ojo[p.clave] = JsonPrimitive(red(a))
            else {
                if (distinto(a, ra)) izq[p.clave] = JsonPrimitive(red(a))
                if (distinto(z, rz)) der[p.clave] = JsonPrimitive(red(z))
            }
        }
        val d = LinkedHashMap<String, JsonElement>()
        if (ojo.isNotEmpty()) d["ojo"] = JsonObject(ojo)
        if (izq.isNotEmpty()) d["izq"] = JsonObject(izq)
        if (der.isNotEmpty()) d["der"] = JsonObject(der)
        for (p in PARAMS_CARA) {
            val a = v[p.clave] ?: p.defecto
            if (distinto(a, ref[p.clave] ?: p.defecto)) d[p.clave] = JsonPrimitive(red(a))
        }
        return JsonObject(d)
    }
}

/** Lo que el laboratorio necesita de la config. */
object Lab {
    /** Lo justo para dibujar las caras paramétricas: el color y las propias sin formas. */
    fun paraDibujar(c: JsonObject): String = buildJsonObject {
        put("version", 2)
        c.obj("cara")?.texto("color")?.let { col -> put("cara", buildJsonObject { put("color", col) }) }
        val propias = propiasParametricas(c)
        if (propias.isNotEmpty()) put("expresiones", JsonObject(propias))
    }.toString()

    fun propiasParametricas(c: JsonObject): Map<String, JsonObject> =
        c.obj("expresiones").orEmpty().mapNotNull { (k, v) -> (v as? JsonObject)?.takeIf { it["formas"] == null }?.let { k to it } }.toMap()

    fun propiosDibujos(c: JsonObject): Map<String, JsonObject> =
        c.obj("expresiones").orEmpty().mapNotNull { (k, v) -> (v as? JsonObject)?.takeIf { it["formas"] != null }?.let { k to it } }.toMap()

    fun ponerEn(c: JsonObject, clave: String, nombre: String, doc: JsonObject?): JsonObject = c.en(clave) { o ->
        val r = if (doc == null) o.sin(nombre) else o.con(nombre, doc)
        r.oNada()
    }

    fun ponerExpresion(c: JsonObject, n: String, doc: JsonObject?) = ponerEn(c, "expresiones", n, doc)
    fun ponerComportamiento(c: JsonObject, n: String, doc: JsonObject?) = ponerEn(c, "comportamientos", n, doc)
    fun ponerAnimacion(c: JsonObject, n: String, doc: JsonObject?) = ponerEn(c, "animaciones", n, doc)

    /** Un nombre libre: base, base-2, base-3… */
    fun libre(base: String, ocupados: Collection<String>): String {
        var n = base.take(24)
        var k = 2
        while (n in ocupados) n = "${base.take(20)}-${k++}"
        return n
    }

    // --- Comportamientos: la duración de un paso es un número o un rango [a, b] ---
    fun rango(v: JsonElement?): Pair<Double, Double> {
        if (v is JsonArray && v.size == 2) {
            val a = (v[0] as? JsonPrimitive)?.doubleOrNull ?: 1.0
            val b = (v[1] as? JsonPrimitive)?.doubleOrNull ?: a
            return a to b
        }
        val x = (v as? JsonPrimitive)?.doubleOrNull ?: 1.0
        return x to x
    }

    fun dura(a: Double, b: Double): JsonElement {
        fun r(x: Double) = Math.round(x.coerceIn(0.1, 60.0) * 10) / 10.0
        val ra = r(a)
        val rb = r(b)
        return if (ra == rb) JsonPrimitive(ra) else JsonArray(listOf(JsonPrimitive(minOf(ra, rb)), JsonPrimitive(maxOf(ra, rb))))
    }

    fun pasos(doc: JsonObject): List<JsonObject> = doc.lista("pasos").orEmpty().mapNotNull { it as? JsonObject }
    fun conPasos(doc: JsonObject, l: List<JsonObject>) = doc.con("pasos", JsonArray(l))
    fun total(doc: JsonObject) = pasos(doc).sumOf { val r = rango(it["dura"]); (r.first + r.second) / 2 }
    fun primeraCara(doc: JsonObject?) = doc?.let { pasos(it).firstNotNullOfOrNull { p -> p.texto("cara") } } ?: "neutral"

    /** Suma un paso después del elegido (copia su cara); hasta 16. */
    fun sumarPaso(doc: JsonObject, i: Int): Pair<JsonObject, Int> {
        val l = pasos(doc).toMutableList()
        if (l.size >= 16) return doc to i
        val cara = l.getOrNull(i)?.texto("cara") ?: "contento"
        val j = minOf(l.size, i + 1)
        l.add(j, buildJsonObject { put("cara", cara); put("dura", 1) })
        return conPasos(doc, l) to j
    }

    fun quitarPaso(doc: JsonObject, i: Int): Pair<JsonObject, Int> {
        val l = pasos(doc).toMutableList()
        if (l.size <= 1 || i !in l.indices) return doc to i
        l.removeAt(i)
        return conPasos(doc, l) to maxOf(0, i - 1)
    }

    fun moverPaso(doc: JsonObject, i: Int, d: Int): Pair<JsonObject, Int> {
        val l = pasos(doc).toMutableList()
        val j = i + d
        if (i !in l.indices || j !in l.indices) return doc to i
        val x = l[i]; l[i] = l[j]; l[j] = x
        return conPasos(doc, l) to j
    }

    fun ponerEnPaso(doc: JsonObject, i: Int, k: String, v: JsonElement?): JsonObject {
        val l = pasos(doc).toMutableList()
        if (i !in l.indices) return doc
        l[i] = l[i].con(k, v)
        return conPasos(doc, l)
    }

    fun afinidad(doc: JsonObject, r: String) = doc.obj("afinidad")?.numero(r) ?: 0.0
    fun ponerAfinidad(doc: JsonObject, r: String, v: Double): JsonObject = doc.en("afinidad") { a ->
        val n = if (Math.abs(v) < 0.001) a.sin(r) else a.con(r, Math.round(v * 10) / 10.0)
        n.oNada()
    }

    // --- Reglas: "reflejo:contento" → (reflejo, contento); "estado.animo+0.2" → (estado, animo+0.2) ---
    fun partir(a: String): Pair<String, String> {
        if (a.startsWith("estado.")) return "estado" to a.removePrefix("estado.")
        val i = a.indexOf(':')
        return if (i < 0) a to "" else a.take(i) to a.substring(i + 1)
    }

    fun unir(tipo: String, arg: String) = if (tipo == "estado") "estado.$arg" else "$tipo:$arg"
}

private fun JsonObject.oNada(): JsonObject? = if (isEmpty()) null else this

/** Una regla: cuando pasa `si` (y se cumple `y`), hace `entonces`. */
data class Regla(val si: String, val y: String? = null, val entonces: List<String>) {
    fun aJson(): JsonObject = buildJsonObject {
        put("si", si)
        if (!y.isNullOrEmpty()) put("y", y)
        put("entonces", textosJson(entonces))
    }

    companion object {
        fun de(e: JsonElement): Regla? {
            val o = e as? JsonObject ?: return null
            return Regla(o.texto("si") ?: return null, o.texto("y"), o.textos("entonces").ifEmpty { return null })
        }

        fun leer(c: JsonObject): List<Regla> = c.lista("reglas").orEmpty().mapNotNull { de(it) }
        fun escribir(c: JsonObject, l: List<Regla>) = if (l.isEmpty()) c.sin("reglas") else c.con("reglas", JsonArray(l.take(Reflejos.MAX_REGLAS).map { it.aJson() }))
    }
}

val ESTADO_VIDA = listOf("energia", "animo", "afecto", "aburrimiento", "curiosidad", "sueno", "estres")
val NOMBRES_ESTADO = mapOf(
    "energia" to "Energía", "animo" to "Ánimo", "afecto" to "Afecto", "aburrimiento" to "Aburrido", "curiosidad" to "Curiosidad", "sueno" to "Sueño", "estres" to "Estrés",
)

val NOMBRES_EVENTO = mapOf(
    "tacto" to "Lo tocan", "doble-toque" to "Doble toque", "levantado" to "Lo levantan", "sacudida" to "Lo sacuden",
    "boca-abajo" to "Boca abajo", "vinculado" to "Se vincula", "app-abierta" to "Abren la app", "despertado" to "Lo despiertan",
    "caricia" to "Una caricia", "muchos-toques" to "Muchos toques", "sacudida-suave" to "Sacudida suave",
    "sacudida-fuerte" to "Sacudida fuerte", "inclinado" to "Lo inclinan", "caida" to "Se cae", "voz" to "Le hablan", "nombre" to "Su nombre",
    "ruido" to "Un ruido", "musica" to "Música", "silencio" to "Silencio", "notificacion" to "Notificación",
    "llamada" to "Una llamada", "bateria-baja" to "Batería baja", "bateria-critica" to "Batería crítica",
    "enchufado" to "Lo enchufan", "desenchufado" to "Lo desenchufan", "cargado" to "Carga completa", "sin-wifi" to "Sin WiFi",
    "cambio-de-cara" to "Cambio de cara", "bicho-cerca" to "Otro robot cerca", "volvio" to "Volvés", "hito" to "Un hito",
)
