package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * La v1.9 en la config (web/src/lib/formato.ts): los hábitos y la presencia,
 * y lo que la consola necesita para los amigos, la agenda, los avisos de
 * afuera y la música. Todo puro.
 */
object Compania {
    const val MAX_HABITOS = 4
    const val MAX_NOMBRE_HABITO = 16
    const val MAX_TEXTO_PRESENCIA = 40
    const val MAX_CHARLA = 200
    val ICONOS_HABITO = listOf("gota", "libro", "pesa", "zen", "pastilla", "cama", "fruta", "estrella")
    val GLIFO_HABITO = mapOf("gota" to "💧", "libro" to "📖", "pesa" to "🏋", "zen" to "🧘", "pastilla" to "💊", "cama" to "🛏", "fruta" to "🍎", "estrella" to "★")
    val TIPOS_TOQUE = listOf("toque" to "Toque", "abrazo" to "Abrazo", "beso" to "Beso", "carita" to "Carita")
    val GLIFO_TOQUE = mapOf("toque" to "👉", "abrazo" to "🤗", "beso" to "💋", "carita" to "😊")
    val CARITAS = listOf("contento", "risa", "enamorado", "guino", "sorpresa", "triste", "orgulloso", "pensando")
    val TIPOS_ENTRANTE = listOf("pago" to "Pago", "mail" to "Mail", "github" to "GitHub", "pedido" to "Pedido", "otro" to "Otro")
    val EJEMPLO_ENTRANTE = mapOf("pago" to ("$12.000" to "Ana"), "mail" to ("Hola" to "Ana"), "github" to ("PR #12" to "ana"),
        "pedido" to ("Pedido #8" to "La tienda"), "otro" to ("Aviso" to "Vos"))
    private val DIAS = listOf("L", "M", "X", "J", "V", "S", "D")

    // --- Hábitos (hasta 4) ---
    data class Habito(val nombre: String, val icono: String = "estrella", val hora: String? = null, val dias: String? = null) {
        fun aJson(): JsonObject = buildJsonObject {
            put("nombre", nombre)
            put("icono", icono)
            if (!hora.isNullOrEmpty()) put("hora", hora)
            if (!dias.isNullOrEmpty() && dias.length < 7) put("dias", dias)
        }
    }

    private val EJEMPLOS = listOf(Habito("Agua", "gota"), Habito("Leer", "libro", "22:00"), Habito("Moverme", "pesa", "18:00", "LMXJV"), Habito("Meditar", "zen", "08:00"))

    fun habitos(c: JsonObject): List<Habito> = c.lista("habitos").orEmpty().mapNotNull { e ->
        val o = e as? JsonObject ?: return@mapNotNull null
        Habito(o.texto("nombre") ?: return@mapNotNull null, o.texto("icono") ?: "estrella", o.texto("hora"), o.texto("dias"))
    }

    fun ponerHabitos(c: JsonObject, l: List<Habito>) = if (l.isEmpty()) c.sin("habitos") else c.con("habitos", JsonArray(l.take(MAX_HABITOS).map { it.aJson() }))

    /** Uno nuevo: el primer ejemplo con un ícono que no esté usado. */
    fun sumarHabito(c: JsonObject): JsonObject {
        val l = habitos(c)
        if (l.size >= MAX_HABITOS) return c
        val usados = l.map { it.icono }.toSet()
        return ponerHabitos(c, l + (EJEMPLOS.firstOrNull { it.icono !in usados } ?: Habito("Hábito", "estrella")))
    }

    fun cambiarHabito(c: JsonObject, i: Int, f: (Habito) -> Habito): JsonObject {
        val l = habitos(c).toMutableList()
        if (i !in l.indices) return c
        l[i] = f(l[i])
        return ponerHabitos(c, l)
    }

    fun quitarHabito(c: JsonObject, i: Int) = ponerHabitos(c, habitos(c).filterIndexed { k, _ -> k != i })

    /** El nombre: 1 a 16 bytes; vacío no cambia. */
    fun conNombre(h: Habito, t: String): Habito = cortarBytes(t.trim(), MAX_NOMBRE_HABITO).ifEmpty { null }?.let { h.copy(nombre = it) } ?: h

    /** Prende o apaga un día (nunca ninguno; los siete = todos). */
    fun alternarDia(h: Habito, letra: String): Habito {
        val ya = h.dias?.ifEmpty { null } ?: DIAS.joinToString("")
        val nuevo = DIAS.filter { if (it == letra) !ya.contains(it) else ya.contains(it) }.joinToString("")
        if (nuevo.isEmpty()) return h
        return h.copy(dias = if (nuevo.length == 7) null else nuevo)
    }

    fun textoRacha(n: Int) = if (n == 1) "día" else "días"

    // --- Presencia ---
    data class Presencia(val preguntar: Boolean = true, val texto: String = "")

    fun presencia(c: JsonObject): Presencia = c.obj("presencia")?.let { Presencia(it.si("preguntar") ?: true, it.texto("texto").orEmpty()) } ?: Presencia()

    fun ponerPresencia(c: JsonObject, f: (Presencia) -> Presencia): JsonObject {
        val p = f(presencia(c))
        val t = cortarBytes(p.texto.trim(), MAX_TEXTO_PRESENCIA)
        return c.con("presencia", buildJsonObject { put("preguntar", p.preguntar); if (t.isNotEmpty()) put("texto", t) })
    }

    // --- Amigos ---
    /** "abcd efgh" o "ABCD-EFGH" → "ABCDEFGH". */
    fun limpiarCodigo(c: String) = c.filter { it.isLetter() }.uppercase().take(8)

    /** La lista de mis robots que reciben sus toques al tocar uno (null = todos). */
    fun recibir(todos: Boolean, actuales: List<String>, mios: List<String>, id: String?): List<String>? {
        if (id == null) return if (todos) emptyList() else null
        val ya = if (todos) mios else actuales
        val l = if (id in ya) ya - id else ya + id
        return if (l.size == mios.size && mios.all { it in l }) null else l
    }

    // --- Habla: las fichas de cada situación (las de siempre y las que usan sus frases de fábrica) ---
    val SIEMPRE = listOf("{dueno}", "{robot}", "{clima}")

    fun fichas(fabrica: JsonObject?): List<String> =
        SIEMPRE + Regex("\\{[a-z]+\\}").findAll(fabrica?.toString().orEmpty()).map { it.value }.distinct().filter { it !in SIEMPRE }.toList()

    /** La primera frase de fábrica (de ejemplo). */
    fun deFabrica(fabrica: JsonObject?): String? = fabrica?.obj("frases")?.textos("todos")?.firstOrNull()

    /** Las situaciones de Habla, agrupadas como en la web. */
    val GRUPOS_HABLA = listOf(
        "Su día" to listOf("despertar", "buen-dia", "comiste", "tardaste", "a-dormir", "buenas-noches", "despertador", "posponer", "sueno", "agenda", "agenda-manana"),
        "Con vos" to listOf("saludo", "volviste", "te-vas", "te-extrane", "caricia", "aburrido", "como-estas", "animo-bien", "animo-medio", "animo-mal", "musica"),
        "Amigos" to listOf("toque-recibido", "toque-devuelto"),
        "Cuidarte" to listOf("agua", "pausa", "recordatorio-hecho", "habito-hecho", "habito-racha", "habito-cortado", "bateria-baja", "cargando"),
        "Avisos" to listOf("pago", "mail", "github", "pedido", "aviso"),
    )
    val NOMBRES_NUEVOS = mapOf(
        "toque-recibido" to "Un toque", "toque-devuelto" to "Lo devolvió", "agenda" to "Se viene algo", "agenda-manana" to "Mañana",
        "te-vas" to "Te vas", "habito-hecho" to "Hábito hecho", "habito-racha" to "Racha", "habito-cortado" to "Se cortó",
        "musica" to "Música", "pago" to "Pago", "mail" to "Mail", "github" to "GitHub", "pedido" to "Pedido", "aviso" to "Aviso",
    )

    // --- Automatizaciones: las plantillas que solo tienen sentido con un cuando ---
    val PLANTILLAS_CUANDO = mapOf("juego" to listOf("{juego}", "{resultado}"), "animo" to listOf("{animo}"), "habito" to listOf("{habito}", "{racha}"))

    // --- Los avisos del celu con tipo (la misma tabla que el robot-celu: android-bicho/…/Avisos.kt) ---
    private val PAGOS = setOf("com.mercadopago.wallet", "com.uala", "ar.com.bancar.uala", "com.naranjax.app")
    private val MAILS = setOf("com.google.android.gm", "com.microsoft.office.outlook", "com.google.android.gm.lite")
    private val PEDIDOS = setOf("com.mercadolibre", "com.pedidosya", "com.grability.rappi")

    /** El tipo de aviso según la app: llamada, pago, mail, github, pedido o notificacion. */
    fun tipoAviso(paquete: String, llamada: Boolean) = when {
        llamada -> "llamada"
        paquete in PAGOS -> "pago"
        paquete in MAILS -> "mail"
        paquete == "com.github.android" -> "github"
        paquete in PEDIDOS -> "pedido"
        else -> "notificacion"
    }

    /** "$12.000" de "Recibiste $12.000 de Ana", o null. */
    fun monto(texto: String) = Regex("\\$\\s?[0-9][0-9.,]*").find(texto)?.value?.replace(" ", "")

    /** Lo que va en app y contacto según el tipo (como el robot-celu). */
    fun camposAviso(tipo: String, app: String, quien: String): Pair<String, String> = when (tipo) {
        "pago" -> (monto(quien) ?: app) to ""
        "mail" -> app to quien
        "github", "pedido" -> quien to ""
        else -> app to quien
    }
}

/** Los links de la web que abren la app (https://sintetica.grupovimo.com.ar/robots?…). */
object LinksWeb {
    const val HOST = "sintetica.grupovimo.com.ar"

    /** ("amigo", "ABCDEFGH") o ("spotify", "ok"); null si no es uno de esos. */
    fun leer(esquema: String?, host: String?, ruta: String?, amigo: String?, spotify: String?): Pair<String, String>? {
        if (esquema != "https" || host != HOST || ruta?.startsWith("/robots") != true) return null
        val c = Compania.limpiarCodigo(amigo.orEmpty())
        if (c.length == 8) return "amigo" to c
        if (!spotify.isNullOrEmpty()) return "spotify" to spotify
        return null
    }

    fun textoSpotify(v: String) = when (v) { "ok" -> "Spotify conectado"; "cancelado" -> "Spotify: cancelado"; else -> "Spotify no se conectó" }
}
