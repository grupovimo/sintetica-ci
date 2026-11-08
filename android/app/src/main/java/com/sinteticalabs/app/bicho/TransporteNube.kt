package com.sinteticalabs.app.bicho

import android.util.Base64
import com.sinteticalabs.app.datos.ApiSintetica
import com.sinteticalabs.app.protocolo.Car
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonPrimitive
import java.util.UUID

/**
 * Un bicho lejos, por la nube: los mismos bytes que por BLE, por la API. Lo
 * que cambia la config o las imágenes lo guarda la API y se lo lleva al bicho
 * (cuando se conecte, si está apagado); los comandos van directo.
 */
class TransporteNube(private val api: ApiSintetica, private val id: String) : Transporte {
    override val notificaciones = MutableSharedFlow<Pair<Car, ByteArray>>(extraBufferCapacity = 512)
    private val conexion = UUID.randomUUID().toString()
    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var eventos: Job? = null
    private var ultimoEstado: ByteArray? = null

    override suspend fun conectar() {
        ultimoEstado = api.bicho(id).estadoCrudo
        eventos = alcance.launch {
            while (isActive) {
                runCatching {
                    api.eventos(id).collect { e ->
                        when (e.tipo) {
                            "estado" -> (e.datos["crudo"] as? JsonPrimitive)?.contentOrNull?.let {
                                val b = Base64.decode(it, Base64.DEFAULT)
                                ultimoEstado = b
                                notificaciones.emit(Car.ESTADO to b)
                            }
                            "marco" -> {
                                val car = e.datos["car"]?.jsonPrimitive?.intOrNull?.let { Car.de(it) }
                                val datos = e.datos["datos"]?.jsonPrimitive?.contentOrNull?.let { Base64.decode(it, Base64.DEFAULT) }
                                if (car != null && datos != null) notificaciones.emit(car to datos)
                            }
                        }
                    }
                }
                delay(3_000) // se cortó: reconectar
            }
        }
    }

    override suspend fun escribir(car: Car, datos: ByteArray) {
        for ((c, b) in api.marco(id, car.id, datos, conexion)) Car.de(c)?.let { notificaciones.emit(it to b) }
    }

    override suspend fun leerEstado(): ByteArray =
        ultimoEstado ?: api.bicho(id).estadoCrudo?.also { ultimoEstado = it } ?: throw ErrorBicho("Todavía no sé cómo está.")

    override fun cerrar() {
        eventos?.cancel()
    }
}
