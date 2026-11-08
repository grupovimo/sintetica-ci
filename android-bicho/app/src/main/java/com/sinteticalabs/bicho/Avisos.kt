package com.sinteticalabs.bicho

import android.app.Notification
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import android.util.Log

/**
 * Las notificaciones del propio celu, como el comando Aviso (0x44): la app y
 * quién (el título). Del texto de la notificación no se lee nada.
 * El permiso se da una vez por adb (ver el README).
 */
class Avisos : NotificationListenerService() {
    override fun onNotificationPosted(n: StatusBarNotification) {
        if (n.packageName == packageName || n.isOngoing) return
        val extras = n.notification.extras
        val llamada = n.notification.category == Notification.CATEGORY_CALL
        val app = runCatching {
            packageManager.getApplicationLabel(packageManager.getApplicationInfo(n.packageName, 0)).toString()
        }.getOrDefault(n.packageName)
        val quien = extras.getCharSequence(Notification.EXTRA_TITLE)?.toString() ?: ""
        Log.i("Bicho", "aviso de $app")
        alAviso?.invoke(comando(if (llamada) 1 else 0, app, quien))
    }

    companion object {
        /** Lo pone el bicho: recibe el comando armado y lo pasa al núcleo. */
        @Volatile var alAviso: ((ByteArray) -> Unit)? = null

        /** [0x44][tipo][n][app][n][contacto], cada texto de hasta 24 bytes de UTF-8. */
        fun comando(tipo: Int, app: String, contacto: String): ByteArray {
            val a = recortar(app)
            val c = recortar(contacto)
            return byteArrayOf(0x44, tipo.toByte(), a.size.toByte()) + a + byteArrayOf(c.size.toByte()) + c
        }

        /** Hasta 24 bytes sin partir una letra. */
        fun recortar(texto: String, maximo: Int = 24): ByteArray {
            var b = texto.toByteArray(Charsets.UTF_8)
            if (b.size <= maximo) return b
            var n = maximo
            while (n > 0 && (b[n].toInt() and 0xC0) == 0x80) n--
            b = b.copyOf(n)
            return b
        }
    }
}
