package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.sePuedeSoltar
import com.sinteticalabs.app.datos.Actualizacion
import com.sinteticalabs.app.datos.Firmware
import com.sinteticalabs.app.datos.VersionConfig
import com.sinteticalabs.app.datos.actualizaciones
import com.sinteticalabs.app.datos.actualizar
import com.sinteticalabs.app.datos.firmwaresDe
import com.sinteticalabs.app.datos.historialConfig
import com.sinteticalabs.app.datos.restaurarConfig
import com.sinteticalabs.app.nucleo.Nucleo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

private val FECHA = SimpleDateFormat("d MMM HH:mm", Locale.forLanguageTag("es-AR"))
private fun fecha(ms: Long?) = ms?.let { FECHA.format(Date(it)) } ?: "—"

private enum class Rack(val nombre: String) {
    EXPRESIONES("Expresiones"), COMPORTAMIENTOS("Comportamientos"), REGLAS("Reglas"), VIDA("Vida"), DIBUJOS("Dibujos"),
    ANIMACIONES("Animaciones"), CARAS("Caras"), HISTORIAL("Historial"), FIRMWARE("Firmware"),
}

/** El laboratorio: lo avanzado, como módulos de rack (web: routes/robots/[id]/+layout.svelte). */
@Composable
fun PantallaLaboratorio(vm: RobotViewModel, modelo: Modelo, alVolver: () -> Unit) {
    var rack by rememberSaveable { mutableStateOf(Rack.EXPRESIONES) }
    val llave by vm.vistaPrevia.collectAsState()
    val robot by vm.robot.collectAsState()
    Column(Modifier.fillMaxSize().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
        Row(Modifier.fillMaxWidth().padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Tecla("←", alVolver, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Consola")
            Grabado("Laboratorio · ${robot?.nombre?.ifEmpty { null } ?: "tu robot"}", color = C.brasa)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(14.dp)) {
            Box(Modifier.widthIn(max = 120.dp)) {
                // Un solo dibujo vivo de la instancia a la vez: si el rack trae la suya, acá va quieta.
                val propia = rack in setOf(Rack.COMPORTAMIENTOS, Rack.REGLAS, Rack.DIBUJOS, Rack.ANIMACIONES)
                Producto(modelo, radio = 18.dp, apagado = robot?.conectado != true) { if (llave > 0 && !propia) PantallaViva(llave, "Vista previa") }
            }
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Rack.entries.forEach { r ->
                    if (r == Rack.HISTORIAL && vm.via != Via.NUBE) return@forEach
                    Tecla(r.nombre, { rack = r }, acabado = if (rack == r) Acabado.NEGRA else Acabado.BLANCA, chica = true, apretada = rack == r)
                }
            }
        }
        when (rack) {
            Rack.EXPRESIONES -> LabExpresiones(vm, modelo)
            Rack.COMPORTAMIENTOS -> LabComportamientos(vm, modelo)
            Rack.REGLAS -> LabReglas(vm, modelo)
            Rack.VIDA -> LabVida(vm)
            Rack.DIBUJOS -> LabDibujos(vm, modelo)
            Rack.ANIMACIONES -> LabAnimaciones(vm, modelo)
            else -> Chapa(Modifier.fillMaxWidth()) {
                when (rack) {
                    Rack.HISTORIAL -> RackHistorial(vm)
                    Rack.FIRMWARE -> RackFirmware(vm, modelo)
                    else -> RackCaras(vm)
                }
            }
        }
        Spacer(Modifier.height(32.dp))
    }
}

/** Todas las caras que sabe hacer (las del núcleo y las propias): se fijan en el robot. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun RackCaras(vm: RobotViewModel) {
    val r by vm.robot.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var error by remember { mutableStateOf<String?>(null) }
    val caras = remember(llave) { runCatching { Nucleo.expresiones().toList() }.getOrDefault(emptyList()) }
    val conectado = r?.conectado ?: false
    val e = r?.estado
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
        Grabado("${caras.size} caras")
        Tecla("Soltar", { alcance.launch { error = vm.comando(expresion = "") } }, acabado = Acabado.NARANJA, chica = true, habilitada = conectado && sePuedeSoltar(e))
    }
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        caras.forEach { n ->
            Tecla(n, { alcance.launch { error = vm.comando(expresion = n) } }, acabado = Acabado.NEGRA, chica = true, habilitada = conectado,
                apretada = e?.fija == true && e.expresion == n)
        }
    }
    error?.let { AvisoLinea(it, error = true) }
}

/** Cada cambio de la config, con quién lo hizo; se puede volver a uno anterior. */
@Composable
private fun RackHistorial(vm: RobotViewModel) {
    val api = vm.apiNube() ?: return
    val actual by vm.versionConfig.collectAsState()
    val alcance = rememberCoroutineScope()
    var versiones by remember { mutableStateOf<List<VersionConfig>>(emptyList()) }
    var aviso by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(actual) { runCatching { versiones = api.historialConfig(vm.idRobot) } }
    val autores = mapOf("web" to "Web", "app" to "App", "bicho" to "Robot", "api" to "Nube")
    Grabado("Cambios")
    aviso?.let { AvisoLinea(it) }
    if (versiones.isEmpty()) Text("De fábrica.", style = T.texto)
    versiones.forEach { v ->
        val es = v.version == actual
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(if (es) C.hundido else C.chapa).padding(start = 14.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Led(if (es) C.led else null, tamano = 7.dp)
            Text("v${v.version}", style = T.tecla, color = C.papel)
            Column(Modifier.weight(1f)) {
                Text(autores[v.autor] ?: if (v.autor.startsWith("mcp:")) "IA · ${v.autor.drop(4)}" else v.autor, style = T.dato, maxLines = 1)
                Text(fecha(v.creada), style = T.dato.copy(fontSize = 12.sp), color = C.gris)
            }
            if (es) Grabado("Actual", color = C.brasa)
            else Tecla("↺", {
                alcance.launch {
                    aviso = runCatching { api.restaurarConfig(vm.idRobot, v.version); vm.recargarConfig(); "Volviste a la v${v.version}" }.getOrElse { it.message }
                    runCatching { versiones = api.historialConfig(vm.idRobot) }
                    delay(3000)
                    aviso = null
                }
            }, chica = true, descripcion = "Volver a la v${v.version}")
        }
    }
}

/** El firmware: la versión que tiene, las que hay y cómo van las actualizaciones. */
@Composable
private fun RackFirmware(vm: RobotViewModel, modelo: Modelo) {
    val r by vm.robot.collectAsState()
    val ota by vm.ota.collectAsState()
    val progresoBle by vm.progresoOta.collectAsState()
    val mensaje by vm.mensajeCerca.collectAsState()
    val alcance = rememberCoroutineScope()
    var firmwares by remember { mutableStateOf<List<Firmware>>(emptyList()) }
    var historial by remember { mutableStateOf<List<Actualizacion>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    var seguro by remember { mutableLongStateOf(0L) }
    val e = r?.estado
    val perfil = e?.perfil?.ifEmpty { null } ?: r?.perfil.orEmpty()
    val api = vm.apiNube()
    LaunchedEffect(perfil, api) {
        if (api == null || perfil.isEmpty()) return@LaunchedEffect
        runCatching { firmwares = api.firmwaresDe(perfil) }.onFailure { error = it.message }
        if (vm.via == Via.NUBE) runCatching { historial = api.actualizaciones(vm.idRobot) }
    }
    LaunchedEffect(ota) {
        val o = ota ?: return@LaunchedEffect
        historial = if (historial.any { it.id == o.id }) historial.map { if (it.id == o.id) o else it } else listOf(o) + historial
    }
    val actual = e?.version?.ifEmpty { null } ?: "—"
    val puede = when (vm.via) {
        Via.NUBE -> r?.conectado == true && e?.wifi == true && e?.actualizando != true
        Via.CERCA -> progresoBle == null
        Via.DEMO -> false
    }
    Visor(Modifier.fillMaxWidth()) {
        Grabado("${modelo.nombre} · tiene")
        Text(actual, style = T.digitos.copy(fontSize = 32.sp))
    }
    if (api == null) Text("Entrá a tu cuenta.", style = T.texto)
    error?.let { AvisoLinea(it, error = true) }
    progresoBle?.let { LinearProgressIndicator(progress = { it }, color = C.led, trackColor = C.hundido, modifier = Modifier.fillMaxWidth()) }
    if (vm.via == Via.CERCA) mensaje?.let { AvisoLinea(it) }
    firmwares.forEach { f ->
        val es = f.version == actual
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(C.hundido).padding(start = 14.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Column(Modifier.weight(1f)) {
                Text(f.version, style = T.tecla, color = C.papel)
                Text(fecha(f.creado), style = T.dato.copy(fontSize = 12.sp), color = C.gris)
            }
            Tecla(
                if (es) "Instalada" else if (seguro == f.id) "¿Seguro?" else "Instalar",
                {
                    if (seguro != f.id) { seguro = f.id; alcance.launch { delay(5000); if (seguro == f.id) seguro = 0L } }
                    else {
                        seguro = 0L
                        if (vm.via == Via.CERCA) vm.instalarPorBle(f)
                        else alcance.launch {
                            runCatching { val u = api!!.actualizar(vm.idRobot, f.id); historial = listOf(u) + historial; error = null }
                                .onFailure { error = it.message }
                        }
                    }
                },
                acabado = if (seguro == f.id) Acabado.NARANJA else Acabado.NEGRA, chica = true, habilitada = !es && puede,
            )
        }
    }
    if (firmwares.isEmpty() && api != null) Text("No hay versiones nuevas.", style = T.texto)
    if (historial.isNotEmpty()) {
        Grabado("Historial")
        val nombres = mapOf("enviada" to "Enviada", "descargando" to "Bajando", "instalada" to "Probando", "confirmada" to "Lista", "fallida" to "Falló")
        historial.forEach { h ->
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Led(when (h.estado) { "confirmada" -> C.verde; "fallida" -> C.rojo; else -> C.led }, tamano = 7.dp)
                Text(h.version, style = T.dato.copy(color = C.papel))
                Text(nombres[h.estado] ?: h.estado, style = T.dato, color = if (h.estado == "fallida") C.rojo else C.crema)
                if (h.estado == "descargando") LinearProgressIndicator(progress = { h.progreso / 100f }, color = C.led, trackColor = C.hundido, modifier = Modifier.size(60.dp, 4.dp))
                Text(fecha(h.creada), style = T.dato.copy(fontSize = 12.sp), color = C.gris)
            }
        }
    }
}
