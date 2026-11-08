package com.sinteticalabs.app.datos

import com.sinteticalabs.app.consola.EventoCuenta
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * Los eventos en vivo de la cuenta (SSE /v1/eventos), compartidos por todas
 * las pantallas: una sola conexión que se reconecta sola con espera creciente
 * (1 s, 2 s, 4 s… hasta 30 s) y se rehace al entrar o salir de la cuenta.
 */
object EnVivo {
    private val _eventos = MutableSharedFlow<EventoCuenta>(extraBufferCapacity = 256)
    val eventos: SharedFlow<EventoCuenta> = _eventos
    private val _conectado = MutableStateFlow(false)
    val conectado: StateFlow<Boolean> = _conectado
    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var trabajo: Job? = null

    @OptIn(ExperimentalCoroutinesApi::class)
    fun arrancar(cuenta: Cuenta) {
        if (trabajo != null) return
        trabajo = alcance.launch {
            cuenta.sesion.collectLatest { s ->
                _conectado.value = false
                if (!s.activa) return@collectLatest
                var espera = 1_000L
                while (isActive) {
                    try {
                        cuenta.api(s).eventosCuenta().collect { e ->
                            _conectado.value = true
                            espera = 1_000L
                            _eventos.emit(e)
                        }
                    } catch (e: ErrorApi) {
                        if (e.estado == 401) { cuenta.salir(); return@collectLatest }
                    } catch (_: Exception) {
                    }
                    _conectado.value = false
                    delay(espera)
                    espera = minOf(30_000L, espera * 2)
                }
            }
        }
    }
}
