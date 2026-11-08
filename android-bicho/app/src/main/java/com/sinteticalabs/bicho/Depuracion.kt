package com.sinteticalabs.bicho

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.opengl.GLSurfaceView
import android.util.Log

/**
 * Eventos por adb, para probar sin tocar el celu:
 *   adb shell am broadcast -a sintetica.EVENT --es tipo sacudida
 *   adb shell am broadcast -a sintetica.EVENT --es tipo vincular
 *   adb shell am broadcast -a sintetica.EVENT --es tipo tiempo --ei x 720   (un día en 2 min; 1 = normal)
 *   adb shell am broadcast -a sintetica.EVENT --es tipo depuracion          (muestra u oculta los datos)
 *   adb shell am broadcast -a sintetica.EVENT --es tipo comando --es hex 4202  (un comando del protocolo, como de la nube)
 *   adb shell am broadcast -a sintetica.EVENT --es tipo silencio --ez si true  (calla la voz; el despertador suena igual)
 */
class Depuracion(private val vista: GLSurfaceView, private val alternar: () -> Unit) : BroadcastReceiver() {
    @Volatile var factor = 1
        private set

    override fun onReceive(contexto: Context, intent: Intent) {
        val tipo = intent.getStringExtra("tipo") ?: return
        if (tipo == "depuracion") return alternar()
        if (tipo == "silencio") return Voz.silenciar(contexto, intent.getBooleanExtra("si", true))
        vista.queueEvent {
            val ok = when (tipo) {
                "vincular" -> Nucleo.vincularSolo()
                "comando" -> true.also {
                    val hex = intent.getStringExtra("hex").orEmpty()
                    Nucleo.nubeEscribir(2, ByteArray(hex.length / 2) { hex.substring(it * 2, it * 2 + 2).toInt(16).toByte() })
                }
                "tiempo" -> true.also {
                    factor = intent.getIntExtra("x", 1).coerceAtLeast(1)
                    Nucleo.acelerar(factor)
                }
                else -> Nucleo.eventoPorNombre(tipo)
            }
            Log.i("Bicho", "adb: $tipo ${if (ok) "ok" else "no existe"}")
        }
    }

    companion object {
        const val ACCION = "sintetica.EVENT"
    }
}
