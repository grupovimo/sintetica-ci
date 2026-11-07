package com.sinteticalabs.app.bicho

import com.sinteticalabs.app.nucleo.AnfitrionEnMemoria
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Car
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.withContext

/** Por dónde se le habla a un bicho: BLE de verdad o el bicho virtual. */
interface Transporte {
    /** Notificaciones (característica, bytes). */
    val notificaciones: SharedFlow<Pair<Car, ByteArray>>
    suspend fun conectar()
    suspend fun escribir(car: Car, datos: ByteArray)
    suspend fun leerEstado(): ByteArray
    fun cerrar()
}

/**
 * "Probar sin bicho": el núcleo corre en el celu y se le habla con los mismos
 * bytes que por BLE. Dibuja con la misma instancia que la vista previa.
 */
class TransporteVirtual(private val broche: Boolean = false) : Transporte {
    override val notificaciones = MutableSharedFlow<Pair<Car, ByteArray>>(extraBufferCapacity = 256)
    private val anfitrion = AnfitrionEnMemoria().apply {
        alNotificar = { c, d -> Car.de(c)?.let { notificaciones.tryEmit(it to d) } }
    }

    override suspend fun conectar() = withContext(Dispatchers.Main) {
        // Al doble de la pantalla, para que se vea nítido; las imágenes se recortan a este tamaño.
        if (broche) Nucleo.iniciar(480, 480, "000000000000", anfitrion, redonda = true, broche = true)
        else Nucleo.iniciar(640, 344, "000000000000", anfitrion)
        Nucleo.vistaPrevia()
        Nucleo.bleConexion(true)
    }

    override suspend fun escribir(car: Car, datos: ByteArray) = withContext(Dispatchers.Main) { Nucleo.bleEscribir(car.id, datos) }
    override suspend fun leerEstado(): ByteArray = withContext(Dispatchers.Main) { Nucleo.estado() }
    override fun cerrar() {}
}
