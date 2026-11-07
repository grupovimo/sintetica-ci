package com.sinteticalabs.app.ui

import android.app.Application
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.sinteticalabs.app.bicho.Ajustes
import com.sinteticalabs.app.bicho.ClienteBicho
import com.sinteticalabs.app.bicho.EditorConfig
import com.sinteticalabs.app.bicho.TransporteBle
import com.sinteticalabs.app.bicho.TransporteVirtual
import com.sinteticalabs.app.bicho.buscarBicho
import com.sinteticalabs.app.datos.Vinculados
import com.sinteticalabs.app.nucleo.AnfitrionEnMemoria
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.Estado
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

/** "demo" = probar sin bicho (el núcleo corre en el celu). */
const val ID_DEMO = "demo"

sealed interface Conexion {
    data object Conectando : Conexion
    data object Lista : Conexion
    data class Error(val mensaje: String, val revincular: Boolean = false) : Conexion
}

class BichoViewModel(app: Application) : AndroidViewModel(app) {
    private val _conexion = MutableStateFlow<Conexion>(Conexion.Conectando)
    val conexion: StateFlow<Conexion> = _conexion
    private val _ajustes = MutableStateFlow(Ajustes())
    val ajustes: StateFlow<Ajustes> = _ajustes
    private val _guardando = MutableStateFlow(false)
    val guardando: StateFlow<Boolean> = _guardando
    private val _aviso = MutableStateFlow<String?>(null)
    val aviso: StateFlow<String?> = _aviso
    val expresiones = MutableStateFlow<List<String>>(emptyList())

    var cliente: ClienteBicho? = null
        private set
    var id: String = ""
        private set
    private var configTexto = "{\"version\":1}"
    val esDemo get() = id == ID_DEMO

    fun abrir(id: String) {
        if (this.id == id) return
        this.id = id
        viewModelScope.launch { conectar() }
    }

    suspend fun conectar() {
        _conexion.value = Conexion.Conectando
        cliente?.cerrar()
        try {
            val c: ClienteBicho
            if (esDemo) {
                c = ClienteBicho(TransporteVirtual(), viewModelScope)
                c.conectar()
            } else {
                val v = Vinculados(getApplication()).buscar(id) ?: throw IllegalStateException("Este bicho no está vinculado.")
                val cerca = buscarBicho(getApplication(), id)
                c = ClienteBicho(TransporteBle(getApplication(), cerca.dispositivo), viewModelScope)
                c.conectar()
                try {
                    c.autenticar(v.clave)
                } catch (e: Exception) {
                    _conexion.value = Conexion.Error("Este bicho ya no reconoce a este celu. Vinculalo de nuevo.", revincular = true)
                    c.cerrar()
                    return
                }
                // La vista previa: un núcleo local con la misma config.
                Nucleo.iniciar(640, 344, id, AnfitrionEnMemoria())
                Nucleo.vistaPrevia()
            }
            cliente = c
            configTexto = c.pedirConfig()
            _ajustes.value = EditorConfig.leer(configTexto)
            if (!esDemo) aplicarEnVistaPrevia(configTexto)
            expresiones.value = Nucleo.expresiones().toList()
            _conexion.value = Conexion.Lista
            // Copiar la expresión del bicho en la vista previa.
            if (!esDemo) viewModelScope.launch {
                c.estado.collect { e -> if (e != null) Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(e.expresion)) }
            }
        } catch (e: Exception) {
            _conexion.value = Conexion.Error(e.message ?: "No me pude conectar con el bicho.")
        }
    }

    private fun aplicarEnVistaPrevia(json: String) {
        for (e in escriturasDeConfig(json, 512)) Nucleo.bleEscribir(Car.CONFIG.id, e)
    }

    /** Cambia los ajustes en la vista previa; se mandan al bicho con guardar(). */
    fun editar(a: Ajustes) {
        _ajustes.value = a
        if (!esDemo) aplicarEnVistaPrevia(EditorConfig.escribir(configTexto, a))
    }

    fun guardar() = viewModelScope.launch {
        val c = cliente ?: return@launch
        _guardando.value = true
        try {
            val nueva = EditorConfig.escribir(configTexto, _ajustes.value)
            c.mandarConfig(nueva)
            configTexto = nueva
            if (!esDemo) Vinculados(getApplication()).buscar(id)?.let { Vinculados(getApplication()).guardar(it.copy(nombre = _ajustes.value.nombre)) }
            _aviso.value = "Guardado en el bicho."
        } catch (e: Exception) {
            _aviso.value = e.message
        } finally {
            _guardando.value = false
        }
    }

    fun fijarExpresion(nombre: String?) = viewModelScope.launch {
        runCatching { cliente?.fijarExpresion(nombre) }.onFailure { _aviso.value = it.message }
    }

    fun tocar(apretado: Boolean) {
        if (esDemo) Nucleo.tacto(apretado)
        else if (apretado) viewModelScope.launch { runCatching { cliente?.evento("tacto") } }
    }

    fun desvincular(listo: () -> Unit) = viewModelScope.launch {
        runCatching { cliente?.desvincular() }
        if (!esDemo) Vinculados(getApplication()).olvidar(id)
        listo()
    }

    fun olvidarLocal(listo: () -> Unit) = viewModelScope.launch {
        Vinculados(getApplication()).olvidar(id)
        listo()
    }

    fun cerrarAviso() { _aviso.value = null }

    override fun onCleared() { cliente?.cerrar() }
}

private val COLORES_OJOS = listOf(
    null to "Naranja", "#F5F2EC" to "Hueso", "#FFD23F" to "Amarillo", "#7BD389" to "Verde", "#5AC8FA" to "Celeste", "#FF8FB1" to "Rosa",
)

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun PantallaBicho(vm: BichoViewModel, alVolver: () -> Unit, alRevincular: () -> Unit) {
    val p = LocalPaleta.current
    val conexion by vm.conexion.collectAsState()
    Surface(color = p.papel, modifier = Modifier.fillMaxSize()) {
        when (val c = conexion) {
            Conexion.Conectando -> Column(Modifier.safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Etiqueta(if (vm.esDemo) "Bicho virtual" else vm.id)
                Text(if (vm.esDemo) "Despertando al bicho virtual…" else "Buscando el bicho…", style = Tipos.display, color = p.tinta)
                CircularProgressIndicator(color = p.naranja)
                BotonSecundario("Volver", alVolver, Modifier.fillMaxWidth())
            }
            is Conexion.Error -> Column(Modifier.safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Etiqueta(vm.id)
                Text("No me pude conectar.", style = Tipos.display, color = p.tinta)
                Aviso(c.mensaje)
                if (c.revincular) BotonPrimario("Vincular de nuevo", { vm.olvidarLocal(alRevincular) })
                else BotonPrimario("Probar de nuevo", { vm.abrirDeNuevo() })
                BotonSecundario("Volver", alVolver, Modifier.fillMaxWidth())
            }
            Conexion.Lista -> Contenido(vm, alVolver)
        }
    }
}

private fun BichoViewModel.abrirDeNuevo() = viewModelScope.launch { conectar() }

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Contenido(vm: BichoViewModel, alVolver: () -> Unit) {
    val p = LocalPaleta.current
    val estado by (vm.cliente?.estado ?: MutableStateFlow<Estado?>(null)).collectAsState()
    val a by vm.ajustes.collectAsState()
    val guardando by vm.guardando.collectAsState()
    val aviso by vm.aviso.collectAsState()
    val expresiones by vm.expresiones.collectAsState()
    var nombre by remember(a.nombre) { mutableStateOf(a.nombre) }
    var confirmar by remember { mutableStateOf(false) }

    Column(
        Modifier.safeDrawingPadding().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(28.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
            TextButton(onClick = alVolver) { Text("← BICHOS", style = Tipos.boton, color = p.tintaSuave) }
            Etiqueta(if (vm.esDemo) "Bicho virtual" else "Conectado")
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            val serie = estado?.serie.orEmpty()
            Text(if (serie.isNotEmpty()) "UNIDAD #$serie" else "UNIDAD #—", style = Tipos.serie, color = p.naranjaProfundo)
            Text(estado?.nombre ?: a.nombre, style = Tipos.display, color = p.tinta)
        }
        VistaPrevia("Cara de ${estado?.nombre ?: "el bicho"}: ${estado?.expresion ?: ""}", vm::tocar)

        Column {
            FilaDato("Expresión", (estado?.expresion ?: "—") + if (estado?.fija == true) " (fija)" else "")
            FilaDato("Humor", estado?.humor?.let { (if (it >= 0) "+" else "") + "%.2f".format(it) } ?: "—")
            FilaDato("Batería", estado?.bateria?.let { "$it%" } ?: "sin batería")
            FilaDato("Estado", if (estado?.dormido == true) "durmiendo" else "despierto")
        }

        Seccion("Caras") {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Chip("Automático", seleccionado = estado?.fija == false) { vm.fijarExpresion(null) }
                expresiones.forEach { e -> Chip(e, seleccionado = estado?.fija == true && estado?.expresion == e) { vm.fijarExpresion(e) } }
            }
        }

        Seccion("Nombre") {
            OutlinedTextField(
                value = nombre, onValueChange = { if (it.toByteArray().size <= 24) { nombre = it; vm.editar(a.copy(nombre = it)) } },
                singleLine = true, textStyle = Tipos.cuerpo, modifier = Modifier.fillMaxWidth(),
            )
        }

        Seccion("Ojos") {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                COLORES_OJOS.forEach { (hex, nombreColor) ->
                    val color = hex?.let { Color(android.graphics.Color.parseColor(it)) } ?: Color(0xFFFF6A33)
                    val elegido = a.colorOjos == hex
                    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(4.dp),
                        modifier = Modifier.clickable { vm.editar(a.copy(colorOjos = hex)) }) {
                        Box(Modifier.size(44.dp).background(p.pantalla, CircleShape).border(if (elegido) 3.dp else 1.dp, if (elegido) p.naranja else p.linea, CircleShape), contentAlignment = Alignment.Center) {
                            Box(Modifier.size(18.dp, 28.dp).background(color, RoundedCornerShape(9.dp)))
                        }
                        Etiqueta(nombreColor)
                    }
                }
            }
            Deslizador("Tamaño", a.tamano, 0.5f..1.5f, "%.0f%%".format(a.tamano * 100)) { vm.editar(a.copy(tamano = it)) }
        }

        Seccion("Personalidad") {
            Deslizador("Energía", a.energia, 0f..1f, "%.0f%%".format(a.energia * 100)) { vm.editar(a.copy(energia = it)) }
            Deslizador("Timidez", a.timidez, 0f..1f, "%.0f%%".format(a.timidez * 100)) { vm.editar(a.copy(timidez = it)) }
            val conSueno = a.suenoDesde != null
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth()) {
                Text("Duerme de noche", style = Tipos.cuerpo, color = p.tinta, modifier = Modifier.weight(1f))
                Switch(checked = conSueno, onCheckedChange = { si ->
                    vm.editar(if (si) a.copy(suenoDesde = "23:30", suenoHasta = "08:00") else a.copy(suenoDesde = null, suenoHasta = null))
                }, colors = SwitchDefaults.colors(checkedTrackColor = p.naranja))
            }
            if (conSueno) Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                CampoHora("Desde", a.suenoDesde!!, Modifier.weight(1f)) { vm.editar(a.copy(suenoDesde = it)) }
                CampoHora("Hasta", a.suenoHasta!!, Modifier.weight(1f)) { vm.editar(a.copy(suenoHasta = it)) }
            }
        }

        aviso?.let { Aviso(it, Modifier.clickable { vm.cerrarAviso() }) }
        BotonPrimario(if (guardando) "Guardando…" else "Guardar en el bicho", { vm.guardar() }, habilitado = !guardando)
        TextButton(onClick = { confirmar = true }, modifier = Modifier.fillMaxWidth()) {
            Text(if (vm.esDemo) "REINICIAR EL BICHO VIRTUAL" else "DESVINCULAR ESTE BICHO", style = Tipos.boton, color = p.naranjaProfundo)
        }
    }

    if (confirmar) AlertDialog(
        onDismissRequest = { confirmar = false },
        title = { Text("¿Desvincular?", style = Tipos.titulo) },
        text = { Text("El bicho olvida a todos los celus vinculados y vuelve a mostrar su QR.", style = Tipos.cuerpo) },
        confirmButton = { TextButton(onClick = { confirmar = false; vm.desvincular(alVolver) }) { Text("DESVINCULAR", style = Tipos.boton, color = p.naranjaProfundo) } },
        dismissButton = { TextButton(onClick = { confirmar = false }) { Text("CANCELAR", style = Tipos.boton) } },
    )
}

@Composable
private fun Chip(texto: String, seleccionado: Boolean, alTocar: () -> Unit) {
    val p = LocalPaleta.current
    Surface(
        shape = RoundedCornerShape(12.dp), color = if (seleccionado) p.naranjaClaro else p.papelAlto,
        border = BorderStroke(1.dp, if (seleccionado) p.naranja else p.linea), modifier = Modifier.clickable(onClick = alTocar),
    ) { Text(texto, style = Tipos.dato, color = p.tinta, modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) }
}

@Composable
private fun Deslizador(etiqueta: String, valor: Float, rango: ClosedFloatingPointRange<Float>, texto: String, alCambiar: (Float) -> Unit) {
    val p = LocalPaleta.current
    Column {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Etiqueta(etiqueta)
            Text(texto, style = Tipos.dato, color = p.tinta)
        }
        Slider(value = valor, onValueChange = alCambiar, valueRange = rango,
            colors = SliderDefaults.colors(thumbColor = p.naranja, activeTrackColor = p.naranja, inactiveTrackColor = p.linea))
    }
}

@Composable
private fun CampoHora(etiqueta: String, valor: String, modifier: Modifier, alCambiar: (String) -> Unit) {
    var texto by remember(valor) { mutableStateOf(valor) }
    OutlinedTextField(
        value = texto, label = { Etiqueta(etiqueta) }, singleLine = true, textStyle = Tipos.dato, modifier = modifier,
        isError = !EditorConfig.hhmmValido(texto),
        onValueChange = { t -> texto = t.take(5); if (EditorConfig.hhmmValido(texto)) alCambiar(texto) },
    )
}
