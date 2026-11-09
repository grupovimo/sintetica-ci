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

    @Test fun laRespuestaDelNucleo() {
        val r = Charla.Respuesta.leer("1\talarma\talarma\t¡Listo! A las 7.".toByteArray())!!
        assertEquals(1, r.codigo)
        assertEquals("alarma", r.accion)
        assertEquals("¡Listo! A las 7.", r.frase)
        assertEquals(0, Charla.Respuesta.leer("0\t\t\tNo te entendí.".toByteArray())!!.codigo)
        assertEquals(null, Charla.Respuesta.leer("basura".toByteArray()))
        assertEquals(true, Charla.Respuesta.leer("1\talarma\t\t¿Te pongo una alarma a las 6?\t1".toByteArray())!!.pregunta)
    }

    @Test fun lasPalabrasDeLosPatrones() {
        val j = org.json.JSONObject("""{"intenciones":{"clima":{"patrones":["como esta el clima|dia","pone* alarma?"],"respuestas":{"todos":["Hace frío"]}}}}""")
        assertEquals(sortedSetOf("alarma", "clima", "como", "dia", "el", "esta", "pone"), Charla.palabrasDe(j))
    }

    @Test fun laConfianzaDeLaPalabra() {
        val j = """{"result":[{"conf":0.55,"word":"[unk]"},{"conf":0.92,"word":"coco"}],"text":"coco"}"""
        assertEquals(0.92f, Charla.confianzaDe(j, "coco"), 0.001f)
        assertEquals(-1f, Charla.confianzaDe("""{"text":""}""", "coco"), 0.001f)
    }
}
