package com.sinteticalabs.app.bicho

/** Qué cuadros se toman de un video corto (como web/src/lib/imagenes.ts). */
object Video {
    const val SEGUNDOS = 6
    const val MAX_CUADROS = 64

    private fun pasoExacto(duracionMs: Long): Double {
        val dura = minOf(duracionMs, SEGUNDOS * 1000L).toDouble()
        return maxOf(1000.0 / 12, dura / MAX_CUADROS)
    }

    /** Cada cuánto (ms): 12 por segundo, o menos si no entran en 64. */
    fun paso(duracionMs: Long): Long = Math.round(pasoExacto(duracionMs))

    /** Los momentos (ms) de cada cuadro, desde el principio. */
    fun tiempos(duracionMs: Long): List<Long> {
        val dura = minOf(duracionMs, SEGUNDOS * 1000L).toDouble()
        if (dura <= 0) return listOf(0L)
        val p = pasoExacto(duracionMs)
        val l = mutableListOf<Long>()
        var t = 0.0
        while (t < dura - 0.001 && l.size < MAX_CUADROS) { l += Math.round(t); t += p }
        return l
    }
}
