package com.sinteticalabs.bicho

import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioTrack
import android.os.SystemClock
import android.util.Base64
import android.util.Log
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Una conversación en tiempo real con el agente de ElevenLabs del dueño, por
 * el websocket de la URL firmada que da la nube (docs/nube.md, "Voz con
 * ElevenLabs"). El micrófono va en PCM de 16 kHz; lo que dice el agente se
 * reproduce acá. Mientras el agente habla se manda silencio (el parlante está
 * al lado del micrófono). La cara: "escuchando" cuando escucha; lo automático
 * (o la expresión que pidió el agente) cuando habla.
 */
class Conversacion(
    url: String,
    private val segundos: Int,
    private val expresiones: Set<String>,
    private val volumen: () -> Float,
    /** Un comando al núcleo propio. */
    private val alNucleo: (ByteArray) -> Unit,
    private val alTerminar: () -> Unit,
) {
    private val terminada = AtomicBoolean(false)
    @Volatile private var abierta = false
    private val empezo = SystemClock.elapsedRealtime()

    private val sonar = LinkedBlockingQueue<ShortArray>()
    @Volatile private var finDeLoQueSuena = 0L
    @Volatile private var expresionPuesta = false
    private var hablabaAntes: Boolean? = null

    private val pista = AudioTrack.Builder()
        .setAudioAttributes(AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA)
            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build())
        .setAudioFormat(AudioFormat.Builder().setEncoding(AudioFormat.ENCODING_PCM_16BIT)
            .setSampleRate(TASA).setChannelMask(AudioFormat.CHANNEL_OUT_MONO).build())
        .setTransferMode(AudioTrack.MODE_STREAM)
        .setBufferSizeInBytes(maxOf(TASA, AudioTrack.getMinBufferSize(TASA, AudioFormat.CHANNEL_OUT_MONO, AudioFormat.ENCODING_PCM_16BIT)))
        .build()

    private val parlante = Thread({
        pista.play()
        try {
            while (!terminada.get()) {
                val s = sonar.poll(200, TimeUnit.MILLISECONDS) ?: continue
                pista.setVolume(volumen())
                pista.write(s, 0, s.size)
            }
        } catch (_: InterruptedException) {
        }
    }, "conversacion").apply { start() }

    private val ws: WebSocket = cliente.newWebSocket(Request.Builder().url(url).build(), object : WebSocketListener() {
        override fun onOpen(w: WebSocket, r: Response) {
            abierta = true
            w.send(JSONObject().put("type", "conversation_initiation_client_data").toString())
            Log.i("Bicho", "conversación: empezó")
        }

        override fun onMessage(w: WebSocket, texto: String) {
            runCatching { mensaje(w, JSONObject(texto)) }.onFailure { Log.w("Bicho", "conversación: mensaje", it) }
        }

        override fun onClosing(w: WebSocket, codigo: Int, razon: String) {
            w.close(1000, null)
            terminar("cerró ElevenLabs ($codigo)")
        }

        override fun onFailure(w: WebSocket, t: Throwable, r: Response?) = terminar("falló: ${t.message}")
    })

    val termino get() = terminada.get()

    /** Un tramo del micrófono (hilo de la charla); también mira la cara y el tiempo. */
    fun enviar(tramo: ShortArray) {
        if (terminada.get()) return
        if (SystemClock.elapsedRealtime() - empezo > (segundos + 5) * 1000L) return terminar("se pasó del tiempo")
        val habla = hablando()
        cara(habla)
        if (!abierta) return
        val b = ByteArray(tramo.size * 2)
        if (!habla) for (i in tramo.indices) {
            b[2 * i] = tramo[i].toByte()
            b[2 * i + 1] = (tramo[i].toInt() shr 8).toByte()
        }
        ws.send(JSONObject().put("user_audio_chunk", Base64.encodeToString(b, Base64.NO_WRAP)).toString())
    }

    /** Corta desde afuera (lo sacudieron, se fue el wifi, apagan el bicho). */
    fun terminar(por: String) {
        if (!terminada.compareAndSet(false, true)) return
        Log.i("Bicho", "conversación: terminó ($por)")
        runCatching { ws.close(1000, null) }
        parlante.interrupt()
        sonar.clear()
        runCatching { pista.pause(); pista.flush(); pista.release() }
        alNucleo(byteArrayOf(0x10))  // vuelve a lo automático
        alTerminar()
    }

    private fun mensaje(w: WebSocket, m: JSONObject) {
        when (m.optString("type")) {
            "ping" -> {
                val id = m.optJSONObject("ping_event")?.optInt("event_id") ?: return
                w.send(JSONObject().put("type", "pong").put("event_id", id).toString())
            }
            "audio" -> {
                val b64 = m.optJSONObject("audio_event")?.optString("audio_base_64").orEmpty()
                if (b64.isEmpty()) return
                val pcm = Conversaciones.pcm(Base64.decode(b64, Base64.DEFAULT))
                val ahora = SystemClock.elapsedRealtime()
                finDeLoQueSuena = maxOf(ahora, finDeLoQueSuena) + pcm.size * 1000L / TASA
                sonar.offer(pcm)
            }
            "interruption" -> {
                sonar.clear()
                runCatching { pista.pause(); pista.flush(); pista.play() }
                finDeLoQueSuena = 0
            }
            "client_tool_call" -> herramienta(w, m.getJSONObject("client_tool_call"))
            "agent_response", "user_transcript" -> Log.i("Bicho", "conversación: ${m.optString("type")}")
        }
    }

    private fun herramienta(w: WebSocket, c: JSONObject) {
        val p = c.optJSONObject("parameters") ?: JSONObject()
        when (c.optString("tool_name")) {
            "poner_expresion" -> {
                val nombre = p.optString("nombre")
                if (nombre in expresiones) {
                    expresionPuesta = true
                    alNucleo(byteArrayOf(0x10) + nombre.toByteArray())
                }
            }
            "mirar" -> alNucleo(byteArrayOf(0x46, Conversaciones.cien(p.optDouble("x", 0.0)), Conversaciones.cien(p.optDouble("y", 0.0))))
        }
        if (c.optBoolean("expects_response")) {
            w.send(JSONObject().put("type", "client_tool_result").put("tool_call_id", c.optString("tool_call_id"))
                .put("result", "ok").put("is_error", false).toString())
        }
    }

    /** Si todavía suena lo que dijo el agente (con un margen para las pausas entre trozos). */
    private fun hablando() = SystemClock.elapsedRealtime() < finDeLoQueSuena + 300

    private fun cara(habla: Boolean) {
        if (habla == hablabaAntes) return
        hablabaAntes = habla
        if (habla) {
            if (!expresionPuesta) alNucleo(byteArrayOf(0x10))
        } else {
            expresionPuesta = false
            alNucleo(byteArrayOf(0x10) + "escuchando".toByteArray())
        }
    }

    companion object {
        const val TASA = 16000
        private val cliente by lazy { OkHttpClient.Builder().pingInterval(20, TimeUnit.SECONDS).build() }
    }
}

/** Lo puro de la conversación, para probarlo sin el celu. */
object Conversaciones {
    /** PCM de 16 bits little endian a muestras. */
    fun pcm(b: ByteArray) = ShortArray(b.size / 2) { ((b[2 * it].toInt() and 0xFF) or (b[2 * it + 1].toInt() shl 8)).toShort() }

    /** De -100 a 100, como el 0x46 Mirar. */
    fun cien(v: Double) = v.coerceIn(-100.0, 100.0).toInt().toByte()

    /** La respuesta de bicho/{id}/conversacion/url. */
    class Respuesta(val url: String?, val error: String?, val segundos: Int, val expresiones: Set<String>)

    fun leer(json: String): Respuesta {
        val o = JSONObject(json)
        val ex = o.optJSONArray("expresiones")
        return Respuesta(
            url = o.optString("url").ifEmpty { null },
            error = o.optString("error").ifEmpty { null },
            segundos = o.optInt("segundos", 300),
            expresiones = if (ex == null) emptySet() else (0 until ex.length()).map { ex.getString(it) }.toSet(),
        )
    }
}
