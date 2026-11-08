package com.sinteticalabs.bicho

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioManager
import android.media.AudioTrack
import android.os.Handler
import android.os.HandlerThread
import android.os.VibrationEffect
import android.os.Vibrator
import android.util.Log
import kotlin.math.PI
import kotlin.math.exp
import kotlin.math.sin
import kotlin.random.Random

/**
 * La voz de criatura: los sonidos del núcleo (Hal::sonido) sintetizados acá,
 * sin archivos. El tono depende de voz.tipo y el volumen de voz.volumen; el
 * despertador va por el canal de alarma y sube de 1 a 5.
 */
class Voz(contexto: Context) {
    private val hilo = HandlerThread("voz").apply { start() }
    private val cola = Handler(hilo.looper)
    private val vibrador = contexto.getSystemService(Context.VIBRATOR_SERVICE) as Vibrator?

    /** {volumen, micrófono, tipo, conversa}, como lo da [Nucleo.voz]; lo actualiza el hilo de GL. */
    @Volatile var config = intArrayOf(70, 60, 1, 0)

    /** Lo llama el núcleo (hilo de GL): se arma y suena en el hilo de la voz. */
    fun sonar(nombre: String) {
        val volumen = config[0] / 100f
        val tipo = config[2]
        cola.post {
            runCatching { tocar(nombre, volumen, tipo) }.onFailure { Log.w("Bicho", "voz: $nombre", it) }
        }
    }

    fun apagar() = hilo.quitSafely()

    private fun tocar(nombre: String, volumen: Float, tipo: Int) {
        Log.i("Bicho", "voz: $nombre")
        if (nombre.startsWith("despertador:")) {
            val nivel = nombre.substringAfter(':').toIntOrNull()?.coerceIn(1, 5) ?: 1
            reproducir(Sintesis.despertador(nivel), alarma = true, volumen = 1f)
            return
        }
        if (nombre == "ronroneo") vibrar(longArrayOf(0, 60, 40, 60, 40, 60, 40, 60))
        if (nombre == "llamada") vibrar(longArrayOf(0, 400, 200, 400))
        val pcm = Sintesis.sonido(nombre, Sintesis.base(tipo), tipo == 3) ?: return
        reproducir(pcm, alarma = false, volumen = volumen)
    }

    private fun vibrar(patron: LongArray) {
        val v = vibrador ?: return
        if (android.os.Build.VERSION.SDK_INT >= 26) v.vibrate(VibrationEffect.createWaveform(patron, -1))
        else @Suppress("DEPRECATION") v.vibrate(patron, -1)
    }

    private fun reproducir(pcm: ShortArray, alarma: Boolean, volumen: Float) {
        if (volumen <= 0f || pcm.isEmpty()) return
        val uso = if (alarma) AudioAttributes.USAGE_ALARM else AudioAttributes.USAGE_MEDIA
        val pista = AudioTrack.Builder()
            .setAudioAttributes(AudioAttributes.Builder().setUsage(uso)
                .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION).build())
            .setAudioFormat(AudioFormat.Builder().setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                .setSampleRate(Sintesis.TASA).setChannelMask(AudioFormat.CHANNEL_OUT_MONO).build())
            .setTransferMode(AudioTrack.MODE_STATIC)
            .setBufferSizeInBytes(pcm.size * 2)
            .build()
        pista.write(pcm, 0, pcm.size)
        pista.setVolume(volumen)
        pista.play()
        // Suena solo; se suelta cuando termina (el hilo de la voz espera).
        Thread.sleep(pcm.size * 1000L / Sintesis.TASA + 50)
        pista.release()
    }

    companion object {
        /** Los canales que usa el bicho, al máximo (el volumen lo pone él). */
        fun canales(audio: AudioManager) {
            for (c in intArrayOf(AudioManager.STREAM_MUSIC, AudioManager.STREAM_ALARM))
                runCatching { audio.setStreamVolume(c, audio.getStreamMaxVolume(c), 0) }
        }
    }
}

/** Los sonidos como notas: puro, para probarlo sin el celu. */
object Sintesis {
    const val TASA = 22050

    /** Una nota: de f0 a f1 (en veces la base), dura ms, después un silencio. */
    data class Nota(val f0: Float, val f1: Float, val ms: Int, val silencio: Int = 30, val fuerza: Float = 1f)

    /** El tono base según voz.tipo: grave, media, aguda, robot. */
    fun base(tipo: Int) = when (tipo) {
        0 -> 260f
        2 -> 620f
        else -> 400f
    }

    fun notas(nombre: String): List<Nota>? = when (nombre) {
        "llamar" -> listOf(Nota(1f, 1.5f, 120), Nota(1.2f, 1.8f, 150))
        "risa" -> List(4) { Nota(1.45f - it * 0.05f, 1.1f, 90, 40) }
        "tarareo" -> listOf(1f, 1.12f, 1.26f, 1.12f, 1f).map { Nota(it, it, 220, 20, 0.6f) }
        "hip" -> listOf(Nota(1.6f, 2f, 70))
        "festejo" -> listOf(Nota(1f, 1.5f, 100), Nota(1.25f, 1.9f, 100), Nota(1.5f, 2.2f, 180))
        "ayuda" -> listOf(Nota(1.3f, 0.9f, 300, 80), Nota(1.2f, 0.8f, 350))
        "achis" -> listOf(Nota(1.2f, 1.6f, 150, 60))
        "murmullo", "hablando" -> List(3) { Nota(0.7f + it * 0.06f, 0.65f, 110, 50, 0.45f) }
        "escuchando" -> listOf(Nota(1f, 1.3f, 90, 0, 0.5f))
        "pensando" -> listOf(Nota(1f, 1f, 80, 40, 0.5f), Nota(1.1f, 1.1f, 80, 0, 0.5f))
        "notificacion" -> listOf(Nota(1.5f, 1.5f, 90), Nota(2f, 2f, 140))
        "llamada" -> List(8) { Nota(if (it % 2 == 0) 1.5f else 2f, if (it % 2 == 0) 1.5f else 2f, 60, 10) }
        "recordatorio" -> listOf(Nota(1f, 1f, 150), Nota(1.26f, 1.26f, 150), Nota(1.5f, 1.5f, 250))
        "pomodoro" -> listOf(Nota(1.5f, 1.5f, 200), Nota(1f, 1f, 300))
        "ronroneo", "ronquido", "dado" -> emptyList()  // de ruido: ver sonido()
        else -> listOf(Nota(1f, 1.3f, 120))  // cualquier otro: un piquito
    }

    fun sonido(nombre: String, base: Float, robot: Boolean): ShortArray? {
        val pcm = when (nombre) {
            "ronroneo" -> ronroneo(base)
            "ronquido" -> ronquido()
            "dado" -> dado()
            else -> {
                val n = notas(nombre) ?: return null
                val partes = n.map { tono(it, base, robot) }
                val salida = FloatArray(partes.sumOf { it.size })
                var i = 0
                for (p in partes) { p.copyInto(salida, i); i += p.size }
                if (nombre == "achis") salida + ruido(180, 0.5f) else salida
            }
        }
        return aPcm(pcm)
    }

    /** Tres pitidos, más fuertes según el nivel (1 a 5). */
    fun despertador(nivel: Int): ShortArray {
        val f = 0.25f + 0.75f * (nivel - 1) / 4f
        val un = tono(Nota(2f, 2f, 120, 100, f), 440f, false)
        return aPcm(un + un + un)
    }

    private fun tono(n: Nota, base: Float, robot: Boolean): FloatArray {
        val muestras = n.ms * TASA / 1000
        val salida = FloatArray(muestras + n.silencio * TASA / 1000)
        var fase = 0.0
        for (i in 0 until muestras) {
            val t = i.toFloat() / muestras
            val f = base * (n.f0 + (n.f1 - n.f0) * t)
            fase += 2 * PI * f / TASA
            val s = sin(fase).toFloat()
            val onda = if (robot) (if (s >= 0) 0.6f else -0.6f) else s + 0.2f * sin(2 * fase).toFloat()
            salida[i] = onda * envolvente(i, muestras) * n.fuerza * 0.7f
        }
        return salida
    }

    /** Sube en 8 ms y baja en 25 ms: sin chasquidos. */
    private fun envolvente(i: Int, n: Int): Float {
        val sube = TASA * 8 / 1000
        val baja = TASA * 25 / 1000
        return minOf(1f, i.toFloat() / sube, (n - i).toFloat() / baja).coerceAtLeast(0f)
    }

    private fun ronroneo(base: Float): FloatArray {
        val n = TASA * 1200 / 1000
        var fase = 0.0
        return FloatArray(n) { i ->
            fase += 2 * PI * base * 0.25 / TASA
            val am = 0.5f + 0.5f * sin(2 * PI * 22 * i / TASA).toFloat()
            sin(fase).toFloat() * am * envolvente(i, n) * 0.35f
        }
    }

    private fun ronquido(): FloatArray {
        val n = TASA * 1500 / 1000
        var y = 0f
        return FloatArray(n) { i ->
            y += 0.04f * (Random.nextFloat() * 2 - 1 - y)  // ruido grave (pasabajos)
            val swell = sin(PI * i / n).toFloat()
            y * 6f * swell * 0.4f
        }
    }

    private fun dado(): FloatArray {
        val partes = mutableListOf<FloatArray>()
        var espera = 40
        repeat(7) {
            partes += ruido(25, 0.6f)
            partes += FloatArray(espera * TASA / 1000)
            espera += 25
        }
        val salida = FloatArray(partes.sumOf { it.size })
        var i = 0
        for (p in partes) { p.copyInto(salida, i); i += p.size }
        return salida
    }

    private fun ruido(ms: Int, fuerza: Float): FloatArray {
        val n = ms * TASA / 1000
        return FloatArray(n) { i -> (Random.nextFloat() * 2 - 1) * fuerza * exp(-4f * i / n) }
    }

    private fun aPcm(f: FloatArray) = ShortArray(f.size) { (f[it].coerceIn(-1f, 1f) * 32000).toInt().toShort() }
}
