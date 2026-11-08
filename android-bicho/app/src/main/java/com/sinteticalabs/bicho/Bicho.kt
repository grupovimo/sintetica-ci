package com.sinteticalabs.bicho

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.opengl.GLSurfaceView
import android.os.Bundle
import android.view.MotionEvent
import android.view.View
import android.view.Gravity
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.Button
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView

/** La única pantalla: la cara a pantalla completa, apaisada y siempre prendida. */
class Bicho : Activity() {
    private lateinit var vista: GLSurfaceView
    private lateinit var depuracion: Depuracion
    private lateinit var sentidos: Sentidos
    private lateinit var datos: Datos
    private lateinit var raiz: FrameLayout
    private var menu: View? = null
    private val secreto by lazy { Secreto(resources.displayMetrics.density * 80) { abrirMenu() } }

    override fun onCreate(estado: Bundle?) {
        super.onCreate(estado)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        val red = Red(filesDir)
        val archivos = java.io.File(filesDir, "archivos").apply { mkdirs() }.path
        val lienzo = Lienzo(ANCHO, ALTO, red.id, archivos, Guardado(filesDir, red))
        vista = GLSurfaceView(this).apply {
            setEGLContextClientVersion(2)
            setRenderer(lienzo)
            renderMode = GLSurfaceView.RENDERMODE_CONTINUOUSLY
        }
        val texto = TextView(this).apply { visibility = View.GONE }
        raiz = FrameLayout(this).apply {
            addView(vista)
            addView(texto, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT))
        }
        setContentView(raiz)
        red.vista = vista
        red.prender()
        depuracion = Depuracion(vista) { datos.alternar() }
        registerReceiver(depuracion, IntentFilter(Depuracion.ACCION))
        sentidos = Sentidos(this, vista, lienzo)
        datos = Datos(texto, lienzo, sentidos, depuracion)
        lienzo.alDibujar = { datos.leerNucleo() }
    }

    override fun onDestroy() {
        unregisterReceiver(depuracion)
        super.onDestroy()
    }

    override fun onResume() {
        super.onResume()
        inmersivo()
        vista.onResume()
        Kiosco.entrar(this)
        sentidos.prender()
    }

    override fun onPause() {
        sentidos.apagar()
        vista.onPause()
        super.onPause()
    }

    override fun onWindowFocusChanged(foco: Boolean) {
        super.onWindowFocusChanged(foco)
        if (foco) inmersivo()
    }

    override fun onTouchEvent(e: MotionEvent): Boolean {
        if (menu != null || secreto.mirar(e)) return true
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                soltadoAntes = false
                vista.queueEvent { Nucleo.tacto(true) }
                reloj.postDelayed(soltarLargo, TOQUE_MAX_MS)
            }
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                reloj.removeCallbacks(soltarLargo)
                if (!soltadoAntes) vista.queueEvent { Nucleo.tacto(false) }
            }
        }
        return true
    }

    // En el núcleo, mantener 5 s muestra el QR (como el botón de la placa); en el
    // celu, dejar el dedo es acariciarlo. Al núcleo le llega soltado antes de eso;
    // el QR se pide desde el menú secreto o por la red (/mostrar-qr).
    private val reloj = android.os.Handler(android.os.Looper.getMainLooper())
    private var soltadoAntes = false
    private val soltarLargo = Runnable {
        soltadoAntes = true
        vista.queueEvent { Nucleo.tacto(false) }
    }

    /** El menú de la salida secreta (sin diálogos del sistema). */
    private fun abrirMenu() {
        vista.queueEvent { Nucleo.tacto(false) }
        val caja = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER
            setBackgroundColor(0xE0101010.toInt())
        }
        fun boton(texto: String, accion: () -> Unit) = caja.addView(Button(this).apply {
            text = texto
            textSize = 20f
            setOnClickListener { accion() }
        }, LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT))
        boton("Volver al bicho") { cerrarMenu() }
        boton(if (datos.visible) "Ocultar datos" else "Ver datos") {
            cerrarMenu()
            datos.alternar()
        }
        boton("Mostrar el QR para vincular") {
            cerrarMenu()
            vista.queueEvent { Nucleo.tacto(true) }
            reloj.postDelayed({ vista.queueEvent { Nucleo.tacto(false) } }, 5200)
        }
        boton("Salir del kiosco") {
            cerrarMenu()
            Kiosco.salir(this)
        }
        boton("Soltar el celu (deja de ser dueño)") {
            cerrarMenu()
            Kiosco.soltar(this)
        }
        menu = caja
        raiz.addView(caja, FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
    }

    private fun cerrarMenu() {
        menu?.let { raiz.removeView(it) }
        menu = null
        inmersivo()
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent)
        // Abrir la app otra vez (o el botón de inicio) vuelve a armar el kiosco.
        if (!Kiosco.activo(this)) Kiosco.volver(this)
    }

    @Deprecated("El bicho no se cierra con atrás")
    override fun onBackPressed() = Unit

    @Suppress("DEPRECATION")
    private fun inmersivo() {
        window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY or
            View.SYSTEM_UI_FLAG_FULLSCREEN or View.SYSTEM_UI_FLAG_HIDE_NAVIGATION or
            View.SYSTEM_UI_FLAG_LAYOUT_STABLE or View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN or
            View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
    }

    private companion object {
        // La mitad de 1280×720: el núcleo dibuja acá y GL lo estira ×2.
        const val ANCHO = 640
        const val ALTO = 360
        const val TOQUE_MAX_MS = 4500L
    }
}
