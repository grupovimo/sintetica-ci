package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * El paquete de vida en la config v2 (web/src/lib/formato.ts): cuánto habla y
 * sus frases, el despertador, dónde vive (para el clima), cómo te cuida y cómo
 * reacciona a los avisos del celu. Todo puro: config en, config afuera.
 */
object Acompanar {
    const val MAX_TEXTO = 60
    const val MAX_FRASES = 12
    const val MAX_AVISOS = 8
    const val MAX_DECIR_AVISO = 40
    const val MAX_NOMBRE_AVISO = 24
    private val DIAS = listOf("L", "M", "X", "J", "V", "S", "D")

    // --- Habla ---
    val SITUACIONES = listOf(
        "despertar" to "Al despertar", "buenas-noches" to "Buenas noches", "volviste" to "Volviste", "te-extrane" to "Te extrañó",
        "aburrido" to "Aburrido", "sueno" to "Con sueño", "bateria-baja" to "Sin pila", "cargando" to "Cargando", "caricia" to "Caricia",
        "saludo" to "Saludo", "hito" to "Un hito", "cumple-dueno" to "Tu cumple", "cumple-robot" to "Su cumple",
        "recordatorio-hecho" to "Recordatorio hecho", "despertador" to "Despertador", "posponer" to "Posponer",
        "clima-frio" to "Hace frío", "clima-calor" to "Hace calor", "clima-lluvia" to "Llueve", "clima-lindo" to "Día lindo",
    )
    val EJEMPLO = mapOf(
        "despertar" to "¡Arriba, {dueno}!", "buenas-noches" to "Hasta mañana, {dueno}", "volviste" to "¡Volviste!", "te-extrane" to "Te extrañé",
        "aburrido" to "Qué embole...", "caricia" to "Más, más", "saludo" to "Hola, soy {robot}", "despertador" to "¡Arriba!", "clima-lluvia" to "Llevá paraguas",
    )

    fun frecuencia(c: JsonObject) = c.obj("habla")?.numero("frecuencia") ?: 0.5
    fun ponerFrecuencia(c: JsonObject, v: Double) = c.en("habla") { it.con("frecuencia", redondo(v)) }
    fun tono(v: Double) = if (v <= 0.05) "Callado" else if (v >= 0.8) "Charlatán" else "Normal"

    fun frases(c: JsonObject, situacion: String): List<String> = c.obj("habla")?.obj("propias")?.textos(situacion).orEmpty()
    fun totalFrases(c: JsonObject): Int = c.obj("habla")?.obj("propias").orEmpty().values.sumOf { (it as? JsonArray)?.size ?: 0 }

    /** Cambia las frases de una situación (sin frases, la situación y "propias" se sacan). */
    fun ponerFrases(c: JsonObject, situacion: String, l: List<String>): JsonObject = c.en("habla") { h ->
        val p = (h.obj("propias") ?: VACIO).let { if (l.isEmpty()) it.sin(situacion) else it.con(situacion, textosJson(l)) }
        if (p.isEmpty()) h.sin("propias") else h.con("propias", p)
    }

    fun sumarFrase(c: JsonObject, situacion: String, t: String): JsonObject {
        val x = cortarBytes(t.trim(), MAX_TEXTO)
        if (x.isEmpty() || totalFrases(c) >= MAX_FRASES) return c
        return ponerFrases(c, situacion, frases(c, situacion) + x)
    }

    /** Lo que dice de verdad: con {dueno} y {robot} reemplazados, en una línea y en 60 bytes. */
    fun completar(t: String, dueno: String, robot: String) =
        cortarBytes(t.replace("{dueno}", dueno).replace("{robot}", robot).replace(Regex("\\s+"), " ").trim(), MAX_TEXTO)

    // --- Despertador (uno solo) ---
    data class Despertador(val hora: String = "07:30", val dias: String = "LMXJV", val activo: Boolean = true, val sonido: Boolean = true, val texto: String? = null, val posponer: Int = 9)

    /** Sin config, se ve apagado; tocar algo lo prende. */
    fun despertador(c: JsonObject): Despertador {
        val d = c.obj("despertador") ?: return Despertador(activo = false)
        return Despertador(
            d.texto("hora") ?: "07:30", d.texto("dias") ?: DIAS.joinToString(""), d.si("activo") != false, d.si("sonido") != false,
            d.texto("texto"), d.numero("posponer")?.toInt() ?: 9,
        )
    }

    fun ponerDespertador(c: JsonObject, f: (Despertador) -> Despertador): JsonObject {
        val actual = if (c.obj("despertador") == null) Despertador() else despertador(c)
        val d = f(actual)
        return c.con("despertador", buildJsonObject {
            put("hora", d.hora)
            if (d.dias.length < 7) put("dias", d.dias)
            put("activo", d.activo)
            put("sonido", d.sonido)
            if (!d.texto.isNullOrEmpty()) put("texto", d.texto)
            put("posponer", d.posponer.coerceIn(1, 30))
        })
    }

    /** Prende o apaga un día (nunca deja ninguno). */
    fun alternarDia(dias: String, letra: String): String {
        val nuevo = DIAS.filter { if (it == letra) !dias.contains(it) else dias.contains(it) }.joinToString("")
        return nuevo.ifEmpty { dias }
    }

    // --- Ubicación (para el clima) ---
    data class Ubicacion(val lat: Double, val lon: Double, val ciudad: String)

    fun ubicacion(c: JsonObject): Ubicacion? = c.obj("ubicacion")?.let { u ->
        Ubicacion(u.numero("lat") ?: return null, u.numero("lon") ?: return null, u.texto("ciudad").orEmpty())
    }

    fun ponerUbicacion(c: JsonObject, lat: Double, lon: Double, ciudad: String) = c.con("ubicacion", buildJsonObject {
        put("lat", redondo(lat, 4)); put("lon", redondo(lon, 4)); put("ciudad", cortarBytes(ciudad, 40))
    })

    // --- Cuidado ---
    data class Cuidado(
        val agua: Int = 0, val pausa: Int = 0, val desde: String = "09:00", val hasta: String = "18:00", val dias: String = "LMXJV",
        val animo: Boolean = true, val extranar: Boolean = true, val extranarDias: Int = 2, val resumen: Boolean = false,
    )

    fun cuidado(c: JsonObject): Cuidado {
        val o = c.obj("cuidado") ?: return Cuidado()
        val t = o.obj("trabajo")
        val b = Cuidado()
        return Cuidado(
            o.numero("agua")?.toInt() ?: b.agua, o.numero("pausa")?.toInt() ?: b.pausa,
            t?.texto("desde") ?: b.desde, t?.texto("hasta") ?: b.hasta, t?.texto("dias") ?: b.dias,
            o.si("animo") ?: b.animo, o.si("extranar") ?: b.extranar, o.numero("extranarDias")?.toInt() ?: b.extranarDias, o.si("resumen") ?: b.resumen,
        )
    }

    fun ponerCuidado(c: JsonObject, f: (Cuidado) -> Cuidado): JsonObject {
        val x = f(cuidado(c))
        return c.con("cuidado", buildJsonObject {
            put("agua", x.agua.coerceIn(0, 8))
            // La pausa salta de "no" a 30 minutos (menos no tiene sentido).
            put("pausa", if (x.pausa <= 0) 0 else x.pausa.coerceIn(30, 240))
            put("trabajo", buildJsonObject { put("desde", x.desde); put("hasta", x.hasta); put("dias", x.dias) })
            put("animo", x.animo)
            put("extranar", x.extranar)
            put("extranarDias", x.extranarDias.coerceIn(1, 7))
            put("resumen", x.resumen)
        })
    }

    /** Mueve el desde o el hasta del trabajo (no los deja iguales). */
    fun moverTrabajo(x: Cuidado, desde: Boolean, minutos: Int): Cuidado {
        val n = Editar.aHora(Editar.aMin(if (desde) x.desde else x.hasta) + minutos)
        if (n == (if (desde) x.hasta else x.desde)) return x
        return if (desde) x.copy(desde = n) else x.copy(hasta = n)
    }

    // --- Avisos del celu ---
    data class AvisoCelu(
        val llamada: Boolean = false, val app: String = "", val contacto: String = "",
        val expresion: String = "", val comportamiento: String = "", val decir: String = "", val sonido: Boolean = true,
    ) {
        fun aJson(): JsonObject = buildJsonObject {
            put("tipo", if (llamada) "llamada" else "notificacion")
            if (app.isNotEmpty()) put("app", app)
            if (contacto.isNotEmpty()) put("contacto", contacto)
            if (expresion.isNotEmpty()) put("expresion", expresion)
            if (comportamiento.isNotEmpty()) put("comportamiento", comportamiento)
            if (decir.isNotEmpty()) put("decir", decir)
            if (!sonido) put("sonido", false)
        }
    }

    fun avisos(c: JsonObject): List<AvisoCelu> = c.lista("avisos").orEmpty().mapNotNull { e ->
        val o = e as? JsonObject ?: return@mapNotNull null
        AvisoCelu(o.texto("tipo") == "llamada", o.texto("app").orEmpty(), o.texto("contacto").orEmpty(), o.texto("expresion").orEmpty(),
            o.texto("comportamiento").orEmpty(), o.texto("decir").orEmpty(), o.si("sonido") != false)
    }

    fun ponerAvisos(c: JsonObject, l: List<AvisoCelu>) = if (l.isEmpty()) c.sin("avisos") else c.con("avisos", JsonArray(l.take(MAX_AVISOS).map { it.aJson() }))

    fun nuevoAviso(c: JsonObject) = ponerAvisos(c, avisos(c) + AvisoCelu(expresion = "sorpresa", decir = "{app}"))

    fun nombreAviso(v: String) = cortarBytes(v.trim(), MAX_NOMBRE_AVISO)
    fun decirAviso(v: String) = cortarBytes(v.replace(Regex("\\s+"), " ").trim(), MAX_DECIR_AVISO)

    /**
     * ¿Esta notificación del celu le importa a este robot? Coincide con un
     * aviso si es del mismo tipo y la app y el contacto coinciden (vacío =
     * cualquiera; sin distinguir mayúsculas, la app puede ser parte del nombre).
     */
    fun coincide(l: List<AvisoCelu>, llamada: Boolean, app: String, contacto: String): Boolean = l.any { a ->
        a.llamada == llamada &&
            (a.app.isEmpty() || app.contains(a.app, ignoreCase = true)) &&
            (a.contacto.isEmpty() || contacto.contains(a.contacto, ignoreCase = true))
    }

    @Suppress("unused")
    private val SIN = JsonPrimitive("")
}
