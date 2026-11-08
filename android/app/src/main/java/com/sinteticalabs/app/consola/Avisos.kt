package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonObject

/** El robot cuenta los textos en bytes UTF-8 (una tilde ocupa 2): se corta sin partir una letra. */
fun cortarBytes(t: String, max: Int): String {
    val r = StringBuilder()
    var n = 0
    var i = 0
    while (i < t.length) {
        val cp = t.codePointAt(i)
        val letra = String(Character.toChars(cp))
        val largo = letra.toByteArray(Charsets.UTF_8).size
        if (n + largo > max) break
        r.append(letra)
        n += largo
        i += Character.charCount(cp)
    }
    return r.toString()
}

fun largoBytes(t: String) = t.toByteArray(Charsets.UTF_8).size

/**
 * Lo que el robot avisa solo, con un registro cortito (marco car 2:
 * [0x9E][0][n ≤ 2] y n entradas de 6 bytes [segundos u32][tipo][dato]):
 * un recordatorio (tipo 4, por su índice en la config) o el fin de una tanda
 * de pomodoro (evento 32). Igual que web/src/lib/bicho.svelte.ts.
 */
sealed interface AvisoRobot {
    data class Recordatorio(val indice: Int) : AvisoRobot
    data object Pomodoro : AvisoRobot
    /** v1.6: empezó a sonar el despertador (evento 33). */
    data object Despertador : AvisoRobot
    /** v1.7: terminó una partida (registro tipo 5: juego·16 + resultado: 0 perdiste, 1 ganaste, 2 empate, 3 sin resultado). */
    data class Juego(val juego: Int, val resultado: Int) : AvisoRobot
    /** v1.7: le contestaron el "¿cómo estás?" (registro tipo 6: 0 bien, 1 más o menos, 2 mal). */
    data class Animo(val valor: Int) : AvisoRobot
    /** v1.9: le llegó un toque (registro tipo 7: el tipo de toque). */
    data class Toque(val tipo: Int) : AvisoRobot
    /** v1.9: se marcó un hábito (registro tipo 8: su índice). */
    data class Habito(val indice: Int) : AvisoRobot
}

fun avisosDeMarco(car: Int, b: ByteArray): List<AvisoRobot> {
    if (car != 2 || b.size < 3 || (b[0].toInt() and 0xFF) != 0x9E || b[1].toInt() != 0 || (b[2].toInt() and 0xFF) > 2) return emptyList()
    val l = mutableListOf<AvisoRobot>()
    for (k in 0 until (b[2].toInt() and 0xFF)) {
        val i = 3 + k * 6
        if (i + 6 > b.size) break
        val tipo = b[i + 4].toInt() and 0xFF
        val dato = b[i + 5].toInt() and 0xFF
        when {
            tipo == 4 -> l += AvisoRobot.Recordatorio(dato)
            tipo == 5 -> l += AvisoRobot.Juego(dato shr 4, dato and 15)
            tipo == 6 -> l += AvisoRobot.Animo(dato)
            tipo == 7 -> l += AvisoRobot.Toque(dato)
            tipo == 8 -> l += AvisoRobot.Habito(dato)
            tipo == 0 && dato == 32 -> l += AvisoRobot.Pomodoro
            tipo == 0 && dato == 33 -> l += AvisoRobot.Despertador
        }
    }
    return l
}

/** Las respuestas que no son para mostrar (vida, memoria, registro). */
fun esRespuestaInterna(car: Int, b: ByteArray): Boolean {
    if (car != 2 || b.isEmpty()) return false
    val op = b[0].toInt() and 0xFF
    return op == 0x9A || op == 0x9B || op == 0x9C || op == 0x9E
}

/** Un aviso arriba de la consola: el de un recordatorio queda hasta que lo tocan (o 2 min); los demás, 5 s. */
data class Aviso(val texto: String, val icono: String, val cuando: Long, val despertador: Boolean = false) {
    val dura: Long get() = if (icono == ICONO_RECORDATORIO) 120_000 else 5_000
}

const val ICONO_RECORDATORIO = "⏰"
const val ICONO_IA = "✦"

/** El texto del aviso; null = no se muestra (el juego y el ánimo solo se cuentan). */
fun textoDeAviso(a: AvisoRobot, recordatorios: List<Recordatorio>, expresionActual: String, textoDespertador: String? = null): String? = when (a) {
    is AvisoRobot.Recordatorio -> recordatorios.getOrNull(a.indice)?.texto ?: "Recordatorio"
    AvisoRobot.Pomodoro -> if (expresionActual == "pomodoro:trabajo") "A trabajar" else "Tanda terminada"
    AvisoRobot.Despertador -> textoDespertador?.ifEmpty { null } ?: "Suena el despertador"
    is AvisoRobot.Juego, is AvisoRobot.Animo, is AvisoRobot.Toque, is AvisoRobot.Habito -> null
}

/** Lo que hizo una IA conectada por MCP: "Claude: poner cara". */
fun textoDeMcp(datos: JsonObject): String {
    val cliente = datos.texto("cliente").orEmpty().ifEmpty { "Tu IA" }
    return "$cliente: ${datos.texto("herramienta").orEmpty().replace('_', ' ')}"
}

/** Un evento en vivo de la cuenta (SSE /v1/eventos): {tipo, bicho, datos}. */
data class EventoCuenta(val tipo: String, val robot: String, val datos: JsonObject)

/** Lee un flujo SSE renglón por renglón ("event: x", "data: {json}"). */
class LectorSse {
    private var tipo = ""

    fun leer(linea: String): EventoCuenta? {
        if (linea.isEmpty()) { tipo = ""; return null }
        if (linea.startsWith(":")) return null
        val i = linea.indexOf(':')
        val campo = if (i < 0) linea else linea.substring(0, i)
        val valor = if (i < 0) "" else linea.substring(i + 1).removePrefix(" ")
        if (campo == "event") { tipo = valor; return null }
        if (campo != "data") return null
        val o = leerObjeto(valor)
        if (o.isEmpty()) return null
        val t = o.texto("tipo") ?: tipo
        tipo = ""
        return EventoCuenta(t, o.texto("bicho").orEmpty().uppercase(), o.obj("datos") ?: VACIO)
    }
}

/** Un toque que llegó (SSE `toque`: {de, tipo, devuelto?}): el texto y su ícono. */
fun textoDeToque(datos: JsonObject): Pair<String, String> {
    val t = datos.texto("tipo") ?: "toque"
    val de = datos.texto("de").orEmpty().ifEmpty { "Un amigo" }
    val icono = mapOf("toque" to "👉", "abrazo" to "🤗", "beso" to "💋", "carita" to "😊")[t] ?: "👉"
    return if (datos.si("devuelto") == true) {
        val d = mapOf("toque" to "el toque", "abrazo" to "el abrazo", "beso" to "el beso", "carita" to "la carita")[t] ?: "el toque"
        "$de te devolvió $d" to icono
    } else {
        val q = mapOf("toque" to "un toque", "abrazo" to "un abrazo", "beso" to "un beso", "carita" to "una carita")[t] ?: "un toque"
        "$de te mandó $q" to icono
    }
}
