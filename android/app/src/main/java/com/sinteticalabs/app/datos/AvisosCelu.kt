package com.sinteticalabs.app.datos

import android.app.Notification
import android.app.NotificationManager
import android.content.ComponentName
import android.content.Context
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import com.sinteticalabs.app.consola.Acompanar
import com.sinteticalabs.app.consola.VACIO
import com.sinteticalabs.app.consola.leerObjeto
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Qué robots quieren los avisos de este celu y con qué reglas (las `avisos`
 * de su config). Lo usa el servicio que escucha las notificaciones: solo
 * manda la app y el contacto de las que coinciden, nunca el texto.
 */
object AvisosCelu {
    /** Los robots conectados por BLE ahora (su id en las preferencias es "cerca:<id>"). */
    val cerca = java.util.concurrent.ConcurrentHashMap<String, com.sinteticalabs.app.bicho.ClienteBicho>()
    fun claveCerca(id: String) = "cerca:$id"

    private fun prefs(c: Context) = c.getSharedPreferences("avisos_celu", Context.MODE_PRIVATE)

    fun prendido(c: Context, robot: String) = prefs(c).contains(robot)

    /** Prende (con sus avisos) o apaga un robot. */
    fun poner(c: Context, robot: String, avisos: List<Acompanar.AvisoCelu>?) {
        val e = prefs(c).edit()
        if (avisos == null) e.remove(robot) else e.putString(robot, Acompanar.ponerAvisos(VACIO, avisos).toString())
        e.apply()
    }

    /** Si está prendido, guarda los avisos nuevos. */
    fun actualizar(c: Context, robot: String, avisos: List<Acompanar.AvisoCelu>) { if (prendido(c, robot)) poner(c, robot, avisos) }

    fun activos(c: Context): Map<String, List<Acompanar.AvisoCelu>> =
        prefs(c).all.mapNotNull { (k, v) -> (v as? String)?.let { k to Acompanar.avisos(leerObjeto(it)) } }.toMap()

    /** ¿El usuario le dio a la app el acceso a las notificaciones? */
    fun permiso(c: Context): Boolean =
        c.getSystemService(NotificationManager::class.java)?.isNotificationListenerAccessGranted(ComponentName(c, EscuchaAvisos::class.java)) == true
}

/** Escucha las notificaciones y le pasa al robot las que le importan (comando 0x44 Aviso, por la nube). */
class EscuchaAvisos : NotificationListenerService() {
    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val ultimos = mutableMapOf<String, Long>()

    override fun onNotificationPosted(sbn: StatusBarNotification) {
        if (sbn.packageName == packageName || sbn.isOngoing) return
        val n = sbn.notification ?: return
        if (n.flags and Notification.FLAG_GROUP_SUMMARY != 0) return
        val robots = AvisosCelu.activos(this)
        if (robots.isEmpty()) return
        val app = runCatching { packageManager.getApplicationLabel(packageManager.getApplicationInfo(sbn.packageName, 0)).toString() }.getOrDefault(sbn.packageName)
        val contacto = n.extras?.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty()
        val llamada = n.category == Notification.CATEGORY_CALL
        // La misma notificación se actualiza seguido: una vez cada 15 s.
        val clave = "${sbn.packageName}|$contacto|$llamada"
        val ahora = System.currentTimeMillis()
        if ((ultimos[clave] ?: 0) > ahora - 15_000) return
        ultimos[clave] = ahora
        val destino = robots.filter { (_, l) -> Acompanar.coincide(l, llamada, app, contacto) }.keys
        if (destino.isEmpty()) return
        val a = com.sinteticalabs.app.consola.cortarBytes(app, 24)
        val k = com.sinteticalabs.app.consola.cortarBytes(contacto, 24)
        // Los que están cerca: el 0x44 directo por BLE.
        for (r in destino.filter { it.startsWith("cerca:") }) {
            val cliente = AvisosCelu.cerca[r.removePrefix("cerca:")] ?: continue
            alcance.launch(Dispatchers.Main) {
                runCatching { cliente.escribirCrudo(com.sinteticalabs.app.protocolo.Car.COMANDO, com.sinteticalabs.app.protocolo.Comando.aviso(llamada, a, k)) }
            }
        }
        val porNube = destino.filter { !it.startsWith("cerca:") }
        if (porNube.isEmpty()) return
        alcance.launch {
            val cuenta = Cuenta(this@EscuchaAvisos)
            val s = cuenta.actual()
            if (!s.activa) return@launch
            val api = cuenta.api(s)
            for (r in porNube) runCatching {
                api.pedir("/v1/bichos/$r/aviso", "POST", buildJsonObject {
                    put("tipo", if (llamada) "llamada" else "notificacion")
                    put("app", com.sinteticalabs.app.consola.cortarBytes(app, 24))
                    put("contacto", com.sinteticalabs.app.consola.cortarBytes(contacto, 24))
                })
            }
        }
    }

    override fun onDestroy() {
        alcance.cancel()
        super.onDestroy()
    }
}
