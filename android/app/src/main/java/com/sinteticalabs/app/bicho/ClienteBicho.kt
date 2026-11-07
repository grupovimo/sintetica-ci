package com.sinteticalabs.app.bicho

import com.sinteticalabs.app.protocolo.ArmadorDeConfig
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.EspacioImagen
import com.sinteticalabs.app.protocolo.Estado
import com.sinteticalabs.app.protocolo.Imagen
import com.sinteticalabs.app.protocolo.escriturasDeImagen
import com.sinteticalabs.app.protocolo.leerListaImagenes
import com.sinteticalabs.app.protocolo.Op
import com.sinteticalabs.app.protocolo.Respuesta
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import com.sinteticalabs.app.protocolo.leerEstado
import com.sinteticalabs.app.protocolo.leerRespuesta
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.mapNotNull
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout

class ErrorBicho(mensaje: String) : Exception(mensaje)

/** Habla con un bicho por un transporte: comandos, estado y configuración. */
class ClienteBicho(private val transporte: Transporte, alcance: CoroutineScope) {
    private val _estado = MutableStateFlow<Estado?>(null)
    val estado: StateFlow<Estado?> = _estado

    init {
        alcance.launch {
            transporte.notificaciones.collect { (car, b) -> if (car == Car.ESTADO) runCatching { _estado.value = leerEstado(b) } }
        }
    }

    suspend fun conectar() {
        transporte.conectar()
        actualizarEstado()
    }

    suspend fun actualizarEstado(): Estado = leerEstado(transporte.leerEstado()).also { _estado.value = it }

    /** Escribe y espera la respuesta a esa misma operación. */
    private suspend fun pedir(car: Car, datos: ByteArray, opEsperado: Int = datos[0].toInt() and 0x7F, plazo: Long = 5_000): Respuesta.Resultado =
        coroutineScope {
            val espera = async(start = CoroutineStart.UNDISPATCHED) {
                transporte.notificaciones
                    .mapNotNull { (c, b) -> if (c == car) leerRespuesta(b) as? Respuesta.Resultado else null }
                    .first { it.op == opEsperado }
            }
            transporte.escribir(car, datos)
            withTimeout(plazo) { espera.await() }
        }

    private suspend fun exigir(car: Car, datos: ByteArray): Respuesta.Resultado {
        val r = pedir(car, datos)
        if (!r.ok) throw ErrorBicho(r.detalle.ifEmpty { r.mensaje })
        return r
    }

    /** Vincula con el código del QR y devuelve la clave que hay que guardar. */
    suspend fun vincular(codigo: String): ByteArray {
        val r = exigir(Car.COMANDO, Comando.vincular(codigo))
        mandarHora()
        actualizarEstado()
        return r.clave ?: throw ErrorBicho("El bicho no mandó la clave.")
    }

    suspend fun autenticar(clave: ByteArray) {
        exigir(Car.COMANDO, Comando.autenticar(clave))
        mandarHora()
        actualizarEstado()
    }

    /** Le pasa la hora del celu (el broche no tiene WiFi: es la única forma de saberla). */
    private suspend fun mandarHora() {
        val ahora = System.currentTimeMillis()
        val zona = java.util.TimeZone.getDefault().getOffset(ahora) / 60_000
        // Un firmware anterior a la v1.4 no lo entiende: no pasa nada.
        runCatching { pedir(Car.COMANDO, Comando.hora(ahora / 1000, zona)) }
    }

    suspend fun probarComportamiento(nombre: String) { exigir(Car.COMANDO, Comando.comportamiento(nombre)) }

    suspend fun fijarExpresion(nombre: String?) { exigir(Car.COMANDO, Comando.expresion(nombre ?: "")); actualizarEstado() }
    suspend fun cambiarNombre(nombre: String) { exigir(Car.COMANDO, Comando.nombre(nombre)); actualizarEstado() }
    suspend fun evento(nombre: String) { exigir(Car.COMANDO, Comando.evento(nombre)) }
    suspend fun desvincular() { exigir(Car.COMANDO, Comando.desvincular()) }
    suspend fun reproducir(animacion: String?) { exigir(Car.COMANDO, Comando.reproducir(animacion ?: "")); actualizarEstado() }

    /** El WiFi del bicho (solo por BLE). */
    suspend fun configurarRed(ssid: String, clave: String) { exigir(Car.COMANDO, Comando.red(ssid, clave)); actualizarEstado() }
    /** Las credenciales de la nube que dio la API al sumarlo a la cuenta (solo por BLE). */
    suspend fun configurarNube(url: String, usuario: String, clave: String) { exigir(Car.COMANDO, Comando.nube(url, usuario, clave)) }
    suspend fun olvidarRed() { exigir(Car.COMANDO, Comando.olvidarRed()); actualizarEstado() }

    /** Pide la configuración completa (JSON). */
    suspend fun pedirConfig(): String = coroutineScope {
        val armador = ArmadorDeConfig()
        val espera = async(start = CoroutineStart.UNDISPATCHED) {
            transporte.notificaciones
                .mapNotNull { (c, b) -> if (c == Car.CONFIG) leerRespuesta(b) else null }
                .mapNotNull { r ->
                    if (r is Respuesta.Resultado && !r.ok) throw ErrorBicho(r.detalle.ifEmpty { r.mensaje })
                    armador.agregar(r)
                }
                .first()
        }
        transporte.escribir(Car.CONFIG, Comando.pedirConfig())
        withTimeout(10_000) { espera.await() }
    }

    /** Manda una configuración entera; si el bicho la rechaza, explica por qué. */
    suspend fun mandarConfig(json: String) {
        val escrituras = escriturasDeConfig(json)
        coroutineScope {
            val fin = async(start = CoroutineStart.UNDISPATCHED) {
                transporte.notificaciones
                    .mapNotNull { (c, b) -> if (c == Car.CONFIG) leerRespuesta(b) as? Respuesta.Resultado else null }
                    .first { it.op == Op.CONFIG_FIN || !it.ok }
            }
            for (e in escrituras) transporte.escribir(Car.CONFIG, e)
            val r = withTimeout(10_000) { fin.await() }
            if (!r.ok) throw ErrorBicho("El bicho no aceptó la configuración: ${r.detalle.ifEmpty { r.mensaje }}")
        }
        actualizarEstado()
    }

    /** Manda una imagen QOI (del tamaño de la pantalla) a un espacio, avisando el progreso (0 a 1). */
    suspend fun subirImagen(espacio: Int, qoi: ByteArray, alProgreso: (Float) -> Unit = {}) {
        val escrituras = escriturasDeImagen(espacio, qoi)
        coroutineScope {
            val fin = async(start = CoroutineStart.UNDISPATCHED) {
                transporte.notificaciones
                    .mapNotNull { (c, b) -> if (c == Car.IMAGEN) leerRespuesta(b) as? Respuesta.Resultado else null }
                    .first { it.op == Op.IMG_FIN || (it.op != Op.IMG_LISTAR && !it.ok) }
            }
            escrituras.forEachIndexed { i, e ->
                transporte.escribir(Car.IMAGEN, e)
                alProgreso((i + 1f) / escrituras.size)
            }
            val r = withTimeout(15_000) { fin.await() }
            if (!r.ok) throw ErrorBicho("El bicho no aceptó la imagen: ${r.detalle.ifEmpty { r.mensaje }}")
        }
    }

    suspend fun borrarImagen(espacio: Int) { exigir(Car.IMAGEN, Imagen.borrar(espacio)) }

    suspend fun listarImagenes(): List<EspacioImagen> = coroutineScope {
        val espera = async(start = CoroutineStart.UNDISPATCHED) {
            transporte.notificaciones.mapNotNull { (c, b) -> if (c == Car.IMAGEN && (b[0].toInt() and 0x7F) == Op.IMG_LISTAR) b else null }.first()
        }
        transporte.escribir(Car.IMAGEN, Imagen.listar())
        val b = withTimeout(5_000) { espera.await() }
        if (b[1].toInt() != 0) throw ErrorBicho((leerRespuesta(b) as Respuesta.Resultado).mensaje)
        leerListaImagenes(b)
    }

    /**
     * Instala un firmware firmado por BLE (el manifiesto lo da la API). El bicho
     * verifica la firma antes de aceptar, borra la otra partición (unos
     * segundos), recibe los bloques, verifica el SHA-256 y reinicia.
     */
    suspend fun instalarFirmware(manifiesto: ByteArray, binario: ByteArray, alProgreso: (Float) -> Unit = {}) {
        val ini = pedir(Car.OTA, byteArrayOf(Op.OTA_INICIO.toByte()) + manifiesto, plazo = 60_000)
        if (!ini.ok) throw ErrorBicho("El bicho no aceptó el firmware: ${ini.detalle.ifEmpty { ini.mensaje }}")
        coroutineScope {
            // Si un bloque falla, el bicho avisa con [0xB1][código] y aborta.
            val falla = async(start = CoroutineStart.UNDISPATCHED) {
                transporte.notificaciones
                    .mapNotNull { (c, b) -> if (c == Car.OTA) leerRespuesta(b) as? Respuesta.Resultado else null }
                    .first { it.op == Op.OTA_BLOQUE && !it.ok }
            }
            val porBloque = 175
            var off = 0
            var n = 0
            while (off < binario.size) {
                if (falla.isCompleted) throw ErrorBicho("Se cortó la actualización: ${falla.await().mensaje}")
                val fin = minOf(binario.size, off + porBloque)
                val b = byteArrayOf(Op.OTA_BLOQUE.toByte(), (off and 0xFF).toByte(), (off shr 8 and 0xFF).toByte(),
                    (off shr 16 and 0xFF).toByte(), (off shr 24 and 0xFF).toByte()) + binario.copyOfRange(off, fin)
                transporte.escribir(Car.OTA, b)
                off = fin
                // Cada tanto, una lectura: espera a que lo anterior haya salido (sin respuesta, se encolan).
                if (++n % 16 == 0) transporte.leerEstado()
                alProgreso(off.toFloat() / binario.size)
            }
            falla.cancel()
        }
        val fin = pedir(Car.OTA, byteArrayOf(Op.OTA_FIN.toByte()), plazo = 60_000)
        if (!fin.ok) throw ErrorBicho("El bicho no instaló el firmware: ${fin.detalle.ifEmpty { fin.mensaje }}")
    }

    fun cerrar() = transporte.cerrar()
}
