package com.sinteticalabs.app.ui.consola

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.protocolo.Animacion
import kotlinx.coroutines.launch

/**
 * Encender un robot: el onboarding de la primera vez. Cuatro pasos (nombre,
 * carácter, su día y una primera imagen), con el robot reaccionando a cada
 * elección (web: routes/robots/[id]/encender).
 */
@Composable
fun PantallaEncender(vm: RobotViewModel, modelo: Modelo, alTerminar: () -> Unit) {
    val base by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var paso by remember { mutableIntStateOf(1) }
    var nombre by remember { mutableStateOf(robot?.apodo?.ifEmpty { null } ?: base.texto("nombre").orEmpty()) }
    var tuNombre by remember { mutableStateOf(base.obj("dueno")?.texto("nombre").orEmpty()) }
    var cumple by remember { mutableStateOf(Editar.partes(base.obj("dueno")?.texto("cumple"))) }
    var preset by remember { mutableStateOf(base.obj("personalidad")?.texto("preset") ?: "picaro") }
    var despierta by remember { mutableStateOf(Editar.despierta(base)) }
    var duerme by remember { mutableStateOf(Editar.duerme(base)) }
    var foto by remember { mutableStateOf<ByteArray?>(null) }
    var vistaFoto by remember { mutableStateOf<Vista?>(null) }
    var protector by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var encendiendo by remember { mutableStateOf(false) }
    val quien = nombre.trim().ifEmpty { "Tu robot" }

    val borrador = Editar.encendido(base, nombre, preset, despierta, duerme, tuNombre, cumple.first, cumple.second)
    val cara = when (paso) {
        1 -> "contento"
        2 -> when (preset) { "dormilon" -> "cansado"; "grunon" -> "grunon"; "timido" -> "verguenza"; "carinoso" -> "enamorado"; "electrico" -> "emocionado"; "curioso" -> "curioso"; else -> "guino" }
        3 -> "dormido"
        else -> "enamorado"
    }
    val dice = when (paso) {
        1 -> "$quien se despierta"
        2 -> "$quien prueba ser ${(Editar.NOMBRES_PRESET[preset] ?: preset).lowercase()}"
        3 -> "De noche, $quien duerme"
        else -> "$quien ya te quiere"
    }
    // Lo que se ve al lado es la config que se está armando.
    LaunchedEffect(borrador.toString(), llave) { vm.aplicarEnVistaPrevia(borrador) }
    LaunchedEffect(cara, llave) { vm.mostrarEnVistaPrevia(cara) }

    val galeria = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri: Uri? ->
        if (uri != null) alcance.launch {
            error = null
            runCatching { vm.preparar(uri) }.onSuccess { q ->
                foto = q
                val primero = Animacion.cuadros(q).firstOrNull()?.qoi
                vistaFoto = primero?.let { com.sinteticalabs.app.bicho.PrepararImagen.decodificar(it) }?.let { Vista(listOf(it.asImageBitmap() to 0)) }
            }.onFailure { error = it.message ?: "No pude prepararla." }
        }
    }

    fun encender() = alcance.launch {
        encendiendo = true
        error = vm.encender(nombre, borrador, foto, protector)
        encendiendo = false
        if (error == null) alTerminar()
    }

    Column(Modifier.fillMaxSize().safeDrawingPadding().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
        // El progreso: cuatro LEDs.
        Row(Modifier.fillMaxWidth().padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
            listOf("Nombre", "Carácter", "Día", "Foto").forEachIndexed { i, p ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Led(if (i < paso) C.led else null, tamano = 7.dp)
                    Grabado(p, color = if (i + 1 == paso) C.papel else C.gris)
                }
            }
        }
        Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
            Producto(modelo, Modifier.widthIn(max = 360.dp), etiqueta = dice) {
                if (llave > 0) PantallaViva(llave, dice)
            }
        }
        Grabado("Paso $paso de 4", color = C.brasa)
        when (paso) {
            1 -> {
                Text("PONELE\nNOMBRE.", style = T.titular, color = C.papel)
                Visor(Modifier.fillMaxWidth()) {
                    Grabado("Se llama")
                    CampoVisor(nombre, { nombre = it }, placeholder = "Lucho", alEscribir = { nombre = it })
                }
                Visor(Modifier.fillMaxWidth()) {
                    Grabado("Vos")
                    CampoVisor(tuNombre, { tuNombre = it }, placeholder = "Tu nombre", estilo = T.campoClaro, alEscribir = { tuNombre = it })
                }
                VisorCumple("Tu cumple", cumple, Modifier.fillMaxWidth()) { m, d -> cumple = m to d }
            }
            2 -> {
                Text("¿CÓMO\nES?", style = T.titular, color = C.papel)
                PlacaNaranja(Modifier.fillMaxWidth()) {
                    Dial(preset, { preset = it }, Modifier.fillMaxWidth())
                    Grabado("Carácter", color = C.sobrePlaca)
                    Text((Editar.NOMBRES_PRESET[preset] ?: preset).uppercase(), style = T.nombre, color = C.tinta)
                    Text(DICE_PRESET[preset].orEmpty(), style = T.texto, color = androidx.compose.ui.graphics.Color(0xCC1A1714))
                }
            }
            3 -> {
                Text("SU DÍA.", style = T.titular, color = C.papel)
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    VisorHora("Despierta", despierta, Modifier.weight(1f)) { d ->
                        val n = Editar.aHora(Editar.aMin(despierta) + d)
                        if (n != duerme) despierta = n
                    }
                    VisorHora("Duerme", duerme, Modifier.weight(1f)) { d ->
                        val n = Editar.aHora(Editar.aMin(duerme) + d)
                        if (n != despierta) duerme = n
                    }
                }
                Barra24(despierta, duerme)
            }
            else -> {
                Text("¿UNA\nFOTO?", style = T.titular, color = C.papel)
                Box(
                    Modifier.fillMaxWidth().aspectRatio(if (modelo.redonda) 1f else modelo.pantalla.ancho.toFloat() / modelo.pantalla.alto)
                        .clip(if (modelo.redonda) CircleShape else RoundedCornerShape(18.dp)).background(C.hundido)
                        .clickable { galeria.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageAndVideo)) },
                ) {
                    val v = vistaFoto
                    if (v != null) Animada(v) else Mas()
                    Grabado("Bahía 01", Modifier.align(Alignment.TopStart).padding(12.dp), color = C.grisClaro)
                }
                Llave("Protector", protector, { protector = !protector })
            }
        }
        error?.let { AvisoLinea(it, error = true) }
        Row(Modifier.fillMaxWidth().padding(bottom = 32.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            if (paso > 1) Tecla("←", { paso-- }, cuadrada = true, alto = 52.dp, descripcion = "Atrás")
            if (paso < 4) Tecla("Siguiente", { paso++ }, Modifier.weight(1f), acabado = Acabado.NARANJA)
            else Tecla(if (encendiendo) "Encendiendo…" else "Encender", { encender() }, Modifier.weight(1f), acabado = Acabado.NARANJA, habilitada = !encendiendo)
            Tecla("Después", { vm.marcarEncendido(); alTerminar() }, acabado = Acabado.NEGRA)
        }
    }
}
