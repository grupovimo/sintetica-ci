package com.sinteticalabs.bicho

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioManager
import android.media.AudioTrack
import android.media.MediaPlayer
import android.os.Handler
import android.os.HandlerThread
import android.os.Bundle
import android.os.SystemClock
import android.os.VibrationEffect
import android.os.Vibrator
import android.speech.tts.TextToSpeech
import android.util.Log
import java.util.Locale
import kotlin.math.PI
import kotlin.math.exp
import kotlin.math.sin
import kotlin.random.Random

/**
 * La voz de criatura: los sonidos del núcleo (Hal::sonido) sintetizados acá,
 * sin archivos. El tono depende de voz.tipo y el volumen de voz.volumen; el
 * despertador va por el canal de alarma y sube de 1 a 5. Lo que dice el globo
 * lo lee en voz alta el TTS del celu (sin red: si no hay voz en español, calla),
 * salvo que la nube mande su audio (ElevenLabs, ver [VozNube]): entonces suena ese.
 */
class Voz(private val contexto: Context) {
    private val hilo = HandlerThread("voz").apply { start() }
    private val cola = Handler(hilo.looper)
    // El globo (TTS o el mp3 de la nube) va aparte: los sonidos ocupan su hilo mientras suenan.
    private val hiloHabla = HandlerThread("habla").apply { start() }
    private val colaHabla = Handler(hiloHabla.looper)
    val nube = VozNube().also { n -> n.alLlegar = { a -> colaHabla.post { audioDeLaNube(a) } } }

    /**
     * La voz propia del núcleo (el balbuceo, firmware/nucleo/voz_propia): si
     * está prendida, reemplaza al TTS del celu como última opción; un mp3 del
     * paquete o de la nube la corta. Se elige por adb (tipo voz-propia).
     */
    @Volatile var conPropia = false
    private var pistaPropia: AudioTrack? = null
    @Volatile private var mp3Hasta = 0L

    /** Lo llama el núcleo (hilo de GL) con cada bloque de la voz propia. */
    fun propia(m: ShortArray) {
        if (!conPropia || callada || config[0] <= 0 || SystemClock.elapsedRealtime() < mp3Hasta) return
        val p = pistaPropia ?: AudioTrack.Builder()
            .setAudioAttributes(AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA)
                .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build())
            .setAudioFormat(AudioFormat.Builder().setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                .setSampleRate(16000).setChannelMask(AudioFormat.CHANNEL_OUT_MONO).build())
            .setTransferMode(AudioTrack.MODE_STREAM)
            .setBufferSizeInBytes(16000 * 2)
            .build().also { it.play(); pistaPropia = it }
        p.setVolume(config[0] / 100f)
        p.write(m, 0, m.size, AudioTrack.WRITE_NON_BLOCKING)
        ocupar(m.size * 1000L / 16000)
    }

    private fun cortarPropia() {
        val p = pistaPropia ?: return
        runCatching { p.pause(); p.flush(); p.play() }
    }

    /** El paquete de voz sin internet (lo pone [Bicho]). */
    @Volatile var paquete: PaqueteVoz? = null

    /** Mientras hay una conversación en tiempo real, el globo no se lee (habla el agente). */
    @Volatile var callada = false
    private val vibrador = contexto.getSystemService(Context.VIBRATOR_SERVICE) as Vibrator?

    /** {volumen, micrófono, tipo, conversa}, como lo da [Nucleo.voz]; lo actualiza el hilo de GL. */
    @Volatile var config = intArrayOf(70, 60, 1, 0)

    @Volatile private var habla: TextToSpeech? = null
    @Volatile private var hablaLista = false
    private var ultimoGlobo = ""
    // Hasta cuándo suena algo suyo (el oído no se escucha a sí mismo).
    @Volatile private var ocupadaHasta = 0L

    /** Si está hablando o sonando (más un margen por el eco de la pieza). */
    fun ocupada() = SystemClock.elapsedRealtime() < ocupadaHasta || habla?.isSpeaking == true

    private fun ocupar(ms: Long) {
        ocupadaHasta = maxOf(ocupadaHasta, SystemClock.elapsedRealtime() + ms + ECO_MS)
    }

    /** Cuándo apareció el último globo (la charla mira si le contestaron). */
    @Volatile var ultimoGloboEn = 0L
        private set
    // Del hilo de la habla: el globo que se ve, si ya se dijo y la espera del audio de la nube.
    private var globoCrc = -1L
    private var globoLimpio = ""
    private var globoDicho = true
    private var reproductor: MediaPlayer? = null

    init {
        habla = TextToSpeech(contexto.applicationContext) { estado ->
            // Puede llegar antes de que `habla` quede asignado: se mira en el hilo de la voz.
            cola.post { alIniciar(estado) }
        }
    }

    private fun alIniciar(estado: Int) {
        val h = habla ?: return
        if (estado != TextToSpeech.SUCCESS) return Log.w("Bicho", "habla: el TTS no arrancó ($estado)").let { }
        // El primer español que tenga instalado.
        val idioma = ESPANOLES.firstOrNull { h.isLanguageAvailable(it) >= TextToSpeech.LANG_AVAILABLE }
        hablaLista = idioma != null && h.setLanguage(idioma) >= TextToSpeech.LANG_AVAILABLE
        Log.i("Bicho", "habla: ${if (hablaLista) "lista ($idioma)" else "sin voz en español"}; voces: ${
            runCatching { h.voices?.map { it.locale }?.distinct()?.joinToString() }.getOrNull()
        }")
    }

    /** Lo llama el hilo de GL con el texto del globo: lo lee cuando aparece uno nuevo. */
    fun globo(utf8: ByteArray) {
        val texto = String(utf8, Charsets.UTF_8)
        if (texto == ultimoGlobo) return
        ultimoGlobo = texto
        if (texto.isNotEmpty()) ultimoGloboEn = SystemClock.elapsedRealtime()
        colaHabla.post { globoNuevo(utf8, texto) }
    }

    private fun globoNuevo(utf8: ByteArray, texto: String) {
        colaHabla.removeCallbacks(esperaNube)
        globoCrc = if (texto.isEmpty()) -1L else VozNube.crc(utf8)
        globoLimpio = Sintesis.paraLeer(texto)
        globoDicho = texto.isEmpty() || callada
        if (globoDicho) return
        // Primero el paquete del celu; después el audio que mandó la nube (llega justo antes del Decir).
        paquete?.mp3(globoCrc)?.let {
            Log.i("Bicho", "voz: del paquete")
            return reproducirMp3(it)
        }
        nube.tomar(globoCrc)?.let { return reproducirMp3(it) }
        // Los recordatorios y el despertador: la nube manda el suyo ~1 s después.
        if (nube.leeGlobos) colaHabla.postDelayed(esperaNube, ESPERA_NUBE_MS) else decirLocal()
    }

    private val esperaNube = Runnable { decirLocal() }

    private fun audioDeLaNube(a: VozNube.Audio) {
        if (globoDicho || a.crc != globoCrc) return  // el que corresponde lo toma globoNuevo
        colaHabla.removeCallbacks(esperaNube)
        nube.tomar(a.crc)?.let(::reproducirMp3)
    }

    private fun reproducirMp3(a: VozNube.Audio) {
        if (config[0] > 0) reproducirMp3(java.io.File(contexto.cacheDir, "globo.mp3").apply { writeBytes(a.mp3) })
        globoDicho = true
    }

    private fun reproducirMp3(archivo: java.io.File) {
        globoDicho = true
        if (config[0] <= 0) return
        habla?.stop()
        reproductor?.release()
        cortarPropia()
        reproductor = runCatching {
            MediaPlayer().apply {
                setAudioAttributes(AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build())
                setDataSource(archivo.path)
                prepare()
                setVolume(config[0] / 100f, config[0] / 100f)
                setOnCompletionListener { it.release(); if (reproductor === it) reproductor = null }
                start()
                ocupar(duration.toLong())
                mp3Hasta = SystemClock.elapsedRealtime() + duration
            }
        }.onFailure { Log.w("Bicho", "voz de la nube: no sonó", it) }.getOrNull()
    }

    private fun decirLocal() {
        if (globoDicho) return
        globoDicho = true
        if (conPropia) return  // lo dice la voz propia del núcleo
        val limpio = globoLimpio
        val h = habla ?: return
        if (limpio.isEmpty() || !hablaLista || config[0] <= 0) return
        val (tono, ritmo) = Sintesis.tonoHabla(config[2])
        h.setPitch(tono)
        h.setSpeechRate(ritmo)
        val p = Bundle().apply { putFloat(TextToSpeech.Engine.KEY_PARAM_VOLUME, config[0] / 100f) }
        h.speak(limpio, TextToSpeech.QUEUE_FLUSH, p, "globo")
    }

    /** Lo llama el núcleo (hilo de GL): se arma y suena en el hilo de la voz. */
    fun sonar(nombre: String) {
        val volumen = config[0] / 100f
        val tipo = config[2]
        cola.post {
            runCatching { tocar(nombre, volumen, tipo) }.onFailure { Log.w("Bicho", "voz: $nombre", it) }
        }
    }

    fun apagar() {
        habla?.shutdown()
        habla = null
        reproductor?.release()
        pistaPropia?.release()
        hilo.quitSafely()
        hiloHabla.quitSafely()
    }

    /** Corta lo que se esté diciendo (empieza una conversación). */
    fun cortar() = colaHabla.post {
        habla?.stop()
        reproductor?.release()
        reproductor = null
        globoDicho = true
    }

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
        ocupar(pcm.size * 1000L / Sintesis.TASA)
        pista.setVolume(volumen)
        pista.play()
        // Suena solo; se suelta cuando termina (el hilo de la voz espera).
        Thread.sleep(pcm.size * 1000L / Sintesis.TASA + 50)
        pista.release()
    }

    companion object {
        const val ESPERA_NUBE_MS = 3000L
        const val ECO_MS = 400L
        private val ESPANOLES = listOf(Locale("es", "AR"), Locale("es", "US"), Locale("es", "MX"), Locale("es", "ES"), Locale("es"))

        /**
         * Los canales que usa el bicho, al máximo (el volumen lo pone él), salvo
         * que lo hayan silenciado por adb (el despertador suena igual).
         */
        fun canales(contexto: Context, audio: AudioManager) {
            val callado = contexto.getSharedPreferences("voz", 0).getBoolean("silencio", false)
            runCatching {
                audio.setStreamVolume(AudioManager.STREAM_MUSIC, if (callado) 0 else audio.getStreamMaxVolume(AudioManager.STREAM_MUSIC), 0)
            }
            runCatching { audio.setStreamVolume(AudioManager.STREAM_ALARM, audio.getStreamMaxVolume(AudioManager.STREAM_ALARM), 0) }
        }

        /** Si la voz propia del núcleo reemplaza al TTS del celu (por adb; queda guardado). */
        fun propiaPrendida(contexto: Context) = contexto.getSharedPreferences("voz", 0).getBoolean("propia", false)

        fun ponerPropia(contexto: Context, si: Boolean) =
            contexto.getSharedPreferences("voz", 0).edit().putBoolean("propia", si).apply()

        /** Silencia (o no) la voz del bicho; queda guardado. */
        fun silenciar(contexto: Context, si: Boolean) {
            contexto.getSharedPreferences("voz", 0).edit().putBoolean("silencio", si).apply()
            canales(contexto, contexto.getSystemService(Context.AUDIO_SERVICE) as AudioManager)
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

    /** Tono y ritmo del TTS según voz.tipo (el robot, grave y parejo). */
    fun tonoHabla(tipo: Int) = when (tipo) {
        0 -> 0.8f to 0.95f
        2 -> 1.5f to 1.05f
        3 -> 0.6f to 0.9f
        else -> 1.15f to 1f
    }

    /** El texto sin emojis ni símbolos (el TTS los nombra): letras, números y puntuación. */
    fun paraLeer(texto: String): String {
        val b = StringBuilder()
        var i = 0
        while (i < texto.length) {
            val c = texto.codePointAt(i)
            i += Character.charCount(c)
            b.append(if (Character.isLetterOrDigit(c) || c in PUNTUACION) String(Character.toChars(c)) else " ")
        }
        return b.toString().replace(Regex("\\s+"), " ").trim()
    }

    private val PUNTUACION = " .,;:¡!¿?-'\"()%\n".map { it.code }.toSet()

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
        // v1.9: los toques de amigos, la agenda, los pagos y los hábitos.
        "toque" -> listOf(Nota(1.4f, 1.8f, 70, 40, 0.7f), Nota(1.6f, 2f, 70, 0, 0.7f))
        "abrazo" -> listOf(Nota(0.9f, 1.2f, 250, 40, 0.6f), Nota(1.2f, 1f, 350, 0, 0.5f))
        "beso" -> listOf(Nota(2.2f, 1.6f, 60, 20, 0.6f))
        "agenda" -> listOf(Nota(1.26f, 1.26f, 120, 60), Nota(1.26f, 1.26f, 120, 60), Nota(1f, 1f, 200))
        "pago" -> listOf(Nota(1.5f, 1.5f, 70, 20), Nota(2f, 2f, 70, 20), Nota(2.5f, 2.5f, 220))
        "habito" -> listOf(Nota(1f, 1.25f, 110, 20), Nota(1.5f, 1.5f, 220))
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
