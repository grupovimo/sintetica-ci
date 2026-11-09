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
        // El micrófono del Moto llega muy bajo (la voz a un metro, -55 dB): se sube para Vosk.
        val t = ShortArray(n) { (tramo[it] * GANANCIA).coerceIn(-32768, 32767).toShort() }
        // Esperando la palabra, solo cuando hay sonido; escuchando una frase o conversando, todo.
        if (atento) return run { cola.offer(t) }
        val db = Oido.dbfs(tramo, n)
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

    /** Para diagnosticar: anota en el registro (solo en el celu) lo que transcribió. Apagado por defecto. */
    @Volatile var transcribir = false
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
                        .apply { setWords(true) }
                    Log.i("Bicho", "charla: espera \"$p\"")
                }
                // Solo con un resultado final y la palabra con buena confianza: con una
                // gramática de una sola palabra, los parciales la ven en cualquier ruido.
                if (!espera.acceptWaveForm(tramo, tramo.size)) continue
                val confianza = confianzaDe(espera.result, palabra)
                if (transcribir && confianza >= 0) Log.i("Bicho", "charla: palabra con confianza ${(confianza * 100).toInt()} %")
                if (confianza < UMBRAL_PALABRA) continue
                // Con una gramática de una sola palabra, la confianza engaña (una tele da 100 %):
                // se confirma con la lectura libre de los últimos segundos.
                // Solo el último segundo y medio (la palabra está ahí): así confirma rápido.
                val libre = Recognizer(model, TASA).use { r ->
                    previo.takeLast(CONFIRMAR).forEach { r.acceptWaveForm(it, it.size) }
                    texto(r.finalResult, "text")
                }
                if (!pareceLaPalabra(libre, palabra)) {
                    if (transcribir) Log.i("Bicho", "charla: no era la palabra (\"$libre\")")
                    espera.reset()
                    continue
                }
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
                    // Que su respuesta no lo vuelva a despertar.
                    esperarQueTermineDeHablar()
                    espera.reset()
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
        Thread.sleep(800)  // lo que tarda en empezar a hablar (el globo, el TTS)
        while (voz.ocupada() && System.currentTimeMillis() < hasta) Thread.sleep(100)
        cola.clear()
    }

    /** Devuelve true si el motor quedó preguntando algo (hay que seguir escuchando). */
    private fun frase(model: Model, palabra: String, previo: List<ShortArray>, conPalabra: Boolean = true): Boolean {
        Log.i("Bicho", "charla: escuchando")
        voz.sonar("escuchando")
        val r = Recognizer(model, TASA)
        // La misma frase, acotada a las palabras que conoce el motor ("clima" le gana a "simple").
        val g = Recognizer(model, TASA, gramatica(palabra))
        val partes = mutableListOf<String>()
        val acotada = mutableListOf<String>()
        for (t in previo) {
            if (r.acceptWaveForm(t, t.size)) partes += texto(r.result, "text")
            if (g.acceptWaveForm(t, t.size)) acotada += texto(g.result, "text")
        }
        var ms = 0
        var mostrado = -MOSTRAR_MS
        // Si la frase vino pegada a la palabra ("coco qué hora es"), ya está casi toda: poca espera más.
        val yaDicho = sinPalabra((partes + texto(r.partialResult, "partial")).joinToString(" "), palabra).isNotEmpty()
        val tope = if (yaDicho) COLA_MS else MAXIMO_MS
        try {
            while (andando && ms < tope) {
                // La cara de escuchar se renueva mientras escucha (el comportamiento dura 2,5 s).
                if (ms - mostrado >= MOSTRAR_MS) {
                    mostrado = ms
                    alNucleo(byteArrayOf(0x1F) + "escuchar".toByteArray())
                }
                val tramo = cola.poll(1, TimeUnit.SECONDS) ?: break
                ms += tramo.size * 1000 / TASA.toInt()
                if (g.acceptWaveForm(tramo, tramo.size)) acotada += texto(g.result, "text")
                // Termina con la primera pausa después de algo más que la palabra.
                if (r.acceptWaveForm(tramo, tramo.size)) {
                    partes += texto(r.result, "text")
                    if (sinPalabra(partes.joinToString(" "), palabra).isNotEmpty()) break
                }
            }
            if (sinPalabra(partes.joinToString(" "), palabra).isEmpty()) partes += texto(r.finalResult, "text")
            acotada += texto(g.finalResult, "text")
        } finally {
            r.close()
            g.close()
        }
        val dicho = partes.joinToString(" ")
        val dichoAcotado = acotada.joinToString(" ").replace("[unk]", " ").replace(Regex("\\s+"), " ").trim()
        if (transcribir) Log.i("Bicho", "charla: oí \"$dicho\" (acotada: \"$dichoAcotado\")")
        val limpiar = { d: String -> if (conPalabra) sinPalabra(d, palabra) else String(Avisos.recortar(d.trim(), 200), Charsets.UTF_8) }
        val enviar = limpiar(dicho)
        val enviarAcotada = limpiar(dichoAcotado)
        if (enviar.isEmpty() && enviarAcotada.isEmpty()) {
            Log.i("Bicho", "charla: no entendí nada")
            return false
        }
        // Primero el motor de charla del celu: sin red y al toque. Con lo que oyó
        // libre y, si no lo entiende, con la lectura acotada a sus palabras.
        var local = if (enviar.isNotEmpty()) charlar(enviar) else null
        // La acotada solo si se parece a lo que se oyó libre (si no, son voces de fondo que la gramática disfraza).
        if (local?.codigo != 1 && enviarAcotada.isNotEmpty() && enviarAcotada != enviar && parecido(enviarAcotada, enviar) >= 0.6f) {
            charlar(enviarAcotada)?.takeIf { it.codigo == 1 }?.let { local = it }
        }
        val l = local
        if (l?.codigo == 1) {
            Log.i("Bicho", "charla: la entendí sin IA (${l.intencion}${if (l.accion.isNotEmpty()) ", " + l.accion else ""}${if (l.pregunta) ", pregunta" else ""})")
            return l.pregunta
        }
        if (l?.codigo == 0 && !enLinea()) {
            // Sin nube no hay a quién preguntarle: el "no entendí" del motor.
            Log.i("Bicho", "charla: no la entendí y no hay nube")
            if (l.frase.isNotEmpty()) alNucleo(byteArrayOf(0x40, 0) + l.frase.toByteArray())
            return false
        }
        if (enviar.isEmpty()) return false
        Log.i("Bicho", "charla: frase de ${enviar.length} letras a la nube")
        val mandada = android.os.SystemClock.elapsedRealtime()
        alPublicar("charla", JSONObject().put("texto", enviar).toString())
        // Si nadie contesta (sin nube, sin motor ni IA), no se queda mudo: duda.
        while (android.os.SystemClock.elapsedRealtime() - mandada < SIN_RESPUESTA_MS) {
            if (voz.ultimoGloboEn > mandada) return false
            Thread.sleep(200)
        }
        Log.i("Bicho", "charla: nadie contestó")
        // El "no entendí" del motor ("¿me lo repetís?"); si no hay, la cara de duda.
        if (l?.frase?.isNotEmpty() == true) alNucleo(byteArrayOf(0x40, 0) + l.frase.toByteArray())
        else alNucleo(byteArrayOf(0x1F) + "dudar".toByteArray())
        return false
    }

    private var gramaticaHecha: String? = null

    /** Las palabras de las intenciones (assets/intenciones.json), los números y la palabra, para Vosk. */
    private fun gramatica(palabra: String): String {
        gramaticaHecha?.let { return it }
        val palabras = sortedSetOf<String>()
        runCatching {
            val json = contexto.assets.open("intenciones.json").bufferedReader().readText()
            palabras += palabrasDe(JSONObject(json))
        }.onFailure { Log.w("Bicho", "charla: sin las intenciones para la gramática", it) }
        palabras += NUMEROS
        if (palabra.isNotEmpty()) palabras += palabra
        return JSONArray(palabras.toList() + "[unk]").toString().also { gramaticaHecha = it }
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
        const val COLA_MS = 1500  // después de una frase pegada a la palabra
        const val CONFIRMAR = 30  // tramos (1,5 s) para confirmar la palabra
        const val MOSTRAR_MS = 2000
        const val PREVIO = 60  // tramos de 50 ms: 3 s, lo dicho junto con la palabra
        const val UMBRAL_PALABRA = 0.7f

        /** Del 0 al 1, qué tan parecidas son dos palabras (por la distancia de edición). */
        fun similitud(a: String, b: String): Float {
            if (a.isEmpty() || b.isEmpty()) return 0f
            var antes = IntArray(b.length + 1) { it }
            for (i in 1..a.length) {
                val ahora = IntArray(b.length + 1)
                ahora[0] = i
                for (j in 1..b.length) ahora[j] = minOf(antes[j] + 1, ahora[j - 1] + 1, antes[j - 1] + if (a[i - 1] == b[j - 1]) 0 else 1)
                antes = ahora
            }
            return 1f - antes[b.length].toFloat() / maxOf(a.length, b.length)
        }

        /** Si en lo que se oyó libre hay algo como la palabra ("poco" o "con" por "coco"). */
        fun pareceLaPalabra(libre: String, palabra: String): Boolean {
            val p = normalizar(libre).split(' ').filter { it.isNotEmpty() }
            // Una palabra parecida ("poco") o dos pegadas ("con con"); "con" sola, no.
            return p.any { similitud(it, palabra) >= 0.75f } ||
                p.zipWithNext().any { (a, b) -> similitud(a + b, palabra) >= 0.6f }
        }

        /** Qué parte de las palabras de `acotada` tiene una parecida en `libre`. */
        fun parecido(acotada: String, libre: String): Float {
            val a = normalizar(acotada).split(' ').filter { it.isNotEmpty() }
            val l = normalizar(libre).split(' ').filter { it.isNotEmpty() }
            if (a.isEmpty()) return 0f
            return a.count { x -> l.any { similitud(x, it) >= 0.5f } }.toFloat() / a.size
        }

        /** La confianza de la palabra en un resultado de Vosk con "result" (setWords), o -1 si no está. */
        fun confianzaDe(json: String, palabra: String): Float = runCatching {
            val r = JSONObject(json).optJSONArray("result") ?: return -1f
            (0 until r.length()).map { r.getJSONObject(it) }
                .filter { normalizar(it.optString("word")) == palabra }
                .maxOfOrNull { it.optDouble("conf", 0.0).toFloat() } ?: -1f
        }.getOrDefault(-1f)
        const val GANANCIA = 4  // +12 dB
        private val NUMEROS = ("cero uno una dos tres cuatro cinco seis siete ocho nueve diez once doce trece catorce quince " +
            "veinte treinta cuarenta cincuenta media cuarto y las a en de minutos minuto horas hora mañana hoy si no").split(' ')

        /** Todas las palabras de los "patrones" (sin la sintaxis: | * ? { }). */
        fun palabrasDe(o: Any?, dentro: Boolean = false, salida: MutableSet<String> = sortedSetOf()): Set<String> {
            when (o) {
                is JSONObject -> o.keys().forEach { k -> palabrasDe(o.get(k), dentro || k == "patrones", salida) }
                is JSONArray -> for (i in 0 until o.length()) palabrasDe(o.get(i), dentro, salida)
                is String -> if (dentro) o.replace(Regex("[*?{}]"), "").split(Regex("[ |]+")).filter { it.isNotEmpty() }.forEach { salida += normalizar(it) }
            }
            return salida
        }
        const val ESPERA_URL_MS = 3000L
        const val SIN_RESPUESTA_MS = 4000L

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
