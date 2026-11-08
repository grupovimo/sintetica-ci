package com.sinteticalabs.bicho

import android.Manifest
import android.app.Activity
import android.app.admin.DevicePolicyManager
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.hardware.Sensor
import android.hardware.SensorEvent
import android.hardware.SensorEventListener
import android.hardware.SensorManager
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import android.opengl.GLSurfaceView
import android.os.BatteryManager
import android.util.Log
import android.view.Surface
import kotlin.math.ln

/**
 * Los sentidos del celu, convertidos a lo que entiende el núcleo: el
 * acelerómetro (en g y en ejes de la pantalla), la batería y el cargador, el
 * oído (solo el nivel) y la luz (el brillo de la pantalla).
 * Todo lo que va al núcleo pasa por el hilo de GL.
 */
class Sentidos(private val a: Activity, private val vista: GLSurfaceView) : SensorEventListener {
    private val sensores = a.getSystemService(Context.SENSOR_SERVICE) as SensorManager
    private var oyendo: Thread? = null
    @Volatile private var escuchar = false
    private var enchufado: Boolean? = null
    private var brillo = -1f

    /** Lo último que pasó, para la depuración. */
    @Volatile var temperatura = 0f
        private set
    @Volatile var nivelOido = 0f
        private set
    @Volatile var lux = -1f
        private set
    /** Dónde está la cara que ve (de -1 a 1), o null. */
    @Volatile var cara: Pair<Float, Float>? = null
        private set

    private var sinCaraDesde = 0L
    private val ojos = Ojos { c -> ver(c) }

    fun prender() {
        sensores.getDefaultSensor(Sensor.TYPE_ACCELEROMETER)?.let {
            sensores.registerListener(this, it, SensorManager.SENSOR_DELAY_GAME)
        }
        sensores.getDefaultSensor(Sensor.TYPE_LIGHT)?.let {
            sensores.registerListener(this, it, SensorManager.SENSOR_DELAY_NORMAL)
        }
        a.registerReceiver(bateria, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
        oir()
        if (permiso(Manifest.permission.CAMERA)) ojos.prender()
    }

    fun apagar() {
        sensores.unregisterListener(this)
        ojos.apagar()
        runCatching { a.unregisterReceiver(bateria) }
        escuchar = false
        oyendo?.join(500)
        oyendo = null
    }

    override fun onSensorChanged(e: SensorEvent) {
        when (e.sensor.type) {
            Sensor.TYPE_ACCELEROMETER -> {
                val (x, y) = aPantalla(e.values[0], e.values[1], rotacion())
                val g = SensorManager.GRAVITY_EARTH
                vista.queueEvent { Nucleo.imu(x / g, y / g, e.values[2] / g) }
            }
            Sensor.TYPE_LIGHT -> luz(e.values[0])
        }
    }

    override fun onAccuracyChanged(s: Sensor?, precision: Int) = Unit

    @Suppress("DEPRECATION")
    private fun rotacion() = a.windowManager.defaultDisplay.rotation

    /** Brillo según la luz: de noche casi apagado, con sol a pleno. Cambia de a poco. */
    private fun luz(valor: Float) {
        lux = valor
        val objetivo = (0.04f + ln(1f + valor) / ln(1f + 400f)).coerceIn(0.04f, 1f)
        brillo = if (brillo < 0) objetivo else brillo + (objetivo - brillo) * 0.2f
        val w = a.window
        w.attributes = w.attributes.apply { screenBrightness = brillo }
    }

    private val bateria = object : BroadcastReceiver() {
        override fun onReceive(c: Context, i: Intent) {
            val nivel = i.getIntExtra(BatteryManager.EXTRA_LEVEL, -1)
            val escala = i.getIntExtra(BatteryManager.EXTRA_SCALE, 100)
            temperatura = i.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, 0) / 10f
            if (nivel >= 0) Nucleo.bateria(nivel * 100 / escala)
            // Provisorio hasta que la API C tenga hal.cargando: el evento a mano.
            val ahora = i.getIntExtra(BatteryManager.EXTRA_PLUGGED, 0) != 0
            if (enchufado != null && enchufado != ahora) evento(if (ahora) "enchufado" else "desenchufado")
            enchufado = ahora
        }
    }

    /**
     * Lo que ven los ojos. Provisorio hasta que el núcleo tenga "cara-detectada"
     * y una entrada para la mirada: por ahora solo se registra.
     */
    private fun ver(c: Pair<Float, Float>?) {
        val ahora = android.os.SystemClock.elapsedRealtime()
        if (c != null && cara == null) {
            val ausente = if (sinCaraDesde == 0L) 0L else ahora - sinCaraDesde
            Log.i("Bicho", "ojos: te veo (después de ${ausente / 1000} s)")
            if (ausente > 30_000) evento("cara-detectada")
        }
        if (c == null && cara != null) sinCaraDesde = ahora
        cara = c
    }

    private fun evento(nombre: String) = vista.queueEvent { Nucleo.eventoPorNombre(nombre) }

    /** El oído en su hilo: tramos de 50 ms que se miden y se tiran. */
    private fun oir() {
        if (!permiso(Manifest.permission.RECORD_AUDIO)) {
            Log.w("Bicho", "sin permiso de micrófono")
            return
        }
        escuchar = true
        oyendo = Thread({
            val tasa = 16000
            val tramo = ShortArray(tasa / 20)
            val minimo = AudioRecord.getMinBufferSize(tasa, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT)
            val mic = runCatching {
                AudioRecord(MediaRecorder.AudioSource.MIC, tasa, AudioFormat.CHANNEL_IN_MONO,
                    AudioFormat.ENCODING_PCM_16BIT, maxOf(minimo, tramo.size * 4))
            }.getOrNull()
            if (mic == null || mic.state != AudioRecord.STATE_INITIALIZED) {
                Log.w("Bicho", "no pude abrir el micrófono")
                return@Thread
            }
            val oido = Oido()
            mic.startRecording()
            while (escuchar) {
                val n = mic.read(tramo, 0, tramo.size)
                if (n <= 0) continue
                val oyo = oido.tramo(tramo, n)
                tramo.fill(0)
                nivelOido = oido.nivel
                if (oyo != null) {
                    Log.i("Bicho", "oído: ${oyo.nombre}")
                    evento(oyo.nombre)
                }
            }
            mic.stop()
            mic.release()
        }, "oido").apply { start() }
    }

    /** Siendo dueña del dispositivo, se da el permiso sola; si no, lo pide. */
    private fun permiso(cual: String): Boolean {
        if (a.checkSelfPermission(cual) == PackageManager.PERMISSION_GRANTED) return true
        if (Kiosco.esDueno(a)) {
            val d = a.getSystemService(Context.DEVICE_POLICY_SERVICE) as DevicePolicyManager
            runCatching {
                d.setPermissionGrantState(ComponentName(a, Admin::class.java), a.packageName,
                    cual, DevicePolicyManager.PERMISSION_GRANT_STATE_GRANTED)
            }
            return a.checkSelfPermission(cual) == PackageManager.PERMISSION_GRANTED
        }
        a.requestPermissions(arrayOf(cual), 1)
        return false
    }

    companion object {
        /**
         * Del sistema del sensor (el del celu parado, en vertical) a la pantalla
         * tal como se ve: x a la derecha, y hacia arriba.
         */
        fun aPantalla(x: Float, y: Float, rotacion: Int): Pair<Float, Float> = when (rotacion) {
            Surface.ROTATION_90 -> Pair(-y, x)
            Surface.ROTATION_180 -> Pair(-x, -y)
            Surface.ROTATION_270 -> Pair(y, -x)
            else -> Pair(x, y)
        }
    }
}
