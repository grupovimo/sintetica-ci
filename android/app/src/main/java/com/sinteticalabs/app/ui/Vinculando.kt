package com.sinteticalabs.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.bicho.ClienteBicho
import com.sinteticalabs.app.bicho.TransporteBle
import com.sinteticalabs.app.bicho.buscarBicho
import com.sinteticalabs.app.datos.Vinculado
import com.sinteticalabs.app.datos.Vinculados
import com.sinteticalabs.app.protocolo.DatosQr

@Composable
fun PantallaVinculando(qr: DatosQr, alTerminar: (String) -> Unit, alVolver: () -> Unit) {
    val p = LocalPaleta.current
    val contexto = LocalContext.current
    val permiso = conPermisos(PERMISOS_BLE)
    val alcance = rememberCoroutineScope()
    var paso by remember { mutableStateOf("Buscando el bicho…") }
    var error by remember { mutableStateOf<String?>(null) }
    var intento by remember { mutableIntStateOf(0) }

    LaunchedEffect(permiso, intento) {
        if (!permiso) { error = "Para vincular hace falta el permiso de Bluetooth."; return@LaunchedEffect }
        error = null
        var cliente: ClienteBicho? = null
        try {
            paso = "Buscando el bicho…"
            val cerca = buscarBicho(contexto, qr.id)
            paso = "Conectando…"
            cliente = ClienteBicho(TransporteBle(contexto, cerca.dispositivo), alcance)
            cliente.conectar()
            paso = "Vinculando…"
            val clave = cliente.vincular(qr.codigo)
            val e = cliente.estado.value
            Vinculados(contexto).guardar(Vinculado(qr.id, e?.nombre ?: "bicho", e?.serie ?: "", clave))
            alTerminar(qr.id)
        } catch (e: Exception) {
            error = e.message ?: "Algo salió mal."
        } finally {
            cliente?.cerrar()
        }
    }

    Surface(color = p.papel, modifier = Modifier.fillMaxSize()) {
        Column(Modifier.safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Etiqueta("Vincular · ${qr.id}")
            Text(if (error == null) paso else "No se pudo vincular.", style = Tipos.display, color = p.tinta)
            if (error == null) CircularProgressIndicator(color = p.naranja)
            else {
                Aviso(error!!)
                BotonPrimario("Probar de nuevo", { intento++ })
            }
            BotonSecundario("Volver", alVolver, Modifier.fillMaxWidth())
        }
    }
}
