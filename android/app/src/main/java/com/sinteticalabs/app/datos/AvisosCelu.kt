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

/**
 * Escucha las notificaciones y le pasa al robot las que le importan (comando
 * 0x44 Aviso, por la nube o por BLE): las de pago, mail, GitHub y pedidos
 * (la misma tabla que el robot-celu) y las que coinciden con sus avisos.
 * Con el mismo permiso lee qué música suena (la sesión de medios), si se pidió.
 */
class EscuchaAvisos : NotificationListenerService() {
    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val ultimos = mutableMapOf<String, Long>()
    private var medios: android.media.session.MediaSessionManager? = null
    private var controlador: android.media.session.MediaController? = null
    private var ultimaMusica: Triple<Boolean, String, String>? = null

    private val alCambiarSesiones = android.media.session.MediaSessionManager.OnActiveSessionsChangedListener { elegir(it) }
    private val alCambiarTema = object : android.media.session.MediaController.Callback() {
        override fun onMetadataChanged(m: android.media.MediaMetadata?) = musica()
        override fun onPlaybackStateChanged(s: android.media.session.PlaybackState?) = musica()
        override fun onSessionDestroyed() = elegir(null)
    }

    override fun onListenerConnected() {
        val m = getSystemService(android.media.session.MediaSessionManager::class.java) ?: return
        val yo = ComponentName(this, EscuchaAvisos::class.java)
        runCatching {
            m.addOnActiveSessionsChangedListener(alCambiarSesiones, yo)
            medios = m
            elegir(m.getActiveSessions(yo))
        }
    }

    override fun onListenerDisconnected() {
        runCatching { medios?.removeOnActiveSessionsChangedListener(alCambiarSesiones) }
        controlador?.unregisterCallback(alCambiarTema)
        controlador = null
    }

    /** Sigue la sesión que suena (o la primera). */
    private fun elegir(sesiones: List<android.media.session.MediaController>?) {
        val lista = sesiones ?: runCatching { medios?.getActiveSessions(ComponentName(this, EscuchaAvisos::class.java)) }.getOrNull() ?: emptyList()
        val nueva = lista.firstOrNull { it.playbackState?.state == android.media.session.PlaybackState.STATE_PLAYING } ?: lista.firstOrNull()
        if (nueva?.sessionToken != controlador?.sessionToken) {
            controlador?.unregisterCallback(alCambiarTema)
            controlador = nueva
            nueva?.registerCallback(alCambiarTema)
        }
        musica()
    }

    private fun musica() {
        if (Celu.robots(this, "musica").isEmpty()) return
        val c = controlador
        val sonando = c?.playbackState?.state == android.media.session.PlaybackState.STATE_PLAYING
        val m = c?.metadata
        val ahora = Triple(sonando, m?.getString(android.media.MediaMetadata.METADATA_KEY_TITLE).orEmpty(), m?.getString(android.media.MediaMetadata.METADATA_KEY_ARTIST).orEmpty())
        if (ahora == ultimaMusica) return
        // Sin música y sin haber sonado nada: no hay nada que avisar.
        if (!sonando && ultimaMusica == null) return
        ultimaMusica = ahora
        Celu.musica(this, ahora.first, ahora.second, ahora.third)
    }

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
        val tipo = com.sinteticalabs.app.consola.Compania.tipoAviso(sbn.packageName, llamada)
        val conTipo = tipo in setOf("pago", "mail", "github", "pedido")
        // Los de pago, mail, GitHub y pedidos tienen su reacción: van a todos los que escuchan; los demás, si coinciden con sus avisos.
        val destino = robots.filter { (_, l) -> conTipo || Acompanar.coincide(l, llamada, app, contacto) }.keys
        if (destino.isEmpty()) return
        val (a, k) = com.sinteticalabs.app.consola.Compania.camposAviso(tipo, app, contacto)
        val ca = com.sinteticalabs.app.consola.cortarBytes(a, 24)
        val ck = com.sinteticalabs.app.consola.cortarBytes(k, 24)
        for (r in destino) {
            Celu.mandar(this, r, "aviso", buildJsonObject { put("tipo", tipo); put("app", ca); put("contacto", ck) },
                com.sinteticalabs.app.protocolo.Comando.aviso(tipo, ca, ck))
        }
    }

    override fun onDestroy() {
        alcance.cancel()
        super.onDestroy()
    }
}
