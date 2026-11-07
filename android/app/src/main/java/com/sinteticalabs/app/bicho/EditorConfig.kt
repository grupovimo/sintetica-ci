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
    // La personalidad (seis rasgos de 0 a 1, config v2).
    val energia: Float = 0.5f,
    val sociabilidad: Float = 0.5f,
    val picardia: Float = 0.45f,
    val timidez: Float = 0.3f,
    val curiosidad: Float = 0.55f,
    val malGenio: Float = 0.25f,
    val preset: String? = null,
    // El horario: se duerme a suenoDesde y se despierta a suenoHasta ("HH:MM").
    // null = el de fábrica (23:30 a 08:00).
    val suenoDesde: String? = null,
    val suenoHasta: String? = null,
    val lista: List<String> = emptyList(),  // "cara:<nombre>" o "imagen:<0-7>"
    val rotar: Int = 0,                      // minutos; 0 = no rota
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
        val horario = raiz["horario"] as? JsonObject
        // v2: horario {despierta, duerme}; v1: personalidad.sueno "HH:MM-HH:MM" (cuándo duerme).
        val sueno = (per?.get("sueno") as? JsonPrimitive)?.contentOrNull?.split("-")
            ?: horario?.let { listOfNotNull((it["duerme"] as? JsonPrimitive)?.contentOrNull ?: "23:30", (it["despierta"] as? JsonPrimitive)?.contentOrNull ?: "08:00") }
        fun rasgo(n: String, d: Float) = (per?.get(n) as? JsonPrimitive)?.floatOrNull ?: d
        return Ajustes(
            nombre = (raiz["nombre"] as? JsonPrimitive)?.contentOrNull ?: "bicho",
            colorOjos = (cara?.get("color") as? JsonPrimitive)?.contentOrNull,
            tamano = (cara?.get("tamano") as? JsonPrimitive)?.floatOrNull ?: 1f,
            energia = rasgo("energia", 0.5f),
            sociabilidad = rasgo("sociabilidad", 0.5f),
            picardia = rasgo("picardia", 0.45f),
            timidez = rasgo("timidez", 0.3f),
            curiosidad = rasgo("curiosidad", 0.55f),
            malGenio = rasgo("malGenio", 0.25f),
            preset = (per?.get("preset") as? JsonPrimitive)?.contentOrNull,
            suenoDesde = sueno?.getOrNull(0),
            suenoHasta = sueno?.getOrNull(1),
            lista = (raiz["lista"] as? kotlinx.serialization.json.JsonArray)?.mapNotNull { (it as? JsonPrimitive)?.contentOrNull }.orEmpty(),
            rotar = (raiz["rotar"] as? JsonPrimitive)?.contentOrNull?.toIntOrNull() ?: 0,
        )
    }

    fun escribir(texto: String, a: Ajustes): String {
        val raiz = json.parseToJsonElement(texto).jsonObject
        fun mezclar(original: JsonElement?, cambios: Map<String, JsonElement?>): JsonObject = buildJsonObject {
            (original as? JsonObject)?.forEach { (k, v) -> if (k !in cambios) put(k, v) }
            cambios.forEach { (k, v) -> if (v != null) put(k, v) }
        }
        val redondo = { f: Float -> JsonPrimitive(Math.round(f * 100) / 100.0) }
        val horario = if (a.suenoDesde != null && a.suenoHasta != null)
            buildJsonObject { put("despierta", JsonPrimitive(a.suenoHasta)); put("duerme", JsonPrimitive(a.suenoDesde)) } else null
        val nueva = mezclar(raiz, mapOf(
            "version" to JsonPrimitive(2),
            "nombre" to JsonPrimitive(a.nombre),
            "cara" to mezclar(raiz["cara"], mapOf("color" to a.colorOjos?.let { JsonPrimitive(it) }, "tamano" to redondo(a.tamano))),
            "personalidad" to mezclar(raiz["personalidad"], mapOf(
                "energia" to redondo(a.energia), "sociabilidad" to redondo(a.sociabilidad), "picardia" to redondo(a.picardia),
                "timidez" to redondo(a.timidez), "curiosidad" to redondo(a.curiosidad), "malGenio" to redondo(a.malGenio),
                "preset" to a.preset?.let { JsonPrimitive(it) }, "sueno" to null,
            )),
            "horario" to horario,
            "lista" to kotlinx.serialization.json.JsonArray(a.lista.map { JsonPrimitive(it) }),
            "rotar" to JsonPrimitive(a.rotar),
        ))
        // Sin color o sin sueño: se sacan del JSON (vuelven al valor de fábrica).
        val limpia = buildJsonObject {
            nueva.forEach { (k, v) ->
                when (k) {
                    "cara" -> put(k, buildJsonObject { v.jsonObject.forEach { (ck, cv) -> if (ck != "color" || a.colorOjos != null) put(ck, cv) } })
                    "personalidad" -> put(k, buildJsonObject { v.jsonObject.forEach { (pk, pv) -> if (pk != "sueno" && (pk != "preset" || a.preset != null)) put(pk, pv) } })
                    "horario" -> if (horario != null) put(k, v)
                    else -> put(k, v)
                }
            }
        }
        return json.encodeToString(JsonObject.serializer(), limpia)
    }

    fun hhmmValido(s: String) = Regex("^([01][0-9]|2[0-3]):[0-5][0-9]$").matches(s)

    /** Los presets de shared/formato/vida/personalidades.json (los mismos que la web y el bicho). */
    val PRESETS: Map<String, List<Float>> = mapOf(
        "equilibrado" to listOf(0.5f, 0.5f, 0.45f, 0.3f, 0.55f, 0.25f),
        "timido" to listOf(0.4f, 0.35f, 0.2f, 0.9f, 0.5f, 0.15f),
        "picaro" to listOf(0.65f, 0.6f, 0.95f, 0.15f, 0.6f, 0.3f),
        "dormilon" to listOf(0.15f, 0.45f, 0.3f, 0.35f, 0.3f, 0.3f),
        "carinoso" to listOf(0.5f, 0.95f, 0.4f, 0.3f, 0.45f, 0.05f),
        "curioso" to listOf(0.65f, 0.5f, 0.45f, 0.2f, 0.95f, 0.2f),
        "grunon" to listOf(0.45f, 0.25f, 0.3f, 0.2f, 0.35f, 0.9f),
        "electrico" to listOf(0.95f, 0.7f, 0.65f, 0.1f, 0.75f, 0.35f),
    )

    fun conPreset(a: Ajustes, nombre: String): Ajustes {
        val r = PRESETS[nombre] ?: return a
        return a.copy(energia = r[0], sociabilidad = r[1], picardia = r[2], timidez = r[3], curiosidad = r[4], malGenio = r[5], preset = nombre)
    }
}
