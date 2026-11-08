package com.sinteticalabs.bicho

/**
 * El núcleo de SintéticaOS por JNI. Hay una sola instancia por proceso y todo se
 * llama desde el hilo de GL (salvo [bateria], que es atómica).
 */
object Nucleo {
    init {
        System.loadLibrary("bicho")
    }

    /**
     * Lo que el núcleo le pide al celu: guardar y leer (la memoria), los
     * registros y lo que sale por "BLE" y por la nube. Se llama en el hilo de GL.
     */
    interface Anfitrion {
        fun guardar(clave: String, datos: ByteArray): Boolean
        fun leer(clave: String): ByteArray?
        fun registrar(mensaje: String)
        fun notificar(caracteristica: Int, datos: ByteArray)
        fun notificarNube(caracteristica: Int, datos: ByteArray)
        fun redCambio()
    }

    /** Una sola vez por proceso: si GL recrea la superficie, el núcleo sigue igual. */
    /** [carpeta]: donde el núcleo guarda los archivos grandes (imágenes y sprites), por partes. */
    fun iniciar(ancho: Int, alto: Int, id: String, carpeta: String, anfitrion: Anfitrion) {
        if (!listo()) iniciarNativo(ancho, alto, id, carpeta, anfitrion)
    }
    external fun listo(): Boolean
    private external fun iniciarNativo(ancho: Int, alto: Int, id: String, carpeta: String, anfitrion: Anfitrion)

    external fun tick()
    /** Sube el cuadro a la textura ligada; [crear] la primera vez. */
    external fun subirCuadro(crear: Boolean)
    external fun ancho(): Int
    external fun alto(): Int
    external fun tacto(apretado: Boolean)
    /** Aceleración en g, en ejes de la pantalla (z sale hacia el frente). */
    external fun imu(ax: Float, ay: Float, az: Float)
    external fun evento(evento: Int)
    /** Un evento por su nombre del JSON ("sacudida-fuerte"); false si no existe. */
    external fun eventoPorNombre(nombre: String): Boolean
    /** Para desarrollar: se vincula solo, como si un celu escaneara su QR. */
    external fun vincularSolo(): Boolean
    /** El reloj de pared del bicho ×[factor] (720 = un día en 2 minutos; 1 = la hora real). */
    external fun acelerar(factor: Int)
    external fun minutoDelDia(): Int
    external fun bleConexion(conectado: Boolean)
    external fun bleEscribir(caracteristica: Int, datos: ByteArray)
    external fun nubeEscribir(caracteristica: Int, datos: ByteArray)
    external fun conexiones(wifi: Boolean, nube: Boolean)
    /** El estado (característica 3) tal cual lo arma el núcleo. */
    external fun estado(): ByteArray
    /** {url, usuario, clave} o null si todavía no lo sumaron a una cuenta. */
    external fun nube(): Array<String>?
    external fun qr(): String
    external fun bateria(nivel: Int)
    external fun expresionVisible(): String
    external fun version(): String
}
