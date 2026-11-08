package com.sinteticalabs.bicho

import android.graphics.Color
import android.graphics.Typeface
import android.os.Handler
import android.os.Looper
import android.widget.TextView

/** La superposición de depuración: lo que pasa adentro, actualizado dos veces por segundo. */
class Datos(
    private val texto: TextView,
    private val lienzo: Lienzo,
    private val sentidos: Sentidos,
    private val depuracion: Depuracion,
) {
    private val reloj = Handler(Looper.getMainLooper())
    @Volatile private var expresion = ""
    @Volatile private var minuto = 0
    var visible = false
        private set

    init {
        texto.setTextColor(Color.WHITE)
        texto.setShadowLayer(4f, 0f, 0f, Color.BLACK)
        texto.typeface = Typeface.MONOSPACE
        texto.textSize = 13f
        texto.setPadding(24, 16, 24, 16)
    }

    fun alternar() {
        visible = !visible
        texto.visibility = if (visible) TextView.VISIBLE else TextView.GONE
        reloj.removeCallbacks(paso)
        if (visible) reloj.post(paso)
    }

    /** Lo pide el hilo de GL (el núcleo se lee solo desde ahí). */
    fun leerNucleo() {
        if (!visible) return
        expresion = Nucleo.expresionVisible()
        minuto = Nucleo.minutoDelDia()
    }

    private val paso = object : Runnable {
        override fun run() {
            texto.text = buildString {
                appendLine("%.1f fps".format(lienzo.fps))
                appendLine("cara      $expresion")
                appendLine("hora      %02d:%02d  ×%d".format(minuto / 60, minuto % 60, depuracion.factor))
                appendLine("batería   %.1f °C".format(sentidos.temperatura))
                appendLine("luz       %.0f lux".format(sentidos.lux))
                appendLine("ojos      " + (sentidos.cara?.let { "te veo %+.2f %+.2f".format(it.first, it.second) } ?: "·"))
                append("oído      ").appendLine("▮".repeat((sentidos.nivelOido * 20).toInt()).ifEmpty { "·" })
            }
            reloj.postDelayed(this, 500)
        }
    }
}
