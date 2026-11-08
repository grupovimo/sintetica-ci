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
import com.sinteticalabs.app.consola.GLIFO
import com.sinteticalabs.app.consola.LIMITES
import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.MOVIMIENTOS
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.PARPADEOS
import com.sinteticalabs.app.consola.PARTICULAS
import com.sinteticalabs.app.consola.Rostro
import com.sinteticalabs.app.consola.con
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.nombreValido
import com.sinteticalabs.app.consola.texto
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonObject

private data class Grupo(val clave: String, val nombre: String, val icono: String, val ojo: Boolean, val perillas: List<Pair<String, String>>)

private val GRUPOS = listOf(
    Grupo("ojos", "Ojos", "◉", true, listOf("w" to "Ancho", "h" to "Alto", "r" to "Redondez", "tilt" to "Giro", "squash" to "Aplastado")),
    Grupo("parpados", "Párpados", "◒", true, listOf("lt" to "Arriba", "lta" to "Ángulo", "lb" to "Abajo", "lba" to "Ángulo")),
    Grupo("adornos", "Adornos", "♥", true, listOf("smile" to "Feliz", "heart" to "Corazón", "star" to "Estrella", "cross" to "Cruz", "ring" to "Anillo", "pupil" to "Pupila")),
    Grupo("mirada", "Mirada", "⊕", false, listOf("ey" to "Altura", "sep" to "Separación", "fscale" to "Tamaño", "ftilt" to "Giro")),
    Grupo("boca", "Boca", "◡", false, listOf("mcurve" to "Sonrisa", "mo" to "Abierta", "mw" to "Ancha", "blush" to "Cachetes")),
    Grupo("efectos", "Efectos", "✦", false, listOf("pulse" to "Pulso", "beat" to "Latido", "shake" to "Temblor", "spin" to "Remolino", "orbit" to "Órbita", "glitch" to "Glitch", "dim" to "Apagado", "hue" to "Rojo")),
    Grupo("vida", "Gestos", "≈", false, emptyList()),
)

private data class Extra(val movimiento: String = "ninguno", val particula: String = "ninguna", val parpadeo: String = "normal")

private fun extraDe(d: JsonObject?) = Extra(d?.texto("movimiento") ?: "ninguno", d?.texto("particula") ?: "ninguna", d?.texto("parpadeo") ?: "normal")

/**
 * Expresiones: una galería con las caras de verdad (las dibuja el núcleo), la
 * elegida grande y viva, y perillas por parte de la cara. Se guarda solo lo
 * que cambia respecto de la original (web: routes/robots/[id]/expresiones).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LabExpresiones(vm: RobotViewModel, modelo: Modelo) {
    val config by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val alcance = rememberCoroutineScope()
    var elegida by remember { mutableStateOf("") }
    var nombre by remember { mutableStateOf("") }
    var valores by remember { mutableStateOf(Rostro.neutral) }
    var referencia by remember { mutableStateOf(Rostro.neutral) }
    var extra by remember { mutableStateOf(Extra()) }
    var extraRef by remember { mutableStateOf(Extra()) }
    var lado by remember { mutableStateOf("ojo") }
    var grupo by remember { mutableStateOf("ojos") }
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf("") }
    var guardando by remember { mutableStateOf(false) }
    // Una cara de imagen abierta: null = el editor de perillas; "" = una nueva.
    var imagen by remember { mutableStateOf<String?>(null) }
    val sprites by vm.sprites.collectAsState()
    val caraImagen = com.sinteticalabs.app.consola.CarasDeImagen.deImagen(config)

    val propias = Lab.propiasParametricas(config) - caraImagen.keys
    val base = VidaBase.nombresCaras
    val nombres = base + (propias.keys + caraImagen.keys).filter { it !in base }.sorted()
    val dibujar = Lab.paraDibujar(config)
    val esBase = elegida in VidaBase.expresiones
    val nombreDibujo = if (nombreValido(nombre)) nombre else "mi-cara"

    var documento = Rostro.documento(valores, referencia)
    if (extra.movimiento != extraRef.movimiento) documento = documento.con("movimiento", extra.movimiento)
    if (extra.particula != extraRef.particula) documento = documento.con("particula", extra.particula)
    if (extra.parpadeo != extraRef.parpadeo) documento = documento.con("parpadeo", extra.parpadeo)
    val cambios = documento.size
    val enVivo = Lab.ponerExpresion(leerObjeto(dibujar), nombreDibujo, documento).toString()

    fun elegir(n: String) {
        error = null
        aviso = ""
        imagen = if (n in caraImagen) n else null
        elegida = n
        if (imagen != null) return
        nombre = n
        val ref = if (n in VidaBase.expresiones) Rostro.deFabrica(n, VidaBase.expresiones) else Rostro.neutral
        referencia = ref
        val propia = propias[n]
        var v = propia?.texto("extiende")?.let { Rostro.deFabrica(it, VidaBase.expresiones) } ?: ref
        if (propia != null) v = Rostro.aplicar(propia, v)
        valores = v
        val er = extraDe(VidaBase.expresiones[n])
        extraRef = er
        extra = Extra(propia?.texto("movimiento") ?: er.movimiento, propia?.texto("particula") ?: er.particula, propia?.texto("parpadeo") ?: er.parpadeo)
    }

    fun nueva() {
        imagen = null
        elegida = ""
        nombre = Lab.libre("mi-cara", nombres)
        referencia = Rostro.neutral
        extraRef = Extra()
    }

    LaunchedEffect(Unit) { if (elegida.isEmpty() && nombre.isEmpty()) elegir("contento") }

    // La galería.
    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        item {
            Column(Modifier.clickable { nueva() }.padding(4.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Box(
                    Modifier.width(96.dp).aspectRatio(if (modelo.redonda) 1f else 320f / 172).clip(RoundedCornerShape(9.dp))
                        .border(2.dp, Color(0xFF3A3632), RoundedCornerShape(9.dp)),
                    contentAlignment = Alignment.Center,
                ) { Text("+", style = T.nombre, color = C.brasa) }
                Grabado("Nueva", color = C.grisClaro)
            }
        }
        item {
            Column(Modifier.clickable { imagen = ""; elegida = "" }.padding(4.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Box(
                    Modifier.width(96.dp).aspectRatio(if (modelo.redonda) 1f else 320f / 172).clip(RoundedCornerShape(9.dp))
                        .border(2.dp, if (imagen == "") C.brasa else Color(0xFF3A3632), RoundedCornerShape(9.dp)),
                    contentAlignment = Alignment.Center,
                ) { Text("▣", style = T.nombre, color = C.brasa) }
                Grabado("De imagen", color = C.grisClaro)
            }
        }
        items(nombres.size) { i ->
            val n = nombres[i]
            val sel = n == elegida
            Column(
                Modifier.clip(RoundedCornerShape(12.dp)).background(if (sel) C.chapa else Color.Transparent)
                    .then(if (sel) Modifier.border(2.dp, C.brasa, RoundedCornerShape(12.dp)) else Modifier)
                    .clickable { elegir(n) }.padding(5.dp),
                horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                val ojo = caraImagen[n]?.let { com.sinteticalabs.app.consola.CarasDeImagen.sprites(it)["ojo"] }
                if (n in caraImagen) Box(
                    Modifier.width(96.dp).aspectRatio(if (modelo.redonda) 1f else 320f / 172).clip(RoundedCornerShape(8.dp)).background(Color.Black),
                    contentAlignment = Alignment.Center,
                ) {
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        SpriteImagen(ojo?.let { sprites[it] }, Modifier.width(30.dp).aspectRatio(1f))
                        SpriteImagen(ojo?.let { sprites[it] }, Modifier.width(30.dp).aspectRatio(1f), espejo = true)
                    }
                } else MiniCara(n, dibujar, modelo.pantalla, 96.dp)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(n, style = T.dato.copy(fontSize = 10.sp), color = if (sel) C.papel else C.grisClaro, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.widthIn(max = 84.dp))
                    if (n in propias || n in caraImagen) Led(C.led, tamano = 6.dp)
                }
            }
        }
    }

    val ini = imagen
    if (ini != null) {
        DeImagen(vm, modelo, ini) { n -> if (n.isNotEmpty()) { imagen = n; elegida = n } else elegir("neutral") }
        return
    }

    MarcoPantalla(modelo.pantalla) { CaraViva(enVivo, nombreDibujo, modelo.pantalla, Modifier.fillMaxWidth().aspectRatio(modelo.pantalla.ancho.toFloat() / modelo.pantalla.alto)) }

    BarraLab(
        nombre = nombre, alNombre = { nombre = it.take(24) }, fijo = elegida.isNotEmpty() && esBase,
        enRobot = if (elegida.isNotEmpty() && robot?.conectado == true) ({
            val n = elegida
            alcance.launch {
                error = vm.comando(expresion = n)
                delay(4000)
                vm.comando(expresion = "")
            }
            Unit
        }) else null,
        original = if (elegida.isNotEmpty() && elegida in propias) ({
            alcance.launch {
                error = vm.guardarAhora(Lab.ponerExpresion(config, elegida, null))
                if (error == null) elegir(if (elegida in VidaBase.expresiones) elegida else "neutral")
            }
            Unit
        }) else null,
        textoOriginal = if (esBase) "Original" else "Borrar",
        guardar = {
            alcance.launch {
                error = null
                if (!nombreValido(nombre)) { error = "Minúsculas, sin espacios."; return@launch }
                val doc = documento
                val c = if (nombre in VidaBase.expresiones && doc.isEmpty()) Lab.ponerExpresion(config, nombre, null) else Lab.ponerExpresion(config, nombre, doc)
                guardando = true
                error = vm.guardarAhora(c)
                guardando = false
                if (error != null) return@launch
                elegida = nombre
                aviso = "Guardada"
                delay(2500)
                aviso = ""
            }
        },
        textoGuardar = if (guardando) "Guardando…" else aviso.ifEmpty { if (cambios > 0) "Guardar ($cambios)" else "Guardar" },
        puedeGuardar = !guardando && (cambios > 0 || elegida.isEmpty()),
    )
    error?.let { AvisoLinea(it, error = true) }

    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        // Las solapas: una tecla por parte de la cara.
        GRUPOS.chunked(4).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                fila.forEach { g ->
                    val sel = grupo == g.clave
                    Tecla(null, { grupo = g.clave }, Modifier.weight(1f), acabado = if (sel) Acabado.NEGRA else Acabado.BLANCA, apretada = sel, alto = 54.dp) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text(g.icono, style = T.tecla, color = if (sel) C.brasa else C.tinta)
                            Text(g.nombre.uppercase(), style = T.modulo.copy(fontSize = 9.sp), color = if (sel) C.papel else C.tinta, maxLines = 1)
                        }
                    }
                }
                repeat(4 - fila.size) { Spacer(Modifier.weight(1f)) }
            }
        }
        Bandeja {
            val g = GRUPOS.first { it.clave == grupo }
            if (g.ojo) Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                listOf("ojo" to "Los dos", "izq" to "Izq.", "der" to "Der.").forEach { (k, t) -> Llave(t, lado == k, { lado = k }, Modifier.weight(1f)) }
            }
            if (grupo == "mirada") Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                Pad(valores["gx"] ?: 0.0, valores["gy"] ?: 0.0, -0.3, 0.3, -0.25, 0.25, "Hacia dónde mira", Modifier.width(220.dp)) { x, y ->
                    valores = valores + mapOf("gx" to x, "gy" to y)
                }
            }
            if (grupo == "vida") {
                Grabado("Se mueve", color = C.sobrePlaca)
                Chips(MOVIMIENTOS, extra.movimiento) { extra = extra.copy(movimiento = it) }
                Grabado("Le sale", color = C.sobrePlaca)
                Chips(listOf("ninguna") + PARTICULAS, extra.particula, { if (it == "ninguna") "—" else GLIFO[it] ?: it }) { extra = extra.copy(particula = it) }
                Grabado("Parpadea", color = C.sobrePlaca)
                Chips(PARPADEOS, extra.parpadeo) { extra = extra.copy(parpadeo = it) }
            } else FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(16.dp), maxItemsInEachRow = 3) {
                g.perillas.forEach { (p, t) ->
                    val lim = LIMITES[p]
                    val clave = if (g.ojo) (if (lado == "der") "der.$p" else "izq.$p") else p
                    Perilla(valores[clave] ?: lim?.defecto ?: 0.0, t, { v ->
                        valores = if (!g.ojo) valores + (p to v) else valores.toMutableMap().apply {
                            if (lado != "der") put("izq.$p", v)
                            if (lado != "izq") put("der.$p", v)
                        }
                    }, Modifier.width(84.dp), min = lim?.min ?: 0.0, max = lim?.max ?: 1.0, paso = 0.01, tamano = 52.dp)
                }
            }
        }
    }
}
