package com.sinteticalabs.app.ui.consola

import android.graphics.Bitmap
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.layout
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.R
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.Rect
import com.sinteticalabs.app.consola.esEstadoDelSistema
import com.sinteticalabs.app.nucleo.AnfitrionEnMemoria
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.Pantalla
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.json.JsonObject
import kotlin.math.roundToInt

/** Pone un hijo en un rectángulo (px) de su caja, aunque se salga (la caja lo recorta). */
fun Modifier.colocar(r: Rect) = this.layout { m, c ->
    val p = m.measure(Constraints.fixed(r.ancho.roundToInt().coerceAtLeast(1), r.alto.roundToInt().coerceAtLeast(1)))
    layout(c.maxWidth, c.maxHeight) { p.place(r.x.roundToInt(), r.y.roundToInt()) }
}

private fun fotoDe(nombre: String) = if (nombre == "pieza_colgante") R.drawable.pieza_colgante else R.drawable.hebilla

private val APAGADO = ColorFilter.colorMatrix(ColorMatrix().apply {
    setToSaturation(0.3f)
    timesAssign(ColorMatrix().apply { setToScale(0.62f, 0.62f, 0.62f, 1f) })
})

/**
 * La foto real del robot con su pantalla viva encima: el mismo núcleo que
 * corre en el robot, encastrado donde cae la pantalla en la foto
 * (web/src/lib/consola/Producto.svelte).
 */
@Composable
fun Producto(
    modelo: Modelo,
    modifier: Modifier = Modifier,
    apagado: Boolean = false,
    radio: Dp = 30.dp,
    etiqueta: String? = null,
    pantalla: @Composable BoxScope.() -> Unit,
) {
    val e = modelo.encuadre
    BoxWithConstraints(modifier.fillMaxWidth().aspectRatio(4f / 5f).clip(RoundedCornerShape(radio)).background(Color(e.fondo))) {
        val w = constraints.maxWidth.toFloat()
        Image(
            painterResource(fotoDe(e.foto)), contentDescription = e.alt, contentScale = ContentScale.FillBounds,
            colorFilter = if (apagado) APAGADO else null, modifier = Modifier.colocar(e.rectFoto(w)),
        )
        Box(
            Modifier.colocar(e.rectPantalla(w)).clip(if (modelo.redonda) CircleShape else RoundedCornerShape(10.dp))
                .background(Color.Black).alpha(if (apagado) 0.25f else 1f),
        ) {
            Box(Modifier.colocar(e.encastre(w, modelo.redonda, modelo.pantalla.ancho, modelo.pantalla.alto)), content = pantalla)
        }
        if (etiqueta != null) Row(
            Modifier.align(Alignment.BottomCenter).padding(14.dp).fillMaxWidth().heightIn(min = 40.dp)
                .clip(RoundedCornerShape(12.dp)).background(Color(0xD90E0D0B)).padding(horizontal = 14.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Led(C.led)
            Text(etiqueta, style = T.dato, color = C.papel, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/**
 * La pantalla viva: el núcleo (uno solo por proceso) dibujando cuadro a
 * cuadro. `llave` cambia cuando se reinicia el núcleo. Tocarla es tocar el
 * robot de la vista previa.
 */
@Composable
fun PantallaViva(llave: Int, descripcion: String, alApretar: ((Boolean) -> Unit)? = null) {
    val ancho = remember(llave) { Nucleo.ancho() }
    val alto = remember(llave) { Nucleo.alto() }
    val bitmap = remember(llave, ancho, alto) { Bitmap.createBitmap(ancho, alto, Bitmap.Config.ARGB_8888) }
    val imagen = remember(bitmap) { bitmap.asImageBitmap() }
    val buffer = remember(llave, ancho, alto) { IntArray(ancho * alto) }
    var cuadro by remember { mutableIntStateOf(0) }
    val apretar by rememberUpdatedState(alApretar)
    LaunchedEffect(llave, ancho, alto) {
        while (true) {
            withFrameNanos {
                Nucleo.tick()
                Nucleo.pixeles(buffer)
                bitmap.setPixels(buffer, 0, ancho, 0, 0, ancho, alto)
                cuadro++
            }
        }
    }
    Canvas(
        Modifier.fillMaxSize().semantics { contentDescription = descripcion }.pointerInput(Unit) {
            detectTapGestures(onPress = {
                apretar?.invoke(true)
                tryAwaitRelease()
                apretar?.invoke(false)
            })
        },
    ) {
        @Suppress("UNUSED_EXPRESSION") cuadro // se redibuja en cada cuadro
        drawImage(imagen, dstOffset = IntOffset.Zero, dstSize = IntSize(size.width.roundToInt(), size.height.roundToInt()), filterQuality = FilterQuality.Low)
    }
}

/** Una imagen quieta en la pantalla (la foto de cómo está, en el dock). */
@Composable
fun PantallaQuieta(imagen: ImageBitmap?) {
    if (imagen == null) Box(Modifier.fillMaxSize().background(Color.Black))
    else Image(imagen, contentDescription = null, contentScale = ContentScale.FillBounds, filterQuality = FilterQuality.Low, modifier = Modifier.fillMaxSize())
}

/**
 * Las fotos de la pantalla de cada robot para el dock: el núcleo es uno solo,
 * así que se prende con la config de cada uno, se lo deja vivir un momento y
 * se saca una foto.
 */
object Instantaneas {
    val fotos = mutableStateMapOf<String, ImageBitmap>()
    private val claves = mutableMapOf<String, String>()
    private val turno = Mutex()
    /** Mientras hay una consola abierta, el núcleo es de ella: no se sacan fotos. */
    var pausado by mutableStateOf(false)

    suspend fun sacar(id: String, pantalla: Pantalla, config: JsonObject, expresion: String) {
        val clave = "${config.hashCode()}:$expresion:${pantalla.ancho}x${pantalla.alto}"
        if (claves[id] == clave && fotos[id] != null) return
        turno.withLock {
            if (pausado) return
            Nucleo.iniciar(pantalla.ancho, pantalla.alto, id, AnfitrionEnMemoria(), redonda = pantalla.redonda)
            Nucleo.vistaPrevia()
            for (e in escriturasDeConfig(config.toString(), 512)) Nucleo.bleEscribir(Car.CONFIG.id, e)
            if (expresion.isNotEmpty() && !esEstadoDelSistema(expresion)) Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(expresion))
            val inicio = withFrameNanos { it }
            var t = inicio
            while (t - inicio < 1_200_000_000L) {
                if (pausado) return
                t = withFrameNanos { Nucleo.tick(); it }
            }
            val w = Nucleo.ancho()
            val h = Nucleo.alto()
            val px = IntArray(w * h)
            Nucleo.pixeles(px)
            fotos[id] = Bitmap.createBitmap(px, w, h, Bitmap.Config.ARGB_8888).asImageBitmap()
            claves[id] = clave
        }
    }
}
