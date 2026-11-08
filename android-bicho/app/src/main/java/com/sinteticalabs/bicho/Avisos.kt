package com.sinteticalabs.bicho

import android.app.Notification
import android.content.ComponentName
import android.content.Context
import android.media.MediaMetadata
import android.media.session.MediaController
import android.media.session.MediaSessionManager
import android.media.session.PlaybackState
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import android.util.Log

/**
 * Las notificaciones del propio celu, como el comando Aviso (0x44): la app y
 * quién (el título). Del texto de la notificación no se lee nada.
 * Con el mismo permiso lee qué música suena (la sesión de medios) y lo manda
 * como el comando Música (0x4B).
 * El permiso se da una vez por adb (ver el README).
 */
class Avisos : NotificationListenerService() {
    private var medios: MediaSessionManager? = null
    private var controlador: MediaController? = null
    private var ultimaMusica: ByteArray? = null

    private val alCambiarSesiones = MediaSessionManager.OnActiveSessionsChangedListener { elegir(it) }
    private val alCambiarTema = object : MediaController.Callback() {
        override fun onMetadataChanged(m: MediaMetadata?) = musica()
        override fun onPlaybackStateChanged(s: PlaybackState?) = musica()
        override fun onSessionDestroyed() = elegir(null)
    }

    override fun onListenerConnected() {
        val m = getSystemService(Context.MEDIA_SESSION_SERVICE) as MediaSessionManager
        val yo = ComponentName(this, Avisos::class.java)
        runCatching {
            m.addOnActiveSessionsChangedListener(alCambiarSesiones, yo)
            medios = m
            elegir(m.getActiveSessions(yo))
        }.onFailure { Log.w("Bicho", "música: sin acceso a la sesión de medios", it) }
    }

    override fun onListenerDisconnected() {
        medios?.removeOnActiveSessionsChangedListener(alCambiarSesiones)
        controlador?.unregisterCallback(alCambiarTema)
        controlador = null
    }

    /** Sigue la sesión que suena (o la primera). */
    private fun elegir(sesiones: List<MediaController>?) {
        val lista = sesiones ?: runCatching {
            medios?.getActiveSessions(ComponentName(this, Avisos::class.java))
        }.getOrNull() ?: emptyList()
        val nueva = lista.firstOrNull { it.playbackState?.state == PlaybackState.STATE_PLAYING } ?: lista.firstOrNull()
        if (nueva?.sessionToken != controlador?.sessionToken) {
            controlador?.unregisterCallback(alCambiarTema)
            controlador = nueva
            nueva?.registerCallback(alCambiarTema)
        }
        musica()
    }

    private fun musica() {
        val c = controlador
        val sonando = c?.playbackState?.state == PlaybackState.STATE_PLAYING
        val m = c?.metadata
        val titulo = m?.getString(MediaMetadata.METADATA_KEY_TITLE) ?: ""
        val artista = m?.getString(MediaMetadata.METADATA_KEY_ARTIST) ?: ""
        val cmd = comandoMusica(sonando, titulo, artista)
        if (cmd.contentEquals(ultimaMusica)) return
        // Sin música y sin haber sonado nada: no hay nada que avisar.
        if (!sonando && ultimaMusica == null) return
        ultimaMusica = cmd
        Log.i("Bicho", "música: ${if (sonando) "suena" else "paró"}")
        alAviso?.invoke(cmd)
    }

    override fun onNotificationPosted(n: StatusBarNotification) {
        if (n.packageName == packageName || n.isOngoing) return
        val extras = n.notification.extras
        val app = runCatching {
            packageManager.getApplicationLabel(packageManager.getApplicationInfo(n.packageName, 0)).toString()
        }.getOrDefault(n.packageName)
        val quien = extras.getCharSequence(Notification.EXTRA_TITLE)?.toString() ?: ""
        val tipo = tipo(n.packageName, n.notification.category).let { if (it > 1 && !TIPOS_V19) 0 else it }
        Log.i("Bicho", "aviso de $app (tipo $tipo)")
        alAviso?.invoke(
            when (tipo) {
                // Pago: el monto del título como "app" (si no lo dice, la app).
                2 -> comando(2, monto(quien) ?: app, "")
                // Mail: la app y quién lo manda (el título de la notificación).
                3 -> comando(3, app, quien)
                // GitHub y pedidos: el título corto (el de la notificación).
                4, 5 -> comando(tipo, quien, "")
                else -> comando(tipo, app, quien)
            }
        )
    }

    companion object {
        /** Lo pone el bicho: recibe el comando armado y lo pasa al núcleo. */
        @Volatile var alAviso: ((ByteArray) -> Unit)? = null

        /** Los tipos de aviso 2 a 6 los entiende el núcleo desde la v1.9; antes van como notificación. */
        const val TIPOS_V19 = true

        private val PAGOS = setOf("com.mercadopago.wallet", "com.uala", "ar.com.bancar.uala", "com.naranjax.app")
        private val MAILS = setOf("com.google.android.gm", "com.microsoft.office.outlook", "com.google.android.gm.lite")
        private val PEDIDOS = setOf("com.mercadolibre", "com.pedidosya", "com.grability.rappi")

        /** El tipo de Aviso (0x44) según la app: 1 llamada, 2 pago, 3 mail, 4 github, 5 pedido, 0 el resto. */
        fun tipo(paquete: String, categoria: String?) = when {
            categoria == Notification.CATEGORY_CALL -> 1
            paquete in PAGOS -> 2
            paquete in MAILS -> 3
            paquete == "com.github.android" -> 4
            paquete in PEDIDOS -> 5
            else -> 0
        }

        /** "$12.000" de "Recibiste $12.000 de Ana", o null. */
        fun monto(texto: String) = Regex("\\$\\s?[0-9][0-9.,]*").find(texto)?.value?.replace(" ", "")

        /** [0x44][tipo][n][app][n][contacto], cada texto de hasta 24 bytes de UTF-8. */
        fun comando(tipo: Int, app: String, contacto: String): ByteArray {
            val a = recortar(app)
            val c = recortar(contacto)
            return byteArrayOf(0x44, tipo.toByte(), a.size.toByte()) + a + byteArrayOf(c.size.toByte()) + c
        }

        /** [0x4B][bpm = 0, no se sabe][flags: bit 0 sonando][n][título ≤ 40][n][artista ≤ 24]. */
        fun comandoMusica(sonando: Boolean, titulo: String, artista: String): ByteArray {
            val t = recortar(titulo, 40)
            val a = recortar(artista)
            return byteArrayOf(0x4B, 0, if (sonando) 1 else 0, t.size.toByte()) + t + byteArrayOf(a.size.toByte()) + a
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
