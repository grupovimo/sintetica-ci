package com.sinteticalabs.app.ui.consola

import android.app.TimePickerDialog
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Capacidad
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.Recordatorio
import com.sinteticalabs.app.consola.Recordatorios
import com.sinteticalabs.app.consola.Reflejos
import com.sinteticalabs.app.consola.con
import com.sinteticalabs.app.consola.cortarBytes
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.consola.numeroDe
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.sePuedeSoltar
import com.sinteticalabs.app.consola.si
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.consola.sin
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlin.math.roundToInt

/** Lo que es cada carácter, en una línea. */
val DICE_PRESET = mapOf(
    "picaro" to "Bromas, guiños, pide mimos.",
    "timido" to "Al principio se esconde.",
    "carinoso" to "Te busca y te extraña.",
    "curioso" to "Investiga cada ruido.",
    "dormilon" to "Siestas largas.",
    "grunon" to "Rezonga, pero te quiere.",
    "electrico" to "No para nunca.",
    "equilibrado" to "Un poco de todo.",
)

private val porcentaje = { v: Double -> "${(v * 100).roundToInt()}%" }

// --- 01 Carácter: un dial con los 8 presets y una perilla por rasgo ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModCaracter(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val preset = Editar.preset(c)
    Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Dial(preset, { p -> vm.cambiar { Editar.ponerPreset(it, p) } }, Modifier.fillMaxWidth())
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Grabado(if (Editar.aMedida(c)) "Carácter · a tu medida" else "Carácter", color = C.sobrePlaca)
            Text((Editar.NOMBRES_PRESET[preset] ?: preset).uppercase(), style = T.nombre, color = C.tinta)
            Text(DICE_PRESET[preset].orEmpty(), style = T.texto, color = Color(0xCC1A1714))
        }
        Bandeja {
            FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(16.dp), maxItemsInEachRow = 3) {
                Editar.RASGOS.forEach { r ->
                    Perilla(Editar.rasgo(c, r), Editar.NOMBRES_RASGO[r] ?: r, { v -> vm.cambiar { Editar.ponerRasgo(it, r, v) } }, Modifier.width(88.dp))
                }
            }
        }
    }
}

// --- 02 Cara: el color y el tamaño de los ojos, y ponerle una cara ya ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModCara(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val r by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    var error by remember { mutableStateOf<String?>(null) }
    val color = Editar.color(c)
    val conectado = r?.conectado ?: false
    val estado = r?.estado
    val fija = estado?.fija ?: false
    fun fijar(n: String) = alcance.launch { error = vm.comando(expresion = n) }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Bandeja {
            Grabado("Ojos", color = C.sobrePlaca)
            FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween, verticalArrangement = Arrangement.spacedBy(14.dp), maxItemsInEachRow = 4) {
                Editar.COLORES.forEach { (hex, n) ->
                    val tono = Color(android.graphics.Color.parseColor(hex))
                    val elegido = color == hex
                    Column(
                        Modifier.width(64.dp).clickable(role = Role.RadioButton) { vm.cambiar { Editar.ponerColor(it, hex) } },
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Canvas(Modifier.size(34.dp)) {
                            val rr = size.minDimension / 2
                            drawCircle(C.hundido, rr)
                            if (elegido) drawCircle(Brush.radialGradient(listOf(tono.copy(alpha = 0.7f), Color.Transparent), center, rr), rr)
                            drawCircle(tono.copy(alpha = if (elegido) 1f else 0.55f), rr * 0.45f)
                            if (elegido) drawCircle(C.tinta, rr, style = Stroke(2.dp.toPx()))
                        }
                        Text(n.uppercase(), style = T.grabado.copy(fontSize = 9.sp), color = C.sobrePlaca, maxLines = 1)
                    }
                }
            }
        }
        Bandeja {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Perilla(Editar.tamano(c), "Tamaño", { v -> vm.cambiar { Editar.ponerTamano(it, v) } }, min = 0.5, max = 1.5, paso = 0.05, tamano = 70.dp, mostrar = porcentaje)
            }
        }
        // Su cara de siempre: la de fábrica o una de imagen, con la miniatura de su ojo.
        val deImagen = com.sinteticalabs.app.consola.CarasDeImagen.deImagen(c)
        val siempre = com.sinteticalabs.app.consola.CarasDeImagen.siempre(c)
        val sprites by vm.sprites.collectAsState()
        if (deImagen.isNotEmpty() || siempre.isNotEmpty()) Bandeja {
            Grabado("Su cara", color = C.sobrePlaca)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                OpcionCara("De fábrica", siempre.isEmpty(), { vm.cambiar { com.sinteticalabs.app.consola.CarasDeImagen.ponerSiempre(it, "") } }) {
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        repeat(2) { Box(Modifier.width(10.dp).height(22.dp).clip(RoundedCornerShape(5.dp)).background(Color(android.graphics.Color.parseColor(color)))) }
                    }
                }
                deImagen.keys.sorted().forEach { n ->
                    val ojo = com.sinteticalabs.app.consola.CarasDeImagen.sprites(deImagen[n])["ojo"]?.let { sprites[it] }
                    OpcionCara(n, siempre == n, { vm.cambiar { com.sinteticalabs.app.consola.CarasDeImagen.ponerSiempre(it, n) } }) {
                        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                            SpriteImagen(ojo, Modifier.size(26.dp))
                            SpriteImagen(ojo, Modifier.size(26.dp), espejo = true)
                        }
                    }
                }
            }
        }
        Bandeja {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
                Grabado("Ponele una cara", color = C.sobrePlaca)
                Tecla(if (fija) "Soltar" else "Sola", { fijar("") }, acabado = Acabado.NEGRA, chica = true, habilitada = conectado && sePuedeSoltar(estado))
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Editar.RAPIDAS.forEach { (n, t) ->
                    Tecla(t, { fijar(n) }, chica = true, habilitada = conectado, apretada = fija && estado?.expresion == n)
                }
            }
            val animaciones = c.obj("animaciones")?.keys.orEmpty()
            if (animaciones.isNotEmpty()) {
                Grabado("Animaciones", color = C.sobrePlaca)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    animaciones.forEach { n -> Tecla("▶ $n", { alcance.launch { error = vm.comando(reproducir = n) } }, acabado = Acabado.NEGRA, chica = true, habilitada = conectado) }
                }
            }
            error?.let { AvisoLinea(it, error = true) }
        }
    }
}

// --- 03 Imágenes: ocho bahías para fotos, GIFs o videos cortos ---

@Composable
fun ModImagenes(vm: RobotViewModel, modelo: Modelo) {
    val c by vm.config.collectAsState()
    val espacios by vm.espacios.collectAsState()
    val vistas by vm.vistas.collectAsState()
    val subiendo by vm.subiendo.collectAsState()
    val error by vm.errorImagen.collectAsState()
    var elegido by remember { mutableIntStateOf(0) }
    val galeria = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri -> if (uri != null) vm.subirImagen(elegido, uri) }
    val protector = Editar.protector(c)
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Bandeja {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Grabado("Protector", color = C.sobrePlaca)
                    Text(if (protector != null) "BAHÍA %02d".format(protector + 1) else "APAGADO", style = T.nombre.copy(fontSize = 22.sp), color = C.tinta)
                }
                if (protector != null) Tecla("Apagar", { vm.cambiar { Editar.ponerProtector(it, null) } }, acabado = Acabado.NEGRA, chica = true)
            }
            if (protector != null) Visor(Modifier.fillMaxWidth()) {
                Grabado("Aparece tras")
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Tecla("−", { vm.cambiar { Editar.moverEspera(it, -1) } }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Menos")
                    Text(Editar.textoEspera(Editar.espera(c)), style = T.digitosChicos, modifier = Modifier.weight(1f))
                    Tecla("+", { vm.cambiar { Editar.moverEspera(it, 1) } }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Más")
                }
            }
        }
        error?.let { AvisoLinea(it, error = true) }
        (0 until 8).chunked(2).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                fila.forEach { e ->
                    val ocupado = espacios.any { it.espacio == e }
                    Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        val forma = if (modelo.redonda) CircleShape else RoundedCornerShape(14.dp)
                        Box(
                            Modifier.fillMaxWidth().aspectRatio(if (modelo.redonda) 1f else modelo.pantalla.ancho.toFloat() / modelo.pantalla.alto)
                                .clip(forma).background(C.hundido)
                                .clickable(enabled = subiendo < 0) {
                                    elegido = e
                                    galeria.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageAndVideo))
                                },
                        ) {
                            val v = vistas[e]
                            when {
                                subiendo == e -> Grabado("Subiendo…", Modifier.align(Alignment.Center), color = C.brasa)
                                v != null -> Animada(v)
                                ocupado -> Grabado("…", Modifier.align(Alignment.Center))
                                else -> Mas(36.dp)
                            }
                            Grabado("%02d".format(e + 1), Modifier.align(Alignment.TopStart).padding(10.dp), color = C.grisClaro)
                            if ((v?.cuadros?.size ?: 0) > 1) Grabado("GIF", Modifier.align(Alignment.BottomEnd).padding(10.dp), color = C.brasa)
                        }
                        if (ocupado) Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                            Llave("Lista", Editar.enLista(c, e), { vm.cambiar { Editar.mostrar(it, e, !Editar.enLista(it, e)) } }, Modifier.weight(1f))
                            Llave("Prot.", protector == e, { vm.cambiar { Editar.ponerProtector(it, if (protector == e) null else e) } }, Modifier.weight(1f))
                        }
                        if (ocupado) Tecla("✕", { vm.borrarImagen(e) }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA, chica = true, descripcion = "Borrar la bahía ${e + 1}")
                    }
                }
            }
        }
    }
}

/** Una imagen de una bahía: si es un GIF o un video, se mueve. */
@Composable
fun Animada(v: Vista) {
    var i by remember(v) { mutableIntStateOf(0) }
    LaunchedEffect(v) {
        if (v.cuadros.size < 2) return@LaunchedEffect
        while (true) {
            delay(v.cuadros[i].second.coerceAtLeast(20).toLong())
            i = (i + 1) % v.cuadros.size
        }
    }
    Image(v.cuadros[i.coerceIn(0, v.cuadros.size - 1)].first, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
}

// --- 04 Su día: a qué hora se despierta y se duerme, y los cumpleaños ---

@Composable
fun ModDia(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val despierta = Editar.despierta(c)
    val duerme = Editar.duerme(c)
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            VisorHora("Despierta", despierta, Modifier.weight(1f)) { d -> vm.cambiar { Editar.moverHora(it, "despierta", d) } }
            VisorHora("Duerme", duerme, Modifier.weight(1f)) { d -> vm.cambiar { Editar.moverHora(it, "duerme", d) } }
        }
        Bandeja { Barra24(despierta, duerme) }
        SeccionDespertador(vm)
        SeccionPresencia(vm)
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Vos")
            CampoVisor(c.obj("dueno")?.texto("nombre").orEmpty(), { v -> vm.cambiar { Editar.ponerDueno(it, v) } }, placeholder = "Tu nombre", estilo = T.campoClaro)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            VisorCumple("Tu cumple", Editar.partes(c.obj("dueno")?.texto("cumple")), Modifier.weight(1f)) { m, d -> vm.cambiar { Editar.ponerCumple(it, false, m, d) } }
            VisorCumple("Su cumple", Editar.partes(c.texto("cumple")), Modifier.weight(1f)) { m, d -> vm.cambiar { Editar.ponerCumple(it, true, m, d) } }
        }
    }
}

@Composable
fun VisorHora(nombre: String, hora: String, modifier: Modifier = Modifier, alMover: (Int) -> Unit) {
    Visor(modifier) {
        Grabado(nombre)
        Text(hora, style = T.digitos.copy(fontSize = 34.sp, lineHeight = 36.sp), maxLines = 1)
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Tecla("−", { alMover(-15) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: más temprano")
            Tecla("+", { alMover(15) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: más tarde")
        }
    }
}

/** La barra de 24 h: dónde está despierto y dónde estamos ahora. */
@Composable
fun Barra24(despierta: String, duerme: String) {
    var ahora by remember { mutableStateOf(java.time.LocalTime.now()) }
    LaunchedEffect(Unit) { while (true) { delay(60_000); ahora = java.time.LocalTime.now() } }
    val tramos = Editar.tramos(despierta, duerme)
    val enAhora = (ahora.hour * 60 + ahora.minute) / 1440f
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Canvas(Modifier.fillMaxWidth().height(34.dp).clip(RoundedCornerShape(10.dp)).background(C.hundido)) {
            tramos.forEach { (a, z) ->
                drawRect(Brush.horizontalGradient(listOf(C.ambar, C.led)), Offset(size.width * a, 0f), androidx.compose.ui.geometry.Size(size.width * (z - a), size.height))
            }
            drawLine(C.papel, Offset(size.width * enAhora, 0f), Offset(size.width * enAhora, size.height), 2.dp.toPx())
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            listOf(0, 6, 12, 18, 24).forEach { Text("%02d".format(it), style = T.grabado, color = C.sobrePlaca) }
        }
    }
}

@Composable
fun VisorCumple(nombre: String, actual: Pair<Int, Int>, modifier: Modifier = Modifier, alCambiar: (Int, Int) -> Unit) {
    val (mes, dia) = actual
    Visor(modifier) {
        Grabado(nombre)
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Selector(if (dia > 0) "$dia" else "—", listOf("0" to "—") + (1..31).map { "$it" to "$it" }, { d -> alCambiar(if (mes > 0) mes else 1, d.toInt()) })
            Selector(if (mes > 0) Editar.MESES[mes - 1] else "—", listOf("0" to "—") + Editar.MESES.mapIndexed { i, m -> "${i + 1}" to m }, { m -> alCambiar(m.toInt(), if (dia > 0) dia else 1) })
        }
    }
}

// --- 05 Recordatorios: a la hora, lo muestra en su pantalla hasta que lo tocás ---

@Composable
fun ModRecordatorios(vm: RobotViewModel, modelo: Modelo) {
    val c by vm.config.collectAsState()
    val lista = Recordatorios.leer(c)
    val conSonido = Capacidad.PARLANTE in modelo.capacidades
    fun cambiar(i: Int, f: (Recordatorio) -> Recordatorio) = vm.cambiar { cc ->
        val l = Recordatorios.leer(cc).toMutableList()
        if (i in l.indices) l[i] = f(l[i])
        Recordatorios.escribir(cc, l)
    }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado("${lista.size} de ${Recordatorios.MAXIMO}", color = C.sobrePlaca)
            Tecla("+ Nuevo", { vm.cambiar { Recordatorios.escribir(it, Recordatorios.nuevo(Recordatorios.leer(it), java.time.LocalTime.now().hour)) } },
                acabado = Acabado.NEGRA, habilitada = lista.size < Recordatorios.MAXIMO)
        }
        if (lista.isEmpty()) Bandeja {
            Text("--:--", style = T.digitos.copy(color = C.tinta.copy(alpha = 0.5f), shadow = null))
            Text("08:30 · Tomar la pastilla", style = T.texto, color = Color(0xCC1A1714))
        }
        Recordatorios.ordenados(lista).forEach { (i, r) -> TarjetaRecordatorio(r, conSonido, { f -> cambiar(i, f) }, {
            vm.cambiar { Recordatorios.escribir(it, Recordatorios.leer(it).filterIndexed { k, _ -> k != i }) }
        }) }
    }
}

@Composable
private fun TarjetaRecordatorio(r: Recordatorio, conSonido: Boolean, cambiar: ((Recordatorio) -> Recordatorio) -> Unit, borrar: () -> Unit) {
    val contexto = LocalContext.current
    var bytes by remember(r.texto) { mutableIntStateOf(largoBytes(r.texto)) }
    Bandeja(Modifier.then(if (r.activo == false) Modifier.drawBehind { drawRect(Color(0x33000000)) } else Modifier)) {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
            Visor(Modifier.clickable {
                TimePickerDialog(contexto, { _, h, m -> cambiar { it.copy(hora = "%02d:%02d".format(h, m)) } }, Editar.aMin(r.hora) / 60, Editar.aMin(r.hora) % 60, true).show()
            }) {
                Grabado("A las")
                Text(r.hora, style = T.digitosChicos)
            }
            Visor(Modifier.weight(1f)) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Grabado("Te dice")
                    Grabado("$bytes/${Recordatorios.MAX_TEXTO}", color = if (bytes >= Recordatorios.MAX_TEXTO) C.brasa else C.grisOscuro)
                }
                CampoVisor(r.texto, { t -> Recordatorios.conTexto(r, t)?.let { n -> cambiar { it.copy(texto = n.texto) } } },
                    placeholder = "Tomar la pastilla", estilo = T.campoClaro, maxBytes = Recordatorios.MAX_TEXTO, alEscribir = { bytes = largoBytes(it) })
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            Recordatorios.DIAS.forEach { (l, n) ->
                val si = Recordatorios.tiene(r, l)
                Box(
                    Modifier.weight(1f).height(36.dp).clip(RoundedCornerShape(9.dp)).background(if (si) C.tinta else Color(0x241A1714))
                        .clickable(role = Role.Checkbox, onClickLabel = n) { cambiar { Recordatorios.alternarDia(it, l) } },
                    contentAlignment = Alignment.Center,
                ) { Text(l, style = T.teclaChica, color = if (si) C.papel else C.tinta) }
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Visor(Modifier.weight(1f)) {
                Selector(Recordatorios.CARAS.firstOrNull { it.first == (r.expresion ?: "sorpresa") }?.second ?: "Sorpresa", Recordatorios.CARAS,
                    { k -> cambiar { it.copy(expresion = if (k == "sorpresa") null else k) } }, estilo = T.dato.copy(color = C.papel))
            }
            Tecla("Borrar", borrar, acabado = Acabado.NEGRA, chica = true, descripcion = "Borrar el de las ${r.hora}")
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Llave(if (r.activo == false) "Apagado" else "Activo", r.activo != false, { cambiar { it.copy(activo = if (it.activo == false) null else false) } })
            if (conSonido) Llave("Suena", r.sonido != false, { cambiar { it.copy(sonido = if (it.sonido == false) null else false) } })
        }
    }
}

// --- 06 Modos: compañía, reloj o pomodoro (clima y avisos, pronto) ---

@Composable
fun ModModos(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val modo = Editar.modo(c)
    val modos = listOf(
        Triple("mascota", "Compañía", true), Triple("reloj", "Reloj", true), Triple("pomodoro", "Pomodoro", true),
        Triple("clima", "Clima", true), Triple("notificaciones", "Avisos", false),
    )
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        modos.chunked(2).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                fila.forEach { (k, n, listo) ->
                    val elegido = modo == k
                    Tecla(null, { vm.cambiar { Editar.ponerModo(it, k) } }, Modifier.weight(1f), acabado = if (elegido) Acabado.NEGRA else Acabado.BLANCA,
                        habilitada = listo, apretada = elegido, alto = 64.dp) {
                        Led(if (elegido) C.led else null)
                        Text(n.uppercase(), style = T.tecla, color = if (elegido) C.papel else C.tinta, maxLines = 1)
                        if (!listo) Text("PRONTO", style = T.grabado.copy(fontSize = 9.sp), color = C.gris)
                    }
                }
                if (fila.size == 1) Spacer(Modifier.weight(1f))
            }
        }
        if (modo == "pomodoro") Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            VisorMinutos("Foco", Editar.trabajo(c), Modifier.weight(1f)) { d -> vm.cambiar { Editar.moverPomodoro(it, "trabajo", d * 5) } }
            VisorMinutos("Descanso", Editar.descanso(c), Modifier.weight(1f)) { d -> vm.cambiar { Editar.moverPomodoro(it, "descanso", d) } }
        }
        if (modo == "clima") SeccionUbicacion(vm)
        if (modo == "pomodoro") Text("Un toque arranca o pausa.", style = T.texto, color = Color(0xCC1A1714))
    }
}

@Composable
private fun VisorMinutos(nombre: String, minutos: Int, modifier: Modifier, alMover: (Int) -> Unit) {
    Visor(modifier) {
        Grabado(nombre)
        Row(verticalAlignment = Alignment.Bottom) {
            Text("%02d".format(minutos), style = T.digitos.copy(fontSize = 34.sp))
            Text(" MIN", style = T.grabado, color = C.brasa, modifier = Modifier.padding(bottom = 6.dp))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Tecla("−", { alMover(-1) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: menos")
            Tecla("+", { alMover(1) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: más")
        }
    }
}

// --- 07 Reflejos: un patchbay; un cable une lo que le pasa con cómo reacciona ---

@Composable
fun ModReflejos(vm: RobotViewModel, modelo: Modelo) {
    val c by vm.config.collectAsState()
    val cables = Reflejos.cables(c)
    val sinMovimiento = Capacidad.MOVIMIENTO !in modelo.capacidades
    var elegida by remember { mutableStateOf<String?>(null) }
    val fila = 48.dp
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Grabado(elegida?.let { "${Reflejos.NOMBRES_ENTRADA[it]} → ¿qué hace?" } ?: "Uní un punto con otro", color = C.sobrePlaca)
        Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp)).background(C.hundido).padding(vertical = 8.dp)) {
            Column(Modifier.weight(1f)) {
                Reflejos.ENTRADAS.forEach { e ->
                    val apagada = sinMovimiento && e in Reflejos.DE_MOVIMIENTO
                    Jack(Reflejos.NOMBRES_ENTRADA[e] ?: e, izquierda = true, elegida = elegida == e, conectada = cables[e] != null, habilitado = !apagada, alto = fila) {
                        elegida = if (elegida == e) null else e
                    }
                }
            }
            Canvas(Modifier.width(44.dp).height(fila * Reflejos.ENTRADAS.size)) {
                val f = fila.toPx()
                Reflejos.ENTRADAS.forEachIndexed { i, e ->
                    val s = cables[e] ?: return@forEachIndexed
                    val j = Reflejos.SALIDAS.indexOfFirst { it.first == s }
                    val y1 = i * f + f / 2
                    val y2 = j * f + f / 2
                    val p = Path().apply { moveTo(0f, y1); cubicTo(size.width * 0.45f, y1 + 26f, size.width * 0.55f, y2 + 26f, size.width, y2) }
                    drawPath(p, if (elegida == e) C.brasa else C.led, style = Stroke(4.dp.toPx(), cap = androidx.compose.ui.graphics.StrokeCap.Round))
                }
            }
            Column(Modifier.weight(1f)) {
                Reflejos.SALIDAS.forEach { (s, t) ->
                    Jack(t, izquierda = false, elegida = elegida != null && cables[elegida] == s, conectada = s in cables.values, habilitado = elegida != null, alto = fila) {
                        elegida?.let { en -> vm.cambiar { Reflejos.tocar(it, en, s) } }
                        elegida = null
                    }
                }
            }
        }
        if (sinMovimiento) Text("No siente el movimiento.", style = T.texto, color = Color(0xCC1A1714))
        SeccionAvisosCelu(vm)
    }
}

@Composable
private fun Jack(nombre: String, izquierda: Boolean, elegida: Boolean, conectada: Boolean, habilitado: Boolean, alto: androidx.compose.ui.unit.Dp, alTocar: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().height(alto).clickable(enabled = habilitado, role = Role.Button, onClick = alTocar).padding(horizontal = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = if (izquierda) Arrangement.End else Arrangement.Start,
    ) {
        val toma: @Composable RowScope.() -> Unit = {
            Box(
                Modifier.size(20.dp).clip(CircleShape).background(if (conectada || elegida) C.led else C.chapa)
                    .border(3.dp, if (elegida) C.papel else Color(0xFF3A3632), CircleShape),
            )
        }
        val texto: @Composable RowScope.() -> Unit = {
            Text(nombre.uppercase(), style = T.teclaChica.copy(fontSize = 10.sp), color = if (!habilitado && izquierda) C.grisOscuro else if (elegida) C.brasa else C.crema,
                maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false).padding(horizontal = 8.dp))
        }
        if (izquierda) { texto(); toma() } else { toma(); texto() }
    }
}

// --- 09 Voz: para los modelos con micrófono y parlante ---

@Composable
fun ModVoz(vm: RobotViewModel, modelo: Modelo) {
    val c by vm.config.collectAsState()
    val r by vm.robot.collectAsState()
    val habla = Capacidad.PARLANTE in modelo.capacidades
    val escucha = Capacidad.MICROFONO in modelo.capacidades
    if (!habla && !escucha) {
        Bandeja {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Canvas(Modifier.size(40.dp)) {
                    val w = size.width
                    drawRoundRect(C.tinta, Offset(w * 0.2f, w * 0.45f), androidx.compose.ui.geometry.Size(w * 0.6f, w * 0.45f), androidx.compose.ui.geometry.CornerRadius(4.dp.toPx()))
                    drawArc(C.tinta, 180f, 180f, false, Offset(w * 0.32f, w * 0.12f), androidx.compose.ui.geometry.Size(w * 0.36f, w * 0.5f), style = Stroke(3.dp.toPx()))
                }
                Column {
                    Text("SIN VOZ", style = T.nombre.copy(fontSize = 22.sp), color = C.tinta)
                    Text("Llega con los que hablan.", style = T.texto, color = Color(0xCC1A1714))
                }
            }
        }
        // Igual se puede conectar la voz: le hablás desde acá, con el micrófono del celu.
        SeccionLeerVoz(vm)
        return
    }
    val v = Editar.voz(c)
    val nombre = r?.apodo?.ifEmpty { null } ?: c.texto("nombre") ?: "tu robot"
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Bandeja {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                if (habla) Perilla(v.numero("volumen") ?: 0.7, "Volumen", { x -> vm.cambiar { Editar.ponerVoz(it) { y -> y.con("volumen", x) } } })
                if (escucha) Perilla(v.numero("microfono") ?: 0.6, "Oído", { x -> vm.cambiar { Editar.ponerVoz(it) { y -> y.con("microfono", x) } } })
            }
            if (habla) {
                Grabado("Su voz", color = C.sobrePlaca)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf("grave" to "Grave", "media" to "Media", "aguda" to "Aguda", "robot" to "Robot").forEach { (k, t) ->
                        Tecla(t, { vm.cambiar { Editar.ponerVoz(it) { y -> y.con("tipo", k) } } }, Modifier.weight(1f), chica = true, apretada = (v.texto("tipo") ?: "media") == k)
                    }
                }
            }
        }
        if (escucha) {
            Visor(Modifier.fillMaxWidth()) {
                Grabado("Responde a")
                CampoVisor(v.texto("palabra").orEmpty(), { t ->
                    val x = cortarBytes(t.trim(), Editar.MAX_PALABRA)
                    vm.cambiar { Editar.ponerVoz(it) { y -> if (x.isEmpty()) y.sin("palabra") else y.con("palabra", x) } }
                }, placeholder = nombre, maxBytes = Editar.MAX_PALABRA)
            }
            Llave("Charlar · después de su nombre", v.si("conversar") == true, { vm.cambiar { Editar.ponerVoz(it) { y -> y.con("conversar", !(y.si("conversar") ?: false)) } } })
            Text("“${v.texto("palabra") ?: nombre}, ¿qué hora es?”", style = T.texto, color = Color(0xCC1A1714))
        }
        SeccionLeerVoz(vm)
        if (habla) SeccionPaqueteVoz(vm)
    }
}

// --- 10 Ajustes: su nombre, el brillo y lo de fondo ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModAjustes(vm: RobotViewModel, modelo: Modelo, alSalir: () -> Unit, alLab: () -> Unit) {
    val c by vm.config.collectAsState()
    val r by vm.robot.collectAsState()
    val version by vm.versionConfig.collectAsState()
    var seguro by remember { mutableStateOf(false) }
    val alcance = rememberCoroutineScope()
    val e = r?.estado
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Se llama")
            CampoVisor(r?.apodo?.ifEmpty { null } ?: c.texto("nombre").orEmpty(), { vm.ponerNombre(it) }, placeholder = "Ponele nombre")
        }
        Bandeja {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                Perilla(Editar.brillo(c), "Brillo", { v -> vm.cambiar { Editar.ponerBrillo(it, v) } }, min = 0.2, max = 1.0, mostrar = porcentaje)
            }
        }
        val datos = listOf(
            "Modelo" to "${modelo.nombre} ${numeroDe(vm.idRobot)}",
            "Firmware" to (e?.version?.ifEmpty { null } ?: "—"),
            "Config" to if (vm.via == Via.NUBE) "v$version · ${r?.sincroConfig?.ifEmpty { "—" } ?: "—"}" else "—",
            "Batería" to (e?.bateria?.let { "$it%" } ?: "—"),
            "WiFi" to if (e?.wifi == true) "Sí" else "No",
            "Serie" to vm.idRobot.uppercase(),
        )
        datos.chunked(2).forEach { par ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                par.forEach { (k, v) ->
                    Column(Modifier.weight(1f).clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(12.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        Grabado(k)
                        Text(v, style = T.dato.copy(color = C.papel), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
        }
        SeccionUbicacion(vm)
        Tecla("Laboratorio", alLab, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
        if (vm.via == Via.CERCA) DeCerca(vm)
        if (vm.via != Via.DEMO) Bandeja {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
                Text(if (vm.via == Via.NUBE) "SACARLO" else "DESVINCULAR", style = T.tecla, color = C.tinta)
                Tecla(if (seguro) "¿Seguro?" else "Sacar", {
                    if (!seguro) { seguro = true; alcance.launch { delay(5000); seguro = false } } else vm.quitar(alSalir)
                }, acabado = if (seguro) Acabado.NARANJA else Acabado.NEGRA, chica = true)
            }
        }
    }
}

/** Solo de cerca (por BLE): pasarle el WiFi y sumarlo a la cuenta. */
@Composable
private fun DeCerca(vm: RobotViewModel) {
    val estadoFlujo = vm.estadoCerca ?: return
    val e by estadoFlujo.collectAsState()
    val mensaje by vm.mensajeCerca.collectAsState()
    val ocupado by vm.ocupadoCerca.collectAsState()
    var ssid by remember { mutableStateOf("") }
    var clave by remember { mutableStateOf("") }
    Bandeja {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Led(if (e?.wifi == true) C.verde else if (e?.redConfigurada == true) C.ambar else null)
            Grabado("WiFi", color = C.sobrePlaca)
        }
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Red")
            CampoVisor(ssid, { ssid = it }, placeholder = "Nombre", estilo = T.campoClaro, maxBytes = 32, alEscribir = { ssid = it })
        }
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Clave")
            CampoVisor(clave, { clave = it }, placeholder = "••••••", estilo = T.campoClaro, maxBytes = 64,
                teclado = androidx.compose.ui.text.input.KeyboardType.Password, alEscribir = { clave = it })
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Tecla("Pasarle", { vm.configurarRed(ssid, clave) }, Modifier.weight(1f), acabado = Acabado.NEGRA, chica = true, habilitada = ssid.isNotBlank() && !ocupado)
            Tecla("A mi cuenta", { vm.sumarACuenta() }, Modifier.weight(1f), acabado = Acabado.NARANJA, chica = true, habilitada = !ocupado && e?.redConfigurada == true)
        }
        mensaje?.let { AvisoLinea(it) }
    }
}


/** Una opción de "Su cara": una muestra en una ventanita negra y su nombre. */
@Composable
private fun OpcionCara(nombre: String, elegida: Boolean, alTocar: () -> Unit, muestra: @Composable () -> Unit) {
    Column(
        Modifier.width(92.dp).clip(RoundedCornerShape(14.dp)).background(if (elegida) C.tinta else Color(0x241A1714))
            .clickable(role = Role.RadioButton, onClick = alTocar).padding(8.dp),
        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Box(Modifier.fillMaxWidth().height(44.dp).clip(RoundedCornerShape(10.dp)).background(Color.Black), contentAlignment = Alignment.Center) { muestra() }
        Text(nombre.uppercase(), style = T.grabado.copy(fontSize = 9.sp), color = if (elegida) C.papel else C.tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}
