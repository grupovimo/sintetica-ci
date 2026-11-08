package com.sinteticalabs.bicho

import android.os.SystemClock
import android.util.Log
import java.util.zip.CRC32

/**
 * La voz que manda la nube (ElevenLabs, con la cuenta del dueño) por
 * bicho/{id}/audio: los mp3 enteros (tipo 2) que se juntan por id y se guardan
 * por el CRC de su texto un rato, y si la nube lee los globos (tipo 3).
 * Ver docs/protocolo-ble.md, "Voz".
 */
class VozNube(private val ahora: () -> Long = { SystemClock.elapsedRealtime() }) {
    class Audio(val crc: Long, val origen: Int, val mp3: ByteArray, val llego: Long)

    /** La nube lee en voz alta los recordatorios y el despertador (tipo 3, bit 0). */
    @Volatile var leeGlobos = false
        private set

    /** Lo llama Voz cuando junta un audio entero. */
    var alLlegar: ((Audio) -> Unit)? = null

    private val trozos = HashMap<Int, Array<ByteArray?>>()
    private val listos = ArrayList<Audio>()

    /** Un paquete de bicho/{id}/audio. */
    @Synchronized
    fun recibir(p: ByteArray) {
        if (p.isEmpty()) return
        when (p[0].toInt()) {
            3 -> if (p.size >= 2) leeGlobos = (p[1].toInt() and 1) == 1
            2 -> trozo(p)
            else -> Unit  // 0/1: Opus del cubito (M7), todavía no
        }
    }

    /** El audio de ese texto (por su CRC), si llegó hace menos de 10 s; se usa una vez. */
    @Synchronized
    fun tomar(crc: Long): Audio? {
        limpiar()
        val a = listos.firstOrNull { it.crc == crc } ?: return null
        listos.remove(a)
        return a
    }

    private fun trozo(p: ByteArray) {
        if (p.size < CABECERA) return
        val id = u16(p, 1)
        val formato = p[3].toInt()
        val origen = p[4].toInt() and 0xFF
        val crc = u32(p, 5)
        val indice = p[9].toInt() and 0xFF
        val total = p[10].toInt() and 0xFF
        if (formato != 1 || total == 0 || indice >= total) return
        val partes = trozos.getOrPut(id) { arrayOfNulls(total) }
        if (partes.size != total) return
        partes[indice] = p.copyOfRange(CABECERA, p.size)
        if (partes.any { it == null }) return
        trozos.remove(id)
        val mp3 = ByteArray(partes.sumOf { it!!.size }).also { var i = 0; for (x in partes) { x!!.copyInto(it, i); i += x.size } }
        if (mp3.size > TOPE) return Log.w("Bicho", "voz de la nube: audio de más (${mp3.size} bytes)").let { }
        val a = Audio(crc, origen, mp3, ahora())
        limpiar()
        listos += a
        Log.i("Bicho", "voz de la nube: ${mp3.size / 1024} KB (origen $origen)")
        alLlegar?.invoke(a)
    }

    private fun limpiar() {
        val t = ahora()
        listos.removeAll { t - it.llego > GUARDAR_MS }
        if (trozos.size > 8) trozos.clear()
    }

    companion object {
        const val CABECERA = 11
        const val TOPE = 512 * 1024
        const val GUARDAR_MS = 10_000L

        fun crc(utf8: ByteArray) = CRC32().apply { update(utf8) }.value

        private fun u16(p: ByteArray, i: Int) = (p[i].toInt() and 0xFF) or ((p[i + 1].toInt() and 0xFF) shl 8)
        private fun u32(p: ByteArray, i: Int) =
            (u16(p, i).toLong()) or (u16(p, i + 2).toLong() shl 16)
    }
}
