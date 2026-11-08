package com.sinteticalabs.bicho

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.ByteArrayInputStream
import java.util.zip.CRC32

class PaqueteVozTest {
    private fun cabecera(nombre: String, largo: Int): ByteArray {
        val h = ByteArray(512)
        nombre.toByteArray().copyInto(h, 0)
        "%011o".format(largo).toByteArray().copyInto(h, 124)
        h[156] = '0'.code.toByte()
        "ustar".toByteArray().copyInto(h, 257)
        return h
    }

    private fun archivo(nombre: String, datos: ByteArray) =
        cabecera(nombre, datos.size) + datos + ByteArray((512 - datos.size % 512) % 512)

    @Test fun leeElTar() {
        val tar = archivo("indice.json", "{}".toByteArray()) + archivo("b34a676f.mp3", ByteArray(700) { 7 }) + ByteArray(1024)
        val vistos = mutableListOf<Pair<String, Int>>()
        Tar.leer(ByteArrayInputStream(tar)) { n, d -> vistos += n to d.size }
        assertEquals(listOf("indice.json" to 2, "b34a676f.mp3" to 700), vistos)
    }

    @Test fun elHashEsElDelContrato() {
        val c = CRC32().apply { update("¡Hola, Ana!".toByteArray()) }.value
        assertEquals("b34a676f", "%08x".format(c))
        assertEquals(c, VozNube.crc("¡Hola, Ana!".toByteArray()))
    }
}
