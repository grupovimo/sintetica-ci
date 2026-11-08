package com.sinteticalabs.bicho

import android.opengl.GLES20.*
import android.opengl.GLSurfaceView
import android.util.Log
import java.nio.ByteBuffer
import java.nio.ByteOrder
import javax.microedition.khronos.egl.EGLConfig
import javax.microedition.khronos.opengles.GL10

/**
 * Dibuja la cara: el núcleo pinta el cuadro (a la mitad de la pantalla) y acá se
 * sube como textura y se estira a pantalla completa con filtro lineal.
 */
class Lienzo(
    private val ancho: Int,
    private val alto: Int,
    private val id: String,
    private val carpeta: String,
    private val anfitrion: Nucleo.Anfitrion,
) :
    GLSurfaceView.Renderer {

    private var programa = 0
    private var textura = 0
    private var creada = false
    private var cuadros = 0
    private var desde = 0L

    /** Cuadros por segundo de los últimos 5 s (para la depuración). */
    @Volatile var fps = 0f
        private set

    /** Se llama en cada cuadro, en el hilo de GL, después del tick. */
    var alDibujar: (() -> Unit)? = null

    override fun onSurfaceCreated(gl: GL10?, config: EGLConfig?) {
        Nucleo.iniciar(ancho, alto, id, carpeta, anfitrion)
        Log.i("Bicho", "núcleo ${Nucleo.version()} a ${Nucleo.ancho()}×${Nucleo.alto()}")
        programa = programa(VERTICES, FRAGMENTOS)
        val t = IntArray(1)
        glGenTextures(1, t, 0)
        textura = t[0]
        glBindTexture(GL_TEXTURE_2D, textura)
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR)
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR)
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE)
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE)
        creada = false
    }

    override fun onSurfaceChanged(gl: GL10?, w: Int, h: Int) = glViewport(0, 0, w, h)

    override fun onDrawFrame(gl: GL10?) {
        esperar()
        Nucleo.tick()
        glBindTexture(GL_TEXTURE_2D, textura)
        Nucleo.subirCuadro(!creada)
        creada = true
        glUseProgram(programa)
        val pos = glGetAttribLocation(programa, "pos")
        glEnableVertexAttribArray(pos)
        glVertexAttribPointer(pos, 2, GL_FLOAT, false, 0, QUAD)
        glDrawArrays(GL_TRIANGLE_STRIP, 0, 4)
        alDibujar?.invoke()
        contar()
    }

    private var ultimo = 0L

    /** Cuadros por segundo como máximo: 30 (lo que pide el núcleo), menos si hace calor. */
    @Volatile var tope = 30

    private fun esperar() {
        val falta = ultimo + 1_000_000_000L / tope - System.nanoTime()
        if (falta > 1_000_000L) Thread.sleep(falta / 1_000_000L)
        ultimo = System.nanoTime()
    }

    private fun contar() {
        val ahora = System.nanoTime()
        if (desde == 0L) desde = ahora
        cuadros++
        if (ahora - desde >= 5_000_000_000L) {
            fps = cuadros * 1e9f / (ahora - desde)
            Log.i("Bicho", "fps %.1f · %s".format(fps, Nucleo.expresionVisible()))
            cuadros = 0
            desde = ahora
        }
    }

    private fun programa(v: String, f: String): Int {
        fun sombreador(tipo: Int, fuente: String) = glCreateShader(tipo).also {
            glShaderSource(it, fuente)
            glCompileShader(it)
        }
        return glCreateProgram().also {
            glAttachShader(it, sombreador(GL_VERTEX_SHADER, v))
            glAttachShader(it, sombreador(GL_FRAGMENT_SHADER, f))
            glLinkProgram(it)
        }
    }

    private companion object {
        val QUAD = ByteBuffer.allocateDirect(8 * 4).order(ByteOrder.nativeOrder()).asFloatBuffer().apply {
            put(floatArrayOf(-1f, -1f, 1f, -1f, -1f, 1f, 1f, 1f))
            position(0)
        }
        const val VERTICES = """
            attribute vec2 pos;
            varying vec2 uv;
            void main() {
                uv = vec2(pos.x * 0.5 + 0.5, 0.5 - pos.y * 0.5);
                gl_Position = vec4(pos, 0.0, 1.0);
            }"""
        const val FRAGMENTOS = """
            precision mediump float;
            varying vec2 uv;
            uniform sampler2D cuadro;
            void main() { gl_FragColor = texture2D(cuadro, uv); }"""
    }
}
