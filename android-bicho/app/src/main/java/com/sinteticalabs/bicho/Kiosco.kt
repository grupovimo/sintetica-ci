package com.sinteticalabs.bicho

import android.app.Activity
import android.app.admin.DeviceAdminReceiver
import android.app.admin.DevicePolicyManager
import android.app.admin.SystemUpdatePolicy
import android.bluetooth.BluetoothAdapter
import android.media.AudioManager
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.BatteryManager
import android.provider.Settings
import android.util.Log

/** El receptor de administrador: la app es dueña del dispositivo (ver README). */
class Admin : DeviceAdminReceiver()

/** Al prender el celu abre el bicho (además de ser la pantalla de inicio). */
class Arranque : BroadcastReceiver() {
    override fun onReceive(contexto: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return
        contexto.startActivity(Intent(contexto, Bicho::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }
}

/**
 * El celu como bicho y nada más: si la app es dueña del dispositivo, queda
 * como pantalla de inicio fija, sin barra de estado ni pantalla de bloqueo,
 * prendida mientras esté enchufada y bloqueada en la app. Sin ser dueña, solo
 * pantalla completa (sirve para desarrollar).
 */
object Kiosco {
    private const val PREFS = "kiosco"

    fun activo(c: Context) = c.getSharedPreferences(PREFS, 0).getBoolean("activo", true)

    private fun admin(c: Context) = ComponentName(c, Admin::class.java)
    private fun dpm(c: Context) = c.getSystemService(Context.DEVICE_POLICY_SERVICE) as DevicePolicyManager
    fun esDueno(c: Context) = dpm(c).isDeviceOwnerApp(c.packageName)

    /** Lo aplica de nuevo cada vez que el bicho vuelve al frente. */
    fun entrar(a: Activity) {
        if (!activo(a) || !esDueno(a)) return
        val d = dpm(a)
        val adm = admin(a)
        runCatching {
            d.setLockTaskPackages(adm, arrayOf(a.packageName))
            val inicio = IntentFilter(Intent.ACTION_MAIN).apply {
                addCategory(Intent.CATEGORY_HOME)
                addCategory(Intent.CATEGORY_DEFAULT)
            }
            d.addPersistentPreferredActivity(adm, inicio, ComponentName(a, Bicho::class.java))
            d.setStatusBarDisabled(adm, true)
            d.setKeyguardDisabled(adm, true)
            val siempre = BatteryManager.BATTERY_PLUGGED_AC or BatteryManager.BATTERY_PLUGGED_USB or
                BatteryManager.BATTERY_PLUGGED_WIRELESS
            d.setGlobalSetting(adm, Settings.Global.STAY_ON_WHILE_PLUGGED_IN, siempre.toString())
            celuDedicado(a, d, adm)
            a.startLockTask()
        }.onFailure { Log.w("Bicho", "kiosco: ${it.message}") }
        a.getSharedPreferences(PREFS, 0).edit().putBoolean("activo", true).apply()
    }

    /**
     * El celu es solo el bicho (ver herramientas/ajustar.sh para lo que va por
     * adb): hora automática, wifi siempre prendido, sin actualizaciones del
     * sistema que lo reinicien, bluetooth apagado (salvo que haya un celu del dueño
     * para la presencia), sin sonidos del sistema y la
     * voz del bicho al máximo (el volumen lo elige él, con voz.volumen).
     */
    @Suppress("DEPRECATION")
    private fun celuDedicado(a: Activity, d: DevicePolicyManager, adm: ComponentName) {
        d.setGlobalSetting(adm, Settings.Global.AUTO_TIME, "1")
        d.setGlobalSetting(adm, Settings.Global.AUTO_TIME_ZONE, "1")
        d.setGlobalSetting(adm, Settings.Global.WIFI_SLEEP_POLICY, Settings.Global.WIFI_SLEEP_POLICY_NEVER.toString())
        d.setSystemUpdatePolicy(adm, SystemUpdatePolicy.createPostponeInstallPolicy())
        runCatching {
            val bt = BluetoothAdapter.getDefaultAdapter()
            if (Presencia.conBluetooth(a)) bt?.takeIf { !it.isEnabled }?.enable()
            else bt?.takeIf { it.isEnabled }?.disable()
        }
        val audio = a.getSystemService(Context.AUDIO_SERVICE) as AudioManager
        for (canal in intArrayOf(AudioManager.STREAM_RING, AudioManager.STREAM_NOTIFICATION, AudioManager.STREAM_SYSTEM))
            runCatching { audio.setStreamVolume(canal, 0, 0) }
        Voz.canales(a, audio)
    }

    /** La salida secreta: suelta todo y abre los ajustes. */
    fun salir(a: Activity) {
        a.getSharedPreferences(PREFS, 0).edit().putBoolean("activo", false).apply()
        runCatching { a.stopLockTask() }
        if (esDueno(a)) runCatching {
            val d = dpm(a)
            val adm = admin(a)
            d.clearPackagePersistentPreferredActivities(adm, a.packageName)
            d.setStatusBarDisabled(adm, false)
            d.setKeyguardDisabled(adm, false)
        }
        runCatching { a.startActivity(Intent(Settings.ACTION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
    }

    /** Deja de ser dueña del dispositivo (para devolver el celu a la normalidad). */
    fun soltar(a: Activity) {
        salir(a)
        if (esDueno(a)) runCatching { dpm(a).clearDeviceOwnerApp(a.packageName) }
    }

    /** Volver al bicho desde el menú o al abrir la app otra vez. */
    fun volver(a: Activity) {
        a.getSharedPreferences(PREFS, 0).edit().putBoolean("activo", true).apply()
        entrar(a)
    }
}
