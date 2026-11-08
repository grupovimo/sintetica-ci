package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** La voz y la IA de la nube (web/src/lib/consola/voz.ts): lo que no es pantalla. */
object Voz {
    // Los topes van en escalones (una perilla de 0 a 200 000 de a uno no se puede girar).
    val LETRAS = listOf(0, 1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000)
    val MINUTOS = listOf(0, 5, 10, 15, 20, 30, 45, 60, 90, 120, 240)
    val TOPES_IA = listOf(0, 10, 20, 30, 50, 100, 200, 500, 1000, 2000, 5000, 10000)

    /** El escalón más cercano a un valor. */
    fun escalon(l: List<Int>, v: Int): Int = l.indices.minByOrNull { Math.abs(l[it] - v) } ?: 0

    /** 1500 → "1,5 k"; 200000 → "200 k". */
    fun miles(n: Int): String = if (n >= 1000) {
        val k = n / 1000.0
        (if (k == Math.floor(k)) "${k.toInt()}" else "%.1f".format(java.util.Locale.forLanguageTag("es-AR"), k)) + " k"
    } else "$n"

    fun megas(b: Long): String = "%.1f".format(java.util.Locale.forLanguageTag("es-AR"), b / 1048576.0).removeSuffix(",0")

    /** La primera pestaña: ElevenLabs si lee con ella (o no hay gratis); si no, la gratis. */
    fun pestanaInicial(edge: Boolean, elevenConectado: Boolean, elevenLee: Boolean) =
        if (elevenConectado && (elevenLee || !edge)) "eleven" else if (edge) "edge" else "eleven"

    val ETIQUETA_PAQUETE = mapOf("listo" to "Listo", "parcial" to "A medias", "generando" to "Grabando", "nuevo" to "Sin paquete", "error" to "Falló")

    /** Si se ofrece generarlo (o actualizarlo). */
    fun hayQueGenerar(estado: String, hay: Boolean, faltan: Int, voz: String, vozNueva: String?) =
        estado != "generando" && (!hay || faltan > 0 || (!vozNueva.isNullOrEmpty() && vozNueva != voz) || estado == "error" || estado == "parcial")

    // --- Tu IA ---
    data class Servicio(val clave: String, val nombre: String, val proveedor: String, val url: String, val modelo: String, val claves: String)

    val SERVICIOS = listOf(
        Servicio("anthropic", "Anthropic", "anthropic", "", "claude-haiku-5-5", "https://console.anthropic.com/settings/keys"),
        Servicio("groq", "Groq", "compatible", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile", "https://console.groq.com/keys"),
        Servicio("openrouter", "OpenRouter", "compatible", "https://openrouter.ai/api/v1", "meta-llama/llama-3.3-70b-instruct", "https://openrouter.ai/keys"),
        Servicio("gemini", "Gemini", "compatible", "https://generativelanguage.googleapis.com/v1beta/openai", "gemini-2.5-flash", "https://aistudio.google.com/apikey"),
        Servicio("otro", "Otro", "compatible", "", "", ""),
    )

    /** Cuál usa: "tuya", "servidor" o "ninguna" (charla su motor propio). */
    fun cualIA(propia: Boolean, servidor: Boolean) = if (propia) "tuya" else if (servidor) "servidor" else "ninguna"

    fun servicioDe(proveedor: String, urlPista: String): String =
        if (proveedor == "anthropic") "Anthropic" else SERVICIOS.firstOrNull { it.url.isNotEmpty() && urlPista.isNotEmpty() && it.url.contains(urlPista) }?.nombre ?: urlPista.ifEmpty { "Compatible" }

    fun cuerpoIA(proveedor: String, clave: String, modelo: String, url: String): JsonObject = buildJsonObject {
        put("proveedor", proveedor)
        put("clave", clave.trim())
        if (modelo.isNotBlank()) put("modelo", modelo.trim())
        if (proveedor == "compatible") put("url", url.trim())
    }

    fun puedeConectar(proveedor: String, clave: String, modelo: String, url: String) =
        clave.isNotBlank() && (proveedor != "compatible" || (url.isNotBlank() && modelo.isNotBlank()))

    // --- La conversación en tiempo real ---

    /** mirar {x, y} del agente (−100 a 100) → la mirada de −1 a 1. */
    fun mirada(x: Any?, y: Any?): Pair<Double, Double> {
        fun n(v: Any?) = ((v as? Number)?.toDouble() ?: (v as? String)?.toDoubleOrNull() ?: 0.0).div(100).coerceIn(-1.0, 1.0)
        return n(x) to n(y)
    }

    /** poner_expresion {nombre}: la cara, si es una de las suyas (sin lista, cualquiera). null = no la tiene. */
    fun expresion(nombre: Any?, caras: List<String>): String? {
        val n = (nombre as? String)?.trim().orEmpty()
        if (n.isEmpty()) return null
        return if (caras.isEmpty() || n in caras) n else null
    }

    fun mmss(s: Int) = "${maxOf(0, s) / 60}:${"%02d".format(maxOf(0, s) % 60)}"
}
