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
        assertEquals("che lucho", Charla.sinPalabra("che lucho", "lucho"))
    }
}
