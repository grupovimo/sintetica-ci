package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.intOrNull

/** El motor de vida en vivo: la respuesta al comando Vida ([0x9B][0]…), como leerVida de sim/web/protocolo.js. */
data class VidaEnVivo(
    val estado: Map<String, Double>,
    val rasgos: Map<String, Double>,
    val dormido: Boolean,
    val sonando: Boolean,
    val superposicion: Boolean,
    val comportamiento: String,
    val visible: String,
    val candidatos: List<Pair<String, Double>>,
)

fun leerVida(b: ByteArray): VidaEnVivo? {
    if (b.size < 2 + ESTADO_VIDA.size + Editar.RASGOS.size + 1) return null
    if ((b[0].toInt() and 0x7F) != 0x1B || b[1].toInt() != 0) return null
    var i = 2
    fun u8() = (b[i++].toInt() and 0xFF)
    val estado = ESTADO_VIDA.associateWith { u8() / 255.0 }
    val rasgos = Editar.RASGOS.associateWith { u8() / 255.0 }
    val flags = u8()
    fun campo(): String? {
        if (i >= b.size) return null
        val n = b[i].toInt() and 0xFF
        if (i + 1 + n > b.size) return null
        val s = String(b, i + 1, n, Charsets.UTF_8)
        i += 1 + n
        return s
    }
    val comportamiento = campo().orEmpty()
    val visible = campo().orEmpty()
    val candidatos = mutableListOf<Pair<String, Double>>()
    if (i < b.size) {
        val nc = u8()
        repeat(nc) {
            val n = campo() ?: return@repeat
            if (i >= b.size) return@repeat
            candidatos += n to (u8() / 100.0)
        }
    }
    return VidaEnVivo(estado, rasgos, flags and 1 != 0, flags and 2 != 0, flags and 4 != 0, comportamiento, visible, candidatos)
}

/** Lo que recuerda (GET /v1/bichos/{id}/memoria). */
data class Memoria(
    val dias: Int = 0, val racha: Int = 0, val caricias: Int = 0, val toques: Int = 0,
    val horas: List<Int> = List(24) { 0 },
    val gustos: Map<String, Double> = emptyMap(),
    val hitos: List<String> = emptyList(),
    val deriva: Map<String, Double> = emptyMap(),
) {
    /** La hora a la que más aparecés (si aparecés seguido). */
    val horaPreferida: Int? get() = horas.withIndex().filter { it.value >= 3 }.maxByOrNull { it.value }?.index

    /** Los tres gustos más fuertes. */
    val mejoresGustos: List<String> get() = gustos.filter { it.value > 0 }.entries.sortedByDescending { it.value }.take(3).map { it.key }

    companion object {
        fun de(o: JsonObject): Memoria {
            fun n(k: String) = o.numero(k)?.toInt() ?: 0
            fun mapa(k: String) = o.obj(k).orEmpty().mapValues { (it.value as? JsonPrimitive)?.doubleOrNull ?: 0.0 }
            val h = o.lista("horas").orEmpty().map { (it as? JsonPrimitive)?.intOrNull ?: 0 }
            return Memoria(n("dias"), n("racha"), n("caricias"), n("toques"), if (h.size == 24) h else List(24) { 0 }, mapa("gustos"), o.textos("hitos"), mapa("deriva"))
        }
    }
}

val NOMBRES_HITO = mapOf(
    "primer-dia" to "El primer día", "siete-dias" to "Una semana", "treinta-dias" to "Un mes",
    "cien-caricias" to "100 caricias", "cien-dias" to "100 días", "mil-caricias" to "1000 caricias",
)
