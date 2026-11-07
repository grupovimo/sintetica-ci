package com.sinteticalabs.app

import com.sinteticalabs.app.bicho.Ajustes
import com.sinteticalabs.app.bicho.EditorConfig
import com.sinteticalabs.app.datos.Vinculado
import com.sinteticalabs.app.datos.Vinculados
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class EditorConfigTest {
    private val ejemplo = File("../../shared/formato/config-ejemplo.json").readText()

    @Test fun leeElEjemploDelDoc() {
        val a = EditorConfig.leer(ejemplo)
        assertEquals("Toto", a.nombre)
        assertEquals("#FF6A33", a.colorOjos)
        assertEquals(0.6f, a.energia)
        assertEquals("23:30", a.suenoDesde)
        assertEquals("08:00", a.suenoHasta)
    }

    @Test fun escribeSinPerderLoQueNoEdita() {
        val nueva = EditorConfig.escribir(ejemplo, EditorConfig.leer(ejemplo).copy(nombre = "Pochi", tamano = 1.2f, colorOjos = null, suenoDesde = null, suenoHasta = null))
        val o = Json.parseToJsonElement(nueva).jsonObject
        assertTrue("las expresiones propias siguen", o.containsKey("expresiones"))
        assertTrue("las reglas siguen", o.containsKey("reglas"))
        assertEquals("\"mascota\"", o["modo"].toString())
        val a = EditorConfig.leer(nueva)
        assertEquals("Pochi", a.nombre)
        assertEquals(1.2f, a.tamano, 0.001f)
        assertNull(a.colorOjos)
        assertNull(a.suenoDesde)
        assertFalse(o["cara"]!!.jsonObject.containsKey("color"))
    }

    @Test fun desdeUnaConfigVacia() {
        val nueva = EditorConfig.escribir("{\"version\":1}", Ajustes(nombre = "Toto", energia = 0.9f, suenoDesde = "22:00", suenoHasta = "07:30"))
        val a = EditorConfig.leer(nueva)
        assertEquals("Toto", a.nombre)
        assertEquals(0.9f, a.energia, 0.001f)
        assertEquals("22:00", a.suenoDesde)
    }

    @Test fun listaYRotar() {
        val nueva = EditorConfig.escribir(ejemplo, EditorConfig.leer(ejemplo).copy(lista = listOf("imagen:0", "cara:guino"), rotar = 15))
        val a = EditorConfig.leer(nueva)
        assertEquals(listOf("imagen:0", "cara:guino"), a.lista)
        assertEquals(15, a.rotar)
    }

    @Test fun horas() {
        assertTrue(EditorConfig.hhmmValido("23:30"))
        assertFalse(EditorConfig.hhmmValido("24:00"))
        assertFalse(EditorConfig.hhmmValido("7:30"))
    }

    @Test fun vinculadosIdaYVuelta() {
        val v = Vinculado("A1B2C3D4E5F6", "Toto", "001", ByteArray(16) { it.toByte() })
        val l = Vinculados.leer(Vinculados.escribir(listOf(v)))
        assertEquals(1, l.size)
        assertEquals("Toto", l[0].nombre)
        assertArrayEquals(v.clave, l[0].clave)
        assertEquals(emptyList<Vinculado>(), Vinculados.leer("basura"))
    }

    @Test fun personalidadYHorarioV2() {
        val nueva = EditorConfig.escribir("{\"version\":1,\"personalidad\":{\"sueno\":\"23:00-07:00\"}}",
            EditorConfig.conPreset(Ajustes(suenoDesde = "22:30", suenoHasta = "07:15"), "picaro"))
        val o = Json.parseToJsonElement(nueva).jsonObject
        assertEquals("2", o["version"].toString())
        assertFalse("el sueño de la v1 se va", o["personalidad"]!!.jsonObject.containsKey("sueno"))
        assertEquals("\"07:15\"", o["horario"]!!.jsonObject["despierta"].toString())
        val a = EditorConfig.leer(nueva)
        assertEquals("picaro", a.preset)
        assertEquals(0.95f, a.picardia, 0.001f)
        assertEquals("22:30", a.suenoDesde)
        // Una config v2 del ejemplo se lee.
        val v2 = EditorConfig.leer(File("../../shared/formato/config-ejemplo-v2.json").readText())
        assertEquals(0.8f, v2.sociabilidad, 0.001f)
        assertEquals("23:30", v2.suenoDesde)
    }
}
