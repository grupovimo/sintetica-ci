package com.sinteticalabs.app.datos

import android.content.Context
import java.io.File

/**
 * Las imágenes que este celu le mandó a cada bicho. El bicho no las devuelve,
 * así que la app guarda una copia para mostrarlas en la vista previa: la del
 * tamaño de la pantalla (la que se mandó) y una al doble (la de la vista previa).
 */
class CacheImagenes(contexto: Context) {
    private val raiz = File(contexto.filesDir, "imagenes")

    private fun archivo(id: String, espacio: Int, vista: Boolean) =
        File(File(raiz, id), if (vista) "$espacio-vista.qoi" else "$espacio.qoi")

    fun guardar(id: String, espacio: Int, qoi: ByteArray, vistaPrevia: ByteArray) {
        archivo(id, espacio, false).also { it.parentFile?.mkdirs() }.writeBytes(qoi)
        archivo(id, espacio, true).writeBytes(vistaPrevia)
    }

    fun mandada(id: String, espacio: Int): ByteArray? = archivo(id, espacio, false).takeIf { it.exists() }?.readBytes()
    fun vistaPrevia(id: String, espacio: Int): ByteArray? = archivo(id, espacio, true).takeIf { it.exists() }?.readBytes()

    fun borrar(id: String, espacio: Int) {
        archivo(id, espacio, false).delete()
        archivo(id, espacio, true).delete()
    }
}
