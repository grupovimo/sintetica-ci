package com.sinteticalabs.app.consola

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.jsonObject

/**
 * Atajos para leer y cambiar la config (un JsonObject inmutable): cada cambio
 * devuelve una copia, así lo que la consola no toca queda igual.
 */
val VACIO = JsonObject(emptyMap())

fun JsonObject.obj(k: String): JsonObject? = this[k] as? JsonObject
fun JsonObject.texto(k: String): String? = (this[k] as? JsonPrimitive)?.contentOrNull
fun JsonObject.numero(k: String): Double? = (this[k] as? JsonPrimitive)?.doubleOrNull
fun JsonObject.si(k: String): Boolean? = (this[k] as? JsonPrimitive)?.booleanOrNull
fun JsonObject.lista(k: String): JsonArray? = this[k] as? JsonArray
fun JsonObject.textos(k: String): List<String> = lista(k)?.mapNotNull { (it as? JsonPrimitive)?.contentOrNull }.orEmpty()

/** Con la clave puesta (o sacada, si es null). */
fun JsonObject.con(k: String, v: JsonElement?): JsonObject =
    JsonObject(LinkedHashMap(this).apply { if (v == null) remove(k) else put(k, v) })

fun JsonObject.sin(k: String): JsonObject = con(k, null as JsonElement?)
fun JsonObject.con(k: String, v: String?) = con(k, v?.let { JsonPrimitive(it) })
fun JsonObject.con(k: String, v: Number?) = con(k, v?.let { JsonPrimitive(it) })
fun JsonObject.con(k: String, v: Boolean?) = con(k, v?.let { JsonPrimitive(it) })

/** Cambia un objeto de adentro (si no existe, arranca vacío); null lo saca. */
fun JsonObject.en(k: String, f: (JsonObject) -> JsonObject?): JsonObject = con(k, f(obj(k) ?: VACIO))

fun textosJson(l: List<String>) = JsonArray(l.map { JsonPrimitive(it) })

/** Redondeado a dos decimales, como lo escriben las perillas. */
fun redondo(v: Double, decimales: Int = 2): Double {
    var k = 1.0
    repeat(decimales) { k *= 10 }
    return Math.round(v * k) / k
}

private val lector = Json { ignoreUnknownKeys = true }

fun leerObjeto(texto: String): JsonObject = runCatching { lector.parseToJsonElement(texto).jsonObject }.getOrDefault(VACIO)
