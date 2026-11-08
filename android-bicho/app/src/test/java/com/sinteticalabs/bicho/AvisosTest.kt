package com.sinteticalabs.bicho

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class AvisosTest {
    @Test fun tipoSegunLaApp() {
        assertEquals(2, Avisos.tipo("com.mercadopago.wallet", null))
        assertEquals(3, Avisos.tipo("com.google.android.gm", null))
        assertEquals(4, Avisos.tipo("com.github.android", null))
        assertEquals(1, Avisos.tipo("com.whatsapp", "call"))
        assertEquals(0, Avisos.tipo("com.whatsapp", "msg"))
    }

    @Test fun montoDelTitulo() {
        assertEquals("$12.000", Avisos.monto("Recibiste $ 12.000 de Ana"))
        assertNull(Avisos.monto("Te pagaron"))
    }

    @Test fun musicaComoEl0x4B() {
        val c = Avisos.comandoMusica(true, "Ojalá", "Silvio")
        val t = "Ojalá".toByteArray()
        assertArrayEquals(byteArrayOf(0x4B, 0, 1, t.size.toByte()) + t + byteArrayOf(6) + "Silvio".toByteArray(), c)
        assertEquals(40, Avisos.comandoMusica(false, "x".repeat(60), "")[3].toInt())
    }
}
