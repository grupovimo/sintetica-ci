package com.sinteticalabs.app.ui.consola

import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableDoubleStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.setProgress
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Editar
import kotlin.math.cos
import kotlin.math.roundToInt
import kotlin.math.sin

/**
 * Una perilla: se gira arrastrando para arriba o abajo. Muestra el valor en
 * un visor. `clara`: sobre la placa naranja (marcas oscuras).
 */
@Composable
fun Perilla(
    valor: Double,
    nombre: String,
    alCambiar: (Double) -> Unit,
    modifier: Modifier = Modifier,
    min: Double = 0.0,
    max: Double = 1.0,
    paso: Double = 0.05,
    tamano: Dp = 60.dp,
    mostrar: (Double) -> String = { ((it - min) / (max - min) * 100).roundToInt().toString() },
) {
    val actual by rememberUpdatedState(valor)
    val cambiar by rememberUpdatedState(alCambiar)
    var desde by remember { mutableDoubleStateOf(0.0) }
    var acumulado by remember { mutableDoubleStateOf(0.0) }
    val fraccion = ((valor - min) / (max - min)).toFloat().coerceIn(0f, 1f)
    fun ajustar(v: Double) {
        val r = Math.round(v.coerceIn(min, max) / paso) * paso
        val x = Math.round(r * 10000) / 10000.0
        if (x != actual) cambiar(x)
    }
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Canvas(
            Modifier.padding(9.dp).size(tamano)
                .semantics {
                    contentDescription = nombre
                    stateDescription = mostrar(valor)
                    progressBarRangeInfo = ProgressBarRangeInfo(valor.toFloat(), min.toFloat()..max.toFloat())
                    setProgress { ajustar(it.toDouble()); true }
                }
                .pointerInput(min, max, paso) {
                    detectVerticalDragGestures(
                        onDragStart = { desde = actual; acumulado = 0.0 },
                        onVerticalDrag = { c, dy ->
                            c.consume()
                            acumulado -= dy
                            ajustar(desde + acumulado / 160.dp.toPx() * (max - min))
                        },
                    )
                },
        ) {
            val r = size.minDimension / 2
            val c = center
            for (i in 0..10) {
                val a = Math.toRadians(-135.0 + i * 27).toFloat()
                val p1 = Offset(c.x + sin(a) * (r + 2.dp.toPx()), c.y - cos(a) * (r + 2.dp.toPx()))
                val p2 = Offset(c.x + sin(a) * (r + 7.dp.toPx()), c.y - cos(a) * (r + 7.dp.toPx()))
                drawLine(if (i / 10f <= fraccion + 0.001f) C.tinta else Color(0x591A1714), p1, p2, 3.dp.toPx(), StrokeCap.Round)
            }
            drawCircle(Color.Black, r, c + Offset(0f, 5.dp.toPx()))
            drawCircle(Brush.radialGradient(listOf(Color(0xFF3E3A35), Color(0xFF161412)), c + Offset(-r * 0.3f, -r * 0.4f), r * 1.5f), r, c)
            rotate(-135f + fraccion * 270f, c) {
                drawRoundRect(C.brasa, Offset(c.x - 2.5.dp.toPx(), c.y - r * 0.86f), Size(5.dp.toPx(), r * 0.52f), CornerRadius(3.dp.toPx()))
            }
        }
        Grabado(nombre, color = C.sobrePlaca)
        Text(
            mostrar(valor), style = T.digitosChicos.copy(fontSize = 15.sp, lineHeight = 16.sp, shadow = null),
            modifier = Modifier.clip(RoundedCornerShape(7.dp)).background(C.hundido).padding(horizontal = 8.dp, vertical = 5.dp),
        )
    }
}

/** El dial de personalidad: ocho posiciones alrededor de una perilla grande. */
@Composable
fun Dial(valor: String, alCambiar: (String) -> Unit, modifier: Modifier = Modifier) {
    val i = Editar.PRESETS_DIAL.indexOf(valor).coerceAtLeast(0)
    val giro by animateFloatAsState(i * 45f, spring(dampingRatio = 0.55f, stiffness = Spring.StiffnessLow), label = "dial")
    BoxWithConstraints(modifier) {
        val ancho = minOf(maxWidth, 320.dp)
        val k = ancho / 320.dp
        Box(Modifier.size(ancho, 280.dp * k)) {
            // La perilla grande, en el centro.
            Canvas(Modifier.size(150.dp * k).align(Alignment.Center)) {
                val r = size.minDimension / 2
                drawCircle(Color(0xFF8F887C), r, center + Offset(0f, 9.dp.toPx()))
                drawCircle(Brush.sweepGradient(listOf(Color(0xFFFBF9F5), Color(0xFFD4CCBF), Color(0xFFFBF9F5), Color(0xFFCFC7B9), Color(0xFFFBF9F5)), center), r)
                rotate(giro, center) {
                    drawRoundRect(C.naranja, Offset(center.x - 3.5.dp.toPx(), 10.dp.toPx()), Size(7.dp.toPx(), 40.dp.toPx() * k), CornerRadius(4.dp.toPx()))
                }
                drawCircle(Brush.radialGradient(listOf(C.papel, Color(0xFFBDB4A6)), center + Offset(-6f, -8f), r * 0.4f), r * 0.31f)
            }
            Editar.PRESETS_DIAL.forEachIndexed { j, p ->
                val a = Math.toRadians(-90.0 + j * 45)
                val x = 160.dp * k + 132.dp * k * cos(a).toFloat()
                val y = 140.dp * k + 118.dp * k * sin(a).toFloat()
                val elegida = p == valor
                Box(Modifier.offset(x - 56.dp, y - 16.dp).size(112.dp, 32.dp), contentAlignment = Alignment.Center) {
                    Text(
                        (Editar.NOMBRES_PRESET[p] ?: p).uppercase(), style = T.teclaChica.copy(fontSize = 10.sp), color = if (elegida) Color.White else Color(0xC71A1714),
                        maxLines = 1,
                        modifier = Modifier.clip(RoundedCornerShape(9.dp)).background(if (elegida) C.tinta else Color.Transparent)
                            .clickable(role = Role.RadioButton) { alCambiar(p) }
                            .padding(horizontal = 9.dp, vertical = 8.dp),
                    )
                }
            }
        }
    }
}

/** Un ± con su visor (horas, minutos): dos teclas negras al costado. */
@Composable
fun MasMenos(alMas: () -> Unit, alMenos: () -> Unit, nombre: String) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Tecla("+", alMas, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: más")
        Tecla("−", alMenos, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "$nombre: menos")
    }
}
