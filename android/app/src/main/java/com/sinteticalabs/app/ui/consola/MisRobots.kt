package com.sinteticalabs.app.ui.consola

import android.app.Application
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.sinteticalabs.app.R
import com.sinteticalabs.app.consola.VACIO
import com.sinteticalabs.app.consola.animoDe
import com.sinteticalabs.app.consola.hace
import com.sinteticalabs.app.consola.lista
import com.sinteticalabs.app.consola.modeloDe
import com.sinteticalabs.app.consola.numeroDe
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.pantallaDe
import com.sinteticalabs.app.consola.si
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.datos.Cuenta
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.RobotNube
import com.sinteticalabs.app.datos.Sesion
import com.sinteticalabs.app.datos.Vinculado
import com.sinteticalabs.app.datos.Vinculados
import com.sinteticalabs.app.datos.config
import com.sinteticalabs.app.datos.estadoDeJson
import com.sinteticalabs.app.datos.robots
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** Mis robots: los de la cuenta (en vivo por la nube) y los vinculados a este celu. */
class RobotsViewModel(app: Application) : AndroidViewModel(app) {
    private val cuenta = Cuenta(app)
    val sesion: StateFlow<Sesion?> = cuenta.sesion.stateIn(viewModelScope, SharingStarted.Eagerly, null)
    val robots = MutableStateFlow<List<RobotNube>>(emptyList())
    val configs = MutableStateFlow<Map<String, JsonObject>>(emptyMap())
    val vinculados: StateFlow<List<Vinculado>> = Vinculados(app).todos.stateIn(viewModelScope, SharingStarted.Eagerly, emptyList())
    val cargando = MutableStateFlow(true)
    val error = MutableStateFlow<String?>(null)
    val virtuales = MutableStateFlow<List<Pair<String, Boolean>>>(emptyList())

    init {
        EnVivo.arrancar(cuenta)
        viewModelScope.launch {
            sesion.collect { s -> if (s?.activa == true) cargar() else { robots.value = emptyList(); cargando.value = false } }
        }
        viewModelScope.launch {
            EnVivo.eventos.collect { e ->
                if (e.tipo == "bicho" && e.datos.texto("accion") != "actual") { cargar(); return@collect }
                val l = robots.value
                val i = l.indexOfFirst { it.id == e.robot }
                if (i < 0) return@collect
                val r = l[i]
                val nuevo = when (e.tipo) {
                    "estado" -> estadoDeJson(e.datos)?.let { r.copy(estado = it) }
                    "conexion" -> r.copy(conectado = e.datos.si("conectado") ?: r.conectado)
                    "sincro" -> r.copy(sincroConfig = e.datos.texto("config") ?: r.sincroConfig)
                    "config" -> { cargarConfig(r.id); null }
                    else -> null
                } ?: return@collect
                robots.value = l.toMutableList().also { it[i] = nuevo }
            }
        }
    }

    fun cargar() = viewModelScope.launch {
        val s = cuenta.actual()
        if (!s.activa) { cargando.value = false; return@launch }
        try {
            robots.value = cuenta.api(s).robots()
            robots.value.forEach { cargarConfig(it.id) }
            error.value = null
        } catch (e: ErrorApi) {
            if (e.estado == 401) cuenta.salir() else error.value = e.message
        } catch (e: Exception) {
            error.value = e.message
        } finally {
            cargando.value = false
        }
    }

    private fun cargarConfig(id: String) = viewModelScope.launch {
        val s = cuenta.actual()
        runCatching { cuenta.api(s).config(id).second }.onSuccess { c -> configs.value = configs.value + (id to c) }
    }

    /** Los robots virtuales (solo en una nube local, para probar sin hardware). */
    fun buscarVirtuales() = viewModelScope.launch {
        val s = cuenta.actual()
        if (!s.activa) return@launch
        virtuales.value = runCatching {
            cuenta.api(s).pedir("/v1/bichos/virtuales").lista("virtuales").orEmpty().mapNotNull {
                val o = it as? JsonObject ?: return@mapNotNull null
                (o.texto("id") ?: return@mapNotNull null) to (o.si("sumado") ?: false)
            }
        }.getOrDefault(emptyList())
    }

    fun sumarVirtual(id: String) = viewModelScope.launch {
        val s = cuenta.actual()
        runCatching { cuenta.api(s).pedir("/v1/bichos/reclamar-virtual", "POST", buildJsonObject { put("id", id) }) }
            .onFailure { error.value = it.message }
        buscarVirtuales()
        cargar()
    }
}

/** El dock: cada robot con su foto real, su cara, cómo está y cuánta batería tiene. */
@Composable
fun PantallaMisRobots(vm: RobotsViewModel, alAbrir: (String) -> Unit, alEscanear: () -> Unit, alCuenta: () -> Unit) {
    val sesion by vm.sesion.collectAsState()
    val robots by vm.robots.collectAsState()
    val configs by vm.configs.collectAsState()
    val vinculados by vm.vinculados.collectAsState()
    val cargando by vm.cargando.collectAsState()
    val error by vm.error.collectAsState()
    val enVivo by EnVivo.conectado.collectAsState()
    var sumando by remember { mutableStateOf(false) }
    val activa = sesion?.activa == true
    val cerca = vinculados.filter { v -> robots.none { it.id.equals(v.id, ignoreCase = true) } }
    val enLinea = robots.count { it.conectado } + cerca.size

    // Las fotos de cada pantalla (el núcleo es uno solo: de a uno).
    val clave = robots.joinToString { "${it.id}:${configs[it.id]?.hashCode()}:${it.estado?.expresion}" }
    val pausado = Instantaneas.pausado
    LaunchedEffect(clave, pausado) {
        if (pausado) return@LaunchedEffect
        for (r in robots) Instantaneas.sacar(r.id, pantallaDe(r.perfil, r.estado), configs[r.id] ?: buildJsonObject { put("version", 2) }, r.estado?.expresion.orEmpty())
    }

    Column(
        Modifier.fillMaxSize().fondoConsola().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
        // La barra: la marca y la cuenta.
        Row(Modifier.fillMaxWidth().padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Image(painterResource(R.drawable.ic_isotipo), contentDescription = null, modifier = Modifier.size(34.dp, 22.dp))
                Text("SINTÉTICA", style = T.nombre.copy(fontSize = 20.sp), color = C.papel)
            }
            Tecla(null, alCuenta, acabado = Acabado.NEGRA, chica = true, descripcion = "Tu cuenta") {
                Led(if (activa && enVivo) C.verde else null)
                Text((if (activa) sesion?.email?.substringBefore('@')?.ifEmpty { "Vos" } ?: "Vos" else "Entrar").uppercase(), style = T.teclaChica, color = C.papel, maxLines = 1)
            }
        }

        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Grabado(if (cargando && activa) "Buscando…" else "${robots.size + cerca.size} · $enLinea en línea", color = C.brasa)
            Text("MIS\nROBOTS", style = T.titular, color = C.papel)
            Tecla("+ Sumar uno", { sumando = true; vm.buscarVirtuales() }, acabado = Acabado.NARANJA)
        }
        error?.let { AvisoLinea(it, error = true) }

        if (!activa) Chapa(Modifier.fillMaxWidth()) {
            Grabado("Tu cuenta", color = C.brasa)
            Text("Tus robots, desde cualquier lado.", style = T.texto)
            Tecla("Entrar", alCuenta, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
        }

        BoxWithConstraints(Modifier.fillMaxWidth()) {
            val columnas = if (maxWidth > 600.dp) 3 else if (maxWidth > 420.dp) 2 else 1
            val items: List<Any> = robots + cerca + listOf(Unit)
            Column(verticalArrangement = Arrangement.spacedBy(20.dp)) {
                items.chunked(columnas).forEach { fila ->
                    Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        fila.forEach { x ->
                            when (x) {
                                is RobotNube -> TarjetaRobot(x, Instantaneas.fotos[x.id], configs[x.id], Modifier.weight(1f)) { alAbrir(PREFIJO_NUBE + x.id) }
                                is Vinculado -> TarjetaCerca(x, Modifier.weight(1f)) { alAbrir(x.id) }
                                else -> BahiaLibre(Modifier.weight(1f)) { sumando = true; vm.buscarVirtuales() }
                            }
                        }
                        repeat(columnas - fila.size) { Spacer(Modifier.weight(1f)) }
                    }
                }
            }
        }

        // Sin robot: probarlo en el celu.
        Column(Modifier.padding(bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Grabado("Probar sin robot")
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Tecla("Unidad", { alAbrir(ID_DEMO) }, Modifier.weight(1f), acabado = Acabado.NEGRA, chica = true)
                Tecla("Broche", { alAbrir(ID_DEMO_BROCHE) }, Modifier.weight(1f), acabado = Acabado.NEGRA, chica = true)
            }
        }
    }

    if (sumando) DialogoSumar(vm, alEscanear = { sumando = false; alEscanear() }, alCerrar = { sumando = false })
}

@Composable
private fun TarjetaRobot(r: RobotNube, foto: androidx.compose.ui.graphics.ImageBitmap?, config: JsonObject?, modifier: Modifier, alTocar: () -> Unit) {
    val m = modeloDe(r.perfil, r.virtual, r.estado)
    val bat = r.estado?.bateria
    // La cara en vivo solo mientras la tarjeta se ve en pantalla; si no (o si
    // lo que muestra no es una cara paramétrica), la última foto.
    var visible by remember { mutableStateOf(false) }
    val alto = with(androidx.compose.ui.platform.LocalDensity.current) { androidx.compose.ui.platform.LocalConfiguration.current.screenHeightDp.dp.toPx() }
    val dibujar = remember(config) { com.sinteticalabs.app.consola.Lab.paraDibujar(config ?: buildJsonObject { put("version", 2) }) }
    val expresion = r.estado?.expresion.orEmpty().ifEmpty { "neutral" }
    Chapa(modifier.clickable(onClick = alTocar).onGloballyPositioned { c ->
        val b = c.boundsInWindow()
        visible = b.bottom > 0 && b.top < alto
    }, radio = 30.dp, relleno = 12.dp) {
        Box {
            Producto(m, radio = 22.dp, apagado = !r.conectado) {
                if (visible && r.conectado) CaraViva(dibujar, expresion, m.pantalla, Modifier.fillMaxSize(), fps = 15) { PantallaQuieta(foto) }
                else PantallaQuieta(foto)
            }
            val sello = when {
                r.conectado && r.estado?.dormido == true -> "☾ Durmiendo"
                !r.conectado -> "Fuera de alcance"
                else -> null
            }
            if (sello != null) Text(sello.uppercase(), style = T.grabado, color = C.grisClaro,
                modifier = Modifier.padding(12.dp).clip(RoundedCornerShape(9.dp)).background(Color(0xD10E0D0B)).padding(horizontal = 12.dp, vertical = 8.dp))
        }
        Column(Modifier.padding(horizontal = 6.dp, vertical = 4.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Grabado("${m.nombre} ${numeroDe(r.id)}" + if (r.virtual) " · virtual" else "")
                Led(if (!r.conectado) null else if (r.estado?.dormido == true) C.ambar else C.verde)
            }
            Text(r.nombre.ifEmpty { "Sin nombre" }.uppercase(), style = T.nombre, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Text(if (r.conectado) animoDe(true, r.estado) else "Visto ${hace(r.ultimaConexion)}", style = T.dato, color = if (r.conectado) C.crema else C.gris)
                Text(bat?.let { "$it%" } ?: "—", style = T.digitosChicos.copy(fontSize = 15.sp, shadow = null), color = if (bat == null) C.grisOscuro else C.brasa,
                    modifier = Modifier.clip(RoundedCornerShape(8.dp)).background(C.hundido).padding(horizontal = 10.dp, vertical = 6.dp))
            }
            Box(Modifier.fillMaxWidth().height(6.dp).clip(RoundedCornerShape(3.dp)).background(C.hundido)) {
                if (bat != null) Box(Modifier.fillMaxWidth(bat / 100f).height(6.dp).background(if (bat < 15) C.rojo else C.led))
            }
        }
    }
}

@Composable
private fun TarjetaCerca(v: Vinculado, modifier: Modifier, alTocar: () -> Unit) {
    val m = modeloDe("", false, null)
    Chapa(modifier.clickable(onClick = alTocar), radio = 30.dp, relleno = 12.dp) {
        Producto(m, radio = 22.dp, apagado = true) { PantallaQuieta(null) }
        Column(Modifier.padding(horizontal = 6.dp, vertical = 4.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Grabado("${m.nombre} ${numeroDe(v.id)} · cerca")
                Led(C.led)
            }
            Text(v.nombre.ifEmpty { "Sin nombre" }.uppercase(), style = T.nombre, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text("Por Bluetooth", style = T.dato, color = C.gris)
        }
    }
}

@Composable
private fun BahiaLibre(modifier: Modifier, alTocar: () -> Unit) {
    Column(
        modifier.clip(RoundedCornerShape(30.dp)).border(2.dp, Color(0xFF3A3632), RoundedCornerShape(30.dp)).clickable(onClick = alTocar).padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.fillMaxWidth().aspectRatio(4f / 5f).clip(RoundedCornerShape(22.dp)).background(C.hundido)) { Mas(72.dp) }
        Column(Modifier.padding(horizontal = 6.dp, vertical = 4.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Grabado("Bahía libre")
            Text("SUMAR UNO", style = T.nombre.copy(fontSize = 24.sp))
        }
    }
}

@Composable
private fun DialogoSumar(vm: RobotsViewModel, alEscanear: () -> Unit, alCerrar: () -> Unit) {
    val virtuales by vm.virtuales.collectAsState()
    Dialog(onDismissRequest = alCerrar) {
        Chapa(Modifier.fillMaxWidth().widthIn(max = 480.dp), radio = 30.dp, relleno = 22.dp) {
            Grabado("Bahía libre", color = C.brasa)
            Text("SUMAR\nUNO", style = T.titular.copy(fontSize = 40.sp, lineHeight = 38.sp), color = C.papel)
            listOf("Prendelo: muestra un QR.", "Escanealo.", "Listo: es tuyo.").forEachIndexed { i, t ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                    Box(Modifier.size(30.dp).clip(RoundedCornerShape(9.dp)).background(C.hundido), contentAlignment = Alignment.Center) {
                        Text("${i + 1}", style = T.tecla, color = C.brasa)
                    }
                    Text(t, style = T.texto.copy(color = C.crema))
                }
            }
            Tecla("Escanear", alEscanear, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
            if (virtuales.isNotEmpty()) {
                Grabado("Virtuales")
                virtuales.forEach { (id, sumado) ->
                    Row(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(start = 14.dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Text(id, style = T.tecla, color = C.papel)
                        if (sumado) Grabado("Tuyo", color = C.verde) else Tecla("Sumar", { vm.sumarVirtual(id) }, chica = true)
                    }
                }
            }
            Tecla("Cerrar", alCerrar, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
        }
    }
}
