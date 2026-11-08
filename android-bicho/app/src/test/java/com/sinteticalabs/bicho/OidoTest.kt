package com.sinteticalabs.bicho

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class OidoTest {
    private fun oir(o: Oido, db: Float, tramos: Int) = (0 until tramos).mapNotNull { o.nivelDb(db) }

    @Test fun ruidoFuerteDespuesDeCalma() {
        val o = Oido()
        oir(o, -60f, 100)
        assertEquals(Oido.Oyo.Ruido, o.nivelDb(-10f))
    }

    @Test fun vozVaYViene() {
        val o = Oido()
        oir(o, -60f, 200)
        val oyo = (0 until 60).mapNotNull { o.nivelDb(if (it % 3 == 0) -60f else -35f) }
        assertTrue(oyo.toString(), Oido.Oyo.Voz in oyo)
        assertTrue(Oido.Oyo.Musica !in oyo)
    }

    @Test fun musicaSeguida() {
        val o = Oido()
        oir(o, -60f, 200)
        val oyo = oir(o, -30f, 200)
        assertTrue(oyo.toString(), Oido.Oyo.Musica in oyo)
    }

    @Test fun silencioLargoUnaSolaVez() {
        val o = Oido()
        val tramos = (Oido.SILENCIO_MS / 50).toInt() * 2
        val oyo = oir(o, -60f, tramos)
        assertEquals(listOf(Oido.Oyo.Silencio), oyo)
    }

    @Test fun dbfsDeUnTono() {
        val m = ShortArray(800) { if (it % 2 == 0) 16384 else -16384 }
        assertEquals(-6f, Oido.dbfs(m, m.size), 0.1f)
    }
}

class CompuertaTest {
    private val t = ShortArray(800)

    @Test fun enSilencioNoPasaNada() {
        val c = Compuerta()
        repeat(200) { org.junit.Assert.assertTrue(c.tramo(t, -60f).isEmpty()) }
    }

    @Test fun alHablarPasaConElSegundoDeAntes() {
        val c = Compuerta(antes = 20, despues = 30)
        repeat(100) { c.tramo(t, -70f) }
        org.junit.Assert.assertTrue(c.tramo(t, -30f).isEmpty())  // un golpecito solo no abre
        org.junit.Assert.assertEquals(21, c.tramo(t, -30f).size)
        org.junit.Assert.assertEquals(1, c.tramo(t, -30f).size)
        repeat(30) { org.junit.Assert.assertEquals(1, c.tramo(t, -70f).size) }  // la cola de 1,5 s
        org.junit.Assert.assertTrue(c.tramo(t, -70f).isEmpty())
    }

    @Test fun losClicsSueltosNoAbren() {
        val c = Compuerta()
        repeat(400) { i -> org.junit.Assert.assertTrue(c.tramo(t, if (i % 7 == 0) -40f else -76f).isEmpty()) }
    }
}
