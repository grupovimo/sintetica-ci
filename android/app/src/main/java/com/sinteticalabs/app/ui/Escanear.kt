package com.sinteticalabs.app.ui

import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import com.sinteticalabs.app.protocolo.DatosQr
import com.sinteticalabs.app.protocolo.leerQr
import java.util.concurrent.Executors

@Composable
fun PantallaEscanear(alLeer: (DatosQr) -> Unit, alVolver: () -> Unit) {
    val p = LocalPaleta.current
    val permiso = conPermisos(PERMISOS_CAMARA)
    var aviso by remember { mutableStateOf<String?>(null) }
    Surface(color = p.papel, modifier = Modifier.fillMaxSize()) {
        Column(Modifier.safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Etiqueta("Vincular")
            Text("Escaneá el QR de su pantalla.", style = Tipos.titulo, color = p.tinta)
            Box(
                Modifier.fillMaxWidth().aspectRatio(1f).clip(RoundedCornerShape(24.dp)).background(p.pantalla)
                    .border(1.dp, p.linea, RoundedCornerShape(24.dp)),
                contentAlignment = Alignment.Center,
            ) {
                if (permiso) Camara { texto ->
                    val datos = leerQr(texto)
                    if (datos != null) alLeer(datos) else aviso = "Ese QR no es de un robot."
                } else Text("Para escanear hace falta la cámara.", style = Tipos.chico, color = p.tintaSuave, modifier = Modifier.padding(24.dp))
            }
            aviso?.let { Aviso(it) }
            Text("¿Sin QR? Apretá su botón 5 s.", style = Tipos.chico, color = p.tintaSuave)
            BotonSecundario("Volver", alVolver, Modifier.fillMaxWidth())
        }
    }
}

@Composable
private fun Camara(alLeer: (String) -> Unit) {
    val contexto = LocalContext.current
    val ciclo = LocalLifecycleOwner.current
    val vista = remember { PreviewView(contexto) }
    val hilo = remember { Executors.newSingleThreadExecutor() }
    var leido by remember { mutableStateOf(false) }
    DisposableEffect(Unit) {
        val futuro = ProcessCameraProvider.getInstance(contexto)
        val lector = BarcodeScanning.getClient(BarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build())
        futuro.addListener({
            val proveedor = futuro.get()
            val vistaPrevia = Preview.Builder().build().also { it.surfaceProvider = vista.surfaceProvider }
            val analisis = ImageAnalysis.Builder().setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST).build()
            analisis.setAnalyzer(hilo) { imagen ->
                val media = imagen.image
                if (media == null || leido) { imagen.close(); return@setAnalyzer }
                lector.process(InputImage.fromMediaImage(media, imagen.imageInfo.rotationDegrees))
                    .addOnSuccessListener { codigos ->
                        codigos.firstNotNullOfOrNull { it.rawValue }?.let { t ->
                            if (!leido) { leido = leerQr(t) != null; alLeer(t) }
                        }
                    }
                    .addOnCompleteListener { imagen.close() }
            }
            proveedor.unbindAll()
            proveedor.bindToLifecycle(ciclo, CameraSelector.DEFAULT_BACK_CAMERA, vistaPrevia, analisis)
        }, ContextCompat.getMainExecutor(contexto))
        onDispose {
            runCatching { futuro.get().unbindAll() }
            lector.close()
            hilo.shutdown()
        }
    }
    AndroidView(factory = { vista }, modifier = Modifier.fillMaxSize())
}
