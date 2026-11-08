package com.sinteticalabs.bicho

import android.graphics.ImageFormat
import android.graphics.SurfaceTexture
import android.hardware.Camera
import android.os.SystemClock
import android.util.Log
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.face.FaceDetection
import com.google.mlkit.vision.face.FaceDetectorOptions

/**
 * La cámara de adelante como ojos: busca una cara con ML Kit (el modelo viene
 * en la app, sin internet) a baja resolución y como mucho 5 veces por segundo.
 * Cada cuadro se analiza en memoria y se tira: nada se guarda ni sale del celu.
 *
 * [alVer] recibe dónde está la cara, de -1 a 1 en ejes de la pantalla (x a la
 * derecha, y hacia arriba), o null si no hay nadie.
 */
@Suppress("DEPRECATION")
class Ojos(private val alVer: (Pair<Float, Float>?) -> Unit) {
    private var camara: Camera? = null
    private val textura = SurfaceTexture(10)
    private var ultimo = 0L
    @Volatile private var ocupado = false
    private val detector = FaceDetection.getClient(
        FaceDetectorOptions.Builder()
            .setPerformanceMode(FaceDetectorOptions.PERFORMANCE_MODE_FAST)
            .setMinFaceSize(0.15f)
            .build(),
    )

    fun prender() {
        if (camara != null) return
        val frente = (0 until Camera.getNumberOfCameras()).firstOrNull {
            Camera.CameraInfo().also { i -> Camera.getCameraInfo(it, i) }.facing == Camera.CameraInfo.CAMERA_FACING_FRONT
        } ?: return
        val c = runCatching { Camera.open(frente) }.getOrElse {
            Log.w("Bicho", "ojos: ${it.message}")
            return
        }
        val p = c.parameters
        val tam = p.supportedPreviewSizes.filter { it.width >= 320 }.minByOrNull { it.width * it.height }
            ?: p.supportedPreviewSizes.first()
        p.setPreviewSize(tam.width, tam.height)
        p.previewFormat = ImageFormat.NV21
        p.supportedPreviewFpsRange.minByOrNull { it[0] }?.let { p.setPreviewFpsRange(it[0], it[1]) }
        c.parameters = p
        c.setPreviewTexture(textura)
        val buffer = ByteArray(tam.width * tam.height * 3 / 2)
        c.addCallbackBuffer(buffer)
        c.setPreviewCallbackWithBuffer { datos, cam ->
            val ahora = SystemClock.elapsedRealtime()
            if (ocupado || ahora - ultimo < PERIODO_MS) {
                cam.addCallbackBuffer(datos)
                return@setPreviewCallbackWithBuffer
            }
            ultimo = ahora
            ocupado = true
            mirar(datos, tam.width, tam.height) { cam.addCallbackBuffer(datos) }
        }
        c.startPreview()
        camara = c
        Log.i("Bicho", "ojos: ${tam.width}×${tam.height}")
    }

    fun apagar() {
        camara?.run {
            setPreviewCallbackWithBuffer(null)
            stopPreview()
            release()
        }
        camara = null
    }

    private fun mirar(nv21: ByteArray, w: Int, h: Int, devolver: () -> Unit) {
        // La cámara de adelante del celu apaisado: el sensor ya está "acostado",
        // así que la imagen va sin rotar. La imagen de la cámara de adelante ya
        // viene espejada: x crece hacia la derecha de la pantalla tal como se ve
        // (probado en el Moto E4: alguien que se corre a su derecha da x > 0).
        val img = InputImage.fromByteArray(nv21, w, h, 0, InputImage.IMAGE_FORMAT_NV21)
        detector.process(img)
            .addOnSuccessListener { caras ->
                val c = caras.maxByOrNull { it.boundingBox.width() }
                alVer(c?.let {
                    val x = it.boundingBox.exactCenterX() / w * 2f - 1f
                    val y = -(it.boundingBox.exactCenterY() / h * 2f - 1f)
                    x.coerceIn(-1f, 1f) to y.coerceIn(-1f, 1f)
                })
            }
            .addOnCompleteListener {
                ocupado = false
                devolver()
            }
    }

    private companion object {
        const val PERIODO_MS = 200L // 5 por segundo
    }
}
