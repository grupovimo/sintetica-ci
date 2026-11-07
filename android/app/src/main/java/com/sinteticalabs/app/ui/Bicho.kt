package com.sinteticalabs.app.ui

import android.app.Application
import android.graphics.Bitmap
import android.graphics.ImageDecoder
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import com.sinteticalabs.app.bicho.PrepararImagen
import com.sinteticalabs.app.datos.CacheImagenes
import com.sinteticalabs.app.protocolo.EspacioImagen
import com.sinteticalabs.app.protocolo.Pantalla
import com.sinteticalabs.app.protocolo.crc32
import com.sinteticalabs.app.protocolo.escriturasDeImagen
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
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
import com.sinteticalabs.app.bicho.TransporteNube
import com.sinteticalabs.app.bicho.TransporteVirtual
import com.sinteticalabs.app.datos.Cuenta
import com.sinteticalabs.app.datos.ErrorApi
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

/** "demo" = probar sin bicho (el núcleo corre en el celu), como la 1.47B o como el broche. */
const val ID_DEMO = "demo"
const val ID_DEMO_BROCHE = "demo-broche"
/** "nube:<id>" = un bicho de la cuenta, lejos: por la API en vez de BLE. */
const val PREFIJO_NUBE = "nube:"

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
    val espacios = MutableStateFlow<List<EspacioImagen>>(emptyList())
    val miniaturas = MutableStateFlow<Map<Int, Bitmap>>(emptyMap())
    val progreso = MutableStateFlow<Float?>(null)
    /** La pantalla del bicho (la que dice el estado v3); la vista previa va al doble. */
    var pantalla = Pantalla(320, 172, false)
        private set
    private val cache = CacheImagenes(app)

    var cliente: ClienteBicho? = null
        private set
    var id: String = ""
        private set
    private var configTexto = "{\"version\":1}"
    val esDemo get() = id.startsWith(ID_DEMO)
    val esNube get() = id.startsWith(PREFIJO_NUBE)
    private val idBicho get() = id.removePrefix(PREFIJO_NUBE)
    private val cuenta = Cuenta(app)
    val sesion = cuenta.sesion
    private val _nube = MutableStateFlow<String?>(null)
    /** Cómo va lo de la nube (WiFi, sumar a la cuenta). */
    val nube: StateFlow<String?> = _nube
    val ocupadoNube = MutableStateFlow(false)

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
                c = ClienteBicho(TransporteVirtual(broche = id == ID_DEMO_BROCHE), viewModelScope)
                c.conectar()
                pantalla = c.estado.value?.pantalla ?: pantalla
            } else if (esNube) {
                val s = cuenta.actual()
                if (!s.activa) throw IllegalStateException("Entrá a tu cuenta para ver los bichos lejos.")
                c = ClienteBicho(TransporteNube(cuenta.api(s), idBicho), viewModelScope)
                c.conectar()
                pantalla = c.estado.value?.pantalla ?: pantalla
                Nucleo.iniciar(pantalla.ancho * 2, pantalla.alto * 2, idBicho, AnfitrionEnMemoria(), redonda = pantalla.redonda)
                Nucleo.vistaPrevia()
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
                // La vista previa: un núcleo local con la misma config, al doble de la pantalla.
                pantalla = c.estado.value?.pantalla ?: pantalla
                Nucleo.iniciar(pantalla.ancho * 2, pantalla.alto * 2, id, AnfitrionEnMemoria(), redonda = pantalla.redonda)
                Nucleo.vistaPrevia()
            }
            cliente = c
            configTexto = c.pedirConfig()
            _ajustes.value = EditorConfig.leer(configTexto)
            if (!esDemo) aplicarEnVistaPrevia(configTexto)
            expresiones.value = Nucleo.expresiones().toList()
            refrescarImagenes()
            _conexion.value = Conexion.Lista
            // Copiar la expresión del bicho en la vista previa.
            if (!esDemo) viewModelScope.launch {
                c.estado.collect { e -> if (e != null) Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(e.expresion)) }
            }
        } catch (e: Exception) {
            _conexion.value = Conexion.Error(e.message ?: "No me pude conectar con el bicho.")
        }
    }

    /** Los espacios ocupados; la vista previa recibe las que este celu tiene guardadas. */
    private suspend fun refrescarImagenes() {
        val c = cliente ?: return
        val lista = runCatching { c.listarImagenes() }.getOrDefault(emptyList())
        espacios.value = lista
        val minis = mutableMapOf<Int, Bitmap>()
        for (e in lista) {
            val mandada = cache.mandada(id, e.espacio) ?: continue
            if (crc32(mandada) != e.crc) continue  // la cambió otro celu
            val vista = cache.vistaPrevia(id, e.espacio) ?: continue
            if (!esDemo) for (w in escriturasDeImagen(e.espacio, vista, 512)) Nucleo.bleEscribir(Car.IMAGEN.id, w)
            withContext(Dispatchers.Default) { PrepararImagen.decodificar(mandada) }?.let { minis[e.espacio] = it }
        }
        miniaturas.value = minis
    }

    fun subirImagen(espacio: Int, uri: Uri) = viewModelScope.launch {
        val c = cliente ?: return@launch
        try {
            progreso.value = 0f
            val (qoi, vista) = withContext(Dispatchers.Default) {
                val fuente = ImageDecoder.decodeBitmap(ImageDecoder.createSource(getApplication<Application>().contentResolver, uri)) { d, _, _ ->
                    d.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
                }
                val p = pantalla
                val q = PrepararImagen.qoi(PrepararImagen.recortar(fuente, p.ancho, p.alto, p.redonda))
                // En el modo demo el núcleo ya está al doble: la misma imagen sirve para las dos.
                q to (if (esDemo) q else PrepararImagen.qoi(PrepararImagen.recortar(fuente, p.ancho * 2, p.alto * 2, p.redonda)))
            }
            c.subirImagen(espacio, qoi) { progreso.value = it }
            cache.guardar(id, espacio, qoi, vista)
            refrescarImagenes()
            _aviso.value = "Imagen guardada en el espacio $espacio."
        } catch (e: Exception) {
            _aviso.value = e.message ?: "No pude mandar la imagen."
        } finally {
            progreso.value = null
        }
    }

    fun borrarImagen(espacio: Int) = viewModelScope.launch {
        runCatching { cliente?.borrarImagen(espacio) }.onFailure { _aviso.value = it.message }
        cache.borrar(id, espacio)
        if (!esDemo) Nucleo.bleEscribir(Car.IMAGEN.id, com.sinteticalabs.app.protocolo.Imagen.borrar(espacio))
        refrescarImagenes()
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
            if (!esDemo && !esNube) Vinculados(getApplication()).buscar(id)?.let { Vinculados(getApplication()).guardar(it.copy(nombre = _ajustes.value.nombre)) }
            _aviso.value = if (esNube) "Guardado en la nube: le llega al bicho apenas esté conectado." else "Guardado en el bicho."
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

    val firmwares = MutableStateFlow<List<com.sinteticalabs.app.datos.FirmwareNube>>(emptyList())
    val progresoOta = MutableStateFlow<Float?>(null)
    val mensajeOta = MutableStateFlow<String?>(null)

    fun buscarFirmwares() = viewModelScope.launch {
        val s = cuenta.actual()
        val perfil = cliente?.estado?.value?.perfil.orEmpty()
        if (!s.activa || perfil.isEmpty()) { mensajeOta.value = "Entrá a tu cuenta para ver las versiones."; return@launch }
        runCatching { firmwares.value = cuenta.api(s).firmwares(perfil) }.onFailure { mensajeOta.value = it.message }
        if (firmwares.value.isEmpty()) mensajeOta.value = "No hay versiones para $perfil."
    }

    /** Baja el firmware firmado de la API y se lo manda al bicho por BLE. */
    fun instalarPorBle(f: com.sinteticalabs.app.datos.FirmwareNube) = viewModelScope.launch {
        val c = cliente ?: return@launch
        val api = cuenta.api(cuenta.actual())
        try {
            progresoOta.value = 0f
            mensajeOta.value = "Bajando ${f.version}…"
            val man = api.manifiesto(f.id)
            val bin = api.binario(f.id)
            mensajeOta.value = "Mandándolo al bicho (no lo alejes)…"
            c.instalarFirmware(man, bin) { progresoOta.value = it }
            mensajeOta.value = "Listo: reinicia con la ${f.version}. Si no anda bien, vuelve solo a la anterior."
        } catch (e: Exception) {
            mensajeOta.value = e.message ?: "No pude actualizarlo."
        } finally {
            progresoOta.value = null
        }
    }

    /** El WiFi, por BLE. */
    fun configurarRed(ssid: String, clave: String) = viewModelScope.launch {
        ocupadoNube.value = true
        _nube.value = runCatching { cliente?.configurarRed(ssid, clave); "Listo: se conecta a \"$ssid\"." }
            .getOrElse { it.message ?: "No pude." }
        ocupadoNube.value = false
    }

    fun olvidarRed() = viewModelScope.launch {
        _nube.value = runCatching { cliente?.olvidarRed(); "Se olvidó del WiFi y de la nube." }.getOrElse { it.message }
    }

    /**
     * Suma el bicho a la cuenta: la API da credenciales nuevas, se las pasamos
     * por BLE y, cuando el bicho se conecta con ellas, queda en la cuenta.
     */
    fun sumarACuenta() = viewModelScope.launch {
        val c = cliente ?: return@launch
        ocupadoNube.value = true
        try {
            val s = cuenta.actual()
            if (!s.activa) throw IllegalStateException("Primero entrá a tu cuenta.")
            val api = cuenta.api(s)
            val e = c.estado.value
            if (e?.redConfigurada != true) throw IllegalStateException("Primero pasale el WiFi.")
            _nube.value = "Pidiendo la clave a la nube…"
            val cred = api.reclamar(idBicho, e.perfil)
            c.configurarNube(cred.url, cred.usuario, cred.clave)
            _nube.value = "Esperando que se conecte (${cred.url})…"
            _nube.value = if (api.esperarReclamo(idBicho)) "Listo: está en tu cuenta. Desde la web o desde otro celu lo cambiás estés donde estés."
            else "No se conectó todavía: revisá el WiFi y que el servidor (${cred.url}) se vea desde la red del bicho."
        } catch (e: ErrorApi) {
            _nube.value = e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
        } catch (e: Exception) {
            _nube.value = e.message ?: "No pude."
        } finally {
            ocupadoNube.value = false
        }
    }

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
            Etiqueta(if (vm.esDemo) "Bicho virtual" else if (vm.esNube) "Por la nube" else "Conectado")
        }
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            val serie = estado?.serie.orEmpty()
            Text(if (serie.isNotEmpty()) "UNIDAD #$serie" else "UNIDAD #—", style = Tipos.serie, color = p.naranjaProfundo)
            Text(estado?.nombre ?: a.nombre, style = Tipos.display, color = p.tinta)
        }
        VistaPrevia("Cara de ${estado?.nombre ?: "el bicho"}: ${estado?.expresion ?: ""}", vm.pantalla.redonda, vm::tocar)

        Column {
            FilaDato("Expresión", (estado?.expresion ?: "—") + if (estado?.fija == true) " (fija)" else "")
            FilaDato("Humor", estado?.humor?.let { (if (it >= 0) "+" else "") + "%.2f".format(it) } ?: "—")
            FilaDato("Batería", estado?.bateria?.let { "$it%" } ?: "sin batería")
            FilaDato("Estado", if (estado?.dormido == true) "durmiendo" else "despierto")
        }

        if (!vm.esDemo && !vm.esNube) SeccionNube(vm, estado)
        if (!vm.esDemo && !vm.esNube) SeccionFirmware(vm, estado)
        if (vm.esNube && estado?.nube != true) Aviso("El bicho no está conectado ahora: lo que cambies le llega cuando vuelva.")

        Seccion("Caras") {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Chip("Automático", seleccionado = estado?.fija == false) { vm.fijarExpresion(null) }
                expresiones.forEach { e -> Chip(e, seleccionado = estado?.fija == true && estado?.expresion == e) { vm.fijarExpresion(e) } }
            }
        }

        SeccionImagenes(vm)
        SeccionLista(vm, a, expresiones)

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

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SeccionImagenes(vm: BichoViewModel) {
    val p = LocalPaleta.current
    val espacios by vm.espacios.collectAsState()
    val miniaturas by vm.miniaturas.collectAsState()
    val progreso by vm.progreso.collectAsState()
    var elegido by remember { mutableStateOf(0) }
    val galeria = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri != null) vm.subirImagen(elegido, uri)
    }
    val ocupados = espacios.map { it.espacio }.toSet()
    Seccion("Imágenes") {
        Text("Hasta 8. Se recortan a la forma de la pantalla del bicho.", style = Tipos.chico, color = p.tintaSuave)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            for (i in 0 until 8) {
                val mini = miniaturas[i]
                Box(
                    Modifier.size(64.dp).clip(if (vm.pantalla.redonda) CircleShape else RoundedCornerShape(10.dp))
                        .background(p.pantalla)
                        .border(if (i == elegido) 3.dp else 1.dp, if (i == elegido) p.naranja else p.linea, if (vm.pantalla.redonda) CircleShape else RoundedCornerShape(10.dp))
                        .clickable { elegido = i },
                    contentAlignment = Alignment.Center,
                ) {
                    if (mini != null) Image(mini.asImageBitmap(), contentDescription = "Imagen $i", modifier = Modifier.fillMaxSize())
                    else Text(if (i in ocupados) "$i ·" else "$i", style = Tipos.dato, color = p.tintaSuave)
                }
            }
        }
        progreso?.let { LinearProgressIndicator(progress = { it }, color = p.naranja, trackColor = p.linea, modifier = Modifier.fillMaxWidth()) }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            BotonSecundario("Elegir de la galería", { galeria.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }, habilitado = progreso == null)
            if (elegido in ocupados) BotonSecundario("Borrar", { vm.borrarImagen(elegido) })
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SeccionLista(vm: BichoViewModel, a: Ajustes, expresiones: List<String>) {
    val p = LocalPaleta.current
    val espacios by vm.espacios.collectAsState()
    var agregando by remember { mutableStateOf(false) }
    Seccion("Lista") {
        Text(
            if (vm.pantalla.redonda) "Un toque en el broche pasa a lo siguiente." else "Lo que muestra, en orden; puede rotar solo.",
            style = Tipos.chico, color = p.tintaSuave,
        )
        a.lista.forEachIndexed { i, item ->
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.fillMaxWidth()) {
                Text("${i + 1}. " + item.replace("cara:", "").replace("imagen:", "imagen "), style = Tipos.dato, color = p.tinta, modifier = Modifier.weight(1f))
                TextButton(onClick = { vm.editar(a.copy(lista = a.lista.toMutableList().apply { if (i > 0) add(i - 1, removeAt(i)) })) }, enabled = i > 0) { Text("↑", style = Tipos.boton) }
                TextButton(onClick = { vm.editar(a.copy(lista = a.lista.toMutableList().apply { if (i < size - 1) add(i + 1, removeAt(i)) })) }, enabled = i < a.lista.size - 1) { Text("↓", style = Tipos.boton) }
                TextButton(onClick = { vm.editar(a.copy(lista = a.lista.filterIndexed { k, _ -> k != i })) }) { Text("QUITAR", style = Tipos.boton, color = p.naranjaProfundo) }
            }
        }
        if (a.lista.isEmpty()) Text(if (vm.pantalla.redonda) "Vacía: recorre las 16 caras." else "Vacía: se maneja solo.", style = Tipos.dato, color = p.tintaSuave)
        if (agregando) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                espacios.forEach { e -> Chip("imagen ${e.espacio}", false) { vm.editar(a.copy(lista = a.lista + "imagen:${e.espacio}")); agregando = false } }
                expresiones.forEach { x -> Chip(x, false) { vm.editar(a.copy(lista = a.lista + "cara:$x")); agregando = false } }
            }
        } else if (a.lista.size < 16) BotonSecundario("Agregar", { agregando = true })
        Deslizador("Rotar sola", a.rotar.toFloat(), 0f..60f, if (a.rotar == 0) "no" else "cada ${a.rotar} min") { vm.editar(a.copy(rotar = it.toInt())) }
    }
}

@Composable
private fun SeccionNube(vm: BichoViewModel, estado: Estado?) {
    val p = LocalPaleta.current
    val sesion by vm.sesion.collectAsState(initial = null)
    val mensaje by vm.nube.collectAsState()
    val ocupado by vm.ocupadoNube.collectAsState()
    var ssid by remember { mutableStateOf("") }
    var clave by remember { mutableStateOf("") }
    Seccion("WiFi y nube") {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            FilaDato("WiFi", when {
                estado?.wifi == true -> "conectado"
                estado?.redConfigurada == true -> "configurado, sin conexión"
                else -> "sin configurar"
            })
            FilaDato("Nube", if (estado?.nube == true) "conectado" else "sin conectar")
            OutlinedTextField(value = ssid, onValueChange = { if (it.toByteArray().size <= 32) ssid = it }, label = { Etiqueta("Red") },
                singleLine = true, textStyle = Tipos.cuerpo, modifier = Modifier.fillMaxWidth())
            OutlinedTextField(value = clave, onValueChange = { if (it.toByteArray().size <= 64) clave = it }, label = { Etiqueta("Clave") },
                singleLine = true, textStyle = Tipos.cuerpo, modifier = Modifier.fillMaxWidth(),
                visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation())
            BotonSecundario("Pasarle el WiFi", { vm.configurarRed(ssid, clave) }, Modifier.fillMaxWidth(), habilitado = ssid.isNotBlank() && !ocupado)
            if (sesion?.activa == true) {
                BotonPrimario("Sumar a mi cuenta", { vm.sumarACuenta() }, Modifier.fillMaxWidth(), habilitado = !ocupado && estado?.redConfigurada == true)
            } else {
                Text("Entrá a tu cuenta (en el inicio) para sumarlo y manejarlo desde lejos.", style = Tipos.cuerpo, color = p.tintaSuave)
            }
            if (estado?.redConfigurada == true) TextButton(onClick = { vm.olvidarRed() }) { Text("OLVIDAR WIFI Y NUBE", style = Tipos.boton, color = p.tintaSuave) }
            mensaje?.let { Aviso(it) }
        }
    }
}

@Composable
private fun SeccionFirmware(vm: BichoViewModel, estado: Estado?) {
    val p = LocalPaleta.current
    val lista by vm.firmwares.collectAsState()
    val progreso by vm.progresoOta.collectAsState()
    val mensaje by vm.mensajeOta.collectAsState()
    Seccion("Firmware") {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            FilaDato("Versión", estado?.version?.ifEmpty { "—" } ?: "—")
            FilaDato("Perfil", estado?.perfil?.ifEmpty { "—" } ?: "—")
            if (lista.isEmpty()) BotonSecundario("Buscar versiones", { vm.buscarFirmwares() }, Modifier.fillMaxWidth(), habilitado = progreso == null)
            for (f in lista) {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween, modifier = Modifier.fillMaxWidth()) {
                    Text("${f.version} · ${f.tamano / 1024} KB", style = Tipos.dato, color = p.tinta)
                    TextButton(onClick = { vm.instalarPorBle(f) }, enabled = progreso == null && f.version != estado?.version) {
                        Text(if (f.version == estado?.version) "INSTALADA" else "INSTALAR", style = Tipos.boton, color = p.naranjaProfundo)
                    }
                }
            }
            progreso?.let { LinearProgressIndicator(progress = { it }, modifier = Modifier.fillMaxWidth(), color = p.naranja) }
            mensaje?.let { Aviso(it) }
        }
    }
}
