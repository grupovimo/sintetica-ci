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

// --- v1.9: amigos, agenda, presencia, hábitos, avisos de afuera, música y charla ---

data class Amigo(val id: Long, val nombre: String, val robots: List<String>, val todos: Boolean, val conectado: Boolean)
data class Invitacion(val codigo: String, val link: String, val telegram: String?, val vence: Long?)
data class EventoAgenda(val titulo: String, val inicio: String, val todoElDia: Boolean)
data class Agenda(val conectada: Boolean, val ics: String, val aviso: Int, val proximos: List<EventoAgenda>, val error: String?)
data class Horarios(val llegas: String?, val salis: String?, val dormis: String?, val seguridad: Double, val dias: Int)
data class HabitoVista(val indice: Int, val nombre: String, val racha: Int, val hoy: Boolean, val dias: Set<String>)
data class Entrantes(val url: String?, val token: String?, val mercadoPago: Boolean, val regenerar: Boolean)
data class Suena(val sonando: Boolean, val titulo: String, val artista: String)
data class Musica(val disponible: Boolean, val conectada: Boolean, val nombre: String, val suena: Suena?)
/** La charla siempre está (sin IA, contesta el motor propio); quedan = -1 es sin tope. */
data class Charla(val disponible: Boolean, val quedan: Int, val porDia: Int, val ia: Boolean = false)
/** origen: "motor" (su motor propio) o "ia". */
data class RespuestaCharla(val texto: String, val expresion: String?, val quedan: Int, val origen: String = "ia")

/** Quién contestó: lo dice la API; si no, sin tope y sin IA propia es el motor (como la web). */
fun origenCharla(origen: String?, propia: Boolean, quedan: Int): String =
    if (origen != null) (if (Regex("^(local|motor|nucleo)").containsMatchIn(origen)) "motor" else "ia") else if (!propia && quedan < 0) "motor" else "ia"

private fun amigoDe(o: JsonObject) = Amigo(o.numero("id")?.toLong() ?: 0, o.texto("nombre").orEmpty().ifEmpty { "Tu amigo" },
    o.lista("robots").orEmpty().mapNotNull { (it as? kotlinx.serialization.json.JsonPrimitive)?.content }, o.si("todos") ?: true, o.si("conectado") ?: false)

suspend fun ApiSintetica.amigos(): List<Amigo> = pedir("/v1/amigos").lista("amigos").orEmpty().mapNotNull { (it as? JsonObject)?.let(::amigoDe) }
suspend fun ApiSintetica.invitar(): Invitacion = pedir("/v1/amigos/invitacion", "POST").let {
    Invitacion(it.texto("codigo").orEmpty(), it.texto("link").orEmpty(), it.texto("telegram"), it.texto("vence")?.let(::fechaMs))
}
suspend fun ApiSintetica.aceptarAmigo(codigo: String): Amigo = amigoDe(pedir("/v1/amigos", "POST", buildJsonObject { put("codigo", codigo) }))
suspend fun ApiSintetica.borrarAmigo(id: Long) { crudo("/v1/amigos/$id", "DELETE") }

/** null = todos los míos. */
suspend fun ApiSintetica.amigoRecibe(id: Long, robots: List<String>?) {
    pedir("/v1/amigos/$id", "PUT", buildJsonObject {
        put("robots", robots?.let { l -> kotlinx.serialization.json.JsonArray(l.map { kotlinx.serialization.json.JsonPrimitive(it) }) } ?: kotlinx.serialization.json.JsonNull)
    })
}

/** El toque: a cuántos robots llegó y si fue por Telegram. */
suspend fun ApiSintetica.tocar(id: Long, tipo: String, expresion: String?, desde: String?): Pair<Int, Boolean> {
    val r = pedir("/v1/amigos/$id/toque", "POST", buildJsonObject {
        put("tipo", tipo); if (expresion != null) put("expresion", expresion); if (desde != null) put("desde", desde)
    })
    return (r.numero("robots")?.toInt() ?: 0) to (r.si("telegram") ?: false)
}

private fun agendaDe(o: JsonObject) = Agenda(o.si("conectada") ?: false, o.texto("ics").orEmpty(), o.numero("aviso")?.toInt() ?: 10,
    o.lista("proximos").orEmpty().mapNotNull { e -> (e as? JsonObject)?.let { EventoAgenda(it.texto("titulo").orEmpty(), it.texto("inicio").orEmpty(), it.si("todoElDia") ?: false) } },
    o.texto("error")?.ifEmpty { null })

suspend fun ApiSintetica.agenda(): Agenda = agendaDe(pedir("/v1/agenda"))
suspend fun ApiSintetica.conectarAgenda(ics: String): Agenda = agendaDe(pedir("/v1/agenda", "PUT", buildJsonObject { put("ics", ics) }))
suspend fun ApiSintetica.avisoAgenda(minutos: Int) { pedir("/v1/agenda", "PUT", buildJsonObject { put("aviso", minutos) }) }
suspend fun ApiSintetica.sacarAgenda() { crudo("/v1/agenda", "DELETE") }

suspend fun ApiSintetica.presencia(id: String, llegaste: Boolean) { pedir("/v1/bichos/$id/presencia", "POST", buildJsonObject { put("llegaste", llegaste) }) }
suspend fun ApiSintetica.horarios(id: String): Horarios = pedir("/v1/bichos/$id/horarios").let {
    Horarios(it.texto("llegas"), it.texto("salis"), it.texto("dormis"), it.numero("seguridad") ?: 0.0, it.numero("dias")?.toInt() ?: 0)
}

private fun habitoDe(o: JsonObject) = HabitoVista(o.numero("indice")?.toInt() ?: 0, o.texto("nombre").orEmpty(), o.numero("racha")?.toInt() ?: 0,
    o.si("hoy") ?: false, o.obj("dias").orEmpty().filterValues { (it as? kotlinx.serialization.json.JsonPrimitive)?.content == "true" }.keys)

/** El GET devuelve una lista suelta. */
suspend fun ApiSintetica.habitos(id: String): List<HabitoVista> {
    val e = runCatching { kotlinx.serialization.json.Json.parseToJsonElement(String(crudo("/v1/bichos/$id/habitos"))) }.getOrNull()
    return (e as? kotlinx.serialization.json.JsonArray).orEmpty().mapNotNull { (it as? JsonObject)?.let(::habitoDe) }
}
suspend fun ApiSintetica.marcarHabito(id: String, i: Int, hecho: Boolean): HabitoVista? =
    pedir("/v1/bichos/$id/habitos/$i", "POST", buildJsonObject { put("hecho", hecho) }).obj("habito")?.let(::habitoDe)

private fun entrantesDe(o: JsonObject) = Entrantes(o.texto("url"), o.texto("token"), o.si("mercadoPago") ?: false, o.si("regenerar") ?: false)
suspend fun ApiSintetica.entrantes(): Entrantes = entrantesDe(pedir("/v1/entrantes"))
suspend fun ApiSintetica.regenerarEntrantes(): Entrantes = entrantesDe(pedir("/v1/entrantes", "POST"))
suspend fun ApiSintetica.mercadoPago(token: String?) {
    if (token == null) crudo("/v1/entrantes/mercadopago", "DELETE") else pedir("/v1/entrantes/mercadopago", "PUT", buildJsonObject { put("accessToken", token) })
}
/** Probar un tipo: a cuántos robots llegó. */
suspend fun ApiSintetica.probarEntrante(token: String, tipo: String, titulo: String, de: String, robot: String): Int =
    pedir("/v1/entrantes/$token", "POST", buildJsonObject { put("tipo", tipo); put("titulo", titulo); put("de", de); put("robot", robot) }).numero("robots")?.toInt() ?: 0

fun suenaDe(o: JsonObject?) = o?.let { Suena(it.si("sonando") ?: false, it.texto("titulo").orEmpty(), it.texto("artista").orEmpty()) }
suspend fun ApiSintetica.musica(): Musica = pedir("/v1/musica").let { Musica(it.si("disponible") ?: false, it.si("conectada") ?: false, it.texto("nombre").orEmpty(), suenaDe(it.obj("suena"))) }
suspend fun ApiSintetica.urlSpotify(): String = pedir("/v1/musica/spotify").texto("url").orEmpty()
suspend fun ApiSintetica.sacarSpotify() { crudo("/v1/musica/spotify", "DELETE") }
suspend fun ApiSintetica.mandarMusica(id: String, sonando: Boolean, titulo: String, artista: String) {
    pedir("/v1/bichos/$id/musica", "POST", buildJsonObject { put("sonando", sonando); put("titulo", titulo); put("artista", artista) })
}

suspend fun ApiSintetica.charla(): Charla = pedir("/v1/charla").let {
    Charla(it.si("disponible") ?: true, it.numero("quedan")?.toInt() ?: -1, it.numero("porDia")?.toInt() ?: 0, it.si("ia") ?: false)
}
suspend fun ApiSintetica.charlar(id: String, texto: String): RespuestaCharla = pedir("/v1/bichos/$id/charla", "POST", buildJsonObject { put("texto", texto) }).let {
    val quedan = it.numero("quedan")?.toInt() ?: -1
    RespuestaCharla(it.texto("texto").orEmpty(), it.texto("expresion"), quedan, origenCharla(it.texto("origen"), it.si("propia") ?: false, quedan))
}

// --- La voz (Edge gratis y ElevenLabs), el paquete sin internet, tu IA y la conversación ---

data class VozOpcion(val id: String, val nombre: String)
data class VozEdge(val disponible: Boolean, val prendida: Boolean, val voz: String, val voces: List<VozOpcion>, val usados: Int, val tope: Int)
data class VozEleven(val conectado: Boolean, val voz: String, val voces: List<VozOpcion>, val leer: Boolean, val usados: Int, val tope: Int, val segundos: Int, val topeMinutos: Int)
data class PaqueteVoz(
    val estado: String, val error: String?, val version: Int, val voz: String, val frases: Int, val total: Int, val bytes: Long,
    val bytesEstimados: Long, val disponible: Boolean, val proveedor: String, val vozNueva: String?, val faltan: Int, val gasta: Int,
    val hechas: Int, val de: Int,
)
data class IAPropia(val propia: Boolean, val servidor: Boolean, val proveedor: String, val modelo: String, val urlPista: String, val usadas: Int, val tope: Int)
data class Conversacion(val token: String, val segundos: Int, val quedan: Int, val expresiones: List<String>)

private fun vozEdgeDe(o: JsonObject) = VozEdge(o.si("disponible") ?: false, o.si("prendida") ?: false, o.texto("voz").orEmpty(),
    o.lista("voces").orEmpty().mapNotNull { (it as? JsonObject)?.let { v -> VozOpcion(v.texto("id").orEmpty(), "${v.texto("nombre").orEmpty().substringBefore(' ')} · ${v.texto("idioma").orEmpty().drop(3)}") } },
    o.numero("usados_hoy")?.toInt() ?: 0, o.numero("tope")?.toInt() ?: 0)

private fun vozElevenDe(o: JsonObject) = VozEleven(o.si("conectado") ?: false, o.texto("voz").orEmpty(),
    o.lista("voces").orEmpty().mapNotNull { (it as? JsonObject)?.let { v -> VozOpcion(v.texto("id").orEmpty(), v.texto("nombre").orEmpty()) } },
    o.si("leer") ?: true, o.numero("usados_hoy")?.toInt() ?: 0, o.numero("tope")?.toInt() ?: 5000, o.numero("segundos_hoy")?.toInt() ?: 0, o.numero("tope_minutos")?.toInt() ?: 10)

fun paqueteDe(o: JsonObject) = PaqueteVoz(
    o.texto("estado") ?: "nuevo", o.texto("error")?.ifEmpty { null }, o.numero("version")?.toInt() ?: 0, o.texto("voz").orEmpty(), o.numero("frases")?.toInt() ?: 0,
    o.numero("total")?.toInt() ?: 0, o.numero("bytes")?.toLong() ?: 0, o.numero("bytes_estimados")?.toLong() ?: 0, o.si("disponible") ?: false,
    o.texto("proveedor").orEmpty(), o.texto("voz_nueva"), o.numero("faltan")?.toInt() ?: 0, o.numero("gasta")?.toInt() ?: 0,
    o.obj("progreso")?.numero("hechas")?.toInt() ?: 0, o.obj("progreso")?.numero("de")?.toInt() ?: 0,
)

private fun iaDe(o: JsonObject) = IAPropia(o.si("propia") ?: false, o.si("servidor") ?: false, o.texto("proveedor").orEmpty(), o.texto("modelo").orEmpty(),
    o.texto("url_pista").orEmpty(), o.numero("usadas_hoy")?.toInt() ?: 0, o.numero("tope")?.toInt() ?: 0)

suspend fun ApiSintetica.vozEdge(): VozEdge = vozEdgeDe(pedir("/v1/voz/edge"))
suspend fun ApiSintetica.ponerVozEdge(cuerpo: JsonObject): VozEdge = vozEdgeDe(pedir("/v1/voz/edge", "PUT", cuerpo))
suspend fun ApiSintetica.vozEleven(): VozEleven = vozElevenDe(pedir("/v1/voz/elevenlabs"))
suspend fun ApiSintetica.ponerVozEleven(cuerpo: JsonObject): VozEleven = vozElevenDe(pedir("/v1/voz/elevenlabs", "PUT", cuerpo))
suspend fun ApiSintetica.sacarVozEleven(): VozEleven = vozElevenDe(pedir("/v1/voz/elevenlabs", "DELETE"))

/** Una prueba de la voz: el mp3 (cual: "edge" o "elevenlabs"). */
suspend fun ApiSintetica.probarVoz(cual: String, texto: String): ByteArray =
    crudo("/v1/voz/$cual/probar", "POST", buildJsonObject { put("texto", texto) }.toString().toByteArray())

suspend fun ApiSintetica.paqueteVoz(id: String): PaqueteVoz = paqueteDe(pedir("/v1/bichos/$id/voz/paquete"))

/** Generarlo: el estado, o (con ElevenLabs y sin confirmar) el 409 con cuánto gasta. */
suspend fun ApiSintetica.generarPaquete(id: String, confirmar: Boolean): Pair<Int, PaqueteVoz?> {
    val (e, o) = conEstado("/v1/bichos/$id/voz/paquete", "POST", if (confirmar) buildJsonObject { put("confirmar", true) } else buildJsonObject { })
    if (e >= 400 && e != 409) throw ErrorApi(e, o.texto("error") ?: "Error $e", o.texto("detalle").orEmpty())
    return e to (if (o.texto("estado") != null) paqueteDe(o) else null)
}

suspend fun ApiSintetica.ia(): IAPropia = iaDe(pedir("/v1/ia"))
suspend fun ApiSintetica.ponerIa(cuerpo: JsonObject): IAPropia = iaDe(pedir("/v1/ia", "PUT", cuerpo))
suspend fun ApiSintetica.sacarIa() { crudo("/v1/ia", "DELETE") }

/** El token para hablar en tiempo real (WebRTC, el SDK de Android), con los segundos que puede durar y sus caras. */
suspend fun ApiSintetica.conversacion(id: String): Conversacion = pedir("/v1/bichos/$id/conversacion?modo=webrtc").let {
    Conversacion(it.texto("token") ?: it.texto("url").orEmpty(), it.numero("segundos")?.toInt() ?: 60, it.numero("quedan")?.toInt() ?: 0, it.textos("expresiones"))
}
