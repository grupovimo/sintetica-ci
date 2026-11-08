package com.sinteticalabs.bicho

import org.junit.Assert.assertEquals
import org.junit.Test

class VozTest {
    @Test fun leeSinEmojis() {
        assertEquals("¡Buen día!", Sintesis.paraLeer("¡Buen día! ☀️"))
        assertEquals("Hace 24 , soleado", Sintesis.paraLeer("Hace 24° , soleado 🌤"))
        assertEquals("", Sintesis.paraLeer("🎲✨"))
    }

    @Test fun tonoSegunTipo() {
        assert(Sintesis.tonoHabla(0).first < Sintesis.tonoHabla(1).first)
        assert(Sintesis.tonoHabla(2).first > Sintesis.tonoHabla(1).first)
    }
}
