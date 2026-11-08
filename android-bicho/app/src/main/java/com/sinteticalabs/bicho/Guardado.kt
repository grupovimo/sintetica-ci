package com.sinteticalabs.bicho

import android.util.Log
import java.io.File

/** Lo que guarda el núcleo (config, memoria de la relación) en archivos de la app; lo de red va a [Red]. */
class Guardado(raiz: File, private val red: Red) : Nucleo.Anfitrion {
    private val carpeta = File(raiz, "nucleo").apply { mkdirs() }

    private fun archivo(clave: String) = File(carpeta, clave.replace(Regex("[^A-Za-z0-9_.-]"), "_"))

    override fun guardar(clave: String, datos: ByteArray): Boolean = runCatching {
        val tmp = File(carpeta, "${archivo(clave).name}.tmp")
        tmp.writeBytes(datos)
        tmp.renameTo(archivo(clave))
    }.getOrDefault(false)

    override fun leer(clave: String): ByteArray? = archivo(clave).takeIf { it.exists() }?.readBytes()

    override fun registrar(mensaje: String) {
        Log.i("Bicho", mensaje)
    }

    override fun notificar(caracteristica: Int, datos: ByteArray) = red.notificar(caracteristica, datos)
    override fun notificarNube(caracteristica: Int, datos: ByteArray) = red.notificarNube(caracteristica, datos)
    override fun redCambio() = red.redCambio()
}
