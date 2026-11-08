package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** A la hora, el robot lo muestra en su pantalla hasta que lo tocás (config `recordatorios`). */
data class Recordatorio(
    val hora: String,
    val texto: String,
    val dias: String = "",          // "" = todos; si no, letras de LMXJVSD
    val expresion: String? = null,  // null = sorpresa
    val activo: Boolean? = null,    // null = activo
    val sonido: Boolean? = null,    // null = suena
)

object Recordatorios {
    const val MAXIMO = 12
    const val MAX_TEXTO = 40
    val DIAS = listOf("L" to "Lunes", "M" to "Martes", "X" to "Miércoles", "J" to "Jueves", "V" to "Viernes", "S" to "Sábado", "D" to "Domingo")
    val CARAS = listOf("sorpresa" to "Sorpresa", "contento" to "Contento", "pensando" to "Pensando", "guino" to "Guiño", "enamorado" to "Enamorado", "escuchando" to "Atento")

    fun leer(c: JsonObject): List<Recordatorio> = c.lista("recordatorios").orEmpty().mapNotNull { e ->
        val o = e as? JsonObject ?: return@mapNotNull null
        Recordatorio(
            hora = o.texto("hora") ?: return@mapNotNull null,
            texto = o.texto("texto").orEmpty(),
            dias = o.texto("dias").orEmpty(),
            expresion = o.texto("expresion"),
            activo = o.si("activo"),
            sonido = o.si("sonido"),
        )
    }

    fun aJson(r: Recordatorio): JsonObject = buildJsonObject {
        put("hora", r.hora)
        put("texto", r.texto)
        put("dias", r.dias)
        r.expresion?.let { put("expresion", it) }
        r.activo?.let { put("activo", it) }
        r.sonido?.let { put("sonido", it) }
    }

    /** Sin recordatorios, la clave se saca. */
    fun escribir(c: JsonObject, l: List<Recordatorio>): JsonObject =
        if (l.isEmpty()) c.sin("recordatorios") else c.con("recordatorios", JsonArray(l.take(MAXIMO).map { aJson(it) }))

    /** Uno nuevo a la hora que viene en punto: "Tomar agua". */
    fun nuevo(l: List<Recordatorio>, horaActual: Int): List<Recordatorio> =
        (l + Recordatorio(hora = "%02d:00".format((horaActual + 1) % 24), texto = "Tomar agua")).take(MAXIMO)

    fun tiene(r: Recordatorio, letra: String) = r.dias.isEmpty() || r.dias.contains(letra)

    /** Prende o apaga un día (vacío = todos: se arranca de todos; los siete vuelven a ser vacío). */
    fun alternarDia(r: Recordatorio, letra: String): Recordatorio {
        val d = r.dias.map { it.toString() }.toMutableSet()
        if (d.isEmpty()) DIAS.forEach { d += it.first }
        if (letra in d) d -= letra else d += letra
        val orden = DIAS.map { it.first }.filter { it in d }.joinToString("")
        return r.copy(dias = if (orden.length == 7) "" else orden)
    }

    /** Con el texto cortado a lo que entra (40 bytes); null si queda vacío. */
    fun conTexto(r: Recordatorio, t: String): Recordatorio? = cortarBytes(t.trim(), MAX_TEXTO).takeIf { it.isNotEmpty() }?.let { r.copy(texto = it) }

    /** En el orden del día, con su índice en la config. */
    fun ordenados(l: List<Recordatorio>): List<Pair<Int, Recordatorio>> = l.withIndex().sortedBy { it.value.hora }.map { it.index to it.value }
}
