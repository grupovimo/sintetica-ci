package com.sinteticalabs.app

import com.sinteticalabs.app.consola.Acompanar
import com.sinteticalabs.app.consola.AvisoRobot
import com.sinteticalabs.app.consola.avisosDeMarco
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.textoDeAviso
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class AcompanarTest {
    private fun aviso(vararg entradas: Pair<Int, Int>): ByteArray {
        val b = mutableListOf<Byte>(0x9E.toByte(), 0, entradas.size.toByte())
        for ((tipo, dato) in entradas) b += listOf<Byte>(0, 0, 0, 1, tipo.toByte(), dato.toByte())
        return b.toByteArray()
    }

    @Test fun avisosNuevos() {
        assertEquals(listOf(AvisoRobot.Despertador), avisosDeMarco(2, aviso(0 to 0x21)))
        // Juego: evento 34 + tipo 5 (juego·16 + resultado): el dado (3), ganaste (1).
        assertEquals(listOf(AvisoRobot.Juego(3, 1)), avisosDeMarco(2, aviso(0 to 0x22, 5 to 0x31)))
        assertEquals(listOf(AvisoRobot.Animo(2)), avisosDeMarco(2, aviso(0 to 0x23, 6 to 2)))
        assertEquals("Suena el despertador", textoDeAviso(AvisoRobot.Despertador, emptyList(), "", null))
        assertEquals("¡Arriba!", textoDeAviso(AvisoRobot.Despertador, emptyList(), "", "¡Arriba!"))
        assertNull(textoDeAviso(AvisoRobot.Juego(3, 1), emptyList(), ""))
    }

    @Test fun habla() {
        var c = leerObjeto("{}")
        assertEquals(0.5, Acompanar.frecuencia(c), 1e-9)
        assertEquals("Callado", Acompanar.tono(0.0))
        assertEquals("Charlatán", Acompanar.tono(0.9))
        c = Acompanar.sumarFrase(c, "despertar", "  ¡Arriba, {dueno}!  ")
        c = Acompanar.sumarFrase(c, "saludo", "Hola")
        assertEquals(listOf("¡Arriba, {dueno}!"), Acompanar.frases(c, "despertar"))
        assertEquals(2, Acompanar.totalFrases(c))
        c = Acompanar.ponerFrases(c, "saludo", emptyList())
        c = Acompanar.ponerFrases(c, "despertar", emptyList())
        assertNull(c.obj("habla")!!["propias"]) // sin frases, se saca
        repeat(15) { c = Acompanar.sumarFrase(c, "aburrido", "x$it") }
        assertEquals(Acompanar.MAX_FRASES, Acompanar.totalFrases(c))
        assertEquals("¡Arriba, Nahuel!", Acompanar.completar("¡Arriba,   {dueno}!", "Nahuel", "Lucho"))
        assertEquals(60, largoBytes(Acompanar.completar("é".repeat(40), "", "")))
    }

    @Test fun despertador() {
        var c = leerObjeto("{}")
        assertFalse(Acompanar.despertador(c).activo) // sin config, apagado
        c = Acompanar.ponerDespertador(c) { it.copy(hora = "06:45") }
        val d = Acompanar.despertador(c)
        assertTrue(d.activo)
        assertEquals("06:45", d.hora)
        assertEquals("""{"hora":"06:45","dias":"LMXJV","activo":true,"sonido":true,"posponer":9}""", c.obj("despertador").toString())
        c = Acompanar.ponerDespertador(c) { it.copy(dias = "LMXJVSD", texto = "¡Buen día!", posponer = 99) }
        assertNull(c.obj("despertador")!!["dias"]) // los siete: no se escribe
        assertEquals(30, Acompanar.despertador(c).posponer)
        assertEquals("LMXJVSD", Acompanar.despertador(c).dias)
        assertEquals("LMJV", Acompanar.alternarDia("LMXJV", "X"))
        assertEquals("L", Acompanar.alternarDia("L", "L")) // nunca ninguno
    }

    @Test fun ubicacionYCuidado() {
        var c = Acompanar.ponerUbicacion(leerObjeto("{}"), -31.41666, -64.183333, "Córdoba, Argentina")
        assertEquals(Acompanar.Ubicacion(-31.4167, -64.1833, "Córdoba, Argentina"), Acompanar.ubicacion(c))
        assertNull(Acompanar.ubicacion(leerObjeto("{}")))
        assertEquals(Acompanar.Cuidado(), Acompanar.cuidado(c))
        c = Acompanar.ponerCuidado(c) { it.copy(pausa = 15, agua = 2) }
        assertEquals(30, Acompanar.cuidado(c).pausa) // de "no" salta a 30
        assertEquals(2, Acompanar.cuidado(c).agua)
        assertEquals("09:00", Acompanar.cuidado(c).desde)
        val t = Acompanar.moverTrabajo(Acompanar.Cuidado(desde = "17:30", hasta = "18:00"), true, 30)
        assertEquals("17:30", t.desde) // no quedan iguales
    }

    @Test fun avisosDelCelu() {
        var c = Acompanar.nuevoAviso(leerObjeto("{}"))
        assertEquals("""[{"tipo":"notificacion","expresion":"sorpresa","decir":"{app}"}]""", c.lista("avisos").toString())
        val l = Acompanar.avisos(c).map { it.copy(app = "WhatsApp", contacto = "Mamá", sonido = false) }
        c = Acompanar.ponerAvisos(c, l)
        assertEquals("""{"tipo":"notificacion","app":"WhatsApp","contacto":"Mamá","expresion":"sorpresa","decir":"{app}","sonido":false}""", c.lista("avisos")!![0].toString())
        assertTrue(Acompanar.coincide(l, false, "WhatsApp", "Mamá"))
        assertTrue(Acompanar.coincide(l, false, "WhatsApp Business", "mamá"))
        assertFalse(Acompanar.coincide(l, true, "WhatsApp", "Mamá"))
        assertFalse(Acompanar.coincide(l, false, "Telegram", "Mamá"))
        assertTrue(Acompanar.coincide(listOf(Acompanar.AvisoCelu()), false, "Lo que sea", ""))
        assertEquals(24, largoBytes(Acompanar.nombreAviso("ñ".repeat(20))))
        assertEquals("{contacto} te escribe", Acompanar.decirAviso("  {contacto}   te escribe "))
        assertNull(Acompanar.ponerAvisos(c, emptyList())["avisos"])
    }
}
