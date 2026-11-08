package com.sinteticalabs.app.ui.consola

import android.content.Context
import com.sinteticalabs.app.consola.Regla
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.nucleo.Nucleo
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray

/**
 * Lo de fábrica del motor de vida: las expresiones y los comportamientos
 * (los embebe el núcleo), las reglas, los 16 dibujos del manual y las
 * animaciones de ejemplo (shared/formato, como assets).
 */
object VidaBase {
    var expresiones: Map<String, JsonObject> = emptyMap(); private set
    var comportamientos: Map<String, JsonObject> = emptyMap(); private set
    var reglas: List<Regla> = emptyList(); private set
    var dibujos: List<Pair<String, JsonObject>> = emptyList(); private set
    var animaciones: List<Pair<String, JsonObject>> = emptyList(); private set
    private var cargada = false

    /** Primero las de siempre, como en el archivo. */
    private val ORDEN = listOf("neutral", "contento", "emocionado", "risa", "sospecha", "sorpresa", "miedo", "dormido", "bostezo", "enojado",
        "grunon", "triste", "verguenza", "guino", "enamorado", "mareado", "escuchando", "hablando", "pensando",
        "confundido", "curioso", "aburrido", "orgulloso", "concentrado", "bateria-baja", "cargando", "actualizando",
        "sin-conexion", "ko", "glitch", "cansado", "ofendido", "estornudo", "espiar", "estirarse", "sonando", "pesadilla")

    val nombresCaras: List<String> get() = ORDEN.filter { it in expresiones } + expresiones.keys.filter { it !in ORDEN }.sorted()

    fun cargar(contexto: Context) {
        if (cargada) return
        cargada = true
        runCatching { expresiones = Nucleo.deFabrica(0).associate { it.first to leerObjeto(it.second) } }
        runCatching { comportamientos = Nucleo.deFabrica(1).associate { it.first to leerObjeto(it.second) } }
        val a = contexto.assets
        fun leer(ruta: String) = a.open(ruta).bufferedReader().use { it.readText() }
        runCatching { reglas = Json.parseToJsonElement(leer("vida/reglas.json")).jsonArray.mapNotNull { Regla.de(it) } }
        runCatching {
            dibujos = a.list("caras").orEmpty().sorted().map { leerObjeto(leer("caras/$it")) }
                .filter { it["formas"] != null && it.texto("nombre") != null }
                .sortedBy { it.numero("numero") ?: 99.0 }
                .map { o -> o.texto("nombre")!! to JsonObject(o.filterKeys { it == "formas" || it == "parpadea" }) }
        }
        runCatching {
            animaciones = a.list("animaciones").orEmpty().sorted().map { leerObjeto(leer("animaciones/$it")) }
                .filter { it["pistas"] != null && it.texto("nombre") != null }
                .map { o -> o.texto("nombre")!! to JsonObject(o.filterKeys { it != "nombre" }) }
        }
    }
}
