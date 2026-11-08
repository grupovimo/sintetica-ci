package com.sinteticalabs.app

import com.sinteticalabs.app.consola.Acompanar
import com.sinteticalabs.app.consola.CarasDeImagen
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.Recordatorio
import com.sinteticalabs.app.consola.Recordatorios
import com.sinteticalabs.app.consola.Reflejos
import com.sinteticalabs.app.consola.VACIO
import com.sinteticalabs.app.consola.cortarBytes
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.texto
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/**
 * Lo que la app valida de su lado contra shared/formato/paridad/paridad.json:
 * los topes son los del núcleo y los textos se cortan en bytes igual que él.
 */
class ParidadTest {
    private val p: JsonObject = Json.parseToJsonElement(File("../../shared/formato/paridad/paridad.json").readText()).jsonObject
    private fun limite(k: String) = p["limites"]!!.jsonObject[k]!!.jsonPrimitive.int

    private data class Corte(val texto: String, val max: Int, val resultado: String, val bytes: Int)
    private val listaCortes = p["cortes"]!!.jsonArray.map { it.jsonObject }.map {
        Corte(it["texto"]!!.jsonPrimitive.content, it["max"]!!.jsonPrimitive.int, it["resultado"]!!.jsonPrimitive.content, it["bytes"]!!.jsonPrimitive.int)
    }

    @Test fun topes() {
        assertEquals(limite("recordatorios"), Recordatorios.MAXIMO)
        assertEquals(limite("bytesRecordatorio"), Recordatorios.MAX_TEXTO)
        assertEquals(limite("frasesPropias"), Acompanar.MAX_FRASES)
        assertEquals(limite("bytesFrase"), Acompanar.MAX_TEXTO)
        assertEquals(limite("avisos"), Acompanar.MAX_AVISOS)
        assertEquals(limite("bytesDecirAviso"), Acompanar.MAX_DECIR_AVISO)
        assertEquals(limite("bytesAppAviso"), Acompanar.MAX_NOMBRE_AVISO)
        assertEquals(limite("bytesCiudad"), Acompanar.MAX_CIUDAD)
        assertEquals(limite("bytesPalabra"), Editar.MAX_PALABRA)
        assertEquals(limite("bytesNombre"), Editar.MAX_NOMBRE)
        assertEquals(limite("lista"), Editar.MAX_LISTA)
        assertEquals(limite("reglasUsuario"), Reflejos.MAX_REGLAS)
        assertEquals(limite("sprites"), CarasDeImagen.MAX_SPRITES)
        assertEquals(limite("sprites"), com.sinteticalabs.app.protocolo.MAX_SPRITES)
    }

    @Test fun cortes() {
        assertTrue(listaCortes.isNotEmpty())
        for (c in listaCortes) {
            val r = cortarBytes(c.texto, c.max)
            assertEquals("cortar \"${c.texto}\" a ${c.max}", c.resultado, r)
            assertEquals("bytes de \"${c.resultado}\"", c.bytes, largoBytes(r))
        }
    }

    /** Las funciones que cortan con su tope, con los cortes de ese tope. */
    @Test fun cortesConSuTope() {
        fun deTope(max: Int) = listaCortes.filter { it.max == max && it.texto == it.texto.trim() && !it.texto.contains("  ") }
        for (c in deTope(limite("bytesRecordatorio"))) {
            assertEquals(c.resultado.ifEmpty { null }, Recordatorios.conTexto(Recordatorio(hora = "08:00", texto = "x"), c.texto)?.texto)
        }
        for (c in deTope(limite("bytesDecirAviso"))) assertEquals(c.resultado, Acompanar.decirAviso(c.texto))
        for (c in deTope(limite("bytesAppAviso"))) assertEquals(c.resultado, Acompanar.nombreAviso(c.texto))
        for (c in deTope(limite("bytesFrase"))) {
            assertEquals(c.resultado, Acompanar.completar(c.texto, "", ""))
            val f = Acompanar.frases(Acompanar.sumarFrase(VACIO, "saludo", c.texto), "saludo")
            assertEquals(if (c.resultado.isEmpty()) emptyList() else listOf(c.resultado), f)
        }
        for (c in deTope(limite("bytesCiudad"))) {
            assertEquals(c.resultado, Acompanar.ponerUbicacion(VACIO, -34.6, -58.4, c.texto).obj("ubicacion")?.texto("ciudad"))
        }
        for (c in deTope(limite("bytesNombre"))) {
            assertEquals(c.resultado.ifEmpty { null }, Editar.ponerDueno(VACIO, c.texto).obj("dueno")?.texto("nombre"))
        }
    }
}
