package com.sinteticalabs.app.ui.consola

import android.app.Application
import android.graphics.Bitmap
import android.graphics.ImageDecoder
import android.net.Uri
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.sinteticalabs.app.bicho.ClienteBicho
import com.sinteticalabs.app.bicho.Gif
import com.sinteticalabs.app.bicho.PrepararImagen
import com.sinteticalabs.app.bicho.TransporteBle
import com.sinteticalabs.app.bicho.TransporteVirtual
import com.sinteticalabs.app.bicho.buscarBicho
import com.sinteticalabs.app.consola.Aviso
import com.sinteticalabs.app.consola.Editar
import com.sinteticalabs.app.consola.ICONO_IA
import com.sinteticalabs.app.consola.ICONO_RECORDATORIO
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.Recordatorios
import com.sinteticalabs.app.consola.avisosDeMarco
import com.sinteticalabs.app.consola.con
import com.sinteticalabs.app.consola.esEstadoDelSistema
import com.sinteticalabs.app.consola.leerObjeto
import com.sinteticalabs.app.consola.modeloDe
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.si
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.consola.textoDeAviso
import com.sinteticalabs.app.consola.textoDeMcp
import com.sinteticalabs.app.datos.Actualizacion
import com.sinteticalabs.app.datos.ApiSintetica
import com.sinteticalabs.app.datos.CacheImagenes
import com.sinteticalabs.app.datos.Cuenta
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.RobotNube
import com.sinteticalabs.app.datos.Vinculados
import com.sinteticalabs.app.datos.actualizacionDeEvento
import com.sinteticalabs.app.datos.borrarImagen
import com.sinteticalabs.app.datos.comando
import com.sinteticalabs.app.datos.config
import com.sinteticalabs.app.datos.estadoDeJson
import com.sinteticalabs.app.datos.guardarConfig
import com.sinteticalabs.app.datos.imagen
import com.sinteticalabs.app.datos.imagenes
import com.sinteticalabs.app.datos.ponerApodo
import com.sinteticalabs.app.datos.quitar
import com.sinteticalabs.app.datos.robot
import com.sinteticalabs.app.datos.subirImagen
import com.sinteticalabs.app.nucleo.AnfitrionEnMemoria
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Animacion
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.EspacioImagen
import com.sinteticalabs.app.protocolo.Op
import com.sinteticalabs.app.protocolo.Respuesta
import com.sinteticalabs.app.protocolo.crc32
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import com.sinteticalabs.app.protocolo.leerRespuesta
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/** "demo" = probar sin robot (el núcleo corre en el celu), como la Unidad o como el Broche. */
const val ID_DEMO = "demo"
const val ID_DEMO_BROCHE = "demo-broche"
/** "nube:<id>" = un robot de la cuenta: por la API. Sin prefijo: uno vinculado a este celu, por BLE. */
const val PREFIJO_NUBE = "nube:"

/** Por dónde se le habla: la nube (como la web), BLE de cerca o el robot virtual del celu. */
enum class Via { NUBE, CERCA, DEMO }

enum class EstadoGuardado { LISTO, GUARDANDO, GUARDADO, ERROR }

/** Una bahía con su imagen: los cuadros (miniaturas) y cuánto dura cada uno. */
data class Vista(val cuadros: List<Pair<ImageBitmap, Int>>)

/**
 * El robot abierto en la consola: su estado en vivo, su config y lo que se
 * guarda solo (como web/src/lib/bicho.svelte.ts y consola/guardado.svelte.ts).
 * La vista previa es el mismo núcleo del robot corriendo en el celu.
 */
class RobotViewModel(app: Application) : AndroidViewModel(app) {
    var id = ""
        private set
    val via get() = when {
        id.startsWith(ID_DEMO) -> Via.DEMO
        id.startsWith(PREFIJO_NUBE) -> Via.NUBE
        else -> Via.CERCA
    }
    /** El id del robot (sin "nube:"). */
    val idRobot get() = id.removePrefix(PREFIJO_NUBE)

    val robot = MutableStateFlow<RobotNube?>(null)
    val config = MutableStateFlow(JsonObject(emptyMap()))
    val versionConfig = MutableStateFlow(0)
    val error = MutableStateFlow<String?>(null)
    val guardado = MutableStateFlow(EstadoGuardado.LISTO)
    val errorGuardado = MutableStateFlow("")
    val aviso = MutableStateFlow<Aviso?>(null)
    val ota = MutableStateFlow<Actualizacion?>(null)
    val espacios = MutableStateFlow<List<EspacioImagen>>(emptyList())
    val vistas = MutableStateFlow<Map<Int, Vista>>(emptyMap())
    val subiendo = MutableStateFlow(-1)
    val errorImagen = MutableStateFlow<String?>(null)
    /** Sube cada vez que el núcleo de la vista previa se reinicia (para redibujarla). */
    val vistaPrevia = MutableStateFlow(0)

    private val cuenta = Cuenta(app)
    private val cache = CacheImagenes(app)
    private var api: ApiSintetica? = null
    private var cliente: ClienteBicho? = null
    private var pendiente: JsonObject? = null
    private var validez: String? = null
    private var reloj: Job? = null
    private var resultadoConfig: Respuesta.Resultado? = null
    private val crcVistas = mutableMapOf<Int, Long>()
    private val anfitrion = AnfitrionEnMemoria().apply {
        alNotificar = { car, d ->
            if (car == Car.CONFIG.id) (runCatching { leerRespuesta(d) }.getOrNull() as? Respuesta.Resultado)?.let {
                if (!it.ok || it.op == Op.CONFIG_FIN) resultadoConfig = it
            }
        }
    }

    val modelo: StateFlow<Modelo?> = MutableStateFlow<Modelo?>(null).also { m ->
        viewModelScope.launch {
            robot.map { r -> r?.let { modeloDe(it.perfil, it.virtual, it.estado) } }.distinctUntilChanged().collect { m.value = it }
        }
    }

    init {
        Instantaneas.pausado = true
    }

    fun apiNube(): ApiSintetica? = api

    fun abrir(id: String) {
        if (this.id == id) return
        this.id = id
        viewModelScope.launch { conectar() }
    }

    fun reintentar() = viewModelScope.launch { conectar() }

    private suspend fun conectar() {
        error.value = null
        cliente?.cerrar()
        try {
            when (via) {
                Via.NUBE -> {
                    val s = cuenta.actual()
                    if (!s.activa) throw IllegalStateException("Entrá a tu cuenta.")
                    val a = cuenta.api(s)
                    api = a
                    EnVivo.arrancar(cuenta)
                    val r = a.robot(idRobot)
                    robot.value = r
                    iniciarVistaPrevia()
                    recargarConfig()
                    escucharNube()
                    refrescarImagenes()
                }
                Via.DEMO -> {
                    val broche = id == ID_DEMO_BROCHE
                    val c = ClienteBicho(TransporteVirtual(broche = broche), viewModelScope)
                    c.conectar()
                    cliente = c
                    vistaPrevia.value++
                    escucharCliente(c, if (broche) "broche" else "", virtual = true, apodo = "")
                    config.value = leerObjeto(c.pedirConfig())
                    refrescarImagenes()
                }
                Via.CERCA -> {
                    val v = Vinculados(getApplication()).buscar(id) ?: throw IllegalStateException("No está vinculado a este celu.")
                    val cerca = buscarBicho(getApplication(), id)
                    val c = ClienteBicho(TransporteBle(getApplication(), cerca.dispositivo), viewModelScope)
                    c.conectar()
                    c.autenticar(v.clave)
                    cliente = c
                    escucharCliente(c, c.estado.value?.perfil.orEmpty(), virtual = false, apodo = v.nombre)
                    iniciarVistaPrevia()
                    config.value = leerObjeto(c.pedirConfig())
                    aplicarEnVistaPrevia(config.value)
                    refrescarImagenes()
                }
            }
            api = api ?: cuenta.actual().takeIf { it.activa }?.let { cuenta.api(it) }
        } catch (e: ErrorApi) {
            error.value = e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
        } catch (e: Exception) {
            error.value = e.message ?: "No me pude conectar."
        }
    }

    /** La vista previa: un núcleo local con la misma config, del tamaño de su pantalla. */
    fun iniciarVistaPrevia() {
        if (via == Via.DEMO) return
        val r = robot.value
        val p = modeloDe(r?.perfil.orEmpty(), r?.virtual ?: false, r?.estado).pantalla
        Nucleo.iniciar(p.ancho, p.alto, idRobot, anfitrion, redonda = p.redonda)
        Nucleo.vistaPrevia()
        aplicarEnVistaPrevia(config.value)
        fijado = null
        espejar(r?.estado?.expresion.orEmpty())
        vistaPrevia.value++
    }

    /** Aplica una config en la vista previa; devuelve el error del núcleo (el mismo del robot), si no la acepta. */
    fun aplicarEnVistaPrevia(c: JsonObject): String? {
        if (via == Via.DEMO) return null
        resultadoConfig = null
        for (e in escriturasDeConfig(c.toString(), 512)) Nucleo.bleEscribir(Car.CONFIG.id, e)
        val r = resultadoConfig
        return if (r != null && !r.ok) "No la acepta: ${r.detalle.ifEmpty { r.mensaje }}" else null
    }

    private var fijado: String? = null

    /** Lo que se ve en el robot se copia en la vista previa (menos los estados del sistema). */
    fun espejar(expresion: String) {
        if (via == Via.DEMO || esEstadoDelSistema(expresion) || expresion == fijado) return
        fijado = expresion
        Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(expresion))
    }

    /** Fija una cara en la vista previa (el onboarding). */
    fun mostrarEnVistaPrevia(expresion: String) {
        fijado = expresion
        if (via == Via.DEMO) viewModelScope.launch { runCatching { cliente?.fijarExpresion(expresion) } }
        else Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(expresion))
    }

    suspend fun recargarConfig() {
        when (via) {
            Via.NUBE -> {
                val (v, c) = api!!.config(idRobot)
                versionConfig.value = v
                config.value = c
            }
            else -> config.value = leerObjeto(cliente?.pedirConfig() ?: return)
        }
        aplicarEnVistaPrevia(config.value)
    }

    private fun escucharNube() = viewModelScope.launch {
        EnVivo.eventos.collect { e ->
            if (e.robot != idRobot) return@collect
            val r = robot.value ?: return@collect
            when (e.tipo) {
                "estado" -> estadoDeJson(e.datos)?.let { est ->
                    robot.value = r.copy(estado = est)
                    espejar(est.expresion)
                }
                "conexion" -> robot.value = r.copy(conectado = e.datos.si("conectado") ?: r.conectado)
                "sincro" -> robot.value = r.copy(sincroConfig = e.datos.texto("config") ?: r.sincroConfig)
                "config" -> if ((e.datos.numero("version")?.toInt() ?: 0) != versionConfig.value && pendiente == null) runCatching { recargarConfig() }
                "imagenes" -> refrescarImagenes()
                "ota" -> ota.value = actualizacionDeEvento(e.datos)
                "mcp" -> avisar(textoDeMcp(e.datos), ICONO_IA)
                "marco" -> {
                    val car = e.datos.numero("car")?.toInt() ?: return@collect
                    val b = e.datos.texto("datos")?.let { runCatching { java.util.Base64.getDecoder().decode(it) }.getOrNull() } ?: return@collect
                    avisosDeMarco(car, b).forEach { avisar(textoDeAviso(it, Recordatorios.leer(config.value), r.estado?.expresion.orEmpty())) }
                }
            }
        }
    }

    private fun escucharCliente(c: ClienteBicho, perfil: String, virtual: Boolean, apodo: String) {
        fun poner(e: com.sinteticalabs.app.protocolo.Estado) {
            robot.value = RobotNube(idRobot.uppercase(), e.perfil.ifEmpty { perfil }, robot.value?.apodo ?: apodo, virtual, true, null, e, "")
        }
        c.estado.value?.let { poner(it) }
        viewModelScope.launch {
            c.estado.collect { e ->
                if (e == null) return@collect
                poner(e)
                espejar(e.expresion)
            }
        }
        viewModelScope.launch {
            c.marcos.collect { (car, b) ->
                avisosDeMarco(car.id, b).forEach { avisar(textoDeAviso(it, Recordatorios.leer(config.value), robot.value?.estado?.expresion.orEmpty())) }
            }
        }
    }

    fun avisar(texto: String, icono: String = ICONO_RECORDATORIO) {
        val a = Aviso(texto, icono, System.currentTimeMillis())
        aviso.value = a
        viewModelScope.launch {
            delay(a.dura)
            if (aviso.value?.cuando == a.cuando) aviso.value = null
        }
    }

    fun cerrarAviso() { aviso.value = null }

    /**
     * La consola guarda sola: cada cambio se ve al toque en la vista previa y,
     * a los 600 ms, se manda. Si el núcleo dice que no, se vuelve a lo guardado.
     */
    fun cambiar(f: (JsonObject) -> JsonObject) {
        val c = Editar.aV2(f(pendiente ?: config.value))
        pendiente = c
        config.value = c
        guardado.value = EstadoGuardado.GUARDANDO
        errorGuardado.value = ""
        validez = aplicarEnVistaPrevia(c)
        reloj?.cancel()
        reloj = viewModelScope.launch {
            delay(600)
            mandar()
        }
    }

    private suspend fun mandar() {
        val c = pendiente ?: return
        pendiente = null
        val e = validez ?: guardar(c)
        if (pendiente != null) return // llegó otro cambio mientras guardaba: lo manda ese
        if (e != null) {
            guardado.value = EstadoGuardado.ERROR
            errorGuardado.value = e
            runCatching { recargarConfig() }
        } else guardado.value = EstadoGuardado.GUARDADO
    }

    /** Guarda una config entera ya. Devuelve el error o null. */
    suspend fun guardar(c: JsonObject): String? = try {
        when (via) {
            Via.NUBE -> versionConfig.value = api!!.guardarConfig(idRobot, c)
            else -> cliente!!.mandarConfig(c.toString())
        }
        config.value = c
        null
    } catch (e: ErrorApi) {
        e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
    } catch (e: Exception) {
        e.message ?: "No se guardó."
    }

    /** Un comando: una cara ("" la suelta), un evento o reproducir una animación. Devuelve el error o null. */
    suspend fun comando(expresion: String? = null, evento: String? = null, reproducir: String? = null): String? = try {
        when (via) {
            Via.NUBE -> api!!.comando(idRobot, buildMap<String, String> {
                expresion?.let { put("expresion", it) }
                evento?.let { put("evento", it) }
                reproducir?.let { put("reproducir", it) }
            })
            else -> {
                val c = cliente!!
                if (expresion != null) c.fijarExpresion(expresion.ifEmpty { null })
                if (evento != null) c.evento(evento)
                if (reproducir != null) c.reproducir(reproducir)
            }
        }
        null
    } catch (e: ErrorApi) {
        e.message
    } catch (e: Exception) {
        e.message ?: "No pude mandarlo."
    }

    // --- Imágenes ---

    fun refrescarImagenes() = viewModelScope.launch {
        val lista = runCatching {
            when (via) {
                Via.NUBE -> api!!.imagenes(idRobot)
                else -> cliente!!.listarImagenes()
            }
        }.getOrNull() ?: return@launch
        espacios.value = lista
        val v = vistas.value.filterKeys { k -> lista.any { it.espacio == k } }.toMutableMap()
        for (e in lista) {
            if (crcVistas[e.espacio] == e.crc && v[e.espacio] != null) continue
            val bytes = runCatching {
                when (via) {
                    Via.NUBE -> api!!.imagen(idRobot, e.espacio)
                    else -> cache.mandada(id, e.espacio)?.takeIf { crc32(it) == e.crc }
                }
            }.getOrNull() ?: continue
            val vista = withContext(Dispatchers.Default) { decodificar(bytes) } ?: continue
            v[e.espacio] = vista
            crcVistas[e.espacio] = e.crc
        }
        vistas.value = v
    }

    private fun decodificar(bytes: ByteArray): Vista? {
        val cuadros = Animacion.cuadros(bytes).take(Animacion.MAX_CUADROS).mapNotNull { c ->
            val b = runCatching { PrepararImagen.decodificar(c.qoi) }.getOrNull() ?: return@mapNotNull null
            val ancho = 200
            val chica = if (b.width > ancho) Bitmap.createScaledBitmap(b, ancho, (b.height * ancho / b.width).coerceAtLeast(1), true).also { b.recycle() } else b
            chica.asImageBitmap() to c.ms
        }
        return if (cuadros.isEmpty()) null else Vista(cuadros)
    }

    /** Prepara una foto, un GIF o un video corto a la pantalla y lo sube a una bahía. */
    fun subirImagen(espacio: Int, uri: Uri, mostrarla: Boolean = true) = viewModelScope.launch {
        subiendo.value = espacio
        errorImagen.value = null
        try {
            val qoi = preparar(uri)
            subir(espacio, qoi)
            if (mostrarla && !Editar.enLista(config.value, espacio) && Editar.protector(config.value) != espacio) cambiar { Editar.mostrar(it, espacio, true) }
            refrescarImagenes()
        } catch (e: ErrorApi) {
            errorImagen.value = e.message
        } catch (e: Exception) {
            errorImagen.value = e.message ?: "No pude prepararla."
        } finally {
            subiendo.value = -1
        }
    }

    suspend fun preparar(uri: Uri): ByteArray = withContext(Dispatchers.Default) {
        val m = modelo.value ?: modeloDe("", false, null)
        val p = m.pantalla
        val contexto = getApplication<Application>()
        val tipo = contexto.contentResolver.getType(uri).orEmpty()
        if (tipo.startsWith("video/")) return@withContext PrepararImagen.video(contexto, uri, p.ancho, p.alto, p.redonda)
        val crudo = contexto.contentResolver.openInputStream(uri)?.use { it.readBytes() } ?: error("No pude leerla.")
        if (Gif.esGif(crudo)) return@withContext PrepararImagen.gif(crudo, p.ancho, p.alto, p.redonda)
        val fuente = ImageDecoder.decodeBitmap(ImageDecoder.createSource(java.nio.ByteBuffer.wrap(crudo))) { d, _, _ ->
            d.allocator = ImageDecoder.ALLOCATOR_SOFTWARE
        }
        PrepararImagen.qoi(PrepararImagen.recortar(fuente, p.ancho, p.alto, p.redonda))
    }

    suspend fun subir(espacio: Int, qoi: ByteArray) {
        when (via) {
            Via.NUBE -> api!!.subirImagen(idRobot, espacio, qoi)
            else -> {
                cliente!!.subirImagen(espacio, qoi)
                cache.guardar(id, espacio, qoi, qoi)
            }
        }
    }

    fun borrarImagen(espacio: Int) = viewModelScope.launch {
        runCatching {
            when (via) {
                Via.NUBE -> api!!.borrarImagen(idRobot, espacio)
                else -> cliente!!.borrarImagen(espacio)
            }
        }.onFailure { errorImagen.value = it.message }
        cache.borrar(id, espacio)
        if (Editar.enLista(config.value, espacio) || Editar.protector(config.value) == espacio) cambiar { Editar.olvidarImagen(it, espacio) }
        refrescarImagenes()
    }

    /**
     * El onboarding, de una: el nombre, la primera imagen (en la bahía 01, en
     * la lista o de protector) y la config armada. Devuelve el error o null.
     */
    suspend fun encender(nombre: String, base: JsonObject, foto: ByteArray?, protector: Boolean): String? = try {
        var c = base
        val n = nombre.trim().take(24)
        if (n.isNotEmpty()) {
            val apodo = if (via == Via.NUBE) api!!.ponerApodo(idRobot, n) else n
            robot.value = robot.value?.copy(apodo = apodo)
        }
        if (foto != null) {
            subir(0, foto)
            c = if (protector) Editar.ponerProtector(c, 0) else Editar.mostrar(c, 0, true)
        }
        guardar(c)?.let { throw IllegalStateException(it) }
        aplicarEnVistaPrevia(c)
        marcarEncendido()
        refrescarImagenes()
        null
    } catch (e: ErrorApi) {
        e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
    } catch (e: Exception) {
        e.message ?: "No pude encenderlo."
    }

    // --- Ajustes ---

    /** El nombre: el apodo en la cuenta y, también, el de la config (el que muestra y al que responde). */
    fun ponerNombre(v: String) = viewModelScope.launch {
        val n = v.trim().take(24)
        try {
            if (via == Via.NUBE) {
                val apodo = api!!.ponerApodo(idRobot, n)
                robot.value = robot.value?.copy(apodo = apodo)
            } else {
                robot.value = robot.value?.copy(apodo = n)
                if (via == Via.CERCA) Vinculados(getApplication()).buscar(id)?.let { Vinculados(getApplication()).guardar(it.copy(nombre = n)) }
            }
            if (n.isNotEmpty()) cambiar { it.con("nombre", com.sinteticalabs.app.consola.cortarBytes(n, 24)) }
        } catch (e: Exception) {
            errorGuardado.value = e.message ?: "No pude guardar el nombre."
        }
    }

    /** Sacarlo de la cuenta (o, si es de cerca, desvincularlo). */
    fun quitar(listo: () -> Unit) = viewModelScope.launch {
        try {
            when (via) {
                Via.NUBE -> api!!.quitar(idRobot)
                Via.CERCA -> {
                    runCatching { cliente?.desvincular() }
                    Vinculados(getApplication()).olvidar(id)
                }
                Via.DEMO -> {}
            }
            listo()
        } catch (e: Exception) {
            errorGuardado.value = e.message ?: "No pude sacarlo."
        }
    }

    // --- Solo de cerca (BLE): el WiFi y sumarlo a la cuenta ---

    val mensajeCerca = MutableStateFlow<String?>(null)
    val ocupadoCerca = MutableStateFlow(false)

    fun configurarRed(ssid: String, clave: String) = viewModelScope.launch {
        ocupadoCerca.value = true
        mensajeCerca.value = runCatching { cliente?.configurarRed(ssid, clave); "Conectándose a $ssid" }.getOrElse { it.message ?: "No pude." }
        ocupadoCerca.value = false
    }

    /** La API da credenciales nuevas, se las pasamos por BLE y, cuando se conecta con ellas, queda en la cuenta. */
    fun sumarACuenta() = viewModelScope.launch {
        val c = cliente ?: return@launch
        ocupadoCerca.value = true
        try {
            val s = cuenta.actual()
            if (!s.activa) throw IllegalStateException("Primero entrá a tu cuenta.")
            val a = cuenta.api(s)
            val e = c.estado.value
            if (e?.redConfigurada != true) throw IllegalStateException("Primero pasale el WiFi.")
            mensajeCerca.value = "Pidiendo la clave…"
            val cred = a.reclamar(idRobot, e.perfil)
            c.configurarNube(cred.url, cred.usuario, cred.clave)
            mensajeCerca.value = "Esperando que se conecte…"
            mensajeCerca.value = if (a.esperarReclamo(idRobot)) "En tu cuenta" else "No se conectó: revisá el WiFi."
        } catch (e: ErrorApi) {
            mensajeCerca.value = e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else ""
        } catch (e: Exception) {
            mensajeCerca.value = e.message ?: "No pude."
        } finally {
            ocupadoCerca.value = false
        }
    }

    // --- El firmware por BLE (de cerca) ---

    val progresoOta = MutableStateFlow<Float?>(null)

    fun instalarPorBle(f: com.sinteticalabs.app.datos.Firmware) = viewModelScope.launch {
        val c = cliente ?: return@launch
        val a = api ?: return@launch
        try {
            progresoOta.value = 0f
            mensajeCerca.value = "Bajando ${f.version}…"
            val man = a.manifiesto(f.id)
            val bin = a.binario(f.id)
            mensajeCerca.value = "No lo alejes…"
            c.instalarFirmware(man, bin) { progresoOta.value = it }
            mensajeCerca.value = "Listo: reinicia con la ${f.version}"
        } catch (e: Exception) {
            mensajeCerca.value = e.message ?: "No pude actualizarlo."
        } finally {
            progresoOta.value = null
        }
    }

    val estadoCerca get() = cliente?.estado

    /** El encendido (onboarding) ya se vio en este celu. */
    fun encendidoVisto(): Boolean = getApplication<Application>().getSharedPreferences("consola", 0).getBoolean("encendido:$idRobot", false)
    fun marcarEncendido() { getApplication<Application>().getSharedPreferences("consola", 0).edit().putBoolean("encendido:$idRobot", true).apply() }

    override fun onCleared() {
        cliente?.cerrar()
        Instantaneas.pausado = false
    }
}

/** La config con la que arranca una pantalla sin robot (la de fábrica). */
val CONFIG_DE_FABRICA: JsonObject = buildJsonObject { put("version", 2) }
