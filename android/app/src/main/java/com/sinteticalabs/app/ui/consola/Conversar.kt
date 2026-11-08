package com.sinteticalabs.app.ui.consola

import android.Manifest
import android.content.pm.PackageManager
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.core.content.ContextCompat
import com.sinteticalabs.app.consola.Voz
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.conversacion
import com.sinteticalabs.app.datos.vozEleven
import com.sinteticalabs.app.protocolo.Comando
import io.elevenlabs.ClientTool
import io.elevenlabs.ClientToolResult
import io.elevenlabs.ConversationClient
import io.elevenlabs.ConversationConfig
import io.elevenlabs.ConversationSession
import io.elevenlabs.models.ConversationMode
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Hablarle en tiempo real (web: consola/Conversar.svelte): una tecla de
 * micrófono que abre la conversación con el agente de ElevenLabs de este robot
 * (la cuenta del dueño), con el SDK oficial de Android y el token de la nube.
 * Sus herramientas (poner_expresion y mirar) mueven la cara de acá y, si el
 * robot está conectado, también la de él. Corta sola al silencio.
 */
@Composable
fun Conversar(vm: RobotViewModel) {
    val api = vm.apiNube() ?: return
    if (vm.via != Via.NUBE) return
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var disponible by remember { mutableStateOf(false) }
    var estado by remember { mutableStateOf("quieto") } // quieto, conectando, escucha, habla
    var quedan by remember { mutableStateOf(0) }
    var aviso by remember { mutableStateOf<String?>(null) }
    var explicando by remember { mutableStateOf(false) }
    var sesion by remember { mutableStateOf<ConversationSession?>(null) }
    var expresion by remember { mutableStateOf<String?>(null) }
    var puso by remember { mutableStateOf(false) }

    suspend fun mirarDisponible() {
        val v = runCatching { api.vozEleven() }.getOrNull()
        disponible = v?.conectado == true && v.topeMinutos > 0
    }
    LaunchedEffect(Unit) {
        mirarDisponible()
        EnVivo.eventos.collect { if (it.tipo == "voz") mirarDisponible() }
    }

    fun avisar(t: String) = alcance.launch { aviso = t; delay(3000); aviso = null }

    fun terminar() {
        sesion = null
        estado = "quieto"
        expresion = null
        vm.caraEnVistaPrevia(null)
        vm.miradaEnVistaPrevia(0.0, 0.0)
        if (puso && vm.robot.value?.conectado == true) alcance.launch { vm.comando(expresion = "") }
        puso = false
    }

    fun cortar() {
        val s = sesion
        sesion = null
        alcance.launch { runCatching { s?.endSession() }; terminar() }
    }

    fun empezar() = alcance.launch {
        estado = "conectando"
        val c = try {
            api.conversacion(vm.idRobot)
        } catch (e: ErrorApi) {
            estado = "quieto"
            if (e.estado == 404) disponible = false
            avisar(if (e.estado == 429) "Por hoy ya hablaron" else e.message ?: "No pude")
            return@launch
        } catch (e: Exception) {
            estado = "quieto"; avisar("No pude"); return@launch
        }
        val caras = c.expresiones
        val herramientas = mapOf(
            "poner_expresion" to object : ClientTool {
                override suspend fun execute(parameters: Map<String, Any>): ClientToolResult? {
                    val n = Voz.expresion(parameters["nombre"], caras) ?: return ClientToolResult.failure("no la tengo")
                    withContext(Dispatchers.Main) {
                        expresion = n
                        vm.caraEnVistaPrevia(n)
                        if (vm.robot.value?.conectado == true) { puso = true; vm.comando(expresion = n) }
                    }
                    return null
                }
            },
            "mirar" to object : ClientTool {
                override suspend fun execute(parameters: Map<String, Any>): ClientToolResult? {
                    val (x, y) = Voz.mirada(parameters["x"], parameters["y"])
                    withContext(Dispatchers.Main) {
                        vm.miradaEnVistaPrevia(x, y)
                        if (vm.robot.value?.conectado == true) vm.mandarMarco(Comando.mirar(x, y))
                    }
                    return null
                }
            },
        )
        try {
            sesion = ConversationClient.startSession(
                ConversationConfig(
                    conversationToken = c.token,
                    clientTools = herramientas,
                    onModeChange = { m ->
                        alcance.launch {
                            estado = if (m == ConversationMode.SPEAKING) "habla" else "escucha"
                            // Escuchando, cara de escuchar (si el agente no eligió otra).
                            vm.caraEnVistaPrevia(expresion ?: if (m == ConversationMode.LISTENING) "escuchando" else null)
                        }
                    },
                    onDisconnect = { _ -> alcance.launch { terminar() } },
                ),
                contexto,
            )
            estado = "escucha"
            quedan = c.segundos
            while (sesion != null && quedan > 0) { delay(1000); quedan-- }
            if (sesion != null) cortar()
        } catch (e: Exception) {
            terminar()
            avisar("No pude conectar")
        }
    }

    val permiso = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { si -> if (si) empezar() else avisar("Sin micrófono") }
    DisposableEffect(Unit) { onDispose { sesion?.let { s -> alcance.launch { runCatching { s.endSession() } } } } }

    if (!disponible) return
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Tecla(if (estado == "quieto" || estado == "conectando") "🎙" else "■", {
            when {
                estado != "quieto" -> cortar()
                ContextCompat.checkSelfPermission(contexto, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED -> empezar()
                else -> explicando = true
            }
        }, acabado = if (estado == "quieto") Acabado.NARANJA else Acabado.NEGRA, cuadrada = true, alto = 56.dp,
            descripcion = if (estado == "quieto") "Hablarle" else "Cortar")
        Visor(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Led(when (estado) { "escucha" -> C.verde; "habla" -> C.led; "conectando" -> C.ambar; else -> null })
                Grabado(aviso ?: when (estado) { "conectando" -> "…"; "escucha" -> "Te escucha"; "habla" -> "Habla"; else -> "Hablale" }, Modifier.weight(1f))
                if (estado == "escucha" || estado == "habla") {
                    Ondas(estado == "habla")
                    Text(Voz.mmss(quedan), style = T.dato.copy(color = C.papel))
                }
            }
        }
    }
    if (explicando) androidx.compose.ui.window.Dialog(onDismissRequest = { explicando = false }) {
        Chapa(Modifier.fillMaxWidth(), radio = 28.dp, relleno = 22.dp) {
            Grabado("Hablarle", color = C.brasa)
            Text("Usa el micrófono mientras charlan.", style = T.texto.copy(color = C.papel))
            Text("El audio va directo a ElevenLabs.", style = T.texto)
            Tecla("Dale", { explicando = false; permiso.launch(Manifest.permission.RECORD_AUDIO) }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
            Tecla("Ahora no", { explicando = false }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
        }
    }
}

/** Unas barritas que se mueven mientras hablan. */
@Composable
private fun Ondas(fuerte: Boolean) {
    var t by remember { mutableStateOf(0) }
    LaunchedEffect(Unit) { while (true) { delay(120); t++ } }
    Canvas(Modifier.width(34.dp).height(18.dp)) {
        val w = size.width / 5
        for (i in 0 until 5) {
            val f = 0.3f + 0.7f * (((t + i * 3) % 7) / 6f) * (if (fuerte) 1f else 0.5f)
            val h = size.height * f
            drawRoundRect(if (fuerte) C.led else C.verde, Offset(i * w + 2, (size.height - h) / 2), Size(w - 4, h), CornerRadius(2f))
        }
    }
}
