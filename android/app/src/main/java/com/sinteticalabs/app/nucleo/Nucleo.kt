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

    /** redonda: el broche; broche: con el pulsador del broche (toque = siguiente). */
    fun iniciar(ancho: Int, alto: Int, identificador: String, anfitrion: Anfitrion?, redonda: Boolean = false, broche: Boolean = false) =
        iniciarNativo(ancho, alto, identificador, anfitrion, redonda, broche)

    private external fun iniciarNativo(ancho: Int, alto: Int, identificador: String, anfitrion: Anfitrion?, redonda: Boolean, broche: Boolean)
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
    external fun imagenVisible(): Int
    external fun version(): String

    /** Adelanta el reloj del núcleo (para sacar fotos sin esperar los fundidos). */
    external fun adelantar(ms: Int)

    private external fun validar(json: ByteArray): String?
    private external fun dibujarNativo(json: ByteArray?, nombre: String, ancho: Int, alto: Int, redonda: Boolean, t: Float, cierre: Float): IntArray?
    private external fun vidaBase(tipo: Int): Array<String>
    private external fun spriteSuelto(sprite: Int, qoi: ByteArray?): Int

    /** Un sprite para dibujar las caras de imagen fuera de la instancia (null lo borra). */
    fun ponerSpriteSuelto(sprite: Int, qoi: ByteArray?): Boolean = spriteSuelto(sprite, qoi) == 0

    /** Valida una config con el mismo núcleo del robot: null si está bien; si no, por qué. */
    fun validarConfig(json: String): String? = validar(json.toByteArray())

    /**
     * Dibuja una cara paramétrica sin tocar la instancia (miniaturas, el
     * laboratorio y el dock), en ARGB. Si la config no sirve, prueba con las de
     * fábrica; null si no hay una cara paramétrica con ese nombre.
     */
    fun dibujar(config: String?, nombre: String, ancho: Int, alto: Int, redonda: Boolean, t: Float, cierre: Float = 0f): IntArray? =
        dibujarNativo(config?.toByteArray(), nombre, ancho, alto, redonda, t, cierre)
            ?: if (config != null) dibujarNativo(null, nombre, ancho, alto, redonda, t, cierre) else null

    /** Lo de fábrica del motor de vida: 0 = expresiones, 1 = comportamientos (nombre → JSON). */
    fun deFabrica(tipo: Int): List<Pair<String, String>> = vidaBase(tipo).toList().chunked(2).map { it[0] to it[1] }
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
