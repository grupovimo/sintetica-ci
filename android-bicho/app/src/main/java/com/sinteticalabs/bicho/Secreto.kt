package com.sinteticalabs.bicho

import android.os.Handler
import android.os.Looper
import android.view.MotionEvent

/**
 * La salida secreta: 5 toques en la esquina superior izquierda (en menos de
 * 4 s) y después mantener apretado 2 s en la misma esquina.
 */
class Secreto(private val esquina: Float, private val alAbrir: () -> Unit) {
    private val toques = ArrayDeque<Long>()
    private var armado = false
    private val reloj = Handler(Looper.getMainLooper())
    private val abrir = Runnable {
        toques.clear()
        armado = false
        alAbrir()
    }

    /** Mira el toque; devuelve true si se lo come (el núcleo no lo ve). */
    fun mirar(e: MotionEvent): Boolean {
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                if (e.x >= esquina || e.y >= esquina) {
                    toques.clear()
                    armado = false
                    return false
                }
                if (armado) {
                    reloj.postDelayed(abrir, 2000)
                    return true
                }
                toques.addLast(e.eventTime)
                while (toques.isNotEmpty() && e.eventTime - toques.first() > 4000) toques.removeFirst()
                if (toques.size >= 5) armado = true
            }
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> reloj.removeCallbacks(abrir)
        }
        return false
    }
}
