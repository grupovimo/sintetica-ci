package com.sinteticalabs.bicho

import android.opengl.GLSurfaceView
import android.util.Base64
import android.util.Log
import org.eclipse.paho.client.mqttv3.IMqttDeliveryToken
import org.eclipse.paho.client.mqttv3.MqttCallbackExtended
import org.eclipse.paho.client.mqttv3.MqttClient
import org.eclipse.paho.client.mqttv3.MqttConnectOptions
import org.eclipse.paho.client.mqttv3.MqttMessage
import org.eclipse.paho.client.mqttv3.persist.MemoryPersistence
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.net.URI
import java.security.SecureRandom
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

/**
 * El bicho en la red, como el bicho virtual de la nube (api/cmd/bicho-virtual):
 * - Una puerta HTTP en el puerto 8090 con /info, /ble y /mostrar-qr: lo que en
 *   una placa es BLE. La API (en desarrollo) lo suma desde la web.
 * - MQTT a la nube con las credenciales que le dejaron por esa puerta.
 * Todo lo que toca el núcleo pasa al hilo de GL.
 */
class Red(private val raiz: File) {
    lateinit var vista: GLSurfaceView

    /** La serie: 12 hex al azar la primera vez (una MAC "administrada localmente"). */
    val id: String by lazy {
        val f = File(raiz, "id")
        f.takeIf { it.exists() }?.readText()?.trim()?.takeIf { it.matches(Regex("[0-9A-F]{12}")) } ?: run {
            val mac = ByteArray(6).also { SecureRandom().nextBytes(it) }
            mac[0] = (mac[0].toInt() and 0xFC or 0x02).toByte()
            mac.joinToString("") { "%02X".format(it) }.also { f.writeText(it) }
        }
    }

    // Lo que notifica el núcleo mientras hay un "celu conectado" por /ble.
    private var capturadas: MutableList<Pair<Int, ByteArray>>? = null
    private val cliente = AtomicReference<MqttClient?>()
    @Volatile var conectado = false
        private set
    private val cambio = Object()
    @Volatile private var hayCambio = true

    fun prender() {
        Thread(::atender, "puerta").apply { isDaemon = true }.start()
        Thread(::mantenerNube, "nube").apply { isDaemon = true }.start()
    }

    // --- Lo que llama el núcleo (hilo de GL) ---

    fun notificar(car: Int, datos: ByteArray) {
        capturadas?.add(car to datos.copyOf())
    }

    fun notificarNube(car: Int, datos: ByteArray) {
        val c = cliente.get() ?: return
        if (!conectado) return
        val d = datos.copyOf()
        Thread {
            runCatching {
                if (car == CAR_ESTADO) c.publish("bicho/$id/state", d, 1, true)
                else c.publish("bicho/$id/resp", byteArrayOf(car.toByte()) + d, 0, false)
            }
        }.start()
    }

    /** Lo que llega por bicho/{id}/audio y bicho/{id}/conversacion/url (hilo de MQTT). */
    @Volatile var alAudio: ((ByteArray) -> Unit)? = null
    @Volatile var alConversacion: ((String) -> Unit)? = null
    @Volatile var alPaquete: ((String) -> Unit)? = null
    /** Recién conectado a la nube (y suscripto). */
    @Volatile var alConectar: (() -> Unit)? = null

    /** Si está conectado a la nube. */
    val enLinea get() = conectado && cliente.get() != null

    /** Un JSON a un tópico propio de la nube (bicho/{id}/{cual}: presencia, charla); si no hay conexión, se pierde. */
    fun publicar(cual: String, json: String) {
        val c = cliente.get() ?: return
        if (!conectado) return
        Thread { runCatching { c.publish("bicho/$id/$cual", json.toByteArray(), 1, false) } }.start()
    }

    fun redCambio() {
        synchronized(cambio) {
            hayCambio = true
            cambio.notifyAll()
        }
    }

    /** Corre [f] en el hilo de GL y espera el resultado. */
    private fun <T> enGl(f: () -> T): T? {
        val r = AtomicReference<T?>()
        val listo = CountDownLatch(1)
        vista.queueEvent {
            try {
                r.set(f())
            } finally {
                listo.countDown()
            }
        }
        listo.await(5, TimeUnit.SECONDS)
        return r.get()
    }

    // --- La puerta HTTP ---

    private fun atender() {
        val s = runCatching { ServerSocket(PUERTO) }.getOrElse {
            Log.w("Bicho", "puerta: ${it.message}")
            return
        }
        Log.i("Bicho", "puerta abierta en :$PUERTO, serie $id")
        while (true) {
            val c = runCatching { s.accept() }.getOrNull() ?: continue
            Thread { runCatching { pedido(c) }.onFailure { Log.w("Bicho", "puerta: ${it.message}") }; c.close() }.start()
        }
    }

    private fun pedido(c: Socket) {
        c.soTimeout = 15_000
        val ent = c.getInputStream().buffered()
        val primera = linea(ent) ?: return
        var largo = 0
        while (true) {
            val l = linea(ent) ?: return
            if (l.isEmpty()) break
            if (l.lowercase().startsWith("content-length:")) largo = l.substringAfter(':').trim().toIntOrNull() ?: 0
        }
        if (largo > 1 shl 20) return responder(c, 413, JSONObject())
        val cuerpo = ByteArray(largo)
        var leidos = 0
        while (leidos < largo) {
            val n = ent.read(cuerpo, leidos, largo - leidos)
            if (n < 0) break
            leidos += n
        }
        val (metodo, ruta) = primera.split(' ').let { (it.getOrNull(0) ?: "") to (it.getOrNull(1) ?: "") }
        when ("$metodo $ruta") {
            "GET /info" -> responder(c, 200, info())
            "POST /ble" -> {
                recordarReclamante(c.inetAddress)
                responder(c, 200, ble(JSONObject(String(cuerpo))))
            }
            "POST /mostrar-qr" -> responder(c, 200, mostrarQr())
            else -> responder(c, 404, JSONObject().put("error", "no existe"))
        }
    }

    private fun linea(e: java.io.InputStream): String? {
        val b = StringBuilder()
        while (true) {
            val x = e.read()
            if (x < 0) return if (b.isEmpty()) null else b.toString()
            if (x == '\n'.code) return b.toString().trimEnd('\r')
            b.append(x.toChar())
        }
    }

    private fun responder(c: Socket, codigo: Int, json: JSONObject) {
        val cuerpo = json.toString().toByteArray()
        val salida = c.getOutputStream()
        salida.write(("HTTP/1.1 $codigo ${if (codigo == 200) "OK" else "Error"}\r\n" +
            "Content-Type: application/json\r\nContent-Length: ${cuerpo.size}\r\nConnection: close\r\n\r\n").toByteArray())
        salida.write(cuerpo)
        salida.flush()
    }

    private fun info(): JSONObject {
        val (estado, qr) = enGl { Nucleo.estado() to Nucleo.qr() } ?: (ByteArray(0) to "")
        val emparejando = estado.size > 4 && estado[4].toInt() and 4 != 0
        return JSONObject().put("id", id).put("perfil", PERFIL).put("qr", qr)
            .put("emparejando", emparejando).put("nube", conectado).put("version", Nucleo.version())
    }

    /** Una conexión "BLE" entera: conectar, escribir todo, desconectar; devuelve lo notificado. */
    private fun ble(p: JSONObject): JSONObject {
        val escrituras = p.optJSONArray("escrituras") ?: JSONArray()
        val notis = enGl {
            val l = mutableListOf<Pair<Int, ByteArray>>()
            capturadas = l
            Nucleo.bleConexion(true)
            for (i in 0 until escrituras.length()) {
                val e = escrituras.getJSONObject(i)
                Nucleo.bleEscribir(e.getInt("car"), Base64.decode(e.optString("datos"), Base64.DEFAULT))
            }
            Nucleo.bleConexion(false)
            capturadas = null
            l
        } ?: emptyList()
        val a = JSONArray()
        for ((car, d) in notis) a.put(JSONObject().put("car", car).put("datos", Base64.encodeToString(d, Base64.NO_WRAP)))
        return JSONObject().put("notificaciones", a)
    }

    /** Como mantener apretado 5 s: muestra el QR para vincular. */
    private fun mostrarQr(): JSONObject {
        val qr = enGl {
            Nucleo.mostrarQr()
            Nucleo.qr()
        }
        return JSONObject().put("qr", qr ?: "")
    }

    // --- La nube (MQTT) ---

    private fun recordarReclamante(a: InetAddress) = File(raiz, "reclamante").writeText(a.hostAddress ?: "")

    /**
     * En desarrollo la API le pasa la dirección de EMQX de adentro de Docker
     * ("emqx"), que el celu no conoce: en ese caso usa la compu que lo sumó.
     */
    /**
     * Una URL de la nube que el celu pueda abrir: en la nube local, la API dice
     * "localhost" y eso es la Mac que lo sumó (el reclamante), no el celu.
     */
    fun alcanzable(url: String): String {
        val u = runCatching { URI(url) }.getOrNull() ?: return url
        if (u.host != "localhost" && u.host != "127.0.0.1") return url
        val reclamante = File(raiz, "reclamante").takeIf { it.exists() }?.readText()?.trim()
        if (reclamante.isNullOrEmpty()) return url
        return URI(u.scheme, u.userInfo, reclamante, u.port, u.path, u.query, u.fragment).toString()
    }

    private fun broker(url: String): String {
        val u = URI(url.replace("mqtts://", "ssl://").replace("mqtt://", "tcp://"))
        val host = u.host ?: return url
        // "localhost" es la Mac de la nube local (quien lo sumó), no el celu.
        val alcanzable = host != "localhost" && host != "127.0.0.1" &&
            runCatching { InetAddress.getByName(host); true }.getOrDefault(false)
        val reclamante = File(raiz, "reclamante").takeIf { it.exists() }?.readText()?.trim()
        val h = if (!alcanzable && !reclamante.isNullOrEmpty()) reclamante else host
        return "${u.scheme}://$h:${if (u.port > 0) u.port else 1883}"
    }

    private fun mantenerNube() {
        // El núcleo arranca con la superficie de GL: hasta entonces no sabe su nube.
        while (!Nucleo.listo()) Thread.sleep(200)
        while (true) {
            synchronized(cambio) {
                while (!hayCambio) cambio.wait()
                hayCambio = false
            }
            cliente.getAndSet(null)?.let { runCatching { it.disconnectForcibly(300) }; it.close() }
            conectado = false
            enGl { Nucleo.conexiones(true, false) }
            Thread.sleep(500) // que salga antes la respuesta del comando
            val n = enGl { Nucleo.nube() }
            if (n == null || n[1].isEmpty()) {
                Log.i("Bicho", "sin nube: esperando que me sumen a una cuenta")
                continue
            }
            conectar(broker(n[0]), n[1], n[2])
        }
    }

    private fun suscribir(c: MqttClient, cual: String, alLlegar: (ByteArray) -> Unit) = runCatching {
        c.subscribe("bicho/$id/$cual", 1) { _, m: MqttMessage -> alLlegar(m.payload) }
    }.onFailure { Log.w("Bicho", "nube: sin bicho/$id/$cual (${(it as? org.eclipse.paho.client.mqttv3.MqttException)?.reasonCode})") }

    private fun conectar(broker: String, usuario: String, clave: String) {
        val c = MqttClient(broker, id, MemoryPersistence())
        val conexion = "bicho/$id/conexion"
        c.setCallback(object : MqttCallbackExtended {
            override fun connectComplete(reconexion: Boolean, uri: String) {
                Thread {
                    runCatching {
                        c.subscribe("bicho/$id/cmd", 1) { _, m: MqttMessage ->
                            val p = m.payload
                            if (p.size >= 2) vista.queueEvent { Nucleo.nubeEscribir(p[0].toInt() and 0xFF, p.copyOfRange(1, p.size)) }
                        }
                        c.publish(conexion, "1".toByteArray(), 1, true)
                        conectado = true
                        // La voz de la nube (los mp3, la conversación y el paquete). Una nube vieja
                        // (sin estos tópicos en la ACL) no tiene que dejarlo sin nube: de a uno.
                        suscribir(c, "audio") { alAudio?.invoke(it) }
                        suscribir(c, "conversacion/url") { alConversacion?.invoke(String(it, Charsets.UTF_8)) }
                        suscribir(c, "voz/paquete/url") { alPaquete?.invoke(String(it, Charsets.UTF_8)) }
                        alConectar?.invoke()
                        val estado = enGl {
                            Nucleo.conexiones(true, true)
                            Nucleo.estado()
                        }
                        if (estado != null) c.publish("bicho/$id/state", estado, 1, true)
                        Log.i("Bicho", "conectado a la nube ($uri)")
                    }.onFailure { Log.w("Bicho", "nube: ${it.message}") }
                }.start()
            }

            override fun connectionLost(causa: Throwable?) {
                Log.w("Bicho", "se cortó la nube: ${causa?.message}")
                conectado = false
                vista.queueEvent { Nucleo.conexiones(true, false) }
            }

            override fun messageArrived(t: String?, m: MqttMessage?) = Unit
            override fun deliveryComplete(t: IMqttDeliveryToken?) = Unit
        })
        val o = MqttConnectOptions().apply {
            userName = usuario
            password = clave.toCharArray()
            isAutomaticReconnect = true
            isCleanSession = true
            keepAliveInterval = 30
            connectionTimeout = 10
            setWill(conexion, "0".toByteArray(), 1, true)
        }
        cliente.set(c)
        Thread {
            // Reintenta hasta conectar; después Paho reconecta solo.
            while (cliente.get() === c && !c.isConnected) {
                runCatching { c.connect(o) }.onFailure {
                    Log.w("Bicho", "nube: no me pude conectar a $broker (${it.message})")
                    Thread.sleep(5000)
                }
            }
        }.start()
    }

    companion object {
        const val PUERTO = 8090
        const val PERFIL = "celu"
        private const val CAR_ESTADO = 3
    }
}
