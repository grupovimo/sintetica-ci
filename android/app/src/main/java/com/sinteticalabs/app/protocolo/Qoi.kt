package com.sinteticalabs.app.protocolo

/**
 * Codificador QOI ("Quite OK Image", https://qoiformat.org): el formato de las
 * imágenes del bicho. Igual que sim/web/qoi.js (los tests comparan los bytes
 * contra shared/protocolo/vectores.json).
 */
object Qoi {
    /** rgb: 3 bytes por píxel, fila por fila. */
    fun codificar(rgb: ByteArray, ancho: Int, alto: Int): ByteArray {
        val s = java.io.ByteArrayOutputStream(ancho * alto + 64)
        fun u32(v: Int) { s.write(v ushr 24 and 255); s.write(v ushr 16 and 255); s.write(v ushr 8 and 255); s.write(v and 255) }
        s.write(byteArrayOf(0x71, 0x6f, 0x69, 0x66)); u32(ancho); u32(alto); s.write(3); s.write(0)
        val indice = IntArray(64) { -1 }
        var pr = 0; var pg = 0; var pb = 0; var racha = 0
        val total = ancho * alto
        for (i in 0 until total) {
            val r = rgb[i * 3].toInt() and 255; val g = rgb[i * 3 + 1].toInt() and 255; val b = rgb[i * 3 + 2].toInt() and 255
            if (r == pr && g == pg && b == pb) {  // el "anterior" del primer píxel es negro, como en QOI
                racha++
                if (racha == 62 || i == total - 1) { s.write(0xc0 or (racha - 1)); racha = 0 }
                continue
            }
            if (racha > 0) { s.write(0xc0 or (racha - 1)); racha = 0 }
            val h = (r * 3 + g * 5 + b * 7 + 255 * 11) % 64
            val color = (r shl 16) or (g shl 8) or b
            if (indice[h] == color) {
                s.write(h)
            } else {
                indice[h] = color
                val vr = (r - pr).toByte().toInt(); val vg = (g - pg).toByte().toInt(); val vb = (b - pb).toByte().toInt()
                val vgr = vr - vg; val vgb = vb - vg
                if (vr in -2..1 && vg in -2..1 && vb in -2..1) {
                    s.write(0x40 or ((vr + 2) shl 4) or ((vg + 2) shl 2) or (vb + 2))
                } else if (vgr in -8..7 && vg in -32..31 && vgb in -8..7) {
                    s.write(0x80 or (vg + 32)); s.write(((vgr + 8) shl 4) or (vgb + 8))
                } else {
                    s.write(0xfe); s.write(r); s.write(g); s.write(b)
                }
            }
            pr = r; pg = g; pb = b
        }
        s.write(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 1))
        return s.toByteArray()
    }

    /** rgba: 4 bytes por píxel (con transparencia), para los sprites de las caras de imagen. */
    fun codificarRgba(rgba: ByteArray, ancho: Int, alto: Int): ByteArray {
        val s = java.io.ByteArrayOutputStream(ancho * alto + 64)
        fun u32(v: Int) { s.write(v ushr 24 and 255); s.write(v ushr 16 and 255); s.write(v ushr 8 and 255); s.write(v and 255) }
        s.write(byteArrayOf(0x71, 0x6f, 0x69, 0x66)); u32(ancho); u32(alto); s.write(4); s.write(0)
        val indice = IntArray(64) { -1 }
        var pr = 0; var pg = 0; var pb = 0; var pa = 255; var racha = 0
        val total = ancho * alto
        for (i in 0 until total) {
            val r = rgba[i * 4].toInt() and 255; val g = rgba[i * 4 + 1].toInt() and 255
            val b = rgba[i * 4 + 2].toInt() and 255; val a = rgba[i * 4 + 3].toInt() and 255
            if (r == pr && g == pg && b == pb && a == pa) {
                racha++
                if (racha == 62 || i == total - 1) { s.write(0xc0 or (racha - 1)); racha = 0 }
                continue
            }
            if (racha > 0) { s.write(0xc0 or (racha - 1)); racha = 0 }
            val h = (r * 3 + g * 5 + b * 7 + a * 11) % 64
            val color = (a shl 24) or (r shl 16) or (g shl 8) or b
            if (indice[h] == color) {
                s.write(h)
            } else {
                indice[h] = color
                if (a == pa) {
                    val vr = (r - pr).toByte().toInt(); val vg = (g - pg).toByte().toInt(); val vb = (b - pb).toByte().toInt()
                    val vgr = vr - vg; val vgb = vb - vg
                    if (vr in -2..1 && vg in -2..1 && vb in -2..1) {
                        s.write(0x40 or ((vr + 2) shl 4) or ((vg + 2) shl 2) or (vb + 2))
                    } else if (vgr in -8..7 && vg in -32..31 && vgb in -8..7) {
                        s.write(0x80 or (vg + 32)); s.write(((vgr + 8) shl 4) or (vgb + 8))
                    } else {
                        s.write(0xfe); s.write(r); s.write(g); s.write(b)
                    }
                } else {
                    s.write(0xff); s.write(r); s.write(g); s.write(b); s.write(a)
                }
            }
            pr = r; pg = g; pb = b; pa = a
        }
        s.write(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 1))
        return s.toByteArray()
    }
}
