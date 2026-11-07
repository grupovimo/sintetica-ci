package com.sinteticalabs.app.protocolo

import java.util.UUID

/**
 * Protocolo de SintéticaOS v1.3 (BLE y nube), del lado de la app. Es la misma lógica que
 * sim/web/protocolo.js; los tests usan shared/protocolo/vectores.json para que
 * las implementaciones no se desincronicen. Ver docs/protocolo-ble.md.
 */
object Uuids {
    val SERVICIO: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000001")
    val CONFIG: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000002")
    val COMANDO: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000003")
    val ESTADO: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000004")
    val OTA: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000005")
    val IMAGEN: UUID = UUID.fromString("5157e71c-a000-4b1c-8a5e-000000000006")
    val CCCD: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
}

enum class Car(val id: Int, val uuid: UUID) {
    CONFIG(1, Uuids.CONFIG), COMANDO(2, Uuids.COMANDO), ESTADO(3, Uuids.ESTADO), OTA(4, Uuids.OTA), IMAGEN(5, Uuids.IMAGEN);

    companion object {
        fun de(id: Int) = entries.firstOrNull { it.id == id }
        fun de(uuid: UUID) = entries.firstOrNull { it.uuid == uuid }
    }
}

object Op {
    const val CONFIG_INICIO = 0x01
    const val CONFIG_BLOQUE = 0x02
    const val CONFIG_FIN = 0x03
    const val CONFIG_PEDIR = 0x04
    const val EXPRESION = 0x10
    const val NOMBRE = 0x11
    const val EVENTO = 0x12
    const val VINCULAR = 0x13
    const val AUTENTICAR = 0x14
    const val DESVINCULAR = 0x15
    const val RED = 0x16
    const val NUBE = 0x17
    const val OLVIDAR_RED = 0x18
    const val REPRODUCIR = 0x19
    const val HORA = 0x1A           // v1.4
    const val VIDA = 0x1B
    const val MEMORIA = 0x1C
    const val RESTAURAR = 0x1D
    const val REGISTRO = 0x1E
    const val COMPORTAMIENTO = 0x1F
    const val IMG_INICIO = 0x20
    const val IMG_BLOQUE = 0x21
    const val IMG_FIN = 0x22
    const val IMG_BORRAR = 0x23
    const val IMG_LISTAR = 0x24
    const val IMG_PEDIR = 0x25
    const val OTA_INICIO = 0x30
    const val OTA_BLOQUE = 0x31
    const val OTA_FIN = 0x32
    const val OTA_DESCARGAR = 0x33
    const val OTA_PROGRESO = 0x34
    const val OTA_CANCELAR = 0x35
    const val RESPUESTA = 0x80
    const val CONFIG_TROZO = 0x84
    const val CONFIG_CIERRE = 0x85
    const val IMG_TROZO = 0xA7
    const val IMG_CIERRE = 0xA8
}

/** En el orden del protocolo (el índice viaja en el comando Evento). Los primeros 8 son de la v1. */
val EVENTOS = listOf(
    "tacto", "doble-toque", "levantado", "sacudida", "boca-abajo", "vinculado", "app-abierta", "despertado",
    "caricia", "muchos-toques", "sacudida-suave", "sacudida-fuerte", "inclinado", "caida", "voz", "nombre",
    "ruido", "musica", "silencio", "notificacion", "llamada", "bateria-baja", "bateria-critica", "enchufado",
    "desenchufado", "cargado", "sin-wifi", "cambio-de-cara", "bicho-cerca", "volvio", "hito",
)
val MODOS = listOf("mascota", "reloj", "pomodoro", "clima", "notificaciones")

private val MENSAJES = mapOf(
    0 to "ok", 1 to "muy grande", 2 to "fuera de orden", 3 to "CRC distinto", 4 to "JSON inválido",
    5 to "config inválida", 6 to "sin inicio", 7 to "no existe", 8 to "malformado", 9 to "no autorizado",
    10 to "no está mostrando el QR", 11 to "imagen inválida", 12 to "firma inválida", 13 to "hash distinto",
    14 to "firmware de otra placa", 15 to "sin WiFi", 16 to "ocupado", 17 to "falló la actualización",
    255 to "no implementado",
)

private fun u16(v: Int) = byteArrayOf((v and 0xFF).toByte(), (v shr 8 and 0xFF).toByte())

/** Texto con su largo adelante (u8). */
private fun corto(s: String, max: Int): ByteArray {
    val b = s.toByteArray()
    require(b.size <= max) { "\"${s.take(20)}\" es muy largo (máximo $max bytes)" }
    return byteArrayOf(b.size.toByte()) + b
}

private fun u32(v: Long) = byteArrayOf((v and 0xFF).toByte(), (v shr 8 and 0xFF).toByte(), (v shr 16 and 0xFF).toByte(), (v shr 24 and 0xFF).toByte())

fun leerU32(b: ByteArray, i: Int): Long =
    (b[i].toLong() and 0xFF) or ((b[i + 1].toLong() and 0xFF) shl 8) or ((b[i + 2].toLong() and 0xFF) shl 16) or ((b[i + 3].toLong() and 0xFF) shl 24)

fun crc32(bytes: ByteArray): Long {
    var c = 0xFFFFFFFFL
    for (b in bytes) {
        c = c xor (b.toLong() and 0xFF)
        repeat(8) { c = if (c and 1L != 0L) (c ushr 1) xor 0xEDB88320L else c ushr 1 }
    }
    return c xor 0xFFFFFFFFL
}

object Comando {
    fun expresion(nombre: String = "") = byteArrayOf(Op.EXPRESION.toByte()) + nombre.toByteArray()
    fun nombre(nombre: String) = byteArrayOf(Op.NOMBRE.toByte()) + nombre.toByteArray()
    fun evento(nombre: String): ByteArray {
        val i = EVENTOS.indexOf(nombre)
        require(i >= 0) { "evento desconocido: $nombre" }
        return byteArrayOf(Op.EVENTO.toByte(), i.toByte())
    }
    fun vincular(codigo: String) = byteArrayOf(Op.VINCULAR.toByte()) + codigo.toByteArray()
    fun autenticar(clave: ByteArray) = byteArrayOf(Op.AUTENTICAR.toByte()) + clave
    fun desvincular() = byteArrayOf(Op.DESVINCULAR.toByte())
    fun red(ssid: String, clave: String = "") = byteArrayOf(Op.RED.toByte()) + corto(ssid, 32) + corto(clave, 64)
    fun nube(url: String, usuario: String, clave: String) =
        byteArrayOf(Op.NUBE.toByte()) + corto(url, 160) + corto(usuario, 64) + corto(clave, 128)
    fun olvidarRed() = byteArrayOf(Op.OLVIDAR_RED.toByte())
    fun reproducir(animacion: String = "") = byteArrayOf(Op.REPRODUCIR.toByte()) + animacion.toByteArray()
    fun pedirConfig() = byteArrayOf(Op.CONFIG_PEDIR.toByte())
    /** La hora (v1.4): epoch UTC en segundos y la zona en minutos (Argentina: -180). */
    fun hora(epochUtc: Long, zonaMinutos: Int) = byteArrayOf(Op.HORA.toByte()) + u32(epochUtc) + u16(zonaMinutos and 0xFFFF)
    fun vida(superposicion: Boolean? = null) =
        if (superposicion == null) byteArrayOf(Op.VIDA.toByte()) else byteArrayOf(Op.VIDA.toByte(), if (superposicion) 1 else 0)
    fun memoria() = byteArrayOf(Op.MEMORIA.toByte())
    fun restaurar(memoria: ByteArray) = byteArrayOf(Op.RESTAURAR.toByte()) + memoria
    fun registro() = byteArrayOf(Op.REGISTRO.toByte())
    fun comportamiento(nombre: String) = byteArrayOf(Op.COMPORTAMIENTO.toByte()) + nombre.toByteArray()
}

/** Una imagen QOI (del tamaño de la pantalla) en las escrituras de la característica Imagen. */
fun escriturasDeImagen(espacio: Int, qoi: ByteArray, tamanoBloque: Int = 180): List<ByteArray> {
    val porBloque = tamanoBloque - 5
    val salida = mutableListOf(byteArrayOf(Op.IMG_INICIO.toByte(), espacio.toByte()) + u32(qoi.size.toLong()) + u32(crc32(qoi)))
    var off = 0
    while (off < qoi.size) {
        val fin = minOf(qoi.size, off + porBloque)
        salida += byteArrayOf(Op.IMG_BLOQUE.toByte()) + u32(off.toLong()) + qoi.copyOfRange(off, fin)
        off = fin
    }
    salida += byteArrayOf(Op.IMG_FIN.toByte())
    return salida
}

object Imagen {
    fun borrar(espacio: Int) = byteArrayOf(Op.IMG_BORRAR.toByte(), espacio.toByte())
    fun listar() = byteArrayOf(Op.IMG_LISTAR.toByte())
    fun pedir(espacio: Int) = byteArrayOf(Op.IMG_PEDIR.toByte(), espacio.toByte())
}

/** Una actualización firmada: lo que manda la API (ver docs/protocolo-ble.md, "OTA"). */
data class Manifiesto(val tamano: Long, val sha256: ByteArray, val firma: ByteArray, val version: String, val perfil: String) {
    fun bytes(): ByteArray {
        require(sha256.size == 32 && firma.size == 64) { "sha256 de 32 bytes y firma de 64" }
        return u32(tamano) + sha256 + firma + corto(version, 23) + corto(perfil, 23)
    }
}

object Ota {
    fun descargar(m: Manifiesto, url: String): ByteArray {
        val u = url.toByteArray()
        return byteArrayOf(Op.OTA_DESCARGAR.toByte()) + m.bytes() + u16(u.size) + u
    }
    fun cancelar() = byteArrayOf(Op.OTA_CANCELAR.toByte())
    /** Por BLE: el manifiesto, los bloques del binario y el fin. */
    fun escrituras(m: Manifiesto, binario: ByteArray, tamanoBloque: Int = 180): List<ByteArray> {
        val porBloque = tamanoBloque - 5
        val salida = mutableListOf(byteArrayOf(Op.OTA_INICIO.toByte()) + m.bytes())
        var off = 0
        while (off < binario.size) {
            val fin = minOf(binario.size, off + porBloque)
            salida += byteArrayOf(Op.OTA_BLOQUE.toByte()) + u32(off.toLong()) + binario.copyOfRange(off, fin)
            off = fin
        }
        salida += byteArrayOf(Op.OTA_FIN.toByte())
        return salida
    }
}

data class ProgresoOta(val codigo: Int, val porcentaje: Int, val detalle: String) {
    val ok get() = codigo == 0
    val mensaje get() = MENSAJES[codigo] ?: "código $codigo"
}

/** La notificación [0xB4][código][porcentaje][detalle], o null si no es eso. */
fun leerProgresoOta(b: ByteArray): ProgresoOta? {
    if (b.size < 3 || (b[0].toInt() and 0xFF) != (Op.OTA_PROGRESO or Op.RESPUESTA)) return null
    return ProgresoOta(b[1].toInt() and 0xFF, b[2].toInt() and 0xFF, if (b.size > 3) String(b, 3, b.size - 3) else "")
}

data class EspacioImagen(val espacio: Int, val largo: Long, val crc: Long)

/** La respuesta a Imagen.listar(): los espacios ocupados. */
fun leerListaImagenes(b: ByteArray): List<EspacioImagen> {
    val l = mutableListOf<EspacioImagen>()
    var i = 3
    while (i + 9 <= b.size) {
        l += EspacioImagen(b[i].toInt() and 0xFF, leerU32(b, i + 1), leerU32(b, i + 5))
        i += 9
    }
    return l
}

/** Parte la config (texto JSON) en las escrituras de la característica Config. */
fun escriturasDeConfig(json: String, tamanoBloque: Int = 180): List<ByteArray> {
    val bytes = json.toByteArray()
    val porBloque = tamanoBloque - 5
    val salida = mutableListOf(byteArrayOf(Op.CONFIG_INICIO.toByte()) + u32(bytes.size.toLong()) + u32(crc32(bytes)))
    var off = 0
    while (off < bytes.size) {
        val fin = minOf(bytes.size, off + porBloque)
        salida += byteArrayOf(Op.CONFIG_BLOQUE.toByte()) + u32(off.toLong()) + bytes.copyOfRange(off, fin)
        off = fin
    }
    salida += byteArrayOf(Op.CONFIG_FIN.toByte())
    return salida
}

sealed interface Respuesta {
    data class Resultado(val op: Int, val codigo: Int, val detalle: String, val clave: ByteArray? = null) : Respuesta {
        val ok get() = codigo == 0
        val mensaje get() = MENSAJES[codigo] ?: "código $codigo"
    }
    data class Trozo(val desplazamiento: Int, val datos: ByteArray) : Respuesta
    data class Cierre(val largo: Int, val crc: Long) : Respuesta
}

fun leerRespuesta(b: ByteArray): Respuesta {
    val op0 = b[0].toInt() and 0xFF
    if (op0 == Op.CONFIG_TROZO) return Respuesta.Trozo(leerU32(b, 1).toInt(), b.copyOfRange(5, b.size))
    if (op0 == Op.CONFIG_CIERRE) return Respuesta.Cierre(leerU32(b, 1).toInt(), leerU32(b, 5))
    val op = op0 and 0x7F
    val codigo = if (b.size > 1) b[1].toInt() and 0xFF else 255
    if (op == Op.VINCULAR && codigo == 0 && b.size >= 18) return Respuesta.Resultado(op, 0, "", b.copyOfRange(2, 18))
    val detalle = if (b.size > 2) String(b, 2, b.size - 2) else ""
    return Respuesta.Resultado(op, codigo, detalle)
}

/** Junta la config que el bicho manda en trozos después de pedirla. */
class ArmadorDeConfig {
    private val partes = mutableListOf<Respuesta.Trozo>()
    fun agregar(r: Respuesta): String? {
        when (r) {
            is Respuesta.Trozo -> { partes += r; return null }
            is Respuesta.Cierre -> {
                val bytes = ByteArray(r.largo)
                for (p in partes) p.datos.copyInto(bytes, p.desplazamiento)
                partes.clear()
                check(crc32(bytes) == r.crc) { "la config llegó con otro CRC" }
                return String(bytes)
            }
            else -> return null
        }
    }
}

data class Pantalla(val ancho: Int, val alto: Int, val redonda: Boolean)

data class Estado(
    val pantalla: Pantalla = Pantalla(320, 172, false),
    val bateria: Int?,
    val humor: Float,
    val modo: String,
    val dormido: Boolean,
    val fija: Boolean,
    val emparejando: Boolean,
    val autenticado: Boolean,
    val expresion: String,
    val nombre: String,
    val serie: String,
    val id: String,
    val wifi: Boolean = false,
    val nube: Boolean = false,
    val redConfigurada: Boolean = false,
    val actualizando: Boolean = false,
    val version: String = "",
    val perfil: String = "",
    val crcConfig: Long = 0,
    val encendido: Long = 0,
)

fun leerEstado(b: ByteArray): Estado {
    val version = b[0].toInt()
    require(version in 1..4) { "versión de estado desconocida: $version" }
    var i = 5
    fun campo(): String {
        val n = b[i].toInt() and 0xFF
        val s = String(b, i + 1, n)
        i += 1 + n
        return s
    }
    val expresion = campo()
    val nombre = campo()
    val serie = if (version >= 2) campo() else ""
    val id = if (version >= 2) campo() else ""
    val flags = b[4].toInt()
    val pantalla = if (version >= 3) Pantalla(
        (b[i].toInt() and 0xFF) or ((b[i + 1].toInt() and 0xFF) shl 8),
        (b[i + 2].toInt() and 0xFF) or ((b[i + 3].toInt() and 0xFF) shl 8),
        b[i + 4].toInt() == 1,
    ) else Pantalla(320, 172, false)
    if (version >= 3) i += 5
    val firmware = if (version >= 4) campo() else ""
    val perfil = if (version >= 4) campo() else ""
    val crcConfig = if (version >= 4) leerU32(b, i) else 0L
    val encendido = if (version >= 4) leerU32(b, i + 4) else 0L
    return Estado(
        pantalla = pantalla,
        bateria = (b[1].toInt() and 0xFF).takeIf { it != 255 },
        humor = b[2] / 100f,
        modo = MODOS.getOrElse(b[3].toInt() and 0xFF) { "?" },
        dormido = flags and 1 != 0,
        fija = flags and 2 != 0,
        emparejando = flags and 4 != 0,
        autenticado = flags and 8 != 0,
        expresion = expresion, nombre = nombre, serie = serie, id = id,
        wifi = flags and 16 != 0,
        nube = flags and 32 != 0,
        redConfigurada = flags and 64 != 0,
        actualizando = flags and 128 != 0,
        version = firmware, perfil = perfil, crcConfig = crcConfig, encendido = encendido,
    )
}

data class DatosQr(val id: String, val codigo: String)

private val PATRON_QR = Regex("^SINTETICA:V1/([0-9A-F]{12})/([2-9A-HJKMNP-Z]{6})$")

/** El QR del bicho: "SINTETICA:V1/<id de 12>/<código de 6>". */
fun leerQr(texto: String): DatosQr? = PATRON_QR.matchEntire(texto.trim())?.let { DatosQr(it.groupValues[1], it.groupValues[2]) }

/** El id en los datos de fabricante del anuncio: 0xFFFF + la MAC (id 0xFFFF ya separado o no). */
fun idDeAnuncio(datos: ByteArray?): String? {
    if (datos == null || datos.size < 8 || datos[0] != 0xFF.toByte() || datos[1] != 0xFF.toByte()) return null
    return datos.copyOfRange(2, 8).joinToString("") { "%02X".format(it) }
}

fun hex(b: ByteArray) = b.joinToString(" ") { "%02x".format(it) }
fun deHex(s: String): ByteArray = s.trim().split(" ").filter { it.isNotEmpty() }.map { it.toInt(16).toByte() }.toByteArray()
