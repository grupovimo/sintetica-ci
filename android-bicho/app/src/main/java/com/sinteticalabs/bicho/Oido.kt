package com.sinteticalabs.bicho

import kotlin.math.log10
import kotlin.math.max
import kotlin.math.sqrt

/**
 * El oído: de cada tramo de 50 ms del micrófono solo se queda con el nivel
 * (RMS en dBFS) y decide si hubo voz, un ruido fuerte, música o un silencio
 * largo. No guarda audio: cada tramo se tira apenas se mide.
 *
 * - Ruido: un salto de más de 20 dB sobre el último segundo, bien fuerte.
 * - Voz: en los últimos 1,5 s, entre el 25 % y el 85 % de los tramos sobre el
 *   piso (la voz va y viene con las sílabas).
 * - Música: 8 s seguidos con casi todos los tramos sobre el piso.
 * - Silencio: 10 min sin nada de lo anterior.
 */
class Oido(private val tramoMs: Int = 50) {
    enum class Oyo(val nombre: String) { Voz("voz"), Ruido("ruido"), Musica("musica"), Silencio("silencio") }

    private val historia = ArrayDeque<Float>()
    private var piso = -60f
    private var ms = 0L
    private var ultimoRuido = -1_000_000L
    private var ultimaVoz = -1_000_000L
    private var ultimaMusica = -1_000_000L
    private var ultimoSonido = 0L
    private var avisoSilencio = false

    /** Nivel de 0 a 1 por encima del piso (para que la cara lo siga). */
    var nivel = 0f
        private set

    /** Mide un tramo y devuelve lo que oyó, si oyó algo. */
    fun tramo(muestras: ShortArray, n: Int = muestras.size): Oyo? = nivelDb(dbfs(muestras, n))

    /** Lo mismo, a partir del nivel ya medido (para los tests). */
    fun nivelDb(db: Float): Oyo? {
        ms += tramoMs
        val antes = if (historia.isEmpty()) db else historia.takeLast(SEGUNDO).average().toFloat()
        historia.addLast(db)
        while (historia.size > MUSICA) historia.removeFirst()
        // El piso baja rápido y sube muy lento: sigue el ruido de fondo de la pieza.
        piso = if (db < piso) piso + (db - piso) * 0.3f else piso + (db - piso) * 0.002f
        nivel = ((db - piso - 6f) / 30f).coerceIn(0f, 1f)

        val r = when {
            db > -25f && db - antes > 20f && ms - ultimoRuido > 3000 -> Oyo.Ruido.also { ultimoRuido = ms }
            esMusica() && ms - ultimaMusica > 60_000 -> Oyo.Musica.also { ultimaMusica = ms }
            esVoz() && ms - ultimaVoz > 8000 && ms - ultimaMusica > 20_000 -> Oyo.Voz.also { ultimaVoz = ms }
            else -> null
        }
        if (r != null || db > piso + 12f) {
            ultimoSonido = ms
            avisoSilencio = false
        } else if (!avisoSilencio && ms - ultimoSonido >= SILENCIO_MS) {
            avisoSilencio = true
            return Oyo.Silencio
        }
        return r
    }

    private fun sobrePiso(cuantos: Int): Float {
        if (historia.size < cuantos) return 0f
        return historia.takeLast(cuantos).count { it > piso + 10f && it > -50f } / cuantos.toFloat()
    }

    private fun esVoz() = sobrePiso(VOZ).let { it in 0.25f..0.85f }
    private fun esMusica() = sobrePiso(MUSICA) >= 0.92f

    companion object {
        private const val SEGUNDO = 20
        private const val VOZ = 30
        private const val MUSICA = 160
        const val SILENCIO_MS = 10 * 60 * 1000L

        fun dbfs(m: ShortArray, n: Int): Float {
            if (n <= 0) return -96f
            var s = 0.0
            for (i in 0 until n) s += m[i].toDouble() * m[i]
            val rms = sqrt(s / n) / 32768.0
            return max(-96.0, 20 * log10(max(rms, 1e-6))).toFloat()
        }
    }
}

/**
 * Para no gastar CPU reconociendo silencio: deja pasar los tramos solo cuando
 * hay sonido sobre el piso (y 1,5 s después), con el último segundo de antes
 * para no cortar el principio de la palabra.
 */
class Compuerta(
    private val antes: Int = 20,
    private val despues: Int = 30,
    private val sobrePiso: Float = 12f,
    /** Nunca abre por debajo de esto (en una pieza muy callada, un clic suelto no es voz). */
    private val minimo: Float = -68f,
    /** Tramos seguidos con sonido para abrir: una sílaba dura más que un golpecito. */
    private val seguidos: Int = 2,
) {
    private var fuertes = 0
    /** El ruido de fondo que estima (para la depuración). */
    val fondo get() = piso

    private val previos = ArrayDeque<ShortArray>()
    private var piso = -60f
    private var quedan = 0

    /** Lo que hay que pasar con este tramo (nada, él solo o los de antes y él). */
    fun tramo(t: ShortArray, db: Float): List<ShortArray> {
        // El piso es el ruido de fondo: baja más rápido de lo que sube, pero sin
        // irse al fondo por un tramo suelto muy callado.
        piso += (if (db < piso) 0.05f else 0.002f) * (db - piso)
        val fuerte = db > maxOf(piso + sobrePiso, minimo)
        fuertes = if (fuerte) fuertes + 1 else 0
        if (fuerte && (quedan > 0 || fuertes >= seguidos)) {
            val abria = quedan > 0
            quedan = despues
            if (abria) return listOf(t)
            val todo = previos.toList() + listOf(t)
            previos.clear()
            return todo
        }
        if (quedan > 0) {
            quedan--
            return listOf(t)
        }
        previos.addLast(t)
        if (previos.size > antes) previos.removeFirst()
        return emptyList()
    }
}
