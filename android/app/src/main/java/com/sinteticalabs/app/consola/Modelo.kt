package com.sinteticalabs.app.consola

import com.sinteticalabs.app.protocolo.Estado
import com.sinteticalabs.app.protocolo.Pantalla

/**
 * Cómo se presenta cada robot en la consola: su nombre de producto, la foto
 * real y dónde cae la pantalla en esa foto, y lo que trae. Igual que
 * web/src/lib/consola/modelo.ts. Nada de acá nombra placas ni componentes.
 */
enum class Capacidad(val nombre: String) {
    PANTALLA("Pantalla"), MOVIMIENTO("Movimiento"), BLUETOOTH("Bluetooth"), WIFI("WiFi"), MICROFONO("Micrófono"), PARLANTE("Parlante"),
}

/** Una caja en % de la ventana (izq y ancho: del ancho; arriba y alto: del alto). */
data class Caja(val izq: Float, val arriba: Float, val ancho: Float, val alto: Float)

/** Un rectángulo en píxeles (o dp), ya calculado para un ancho de ventana. */
data class Rect(val x: Float, val y: Float, val ancho: Float, val alto: Float)

/**
 * La foto ocupa una ventana 4:5. `foto*` es la posición de la imagen (en % de
 * la ventana) y `pantalla` la caja de la pantalla encima de esa imagen.
 */
data class Encuadre(
    val foto: String,
    val alt: String,
    val fotoAncho: Float,
    val fotoIzq: Float,
    val fotoArriba: Float,
    val proporcionFoto: Float, // alto / ancho de la imagen
    val pantalla: Caja,
    val fondo: Long,
) {
    /** Dónde va la imagen, para una ventana de este ancho (el alto es ancho × 5/4). */
    fun rectFoto(ventana: Float): Rect {
        val alto = ventana * 5 / 4
        val w = ventana * fotoAncho / 100
        return Rect(ventana * fotoIzq / 100, alto * fotoArriba / 100, w, w * proporcionFoto)
    }

    fun rectPantalla(ventana: Float): Rect {
        val alto = ventana * 5 / 4
        return Rect(ventana * pantalla.izq / 100, alto * pantalla.arriba / 100, ventana * pantalla.ancho / 100, alto * pantalla.alto / 100)
    }

    /**
     * Dónde va la pantalla dentro de su caja. La redonda la llena. Una
     * apaisada en una caja muy de pie se agranda (185 %) para que la cara
     * llene el ancho (los costados, que suelen estar vacíos, quedan afuera);
     * si no, entra entera (contain), centrada.
     */
    fun encastre(ventana: Float, redonda: Boolean, pantallaAncho: Int, pantallaAlto: Int): Rect {
        val caja = rectPantalla(ventana)
        if (redonda) return Rect(0f, 0f, caja.ancho, caja.alto)
        if (pantalla.alto * 5 > pantalla.ancho * 4 * 1.5f) {
            val w = caja.ancho * 1.85f
            val h = w * pantallaAlto / pantallaAncho
            return Rect((caja.ancho - w) / 2, (caja.alto - h) / 2, w, h)
        }
        val k = minOf(caja.ancho / pantallaAncho, caja.alto / pantallaAlto)
        val w = pantallaAncho * k
        val h = pantallaAlto * k
        return Rect((caja.ancho - w) / 2, (caja.alto - h) / 2, w, h)
    }
}

val RECTANGULAR = Encuadre(
    foto = "unidad", alt = "La Unidad sobre un escritorio",
    fotoAncho = 100f, fotoIzq = 0f, fotoArriba = 0f, proporcionFoto = 1024f / 819f,
    pantalla = Caja(izq = 23.5f, arriba = 29.5f, ancho = 43.5f, alto = 39.5f),
    fondo = 0xFFE9E2D8,
)

val REDONDA = Encuadre(
    foto = "pieza_colgante", alt = "El robot colgado de una cadena",
    fotoAncho = 160f, fotoIzq = -28.2f, fotoArriba = -43.1f, proporcionFoto = 1125f / 900f,
    pantalla = Caja(izq = 26.8f, arriba = 36.9f, ancho = 46.2f, alto = 37f),
    fondo = 0xFFF3EEEA,
)

data class Modelo(val nombre: String, val redonda: Boolean, val encuadre: Encuadre, val capacidades: Set<Capacidad>, val pantalla: Pantalla)

fun pantallaDe(perfil: String, estado: Estado?): Pantalla {
    estado?.pantalla?.takeIf { it.ancho > 0 }?.let { return it }
    return if (perfil.contains("broche")) Pantalla(240, 240, true) else Pantalla(320, 172, false)
}

fun modeloDe(perfil: String, virtual: Boolean, estado: Estado?): Modelo {
    val p = pantallaDe(perfil, estado)
    val broche = perfil.contains("broche")
    // Los virtuales traen todo, así se puede probar cada módulo sin hardware.
    val capacidades = if (virtual) Capacidad.entries.toSet()
    else setOf(Capacidad.PANTALLA, Capacidad.MOVIMIENTO, Capacidad.BLUETOOTH, Capacidad.WIFI)
    return Modelo(if (broche) "Broche" else "Unidad", p.redonda, if (p.redonda) REDONDA else RECTANGULAR, capacidades, p)
}

/** "Nº 001": los últimos dígitos del id, para grabar en la placa. */
fun numeroDe(id: String): String {
    val hex = id.filter { it.isDigit() || it.uppercaseChar() in 'A'..'F' }.takeLast(4).ifEmpty { "0" }
    val n = (hex.toLongOrNull(16) ?: 0L) % 1000
    return "Nº " + n.toString().padStart(3, '0')
}

/** El ánimo en palabras, a partir del humor (-1..1). */
fun animoDe(conectado: Boolean, e: Estado?): String = when {
    e == null -> if (conectado) "Despertando" else "Sin noticias"
    e.dormido -> "Durmiendo"
    e.actualizando -> "Actualizándose"
    e.humor > 0.5f -> "Muy contento"
    e.humor > 0.15f -> "Contento"
    e.humor < -0.5f -> "Bajoneado"
    e.humor < -0.15f -> "Medio molesto"
    else -> "Tranquilo"
}

/** "hace 5 min": cuánto pasó desde `cuando` (ms) hasta `ahora` (ms). */
fun hace(cuando: Long?, ahora: Long = System.currentTimeMillis()): String {
    if (cuando == null) return "nunca"
    val s = (ahora - cuando) / 1000.0
    return when {
        s < 90 -> "recién"
        s < 3600 -> "hace ${Math.round(s / 60)} min"
        s < 86400 -> "hace ${Math.round(s / 3600)} h"
        else -> "hace ${Math.round(s / 86400)} días"
    }
}

/** Los estados del sistema no son caras: la vista previa no los fija. */
fun esEstadoDelSistema(expresion: String) = Regex("^(recordatorio|pomodoro|reloj|imagen|protector):").containsMatchIn(expresion)

/** "Soltar la cara" solo cuando lo fijó alguien (no un recordatorio, el pomodoro o el reloj). */
fun sePuedeSoltar(e: Estado?) = e != null && e.fija && !Regex("^(recordatorio|pomodoro|reloj):").containsMatchIn(e.expresion)
