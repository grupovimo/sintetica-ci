package com.sinteticalabs.app.bicho

import com.sinteticalabs.app.protocolo.ArmadorDeConfig
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.Estado
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
    private suspend fun pedir(car: Car, datos: ByteArray, opEsperado: Int = datos[0].toInt() and 0x7F): Respuesta.Resultado =
        coroutineScope {
            val espera = async(start = CoroutineStart.UNDISPATCHED) {
                transporte.notificaciones
                    .mapNotNull { (c, b) -> if (c == car) leerRespuesta(b) as? Respuesta.Resultado else null }
                    .first { it.op == opEsperado }
            }
            transporte.escribir(car, datos)
            withTimeout(5_000) { espera.await() }
        }

    private suspend fun exigir(car: Car, datos: ByteArray): Respuesta.Resultado {
        val r = pedir(car, datos)
        if (!r.ok) throw ErrorBicho(r.detalle.ifEmpty { r.mensaje })
        return r
    }

    /** Vincula con el código del QR y devuelve la clave que hay que guardar. */
    suspend fun vincular(codigo: String): ByteArray {
        val r = exigir(Car.COMANDO, Comando.vincular(codigo))
        actualizarEstado()
        return r.clave ?: throw ErrorBicho("El bicho no mandó la clave.")
    }

    suspend fun autenticar(clave: ByteArray) {
        exigir(Car.COMANDO, Comando.autenticar(clave))
        actualizarEstado()
    }

    suspend fun fijarExpresion(nombre: String?) { exigir(Car.COMANDO, Comando.expresion(nombre ?: "")); actualizarEstado() }
    suspend fun cambiarNombre(nombre: String) { exigir(Car.COMANDO, Comando.nombre(nombre)); actualizarEstado() }
    suspend fun evento(nombre: String) { exigir(Car.COMANDO, Comando.evento(nombre)) }
    suspend fun desvincular() { exigir(Car.COMANDO, Comando.desvincular()) }

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

    fun cerrar() = transporte.cerrar()
}
