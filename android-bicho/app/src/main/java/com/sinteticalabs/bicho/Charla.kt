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
    /** La charla sin IA del núcleo (firmware/nucleo/charla): primero, siempre. */
    private val charlar: (String) -> Respuesta?,
) {
    /** Lo que devuelve el núcleo: 1 entendió (ya contestó), 0 no entendió, < 0 error. */
    class Respuesta(val codigo: Int, val intencion: String, val accion: String, val frase: String, val pregunta: Boolean = false) {
        companion object {
            fun leer(b: ByteArray): Respuesta? {
                val p = String(b, Charsets.UTF_8).split('\t', limit = 5)
                if (p.size < 4) return null
                return Respuesta(p[0].toIntOrNull() ?: return null, p[1], p[2], p[3], p.getOrNull(4) == "1")
            }
        }
    }

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
        val t = tramo.copyOf(n)
        // Esperando la palabra, solo cuando hay sonido; escuchando una frase o conversando, todo.
        if (atento) return run { cola.offer(t) }
        val db = Oido.dbfs(t, n)
        val pasan = compuerta.tramo(t, db)
        niveles[tramos % niveles.size] = db
        pasan.forEach { cola.offer(it) }
        // Cada minuto, cuánto deja pasar la compuerta (lo que gasta Vosk).
        tramos++
        if (pasan.isNotEmpty()) abiertos++
        if (tramos == 1200) {
            val o = niveles.sorted()
            Log.i("Bicho", "charla: compuerta abierta ${abiertos * 100 / tramos} % · fondo ${compuerta.fondo.toInt()} dB · " +
                "niveles ${o[0].toInt()}/${o[o.size / 2].toInt()}/${o[o.size * 9 / 10].toInt()}/${o.last().toInt()}")
            tramos = 0
            abiertos = 0
        }
    }

    private var tramos = 0
    private val niveles = FloatArray(1200)
    private var abiertos = 0

    private val compuerta = Compuerta()
    @Volatile private var atento = false

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
                atento = true
                try {
                    val resto = conversar(antes)
                    // Si preguntó ("¿Te pongo una alarma a las 6?"), el "sí" entra sin repetir la palabra.
                    var pregunto = resto != null && frase(model, palabra, antes + resto)
                    var vueltas = 0
                    while (pregunto && vueltas++ < 3) {
                        esperarQueTermineDeHablar()
                        pregunto = frase(model, palabra, emptyList(), conPalabra = false)
                    }
                } finally {
                    atento = false
                }
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
    /** Hasta que el bicho termina de decir la pregunta (como mucho 10 s); lo que se oyó mientras, se tira. */
    private fun esperarQueTermineDeHablar() {
        val hasta = System.currentTimeMillis() + 10_000
        Thread.sleep(300)
        while (voz.ocupada() && System.currentTimeMillis() < hasta) Thread.sleep(100)
        cola.clear()
    }

    /** Devuelve true si el motor quedó preguntando algo (hay que seguir escuchando). */
    private fun frase(model: Model, palabra: String, previo: List<ShortArray>, conPalabra: Boolean = true): Boolean {
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
        val dicho = partes.joinToString(" ")
        val enviar = if (conPalabra) sinPalabra(dicho, palabra) else String(Avisos.recortar(dicho.trim(), 200), Charsets.UTF_8)
        if (enviar.isEmpty()) {
            Log.i("Bicho", "charla: no entendí nada")
            return false
        }
        // Primero el motor de charla del celu: sin red y al toque.
        val local = charlar(enviar)
        if (local?.codigo == 1) {
            Log.i("Bicho", "charla: la entendí sin IA (${local.intencion}${if (local.accion.isNotEmpty()) ", " + local.accion else ""}${if (local.pregunta) ", pregunta" else ""})")
            return local.pregunta
        }
        if (local?.codigo == 0 && !enLinea()) {
            // Sin nube no hay a quién preguntarle: el "no entendí" del motor.
            Log.i("Bicho", "charla: no la entendí y no hay nube")
            if (local.frase.isNotEmpty()) alNucleo(byteArrayOf(0x40, 0) + local.frase.toByteArray())
            return false
        }
        Log.i("Bicho", "charla: frase de ${enviar.length} letras a la nube")
        val mandada = android.os.SystemClock.elapsedRealtime()
        alPublicar("charla", JSONObject().put("texto", enviar).toString())
        // Si nadie contesta (sin nube, sin motor ni IA), no se queda mudo: duda.
        while (android.os.SystemClock.elapsedRealtime() - mandada < SIN_RESPUESTA_MS) {
            if (voz.ultimoGloboEn > mandada) return false
            Thread.sleep(200)
        }
        Log.i("Bicho", "charla: nadie contestó")
        alNucleo(byteArrayOf(0x1F) + "dudar".toByteArray())
        return false
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
        const val SIN_RESPUESTA_MS = 6000L

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
