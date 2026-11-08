package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** Cuando pasa algo en el robot, la nube hace cosas (web: modulos/Integraciones.svelte). */
data class Accion(
    val tipo: String,               // "telegram", "webhook" o "robot"
    val texto: String? = null,      // telegram (con plantillas)
    val url: String? = null,        // webhook
    val robot: String? = null,      // robot: otro del mismo dueño ("bicho" en el JSON)
    val expresion: String? = null,
)

data class Automatizacion(val id: String, val activa: Boolean, val cuando: String, val valor: String? = null, val hacer: List<Accion>)

data class Ejecucion(val evento: String, val accion: String, val ok: Boolean, val detalle: String, val en: String)

data class Integraciones(val lista: List<Automatizacion>, val historial: List<Ejecucion>, val cuandos: List<String>)

object Automatizaciones {
    const val MAXIMO = 20
    const val MAX_ACCIONES = 4
    val CUANDOS_DE_FABRICA = listOf("recordatorio", "bateria-baja", "desconectado", "conectado", "despierta", "duerme", "expresion")
    val ACCIONES = listOf("telegram" to "Telegram", "webhook" to "URL", "robot" to "Otro robot")
    val CARAS = listOf("contento", "sorpresa", "enamorado", "guino", "triste", "pensando", "dormido")
    const val PLANTILLAS = "{robot} {bateria} {expresion} {texto} {hora}"

    val NOMBRES_CUANDO = mapOf(
        "recordatorio" to "Un recordatorio", "pomodoro" to "Fin de tanda", "bateria-baja" to "Poca batería",
        "desconectado" to "Se desconecta", "conectado" to "Se conecta", "despierta" to "Se despierta", "duerme" to "Se duerme",
        "expresion" to "Cambia de cara", "tacto" to "Lo tocan", "doble-toque" to "Doble toque", "caricia" to "Una caricia",
        "muchos-toques" to "Muchos toques", "levantado" to "Lo levantan", "sacudida" to "Lo sacuden", "sacudida-suave" to "Sacudida suave",
        "sacudida-fuerte" to "Sacudida fuerte", "boca-abajo" to "Boca abajo", "inclinado" to "Lo inclinan", "caida" to "Se cae",
        "voz" to "Le hablan", "nombre" to "Dicen su nombre", "ruido" to "Un ruido", "musica" to "Música", "cargando" to "Lo enchufan",
        "desenchufado" to "Lo desenchufan", "cargado" to "Carga completa", "volvio" to "Volvés", "hito" to "Un hito",
    )

    fun nombreCuando(k: String) = NOMBRES_CUANDO[k] ?: k

    fun leer(o: JsonObject): Integraciones {
        val lista = o.lista("automatizaciones").orEmpty().mapNotNull { e ->
            val a = e as? JsonObject ?: return@mapNotNull null
            Automatizacion(
                id = a.texto("id").orEmpty(),
                activa = a.si("activa") ?: true,
                cuando = a.texto("cuando").orEmpty(),
                valor = a.texto("valor")?.takeIf { it.isNotEmpty() },
                hacer = a.lista("hacer").orEmpty().mapNotNull { h ->
                    val x = h as? JsonObject ?: return@mapNotNull null
                    Accion(x.texto("tipo").orEmpty(), x.texto("texto"), x.texto("url"), x.texto("bicho"), x.texto("expresion"))
                },
            )
        }
        val historial = o.lista("historial").orEmpty().mapNotNull { e ->
            val h = e as? JsonObject ?: return@mapNotNull null
            Ejecucion(h.texto("evento").orEmpty(), h.texto("accion").orEmpty(), h.si("ok") ?: false, h.texto("detalle").orEmpty(), h.texto("en").orEmpty())
        }
        val cuandos = o.textos("cuandos").ifEmpty { CUANDOS_DE_FABRICA }
        return Integraciones(lista, historial, cuandos)
    }

    fun aJson(a: Automatizacion): JsonObject = buildJsonObject {
        put("id", a.id)
        put("activa", a.activa)
        put("cuando", a.cuando)
        if (a.cuando == "expresion" && !a.valor.isNullOrEmpty()) put("valor", a.valor)
        put("hacer", JsonArray(a.hacer.map { h ->
            buildJsonObject {
                put("tipo", h.tipo)
                when (h.tipo) {
                    "telegram" -> put("texto", h.texto.orEmpty())
                    "webhook" -> put("url", h.url.orEmpty())
                    else -> { put("bicho", h.robot.orEmpty()); put("expresion", h.expresion ?: "contento") }
                }
            }
        }))
    }

    /** El cuerpo del PUT /v1/bichos/{id}/automatizaciones. */
    fun cuerpo(l: List<Automatizacion>): JsonObject = buildJsonObject { put("automatizaciones", JsonArray(l.map { aJson(it) })) }

    /** Una nueva: recordatorio → Telegram. El id, como la web: "a" + el momento en base 36. */
    fun nueva(ahoraMs: Long): Automatizacion =
        Automatizacion("a" + ahoraMs.toString(36), true, "recordatorio", null, listOf(Accion("telegram", texto = "{robot}: {texto}")))

    fun accionNueva(tipo: String, otro: String?): Accion = when (tipo) {
        "telegram" -> Accion(tipo, texto = "{robot}: {evento}")
        "webhook" -> Accion(tipo, url = "https://")
        else -> Accion("robot", robot = otro.orEmpty(), expresion = "contento")
    }

    /** Cambia el "cuando"; el valor solo vale para "Cambia de cara". */
    fun conCuando(a: Automatizacion, cuando: String) = a.copy(cuando = cuando, valor = if (cuando == "expresion") a.valor else null)

    fun sumarAccion(a: Automatizacion, tipo: String, otro: String?): Automatizacion =
        if (a.hacer.size >= MAX_ACCIONES || (tipo == "robot" && otro == null)) a else a.copy(hacer = a.hacer + accionNueva(tipo, otro))

    /** Siempre queda al menos una acción. */
    fun quitarAccion(a: Automatizacion, j: Int): Automatizacion = if (a.hacer.size <= 1) a else a.copy(hacer = a.hacer.filterIndexed { k, _ -> k != j })
}
