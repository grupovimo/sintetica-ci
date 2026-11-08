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
 * Solo con voz.conversar prendido. El modelo se copia una vez por adb (README).
 */
class Charla(
    private val contexto: Context,
    private val voz: Voz,
    /** Un comando al núcleo propio (por el hilo de GL). */
    private val alNucleo: (ByteArray) -> Unit,
    /** La frase, a la nube. */
    private val alDecir: (String) -> Unit,
) {
    private val cola = LinkedBlockingQueue<ShortArray>(40)  // 2 s de audio, como mucho
    @Volatile private var andando = true
    private val hilo = Thread({ correr() }, "charla").apply { start() }

    /** Lo llama el hilo del oído con cada tramo (se copia: el original se borra). */
    fun oir(tramo: ShortArray, n: Int) {
        if (voz.config[3] != 1) return
        cola.offer(tramo.copyOf(n))
    }

    fun apagar() {
        andando = false
        hilo.interrupt()
    }

    private fun correr() {
        var model: Model? = null
        var espera: Recognizer? = null
        var palabra = ""
        try {
            while (andando) {
                val tramo = cola.poll(1, TimeUnit.SECONDS) ?: continue
                if (model == null) {
                    model = cargar()
                    // Sin modelo: prueba de nuevo en un minuto.
                    if (model == null) { Thread.sleep(60_000); cola.clear(); continue }
                }
                val p = normalizar(Nucleo.palabra())
                if (p != palabra || espera == null) {
                    espera?.close()
                    palabra = p
                    espera = Recognizer(model, TASA, JSONArray(listOf(p, "[unk]")).toString())
                    Log.i("Bicho", "charla: espera \"$p\"")
                }
                val listo = espera.acceptWaveForm(tramo, tramo.size)
                val oido = texto(if (listo) espera.result else espera.partialResult, if (listo) "text" else "partial")
                if (!contienePalabra(oido, palabra)) continue
                espera.reset()
                frase(model, palabra)
                cola.clear()
            }
        } catch (_: InterruptedException) {
        } finally {
            espera?.close()
            model?.close()
        }
    }

    /** Escucha una frase (hasta 8 s, o hasta que se calla) con la cara de escuchar. */
    private fun frase(model: Model, palabra: String) {
        Log.i("Bicho", "charla: escuchando")
        voz.sonar("escuchando")
        cola.clear()
        val r = Recognizer(model, TASA)
        var ms = 0
        var mostrado = -MOSTRAR_MS
        var dicho = ""
        try {
            while (andando && ms < MAXIMO_MS) {
                // La cara de escuchar se renueva mientras escucha (el comportamiento dura 2,5 s).
                if (ms - mostrado >= MOSTRAR_MS) {
                    mostrado = ms
                    alNucleo(byteArrayOf(0x1F) + "escuchar".toByteArray())
                }
                val tramo = cola.poll(1, TimeUnit.SECONDS) ?: break
                ms += tramo.size * 1000 / TASA.toInt()
                if (r.acceptWaveForm(tramo, tramo.size)) {
                    dicho = texto(r.result, "text")
                    if (dicho.isNotEmpty()) break
                }
            }
            if (dicho.isEmpty()) dicho = texto(r.finalResult, "text")
        } finally {
            r.close()
        }
        val enviar = sinPalabra(dicho, palabra)
        if (enviar.isEmpty()) {
            Log.i("Bicho", "charla: no entendí nada")
            return
        }
        Log.i("Bicho", "charla: frase de ${enviar.length} letras a la nube")
        alDecir(enviar)
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

        /** En minúsculas y sin tildes (Vosk escribe así; la palabra puede venir con mayúsculas). */
        fun normalizar(s: String): String =
            Normalizer.normalize(s.trim().lowercase(), Normalizer.Form.NFD).replace(Regex("\\p{Mn}+"), "")

        fun contienePalabra(oido: String, palabra: String) =
            palabra.isNotEmpty() && normalizar(oido).split(' ').contains(palabra)

        /** La frase sin la palabra del principio ("lucho qué hora es" → "qué hora es"). */
        fun sinPalabra(frase: String, palabra: String): String {
            val p = frase.trim().split(Regex("\\s+")).filter { it.isNotEmpty() }
            val resto = if (p.isNotEmpty() && normalizar(p[0]) == palabra) p.drop(1) else p
            return resto.joinToString(" ").take(200)
        }
    }
}
