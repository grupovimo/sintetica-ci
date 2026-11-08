package com.sinteticalabs.bicho

import android.util.Log
import org.json.JSONObject
import java.io.File
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL

/**
 * El paquete de voz sin internet: las frases que el bicho dice solo, ya en mp3
 * con su voz de la nube, guardadas en el celu (docs/nube.md, "Paquete de voz").
 * Se pide al conectarse, una vez por día y cuando cambia la config; se baja
 * entero a una carpeta nueva y recién al terminar reemplaza al anterior.
 */
class PaqueteVoz(
    raiz: File,
    private val publicar: (String, String) -> Unit,
    /** La URL que se pueda abrir desde el celu (ver [Red.alcanzable]). */
    private val alcanzable: (String) -> String = { it },
) {
    private val carpeta = File(raiz, "voz").apply { mkdirs() }
    @Volatile private var actual: File? = carpeta.listFiles { f -> f.isDirectory && f.name.startsWith("v") }
        ?.maxByOrNull { it.lastModified() }
    @Volatile private var bajando = false
    private var pedido = 0L

    /** La versión que tiene (el nombre de la carpeta), o null. */
    val version get() = actual?.name?.removePrefix("v")

    /** El mp3 de ese texto (por el CRC de su UTF-8), si está en el paquete. */
    fun mp3(crc: Long): File? = actual?.let { File(it, "%08x.mp3".format(crc)) }?.takeIf { it.isFile }

    /** Al conectarse, al cambiar la config y una vez por día (lo llama [Bicho]). */
    fun pedir(porque: String) {
        val ahora = System.currentTimeMillis()
        if (porque == "dia" && ahora - pedido < DIA_MS) return
        pedido = ahora
        Log.i("Bicho", "paquete de voz: lo pido ($porque)")
        publicar("voz/paquete", "{\"pedir\":true,\"tope\":0}")
    }

    /** Lo que llega por bicho/{id}/voz/paquete/url (hilo de MQTT). */
    fun respuesta(json: String) {
        val o = runCatching { JSONObject(json) }.getOrNull() ?: return
        val url = o.optString("url")
        val v = o.optString("version")
        when {
            o.has("error") -> Log.w("Bicho", "paquete de voz: ${o.optString("error")}")
            url.isEmpty() -> Log.i("Bicho", "paquete de voz: ${o.optString("estado")}")
            v == version -> Log.i("Bicho", "paquete de voz: ya tengo la $v")
            bajando -> Unit
            else -> {
                bajando = true
                Thread({
                    try {
                        bajar(url, v)
                    } catch (e: Exception) {
                        Log.w("Bicho", "paquete de voz: no se pudo bajar (${e.message}); queda el anterior")
                    } finally {
                        bajando = false
                    }
                }, "paquete-voz").start()
            }
        }
    }

    private fun bajar(url: String, v: String) {
        val nueva = File(carpeta, "v$v.tmp")
        nueva.deleteRecursively()
        nueva.mkdirs()
        val c = URL(alcanzable(url)).openConnection() as HttpURLConnection
        c.connectTimeout = 15_000
        c.readTimeout = 30_000
        try {
            if (c.responseCode != 200) throw IllegalStateException("HTTP ${c.responseCode}")
            var n = 0
            c.inputStream.buffered().use { ent ->
                Tar.leer(ent) { nombre, datos ->
                    // Solo nombres planos (el índice y <hash>.mp3): nada de rutas.
                    if (nombre.matches(Regex("[A-Za-z0-9_.-]+"))) {
                        File(nueva, nombre).writeBytes(datos)
                        n++
                    }
                }
            }
            if (!File(nueva, "indice.json").isFile) throw IllegalStateException("sin indice.json")
            val final = File(carpeta, "v$v")
            final.deleteRecursively()
            if (!nueva.renameTo(final)) throw IllegalStateException("no pude renombrar")
            val vieja = actual
            actual = final
            if (vieja != null && vieja != final) vieja.deleteRecursively()
            Log.i("Bicho", "paquete de voz: versión $v, ${n - 1} frases")
        } finally {
            c.disconnect()
            nueva.deleteRecursively()
        }
    }

    companion object {
        const val DIA_MS = 24 * 60 * 60 * 1000L
    }
}

/** Un lector de tar ustar mínimo: archivos comunes, de a uno, en orden. */
object Tar {
    fun leer(ent: InputStream, alArchivo: (String, ByteArray) -> Unit) {
        val cabecera = ByteArray(512)
        while (true) {
            if (!llenar(ent, cabecera)) return
            if (cabecera.all { it.toInt() == 0 }) return  // el final
            val nombre = texto(cabecera, 0, 100).let { n ->
                val prefijo = texto(cabecera, 345, 155)
                if (prefijo.isEmpty()) n else "$prefijo/$n"
            }
            val largo = texto(cabecera, 124, 12).trim().ifEmpty { "0" }.toLong(8)
            val tipo = cabecera[156].toInt().toChar()
            val datos = ByteArray(largo.toInt())
            if (!llenar(ent, datos)) return
            val relleno = ((512 - largo % 512) % 512).toInt()
            if (relleno > 0) llenar(ent, ByteArray(relleno))
            if (tipo == '0' || tipo == '\u0000') alArchivo(nombre.substringAfterLast('/'), datos)
        }
    }

    private fun texto(b: ByteArray, desde: Int, largo: Int): String {
        var fin = desde
        while (fin < desde + largo && b[fin].toInt() != 0) fin++
        return String(b, desde, fin - desde, Charsets.UTF_8)
    }

    private fun llenar(ent: InputStream, b: ByteArray): Boolean {
        var i = 0
        while (i < b.size) {
            val n = ent.read(b, i, b.size - i)
            if (n < 0) return false
            i += n
        }
        return true
    }
}
