package com.sinteticalabs.app.bicho

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RectF
import com.sinteticalabs.app.protocolo.Animacion
import com.sinteticalabs.app.protocolo.Qoi

/** Recorta y escala una imagen a la pantalla del bicho, y la codifica en QOI. */
object PrepararImagen {
    private const val PANTALLA = 0xFF0B0A09.toInt()

    /** Cubre la pantalla centrada (como un fondo de pantalla); si es redonda, fuera del círculo va negro. */
    fun recortar(fuente: Bitmap, ancho: Int, alto: Int, redonda: Boolean): Bitmap {
        val salida = Bitmap.createBitmap(ancho, alto, Bitmap.Config.ARGB_8888)
        val c = Canvas(salida)
        c.drawColor(PANTALLA)
        if (redonda) c.clipPath(Path().apply { addOval(RectF(0f, 0f, ancho.toFloat(), alto.toFloat()), Path.Direction.CW) })
        val k = maxOf(ancho.toFloat() / fuente.width, alto.toFloat() / fuente.height)
        val w = fuente.width * k
        val h = fuente.height * k
        c.drawBitmap(fuente, null, RectF((ancho - w) / 2, (alto - h) / 2, (ancho + w) / 2, (alto + h) / 2), Paint(Paint.FILTER_BITMAP_FLAG))
        return salida
    }

    fun qoi(b: Bitmap): ByteArray {
        val px = IntArray(b.width * b.height)
        b.getPixels(px, 0, b.width, 0, 0, b.width, b.height)
        val rgb = ByteArray(px.size * 3)
        for (i in px.indices) {
            rgb[i * 3] = (px[i] shr 16).toByte()
            rgb[i * 3 + 1] = (px[i] shr 8).toByte()
            rgb[i * 3 + 2] = px[i].toByte()
        }
        return Qoi.codificar(rgb, b.width, b.height)
    }

    /**
     * Un GIF a la pantalla: cada cuadro recortado como una foto, en una animación
     * "sani" (o un QOI si tiene un solo cuadro). Si no entra en un espacio, se
     * saltea uno de cada dos cuadros y el que queda dura lo de los dos, como la web.
     */
    fun gif(bytes: ByteArray, ancho: Int, alto: Int, redonda: Boolean): ByteArray {
        val g = Gif.decodificar(bytes)
        var cuadros = g.cuadros.map { c ->
            val fuente = Bitmap.createBitmap(c.argb, g.ancho, g.alto, Bitmap.Config.ARGB_8888)
            Animacion.Cuadro(qoi(recortar(fuente, ancho, alto, redonda)), c.ms)
        }
        if (cuadros.size == 1) return cuadros[0].qoi
        val limite = ancho * alto * 4 + 22
        var sani = Animacion.codificar(cuadros.take(Animacion.MAX_CUADROS), ancho, alto)
        while (cuadros.size > Animacion.MAX_CUADROS || sani.size > limite) {
            require(cuadros.size > 2) { "El GIF es demasiado pesado para la pantalla: probá con uno más simple." }
            cuadros = cuadros.chunked(2).map { par -> Animacion.Cuadro(par[0].qoi, par.sumOf { it.ms }) }
            sani = Animacion.codificar(cuadros.take(Animacion.MAX_CUADROS), ancho, alto)
        }
        return sani
    }

    /** Para las miniaturas: decodifica un QOI (o el primer cuadro de una animación) a Bitmap. */
    fun decodificar(imagen: ByteArray): Bitmap? {
        val q = Animacion.cuadros(imagen).first().qoi
        if (q.size < 22 || q[0] != 'q'.code.toByte()) return null
        fun be(i: Int) = ((q[i].toInt() and 255) shl 24) or ((q[i + 1].toInt() and 255) shl 16) or ((q[i + 2].toInt() and 255) shl 8) or (q[i + 3].toInt() and 255)
        val w = be(4); val h = be(8)
        val px = IntArray(w * h)
        val ind = IntArray(64)
        var r = 0; var g = 0; var b = 0; var a = 255; var p = 14; var racha = 0
        for (i in 0 until w * h) {
            if (racha > 0) racha-- else {
                val c = q[p++].toInt() and 255
                when {
                    c == 0xfe -> { r = q[p++].toInt() and 255; g = q[p++].toInt() and 255; b = q[p++].toInt() and 255 }
                    c == 0xff -> { r = q[p++].toInt() and 255; g = q[p++].toInt() and 255; b = q[p++].toInt() and 255; a = q[p++].toInt() and 255 }
                    c and 0xc0 == 0x00 -> { val v = ind[c]; a = v ushr 24; r = v shr 16 and 255; g = v shr 8 and 255; b = v and 255 }
                    c and 0xc0 == 0x40 -> { r = (r + (c shr 4 and 3) - 2) and 255; g = (g + (c shr 2 and 3) - 2) and 255; b = (b + (c and 3) - 2) and 255 }
                    c and 0xc0 == 0x80 -> {
                        val c2 = q[p++].toInt() and 255; val vg = (c and 0x3f) - 32
                        r = (r + vg - 8 + (c2 shr 4 and 15)) and 255; g = (g + vg) and 255; b = (b + vg - 8 + (c2 and 15)) and 255
                    }
                    else -> racha = c and 0x3f
                }
                ind[(r * 3 + g * 5 + b * 7 + a * 11) % 64] = (a shl 24) or (r shl 16) or (g shl 8) or b
            }
            px[i] = (0xFF shl 24) or (r shl 16) or (g shl 8) or b
        }
        return Bitmap.createBitmap(px, w, h, Bitmap.Config.ARGB_8888)
    }
}
