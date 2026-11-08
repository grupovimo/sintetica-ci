package com.sinteticalabs.bicho

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothClass
import android.bluetooth.BluetoothDevice
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.wifi.WifiManager
import android.os.Handler
import android.os.HandlerThread
import android.os.SystemClock
import android.util.Log
import java.io.File
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress

/**
 * Si el dueño está en casa: su celu en el wifi (la tabla ARP) o cerca por el
 * Bluetooth emparejado. Al llegar o irse manda la Presencia (0x49) al núcleo.
 * Qué celu es: el teléfono emparejado (si hay uno solo) o por adb:
 *   adb shell am broadcast -a sintetica.EVENT --es tipo duenio --es wifi AA:BB:CC:DD:EE:FF --es bt 11:22:33:44:55:66
 */
class Presencia(private val contexto: Context, private val alCambiar: (llegaste: Boolean) -> Unit) {
    private val hilo = HandlerThread("presencia").apply { start() }
    private val cola = Handler(hilo.looper)
    private val estado = Presente()
    private val prefs = contexto.getSharedPreferences("presencia", 0)
    private val bt: BluetoothAdapter? = BluetoothAdapter.getDefaultAdapter()

    private val alBluetooth = object : BroadcastReceiver() {
        override fun onReceive(c: Context, i: Intent) {
            val d = i.getParcelableExtra<BluetoothDevice>(BluetoothDevice.EXTRA_DEVICE) ?: return
            if (!d.address.equals(celuBt(), ignoreCase = true)) return
            if (i.action == BluetoothDevice.ACTION_ACL_CONNECTED) cola.post { visto("bluetooth") }
        }
    }

    fun prender() {
        contexto.registerReceiver(alBluetooth, IntentFilter(BluetoothDevice.ACTION_ACL_CONNECTED), null, cola)
        cola.post(vuelta)
    }

    fun apagar() {
        runCatching { contexto.unregisterReceiver(alBluetooth) }
        hilo.quitSafely()
    }

    /** Lo que llega por adb (tipo duenio). */
    fun configurar(wifi: String?, bluetooth: String?) {
        prefs.edit().apply {
            if (wifi != null) putString("wifi", wifi.lowercase())
            if (bluetooth != null) putString("bt", bluetooth.uppercase())
        }.apply()
        Log.i("Bicho", "presencia: wifi ${celuWifi() ?: "-"}, bt ${celuBt() ?: "-"}")
    }

    private val vuelta = object : Runnable {
        override fun run() {
            runCatching { mirar() }.onFailure { Log.w("Bicho", "presencia", it) }
            cola.postDelayed(this, VUELTA_MS)
        }
    }

    private fun mirar() {
        celuWifi()?.let { mac ->
            barrerRed()
            // Le damos un momento al ARP para contestar.
            SystemClock.sleep(1500)
            if (Presente.enArp(File("/proc/net/arp").readText(), mac)) visto("wifi")
        }
        // Un pedido de SDP arma el enlace si el celu está cerca (y se ve como ACL_CONNECTED).
        celuBt()?.let { if (bt?.isEnabled == true) bt.getRemoteDevice(it).fetchUuidsWithSdp() }
        estado.tick(SystemClock.elapsedRealtime())?.let(::avisar)
    }

    private fun visto(por: String) {
        estado.visto(SystemClock.elapsedRealtime())?.let {
            Log.i("Bicho", "presencia: por $por")
            avisar(it)
        }
    }

    private fun avisar(llegaste: Boolean) {
        Log.i("Bicho", "presencia: ${if (llegaste) "llegaste" else "te fuiste"}")
        alCambiar(llegaste)
    }

    private fun celuWifi() = prefs.getString("wifi", null)

    private fun celuBt() = celuBt(contexto)

    /** Un paquetito UDP a cada dirección de la red: el kernel pregunta por ARP quién es. */
    @Suppress("DEPRECATION")
    private fun barrerRed() {
        val wifi = contexto.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        val ip = wifi.connectionInfo?.ipAddress ?: return
        if (ip == 0) return
        DatagramSocket().use { s ->
            for (h in 1..254) {
                val b = byteArrayOf((ip and 0xFF).toByte(), (ip shr 8 and 0xFF).toByte(), (ip shr 16 and 0xFF).toByte(), h.toByte())
                runCatching { s.send(DatagramPacket(ByteArray(1), 1, InetAddress.getByAddress(b), 9)) }
            }
        }
    }

    companion object {
        const val VUELTA_MS = 60_000L

        /** El de adb, o el único teléfono emparejado. */
        fun celuBt(c: Context): String? = c.getSharedPreferences("presencia", 0).getString("bt", null) ?: runCatching {
            BluetoothAdapter.getDefaultAdapter()?.bondedDevices
                ?.filter { it.bluetoothClass?.majorDeviceClass == BluetoothClass.Device.Major.PHONE }
                ?.singleOrNull()?.address
        }.getOrNull()

        /** Si hay un celu del dueño por Bluetooth (entonces el bluetooth queda prendido). */
        fun conBluetooth(c: Context) = c.getSharedPreferences("presencia", 0).getString("bt", null) != null
    }
}

/**
 * Llegó o se fue, sin falsas alarmas: se fue después de 15 min sin verlo. Al
 * arrancar no avisa: si lo ve, ya estaba; si no lo ve en 15 min, no está.
 */
class Presente(private val salidaMs: Long = 15 * 60_000L) {
    private var esta: Boolean? = null
    private var ultimo = -1L
    private var desde = -1L

    /** Lo vio: true si recién llegó. */
    fun visto(ahora: Long): Boolean? {
        ultimo = ahora
        val antes = esta
        esta = true
        return if (antes == false) true else null
    }

    /** Cada tanto: false si se acaba de ir. */
    fun tick(ahora: Long): Boolean? {
        if (desde < 0) desde = ahora
        val sinVerlo = ahora - (if (ultimo >= 0) ultimo else desde)
        if (sinVerlo < salidaMs || esta == false) return null
        val antes = esta
        esta = false
        return if (antes == true) false else null
    }

    companion object {
        /** Si la tabla ARP (/proc/net/arp) tiene esa MAC completa (flags 0x2). */
        fun enArp(tabla: String, mac: String) = tabla.lineSequence().drop(1).any { l ->
            val c = l.trim().split(Regex("\\s+"))
            c.size >= 4 && c[3].equals(mac, ignoreCase = true) && c[2] != "0x0"
        }
    }
}
