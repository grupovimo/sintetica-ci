package com.sinteticalabs.app.nucleo

/** Lo que el núcleo le pide a la app: guardar datos, notificaciones BLE y registro. */
interface Anfitrion {
    fun guardar(clave: String, datos: ByteArray): Boolean
    fun leer(clave: String): ByteArray?
    fun notificar(caracteristica: Int, datos: ByteArray)
    fun registrar(mensaje: String)
}

/**
 * El núcleo de SintéticaOS (el mismo C++ del firmware), por JNI. Hay una sola
 * instancia por proceso y se usa siempre desde el hilo principal.
 */
object Nucleo {
    init {
        System.loadLibrary("sintetica")
    }

    external fun iniciar(ancho: Int, alto: Int, identificador: String, anfitrion: Anfitrion?)
    external fun vistaPrevia()
    external fun tick()
    external fun pixeles(destino: IntArray)
    external fun ancho(): Int
    external fun alto(): Int
    external fun tacto(apretado: Boolean)
    external fun evento(evento: Int)
    external fun bleConexion(conectado: Boolean)
    external fun bleEscribir(caracteristica: Int, datos: ByteArray)
    external fun estado(): ByteArray
    external fun expresiones(): Array<String>
    external fun expresionVisible(): String
    external fun qr(): String
    external fun version(): String
}

/** Un anfitrión que guarda en memoria (vista previa y bicho virtual). */
open class AnfitrionEnMemoria : Anfitrion {
    private val datos = mutableMapOf<String, ByteArray>()
    var alNotificar: (Int, ByteArray) -> Unit = { _, _ -> }
    override fun guardar(clave: String, datos: ByteArray): Boolean { this.datos[clave] = datos; return true }
    override fun leer(clave: String): ByteArray? = datos[clave]
    override fun notificar(caracteristica: Int, datos: ByteArray) = alNotificar(caracteristica, datos)
    override fun registrar(mensaje: String) {}
}
