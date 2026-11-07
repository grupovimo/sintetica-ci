package com.sinteticalabs.app.datos

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

/** Un bicho que este celu ya vinculó. La clave es la que dio el bicho al vincular. */
data class Vinculado(val id: String, val nombre: String, val serie: String, val clave: ByteArray)

private val Context.almacen by preferencesDataStore(name = "vinculados")
private val CLAVE = stringPreferencesKey("bichos")

class Vinculados(private val contexto: Context) {
    val todos: Flow<List<Vinculado>> = contexto.almacen.data.map { p -> leer(p[CLAVE]) }

    suspend fun buscar(id: String): Vinculado? = todos.first().firstOrNull { it.id == id }

    suspend fun guardar(v: Vinculado) = contexto.almacen.edit { p ->
        val lista = leer(p[CLAVE]).filter { it.id != v.id } + v
        p[CLAVE] = escribir(lista)
    }

    suspend fun olvidar(id: String) = contexto.almacen.edit { p -> p[CLAVE] = escribir(leer(p[CLAVE]).filter { it.id != id }) }

    companion object {
        fun leer(texto: String?): List<Vinculado> = runCatching {
            Json.parseToJsonElement(texto ?: "[]").jsonArray.map { e ->
                val o = e.jsonObject
                Vinculado(
                    id = o["id"]!!.jsonPrimitive.content,
                    nombre = o["nombre"]?.jsonPrimitive?.contentOrNull ?: "bicho",
                    serie = o["serie"]?.jsonPrimitive?.contentOrNull ?: "",
                    clave = o["clave"]!!.jsonPrimitive.content.chunked(2).map { it.toInt(16).toByte() }.toByteArray(),
                )
            }
        }.getOrDefault(emptyList())

        fun escribir(lista: List<Vinculado>): String = JsonArray(lista.map { v ->
            JsonObject(mapOf(
                "id" to JsonPrimitive(v.id),
                "nombre" to JsonPrimitive(v.nombre),
                "serie" to JsonPrimitive(v.serie),
                "clave" to JsonPrimitive(v.clave.joinToString("") { "%02x".format(it) }),
            ))
        }).toString()
    }
}
