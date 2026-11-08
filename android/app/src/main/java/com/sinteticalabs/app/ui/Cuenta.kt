package com.sinteticalabs.app.ui

import android.app.Application
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

@Composable
fun PantallaCuenta(vm: CuentaViewModel, alVolver: () -> Unit) {
    val p = LocalPaleta.current
    val sesion by vm.sesion.collectAsState()
    val mensaje by vm.mensaje.collectAsState()
    val ocupado by vm.ocupado.collectAsState()
    val pedido by vm.linkPedido.collectAsState()
    val s = sesion ?: return
    var servidor by remember(s.servidor) { mutableStateOf(s.servidor) }
    var email by remember(s.email) { mutableStateOf(s.email) }
    var codigo by remember { mutableStateOf("") }

    Surface(color = p.papel, modifier = Modifier.fillMaxSize()) {
        Column(Modifier.safeDrawingPadding().verticalScroll(rememberScrollState()).padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            TextButton(onClick = alVolver) { Text("←", style = Tipos.boton, color = p.tintaSuave) }
            Etiqueta("Tu cuenta")
            if (s.activa) {
                Text(s.email.ifEmpty { "Adentro" }, style = Tipos.display, color = p.tinta)
                Text("Tus robots, desde cualquier lado.", style = Tipos.cuerpo, color = p.tintaSuave)
                FilaDato("Servidor", s.servidor)
                BotonSecundario("Salir", { vm.salir() }, Modifier.fillMaxWidth(), habilitado = !ocupado)
            } else {
                Text("Entrá con tu email", style = Tipos.display, color = p.tinta)
                Text("Sin contraseñas: te llega un link.", style = Tipos.cuerpo, color = p.tintaSuave)
                OutlinedTextField(value = servidor, onValueChange = { servidor = it }, label = { Etiqueta("Servidor") }, singleLine = true,
                    textStyle = Tipos.dato, modifier = Modifier.fillMaxWidth(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri))
                Text("En local: http://<la IP de tu Mac>:8080", style = Tipos.dato, color = p.tintaSuave)
                OutlinedTextField(value = email, onValueChange = { email = it }, label = { Etiqueta("Email") }, singleLine = true,
                    textStyle = Tipos.cuerpo, modifier = Modifier.fillMaxWidth(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email))
                BotonPrimario("Mandame el link", { vm.guardarServidor(servidor); vm.pedirLink(email) }, Modifier.fillMaxWidth(),
                    habilitado = !ocupado && email.contains('@') && servidor.startsWith("http"))
                if (pedido) {
                    OutlinedTextField(value = codigo, onValueChange = { codigo = it.uppercase().take(9) }, label = { Etiqueta("Código del mail") },
                        singleLine = true, textStyle = Tipos.dato, modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Characters))
                    BotonSecundario("Entrar", { vm.entrarConCodigo(codigo) }, Modifier.fillMaxWidth(), habilitado = !ocupado && codigo.replace("-", "").length == 8)
                }
            }
            mensaje?.let { Aviso(it) }
        }
    }
}
