package com.sinteticalabs.app.bicho

/**
 * Decodificador de GIF (animados también), igual que sim/web/gif.js: cada
 * cuadro ya compuesto (la imagen entera, como se ve) en ARGB, con su duración.
 * Kotlin puro, para poder probarlo sin Android.
 */
object Gif {
    class Cuadro(val argb: IntArray, val ms: Int)
    class Imagen(val ancho: Int, val alto: Int, val cuadros: List<Cuadro>)

    fun esGif(b: ByteArray) = b.size > 6 && b[0] == 'G'.code.toByte() && b[1] == 'I'.code.toByte() && b[2] == 'F'.code.toByte()

    fun decodificar(b: ByteArray): Imagen {
        require(esGif(b)) { "no es un GIF" }
        fun u8(i: Int) = b[i].toInt() and 255
        fun u16(i: Int) = u8(i) or (u8(i + 1) shl 8)
        val ancho = u16(6); val alto = u16(8)
        require(ancho > 0 && alto > 0) { "GIF sin tamaño" }
        var p = 13
        var global: IntArray? = null
        fun paleta(empaquetado: Int): IntArray {
            val n = 1 shl ((empaquetado and 7) + 1)
            val t = IntArray(n) { k -> (0xFF shl 24) or (u8(p + 3 * k) shl 16) or (u8(p + 3 * k + 1) shl 8) or u8(p + 3 * k + 2) }
            p += 3 * n
            return t
        }
        if (u8(10) and 0x80 != 0) global = paleta(u8(10))
        fun subBloques(): ByteArray {
            val s = java.io.ByteArrayOutputStream()
            while (p < b.size) {
                val n = u8(p++)
                if (n == 0) break
                s.write(b, p, minOf(n, b.size - p)); p += n
            }
            return s.toByteArray()
        }

        val lienzo = IntArray(ancho * alto)  // transparente
        val cuadros = mutableListOf<Cuadro>()
        var reemplazo = 0; var transparente = -1; var ms = 100
        var anterior: IntArray? = null
        var ultimo: IntArray? = null  // x, y, w, h, reemplazo del cuadro anterior
        while (p < b.size) {
            when (u8(p++)) {
                0x3b -> break
                0x21 -> {
                    val etiqueta = u8(p++)
                    if (etiqueta == 0xf9 && u8(p) == 4) {
                        val emp = u8(p + 1); val demora = u16(p + 2)
                        reemplazo = (emp shr 2) and 7
                        transparente = if (emp and 1 != 0) u8(p + 4) else -1
                        ms = if (demora < 2) 100 else demora * 10  // como los navegadores
                        p += 5
                    }
                    subBloques()
                }
                0x2c -> {
                    ultimo?.let { u ->
                        if (u[4] == 2) for (yy in u[1] until minOf(u[1] + u[3], alto)) for (xx in u[0] until minOf(u[0] + u[2], ancho)) lienzo[yy * ancho + xx] = 0
                        else if (u[4] == 3) anterior?.copyInto(lienzo)
                    }
                    val x = u16(p); val y = u16(p + 2); val w = u16(p + 4); val h = u16(p + 6); val emp = u8(p + 8)
                    p += 9
                    val tabla = if (emp and 0x80 != 0) paleta(emp) else global ?: error("GIF sin paleta")
                    val minimo = u8(p++)
                    val indices = lzw(minimo, subBloques(), w * h)
                    val filas = if (emp and 0x40 != 0) entrelazado(h) else null
                    anterior = if (reemplazo == 3) lienzo.copyOf() else null
                    for (fy in 0 until h) {
                        val yy = y + (filas?.get(fy) ?: fy)
                        if (yy >= alto) continue
                        for (fx in 0 until w) {
                            val xx = x + fx
                            if (xx >= ancho) continue
                            val k = indices[fy * w + fx].toInt() and 255
                            if (k == transparente || k >= tabla.size) continue
                            lienzo[yy * ancho + xx] = tabla[k]
                        }
                    }
                    cuadros += Cuadro(lienzo.copyOf(), ms)
                    ultimo = intArrayOf(x, y, w, h, reemplazo)
                    reemplazo = 0; transparente = -1; ms = 100
                }
                else -> error("GIF roto")
            }
        }
        require(cuadros.isNotEmpty()) { "GIF sin cuadros" }
        return Imagen(ancho, alto, cuadros)
    }

    private fun entrelazado(h: Int): IntArray {
        val filas = ArrayList<Int>(h)
        for ((desde, paso) in listOf(0 to 8, 4 to 8, 2 to 4, 1 to 2)) for (y in desde until h step paso) filas += y
        return filas.toIntArray()
    }

    private fun lzw(minimo: Int, datos: ByteArray, n: Int): ByteArray {
        val salida = ByteArray(n)
        val limpio = 1 shl minimo; val fin = limpio + 1
        val prefijo = IntArray(4096); val sufijo = ByteArray(4096); val pila = ByteArray(4097)
        for (i in 0 until limpio) sufijo[i] = i.toByte()
        var largo = minimo + 1; var siguiente = fin + 1; var previo = -1; var primero = 0
        var acumulado = 0; var bits = 0; var pos = 0; var o = 0
        while (o < n) {
            while (bits < largo) {
                if (pos >= datos.size) return salida
                acumulado = acumulado or ((datos[pos++].toInt() and 255) shl bits); bits += 8
            }
            val codigo = acumulado and ((1 shl largo) - 1)
            acumulado = acumulado ushr largo; bits -= largo
            if (codigo == limpio) { largo = minimo + 1; siguiente = fin + 1; previo = -1; continue }
            if (codigo == fin) break
            if (previo == -1) { salida[o++] = sufijo[codigo]; previo = codigo; primero = codigo; continue }
            var c = codigo; var sp = 0
            if (codigo >= siguiente) { pila[sp++] = primero.toByte(); c = previo }
            while (c >= limpio) { pila[sp++] = sufijo[c]; c = prefijo[c] }
            pila[sp++] = c.toByte(); primero = c
            while (sp > 0 && o < n) salida[o++] = pila[--sp]
            if (siguiente < 4096) {
                prefijo[siguiente] = previo; sufijo[siguiente] = primero.toByte(); siguiente++
                if (siguiente == (1 shl largo) && largo < 12) largo++
            }
            previo = codigo
        }
        return salida
    }
}
