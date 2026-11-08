package com.sinteticalabs.app.ui.consola

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.consola.Capacidad
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.animoDe
import com.sinteticalabs.app.consola.hace
import com.sinteticalabs.app.consola.numeroDe
import com.sinteticalabs.app.consola.sePuedeSoltar
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.datos.EnVivo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Los diez módulos del teclado (como routes/robots/[id]/+page.svelte). */
enum class Modulo(val nombre: String, val necesita: Set<Capacidad> = emptySet()) {
    CARACTER("Carácter"), CARA("Cara"), IMAGENES("Imágenes"), DIA("Su día"), RECORDATORIOS("Recordatorios"),
    MODOS("Modos"), REFLEJOS("Reflejos"), INTEGRACIONES("Integraciones"), VOZ("Voz", setOf(Capacidad.MICROFONO, Capacidad.PARLANTE)), AJUSTES("Ajustes"),
}

/**
 * La consola de un robot: arriba él (la foto real con su cara en vivo), abajo
 * una placa naranja con un teclado de módulos. Todo se guarda solo.
 */
@Composable
fun PantallaConsola(vm: RobotViewModel, alVolver: () -> Unit) {
    val robot by vm.robot.collectAsState()
    val error by vm.error.collectAsState()
    val modelo by vm.modelo.collectAsState()
    var vista by rememberSaveable { mutableStateOf("consola") }
    val config by vm.config.collectAsState()

    // La primera vez que se abre un robot sin bautizar, se enciende (onboarding).
    var revisado by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(robot != null, config) {
        if (revisado || robot == null || config.isEmpty() || vm.via == Via.DEMO) return@LaunchedEffect
        revisado = true
        if (!vm.encendidoVisto() && Editar.sinEncender(config)) vista = "encender"
    }
    BackHandler(vista != "consola") { vista = "consola"; vm.pausarEspejo(0.0); vm.aplicarEnVistaPrevia(vm.config.value) }

    Box(Modifier.fillMaxSize().fondoConsola()) {
        val m = modelo
        when {
            error != null -> Column(Modifier.safeDrawingPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Tecla("←", alVolver, acabado = Acabado.NEGRA, chica = true, descripcion = "Volver")
                Text("Sin conexión", style = T.titular.copy(color = C.papel))
                AvisoLinea(error!!, error = true)
                Tecla("Probar de nuevo", { vm.reintentar() }, acabado = Acabado.NARANJA)
            }
            robot == null || m == null -> Column(Modifier.safeDrawingPadding().padding(16.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Tecla("←", alVolver, acabado = Acabado.NEGRA, chica = true, descripcion = "Volver")
                Grabado(if (vm.via == Via.CERCA) "Buscando…" else "Cargando…", color = C.brasa)
                CircularProgressIndicator(color = C.led)
            }
            vista == "encender" -> PantallaEncender(vm, m, alTerminar = { vista = "consola"; vm.aplicarEnVistaPrevia(vm.config.value) })
            vista == "lab" -> PantallaLaboratorio(vm, m, alVolver = { vista = "consola"; vm.pausarEspejo(0.0); vm.aplicarEnVistaPrevia(vm.config.value) })
            else -> Consola(vm, m, alVolver, alLab = { vista = "lab" })
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Consola(vm: RobotViewModel, modelo: Modelo, alVolver: () -> Unit, alLab: () -> Unit) {
    val r by vm.robot.collectAsState()
    val robot = r ?: return
    val config by vm.config.collectAsState()
    val guardado by vm.guardado.collectAsState()
    val errorGuardado by vm.errorGuardado.collectAsState()
    val aviso by vm.aviso.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val enVivo by EnVivo.conectado.collectAsState()
    var actual by rememberSaveable { mutableStateOf(Modulo.CARACTER) }
    var respuesta by remember { mutableStateOf<String?>(null) }
    val alcance = rememberCoroutineScope()
    val nombre = robot.apodo.ifEmpty { config.texto("nombre") ?: robot.estado?.nombre ?: "" }.ifEmpty { "Tu robot" }
    val conectado = robot.conectado
    val estado = robot.estado

    fun hacer(evento: String?, expresion: String?, texto: String) = alcance.launch {
        respuesta = vm.comando(expresion = expresion, evento = evento) ?: texto
        delay(2500)
        respuesta = null
    }

    Column(Modifier.fillMaxSize().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
        // La barra: volver, cómo va el guardado y el laboratorio.
        Row(Modifier.fillMaxWidth().padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Tecla("←", alVolver, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Mis robots")
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Led(
                    when (guardado) {
                        EstadoGuardado.GUARDANDO -> C.led
                        EstadoGuardado.GUARDADO -> C.verde
                        EstadoGuardado.ERROR -> C.rojo
                        EstadoGuardado.LISTO -> if (vm.via != Via.NUBE || enVivo) C.verde.copy(alpha = 0.5f) else null
                    },
                )
                Grabado(
                    when (guardado) {
                        EstadoGuardado.LISTO -> "Al día"
                        EstadoGuardado.GUARDANDO -> "Guardando…"
                        EstadoGuardado.GUARDADO -> if (conectado) "Enviado" else "Le llega al volver"
                        EstadoGuardado.ERROR -> "No se guardó"
                    },
                    color = if (guardado == EstadoGuardado.ERROR) Color(0xFFFF7A7A) else C.gris,
                )
            }
            Tecla("Lab", alLab, acabado = Acabado.NEGRA, chica = true, descripcion = "Laboratorio")
        }

        // El aviso del robot: un recordatorio queda hasta que lo tocan (o 2 min); la IA, 5 s.
        AnimatedVisibility(aviso != null, enter = fadeIn() + slideInVertically { -it / 3 }, exit = fadeOut()) {
            val a = aviso
            if (a != null) Row(
                Modifier.fillMaxWidth().drawBehind {
                    drawRoundRect(Color(0xFF9A3209), Offset(0f, 6.dp.toPx()), Size(size.width, size.height - 6.dp.toPx()), CornerRadius(18.dp.toPx()))
                }.padding(bottom = 6.dp).clip(RoundedCornerShape(18.dp))
                    .background(Brush.verticalGradient(listOf(Color(0xFFFF8A5C), Color(0xFFF2541A))))
                    .padding(start = 18.dp, end = 12.dp, top = 12.dp, bottom = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Text(a.icono, style = T.aviso.copy(fontSize = T.aviso.fontSize * 1.2f))
                Text(a.texto.uppercase(), style = T.aviso, modifier = Modifier.weight(1f), maxLines = 3, overflow = TextOverflow.Ellipsis)
                Tecla("OK", { vm.cerrarAviso() }, acabado = Acabado.NEGRA, chica = true)
            }
        }

        // Él: la foto real con su pantalla viva.
        Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
            Producto(modelo, Modifier.widthIn(max = 420.dp), apagado = !conectado, etiqueta = if (conectado) "$nombre · en vivo" else "Así se ve $nombre") {
                if (llave > 0) PantallaViva(llave, "Pantalla de $nombre, en vivo") { com.sinteticalabs.app.nucleo.Nucleo.tacto(it) }
            }
        }

        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Led(if (conectado) C.verde else null)
                Grabado(
                    (if (conectado) "En línea" else "Visto ${hace(robot.ultimaConexion)}") + " · ${modelo.nombre} ${numeroDe(robot.id)}" + if (robot.virtual) " · virtual" else "",
                    color = C.grisClaro,
                )
            }
            Text(nombre.uppercase(), style = T.titular, color = C.papel, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(animoDe(conectado, estado), style = T.campoClaro.copy(color = C.brasa))
                estado?.bateria?.let { b ->
                    Text("$b%", style = T.digitosChicos.copy(fontSize = T.dato.fontSize, shadow = null),
                        modifier = Modifier.clip(RoundedCornerShape(8.dp)).background(C.hundido).padding(horizontal = 10.dp, vertical = 6.dp))
                }
            }
            // Lo que trae.
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Capacidad.entries.forEach { c ->
                    val tiene = c in modelo.capacidades
                    Row(
                        Modifier.height(32.dp).clip(RoundedCornerShape(9.dp)).background(if (tiene) C.chapa else Color.Transparent).padding(horizontal = 12.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Led(if (tiene) C.led else null, tamano = 7.dp)
                        Text(c.nombre.uppercase(), style = T.teclaChica.copy(fontSize = T.grabado.fontSize), color = if (tiene) C.crema else C.grisOscuro)
                    }
                }
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Tecla("Caricia", { hacer("caricia", null, "Le gustó") }, habilitada = conectado)
                Tecla("Llamarlo", { hacer("nombre", null, "Te escuchó") }, habilitada = conectado)
                Tecla("Cosquillas", { hacer("muchos-toques", null, "Se rió") }, acabado = Acabado.NARANJA, habilitada = conectado)
                if (sePuedeSoltar(estado)) Tecla("Soltar", { hacer(null, "", "Vuelve a ser él") }, acabado = Acabado.NEGRA, habilitada = conectado)
            }
            respuesta?.let { AvisoLinea(it) }
            if (!conectado) Text("Le llega cuando vuelva.", style = T.texto)
        }

        // La placa: el teclado de módulos y el módulo elegido.
        PlacaNaranja(Modifier.fillMaxWidth().padding(bottom = 32.dp), relleno = 14.dp) {
            Teclado(actual, modelo) { actual = it }
            if (guardado == EstadoGuardado.ERROR && errorGuardado.isNotEmpty()) AvisoLinea(errorGuardado, error = true)
            when (actual) {
                Modulo.CARACTER -> ModCaracter(vm)
                Modulo.CARA -> ModCara(vm)
                Modulo.IMAGENES -> ModImagenes(vm, modelo)
                Modulo.DIA -> ModDia(vm)
                Modulo.RECORDATORIOS -> ModRecordatorios(vm, modelo)
                Modulo.MODOS -> ModModos(vm)
                Modulo.REFLEJOS -> ModReflejos(vm, modelo)
                Modulo.INTEGRACIONES -> ModIntegraciones(vm)
                Modulo.VOZ -> ModVoz(vm, modelo)
                Modulo.AJUSTES -> ModAjustes(vm, modelo, alVolver, alLab)
            }
        }
    }
}

/** El teclado: una tecla mecánica por módulo, con su número y su LED. */
@Composable
private fun Teclado(actual: Modulo, modelo: Modelo, alElegir: (Modulo) -> Unit) {
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val columnas = (maxWidth / 92.dp).toInt().coerceIn(3, 5)
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Modulo.entries.chunked(columnas).forEach { fila ->
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    fila.forEach { m ->
                        val bloqueado = m.necesita.isNotEmpty() && m.necesita.none { it in modelo.capacidades }
                        TeclaModulo(m.ordinal + 1, m.nombre, actual == m, bloqueado, Modifier.weight(1f)) { alElegir(m) }
                    }
                    repeat(columnas - fila.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
    }
}

@Composable
private fun TeclaModulo(n: Int, nombre: String, elegida: Boolean, bloqueado: Boolean, modifier: Modifier, alTocar: () -> Unit) {
    val fuente = remember { MutableInteractionSource() }
    val apretando by fuente.collectIsPressedAsState()
    val baja by animateDpAsState(if (apretando || elegida) 5.dp else 0.dp, tween(80), label = "modulo")
    val alto = 70.dp
    Box(
        modifier.height(alto + 6.dp)
            .semantics { selected = elegida }
            .clickable(fuente, indication = null, role = Role.Tab, onClick = alTocar)
            .drawBehind {
                val r = CornerRadius(16.dp.toPx())
                val h = alto.toPx()
                val y = baja.toPx()
                drawRoundRect(Color(0x55501400), Offset(0f, 10.dp.toPx()), Size(size.width, h), r)
                drawRoundRect(if (elegida) Color.Black else Color(0xFF8F887C), Offset(0f, 6.dp.toPx()), Size(size.width, h), r)
                val cara = if (elegida) listOf(Color(0xFF2A2622), Color(0xFF151310)) else listOf(Color(0xFFFBF9F5), Color(0xFFE6E0D6))
                drawRoundRect(Brush.verticalGradient(cara, startY = y, endY = y + h), Offset(0f, y), Size(size.width, h), r)
                drawLine(if (elegida) Color(0x1FFFFFFF) else Color.White, Offset(16.dp.toPx(), y + 1f), Offset(size.width - 16.dp.toPx(), y + 1f), 1.dp.toPx())
            },
    ) {
        Column(Modifier.offset(y = baja).height(alto).fillMaxWidth().padding(10.dp), verticalArrangement = Arrangement.SpaceBetween) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
                Text("%02d".format(n), style = T.modulo, color = (if (elegida) C.papel else C.tinta).copy(alpha = 0.55f))
                if (bloqueado) Box(Modifier.size(9.dp).clip(RoundedCornerShape(50)).background(Color.Transparent).alpha(0.5f).drawBehind {
                    drawCircle(if (elegida) C.papel else C.tinta, size.minDimension / 2, style = androidx.compose.ui.graphics.drawscope.Stroke(2.dp.toPx()))
                })
                else Led(if (elegida) C.led else C.grisClaro)
            }
            Text(nombre.uppercase(), style = T.modulo, color = if (elegida) C.papel else C.tinta, maxLines = 2)
        }
    }
}
