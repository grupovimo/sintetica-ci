package com.sinteticalabs.app.ui.consola

import android.graphics.Bitmap
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Pantalla
import kotlin.math.roundToInt

/** Las fotos de las caras (las dibuja el núcleo, sin tocar la instancia), guardadas por config y tamaño. */
object Miniaturas {
    private val cache = object : LinkedHashMap<String, ImageBitmap?>(64, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, ImageBitmap?>?) = size > 160
    }

    /** Cambiaron los sprites: las caras de imagen se dibujan de nuevo. */
    fun vaciar() { cache.clear(); version++ }
    var version = 0
        private set

    fun de(config: String, nombre: String, ancho: Int, alto: Int, redonda: Boolean): ImageBitmap? {
        val k = "$version|${config.hashCode()}|$nombre|${ancho}x$alto|$redonda"
        if (cache.containsKey(k)) return cache[k]
        val px = Nucleo.dibujar(config, nombre, ancho, alto, redonda, 0.4f)
        val img = px?.let { Bitmap.createBitmap(it, ancho, alto, Bitmap.Config.ARGB_8888).asImageBitmap() }
        cache[k] = img
        return img
    }
}

/** El tamaño de una miniatura para un ancho dado (con la forma de la pantalla). */
private fun tamanoMini(p: Pantalla, ancho: Int): Pair<Int, Int> =
    if (p.redonda) ancho to ancho else ancho to maxOf(16, ancho * p.alto / p.ancho)

/** Una cara chiquita, quieta: la que dibuja el núcleo. */
@Composable
fun MiniCara(nombre: String, config: String, pantalla: Pantalla, ancho: Dp, modifier: Modifier = Modifier) {
    val (w, h) = tamanoMini(pantalla, 128)
    val img = remember(nombre, config, pantalla, Miniaturas.version) { Miniaturas.de(config, nombre, w, h, pantalla.redonda) }
    Box(
        modifier.width(ancho).aspectRatio(w.toFloat() / h).clip(if (pantalla.redonda) CircleShape else RoundedCornerShape(8.dp)).background(Color.Black),
        contentAlignment = Alignment.Center,
    ) {
        if (img != null) Image(img, contentDescription = nombre, contentScale = ContentScale.FillBounds, filterQuality = FilterQuality.Low, modifier = Modifier.fillMaxSize())
        else Text("·", style = T.tecla, color = C.gris)
    }
}

/**
 * Una cara viva sin la instancia: el núcleo la dibuja cuadro a cuadro (giros,
 * latidos) con un parpadeo de vez en cuando. `null` en `alFallar` si esa cara
 * no es paramétrica (un dibujo o un estado del sistema).
 */
@Composable
fun CaraViva(config: String, nombre: String, pantalla: Pantalla, modifier: Modifier = Modifier, fps: Int = 30, alFallar: (@Composable BoxScope.() -> Unit)? = null) {
    val w = pantalla.ancho
    val h = pantalla.alto
    val bitmap = remember(w, h) { Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888) }
    val imagen = remember(bitmap) { bitmap.asImageBitmap() }
    var cuadro by remember { mutableIntStateOf(-1) }
    val cfg by rememberUpdatedState(config)
    val nom by rememberUpdatedState(nombre)
    LaunchedEffect(w, h, pantalla.redonda) {
        val inicio = withFrameNanos { it }
        var ultimo = 0L
        while (true) {
            withFrameNanos { ahora ->
                if (ahora - ultimo < 1_000_000_000L / fps) return@withFrameNanos
                ultimo = ahora
                val t = (ahora - inicio) / 1e9f
                // Un parpadeo cada 3,6 s (150 ms).
                val fase = t % 3.6f
                val cierre = if (fase < 0.15f) 1f - Math.abs(fase - 0.075f) / 0.075f else 0f
                val px = Nucleo.dibujar(cfg, nom, w, h, pantalla.redonda, t, cierre)
                if (px == null) cuadro = -1
                else {
                    bitmap.setPixels(px, 0, w, 0, 0, w, h)
                    cuadro = (cuadro + 1) and 0xFFFF
                }
            }
        }
    }
    Box(modifier) {
        if (cuadro < 0 && alFallar != null) alFallar()
        else Canvas(Modifier.fillMaxSize().semantics { contentDescription = nombre }) {
            @Suppress("UNUSED_EXPRESSION") cuadro
            drawImage(imagen, dstOffset = IntOffset.Zero, dstSize = IntSize(size.width.roundToInt(), size.height.roundToInt()), filterQuality = FilterQuality.Low)
        }
    }
}

/** La pantalla grande del laboratorio: la forma de la pantalla del robot, en una chapa. */
@Composable
fun MarcoPantalla(pantalla: Pantalla, modifier: Modifier = Modifier, contenido: @Composable BoxScope.() -> Unit) {
    Box(
        modifier.fillMaxWidth().clip(RoundedCornerShape(if (pantalla.redonda) 200.dp else 30.dp))
            .background(Brush.verticalGradient(listOf(C.chapaAlta, C.chapaBaja))).padding(14.dp),
        contentAlignment = Alignment.Center,
    ) {
        Box(
            Modifier.fillMaxWidth().aspectRatio(if (pantalla.redonda) 1f else pantalla.ancho.toFloat() / pantalla.alto)
                .clip(if (pantalla.redonda) CircleShape else RoundedCornerShape(18.dp)).background(Color.Black),
            content = contenido,
        )
    }
}

/** Un pad 2D: se arrastra un punto (la mirada: x a los costados, y arriba/abajo). Doble toque, al centro. */
@Composable
fun Pad(x: Double, y: Double, minX: Double, maxX: Double, minY: Double, maxY: Double, nombre: String, modifier: Modifier = Modifier, alCambiar: (Double, Double) -> Unit) {
    val cambiar by rememberUpdatedState(alCambiar)
    val fx = ((x - minX) / (maxX - minX)).toFloat()
    val fy = (1 - (y - minY) / (maxY - minY)).toFloat()
    Canvas(
        modifier.aspectRatio(1f).clip(RoundedCornerShape(18.dp)).background(C.hundido)
            .semantics { contentDescription = nombre }
            .pointerInput(minX, maxX, minY, maxY) {
                fun mover(p: Offset) {
                    val a = (p.x / size.width).coerceIn(0f, 1f)
                    val b = (p.y / size.height).coerceIn(0f, 1f)
                    cambiar(Math.round((minX + a * (maxX - minX)) * 1000) / 1000.0, Math.round((minY + (1 - b) * (maxY - minY)) * 1000) / 1000.0)
                }
                detectDragGestures(onDragStart = { mover(it) }) { c, _ -> c.consume(); mover(c.position) }
            }
            .pointerInput(Unit) { detectTapGestures(onDoubleTap = { cambiar(0.0, 0.0) }) },
    ) {
        val paso = 14.dp.toPx()
        var yy = paso / 2
        while (yy < size.height) {
            var xx = paso / 2
            while (xx < size.width) { drawCircle(Color(0x12FFFFFF), 1.dp.toPx(), Offset(xx, yy)); xx += paso }
            yy += paso
        }
        drawLine(Color(0x1AFFFFFF), Offset(size.width * 0.1f, size.height / 2), Offset(size.width * 0.9f, size.height / 2), 1f)
        drawLine(Color(0x1AFFFFFF), Offset(size.width / 2, size.height * 0.1f), Offset(size.width / 2, size.height * 0.9f), 1f)
        val c = Offset(size.width * fx, size.height * fy)
        drawCircle(Brush.radialGradient(listOf(C.led.copy(alpha = 0.6f), Color.Transparent), c, 26.dp.toPx()), 26.dp.toPx(), c)
        drawCircle(Color(0xFF9A3209), 13.dp.toPx(), c + Offset(0f, 3.dp.toPx()))
        drawCircle(Brush.radialGradient(listOf(Color(0xFFFFB08A), Color(0xFFF2541A)), c - Offset(4f, 5f), 15.dp.toPx()), 13.dp.toPx(), c)
    }
}

/** Una fila de chips: una tecla chica por opción. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun Chips(opciones: List<String>, elegida: String?, texto: (String) -> String = { it }, alElegir: (String) -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        opciones.forEach { o -> Tecla(texto(o), { alElegir(o) }, chica = true, apretada = elegida == o, alto = 36.dp) }
    }
}

/** El nombre y las teclas de un editor del laboratorio: en el robot, volver a la original o borrar, y guardar. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun BarraLab(
    nombre: String,
    alNombre: (String) -> Unit,
    fijo: Boolean,
    enRobot: (() -> Unit)?,
    original: (() -> Unit)?,
    textoOriginal: String,
    guardar: () -> Unit,
    textoGuardar: String,
    puedeGuardar: Boolean,
) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Visor(Modifier.fillMaxWidth()) {
            Grabado("Nombre")
            if (fijo) Text(nombre, style = T.campo, maxLines = 1, overflow = TextOverflow.Ellipsis)
            else CampoVisor(nombre, alNombre, placeholder = "mi-cara", alEscribir = { alNombre(it.lowercase().replace(' ', '-')) })
        }
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            if (enRobot != null) Tecla("▶ En el robot", enRobot, acabado = Acabado.NEGRA, chica = true)
            if (original != null) Tecla(textoOriginal, original, acabado = Acabado.NEGRA, chica = true)
            Tecla(textoGuardar, guardar, acabado = Acabado.NARANJA, chica = true, habilitada = puedeGuardar)
        }
    }
}

/** Una galería horizontal de caras para elegir. */
@Composable
fun FilaDeCaras(caras: List<String>, elegida: String?, config: String, pantalla: Pantalla, ancho: Dp = 58.dp, conNada: Boolean = false, alElegir: (String?) -> Unit) {
    androidx.compose.foundation.lazy.LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        if (conNada) item {
            Box(
                Modifier.width(ancho).aspectRatio(if (pantalla.redonda) 1f else 320f / 172).clip(RoundedCornerShape(8.dp)).background(C.hundido)
                    .clickable { alElegir(null) },
                contentAlignment = Alignment.Center,
            ) { Text("·", style = T.tecla, color = C.gris) }
        }
        items(caras.size) { i ->
            val c = caras[i]
            Column(Modifier.clickable { alElegir(c) }.padding(2.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                MiniCara(c, config, pantalla, ancho, if (c == elegida) Modifier.background(C.tinta).padding(2.dp) else Modifier)
            }
        }
    }
}
