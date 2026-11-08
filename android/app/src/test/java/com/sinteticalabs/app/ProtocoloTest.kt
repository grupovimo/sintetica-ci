package com.sinteticalabs.app

import com.sinteticalabs.app.protocolo.ArmadorDeConfig
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.Respuesta
import com.sinteticalabs.app.protocolo.Imagen
import com.sinteticalabs.app.protocolo.Manifiesto
import com.sinteticalabs.app.protocolo.Ota
import com.sinteticalabs.app.protocolo.leerProgresoOta
import com.sinteticalabs.app.protocolo.Qoi
import com.sinteticalabs.app.protocolo.crc32
import com.sinteticalabs.app.protocolo.escriturasDeImagen
import com.sinteticalabs.app.protocolo.deHex
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import com.sinteticalabs.app.protocolo.hex
import com.sinteticalabs.app.protocolo.idDeAnuncio
import com.sinteticalabs.app.protocolo.leerEstado
import com.sinteticalabs.app.protocolo.leerQr
import com.sinteticalabs.app.protocolo.leerRespuesta
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.float
import kotlinx.serialization.json.double
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** El protocolo de la app contra shared/protocolo/vectores.json (los que entiende el núcleo C++). */
class ProtocoloTest {
    private val v: JsonObject = Json.parseToJsonElement(File("../../shared/protocolo/vectores.json").readText()).jsonObject
    private fun s(o: JsonObject, k: String) = o[k]!!.jsonPrimitive.content

    @Test fun comandos() {
        val porNombre = v["comandos"]!!.jsonArray.associateBy({ s(it.jsonObject, "nombre") }, { it.jsonObject })
        assertEquals(s(porNombre["expresion enamorado"]!!, "hex"), hex(Comando.expresion("enamorado")))
        assertEquals(s(porNombre["expresion automatica"]!!, "hex"), hex(Comando.expresion()))
        assertEquals(s(porNombre["nombre Toto"]!!, "hex"), hex(Comando.nombre("Toto")))
        assertEquals(s(porNombre["nombre con tilde"]!!, "hex"), hex(Comando.nombre("Ñandú")))
        assertEquals(s(porNombre["evento sacudida"]!!, "hex"), hex(Comando.evento("sacudida")))
        assertEquals(s(porNombre["vincular"]!!, "hex"), hex(Comando.vincular("K7Q2XM")))
        assertEquals(s(porNombre["autenticar"]!!, "hex"), hex(Comando.autenticar(deHex(s(porNombre["autenticar"]!!, "clave")))))
        assertEquals(s(porNombre["desvincular"]!!, "hex"), hex(Comando.desvincular()))
        assertEquals(s(porNombre["pedir config"]!!, "hex"), hex(Comando.pedirConfig()))
        assertEquals(s(porNombre["imagen borrar 3"]!!, "hex"), hex(Imagen.borrar(3)))
        assertEquals(s(porNombre["imagen listar"]!!, "hex"), hex(Imagen.listar()))
        assertEquals(s(porNombre["imagen pedir 5"]!!, "hex"), hex(Imagen.pedir(5)))
        val red = porNombre["red"]!!
        assertEquals(s(red, "hex"), hex(Comando.red(s(red, "ssid"), s(red, "clave"))))
        assertEquals(s(porNombre["red abierta"]!!, "hex"), hex(Comando.red("Bar")))
        val nube = porNombre["nube"]!!
        assertEquals(s(nube, "hex"), hex(Comando.nube(s(nube, "url"), s(nube, "usuario"), s(nube, "clave"))))
        assertEquals(s(porNombre["olvidar red"]!!, "hex"), hex(Comando.olvidarRed()))
        assertEquals(s(porNombre["reproducir saludo"]!!, "hex"), hex(Comando.reproducir("saludo")))
        assertEquals(s(porNombre["reproducir parar"]!!, "hex"), hex(Comando.reproducir()))
        assertEquals(s(porNombre["ota cancelar"]!!, "hex"), hex(Ota.cancelar()))
        // v1.4
        assertEquals(s(porNombre["hora"]!!, "hex"), hex(Comando.hora(1793000000, -180)))
        assertEquals(s(porNombre["vida"]!!, "hex"), hex(Comando.vida()))
        assertEquals(s(porNombre["vida superposicion"]!!, "hex"), hex(Comando.vida(true)))
        assertEquals(s(porNombre["memoria"]!!, "hex"), hex(Comando.memoria()))
        assertEquals(s(porNombre["registro"]!!, "hex"), hex(Comando.registro()))
        assertEquals(s(porNombre["evento sacudida-fuerte"]!!, "hex"), hex(Comando.evento("sacudida-fuerte")))
        assertEquals(s(porNombre["comportamiento practicar-caras"]!!, "hex"), hex(Comando.comportamiento("practicar-caras")))
        // v1.6 y v1.7: el paquete de vida.
        val decir = porNombre["decir hola"]!!
        assertEquals(s(decir, "hex"), hex(Comando.decir(s(decir, "texto"))))
        val conCara = porNombre["decir con expresion"]!!
        assertEquals(s(conCara, "hex"), hex(Comando.decir(s(conCara, "texto"), s(conCara, "expresion"))))
        val clima = porNombre["clima"]!!
        assertEquals(s(clima, "hex"), hex(Comando.clima(clima["temp"]!!.jsonPrimitive.double, clima["codigo"]!!.jsonPrimitive.int,
            clima["max"]!!.jsonPrimitive.int, clima["min"]!!.jsonPrimitive.int, clima["dia"]!!.jsonPrimitive.boolean, s(clima, "ciudad"))))
        assertEquals(s(porNombre["alarma probar"]!!, "hex"), hex(Comando.alarma(2)))
        assertEquals(s(porNombre["alarma posponer"]!!, "hex"), hex(Comando.alarma(1)))
        assertEquals(s(porNombre["jugar dado"]!!, "hex"), hex(Comando.jugar("dado")))
        assertEquals(s(porNombre["jugar terminar"]!!, "hex"), hex(Comando.jugar("terminar")))
        val aviso = porNombre["aviso whatsapp"]!!
        assertEquals(s(aviso, "hex"), hex(Comando.aviso(s(aviso, "tipo") == "llamada", s(aviso, "app"), s(aviso, "contacto"))))
        assertEquals(s(porNombre["animo medio"]!!, "hex"), hex(Comando.animo(1)))
        val mirar = porNombre["mirar derecha"]!!
        assertEquals(s(mirar, "hex"), hex(Comando.mirar(mirar["x"]!!.jsonPrimitive.double, mirar["y"]!!.jsonPrimitive.double)))
        // v1.9: toques, agenda, presencia, hábitos, música y los avisos nuevos.
        fun b(o: JsonObject, k: String) = o[k]!!.jsonPrimitive.boolean
        fun n(o: JsonObject, k: String) = o[k]!!.jsonPrimitive.int
        for (k in listOf("toque abrazo", "toque carita")) {
            val t = porNombre[k]!!
            assertEquals(k, s(t, "hex"), hex(Comando.toque(s(t, "tipo"), s(t, "de"), s(t, "expresion"))))
        }
        val ag = porNombre["agenda"]!!
        assertEquals(s(ag, "hex"), hex(Comando.agenda(n(ag, "minutos"), n(ag, "duracion"), s(ag, "titulo"))))
        val man = porNombre["agenda manana"]!!
        assertEquals(s(man, "hex"), hex(Comando.agendaManana(n(man, "cantidad"), s(man, "titulo"))))
        for (k in listOf("presencia llegaste", "presencia te fuiste")) assertEquals(k, s(porNombre[k]!!, "hex"), hex(Comando.presencia(b(porNombre[k]!!, "llegaste"))))
        val hab = porNombre["habito hecho"]!!
        assertEquals(s(hab, "hex"), hex(Comando.habito(n(hab, "indice"), b(hab, "hecho"), n(hab, "racha"))))
        for (k in listOf("musica", "musica parar")) {
            val m = porNombre[k]!!
            assertEquals(k, s(m, "hex"), hex(Comando.musica(n(m, "bpm"), b(m, "sonando"), s(m, "titulo"), s(m, "artista"))))
        }
        for (k in listOf("aviso pago", "aviso github", "aviso whatsapp")) {
            val a = porNombre[k]!!
            assertEquals(k, s(a, "hex"), hex(Comando.aviso(s(a, "tipo"), s(a, "app"), s(a, "contacto"))))
        }
        assertEquals(44, com.sinteticalabs.app.protocolo.EVENTOS.indexOf("habito"))
        assertEquals(39, com.sinteticalabs.app.protocolo.EVENTOS.indexOf("toque-recibido"))
        // Los eventos nuevos van al final.
        assertEquals(33, com.sinteticalabs.app.protocolo.EVENTOS.indexOf("despertador"))
        assertEquals(35, com.sinteticalabs.app.protocolo.EVENTOS.indexOf("animo"))
    }

    @Test fun ota() {
        val o = v["ota"]!!.jsonObject
        val bin = deHex(s(o, "binario"))
        val m = Manifiesto(bin.size.toLong(), deHex(s(o, "sha256")), deHex(s(o, "firma")), s(o, "version"), s(o, "perfil"))
        assertEquals(s(o, "manifiesto"), hex(m.bytes()))
        assertEquals(s(o, "descargar"), hex(Ota.descargar(m, s(o, "url"))))
        val esperadas = o["escrituras"]!!.jsonArray.map { it.jsonPrimitive.content }
        assertEquals(esperadas, Ota.escrituras(m, bin, o["tamanoBloque"]!!.jsonPrimitive.int).map(::hex))
        assertEquals(s(o, "sha256"), hex(java.security.MessageDigest.getInstance("SHA-256").digest(bin)))
        val p = leerProgresoOta(byteArrayOf(0xB4.toByte(), 0, 40))!!
        assertEquals(40, p.porcentaje)
        assertEquals(true, p.ok)
    }

    @Test fun configEnBloques() {
        val c = v["config"]!!.jsonObject
        val esperadas = c["escrituras"]!!.jsonArray.map { it.jsonPrimitive.content }
        assertEquals(esperadas, escriturasDeConfig(s(c, "json"), c["tamanoBloque"]!!.jsonPrimitive.int).map(::hex))
    }

    @Test fun estados() {
        for (e in v["estados"]!!.jsonArray.map { it.jsonObject }) {
            val c = e["campos"]!!.jsonObject
            val est = leerEstado(deHex(s(e, "hex")))
            assertEquals(if (c["bateria"] is JsonNull) null else c["bateria"]!!.jsonPrimitive.int, est.bateria)
            assertEquals(c["humor"]!!.jsonPrimitive.float, est.humor, 0.011f)
            assertEquals(s(c, "modo"), est.modo)
            assertEquals(c["dormido"]!!.jsonPrimitive.boolean, est.dormido)
            assertEquals(c["fija"]!!.jsonPrimitive.boolean, est.fija)
            assertEquals(c["emparejando"]!!.jsonPrimitive.boolean, est.emparejando)
            assertEquals(c["autenticado"]!!.jsonPrimitive.boolean, est.autenticado)
            assertEquals(s(c, "expresion"), est.expresion)
            assertEquals(s(c, "nombre"), est.nombre)
            assertEquals(s(c, "serie"), est.serie)
            assertEquals(s(c, "id"), est.id)
            assertEquals(c["ancho"]!!.jsonPrimitive.int, est.pantalla.ancho)
            assertEquals(c["alto"]!!.jsonPrimitive.int, est.pantalla.alto)
            assertEquals(c["redonda"]!!.jsonPrimitive.boolean, est.pantalla.redonda)
            assertEquals(c["wifi"]!!.jsonPrimitive.boolean, est.wifi)
            assertEquals(c["nube"]!!.jsonPrimitive.boolean, est.nube)
            assertEquals(c["redConfigurada"]!!.jsonPrimitive.boolean, est.redConfigurada)
            assertEquals(c["actualizando"]!!.jsonPrimitive.boolean, est.actualizando)
            assertEquals(s(c, "version"), est.version)
            assertEquals(s(c, "perfil"), est.perfil)
            assertEquals(s(c, "crcConfigHex").toLong(16), est.crcConfig)
            assertEquals(c["encendido"]!!.jsonPrimitive.int.toLong(), est.encendido)
        }
    }

    @Test fun respuestas() {
        for (r in v["respuestas"]!!.jsonArray.map { it.jsonObject }) {
            val l = leerRespuesta(deHex(s(r, "hex"))) as Respuesta.Resultado
            assertEquals(r["op"]!!.jsonPrimitive.int, l.op)
            assertEquals(r["codigo"]!!.jsonPrimitive.int, l.codigo)
            r["clave"]?.let { assertArrayEquals(deHex(it.jsonPrimitive.content), l.clave) }
            r["detalle"]?.let { assertEquals(it.jsonPrimitive.content, l.detalle) }
        }
    }

    @Test fun qrYAnuncio() {
        for (q in v["qr"]!!.jsonArray.map { it.jsonObject }) {
            val l = leerQr(s(q, "texto"))
            if (q["id"] is JsonNull) assertNull(l) else {
                assertEquals(s(q, "id"), l!!.id)
                assertEquals(s(q, "codigo"), l.codigo)
            }
        }
        for (a in v["anuncios"]!!.jsonArray.map { it.jsonObject }) {
            val id = idDeAnuncio(deHex(s(a, "hex")))
            if (a["id"] is JsonNull) assertNull(id) else assertEquals(s(a, "id"), id)
        }
    }

    @Test fun qoiIgualAlDeReferencia() {
        val q = v["qoi"]!!.jsonObject
        val qoi = Qoi.codificar(deHex(s(q, "rgb")), q["ancho"]!!.jsonPrimitive.int, q["alto"]!!.jsonPrimitive.int)
        assertEquals(s(q, "hex"), hex(qoi))
        val esperadas = q["escrituras"]!!.jsonArray.map { it.jsonPrimitive.content }
        assertEquals(esperadas, escriturasDeImagen(q["espacio"]!!.jsonPrimitive.int, qoi, q["tamanoBloque"]!!.jsonPrimitive.int).map(::hex))
    }

    @Test fun crc() {
        for (c in v["crc32"]!!.jsonArray.map { it.jsonObject })
            assertEquals(s(c, "crc"), "%08x".format(crc32(s(c, "texto").toByteArray())))
    }

    @Test fun armadorDeConfig() {
        val texto = "{\"version\":1,\"nombre\":\"Ñandú\"}"
        val bytes = texto.toByteArray()
        val a = ArmadorDeConfig()
        assertNull(a.agregar(Respuesta.Trozo(0, bytes.copyOfRange(0, 10))))
        assertNull(a.agregar(Respuesta.Trozo(10, bytes.copyOfRange(10, bytes.size))))
        assertEquals(texto, a.agregar(Respuesta.Cierre(bytes.size, crc32(bytes))))
        assertTrue(runCatching { ArmadorDeConfig().agregar(Respuesta.Cierre(1, 0)) }.isFailure)
    }
}
