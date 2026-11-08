package com.sinteticalabs.app.datos

import android.util.Base64
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.isActive
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL

class ErrorApi(val estado: Int, mensaje: String, val detalle: String = "") : Exception(mensaje)

/** Las credenciales MQTT que la API le da a un bicho (la app se las pasa por BLE). */
data class Credenciales(val url: String, val usuario: String, val clave: String)

/** Un bicho de la cuenta, como lo devuelve la API. */
data class BichoNube(val id: String, val nombre: String, val perfil: String, val conectado: Boolean, val virtual: Boolean, val estadoCrudo: ByteArray?)

/** Un firmware firmado para un perfil. */
data class FirmwareNube(val id: Long, val version: String, val tamano: Int, val notas: String)

/** Un evento en vivo (SSE): tipo y datos. */
data class EventoNube(val tipo: String, val datos: JsonObject)

/** La API de SintéticaOS (docs/nube.md). Todo en Dispatchers.IO. */
class ApiSintetica(private val servidor: String, private val token: String = "") {
    private val json = Json { ignoreUnknownKeys = true }

    private fun abrir(ruta: String, metodo: String): HttpURLConnection {
        val c = URL(servidor.trimEnd('/') + ruta).openConnection() as HttpURLConnection
        c.requestMethod = metodo
        c.connectTimeout = 8_000
        c.readTimeout = 40_000
        c.setRequestProperty("Accept", "application/json")
        if (token.isNotEmpty()) c.setRequestProperty("Authorization", "Bearer $token")
        return c
    }

    suspend fun pedir(ruta: String, metodo: String = "GET", cuerpo: JsonElement? = null, encabezados: Map<String, String> = emptyMap()): JsonObject =
        withContext(Dispatchers.IO) {
            val c = abrir(ruta, metodo)
            try {
                encabezados.forEach { (k, v) -> c.setRequestProperty(k, v) }
                if (cuerpo != null) {
                    c.doOutput = true
                    c.setRequestProperty("Content-Type", "application/json")
                    c.outputStream.use { it.write(cuerpo.toString().toByteArray()) }
                }
                val estado = try { c.responseCode } catch (e: Exception) { throw ErrorApi(0, "Sin conexión con el servidor.") }
                val texto = (if (estado < 400) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() }.orEmpty()
                val obj = runCatching { json.parseToJsonElement(texto).jsonObject }.getOrNull() ?: JsonObject(emptyMap())
                if (estado >= 400) {
                    throw ErrorApi(estado, obj["error"]?.jsonPrimitive?.contentOrNull ?: "Error $estado", obj["detalle"]?.jsonPrimitive?.contentOrNull.orEmpty())
                }
                obj
            } finally {
                c.disconnect()
            }
        }

    suspend fun pedirEnlace(email: String) {
        pedir("/v1/sesion/enlace", "POST", buildJsonObject { put("email", email); put("cliente", "app") })
    }

    /** Con el código del mail (o el token del link). Devuelve el token de la sesión. */
    suspend fun canjear(email: String? = null, codigo: String? = null, tokenEnlace: String? = null): String {
        val r = pedir("/v1/sesion/canjear", "POST", buildJsonObject {
            if (tokenEnlace != null) put("token", tokenEnlace) else { put("email", email.orEmpty()); put("codigo", codigo.orEmpty()) }
            put("descripcion", "App de Android")
        })
        return r["token"]?.jsonPrimitive?.contentOrNull ?: throw ErrorApi(0, "La API no dio el token.")
    }

    suspend fun salir() { runCatching { pedir("/v1/sesion", "DELETE") } }

    suspend fun email(): String = pedir("/v1/sesion")["usuario"]?.jsonObject?.get("email")?.jsonPrimitive?.contentOrNull.orEmpty()

    suspend fun bichos(): List<BichoNube> = pedir("/v1/bichos")["bichos"]?.jsonArray.orEmpty().map { it.jsonObject.aBicho() }

    suspend fun bicho(id: String): BichoNube = pedir("/v1/bichos/$id").aBicho()

    private fun JsonObject.aBicho(): BichoNube {
        val est = this["estado"] as? JsonObject
        val crudo = (est?.get("crudo") as? JsonPrimitive)?.contentOrNull?.let { Base64.decode(it, Base64.DEFAULT) }
        val apodo = this["apodo"]?.jsonPrimitive?.contentOrNull.orEmpty()
        return BichoNube(
            id = this["id"]!!.jsonPrimitive.content,
            nombre = apodo.ifEmpty { est?.get("nombre")?.jsonPrimitive?.contentOrNull ?: "Robot" },
            perfil = this["perfil"]?.jsonPrimitive?.contentOrNull.orEmpty(),
            conectado = this["conectado"]?.jsonPrimitive?.booleanOrNull ?: false,
            virtual = this["virtual"]?.jsonPrimitive?.booleanOrNull ?: false,
            estadoCrudo = crudo,
        )
    }

    /** Pide credenciales para sumar el bicho (que está cerca, vinculado por BLE). */
    suspend fun reclamar(id: String, perfil: String): Credenciales {
        val m = pedir("/v1/bichos/reclamar", "POST", buildJsonObject { put("id", id); put("perfil", perfil) })["mqtt"]!!.jsonObject
        return Credenciales(m["url"]!!.jsonPrimitive.content, m["usuario"]!!.jsonPrimitive.content, m["clave"]!!.jsonPrimitive.content)
    }

    /** Espera (hasta `segundos`) a que el bicho se conecte con la clave nueva. */
    suspend fun esperarReclamo(id: String, segundos: Int = 40): Boolean =
        pedir("/v1/bichos/$id/reclamo?esperar=$segundos")["sumado"]?.jsonPrimitive?.booleanOrNull ?: false

    /** Un marco del protocolo, por la nube. Devuelve las respuestas que da la API misma. */
    suspend fun marco(id: String, car: Int, datos: ByteArray, conexion: String): List<Pair<Int, ByteArray>> {
        val r = pedir("/v1/bichos/$id/marcos", "POST", buildJsonObject {
            put("car", car)
            put("datos", Base64.encodeToString(datos, Base64.NO_WRAP))
        }, mapOf("X-Conexion" to conexion))
        return (r["respuestas"] as? JsonArray).orEmpty().map {
            val o = it.jsonObject
            o["car"]!!.jsonPrimitive.intOrNull!! to Base64.decode(o["datos"]!!.jsonPrimitive.content, Base64.DEFAULT)
        }
    }

    suspend fun firmwares(perfil: String): List<FirmwareNube> =
        (pedir("/v1/firmwares?perfil=$perfil")["firmwares"] as? JsonArray).orEmpty().map {
            val o = it.jsonObject
            FirmwareNube(o["id"]!!.jsonPrimitive.content.toLong(), o["version"]!!.jsonPrimitive.content,
                o["tamano"]!!.jsonPrimitive.intOrNull ?: 0, o["notas"]?.jsonPrimitive?.contentOrNull.orEmpty())
        }

    /** Los bytes del manifiesto firmado (para mandarlo por BLE). */
    suspend fun manifiesto(id: Long): ByteArray =
        Base64.decode(pedir("/v1/firmwares/$id/manifiesto")["manifiesto"]!!.jsonPrimitive.content, Base64.DEFAULT)

    suspend fun binario(id: Long): ByteArray = withContext(Dispatchers.IO) {
        val c = abrir("/v1/firmwares/$id/binario", "GET")
        try {
            if (c.responseCode != 200) throw ErrorApi(c.responseCode, "No pude bajar el firmware.")
            c.inputStream.use { it.readBytes() }
        } finally {
            c.disconnect()
        }
    }

    /** Un pedido con el cuerpo tal cual (JSON crudo o binario). Devuelve los bytes de la respuesta. */
    suspend fun crudo(ruta: String, metodo: String = "GET", cuerpo: ByteArray? = null, tipo: String = "application/json"): ByteArray =
        withContext(Dispatchers.IO) {
            val c = abrir(ruta, metodo)
            try {
                if (cuerpo != null) {
                    c.doOutput = true
                    c.setRequestProperty("Content-Type", tipo)
                    c.outputStream.use { it.write(cuerpo) }
                }
                val estado = try { c.responseCode } catch (e: Exception) { throw ErrorApi(0, "Sin conexión con el servidor.") }
                val bytes = (if (estado < 400) c.inputStream else c.errorStream)?.use { it.readBytes() } ?: ByteArray(0)
                if (estado >= 400) {
                    val obj = runCatching { json.parseToJsonElement(String(bytes)).jsonObject }.getOrNull() ?: JsonObject(emptyMap())
                    throw ErrorApi(estado, obj["error"]?.jsonPrimitive?.contentOrNull ?: "Error $estado", obj["detalle"]?.jsonPrimitive?.contentOrNull.orEmpty())
                }
                bytes
            } finally {
                c.disconnect()
            }
        }

    /**
     * Los eventos en vivo de toda la cuenta (SSE /v1/eventos): estado, conexión,
     * sincro, config, imágenes, OTA, marcos, automatizaciones, Telegram y MCP.
     * La API manda un latido cada 20 s: si en 45 s no llega nada, se corta.
     */
    fun eventosCuenta(): Flow<com.sinteticalabs.app.consola.EventoCuenta> = kotlinx.coroutines.flow.flow {
        val c = abrir("/v1/eventos", "GET")
        c.readTimeout = 45_000
        c.setRequestProperty("Accept", "text/event-stream")
        try {
            val estado = try { c.responseCode } catch (e: Exception) { throw ErrorApi(0, "Sin conexión con el servidor.") }
            if (estado >= 400) throw ErrorApi(estado, "Error $estado")
            val lector = com.sinteticalabs.app.consola.LectorSse()
            BufferedReader(InputStreamReader(c.inputStream)).use { r ->
                while (true) {
                    val linea = r.readLine() ?: break
                    lector.leer(linea)?.let { emit(it) }
                }
            }
        } finally {
            c.disconnect()
        }
    }.flowOn(Dispatchers.IO)

    /** Los eventos en vivo de un bicho (SSE). Se corta al cancelar. */
    fun eventos(id: String): Flow<EventoNube> = callbackFlow {
        val c = abrir("/v1/bichos/$id/eventos", "GET")
        c.readTimeout = 0
        c.setRequestProperty("Accept", "text/event-stream")
        val hilo = Thread {
            try {
                BufferedReader(InputStreamReader(c.inputStream)).use { r ->
                    var tipo = ""
                    while (isActive) {
                        val linea = r.readLine() ?: break
                        when {
                            linea.startsWith("event: ") -> tipo = linea.removePrefix("event: ")
                            linea.startsWith("data: ") -> runCatching {
                                val o = json.parseToJsonElement(linea.removePrefix("data: ")).jsonObject
                                trySend(EventoNube(tipo, (o["datos"] as? JsonObject) ?: JsonObject(emptyMap())))
                            }
                        }
                    }
                }
            } catch (_: Exception) {
            } finally {
                close()
            }
        }
        hilo.start()
        awaitClose { c.disconnect(); hilo.interrupt() }
    }.flowOn(Dispatchers.IO)
}
