package com.sinteticalabs.app.ui

import android.app.Application
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.sinteticalabs.app.datos.BichoNube
import com.sinteticalabs.app.datos.Cuenta
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.Sesion
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

/** La cuenta en la nube: entrar con el link mágico (o su código) y los bichos de la cuenta. */
class CuentaViewModel(app: Application) : AndroidViewModel(app) {
    private val cuenta = Cuenta(app)
    val sesion: StateFlow<Sesion?> = cuenta.sesion.stateIn(viewModelScope, SharingStarted.Eagerly, null)
    val bichos = MutableStateFlow<List<BichoNube>>(emptyList())
    val mensaje = MutableStateFlow<String?>(null)
    val ocupado = MutableStateFlow(false)
    val linkPedido = MutableStateFlow(false)

    private fun mientras(f: suspend () -> Unit) = viewModelScope.launch {
        ocupado.value = true
        try {
            f()
        } catch (e: ErrorApi) {
            mensaje.value = e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
        } catch (e: Exception) {
            mensaje.value = e.message ?: "Algo salió mal."
        } finally {
            ocupado.value = false
        }
    }

    fun guardarServidor(url: String) = viewModelScope.launch { cuenta.guardarServidor(url) }

    fun pedirLink(email: String) = mientras {
        cuenta.guardarEmail(email)
        cuenta.api(cuenta.actual()).pedirEnlace(email)
        linkPedido.value = true
        mensaje.value = "Te mandamos un link: abrilo en este celu, o escribí el código del mail."
    }

    fun entrarConCodigo(codigo: String) = mientras {
        val s = cuenta.actual()
        cuenta.entrar(cuenta.api(s).canjear(email = s.email, codigo = codigo))
        linkPedido.value = false
        mensaje.value = null
        cargarBichos()
    }

    /** El link del mail abrió la app: sintetica://entrar?t=… */
    fun entrarConLink(token: String) = mientras {
        cuenta.entrar(cuenta.api(cuenta.actual()).canjear(tokenEnlace = token))
        linkPedido.value = false
        mensaje.value = "Entraste."
        cargarBichos()
    }

    // --- Entrar con Telegram: el link abre el bot; la app espera la aprobación (o el código de 6 dígitos) ---

    /** null = todavía no se sabe; email / telegram según el servidor. */
    val metodos = MutableStateFlow<Pair<Boolean, Boolean>?>(null)
    /** El pedido en curso: token, link y secreto. */
    val telegram = MutableStateFlow<Triple<String, String, String>?>(null)
    val confirmar = MutableStateFlow(false)
    private var esperando: kotlinx.coroutines.Job? = null

    fun cargarMetodos() = viewModelScope.launch {
        metodos.value = runCatching { cuenta.api(cuenta.actual()).metodos() }.getOrDefault(true to false)
    }

    fun conTelegram(abrir: (String) -> Unit) = mientras {
        val api = cuenta.api(cuenta.actual())
        val p = api.pedirTelegram()
        telegram.value = p
        confirmar.value = false
        abrir(p.second)
        esperando?.cancel()
        esperando = viewModelScope.launch {
            while (telegram.value == p) {
                try {
                    val (t, estado) = api.esperarTelegram(p.first, p.third)
                    if (t != null) { cuenta.entrar(t); telegram.value = null; cargarBichos(); return@launch }
                    confirmar.value = estado == "confirmar"
                } catch (e: ErrorApi) {
                    if (e.estado == 410 || e.estado == 401) { telegram.value = null; mensaje.value = "Venció. Pedí otro."; return@launch }
                    kotlinx.coroutines.delay(3000)
                } catch (e: Exception) {
                    kotlinx.coroutines.delay(3000)
                }
            }
        }
    }

    fun codigoTelegram(codigo: String) = mientras {
        val p = telegram.value ?: return@mientras
        cuenta.entrar(cuenta.api(cuenta.actual()).codigoTelegram(p.first, p.third, codigo))
        telegram.value = null
        esperando?.cancel()
        cargarBichos()
    }

    fun cancelarTelegram() { esperando?.cancel(); telegram.value = null; confirmar.value = false }

    fun cargarBichos() = viewModelScope.launch {
        val s = cuenta.actual()
        if (!s.activa) return@launch
        runCatching { bichos.value = cuenta.api(s).bichos() }.onFailure {
            if (it is ErrorApi && it.estado == 401) cuenta.salir()
        }
    }

    fun salir() = mientras {
        cuenta.api(cuenta.actual()).salir()
        cuenta.salir()
        bichos.value = emptyList()
    }
}

/**
 * Entrar: el botón grande de Telegram (abre el bot y la app espera la
 * aprobación; o el código de 6 dígitos), el mail solo si el servidor lo
 * tiene, y el servidor para depurar escondido.
 */
@Composable
fun PantallaCuenta(vm: CuentaViewModel, alVolver: () -> Unit) {
    val sesion by vm.sesion.collectAsState()
    val mensaje by vm.mensaje.collectAsState()
    val ocupado by vm.ocupado.collectAsState()
    val pedido by vm.linkPedido.collectAsState()
    val metodos by vm.metodos.collectAsState()
    val telegram by vm.telegram.collectAsState()
    val confirmar by vm.confirmar.collectAsState()
    val contexto = androidx.compose.ui.platform.LocalContext.current
    val s = sesion ?: return
    var servidor by remember(s.servidor) { mutableStateOf(s.servidor) }
    var email by remember(s.email) { mutableStateOf(s.email) }
    var codigo by remember { mutableStateOf("") }
    var conMail by remember { mutableStateOf(false) }
    var depurar by remember { mutableStateOf(false) }
    androidx.compose.runtime.LaunchedEffect(s.servidor) { vm.cargarMetodos() }
    val C = com.sinteticalabs.app.ui.consola.C
    val T = com.sinteticalabs.app.ui.consola.T
    fun abrir(u: String) = runCatching { contexto.startActivity(android.content.Intent(android.content.Intent.ACTION_VIEW, android.net.Uri.parse(u)).addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)) }

    Column(
        Modifier.fillMaxSize().background(C.fondo).safeDrawingPadding()
            .verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        com.sinteticalabs.app.ui.consola.Tecla("←", alVolver, acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, cuadrada = true, descripcion = "Volver")
        com.sinteticalabs.app.ui.consola.Grabado("Sintética", color = C.brasa)
        if (s.activa) {
            Text("ADENTRO.", style = T.titular, color = C.papel)
            Text(s.email.ifEmpty { "Tus robots, desde cualquier lado." }, style = T.texto)
            com.sinteticalabs.app.ui.consola.Tecla("Salir", { vm.salir() }, Modifier.fillMaxWidth(), acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, habilitada = !ocupado)
        } else {
            val t = telegram
            Text(if (t != null) (if (confirmar) "CONFIRMÁ EN TELEGRAM." else "TOCÁ START.") else if (pedido) "FIJATE TU MAIL." else "ENTRÁ.", style = T.titular, color = C.papel)
            val m = metodos
            if (t != null) {
                com.sinteticalabs.app.ui.consola.Tecla("Abrir Telegram", { abrir(t.second) }, Modifier.fillMaxWidth(), acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA)
                com.sinteticalabs.app.ui.consola.Visor(Modifier.fillMaxWidth()) {
                    com.sinteticalabs.app.ui.consola.Grabado("O el código del bot")
                    com.sinteticalabs.app.ui.consola.CampoVisor(codigo, { codigo = it }, placeholder = "123 456", maxBytes = 7,
                        teclado = KeyboardType.Number, alEscribir = { codigo = it })
                    com.sinteticalabs.app.ui.consola.Tecla("Entrar", { vm.codigoTelegram(codigo) }, acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA, chica = true,
                        habilitada = !ocupado && codigo.count { it.isDigit() } == 6)
                }
                com.sinteticalabs.app.ui.consola.Tecla("Cancelar", { vm.cancelarTelegram(); codigo = "" }, acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, chica = true)
            } else if (!conMail && !pedido && m?.second == true) {
                com.sinteticalabs.app.ui.consola.Tecla("➤ Entrar con Telegram", { vm.conTelegram { abrir(it) } }, Modifier.fillMaxWidth(),
                    acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA, alto = 64.dp, habilitada = !ocupado)
                if (m.first) com.sinteticalabs.app.ui.consola.Tecla("Con mail", { conMail = true }, acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, chica = true)
            } else if (m != null && (m.first || conMail || pedido)) {
                com.sinteticalabs.app.ui.consola.Visor(Modifier.fillMaxWidth()) {
                    com.sinteticalabs.app.ui.consola.Grabado("Tu mail")
                    com.sinteticalabs.app.ui.consola.CampoVisor(email, { email = it }, placeholder = "vos@mail.com", estilo = T.campoClaro, maxBytes = 120,
                        teclado = KeyboardType.Email, alEscribir = { email = it })
                }
                com.sinteticalabs.app.ui.consola.Tecla("Mandame el link", { vm.pedirLink(email) }, Modifier.fillMaxWidth(), acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA,
                    habilitada = !ocupado && email.contains('@'))
                if (pedido) com.sinteticalabs.app.ui.consola.Visor(Modifier.fillMaxWidth()) {
                    com.sinteticalabs.app.ui.consola.Grabado("Código del mail")
                    com.sinteticalabs.app.ui.consola.CampoVisor(codigo, { codigo = it }, placeholder = "ABCD-1234", maxBytes = 9, alEscribir = { codigo = it.uppercase() })
                    com.sinteticalabs.app.ui.consola.Tecla("Entrar", { vm.entrarConCodigo(codigo) }, acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA, chica = true,
                        habilitada = !ocupado && codigo.replace("-", "").length == 8)
                }
                if (m.second) com.sinteticalabs.app.ui.consola.Tecla("Con Telegram", { conMail = false; vm.linkPedido.value = false }, acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, chica = true)
            } else if (m != null) Text("Este servidor no tiene cómo entrar.", style = T.texto)
        }
        mensaje?.let { com.sinteticalabs.app.ui.consola.AvisoLinea(it) }
        // Para depurar: otro servidor (la nube local en la Mac).
        com.sinteticalabs.app.ui.consola.Llave("Depurar", depurar, { depurar = !depurar })
        if (depurar) com.sinteticalabs.app.ui.consola.Visor(Modifier.fillMaxWidth()) {
            com.sinteticalabs.app.ui.consola.Grabado("Servidor")
            com.sinteticalabs.app.ui.consola.CampoVisor(servidor, { v -> servidor = v; vm.guardarServidor(v) }, estilo = T.dato.copy(color = C.papel), maxBytes = 200,
                teclado = KeyboardType.Uri, alEscribir = { servidor = it })
            com.sinteticalabs.app.ui.consola.Tecla("Producción", { servidor = com.sinteticalabs.app.datos.SERVIDOR_POR_DEFECTO; vm.guardarServidor(servidor) },
                acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, chica = true)
        }
    }
}
