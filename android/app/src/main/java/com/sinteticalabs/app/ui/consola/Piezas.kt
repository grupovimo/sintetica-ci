package com.sinteticalabs.app.ui.consola

import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithCache
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PointMode
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** El acabado de una tecla mecánica. */
enum class Acabado(val arriba: Color, val abajo: Color, val canto: Color, val letra: Color, val brillo: Color) {
    BLANCA(Color(0xFFFBF9F5), Color(0xFFE9E4DC), Color(0xFFA39C90), C.tinta, Color.White),
    NEGRA(Color(0xFF3A3632), Color(0xFF1D1B18), Color.Black, C.papel, Color(0x2EFFFFFF)),
    NARANJA(Color(0xFFFF8A5C), Color(0xFFF2541A), Color(0xFF9A3209), C.tinta, Color(0x73FFFFFF)),
}

/**
 * Una tecla mecánica: una cara con degradé sobre un canto de 6 dp que se ve
 * abajo. Al apretarla, la cara baja 5 dp y el canto casi desaparece.
 */
@Composable
fun Tecla(
    texto: String?,
    alTocar: () -> Unit,
    modifier: Modifier = Modifier,
    acabado: Acabado = Acabado.BLANCA,
    chica: Boolean = false,
    cuadrada: Boolean = false,
    habilitada: Boolean = true,
    apretada: Boolean = false,
    descripcion: String? = null,
    alto: Dp = if (cuadrada) 44.dp else if (chica) 40.dp else 52.dp,
    radio: Dp = if (chica || cuadrada) 11.dp else 15.dp,
    contenido: (@Composable RowScope.() -> Unit)? = null,
) {
    val fuente = remember { MutableInteractionSource() }
    val apretando by fuente.collectIsPressedAsState()
    val baja by animateDpAsState(if ((apretando && habilitada) || apretada) 5.dp else 0.dp, tween(80), label = "tecla")
    Box(
        modifier
            .alpha(if (habilitada) 1f else 0.45f)
            .defaultMinSize(minWidth = if (cuadrada) 44.dp else 0.dp)
            .height(alto + 6.dp)
            .semantics { descripcion?.let { contentDescription = it } }
            .clickable(fuente, indication = null, enabled = habilitada, role = Role.Button, onClick = alTocar)
            .drawBehind {
                val r = CornerRadius(radio.toPx())
                val h = alto.toPx()
                val y = baja.toPx()
                // La sombra blanda y el canto.
                drawRoundRect(Color(0x55000000), Offset(0f, 9.dp.toPx()), Size(size.width, h), r)
                drawRoundRect(acabado.canto, Offset(0f, 6.dp.toPx()), Size(size.width, h), r)
                // La cara.
                drawRoundRect(Brush.verticalGradient(listOf(acabado.arriba, acabado.abajo), startY = y, endY = y + h), Offset(0f, y), Size(size.width, h), r)
                drawRoundRect(Color(0x1F000000), Offset(0f, y + h - 3.dp.toPx()), Size(size.width, 3.dp.toPx()), r)
                drawLine(acabado.brillo, Offset(radio.toPx(), y + 1f), Offset(size.width - radio.toPx(), y + 1f), 1.dp.toPx())
            },
    ) {
        Row(
            Modifier.offset(y = baja).height(alto).padding(horizontal = if (cuadrada) 0.dp else if (chica) 14.dp else 22.dp).align(Alignment.TopCenter),
            horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (contenido != null) contenido()
            else if (texto != null) Text(
                texto.uppercase(), style = if (cuadrada) T.tecla.copy(fontSize = 18.sp) else if (chica) T.teclaChica else T.tecla,
                color = acabado.letra, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center,
            )
        }
    }
}

/** Un LED: apagado, prendido (naranja), verde o ámbar, con su brillo. */
@Composable
fun Led(color: Color?, modifier: Modifier = Modifier, tamano: Dp = 9.dp) {
    Canvas(modifier.size(tamano)) {
        val r = size.minDimension / 2
        if (color != null) drawCircle(Brush.radialGradient(listOf(color.copy(alpha = 0.55f), Color.Transparent), center, r * 2.4f), r * 2.4f)
        drawCircle(color ?: C.ledApagado, r)
    }
}

/** Un título grabado en la chapa. */
@Composable
fun Grabado(texto: String, modifier: Modifier = Modifier, color: Color = C.gris) {
    Text(texto.uppercase(), style = T.grabado, color = color, modifier = modifier, maxLines = 2, overflow = TextOverflow.Ellipsis)
}

/** Un visor hundido en la chapa (con su sombra de adentro). */
@Composable
fun Visor(modifier: Modifier = Modifier, contenido: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier.clip(RoundedCornerShape(18.dp)).background(C.hundido)
            .drawBehind { drawRect(Brush.verticalGradient(listOf(Color(0xCC000000), Color.Transparent), endY = 10.dp.toPx())) }
            .padding(horizontal = 18.dp, vertical = 14.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
        content = contenido,
    )
}

/** La placa naranja anodizada, con su trama de puntos. */
@Composable
fun PlacaNaranja(modifier: Modifier = Modifier, radio: Dp = 30.dp, relleno: Dp = 16.dp, contenido: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier.clip(RoundedCornerShape(radio)).drawWithCache {
            val paso = 7.dp.toPx()
            val puntos = buildList {
                var y = paso / 2
                while (y < size.height) {
                    var x = paso / 2
                    while (x < size.width) { add(Offset(x, y)); x += paso }
                    y += paso
                }
            }
            val degrade = Brush.verticalGradient(0f to C.placaArriba, 0.55f to C.placaMedio, 1f to C.placaAbajo)
            onDrawBehind {
                drawRect(degrade)
                drawPoints(puntos, PointMode.Points, Color(0x24000000), 2.dp.toPx(), StrokeCap.Round)
                drawLine(Color(0x59FFFFFF), Offset(radio.toPx(), 1f), Offset(size.width - radio.toPx(), 1f), 2.dp.toPx())
            }
        }.padding(relleno),
        verticalArrangement = Arrangement.spacedBy(16.dp),
        content = contenido,
    )
}

/** Chapa oscura: las tarjetas del dock y los módulos del laboratorio. */
@Composable
fun Chapa(modifier: Modifier = Modifier, radio: Dp = 28.dp, relleno: Dp = 18.dp, contenido: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier.clip(RoundedCornerShape(radio))
            .background(Brush.verticalGradient(listOf(C.chapaAlta, C.chapaBaja)))
            .drawBehind { drawLine(Color(0x1AFFFFFF), Offset(radio.toPx(), 1f), Offset(size.width - radio.toPx(), 1f), 1.dp.toPx()) }
            .padding(relleno),
        verticalArrangement = Arrangement.spacedBy(14.dp),
        content = contenido,
    )
}

/** Una bandeja hundida en la placa naranja. */
@Composable
fun Bandeja(modifier: Modifier = Modifier, contenido: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier.fillMaxWidth().clip(RoundedCornerShape(22.dp)).background(Color(0x1F1A1714))
            .drawBehind { drawRect(Brush.verticalGradient(listOf(Color(0x33000000), Color.Transparent), endY = 6.dp.toPx())) }
            .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        content = contenido,
    )
}

/** Un interruptor con LED: prendido o apagado. */
@Composable
fun Llave(texto: String, prendida: Boolean, alTocar: () -> Unit, modifier: Modifier = Modifier, habilitada: Boolean = true) {
    Row(
        modifier.alpha(if (habilitada) 1f else 0.45f).height(44.dp).clip(RoundedCornerShape(12.dp)).background(C.hundido)
            .clickable(enabled = habilitada, role = Role.Switch, onClick = alTocar).padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Led(if (prendida) C.led else null)
        Text(texto.uppercase(), style = T.teclaChica, color = if (prendida) C.papel else C.grisClaro, maxLines = 1)
    }
}

/** Un aviso en una línea, con su LED (rojo si es un error). */
@Composable
fun AvisoLinea(texto: String, modifier: Modifier = Modifier, error: Boolean = false) {
    Row(
        modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Led(if (error) C.rojo else C.led)
        Text(texto, style = T.dato, color = C.papel)
    }
}

/** El fondo de la consola: un brillo arriba que se pierde en lo oscuro. */
fun Modifier.fondoConsola() = this.background(C.fondo).drawBehind {
    drawRect(Brush.radialGradient(listOf(C.fondoAlto, C.fondo), Offset(size.width * 0.6f, 0f), size.width * 1.2f))
}

/** Un hueco cuadrado con un + naranja (una bahía libre). */
@Composable
fun BoxScope.Mas(tamano: Dp = 64.dp) {
    Box(
        Modifier.align(Alignment.Center).size(tamano).clip(RoundedCornerShape(50))
            .background(Brush.verticalGradient(listOf(Color(0xFFFF8A5C), Color(0xFFF2541A)))),
        contentAlignment = Alignment.Center,
    ) { Text("+", style = T.titular.copy(fontSize = 34.sp), color = C.tinta) }
}
