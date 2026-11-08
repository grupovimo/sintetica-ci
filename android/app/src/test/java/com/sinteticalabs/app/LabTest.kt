package com.sinteticalabs.app

import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.Memoria
import com.sinteticalabs.app.consola.Regla
import com.sinteticalabs.app.consola.Rostro
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.leerVida
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.nombreValido
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.texto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LabTest {
    private val base = mapOf(
        "contento" to leerObjeto("""{"ojo":{"smile":0.8,"h":0.2},"mcurve":0.5}"""),
        "mas-contento" to leerObjeto("""{"extiende":"contento","pulse":0.3}"""),
    )

    @Test fun deFabricaYExtiende() {
        val c = Rostro.deFabrica("contento", base)
        assertEquals(0.8, c["izq.smile"]!!, 1e-9)
        assertEquals(0.8, c["der.smile"]!!, 1e-9)
        assertEquals(0.5, c["mcurve"]!!, 1e-9)
        val m = Rostro.deFabrica("mas-contento", base)
        assertEquals(0.3, m["pulse"]!!, 1e-9)
        assertEquals(0.2, m["der.h"]!!, 1e-9)
        assertEquals(Rostro.neutral, Rostro.deFabrica("no-existe", base))
    }

    @Test fun guardaSoloLoQueCambio() {
        val ref = Rostro.deFabrica("contento", base)
        assertTrue(Rostro.documento(ref, ref).isEmpty())
        // Los dos ojos iguales: va en "ojo"; uno solo: en "izq".
        val dos = ref + mapOf("izq.w" to 0.15, "der.w" to 0.15)
        assertEquals("""{"ojo":{"w":0.15}}""", Rostro.documento(dos, ref).toString())
        val uno = ref + ("izq.tilt" to 0.2)
        assertEquals("""{"izq":{"tilt":0.2}}""", Rostro.documento(uno, ref).toString())
        val mirada = ref + ("gx" to 0.12345)
        assertEquals("""{"gx":0.123}""", Rostro.documento(mirada, ref).toString())
        // Y de vuelta: aplicar el documento da los mismos valores.
        val v = Rostro.aplicar(Rostro.documento(dos, ref), ref)
        assertEquals(0.15, v["der.w"]!!, 1e-9)
    }

    @Test fun nombres() {
        assertTrue(nombreValido("mi-cara-2"))
        assertFalse(nombreValido("Mi cara"))
        assertFalse(nombreValido("2caras"))
        assertFalse(nombreValido("a".repeat(25)))
        assertEquals("mi-cara-3", Lab.libre("mi-cara", listOf("mi-cara", "mi-cara-2")))
    }

    @Test fun paraDibujarSoloLasParametricas() {
        val c = leerObjeto("""{"version":2,"cara":{"color":"#4ADE80","tamano":1.2},"expresiones":{"mia":{"pulse":1},"dibujo":{"formas":[]}},"reglas":[]}""")
        val d = leerObjeto(Lab.paraDibujar(c))
        assertEquals("#4ADE80", d.obj("cara")!!.texto("color"))
        assertEquals(setOf("mia"), d.obj("expresiones")!!.keys)
        assertNull(d["reglas"])
        val sin = Lab.ponerExpresion(Lab.ponerExpresion(c, "mia", null), "dibujo", null)
        assertNull(sin["expresiones"]) // vacío: se saca
    }

    @Test fun pasosDeUnComportamiento() {
        var d = leerObjeto("""{"pasos":[{"cara":"contento","dura":1.5},{"cara":"guino","dura":[1,2]}]}""")
        assertEquals(3.0, Lab.total(d), 1e-9)
        assertEquals("contento", Lab.primeraCara(d))
        val (s, j) = Lab.sumarPaso(d, 0)
        assertEquals(1, j)
        assertEquals(listOf("contento", "contento", "guino"), Lab.pasos(s).map { it.texto("cara") })
        val (m, k) = Lab.moverPaso(d, 0, 1)
        assertEquals(1, k)
        assertEquals("guino", Lab.pasos(m)[0].texto("cara"))
        d = Lab.ponerEnPaso(d, 1, "dura", Lab.dura(0.3, 0.3))
        assertEquals("0.3", Lab.pasos(d)[1]["dura"].toString())
        assertEquals("[1.0,2.5]", Lab.dura(2.5, 1.0).toString())
        assertEquals(0.1 to 0.1, Lab.rango(Lab.dura(-3.0, -3.0)))
        val (q, _) = Lab.quitarPaso(Lab.quitarPaso(d, 0).first, 0)
        assertEquals(1, Lab.pasos(q).size) // siempre queda uno
        val a = Lab.ponerAfinidad(Lab.ponerAfinidad(d, "energia", 0.44), "energia", 0.0)
        assertNull(a["afinidad"])
        assertEquals(0.4, Lab.afinidad(Lab.ponerAfinidad(d, "energia", 0.44), "energia"), 1e-9)
    }

    @Test fun reglas() {
        assertEquals("estado" to "animo+0.2", Lab.partir("estado.animo+0.2"))
        assertEquals("reflejo" to "contento", Lab.partir("reflejo:contento"))
        assertEquals("estado.animo-0.2", Lab.unir("estado", "animo-0.2"))
        assertEquals("led:#ff6a33", Lab.unir("led", "#ff6a33"))
        val c = Regla.escribir(leerObjeto("{}"), listOf(Regla("tacto", "", listOf("reflejo:risa")), Regla("caricia", "afecto>0.5", listOf("led:#4ade80"))))
        assertEquals("""[{"si":"tacto","entonces":["reflejo:risa"]},{"si":"caricia","y":"afecto>0.5","entonces":["led:#4ade80"]}]""", c.lista("reglas").toString())
        assertEquals(2, Regla.leer(c).size)
        assertNull(Regla.escribir(c, emptyList())["reglas"])
    }

    @Test fun vidaEnVivo() {
        val b = mutableListOf<Byte>(0x9B.toByte(), 0)
        repeat(7) { b += (it * 40).toByte() }
        repeat(6) { b += 128.toByte() }
        b += 3 // dormido y soñando
        b += 5; b += "jugar".toByteArray().toList()
        b += 7; b += "neutral".toByteArray().toList()
        b += 1; b += 3; b += "dar".toByteArray().toList(); b += 80
        val v = leerVida(b.toByteArray())!!
        assertEquals(0.0, v.estado["energia"]!!, 1e-9)
        assertEquals(240 / 255.0, v.estado["estres"]!!, 1e-9)
        assertTrue(v.dormido && v.sonando)
        assertFalse(v.superposicion)
        assertEquals("jugar", v.comportamiento)
        assertEquals("neutral", v.visible)
        assertEquals(listOf("dar" to 0.8), v.candidatos)
        assertNull(leerVida(byteArrayOf(0x9B.toByte(), 1, 0)))
    }

    @Test fun memoria() {
        val h = (0 until 24).map { if (it == 20) 7 else if (it == 8) 3 else 0 }
        val m = Memoria.de(leerObjeto("""{"dias":12,"racha":3,"caricias":40,"toques":9,"horas":$h,"gustos":{"caricia":0.9,"voz":-0.2,"tacto":0.3},"hitos":["siete-dias"],"deriva":{"energia":0.1}}"""))
        assertEquals(12, m.dias)
        assertEquals(20, m.horaPreferida)
        assertEquals(listOf("caricia", "tacto"), m.mejoresGustos)
        assertEquals(listOf("siete-dias"), m.hitos)
        assertNull(Memoria.de(leerObjeto("{}")).horaPreferida)
    }

    // --- Caras de imagen ---

    @Test fun qoiConTransparencia() {
        // Opaca, da lo mismo que el QOI de 3 canales salvo el byte de canales.
        val w = 7; val h = 5
        val rgb = ByteArray(w * h * 3) { (it * 37 % 251).toByte() }
        val rgba = ByteArray(w * h * 4) { i -> if (i % 4 == 3) 255.toByte() else rgb[i / 4 * 3 + i % 4] }
        val tres = com.sinteticalabs.app.protocolo.Qoi.codificar(rgb, w, h)
        val cuatro = com.sinteticalabs.app.protocolo.Qoi.codificarRgba(rgba, w, h)
        assertEquals(4, cuatro[12].toInt())
        assertTrue(tres.copyOfRange(13, tres.size).contentEquals(cuatro.copyOfRange(13, cuatro.size)))
        // Con transparencia: aparece QOI_OP_RGBA (0xFF).
        val trans = ByteArray(4 * 4) { i -> if (i % 4 == 3) (i * 16).toByte() else 9 }
        val q = com.sinteticalabs.app.protocolo.Qoi.codificarRgba(trans, 2, 2)
        assertTrue(q.drop(14).any { it == 0xFF.toByte() })
    }

    @Test fun listaDeSprites() {
        val b = byteArrayOf(0xA6.toByte(), 0, 0x05, 0, 8, 10, 0, 0, 0, 1, 2, 3, 4, 10, 20, 0, 0, 0, 0, 0, 0, 0)
        val l = com.sinteticalabs.app.protocolo.leerListaSprites(b)
        assertEquals(listOf(0, 2), l.map { it.espacio })
        assertEquals(10L, l[0].largo)
        assertEquals(0x04030201L, l[0].crc)
    }

    @Test fun carasDeImagen() {
        val C = com.sinteticalabs.app.consola.CarasDeImagen
        var c = leerObjeto("""{"version":2,"expresiones":{"otra":{"sprites":{"ojo":0,"boca":1}},"param":{"pulse":1}}}""")
        assertEquals(setOf("otra"), C.deImagen(c).keys)
        assertEquals(setOf(0, 1), C.enUso(c))
        var d = C.nueva()
        assertEquals(2, C.libre(c, "", d))
        d = C.conParte(d, "ojo", 2)
        assertEquals(3, C.libre(c, "", d))
        d = C.conPerilla(d, C.PERILLAS[1], 0.3) // la separación de fábrica: no se escribe
        assertNull(d["sep"])
        d = C.conPerilla(d, C.PERILLAS[3], -0.2)
        assertEquals("-0.2", d["boca_y"].toString())
        // La vista previa: la cara como la de siempre.
        assertEquals("mia", C.siempre(C.vista(c, "mia", d)))
        assertEquals("", C.siempre(C.vista(c, "mia", C.nueva()))) // sin ojo, no
        // Guardar de siempre, renombrar y borrar.
        c = C.guardar(c, "", "mia", d, true)
        assertEquals("mia", C.siempre(c))
        assertEquals(setOf(0, 1, 2), C.enUso(c))
        c = C.guardar(c, "mia", "mia2", d, true)
        assertNull(c.obj("expresiones")!!["mia"])
        assertEquals("mia2", C.siempre(c))
        val sinSiempre = C.guardar(c, "mia2", "mia2", d, false)
        assertEquals("", C.siempre(sinSiempre))
        val borrada = C.borrar(c, "mia2")
        assertEquals("", C.siempre(borrada))
        assertEquals(setOf(2), C.sobrantes(c.obj("expresiones")!!.obj("mia2"), borrada))
        assertNull(borrada.obj("cara")) // sin nada adentro, se saca
    }
}
