package com.sinteticalabs.app.protocolo

/**
 * Una imagen animada ("sani", ver firmware/nucleo/qoi.h): la cabecera, la tabla
 * de cuadros y cada cuadro en QOI, en el mismo archivo. Igual que
 * codificarAnimacion de sim/web/qoi.js.
 */
object Animacion {
    const val MAX_CUADROS = 64

    class Cuadro(val qoi: ByteArray, val ms: Int)

    fun codificar(cuadros: List<Cuadro>, ancho: Int, alto: Int): ByteArray {
        val tabla = 10 + 10 * cuadros.size
        val salida = ByteArray(tabla + cuadros.sumOf { it.qoi.size })
        fun u16(p: Int, v: Int) { salida[p] = v.toByte(); salida[p + 1] = (v shr 8).toByte() }
        fun u32(p: Int, v: Int) { for (k in 0..3) salida[p + k] = (v shr (8 * k)).toByte() }
        byteArrayOf(0x73, 0x61, 0x6e, 0x69, 1, cuadros.size.toByte()).copyInto(salida)
        u16(6, ancho); u16(8, alto)
        var off = tabla
        cuadros.forEachIndexed { i, c ->
            u16(10 + 10 * i, c.ms.coerceIn(20, 65535))
            u32(12 + 10 * i, off)
            u32(16 + 10 * i, c.qoi.size)
            c.qoi.copyInto(salida, off)
            off += c.qoi.size
        }
        return salida
    }

    fun esAnimacion(b: ByteArray) = b.size >= 10 && b[0] == 0x73.toByte() && b[1] == 0x61.toByte() && b[2] == 0x6e.toByte() && b[3] == 0x69.toByte()

    /** Los cuadros (una imagen suelta es un cuadro de 0 ms). */
    fun cuadros(b: ByteArray): List<Cuadro> {
        if (!esAnimacion(b)) return listOf(Cuadro(b, 0))
        fun u16(p: Int) = (b[p].toInt() and 255) or ((b[p + 1].toInt() and 255) shl 8)
        fun u32(p: Int) = (0..3).fold(0) { v, k -> v or ((b[p + k].toInt() and 255) shl (8 * k)) }
        return (0 until (b[5].toInt() and 255)).map { i ->
            val off = u32(12 + 10 * i)
            Cuadro(b.copyOfRange(off, off + u32(16 + 10 * i)), u16(10 + 10 * i))
        }
    }
}
