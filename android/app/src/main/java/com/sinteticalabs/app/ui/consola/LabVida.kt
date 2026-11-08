package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
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
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.ESTADO_VIDA
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.Memoria
import com.sinteticalabs.app.consola.NOMBRES_ESTADO
import com.sinteticalabs.app.consola.NOMBRES_EVENTO
import com.sinteticalabs.app.consola.NOMBRES_HITO
import com.sinteticalabs.app.consola.VidaEnVivo
import com.sinteticalabs.app.datos.RobotNube
import com.sinteticalabs.app.datos.memoria
import com.sinteticalabs.app.datos.mudar
import com.sinteticalabs.app.datos.robots
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Vida: lo que siente ahora (vúmetros de los 7 estados, pedidos cada 2 s
 * mientras está abierta), su diario y la mudanza (web: routes/robots/[id]/vida).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LabVida(vm: RobotViewModel) {
    val vida by vm.vida.collectAsState()
    val vidaEn by vm.vidaEn.collectAsState()
    val robot by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    val api = vm.apiNube()
    val nube = vm.via == Via.NUBE && api != null
    var memoria by remember { mutableStateOf<Memoria?>(null) }
    var pendiente by remember { mutableStateOf(false) }
    var otros by remember { mutableStateOf<List<RobotNube>>(emptyList()) }
    var desde by remember { mutableStateOf("") }
    var aviso by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var superposicion by remember { mutableStateOf(false) }
    val conectado = robot?.conectado == true

    LaunchedEffect(Unit) {
        var vuelta = 0
        while (true) {
            vm.pedirVida()
            if (nube && vuelta % 30 == 0) runCatching { val (m, p) = api!!.memoria(vm.idRobot); memoria = m; pendiente = p }.onFailure { error = it.message }
            vuelta++
            delay(2000)
        }
    }
    LaunchedEffect(nube) { if (nube) runCatching { otros = api!!.robots().filter { it.id != vm.idRobot } } }

    // Ahora.
    Chapa(Modifier.fillMaxWidth()) {
        val vieja = System.currentTimeMillis() - vidaEn > 8000
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Led(if (vida != null && !vieja) C.verde else null)
            Grabado("Ahora")
            Spacer(Modifier.weight(1f))
            Llave("En su pantalla", superposicion, {
                superposicion = !superposicion
                val s = superposicion
                alcance.launch { vm.pedirVida(s) }
            }, habilitada = conectado)
        }
        Text(haciendo(conectado, vida).uppercase(), style = T.titular.copy(fontSize = 32.sp, lineHeight = 32.sp),
            color = if (vida == null) C.grisOscuro else C.brasa, maxLines = 2, overflow = TextOverflow.Ellipsis)
        vida?.let { v ->
            Row(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp)).background(C.hundido).padding(horizontal = 10.dp, vertical = 14.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.Bottom,
            ) {
                ESTADO_VIDA.forEach { e -> Vumetro(NOMBRES_ESTADO[e] ?: e, v.estado[e] ?: 0.0, e == "estres", Modifier.weight(1f)) }
            }
        }
    }

    if (!nube) return

    // El diario.
    PlacaNaranja(Modifier.fillMaxWidth()) {
        Grabado("Diario", color = C.sobrePlaca)
        val m = memoria
        if (m == null) Text("Todavía sin recuerdos.", style = T.texto, color = Color(0xCC1A1714))
        else {
            listOf("Días juntos" to m.dias, "Racha" to m.racha, "Caricias" to m.caricias, "Toques" to m.toques).chunked(2).forEach { par ->
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    par.forEach { (t, n) -> Visor(Modifier.weight(1f)) { Grabado(t); Text("$n", style = T.digitos.copy(fontSize = 32.sp)) } }
                }
            }
            if (m.hitos.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                m.hitos.forEach { h -> Etiqueta("★ ${NOMBRES_HITO[h] ?: h}", C.ambar) }
            }
            Bandeja {
                Grabado(m.horaPreferida?.let { "Te espera a las $it:00" } ?: "Cuándo aparecés", color = C.sobrePlaca)
                BarrasHoras(m.horas, m.horaPreferida)
            }
            if (m.mejoresGustos.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Grabado("Le gusta", color = C.sobrePlaca)
                m.mejoresGustos.forEach { g -> Etiqueta("♥ ${NOMBRES_EVENTO[g] ?: g}") }
            }
            val cambiados = Editar.RASGOS.filter { (m.deriva[it] ?: 0.0) != 0.0 }
            if (cambiados.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Grabado("Lo cambiaste", color = C.sobrePlaca)
                cambiados.forEach { r -> Etiqueta("${if ((m.deriva[r] ?: 0.0) > 0) "▲" else "▼"} ${Editar.NOMBRES_RASGO[r] ?: r}") }
            }
        }
    }

    // La mudanza: traer la memoria de otro robot.
    Chapa(Modifier.fillMaxWidth()) {
        Grabado("Mudanza" + if (pendiente) " · en camino" else "")
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Visor(Modifier.weight(1f)) {
                Selector(
                    if (desde.isEmpty()) (if (otros.isEmpty()) "No hay otro" else "Traer de…") else otros.firstOrNull { it.id == desde }?.nombre?.ifEmpty { desde } ?: desde,
                    otros.map { it.id to it.nombre.ifEmpty { it.id } }, { desde = it }, estilo = T.dato.copy(color = C.papel), habilitado = otros.isNotEmpty(),
                )
            }
            Tecla("Mudar", {
                aviso = null; error = null
                val d = desde
                alcance.launch {
                    runCatching { api!!.mudar(vm.idRobot, d) }.onSuccess { aviso = "Le llega al conectarse."; pendiente = true }.onFailure { error = it.message }
                }
            }, acabado = Acabado.NARANJA, chica = true, habilitada = desde.isNotEmpty())
        }
        aviso?.let { AvisoLinea(it) }
        error?.let { AvisoLinea(it, error = true) }
    }
}

private fun haciendo(conectado: Boolean, v: VidaEnVivo?): String = when {
    !conectado -> "Desconectado"
    v == null -> "Escuchando…"
    v.dormido -> if (v.sonando) "Soñando" else "Durmiendo"
    else -> v.comportamiento.ifEmpty { "Tranqui" }.replace('-', ' ')
}

/** Un vúmetro de 12 luces. */
@Composable
private fun Vumetro(nombre: String, valor: Double, estres: Boolean, modifier: Modifier) {
    Column(modifier.semantics { contentDescription = "$nombre ${Math.round(valor * 100)}" }, horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Canvas(Modifier.width(24.dp).height(131.dp)) {
            val h = 8.dp.toPx()
            val gap = 3.dp.toPx()
            for (j in 0 until 12) {
                val k = 11 - j
                val on = valor * 12 > k
                val color = if (!on) Color(0xFF2A2622) else if (estres) Color(0xFFFF3B3B) else if (k >= 9) C.ambar else C.led
                drawRoundRect(color, Offset(0f, j * (h + gap)), Size(size.width, h), CornerRadius(2.dp.toPx()))
            }
        }
        Text(nombre.uppercase(), style = T.modulo.copy(fontSize = 8.sp), color = C.gris, maxLines = 1)
    }
}

/** Las 24 barras de "cuándo aparecés". */
@Composable
private fun BarrasHoras(horas: List<Int>, pico: Int?) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Canvas(Modifier.fillMaxWidth().height(80.dp)) {
            val ancho = size.width / 24
            horas.forEachIndexed { h, v ->
                val alto = (v * 8).coerceIn(4, 100) / 100f * size.height
                drawRoundRect(if (h == pico) C.tinta else C.tinta.copy(alpha = 0.55f), Offset(h * ancho + 1.5f, size.height - alto), Size(ancho - 3f, alto), CornerRadius(2.dp.toPx()))
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            listOf(0, 6, 12, 18, 24).forEach { Text("$it", style = T.grabado, color = C.sobrePlaca) }
        }
    }
}

@Composable
private fun Etiqueta(texto: String, color: Color = C.papel) {
    Box(Modifier.clip(RoundedCornerShape(10.dp)).background(C.tinta).padding(horizontal = 12.dp, vertical = 8.dp)) {
        Text(texto, style = T.teclaChica, color = color)
    }
}
