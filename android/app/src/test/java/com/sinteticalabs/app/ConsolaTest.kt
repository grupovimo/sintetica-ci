package com.sinteticalabs.app

import com.sinteticalabs.app.bicho.Video
import com.sinteticalabs.app.consola.AvisoRobot
import com.sinteticalabs.app.consola.Automatizaciones
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.LectorSse
import com.sinteticalabs.app.consola.REDONDA
import com.sinteticalabs.app.consola.RECTANGULAR
import com.sinteticalabs.app.consola.Recordatorio
import com.sinteticalabs.app.consola.Recordatorios
import com.sinteticalabs.app.consola.Reflejos
import com.sinteticalabs.app.consola.avisosDeMarco
import com.sinteticalabs.app.consola.cortarBytes
import com.sinteticalabs.app.consola.esEstadoDelSistema
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.modeloDe
import com.sinteticalabs.app.consola.numeroDe
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.textoDeAviso
import com.sinteticalabs.app.consola.textoDeMcp
import com.sinteticalabs.app.consola.textos
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.consola.Capacidad
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ConsolaTest {
    // --- Textos contados en bytes UTF-8 (como el robot) ---

    @Test fun cortaEnBytesSinPartirLetras() {
        assertEquals("hola", cortarBytes("hola", 40))
        // "á" ocupa 2 bytes: con 3 entra "a" + "á", no la mitad de una tilde.
        assertEquals("aá", cortarBytes("aáé", 3))
        assertEquals("aáé", cortarBytes("aáé", 5))
        // Un emoji son 4 bytes (y dos chars en Java): entra entero o no entra.
        assertEquals("ok", cortarBytes("ok🙂", 5))
        assertEquals("ok🙂", cortarBytes("ok🙂", 6))
        val largo = "Tomá la pastilla del corazón, ¡ya mismo!"
        assertTrue(largoBytes(cortarBytes(largo, 40)) <= 40)
        assertEquals(40, largoBytes(cortarBytes("ñ".repeat(30), 40)))
    }

    // --- Los avisos del robot (marco car 2: 0x9E 0x00 n≤2) ---

    private fun registro(vararg entradas: Pair<Int, Int>): ByteArray {
        val b = mutableListOf<Byte>(0x9E.toByte(), 0, entradas.size.toByte())
        for ((tipo, dato) in entradas) b += listOf<Byte>(1, 2, 3, 4, tipo.toByte(), dato.toByte())
        return b.toByteArray()
    }

    @Test fun leeRecordatoriosYPomodoro() {
        assertEquals(listOf(AvisoRobot.Recordatorio(3)), avisosDeMarco(2, registro(4 to 3)))
        assertEquals(listOf(AvisoRobot.Recordatorio(0), AvisoRobot.Pomodoro), avisosDeMarco(2, registro(4 to 0, 0 to 32)))
        // Otro evento, otra característica, un registro largo (la respuesta al comando) o un error: nada.
        assertTrue(avisosDeMarco(2, registro(0 to 5)).isEmpty())
        assertTrue(avisosDeMarco(1, registro(4 to 3)).isEmpty())
        assertTrue(avisosDeMarco(2, registro(4 to 1, 4 to 2, 4 to 3)).isEmpty())
        assertTrue(avisosDeMarco(2, byteArrayOf(0x9E.toByte(), 1, 1)).isEmpty())
        // Si viene cortado, se lee lo que hay.
        assertEquals(listOf(AvisoRobot.Recordatorio(1)), avisosDeMarco(2, registro(4 to 1, 4 to 2).copyOf(9)))
    }

    @Test fun textoDelAviso() {
        val l = listOf(Recordatorio("08:00", "Agua"), Recordatorio("09:00", "Pastilla"))
        assertEquals("Pastilla", textoDeAviso(AvisoRobot.Recordatorio(1), l, ""))
        assertEquals("Recordatorio", textoDeAviso(AvisoRobot.Recordatorio(7), l, ""))
        assertEquals("A trabajar", textoDeAviso(AvisoRobot.Pomodoro, l, "pomodoro:trabajo"))
        assertEquals("Tanda terminada", textoDeAviso(AvisoRobot.Pomodoro, l, "pomodoro:descanso"))
        assertEquals("Claude: poner cara", textoDeMcp(leerObjeto("""{"cliente":"Claude","herramienta":"poner_cara"}""")))
        assertEquals("Tu IA: estado", textoDeMcp(leerObjeto("""{"herramienta":"estado"}""")))
    }

    @Test fun losEstadosDelSistemaNoSonCaras() {
        for (e in listOf("recordatorio:0", "pomodoro:trabajo", "reloj:", "imagen:3", "protector:1")) assertTrue(e, esEstadoDelSistema(e))
        for (e in listOf("contento", "", "neutral", "relojero")) assertFalse(e, esEstadoDelSistema(e))
    }

    // --- El SSE de la cuenta ---

    @Test fun leeElSse() {
        val l = LectorSse()
        assertNull(l.leer(": latido"))
        assertNull(l.leer("event: estado"))
        val e = l.leer("""data: {"tipo":"estado","bicho":"a1b2c3d4e5f6","datos":{"expresion":"contento","humor":0.4}}""")!!
        assertEquals("estado", e.tipo)
        assertEquals("A1B2C3D4E5F6", e.robot)
        assertEquals("contento", e.datos.texto("expresion"))
        assertNull(l.leer(""))
        // Sin "tipo" en el JSON, vale el del "event:".
        l.leer("event: mcp")
        val m = l.leer("""data:{"datos":{"herramienta":"x"}}""")!!
        assertEquals("mcp", m.tipo)
        assertEquals("", m.robot)
        assertNull(l.leer("data: no es json"))
    }

    // --- Recordatorios ---

    @Test fun armaRecordatorios() {
        var c = leerObjeto("""{"version":2}""")
        var l = Recordatorios.nuevo(Recordatorios.leer(c), 23)
        assertEquals(Recordatorio("00:00", "Tomar agua"), l.single())
        l = Recordatorios.nuevo(l, 7)
        c = Recordatorios.escribir(c, l)
        val json = c.lista("recordatorios")!!
        assertEquals(2, json.size)
        assertEquals("""{"hora":"00:00","texto":"Tomar agua","dias":""}""", json[0].toString())
        assertEquals(listOf("00:00", "08:00"), Recordatorios.leer(c).map { it.hora })
        // Vacío, se saca la clave.
        assertNull(Recordatorios.escribir(c, emptyList())["recordatorios"])
        // No pasa de 12.
        var muchos = emptyList<Recordatorio>()
        repeat(20) { muchos = Recordatorios.nuevo(muchos, it) }
        assertEquals(Recordatorios.MAXIMO, muchos.size)
    }

    @Test fun losDiasDeUnRecordatorio() {
        val r = Recordatorio("08:00", "x")
        assertTrue(Recordatorios.tiene(r, "D"))
        // Vacío = todos: sacar el domingo deja de lunes a sábado.
        val sinDomingo = Recordatorios.alternarDia(r, "D")
        assertEquals("LMXJVS", sinDomingo.dias)
        assertFalse(Recordatorios.tiene(sinDomingo, "D"))
        // Volver a ponerlo: los siete vuelven a ser "".
        assertEquals("", Recordatorios.alternarDia(sinDomingo, "D").dias)
        assertEquals("LX", Recordatorios.alternarDia(Recordatorio("08:00", "x", dias = "X"), "L").dias)
    }

    @Test fun elTextoDeUnRecordatorio() {
        val r = Recordatorio("08:00", "x")
        assertNull(Recordatorios.conTexto(r, "   "))
        assertEquals(40, largoBytes(Recordatorios.conTexto(r, "é".repeat(50))!!.texto))
        val con = Recordatorios.leer(leerObjeto("""{"recordatorios":[{"hora":"09:30","texto":"a","expresion":"guino","activo":false,"sonido":false},{"texto":"sin hora"}]}"""))
        assertEquals(listOf(Recordatorio("09:30", "a", "", "guino", false, false)), con)
        assertEquals(listOf(1, 0), Recordatorios.ordenados(listOf(Recordatorio("10:00", "b"), Recordatorio("07:00", "a"))).map { it.first })
    }

    // --- Automatizaciones ---

    @Test fun armaAutomatizaciones() {
        val a = Automatizaciones.nueva(1_700_000_000_000)
        assertEquals("a" + 1_700_000_000_000.toString(36), a.id)
        assertEquals("recordatorio", a.cuando)
        assertEquals("telegram", a.hacer.single().tipo)
        var b = Automatizaciones.sumarAccion(a, "webhook", null)
        b = Automatizaciones.sumarAccion(b, "robot", "B2")
        b = Automatizaciones.sumarAccion(b, "robot", null) // sin otro robot: no se suma
        assertEquals(listOf("telegram", "webhook", "robot"), b.hacer.map { it.tipo })
        b = Automatizaciones.sumarAccion(Automatizaciones.sumarAccion(b, "telegram", null), "telegram", null)
        assertEquals(Automatizaciones.MAX_ACCIONES, b.hacer.size)
        assertEquals(1, Automatizaciones.quitarAccion(a, 0).hacer.size) // siempre queda una

        val conCara = Automatizaciones.conCuando(a, "expresion").copy(valor = "guino")
        assertEquals("guino", conCara.valor)
        assertNull(Automatizaciones.conCuando(conCara, "duerme").valor)

        val cuerpo = Automatizaciones.cuerpo(listOf(conCara, Automatizaciones.sumarAccion(a, "robot", "B2")))
        val l = cuerpo.lista("automatizaciones")!!
        assertEquals("""{"id":"${a.id}","activa":true,"cuando":"expresion","valor":"guino","hacer":[{"tipo":"telegram","texto":"{robot}: {texto}"}]}""", l[0].toString())
        assertEquals("""{"tipo":"robot","bicho":"B2","expresion":"contento"}""", (l[1] as kotlinx.serialization.json.JsonObject).lista("hacer")!![1].toString())

        // Y de vuelta, como lo da la API.
        val i = Automatizaciones.leer(leerObjeto("""{"automatizaciones":${l},"historial":[{"evento":"recordatorio","accion":"telegram","ok":true,"en":"2026-11-08T10:00:00Z"}]}"""))
        assertEquals(2, i.lista.size)
        assertEquals("B2", i.lista[1].hacer[1].robot)
        assertTrue(i.historial.single().ok)
        assertEquals(Automatizaciones.CUANDOS_DE_FABRICA, i.cuandos)
    }

    // --- Encuadres de la foto ---

    @Test fun encuadres() {
        val f = RECTANGULAR.rectFoto(400f)
        assertEquals(-111.12f, f.x, 0.01f)
        assertEquals(-127.8f, f.y, 0.01f) // 25,56 % de 500
        assertEquals(622.24f, f.ancho, 0.01f)
        assertEquals(622.24f * 1.25f, f.alto, 0.01f)
        val p = RECTANGULAR.rectPantalla(400f)
        assertEquals(144.44f, p.x, 0.01f)
        assertEquals(116.65f, p.y, 0.01f)
        assertEquals(100f, p.ancho, 0.01f)
        assertEquals(213.9f, p.alto, 0.01f)
        // La rectangular está de pie: la cara apaisada se agranda (185 %) y se centra.
        val e = RECTANGULAR.encastre(400f, false, 320, 172)
        assertEquals(185f, e.ancho, 0.01f)
        assertEquals(185f * 172 / 320, e.alto, 0.01f)
        assertEquals((100f - 185f) / 2, e.x, 0.01f)
        // La redonda llena su caja.
        val r = REDONDA.encastre(400f, true, 240, 240)
        assertEquals(REDONDA.rectPantalla(400f).ancho, r.ancho, 0.01f)
        assertEquals(0f, r.x, 0f)
    }

    @Test fun modelos() {
        val broche = modeloDe("broche-c3", false, null)
        assertEquals("Broche", broche.nombre)
        assertTrue(broche.redonda)
        assertEquals(REDONDA, broche.encuadre)
        assertFalse(Capacidad.MICROFONO in broche.capacidades)
        val unidad = modeloDe("dev-147b", false, null)
        assertEquals("Unidad", unidad.nombre)
        assertEquals(320, unidad.pantalla.ancho)
        // Los virtuales traen todo, para probar cada módulo.
        assertEquals(Capacidad.entries.toSet(), modeloDe("virtual", true, null).capacidades)
        assertEquals("Nº 000", numeroDe(""))
        assertEquals("Nº " + (0xABCD % 1000).toString().padStart(3, '0'), numeroDe("A1B2C3D4ABCD"))
    }

    // --- Lo que cambian los módulos ---

    @Test fun ediciones() {
        var c = leerObjeto("""{"version":1,"personalidad":{"sueno":"23:00-07:30"},"reglas":[{"si":"tacto","y":"x","entonces":["a"]}]}""")
        c = Editar.aV2(c)
        assertEquals("07:30", Editar.despierta(c))
        assertEquals("23:00", Editar.duerme(c))
        assertNull(c.obj("personalidad")!!["sueno"])
        c = Editar.ponerPreset(c, "grunon")
        assertEquals(0.9, Editar.rasgo(c, "malGenio"), 1e-9)
        assertFalse(Editar.aMedida(c))
        c = Editar.ponerRasgo(c, "malGenio", 0.333333)
        assertEquals(0.33, Editar.rasgo(c, "malGenio"), 1e-9)
        assertTrue(Editar.aMedida(c))
        // El naranja es el de fábrica: no se escribe.
        assertNull(Editar.ponerColor(Editar.ponerColor(c, "#4ADE80"), "#ff6a33").obj("cara")!!["color"])
        // No se despierta a la hora en que se duerme.
        val d = Editar.moverHora(leerObjeto("""{"horario":{"despierta":"23:15","duerme":"23:30"}}"""), "despierta", 15)
        assertEquals("23:15", Editar.despierta(d))
        assertEquals("00:15", Editar.moverHora(leerObjeto("{}"), "duerme", 45).obj("horario")!!.texto("duerme"))
        assertEquals(1, Editar.tramos("08:00", "23:30").size)
        assertEquals(2, Editar.tramos("22:00", "06:00").size)
        assertEquals("02-29", Editar.mmdd(2, 31))
        assertNull(Editar.mmdd(0, 5))
    }

    @Test fun laListaYElProtector() {
        var c = leerObjeto("{}")
        c = Editar.mostrar(c, 2, true)
        assertEquals(listOf("cara:neutral", "imagen:2"), c.textos("lista"))
        c = Editar.ponerProtector(c, 2)
        c = Editar.moverEspera(c, 1)
        assertEquals(30, Editar.espera(c))
        c = Editar.olvidarImagen(c, 2)
        assertNull(c["lista"]) // vuelve a ser solo su cara: se saca
        assertNull(c["protector"])
        assertEquals("5 min", Editar.textoEspera(300))
    }

    @Test fun reflejos() {
        var c = leerObjeto("""{"reglas":[{"si":"tacto","y":"x","entonces":["reflejo:risa"]}]}""")
        c = Reflejos.conectar(c, "caricia", "enamorado")
        assertEquals(mapOf("caricia" to "enamorado"), Reflejos.cables(c))
        c = Reflejos.tocar(c, "caricia", "risa")
        assertEquals(mapOf("caricia" to "risa"), Reflejos.cables(c))
        c = Reflejos.tocar(c, "caricia", "risa") // el mismo: desconecta
        assertTrue(Reflejos.cables(c).isEmpty())
        assertEquals(1, c.lista("reglas")!!.size) // la regla compleja no se toca
    }

    @Test fun cuadrosDeVideo() {
        assertEquals(1000L / 12, Video.paso(2000))
        assertEquals(24, Video.tiempos(2000).size)
        // Diez segundos: se toman los primeros seis, en 64 cuadros como mucho.
        val t = Video.tiempos(10_000)
        assertTrue(t.size <= 64)
        assertTrue(t.last() < 6000)
        assertEquals(listOf(0L), Video.tiempos(0))
    }
}
