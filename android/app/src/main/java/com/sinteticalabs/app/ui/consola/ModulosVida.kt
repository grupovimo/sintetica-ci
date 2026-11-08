package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.layout.size
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
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Acompanar
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.cortarBytes
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.datos.Clima
import com.sinteticalabs.app.datos.Lugar
import com.sinteticalabs.app.datos.buscarCiudad
import com.sinteticalabs.app.datos.clima
import com.sinteticalabs.app.datos.memoriaCruda
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val SOBRE = Color(0xCC1A1714)

/** "Decile algo": lo muestra en un globo con su cara (60 bytes). */
@Composable
fun DecileAlgo(vm: RobotViewModel, conectado: Boolean) {
    val alcance = rememberCoroutineScope()
    var frase by remember { mutableStateOf("") }
    var aviso by remember { mutableStateOf<String?>(null) }
    var diciendo by remember { mutableStateOf(false) }
    Visor(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado("Decile algo")
            Grabado("${largoBytes(frase)}/${Acompanar.MAX_TEXTO}", color = if (largoBytes(frase) >= Acompanar.MAX_TEXTO) C.brasa else C.grisOscuro)
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.weight(1f)) {
                CampoVisor(frase, { frase = it }, placeholder = if (conectado) "¡Hola!" else "Desconectado", estilo = T.campoClaro,
                    maxBytes = Acompanar.MAX_TEXTO, alEscribir = { frase = it })
            }
            Tecla("Decir", {
                val t = frase
                alcance.launch {
                    diciendo = true
                    val e = vm.decir(t)
                    diciendo = false
                    if (e == null) frase = ""
                    aviso = e ?: "Lo dijo"
                    delay(2500)
                    aviso = null
                }
            }, acabado = Acabado.NARANJA, chica = true, habilitada = conectado && !diciendo && frase.isNotBlank())
        }
        aviso?.let { Grabado(it, color = C.brasa) }
    }
}

// --- Habla: cuánto habla solo y sus frases propias por situación ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModHabla(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    var situacion by remember { mutableStateOf("despertar") }
    var nueva by remember { mutableStateOf("") }
    var oido by remember { mutableStateOf<String?>(null) }
    val total = Acompanar.totalFrases(c)
    val frases = Acompanar.frases(c, situacion)
    val conectado = robot?.conectado == true
    val nombre = robot?.apodo?.ifEmpty { null } ?: c.texto("nombre") ?: "tu robot"
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
            Bandeja(Modifier.width(120.dp)) {
                Perilla(Acompanar.frecuencia(c), "Cuánto", { v -> vm.cambiar { Acompanar.ponerFrecuencia(it, v) } }, Modifier.fillMaxWidth(), tamano = 64.dp, mostrar = Acompanar::tono)
            }
            Visor(Modifier.weight(1f)) {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                    Grabado("Cuando")
                    Grabado("$total/${Acompanar.MAX_FRASES}", color = if (total >= Acompanar.MAX_FRASES) C.brasa else C.grisOscuro)
                }
                val nombres = Acompanar.SITUACIONES.toMap() + com.sinteticalabs.app.consola.Compania.NOMBRES_NUEVOS
                val agrupadas = com.sinteticalabs.app.consola.Compania.GRUPOS_HABLA.flatMap { (g, l) -> l.map { it to g } }
                val resto = Acompanar.SITUACIONES.map { it.first }.filter { k -> agrupadas.none { it.first == k } }.map { it to "Más" }
                Selector(nombres[situacion] ?: situacion,
                    (agrupadas + resto).map { (k, g) -> k to ("$g · " + (nombres[k] ?: k) + Acompanar.frases(c, k).size.let { if (it > 0) " · $it" else "" }) }, { situacion = it })
            }
        }
        if (frases.isEmpty()) Text("Usa las suyas", style = T.texto, color = SOBRE)
        frases.forEachIndexed { i, f ->
            Row(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(C.hundido).padding(start = 14.dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text("“$f”", style = T.dato.copy(color = C.papel), modifier = Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                Tecla("▶", {
                    alcance.launch {
                        val e = vm.decir(Acompanar.completar(f, c.obj("dueno")?.texto("nombre").orEmpty(), nombre))
                        oido = e ?: "Lo dijo"; delay(2000); oido = null
                    }
                }, chica = true, cuadrada = true, habilitada = conectado, descripcion = "Que lo diga")
                Tecla("✕", { vm.cambiar { Acompanar.ponerFrases(it, situacion, Acompanar.frases(it, situacion).filterIndexed { k, _ -> k != i }) } },
                    acabado = Acabado.NEGRA, chica = true, cuadrada = true, descripcion = "Borrar")
            }
        }
        oido?.let { AvisoLinea(it) }
        Visor(Modifier.fillMaxWidth()) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Grabado("Nueva")
                Grabado("${largoBytes(nueva)}/${Acompanar.MAX_TEXTO}", color = C.grisOscuro)
            }
            val fabrica = VidaBase.frases[situacion] as? kotlinx.serialization.json.JsonObject
            CampoVisor(nueva, { nueva = it }, placeholder = Acompanar.EJEMPLO[situacion] ?: com.sinteticalabs.app.consola.Compania.deFabrica(fabrica) ?: "Frase nueva", estilo = T.campoClaro,
                maxBytes = Acompanar.MAX_TEXTO, alEscribir = { nueva = it })
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                com.sinteticalabs.app.consola.Compania.fichas(fabrica).forEach { f ->
                    Tecla(f, { nueva = cortarBytes("$nueva$f", Acompanar.MAX_TEXTO) }, acabado = Acabado.NEGRA, chica = true, alto = 34.dp)
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                Tecla("+ Sumar", { val t = nueva; vm.cambiar { Acompanar.sumarFrase(it, situacion, t) }; nueva = "" }, acabado = Acabado.NARANJA, chica = true,
                    habilitada = nueva.isNotBlank() && total < Acompanar.MAX_FRASES)
            }
        }
    }
}

// --- Cuidado: agua, pausa en el trabajo, "¿cómo estás?", te extraña y el resumen ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModCuidado(vm: RobotViewModel) {
    val cfg by vm.config.collectAsState()
    val c = Acompanar.cuidado(cfg)
    fun poner(f: (Acompanar.Cuidado) -> Acompanar.Cuidado) = vm.cambiar { Acompanar.ponerCuidado(it, f) }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Bandeja(Modifier.weight(1f)) {
                Perilla(c.agua.toDouble(), "Agua", { v -> poner { it.copy(agua = v.toInt()) } }, Modifier.fillMaxWidth(), min = 0.0, max = 8.0, paso = 1.0, tamano = 64.dp,
                    mostrar = { if (it > 0) "${it.toInt()} h" else "No" })
            }
            Bandeja(Modifier.weight(1f)) {
                Perilla(c.pausa.toDouble(), "Pausa", { v -> poner { it.copy(pausa = v.toInt()) } }, Modifier.fillMaxWidth(), min = 0.0, max = 240.0, paso = 15.0, tamano = 64.dp,
                    mostrar = { if (it > 0) "${it.toInt()}′" else "No" })
            }
        }
        Bandeja {
            Grabado("Trabajo", color = C.sobrePlaca)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                VisorHora("Desde", c.desde, Modifier.weight(1f)) { d -> poner { Acompanar.moverTrabajo(it, true, if (d > 0) 30 else -30) } }
                VisorHora("Hasta", c.hasta, Modifier.weight(1f)) { d -> poner { Acompanar.moverTrabajo(it, false, if (d > 0) 30 else -30) } }
            }
            FilaDias(c.dias) { l -> poner { it.copy(dias = Acompanar.alternarDia(it.dias, l)) } }
        }
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            Llave("¿Cómo estás?", c.animo, { poner { it.copy(animo = !it.animo) } })
            Llave("Resumen", c.resumen, { poner { it.copy(resumen = !it.resumen) } })
        }
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp)).background(C.hundido).padding(8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Llave("Te extraña", c.extranar, { poner { it.copy(extranar = !it.extranar) } }, Modifier.weight(1f))
            Tecla("−", { poner { it.copy(extranarDias = maxOf(1, it.extranarDias - 1)) } }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = c.extranar, descripcion = "Un día menos")
            Text("${c.extranarDias}d", style = T.digitosChicos.copy(color = if (c.extranar) C.brasa else C.grisOscuro))
            Tecla("+", { poner { it.copy(extranarDias = minOf(7, it.extranarDias + 1)) } }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = c.extranar, descripcion = "Un día más")
        }
    }
}

/** Los siete días como teclas (naranjas los elegidos). */
@Composable
fun FilaDias(dias: String, alTocar: (String) -> Unit) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        listOf("L", "M", "X", "J", "V", "S", "D").forEach { l ->
            Tecla(l, { alTocar(l) }, Modifier.weight(1f), acabado = if (l in dias) Acabado.NARANJA else Acabado.BLANCA, chica = true, alto = 36.dp)
        }
    }
}

// --- Jugar: cinco juegos en su pantalla y el marcador ---

private val JUEGOS_UI = listOf("ppt" to "Piedra papel tijera", "reflejos" to "Reflejos", "mirada" to "Mirada", "dado" to "Dado", "moneda" to "Moneda")

@Composable
fun ModJugar(vm: RobotViewModel) {
    val robot by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    val conectado = robot?.conectado == true
    var jugando by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf<String?>(null) }
    var marcador by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    LaunchedEffect(Unit) {
        val m = vm.apiNube()?.takeIf { vm.via == Via.NUBE }?.let { runCatching { it.memoriaCruda(vm.idRobot) }.getOrNull() }
        fun n(vararg k: String) = k.firstNotNullOfOrNull { m?.numero(it) }
        val reflejo = n("mejorReflejo")?.let { "%.2f".format(it / 1000).replace('.', ',') + "s" } ?: "—"
        marcador = listOf("Jugados" to (n("juegosJugados", "jugados")?.toInt()?.toString() ?: "—"), "Ganó" to (n("juegosGanados", "ganados")?.toInt()?.toString() ?: "—"),
            "Hoy" to (n("juegosHoy")?.toInt()?.toString() ?: "—"), "Reflejo" to reflejo)
    }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        JUEGOS_UI.chunked(3).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                fila.forEach { (j, n) ->
                    Tecla(null, {
                        if (jugando == null) alcance.launch {
                            jugando = j
                            aviso = vm.jugar(j) ?: "¡A jugar!"
                            jugando = null
                            delay(2500); aviso = null
                        }
                    }, Modifier.weight(1f), habilitada = conectado, apretada = jugando == j, alto = 86.dp, descripcion = n) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                            IconoJuego(j)
                            Text(n.uppercase(), style = T.modulo.copy(fontSize = 9.sp), color = C.tinta, maxLines = 2)
                        }
                    }
                }
                repeat(3 - fila.size) { Spacer(Modifier.weight(1f)) }
            }
        }
        aviso?.let { AvisoLinea(it) }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            marcador.forEach { (t, v) ->
                Visor(Modifier.weight(1f)) {
                    Grabado(t)
                    Text(v, style = T.digitosChicos, maxLines = 1)
                }
            }
        }
    }
}

@Composable
private fun IconoJuego(j: String) {
    Canvas(Modifier.size(30.dp)) {
        val k = size.width / 40f
        val trazo = Stroke(2.6f * k)
        val c = C.tinta
        fun o(x: Float, y: Float) = Offset(x * k, y * k)
        when (j) {
            "ppt" -> {
                drawCircle(c, 5 * k, o(12f, 29f), style = trazo); drawCircle(c, 5 * k, o(28f, 29f), style = trazo)
                drawLine(c, o(15f, 25f), o(29f, 6f), 2.6f * k); drawLine(c, o(25f, 25f), o(11f, 6f), 2.6f * k)
            }
            "reflejos" -> drawPath(Path().apply { moveTo(23 * k, 3 * k); lineTo(9 * k, 23 * k); lineTo(19 * k, 23 * k); lineTo(16 * k, 37 * k); lineTo(31 * k, 16 * k); lineTo(21 * k, 16 * k); close() }, c)
            "mirada" -> {
                drawPath(Path().apply { moveTo(3 * k, 20 * k); quadraticBezierTo(20 * k, -1 * k, 37 * k, 20 * k); quadraticBezierTo(20 * k, 41 * k, 3 * k, 20 * k); close() }, c, style = trazo)
                drawCircle(C.naranja, 5 * k, o(24f, 20f))
            }
            "dado" -> {
                drawRoundRect(c, o(6f, 6f), Size(28 * k, 28 * k), CornerRadius(7 * k), style = trazo)
                listOf(13f to 13f, 27f to 13f, 20f to 20f, 13f to 27f, 27f to 27f).forEach { (x, y) -> drawCircle(c, 2.6f * k, o(x, y)) }
            }
            else -> {
                drawCircle(c, 15 * k, o(20f, 20f), style = trazo); drawCircle(c, 9.5f * k, o(20f, 20f), style = trazo)
                drawLine(c, o(20f, 15f), o(20f, 25f), 2.6f * k)
            }
        }
    }
}

// --- Su día: el despertador ---

@Composable
fun SeccionDespertador(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    var probando by remember { mutableStateOf<String?>(null) }
    val d = Acompanar.despertador(c)
    fun poner(f: (Acompanar.Despertador) -> Acompanar.Despertador) = vm.cambiar { Acompanar.ponerDespertador(it, f) }
    Bandeja(if (!d.activo) Modifier.background(Color(0x22000000), RoundedCornerShape(22.dp)) else Modifier) {
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Despertador")
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(d.hora, style = T.digitos.copy(fontSize = 34.sp, color = if (d.activo) C.brasa else C.grisOscuro), modifier = Modifier.weight(1f))
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Tecla("+1h", { poner { it.copy(hora = Editar.aHora(Editar.aMin(it.hora) + 60)) } }, acabado = Acabado.NEGRA, chica = true, alto = 32.dp)
                    Tecla("−1h", { poner { it.copy(hora = Editar.aHora(Editar.aMin(it.hora) - 60)) } }, acabado = Acabado.NEGRA, chica = true, alto = 32.dp)
                }
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Tecla("+5", { poner { it.copy(hora = Editar.aHora(Editar.aMin(it.hora) + 5)) } }, acabado = Acabado.NEGRA, chica = true, alto = 32.dp)
                    Tecla("−5", { poner { it.copy(hora = Editar.aHora(Editar.aMin(it.hora) - 5)) } }, acabado = Acabado.NEGRA, chica = true, alto = 32.dp)
                }
            }
        }
        FilaDias(d.dias) { l -> poner { it.copy(dias = Acompanar.alternarDia(it.dias, l)) } }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
            Llave("Activo", d.activo, { val on = d.activo; poner { it.copy(activo = !on) } })
            Llave("Sonido", d.sonido, { poner { it.copy(sonido = !it.sonido) } })
        }
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(C.hundido).padding(8.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Grabado("Posponer", Modifier.weight(1f))
            Tecla("−", { poner { it.copy(posponer = maxOf(1, it.posponer - 1)) } }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Menos")
            Text("${d.posponer}′", style = T.digitosChicos)
            Tecla("+", { poner { it.copy(posponer = minOf(30, it.posponer + 1)) } }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Más")
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Visor(Modifier.weight(1f)) {
                CampoVisor(d.texto.orEmpty(), { t -> val x = cortarBytes(t.trim(), Acompanar.MAX_TEXTO); poner { it.copy(texto = x.ifEmpty { null }) } },
                    placeholder = "¡Buen día!", estilo = T.campoClaro, maxBytes = Acompanar.MAX_TEXTO)
            }
            Tecla(probando ?: "Probar", {
                alcance.launch { probando = "…"; probando = vm.alarma("probar") ?: "Sonando"; delay(3000); probando = null }
            }, acabado = Acabado.NARANJA, chica = true, habilitada = robot?.conectado == true)
        }
    }
}

// --- Dónde vive: el buscador de ciudad y el clima de ahora ---

@Composable
fun SeccionUbicacion(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val api = vm.apiNube()
    val alcance = rememberCoroutineScope()
    val u = Acompanar.ubicacion(c)
    var buscando by remember { mutableStateOf(u == null) }
    var q by remember { mutableStateOf("") }
    var lugares by remember { mutableStateOf<List<Lugar>>(emptyList()) }
    var estado by remember { mutableStateOf("listo") }
    var clima by remember { mutableStateOf<Clima?>(null) }
    var vuelta by remember { mutableStateOf(0) }
    if (api == null) return
    LaunchedEffect(u, vuelta) {
        if (u == null || vm.via != Via.NUBE) return@LaunchedEffect
        clima = runCatching { api.clima(vm.idRobot) }.getOrNull()
        // La nube trae el clima apenas se guarda la ubicación.
        if (clima == null) { delay(3000); clima = runCatching { api.clima(vm.idRobot) }.getOrNull() }
    }
    LaunchedEffect(q) {
        val t = q.trim()
        if (largoBytes(t) < 2) { lugares = emptyList(); estado = "listo"; return@LaunchedEffect }
        delay(350)
        estado = "buscando"
        estado = runCatching { lugares = api.buscarCiudad(cortarBytes(t, 60)); if (lugares.isEmpty()) "nada" else "listo" }.getOrDefault("error")
    }
    if (u != null && !buscando) {
        Row(
            Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp)).background(C.hundido).padding(14.dp),
            verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Column(Modifier.weight(1f)) {
                Grabado("Clima")
                Text(u.ciudad.substringBefore(','), style = T.dato.copy(color = C.papel), maxLines = 1, overflow = TextOverflow.Ellipsis)
                clima?.let { Text("${it.descripcion} · ↑${Math.round(it.max)}° ↓${Math.round(it.min)}°", style = T.dato.copy(fontSize = 12.sp), color = C.gris) }
            }
            Text(clima?.let { "${Math.round(it.temp)}°" } ?: "--°", style = T.digitos.copy(fontSize = 34.sp, color = if (clima == null) C.grisOscuro else C.brasa))
            Tecla("Cambiar", { buscando = true }, acabado = Acabado.NEGRA, chica = true)
        }
    } else Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Visor(Modifier.fillMaxWidth()) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Grabado("Su ciudad", Modifier.weight(1f))
                Led(when (estado) { "buscando" -> C.led; "nada", "error" -> C.ambar; else -> null })
                if (u != null) Tecla("✕", { buscando = false; q = "" }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Cerrar")
            }
            CampoVisor(q, { q = it }, placeholder = "Buscá tu ciudad", estilo = T.campoClaro, maxBytes = 60, alEscribir = { q = it })
        }
        lugares.forEach { l ->
            Column(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.chapa).clickable(role = Role.Button) {
                    vm.cambiar { Acompanar.ponerUbicacion(it, l.lat, l.lon, l.ciudad.ifEmpty { l.nombre }) }
                    buscando = false; q = ""; lugares = emptyList(); clima = null
                    alcance.launch { delay(2500); vuelta++ }
                }.padding(horizontal = 14.dp, vertical = 10.dp),
            ) {
                Text(l.nombre, style = T.dato.copy(color = C.papel))
                Text(listOf(l.region, l.pais).filter { it.isNotEmpty() && it != l.nombre }.joinToString(" · "), style = T.dato.copy(fontSize = 12.sp), color = C.gris)
            }
        }
        if (estado == "nada") Text("Nada con ese nombre", style = T.texto, color = SOBRE)
        if (estado == "error") Text("No pude buscar", style = T.texto, color = SOBRE)
    }
}

// --- Reflejos: los avisos del celu ---

@Composable
fun SeccionAvisosCelu(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val avisos = Acompanar.avisos(c)
    val caras = (VidaBase.expresiones.keys + c.obj("expresiones").orEmpty().keys).toSortedSet().toList()
    val comportamientos = (VidaBase.comportamientos.keys + c.obj("comportamientos").orEmpty().keys).toSortedSet().toList()
    fun poner(i: Int, f: (Acompanar.AvisoCelu) -> Acompanar.AvisoCelu) = vm.cambiar { cfg ->
        val l = Acompanar.avisos(cfg).toMutableList()
        if (i in l.indices) l[i] = f(l[i])
        Acompanar.ponerAvisos(cfg, l)
    }
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado("Del celu · ${avisos.size}/${Acompanar.MAX_AVISOS}", color = C.sobrePlaca)
            Tecla("+ Aviso", { vm.cambiar { Acompanar.nuevoAviso(it) } }, acabado = Acabado.NEGRA, chica = true, habilitada = avisos.size < Acompanar.MAX_AVISOS)
        }
        EscucharCelu(vm)
        if (avisos.isEmpty()) Text("Mamá en WhatsApp → enamorado", style = T.texto, color = SOBRE)
        avisos.forEachIndexed { i, a ->
            Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp)).background(C.hundido).padding(12.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                    Tecla("Mensaje", { poner(i) { it.copy(llamada = false) } }, acabado = if (!a.llamada) Acabado.NARANJA else Acabado.BLANCA, chica = true, alto = 34.dp)
                    Tecla("Llamada", { poner(i) { it.copy(llamada = true) } }, acabado = if (a.llamada) Acabado.NARANJA else Acabado.BLANCA, chica = true, alto = 34.dp)
                    Spacer(Modifier.weight(1f))
                    Tecla("✕", { vm.cambiar { cfg -> Acompanar.ponerAvisos(cfg, Acompanar.avisos(cfg).filterIndexed { k, _ -> k != i }) } },
                        acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Borrar este aviso")
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Box(Modifier.weight(1f)) {
                        CampoVisor(a.app, { v -> poner(i) { it.copy(app = Acompanar.nombreAviso(v)) } }, placeholder = "App", estilo = T.dato.copy(color = C.papel), maxBytes = Acompanar.MAX_NOMBRE_AVISO)
                    }
                    Box(Modifier.weight(1f)) {
                        CampoVisor(a.contacto, { v -> poner(i) { it.copy(contacto = Acompanar.nombreAviso(v)) } }, placeholder = "Contacto", estilo = T.dato.copy(color = C.papel), maxBytes = Acompanar.MAX_NOMBRE_AVISO)
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("→", style = T.tecla, color = C.brasa)
                    Selector(a.expresion.ifEmpty { "Su cara" }, listOf("" to "Su cara") + caras.map { it to it }, { v -> poner(i) { it.copy(expresion = v) } },
                        Modifier.weight(1f), estilo = T.dato.copy(color = C.papel))
                    Selector(a.comportamiento.ifEmpty { "Nada más" }.replace('-', ' '), listOf("" to "Nada más") + comportamientos.map { it to it.replace('-', ' ') },
                        { v -> poner(i) { it.copy(comportamiento = v) } }, Modifier.weight(1f), estilo = T.dato.copy(color = C.papel))
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("“", style = T.tecla, color = C.brasa)
                    Box(Modifier.weight(1f)) {
                        CampoVisor(a.decir, { v -> poner(i) { it.copy(decir = Acompanar.decirAviso(v)) } }, placeholder = "{contacto} te escribe",
                            estilo = T.dato.copy(color = C.papel), maxBytes = Acompanar.MAX_DECIR_AVISO)
                    }
                    Grabado("${largoBytes(a.decir)}/${Acompanar.MAX_DECIR_AVISO}", color = C.grisOscuro)
                }
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                    listOf("{app}", "{contacto}").forEach { f ->
                        Tecla(f, { poner(i) { it.copy(decir = Acompanar.decirAviso("${it.decir} $f")) } }, acabado = Acabado.NEGRA, chica = true, alto = 32.dp,
                            habilitada = largoBytes(a.decir + f) <= Acompanar.MAX_DECIR_AVISO)
                    }
                    Spacer(Modifier.weight(1f))
                    Llave("Sonido", a.sonido, { poner(i) { it.copy(sonido = !it.sonido) } })
                }
            }
        }
    }
}

@Suppress("unused")
private val ALTO_JACK = Modifier.height(1.dp)

/**
 * "Este celu avisa": con permiso, las notificaciones de las apps de los
 * avisos (solo la app y el contacto, nunca el texto) le llegan al robot.
 */
@Composable
fun EscucharCelu(vm: RobotViewModel) {
    if (vm.via == Via.DEMO) return
    // Por la nube, el id del robot; de cerca, "cerca:<id>" (va directo por BLE mientras está conectado).
    val clave = if (vm.via == Via.NUBE) vm.idRobot else com.sinteticalabs.app.datos.AvisosCelu.claveCerca(vm.idRobot)
    val contexto = androidx.compose.ui.platform.LocalContext.current
    val c by vm.config.collectAsState()
    val avisos = Acompanar.avisos(c)
    var prendido by remember { mutableStateOf(com.sinteticalabs.app.datos.AvisosCelu.prendido(contexto, clave)) }
    var permiso by remember { mutableStateOf(com.sinteticalabs.app.datos.AvisosCelu.permiso(contexto)) }
    var explicando by remember { mutableStateOf(false) }
    // Al volver de los ajustes del sistema, se mira de nuevo.
    val dueno = androidx.compose.ui.platform.LocalLifecycleOwner.current
    androidx.compose.runtime.DisposableEffect(dueno) {
        val o = androidx.lifecycle.LifecycleEventObserver { _, e -> if (e == androidx.lifecycle.Lifecycle.Event.ON_RESUME) permiso = com.sinteticalabs.app.datos.AvisosCelu.permiso(contexto) }
        dueno.lifecycle.addObserver(o)
        onDispose { dueno.lifecycle.removeObserver(o) }
    }
    LaunchedEffect(avisos) { com.sinteticalabs.app.datos.AvisosCelu.actualizar(contexto, clave, avisos) }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Llave("Este celu avisa", prendido && permiso, {
            if (!permiso) explicando = true
            else {
                prendido = !prendido
                com.sinteticalabs.app.datos.AvisosCelu.poner(contexto, clave, if (prendido) avisos else null)
            }
        }, Modifier.weight(1f), habilitada = avisos.isNotEmpty())
        if (prendido && !permiso) Grabado("Sin permiso", color = C.sobrePlaca)
    }
    if (explicando) androidx.compose.ui.window.Dialog(onDismissRequest = { explicando = false }) {
        Chapa(Modifier.fillMaxWidth(), radio = 28.dp, relleno = 22.dp) {
            Grabado("Avisos del celu", color = C.brasa)
            Text("Solo de las apps de tus avisos.", style = T.texto.copy(color = C.papel))
            Text("Le pasa la app y el contacto, nunca el texto.", style = T.texto)
            Tecla("Dar permiso", {
                explicando = false
                prendido = true
                com.sinteticalabs.app.datos.AvisosCelu.poner(contexto, clave, avisos)
                runCatching { contexto.startActivity(android.content.Intent(android.provider.Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS).addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)) }
            }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
            Tecla("Ahora no", { explicando = false }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
        }
    }
}
