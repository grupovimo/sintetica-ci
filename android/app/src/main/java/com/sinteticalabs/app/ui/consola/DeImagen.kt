package com.sinteticalabs.app.ui.consola

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.bicho.PrepararImagen
import com.sinteticalabs.app.consola.CarasDeImagen
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.nombreValido
import com.sinteticalabs.app.consola.obj
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonObject

/** Un sprite a la vista (con su transparencia), decodificado una vez por CRC. */
@Composable
fun SpriteImagen(sprite: RobotViewModel.Sprite?, modifier: Modifier = Modifier, espejo: Boolean = false) {
    val img: ImageBitmap? = remember(sprite?.crc) { sprite?.let { PrepararImagen.decodificar(it.qoi, alfa = true)?.asImageBitmap() } }
    if (img != null) Image(img, contentDescription = null, contentScale = ContentScale.Fit, filterQuality = FilterQuality.Low,
        modifier = modifier.graphicsLayer { if (espejo) scaleX = -1f })
}

/**
 * Una cara de imagen: el ojo, el ojo cerrado y la boca son dibujos (SVG, PNG,
 * JPEG o GIF) que el robot anima vivos. Cada parte se sube al toque; la
 * pantalla de arriba es el núcleo dibujándola (web: expresiones/DeImagen.svelte).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun DeImagen(vm: RobotViewModel, modelo: Modelo, inicial: String, alListo: (String) -> Unit) {
    val config by vm.config.collectAsState()
    val sprites by vm.sprites.collectAsState()
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    val original = remember(inicial) { config.obj("expresiones")?.obj(inicial)?.takeIf { CarasDeImagen.esImagen(it) } }
    var doc by remember(inicial) { mutableStateOf(original ?: CarasDeImagen.nueva()) }
    var nombre by remember(inicial) { mutableStateOf(inicial.ifEmpty { com.sinteticalabs.app.consola.Lab.libre("mi-imagen", config.obj("expresiones")?.keys.orEmpty()) }) }
    val eraSiempre = inicial.isNotEmpty() && CarasDeImagen.siempre(config) == inicial
    var siempre by remember(inicial) { mutableStateOf(eraSiempre) }
    var pegando by remember { mutableStateOf<String?>(null) }
    var svg by remember { mutableStateOf("") }
    var subiendo by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var guardando by remember { mutableStateOf(false) }
    var paraSubir by remember { mutableStateOf("ojo") }
    val partes = CarasDeImagen.sprites(doc)
    val cambio = doc != original || nombre != inicial || siempre != eraSiempre

    fun subir(parte: String, datos: ByteArray, tipo: String) = alcance.launch {
        error = null
        val n = partes[parte] ?: CarasDeImagen.libre(config, inicial, doc)
        if (n == null) { error = "No hay lugar: borrá otra cara de imagen."; return@launch }
        subiendo = parte
        val e = vm.subirSprite(n, datos, tipo)
        subiendo = null
        if (e != null) error = e else doc = CarasDeImagen.conParte(doc, parte, n)
    }

    val galeria = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri: Uri? ->
        if (uri != null) {
            val tipo = contexto.contentResolver.getType(uri) ?: "image/png"
            val datos = runCatching { contexto.contentResolver.openInputStream(uri)?.use { it.readBytes() } }.getOrNull()
            if (datos != null) subir(paraSubir, datos, tipo) else error = "No pude leerla."
        }
    }

    suspend fun limpiar(c: JsonObject) { for (n in CarasDeImagen.sobrantes(original, c)) vm.borrarSprite(n); vm.cargarSprites() }

    // La cara, viva: la dibuja el núcleo con sus sprites.
    val vista = CarasDeImagen.vista(config, nombre, doc)
    val nombreVista = if (nombreValido(nombre)) nombre else "vista-previa"
    MarcoPantalla(modelo.pantalla) {
        if (partes["ojo"] != null) CaraViva(vista.toString(), nombreVista, modelo.pantalla, Modifier.fillMaxSize())
        else Grabado("Falta el ojo", Modifier.align(Alignment.Center), color = C.gris)
    }

    Visor(Modifier.fillMaxWidth()) {
        Grabado("Nombre")
        CampoVisor(nombre, { nombre = it }, placeholder = "mi-imagen", alEscribir = { nombre = it.lowercase().replace(' ', '-') })
    }
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Tecla(if (siempre) "★ Siempre" else "☆ Siempre", { siempre = !siempre }, acabado = Acabado.NEGRA, chica = true, apretada = siempre)
        if (inicial.isNotEmpty()) Tecla("Borrar", {
            alcance.launch {
                val c = CarasDeImagen.borrar(config, inicial)
                error = vm.guardarAhora(c)
                if (error == null) { limpiar(c); alListo("") }
            }
        }, acabado = Acabado.NEGRA, chica = true)
        Tecla(if (guardando) "Guardando…" else "Guardar", {
            alcance.launch {
                error = null
                if (!nombreValido(nombre)) { error = "Minúsculas, sin espacios."; return@launch }
                if (partes["ojo"] == null) { error = "Falta el ojo."; return@launch }
                val c = CarasDeImagen.guardar(config, inicial, nombre, doc, siempre)
                guardando = true
                error = vm.guardarAhora(c)
                guardando = false
                if (error == null) { limpiar(c); alListo(nombre) }
            }
        }, acabado = Acabado.NARANJA, chica = true, habilitada = !guardando && partes["ojo"] != null && cambio)
    }
    error?.let { AvisoLinea(it, error = true) }

    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            CarasDeImagen.PARTES_EDITABLES.forEach { (p, t) ->
                val n = partes[p]
                Bandeja(Modifier.weight(1f)) {
                    Box(Modifier.fillMaxWidth().aspectRatio(1f).clip(RoundedCornerShape(12.dp)).background(C.hundido), contentAlignment = Alignment.Center) {
                        if (n != null && sprites[n] != null) SpriteImagen(sprites[n], Modifier.fillMaxSize().padding(8.dp))
                        else Text(when (p) { "ojo" -> "◉"; "boca" -> "◡"; else -> "◠" }, style = T.nombre, color = if (p == "ojo") C.brasa else C.gris)
                    }
                    Grabado(t, color = C.sobrePlaca)
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        Tecla(if (subiendo == p) "…" else "↑", { paraSubir = p; galeria.launch("image/*") }, chica = true, cuadrada = true,
                            habilitada = subiendo == null, descripcion = "Subir $t")
                        Tecla("</>", { pegando = if (pegando == p) null else p }, chica = true, cuadrada = true, apretada = pegando == p,
                            habilitada = subiendo == null && vm.via == Via.NUBE, descripcion = "Pegar un SVG")
                    }
                    if (n != null && p != "ojo") Tecla("✕", { doc = CarasDeImagen.conParte(doc, p, null) }, acabado = Acabado.NEGRA, chica = true, cuadrada = true, descripcion = "Sacar")
                }
            }
        }
        if (pegando != null) Bandeja {
            BasicTextField(
                svg, { svg = it }, textStyle = T.dato.copy(fontSize = 12.sp, color = C.papel), cursorBrush = SolidColor(C.brasa),
                modifier = Modifier.fillMaxWidth().height(120.dp).clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(10.dp),
                decorationBox = { campo -> Box { if (svg.isEmpty()) Text("<svg viewBox=\"0 0 100 100\">…</svg>", style = T.dato.copy(fontSize = 12.sp), color = C.grisOscuro); campo() } },
            )
            Tecla("Listo", {
                val p = pegando ?: return@Tecla
                pegando = null
                subir(p, svg.trim().toByteArray(), "image/svg+xml")
                svg = ""
            }, acabado = Acabado.NARANJA, chica = true, habilitada = svg.trim().startsWith("<"))
        }
        Bandeja {
            FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(16.dp), maxItemsInEachRow = 4) {
                CarasDeImagen.PERILLAS.forEach { k ->
                    Perilla(CarasDeImagen.valor(doc, k), k.nombre, { doc = CarasDeImagen.conPerilla(doc, k, it) }, Modifier.width(76.dp),
                        min = k.min, max = k.max, paso = 0.01, tamano = 50.dp)
                }
            }
        }
    }
}

@Suppress("unused")
private val TRANSPARENTE = Color.Transparent
