package com.sinteticalabs.bicho

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class VozNubeTest {
    private var t = 0L
    private val n = VozNube { t }

    private fun paquete(id: Int, crc: Long, i: Int, total: Int, datos: ByteArray, origen: Int = 0) =
        byteArrayOf(2, id.toByte(), (id shr 8).toByte(), 1, origen.toByte(),
            crc.toByte(), (crc shr 8).toByte(), (crc shr 16).toByte(), (crc shr 24).toByte(),
            i.toByte(), total.toByte()) + datos

    @Test fun juntaLosTrozosYLosDaPorCrc() {
        val crc = VozNube.crc("¡Hola!".toByteArray())
        n.recibir(paquete(7, crc, 1, 2, byteArrayOf(3, 4)))
        assertNull(n.tomar(crc))
        n.recibir(paquete(7, crc, 0, 2, byteArrayOf(1, 2)))
        assertArrayEquals(byteArrayOf(1, 2, 3, 4), n.tomar(crc)!!.mp3)
        assertNull(n.tomar(crc))  // una sola vez
    }

    @Test fun venceALos10s() {
        n.recibir(paquete(1, 99, 0, 1, byteArrayOf(9)))
        t = 11_000
        assertNull(n.tomar(99))
    }

    @Test fun leeGlobos() {
        assertFalse(n.leeGlobos)
        n.recibir(byteArrayOf(3, 1))
        assertTrue(n.leeGlobos)
        n.recibir(byteArrayOf(3, 0))
        assertFalse(n.leeGlobos)
    }

    @Test fun ignoraOpus() {
        n.recibir(byteArrayOf(0, 1, 0, 5))
        n.recibir(byteArrayOf(1, 1, 0, 5))
    }

    @Test fun conversacion() {
        assertArrayEquals(shortArrayOf(1, -2), Conversaciones.pcm(byteArrayOf(1, 0, -2, -1)))
        assertEquals(100.toByte(), Conversaciones.cien(250.0))
        assertEquals((-100).toByte(), Conversaciones.cien(-101.0))
    }
}
