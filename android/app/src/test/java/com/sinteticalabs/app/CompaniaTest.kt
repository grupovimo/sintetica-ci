package com.sinteticalabs.app

import com.sinteticalabs.app.consola.AvisoRobot
import com.sinteticalabs.app.consola.Compania
import com.sinteticalabs.app.consola.avisosDeMarco
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.textoDeToque
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class CompaniaTest {
    @Test fun habitos() {
        var c = leerObjeto("{}")
        repeat(6) { c = Compania.sumarHabito(c) }
        val l = Compania.habitos(c)
        assertEquals(Compania.MAX_HABITOS, l.size)
        assertEquals(listOf("gota", "libro", "pesa", "zen"), l.map { it.icono }) // un ícono distinto cada uno
        assertEquals("""{"nombre":"Moverme","icono":"pesa","hora":"18:00","dias":"LMXJV"}""", c.lista("habitos")!![2].toString())
        c = Compania.cambiarHabito(c, 0) { Compania.alternarDia(it, "D") }
        assertEquals("LMXJVS", Compania.habitos(c)[0].dias)
        c = Compania.cambiarHabito(c, 0) { Compania.alternarDia(it, "D") }
        assertNull(Compania.habitos(c)[0].dias) // los siete: no se escribe
        assertEquals("L", Compania.alternarDia(Compania.Habito("x", dias = "L"), "L").dias) // nunca ninguno
        assertEquals("Agua", Compania.conNombre(Compania.habitos(c)[0], "   ").nombre)
        assertEquals(16, Compania.conNombre(Compania.Habito("x"), "é".repeat(20)).nombre.toByteArray().size)
        c = Compania.quitarHabito(Compania.quitarHabito(Compania.quitarHabito(Compania.quitarHabito(c, 0), 0), 0), 0)
        assertNull(c["habitos"])
        assertEquals("día", Compania.textoRacha(1))
        assertEquals("días", Compania.textoRacha(4))
    }

    @Test fun presencia() {
        assertEquals(Compania.Presencia(), Compania.presencia(leerObjeto("{}")))
        val c = Compania.ponerPresencia(leerObjeto("{}")) { it.copy(preguntar = false, texto = "  ¿Las llaves?  ") }
        assertEquals("""{"preguntar":false,"texto":"¿Las llaves?"}""", c.obj("presencia").toString())
        assertEquals("""{"preguntar":true}""", Compania.ponerPresencia(c) { it.copy(preguntar = true, texto = "") }.obj("presencia").toString())
    }

    @Test fun amigos() {
        assertEquals("ABCDEFGH", Compania.limpiarCodigo(" abcd-efgh "))
        val mios = listOf("A", "B")
        // Con todos, sacar uno deja el otro; con los dos de nuevo, vuelve a ser "todos" (null).
        assertEquals(listOf("B"), Compania.recibir(true, emptyList(), mios, "A"))
        assertNull(Compania.recibir(false, listOf("B"), mios, "A"))
        assertEquals(emptyList<String>(), Compania.recibir(true, emptyList(), mios, null))
        assertNull(Compania.recibir(false, emptyList(), mios, null))
    }

    @Test fun toques() {
        assertEquals("Ana te mandó un abrazo" to "🤗", textoDeToque(leerObjeto("""{"de":"Ana","tipo":"abrazo"}""")))
        assertEquals("Pipo te devolvió la carita" to "😊", textoDeToque(leerObjeto("""{"de":"Pipo","tipo":"carita","devuelto":true}""")))
        assertEquals("Un amigo te mandó un toque", textoDeToque(leerObjeto("{}")).first)
        // El registro: tipo 7 toque, 8 hábito.
        val b = byteArrayOf(0x9E.toByte(), 0, 2, 0, 0, 0, 1, 0, 39, 0, 0, 0, 1, 7, 1)
        assertEquals(listOf(AvisoRobot.Toque(1)), avisosDeMarco(2, b))
        assertEquals(listOf(AvisoRobot.Habito(2)), avisosDeMarco(2, byteArrayOf(0x9E.toByte(), 0, 1, 0, 0, 0, 1, 8, 2)))
    }

    @Test fun fichasDeHabla() {
        val f = leerObjeto("""{"frases":{"todos":["¡{de} te manda un toque!","{de} y {robot}"]}}""")
        assertEquals(listOf("{dueno}", "{robot}", "{clima}", "{de}"), Compania.fichas(f))
        assertEquals("¡{de} te manda un toque!", Compania.deFabrica(f))
        assertEquals(Compania.SIEMPRE, Compania.fichas(null))
        // Todas las situaciones agrupadas existen en las de fábrica de la web.
        assertTrue(Compania.GRUPOS_HABLA.flatMap { it.second }.contains("toque-recibido"))
    }

    @Test fun avisosConTipo() {
        assertEquals("pago", Compania.tipoAviso("com.mercadopago.wallet", false))
        assertEquals("mail", Compania.tipoAviso("com.google.android.gm", false))
        assertEquals("github", Compania.tipoAviso("com.github.android", false))
        assertEquals("pedido", Compania.tipoAviso("com.pedidosya", false))
        assertEquals("llamada", Compania.tipoAviso("com.whatsapp", true))
        assertEquals("notificacion", Compania.tipoAviso("com.whatsapp", false))
        assertEquals("$12.000" to "", Compania.camposAviso("pago", "Mercado Pago", "Recibiste $ 12.000 de Ana"))
        assertEquals("Mercado Pago" to "", Compania.camposAviso("pago", "Mercado Pago", "¡Te pagaron!"))
        assertEquals("Gmail" to "Ana", Compania.camposAviso("mail", "Gmail", "Ana"))
        assertEquals("PR #12" to "", Compania.camposAviso("github", "GitHub", "PR #12"))
        assertFalse(Compania.monto("sin plata") != null)
    }
}
