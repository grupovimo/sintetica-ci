package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyRow
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.GLIFO
import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.MIRADAS
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.PARPADEOS
import com.sinteticalabs.app.consola.PARTICULAS
import com.sinteticalabs.app.consola.con
import com.sinteticalabs.app.consola.nombreValido
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.sin
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

private fun formato(x: Double) = if (x == Math.floor(x)) x.toInt().toString() else "%.1f".format(x)

private fun pasoInicial(cara: String, dura: Double) = buildJsonObject { put("cara", cara); put("dura", dura) }

/**
 * Comportamientos: lo que hace solo, cuando le pasa algo, y sus rutinas. Cada
 * uno es una tira de pasos (una cara un rato). Se prueba acá (la vista previa)
 * o en el robot (web: routes/robots/[id]/comportamientos).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LabComportamientos(vm: RobotViewModel, modelo: Modelo) {
    val config by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var tipo by remember { mutableStateOf("ocioso") }
    var elegido by remember { mutableStateOf("") }
    var nombre by remember { mutableStateOf("") }
    var doc by remember { mutableStateOf(JsonObject(emptyMap())) }
    var original by remember { mutableStateOf("") }
    var paso by remember { mutableIntStateOf(0) }
    var vista by remember { mutableStateOf("pasos") }
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf("") }
    var guardando by remember { mutableStateOf(false) }

    val propios = config.obj("comportamientos").orEmpty().mapNotNull { (k, v) -> (v as? JsonObject)?.let { k to it } }.toMap()
    fun docDe(n: String) = propios[n] ?: VidaBase.comportamientos[n]
    val base = VidaBase.comportamientos.keys.sorted()
    val todos = base + propios.keys.filter { it !in base }.sorted()
    val lista = todos.filter { (docDe(it)?.texto("tipo") ?: "ocioso") == tipo }
    val pasos = Lab.pasos(doc)
    val cambiado = doc.toString() != original
    val dibujar = Lab.paraDibujar(config)
    val caras = VidaBase.nombresCaras
    val esOcioso = (doc.texto("tipo") ?: "ocioso") == "ocioso"
    val nombreProbar = if (nombreValido(nombre)) nombre else "mi-rutina"

    fun elegir(n: String) {
        error = null; aviso = ""
        elegido = n; nombre = n
        doc = docDe(n) ?: buildJsonObject { put("pasos", JsonArray(listOf(pasoInicial("contento", 1.5)))) }
        original = doc.toString()
        paso = 0
    }

    fun nuevo() {
        elegido = ""
        nombre = Lab.libre("mi-rutina", todos)
        var d = buildJsonObject { put("tipo", tipo); put("pasos", JsonArray(listOf(pasoInicial("contento", 1.5), pasoInicial("guino", 0.8)))) }
        if (tipo == "ocioso") d = d.con("cuando", "aburrimiento>0.3").con("enfriamiento", 120)
        doc = d
        original = ""
        paso = 0
    }

    LaunchedEffect(Unit) { if (doc.isEmpty()) elegir("mirar-alrededor") }

    // Los tipos y la lista.
    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        listOf("ocioso" to "☀ Solo", "reaccion" to "⚡ Reacciones", "rutina" to "☾ Rutinas").forEach { (k, t) ->
            Tecla(t, { tipo = k }, Modifier.weight(1f), acabado = Acabado.NEGRA, chica = true, apretada = tipo == k)
        }
    }
    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        item { Tecla("+ Nuevo", { nuevo() }, acabado = Acabado.NEGRA, chica = true) }
        items(lista.size) { i ->
            val n = lista[i]
            val sel = n == elegido
            Row(
                Modifier.clip(RoundedCornerShape(12.dp)).background(if (sel) C.chapa else C.hundido)
                    .then(if (sel) Modifier.border(2.dp, C.brasa, RoundedCornerShape(12.dp)) else Modifier)
                    .clickable { elegir(n) }.padding(6.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                MiniCara(Lab.primeraCara(docDe(n)), dibujar, modelo.pantalla, 44.dp)
                Text(n.replace('-', ' '), style = T.dato.copy(fontSize = 11.sp), color = if (sel) C.papel else C.grisClaro, maxLines = 1)
                if (n in propios) Led(C.led, tamano = 6.dp)
            }
        }
    }
    if (doc.isEmpty()) return

    // La vista previa, con "probar acá".
    Box {
        MarcoPantalla(modelo.pantalla) { if (llave > 0) PantallaViva(llave, "Vista previa") }
        Tecla("▶", {
            val c = Lab.ponerComportamiento(config, nombreProbar, doc)
            val e = Nucleo.validarConfig(c.toString())
            if (e != null) error = e
            else {
                error = null
                vm.pausarEspejo(maxOf(6.0, Lab.total(doc) + 2))
                vm.aplicarEnVistaPrevia(c)
                Nucleo.bleEscribir(Car.COMANDO.id, Comando.comportamiento(nombreProbar))
            }
        }, Modifier.align(Alignment.BottomEnd).padding(10.dp), acabado = Acabado.NARANJA, cuadrada = true, descripcion = "Probarlo acá")
    }

    val esBase = elegido in VidaBase.comportamientos
    BarraLab(
        nombre = nombre, alNombre = { nombre = it.take(24) }, fijo = elegido.isNotEmpty() && esBase,
        enRobot = if (elegido.isNotEmpty() && robot?.conectado == true) ({ val n = elegido; alcance.launch { error = vm.probarComportamiento(n) }; Unit }) else null,
        original = if (elegido.isNotEmpty() && elegido in propios) ({
            alcance.launch {
                error = vm.guardarAhora(Lab.ponerComportamiento(config, elegido, null))
                if (error == null) elegir(if (esBase) elegido else "mirar-alrededor")
            }
            Unit
        }) else null,
        textoOriginal = if (esBase) "Original" else "Borrar",
        guardar = {
            alcance.launch {
                error = null
                if (!nombreValido(nombre)) { error = "Minúsculas, sin espacios."; return@launch }
                guardando = true
                error = vm.guardarAhora(Lab.ponerComportamiento(config, nombre, doc))
                guardando = false
                if (error != null) return@launch
                elegido = nombre
                original = doc.toString()
                aviso = "Guardado"
                delay(2500)
                aviso = ""
            }
        },
        textoGuardar = if (guardando) "Guardando…" else aviso.ifEmpty { "Guardar" },
        puedeGuardar = !guardando && (cambiado || elegido.isEmpty()),
    )
    error?.let { AvisoLinea(it, error = true) }

    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Tecla("Pasos", { vista = "pasos" }, chica = true, apretada = vista == "pasos")
            if (esOcioso) Tecla("Cuándo", { vista = "cuando" }, chica = true, apretada = vista == "cuando")
            Spacer(Modifier.weight(1f))
            Grabado("${pasos.size} · ${"%.1f".format(Lab.total(doc))} s", color = C.sobrePlaca)
        }
        if (vista == "pasos" || !esOcioso) {
            // La tira: un cuadro por paso, más ancho cuanto más dura.
            LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                items(pasos.size) { i ->
                    val q = pasos[i]
                    val r = Lab.rango(q["dura"])
                    Column(
                        Modifier.widthIn(min = (maxOf(0.6, (r.first + r.second) / 2) * 30 + 40).dp).clip(RoundedCornerShape(12.dp)).background(C.hundido)
                            .then(if (i == paso) Modifier.border(2.dp, C.brasa, RoundedCornerShape(12.dp)) else Modifier)
                            .clickable { paso = i }.padding(6.dp),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        val c = q.texto("cara")
                        if (c != null) MiniCara(c, dibujar, modelo.pantalla, 58.dp) else Text("·", style = T.tecla, color = C.gris)
                        Text(if (r.first == r.second) "${formato(r.first)} s" else "${formato(r.first)}–${formato(r.second)} s",
                            style = T.dato.copy(fontSize = 10.sp), color = if (i == paso) C.papel else C.grisClaro)
                    }
                }
                item {
                    Tecla("+", { val (d, j) = Lab.sumarPaso(doc, paso); doc = d; paso = j }, acabado = Acabado.NEGRA, cuadrada = true,
                        habilitada = pasos.size < 16, descripcion = "Sumar un paso")
                }
            }
            if (paso < pasos.size) Bandeja { EditorPaso(doc, paso, caras, dibujar, modelo, { doc = it }, { d, j -> doc = d; paso = j }) }
        } else Bandeja { Cuando(doc) { doc = it } }
    }
}

@Composable
private fun EditorPaso(doc: JsonObject, i: Int, caras: List<String>, dibujar: String, modelo: Modelo, alCambiar: (JsonObject) -> Unit, alMover: (JsonObject, Int) -> Unit) {
    val p = Lab.pasos(doc)[i]
    val r = Lab.rango(p["dura"])
    val n = Lab.pasos(doc).size
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
        Tecla("←", { val (d, j) = Lab.moverPaso(doc, i, -1); alMover(d, j) }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = i > 0, descripcion = "Antes")
        Tecla("→", { val (d, j) = Lab.moverPaso(doc, i, 1); alMover(d, j) }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = i < n - 1, descripcion = "Después")
        Spacer(Modifier.weight(1f))
        Tecla("✕", { val (d, j) = Lab.quitarPaso(doc, i); alMover(d, j) }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = n > 1, descripcion = "Quitar el paso")
    }
    Visor(Modifier.fillMaxWidth()) {
        Grabado("Dura")
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Tecla("−", { alCambiar(Lab.ponerEnPaso(doc, i, "dura", Lab.dura(r.first - 0.5, r.second - 0.5))) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Menos")
            Text("${"%.1f".format((r.first + r.second) / 2)} s", style = T.digitosChicos, modifier = Modifier.weight(1f))
            Tecla("+", { alCambiar(Lab.ponerEnPaso(doc, i, "dura", Lab.dura(r.first + 0.5, r.second + 0.5))) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Más")
        }
    }
    Grabado("Cara", color = C.sobrePlaca)
    FilaDeCaras(caras, p.texto("cara"), dibujar, modelo.pantalla, conNada = true) { c ->
        alCambiar(Lab.ponerEnPaso(doc, i, "cara", c?.let { JsonPrimitive(it) }))
    }
    Grabado("Mira", color = C.sobrePlaca)
    Chips(MIRADAS, p.texto("mirada") ?: "libre") { m -> alCambiar(Lab.ponerEnPaso(doc, i, "mirada", if (m == "libre") null else JsonPrimitive(m))) }
    Grabado("Le sale", color = C.sobrePlaca)
    Chips(listOf("—") + PARTICULAS, p.texto("particula") ?: "—", { GLIFO[it] ?: it }) { m -> alCambiar(Lab.ponerEnPaso(doc, i, "particula", if (m == "—") null else JsonPrimitive(m))) }
    Grabado("Parpadea", color = C.sobrePlaca)
    Chips(PARPADEOS, p.texto("parpadeo") ?: "normal") { m -> alCambiar(Lab.ponerEnPaso(doc, i, "parpadeo", if (m == "normal") null else JsonPrimitive(m))) }
}

/** Cuándo le dan ganas (solo los que hace solo): ganas, seguido, espera, sus rasgos y la condición. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Cuando(doc: JsonObject, alCambiar: (JsonObject) -> Unit) {
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(16.dp), maxItemsInEachRow = 3) {
        Perilla(doc.numero("base") ?: 0.3, "Ganas", { alCambiar(doc.con("base", it)) }, Modifier.width(84.dp), tamano = 52.dp)
        Perilla(doc.numero("probabilidad") ?: 1.0, "Seguido", { alCambiar(doc.con("probabilidad", it)) }, Modifier.width(84.dp), min = 0.01, max = 1.0, paso = 0.01, tamano = 52.dp)
        Perilla(doc.numero("enfriamiento") ?: 120.0, "Espera", { alCambiar(doc.con("enfriamiento", it.toInt())) }, Modifier.width(84.dp), min = 0.0, max = 3600.0, paso = 30.0, tamano = 52.dp,
            mostrar = { if (it < 60) "${it.toInt()}s" else "${Math.round(it / 60)}m" })
    }
    Grabado("Le gusta más si es…", color = C.sobrePlaca)
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(16.dp), maxItemsInEachRow = 3) {
        Editar.RASGOS.forEach { r ->
            Perilla(Lab.afinidad(doc, r), Editar.NOMBRES_RASGO[r] ?: r, { alCambiar(Lab.ponerAfinidad(doc, r, it)) }, Modifier.width(84.dp), min = -1.0, max = 1.0, paso = 0.1, tamano = 46.dp,
                mostrar = { val k = Math.round(it * 10); if (k > 0) "+$k" else "$k" })
        }
    }
    Visor(Modifier.fillMaxWidth()) {
        Grabado("Solo si")
        CampoVisor(doc.texto("cuando").orEmpty(), { t -> val x = t.trim(); alCambiar(if (x.isEmpty()) doc.sin("cuando") else doc.con("cuando", x)) },
            placeholder = "aburrimiento>0.3", estilo = T.dato.copy(color = C.papel), maxBytes = 120)
    }
}

@Suppress("unused")
private val COLOR_NADA = Color.Transparent
