package com.sinteticalabs.bicho

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CharlaTest {
    @Test fun palabraSinTildesNiMayusculas() {
        assertEquals("ramon", Charla.normalizar(" Ramón "))
        assertTrue(Charla.contienePalabra("hola ramon", "ramon"))
        assertFalse(Charla.contienePalabra("ramona", "ramon"))
        assertFalse(Charla.contienePalabra("[unk]", ""))
    }

    @Test fun laFraseSinLaPalabra() {
        assertEquals("qué hora es", Charla.sinPalabra("lucho qué hora es", "lucho"))
        assertEquals("", Charla.sinPalabra("lucho", "lucho"))
        assertEquals("qué hora es", Charla.sinPalabra("che lucho qué hora es", "lucho"))
        assertEquals("hola", Charla.sinPalabra("hola", "lucho"))
        val larga = Charla.sinPalabra("lucho " + "ñ".repeat(150), "lucho")
        assertEquals(200, larga.toByteArray().size)
        assertEquals("ñ".repeat(100), larga)
    }
}
