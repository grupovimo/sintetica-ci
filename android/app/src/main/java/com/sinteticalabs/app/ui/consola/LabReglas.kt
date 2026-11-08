package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyHorizontalGrid
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.ESTADO_VIDA
import com.sinteticalabs.app.consola.GLIFO
import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.NOMBRES_ESTADO
import com.sinteticalabs.app.consola.NOMBRES_EVENTO
import com.sinteticalabs.app.consola.PARTICULAS
import com.sinteticalabs.app.consola.Regla
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.EVENTOS
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** El ícono de cada grupo de eventos. */
fun iconoEvento(e: String) = when (e) {
    "tacto", "doble-toque", "caricia", "muchos-toques", "despertado" -> "☝"
    "levantado", "sacudida", "sacudida-suave", "sacudida-fuerte", "boca-abajo", "inclinado", "caida" -> "↻"
    "voz", "nombre", "ruido", "musica", "silencio" -> "♪"
    "volvio", "hito", "bicho-cerca", "cambio-de-cara", "vinculado", "app-abierta" -> "♥"
    else -> "▮"
}

private val ACCIONES = listOf("reflejo" to "Cara ya", "expresion" to "Cara 3 s", "comportamiento" to "Hace", "particula" to "Le sale", "estado" to "Siente", "led" to "Luz")
private val LEDS = listOf("#FF6A33", "#FFB347", "#F3EEE8", "#4ADE80", "#3DE0FF", "#A78BFA", "#FF5FA2")
private fun colorDe(hex: String) = runCatching { Color(android.graphics.Color.parseColor(hex)) }.getOrDefault(C.led)

/**
 * Reglas: cuando le pasa algo, qué hace. Se elige el evento en la botonera, se
 * ven las respuestas (las tuyas y, atenuadas, las de fábrica) y se simula el
 * evento en la vista previa (web: routes/robots/[id]/reglas).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LabReglas(vm: RobotViewModel, modelo: Modelo) {
    val config by vm.config.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var reglas by remember { mutableStateOf(Regla.leer(config)) }
    var tomadas by remember { mutableStateOf(reglas) }
    var evento by remember { mutableStateOf("tacto") }
    var eligiendo by remember { mutableStateOf<Pair<Int, String>?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf("") }
    var guardando by remember { mutableStateOf(false) }
    val dibujar = Lab.paraDibujar(config)
    val sucio = reglas != tomadas
    LaunchedEffect(config) { if (!sucio) { reglas = Regla.leer(config); tomadas = reglas } }

    fun sumar(i: Int, tipo: String, arg: String) {
        val r = reglas.getOrNull(i) ?: return
        if (r.entonces.size >= 6) return
        reglas = reglas.toMutableList().also { it[i] = r.copy(entonces = r.entonces + Lab.unir(tipo, arg)) }
        eligiendo = null
    }
    fun quitar(i: Int) { reglas = reglas.filterIndexed { k, _ -> k != i }; eligiendo = null }
    fun nueva(desde: Regla?) { if (reglas.size < 24) reglas = reglas + (desde ?: Regla(evento, null, listOf("reflejo:contento"))) }

    // La botonera de eventos: dos filas que se deslizan.
    LazyHorizontalGrid(GridCells.Fixed(2), Modifier.fillMaxWidth().height(140.dp), horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        items(EVENTOS.size) { k ->
            val e = EVENTOS[k]
            val sel = evento == e
            val n = reglas.count { it.si == e }
            Column(
                Modifier.width(108.dp).clip(RoundedCornerShape(12.dp)).background(C.chapa)
                    .then(if (sel) Modifier.border(2.dp, C.brasa, RoundedCornerShape(12.dp)) else Modifier)
                    .clickable { evento = e; eligiendo = null }.padding(9.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Text(iconoEvento(e), style = T.tecla, color = if (sel) C.brasa else C.gris)
                    if (n > 0) Box(Modifier.size(16.dp).clip(RoundedCornerShape(50)).background(C.led), contentAlignment = Alignment.Center) {
                        Text("$n", style = T.modulo.copy(fontSize = 9.sp), color = C.tinta)
                    }
                }
                Text(NOMBRES_EVENTO[e] ?: e, style = T.dato.copy(fontSize = 11.sp), color = if (sel) C.papel else C.grisClaro, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
    }

    Box {
        MarcoPantalla(modelo.pantalla) { if (llave > 0) PantallaViva(llave, "Vista previa") }
        Tecla("${iconoEvento(evento)} ${NOMBRES_EVENTO[evento] ?: evento}", {
            val c = Regla.escribir(config, reglas)
            val e = Nucleo.validarConfig(c.toString())
            if (e != null) error = e
            else {
                error = null
                vm.pausarEspejo(8.0)
                vm.aplicarEnVistaPrevia(c)
                val i = EVENTOS.indexOf(evento)
                if (i >= 0) Nucleo.evento(i)
            }
        }, Modifier.align(Alignment.BottomEnd).padding(10.dp), acabado = Acabado.NARANJA, chica = true)
    }

    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Text((NOMBRES_EVENTO[evento] ?: evento).uppercase(), style = T.nombre.copy(fontSize = 22.sp), maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
        Tecla("+", { nueva(null) }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = reglas.size < 24, descripcion = "Respuesta nueva")
        Tecla(if (guardando) "…" else aviso.ifEmpty { "Guardar" }, {
            alcance.launch {
                guardando = true
                error = vm.guardarAhora(Regla.escribir(config, reglas))
                guardando = false
                if (error == null) { tomadas = reglas; aviso = "Guardado"; delay(2500); aviso = "" }
            }
        }, acabado = Acabado.NARANJA, chica = true, habilitada = sucio && !guardando)
    }
    error?.let { AvisoLinea(it, error = true) }

    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        val propias = reglas.indices.filter { reglas[it].si == evento }
        val fabrica = VidaBase.reglas.filter { it.si == evento }
        propias.forEach { i ->
            val r = reglas[i]
            Bandeja {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    r.entonces.forEachIndexed { k, a ->
                        Row(
                            Modifier.clip(RoundedCornerShape(10.dp)).background(C.hundido).clickable {
                                if (r.entonces.size <= 1) quitar(i)
                                else reglas = reglas.toMutableList().also { it[i] = r.copy(entonces = r.entonces.filterIndexed { j, _ -> j != k }) }
                            }.padding(6.dp),
                            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp),
                        ) {
                            Ficha(a, dibujar, modelo)
                            Text("✕", style = T.dato.copy(fontSize = 10.sp), color = C.gris)
                        }
                    }
                }
                if (r.entonces.size < 6) FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    ACCIONES.forEach { (t, n) ->
                        val abierta = eligiendo?.first == i && eligiendo?.second == t
                        Tecla("+ $n", { eligiendo = if (abierta) null else i to t }, chica = true, apretada = abierta, alto = 36.dp)
                    }
                }
                eligiendo?.takeIf { it.first == i }?.let { (_, t) -> SelectorAccion(t, dibujar, modelo, vm) { tipo, arg -> sumar(i, tipo, arg) } }
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Visor(Modifier.weight(1f)) {
                        Grabado("Solo si")
                        CampoVisor(r.y.orEmpty(), { t ->
                            val x = t.trim()
                            reglas = reglas.toMutableList().also { if (i < it.size) it[i] = it[i].copy(y = x.ifEmpty { null }) }
                        }, placeholder = "afecto>0.5", estilo = T.dato.copy(color = C.papel), maxBytes = 120)
                    }
                    Tecla("✕", { quitar(i) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Borrar")
                }
            }
        }
        fabrica.forEach { r ->
            Column(
                Modifier.fillMaxWidth().drawBehind {
                    drawRoundRect(Color(0x401A1714), cornerRadius = CornerRadius(20.dp.toPx()),
                        style = Stroke(1.5.dp.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(18f, 12f))))
                }.padding(14.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Grabado("De fábrica" + (r.y?.let { " · si $it" } ?: ""), color = C.sobrePlaca)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    r.entonces.forEach { a ->
                        Box(Modifier.clip(RoundedCornerShape(10.dp)).background(C.hundido.copy(alpha = 0.6f)).padding(6.dp)) { Ficha(a, dibujar, modelo) }
                    }
                }
                Tecla("Copiar y cambiar", { nueva(r) }, chica = true, habilitada = reglas.size < 24)
            }
        }
        if (propias.isEmpty() && fabrica.isEmpty()) Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado("Sin respuesta", color = C.sobrePlaca)
            Tecla("+ Respuesta", { nueva(null) }, acabado = Acabado.NEGRA, chica = true)
        }
    }
}

/** Una acción de una regla, a la vista: la cara, lo que hace, lo que siente, la luz. */
@Composable
private fun Ficha(a: String, dibujar: String, modelo: Modelo) {
    val (tipo, arg) = Lab.partir(a)
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        val estilo = T.dato.copy(fontSize = 11.sp, color = C.papel)
        when (tipo) {
            "reflejo", "expresion" -> {
                MiniCara(arg, dibujar, modelo.pantalla, 40.dp)
                Text(if (tipo == "expresion") "$arg · 3 s" else arg, style = estilo, maxLines = 1)
            }
            "comportamiento" -> Text("▶ ${arg.replace('-', ' ')}", style = estilo, maxLines = 1)
            "particula" -> Text("${GLIFO[arg] ?: ""} $arg", style = estilo, maxLines = 1)
            "estado" -> {
                val v = arg.takeWhile { it.isLetter() }
                Text("${if ('+' in arg) "▲" else "▼"} ${NOMBRES_ESTADO[v] ?: v}", style = estilo, maxLines = 1)
            }
            "led" -> {
                Canvas(Modifier.size(14.dp)) { drawCircle(colorDe(arg.uppercase())) }
                Text("Luz", style = estilo)
            }
            else -> Text("♫ $arg", style = estilo, maxLines = 1)
        }
    }
}

/** Lo que se suma a una regla, según el tipo elegido. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SelectorAccion(tipo: String, dibujar: String, modelo: Modelo, vm: RobotViewModel, alSumar: (String, String) -> Unit) {
    val config by vm.config.collectAsState()
    when (tipo) {
        "reflejo", "expresion" -> FilaDeCaras(VidaBase.nombresCaras, null, dibujar, modelo.pantalla) { c -> if (c != null) alSumar(tipo, c) }
        "comportamiento" -> {
            val base = VidaBase.comportamientos.keys.sorted()
            val propios = config.obj("comportamientos").orEmpty().keys.filter { it !in base }.sorted()
            Chips(base + propios, null, { it.replace('-', ' ') }) { alSumar("comportamiento", it) }
        }
        "particula" -> Chips(PARTICULAS, null, { GLIFO[it] ?: it }) { alSumar("particula", it) }
        "estado" -> FlowRow(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            ESTADO_VIDA.forEach { v ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text((NOMBRES_ESTADO[v] ?: v).uppercase(), style = T.modulo.copy(fontSize = 10.sp), color = C.tinta)
                    Tecla("▲", { alSumar("estado", "$v+0.2") }, chica = true, alto = 34.dp, descripcion = "Más")
                    Tecla("▼", { alSumar("estado", "$v-0.2") }, acabado = Acabado.NEGRA, chica = true, alto = 34.dp, descripcion = "Menos")
                }
            }
        }
        else -> Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            LEDS.forEach { c ->
                Canvas(Modifier.size(30.dp).clickable { alSumar("led", c.lowercase()) }) { drawCircle(colorDe(c)) }
            }
        }
    }
}
