package com.sinteticalabs.app.datos

import com.sinteticalabs.app.consola.Automatizacion
import com.sinteticalabs.app.consola.Automatizaciones
import com.sinteticalabs.app.consola.Integraciones
import com.sinteticalabs.app.consola.VACIO
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.si
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.protocolo.EspacioImagen
import com.sinteticalabs.app.protocolo.Estado
import com.sinteticalabs.app.protocolo.MODOS
import com.sinteticalabs.app.protocolo.Pantalla
import com.sinteticalabs.app.protocolo.leerEstado
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import java.time.OffsetDateTime

/** Un robot de la cuenta, como lo da la API (GET /v1/bichos). */
data class RobotNube(
    val id: String,
    val perfil: String,
    val apodo: String,
    val virtual: Boolean,
    val conectado: Boolean,
    val ultimaConexion: Long?,
    val estado: Estado?,
    val sincroConfig: String,
) {
    val nombre get() = apodo.ifEmpty { estado?.nombre.orEmpty() }

    companion object {
        fun de(o: JsonObject) = RobotNube(
            id = o.texto("id").orEmpty().uppercase(),
            perfil = o.texto("perfil").orEmpty(),
            apodo = o.texto("apodo").orEmpty(),
            virtual = o.si("virtual") ?: false,
            conectado = o.si("conectado") ?: false,
            ultimaConexion = o.texto("ultimaConexion")?.let { fechaMs(it) },
            estado = o.obj("estado")?.let { estadoDeJson(it) },
            sincroConfig = o.obj("sincro")?.texto("config").orEmpty(),
        )
    }
}

fun fechaMs(iso: String): Long? = runCatching { OffsetDateTime.parse(iso).toInstant().toEpochMilli() }.getOrNull()

/** El estado como lo manda la API: los bytes crudos (como por BLE) o, si no, el JSON ya decodificado. */
fun estadoDeJson(o: JsonObject): Estado? {
    o.texto("crudo")?.let { s -> runCatching { return leerEstado(java.util.Base64.getDecoder().decode(s)) } }
    if (o["expresion"] == null && o["nombre"] == null) return null
    val p = o.obj("pantalla") ?: VACIO
    val ancho = p.numero("ancho")?.toInt() ?: 0
    return Estado(
        pantalla = Pantalla(if (ancho > 0) ancho else 320, p.numero("alto")?.toInt() ?: 172, p.si("redonda") ?: false),
        bateria = o.numero("bateria")?.toInt(),
        humor = o.numero("humor")?.toFloat() ?: 0f,
        modo = o.texto("modo") ?: MODOS[0],
        dormido = o.si("dormido") ?: false,
        fija = o.si("fija") ?: false,
        emparejando = o.si("emparejando") ?: false,
        autenticado = true,
        expresion = o.texto("expresion").orEmpty(),
        nombre = o.texto("nombre").orEmpty(),
        serie = o.texto("serie").orEmpty(),
        id = o.texto("id").orEmpty(),
        wifi = o.si("wifi") ?: false,
        nube = o.si("nube") ?: false,
        redConfigurada = o.si("redConfigurada") ?: false,
        actualizando = o.si("actualizando") ?: false,
        version = o.texto("version").orEmpty(),
        perfil = o.texto("perfil").orEmpty(),
        crcConfig = o.numero("crcConfig")?.toLong() ?: 0,
        encendido = o.numero("encendido")?.toLong() ?: 0,
    )
}

data class VersionConfig(val version: Int, val autor: String, val creada: Long?)
data class Firmware(val id: Long, val version: String, val notas: String, val creado: Long?)
data class Actualizacion(val id: Long, val version: String, val estado: String, val progreso: Int, val creada: Long?)
data class EstadoTelegram(val activo: Boolean = false, val bot: String = "", val vinculado: Boolean = false, val nombre: String = "")
data class ClienteMcp(val id: String, val cliente: String, val usada: Long?)
data class TokenMcp(val id: String, val nombre: String, val usado: Long?)

// --- Lo que usa la consola (como web/src/lib/bicho.svelte.ts y los módulos) ---

suspend fun ApiSintetica.robots(): List<RobotNube> = pedir("/v1/bichos").lista("bichos").orEmpty().mapNotNull { (it as? JsonObject)?.let(RobotNube::de) }
suspend fun ApiSintetica.robot(id: String): RobotNube = RobotNube.de(pedir("/v1/bichos/$id"))

/** La config y su versión. */
suspend fun ApiSintetica.config(id: String): Pair<Int, JsonObject> {
    val o = pedir("/v1/bichos/$id/config")
    return (o.numero("version")?.toInt() ?: 0) to (o.obj("config") ?: buildJsonObject { put("version", 1) })
}

/** La guarda (la API la valida con el mismo núcleo). Devuelve la versión nueva. */
suspend fun ApiSintetica.guardarConfig(id: String, c: JsonObject): Int =
    leerObjeto(String(crudo("/v1/bichos/$id/config", "PUT", c.toString().toByteArray()))).numero("version")?.toInt() ?: 0

suspend fun ApiSintetica.comando(id: String, c: Map<String, String>) {
    pedir("/v1/bichos/$id/comando", "POST", buildJsonObject { c.forEach { (k, v) -> put(k, v) } })
}

suspend fun ApiSintetica.imagenes(id: String): List<EspacioImagen> = pedir("/v1/bichos/$id/imagenes").lista("imagenes").orEmpty().mapNotNull {
    val o = it as? JsonObject ?: return@mapNotNull null
    EspacioImagen(o.numero("espacio")?.toInt() ?: return@mapNotNull null, o.numero("largo")?.toLong() ?: 0, o.numero("crc")?.toLong() ?: 0)
}

suspend fun ApiSintetica.imagen(id: String, espacio: Int): ByteArray = crudo("/v1/bichos/$id/imagenes/$espacio")
suspend fun ApiSintetica.subirImagen(id: String, espacio: Int, qoi: ByteArray) { crudo("/v1/bichos/$id/imagenes/$espacio", "PUT", qoi, "application/octet-stream") }
suspend fun ApiSintetica.borrarImagen(id: String, espacio: Int) { crudo("/v1/bichos/$id/imagenes/$espacio", "DELETE") }

suspend fun ApiSintetica.ponerApodo(id: String, apodo: String): String =
    pedir("/v1/bichos/$id", "PATCH", buildJsonObject { put("apodo", apodo) }).texto("apodo") ?: apodo

suspend fun ApiSintetica.quitar(id: String) { crudo("/v1/bichos/$id", "DELETE") }

suspend fun ApiSintetica.historialConfig(id: String): List<VersionConfig> = pedir("/v1/bichos/$id/config/historial").lista("versiones").orEmpty().mapNotNull {
    val o = it as? JsonObject ?: return@mapNotNull null
    VersionConfig(o.numero("version")?.toInt() ?: 0, o.texto("autor").orEmpty(), o.texto("creada")?.let(::fechaMs))
}

suspend fun ApiSintetica.restaurarConfig(id: String, version: Int) { pedir("/v1/bichos/$id/config/restaurar", "POST", buildJsonObject { put("version", version) }) }

suspend fun ApiSintetica.firmwaresDe(perfil: String): List<Firmware> =
    pedir("/v1/firmwares?perfil=" + java.net.URLEncoder.encode(perfil, "UTF-8")).lista("firmwares").orEmpty().mapNotNull {
        val o = it as? JsonObject ?: return@mapNotNull null
        Firmware(o.numero("id")?.toLong() ?: 0, o.texto("version").orEmpty(), o.texto("notas").orEmpty(), o.texto("creado")?.let(::fechaMs))
    }

private fun actualizacionDe(o: JsonObject) = Actualizacion(
    o.numero("id")?.toLong() ?: 0, o.texto("version").orEmpty(), o.texto("estado").orEmpty(), o.numero("progreso")?.toInt() ?: 0, o.texto("creada")?.let(::fechaMs),
)

suspend fun ApiSintetica.actualizaciones(id: String): List<Actualizacion> =
    pedir("/v1/bichos/$id/actualizaciones").lista("actualizaciones").orEmpty().mapNotNull { (it as? JsonObject)?.let(::actualizacionDe) }

suspend fun ApiSintetica.actualizar(id: String, firmware: Long): Actualizacion =
    actualizacionDe(pedir("/v1/bichos/$id/actualizar", "POST", buildJsonObject { put("firmware", firmware) }))

fun actualizacionDeEvento(o: JsonObject) = actualizacionDe(o)

// --- Integraciones ---

suspend fun ApiSintetica.telegram(): EstadoTelegram {
    val o = pedir("/v1/telegram")
    return EstadoTelegram(o.si("activo") ?: false, o.texto("bot").orEmpty(), o.si("vinculado") ?: false, o.texto("nombre").orEmpty())
}

/** Un link de un solo uso al bot (t.me/…?start=…). */
suspend fun ApiSintetica.vincularTelegram(): String? = pedir("/v1/telegram/vincular", "POST").texto("enlace")
suspend fun ApiSintetica.desvincularTelegram() { pedir("/v1/telegram", "DELETE") }

suspend fun ApiSintetica.automatizaciones(id: String): Integraciones = Automatizaciones.leer(pedir("/v1/bichos/$id/automatizaciones"))

suspend fun ApiSintetica.guardarAutomatizaciones(id: String, l: List<Automatizacion>) {
    crudo("/v1/bichos/$id/automatizaciones", "PUT", Automatizaciones.cuerpo(l).toString().toByteArray())
}

suspend fun ApiSintetica.probarAutomatizacion(id: String, n: Int) { pedir("/v1/bichos/$id/automatizaciones/$n/probar", "POST") }

/** Las IAs conectadas por MCP (OAuth) y la URL del conector. */
suspend fun ApiSintetica.clientesMcp(): Pair<List<ClienteMcp>, String> {
    val o = pedir("/v1/mcp/clientes")
    val l = o.lista("clientes").orEmpty().mapNotNull {
        val c = it as? JsonObject ?: return@mapNotNull null
        ClienteMcp(c.texto("id").orEmpty(), c.texto("cliente") ?: "IA", c.texto("usada")?.let(::fechaMs))
    }
    return l to o.texto("url").orEmpty()
}

suspend fun ApiSintetica.desconectarMcp(id: String) { crudo("/v1/mcp/clientes/$id", "DELETE") }

suspend fun ApiSintetica.tokensMcp(): List<TokenMcp> = pedir("/v1/mcp/tokens").lista("tokens").orEmpty().mapNotNull {
    val t = it as? JsonObject ?: return@mapNotNull null
    TokenMcp(t.texto("id").orEmpty(), t.texto("nombre").orEmpty(), t.texto("usado")?.let(::fechaMs))
}

suspend fun ApiSintetica.crearTokenMcp(nombre: String): String = pedir("/v1/mcp/tokens", "POST", buildJsonObject { put("nombre", nombre) }).texto("token").orEmpty()
suspend fun ApiSintetica.borrarTokenMcp(id: String) { crudo("/v1/mcp/tokens/$id", "DELETE") }

// --- Laboratorio: el diario, la mudanza y el secreto de los webhooks ---

/** Lo que recuerda y si hay una mudanza en camino. */
suspend fun ApiSintetica.memoria(id: String): Pair<com.sinteticalabs.app.consola.Memoria?, Boolean> {
    val o = pedir("/v1/bichos/$id/memoria")
    return o.obj("memoria")?.let { com.sinteticalabs.app.consola.Memoria.de(it) } to (o.si("mudanzaPendiente") ?: false)
}

/** Trae la memoria de otro robot del mismo dueño (le llega al conectarse). */
suspend fun ApiSintetica.mudar(id: String, desde: String) { pedir("/v1/bichos/$id/mudar", "POST", buildJsonObject { put("desde", desde) }) }

/** El secreto con el que se firman los webhooks (rotar = uno nuevo). */
suspend fun ApiSintetica.secretoWebhooks(rotar: Boolean): String =
    pedir("/v1/webhooks/secreto", if (rotar) "POST" else "GET").texto("secreto").orEmpty()

// --- Sprites de las caras de imagen (0 a 11) ---

/** Los sprites del robot: número → CRC. */
suspend fun ApiSintetica.sprites(id: String): List<EspacioImagen> = pedir("/v1/bichos/$id/sprites").lista("sprites").orEmpty().mapNotNull {
    val o = it as? JsonObject ?: return@mapNotNull null
    EspacioImagen(o.numero("sprite")?.toInt() ?: return@mapNotNull null, o.numero("largo")?.toLong() ?: 0, o.numero("crc")?.toLong() ?: 0)
}

/** El QOI de un sprite. */
suspend fun ApiSintetica.sprite(id: String, n: Int): ByteArray = crudo("/v1/bichos/$id/sprites/$n")

/** Sube un sprite: SVG, PNG, JPEG, GIF o QOI (la API lo recorta y lo achica a 128 × 128 con su transparencia). */
suspend fun ApiSintetica.subirSprite(id: String, n: Int, datos: ByteArray, tipo: String) { crudo("/v1/bichos/$id/sprites/$n", "PUT", datos, tipo) }

suspend fun ApiSintetica.borrarSprite(id: String, n: Int) { crudo("/v1/bichos/$id/sprites/$n", "DELETE") }

// --- El clima (Open-Meteo, por la nube) y el resumen del día ---

data class Lugar(val nombre: String, val region: String, val pais: String, val lat: Double, val lon: Double, val ciudad: String)
data class Clima(val temp: Double, val codigo: Int, val descripcion: String, val max: Double, val min: Double, val dia: Boolean)

/** Busca una ciudad (2 a 60 bytes): hasta 6 lugares. */
suspend fun ApiSintetica.buscarCiudad(q: String): List<Lugar> =
    pedir("/v1/clima/buscar?q=" + java.net.URLEncoder.encode(q, "UTF-8")).lista("lugares").orEmpty().mapNotNull {
        val o = it as? JsonObject ?: return@mapNotNull null
        Lugar(o.texto("nombre").orEmpty(), o.texto("region").orEmpty(), o.texto("pais").orEmpty(),
            o.numero("lat") ?: return@mapNotNull null, o.numero("lon") ?: return@mapNotNull null, o.texto("ciudad").orEmpty())
    }

/** El clima de donde vive (null sin ubicación o si no se pudo traer). */
suspend fun ApiSintetica.clima(id: String): Clima? = pedir("/v1/bichos/$id/clima").obj("clima")?.let { o ->
    Clima(o.numero("temp") ?: 0.0, o.numero("codigo")?.toInt() ?: 0, o.texto("descripcion").orEmpty(), o.numero("max") ?: 0.0, o.numero("min") ?: 0.0, o.si("dia") ?: true)
}

/** Cómo fue su día, contado. */
suspend fun ApiSintetica.resumen(id: String): String? = runCatching { pedir("/v1/bichos/$id/resumen").texto("texto") }.getOrNull()

/** La memoria cruda (para el marcador de los juegos). */
suspend fun ApiSintetica.memoriaCruda(id: String): JsonObject? = pedir("/v1/bichos/$id/memoria").obj("memoria")
