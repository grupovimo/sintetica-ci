package com.sinteticalabs.app.datos

import android.annotation.SuppressLint
import android.app.PendingIntent
import android.bluetooth.le.ScanFilter
import android.companion.AssociationInfo
import android.companion.AssociationRequest
import android.companion.BluetoothLeDeviceFilter
import android.companion.CompanionDeviceManager
import android.companion.CompanionDeviceService
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentSender
import android.os.Build
import android.os.Handler
import android.os.Looper
import androidx.annotation.RequiresApi
import com.google.android.gms.location.Geofence
import com.google.android.gms.location.GeofencingEvent
import com.google.android.gms.location.GeofencingRequest
import com.google.android.gms.location.LocationServices
import com.google.android.gms.location.Priority
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlin.coroutines.resume
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Lo que el celu del dueño le cuenta a sus robots (v1.9): cuándo llegás y te
 * vas (la cercanía del robot por Bluetooth, con CompanionDeviceManager: el
 * sistema avisa solo y casi no gasta batería; de respaldo, una geocerca de
 * casa) y qué música suena. Todo opcional y apagado por defecto.
 *
 * Las claves de los robots: su id (por la nube) o "cerca:<id>" (por BLE,
 * mientras su consola está conectada).
 */
object Celu {
    private fun prefs(c: Context) = c.getSharedPreferences("celu", Context.MODE_PRIVATE)
    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    // --- Qué robots quieren qué ---
    fun quiere(c: Context, que: String, robot: String) = prefs(c).getStringSet(que, emptySet())!!.contains(robot)
    fun robots(c: Context, que: String): Set<String> = prefs(c).getStringSet(que, emptySet())!!
    fun poner(c: Context, que: String, robot: String, si: Boolean) {
        val s = robots(c, que).toMutableSet()
        if (si) s += robot else s -= robot
        prefs(c).edit().putStringSet(que, s).apply()
    }

    /** Le manda algo a un robot: por la nube, la ruta; de cerca, el marco por BLE si está conectado. */
    fun mandar(c: Context, robot: String, ruta: String, cuerpo: kotlinx.serialization.json.JsonObject, marco: ByteArray) {
        if (robot.startsWith("cerca:")) {
            val cliente = AvisosCelu.cerca[robot.removePrefix("cerca:")] ?: return
            alcance.launch(Dispatchers.Main) { runCatching { cliente.escribirCrudo(com.sinteticalabs.app.protocolo.Car.COMANDO, marco) } }
            return
        }
        alcance.launch {
            val cuenta = Cuenta(c.applicationContext)
            val s = cuenta.actual()
            if (s.activa) runCatching { cuenta.api(s).pedir("/v1/bichos/$robot/$ruta", "POST", cuerpo) }
        }
    }

    /** Llegaste o te fuiste: a todos los robots que lo quieren. */
    fun presencia(c: Context, llegaste: Boolean, solo: Set<String>? = null) {
        val ultima = prefs(c).getInt("presencia-ultima", -1)
        val ahora = if (llegaste) 0 else 1
        val t = prefs(c).getLong("presencia-en", 0)
        // Bluetooth y la geocerca pueden avisar lo mismo: una vez cada 2 min.
        if (ultima == ahora && System.currentTimeMillis() - t < 120_000) return
        prefs(c).edit().putInt("presencia-ultima", ahora).putLong("presencia-en", System.currentTimeMillis()).apply()
        for (r in solo ?: robots(c, "presencia")) {
            mandar(c, r, "presencia", buildJsonObject { put("llegaste", llegaste) }, com.sinteticalabs.app.protocolo.Comando.presencia(llegaste))
        }
    }

    /** Lo que suena: a los robots que quieren la música del celu. */
    fun musica(c: Context, sonando: Boolean, titulo: String, artista: String) {
        for (r in robots(c, "musica")) {
            mandar(c, r, "musica", buildJsonObject { put("sonando", sonando); put("titulo", titulo); put("artista", artista) },
                com.sinteticalabs.app.protocolo.Comando.musica(0, sonando, titulo, artista))
        }
    }

    // --- Bluetooth: CompanionDeviceManager ---

    /** El id del robot (12 hex) en los datos del anuncio BLE (fabricante 0xFFFF). */
    fun bytesDeId(id: String): ByteArray? = id.takeIf { it.length == 12 && it.all { c -> c.isLetterOrDigit() } }
        ?.chunked(2)?.map { it.toInt(16).toByte() }?.toByteArray()

    fun disponibleBluetooth() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.S

    /**
     * Pide asociar el robot (el sistema muestra su diálogo). `lanzar` recibe el
     * IntentSender para abrirlo; al volver, `asociado` empieza a observarlo.
     */
    @Suppress("DEPRECATION")
    fun asociar(c: Context, robot: String, idBle: String, lanzar: (IntentSender) -> Unit, alError: (String) -> Unit) {
        val datos = bytesDeId(idBle) ?: return alError("Sin su id")
        val cdm = c.getSystemService(CompanionDeviceManager::class.java) ?: return alError("Este celu no puede")
        val filtro = BluetoothLeDeviceFilter.Builder().setScanFilter(ScanFilter.Builder().setManufacturerData(0xFFFF, datos).build()).build()
        val pedido = AssociationRequest.Builder().addDeviceFilter(filtro).setSingleDevice(true).build()
        prefs(c).edit().putString("asociando", robot).apply()
        cdm.associate(pedido, object : CompanionDeviceManager.Callback() {
            @Deprecated("Deprecated in Java")
            override fun onDeviceFound(intentSender: IntentSender) = lanzar(intentSender)
            override fun onAssociationPending(intentSender: IntentSender) = lanzar(intentSender)
            override fun onFailure(error: CharSequence?) = alError(error?.toString() ?: "No lo encontré")
        }, Handler(Looper.getMainLooper()))
    }

    /** Después del diálogo: la asociación nueva se observa (el sistema avisa cuando aparece o se va). */
    @SuppressLint("MissingPermission")
    @Suppress("DEPRECATION")
    fun asociado(c: Context) {
        if (!disponibleBluetooth()) return
        val robot = prefs(c).getString("asociando", null) ?: return
        val cdm = c.getSystemService(CompanionDeviceManager::class.java) ?: return
        val mac = cdm.associations.lastOrNull() ?: return
        prefs(c).edit().putString("mac:$mac", robot).remove("asociando").apply()
        poner(c, "presencia", robot, true)
        runCatching { cdm.startObservingDevicePresence(mac) }
    }

    @SuppressLint("MissingPermission")
    @Suppress("DEPRECATION")
    fun desasociar(c: Context, robot: String) {
        val cdm = c.getSystemService(CompanionDeviceManager::class.java) ?: return
        for ((k, v) in prefs(c).all) if (k.startsWith("mac:") && v == robot) {
            val mac = k.removePrefix("mac:")
            if (disponibleBluetooth()) runCatching { cdm.stopObservingDevicePresence(mac) }
            runCatching { cdm.disassociate(mac) }
            prefs(c).edit().remove(k).apply()
        }
    }

    fun asociadoA(c: Context, robot: String) = prefs(c).all.any { (k, v) -> k.startsWith("mac:") && v == robot }
    fun robotDeMac(c: Context, mac: String) = prefs(c).getString("mac:${mac.uppercase()}", null) ?: prefs(c).getString("mac:$mac", null)

    // --- La geocerca de casa (de respaldo) ---

    fun conGeocerca(c: Context) = prefs(c).getBoolean("geocerca", false)

    private fun intentGeocerca(c: Context) = PendingIntent.getBroadcast(
        c, 0, Intent(c, GeocercaCasa::class.java), PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE,
    )

    /** Pone la casa donde está el celu ahora (150 m alrededor). Hace falta el permiso de ubicación (también en segundo plano). */
    @SuppressLint("MissingPermission")
    suspend fun ponerGeocerca(c: Context): String? = withContext(Dispatchers.Main) {
        try {
            val f = LocationServices.getFusedLocationProviderClient(c)
            val l = kotlinx.coroutines.suspendCancellableCoroutine<android.location.Location?> { k ->
                f.getCurrentLocation(Priority.PRIORITY_BALANCED_POWER_ACCURACY, null)
                    .addOnSuccessListener { k.resume(it) }.addOnFailureListener { k.resume(null) }
            } ?: return@withContext "No sé dónde estás"
            val g = Geofence.Builder().setRequestId("casa").setCircularRegion(l.latitude, l.longitude, 150f)
                .setExpirationDuration(Geofence.NEVER_EXPIRE)
                .setTransitionTypes(Geofence.GEOFENCE_TRANSITION_ENTER or Geofence.GEOFENCE_TRANSITION_EXIT).build()
            val pedido = GeofencingRequest.Builder().setInitialTrigger(0).addGeofence(g).build()
            LocationServices.getGeofencingClient(c).addGeofences(pedido, intentGeocerca(c))
            prefs(c).edit().putBoolean("geocerca", true).apply()
            null
        } catch (e: Exception) {
            e.message ?: "No pude"
        }
    }

    fun sacarGeocerca(c: Context) {
        runCatching { LocationServices.getGeofencingClient(c).removeGeofences(intentGeocerca(c)) }
        prefs(c).edit().putBoolean("geocerca", false).apply()
    }
}

/** El sistema avisa cuando el robot aparece o se va (por Bluetooth, sin una conexión abierta). */
@RequiresApi(Build.VERSION_CODES.S)
class PresenciaCompanera : CompanionDeviceService() {
    @Deprecated("Deprecated in Java")
    override fun onDeviceAppeared(address: String) = avisar(address, true)

    @Deprecated("Deprecated in Java")
    override fun onDeviceDisappeared(address: String) = avisar(address, false)

    @RequiresApi(Build.VERSION_CODES.TIRAMISU)
    override fun onDeviceAppeared(associationInfo: AssociationInfo) {
        associationInfo.deviceMacAddress?.toString()?.let { avisar(it, true) }
    }

    @RequiresApi(Build.VERSION_CODES.TIRAMISU)
    override fun onDeviceDisappeared(associationInfo: AssociationInfo) {
        associationInfo.deviceMacAddress?.toString()?.let { avisar(it, false) }
    }

    private fun avisar(mac: String, llegaste: Boolean) {
        val robot = Celu.robotDeMac(this, mac) ?: return
        Celu.presencia(this, llegaste, setOf(robot))
    }
}

/** La geocerca de casa: entrar es llegar, salir es irte. */
class GeocercaCasa : BroadcastReceiver() {
    override fun onReceive(c: Context, intent: Intent) {
        val e = GeocercaDe(intent) ?: return
        when (e) {
            Geofence.GEOFENCE_TRANSITION_ENTER -> Celu.presencia(c, true)
            Geofence.GEOFENCE_TRANSITION_EXIT -> Celu.presencia(c, false)
        }
    }

    private fun GeocercaDe(i: Intent): Int? = GeofencingEvent.fromIntent(i)?.takeIf { !it.hasError() }?.geofenceTransition
}
