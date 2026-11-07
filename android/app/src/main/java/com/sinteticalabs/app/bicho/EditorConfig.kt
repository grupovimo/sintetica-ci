package com.sinteticalabs.app.bicho

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.floatOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

/** Lo que la app deja editar de la configuración (shared/formato/config.schema.json). */
data class Ajustes(
    val nombre: String = "bicho",
    val colorOjos: String? = null,   // "#RRGGBB"; null = el naranja de la marca
    val tamano: Float = 1f,          // 0,5 a 1,5
    val energia: Float = 0.5f,       // 0 a 1
    val timidez: Float = 0.3f,       // 0 a 1
    val suenoDesde: String? = null,  // "HH:MM"
    val suenoHasta: String? = null,
)

/**
 * Lee y escribe los ajustes sobre el JSON completo de la config, sin tocar lo
 * que la app no edita (expresiones propias, reglas, modo).
 */
object EditorConfig {
    private val json = Json { prettyPrint = false }

    fun leer(texto: String): Ajustes {
        val raiz = json.parseToJsonElement(texto).jsonObject
        val cara = raiz["cara"] as? JsonObject
        val per = raiz["personalidad"] as? JsonObject
        val sueno = (per?.get("sueno") as? JsonPrimitive)?.contentOrNull?.split("-")
        return Ajustes(
            nombre = (raiz["nombre"] as? JsonPrimitive)?.contentOrNull ?: "bicho",
            colorOjos = (cara?.get("color") as? JsonPrimitive)?.contentOrNull,
            tamano = (cara?.get("tamano") as? JsonPrimitive)?.floatOrNull ?: 1f,
            energia = (per?.get("energia") as? JsonPrimitive)?.floatOrNull ?: 0.5f,
            timidez = (per?.get("timidez") as? JsonPrimitive)?.floatOrNull ?: 0.3f,
            suenoDesde = sueno?.getOrNull(0),
            suenoHasta = sueno?.getOrNull(1),
        )
    }

    fun escribir(texto: String, a: Ajustes): String {
        val raiz = json.parseToJsonElement(texto).jsonObject
        fun mezclar(original: JsonElement?, cambios: Map<String, JsonElement?>): JsonObject = buildJsonObject {
            (original as? JsonObject)?.forEach { (k, v) -> if (k !in cambios) put(k, v) }
            cambios.forEach { (k, v) -> if (v != null) put(k, v) }
        }
        val redondo = { f: Float -> JsonPrimitive(Math.round(f * 100) / 100.0) }
        val sueno = if (a.suenoDesde != null && a.suenoHasta != null) JsonPrimitive("${a.suenoDesde}-${a.suenoHasta}") else null
        val nueva = mezclar(raiz, mapOf(
            "version" to JsonPrimitive(1),
            "nombre" to JsonPrimitive(a.nombre),
            "cara" to mezclar(raiz["cara"], mapOf("color" to a.colorOjos?.let { JsonPrimitive(it) }, "tamano" to redondo(a.tamano))),
            "personalidad" to mezclar(raiz["personalidad"], mapOf("energia" to redondo(a.energia), "timidez" to redondo(a.timidez), "sueno" to sueno)),
        ))
        // Sin color o sin sueño: se sacan del JSON (vuelven al valor de fábrica).
        val limpia = buildJsonObject {
            nueva.forEach { (k, v) ->
                when (k) {
                    "cara" -> put(k, buildJsonObject { v.jsonObject.forEach { (ck, cv) -> if (ck != "color" || a.colorOjos != null) put(ck, cv) } })
                    "personalidad" -> put(k, buildJsonObject { v.jsonObject.forEach { (pk, pv) -> if (pk != "sueno" || sueno != null) put(pk, pv) } })
                    else -> put(k, v)
                }
            }
        }
        return json.encodeToString(JsonObject.serializer(), limpia)
    }

    fun hhmmValido(s: String) = Regex("^([01][0-9]|2[0-3]):[0-5][0-9]$").matches(s)
}
