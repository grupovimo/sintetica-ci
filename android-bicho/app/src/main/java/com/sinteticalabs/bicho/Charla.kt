package com.sinteticalabs.bicho

import android.content.Context
import android.util.Log
import org.json.JSONArray
import org.json.JSONObject
import org.vosk.Model
import org.vosk.Recognizer
import java.io.File
import java.text.Normalizer
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit

/**
 * La charla con IA, con voz → texto en el celu (Vosk, sin red). Mientras
 * espera solo reconoce su palabra (voz.palabra, o su nombre). Al oírla pone la
 * cara de escuchar y transcribe una sola frase, que es lo único que sale del
 * celu: el texto, por la nube (bicho/{id}/charla), que contesta con un Decir
 * (globo + TTS). El audio nunca se guarda ni se manda.
 * Si el dueño conectó ElevenLabs, después de la palabra primero pide una
 * conversación en tiempo real ([Conversacion]); sin cuenta, como antes.
 * Solo con voz.conversar prendido. El modelo se copia una vez por adb (README).
 */
class Charla(
    private val contexto: Context,
    private val voz: Voz,
    /** Un comando al núcleo propio (por el hilo de GL). */
    private val alNucleo: (ByteArray) -> Unit,
    /** Un JSON a un tópico propio de la nube (charla, conversacion). */
    private val alPublicar: (String, String) -> Unit,
    /** Si hay nube (para pedir la conversación). */
    private val enLinea: () -> Boolean,
) {
    private val respuestas = LinkedBlockingQueue<String>(4)
    @Volatile private var conversacion: Conversacion? = null
    // Sin cuenta de ElevenLabs (o sin respuesta): no se pide de nuevo por un rato.
    private var sinConversacionHasta = 0L
    private val cola = LinkedBlockingQueue<ShortArray>(40)  // 2 s de audio, como mucho
    @Volatile private var andando = true
    private val hilo = Thread({ correr() }, "charla").apply { start() }

    /** Lo llama el hilo del oído con cada tramo (se copia: el original se borra). */
    fun oir(tramo: ShortArray, n: Int) {
        if (voz.config[3] != 1) return
        cola.offer(tramo.copyOf(n))
    }

    /** Lo que llega por bicho/{id}/conversacion/url. */
    fun respuesta(json: String) {
        respuestas.offer(json)
    }

    /** Corta la conversación si hay una (lo sacudieron, se fue el wifi). */
    fun cortar(por: String) = conversacion?.terminar(por)

    fun apagar() {
        cortar("se apaga el bicho")
        andando = false
        hilo.interrupt()
    }

    private fun correr() {
        var model: Model? = null
        var espera: Recognizer? = null
        var palabra = ""
        // El último segundo: lo que se dijo pegado a la palabra ("coco qué hora es") no se pierde.
        val previo = ArrayDeque<ShortArray>()
        try {
            while (andando) {
                val tramo = cola.poll(1, TimeUnit.SECONDS) ?: continue
                previo.addLast(tramo)
                if (previo.size > PREVIO) previo.removeFirst()
                if (model == null) {
                    model = cargar()
                    // Sin modelo: prueba de nuevo en un minuto.
                    if (model == null) { Thread.sleep(60_000); cola.clear(); continue }
                }
                val escrita = Nucleo.palabra().trim().lowercase()
                val p = normalizar(escrita)
                if (p != palabra || espera == null) {
                    espera?.close()
                    palabra = p
                    // Las dos formas ("ramón" y "ramon"): Vosk ignora la que no está en su vocabulario.
                    espera = Recognizer(model, TASA, JSONArray(listOf(p, escrita, "[unk]").distinct()).toString())
                    Log.i("Bicho", "charla: espera \"$p\"")
                }
                val listo = espera.acceptWaveForm(tramo, tramo.size)
                val oido = texto(if (listo) espera.result else espera.partialResult, if (listo) "text" else "partial")
                if (!contienePalabra(oido, palabra)) continue
                espera.reset()
                val antes = previo.toList()
                previo.clear()
                // Con conversación (null) ya está; si no, Vosk con lo que se oyó mientras tanto.
                val resto = conversar(antes)
                if (resto != null) frase(model, palabra, antes + resto)
                cola.clear()
            }
        } catch (_: InterruptedException) {
        } finally {
            espera?.close()
            model?.close()
        }
    }

    /**
     * Pide la conversación en tiempo real y, si la dan, la sostiene hasta que
     * termina (null). Si no, devuelve lo que se oyó mientras esperaba, para Vosk.
     */
    private fun conversar(previo: List<ShortArray>): List<ShortArray>? {
        val ahora = System.currentTimeMillis()
        if (ahora < sinConversacionHasta || !enLinea()) return emptyList()
        respuestas.clear()
        alNucleo(byteArrayOf(0x10) + "escuchando".toByteArray())
        alPublicar("conversacion", "{\"pedir\":true}")
        val oido = ArrayList<ShortArray>()
        var r: Conversaciones.Respuesta? = null
        val hasta = ahora + ESPERA_URL_MS
        while (r == null && System.currentTimeMillis() < hasta) {
            cola.poll(50, TimeUnit.MILLISECONDS)?.let { oido += it }
            respuestas.poll()?.let { r = runCatching { Conversaciones.leer(it) }.getOrNull() }
        }
        val res = r
        if (res?.url == null) {
            alNucleo(byteArrayOf(0x10))
            Log.i("Bicho", "conversación: ${res?.error ?: "sin respuesta"}; sigue la charla de siempre")
            sinConversacionHasta = ahora + if (res == null) 2 * 60_000L else 10 * 60_000L
            return oido
        }
        voz.cortar()
        voz.callada = true
        val c = Conversacion(res.url, res.segundos, res.expresiones, { voz.config[0] / 100f }, alNucleo) {}
        conversacion = c
        try {
            for (t in previo + oido) c.enviar(t)
            while (andando && !c.termino) cola.poll(1, TimeUnit.SECONDS)?.let(c::enviar)
        } finally {
            c.terminar("fin")
            conversacion = null
            voz.callada = false
        }
        return null
    }

    /** Escucha una frase (hasta 8 s, o hasta que se calla) con la cara de escuchar. */
    private fun frase(model: Model, palabra: String, previo: List<ShortArray>) {
        Log.i("Bicho", "charla: escuchando")
        voz.sonar("escuchando")
        val r = Recognizer(model, TASA)
        val partes = mutableListOf<String>()
        for (t in previo) if (r.acceptWaveForm(t, t.size)) partes += texto(r.result, "text")
        var ms = 0
        var mostrado = -MOSTRAR_MS
        try {
            while (andando && ms < MAXIMO_MS) {
                // La cara de escuchar se renueva mientras escucha (el comportamiento dura 2,5 s).
                if (ms - mostrado >= MOSTRAR_MS) {
                    mostrado = ms
                    alNucleo(byteArrayOf(0x1F) + "escuchar".toByteArray())
                }
                val tramo = cola.poll(1, TimeUnit.SECONDS) ?: break
                ms += tramo.size * 1000 / TASA.toInt()
                // Termina con la primera pausa después de algo más que la palabra.
                if (r.acceptWaveForm(tramo, tramo.size)) {
                    partes += texto(r.result, "text")
                    if (sinPalabra(partes.joinToString(" "), palabra).isNotEmpty()) break
                }
            }
            if (sinPalabra(partes.joinToString(" "), palabra).isEmpty()) partes += texto(r.finalResult, "text")
        } finally {
            r.close()
        }
        val enviar = sinPalabra(partes.joinToString(" "), palabra)
        if (enviar.isEmpty()) {
            Log.i("Bicho", "charla: no entendí nada")
            return
        }
        Log.i("Bicho", "charla: frase de ${enviar.length} letras a la nube")
        alPublicar("charla", JSONObject().put("texto", enviar).toString())
    }

    private fun cargar(): Model? {
        val dir = File(contexto.getExternalFilesDir(null), "vosk-es")
        if (!File(dir, "am").isDirectory) {
            Log.w("Bicho", "charla: falta el modelo en $dir (ver el README)")
            return null
        }
        return runCatching { Model(dir.path) }.onFailure { Log.w("Bicho", "charla: el modelo no cargó", it) }.getOrNull()
    }

    private fun texto(json: String, campo: String) = runCatching { JSONObject(json).optString(campo) }.getOrDefault("")

    companion object {
        const val TASA = 16000f
        const val MAXIMO_MS = 8000
        const val MOSTRAR_MS = 2000
        const val PREVIO = 20  // tramos de 50 ms
        const val ESPERA_URL_MS = 3000L

        /** En minúsculas y sin tildes (Vosk escribe así; la palabra puede venir con mayúsculas). */
        fun normalizar(s: String): String =
            Normalizer.normalize(s.trim().lowercase(), Normalizer.Form.NFD).replace(Regex("\\p{Mn}+"), "")

        fun contienePalabra(oido: String, palabra: String) =
            palabra.isNotEmpty() && normalizar(oido).split(' ').contains(palabra)

        /** La frase desde después de la palabra, de hasta 200 bytes ("lucho qué hora es" → "qué hora es"). */
        fun sinPalabra(frase: String, palabra: String): String {
            val p = frase.trim().split(Regex("\\s+")).filter { it.isNotEmpty() }
            val resto = p.drop(p.indexOfFirst { normalizar(it) == palabra } + 1)
            // La nube acepta hasta 200 bytes UTF-8: se corta sin partir letras.
            return String(Avisos.recortar(resto.joinToString(" "), 200), Charsets.UTF_8)
        }
    }
}
