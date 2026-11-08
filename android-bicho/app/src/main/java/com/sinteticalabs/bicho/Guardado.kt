package com.sinteticalabs.bicho

import android.util.Log
import java.io.File

/** Lo que guarda el núcleo (config, memoria de la relación) en archivos de la app; lo de red va a [Red] y los sonidos a [Voz]. */
class Guardado(raiz: File, private val red: Red, private val voz: Voz) : Nucleo.Anfitrion {
    private val carpeta = File(raiz, "nucleo").apply { mkdirs() }

    private fun archivo(clave: String) = File(carpeta, clave.replace(Regex("[^A-Za-z0-9_.-]"), "_"))

    override fun guardar(clave: String, datos: ByteArray): Boolean = runCatching {
        val tmp = File(carpeta, "${archivo(clave).name}.tmp")
        tmp.writeBytes(datos)
        tmp.renameTo(archivo(clave))
    }.getOrDefault(false)

    override fun leer(clave: String): ByteArray? = archivo(clave).takeIf { it.exists() }?.readBytes()

    /** Cuando el núcleo aplica una config nueva (por ejemplo, para pedir el paquete de voz). */
    @Volatile var alConfig: (() -> Unit)? = null

    override fun registrar(mensaje: String) {
        Log.i("Bicho", mensaje)
        if (mensaje == "config nueva aplicada") alConfig?.invoke()
    }

    override fun notificar(caracteristica: Int, datos: ByteArray) = red.notificar(caracteristica, datos)
    override fun notificarNube(caracteristica: Int, datos: ByteArray) = red.notificarNube(caracteristica, datos)
    override fun redCambio() = red.redCambio()
    override fun sonido(nombre: String) = voz.sonar(nombre)
}
