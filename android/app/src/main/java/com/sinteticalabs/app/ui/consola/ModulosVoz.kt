package com.sinteticalabs.app.ui.consola

import android.content.Context
import android.media.MediaPlayer
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Voz
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.IAPropia
import com.sinteticalabs.app.datos.PaqueteVoz
import com.sinteticalabs.app.datos.VozEdge
import com.sinteticalabs.app.datos.VozEleven
import com.sinteticalabs.app.datos.generarPaquete
import com.sinteticalabs.app.datos.ia
import com.sinteticalabs.app.datos.paqueteVoz
import com.sinteticalabs.app.datos.ponerIa
import com.sinteticalabs.app.datos.ponerVozEdge
import com.sinteticalabs.app.datos.ponerVozEleven
import com.sinteticalabs.app.datos.probarVoz
import com.sinteticalabs.app.datos.sacarIa
import com.sinteticalabs.app.datos.sacarVozEleven
import com.sinteticalabs.app.datos.vozEdge
import com.sinteticalabs.app.datos.vozEleven
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

private val SOBRE = Color(0xCC1A1714)

/** Una sola prueba sonando a la vez. */
private object Parlante {
    private var mp: MediaPlayer? = null
    suspend fun sonar(c: Context, mp3: ByteArray) {
        val f = withContext(Dispatchers.IO) { java.io.File(c.cacheDir, "prueba-voz.mp3").apply { writeBytes(mp3) } }
        mp?.release()
        mp = MediaPlayer().apply { setDataSource(f.path); prepare(); start(); setOnCompletionListener { it.release(); if (mp === it) mp = null } }
    }
}

private fun mensaje(e: Throwable, por: String) = if (e is ErrorApi) e.message ?: por else e.message ?: por

/** Un medidor: cuánto va de hoy (de = 0, sin tope). */
@Composable
fun Medidor(nombre: String, valor: Int, de: Int, mostrar: (Int) -> String = { "$it" }, modifier: Modifier = Modifier) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado(nombre, color = SOBRE)
            Grabado(if (de > 0) "${mostrar(valor)}/${mostrar(de)}" else mostrar(valor), color = SOBRE)
        }
        Canvas(Modifier.fillMaxWidth().height(8.dp)) {
            drawRoundRect(C.hundido, cornerRadius = CornerRadius(4f))
            val f = if (de > 0) (valor.toFloat() / de).coerceIn(0f, 1f) else 0f
            if (f > 0) drawRoundRect(if (f >= 1f) C.rojo else C.led, size = Size(size.width * f, size.height), cornerRadius = CornerRadius(4f))
        }
    }
}

/**
 * Leer en voz alta: la voz gratis (Edge) o la de tu cuenta de ElevenLabs (tu
 * clave, tus topes). La llave la prende o la apaga (web: modulos/LeerVoz.svelte).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SeccionLeerVoz(vm: RobotViewModel) {
    val api = vm.apiNube() ?: return
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    val r = vm.robot.value
    val nombre = r?.apodo?.ifEmpty { null } ?: "tu robot"
    var edge by remember { mutableStateOf<VozEdge?>(null) }
    var eleven by remember { mutableStateOf<VozEleven?>(null) }
    var pestana by remember { mutableStateOf("edge") }
    var clave by remember { mutableStateOf("") }
    var ocupado by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    suspend fun cargar(primera: Boolean = false) {
        val e = runCatching { api.vozEdge() }.getOrNull()
        val x = runCatching { api.vozEleven() }.getOrNull()
        edge = e; eleven = x
        if (primera) pestana = Voz.pestanaInicial(e?.disponible == true, x?.conectado == true, x?.leer == true)
    }
    LaunchedEffect(Unit) {
        cargar(true)
        EnVivo.eventos.collect { if (it.tipo == "voz") cargar() }
    }
    suspend fun ponerEdge(c: kotlinx.serialization.json.JsonObject) { runCatching { edge = api.ponerVozEdge(c) }.onFailure { error = mensaje(it, "No pude cambiarla.") } }
    suspend fun ponerEleven(c: kotlinx.serialization.json.JsonObject): Boolean =
        runCatching { eleven = api.ponerVozEleven(c) }.onFailure { error = mensaje(it, "No pude cambiarla.") }.isSuccess
    val leyendoEleven = eleven?.conectado == true && eleven?.leer == true
    val leyendo = leyendoEleven || (edge?.disponible == true && edge?.prendida == true)
    fun probar(cual: String) = alcance.launch {
        error = null; ocupado = "probar"
        runCatching { Parlante.sonar(contexto, api.probarVoz(cual, "¡Hola! Soy $nombre.")) }.onFailure { error = mensaje(it, "No sonó.") }
        ocupado = ""
    }
    Bandeja {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Llave("Leer en voz alta", leyendo, {
                alcance.launch {
                    if (leyendo) {
                        if (leyendoEleven) ponerEleven(buildJsonObject { put("leer", false) })
                        if (edge?.prendida == true) ponerEdge(buildJsonObject { put("prendida", false) })
                    } else if (pestana == "eleven" && eleven?.conectado == true) ponerEleven(buildJsonObject { put("leer", true) })
                    else if (edge?.disponible == true) ponerEdge(buildJsonObject { put("prendida", true) })
                }
            }, Modifier.weight(1f))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            if (edge?.disponible == true) Tecla("Gratis", {
                pestana = "edge"
                alcance.launch {
                    if (leyendoEleven) { ponerEleven(buildJsonObject { put("leer", false) }); if (edge?.prendida != true) ponerEdge(buildJsonObject { put("prendida", true) }) }
                }
            }, Modifier.weight(1f), acabado = if (pestana == "edge") Acabado.NEGRA else Acabado.BLANCA, chica = true, apretada = pestana == "edge")
            Tecla("ElevenLabs", {
                pestana = "eleven"
                alcance.launch { if (eleven?.conectado == true && eleven?.leer != true) ponerEleven(buildJsonObject { put("leer", true) }) }
            }, Modifier.weight(1f), acabado = if (pestana == "eleven") Acabado.NEGRA else Acabado.BLANCA, chica = true, apretada = pestana == "eleven")
        }
        error?.let { AvisoLinea(it, error = true) }
        val e = edge
        val x = eleven
        if (pestana == "edge" && e?.disponible == true) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Visor(Modifier.weight(1f)) {
                    Grabado("Voz")
                    Selector(e.voces.firstOrNull { it.id == e.voz }?.nombre ?: e.voz, e.voces.map { it.id to it.nombre },
                        { v -> alcance.launch { ponerEdge(buildJsonObject { put("voz", v) }) } }, estilo = T.dato.copy(color = C.papel))
                }
                Tecla(if (ocupado == "probar") "…" else "▶ Probar", { probar("edge") }, acabado = Acabado.NARANJA, chica = true, habilitada = ocupado != "probar")
            }
            Medidor("Letras hoy", e.usados, e.tope, Voz::miles)
        } else if (x != null && !x.conectado) {
            Visor(Modifier.fillMaxWidth()) {
                Grabado("Clave de ElevenLabs")
                CampoVisor(clave, { clave = it }, placeholder = "sk_…", estilo = T.dato.copy(color = C.papel), maxBytes = 200,
                    teclado = androidx.compose.ui.text.input.KeyboardType.Password, alEscribir = { clave = it })
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                Tecla(if (ocupado == "conectar") "…" else "Conectar", {
                    alcance.launch { ocupado = "conectar"; if (ponerEleven(buildJsonObject { put("clave", clave.trim()) })) clave = ""; ocupado = "" }
                }, acabado = Acabado.NARANJA, chica = true, habilitada = clave.isNotBlank() && ocupado != "conectar")
                Tecla("Sacar una clave ↗", { abrirUrl(contexto, "https://elevenlabs.io/app/settings/api-keys") }, acabado = Acabado.NEGRA, chica = true)
            }
        } else if (x?.conectado == true) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Visor(Modifier.weight(1f)) {
                    Grabado("Voz de tu cuenta")
                    Selector(x.voces.firstOrNull { it.id == x.voz }?.nombre ?: x.voz, x.voces.map { it.id to it.nombre },
                        { v -> alcance.launch { ponerEleven(buildJsonObject { put("voz", v) }) } }, estilo = T.dato.copy(color = C.papel))
                }
                Tecla(if (ocupado == "probar") "…" else "▶ Probar", { probar("elevenlabs") }, acabado = Acabado.NARANJA, chica = true, habilitada = ocupado != "probar")
            }
            var letras by remember(x.tope) { mutableStateOf(Voz.escalon(Voz.LETRAS, x.tope).toDouble()) }
            var minutos by remember(x.topeMinutos) { mutableStateOf(Voz.escalon(Voz.MINUTOS, x.topeMinutos).toDouble()) }
            LaunchedEffect(letras, minutos) {
                val l = Voz.LETRAS[letras.toInt()]
                val m = Voz.MINUTOS[minutos.toInt()]
                if (l == x.tope && m == x.topeMinutos) return@LaunchedEffect
                delay(700)
                ponerEleven(buildJsonObject { put("tope", l); put("tope_minutos", m) })
            }
            FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly, verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Perilla(letras, "Letras/día", { letras = it }, Modifier.width(84.dp), min = 0.0, max = (Voz.LETRAS.size - 1).toDouble(), paso = 1.0, tamano = 52.dp,
                    mostrar = { Voz.miles(Voz.LETRAS[it.toInt()]) })
                Perilla(minutos, "Min/día", { minutos = it }, Modifier.width(84.dp), min = 0.0, max = (Voz.MINUTOS.size - 1).toDouble(), paso = 1.0, tamano = 52.dp,
                    mostrar = { Voz.MINUTOS[it.toInt()].let { m -> if (m > 0) "$m" else "Off" } })
            }
            Medidor("Letras hoy", x.usados, x.tope, Voz::miles)
            Medidor("Min hoy", Math.round(x.segundos / 60.0).toInt(), x.topeMinutos)
            Tecla(if (ocupado == "desconectar") "…" else "Desconectar", {
                alcance.launch { ocupado = "desconectar"; runCatching { eleven = api.sacarVozEleven() }.onFailure { error = mensaje(it, "No pude.") }; ocupado = "" }
            }, acabado = Acabado.NEGRA, chica = true)
        }
    }
}

fun abrirUrl(c: Context, u: String) {
    runCatching { c.startActivity(android.content.Intent(android.content.Intent.ACTION_VIEW, android.net.Uri.parse(u)).addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)) }
}

/**
 * El paquete de voz sin internet: las frases que dice solo, grabadas con su
 * voz. Con ElevenLabs gasta de tu cuenta: antes se muestra cuánto y se confirma.
 */
@Composable
fun SeccionPaqueteVoz(vm: RobotViewModel) {
    val api = vm.apiNube() ?: return
    val alcance = rememberCoroutineScope()
    var p by remember { mutableStateOf<PaqueteVoz?>(null) }
    var confirmar by remember { mutableStateOf<PaqueteVoz?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var pidiendo by remember { mutableStateOf(false) }
    suspend fun cargar() { p = runCatching { api.paqueteVoz(vm.idRobot) }.getOrNull() }
    LaunchedEffect(Unit) {
        cargar()
        launch { while (true) { delay(3000); if (p?.estado == "generando") cargar() } }
        EnVivo.eventos.collect { e -> if ((e.tipo == "voz-paquete" && e.robot == vm.idRobot) || e.tipo == "voz") cargar() }
    }
    fun generar(si: Boolean) = alcance.launch {
        error = null; pidiendo = true
        runCatching { api.generarPaquete(vm.idRobot, si) }
            .onSuccess { (e, x) -> if (e == 409 && x != null) confirmar = x else { if (x != null) p = x; confirmar = null } }
            .onFailure { error = mensaje(it, "No pude pedirlo.") }
        pidiendo = false
    }
    val x = p ?: return
    if (!x.disponible) return
    Bandeja {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Led(when (x.estado) { "listo" -> C.verde; "parcial", "generando" -> C.ambar; "error" -> C.rojo; else -> null })
            Grabado("Sin internet · " + (Voz.ETIQUETA_PAQUETE[x.estado] ?: x.estado), Modifier.weight(1f), color = SOBRE)
            Grabado(if (x.proveedor == "elevenlabs") "ElevenLabs" else "Gratis", color = SOBRE)
        }
        val hay = x.version > 0
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            Visor(Modifier.weight(1f)) { Grabado("Versión"); Text(if (hay) "v${x.version}" else "—", style = T.digitosChicos.copy(fontSize = 18.sp)) }
            Visor(Modifier.weight(1f)) { Grabado("Frases"); Text("${x.frases}/${x.total}", style = T.digitosChicos.copy(fontSize = 18.sp)) }
            Visor(Modifier.weight(1f)) { Grabado("MB"); Text(Voz.megas(if (hay) x.bytes else x.bytesEstimados) + if (hay) "" else "≈", style = T.digitosChicos.copy(fontSize = 18.sp)) }
        }
        if (x.estado == "generando" && x.de > 0) Canvas(Modifier.fillMaxWidth().height(8.dp)) {
            drawRoundRect(C.hundido, cornerRadius = CornerRadius(4f))
            drawRoundRect(C.led, size = Size(size.width * (x.hechas.toFloat() / x.de).coerceIn(0f, 1f), size.height), cornerRadius = CornerRadius(4f))
        }
        x.error?.let { AvisoLinea(it, error = true) }
        error?.let { AvisoLinea(it, error = true) }
        val c = confirmar
        if (c != null) Visor(Modifier.fillMaxWidth()) {
            Grabado("Gasta de tu cuenta")
            Text("${Voz.miles(c.gasta)} letras", style = T.digitosChicos)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Tecla("Generar", { generar(true) }, acabado = Acabado.NARANJA, chica = true, habilitada = !pidiendo)
                Tecla("Cancelar", { confirmar = null }, acabado = Acabado.NEGRA, chica = true)
            }
        } else if (Voz.hayQueGenerar(x.estado, hay, x.faltan, x.voz, x.vozNueva)) {
            Tecla(if (pidiendo) "…" else (if (hay) "Actualizar" else "Generar") + if (x.proveedor == "elevenlabs" && x.gasta > 0) " · ${Voz.miles(x.gasta)} letras" else "",
                { generar(false) }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA, habilitada = !pidiendo)
        }
    }
}

/** La IA de la charla: la del servidor, la tuya (tu clave y tu tope) o ninguna (su motor propio). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SeccionIAPropia(vm: RobotViewModel) {
    val api = vm.apiNube() ?: return
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var ia by remember { mutableStateOf<IAPropia?>(null) }
    var servicio by remember { mutableStateOf("groq") }
    var url by remember { mutableStateOf(Voz.SERVICIOS[1].url) }
    var modelo by remember { mutableStateOf(Voz.SERVICIOS[1].modelo) }
    var clave by remember { mutableStateOf("") }
    var abierto by remember { mutableStateOf(false) }
    var ocupado by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    suspend fun cargar() { ia = runCatching { api.ia() }.getOrNull() }
    LaunchedEffect(Unit) {
        cargar()
        EnVivo.eventos.collect { if (it.tipo == "ia") cargar() }
    }
    val x = ia ?: return
    val s = Voz.SERVICIOS.first { it.clave == servicio }
    val cual = Voz.cualIA(x.propia, x.servidor)
    Bandeja {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(C.tinta), contentAlignment = Alignment.Center) { Text("💬", style = T.tecla) }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Grabado("Charla · con qué IA", color = SOBRE)
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    listOf("servidor" to "Servidor", "tuya" to "Tuya", "ninguna" to "Motor").forEach { (k, n) ->
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                            Led(if (cual == k) (if (k == "ninguna") C.ambar else C.verde) else null, tamano = 7.dp)
                            Text(n.uppercase(), style = T.grabado, color = if (cual == k) C.tinta else SOBRE)
                        }
                    }
                }
            }
            if (!x.propia && !abierto) Tecla("+ Tu clave", { abierto = true }, acabado = Acabado.NARANJA, chica = true)
        }
        error?.let { AvisoLinea(it, error = true) }
        if (x.propia) {
            var tope by remember(x.tope) { mutableStateOf(Voz.escalon(Voz.TOPES_IA, x.tope).toDouble()) }
            LaunchedEffect(tope) {
                val t = Voz.TOPES_IA[tope.toInt()]
                if (t == x.tope) return@LaunchedEffect
                delay(700)
                runCatching { ia = api.ponerIa(buildJsonObject { put("tope", t) }) }.onFailure { error = mensaje(it, "No pude cambiarla.") }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Visor(Modifier.weight(1f)) { Grabado("Servicio"); Text(Voz.servicioDe(x.proveedor, x.urlPista), style = T.dato.copy(color = C.papel), maxLines = 1, overflow = TextOverflow.Ellipsis) }
                Visor(Modifier.weight(1f)) {
                    Grabado("Modelo")
                    CampoVisor(x.modelo, { m -> if (m.isNotBlank() && m != x.modelo) alcance.launch { runCatching { ia = api.ponerIa(buildJsonObject { put("modelo", m.trim()) }) }.onFailure { error = mensaje(it, "No pude.") } } },
                        estilo = T.dato.copy(color = C.papel), maxBytes = 120)
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Perilla(tope, "Tope/día", { tope = it }, Modifier.width(84.dp), min = 0.0, max = (Voz.TOPES_IA.size - 1).toDouble(), paso = 1.0, tamano = 50.dp,
                    mostrar = { Voz.TOPES_IA[it.toInt()].let { t -> if (t > 0) "$t" else "∞" } })
                Medidor("Hoy", x.usadas, x.tope, modifier = Modifier.weight(1f))
            }
            Tecla("Borrar", { alcance.launch { runCatching { api.sacarIa() }.onFailure { error = mensaje(it, "No pude.") }; cargar() } }, acabado = Acabado.NEGRA, chica = true)
        } else if (abierto) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Voz.SERVICIOS.forEach { v ->
                    Tecla(v.nombre, { servicio = v.clave; url = v.url; modelo = v.modelo }, chica = true, apretada = servicio == v.clave, alto = 34.dp)
                }
            }
            if (s.proveedor == "compatible") Visor(Modifier.fillMaxWidth()) {
                Grabado("URL")
                CampoVisor(url, { url = it }, placeholder = "https://…/v1", estilo = T.dato.copy(color = C.papel), maxBytes = 300,
                    teclado = androidx.compose.ui.text.input.KeyboardType.Uri, alEscribir = { url = it })
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Visor(Modifier.weight(1f)) { Grabado("Modelo"); CampoVisor(modelo, { modelo = it }, estilo = T.dato.copy(color = C.papel), maxBytes = 120, alEscribir = { modelo = it }) }
                Visor(Modifier.weight(1f)) {
                    Grabado("Clave")
                    CampoVisor(clave, { clave = it }, estilo = T.dato.copy(color = C.papel), maxBytes = 300, teclado = androidx.compose.ui.text.input.KeyboardType.Password, alEscribir = { clave = it })
                }
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Tecla(if (ocupado) "…" else "Conectar", {
                    alcance.launch {
                        ocupado = true; error = null
                        runCatching { ia = api.ponerIa(Voz.cuerpoIA(s.proveedor, clave, modelo, url)); clave = ""; abierto = false }.onFailure { error = mensaje(it, "No pude conectarla.") }
                        ocupado = false
                    }
                }, acabado = Acabado.NARANJA, chica = true, habilitada = !ocupado && Voz.puedeConectar(s.proveedor, clave, modelo, url))
                Tecla("Cancelar", { abierto = false }, acabado = Acabado.NEGRA, chica = true)
                if (s.claves.isNotEmpty()) Tecla("Sacar una clave ↗", { abrirUrl(contexto, s.claves) }, acabado = Acabado.NEGRA, chica = true)
            }
        } else if (x.servidor) Medidor("Hoy", x.usadas, x.tope)
    }
}
