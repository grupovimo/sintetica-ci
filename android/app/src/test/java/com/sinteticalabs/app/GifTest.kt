package com.sinteticalabs.app

import com.sinteticalabs.app.bicho.Gif
import com.sinteticalabs.app.protocolo.Animacion
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class GifTest {
    /** Un GIF armado a mano, igual que en sim/test/sim.test.mjs (LZW sin comprimir, códigos de 3 bits). */
    private fun gifDePrueba(): ByteArray {
        val b = mutableListOf<Int>()
        "GIF89a".forEach { b += it.code }
        b += listOf(4, 0, 2, 0, 0x81, 0, 0)
        b += listOf(255, 0, 0, 0, 0, 255, 0, 255, 0, 0, 0, 0)
        fun cuadro(x: Int, w: Int, cs: Int, indices: List<Int>) {
            b += listOf(0x21, 0xf9, 4, 0, cs, 0, 0, 0)
            b += listOf(0x2c, x, 0, 0, 0, w, 0, 2, 0, 0, 2)
            val codigos = mutableListOf<Int>()
            indices.forEachIndexed { i, k -> if (i % 2 == 0) codigos += 4; codigos += k }
            codigos += 5
            val datos = mutableListOf<Int>()
            var acc = 0; var bits = 0
            for (c in codigos) { acc = acc or (c shl bits); bits += 3; while (bits >= 8) { datos += acc and 255; acc = acc shr 8; bits -= 8 } }
            if (bits > 0) datos += acc and 255
            b += datos.size; b += datos; b += 0
        }
        cuadro(0, 4, 5, List(8) { 0 })
        cuadro(2, 2, 0, listOf(1, 1, 2, 2))
        b += 0x3b
        return ByteArray(b.size) { b[it].toByte() }
    }

    @Test fun decodificaYCompone() {
        val g = Gif.decodificar(gifDePrueba())
        assertEquals(4, g.ancho); assertEquals(2, g.alto); assertEquals(2, g.cuadros.size)
        assertEquals(50, g.cuadros[0].ms)
        assertEquals(100, g.cuadros[1].ms)  // sin demora: 100 ms
        val rojo = 0xFFFF0000.toInt(); val azul = 0xFF0000FF.toInt(); val verde = 0xFF00FF00.toInt()
        assertEquals(rojo, g.cuadros[0].argb[7])
        assertEquals(rojo, g.cuadros[1].argb[0])  // lo de antes queda
        assertEquals(azul, g.cuadros[1].argb[2])
        assertEquals(verde, g.cuadros[1].argb[7])
    }

    @Test fun animacionIdaYVuelta() {
        val a = byteArrayOf(1, 2, 3); val b = byteArrayOf(4, 5)
        val sani = Animacion.codificar(listOf(Animacion.Cuadro(a, 70), Animacion.Cuadro(b, 5)), 320, 172)
        assertTrue(Animacion.esAnimacion(sani))
        val c = Animacion.cuadros(sani)
        assertArrayEquals(a, c[0].qoi); assertArrayEquals(b, c[1].qoi)
        assertEquals(70, c[0].ms); assertEquals(20, c[1].ms)  // nunca menos de 20 ms
        assertEquals(320, (sani[6].toInt() and 255) or ((sani[7].toInt() and 255) shl 8))
    }
}
