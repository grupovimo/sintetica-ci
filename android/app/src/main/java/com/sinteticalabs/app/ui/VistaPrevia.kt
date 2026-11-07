package com.sinteticalabs.app.ui

import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.nucleo.Nucleo

/**
 * La cara del bicho dibujada por el mismo núcleo C++ que corre en la placa,
 * a 640 × 344 (el doble de la pantalla de 320 × 172, con la misma geometría).
 */
@Composable
fun VistaPrevia(descripcion: String, alApretar: (Boolean) -> Unit, modifier: Modifier = Modifier) {
    val p = LocalPaleta.current
    val ancho = 640
    val alto = 344
    val bitmap = remember { Bitmap.createBitmap(ancho, alto, Bitmap.Config.ARGB_8888) }
    val buffer = remember { IntArray(ancho * alto) }
    var cuadro by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) {
        while (true) {
            withFrameNanos {
                Nucleo.tick()
                Nucleo.pixeles(buffer)
                bitmap.setPixels(buffer, 0, ancho, 0, 0, ancho, alto)
                cuadro++
            }
        }
    }
    Box(modifier.fillMaxWidth().background(p.carcasa, RoundedCornerShape(24.dp)).padding(14.dp)) {
        @Suppress("UNUSED_EXPRESSION") cuadro  // lee el estado para redibujar en cada cuadro
        Image(
            bitmap = bitmap.asImageBitmap(), contentDescription = null, contentScale = ContentScale.FillWidth,
            filterQuality = FilterQuality.High,
            modifier = Modifier.fillMaxWidth().aspectRatio(ancho / alto.toFloat()).clip(RoundedCornerShape(10.dp))
                .semantics { contentDescription = descripcion }
                .pointerInput(Unit) {
                    detectTapGestures(onPress = {
                        alApretar(true)
                        tryAwaitRelease()
                        alApretar(false)
                    })
                },
        )
    }
}
