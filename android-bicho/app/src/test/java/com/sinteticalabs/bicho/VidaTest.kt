package com.sinteticalabs.bicho

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class VidaTest {
    @Test fun avisoArmaElComando() {
        val c = Avisos.comando(0, "Telegram", "Lucho")
        assertArrayEquals(byteArrayOf(0x44, 0, 8) + "Telegram".toByteArray() + byteArrayOf(5) + "Lucho".toByteArray(), c)
    }

    @Test fun recortarNoParteUnaLetra() {
        val b = Avisos.recortar("a".repeat(23) + "ñandú")
        assertEquals(23, b.size)  // la ñ (2 bytes) no entra en el 24
        assertEquals(24, Avisos.recortar("x".repeat(40)).size)
    }

    @Test fun oscuridadSoloSiHabiaLuz() {
        val l = Sentidos.Luces()
        assertFalse(l.medir(1f, 0))           // de noche desde el principio: nada
        assertFalse(l.medir(80f, 1_000))
        assertFalse(l.medir(1f, 10_000))      // luz un ratito: tampoco
        assertFalse(l.medir(80f, 20_000))
        assertTrue(l.medir(1f, 60_000))       // luz más de 30 s y se apaga: sí
        assertFalse(l.medir(80f, 70_000))
        assertFalse(l.medir(1f, 200_000))     // otra vez antes de 10 min: no
    }

    @Test fun todosLosSonidosSuenan() {
        val nombres = listOf("llamar", "risa", "tarareo", "hip", "festejo", "ayuda", "achis", "ronroneo", "ronquido",
            "murmullo", "dado", "notificacion", "llamada", "recordatorio", "pomodoro", "escuchando", "pensando",
            "hablando", "uno-inventado")
        for (n in nombres) {
            val pcm = Sintesis.sonido(n, Sintesis.base(1), false)!!
            assertTrue(n, pcm.size > Sintesis.TASA / 20 && pcm.size < Sintesis.TASA * 3)
        }
        val bajo = Sintesis.despertador(1).maxOf { it.toInt() }
        val alto = Sintesis.despertador(5).maxOf { it.toInt() }
        assertTrue(alto > bajo * 2)
    }
}
