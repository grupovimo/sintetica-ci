package com.sinteticalabs.app

import com.sinteticalabs.app.consola.LinksWeb
import com.sinteticalabs.app.consola.Voz
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.datos.origenCharla
import com.sinteticalabs.app.datos.paqueteDe
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class VozTest {
    @Test fun topesEnEscalones() {
        assertEquals(3, Voz.escalon(Voz.LETRAS, 5000))
        assertEquals(3, Voz.escalon(Voz.LETRAS, 4800)) // el más cercano
        assertEquals(0, Voz.escalon(Voz.TOPES_IA, 0)) // 0 = sin tope
        assertEquals("1,5 k", Voz.miles(1500))
        assertEquals("200 k", Voz.miles(200000))
        assertEquals("950", Voz.miles(950))
        assertEquals("1,5", Voz.megas(1572864))
        assertEquals("1", Voz.megas(1048576))
    }

    @Test fun pestanas() {
        assertEquals("eleven", Voz.pestanaInicial(edge = true, elevenConectado = true, elevenLee = true))
        assertEquals("edge", Voz.pestanaInicial(edge = true, elevenConectado = true, elevenLee = false))
        assertEquals("eleven", Voz.pestanaInicial(edge = false, elevenConectado = true, elevenLee = false))
        assertEquals("eleven", Voz.pestanaInicial(edge = false, elevenConectado = false, elevenLee = false))
    }

    @Test fun paquete() {
        val p = paqueteDe(leerObjeto("""{"estado":"generando","version":2,"voz":"edge:es-AR-ElenaNeural","frases":40,"total":100,"bytes":1500000,
            "disponible":true,"proveedor":"edge","faltan":60,"gasta":0,"progreso":{"hechas":40,"de":100}}"""))
        assertEquals(40, p.hechas)
        assertFalse(Voz.hayQueGenerar(p.estado, true, p.faltan, p.voz, p.vozNueva)) // generando: no
        assertTrue(Voz.hayQueGenerar("listo", true, 0, "edge:a", "edge:b")) // cambió la voz
        assertTrue(Voz.hayQueGenerar("nuevo", false, 0, "", null))
        assertFalse(Voz.hayQueGenerar("listo", true, 0, "edge:a", "edge:a"))
    }

    @Test fun tuIA() {
        assertEquals("tuya", Voz.cualIA(propia = true, servidor = true))
        assertEquals("servidor", Voz.cualIA(propia = false, servidor = true))
        assertEquals("ninguna", Voz.cualIA(propia = false, servidor = false))
        assertEquals("Groq", Voz.servicioDe("compatible", "api.groq.com"))
        assertEquals("Anthropic", Voz.servicioDe("anthropic", ""))
        assertEquals("""{"proveedor":"compatible","clave":"k","modelo":"m","url":"https://x/v1"}""", Voz.cuerpoIA("compatible", " k ", "m", "https://x/v1").toString())
        assertEquals("""{"proveedor":"anthropic","clave":"k"}""", Voz.cuerpoIA("anthropic", "k", "", "").toString())
        assertFalse(Voz.puedeConectar("compatible", "k", "", "https://x"))
        assertTrue(Voz.puedeConectar("anthropic", "k", "", ""))
    }

    @Test fun charla() {
        assertEquals("motor", origenCharla("motor", false, 10))
        assertEquals("motor", origenCharla("nucleo-saludo", true, 10))
        assertEquals("ia", origenCharla("ia", false, -1))
        // Sin origen: sin tope y sin IA propia es el motor.
        assertEquals("motor", origenCharla(null, false, -1))
        assertEquals("ia", origenCharla(null, true, -1))
        assertEquals("ia", origenCharla(null, false, 30))
    }

    @Test fun herramientasDeLaConversacion() {
        assertEquals(0.5 to -1.0, Voz.mirada(50, -150))
        assertEquals(0.25 to 0.0, Voz.mirada("25", null))
        assertEquals("contento", Voz.expresion("contento", listOf("contento", "triste")))
        assertNull(Voz.expresion("bailar", listOf("contento")))
        assertEquals("cualquiera", Voz.expresion("cualquiera", emptyList()))
        assertNull(Voz.expresion("  ", emptyList()))
        assertEquals("1:05", Voz.mmss(65))
        assertEquals("0:00", Voz.mmss(-3))
    }

    @Test fun linksDeLaWeb() {
        assertEquals("amigo" to "ABCDEFGH", LinksWeb.leer("https", "sintetica.grupovimo.com.ar", "/robots", "abcd-efgh", null))
        assertEquals("spotify" to "ok", LinksWeb.leer("https", "sintetica.grupovimo.com.ar", "/robots", null, "ok"))
        assertNull(LinksWeb.leer("https", "otro.com", "/robots", "ABCDEFGH", null))
        assertNull(LinksWeb.leer("https", "sintetica.grupovimo.com.ar", "/robots", "ABC", null))
        assertEquals("Spotify conectado", LinksWeb.textoSpotify("ok"))
    }
}
